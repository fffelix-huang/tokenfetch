// Package claudecode reads Claude Code session transcripts (~/.claude/projects/**/*.jsonl).
package claudecode

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fffelix-huang/tokenfetch/internal/usage"
)

const Provider = "claude-code"

type Source struct {
	// Roots are Claude config dirs (containing projects/).
	Roots []string
}

// New returns a source over $CLAUDE_CONFIG_DIR, or ~/.claude and ~/.config/claude.
func New() *Source {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		var roots []string
		for _, d := range strings.Split(dir, ",") {
			if d = strings.TrimSpace(d); d != "" {
				roots = append(roots, d)
			}
		}
		return &Source{Roots: roots}
	}
	home, _ := os.UserHomeDir()
	return &Source{Roots: []string{
		filepath.Join(home, ".claude"),
		filepath.Join(home, ".config", "claude"),
	}}
}

func (s *Source) Name() string { return Provider }

func (s *Source) Files() ([]string, error) {
	var files []string
	for _, root := range s.Roots {
		dir := filepath.Join(root, "projects")
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
	return files, nil
}

type line struct {
	Type              string    `json:"type"`
	Timestamp         time.Time `json:"timestamp"`
	SessionID         string    `json:"sessionId"`
	Cwd               string    `json:"cwd"`
	GitBranch         string    `json:"gitBranch"`
	RequestID         string    `json:"requestId"`
	UUID              string    `json:"uuid"`
	AttributionSkill  string    `json:"attributionSkill"`
	AttributionPlugin string    `json:"attributionPlugin"`
	AttributionAgent  string    `json:"attributionAgent"`
	Message           struct {
		ID    string     `json:"id"`
		Model string     `json:"model"`
		Usage *lineUsage `json:"usage"`
	} `json:"message"`
}

type lineUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	CacheCreation            *struct {
		Ephemeral5m int64 `json:"ephemeral_5m_input_tokens"`
		Ephemeral1h int64 `json:"ephemeral_1h_input_tokens"`
	} `json:"cache_creation"`
	ServerToolUse *struct {
		WebSearchRequests int64 `json:"web_search_requests"`
	} `json:"server_tool_use"`
	Speed        string `json:"speed"`
	InferenceGeo string `json:"inference_geo"`
}

// ParseLine extracts usage from an assistant line. Callers must dedup by
// MessageID keeping max Output: one response spans several lines and early
// lines carry partial output_tokens.
func (s *Source) ParseLine(b []byte) (usage.Event, bool, error) {
	// Cheap prefilter: most lines are not assistant messages with usage.
	if !bytes.Contains(b, []byte(`"usage"`)) {
		return usage.Event{}, false, nil
	}
	var l line
	if err := json.Unmarshal(b, &l); err != nil {
		return usage.Event{}, false, err
	}
	u := l.Message.Usage
	if l.Type != "assistant" || u == nil || l.Message.Model == "" || l.Message.Model == "<synthetic>" {
		return usage.Event{}, false, nil
	}

	id := l.Message.ID
	if id == "" {
		id = l.RequestID
	}
	if id == "" {
		id = l.UUID
	}

	ev := usage.Event{
		Provider:  Provider,
		MessageID: id,
		Timestamp: l.Timestamp.UTC(),
		Model:     l.Message.Model,
		Project:   l.Cwd,
		Session:   l.SessionID,
		GitBranch: l.GitBranch,
		Skill:     l.AttributionSkill,
		Plugin:    l.AttributionPlugin,
		Agent:     l.AttributionAgent,
		Speed:     u.Speed,
		Tokens: usage.Tokens{
			Input:     u.InputTokens,
			Output:    u.OutputTokens,
			CacheRead: u.CacheReadInputTokens,
		},
	}
	if u.InferenceGeo != "not_available" {
		ev.Geo = u.InferenceGeo
	}
	if u.ServerToolUse != nil {
		ev.WebSearches = u.ServerToolUse.WebSearchRequests
	}
	// Older lines lack the TTL split; unsplit remainder is billed as 5m.
	if cc := u.CacheCreation; cc != nil {
		ev.CacheWrite1h = cc.Ephemeral1h
		ev.CacheWrite5m = cc.Ephemeral5m
	}
	if rest := u.CacheCreationInputTokens - ev.CacheWrite1h - ev.CacheWrite5m; rest > 0 {
		ev.CacheWrite5m += rest
	}
	return ev, true, nil
}
