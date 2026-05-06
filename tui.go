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
	viewGoalsList
	viewGoalsMatrix
	viewStratList
	viewStratMatrix
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
	impStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1"))
	urgStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	quadStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	quadSelStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2")).Background(lipgloss.Color("0"))
	ctxStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
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
	linking bool // selecting entries to link to a goal
	linkIDs []int64

	// Enriched entries (entries with goal/strategy context)
	enriched []EnrichedEntry

	// Strategy list
	strategies []Strategy
	sCursor    int

	// Matrix state
	activeGoals  []Goal
	activeStrats []Strategy
	mQuadrant    int // 0=Q1, 1=Q2, 2=Q3, 3=Q4
	mCursor      int // cursor within current quadrant
	smQuadrant   int // strategy matrix quadrant
	smCursor     int // strategy matrix cursor

	// Remember sub-view preference when switching top-level tabs
	goalsSubMatrix bool // true = matrix, false = list
	stratSubMatrix bool
}

func newTUIModel(db *sql.DB, date string) tuiModel {
	m := tuiModel{db: db, date: date}
	m.loadDates()
	m.loadEntries()
	m.loadGoals()
	m.loadStrategies()
	m.loadActiveGoals()
	m.loadActiveStrats()
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
	m.enriched, _ = GetEnrichedEntries(m.db, m.date)
	m.cursor = 0
	m.offset = 0
}

func (m *tuiModel) loadGoals() {
	m.goals, _ = GetGoals(m.db, m.date)
	if m.gCursor >= len(m.goals) {
		m.gCursor = max(0, len(m.goals)-1)
	}
}

func (m *tuiModel) loadStrategies() {
	m.strategies, _ = GetStrategies(m.db, "active")
	if m.sCursor >= len(m.strategies) {
		m.sCursor = max(0, len(m.strategies)-1)
	}
}

func (m *tuiModel) loadActiveGoals() {
	m.activeGoals, _ = GetActiveGoals(m.db)
	// Clamp cursor — quadrant contents may have changed
	quads := partitionGoals(m.activeGoals)
	if m.mCursor >= len(quads[m.mQuadrant]) {
		m.mCursor = max(0, len(quads[m.mQuadrant])-1)
	}
}

func (m *tuiModel) loadActiveStrats() {
	m.activeStrats, _ = GetStrategies(m.db, "active")
	quads := partitionStrategies(m.activeStrats)
	if m.smCursor >= len(quads[m.smQuadrant]) {
		m.smCursor = max(0, len(quads[m.smQuadrant])-1)
	}
}

func (m *tuiModel) dateIndex() int {
	for i, d := range m.dates {
		if d == m.date {
			return i
		}
	}
	return -1
}

// topView returns which top-level section we're in: entries, goals, or strategies
func (m tuiModel) topView() string {
	switch m.view {
	case viewGoalsList, viewGoalsMatrix:
		return "goals"
	case viewStratList, viewStratMatrix:
		return "strategies"
	default:
		return "entries"
	}
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
			// Cycle top-level: entries -> goals -> strategies -> entries
			switch m.topView() {
			case "entries":
				if m.goalsSubMatrix {
					m.view = viewGoalsMatrix
				} else {
					m.view = viewGoalsList
				}
			case "goals":
				if m.stratSubMatrix {
					m.view = viewStratMatrix
				} else {
					m.view = viewStratList
				}
				m.loadStrategies()
				m.loadActiveStrats()
			case "strategies":
				m.view = viewEntries
			}
		case "m":
			// Toggle sub-view: list <-> matrix (only in goals/strategies)
			switch m.view {
			case viewGoalsList:
				m.view = viewGoalsMatrix
				m.goalsSubMatrix = true
				m.loadActiveGoals()
			case viewGoalsMatrix:
				m.view = viewGoalsList
				m.goalsSubMatrix = false
			case viewStratList:
				m.view = viewStratMatrix
				m.stratSubMatrix = true
				m.loadActiveStrats()
			case viewStratMatrix:
				m.view = viewStratList
				m.stratSubMatrix = false
			}
		case "t":
			m.date = now().Format("2006-01-02")
			m.loadDates()
			m.loadEntries()
			m.loadGoals()
		case "r":
			m.loadDates()
			m.loadEntries()
			m.loadGoals()
			m.loadStrategies()
			m.loadActiveGoals()
			m.loadActiveStrats()
		default:
			switch m.view {
			case viewGoalsList:
				return m.updateGoals(msg)
			case viewGoalsMatrix:
				return m.updateGoalsMatrix(msg)
			case viewStratList:
				return m.updateStrats(msg)
			case viewStratMatrix:
				return m.updateStratsMatrix(msg)
			default:
				return m.updateEntries(msg)
			}
		}
	}
	return m, nil
}

func (m tuiModel) updateEntries(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "h", "left":
		m.navDay(-1)
	case "l", "right":
		m.navDay(1)
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
	case "h", "left":
		m.navDay(-1)
	case "l", "right":
		m.navDay(1)
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
	case "i":
		if len(m.goals) > 0 && m.gCursor < len(m.goals) {
			g := m.goals[m.gCursor]
			UpdateGoal(m.db, g.ID, g.Text, g.StrategyID, !g.Important, g.Urgent)
			m.loadGoals()
			if g.Important {
				m.msg = fmt.Sprintf("goal %d: removed important", g.ID)
			} else {
				m.msg = fmt.Sprintf("goal %d: marked important", g.ID)
			}
		}
	case "u":
		if len(m.goals) > 0 && m.gCursor < len(m.goals) {
			g := m.goals[m.gCursor]
			UpdateGoal(m.db, g.ID, g.Text, g.StrategyID, g.Important, !g.Urgent)
			m.loadGoals()
			if g.Urgent {
				m.msg = fmt.Sprintf("goal %d: removed urgent", g.ID)
			} else {
				m.msg = fmt.Sprintf("goal %d: marked urgent", g.ID)
			}
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

func (m tuiModel) updateStrats(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "j", "down":
		if m.sCursor < len(m.strategies)-1 {
			m.sCursor++
		}
	case "k", "up":
		if m.sCursor > 0 {
			m.sCursor--
		}
	case "i":
		if len(m.strategies) > 0 && m.sCursor < len(m.strategies) {
			s := m.strategies[m.sCursor]
			UpdateStrategy(m.db, s.ID, s.Title, s.Description, !s.Important, s.Urgent)
			m.loadStrategies()
			if s.Important {
				m.msg = fmt.Sprintf("strategy %d: removed important", s.ID)
			} else {
				m.msg = fmt.Sprintf("strategy %d: marked important", s.ID)
			}
		}
	case "u":
		if len(m.strategies) > 0 && m.sCursor < len(m.strategies) {
			s := m.strategies[m.sCursor]
			UpdateStrategy(m.db, s.ID, s.Title, s.Description, s.Important, !s.Urgent)
			m.loadStrategies()
			if s.Urgent {
				m.msg = fmt.Sprintf("strategy %d: removed urgent", s.ID)
			} else {
				m.msg = fmt.Sprintf("strategy %d: marked urgent", s.ID)
			}
		}
	case "d":
		if len(m.strategies) > 0 && m.sCursor < len(m.strategies) {
			s := m.strategies[m.sCursor]
			DeleteStrategy(m.db, s.ID)
			m.loadStrategies()
			if m.sCursor >= len(m.strategies) && m.sCursor > 0 {
				m.sCursor = len(m.strategies) - 1
			}
			m.msg = fmt.Sprintf("deleted strategy %d", s.ID)
		}
	case "g":
		m.sCursor = 0
	case "G":
		m.sCursor = max(0, len(m.strategies)-1)
	}
	return m, nil
}

// Matrix helpers

func partitionGoals(goals []Goal) [4][]Goal {
	var q [4][]Goal
	for _, g := range goals {
		idx := 3 // Q4: not important, not urgent
		if g.Important && g.Urgent {
			idx = 0 // Q1
		} else if g.Important {
			idx = 1 // Q2
		} else if g.Urgent {
			idx = 2 // Q3
		}
		q[idx] = append(q[idx], g)
	}
	return q
}

func partitionStrategies(strats []Strategy) [4][]Strategy {
	var q [4][]Strategy
	for _, s := range strats {
		idx := 3
		if s.Important && s.Urgent {
			idx = 0
		} else if s.Important {
			idx = 1
		} else if s.Urgent {
			idx = 2
		}
		q[idx] = append(q[idx], s)
	}
	return q
}

func (m tuiModel) updateGoalsMatrix(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	quads := partitionGoals(m.activeGoals)
	switch msg.String() {
	case "h", "left":
		if m.mQuadrant == 1 {
			m.mQuadrant = 0
		} else if m.mQuadrant == 3 {
			m.mQuadrant = 2
		}
		m.mCursor = 0
	case "l", "right":
		if m.mQuadrant == 0 {
			m.mQuadrant = 1
		} else if m.mQuadrant == 2 {
			m.mQuadrant = 3
		}
		m.mCursor = 0
	case "j", "down":
		if m.mQuadrant < 2 {
			// Check if we should move down to Q3/Q4
			if m.mCursor < len(quads[m.mQuadrant])-1 {
				m.mCursor++
			} else {
				m.mQuadrant += 2
				m.mCursor = 0
			}
		} else {
			if m.mCursor < len(quads[m.mQuadrant])-1 {
				m.mCursor++
			}
		}
	case "k", "up":
		if m.mCursor > 0 {
			m.mCursor--
		} else if m.mQuadrant >= 2 {
			m.mQuadrant -= 2
			if len(quads[m.mQuadrant]) > 0 {
				m.mCursor = len(quads[m.mQuadrant]) - 1
			}
		}
	case "x":
		if g := m.matrixSelectedGoal(quads); g != nil {
			if g.Completed {
				UncompleteGoal(m.db, g.ID)
			} else {
				CompleteGoal(m.db, g.ID, nil)
			}
			m.loadActiveGoals()
		}
	case "i":
		if g := m.matrixSelectedGoal(quads); g != nil {
			UpdateGoal(m.db, g.ID, g.Text, g.StrategyID, !g.Important, g.Urgent)
			m.loadActiveGoals()
			m.mCursor = 0
		}
	case "u":
		if g := m.matrixSelectedGoal(quads); g != nil {
			UpdateGoal(m.db, g.ID, g.Text, g.StrategyID, g.Important, !g.Urgent)
			m.loadActiveGoals()
			m.mCursor = 0
		}
	case "d":
		if g := m.matrixSelectedGoal(quads); g != nil {
			DeleteGoal(m.db, g.ID)
			m.loadActiveGoals()
			m.mCursor = 0
		}
	}
	// Clamp cursor
	quads = partitionGoals(m.activeGoals)
	if m.mCursor >= len(quads[m.mQuadrant]) {
		m.mCursor = max(0, len(quads[m.mQuadrant])-1)
	}
	return m, nil
}

func (m tuiModel) matrixSelectedGoal(quads [4][]Goal) *Goal {
	q := quads[m.mQuadrant]
	if m.mCursor < len(q) {
		return &q[m.mCursor]
	}
	return nil
}

func (m tuiModel) updateStratsMatrix(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	quads := partitionStrategies(m.activeStrats)
	switch msg.String() {
	case "h", "left":
		if m.smQuadrant == 1 {
			m.smQuadrant = 0
		} else if m.smQuadrant == 3 {
			m.smQuadrant = 2
		}
		m.smCursor = 0
	case "l", "right":
		if m.smQuadrant == 0 {
			m.smQuadrant = 1
		} else if m.smQuadrant == 2 {
			m.smQuadrant = 3
		}
		m.smCursor = 0
	case "j", "down":
		if m.smQuadrant < 2 {
			if m.smCursor < len(quads[m.smQuadrant])-1 {
				m.smCursor++
			} else {
				m.smQuadrant += 2
				m.smCursor = 0
			}
		} else {
			if m.smCursor < len(quads[m.smQuadrant])-1 {
				m.smCursor++
			}
		}
	case "k", "up":
		if m.smCursor > 0 {
			m.smCursor--
		} else if m.smQuadrant >= 2 {
			m.smQuadrant -= 2
			if len(quads[m.smQuadrant]) > 0 {
				m.smCursor = len(quads[m.smQuadrant]) - 1
			}
		}
	case "i":
		q := quads[m.smQuadrant]
		if m.smCursor < len(q) {
			s := q[m.smCursor]
			UpdateStrategy(m.db, s.ID, s.Title, s.Description, !s.Important, s.Urgent)
			m.loadActiveStrats()
			m.smCursor = 0
		}
	case "u":
		q := quads[m.smQuadrant]
		if m.smCursor < len(q) {
			s := q[m.smCursor]
			UpdateStrategy(m.db, s.ID, s.Title, s.Description, s.Important, !s.Urgent)
			m.loadActiveStrats()
			m.smCursor = 0
		}
	case "d":
		q := quads[m.smQuadrant]
		if m.smCursor < len(q) {
			s := q[m.smCursor]
			DeleteStrategy(m.db, s.ID)
			m.loadActiveStrats()
			m.smCursor = 0
		}
	}
	quads = partitionStrategies(m.activeStrats)
	if m.smCursor >= len(quads[m.smQuadrant]) {
		m.smCursor = max(0, len(quads[m.smQuadrant])-1)
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
	// title line + bullet lines + context line (if enriched) + blank line
	if i < len(m.enriched) {
		h := 1 + len(m.enriched[i].Bullets) + 1
		if m.enriched[i].GoalText != nil {
			h++
		}
		return h
	}
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

	// Build tab bar: entries | goals [list|matrix] | strategies [list|matrix]
	entriesTab := tabInactive.Render("entries")
	goalsLabel := "goals"
	stratLabel := "strategies"

	top := m.topView()
	if top == "entries" {
		entriesTab = tabActive.Render("entries")
	}

	// Goals sub-tabs
	goalsListSub := tabInactive.Render("list")
	goalsMatrixSub := tabInactive.Render("matrix")
	if top == "goals" {
		goalsLabel = tabActive.Render("goals") + " "
		if m.view == viewGoalsList {
			goalsListSub = tabActive.Render("list")
		} else {
			goalsMatrixSub = tabActive.Render("matrix")
		}
		goalsLabel += goalsListSub + "|" + goalsMatrixSub
	} else {
		goalsLabel = tabInactive.Render("goals")
	}

	// Strategy sub-tabs
	stratListSub := tabInactive.Render("list")
	stratMatrixSub := tabInactive.Render("matrix")
	if top == "strategies" {
		stratLabel = tabActive.Render("strats") + " "
		if m.view == viewStratList {
			stratListSub = tabActive.Render("list")
		} else {
			stratMatrixSub = tabActive.Render("matrix")
		}
		stratLabel += stratListSub + "|" + stratMatrixSub
	} else {
		stratLabel = tabInactive.Render("strats")
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

	fmt.Fprintf(&b, " %s  %s  %s | %s | %s%s\n\n",
		titleStyle.Render("hrs"), header, entriesTab, goalsLabel, stratLabel, goalsBadge)

	// Content
	if m.linking {
		m.renderLinking(&b)
	} else {
		switch m.view {
		case viewGoalsList:
			m.renderGoals(&b)
		case viewGoalsMatrix:
			m.renderGoalsMatrix(&b)
		case viewStratList:
			m.renderStratList(&b)
		case viewStratMatrix:
			m.renderStratMatrix(&b)
		default:
			m.renderEntries(&b)
		}
	}

	// Status message
	if m.msg != "" {
		fmt.Fprintf(&b, "  %s\n", msgStyle.Render(m.msg))
	}

	// Footer
	if m.linking {
		b.WriteString(helpStyle.Render("  j/k scroll  space toggle  enter confirm  esc skip  q cancel"))
	} else {
		switch m.view {
		case viewGoalsList:
			b.WriteString(helpStyle.Render("  j/k scroll  x done  i/u priority  d delete  h/l day  m matrix  tab strats  t today  r refresh  q quit"))
		case viewGoalsMatrix:
			b.WriteString(helpStyle.Render("  j/k scroll  h/l quadrant  x done  i/u priority  d delete  m list  tab strats  r refresh  q quit"))
		case viewStratList:
			b.WriteString(helpStyle.Render("  j/k scroll  i/u priority  d delete  m matrix  tab entries  r refresh  q quit"))
		case viewStratMatrix:
			b.WriteString(helpStyle.Render("  j/k scroll  h/l quadrant  i/u priority  d delete  m list  tab entries  r refresh  q quit"))
		default:
			b.WriteString(helpStyle.Render("  j/k scroll  h/l day  d delete  e edit  tab goals  g/G top/btm  t today  r refresh  q quit"))
		}
	}

	return b.String()
}

func (m tuiModel) renderEntries(b *strings.Builder) {
	if len(m.enriched) == 0 {
		b.WriteString(hoursStyle.Render("  No entries.\n"))
		return
	}

	vh := m.viewHeight()
	end := m.offset
	visibleLines := 0
	for end < len(m.enriched) {
		h := m.entryHeight(end)
		if visibleLines+h > vh {
			break
		}
		visibleLines += h
		end++
	}
	if end == m.offset && end < len(m.enriched) {
		end++
	}
	var totalHours float64
	for _, e := range m.enriched {
		totalHours += e.HoursEst
	}

	if m.offset > 0 {
		fmt.Fprintf(b, "%s\n", moreStyle.Render(fmt.Sprintf("  ▲ %d more above", m.offset)))
	}

	for i := m.offset; i < end; i++ {
		e := m.enriched[i]
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
		// Show linked goal/strategy context
		if e.GoalText != nil {
			ctx := fmt.Sprintf("-> %q", *e.GoalText)
			if e.StrategyTitle != nil {
				ctx += fmt.Sprintf(" | s: %s", *e.StrategyTitle)
			}
			fmt.Fprintf(b, "    %s\n", ctxStyle.Render(ctx))
		}
		b.WriteByte('\n')
	}

	if end < len(m.enriched) {
		fmt.Fprintf(b, "%s\n", moreStyle.Render(fmt.Sprintf("  ▼ %d more below", len(m.enriched)-end)))
	}

	fmt.Fprintf(b, "%s\n",
		summaryStyle.Render(fmt.Sprintf("  %d entries  ~%gh  %.1fd", len(m.enriched), totalHours, totalHours/8)),
	)
}

func prioIndicator(important, urgent bool) string {
	s := ""
	if important {
		s += impStyle.Render("!")
	}
	if urgent {
		s += urgStyle.Render("^")
	}
	if s != "" {
		s += " "
	}
	return s
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

		prio := prioIndicator(g.Important, g.Urgent)
		stratTag := ""
		if g.StrategyID != nil {
			stratTag = hoursStyle.Render(fmt.Sprintf(" s#%d", *g.StrategyID))
		}

		if g.Completed {
			text := checkStyle.Render("[x] ") + prio + hoursStyle.Render(g.Text)
			if len(g.EntryIDs) > 0 {
				ids := make([]string, len(g.EntryIDs))
				for j, eid := range g.EntryIDs {
					ids[j] = fmt.Sprintf("%d", eid)
				}
				text += hoursStyle.Render(fmt.Sprintf(" (entries: %s)", strings.Join(ids, ",")))
			}
			text += stratTag
			fmt.Fprintf(b, "%s%s\n", prefix, text)
		} else {
			text := openStyle.Render("[ ] ") + prio + g.Text
			fmt.Fprintf(b, "%s%s %s%s\n", prefix, text, hoursStyle.Render(fmt.Sprintf("#%d", g.ID)), stratTag)
		}
	}
	b.WriteByte('\n')
}

func (m tuiModel) renderStratList(b *strings.Builder) {
	if len(m.strategies) == 0 {
		b.WriteString(hoursStyle.Render("  No active strategies.\n"))
		return
	}

	for i, s := range m.strategies {
		prefix := "  "
		if i == m.sCursor {
			prefix = selStyle.Render("> ")
		}

		prio := prioIndicator(s.Important, s.Urgent)
		desc := ""
		if s.Description != "" {
			desc = hoursStyle.Render(" - " + s.Description)
		}
		fmt.Fprintf(b, "%s%s %s%s%s\n", prefix,
			hoursStyle.Render(fmt.Sprintf("#%d", s.ID)),
			prio,
			titleStyle.Render(s.Title),
			desc,
		)
	}
	b.WriteByte('\n')
}

var quadLabels = [4]string{"Q1: Do First", "Q2: Schedule", "Q3: Delegate", "Q4: Eliminate"}

func (m tuiModel) renderGoalsMatrix(b *strings.Builder) {
	quads := partitionGoals(m.activeGoals)
	halfW := m.width/2 - 2
	if halfW < 20 {
		halfW = 30
	}

	for row := 0; row < 2; row++ {
		leftQ := row * 2
		rightQ := row*2 + 1

		// Quadrant headers
		leftLabel := quadLabels[leftQ]
		rightLabel := quadLabels[rightQ]
		if m.mQuadrant == leftQ {
			fmt.Fprintf(b, "  %s", quadSelStyle.Render(fmt.Sprintf(" %s (%d) ", leftLabel, len(quads[leftQ]))))
		} else {
			fmt.Fprintf(b, "  %s", quadStyle.Render(fmt.Sprintf(" %s (%d) ", leftLabel, len(quads[leftQ]))))
		}
		pad := halfW - len(leftLabel) - 8
		if pad > 0 {
			b.WriteString(strings.Repeat(" ", pad))
		}
		if m.mQuadrant == rightQ {
			fmt.Fprintf(b, "%s", quadSelStyle.Render(fmt.Sprintf(" %s (%d) ", rightLabel, len(quads[rightQ]))))
		} else {
			fmt.Fprintf(b, "%s", quadStyle.Render(fmt.Sprintf(" %s (%d) ", rightLabel, len(quads[rightQ]))))
		}
		b.WriteByte('\n')

		// Render items side by side
		maxItems := max(len(quads[leftQ]), len(quads[rightQ]))
		if maxItems == 0 {
			fmt.Fprintf(b, "  %s", hoursStyle.Render("(empty)"))
			pad := halfW - 5
			if pad > 0 {
				b.WriteString(strings.Repeat(" ", pad))
			}
			fmt.Fprintf(b, "%s\n", hoursStyle.Render("(empty)"))
		}
		for i := 0; i < maxItems; i++ {
			leftStr := m.renderMatrixGoalItem(quads[leftQ], i, leftQ, halfW)
			rightStr := m.renderMatrixGoalItem(quads[rightQ], i, rightQ, halfW)
			fmt.Fprintf(b, "%s%s\n", leftStr, rightStr)
		}
		if row == 0 {
			sep := strings.Repeat("─", halfW*2+2)
			b.WriteString(hoursStyle.Render("  " + sep))
			b.WriteByte('\n')
		}
	}
}

func (m tuiModel) renderMatrixGoalItem(goals []Goal, idx, quadrant, colW int) string {
	if idx >= len(goals) {
		return strings.Repeat(" ", colW+2)
	}
	g := goals[idx]
	isSelected := m.mQuadrant == quadrant && m.mCursor == idx

	prefix := "  "
	if isSelected {
		prefix = selStyle.Render("> ")
	}

	check := openStyle.Render("[ ]")
	if g.Completed {
		check = checkStyle.Render("[x]")
	}

	text := g.Text
	maxText := colW - 8
	if maxText < 10 {
		maxText = 10
	}
	if len(text) > maxText {
		text = text[:maxText-3] + "..."
	}

	line := fmt.Sprintf("%s%s %s", prefix, check, text)
	// Pad to column width
	visible := 4 + len(g.Text)
	if len(g.Text) > maxText {
		visible = 4 + maxText
	}
	if visible < colW {
		line += strings.Repeat(" ", colW-visible)
	}
	return line
}

func (m tuiModel) renderStratMatrix(b *strings.Builder) {
	quads := partitionStrategies(m.activeStrats)
	halfW := m.width/2 - 2
	if halfW < 20 {
		halfW = 30
	}

	for row := 0; row < 2; row++ {
		leftQ := row * 2
		rightQ := row*2 + 1

		leftLabel := quadLabels[leftQ]
		rightLabel := quadLabels[rightQ]
		if m.smQuadrant == leftQ {
			fmt.Fprintf(b, "  %s", quadSelStyle.Render(fmt.Sprintf(" %s (%d) ", leftLabel, len(quads[leftQ]))))
		} else {
			fmt.Fprintf(b, "  %s", quadStyle.Render(fmt.Sprintf(" %s (%d) ", leftLabel, len(quads[leftQ]))))
		}
		pad := halfW - len(leftLabel) - 8
		if pad > 0 {
			b.WriteString(strings.Repeat(" ", pad))
		}
		if m.smQuadrant == rightQ {
			fmt.Fprintf(b, "%s", quadSelStyle.Render(fmt.Sprintf(" %s (%d) ", rightLabel, len(quads[rightQ]))))
		} else {
			fmt.Fprintf(b, "%s", quadStyle.Render(fmt.Sprintf(" %s (%d) ", rightLabel, len(quads[rightQ]))))
		}
		b.WriteByte('\n')

		maxItems := max(len(quads[leftQ]), len(quads[rightQ]))
		if maxItems == 0 {
			fmt.Fprintf(b, "  %s", hoursStyle.Render("(empty)"))
			pad := halfW - 5
			if pad > 0 {
				b.WriteString(strings.Repeat(" ", pad))
			}
			fmt.Fprintf(b, "%s\n", hoursStyle.Render("(empty)"))
		}
		for i := 0; i < maxItems; i++ {
			leftStr := m.renderMatrixStratItem(quads[leftQ], i, leftQ, halfW)
			rightStr := m.renderMatrixStratItem(quads[rightQ], i, rightQ, halfW)
			fmt.Fprintf(b, "%s%s\n", leftStr, rightStr)
		}
		if row == 0 {
			sep := strings.Repeat("─", halfW*2+2)
			b.WriteString(hoursStyle.Render("  " + sep))
			b.WriteByte('\n')
		}
	}
}

func (m tuiModel) renderMatrixStratItem(strats []Strategy, idx, quadrant, colW int) string {
	if idx >= len(strats) {
		return strings.Repeat(" ", colW+2)
	}
	s := strats[idx]
	isSelected := m.smQuadrant == quadrant && m.smCursor == idx

	prefix := "  "
	if isSelected {
		prefix = selStyle.Render("> ")
	}

	text := s.Title
	maxText := colW - 6
	if maxText < 10 {
		maxText = 10
	}
	if len(text) > maxText {
		text = text[:maxText-3] + "..."
	}

	line := fmt.Sprintf("%s%s %s", prefix, hoursStyle.Render(fmt.Sprintf("#%d", s.ID)), text)
	visible := 4 + len(fmt.Sprintf("#%d", s.ID)) + 1 + len(text)
	if visible < colW {
		line += strings.Repeat(" ", colW-visible)
	}
	return line
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
