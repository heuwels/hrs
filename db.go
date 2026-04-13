package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

type Entry struct {
	ID       int64    `json:"id,omitempty"`
	Date     string   `json:"date"`
	Time     string   `json:"time"`
	Category string   `json:"category"`
	Title    string   `json:"title"`
	Bullets  []string `json:"bullets"`
	HoursEst float64  `json:"hours_est"`
}

func OpenDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS entries (
		id        INTEGER PRIMARY KEY AUTOINCREMENT,
		date      TEXT NOT NULL,
		time      TEXT NOT NULL,
		category  TEXT NOT NULL,
		title     TEXT NOT NULL,
		bullets   TEXT NOT NULL,
		hours_est REAL NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_entries_date ON entries(date)`)
	return db, err
}

func InsertEntry(db *sql.DB, e *Entry) (int64, error) {
	if e.Date == "" {
		e.Date = now().Format("2006-01-02")
	}
	if e.Time == "" {
		e.Time = now().Format("15:04")
	}
	b, _ := json.Marshal(e.Bullets)
	res, err := db.Exec(
		`INSERT INTO entries (date, time, category, title, bullets, hours_est) VALUES (?,?,?,?,?,?)`,
		e.Date, e.Time, e.Category, e.Title, string(b), e.HoursEst,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func GetEntries(db *sql.DB, date string) ([]Entry, error) {
	rows, err := db.Query(
		`SELECT id, date, time, category, title, bullets, hours_est FROM entries WHERE date=? ORDER BY time, id`,
		date,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var e Entry
		var raw string
		if err := rows.Scan(&e.ID, &e.Date, &e.Time, &e.Category, &e.Title, &raw, &e.HoursEst); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(raw), &e.Bullets)
		out = append(out, e)
	}
	return out, rows.Err()
}

func DeleteEntryByID(db *sql.DB, id int64) (string, error) {
	var date string
	if err := db.QueryRow(`SELECT date FROM entries WHERE id=?`, id).Scan(&date); err != nil {
		return "", err
	}
	_, err := db.Exec(`DELETE FROM entries WHERE id=?`, id)
	return date, err
}

func RenderMarkdown(entries []Entry) string {
	if len(entries) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Worklog — %s\n\n", entries[0].Date)
	var total float64
	for _, e := range entries {
		total += e.HoursEst
		fmt.Fprintf(&b, "## %s - [%s] %s", e.Time, e.Category, e.Title)
		if e.HoursEst > 0 {
			fmt.Fprintf(&b, " (~%gh)", e.HoursEst)
		}
		b.WriteByte('\n')
		for _, bullet := range e.Bullets {
			fmt.Fprintf(&b, "- %s\n", bullet)
		}
		b.WriteByte('\n')
	}
	fmt.Fprintf(&b, "---\n## Daily Summary\n")
	fmt.Fprintf(&b, "- Entries: %d\n", len(entries))
	fmt.Fprintf(&b, "- Est. person-hours saved: %gh\n", total)
	fmt.Fprintf(&b, "- Est. person-days: %.1fd (assuming 8h/day)\n", total/8)
	return b.String()
}
