package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/NimbleMarkets/ntcharts/sparkline"
	"github.com/charmbracelet/lipgloss"
)

// contributorsContent renders the scrollable list of contributor cards.
func (m Model) contributorsContent(width int) string {
	if width <= 0 {
		width = 80
	}
	// Determine scaling bases for bars.
	var maxChanges, maxCommits, maxDirs int
	for _, c := range m.sum.Contributors {
		if c.Additions+c.Deletions > maxChanges {
			maxChanges = c.Additions + c.Deletions
		}
		if c.Commits > maxCommits {
			maxCommits = c.Commits
		}
		if c.DirsTouched > maxDirs {
			maxDirs = c.DirsTouched
		}
	}
	if maxChanges == 0 {
		maxChanges = 1
	}

	innerWidth := width - 4 // account for card border + padding
	if innerWidth < 20 {
		innerWidth = 20
	}
	barWidth := innerWidth - 22
	if barWidth < 10 {
		barWidth = 10
	}
	// Graphs now get their own lines, so they can be wider and taller.
	// Leave room for the first/last commit date columns flanking the graph.
	graphWidth := clamp(innerWidth-32, 10, 60)
	const graphHeight = 3
	// The weekly series run oldest → newest across the repo's active weeks.
	axisNote := func(unit string) string {
		return subtitleStyle.Render(fmt.Sprintf("%s / week", unit)) + "  " +
			helpStyle.Render(fmt.Sprintf("(←older   %d weeks   newer→)", m.sum.WeekCount))
	}

	query := strings.ToLower(strings.TrimSpace(m.query))
	var b strings.Builder
	matched := 0
	// A contributor is "still active" if their last commit lands within
	// stillActiveWeeks of the repository's most recent commit (the dataset's
	// "now"), keeping the flag deterministic across cached runs.
	activeCutoff := m.sum.LastCommit.AddDate(0, 0, -stillActiveWeeks*7)
	for i, c := range m.sum.Contributors {
		if query != "" &&
			!strings.Contains(strings.ToLower(c.Name), query) &&
			!strings.Contains(strings.ToLower(c.Email), query) {
			continue
		}
		if !m.sinceFilter.IsZero() && c.LastSeen.Before(m.sinceFilter) {
			continue
		}
		matched++
		rank := rankStyle.Render(fmt.Sprintf("#%-3d", i+1))
		// Activity status dot before the name: green when the contributor is
		// still active, dark red when dormant.
		active := !c.LastSeen.Before(activeCutoff)
		dotColor := colorGreen
		if !active {
			dotColor = colorDarkRed
		}
		dot := lipgloss.NewStyle().Foreground(dotColor).Render("●")
		who := dot + " " + nameStyle.Render(truncate(c.Name, 28)) + "  " +
			emailStyle.Render("<"+truncate(c.Email, 30)+">")
		line1 := rank + " " + who
		// A cheeky title distilled from the contribution profile
		// (builder/refactorer/balanced) and collaboration breadth
		// (generalist/specialist), followed by the raw labels for reference.
		_, profileLabel := contribProfile(c.Additions, c.Deletions)
		breadthLabel := ""
		if c.FilesTouched > 0 {
			breadthLabel, _ = breadthClass(c.DirsTouched, maxDirs)
		}
		title := contribTitle(profileLabel, breadthLabel)
		// The parenthetical labels keep their own colors: the profile tinted
		// by profileStyle, the breadth by breadthClass' style.
		tags := profileStyle(profileLabel).Render(profileLabel)
		if breadthLabel != "" {
			_, breadthStyle := breadthClass(c.DirsTouched, maxDirs)
			tags += helpStyle.Render(" · ") + breadthStyle.Render(breadthLabel)
		}
		line1 += "  " + profileStyle(profileLabel).Render(title) +
			helpStyle.Render(" (") + tags + helpStyle.Render(")")

		// Impact: balanced 0–100 index (geometric mean of commit count and
		// lines changed, each scaled to the busiest contributor).
		var nc, nl float64
		if maxCommits > 0 {
			nc = float64(c.Commits) / float64(maxCommits)
		}
		if maxChanges > 0 {
			nl = float64(c.Additions+c.Deletions) / float64(maxChanges)
		}
		impact := math.Sqrt(nc * nl)

		// Date columns flanking the graph: first commit on the left,
		// last commit on the right (the series runs oldest → newest).
		firstCol := emailStyle.Render("first\n" + c.FirstSeen.Format("2006-01-02"))
		lastCol := emailStyle.Render("last\n" + c.LastSeen.Format("2006-01-02"))
		flank := func(spark string) string {
			return lipgloss.JoinHorizontal(lipgloss.Center,
				firstCol, "  ", spark, "  ", lastCol)
		}

		var graph string
		if m.metric == metricLines {
			// Diverging chart: additions grow up from the center, deletions grow
			// down from it, both scaled to a shared max so magnitudes compare.
			addSpark, delSpark := divergingBars(c.WeeklyAdd, c.WeeklyDel, graphWidth, graphHeight)
			sparks := lipgloss.JoinVertical(lipgloss.Left, addSpark, delSpark)
			// Left legend: the first commit date tops the block, then the
			// "+ added" / "- removed" series labels. First commit date on the
			// left, last commit date on the right, aligned on the same rows
			// (the series runs oldest → newest).
			labels := []string{
				emailStyle.Render("first:"),
				emailStyle.Render(c.FirstSeen.Format("2006-01-02")),
				addStyle.Render("+ added"),
				delStyle.Render("- removed"),
				"",
				"",
			}
			legend := lipgloss.JoinVertical(lipgloss.Left, labels...)
			lastCol := lipgloss.JoinVertical(lipgloss.Left,
				emailStyle.Render("last:"),
				emailStyle.Render(c.LastSeen.Format("2006-01-02")),
			)
			row := lipgloss.JoinHorizontal(lipgloss.Top, legend, "  ", sparks, "  ", lastCol)
			graph = lipgloss.JoinVertical(lipgloss.Left,
				axisNote("lines added / removed"),
				row,
			)
		} else {
			spark := m.sparklineStyled(c.Weekly, graphWidth, graphHeight, sparkStyle)
			graph = lipgloss.JoinVertical(lipgloss.Left,
				axisNote("commits"),
				flank(spark),
			)
		}

		bar := changeBar(c.Additions, c.Deletions, maxChanges, barWidth)
		totals := addStyle.Render(fmt.Sprintf("+%-7s", humanize(c.Additions))) +
			delStyle.Render(fmt.Sprintf("-%-7s", humanize(c.Deletions))) +
			" " + bar

		// Commit count, impact score and active span (last − first), sitting
		// under the graph and above the retention line.
		activeDays := int(c.LastSeen.Sub(c.FirstSeen).Hours() / 24)
		statsLine := helpStyle.Render("commits  ") +
			commitsStyle.Render(humanize(c.Commits)) +
			helpStyle.Render("  ·  impact  ") +
			nameStyle.Render(fmt.Sprintf("%d", int(100*impact+0.5))) +
			helpStyle.Render("  ·  active  ") +
			nameStyle.Render(fmt.Sprintf("%s days", humanize(activeDays)))

		// Contribution profile: additions/deletions ratio separates builders
		// (net-adders) from refactorers/cleaners (heavy deleters); commit
		// frequency (commits per active week) and a still-active flag summarize
		// tenure vs. recency.
		ratioText, _ := contribProfile(c.Additions, c.Deletions)
		activeWeeks := countActiveWeeks(c.Weekly)
		var perWeek float64
		if activeWeeks > 0 {
			perWeek = float64(c.Commits) / float64(activeWeeks)
		}
		activeText := "active"
		activeTagStyle := addStyle
		if !active {
			activeText = "dormant"
			activeTagStyle = helpStyle
		}
		profileLine := helpStyle.Render("ratio  ") +
			commitsStyle.Render(ratioText) +
			helpStyle.Render(" add/del  ·  ") +
			commitsStyle.Render(fmt.Sprintf("%.1f", perWeek)) +
			helpStyle.Render(" commits/wk  ·  ") +
			activeTagStyle.Render(activeText)
		if c.FilesTouched > 0 {
			profileLine += helpStyle.Render("  ·  ") +
				commitsStyle.Render(humanize(c.FilesTouched)) +
				helpStyle.Render(" files / ") +
				commitsStyle.Render(humanize(c.DirsTouched)) +
				helpStyle.Render(" dirs")
		}

		// Retention: added lines weighted by how long they survived, shown as
		// a rate against the contributor's tracked additions.
		retBase := c.RetentionAdded
		if retBase <= 0 {
			retBase = c.Additions
		}
		var retLine string
		if retBase > 0 {
			rc := 100 * c.RetentionCommits / float64(retBase)
			rd := 100 * c.RetentionDays / float64(retBase)
			kept := humanize(int(c.RetentionCommits + 0.5))
			aliveText := humanize(c.RetentionAlive) + " still alive"
			if c.RetentionAlive <= 0 {
				aliveText = "all dead"
			}
			aliveStyle := lipgloss.NewStyle().Foreground(aliveColor(c.RetentionAlive))
			retLine = helpStyle.Render("retention  ") +
				nameStyle.Render(kept+" lines kept") +
				helpStyle.Render(fmt.Sprintf("  ·  %.0f%% by-commit · %.0f%% by-day  ·  ", rc, rd)) +
				aliveStyle.Render(aliveText)
		} else {
			retLine = helpStyle.Render("retention  n/a")
		}

		card := cardStyle.Width(innerWidth).Render(
			lipgloss.JoinVertical(lipgloss.Left, line1, graph, totals, statsLine, profileLine, retLine))
		b.WriteString(card)
		b.WriteByte('\n')
	}
	if matched == 0 && (query != "" || !m.sinceFilter.IsZero()) {
		return subtitleStyle.Render("No contributors match the current filters.")
	}
	return b.String()
}

// stillActiveWeeks is the recency window (in weeks) within which a contributor
// is considered "still active", measured back from the repository's last commit.
const stillActiveWeeks = 26

// contribProfile classifies a contributor by their additions/deletions ratio.
// Builders add far more than they remove; refactorers/cleaners remove heavily.
// ratioText is a human-readable additions:deletions ratio.
func contribProfile(add, del int) (ratioText, label string) {
	switch {
	case add == 0 && del == 0:
		return "n/a", "idle"
	case del == 0:
		return "∞:1", "builder"
	default:
		r := float64(add) / float64(del)
		ratioText = fmt.Sprintf("%.1f:1", r)
		switch {
		case r >= 2:
			label = "builder"
		case r < 0.5:
			label = "demolisher"
		case r < 1:
			label = "refactorer"
		default:
			label = "balanced"
		}
		return ratioText, label
	}
}

// profileStyle returns the color style for a contribution-profile label:
// green for builders, orange for refactorers, yellow for balanced (and muted
// for anything else, e.g. idle).
func profileStyle(label string) lipgloss.Style {
	switch label {
	case "builder":
		return lipgloss.NewStyle().Bold(true).Foreground(colorGreen)
	case "refactorer":
		return lipgloss.NewStyle().Bold(true).Foreground(colorOrange)
	case "demolisher":
		return lipgloss.NewStyle().Bold(true).Foreground(colorRed)
	case "balanced":
		return lipgloss.NewStyle().Bold(true).Foreground(colorYellow)
	default:
		return helpStyle
	}
}

// contribTitle distils a cheeky honorific from the pairing of contribution
// profile (builder/refactorer/balanced/idle) and collaboration breadth
// (generalist/specialist). When breadth is unknown it falls back to a
// profile-only title.
func contribTitle(profile, breadth string) string {
	switch profile + "/" + breadth {
	case "builder/generalist":
		return "The Architect"
	case "builder/specialist":
		return "The Driller"
	case "refactorer/generalist":
		return "The Janitor"
	case "refactorer/specialist":
		return "The Surgeon"
	case "demolisher/generalist":
		return "The Demolition Expert"
	case "demolisher/specialist":
		return "The Sniper"
	case "balanced/generalist":
		return "The Jack of All Trades"
	case "balanced/specialist":
		return "The Craftsman"
	case "idle/generalist":
		return "The Wandering Ghost"
	case "idle/specialist":
		return "The Dormant Hermit"
	}
	// Breadth unknown (no tracked files): title from profile alone.
	switch profile {
	case "builder":
		return "The Builder"
	case "refactorer":
		return "The Refactorer"
	case "demolisher":
		return "The Demolisher"
	case "balanced":
		return "The All-Rounder"
	default:
		return "The Bystander"
	}
}

// breadthClass classifies a contributor as a generalist (touches many
// directories) or specialist (concentrated in few), relative to the broadest
// contributor in the repository. It returns the label and its color style:
// lilac for generalists, cyan for specialists.
func breadthClass(dirs, maxDirs int) (string, lipgloss.Style) {
	if maxDirs > 0 && dirs*2 >= maxDirs {
		return "generalist", lipgloss.NewStyle().Bold(true).Foreground(colorLilac)
	}
	return "specialist", lipgloss.NewStyle().Bold(true).Foreground(colorCyan)
}

// countActiveWeeks returns the number of week buckets in which the contributor
// made at least one commit.
func countActiveWeeks(weekly []float64) int {
	n := 0
	for _, v := range weekly {
		if v > 0 {
			n++
		}
	}
	return n
}

func (m Model) sparklineStyled(series []float64, width, height int, style lipgloss.Style) string {
	if len(series) == 0 || width <= 0 {
		return ""
	}
	data := resample(series, width)
	sl := sparkline.New(width, height)
	sl.Style = style
	sl.PushAll(data)
	sl.Draw()
	return sl.View()
}

// divergingBars renders two half-block bar charts scaled to a shared maximum:
// the additions series grows upward (baseline at the bottom) and the deletions
// series grows downward (baseline at the top), so their baselines meet in the
// middle. Unlike a naive vertical flip, the downward bars are anchored to the
// top of each cell (using upper-half blocks) so they stay visually connected.
func divergingBars(add, del []float64, width, height int) (string, string) {
	if width <= 0 || height <= 0 {
		return "", ""
	}
	addR := resample(add, width)
	delR := resample(del, width)
	maxV := 0.0
	for _, v := range addR {
		if v > maxV {
			maxV = v
		}
	}
	for _, v := range delR {
		if v > maxV {
			maxV = v
		}
	}
	if maxV <= 0 {
		maxV = 1
	}
	// resample returns the original slice when it is shorter than width, so the
	// two series may have fewer than width columns (and could differ from each
	// other). Render the widest of the two and read missing samples as zero.
	cols := len(addR)
	if len(delR) > cols {
		cols = len(delR)
	}
	at := func(s []float64, i int) float64 {
		if i < len(s) {
			return s[i]
		}
		return 0
	}
	addRows := make([]string, height)
	delRows := make([]string, height)
	for r := 0; r < height; r++ {
		var up, down strings.Builder
		for c := 0; c < cols; c++ {
			up.WriteString(vcell(at(addR, c)/maxV, height, r, false))
			down.WriteString(vcell(at(delR, c)/maxV, height, r, true))
		}
		addRows[r] = addStyle.Render(up.String())
		delRows[r] = delStyle.Render(down.String())
	}
	return strings.Join(addRows, "\n"), strings.Join(delRows, "\n")
}

// vcell returns the block glyph for row r of a bar of fractional height frac
// (0..1) drawn over height rows at half-block resolution. When down is true the
// bar is anchored to the top of the block and uses upper-half blocks.
func vcell(frac float64, height, r int, down bool) string {
	units := int(math.Round(frac * float64(height*2)))
	if units < 0 {
		units = 0
	}
	if units > height*2 {
		units = height * 2
	}
	// Distance of this row from the baseline (0 == baseline row).
	var dist int
	if down {
		dist = r // baseline at the top
	} else {
		dist = height - 1 - r // baseline at the bottom
	}
	u := units - dist*2
	switch {
	case u >= 2:
		return "█"
	case u == 1:
		if down {
			return "▀"
		}
		return "▄"
	default:
		return " "
	}
}

// changeBar renders a proportional additions/deletions bar (green/red).
func changeBar(add, del, maxChanges, width int) string {
	total := add + del
	if total == 0 {
		return lipgloss.NewStyle().Foreground(colorMuted).Render(strings.Repeat("░", width))
	}
	// Overall length scaled to the busiest contributor.
	full := int(float64(width) * float64(total) / float64(maxChanges))
	if full < 1 {
		full = 1
	}
	if full > width {
		full = width
	}
	greenLen := int(float64(full) * float64(add) / float64(total))
	if greenLen == 0 && add > 0 {
		greenLen = 1
	}
	redLen := full - greenLen
	if redLen < 0 {
		redLen = 0
	}
	green := addStyle.Render(strings.Repeat("█", greenLen))
	red := delStyle.Render(strings.Repeat("█", redLen))
	rest := ""
	if full < width {
		rest = lipgloss.NewStyle().Foreground(lipgloss.Color("#30363d")).
			Render(strings.Repeat("░", width-full))
	}
	return green + red + rest
}
