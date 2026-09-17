package render

import (
	"strings"
	"testing"
	"time"

	"github.com/fffelix-huang/tokenfetch/internal/report"
)

// Tests run without a TTY, so lipgloss emits no color codes.

func reportWithDays(t *testing.T, rangeName string, now time.Time, days map[string]int64) *report.Report {
	t.Helper()
	rng, ok := report.NewRange(rangeName, now)
	if !ok {
		t.Fatalf("bad range %q", rangeName)
	}
	r := &report.Report{Range: rng}
	for _, d := range dayRange(rng.From, now) {
		if v, ok := days[dateKey(d)]; ok {
			r.Daily = append(r.Daily, report.Day{Date: dateKey(d), Bucket: report.Bucket{Total: v}})
			r.Total += v
		}
	}
	return r
}

func TestDayBars(t *testing.T) {
	now := time.Date(2026, 9, 17, 23, 0, 0, 0, time.UTC) // Thursday
	out := dayBars(reportWithDays(t, "week", now, map[string]int64{"2026-09-16": 27_800_000, "2026-09-12": 500}))
	lines := strings.Split(out, "\n")
	if got := strings.Fields(lines[len(lines)-1]); strings.Join(got, " ") != "9/11 9/12 9/13 9/14 9/15 9/16 9/17" {
		t.Errorf("date row = %q", got)
	}
	if got := strings.Fields(lines[len(lines)-2]); got[0] != "Fri" || got[6] != "Thu" {
		t.Errorf("weekday row = %q", got)
	}
	if !strings.Contains(lines[1], "27.8M") || !strings.Contains(lines[1], "500") {
		t.Errorf("value row = %q", lines[1])
	}
}

func TestCalendar(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	out := calendar(reportWithDays(t, "month", now, map[string]int64{"2026-09-02": 10}))
	lines := strings.Split(out, "\n")
	// Range starts Wed Aug 19; first week row is indented three blank Sun/Mon/Tue cells.
	if !strings.HasPrefix(lines[1], "    Sun") || !strings.HasPrefix(lines[2], "Aug                   19 ··") {
		t.Errorf("first week row = %q", lines[2])
	}
	if !strings.Contains(out, " 2 ░░") || !strings.Contains(out, " 3 ··") {
		t.Errorf("fixed tiers: 10 tokens should be <10M level, idle day empty:\n%s", out)
	}
	if strings.Contains(out, "18 ") {
		t.Errorf("day after range end rendered:\n%s", out)
	}
}

func TestContributions(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	r := &report.Report{Range: report.Range{Name: "all", From: time.Unix(0, 0), To: now}}
	r.Daily = []report.Day{
		{Date: "2025-01-06", Bucket: report.Bucket{Total: 5e9}}, // older than a year: not shown
		{Date: "2026-09-16", Bucket: report.Bucket{Total: 2e9}},
		{Date: "2026-09-17", Bucket: report.Bucket{Total: 5e6}},
	}
	out := contributions(r, 0)
	lines := strings.Split(out, "\n")
	if n := len(lines); n != 10 { // title, months, 7 weekdays, legend
		t.Fatalf("got %d lines:\n%s", n, out)
	}
	if !strings.HasPrefix(lines[2], "Sun") || !strings.HasPrefix(lines[8], "Sat") {
		t.Errorf("rows should run Sun..Sat: first=%q last=%q", lines[2], lines[8])
	}
	if wed, thu := lines[5], lines[6]; !strings.HasSuffix(wed, "██") || !strings.HasSuffix(thu, "░░") {
		t.Errorf("last column: Wed=%q Thu=%q", wed, thu)
	}
	if strings.Count(out, "██") != 2 { // Wed cell + legend
		t.Errorf("unexpected ≥1B cells:\n%s", out)
	}
	for _, line := range strings.Split(contributions(r, 70), "\n") {
		if w := len([]rune(line)); w > 70 {
			t.Errorf("narrow terminal: line width %d > 70: %q", w, line)
		}
	}
}

func TestTier(t *testing.T) {
	for v, want := range map[int64]int{0: 0, 1: 1, 9_999_999: 1, 10_000_000: 2, 99_999_999: 2, 100_000_000: 3, 999_999_999: 3, 1_000_000_000: 4} {
		if got := tier(v); got != want {
			t.Errorf("tier(%d) = %d, want %d", v, got, want)
		}
	}
}
