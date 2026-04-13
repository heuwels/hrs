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

func (s *Server) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) ListEntries(w http.ResponseWriter, r *http.Request) {
	date := r.URL.Query().Get("date")
	if date == "" {
		date = now().Format("2006-01-02")
	}
	entries, err := GetEntries(s.db, date)
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
