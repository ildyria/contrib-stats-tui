package ui

import "github.com/charmbracelet/lipgloss"

var (
	colorGreen   = lipgloss.Color("#3fb950")
	colorRed     = lipgloss.Color("#f85149")
	colorDarkRed = lipgloss.Color("#8b1a14")
	colorOrange  = lipgloss.Color("#db6d28")
	colorYellow  = lipgloss.Color("#e3b341")
	colorAccent  = lipgloss.Color("#58a6ff")
	colorLilac   = lipgloss.Color("#b392f0")
	colorCyan    = lipgloss.Color("#39d0d8")
	colorBg      = lipgloss.Color("#161b22")

	// colorText and colorMuted adapt to the terminal background so text stays
	// legible on both light and dark themes.
	colorText  = lipgloss.AdaptiveColor{Light: "#24292f", Dark: "#c9d1d9"}
	colorMuted = lipgloss.AdaptiveColor{Light: "#57606a", Dark: "#8b949e"}

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#c9d1d9")).
			Background(lipgloss.Color("#1f6feb")).
			Padding(0, 1)

	subtitleStyle = lipgloss.NewStyle().Foreground(colorMuted)

	tabActiveStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#c9d1d9")).
			Background(lipgloss.Color("#30363d")).
			Padding(0, 2)

	tabInactiveStyle = lipgloss.NewStyle().
				Foreground(colorMuted).
				Padding(0, 2)

	rankStyle    = lipgloss.NewStyle().Bold(true).Foreground(colorAccent)
	nameStyle    = lipgloss.NewStyle().Bold(true).Foreground(colorText)
	emailStyle   = lipgloss.NewStyle().Foreground(colorMuted)
	commitsStyle = lipgloss.NewStyle().Foreground(colorText)
	addStyle     = lipgloss.NewStyle().Foreground(colorGreen)
	delStyle     = lipgloss.NewStyle().Foreground(colorRed)
	sparkStyle   = lipgloss.NewStyle().Foreground(colorAccent)
	helpStyle    = lipgloss.NewStyle().Foreground(colorMuted)

	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#30363d")).
			Padding(0, 1).
			MarginBottom(1)

	// sidebarStyle frames the explanatory side panel shown next to the
	// contributor list (e.g. the retention metric help).
	sidebarStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorAccent).
			Padding(0, 1)

	// sectionStyle is a prominent header used to title the calendar and
	// punch-card sections.
	sectionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#c9d1d9")).
			Background(lipgloss.Color("#1f6feb")).
			Padding(0, 1)

	// summaryStyle highlights the key takeaway line under a heatmap.
	summaryStyle = lipgloss.NewStyle().Bold(true).Foreground(colorGreen)

	// labelStyle renders muted descriptive text; valueStyle renders the white
	// numeric values highlighted next to those labels.
	labelStyle = lipgloss.NewStyle().Foreground(colorMuted)
	valueStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ffffff"))
)

// aliveColor grades the "still alive" line count: dark red when nothing
// survives, then red → orange → yellow → green as more lines remain in HEAD.
func aliveColor(n int) lipgloss.TerminalColor {
	switch {
	case n <= 0:
		return colorDarkRed
	case n >= 1000:
		return colorGreen
	case n >= 500:
		return colorYellow
	case n >= 100:
		return colorOrange
	default:
		return colorRed
	}
}

// heatColors is the heatmap intensity palette (index 0 -> empty, higher
// indices -> busier). It ramps warm-to-cool across a wide range of hues
// (orange → yellow → green → teal → cyan → blue) so adjacent levels are easy to
// tell apart on the calendar and punch-card heatmaps. Blue (not red) marks the
// peak, since more activity is a good thing. The empty cell adapts to the
// terminal background so it stays visible on light themes.
var heatColors = []lipgloss.TerminalColor{
	lipgloss.AdaptiveColor{Light: "#ebedf0", Dark: "#161b22"},
	lipgloss.Color("#93430f"),
	lipgloss.Color("#c9a218"),
	lipgloss.Color("#2ea043"),
	lipgloss.Color("#17b8a6"),
	lipgloss.Color("#22c3e6"),
	lipgloss.Color("#1f5fe0"),
}
