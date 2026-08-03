package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// heatThresholds holds the ascending ratio cutoffs (relative to the busiest
// bucket) for intensity levels 2..N. A bucket whose ratio exceeds cuts[i] maps
// to level i+2; anything above zero but below cuts[0] is level 1. Keeping these
// as named values lets the calendar and the weekday×hour heatmap use different
// scales, and the length of cuts controls how many intensity levels there are.
type heatThresholds struct {
	cuts []float64
}

var (
	// calendarScale tunes the daily contribution calendar. Daily counts tend
	// to be small, so the lower levels kick in quickly.
	calendarScale = heatThresholds{cuts: []float64{0.03, 0.08, 0.20, 0.35, 0.50}}
	// punchScale tunes the weekday×hour heatmap, whose buckets accumulate over
	// the whole history and are dominated by a few peak slots; higher cutoffs
	// keep the map from looking uniformly hot.
	punchScale = heatThresholds{cuts: []float64{0.02, 0.10, 0.30, 0.50, 0.75}}
)

// legend renders the shared intensity scale used by the calendar and activity
// heatmaps, annotating each swatch with the minimum commit count that reaches
// its level so the colors map to concrete numbers. maxCount is the busiest
// bucket, which anchors the thresholds used by level.
func (m Model) legend(maxCount int, t heatThresholds) string {
	if maxCount < 1 {
		maxCount = 1
	}
	n := len(t.cuts) + 1 // number of active levels (1..n)

	var b strings.Builder
	b.WriteString(subtitleStyle.Render("Less "))
	b.WriteString(lipgloss.NewStyle().Foreground(heatColors[0]).Render("■"))
	b.WriteString(subtitleStyle.Render(" 0 "))
	for lvl := 1; lvl <= n; lvl++ {
		min := 1
		if lvl >= 2 {
			min = int(t.cuts[lvl-2]*float64(maxCount)) + 1
		}
		color := heatColors[clamp(lvl, 0, len(heatColors)-1)]
		b.WriteString(lipgloss.NewStyle().Foreground(color).Render("■"))
		b.WriteString(subtitleStyle.Render(fmt.Sprintf(" ≥%d ", min)))
	}
	b.WriteString(subtitleStyle.Render(fmt.Sprintf("More (max %d)", maxCount)))
	return b.String()
}

// level maps a count to a heatColors index (0..N) relative to a maximum, using
// the supplied thresholds.
func level(count, maxCount int, t heatThresholds) int {
	if count <= 0 {
		return 0
	}
	ratio := float64(count) / float64(maxCount)
	lvl := 1
	for i, c := range t.cuts {
		if ratio > c {
			lvl = i + 2
		}
	}
	return clamp(lvl, 0, len(heatColors)-1)
}

func gutterPad(n int) string { return strings.Repeat(" ", n) }
