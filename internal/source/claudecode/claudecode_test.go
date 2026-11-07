package claudecode

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

func parseFixture(t *testing.T) []usage.Event {
	t.Helper()
	f, err := os.Open("testdata/session.jsonl")
	require.NoError(t, err)
	defer f.Close()
	s := &Source{}
	var evs []usage.Event
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		ev, ok, err := s.ParseLine(sc.Bytes())
		require.NoError(t, err, "ParseLine")
		if ok {
			evs = append(evs, ev)
		}
	}
	return evs
}

func TestParseLine(t *testing.T) {
	evs := parseFixture(t)
	require.Len(t, evs, 4, "user, system, synthetic skipped")

	want := usage.Event{
		Provider:    Provider,
		MessageID:   "msg_1",
		Timestamp:   time.Date(2026, 9, 16, 14, 58, 35, 100e6, time.UTC),
		Model:       "claude-opus-5",
		Project:     "/home/u/proj",
		Session:     "s1",
		GitBranch:   "main",
		Skill:       "mattpocock-skills:tdd",
		Plugin:      "mattpocock-skills",
		Speed:       "fast",
		Tokens:      usage.Tokens{Input: 2, Output: 372, CacheRead: 27373, CacheWrite1h: 11084},
		WebSearches: 1,
	}
	assert.Equal(t, want, evs[1], "final line")
	// Partial streaming line shares id with the final one, with partial output.
	assert.Equal(t, evs[1].MessageID, evs[0].MessageID)
	assert.EqualValues(t, 5, evs[0].Output)
}

func TestParseLineLegacyCacheWrite(t *testing.T) {
	ev := parseFixture(t)[2]
	// Unsplit cache_creation counts as 5m.
	assert.EqualValues(t, 100, ev.CacheWrite5m)
	assert.Zero(t, ev.CacheWrite1h)
}

func TestParseLineSubagentAndGeo(t *testing.T) {
	ev := parseFixture(t)[3]
	assert.Equal(t, "general-purpose", ev.Agent)
	assert.Equal(t, "us", ev.Geo)
}

func TestFiles(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{
		"projects/-home-u-proj/s1.jsonl",
		"projects/-home-u-proj/s1/subagents/agent-a1.jsonl",
		"projects/-home-u-proj/s1/subagents/agent-a1.meta.json",
	} {
		full := filepath.Join(root, p)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, nil, 0o644))
	}
	s := &Source{Roots: []string{root, filepath.Join(root, "missing")}}
	files, err := s.Files()
	require.NoError(t, err)
	assert.Len(t, files, 2, "want 2 jsonl files")
}

// Lines that can't be keyed or placed in time are malformed, not silently
// merged into one event (empty id) or hidden before the epoch (no timestamp).
func TestParseLineRejectsUnkeyedOrUntimed(t *testing.T) {
	s := &Source{}
	for name, l := range map[string]string{
		"no id":        `{"type":"assistant","timestamp":"2026-09-16T14:58:35Z","message":{"model":"claude-opus-5","usage":{"output_tokens":1}}}`,
		"no timestamp": `{"type":"assistant","message":{"id":"msg_1","model":"claude-opus-5","usage":{"output_tokens":1}}}`,
	} {
		_, _, err := s.ParseLine([]byte(l))
		assert.Error(t, err, name)
	}
}
