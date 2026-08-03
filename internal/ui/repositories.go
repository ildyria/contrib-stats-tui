package ui

// repositories.go renders the Repositories tab: a selectable list of the
// analyzed repositories plus an aggregate ("All repositories") entry, with any
// repositories that failed to collect listed below. It is only shown in
// multi-repository mode.

import (
	"fmt"
	"strings"

	"github.com/ildyria/contrib-stats-tui/internal/gitstats"

	"github.com/charmbracelet/lipgloss"
)

// repositoriesView renders the list of repositories with per-repo headline
// stats. The row under the cursor is highlighted and the active selection is
// marked; failed repositories are listed at the bottom.
func (m Model) repositoriesView() string {
	var b strings.Builder
	b.WriteString(sectionStyle.Render(" Repositories "))
	b.WriteString("\n\n")

	// Build the selectable rows: index 0 is the aggregate, 1..N the repos.
	rows := make([]string, 0, len(m.repos)+1)
	rows = append(rows, m.repoRow(0, "All repositories", m.global, false))
	for i, s := range m.repos {
		cached := false
		if i < len(m.results) && m.results[i].Cached {
			cached = true
		}
		rows = append(rows, m.repoRow(i+1, s.Repo, s, cached))
	}

	for i, r := range rows {
		cursor := "  "
		if i == m.repoCur {
			cursor = lipgloss.NewStyle().Foreground(colorAccent).Render("▸ ")
		}
		marker := " "
		if i == m.selected {
			marker = lipgloss.NewStyle().Foreground(colorGreen).Render("●")
		}
		line := cursor + marker + " " + r
		if i == m.repoCur {
			line = lipgloss.NewStyle().Background(colorBg).Render(line)
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}

	// Failed repositories, if any.
	var failed []gitstats.RepoResult
	for _, r := range m.results {
		if r.Err != nil {
			failed = append(failed, r)
		}
	}
	if len(failed) > 0 {
		b.WriteByte('\n')
		b.WriteString(sectionStyle.Render(" Skipped "))
		b.WriteString("\n\n")
		for _, r := range failed {
			name := r.Spec.DisplayName
			if name == "" {
				name = r.Spec.Raw
			}
			b.WriteString("  " +
				delStyle.Render("✗ ") + nameStyle.Render(name) + "  " +
				helpStyle.Render(truncate(r.Err.Error(), max(20, m.width-len(name)-10))))
			b.WriteByte('\n')
		}
	}

	b.WriteByte('\n')
	b.WriteString(helpStyle.Render("Select a repository to view its stats, or “All repositories” for the aggregate."))
	return b.String()
}

// repoRow formats a single repository row: its name followed by headline stats.
func (m Model) repoRow(_ int, name string, s *gitstats.Summary, cached bool) string {
	label := nameStyle.Render(fmt.Sprintf("%-24s", truncate(name, 24)))
	if s == nil || s.TotalCommits == 0 {
		return label + subtitleStyle.Render("  (no data)")
	}
	stats := fmt.Sprintf("%s commits  %s  %s  %s contributors",
		humanize(s.TotalCommits),
		addStyle.Render("+"+humanize(s.Additions)),
		delStyle.Render("-"+humanize(s.Deletions)),
		humanize(len(s.Contributors)),
	)
	span := ""
	if !s.FirstCommit.IsZero() {
		span = fmt.Sprintf("  %s → %s",
			s.FirstCommit.Format("Jan 2006"),
			s.LastCommit.Format("Jan 2006"))
	}
	out := label + "  " + subtitleStyle.Render(stats+span)
	if cached {
		out += "  " + lipgloss.NewStyle().Foreground(colorAccent).Render("⚡")
	}
	return out
}

// reposSidebar renders a compact panel listing every scanned repository (plus
// the aggregate entry) so the activity tab shows at a glance what was analyzed.
// The currently-selected entry is marked, cached repos carry a ⚡, and any
// skipped repositories are listed at the bottom. The list scrolls to keep the
// selection visible when it is taller than the panel. It is empty outside
// multi-repo mode or when the terminal is too narrow.
func (m Model) reposSidebar() string {
	sw := m.reposSidebarWidth()
	if sw == 0 {
		return ""
	}
	textW := max(1, sw-4)
	h := m.avp.Height
	inner := max(1, h-2)
	header, items, focus := m.reposListParts(textW)
	items = scrollLines(items, focus, max(1, inner-len(header)))
	body := strings.Join(append(header, items...), "\n")
	return sidebarStyle.Width(sw - 2).Height(inner).MaxHeight(h).Render(body)
}

// reposListParts builds the shared "scanned repositories" content used by both
// the activity-tab sidebar and the contributors-tab sidebar. It returns the
// fixed header lines, the scrollable item lines (the aggregate entry, each repo
// with headline counts, and any skipped repositories), and the item-line index
// of the currently-selected entry so callers can keep it in view. textW is the
// available text width inside the panel.
func (m Model) reposListParts(textW int) (header, items []string, focus int) {
	add := func(idx int, name string, s *gitstats.Summary, cached bool) {
		marker := "  "
		if idx == m.selected {
			marker = lipgloss.NewStyle().Foreground(colorGreen).Render("● ")
			focus = len(items)
		}
		items = append(items, marker+nameStyle.Render(truncate(name, textW-2)))
		detail := subtitleStyle.Render("   (no data)")
		if s != nil && s.TotalCommits > 0 {
			d := fmt.Sprintf("   %s commits · %s contributors",
				humanize(s.TotalCommits), humanize(len(s.Contributors)))
			detail = subtitleStyle.Render(truncate(d, textW))
		}
		if cached {
			detail += " " + lipgloss.NewStyle().Foreground(colorAccent).Render("⚡")
		}
		items = append(items, detail)
	}

	header = []string{titleStyle.Render(" Scanned repositories "), ""}
	add(0, "All repositories", m.global, false)
	for i, s := range m.repos {
		items = append(items, "")
		cached := i < len(m.results) && m.results[i].Cached
		add(i+1, s.Repo, s, cached)
	}

	var skipped []string
	for _, r := range m.results {
		if r.Err != nil {
			name := r.Spec.DisplayName
			if name == "" {
				name = r.Spec.Raw
			}
			skipped = append(skipped, delStyle.Render("✗ ")+subtitleStyle.Render(truncate(name, textW-2)))
		}
	}
	if len(skipped) > 0 {
		items = append(items, "", subtitleStyle.Render("Skipped"))
		items = append(items, skipped...)
	}
	return header, items, focus
}

// scrollLines returns at most maxLines of lines, shifted so the line at focus
// stays visible. When content is clipped, the first and/or last returned line is
// replaced with a "▲ N more" / "▼ N more" indicator so the truncation is clear.
func scrollLines(lines []string, focus, maxLines int) []string {
	if maxLines <= 0 || len(lines) <= maxLines {
		return lines
	}
	off := focus - maxLines/2
	if off < 0 {
		off = 0
	}
	if off > len(lines)-maxLines {
		off = len(lines) - maxLines
	}
	win := make([]string, maxLines)
	copy(win, lines[off:off+maxLines])
	if off > 0 {
		win[0] = subtitleStyle.Render(fmt.Sprintf("  ▲ %d more", off))
	}
	if off+maxLines < len(lines) {
		win[maxLines-1] = subtitleStyle.Render(fmt.Sprintf("  ▼ %d more", len(lines)-(off+maxLines)))
	}
	return win
}
