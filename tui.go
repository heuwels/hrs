package main

import (
	"database/sql"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
)

type tuiModel struct {
	db      *sql.DB
	date    string
	dates   []string
	entries []Entry
	cursor  int
	offset  int
	height  int
	width   int
	msg     string
}

func newTUIModel(db *sql.DB, date string) tuiModel {
	m := tuiModel{db: db, date: date}
	m.loadDates()
	m.loadEntries()
	// If requested date has no entries and we have other dates, jump to most recent
	if len(m.entries) == 0 && len(m.dates) > 0 {
		m.date = m.dates[0]
		m.loadEntries()
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
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		// Vim navigation: j/k for entries, h/l for days
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
		case "h", "left":
			i := m.dateIndex()
			if i < 0 && len(m.dates) > 0 {
				m.date = m.dates[0]
				m.loadEntries()
			} else if i >= 0 && i < len(m.dates)-1 {
				m.date = m.dates[i+1]
				m.loadEntries()
			}
		case "l", "right":
			i := m.dateIndex()
			if i < 0 && len(m.dates) > 0 {
				m.date = m.dates[0]
				m.loadEntries()
			} else if i > 0 {
				m.date = m.dates[i-1]
				m.loadEntries()
			}
		case "g":
			m.cursor = 0
			m.offset = 0
		case "G":
			m.cursor = max(0, len(m.entries)-1)
			m.scrollToCursor()
		case "t":
			m.date = now().Format("2006-01-02")
			m.loadDates()
			m.loadEntries()
		case "r":
			m.loadDates()
			m.loadEntries()
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
	}
	return m, nil
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

	// Header
	today := now().Format("2006-01-02")
	header := dateStyle.Render(m.date)
	if m.date == today {
		header += " (today)"
	}
	fmt.Fprintf(&b, " %s  %s\n\n", titleStyle.Render("hrs"), header)

	if len(m.entries) == 0 {
		b.WriteString(hoursStyle.Render("  No entries.\n"))
	} else {
		vh := m.viewHeight()
		// Find how many entries fit from offset
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

		// "More above" indicator
		if m.offset > 0 {
			fmt.Fprintf(&b, "%s\n", moreStyle.Render(fmt.Sprintf("  ▲ %d more above", m.offset)))
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

			fmt.Fprintf(&b, "%s%s %s%s\n",
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
				fmt.Fprintf(&b, "    %s\n", bulletStyle.Render("- "+line))
			}
			b.WriteByte('\n')
		}

		// "More below" indicator
		if end < len(m.entries) {
			fmt.Fprintf(&b, "%s\n", moreStyle.Render(fmt.Sprintf("  ▼ %d more below", len(m.entries)-end)))
		}

		// Summary line
		fmt.Fprintf(&b, "%s\n",
			summaryStyle.Render(fmt.Sprintf("  %d entries  ~%gh  %.1fd", len(m.entries), totalHours, totalHours/8)),
		)
	}

	// Status message
	if m.msg != "" {
		fmt.Fprintf(&b, "  %s\n", msgStyle.Render(m.msg))
	}

	// Footer
	b.WriteString(helpStyle.Render("  j/k scroll  h/l day  d delete  e edit  g/G top/bottom  t today  r refresh  q quit"))

	return b.String()
}
