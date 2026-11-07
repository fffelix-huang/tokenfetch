// Package usage defines the provider-agnostic usage record every source emits.
package usage

import "time"

// Event is one billed model response, normalized across providers.
type Event struct {
	Provider  string // e.g. "claude-code"
	MessageID string // dedup key, unique within Provider
	Timestamp time.Time

	Model     string
	Project   string // absolute working directory
	Session   string
	GitBranch string
	Skill     string // "" = none
	Plugin    string // "" = none
	Agent     string // subagent type, "" = main thread
	Speed     string // "standard" | "fast" | ""
	Geo       string // inference_geo, e.g. "us"

	Tokens
	WebSearches int64
}

// Tokens holds token counts by billing category.
type Tokens struct {
	Input        int64 `json:"input"`
	Output       int64 `json:"output"`
	CacheRead    int64 `json:"cache_read"`
	CacheWrite5m int64 `json:"cache_write_5m"`
	CacheWrite1h int64 `json:"cache_write_1h"`
}

// Total is the sum of all categories.
func (t Tokens) Total() int64 {
	return t.Input + t.Output + t.CacheRead + t.CacheWrite5m + t.CacheWrite1h
}

// Add accumulates o into t.
func (t *Tokens) Add(o Tokens) {
	t.Input += o.Input
	t.Output += o.Output
	t.CacheRead += o.CacheRead
	t.CacheWrite5m += o.CacheWrite5m
	t.CacheWrite1h += o.CacheWrite1h
}

// Source discovers and parses one agent's local usage logs.
type Source interface {
	// Name is the provider id stored on events.
	Name() string
	// Files lists log files to scan. Missing data dir is not an error.
	Files() ([]string, error)
	// NewParser returns a parser for one file; it is fed that file's lines in order.
	NewParser() Parser
}

// Parser decodes one file's log lines.
type Parser interface {
	// ParseLine decodes one log line; ok=false for lines without usage.
	ParseLine(line []byte) (ev Event, ok bool, err error)
}

// ContextParser is a Parser whose lines depend on earlier lines of the same
// file (e.g. Codex logs the model once per turn, not per response). When
// ingest resumes mid-file it first replays the already-ingested lines through
// Context, which records state and emits nothing.
type ContextParser interface {
	Parser
	Context(line []byte)
}
