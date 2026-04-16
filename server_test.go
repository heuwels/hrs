package main

import (
	"net/http"
	"path/filepath"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := OpenDB(dbPath)
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
	mux.HandleFunc("PUT /entries/{id}", s.UpdateEntryHandler)
	mux.HandleFunc("DELETE /entries/{id}", s.DeleteEntry)
	mux.HandleFunc("GET /goals", s.ListGoals)
	mux.HandleFunc("POST /goals", s.CreateGoal)
	mux.HandleFunc("PUT /goals/{id}/done", s.CompleteGoalHandler)
	mux.HandleFunc("PUT /goals/{id}/undo", s.UncompleteGoalHandler)
	mux.HandleFunc("POST /goals/{id}/link", s.LinkGoalEntriesHandler)
	mux.HandleFunc("DELETE /goals/{id}", s.DeleteGoalHandler)
	return mux
}

func TestCreateAndList(t *testing.T) {
	s := testServer(t)
	mux := testMux(s)

	body := `{"date":"2026-04-13","time":"10:00","category":"dev","title":"Built hrs server","bullets":["wrote server","added tests"],"hours_est":2}`
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
	if !strings.Contains(w.Body.String(), "Built hrs server") {
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

func TestInputValidation(t *testing.T) {
	s := testServer(t)
	mux := testMux(s)

	for _, tc := range []struct {
		name, body string
	}{
		{"bad date", `{"date":"13-04-2026","time":"10:00","category":"dev","title":"x","bullets":["y"]}`},
		{"bad time", `{"date":"2026-04-13","time":"10:00:00","category":"dev","title":"x","bullets":["y"]}`},
		{"negative hours", `{"date":"2026-04-13","time":"10:00","category":"dev","title":"x","bullets":["y"],"hours_est":-1}`},
		{"hours over 24", `{"date":"2026-04-13","time":"10:00","category":"dev","title":"x","bullets":["y"],"hours_est":25}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest("POST", "/entries", strings.NewReader(tc.body)))
			if w.Code == http.StatusCreated {
				t.Errorf("expected error for %s but got 201", tc.name)
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

func TestUpdateEntry(t *testing.T) {
	s := testServer(t)
	mux := testMux(s)

	// Create an entry
	body := `{"date":"2026-04-13","time":"10:00","category":"dev","title":"Original","bullets":["first"],"hours_est":1}`
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/entries", strings.NewReader(body)))
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}

	// Update it
	updated := `{"date":"2026-04-13","time":"10:00","category":"dev","title":"Updated","bullets":["second"],"hours_est":2}`
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("PUT", "/entries/1", strings.NewReader(updated)))
	if w.Code != http.StatusOK {
		t.Fatalf("update: %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Updated") {
		t.Fatalf("expected Updated in response: %s", w.Body.String())
	}

	// Verify the update persisted
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/entries?date=2026-04-13", nil))
	if !strings.Contains(w.Body.String(), "Updated") {
		t.Fatalf("update not persisted: %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "Original") {
		t.Fatalf("old title still present: %s", w.Body.String())
	}
}

func TestDateRangeQuery(t *testing.T) {
	s := testServer(t)
	mux := testMux(s)

	// Create entries on different dates
	for _, body := range []string{
		`{"date":"2026-04-10","time":"10:00","category":"dev","title":"Day10","bullets":["a"],"hours_est":1}`,
		`{"date":"2026-04-11","time":"10:00","category":"admin","title":"Day11","bullets":["b"],"hours_est":1}`,
		`{"date":"2026-04-12","time":"10:00","category":"dev","title":"Day12","bullets":["c"],"hours_est":1}`,
		`{"date":"2026-04-13","time":"10:00","category":"dev","title":"Day13","bullets":["d"],"hours_est":1}`,
	} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("POST", "/entries", strings.NewReader(body)))
		if w.Code != http.StatusCreated {
			t.Fatalf("create: %d %s", w.Code, w.Body.String())
		}
	}

	// Query range
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/entries?from=2026-04-11&to=2026-04-12", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("range query: %d", w.Code)
	}
	resp := w.Body.String()
	if !strings.Contains(resp, "Day11") || !strings.Contains(resp, "Day12") {
		t.Fatalf("range missing expected entries: %s", resp)
	}
	if strings.Contains(resp, "Day10") || strings.Contains(resp, "Day13") {
		t.Fatalf("range includes entries outside range: %s", resp)
	}
}

func TestCategoryFilter(t *testing.T) {
	s := testServer(t)
	mux := testMux(s)

	for _, body := range []string{
		`{"date":"2026-04-10","time":"10:00","category":"dev","title":"DevWork","bullets":["a"],"hours_est":1}`,
		`{"date":"2026-04-10","time":"11:00","category":"admin","title":"AdminWork","bullets":["b"],"hours_est":1}`,
		`{"date":"2026-04-11","time":"10:00","category":"dev","title":"DevWork2","bullets":["c"],"hours_est":1}`,
	} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("POST", "/entries", strings.NewReader(body)))
	}

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/entries?from=2026-04-10&to=2026-04-11&category=dev", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("category query: %d", w.Code)
	}
	resp := w.Body.String()
	if !strings.Contains(resp, "DevWork") || !strings.Contains(resp, "DevWork2") {
		t.Fatalf("missing dev entries: %s", resp)
	}
	if strings.Contains(resp, "AdminWork") {
		t.Fatalf("admin entry should be filtered out: %s", resp)
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
		"person-hours (without AI): 2.5h",
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

func TestSchema(t *testing.T) {
	s := testServer(t)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /schema", s.Schema)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/schema", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("got %d", w.Code)
	}
	body := w.Body.String()
	for _, want := range []string{"category", "title", "bullets", "hours_est", "required", "description"} {
		if !strings.Contains(body, want) {
			t.Errorf("schema missing %q", want)
		}
	}
}

func TestGoalsCRUD(t *testing.T) {
	s := testServer(t)
	mux := testMux(s)

	// Create two entries to link later
	for _, body := range []string{
		`{"date":"2026-04-16","time":"09:00","category":"dev","title":"Entry A","bullets":["a"],"hours_est":1}`,
		`{"date":"2026-04-16","time":"10:00","category":"dev","title":"Entry B","bullets":["b"],"hours_est":2}`,
	} {
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, httptest.NewRequest("POST", "/entries", strings.NewReader(body)))
		if w.Code != http.StatusCreated {
			t.Fatalf("create entry: %d %s", w.Code, w.Body.String())
		}
	}

	// Create a goal
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/goals",
		strings.NewReader(`{"date":"2026-04-16","text":"Ship goals feature"}`)))
	if w.Code != http.StatusCreated {
		t.Fatalf("create goal: %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Ship goals feature") {
		t.Fatalf("missing text: %s", w.Body.String())
	}

	// List goals
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/goals?date=2026-04-16", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("list goals: %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Ship goals feature") {
		t.Fatalf("goal not in list: %s", w.Body.String())
	}

	// Complete the goal, linking to the two real entries (IDs 1 and 2)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("PUT", "/goals/1/done",
		strings.NewReader(`{"entry_ids":[1,2]}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("complete goal: %d %s", w.Code, w.Body.String())
	}

	// Verify completed with entry links
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/goals?date=2026-04-16", nil))
	resp := w.Body.String()
	if !strings.Contains(resp, `"completed":true`) {
		t.Fatalf("goal not completed: %s", resp)
	}
	if !strings.Contains(resp, `"entry_ids":[1,2]`) {
		t.Fatalf("entry_ids not linked: %s", resp)
	}

	// Undo completion
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("PUT", "/goals/1/undo", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("undo goal: %d %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/goals?date=2026-04-16", nil))
	if strings.Contains(w.Body.String(), `"completed":true`) {
		t.Fatalf("goal still completed after undo: %s", w.Body.String())
	}

	// Delete the goal
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("DELETE", "/goals/1", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("delete goal: %d", w.Code)
	}

	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/goals?date=2026-04-16", nil))
	if strings.Contains(w.Body.String(), "Ship goals feature") {
		t.Fatal("goal still exists after delete")
	}
}

func TestGoalValidation(t *testing.T) {
	s := testServer(t)
	mux := testMux(s)

	for _, tc := range []struct {
		name, body string
	}{
		{"no text", `{"date":"2026-04-16"}`},
		{"empty text", `{"date":"2026-04-16","text":""}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest("POST", "/goals", strings.NewReader(tc.body)))
			if w.Code != http.StatusBadRequest {
				t.Errorf("got %d want 400", w.Code)
			}
		})
	}
}

func TestGoalLinkEntries(t *testing.T) {
	s := testServer(t)
	mux := testMux(s)

	// Create goal and entry
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/goals",
		strings.NewReader(`{"date":"2026-04-16","text":"Test linking"}`)))

	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/entries",
		strings.NewReader(`{"date":"2026-04-16","time":"10:00","category":"dev","title":"Did work","bullets":["stuff"],"hours_est":1}`)))

	// Link entry to goal
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/goals/1/link",
		strings.NewReader(`{"entry_ids":[1]}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("link: %d %s", w.Code, w.Body.String())
	}

	// Verify link shows up
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/goals?date=2026-04-16", nil))
	if !strings.Contains(w.Body.String(), `"entry_ids":[1]`) {
		t.Fatalf("entry not linked: %s", w.Body.String())
	}
}

func TestGoalDefaultDate(t *testing.T) {
	now = func() time.Time { return time.Date(2026, 4, 16, 9, 0, 0, 0, time.UTC) }
	defer func() { now = time.Now }()

	s := testServer(t)
	mux := testMux(s)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("POST", "/goals",
		strings.NewReader(`{"text":"Default date goal"}`)))
	if w.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/goals?date=2026-04-16", nil))
	if !strings.Contains(w.Body.String(), "Default date goal") {
		t.Fatalf("goal not found on default date: %s", w.Body.String())
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
