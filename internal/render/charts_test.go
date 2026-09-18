package render

import (
	"slices"
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
	// Labels hug each bar: tokens, then cost, then the bar's top row, same column.
	for _, want := range []string{"27.80M", "500"} {
		i := slices.IndexFunc(lines, func(l string) bool { return strings.Contains(l, want) })
		if i < 0 || i+2 >= len(lines) {
			t.Fatalf("label %q missing:\n%s", want, out)
		}
		col := strings.Index(lines[i], want)
		cost, top := []rune(lines[i+1]), []rune(lines[i+2])
		if !strings.HasPrefix(string(cost[col:]), "$") || !strings.ContainsAny(string(top[col:col+1]), "▁▂▃▄▅▆▇█") {
			t.Errorf("label %q not directly above its bar:\n%s", want, out)
		}
	}
}

func TestHourBars(t *testing.T) {
	r := &report.Report{Range: report.Range{Name: "all"}}
	r.HourOfDay[1].Total = 100 // 1am
	r.HourOfDay[13].Total = 50 // 1pm
	lines := strings.Split(hourBars(r), "\n")
	axis := slices.IndexFunc(lines, func(l string) bool { return strings.HasPrefix(l, "    12 ") })
	if axis < 0 || !strings.HasSuffix(strings.TrimSpace(lines[axis]), "11") {
		t.Fatalf("axis should run 12,1..11:\n%s", strings.Join(lines, "\n"))
	}
	if !strings.HasPrefix(lines[axis-1], " am") || !strings.HasPrefix(lines[axis+1], " pm") {
		t.Errorf("am/pm markers should flank the axis")
	}
	// 1am grows up from the axis, 1pm grows down, both in column 1.
	col := strings.Index(lines[axis], " 1 ") + 1
	if []rune(lines[axis-1])[col] != '█' || []rune(lines[axis+1])[col] != '█' {
		t.Errorf("1am/1pm bars not in column 1:\n%s", strings.Join(lines, "\n"))
	}
	if !strings.Contains(lines[len(lines)-1], "50") {
		t.Errorf("pm token label should sit below the pm bar, got %q", lines[len(lines)-1])
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

func TestFitTokens(t *testing.T) {
	for n, want := range map[int64]string{
		500:           "500",
		796_900:       "796.9K",
		1_935_000:     "1.935M",
		27_834_000:    "27.83M",
		123_456_789:   "123.5M",
		999_960_000:   "1.000B", // rounds past 999.9M: next unit
		1_234_000_000: "1.234B",
	} {
		if got := fitTokens(n, 6); got != want {
			t.Errorf("fitTokens(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestFitMoney(t *testing.T) {
	for v, want := range map[float64]string{
		0:         "$0.00",
		4.554:     "$4.55",
		23.084:    "$23.08",
		188.14:    "$188.1",
		1234.6:    "$1235",
		99_999:    "$99999",
		123_456:   "$123K",
		1_234_567: "$1.23M",
	} {
		if got := fitMoney(v, 6); got != want {
			t.Errorf("fitMoney(%v) = %q, want %q", v, got, want)
		}
	}
}

func TestHourBarsDown(t *testing.T) {
	r := &report.Report{Range: report.Range{Name: "all"}}
	r.HourOfDay[12] = report.Bucket{Total: 40_000_000, CostUSD: 29.8} // 12pm: the peak
	r.HourOfDay[15] = report.Bucket{Total: 4_800_000, CostUSD: 3.58}  // 3pm: half a row
	r.HourOfDay[23] = report.Bucket{Total: 250_000, CostUSD: 0.19}    // 11pm: tiny
	lines := strings.Split(hourBars(r), "\n")
	out := strings.Join(lines, "\n")
	axis := slices.IndexFunc(lines, func(l string) bool { return strings.HasPrefix(l, "    12 ") })
	if axis < 0 {
		t.Fatalf("no axis:\n%s", out)
	}
	cell := func(row, col int) string {
		const colW, gap = 6, 1
		start := len(indent) + col*(colW+gap)
		rs := []rune(lines[row])
		if start+colW > len(rs) {
			return ""
		}
		return strings.TrimSpace(string(rs[start : start+colW]))
	}
	// 12pm fills all 4 rows downward, then cost, then tokens.
	for d := 1; d <= 4; d++ {
		if got := cell(axis+d, 0); got != "██████" {
			t.Errorf("12pm row %d = %q, want full block\n%s", d, got, out)
		}
	}
	if cell(axis+5, 0) != "$29.80" || cell(axis+6, 0) != "40.00M" {
		t.Errorf("12pm labels should follow the bar: cost then tokens\n%s", out)
	}
	// Partial rows use top-aligned blocks: ▀ for ≥ half, ▔ below that.
	if cell(axis+1, 3) != "▀▀▀▀▀▀" || cell(axis+2, 3) != "$3.58" || cell(axis+3, 3) != "4.800M" {
		t.Errorf("3pm should be one ▀ row then labels\n%s", out)
	}
	if cell(axis+1, 11) != "▔▔▔▔▔▔" || cell(axis+2, 11) != "$0.19" {
		t.Errorf("tiny 11pm value should still show ▔\n%s", out)
	}
	// No am usage: nothing above the axis except the title and the am marker row.
	if axis != 2 || strings.TrimSpace(lines[1]) != "am" {
		t.Errorf("empty am half should collapse to its marker row\n%s", out)
	}
}

func TestDownBlock(t *testing.T) {
	for eighths, want := range map[int]rune{0: ' ', 1: '▔', 3: '▔', 4: '▀', 7: '▀', 8: '█'} {
		if got := downBlock(eighths); got != want {
			t.Errorf("downBlock(%d) = %q, want %q", eighths, got, want)
		}
	}
}
