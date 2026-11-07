package codex

import (
	"bufio"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fffelix-huang/tokenfetch/internal/usage"
)

func parseFixture(t *testing.T) (evs []usage.Event, bad int) {
	t.Helper()
	f, err := os.Open("testdata/session.jsonl")
	require.NoError(t, err)
	defer f.Close()
	p := (&Source{}).NewParser()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		ev, ok, err := p.ParseLine(sc.Bytes())
		if err != nil {
			bad++
			continue
		}
		if ok {
			evs = append(evs, ev)
		}
	}
	require.NoError(t, sc.Err())
	return evs, bad
}

func TestParseLine(t *testing.T) {
	evs, bad := parseFixture(t)
	// r1 twice (dedup happens in the store), r2, r3; snapshots and messages skipped.
	require.Len(t, evs, 4)
	// r4 cached > input, missing response_id, r6 total mismatch, r7 reasoning > output.
	assert.Equal(t, 4, bad)

	assert.Equal(t, usage.Event{
		Provider:  Provider,
		MessageID: "r1",
		Timestamp: time.Date(2026, 11, 7, 17, 12, 40, 0, time.UTC),
		Model:     "gpt-6-luna",
		Project:   "/home/u/proj/sub", // turn cwd beats session cwd
		Session:   "s1",
		GitBranch: "main",
		Speed:     "flex", // thread service_tier
		// input_tokens includes cached reads; only the uncached rest is Input.
		Tokens: usage.Tokens{Input: 13002 - 10752, CacheRead: 10752, Output: 126},
	}, evs[0])
	assert.Equal(t, evs[0], evs[1], "repeated record parses identically")
}

func TestParseLineTurnContext(t *testing.T) {
	evs, _ := parseFixture(t)
	r2, r3 := evs[2], evs[3]
	assert.Equal(t, "gpt-6-sol", r2.Model, "model follows the turn, not the thread")
	assert.Equal(t, "/home/u/proj", r2.Project, "no turn cwd: session cwd")
	assert.Equal(t, "s1", r2.Session, "no session_id on record: session_meta")
	assert.Equal(t, "explorer", r2.Agent)
	// Cache writes are part of input_tokens too; reasoning stays inside output.
	assert.Equal(t, usage.Tokens{Input: 50_000, CacheRead: 300_000, CacheWrite5m: 50_000, Output: 2000}, r2.Tokens)
	assert.Equal(t, int64(402_000), r2.Total(), "totals match total_tokens")
	assert.Equal(t, UnknownModel, r3.Model, "turn without turn_context is not guessed")
}

func TestSpeed(t *testing.T) {
	for tier, want := range map[string]string{"": "", "default": "", "flex": "flex", "fast": "fast", "priority": "fast", "ultrafast": "ultrafast"} {
		assert.Equal(t, want, speed(tier), "tier %q", tier)
	}
}

func TestFiles(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{
		"sessions/2026/11/08/rollout-a.jsonl",
		"archived_sessions/rollout-b.jsonl",
		"sessions/2026/11/08/notes.txt",
		"history.jsonl", // not a session log
	} {
		full := filepath.Join(root, p)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, nil, 0o644))
	}
	files, err := (&Source{Roots: []string{root, filepath.Join(root, "missing")}}).Files()
	require.NoError(t, err)
	assert.Len(t, files, 2)
}
