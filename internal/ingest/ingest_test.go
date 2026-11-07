package ingest

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	require.NoError(t, os.MkdirAll(filepath.Dir(log), 0o755))
	st, err := store.Open(filepath.Join(t.TempDir(), "cache.db"))
	require.NoError(t, err)
	t.Cleanup(func() { st.Close() })
	return st, []usage.Source{&claudecode.Source{Roots: []string{root}}}, log
}

func appendTo(t *testing.T, path, s string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	require.NoError(t, err)
	defer f.Close()
	_, err = f.WriteString(s)
	require.NoError(t, err)
}

func build(t *testing.T, st *store.Store, loc *time.Location) *report.Report {
	t.Helper()
	rng, _ := report.NewRange("all", time.Date(2026, 9, 17, 0, 0, 0, 0, time.UTC))
	rep, err := report.Build(st, rng, loc)
	require.NoError(t, err)
	return rep
}

func TestIncrementalDedup(t *testing.T) {
	st, srcs, log := setup(t)

	// Unterminated line must not be consumed yet.
	appendTo(t, log, partial+"\n"+final[:40])
	_, err := Run(st, srcs)
	require.NoError(t, err)
	rep := build(t, st, time.UTC)
	require.EqualValues(t, 5, rep.Tokens.Output, "after partial write")
	require.EqualValues(t, 1, rep.Messages, "after partial write")

	appendTo(t, log, final[40:]+"\n"+other+"\n")
	stats, err := Run(st, srcs)
	require.NoError(t, err)
	assert.EqualValues(t, 2, stats.Events, "second run should parse only appended lines")
	rep = build(t, st, time.UTC)
	assert.EqualValues(t, 310, rep.Tokens.Output)
	assert.EqualValues(t, 2, rep.Messages)
	assert.EqualValues(t, 2, rep.Sessions)

	// Unchanged file: nothing rescanned.
	stats, _ = Run(st, srcs)
	assert.Zero(t, stats.FilesScanned, "unchanged file rescanned")
}

func TestHalfHourTimezoneBuckets(t *testing.T) {
	st, srcs, log := setup(t)
	appendTo(t, log, final+"\n"+other+"\n")
	_, err := Run(st, srcs)
	require.NoError(t, err)
	ist := time.FixedZone("IST", 5*3600+30*60)
	rep := build(t, st, ist)
	// 14:58Z = 20:28 IST, 16:10Z = 21:40 IST
	assert.EqualValues(t, 302, rep.HourOfDay[20].Total)
	assert.EqualValues(t, 20, rep.HourOfDay[21].Total)
	assert.EqualValues(t, 20, rep.PeakHour)
	// 2026-09-16 is a Wednesday.
	assert.EqualValues(t, 302, rep.Heatmap[2][20].Total, "heatmap Wed 20h")
}
