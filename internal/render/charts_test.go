package render

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fffelix-huang/tokenfetch/internal/report"
)

// Tests run without a TTY, so lipgloss emits no color codes.

func reportWithDays(t *testing.T, rangeName string, now time.Time, days map[string]int64) *report.Report {
	t.Helper()
	rng, ok := report.NewRange(rangeName, now)
	require.True(t, ok, "bad range %q", rangeName)
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
	assert.Equal(t, []string{"9/11", "9/12", "9/13", "9/14", "9/15", "9/16", "9/17"}, strings.Fields(lines[len(lines)-1]), "date row")
	weekdays := strings.Fields(lines[len(lines)-2])
	require.Len(t, weekdays, 7, "weekday row")
	assert.Equal(t, "Fri", weekdays[0])
	assert.Equal(t, "Thu", weekdays[6])
	// Labels hug each bar: tokens, then cost, then the bar's top row, same column.
	for _, want := range []string{"27.80M", "500"} {
		i := slices.IndexFunc(lines, func(l string) bool { return strings.Contains(l, want) })
		require.True(t, i >= 0 && i+2 < len(lines), "label %q missing:\n%s", want, out)
		col := strings.Index(lines[i], want)
		cost, top := []rune(lines[i+1]), []rune(lines[i+2])
		assert.True(t, strings.HasPrefix(string(cost[col:]), "$") && strings.ContainsAny(string(top[col:col+1]), "▁▂▃▄▅▆▇█"),
			"label %q not directly above its bar:\n%s", want, out)
	}
}

func TestHourBars(t *testing.T) {
	r := &report.Report{Range: report.Range{Name: "all"}}
	r.HourOfDay[1].Total = 100 // 1am
	r.HourOfDay[13].Total = 50 // 1pm
	lines := strings.Split(hourBars(r), "\n")
	out := strings.Join(lines, "\n")
	axis := slices.IndexFunc(lines, func(l string) bool { return strings.HasPrefix(l, "    12 ") })
	require.True(t, axis >= 0 && strings.HasSuffix(strings.TrimSpace(lines[axis]), "11"), "axis should run 12,1..11:\n%s", out)
	assert.True(t, strings.HasPrefix(lines[axis-1], " am") && strings.HasPrefix(lines[axis+1], " pm"), "am/pm markers should flank the axis")
	// 1am grows up from the axis, 1pm grows down, both in column 1.
	col := strings.Index(lines[axis], " 1 ") + 1
	assert.Equal(t, '█', []rune(lines[axis-1])[col], "1am bar not in column 1:\n%s", out)
	assert.Equal(t, '█', []rune(lines[axis+1])[col], "1pm bar not in column 1:\n%s", out)
	assert.Contains(t, lines[len(lines)-1], "50", "pm token label should sit below the pm bar")
}

func TestCalendar(t *testing.T) {
	now := time.Date(2026, 9, 17, 12, 0, 0, 0, time.UTC)
	out := calendar(reportWithDays(t, "month", now, map[string]int64{"2026-09-02": 10}))
	lines := strings.Split(out, "\n")
	// Range starts Wed Aug 19; first week row is indented three blank Sun/Mon/Tue cells.
	assert.True(t, strings.HasPrefix(lines[1], "    Sun"), "header row = %q", lines[1])
	assert.True(t, strings.HasPrefix(lines[2], "Aug                   19 ··"), "first week row = %q", lines[2])
	assert.Contains(t, out, " 2 ░░", "fixed tiers: 10 tokens should be <10M level")
	assert.Contains(t, out, " 3 ··", "idle day should be empty")
	assert.NotContains(t, out, "18 ", "day after range end rendered")
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
	require.Len(t, lines, 10, "title, months, 7 weekdays, legend:\n%s", out)
	assert.True(t, strings.HasPrefix(lines[2], "Sun"), "first row = %q", lines[2])
	assert.True(t, strings.HasPrefix(lines[8], "Sat"), "last row = %q", lines[8])
	assert.True(t, strings.HasSuffix(lines[5], "██"), "last column Wed = %q", lines[5])
	assert.True(t, strings.HasSuffix(lines[6], "░░"), "last column Thu = %q", lines[6])
	assert.Equal(t, 2, strings.Count(out, "██"), "only Wed cell + legend should be ≥1B:\n%s", out)
	for _, line := range strings.Split(contributions(r, 70), "\n") {
		assert.LessOrEqual(t, len([]rune(line)), 70, "narrow terminal: %q", line)
	}
}

func TestTier(t *testing.T) {
	for v, want := range map[int64]int{0: 0, 1: 1, 9_999_999: 1, 10_000_000: 2, 99_999_999: 2, 100_000_000: 3, 999_999_999: 3, 1_000_000_000: 4} {
		assert.Equal(t, want, tier(v), "tier(%d)", v)
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
		assert.Equal(t, want, fitTokens(n, 6), "fitTokens(%d)", n)
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
		assert.Equal(t, want, fitMoney(v, 6), "fitMoney(%v)", v)
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
	require.GreaterOrEqual(t, axis, 0, "no axis:\n%s", out)
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
		assert.Equal(t, "██████", cell(axis+d, 0), "12pm row %d should be full block\n%s", d, out)
	}
	assert.Equal(t, "$29.80", cell(axis+5, 0), "12pm cost should follow the bar\n%s", out)
	assert.Equal(t, "40.00M", cell(axis+6, 0), "12pm tokens should follow cost\n%s", out)
	// Partial rows use top-aligned blocks: ▀ for ≥ half, ▔ below that.
	assert.Equal(t, "▀▀▀▀▀▀", cell(axis+1, 3), "3pm should be one ▀ row\n%s", out)
	assert.Equal(t, "$3.58", cell(axis+2, 3), "3pm cost\n%s", out)
	assert.Equal(t, "4.800M", cell(axis+3, 3), "3pm tokens\n%s", out)
	assert.Equal(t, "▔▔▔▔▔▔", cell(axis+1, 11), "tiny 11pm value should still show ▔\n%s", out)
	assert.Equal(t, "$0.19", cell(axis+2, 11), "11pm cost\n%s", out)
	// No am usage: nothing above the axis except the title and the am marker row.
	assert.Equal(t, 2, axis, "empty am half should collapse to its marker row\n%s", out)
	assert.Equal(t, "am", strings.TrimSpace(lines[1]), "empty am half should collapse to its marker row\n%s", out)
}

func TestDownBlock(t *testing.T) {
	for eighths, want := range map[int]rune{0: ' ', 1: '▔', 3: '▔', 4: '▀', 7: '▀', 8: '█'} {
		assert.Equal(t, want, downBlock(eighths), "downBlock(%d)", eighths)
	}
}

// Harness and model are independent, so Harnesses shows even for one harness,
// right above Models.
func TestSidePanelHarnessesAboveModels(t *testing.T) {
	r := &report.Report{
		Total:     10,
		Harnesses: []report.Row{{Name: "claude-code", Total: 10}},
		Models:    []report.Row{{Name: "claude-opus-5-5", Total: 6}, {Name: "gpt-6-luna", Total: 4}},
	}
	out := sidePanel(r, 10)
	h, m := strings.Index(out, "Harnesses"), strings.Index(out, "Models")
	require.GreaterOrEqual(t, h, 0, "no Harnesses section:\n%s", out)
	assert.Less(t, h, m, "Harnesses should precede Models:\n%s", out)
	assert.Contains(t, out, "claude-code")
}

func TestCachedShare(t *testing.T) {
	for _, tc := range []struct {
		read, total int64
		want        string
	}{
		{970, 1000, " · 97.0% cached"},
		{5, 100, " · 5.0% cached"}, // single digit: no padding
		{1, 3, " · 33.3% cached"},
		{0, 10, " · 0.0% cached"},
		{0, 0, ""}, // no usage: nothing to show
	} {
		r := &report.Report{Total: tc.total}
		r.Tokens.CacheRead = tc.read
		assert.Equal(t, tc.want, cachedShare(r), "%d/%d", tc.read, tc.total)
	}
	r := &report.Report{Total: 1000}
	r.Tokens.CacheRead = 970
	assert.Contains(t, infoLines(r), "97.0% cached", "shown on the Tokens line")
}
