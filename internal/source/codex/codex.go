// Package codex reads Codex CLI session logs (~/.codex/sessions/**/*.jsonl).
//
// Usage comes from token_usage_record lines, one per response. Model, cwd and
// service tier are logged once per turn or thread on earlier lines, so the
// parser is a usage.ContextParser that tracks them per file.
package codex

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fffelix-huang/tokenfetch/internal/usage"
)

const Provider = "codex"

// UnknownModel marks responses whose turn had no turn_context line; they are
// reported as unpriced rather than guessed from config.toml.
const UnknownModel = "unknown"

type Source struct {
	// Roots are Codex home dirs (containing sessions/).
	Roots []string
}

// New returns a source over $CODEX_HOME, or ~/.codex.
func New() *Source {
	if dir := os.Getenv("CODEX_HOME"); dir != "" {
		return &Source{Roots: []string{dir}}
	}
	home, _ := os.UserHomeDir()
	return &Source{Roots: []string{filepath.Join(home, ".codex")}}
}

func (s *Source) Name() string { return Provider }

func (s *Source) Files() ([]string, error) {
	var files []string
	for _, root := range s.Roots {
		for _, sub := range []string{"sessions", "archived_sessions"} {
			dir := filepath.Join(root, sub)
			if _, err := os.Stat(dir); err != nil {
				continue
			}
			err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return nil // unreadable subtree: skip
				}
				if !d.IsDir() && strings.HasSuffix(path, ".jsonl") {
					files = append(files, path)
				}
				return nil
			})
			if err != nil {
				return nil, err
			}
		}
	}
	return files, nil
}

func (s *Source) NewParser() usage.Parser {
	return &parser{turns: map[string]turn{}}
}

type turn struct{ model, cwd, agent string }

// parser holds one file's context: the session's metadata, the latest
// thread settings, and what each turn_context said about its turn.
type parser struct {
	session, cwd, branch string
	tier                 string
	turns                map[string]turn
}

type line struct {
	Timestamp time.Time       `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type tokenUsage struct {
	Input       int64 `json:"input_tokens"`
	CachedInput int64 `json:"cached_input_tokens"`
	CacheWrite  int64 `json:"cache_write_input_tokens"`
	Output      int64 `json:"output_tokens"`
	Reasoning   int64 `json:"reasoning_output_tokens"`
	Total       int64 `json:"total_tokens"`
}

// interesting are the record types the parser reads; everything else
// (messages, reasoning, tool calls) is skipped without a full decode.
var interesting = [][]byte{
	[]byte(`"type":"token_usage_record"`),
	[]byte(`"type":"turn_context"`),
	[]byte(`"type":"session_meta"`),
	[]byte(`"type":"event_msg"`),
}

func relevant(b []byte) bool {
	head := b[:min(len(b), 200)]
	if !bytes.Contains(head, []byte(`"type":"`)) {
		return true // unfamiliar layout: decode to be safe
	}
	for _, m := range interesting {
		if bytes.Contains(head, m) {
			return true
		}
	}
	return false
}

// Context records state from an already-ingested line.
func (p *parser) Context(b []byte) { p.ParseLine(b) }

// ParseLine updates per-file context and returns an event for each
// token_usage_record. Only the per-response usage delta is read:
// turn_token_usage, thread_token_usage and token_count snapshots are
// cumulative and would double-count.
func (p *parser) ParseLine(b []byte) (usage.Event, bool, error) {
	if !relevant(b) {
		return usage.Event{}, false, nil
	}
	var l line
	if err := json.Unmarshal(b, &l); err != nil {
		return usage.Event{}, false, err
	}
	switch l.Type {
	case "session_meta":
		var m struct {
			ID        string `json:"id"`
			SessionID string `json:"session_id"`
			Cwd       string `json:"cwd"`
			Git       *struct {
				Branch string `json:"branch"`
			} `json:"git"`
		}
		if err := json.Unmarshal(l.Payload, &m); err != nil {
			return usage.Event{}, false, err
		}
		p.session = firstNonEmpty(m.SessionID, m.ID)
		p.cwd = m.Cwd
		if m.Git != nil {
			p.branch = m.Git.Branch
		}
	case "turn_context":
		var t struct {
			TurnID string `json:"turn_id"`
			Model  string `json:"model"`
			Cwd    string `json:"cwd"`
		}
		if err := json.Unmarshal(l.Payload, &t); err != nil {
			return usage.Event{}, false, err
		}
		cur := p.turns[t.TurnID]
		cur.model, cur.cwd = t.Model, t.Cwd
		p.turns[t.TurnID] = cur
	case "event_msg":
		return usage.Event{}, false, p.eventMsg(l.Payload)
	case "token_usage_record":
		return p.usage(l)
	}
	return usage.Event{}, false, nil
}

func (p *parser) eventMsg(payload json.RawMessage) error {
	var e struct {
		Type     string `json:"type"`
		TurnID   string `json:"turn_id"`
		Settings struct {
			ServiceTier string `json:"service_tier"`
		} `json:"thread_settings"`
		Attribution struct {
			// null on the main thread; only a string is taken as an agent name.
			InitiatingAgentPath json.RawMessage `json:"initiating_agent_path"`
		} `json:"turn_attribution"`
	}
	switch {
	case bytes.Contains(payload, []byte(`"thread_settings_applied"`)),
		bytes.Contains(payload, []byte(`"task_started"`)):
	default:
		return nil // token_count snapshots, item events: not usage
	}
	if err := json.Unmarshal(payload, &e); err != nil {
		return err
	}
	switch e.Type {
	case "thread_settings_applied":
		p.tier = e.Settings.ServiceTier
	case "task_started":
		var agent string
		if json.Unmarshal(e.Attribution.InitiatingAgentPath, &agent) == nil && agent != "" {
			cur := p.turns[e.TurnID]
			cur.agent = agent
			p.turns[e.TurnID] = cur
		}
	}
	return nil
}

func (p *parser) usage(l line) (usage.Event, bool, error) {
	var r struct {
		ResponseID string      `json:"response_id"`
		SessionID  string      `json:"session_id"`
		TurnID     string      `json:"turn_id"`
		Usage      *tokenUsage `json:"usage"`
	}
	if err := json.Unmarshal(l.Payload, &r); err != nil {
		return usage.Event{}, false, err
	}
	if r.ResponseID == "" {
		return usage.Event{}, false, errors.New("token_usage_record without response_id")
	}
	if l.Timestamp.IsZero() {
		return usage.Event{}, false, errors.New("token_usage_record without timestamp")
	}
	if r.Usage == nil {
		return usage.Event{}, false, errors.New("token_usage_record without usage")
	}
	tokens, err := normalize(*r.Usage)
	if err != nil {
		return usage.Event{}, false, err
	}
	t := p.turns[r.TurnID]
	return usage.Event{
		Provider:  Provider,
		MessageID: r.ResponseID,
		Timestamp: l.Timestamp.UTC(),
		Model:     firstNonEmpty(t.model, UnknownModel),
		Project:   firstNonEmpty(t.cwd, p.cwd),
		Session:   firstNonEmpty(r.SessionID, p.session),
		GitBranch: p.branch,
		Agent:     t.agent,
		Speed:     speed(p.tier),
		Tokens:    tokens,
	}, true, nil
}

// normalize maps OpenAI usage onto tokenfetch's disjoint categories.
// input_tokens includes cached reads and cache writes (total_tokens ==
// input_tokens + output_tokens), and reasoning tokens are part of
// output_tokens, so neither is added again.
//
// Codex doesn't log the cache-write lifetime; writes are filed as the
// short-lifetime category (OpenAI's 30m tier, priced via CacheWrite5m).
func normalize(u tokenUsage) (usage.Tokens, error) {
	for _, v := range []int64{u.Input, u.CachedInput, u.CacheWrite, u.Output, u.Reasoning, u.Total} {
		if v < 0 {
			return usage.Tokens{}, errors.New("negative token count")
		}
	}
	uncached := u.Input - u.CachedInput - u.CacheWrite
	if uncached < 0 {
		return usage.Tokens{}, fmt.Errorf("cached+cache-write tokens exceed input (%d+%d > %d)", u.CachedInput, u.CacheWrite, u.Input)
	}
	if u.Reasoning > u.Output {
		return usage.Tokens{}, errors.New("reasoning tokens exceed output")
	}
	if u.Total != 0 && u.Total != u.Input+u.Output {
		return usage.Tokens{}, fmt.Errorf("total_tokens %d != input %d + output %d", u.Total, u.Input, u.Output)
	}
	return usage.Tokens{Input: uncached, Output: u.Output, CacheRead: u.CachedInput, CacheWrite5m: u.CacheWrite}, nil
}

// speed maps a Codex service_tier to tokenfetch's speed values.
func speed(tier string) string {
	switch tier {
	case "fast", "priority":
		return "fast"
	case "flex", "ultrafast":
		return tier
	}
	return ""
}

func firstNonEmpty(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}
