// Package store is the SQLite database of parsed usage events.
//
// It is long-term history, not a cache: Claude Code prunes old transcripts, so
// some events exist only here. Never drop or wipe the events table; schema
// changes are append-only entries in migrations.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"time"

	_ "modernc.org/sqlite"

	"github.com/fffelix-huang/tokenfetch/internal/usage"
)

const fileName = "tokenfetch.db"

// migrations[i] upgrades PRAGMA user_version i to i+1.
var migrations = []string{schema}

const schema = `
CREATE TABLE files (
	path   TEXT PRIMARY KEY,
	offset INTEGER NOT NULL,
	size   INTEGER NOT NULL,
	mtime  INTEGER NOT NULL
);
CREATE TABLE events (
	provider       TEXT NOT NULL,
	message_id     TEXT NOT NULL,
	ts             INTEGER NOT NULL, -- unix seconds UTC
	model          TEXT NOT NULL,
	project        TEXT NOT NULL,
	session        TEXT NOT NULL,
	git_branch     TEXT NOT NULL,
	skill          TEXT NOT NULL,
	plugin         TEXT NOT NULL,
	agent          TEXT NOT NULL,
	speed          TEXT NOT NULL,
	geo            TEXT NOT NULL,
	input          INTEGER NOT NULL,
	output         INTEGER NOT NULL,
	cache_read     INTEGER NOT NULL,
	cache_write_5m INTEGER NOT NULL,
	cache_write_1h INTEGER NOT NULL,
	web_searches   INTEGER NOT NULL,
	PRIMARY KEY (provider, message_id)
);
CREATE INDEX events_ts ON events (ts);
`

type Store struct {
	db *sql.DB
}

// DataDirEnv overrides where the database lives.
const DataDirEnv = "TOKENFETCH_DATA_DIR"

// DefaultDataDir is the per-OS user data dir: ~/Library/Application Support/tokenfetch
// on macOS, %AppData%\tokenfetch on Windows, $XDG_DATA_HOME/tokenfetch elsewhere.
func DefaultDataDir() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", "tokenfetch"), nil
	case "windows":
		base, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(base, "tokenfetch"), nil
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "tokenfetch"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "tokenfetch"), nil
}

// Path is the database path: $TOKENFETCH_DATA_DIR/tokenfetch.db or the default data dir.
func Path() (string, error) {
	dir := os.Getenv(DataDirEnv)
	if dir == "" {
		var err error
		if dir, err = DefaultDataDir(); err != nil {
			return "", err
		}
	}
	return filepath.Join(dir, fileName), nil
}

// LegacyPath is where versions before the data-dir move kept the database.
func LegacyPath() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "tokenfetch", fileName), nil
}

// MoveLegacy moves the database (with WAL files) from old to path when path
// doesn't exist yet. WAL files move first so an interrupted move resumes safely.
func MoveLegacy(old, path string) (moved bool, err error) {
	if _, err := os.Stat(path); err == nil {
		return false, nil
	}
	if _, err := os.Stat(old); err != nil {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	for _, suffix := range []string{"-wal", "-shm", ""} {
		if err := os.Rename(old+suffix, path+suffix); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return false, err
		}
	}
	os.Remove(filepath.Dir(old)) // only succeeds if empty
	return true, nil
}

// Open opens or creates the database and applies pending migrations.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=synchronous(NORMAL)")
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	var v int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		return err
	}
	if v > len(migrations) {
		return fmt.Errorf("database schema v%d is newer than this tokenfetch (v%d); upgrade tokenfetch", v, len(migrations))
	}
	for ; v < len(migrations); v++ {
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(migrations[v]); err != nil {
			tx.Rollback()
			return fmt.Errorf("migrate to v%d: %w", v+1, err)
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", v+1)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// ForgetFiles clears ingest offsets so the next ingest re-reads every log.
// Events are kept: logs Claude Code already deleted can't be re-read.
func (s *Store) ForgetFiles() error {
	_, err := s.db.Exec("DELETE FROM files")
	return err
}

// FileState is how far a log file has been ingested.
type FileState struct {
	Offset, Size int64
	MTime        time.Time
}

func (s *Store) FileStates() (map[string]FileState, error) {
	rows, err := s.db.Query("SELECT path, offset, size, mtime FROM files")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]FileState{}
	for rows.Next() {
		var p string
		var st FileState
		var mt int64
		if err := rows.Scan(&p, &st.Offset, &st.Size, &mt); err != nil {
			return nil, err
		}
		st.MTime = time.Unix(0, mt)
		out[p] = st
	}
	return out, rows.Err()
}

// Tx batches ingestion writes.
type Tx struct {
	tx       *sql.Tx
	upsert   *sql.Stmt
	setState *sql.Stmt
}

func (s *Store) Begin() (*Tx, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	// Streaming writes one response over several lines with growing
	// output_tokens; keep the most complete one. Re-reads (--rebuild) refresh
	// every column so parser fixes apply.
	upsert, err := tx.Prepare(`
INSERT INTO events VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT (provider, message_id) DO UPDATE SET
	ts = excluded.ts, model = excluded.model, project = excluded.project, session = excluded.session,
	git_branch = excluded.git_branch, skill = excluded.skill, plugin = excluded.plugin, agent = excluded.agent,
	speed = excluded.speed, geo = excluded.geo,
	output = excluded.output, input = excluded.input, cache_read = excluded.cache_read,
	cache_write_5m = excluded.cache_write_5m, cache_write_1h = excluded.cache_write_1h,
	web_searches = excluded.web_searches
WHERE excluded.output >= events.output`)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	setState, err := tx.Prepare(`INSERT OR REPLACE INTO files VALUES (?,?,?,?)`)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	return &Tx{tx: tx, upsert: upsert, setState: setState}, nil
}

func (t *Tx) Upsert(e usage.Event) error {
	_, err := t.upsert.Exec(e.Provider, e.MessageID, e.Timestamp.Unix(), e.Model, e.Project, e.Session,
		e.GitBranch, e.Skill, e.Plugin, e.Agent, e.Speed, e.Geo,
		e.Input, e.Output, e.CacheRead, e.CacheWrite5m, e.CacheWrite1h, e.WebSearches)
	return err
}

func (t *Tx) SetFileState(path string, st FileState) error {
	_, err := t.setState.Exec(path, st.Offset, st.Size, st.MTime.UnixNano())
	return err
}

func (t *Tx) Commit() error   { return t.tx.Commit() }
func (t *Tx) Rollback() error { return t.tx.Rollback() }

// SlotSeconds is the bucket width. 15 min so every real UTC offset
// (+5:30, +5:45, ...) maps a slot to exactly one local hour.
const SlotSeconds = 900

// Group is events summed over one (slot, dimensions) combination.
type Group struct {
	Slot     time.Time // bucket start
	Provider string
	Model    string
	Project  string
	Skill    string
	Plugin   string
	Agent    string
	Speed    string
	Geo      string
	usage.Tokens
	WebSearches int64
	Messages    int64
}

// Groups returns slot groups with ts in [from, to).
func (s *Store) Groups(from, to time.Time) ([]Group, error) {
	rows, err := s.db.Query(`
SELECT ts / ?1 * ?1 AS slot, provider, model, project, skill, plugin, agent, speed, geo,
	SUM(input), SUM(output), SUM(cache_read), SUM(cache_write_5m), SUM(cache_write_1h),
	SUM(web_searches), COUNT(*)
FROM events WHERE ts >= ?2 AND ts < ?3
GROUP BY slot, provider, model, project, skill, plugin, agent, speed, geo
ORDER BY slot`, SlotSeconds, from.Unix(), to.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Group
	for rows.Next() {
		var g Group
		var h int64
		if err := rows.Scan(&h, &g.Provider, &g.Model, &g.Project, &g.Skill, &g.Plugin, &g.Agent, &g.Speed, &g.Geo,
			&g.Input, &g.Output, &g.CacheRead, &g.CacheWrite5m, &g.CacheWrite1h, &g.WebSearches, &g.Messages); err != nil {
			return nil, err
		}
		g.Slot = time.Unix(h, 0)
		out = append(out, g)
	}
	return out, rows.Err()
}

// Sessions counts distinct sessions with ts in [from, to).
func (s *Store) Sessions(from, to time.Time) (int64, error) {
	var n int64
	err := s.db.QueryRow(`SELECT COUNT(DISTINCT provider || ':' || session) FROM events WHERE ts >= ? AND ts < ?`,
		from.Unix(), to.Unix()).Scan(&n)
	return n, err
}
