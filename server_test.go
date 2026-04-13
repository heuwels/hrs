package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return &Server{db: db, logDir: t.TempDir()}
}

func testMux(s *Server) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /entries", s.ListEntries)
	mux.HandleFunc("POST /entries", s.CreateEntry)
	mux.HandleFunc("DELETE /entries/{id}", s.DeleteEntry)
	return mux
}

func TestCreateAndList(t *testing.T) {
	s := testServer(t)
	mux := testMux(s)

	body := `{"date":"2026-04-13","time":"10:00","category":"dev","title":"Built worklogd","bullets":["wrote server","added tests"],"hours_est":2}`
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/entries", strings.NewReader(body)))
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/entries?date=2026-04-13", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("list: %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Built worklogd") {
		t.Fatalf("missing entry: %s", w.Body.String())
	}
}

func TestValidation(t *testing.T) {
	s := testServer(t)
	mux := testMux(s)

	for _, tc := range []struct {
		name, body string
	}{
		{"no category", `{"title":"x","bullets":["y"]}`},
		{"no title", `{"category":"x","bullets":["y"]}`},
		{"no bullets", `{"category":"x","title":"y"}`},
		{"empty bullets", `{"category":"x","title":"y","bullets":[]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest("POST", "/entries", strings.NewReader(tc.body)))
			if w.Code != http.StatusBadRequest {
				t.Errorf("got %d want 400", w.Code)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	s := testServer(t)
	mux := testMux(s)

	body := `{"date":"2026-04-13","time":"09:00","category":"dev","title":"temp","bullets":["x"],"hours_est":1}`
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/entries", strings.NewReader(body)))

	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("DELETE", "/entries/1", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("delete: %d", w.Code)
	}

	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/entries?date=2026-04-13", nil))
	if strings.Contains(w.Body.String(), "temp") {
		t.Fatal("entry still exists")
	}
}

func TestDefaultsDateAndTime(t *testing.T) {
	now = func() time.Time { return time.Date(2026, 4, 13, 14, 30, 0, 0, time.UTC) }
	defer func() { now = time.Now }()

	s := testServer(t)
	mux := testMux(s)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/entries",
		strings.NewReader(`{"category":"dev","title":"auto","bullets":["test"]}`)))
	if w.Code != http.StatusCreated {
		t.Fatalf("got %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/entries?date=2026-04-13", nil))
	if !strings.Contains(w.Body.String(), "14:30") {
		t.Fatalf("expected 14:30: %s", w.Body.String())
	}
}

func TestRenderMarkdown(t *testing.T) {
	md := RenderMarkdown([]Entry{
		{Date: "2026-04-13", Time: "10:00", Category: "dev", Title: "Thing one", Bullets: []string{"did a", "did b"}, HoursEst: 2},
		{Date: "2026-04-13", Time: "14:00", Category: "admin", Title: "Thing two", Bullets: []string{"did c"}, HoursEst: 0.5},
	})
	for _, want := range []string{
		"# Worklog — 2026-04-13",
		"## 10:00 - [dev] Thing one (~2h)",
		"- did a",
		"## 14:00 - [admin] Thing two (~0.5h)",
		"Entries: 2",
		"person-hours saved: 2.5h",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("missing %q in:\n%s", want, md)
		}
	}
}

func TestSyncWritesFile(t *testing.T) {
	s := testServer(t)
	mux := testMux(s)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/entries",
		strings.NewReader(`{"date":"2026-04-13","time":"10:00","category":"dev","title":"sync","bullets":["x"],"hours_est":1}`)))

	data, err := os.ReadFile(s.logDir + "/2026-04-13.md")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "sync") {
		t.Fatalf("markdown missing entry: %s", data)
	}
}

func TestEmptyList(t *testing.T) {
	s := testServer(t)
	mux := testMux(s)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/entries?date=2099-01-01", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "[]") {
		t.Fatalf("expected empty array: %s", w.Body.String())
	}
}
