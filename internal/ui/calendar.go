package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/ildyria/contrib-stats-tui/internal/gitstats"

	"github.com/charmbracelet/lipgloss"
)

// calendarView renders a GitHub-style contribution heatmap of daily activity.
func (m Model) calendarView(width, height int) string {
	return m.calendarViewOf(m.sum, " Contribution calendar ", width, height)
}

// calendarViewOf renders the contribution heatmap for an arbitrary summary with
// a caller-supplied section heading, so aggregate and per-repository calendars
// can share the same layout.
func (m Model) calendarViewOf(s *gitstats.Summary, heading string, width, height int) string {
	if s == nil {
		return ""
	}
	if width <= 0 {
		width = 80
	}
	first := gitstats.DayKey(s.FirstCommit)
	last := gitstats.DayKey(s.LastCommit)
	if first.IsZero() {
		return "no data"
	}

	// Align start to the configured week-start weekday on/before the first commit.
	start := first
	for start.Weekday() != m.weekStart {
		start = start.AddDate(0, 0, -1)
	}
	totalWeeks := int(last.Sub(start).Hours()/24)/7 + 1
	if totalWeeks < 1 {
		totalWeeks = 1
	}

	const gutter = 5 // weekday label column width
	const cellW = 2  // width of one day cell
	avail := (width - gutter) / cellW
	if avail < 1 {
		avail = 1
	}
	visibleWeeks := min(avail, totalWeeks)

	// Clamp offset and default to showing the most recent weeks.
	maxOffset := totalWeeks - visibleWeeks
	if m.calOffset > maxOffset {
		m.calOffset = maxOffset
	}
	// When the user hasn't scrolled, prefer the latest weeks.
	startWeek := m.calOffset
	if m.calOffset == 0 {
		startWeek = maxOffset
	}

	// Determine max daily count for intensity scaling.
	maxDaily := 0
	for _, v := range s.Daily {
		if v > maxDaily {
			maxDaily = v
		}
	}
	if maxDaily == 0 {
		maxDaily = 1
	}

	// Weekday row labels for all seven rows, ordered from the configured
	// week-start day.
	dayAbbr := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	weekdayLabels := make([]string, 7)
	for d := 0; d < 7; d++ {
		wd := (int(m.weekStart) + d) % 7
		weekdayLabels[d] = dayAbbr[wd]
	}

	// Month header row. Labels are placed at the exact column where a new
	// month first appears, guarding against overlap. The year is shown
	// whenever it changes (and for the first label) so multi-year ranges are
	// unambiguous.
	monthCells := []rune(strings.Repeat(" ", visibleWeeks*cellW))
	// yearMask marks the columns occupied by a year label so they can be
	// rendered white while the month stays gray.
	yearMask := make([]bool, len(monthCells))
	lastMonth := time.Month(0)
	lastYear := 0
	nextFree := 0
	for w := 0; w < visibleWeeks; w++ {
		ws := start.AddDate(0, 0, (startWeek+w)*7)
		mo, yr := ws.Month(), ws.Year()
		if mo == lastMonth && yr == lastYear {
			continue
		}
		label := ws.Format("Jan")
		yearAt := -1 // rune offset within label where the year starts
		if yr != lastYear {
			yearAt = len([]rune(label)) + 1 // after "Jan "
			label = ws.Format("Jan 2006")
		}
		lastMonth, lastYear = mo, yr
		pos := w * cellW
		if pos < nextFree {
			continue
		}
		for i, r := range []rune(label) {
			if pos+i < len(monthCells) {
				monthCells[pos+i] = r
				if yearAt >= 0 && i >= yearAt {
					yearMask[pos+i] = true
				}
			}
		}
		nextFree = pos + len([]rune(label)) + 1
	}
	// Render the row in styled runs: year digits in white, everything else in
	// the muted month color.
	yearStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff"))
	var monthBuf strings.Builder
	for i := 0; i < len(monthCells); {
		j := i
		for j < len(monthCells) && yearMask[j] == yearMask[i] {
			j++
		}
		seg := string(monthCells[i:j])
		if yearMask[i] {
			monthBuf.WriteString(yearStyle.Render(seg))
		} else {
			monthBuf.WriteString(subtitleStyle.Render(seg))
		}
		i = j
	}
	monthRow := gutterPad(gutter) + monthBuf.String()

	// Day rows.
	rows := make([]strings.Builder, 7)
	for d := 0; d < 7; d++ {
		rows[d].WriteString(subtitleStyle.Render(fmt.Sprintf("%-4s", weekdayLabels[d])) + " ")
	}
	for w := 0; w < visibleWeeks; w++ {
		for d := 0; d < 7; d++ {
			day := start.AddDate(0, 0, (startWeek+w)*7+d)
			if day.After(last) || day.Before(first) {
				rows[d].WriteString(lipgloss.NewStyle().
					Foreground(lipgloss.Color("#0d1117")).Render("  "))
				continue
			}
			count := s.Daily[day]
			lvl := level(count, maxDaily, calendarScale)
			cell := lipgloss.NewStyle().Foreground(heatColors[lvl]).Render("■ ")
			rows[d].WriteString(cell)
		}
	}

	var b strings.Builder
	b.WriteString(sectionStyle.Render(heading))
	b.WriteString("\n\n")
	b.WriteString(monthRow)
	b.WriteByte('\n')
	for d := 0; d < 7; d++ {
		b.WriteString(rows[d].String())
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	b.WriteString(m.legend(maxDaily, calendarScale))
	b.WriteString("\n\n")
	busiestDay, busiest, activeDays := dailyStatsOf(s)
	b.WriteString(labelStyle.Render("Commits per day — busiest: ") +
		valueStyle.Render(busiestDay.Format("Mon Jan 2 2006")) +
		labelStyle.Render(" (") + valueStyle.Render(humanize(busiest)) + labelStyle.Render(" commits)") +
		labelStyle.Render("  ·  ") + valueStyle.Render(humanize(activeDays)) + labelStyle.Render(" active days") +
		labelStyle.Render("  ·  ") + valueStyle.Render(humanize(s.TotalCommits)) + labelStyle.Render(" commits total"))
	return b.String()
}

// dailyStats returns the busiest day and its commit count, plus the number of
// days that saw any commits, across the currently-selected summary.
func (m Model) dailyStats() (busiestDay time.Time, busiest, activeDays int) {
	return dailyStatsOf(m.sum)
}

// dailyStatsOf returns the busiest day and its commit count, plus the number of
// days that saw any commits, for the given summary.
func dailyStatsOf(s *gitstats.Summary) (busiestDay time.Time, busiest, activeDays int) {
	if s == nil {
		return
	}
	for day, cnt := range s.Daily {
		if cnt > 0 {
			activeDays++
		}
		if cnt > busiest {
			busiest, busiestDay = cnt, day
		}
	}
	return
}
