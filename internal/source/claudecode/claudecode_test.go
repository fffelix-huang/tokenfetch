package claudecode

import (
	"bufio"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fffelix-huang/tokenfetch/internal/usage"
)

func parseFixture(t *testing.T) []usage.Event {
	t.Helper()
	f, err := os.Open("testdata/session.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	s := &Source{}
	var evs []usage.Event
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		ev, ok, err := s.ParseLine(sc.Bytes())
		if err != nil {
			t.Fatalf("ParseLine: %v", err)
		}
		if ok {
			evs = append(evs, ev)
		}
	}
	return evs
}

func TestParseLine(t *testing.T) {
	evs := parseFixture(t)
	if len(evs) != 4 {
		t.Fatalf("got %d events, want 4 (user, system, synthetic skipped)", len(evs))
	}

	final := evs[1]
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
	if final != want {
		t.Errorf("final line:\n got %+v\nwant %+v", final, want)
	}
	if evs[0].MessageID != evs[1].MessageID || evs[0].Output != 5 {
		t.Errorf("partial streaming line should share id with partial output, got %+v", evs[0])
	}
}

func TestParseLineLegacyCacheWrite(t *testing.T) {
	ev := parseFixture(t)[2]
	if ev.CacheWrite5m != 100 || ev.CacheWrite1h != 0 {
		t.Errorf("unsplit cache_creation should count as 5m, got %+v", ev.Tokens)
	}
}

func TestParseLineSubagentAndGeo(t *testing.T) {
	ev := parseFixture(t)[3]
	if ev.Agent != "general-purpose" || ev.Geo != "us" {
		t.Errorf("got agent=%q geo=%q", ev.Agent, ev.Geo)
	}
}

func TestFiles(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{
		"projects/-home-u-proj/s1.jsonl",
		"projects/-home-u-proj/s1/subagents/agent-a1.jsonl",
		"projects/-home-u-proj/s1/subagents/agent-a1.meta.json",
	} {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	s := &Source{Roots: []string{root, filepath.Join(root, "missing")}}
	files, err := s.Files()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Errorf("got %v, want 2 jsonl files", files)
	}
}
