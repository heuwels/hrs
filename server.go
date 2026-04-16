package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

type Server struct {
	db     *sql.DB
	logDir string
}

var schema = map[string]any{
	"endpoints": []map[string]any{
		{
			"method": "POST",
			"path":   "/entries",
			"fields": []map[string]any{
				{"name": "category", "type": "string", "required": true, "description": "Work category, e.g. dev, security, admin, docs, infra"},
				{"name": "title", "type": "string", "required": true, "description": "Concise label for the work performed"},
				{"name": "bullets", "type": "[]string", "required": true, "description": "Terse, outcome-focused bullet points"},
				{"name": "hours_est", "type": "number", "required": false, "description": "Estimated person-hours this would take without AI assistance", "default": 0},
				{"name": "date", "type": "string", "required": false, "description": "YYYY-MM-DD, defaults to today"},
				{"name": "time", "type": "string", "required": false, "description": "HH:MM, defaults to now"},
			},
		},
		{
			"method":      "PUT",
			"path":        "/entries/{id}",
			"description": "Update an existing entry by ID. All fields from POST /entries apply.",
		},
		{
			"method":      "GET",
			"path":        "/entries",
			"description": "List entries. Query params: date (single day), from/to (range, inclusive), category (filter).",
		},
		{
			"method":      "DELETE",
			"path":        "/entries/{id}",
			"description": "Delete an entry by ID.",
		},
		{
			"method": "POST",
			"path":   "/goals",
			"fields": []map[string]any{
				{"name": "text", "type": "string", "required": true, "description": "Goal description"},
				{"name": "date", "type": "string", "required": false, "description": "YYYY-MM-DD, defaults to today"},
				{"name": "strategy_id", "type": "number", "required": false, "description": "Link to a strategic goal"},
			},
		},
		{
			"method":      "GET",
			"path":        "/goals",
			"description": "List goals. Query params: date (YYYY-MM-DD, defaults to today).",
		},
		{
			"method":      "PUT",
			"path":        "/goals/{id}/done",
			"description": "Mark a goal complete. Optional body: {\"entry_ids\": [1,2]} to link entries.",
		},
		{
			"method":      "PUT",
			"path":        "/goals/{id}/undo",
			"description": "Reopen a completed goal.",
		},
		{
			"method":      "POST",
			"path":        "/goals/{id}/link",
			"description": "Link entry IDs to a goal. Body: {\"entry_ids\": [1,2]}.",
		},
		{
			"method":      "DELETE",
			"path":        "/goals/{id}",
			"description": "Delete a goal.",
		},
		{
			"method": "POST",
			"path":   "/strategies",
			"fields": []map[string]any{
				{"name": "title", "type": "string", "required": true, "description": "Strategic goal title"},
				{"name": "description", "type": "string", "required": false, "description": "Longer description of the strategic goal"},
			},
		},
		{
			"method":      "GET",
			"path":        "/strategies",
			"description": "List strategies. Query param: status (active|completed|archived, default: all).",
		},
		{
			"method":      "GET",
			"path":        "/strategies/{id}",
			"description": "Get strategy report with goal counts and total hours.",
		},
		{
			"method":      "PUT",
			"path":        "/strategies/{id}",
			"description": "Update strategy status. Body: {\"status\": \"active|completed|archived\"}.",
		},
		{
			"method":      "DELETE",
			"path":        "/strategies/{id}",
			"description": "Delete a strategy (unlinks goals, does not delete them).",
		},
	},
}

func (s *Server) Schema(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, schema)
}

func (s *Server) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) ListEntries(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	from := q.Get("from")
	to := q.Get("to")
	category := q.Get("category")

	var entries []Entry
	var err error

	if from != "" {
		if to == "" {
			to = now().Format("2006-01-02")
		}
		entries, err = GetEntriesRange(s.db, from, to, category)
	} else {
		date := q.Get("date")
		if date == "" {
			date = now().Format("2006-01-02")
		}
		entries, err = GetEntries(s.db, date)
	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if entries == nil {
		entries = []Entry{}
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) CreateEntry(w http.ResponseWriter, r *http.Request) {
	var e Entry
	if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json: " + err.Error()})
		return
	}
	if e.Category == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "category required"})
		return
	}
	if e.Title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title required"})
		return
	}
	if len(e.Bullets) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bullets required"})
		return
	}
	id, err := InsertEntry(s.db, &e)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.syncMarkdown(e.Date)
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "date": e.Date})
}

func (s *Server) UpdateEntryHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	var e Entry
	if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json: " + err.Error()})
		return
	}
	if err := UpdateEntry(s.db, id, &e); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	e.ID = id
	s.syncMarkdown(e.Date)
	writeJSON(w, http.StatusOK, e)
}

func (s *Server) DeleteEntry(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	date, err := DeleteEntryByID(s.db, id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	s.syncMarkdown(date)
	writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
}

// --- Goal handlers ---

func (s *Server) ListGoals(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		date = now().Format("2006-01-02")
	}
	goals, err := GetGoals(s.db, date)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if goals == nil {
		goals = []Goal{}
	}
	writeJSON(w, http.StatusOK, goals)
}

func (s *Server) CreateGoal(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Date       string `json:"date"`
		Text       string `json:"text"`
		StrategyID *int64 `json:"strategy_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json: " + err.Error()})
		return
	}
	if body.Text == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "text required"})
		return
	}
	id, err := InsertGoal(s.db, body.Date, body.Text, body.StrategyID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if body.Date == "" {
		body.Date = now().Format("2006-01-02")
	}
	resp := map[string]any{"id": id, "date": body.Date, "text": body.Text}
	if body.StrategyID != nil {
		resp["strategy_id"] = *body.StrategyID
	}
	writeJSON(w, http.StatusCreated, resp)
}

func (s *Server) CompleteGoalHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	var body struct {
		EntryIDs []int64 `json:"entry_ids"`
	}
	json.NewDecoder(r.Body).Decode(&body) // optional body
	if err := CompleteGoal(s.db, id, body.EntryIDs); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"completed": id, "entry_ids": body.EntryIDs})
}

func (s *Server) UncompleteGoalHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	if err := UncompleteGoal(s.db, id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reopened": id})
}

func (s *Server) LinkGoalEntriesHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	var body struct {
		EntryIDs []int64 `json:"entry_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json: " + err.Error()})
		return
	}
	if len(body.EntryIDs) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "entry_ids required"})
		return
	}
	if err := LinkGoalEntries(s.db, id, body.EntryIDs); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"goal_id": id, "linked_entries": body.EntryIDs})
}

func (s *Server) DeleteGoalHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	if err := DeleteGoal(s.db, id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
}

// --- Strategy handlers ---

func (s *Server) ListStrategies(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	strategies, err := GetStrategies(s.db, status)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if strategies == nil {
		strategies = []Strategy{}
	}
	writeJSON(w, http.StatusOK, strategies)
}

func (s *Server) CreateStrategy(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json: " + err.Error()})
		return
	}
	if body.Title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title required"})
		return
	}
	id, err := InsertStrategy(s.db, body.Title, body.Description)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "title": body.Title})
}

func (s *Server) GetStrategyReportHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	report, err := GetStrategyReport(s.db, id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (s *Server) UpdateStrategyStatusHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json: " + err.Error()})
		return
	}
	switch body.Status {
	case "active", "completed", "archived":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "status must be active, completed, or archived"})
		return
	}
	if err := UpdateStrategyStatus(s.db, id, body.Status); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "status": body.Status})
}

func (s *Server) DeleteStrategyHandler(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	if err := DeleteStrategy(s.db, id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": id})
}

func (s *Server) syncMarkdown(date string) {
	entries, _ := GetEntries(s.db, date)
	path := filepath.Join(s.logDir, date+".md")
	if md := RenderMarkdown(entries); md != "" {
		os.WriteFile(path, []byte(md), 0644)
	} else {
		os.Remove(path)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
