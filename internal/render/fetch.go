// Package render draws a report as fetch-style terminal output.
package render

import (
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/fffelix-huang/tokenfetch/internal/report"
)

var (
	accent = lipgloss.AdaptiveColor{Light: "#C15F3C", Dark: "#D97757"}
	muted  = lipgloss.AdaptiveColor{Light: "#8A8A8A", Dark: "#6C6C6C"}
	// Orange-only heat ramp, darkest→brightest on dark backgrounds (reversed on
	// light). Truecolor values sit on the xterm-256 cube so every terminal shows
	// the same steps; 16-color has no orange, so it falls back to shade glyphs.
	heatLvl = []lipgloss.CompleteAdaptiveColor{
		heat("#bcbcbc", "250", "7", "#4e4e4e", "239", "8"), // none (glyph only)
		heat("#ffd7af", "223", "11", "#af5f00", "130", "3"),
		heat("#ffaf5f", "215", "11", "#ff8700", "208", "3"),
		heat("#ff8700", "208", "3", "#ffaf5f", "215", "11"),
		heat("#af5f00", "130", "3", "#ffd7af", "223", "11"),
	}

	// heatGlyph is the fallback without a 256-color ramp (pipes, NO_COLOR, 16-color).
	heatGlyph = []string{"··", "░░", "▒▒", "▓▓", "██"}

	keyStyle   = lipgloss.NewStyle().Foreground(accent).Bold(true)
	mutedStyle = lipgloss.NewStyle().Foreground(muted)
	titleStyle = lipgloss.NewStyle().Foreground(accent).Bold(true)
)

func heat(lt, l256, l16, dt, d256, d16 string) lipgloss.CompleteAdaptiveColor {
	return lipgloss.CompleteAdaptiveColor{
		Light: lipgloss.CompleteColor{TrueColor: lt, ANSI256: l256, ANSI: l16},
		Dark:  lipgloss.CompleteColor{TrueColor: dt, ANSI256: d256, ANSI: d16},
	}
}

var rangeLabel = map[string]string{
	"today": "today",
	"week":  "last 7 days",
	"month": "last 30 days",
	"all":   "all time",
}

// Fetch writes the full report. width is the terminal width (0 = unknown).
func Fetch(w io.Writer, r *report.Report, width int) {
	info := infoLines(r)
	side := sidePanel(r, lipgloss.Height(info))
	joined := lipgloss.JoinHorizontal(lipgloss.Top, info, "    ", side)
	switch {
	case side == "":
		fmt.Fprintln(w, info)
	case width > 0 && lipgloss.Width(joined) > width:
		fmt.Fprintln(w, info+"\n\n"+side)
	default:
		fmt.Fprintln(w, joined)
	}
	if r.Total == 0 {
		return
	}

	fmt.Fprintln(w)
	switch r.Range.Name {
	case "today":
		fmt.Fprintln(w, hourBars(r))
	case "week":
		fmt.Fprintln(w, dayBars(r))
	case "month":
		fmt.Fprintln(w, calendar(r))
	default:
		fmt.Fprintln(w, contributions(r, width))
	}
	if r.Range.Name != "today" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, hourBars(r))
	}
}

func infoLines(r *report.Report) string {
	var b strings.Builder
	title := titleStyle.Render("tokenfetch") + mutedStyle.Render(" · "+rangeLabel[r.Range.Name])
	b.WriteString(title + "\n")
	b.WriteString(mutedStyle.Render(strings.Repeat("─", lipgloss.Width(title))) + "\n")

	kv := func(k, v string) { b.WriteString(keyStyle.Render(fmt.Sprintf("%-10s", k)) + " " + v + "\n") }
	from := r.Range.From
	if r.Range.Name == "all" && len(r.Daily) > 0 {
		from, _ = time.ParseInLocation("2006-01-02", r.Daily[0].Date, r.Range.To.Location())
	}
	period := from.Format("Jan 2, 2006") + " – " + lastDay(r).Format("Jan 2, 2006")
	kv("Period", period)
	cacheW := r.Tokens.CacheWrite5m + r.Tokens.CacheWrite1h
	kv("Tokens", compact(r.Total))
	kv("  Input", compact(r.Tokens.Input+r.Tokens.CacheRead+cacheW))
	sub := func(k string, v int64) { kv("", mutedStyle.Render(fmt.Sprintf("%-12s", k))+compact(v)) }
	sub("uncached", r.Tokens.Input)
	sub("cache read", r.Tokens.CacheRead)
	sub("cache write", cacheW)
	kv("  Output", compact(r.Tokens.Output))
	cost := fmt.Sprintf("$%.2f", r.CostUSD) + mutedStyle.Render(" API list price")
	if len(r.Unpriced) > 0 {
		cost += mutedStyle.Render(" (excl. " + strings.Join(r.Unpriced, ", ") + ")")
	}
	kv("Cost", cost)
	kv("Messages", fmt.Sprintf("%s in %d sessions", compact(r.Messages), r.Sessions))
	if r.PeakHour >= 0 {
		kv("Peak hour", fmt.Sprintf("%02d:00–%02d:00", r.PeakHour, (r.PeakHour+1)%24))
	}
	return strings.TrimRight(b.String(), "\n")
}

// sidePanel lists top models, projects, skills and plugins in at most height
// lines, sharing rows round-robin so it lines up with the stats column.
func sidePanel(r *report.Report, height int) string {
	type section struct {
		title string
		rows  []report.Row
		name  func(string) string
	}
	var secs []section
	for _, s := range []section{
		{"Models", r.Models, ident},
		{"Projects", r.Projects, shortPath},
		{"Skills", r.Skills, ident},
		{"Plugins", r.Plugins, ident},
	} {
		if len(s.rows) > 0 {
			secs = append(secs, s)
		}
	}
	counts := make([]int, len(secs))
	for budget := height - len(secs); budget > 0; {
		progressed := false
		for i := range secs {
			if budget > 0 && counts[i] < len(secs[i].rows) {
				counts[i]++
				budget--
				progressed = true
			}
		}
		if !progressed {
			break
		}
	}

	const maxName, barWidth = 28, 10
	nameWidth := 0
	for i, sec := range secs {
		for _, row := range sec.rows[:counts[i]] {
			nameWidth = max(nameWidth, lipgloss.Width(sec.name(row.Name)))
		}
	}
	nameWidth = min(nameWidth, maxName)

	bar := lipgloss.NewStyle().Foreground(accent)
	var lines []string
	for i, sec := range secs {
		title := titleStyle.Render(sec.title)
		if more := len(sec.rows) - counts[i]; more > 0 {
			title += mutedStyle.Render(fmt.Sprintf(" +%d more", more))
		}
		lines = append(lines, title)
		for _, row := range sec.rows[:counts[i]] {
			n := truncate(sec.name(row.Name), nameWidth)
			filled := 0
			if r.Total > 0 {
				filled = int(math.Round(float64(row.Total) / float64(r.Total) * barWidth))
			}
			lines = append(lines, fmt.Sprintf("  %s%s %s%s %7s %s",
				n, strings.Repeat(" ", nameWidth-lipgloss.Width(n)),
				bar.Render(strings.Repeat("█", filled)), mutedStyle.Render(strings.Repeat("·", barWidth-filled)),
				compact(row.Total), mutedStyle.Render(fmt.Sprintf("$%.2f", row.CostUSD))))
		}
	}
	return strings.Join(lines, "\n")
}

func compact(n int64) string {
	f := float64(n)
	switch {
	case f >= 1e9:
		return fmt.Sprintf("%.2fB", f/1e9)
	case f >= 1e6:
		return fmt.Sprintf("%.1fM", f/1e6)
	case f >= 1e3:
		return fmt.Sprintf("%.1fK", f/1e3)
	}
	return fmt.Sprintf("%d", n)
}

func ident(s string) string { return s }

func shortPath(p string) string {
	if p == "" {
		return "(unknown)"
	}
	if home, err := os.UserHomeDir(); err == nil {
		if rel, err := filepath.Rel(home, p); err == nil && !strings.HasPrefix(rel, "..") {
			return filepath.Join("~", rel)
		}
	}
	return p
}

func truncate(s string, w int) string {
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	return "…" + string(r[len(r)-w+1:])
}
