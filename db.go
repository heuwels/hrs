package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
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

var (
	dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	timeRe = regexp.MustCompile(`^\d{2}:\d{2}$`)
)

func validateEntry(e *Entry) error {
	if e.Date != "" && !dateRe.MatchString(e.Date) {
		return fmt.Errorf("invalid date format: %q (expected YYYY-MM-DD)", e.Date)
	}
	if e.Time != "" && !timeRe.MatchString(e.Time) {
		return fmt.Errorf("invalid time format: %q (expected HH:MM)", e.Time)
	}
	if e.HoursEst < 0 || e.HoursEst > 24 {
		return fmt.Errorf("hours_est must be between 0 and 24, got %g", e.HoursEst)
	}
	if strings.TrimSpace(e.Category) == "" {
		return fmt.Errorf("category must be non-empty")
	}
	if strings.TrimSpace(e.Title) == "" {
		return fmt.Errorf("title must be non-empty")
	}
	hasNonEmpty := false
	for _, b := range e.Bullets {
		if strings.TrimSpace(b) != "" {
			hasNonEmpty = true
			break
		}
	}
	if !hasNonEmpty {
		return fmt.Errorf("bullets must have at least one non-empty string")
	}
	return nil
}

func OpenDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", "file:"+path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)")
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
	if err := validateEntry(e); err != nil {
		return 0, err
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

func UpdateEntry(db *sql.DB, id int64, e *Entry) error {
	if err := validateEntry(e); err != nil {
		return err
	}
	b, _ := json.Marshal(e.Bullets)
	_, err := db.Exec(
		`UPDATE entries SET date=?, time=?, category=?, title=?, bullets=?, hours_est=? WHERE id=?`,
		e.Date, e.Time, e.Category, e.Title, string(b), e.HoursEst, id,
	)
	return err
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

func GetEntriesRange(db *sql.DB, from, to, category string) ([]Entry, error) {
	var rows *sql.Rows
	var err error
	if category != "" {
		rows, err = db.Query(
			`SELECT id, date, time, category, title, bullets, hours_est FROM entries WHERE date>=? AND date<=? AND category=? ORDER BY date, time, id`,
			from, to, category,
		)
	} else {
		rows, err = db.Query(
			`SELECT id, date, time, category, title, bullets, hours_est FROM entries WHERE date>=? AND date<=? ORDER BY date, time, id`,
			from, to,
		)
	}
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

func GetEntryByID(db *sql.DB, id int64) (*Entry, error) {
	var e Entry
	var raw string
	err := db.QueryRow(
		`SELECT id, date, time, category, title, bullets, hours_est FROM entries WHERE id=?`, id,
	).Scan(&e.ID, &e.Date, &e.Time, &e.Category, &e.Title, &raw, &e.HoursEst)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(raw), &e.Bullets)
	return &e, nil
}

func GetCategories(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`SELECT DISTINCT category FROM entries ORDER BY category`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		out = append(out, c)
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
	fmt.Fprintf(&b, "- Est. person-hours (without AI): %gh\n", total)
	fmt.Fprintf(&b, "- Est. person-days: %.1fd (assuming 8h/day)\n", total/8)
	return b.String()
}
