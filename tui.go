package main

import (
	"database/sql"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type tuiView int

const (
	viewEntries tuiView = iota
	viewGoals
)

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	dateStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3"))
	catStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	hoursStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	bulletStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	summaryStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	selStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))
	moreStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
	msgStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Italic(true)
	checkStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	openStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	tabActive    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6")).Underline(true)
	tabInactive  = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

type tuiModel struct {
	db      *sql.DB
	date    string
	dates   []string
	entries []Entry
	goals   []Goal
	view    tuiView
	cursor  int
	offset  int
	gCursor int // goal cursor
	height  int
	width   int
	msg     string
	linking bool  // selecting entries to link to a goal
	linkIDs []int64
}

func newTUIModel(db *sql.DB, date string) tuiModel {
	m := tuiModel{db: db, date: date}
	m.loadDates()
	m.loadEntries()
	m.loadGoals()
	// If requested date has no entries and we have other dates, jump to most recent
	if len(m.entries) == 0 && len(m.dates) > 0 {
		m.date = m.dates[0]
		m.loadEntries()
		m.loadGoals()
	}
	return m
}

func (m *tuiModel) loadDates() {
	rows, err := m.db.Query(`SELECT DISTINCT date FROM entries ORDER BY date DESC`)
	if err != nil {
		return
	}
	defer rows.Close()
	m.dates = nil
	for rows.Next() {
		var d string
		rows.Scan(&d)
		m.dates = append(m.dates, d)
	}
}

func (m *tuiModel) loadEntries() {
	m.entries, _ = GetEntries(m.db, m.date)
	m.cursor = 0
	m.offset = 0
}

func (m *tuiModel) loadGoals() {
	m.goals, _ = GetGoals(m.db, m.date)
	m.gCursor = 0
}

func (m *tuiModel) dateIndex() int {
	for i, d := range m.dates {
		if d == m.date {
			return i
		}
	}
	return -1
}

func (m tuiModel) Init() tea.Cmd { return nil }

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
		m.width = msg.Width
	case tea.KeyMsg:
		m.msg = "" // clear any status message

		// Handle linking mode (selecting entries to link to a goal)
		if m.linking {
			return m.updateLinking(msg)
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "tab":
			if m.view == viewEntries {
				m.view = viewGoals
			} else {
				m.view = viewEntries
			}

		// Day navigation (shared)
		case "h", "left":
			m.navDay(-1)
		case "l", "right":
			m.navDay(1)
		case "t":
			m.date = now().Format("2006-01-02")
			m.loadDates()
			m.loadEntries()
			m.loadGoals()
		case "r":
			m.loadDates()
			m.loadEntries()
			m.loadGoals()
		default:
			if m.view == viewGoals {
				return m.updateGoals(msg)
			}
			return m.updateEntries(msg)
		}
	}
	return m, nil
}

func (m tuiModel) updateEntries(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.cursor < len(m.entries)-1 {
			m.cursor++
			m.scrollToCursor()
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
			m.scrollToCursor()
		}
	case "g":
		m.cursor = 0
		m.offset = 0
	case "G":
		m.cursor = max(0, len(m.entries)-1)
		m.scrollToCursor()
	case "d":
		if len(m.entries) > 0 && m.cursor < len(m.entries) {
			e := m.entries[m.cursor]
			if _, err := DeleteEntryByID(m.db, e.ID); err == nil {
				m.loadDates()
				m.loadEntries()
				if m.cursor >= len(m.entries) && m.cursor > 0 {
					m.cursor = len(m.entries) - 1
				}
				m.msg = fmt.Sprintf("deleted entry %d", e.ID)
			}
		}
	case "e":
		if len(m.entries) > 0 && m.cursor < len(m.entries) {
			e := m.entries[m.cursor]
			m.msg = fmt.Sprintf("use: hrs edit %d", e.ID)
		}
	}
	return m, nil
}

func (m tuiModel) updateGoals(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.gCursor < len(m.goals)-1 {
			m.gCursor++
		}
	case "k", "up":
		if m.gCursor > 0 {
			m.gCursor--
		}
	case "x":
		// Toggle goal completion
		if len(m.goals) > 0 && m.gCursor < len(m.goals) {
			g := m.goals[m.gCursor]
			if g.Completed {
				UncompleteGoal(m.db, g.ID)
				m.msg = fmt.Sprintf("reopened goal %d", g.ID)
			} else {
				// If there are entries, enter linking mode
				if len(m.entries) > 0 {
					m.linking = true
					m.linkIDs = nil
					m.cursor = 0
					m.offset = 0
					m.msg = "select entries to link (space to toggle, enter to confirm, esc to skip)"
					return m, nil
				}
				CompleteGoal(m.db, g.ID, nil)
				m.msg = fmt.Sprintf("completed goal %d", g.ID)
			}
			m.loadGoals()
		}
	case "d":
		if len(m.goals) > 0 && m.gCursor < len(m.goals) {
			g := m.goals[m.gCursor]
			DeleteGoal(m.db, g.ID)
			m.loadGoals()
			if m.gCursor >= len(m.goals) && m.gCursor > 0 {
				m.gCursor = len(m.goals) - 1
			}
			m.msg = fmt.Sprintf("deleted goal %d", g.ID)
		}
	case "g":
		m.gCursor = 0
	case "G":
		m.gCursor = max(0, len(m.goals)-1)
	}
	return m, nil
}

func (m tuiModel) updateLinking(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.cursor < len(m.entries)-1 {
			m.cursor++
			m.scrollToCursor()
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
			m.scrollToCursor()
		}
	case " ":
		// Toggle entry selection
		if m.cursor < len(m.entries) {
			eid := m.entries[m.cursor].ID
			found := false
			for i, id := range m.linkIDs {
				if id == eid {
					m.linkIDs = append(m.linkIDs[:i], m.linkIDs[i+1:]...)
					found = true
					break
				}
			}
			if !found {
				m.linkIDs = append(m.linkIDs, eid)
			}
		}
	case "enter":
		// Confirm linking
		if m.gCursor < len(m.goals) {
			g := m.goals[m.gCursor]
			CompleteGoal(m.db, g.ID, m.linkIDs)
			m.loadGoals()
			n := len(m.linkIDs)
			m.msg = fmt.Sprintf("completed goal %d (linked %d entries)", g.ID, n)
		}
		m.linking = false
		m.linkIDs = nil
	case "esc":
		// Skip linking, just complete
		if m.gCursor < len(m.goals) {
			g := m.goals[m.gCursor]
			CompleteGoal(m.db, g.ID, nil)
			m.loadGoals()
			m.msg = fmt.Sprintf("completed goal %d", g.ID)
		}
		m.linking = false
		m.linkIDs = nil
	case "q", "ctrl+c":
		// Cancel entirely
		m.linking = false
		m.linkIDs = nil
		m.msg = "cancelled"
	}
	return m, nil
}

func (m *tuiModel) navDay(dir int) {
	i := m.dateIndex()
	if dir < 0 {
		// older (h/left)
		if i < 0 && len(m.dates) > 0 {
			m.date = m.dates[0]
		} else if i >= 0 && i < len(m.dates)-1 {
			m.date = m.dates[i+1]
		} else {
			return
		}
	} else {
		// newer (l/right)
		if i < 0 && len(m.dates) > 0 {
			m.date = m.dates[0]
		} else if i > 0 {
			m.date = m.dates[i-1]
		} else {
			return
		}
	}
	m.loadEntries()
	m.loadGoals()
}

func (m tuiModel) entryHeight(i int) int {
	// title line + bullet lines + blank line
	return 1 + len(m.entries[i].Bullets) + 1
}

func (m tuiModel) viewHeight() int {
	h := m.height - 4 // header + footer + summary
	if h < 1 {
		h = 20
	}
	return h
}

func (m *tuiModel) scrollToCursor() {
	vh := m.viewHeight()

	// Ensure offset doesn't go past cursor
	if m.cursor < m.offset {
		m.offset = m.cursor
	}

	// Calculate lines from offset to end of cursor entry
	for {
		lines := 0
		for i := m.offset; i <= m.cursor && i < len(m.entries); i++ {
			lines += m.entryHeight(i)
		}
		if lines <= vh || m.offset >= m.cursor {
			break
		}
		m.offset++
	}
}

func (m tuiModel) View() string {
	var b strings.Builder

	// Header with tabs
	today := now().Format("2006-01-02")
	header := dateStyle.Render(m.date)
	if m.date == today {
		header += " (today)"
	}

	entriesTab := tabInactive.Render("entries")
	goalsTab := tabInactive.Render("goals")
	if m.view == viewEntries {
		entriesTab = tabActive.Render("entries")
	} else {
		goalsTab = tabActive.Render("goals")
	}

	// Goals summary badge
	goalsBadge := ""
	if len(m.goals) > 0 {
		done := 0
		for _, g := range m.goals {
			if g.Completed {
				done++
			}
		}
		goalsBadge = hoursStyle.Render(fmt.Sprintf("  [%d/%d goals]", done, len(m.goals)))
	}

	fmt.Fprintf(&b, " %s  %s  %s | %s%s\n\n",
		titleStyle.Render("hrs"), header, entriesTab, goalsTab, goalsBadge)

	if m.linking {
		m.renderLinking(&b)
	} else if m.view == viewGoals {
		m.renderGoals(&b)
	} else {
		m.renderEntries(&b)
	}

	// Status message
	if m.msg != "" {
		fmt.Fprintf(&b, "  %s\n", msgStyle.Render(m.msg))
	}

	// Footer
	if m.linking {
		b.WriteString(helpStyle.Render("  j/k scroll  space toggle  enter confirm  esc skip  q cancel"))
	} else if m.view == viewGoals {
		b.WriteString(helpStyle.Render("  j/k scroll  x toggle done  d delete  h/l day  tab entries  t today  r refresh  q quit"))
	} else {
		b.WriteString(helpStyle.Render("  j/k scroll  h/l day  d delete  e edit  tab goals  g/G top/btm  t today  r refresh  q quit"))
	}

	return b.String()
}

func (m tuiModel) renderEntries(b *strings.Builder) {
	if len(m.entries) == 0 {
		b.WriteString(hoursStyle.Render("  No entries.\n"))
		return
	}

	vh := m.viewHeight()
	end := m.offset
	visibleLines := 0
	for end < len(m.entries) {
		h := m.entryHeight(end)
		if visibleLines+h > vh {
			break
		}
		visibleLines += h
		end++
	}
	if end == m.offset && end < len(m.entries) {
		end++
	}
	var totalHours float64
	for _, e := range m.entries {
		totalHours += e.HoursEst
	}

	if m.offset > 0 {
		fmt.Fprintf(b, "%s\n", moreStyle.Render(fmt.Sprintf("  ▲ %d more above", m.offset)))
	}

	for i := m.offset; i < end; i++ {
		e := m.entries[i]
		prefix := "  "
		style := catStyle
		if i == m.cursor {
			prefix = selStyle.Render("> ")
			style = selStyle
		}

		hours := ""
		if e.HoursEst > 0 {
			hours = hoursStyle.Render(fmt.Sprintf(" (~%gh)", e.HoursEst))
		}

		fmt.Fprintf(b, "%s%s %s%s\n",
			prefix,
			style.Render(fmt.Sprintf("[%s]", e.Category)),
			titleStyle.Render(e.Title),
			hours,
		)
		for _, bullet := range e.Bullets {
			line := bullet
			if m.width > 0 && len(line) > m.width-8 {
				line = line[:m.width-11] + "..."
			}
			fmt.Fprintf(b, "    %s\n", bulletStyle.Render("- "+line))
		}
		b.WriteByte('\n')
	}

	if end < len(m.entries) {
		fmt.Fprintf(b, "%s\n", moreStyle.Render(fmt.Sprintf("  ▼ %d more below", len(m.entries)-end)))
	}

	fmt.Fprintf(b, "%s\n",
		summaryStyle.Render(fmt.Sprintf("  %d entries  ~%gh  %.1fd", len(m.entries), totalHours, totalHours/8)),
	)
}

func (m tuiModel) renderGoals(b *strings.Builder) {
	if len(m.goals) == 0 {
		b.WriteString(hoursStyle.Render("  No goals set. Use: hrs goals add \"your goal\"\n"))
		return
	}

	for i, g := range m.goals {
		prefix := "  "
		if i == m.gCursor {
			prefix = selStyle.Render("> ")
		}

		if g.Completed {
			text := checkStyle.Render("[x] ") + hoursStyle.Render(g.Text)
			if len(g.EntryIDs) > 0 {
				ids := make([]string, len(g.EntryIDs))
				for j, eid := range g.EntryIDs {
					ids[j] = fmt.Sprintf("%d", eid)
				}
				text += hoursStyle.Render(fmt.Sprintf(" (entries: %s)", strings.Join(ids, ",")))
			}
			fmt.Fprintf(b, "%s%s\n", prefix, text)
		} else {
			text := openStyle.Render("[ ] ") + g.Text
			fmt.Fprintf(b, "%s%s %s\n", prefix, text, hoursStyle.Render(fmt.Sprintf("#%d", g.ID)))
		}
	}
	b.WriteByte('\n')
}

func (m tuiModel) renderLinking(b *strings.Builder) {
	g := m.goals[m.gCursor]
	fmt.Fprintf(b, "  %s %s\n\n",
		titleStyle.Render("Link entries to goal:"),
		openStyle.Render(g.Text))

	vh := m.viewHeight() - 2 // account for goal header
	end := m.offset
	visibleLines := 0
	for end < len(m.entries) {
		h := m.entryHeight(end)
		if visibleLines+h > vh {
			break
		}
		visibleLines += h
		end++
	}
	if end == m.offset && end < len(m.entries) {
		end++
	}

	for i := m.offset; i < end; i++ {
		e := m.entries[i]
		selected := false
		for _, id := range m.linkIDs {
			if id == e.ID {
				selected = true
				break
			}
		}

		prefix := "  "
		if i == m.cursor {
			prefix = selStyle.Render("> ")
		}

		check := openStyle.Render("[ ] ")
		if selected {
			check = checkStyle.Render("[x] ")
		}

		hours := ""
		if e.HoursEst > 0 {
			hours = hoursStyle.Render(fmt.Sprintf(" (~%gh)", e.HoursEst))
		}

		fmt.Fprintf(b, "%s%s%s %s%s\n",
			prefix, check,
			catStyle.Render(fmt.Sprintf("[%s]", e.Category)),
			titleStyle.Render(e.Title),
			hours,
		)
		for _, bullet := range e.Bullets {
			line := bullet
			if m.width > 0 && len(line) > m.width-12 {
				line = line[:m.width-15] + "..."
			}
			fmt.Fprintf(b, "        %s\n", bulletStyle.Render("- "+line))
		}
		b.WriteByte('\n')
	}
}
