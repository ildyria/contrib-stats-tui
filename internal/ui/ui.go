// Package ui implements the bubbletea terminal UI that renders the
// GitHub-style contributors page and contribution calendar.
package ui

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/ildyria/contrib-stats-tui/internal/gitstats"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tab int

const (
	tabActivity tab = iota
	tabContributors
	tabRepositories
)

// metric selects which activity series is shown for a contributor.
type metric int

const (
	metricCommits metric = iota
	metricLines
)

// Model is the root bubbletea model.
type Model struct {
	specs       []gitstats.RepoSpec
	useCache    bool
	ignore      []string
	excludeDocs bool
	identities  []gitstats.Identity
	window      gitstats.Window
	sum         *gitstats.Summary
	cached      bool
	active      tab
	metric      metric
	sortBy      gitstats.SortKey
	weekStart   time.Weekday

	// multi-repository state
	repos    []*gitstats.Summary   // per-repo summaries (successful only)
	global   *gitstats.Summary     // merged aggregate across repos
	results  []gitstats.RepoResult // per-spec outcomes (incl. errors)
	selected int                   // 0 = All/global, 1..N = repos[selected-1]
	repoCur  int                   // cursor in the Repositories list

	// contributor name search
	searching bool
	query     string

	// last-commit date filter (contributors with an older last commit are hidden)
	filtering   bool
	dateInput   string
	sinceFilter time.Time

	// loading state
	loading   bool
	progress  progress.Model
	scanCh    chan any
	scanDone  int
	scanTot   int
	cheekyIdx int
	// cheekyOrder is a shuffled permutation of cheekyLines indices; cheekyPos
	// walks through it so every flavor line is shown once before any repeats.
	cheekyOrder []int
	cheekyPos   int
	err         error

	vp        viewport.Model
	avp       viewport.Model // vertical scroll for the activity tab
	calOffset int            // horizontal scroll offset in weeks for the calendar
	width     int
	height    int
	ready     bool
}

// New builds the root model. It scans the given repositories for git statistics
// after the program starts, showing a progress bar while doing so. When
// useCache is true, results are cached to disk keyed by each repository HEAD to
// avoid rescanning. weekStart controls which weekday the calendar and punch-card
// rows begin on. ignore lists contributor names/emails whose commits are
// excluded. excludeDocs drops Markdown files and docs/ folders from all
// statistics. identities aggregates several git author identities (matching
// emails/usernames) into a single contributor. When more than one repository is
// given, an aggregate ("All repositories") view and a Repositories tab are
// enabled.
func New(specs []gitstats.RepoSpec, weekStart time.Weekday, useCache bool, ignore []string, excludeDocs bool, identities []gitstats.Identity, window gitstats.Window) Model {
	return Model{
		specs:       specs,
		useCache:    useCache,
		ignore:      ignore,
		excludeDocs: excludeDocs,
		identities:  identities,
		window:      window,
		active:      tabActivity,
		metric:      metricCommits,
		sortBy:      gitstats.SortImpact,
		weekStart:   weekStart,
		loading:     true,
		progress:    progress.New(progress.WithDefaultGradient()),
		scanCh:      make(chan any, 16),
	}
}

// multi reports whether more than one repository is being analyzed, which
// enables the aggregate view and the Repositories tab.
func (m Model) multi() bool { return len(m.repos) > 1 }

// tabCount returns the number of tabs available: three (Activity,
// Contributors, Repositories) in multi-repo mode, two otherwise.
func (m Model) tabCount() int {
	if m.multi() {
		return 3
	}
	return 2
}

// current returns the summary for the active selection: the global aggregate
// when selected is 0 in multi-repo mode, otherwise the chosen repository.
func (m Model) current() *gitstats.Summary {
	if len(m.repos) == 0 {
		return m.global
	}
	if !m.multi() {
		return m.repos[0]
	}
	if m.selected <= 0 {
		return m.global
	}
	if m.selected-1 < len(m.repos) {
		return m.repos[m.selected-1]
	}
	return m.global
}

// selectionName returns a human-readable label for the active selection.
func (m Model) selectionName() string {
	if s := m.current(); s != nil {
		return s.Repo
	}
	return ""
}

// setSelected changes the active repository selection (0 = All/global, 1..N =
// repos[sel-1]), re-sorts and re-renders the dependent views.
func (m *Model) setSelected(sel int) {
	if !m.multi() {
		return
	}
	if sel < 0 || sel > len(m.repos) {
		return
	}
	m.selected = sel
	m.repoCur = sel
	m.sum = m.current()
	if m.sum != nil {
		gitstats.SortBy(m.sum.Contributors, m.sortBy)
	}
	if m.ready {
		m.vp.GotoTop()
		m.avp.GotoTop()
		m.calOffset = 0
		m.refreshContributors()
		m.refreshActivity()
	}
}

// scanTarget describes what is being scanned, shown under the progress bar
// before any commit counts are available.
func (m Model) scanTarget() string {
	switch len(m.specs) {
	case 0:
		return ""
	case 1:
		if m.specs[0].DisplayName != "" {
			return m.specs[0].DisplayName
		}
		return m.specs[0].Raw
	default:
		return fmt.Sprintf("%d repositories", len(m.specs))
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.startScan(), waitForScan(m.scanCh), cheekyTick())
}

// Err returns the error, if any, encountered while scanning the repository.
func (m Model) Err() error { return m.err }

// startScan launches the background collector goroutine.
func (m Model) startScan() tea.Cmd {
	ch := m.scanCh
	specs := m.specs
	useCache := m.useCache
	ignore := m.ignore
	excludeDocs := m.excludeDocs
	identities := m.identities
	window := m.window
	return func() tea.Msg {
		go func() {
			repos, global, results := gitstats.CollectMulti(specs, useCache, ignore, excludeDocs, identities, window, func(done, total int) {
				select {
				case ch <- scanProgressMsg{done: done, total: total}:
				default: // drop intermediate updates if the UI is busy
				}
			})
			var err error
			if len(repos) == 0 {
				err = firstScanError(results)
			}
			cached := false
			if len(results) == 1 {
				cached = results[0].Cached
			}
			ch <- scanDoneMsg{repos: repos, global: global, results: results, cached: cached, err: err}
		}()
		return nil
	}
}

// firstScanError returns a representative error when every repository failed to
// collect, summarizing how many were skipped.
func firstScanError(results []gitstats.RepoResult) error {
	var first error
	failed := 0
	for _, r := range results {
		if r.Err != nil {
			failed++
			if first == nil {
				first = r.Err
			}
		}
	}
	if first == nil {
		return fmt.Errorf("no repositories to analyze")
	}
	if failed > 1 {
		return fmt.Errorf("%w (and %d more repositories failed)", first, failed-1)
	}
	return first
}

// waitForScan blocks on the scan channel and forwards the next message.
func waitForScan(ch chan any) tea.Cmd {
	return func() tea.Msg { return <-ch }
}

// cheekyTick schedules the next rotation of the loading flavor text.
func cheekyTick() tea.Cmd {
	return tea.Tick(1000*time.Millisecond, func(time.Time) tea.Msg {
		return cheekyTickMsg{}
	})
}

// advanceCheeky moves to the next flavor line in a shuffled deck of all
// cheekyLines, so none repeats until every line has been shown. When the deck
// runs out it is reshuffled, avoiding an immediate repeat across the boundary.
func (m *Model) advanceCheeky() {
	n := len(cheekyLines)
	if n <= 1 {
		return
	}
	if m.cheekyPos >= len(m.cheekyOrder) {
		last := -1
		if len(m.cheekyOrder) > 0 {
			last = m.cheekyOrder[len(m.cheekyOrder)-1]
		}
		order := rand.Perm(n)
		if order[0] == last {
			order[0], order[len(order)-1] = order[len(order)-1], order[0]
		}
		m.cheekyOrder = order
		m.cheekyPos = 0
	}
	m.cheekyIdx = m.cheekyOrder[m.cheekyPos]
	m.cheekyPos++
}

// ---- update -------------------------------------------------------------

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.progress.Width = clamp(msg.Width-4, 10, 60)
		headerH := m.headerHeight()
		if !m.ready {
			m.vp = viewport.New(m.contentWidth(), max(1, msg.Height-headerH))
			m.avp = viewport.New(m.activityContentWidth(), max(1, msg.Height-headerH))
			m.ready = true
		} else {
			m.vp.Height = max(1, msg.Height-headerH)
			m.avp.Width = m.activityContentWidth()
			m.avp.Height = max(1, msg.Height-headerH)
		}
		m.refreshContributors()
		m.refreshActivity()
		return m, nil

	case scanProgressMsg:
		m.scanDone = msg.done
		m.scanTot = msg.total
		var cmd tea.Cmd
		if msg.total > 0 {
			cmd = m.progress.SetPercent(float64(msg.done) / float64(msg.total))
		}
		return m, tea.Batch(cmd, waitForScan(m.scanCh))

	case scanDoneMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.repos = msg.repos
		m.global = msg.global
		m.results = msg.results
		m.cached = msg.cached
		// Default selection: the aggregate view in multi-repo mode, else the
		// single repository.
		if len(m.repos) > 1 {
			m.selected = 0
		} else {
			m.selected = 0
		}
		m.sum = m.current()
		m.loading = false
		m.refreshContributors()
		m.refreshActivity()
		return m, m.progress.SetPercent(1.0)

	case cheekyTickMsg:
		if !m.loading {
			return m, nil
		}
		// Walk a shuffled permutation so every flavor line appears once before
		// any repeats; reshuffle when the deck is exhausted.
		m.advanceCheeky()
		return m, cheekyTick()

	case progress.FrameMsg:
		pm, cmd := m.progress.Update(msg)
		m.progress = pm.(progress.Model)
		return m, cmd

	case tea.KeyMsg:
		if m.loading {
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			}
			return m, nil
		}

		// Search mode captures typed input to filter the contributor list.
		if m.searching {
			switch msg.Type {
			case tea.KeyCtrlC:
				return m, tea.Quit
			case tea.KeyEsc:
				m.searching = false
				m.query = ""
			case tea.KeyEnter:
				m.searching = false
			case tea.KeyBackspace:
				if r := []rune(m.query); len(r) > 0 {
					m.query = string(r[:len(r)-1])
				}
			case tea.KeySpace:
				m.query += " "
			case tea.KeyRunes:
				m.query += string(msg.Runes)
			}
			if m.ready && m.sum != nil {
				m.vp.GotoTop()
				m.refreshContributors()
			}
			return m, nil
		}

		// Date-filter mode captures a YYYY-MM-DD "since" date.
		if m.filtering {
			switch msg.Type {
			case tea.KeyCtrlC:
				return m, tea.Quit
			case tea.KeyEsc:
				m.filtering = false
				m.dateInput = ""
				m.sinceFilter = time.Time{}
			case tea.KeyEnter:
				m.filtering = false
				if s := strings.TrimSpace(m.dateInput); s != "" {
					if t, err := time.Parse("2006-01-02", s); err == nil {
						m.sinceFilter = t
					}
				} else {
					m.sinceFilter = time.Time{}
				}
			case tea.KeyBackspace:
				if r := []rune(m.dateInput); len(r) > 0 {
					m.dateInput = string(r[:len(r)-1])
				}
			case tea.KeyRunes:
				m.dateInput += string(msg.Runes)
			}
			if m.ready && m.sum != nil {
				m.vp.GotoTop()
				m.refreshContributors()
			}
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "/":
			if m.active == tabContributors {
				m.searching = true
				return m, nil
			}
		case "f":
			if m.active == tabContributors {
				m.filtering = true
				m.dateInput = m.sinceFilter.Format("2006-01-02")
				if m.sinceFilter.IsZero() {
					m.dateInput = ""
				}
				return m, nil
			}
		case "esc":
			if m.query != "" || !m.sinceFilter.IsZero() {
				m.query = ""
				m.sinceFilter = time.Time{}
				m.dateInput = ""
				if m.ready && m.sum != nil {
					m.vp.GotoTop()
					m.refreshContributors()
				}
			}
			return m, nil
		case "right", "l":
			if m.active == tabActivity {
				m.calOffset++
				m.refreshActivity()
				return m, nil
			}
			m.active = (m.active + 1) % tab(m.tabCount())
			return m, nil
		case "left", "h":
			if m.active == tabActivity {
				if m.calOffset > 0 {
					m.calOffset--
				}
				m.refreshActivity()
				return m, nil
			}
			m.active = (m.active + tab(m.tabCount()) - 1) % tab(m.tabCount())
			return m, nil
		case "tab":
			m.active = (m.active + 1) % tab(m.tabCount())
			return m, nil
		case "shift+tab":
			m.active = (m.active + tab(m.tabCount()) - 1) % tab(m.tabCount())
			return m, nil
		case "1":
			m.active = tabActivity
			return m, nil
		case "2":
			m.active = tabContributors
			return m, nil
		case "3":
			if m.multi() {
				m.active = tabRepositories
			}
			return m, nil
		case "]", ")":
			// Cycle the active selection forward across All + each repository.
			if m.multi() {
				m.setSelected((m.selected + 1) % (len(m.repos) + 1))
			}
			return m, nil
		case "[", "(":
			// Cycle the active selection backward across All + each repository.
			if m.multi() {
				m.setSelected((m.selected + len(m.repos)) % (len(m.repos) + 1))
			}
			return m, nil
		case "up", "k":
			if m.active == tabRepositories {
				if m.repoCur > 0 {
					m.repoCur--
				}
				return m, nil
			}
		case "down", "j":
			if m.active == tabRepositories {
				if m.repoCur < len(m.repos) {
					m.repoCur++
				}
				return m, nil
			}
		case "enter":
			if m.active == tabRepositories {
				m.setSelected(m.repoCur)
				m.active = tabActivity
				return m, nil
			}
		case "m":
			// Toggle the contributor activity metric.
			if m.metric == metricCommits {
				m.metric = metricLines
			} else {
				m.metric = metricCommits
			}
			if m.ready && m.sum != nil {
				m.refreshContributors()
			}
			return m, nil
		case "s":
			// Cycle contributor sort order forward:
			// impact → commits → changes → recent → retention → alive → active.
			switch m.sortBy {
			case gitstats.SortImpact:
				m.sortBy = gitstats.SortCommits
			case gitstats.SortCommits:
				m.sortBy = gitstats.SortChanges
			case gitstats.SortChanges:
				m.sortBy = gitstats.SortRecent
			case gitstats.SortRecent:
				m.sortBy = gitstats.SortRetention
			case gitstats.SortRetention:
				m.sortBy = gitstats.SortAlive
			case gitstats.SortAlive:
				m.sortBy = gitstats.SortActive
			default:
				m.sortBy = gitstats.SortImpact
			}
			if m.sum != nil {
				gitstats.SortBy(m.sum.Contributors, m.sortBy)
			}
			if m.ready && m.sum != nil {
				m.vp.GotoTop()
				m.refreshContributors()
			}
			return m, nil
		case "S":
			// Cycle contributor sort order backward (SHIFT+S):
			// impact → active → alive → retention → recent → changes → commits.
			switch m.sortBy {
			case gitstats.SortCommits:
				m.sortBy = gitstats.SortImpact
			case gitstats.SortChanges:
				m.sortBy = gitstats.SortCommits
			case gitstats.SortRecent:
				m.sortBy = gitstats.SortChanges
			case gitstats.SortRetention:
				m.sortBy = gitstats.SortRecent
			case gitstats.SortAlive:
				m.sortBy = gitstats.SortRetention
			case gitstats.SortActive:
				m.sortBy = gitstats.SortAlive
			default:
				m.sortBy = gitstats.SortActive
			}
			if m.sum != nil {
				gitstats.SortBy(m.sum.Contributors, m.sortBy)
			}
			if m.ready && m.sum != nil {
				m.vp.GotoTop()
				m.refreshContributors()
			}
			return m, nil
		case "home", "g":
			if m.active == tabActivity {
				m.calOffset = 0
				m.refreshActivity()
				m.avp.GotoTop()
			}
		case "end", "G":
			if m.active == tabActivity {
				m.calOffset = 1 << 30 // clamped during render
				m.refreshActivity()
			}
		}
	}

	if !m.loading && m.ready {
		switch m.active {
		case tabContributors:
			var cmd tea.Cmd
			m.vp, cmd = m.vp.Update(msg)
			return m, cmd
		case tabActivity:
			var cmd tea.Cmd
			m.avp, cmd = m.avp.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

// ---- view ---------------------------------------------------------------

func (m Model) View() string {
	if !m.ready {
		return "starting…"
	}
	if m.loading {
		return m.loadingView()
	}
	var b strings.Builder
	b.WriteString(m.header())
	b.WriteByte('\n')
	switch m.active {
	case tabContributors:
		mainBlock := m.vp.View()
		if sidebar := m.contribSidebar(); sidebar != "" {
			b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, mainBlock, " ", sidebar))
		} else {
			b.WriteString(mainBlock)
		}
	case tabActivity:
		mainBlock := m.avp.View()
		if sidebar := m.reposSidebar(); sidebar != "" {
			b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, mainBlock, " ", sidebar))
		} else {
			b.WriteString(mainBlock)
		}
	case tabRepositories:
		b.WriteString(m.repositoriesView())
	}
	return b.String()
}

// sidebarWidth returns the width reserved for the explanatory side panel, or 0
// when the terminal is too small to spare room for it. Every sort mode has a
// panel describing what it ranks by.
func (m Model) sidebarWidth() int {
	if m.width < 72 || m.height < 18 {
		return 0
	}
	w := 40
	if third := m.width / 3; w > third {
		w = third
	}
	if w < 28 {
		return 0
	}
	return w
}

// contentWidth is the width available to the contributor list, leaving room for
// the side panel when it is shown.
func (m Model) contentWidth() int {
	sw := m.sidebarWidth()
	if sw == 0 {
		return m.width
	}
	return max(20, m.width-sw-1)
}

// reposSidebarWidth returns the width reserved for the scanned-repositories side
// panel on the activity tab, or 0 when not in multi-repo mode or the terminal
// is too small.
func (m Model) reposSidebarWidth() int {
	if !m.multi() {
		return 0
	}
	return m.sidebarWidth()
}

// activityContentWidth is the width available to the activity tab content,
// leaving room for the scanned-repositories side panel when it is shown.
func (m Model) activityContentWidth() int {
	sw := m.reposSidebarWidth()
	if sw == 0 {
		return m.width
	}
	return max(20, m.width-sw-1)
}

// refreshContributors re-renders the contributor list at the current content
// width, which shrinks to leave room for the retention sidebar when active.
func (m *Model) refreshContributors() {
	if !m.ready {
		return
	}
	w := m.contentWidth()
	m.vp.Width = w
	if m.sum != nil {
		m.vp.SetContent(m.contributorsContent(w))
	}
}

// refreshActivity re-renders the activity tab content into its own vertical
// viewport so the stacked calendar, churn graph, punch card and risk panels can
// be scrolled when they exceed the terminal height.
func (m *Model) refreshActivity() {
	if !m.ready || m.sum == nil {
		return
	}
	h := max(1, m.height-m.headerHeight())
	w := m.activityContentWidth()
	m.avp.Width = w
	m.avp.Height = h
	m.avp.SetContent(m.activityView(w, h))
}

// contribSidebar returns an explanatory panel describing the active sort mode
// and what it ranks by, shown to the right of the contributor list.
func (m Model) contribSidebar() string {
	sw := m.sidebarWidth()
	if sw == 0 {
		return ""
	}
	textW := max(1, sw-4)
	para := func(s string) string { return subtitleStyle.Width(textW).Render(s) }
	var body string
	switch m.sortBy {
	case gitstats.SortImpact:
		body = lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render(" Impact "),
			"",
			para("A balanced rank that rewards both breadth and volume of work."),
			"",
			para("Each contributor is scored on two axes, each scaled to the busiest contributor:"),
			para("• commits made"),
			para("• lines changed (added + removed)"),
			"",
			para("The two are combined with a geometric mean, so a contributor must score on BOTH to rank highly."),
			"",
			para("This avoids the bias of ranking by commits alone (many tiny commits) or by lines alone (a few huge or generated changes)."),
			"",
			nameStyle.Render("impact"),
			para("the combined score, shown as a 0–100 index (100 = top on both axes)"),
		)
	case gitstats.SortCommits:
		body = lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render(" Commits "),
			"",
			para("Ranks by the total number of commits each contributor authored."),
			"",
			para("A simple measure of how often someone lands changes."),
			"",
			para("Note: it counts commits regardless of size, so many tiny commits can outrank a few large ones. Use Impact for a size-aware view."),
			"",
			nameStyle.Render("commits"),
			para("total commits authored"),
		)
	case gitstats.SortChanges:
		body = lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render(" Changes "),
			"",
			para("Ranks by the total lines changed: additions + deletions across all commits."),
			"",
			para("A measure of raw code volume touched."),
			"",
			para("Note: large or generated files can inflate this, and a rewrite counts both the removed and added lines. Use Impact for a balanced view."),
			"",
			nameStyle.Render("+ / -"),
			para("lines added / removed"),
		)
	case gitstats.SortRecent:
		body = lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render(" Recent "),
			"",
			para("Ranks by most recent commit date — who has been active latest."),
			"",
			para("Useful for spotting current maintainers versus contributors who have moved on."),
			"",
			nameStyle.Render("last"),
			para("date of the contributor's most recent commit"),
		)
	case gitstats.SortActive:
		body = lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render(" Active span "),
			"",
			para("Ranks by how many days separate a contributor's first and last commit."),
			"",
			para("A measure of longevity — how long someone has been around — rather than how intensely they worked."),
			"",
			para("Someone with two commits a year apart outranks someone with a busy single week."),
			"",
			nameStyle.Render("active"),
			para("days from first to last commit (last − first)"),
		)
	case gitstats.SortAlive:
		body = lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render(" Still alive "),
			"",
			para("Ranks by how many of a contributor's added lines are still present in the current HEAD."),
			"",
			para("Surviving code that nobody has removed or rewritten — the lasting footprint."),
			"",
			nameStyle.Render("still alive"),
			para("added lines still in HEAD, color-graded:"),
			lipgloss.NewStyle().Foreground(colorDarkRed).Render("all dead")+para(" (0)"),
			lipgloss.NewStyle().Foreground(colorRed).Render("red")+para(" (< 100)"),
			lipgloss.NewStyle().Foreground(colorOrange).Render("orange")+para(" (≥ 100)"),
			lipgloss.NewStyle().Foreground(colorYellow).Render("yellow")+para(" (≥ 500)"),
			lipgloss.NewStyle().Foreground(colorGreen).Render("green")+para(" (≥ 1000)"),
		)
	case gitstats.SortRetention:
		body = lipgloss.JoinVertical(lipgloss.Left,
			titleStyle.Render(" Retention "),
			"",
			para("Lines you added, weighted by how long they survive in the code."),
			"",
			para("Every added line scores 0–1:"),
			para("• still in HEAD → 1 (max)"),
			para("• removed later → the fraction of its possible lifetime it lasted"),
			para("• removed same day → 0 (by-day)"),
			"",
			para("The score sums these weights, so it grows with how much you wrote AND how long it lasted — not just a percentage."),
			"",
			nameStyle.Render("lines kept"),
			para("retained-line-equivalents (the score)"),
			"",
			nameStyle.Render("still alive"),
			para("added lines still present in HEAD"),
			"",
			nameStyle.Render("by-commit / by-day"),
			para("lifetime measured in commits / days"),
			"",
			nameStyle.Render("%"),
			para("score ÷ lines you added"),
		)
	default:
		return ""
	}
	// In multi-repo mode, append the scanned-repositories list below the sort
	// explanation so the repo list stays visible on the contributors tab too.
	// The list scrolls within the remaining height to keep the selection shown.
	h := m.vp.Height
	inner := max(1, h-2)
	if m.multi() {
		preamble := []string{body, "", strings.Repeat("─", textW)}
		preLines := strings.Count(strings.Join(preamble, "\n"), "\n") + 1
		header, items, focus := m.reposListParts(textW)
		budget := max(1, inner-preLines-len(header))
		items = scrollLines(items, focus, budget)
		rows := append(preamble, header...)
		rows = append(rows, items...)
		body = strings.Join(rows, "\n")
	}
	return sidebarStyle.Width(sw - 2).Height(inner).MaxHeight(h).Render(body)
}

func (m Model) loadingView() string {
	title := titleStyle.Render(" Contributors ")
	detail := ""
	if m.scanTot > 0 {
		detail = subtitleStyle.Render(fmt.Sprintf("%s / %s commits",
			humanize(m.scanDone), humanize(m.scanTot)))
	} else if m.scanDone > 0 {
		detail = subtitleStyle.Render(fmt.Sprintf("%s commits", humanize(m.scanDone)))
	} else {
		detail = subtitleStyle.Render(m.scanTarget())
	}
	bar := m.progress.View()
	cheeky := nameStyle.Render(cheekyLines[m.cheekyIdx%len(cheekyLines)])
	body := lipgloss.JoinVertical(lipgloss.Center,
		title, "", cheeky, bar, detail, "", helpStyle.Render("q quit"))

	if m.width > 0 && m.height > 0 {
		return lipgloss.Place(m.width, m.height,
			lipgloss.Center, lipgloss.Center, body)
	}
	return body
}

func (m Model) headerHeight() int {
	// title line + stats line + tabs line + blank separators + help line
	return 6
}

func (m Model) header() string {
	repo := m.sum.Repo
	if i := strings.LastIndexByte(repo, '/'); i >= 0 {
		repo = repo[i+1:]
	}
	title := titleStyle.Render(" Contributors ") + " " +
		nameStyle.Render(repo)

	stats := fmt.Sprintf("%s commits   %s   %s   %s contributors",
		humanize(m.sum.TotalCommits),
		addStyle.Render("+"+humanize(m.sum.Additions)),
		delStyle.Render("-"+humanize(m.sum.Deletions)),
		humanize(len(m.sum.Contributors)),
	)
	span := ""
	if !m.sum.FirstCommit.IsZero() {
		span = fmt.Sprintf("   %s → %s",
			m.sum.FirstCommit.Format("Jan 2006"),
			m.sum.LastCommit.Format("Jan 2006"))
	}
	stats = subtitleStyle.Render(stats + span)
	if m.cached {
		stats += "   " + lipgloss.NewStyle().Foreground(colorAccent).Render("⚡ cached")
	}

	var tabs string
	names := []string{" Activity ", " Contributors "}
	if m.multi() {
		names = append(names, " Repositories ")
	}
	for i, n := range names {
		if tab(i) == m.active {
			tabs += tabActiveStyle.Render(n)
		} else {
			tabs += tabInactiveStyle.Render(n)
		}
	}
	if m.multi() {
		label := m.selectionName()
		if m.selected == 0 {
			label = fmt.Sprintf("All repositories (%d)", len(m.repos))
		}
		tabs += "   " + subtitleStyle.Render("repo: ") + nameStyle.Render(label)
	}

	var help string
	switch m.active {
	case tabContributors:
		if m.searching {
			help = subtitleStyle.Render("search: ") +
				nameStyle.Render(m.query+"▏") + "   " +
				helpStyle.Render("enter apply · esc clear")
			break
		}
		if m.filtering {
			help = subtitleStyle.Render("last commit on/after (YYYY-MM-DD): ") +
				nameStyle.Render(m.dateInput+"▏") + "   " +
				helpStyle.Render("enter apply · esc clear")
			break
		}
		metricName := "commits"
		if m.metric == metricLines {
			metricName = "lines"
		}
		sortName := "impact"
		switch m.sortBy {
		case gitstats.SortCommits:
			sortName = "commits"
		case gitstats.SortChanges:
			sortName = "changes"
		case gitstats.SortRecent:
			sortName = "recent"
		case gitstats.SortRetention:
			sortName = "retention"
		case gitstats.SortAlive:
			sortName = "alive"
		case gitstats.SortActive:
			sortName = "active"
		}
		state := subtitleStyle.Render("graph: ") + nameStyle.Render(metricName) +
			subtitleStyle.Render("  sort: ") + nameStyle.Render(sortName)
		if m.query != "" {
			state += subtitleStyle.Render("  name: ") + nameStyle.Render(m.query)
		}
		if !m.sinceFilter.IsZero() {
			state += subtitleStyle.Render("  since: ") +
				nameStyle.Render(m.sinceFilter.Format("2006-01-02"))
		}
		help = state + "   " +
			helpStyle.Render("m graph · s/S sort · / name · f date · tab switch · q quit")
	case tabRepositories:
		help = helpStyle.Render("↑/↓ move · enter select · [ / ] cycle repo · tab switch · q quit")
	default:
		hint := "↑/↓ scroll · ←/→ weeks · home/end cal ends · tab switch · q quit"
		if m.multi() {
			hint = "↑/↓ scroll · ←/→ weeks · [ / ] repo · tab switch · q quit"
		}
		help = helpStyle.Render(hint)
	}

	return strings.Join([]string{title, stats, "", tabs, help}, "\n")
}
