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
	RD       bool     `json:"rd,omitempty"`
}

type Goal struct {
	ID         int64   `json:"id"`
	Date       string  `json:"date"`
	Text       string  `json:"text"`
	Completed  bool    `json:"completed"`
	EntryIDs   []int64 `json:"entry_ids,omitempty"`
	StrategyID *int64  `json:"strategy_id,omitempty"`
	Important  bool    `json:"important"`
	Urgent     bool    `json:"urgent"`
	TicketRef  *string `json:"ticket_ref,omitempty"`
}

type Strategy struct {
	ID          int64   `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description,omitempty"`
	Status      string  `json:"status"`
	CreatedAt   string  `json:"created_at"`
	CompletedAt *string `json:"completed_at,omitempty"`
	Important   bool    `json:"important"`
	Urgent      bool    `json:"urgent"`
	TicketRef   *string `json:"ticket_ref,omitempty"`
}

type EnrichedEntry struct {
	Entry
	GoalID            *int64  `json:"goal_id,omitempty"`
	GoalText          *string `json:"goal_text,omitempty"`
	GoalTicketRef     *string `json:"goal_ticket_ref,omitempty"`
	StrategyID        *int64  `json:"strategy_id,omitempty"`
	StrategyTitle     *string `json:"strategy_title,omitempty"`
	StrategyTicketRef *string `json:"strategy_ticket_ref,omitempty"`
}

type StrategyReport struct {
	Strategy
	GoalsDone  int     `json:"goals_done"`
	GoalsTotal int     `json:"goals_total"`
	TotalHours float64 `json:"total_hours"`
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
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS goals (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		date       TEXT NOT NULL,
		text       TEXT NOT NULL,
		completed  INTEGER NOT NULL DEFAULT 0,
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_goals_date ON goals(date)`)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS goal_entries (
		goal_id  INTEGER NOT NULL REFERENCES goals(id) ON DELETE CASCADE,
		entry_id INTEGER NOT NULL REFERENCES entries(id) ON DELETE CASCADE,
		PRIMARY KEY (goal_id, entry_id)
	)`)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS strategies (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		title        TEXT NOT NULL,
		description  TEXT NOT NULL DEFAULT '',
		status       TEXT NOT NULL DEFAULT 'active',
		created_at   TEXT NOT NULL DEFAULT (datetime('now')),
		completed_at TEXT
	)`)
	if err != nil {
		return nil, err
	}
	// Add strategy_id to goals (safe to call repeatedly -- ignore "duplicate column" error)
	db.Exec(`ALTER TABLE goals ADD COLUMN strategy_id INTEGER REFERENCES strategies(id)`)
	// Add importance/urgency to goals and strategies
	db.Exec(`ALTER TABLE goals ADD COLUMN important INTEGER NOT NULL DEFAULT 0`)
	db.Exec(`ALTER TABLE goals ADD COLUMN urgent INTEGER NOT NULL DEFAULT 0`)
	db.Exec(`ALTER TABLE strategies ADD COLUMN important INTEGER NOT NULL DEFAULT 0`)
	db.Exec(`ALTER TABLE strategies ADD COLUMN urgent INTEGER NOT NULL DEFAULT 0`)
	// Add ticket_ref to goals and strategies
	db.Exec(`ALTER TABLE goals ADD COLUMN ticket_ref TEXT`)
	db.Exec(`ALTER TABLE strategies ADD COLUMN ticket_ref TEXT`)
	// Tag entries that qualify as R&D for tax/grant reporting
	db.Exec(`ALTER TABLE entries ADD COLUMN rd INTEGER NOT NULL DEFAULT 0`)
	// Enable foreign keys
	_, err = db.Exec(`PRAGMA foreign_keys = ON`)
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
		`INSERT INTO entries (date, time, category, title, bullets, hours_est, rd) VALUES (?,?,?,?,?,?,?)`,
		e.Date, e.Time, e.Category, e.Title, string(b), e.HoursEst, e.RD,
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
		`UPDATE entries SET date=?, time=?, category=?, title=?, bullets=?, hours_est=?, rd=? WHERE id=?`,
		e.Date, e.Time, e.Category, e.Title, string(b), e.HoursEst, e.RD, id,
	)
	return err
}

func GetEntries(db *sql.DB, date string) ([]Entry, error) {
	rows, err := db.Query(
		`SELECT id, date, time, category, title, bullets, hours_est, rd FROM entries WHERE date=? ORDER BY time, id`,
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
		if err := rows.Scan(&e.ID, &e.Date, &e.Time, &e.Category, &e.Title, &raw, &e.HoursEst, &e.RD); err != nil {
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
			`SELECT id, date, time, category, title, bullets, hours_est, rd FROM entries WHERE date>=? AND date<=? AND category=? ORDER BY date, time, id`,
			from, to, category,
		)
	} else {
		rows, err = db.Query(
			`SELECT id, date, time, category, title, bullets, hours_est, rd FROM entries WHERE date>=? AND date<=? ORDER BY date, time, id`,
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
		if err := rows.Scan(&e.ID, &e.Date, &e.Time, &e.Category, &e.Title, &raw, &e.HoursEst, &e.RD); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(raw), &e.Bullets)
		out = append(out, e)
	}
	return out, rows.Err()
}

// GetEntriesByTicket returns entries linked to goals or strategies whose
// ticket_ref matches the given LIKE pattern (e.g. "PROMO-%"). Date range and
// optional category are applied as additional filters. Distinct: an entry
// linked to multiple matching goals appears once.
func GetEntriesByTicket(db *sql.DB, from, to, ticketLike, category string) ([]Entry, error) {
	q := `SELECT DISTINCT e.id, e.date, e.time, e.category, e.title, e.bullets, e.hours_est, e.rd
	      FROM entries e
	      JOIN goal_entries ge ON ge.entry_id = e.id
	      JOIN goals g ON g.id = ge.goal_id
	      LEFT JOIN strategies s ON s.id = g.strategy_id
	      WHERE e.date >= ? AND e.date <= ?
	        AND (g.ticket_ref LIKE ? OR s.ticket_ref LIKE ?)`
	args := []any{from, to, ticketLike, ticketLike}
	if category != "" {
		q += ` AND e.category = ?`
		args = append(args, category)
	}
	q += ` ORDER BY e.date, e.time, e.id`
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		var e Entry
		var raw string
		if err := rows.Scan(&e.ID, &e.Date, &e.Time, &e.Category, &e.Title, &raw, &e.HoursEst, &e.RD); err != nil {
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
		`SELECT id, date, time, category, title, bullets, hours_est, rd FROM entries WHERE id=?`, id,
	).Scan(&e.ID, &e.Date, &e.Time, &e.Category, &e.Title, &raw, &e.HoursEst, &e.RD)
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

// --- Goals ---

func InsertGoal(db *sql.DB, date, text string, strategyID *int64, important, urgent bool, ticketRef *string) (int64, error) {
	if date == "" {
		date = now().Format("2006-01-02")
	}
	if !dateRe.MatchString(date) {
		return 0, fmt.Errorf("invalid date format: %q (expected YYYY-MM-DD)", date)
	}
	if strings.TrimSpace(text) == "" {
		return 0, fmt.Errorf("goal text must be non-empty")
	}
	res, err := db.Exec(`INSERT INTO goals (date, text, strategy_id, important, urgent, ticket_ref) VALUES (?, ?, ?, ?, ?, ?)`,
		date, text, strategyID, important, urgent, ticketRef)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func GetGoals(db *sql.DB, date string) ([]Goal, error) {
	rows, err := db.Query(
		`SELECT id, date, text, completed, strategy_id, important, urgent, ticket_ref FROM goals WHERE date=? ORDER BY id`, date,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Goal
	for rows.Next() {
		var g Goal
		if err := rows.Scan(&g.ID, &g.Date, &g.Text, &g.Completed, &g.StrategyID, &g.Important, &g.Urgent, &g.TicketRef); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Load linked entry IDs
	for i := range out {
		out[i].EntryIDs, _ = getGoalEntryIDs(db, out[i].ID)
	}
	return out, nil
}

func getGoalEntryIDs(db *sql.DB, goalID int64) ([]int64, error) {
	rows, err := db.Query(`SELECT entry_id FROM goal_entries WHERE goal_id=? ORDER BY entry_id`, goalID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		rows.Scan(&id)
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func CompleteGoal(db *sql.DB, id int64, entryIDs []int64) error {
	_, err := db.Exec(`UPDATE goals SET completed=1 WHERE id=?`, id)
	if err != nil {
		return err
	}
	for _, eid := range entryIDs {
		db.Exec(`INSERT OR IGNORE INTO goal_entries (goal_id, entry_id) VALUES (?, ?)`, id, eid)
	}
	return nil
}

func UncompleteGoal(db *sql.DB, id int64) error {
	_, err := db.Exec(`UPDATE goals SET completed=0 WHERE id=?`, id)
	if err != nil {
		return err
	}
	_, err = db.Exec(`DELETE FROM goal_entries WHERE goal_id=?`, id)
	return err
}

func DeleteGoal(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM goals WHERE id=?`, id)
	return err
}

func GetGoalByID(db *sql.DB, id int64) (*Goal, error) {
	var g Goal
	err := db.QueryRow(`SELECT id, date, text, completed, strategy_id, important, urgent, ticket_ref FROM goals WHERE id=?`, id).
		Scan(&g.ID, &g.Date, &g.Text, &g.Completed, &g.StrategyID, &g.Important, &g.Urgent, &g.TicketRef)
	if err != nil {
		return nil, err
	}
	g.EntryIDs, _ = getGoalEntryIDs(db, g.ID)
	return &g, nil
}

func LinkGoalEntries(db *sql.DB, goalID int64, entryIDs []int64) error {
	for _, eid := range entryIDs {
		_, err := db.Exec(`INSERT OR IGNORE INTO goal_entries (goal_id, entry_id) VALUES (?, ?)`, goalID, eid)
		if err != nil {
			return err
		}
	}
	return nil
}

// UnlinkGoalEntries removes entry-to-goal links. Idempotent — removing an
// already-absent link is a no-op rather than an error. Returns the count of
// rows actually removed so callers can distinguish "all five removed" from
// "three removed, two were never linked".
func UnlinkGoalEntries(db *sql.DB, goalID int64, entryIDs []int64) (int64, error) {
	var total int64
	for _, eid := range entryIDs {
		res, err := db.Exec(`DELETE FROM goal_entries WHERE goal_id=? AND entry_id=?`, goalID, eid)
		if err != nil {
			return total, err
		}
		n, _ := res.RowsAffected()
		total += n
	}
	return total, nil
}

// --- Strategies ---

func InsertStrategy(db *sql.DB, title, description string, important, urgent bool, ticketRef *string) (int64, error) {
	if strings.TrimSpace(title) == "" {
		return 0, fmt.Errorf("strategy title must be non-empty")
	}
	res, err := db.Exec(`INSERT INTO strategies (title, description, important, urgent, ticket_ref) VALUES (?, ?, ?, ?, ?)`,
		title, description, important, urgent, ticketRef)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetStrategiesForDate returns active strategies plus any completed/archived on the given date.
func GetStrategiesForDate(db *sql.DB, date string) ([]Strategy, error) {
	rows, err := db.Query(
		`SELECT id, title, description, status, created_at, completed_at, important, urgent, ticket_ref
		 FROM strategies
		 WHERE status = 'active' OR DATE(completed_at) = ?
		 ORDER BY id`, date,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Strategy
	for rows.Next() {
		var s Strategy
		if err := rows.Scan(&s.ID, &s.Title, &s.Description, &s.Status, &s.CreatedAt, &s.CompletedAt, &s.Important, &s.Urgent, &s.TicketRef); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func GetStrategies(db *sql.DB, status string) ([]Strategy, error) {
	q := `SELECT id, title, description, status, created_at, completed_at, important, urgent, ticket_ref FROM strategies`
	var args []any
	if status != "" {
		q += ` WHERE status=?`
		args = append(args, status)
	}
	q += ` ORDER BY id`
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Strategy
	for rows.Next() {
		var s Strategy
		if err := rows.Scan(&s.ID, &s.Title, &s.Description, &s.Status, &s.CreatedAt, &s.CompletedAt, &s.Important, &s.Urgent, &s.TicketRef); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func GetStrategyByID(db *sql.DB, id int64) (*Strategy, error) {
	var s Strategy
	err := db.QueryRow(
		`SELECT id, title, description, status, created_at, completed_at, important, urgent, ticket_ref FROM strategies WHERE id=?`, id,
	).Scan(&s.ID, &s.Title, &s.Description, &s.Status, &s.CreatedAt, &s.CompletedAt, &s.Important, &s.Urgent, &s.TicketRef)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func UpdateStrategyStatus(db *sql.DB, id int64, status string) error {
	var completedAt any
	if status == "completed" {
		t := now().Format("2006-01-02T15:04:05")
		completedAt = t
	}
	_, err := db.Exec(`UPDATE strategies SET status=?, completed_at=? WHERE id=?`, status, completedAt, id)
	return err
}

func UpdateStrategy(db *sql.DB, id int64, title, description string, important, urgent bool, ticketRef *string) error {
	_, err := db.Exec(`UPDATE strategies SET title=?, description=?, important=?, urgent=?, ticket_ref=? WHERE id=?`,
		title, description, important, urgent, ticketRef, id)
	return err
}

func DeleteStrategy(db *sql.DB, id int64) error {
	// Unlink goals first (set strategy_id to NULL)
	db.Exec(`UPDATE goals SET strategy_id=NULL WHERE strategy_id=?`, id)
	_, err := db.Exec(`DELETE FROM strategies WHERE id=?`, id)
	return err
}

func GetStrategyReport(db *sql.DB, id int64) (*StrategyReport, error) {
	s, err := GetStrategyByID(db, id)
	if err != nil {
		return nil, err
	}
	r := &StrategyReport{Strategy: *s}

	// Count goals
	db.QueryRow(`SELECT COUNT(*) FROM goals WHERE strategy_id=?`, id).Scan(&r.GoalsTotal)
	db.QueryRow(`SELECT COUNT(*) FROM goals WHERE strategy_id=? AND completed=1`, id).Scan(&r.GoalsDone)

	// Sum hours from linked entries (entries linked to goals linked to this strategy)
	db.QueryRow(`
		SELECT COALESCE(SUM(e.hours_est), 0)
		FROM entries e
		JOIN goal_entries ge ON ge.entry_id = e.id
		JOIN goals g ON g.id = ge.goal_id
		WHERE g.strategy_id = ?
	`, id).Scan(&r.TotalHours)

	return r, nil
}

func GetStrategyGoals(db *sql.DB, strategyID int64) ([]Goal, error) {
	rows, err := db.Query(
		`SELECT id, date, text, completed, strategy_id, important, urgent, ticket_ref FROM goals WHERE strategy_id=? ORDER BY date DESC, id`, strategyID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Goal
	for rows.Next() {
		var g Goal
		if err := rows.Scan(&g.ID, &g.Date, &g.Text, &g.Completed, &g.StrategyID, &g.Important, &g.Urgent, &g.TicketRef); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		out[i].EntryIDs, _ = getGoalEntryIDs(db, out[i].ID)
	}
	return out, nil
}

func UpdateGoal(db *sql.DB, id int64, text string, strategyID *int64, important, urgent bool, ticketRef *string) error {
	_, err := db.Exec(`UPDATE goals SET text=?, strategy_id=?, important=?, urgent=?, ticket_ref=? WHERE id=?`,
		text, strategyID, important, urgent, ticketRef, id)
	return err
}

// UpdateGoalDate moves a goal to a different day. Used by the
// "carry yesterday's incomplete goal to today" workflow.
func UpdateGoalDate(db *sql.DB, id int64, date string) error {
	if !dateRe.MatchString(date) {
		return fmt.Errorf("invalid date format: %q (expected YYYY-MM-DD)", date)
	}
	_, err := db.Exec(`UPDATE goals SET date=? WHERE id=?`, date, id)
	return err
}

// GetAllGoals returns every goal in the database, completed and active, with
// EntryIDs populated. Used by `hrs backup` to produce a full-state snapshot.
func GetAllGoals(db *sql.DB) ([]Goal, error) {
	rows, err := db.Query(
		`SELECT id, date, text, completed, strategy_id, important, urgent, ticket_ref
		 FROM goals ORDER BY date, id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Goal
	for rows.Next() {
		var g Goal
		if err := rows.Scan(&g.ID, &g.Date, &g.Text, &g.Completed, &g.StrategyID, &g.Important, &g.Urgent, &g.TicketRef); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		out[i].EntryIDs, _ = getGoalEntryIDs(db, out[i].ID)
	}
	return out, nil
}

func GetActiveGoals(db *sql.DB) ([]Goal, error) {
	rows, err := db.Query(
		`SELECT id, date, text, completed, strategy_id, important, urgent, ticket_ref
		 FROM goals WHERE completed=0
		 ORDER BY important DESC, urgent DESC, date DESC, id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Goal
	for rows.Next() {
		var g Goal
		if err := rows.Scan(&g.ID, &g.Date, &g.Text, &g.Completed, &g.StrategyID, &g.Important, &g.Urgent, &g.TicketRef); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		out[i].EntryIDs, _ = getGoalEntryIDs(db, out[i].ID)
	}
	return out, nil
}

func GetEnrichedEntries(db *sql.DB, date string) ([]EnrichedEntry, error) {
	rows, err := db.Query(`
		SELECT e.id, e.date, e.time, e.category, e.title, e.bullets, e.hours_est, e.rd,
		       g.id, g.text, g.ticket_ref, s.id, s.title, s.ticket_ref
		FROM entries e
		LEFT JOIN (
			SELECT ge.entry_id, MIN(ge.goal_id) as goal_id
			FROM goal_entries ge
			GROUP BY ge.entry_id
		) first_goal ON first_goal.entry_id = e.id
		LEFT JOIN goals g ON g.id = first_goal.goal_id
		LEFT JOIN strategies s ON s.id = g.strategy_id
		WHERE e.date=?
		ORDER BY e.time, e.id
	`, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EnrichedEntry
	for rows.Next() {
		var ee EnrichedEntry
		var raw string
		var gID, sID sql.NullInt64
		var gText, sTitle, gTicket, sTicket sql.NullString
		if err := rows.Scan(&ee.ID, &ee.Date, &ee.Time, &ee.Category, &ee.Title, &raw, &ee.HoursEst, &ee.RD,
			&gID, &gText, &gTicket, &sID, &sTitle, &sTicket); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(raw), &ee.Bullets)
		if gID.Valid {
			ee.GoalID = &gID.Int64
			ee.GoalText = &gText.String
		}
		if gTicket.Valid {
			t := gTicket.String
			ee.GoalTicketRef = &t
		}
		if sID.Valid {
			ee.StrategyID = &sID.Int64
			ee.StrategyTitle = &sTitle.String
		}
		if sTicket.Valid {
			t := sTicket.String
			ee.StrategyTicketRef = &t
		}
		out = append(out, ee)
	}
	return out, rows.Err()
}

func RenderMarkdown(entries []Entry) string {
	if len(entries) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# Worklog — %s\n\n", entries[0].Date)
	var total, totalRD float64
	for _, e := range entries {
		total += e.HoursEst
		if e.RD {
			totalRD += e.HoursEst
		}
		fmt.Fprintf(&b, "## %s - [%s] %s", e.Time, e.Category, e.Title)
		if e.HoursEst > 0 {
			fmt.Fprintf(&b, " (~%gh)", e.HoursEst)
		}
		if e.RD {
			b.WriteString(" [R&D]")
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
	if totalRD > 0 {
		fmt.Fprintf(&b, "- R&D hours: %gh (%.0f%%)\n", totalRD, 100*totalRD/total)
	}
	return b.String()
}
