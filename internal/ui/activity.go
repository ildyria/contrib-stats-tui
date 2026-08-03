package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ildyria/contrib-stats-tui/internal/gitstats"

	"github.com/charmbracelet/lipgloss"
)

// punchCardView renders a weekday×hour heatmap of commit frequency across the
// whole repository, with rows ordered from the configured week-start day.
func (m Model) punchCardView(width int) string {
	return m.punchCardOf(m.sum, " Commits per weekday & hour ", width)
}

// punchCardOf renders a weekday×hour commit heatmap for the given summary,
// titled with heading. It powers both the single/aggregate punch card and the
// per-repository cards stacked beneath it on the aggregate activity view.
func (m Model) punchCardOf(s *gitstats.Summary, heading string, width int) string {
	if s == nil {
		return "no data"
	}

	// Max bucket value for intensity scaling, plus per-weekday totals for the
	// row labels.
	maxVal := 0
	var dayTotals [7]int
	for d := 0; d < 7; d++ {
		for h := 0; h < 24; h++ {
			dayTotals[d] += s.Punch[d][h]
			if s.Punch[d][h] > maxVal {
				maxVal = s.Punch[d][h]
			}
		}
	}
	if maxVal == 0 {
		return subtitleStyle.Render("No commit activity to display.")
	}

	const gutter = 4 // weekday label column
	dayNames := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}

	// Hour header row (labels every 3 hours), aligned to 2-wide cells.
	var hdr strings.Builder
	hdr.WriteString(strings.Repeat(" ", gutter))
	for h := 0; h < 24; h++ {
		if h%3 == 0 {
			hdr.WriteString(subtitleStyle.Render(fmt.Sprintf("%-2d", h)))
		} else {
			hdr.WriteString("  ")
		}
	}

	var b strings.Builder
	b.WriteString(sectionStyle.Render(heading))
	b.WriteString("\n\n")
	b.WriteString(hdr.String())
	b.WriteByte('\n')

	for row := 0; row < 7; row++ {
		wd := int(m.weekStart+time.Weekday(row)) % 7
		b.WriteString(subtitleStyle.Render(fmt.Sprintf("%-3s ", dayNames[wd])))
		for h := 0; h < 24; h++ {
			lvl := level(s.Punch[wd][h], maxVal, punchScale)
			b.WriteString(lipgloss.NewStyle().Foreground(heatColors[lvl]).Render("■ "))
		}
		b.WriteString(commitsStyle.Render(fmt.Sprintf(" %s", humanize(dayTotals[wd]))))
		b.WriteByte('\n')
	}

	b.WriteByte('\n')
	b.WriteString(gutterPad(gutter) + m.legend(maxVal, punchScale))
	st := punchStatsOf(s)
	b.WriteString("\n\n")
	b.WriteString(labelStyle.Render(fmt.Sprintf("%-26s", "Busiest slot:")) +
		valueStyle.Render(fmt.Sprintf("%s %02d:00", dayNames[st.bestD], st.bestH)) +
		labelStyle.Render(" (") + valueStyle.Render(humanize(st.slotCount)) + labelStyle.Render(" commits)"))
	b.WriteByte('\n')
	b.WriteString(labelStyle.Render(fmt.Sprintf("%-26s", "Most active hour overall:")) +
		valueStyle.Render(fmt.Sprintf("%02d:00", st.bestHour)) +
		labelStyle.Render(" (") + valueStyle.Render(humanize(st.bestHourCount)) + labelStyle.Render(" commits)"))
	b.WriteByte('\n')
	b.WriteString(labelStyle.Render(fmt.Sprintf("%-26s", "Busiest day:")) +
		valueStyle.Render(dayNames[st.bestDay]) +
		labelStyle.Render(" (") + valueStyle.Render(humanize(st.bestDayCount)) + labelStyle.Render(" commits)"))
	b.WriteByte('\n')
	weekend, afterHours := workPatternOf(s)
	b.WriteString(labelStyle.Render(fmt.Sprintf("%-26s", "Weekend commits:")) +
		valueStyle.Render(fmt.Sprintf("%.0f%%", weekend*100)) +
		labelStyle.Render(fmt.Sprintf("  (weekday %.0f%%)", (1-weekend)*100)))
	b.WriteByte('\n')
	b.WriteString(labelStyle.Render(fmt.Sprintf("%-26s", "After-hours commits:")) +
		valueStyle.Render(fmt.Sprintf("%.0f%%", afterHours*100)) +
		labelStyle.Render("  (outside 08:00–18:00)"))
	b.WriteByte('\n')
	b.WriteString(subtitleStyle.Render("times are local to each commit"))
	return b.String()
}

// punchStat holds the highlights of the weekday×hour commit matrix.
type punchStat struct {
	bestD, bestH, slotCount int
	bestHour, bestHourCount int
	bestDay, bestDayCount   int
}

// computePunchStats derives the busiest slot, hour and weekday from the
// weekday×hour commit matrix.
func punchStatsOf(s *gitstats.Summary) punchStat {
	var st punchStat
	var hourTotals [24]int
	var dayTotals [7]int
	for d := 0; d < 7; d++ {
		for h := 0; h < 24; h++ {
			hourTotals[h] += s.Punch[d][h]
			dayTotals[d] += s.Punch[d][h]
			if s.Punch[d][h] > s.Punch[st.bestD][st.bestH] {
				st.bestD, st.bestH = d, h
			}
		}
	}
	st.slotCount = s.Punch[st.bestD][st.bestH]
	for h := 0; h < 24; h++ {
		if hourTotals[h] > st.bestHourCount {
			st.bestHourCount, st.bestHour = hourTotals[h], h
		}
	}
	for d := 0; d < 7; d++ {
		if dayTotals[d] > st.bestDayCount {
			st.bestDayCount, st.bestDay = dayTotals[d], d
		}
	}
	return st
}

// workPatternStats returns the share of commits made on weekends (Saturday or
// Sunday) and outside working hours (before 08:00 or from 18:00), derived from
// the weekday×hour punch matrix.
func workPatternOf(s *gitstats.Summary) (weekend, afterHours float64) {
	total, we, ah := 0, 0, 0
	for d := 0; d < 7; d++ {
		for h := 0; h < 24; h++ {
			n := s.Punch[d][h]
			if n == 0 {
				continue
			}
			total += n
			if d == 0 || d == 6 {
				we += n
			}
			if h < 8 || h >= 18 {
				ah += n
			}
		}
	}
	if total == 0 {
		return 0, 0
	}
	return float64(we) / float64(total), float64(ah) / float64(total)
}

// activityView stacks the contribution calendar, a lines-changed-over-time
// graph, the weekday×hour punch card, the project-risk indicators (bus factor
// and Gini coefficient) and the commit-size distribution so they all live on a
// single tab.
func (m Model) activityView(width, height int) string {
	sections := []string{
		m.calendarView(width, height),
	}
	// On the aggregate ("All repositories") view, show one contribution calendar
	// per repository below the combined one so each repo's cadence is visible.
	if m.multi() && m.selected == 0 {
		for _, s := range m.repos {
			sections = append(sections, m.calendarViewOf(s, " "+s.Repo+" — contribution calendar ", width, height))
		}
	}
	sections = append(sections, m.linesOverTimeView(width))
	// On the aggregate ("All repositories") view, show the combined punch card
	// followed by one card per repository; otherwise just the selected card.
	if m.multi() && m.selected == 0 {
		sections = append(sections, m.punchCardOf(m.global, " Commits per weekday & hour — all repositories ", width))
		for _, s := range m.repos {
			sections = append(sections, m.punchCardOf(s, " "+s.Repo+" ", width))
		}
	} else {
		sections = append(sections, m.punchCardView(width))
	}
	sections = append(sections,
		m.projectRiskView(width),
		m.commitSizeView(width),
	)
	nonEmpty := sections[:0]
	for _, s := range sections {
		if strings.TrimSpace(s) != "" {
			nonEmpty = append(nonEmpty, s)
		}
	}
	return strings.Join(nonEmpty, "\n\n")
}

// linesOverTimeView renders a diverging chart of repository-wide lines added
// (growing up) and deleted (growing down) per week, oldest to newest. The chart
// is flanked by the first and last commit dates and captioned with a year row
// marking where each year begins.
func (m Model) linesOverTimeView(width int) string {
	if m.sum == nil || m.sum.WeekCount <= 0 {
		return ""
	}
	if len(m.sum.WeeklyAdd) == 0 && len(m.sum.WeeklyDel) == 0 {
		return ""
	}

	graphWidth := clamp(width-38, 10, 80)
	const graphHeight = 4
	addSpark, delSpark := divergingBars(m.sum.WeeklyAdd, m.sum.WeeklyDel, graphWidth, graphHeight)
	sparks := lipgloss.JoinVertical(lipgloss.Left, addSpark, delSpark)

	legendRows := make([]string, 2*graphHeight)
	for i := range legendRows {
		legendRows[i] = " "
	}
	legendRows[0] = addStyle.Render("+ added")
	legendRows[graphHeight] = delStyle.Render("- removed")
	legend := lipgloss.JoinVertical(lipgloss.Left, legendRows...)

	// Date columns flanking the graph: first commit on the left, last commit on
	// the right (the series runs oldest → newest).
	firstCol := lipgloss.JoinVertical(lipgloss.Left,
		emailStyle.Render("first"),
		emailStyle.Render(m.sum.FirstCommit.Format("2006-01-02")))
	lastCol := lipgloss.JoinVertical(lipgloss.Left,
		emailStyle.Render("last"),
		emailStyle.Render(m.sum.LastCommit.Format("2006-01-02")))

	row := lipgloss.JoinHorizontal(lipgloss.Center,
		firstCol, "  ", legend, "  ", sparks, "  ", lastCol)

	// The year row aligns under the sparks, so it is indented by everything to
	// the left of them (first-date column + legend + their separators).
	offset := lipgloss.Width(firstCol) + 2 + lipgloss.Width(legend) + 2
	yearRow := m.linesYearRow(graphWidth, offset)

	var b strings.Builder
	b.WriteString(sectionStyle.Render(" Lines changed over time "))
	b.WriteString("\n\n")
	b.WriteString(row)
	b.WriteByte('\n')
	if yearRow != "" {
		b.WriteString(yearRow)
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	b.WriteString(subtitleStyle.Render(fmt.Sprintf("lines added / removed per week over %d weeks", m.sum.WeekCount)))
	b.WriteByte('\n')
	b.WriteString(
		addStyle.Render(humanize(m.sum.Additions)+" added") + labelStyle.Render(" · ") +
			delStyle.Render(humanize(m.sum.Deletions)+" removed") + labelStyle.Render(" · ") +
			valueStyle.Render(humanize(m.sum.Additions-m.sum.Deletions)+" net"))
	return b.String()
}

// linesYearRow builds a caption row, width columns wide and indented by offset
// spaces, that places each year label at the column where that year begins along
// the first→last commit time span. The starting year sits at the left edge.
func (m Model) linesYearRow(width, offset int) string {
	first, last := m.sum.FirstCommit, m.sum.LastCommit
	if !last.After(first) || width <= 0 {
		return ""
	}
	span := last.Sub(first)
	cells := []rune(strings.Repeat(" ", width))
	nextFree := 0
	for y := first.Year(); y <= last.Year(); y++ {
		pos := 0
		if y != first.Year() {
			jan1 := time.Date(y, time.January, 1, 0, 0, 0, 0, first.Location())
			frac := float64(jan1.Sub(first)) / float64(span)
			pos = int(frac*float64(width-1) + 0.5)
		}
		if pos < nextFree {
			continue
		}
		label := fmt.Sprintf("%d", y)
		for i, r := range []rune(label) {
			if pos+i < len(cells) {
				cells[pos+i] = r
			}
		}
		nextFree = pos + len([]rune(label)) + 1
	}
	return strings.Repeat(" ", offset) + subtitleStyle.Render(string(cells))
}

// projectRiskView surfaces two knowledge-concentration indicators under the
// heatmap: the bus factor (how few contributors hold most of the surviving
// code) and the Gini coefficient of contribution (how unevenly work is spread).
func (m Model) projectRiskView(width int) string {
	if m.sum == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(sectionStyle.Render(" Project risk "))
	b.WriteString("\n\n")

	bf, share := m.busFactor()
	if bf == 0 {
		b.WriteString(labelStyle.Render(fmt.Sprintf("%-20s ", "Bus factor:")) +
			subtitleStyle.Render("n/a (no surviving lines tracked)"))
	} else {
		b.WriteString(labelStyle.Render(fmt.Sprintf("%-20s", "Bus factor:")) +
			valueStyle.Render(humanize(bf)))
		b.WriteByte('\n')
		b.WriteString(subtitleStyle.Render(fmt.Sprintf(
			"fewest contributors holding ≥50%% of surviving lines (they hold %.0f%%)",
			share*100)))
	}
	b.WriteByte('\n')

	g := m.giniContribution()
	b.WriteString(labelStyle.Render(fmt.Sprintf("%-20s", "Gini coefficient:")) +
		valueStyle.Render(fmt.Sprintf("%.2f", g)))
	b.WriteByte('\n')
	b.WriteString(subtitleStyle.Render(
		"inequality of lines changed across contributors (0 = even · 1 = one-person show)"))

	if m.sum.MedianLineLifetimeDays > 0 {
		b.WriteByte('\n')
		b.WriteString(labelStyle.Render(fmt.Sprintf("%-20s", "Time-to-legacy:")) +
			valueStyle.Render(humanizeDays(m.sum.MedianLineLifetimeDays)))
		b.WriteByte('\n')
		b.WriteString(subtitleStyle.Render("median age at which a line gets overwritten"))
	}

	newPerMonth := m.newcomersPerMonth()
	if m.sum.MedianOnboardingDays > 0 || newPerMonth > 0 {
		b.WriteByte('\n')
		b.WriteString(labelStyle.Render(fmt.Sprintf("%-20s", "Onboarding:")) +
			valueStyle.Render(humanizeDays(m.sum.MedianOnboardingDays)) +
			labelStyle.Render(" to first surviving change"))
		b.WriteByte('\n')
		b.WriteString(labelStyle.Render(fmt.Sprintf("%-20s", "New contributors:")) +
			valueStyle.Render(fmt.Sprintf("%.1f", newPerMonth)) +
			labelStyle.Render(" per month"))
	}
	return b.String()
}

// newcomersPerMonth estimates how many first-time contributors appear per month,
// spreading the distinct contributor count across the repository's active span.
func (m Model) newcomersPerMonth() float64 {
	if m.sum == nil || len(m.sum.Contributors) == 0 {
		return 0
	}
	months := m.sum.LastCommit.Sub(m.sum.FirstCommit).Hours() / 24 / 30.44
	if months < 1 {
		months = 1
	}
	return float64(len(m.sum.Contributors)) / months
}

// humanizeDays renders a day count as a readable duration, scaling to months or
// years for longer spans.
func humanizeDays(d int) string {
	switch {
	case d <= 0:
		return "0 days"
	case d == 1:
		return "1 day"
	case d < 60:
		return fmt.Sprintf("%d days", d)
	case d < 365:
		return fmt.Sprintf("%.1f months", float64(d)/30.44)
	default:
		return fmt.Sprintf("%.1f years", float64(d)/365.25)
	}
}

// commitSizeView renders a histogram of lines changed per commit, plus the
// median commit size and the share of "large" commits.
func (m Model) commitSizeView(width int) string {
	if m.sum == nil || len(m.sum.CommitSizeHist) == 0 || m.sum.TotalCommits == 0 {
		return ""
	}
	hist := m.sum.CommitSizeHist
	maxCount := 0
	for _, c := range hist {
		if c > maxCount {
			maxCount = c
		}
	}
	if maxCount == 0 {
		return ""
	}

	barW := clamp(width-32, 10, 48)
	var b strings.Builder
	b.WriteString(sectionStyle.Render(" Commit size distribution "))
	b.WriteString("\n\n")
	for i, cnt := range hist {
		fill := int(float64(barW)*float64(cnt)/float64(maxCount) + 0.5)
		if cnt > 0 && fill < 1 {
			fill = 1
		}
		bar := sparkStyle.Render(strings.Repeat("█", fill)) +
			lipgloss.NewStyle().Foreground(colorMuted).Render(strings.Repeat("░", barW-fill))
		pct := 100 * float64(cnt) / float64(m.sum.TotalCommits)
		b.WriteString(fmt.Sprintf("%s %s %s\n",
			subtitleStyle.Render(fmt.Sprintf("%12s", sizeBucketLabel(i))),
			bar,
			commitsStyle.Render(fmt.Sprintf("%s (%.0f%%)", humanize(cnt), pct))))
	}
	b.WriteByte('\n')
	largePct := 100 * float64(m.sum.LargeCommits) / float64(m.sum.TotalCommits)
	b.WriteString(summaryStyle.Render(fmt.Sprintf(
		"median %s lines · %s large commits (>%d lines, %.0f%%)",
		humanize(m.sum.MedianCommitSize),
		humanize(m.sum.LargeCommits),
		gitstats.LargeCommitThreshold,
		largePct)))
	return b.String()
}

// sizeBucketLabel formats the lines-changed range covered by commit-size
// histogram bucket i, using gitstats.CommitSizeBuckets as the bucket edges.
func sizeBucketLabel(i int) string {
	edges := gitstats.CommitSizeBuckets
	n := len(edges)
	switch {
	case i == 0:
		return "0"
	case i < n:
		lo, hi := edges[i-1]+1, edges[i]
		if lo == hi {
			return humanize(lo)
		}
		return fmt.Sprintf("%s–%s", humanize(lo), humanize(hi))
	default:
		return humanize(edges[n-1]+1) + "+"
	}
}

// busFactor returns the fewest contributors whose surviving lines
// (RetentionAlive) together account for at least half of all surviving lines,
// along with that group's share (0..1). A low number flags a project where
// little of the living code is spread across many people.
func (m Model) busFactor() (n int, share float64) {
	alive := make([]int, 0, len(m.sum.Contributors))
	total := 0
	for _, c := range m.sum.Contributors {
		if c.RetentionAlive > 0 {
			alive = append(alive, c.RetentionAlive)
			total += c.RetentionAlive
		}
	}
	if total == 0 {
		return 0, 0
	}
	sort.Sort(sort.Reverse(sort.IntSlice(alive)))
	acc := 0
	for i, v := range alive {
		acc += v
		if float64(acc) >= 0.5*float64(total) {
			return i + 1, float64(acc) / float64(total)
		}
	}
	return len(alive), 1
}

// giniContribution returns the Gini coefficient (0..1) of contribution across
// contributors, measured by total lines changed (additions + deletions). 0
// means everyone contributed equally; values near 1 mean a few people did
// almost all the work.
func (m Model) giniContribution() float64 {
	vals := make([]int, 0, len(m.sum.Contributors))
	for _, c := range m.sum.Contributors {
		vals = append(vals, c.Additions+c.Deletions)
	}
	return giniCoefficient(vals)
}

// giniCoefficient computes the Gini coefficient of a set of non-negative
// values using the sorted-rank formula.
func giniCoefficient(vals []int) float64 {
	n := len(vals)
	if n == 0 {
		return 0
	}
	sorted := append([]int(nil), vals...)
	sort.Ints(sorted)
	var sum, weighted int64
	for i, v := range sorted {
		sum += int64(v)
		weighted += int64(i+1) * int64(v)
	}
	if sum == 0 {
		return 0
	}
	g := (2*float64(weighted))/(float64(n)*float64(sum)) - float64(n+1)/float64(n)
	if g < 0 {
		g = 0
	}
	return g
}

// activitySidebarWidth returns the width for the activity statistics panel, or
// 0 when the terminal is too small to spare room for it.
func (m Model) activitySidebarWidth() int {
	if m.width < 72 || m.height < 18 {
		return 0
	}
	w := 44
	if third := m.width / 3; w > third {
		w = third
	}
	if w < 30 {
		return 0
	}
	return w
}

// activitySidebar renders the calendar and punch-card summary statistics in a
// bordered panel for wide terminals.
func (m Model) activitySidebar(sw, height int) string {
	textW := max(1, sw-4)
	para := func(s string) string { return subtitleStyle.Width(textW).Render(s) }
	dayNames := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	busiestDay, busiest, activeDays := m.dailyStats()
	st := punchStatsOf(m.sum)
	body := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render(" Activity "),
		"",
		nameStyle.Render("Commits per day"),
		para(fmt.Sprintf("busiest: %s (%s commits)",
			busiestDay.Format("Mon Jan 2 2006"), humanize(busiest))),
		para(fmt.Sprintf("%s active days", humanize(activeDays))),
		para(fmt.Sprintf("%s commits total", humanize(m.sum.TotalCommits))),
		"",
		nameStyle.Render("Weekday × hour"),
		para(fmt.Sprintf("busiest slot: %s %02d:00 (%s commits)",
			dayNames[st.bestD], st.bestH, humanize(st.slotCount))),
		para(fmt.Sprintf("most active hour: %02d:00 (%s commits)",
			st.bestHour, humanize(st.bestHourCount))),
		para(fmt.Sprintf("busiest day: %s (%s commits)",
			dayNames[st.bestDay], humanize(st.bestDayCount))),
		"",
		para("times are local to each commit"),
	)
	return sidebarStyle.Width(sw - 2).Height(max(1, height-2)).MaxHeight(height).Render(body)
}
