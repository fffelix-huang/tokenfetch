package render

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/fffelix-huang/tokenfetch/internal/report"
)

const indent = "    "

// hourBars: tokens per local hour, mirrored around a shared 12,1..11 axis:
// am bars grow up, pm bars grow down, each labeled at its end.
func hourBars(r *report.Report) string {
	const colW, gap, height = 6, 1, 4
	am, pm := make([]int64, 12), make([]int64, 12)
	amCost, pmCost := make([]float64, 12), make([]float64, 12)
	var peak int64
	for h, b := range r.HourOfDay {
		if h < 12 {
			am[h], amCost[h] = b.Total, b.CostUSD
		} else {
			pm[h-12], pmCost[h-12] = b.Total, b.CostUSD
		}
		peak = max(peak, b.Total)
	}
	subtitle := " · tokens by hour of day"
	if r.Range.Name == "today" {
		subtitle = " · tokens by hour"
	}
	side := func(s string) string { return mutedStyle.Render(fmt.Sprintf("%-4s", s)) }

	var b strings.Builder
	b.WriteString(titleStyle.Render("Hours") + mutedStyle.Render(subtitle) + "\n")
	up := labeledBars(am, amCost, peak, colW, gap, height, false)
	for i, l := range up {
		margin := indent
		if i == len(up)-1 {
			margin = side(" am")
		}
		b.WriteString(margin + l + "\n")
	}
	b.WriteString(indent + labels(12, colW, gap, func(i int) string {
		if i == 0 {
			return "12"
		}
		return fmt.Sprint(i)
	}))
	for i, l := range labeledBars(pm, pmCost, peak, colW, gap, height, true) {
		margin := indent
		if i == 0 {
			margin = side(" pm")
		}
		b.WriteString("\n" + margin + l)
	}
	return b.String()
}

// dayBars: tokens per day, dates along the bottom.
func dayBars(r *report.Report) string {
	const colW, gap = 6, 1
	days := dayRange(r.Range.From, lastDay(r))
	buckets := dailyBuckets(r)
	values := make([]int64, len(days))
	costs := make([]float64, len(days))
	for i, d := range days {
		bucket := buckets[dateKey(d)]
		values[i], costs[i] = bucket.Total, bucket.CostUSD
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render("Days") + mutedStyle.Render(" · tokens per day") + "\n")
	for _, l := range labeledBars(values, costs, peakOf(values), colW, gap, 6, false) {
		b.WriteString(indent + l + "\n")
	}
	b.WriteString(indent + labels(len(days), colW, gap, func(i int) string { return days[i].Format("Mon") }) + "\n")
	b.WriteString(indent + mutedStyle.Render(labels(len(days), colW, gap, func(i int) string { return days[i].Format("1/2") })))
	return b.String()
}

// calendar: month-style grid (Sunday first), one cell per day.
func calendar(r *report.Report) string {
	first, last := startOfDay(r.Range.From), lastDay(r)
	buckets := dailyBuckets(r)

	var b strings.Builder
	b.WriteString(titleStyle.Render("Calendar") + mutedStyle.Render(" · tokens per day") + "\n")
	b.WriteString(indent)
	for _, wd := range []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"} {
		b.WriteString(mutedStyle.Render(fmt.Sprintf("%-6s", wd)))
	}
	b.WriteString("\n")
	start := sunday(first)
	for ws := start; !ws.After(last); ws = ws.AddDate(0, 0, 7) {
		label := ""
		var cells strings.Builder
		for i := 0; i < 7; i++ {
			d := ws.AddDate(0, 0, i)
			if d.Before(first) || d.After(last) {
				cells.WriteString(strings.Repeat(" ", 6))
				continue
			}
			if label == "" && (ws.Equal(start) || d.Day() == 1) {
				label = d.Format("Jan")
			}
			cells.WriteString(fmt.Sprintf("%2d ", d.Day()) + heatCell(tier(buckets[dateKey(d)].Total), 2) + " ")
		}
		b.WriteString(mutedStyle.Render(fmt.Sprintf("%-4s", label)) + strings.TrimRight(cells.String(), " ") + "\n")
	}
	b.WriteString(tierLegend())
	return b.String()
}

// contributions: GitHub-style weekday × week grid (Sunday first) over the past year, fixed token tiers.
func contributions(r *report.Report, width int) string {
	last := lastDay(r)
	first := last.AddDate(-1, 0, 1)
	start := sunday(first)
	weeks := daysBetween(start, last)/7 + 1
	cellW := 2
	if width > 0 && len(indent)+weeks*cellW > width {
		cellW = 1
	}
	buckets := dailyBuckets(r)

	var b strings.Builder
	b.WriteString(titleStyle.Render("Activity") + mutedStyle.Render(" · tokens per day, past year") + "\n")
	b.WriteString(indent + mutedStyle.Render(labels(weeks, cellW, 0, func(w int) string {
		ws := start.AddDate(0, 0, 7*w)
		// Label the first week of each month; skip a partial leading month.
		if (w == 0 && ws.AddDate(0, 0, 14).Month() == ws.Month()) || (w > 0 && ws.Month() != ws.AddDate(0, 0, -7).Month()) {
			return ws.Format("Jan")
		}
		return ""
	})) + "\n")
	rowLabel := []string{"Sun", "", "Tue", "", "Thu", "", "Sat"}
	for wd := 0; wd < 7; wd++ {
		b.WriteString(mutedStyle.Render(fmt.Sprintf("%-4s", rowLabel[wd])))
		for w := 0; w < weeks; w++ {
			d := start.AddDate(0, 0, 7*w+wd)
			if d.Before(first) || d.After(last) {
				b.WriteString(strings.Repeat(" ", cellW))
				continue
			}
			b.WriteString(heatCell(tier(buckets[dateKey(d)].Total), cellW))
		}
		b.WriteString("\n")
	}
	b.WriteString(tierLegend())
	return b.String()
}

func tierLegend() string {
	var b strings.Builder
	b.WriteString(indent + mutedStyle.Render("0"))
	for l, name := range []string{"<10M", "<100M", "<1B", "≥1B"} {
		b.WriteString("  " + heatCell(l+1, 2) + " " + mutedStyle.Render(name))
	}
	return b.String()
}

var tierLimits = []int64{10_000_000, 100_000_000, 1_000_000_000}

// tier maps daily tokens to a fixed level: 0 none, then <10M, <100M, <1B, ≥1B.
func tier(v int64) int {
	if v <= 0 {
		return 0
	}
	for i, limit := range tierLimits {
		if v < limit {
			return i + 1
		}
	}
	return len(tierLimits) + 1
}

var blocks = []rune(" ▁▂▃▄▅▆▇█")

func peakOf(values []int64) int64 {
	var peak int64
	for _, v := range values {
		peak = max(peak, v)
	}
	return peak
}

// labeledBars renders one bar per value scaled to peak (colW cells wide, gap
// apart, at most height rows), with cost then token labels just past each
// bar's end. Bars grow up, or down from the first row if down. Rows that stay
// empty at the outer edge are dropped.
func labeledBars(values []int64, costs []float64, peak int64, colW, gap, height int, down bool) []string {
	rows := height + 2
	// grid[d][i]: cell at distance d from the baseline, column i
	grid := make([][]string, rows)
	blank := strings.Repeat(" ", colW)
	for d := range grid {
		grid[d] = make([]string, len(values))
		for i := range grid[d] {
			grid[d][i] = blank
		}
	}
	bar := lipgloss.NewStyle().Foreground(accent)
	pad := func(s string) string { return fmt.Sprintf("%-*s", colW, s) }
	for i, v := range values {
		if v == 0 || peak == 0 {
			continue
		}
		total := int(math.Round(float64(v) / float64(peak) * float64(height*8)))
		total = max(total, 1) // keep tiny non-zero values visible
		n := 0
		for ; n < height && total-n*8 > 0; n++ {
			fill := min(total-n*8, 8)
			glyph := blocks[fill]
			if down {
				glyph = downBlock(fill)
			}
			grid[n][i] = bar.Render(strings.Repeat(string(glyph), colW))
		}
		grid[n][i] = mutedStyle.Render(pad(fitMoney(costs[i], colW)))
		grid[n+1][i] = pad(fitTokens(v, colW))
	}

	lines := make([]string, 0, rows)
	for d := range grid {
		line := strings.Join(grid[d], strings.Repeat(" ", gap))
		if strings.TrimSpace(line) == "" && d > 0 {
			break // nothing further out
		}
		lines = append(lines, line)
	}
	if !down {
		slices.Reverse(lines)
	}
	return lines
}

// downBlock is a top-aligned block for a downward bar. Unicode only has upper
// 1/8 and 1/2 blocks, so partial rows round to those.
func downBlock(eighths int) rune {
	switch {
	case eighths >= 8:
		return '█'
	case eighths >= 4:
		return '▀'
	case eighths >= 1:
		return '▔'
	}
	return ' '
}

// labels places label(i) at column i's start, dropping labels that would overlap.
func labels(n, colW, gap int, label func(int) string) string {
	var row []rune
	end := 0
	for i := 0; i < n; i++ {
		l := []rune(label(i))
		pos := i * (colW + gap)
		if len(l) == 0 || pos < end {
			continue
		}
		for len(row) < pos {
			row = append(row, ' ')
		}
		row = append(row[:pos], l...)
		end = pos + len(l) + 1
	}
	return string(row)
}

func heatCell(level, width int) string {
	glyph := string([]rune(heatGlyph[level])[:width])
	// Solid needs a real orange ramp (256+ colors); 16-color keeps shade glyphs.
	if p := lipgloss.ColorProfile(); level > 0 && (p == termenv.TrueColor || p == termenv.ANSI256) {
		glyph = strings.Repeat("█", width)
	}
	return lipgloss.NewStyle().Foreground(heatLvl[level]).Render(glyph)
}

func dailyBuckets(r *report.Report) map[string]report.Bucket {
	m := make(map[string]report.Bucket, len(r.Daily))
	for _, d := range r.Daily {
		m[d.Date] = d.Bucket
	}
	return m
}

func startOfDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// lastDay is the local date of the range end (report ranges end just after now).
func lastDay(r *report.Report) time.Time { return startOfDay(r.Range.To.Add(-time.Minute)) }

func sunday(t time.Time) time.Time {
	return startOfDay(t).AddDate(0, 0, -int(t.Weekday()))
}

func dayRange(from, to time.Time) []time.Time {
	var out []time.Time
	for d := startOfDay(from); !d.After(to); d = d.AddDate(0, 0, 1) {
		out = append(out, d)
	}
	return out
}

func daysBetween(a, b time.Time) int {
	return int(math.Round(b.Sub(a).Hours() / 24))
}

func dateKey(t time.Time) string { return t.Format("2006-01-02") }

type unit struct {
	scale  float64
	suffix string
}

var tokenUnits = []unit{{1, ""}, {1e3, "K"}, {1e6, "M"}, {1e9, "B"}, {1e12, "T"}}

// fitTokens formats n in its natural unit (K/M/B/T) with as many decimals as
// fit width: 27.83M, 123.4M, 796.9K. Counts under 1000 print as integers.
func fitTokens(n int64, width int) string {
	v := float64(n)
	if v < 1000 {
		return strconv.FormatInt(n, 10)
	}
	u := 1
	for u+1 < len(tokenUnits) && v >= tokenUnits[u+1].scale {
		u++
	}
	for ; u < len(tokenUnits); u++ {
		if s, ok := fitIn(v, tokenUnits[u], "", 3, width); ok {
			return s
		}
	}
	return compact(n)
}

var moneyUnits = []unit{{1, ""}, {1e3, "K"}, {1e6, "M"}}

// fitMoney formats a cost with up to two decimals, as many as fit width,
// switching to K/M only when the plain amount can't fit: $4.55, $23.08,
// $188.1, $1235, $123K.
func fitMoney(v float64, width int) string {
	for _, u := range moneyUnits {
		if s, ok := fitIn(v, u, "$", 2, width); ok {
			return s
		}
	}
	return fmt.Sprintf("$%.0fM", v/1e6)
}

// fitIn tries maxDec..0 decimals of v in unit u. A result that rounds up to
// 1000 of a scaled unit is rejected so the caller moves to the next unit.
func fitIn(v float64, u unit, prefix string, maxDec, width int) (string, bool) {
	x := v / u.scale
	for d := maxDec; d >= 0; d-- {
		num := strconv.FormatFloat(x, 'f', d, 64)
		if u.scale > 1 && math.Abs(mustParse(num)) >= 1000 {
			return "", false
		}
		if s := prefix + num + u.suffix; len(s) <= width {
			return s, true
		}
	}
	return "", false
}

func mustParse(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}
