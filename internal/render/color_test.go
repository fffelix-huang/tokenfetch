package render

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestHeatLevelsDistinct(t *testing.T) {
	defer lipgloss.SetColorProfile(lipgloss.ColorProfile())
	for _, p := range []termenv.Profile{termenv.TrueColor, termenv.ANSI256, termenv.ANSI} {
		lipgloss.SetColorProfile(p)
		seen := map[string]int{}
		for l := 1; l < len(heatLvl); l++ {
			cell := heatCell(l, 2)
			if prev, dup := seen[cell]; dup {
				t.Errorf("profile %v: levels %d and %d render identically: %q", p, prev, l, cell)
			}
			seen[cell] = l
		}
	}
}
