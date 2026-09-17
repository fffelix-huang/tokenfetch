package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fffelix-huang/tokenfetch/internal/usage"
)

func insert(t *testing.T, st *Store, ev usage.Event) {
	t.Helper()
	tx, err := st.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Upsert(ev); err != nil {
		t.Fatal(err)
	}
	if err := tx.SetFileState("/log.jsonl", FileState{Offset: 1, Size: 1, MTime: time.Unix(1, 0)}); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func count(t *testing.T, st *Store) int64 {
	t.Helper()
	groups, err := st.Groups(time.Unix(0, 0), time.Unix(1<<40, 0))
	if err != nil {
		t.Fatal(err)
	}
	var n int64
	for _, g := range groups {
		n += g.Messages
	}
	return n
}

var ev = usage.Event{Provider: "p", MessageID: "m1", Timestamp: time.Unix(1_000_000, 0), Model: "claude-opus-5",
	Tokens: usage.Tokens{Output: 5}}

func TestForgetFilesKeepsEvents(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), fileName))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	insert(t, st, ev)
	if err := st.ForgetFiles(); err != nil {
		t.Fatal(err)
	}
	states, _ := st.FileStates()
	if len(states) != 0 || count(t, st) != 1 {
		t.Errorf("after ForgetFiles: %d file states, %d events; want 0, 1", len(states), count(t, st))
	}
}

func TestUpsertRefreshesAllColumns(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), fileName))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	insert(t, st, ev)
	fixed := ev
	fixed.Skill = "tdd"
	insert(t, st, fixed)
	groups, _ := st.Groups(time.Unix(0, 0), time.Unix(1<<40, 0))
	if len(groups) != 1 || groups[0].Skill != "tdd" {
		t.Errorf("re-read should refresh non-token columns, got %+v", groups)
	}
}

func TestReopenKeepsData(t *testing.T) {
	path := filepath.Join(t.TempDir(), fileName)
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	insert(t, st, ev)
	st.Close()
	if st, err = Open(path); err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if count(t, st) != 1 {
		t.Error("events lost on reopen")
	}
}

func TestNewerSchemaRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), fileName)
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	st.db.Exec("PRAGMA user_version = 99")
	st.Close()
	if _, err := Open(path); err == nil || !strings.Contains(err.Error(), "newer") {
		t.Errorf("want newer-schema error, got %v", err)
	}
}

func TestMoveLegacy(t *testing.T) {
	old := filepath.Join(t.TempDir(), "Caches", "tokenfetch", fileName)
	st, err := Open(old)
	if err != nil {
		t.Fatal(err)
	}
	insert(t, st, ev) // leaves data in the WAL while open
	newPath := filepath.Join(t.TempDir(), "Application Support", "tokenfetch", fileName)
	moved, err := MoveLegacy(old, newPath)
	st.Close()
	if err != nil || !moved {
		t.Fatalf("moved=%v err=%v", moved, err)
	}
	if _, err := os.Stat(filepath.Dir(old)); !os.IsNotExist(err) {
		t.Errorf("empty legacy dir should be removed")
	}
	st, err = Open(newPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if count(t, st) != 1 {
		t.Error("events lost in move")
	}
	if moved, _ := MoveLegacy(old, newPath); moved {
		t.Error("second move should be a no-op")
	}
}
