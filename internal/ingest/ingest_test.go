package ingest

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fffelix-huang/tokenfetch/internal/report"
	"github.com/fffelix-huang/tokenfetch/internal/source/claudecode"
	"github.com/fffelix-huang/tokenfetch/internal/store"
	"github.com/fffelix-huang/tokenfetch/internal/usage"
)

const (
	partial = `{"type":"assistant","timestamp":"2026-09-16T14:58:33Z","sessionId":"s1","cwd":"/p","message":{"id":"msg_1","model":"claude-opus-5","usage":{"input_tokens":2,"output_tokens":5}}}`
	final   = `{"type":"assistant","timestamp":"2026-09-16T14:58:35Z","sessionId":"s1","cwd":"/p","message":{"id":"msg_1","model":"claude-opus-5","usage":{"input_tokens":2,"output_tokens":300}}}`
	other   = `{"type":"assistant","timestamp":"2026-09-16T16:10:00Z","sessionId":"s2","cwd":"/q","message":{"id":"msg_2","model":"claude-opus-5","usage":{"input_tokens":10,"output_tokens":10}}}`
)

func setup(t *testing.T) (*store.Store, []usage.Source, string) {
	t.Helper()
	root := t.TempDir()
	log := filepath.Join(root, "projects", "-p", "s1.jsonl")
	if err := os.MkdirAll(filepath.Dir(log), 0o755); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "cache.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st, []usage.Source{&claudecode.Source{Roots: []string{root}}}, log
}

func appendTo(t *testing.T, path, s string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(s); err != nil {
		t.Fatal(err)
	}
}

func build(t *testing.T, st *store.Store, loc *time.Location) *report.Report {
	t.Helper()
	rng, _ := report.NewRange("all", time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC))
	rep, err := report.Build(st, rng, loc)
	if err != nil {
		t.Fatal(err)
	}
	return rep
}

func TestIncrementalDedup(t *testing.T) {
	st, srcs, log := setup(t)

	// Unterminated line must not be consumed yet.
	appendTo(t, log, partial+"\n"+final[:40])
	if _, err := Run(st, srcs); err != nil {
		t.Fatal(err)
	}
	if rep := build(t, st, time.UTC); rep.Tokens.Output != 5 || rep.Messages != 1 {
		t.Fatalf("after partial write: output=%d messages=%d, want 5/1", rep.Tokens.Output, rep.Messages)
	}

	appendTo(t, log, final[40:]+"\n"+other+"\n")
	stats, err := Run(st, srcs)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Events != 2 {
		t.Errorf("second run parsed %d events, want 2 (only appended lines)", stats.Events)
	}
	rep := build(t, st, time.UTC)
	if rep.Tokens.Output != 310 || rep.Messages != 2 || rep.Sessions != 2 {
		t.Errorf("got output=%d messages=%d sessions=%d, want 310/2/2", rep.Tokens.Output, rep.Messages, rep.Sessions)
	}

	// Unchanged file: nothing rescanned.
	if stats, _ := Run(st, srcs); stats.FilesScanned != 0 {
		t.Errorf("unchanged file rescanned")
	}
}

func TestHalfHourTimezoneBuckets(t *testing.T) {
	st, srcs, log := setup(t)
	appendTo(t, log, final+"\n"+other+"\n")
	if _, err := Run(st, srcs); err != nil {
		t.Fatal(err)
	}
	ist := time.FixedZone("IST", 5*3600+30*60)
	rep := build(t, st, ist)
	// 14:58Z = 20:28 IST, 16:10Z = 21:40 IST
	if rep.HourOfDay[20].Total != 302 || rep.HourOfDay[21].Total != 20 {
		t.Errorf("hour 20=%d 21=%d, want 302/20", rep.HourOfDay[20].Total, rep.HourOfDay[21].Total)
	}
	if rep.PeakHour != 20 {
		t.Errorf("peak hour %d, want 20", rep.PeakHour)
	}
	// 2026-09-16 is a Wednesday.
	if rep.Heatmap[2][20].Total != 302 {
		t.Errorf("heatmap Wed 20h = %d, want 302", rep.Heatmap[2][20].Total)
	}
}
