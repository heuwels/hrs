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
)

type tuiModel struct {
	db       *sql.DB
	date     string
	dates    []string
	entries  []Entry
	cursor   int
	offset   int
	height   int
	width    int
}

func newTUIModel(db *sql.DB, date string) tuiModel {
	m := tuiModel{db: db, date: date}
	m.loadDates()
	m.loadEntries()
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
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		// Vim navigation: j/k for entries, h/l for days
		case "j", "down":
			if m.cursor < len(m.entries)-1 {
				m.cursor++
				if m.cursor-m.offset >= m.viewHeight() {
					m.offset++
				}
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
				if m.cursor < m.offset {
					m.offset = m.cursor
				}
			}
		case "h", "left":
			// Older (dates are descending)
			if i := m.dateIndex(); i >= 0 && i < len(m.dates)-1 {
				m.date = m.dates[i+1]
				m.loadEntries()
			}
		case "l", "right":
			// Newer
			if i := m.dateIndex(); i > 0 {
				m.date = m.dates[i-1]
				m.loadEntries()
			}
		case "g":
			m.cursor = 0
			m.offset = 0
		case "G":
			m.cursor = max(0, len(m.entries)-1)
			if m.cursor-m.offset >= m.viewHeight() {
				m.offset = m.cursor - m.viewHeight() + 1
			}
		case "t":
			m.date = now().Format("2006-01-02")
			m.loadDates()
			m.loadEntries()
		case "r":
			m.loadDates()
			m.loadEntries()
		}
	}
	return m, nil
}

func (m tuiModel) viewHeight() int {
	h := m.height - 4 // header + footer
	if h < 1 {
		h = 20
	}
	return h
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
		end := m.offset + vh
		if end > len(m.entries) {
			end = len(m.entries)
		}
		var totalHours float64
		for _, e := range m.entries {
			totalHours += e.HoursEst
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

		// Summary line
		fmt.Fprintf(&b, "%s\n",
			summaryStyle.Render(fmt.Sprintf("  %d entries  ~%gh  %.1fd", len(m.entries), totalHours, totalHours/8)),
		)
	}

	// Footer
	b.WriteString(helpStyle.Render("  j/k scroll  h/l day  g/G top/bottom  t today  r refresh  q quit"))

	return b.String()
}
