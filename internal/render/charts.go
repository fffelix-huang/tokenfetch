package render

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/fffelix-huang/tokenfetch/internal/report"
)

const indent = "    "

// hourBars: tokens per local hour of day.
func hourBars(r *report.Report) string {
	values := make([]int64, 24)
	for h, b := range r.HourOfDay {
		values[h] = b.Total
	}
	subtitle := " · tokens by hour of day"
	if r.Range.Name == "today" {
		subtitle = " · tokens by hour"
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render("Hours") + mutedStyle.Render(subtitle) + "\n")
	for _, l := range vbars(values, 2, 0, 5) {
		b.WriteString(indent + l + "\n")
	}
	b.WriteString(indent + mutedStyle.Render(labels(24, 2, 0, func(h int) string {
		if h%3 != 0 {
			return ""
		}
		return fmt.Sprint(h)
	})))
	return b.String()
}

// dayBars: tokens per day, dates along the bottom.
func dayBars(r *report.Report) string {
	const colW, gap = 6, 1
	days := dayRange(r.Range.From, lastDay(r))
	totals := dailyTotals(r)
	values := make([]int64, len(days))
	for i, d := range days {
		values[i] = totals[dateKey(d)]
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render("Days") + mutedStyle.Render(" · tokens per day") + "\n")
	b.WriteString(indent + mutedStyle.Render(labels(len(days), colW, gap, func(i int) string {
		if values[i] == 0 {
			return ""
		}
		return compact(values[i])
	})) + "\n")
	for _, l := range vbars(values, colW, gap, 6) {
		b.WriteString(indent + l + "\n")
	}
	b.WriteString(indent + labels(len(days), colW, gap, func(i int) string { return days[i].Format("Mon") }) + "\n")
	b.WriteString(indent + mutedStyle.Render(labels(len(days), colW, gap, func(i int) string { return days[i].Format("1/2") })))
	return b.String()
}

// calendar: month-style grid (Sunday first), one cell per day.
func calendar(r *report.Report) string {
	first, last := startOfDay(r.Range.From), lastDay(r)
	totals := dailyTotals(r)

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
			cells.WriteString(fmt.Sprintf("%2d ", d.Day()) + heatCell(tier(totals[dateKey(d)]), 2) + " ")
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
	totals := dailyTotals(r)

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
			b.WriteString(heatCell(tier(totals[dateKey(d)]), cellW))
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

// vbars renders one bar per value, colW cells wide, gap spaces apart, height rows tall.
func vbars(values []int64, colW, gap, height int) []string {
	var peak int64
	for _, v := range values {
		peak = max(peak, v)
	}
	bar := lipgloss.NewStyle().Foreground(accent)
	lines := make([]string, height)
	for row := range lines {
		base := (height - 1 - row) * 8 // eighths below this row
		var b strings.Builder
		for i, v := range values {
			if i > 0 {
				b.WriteString(strings.Repeat(" ", gap))
			}
			fill := 0
			if peak > 0 {
				fill = int(math.Round(float64(v)/float64(peak)*float64(height*8))) - base
			}
			fill = min(max(fill, 0), 8)
			if v > 0 && base == 0 && fill == 0 {
				fill = 1 // keep tiny non-zero values visible
			}
			b.WriteString(bar.Render(strings.Repeat(string(blocks[fill]), colW)))
		}
		lines[row] = b.String()
	}
	return lines
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

func dailyTotals(r *report.Report) map[string]int64 {
	m := make(map[string]int64, len(r.Daily))
	for _, d := range r.Daily {
		m[d.Date] = d.Total
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
