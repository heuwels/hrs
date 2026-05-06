package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// --- helpers ---

func testDB(t *testing.T) *tuiModel {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	m := newTUIModel(db, "2026-05-06")
	m.height = 40
	m.width = 120
	return &m
}

func seedEntries(t *testing.T, m *tuiModel) {
	t.Helper()
	InsertEntry(m.db, &Entry{Date: "2026-05-06", Time: "09:00", Category: "dev", Title: "Entry A", Bullets: []string{"did a"}, HoursEst: 1})
	InsertEntry(m.db, &Entry{Date: "2026-05-06", Time: "10:00", Category: "admin", Title: "Entry B", Bullets: []string{"did b"}, HoursEst: 2})
	InsertEntry(m.db, &Entry{Date: "2026-05-06", Time: "11:00", Category: "dev", Title: "Entry C", Bullets: []string{"did c"}, HoursEst: 1.5})
	m.loadEntries()
}

func seedGoals(t *testing.T, m *tuiModel) {
	t.Helper()
	InsertGoal(m.db, "2026-05-06", "Fix the bug", nil, true, true)
	InsertGoal(m.db, "2026-05-06", "Write docs", nil, true, false)
	InsertGoal(m.db, "2026-05-06", "Reply to email", nil, false, true)
	InsertGoal(m.db, "2026-05-06", "Tidy bookmarks", nil, false, false)
	m.loadGoals()
	m.loadActiveGoals()
}

func seedStrategies(t *testing.T, m *tuiModel) {
	t.Helper()
	InsertStrategy(m.db, "Ship v2", "Major release", true, true)
	InsertStrategy(m.db, "Improve docs", "Better onboarding", true, false)
	InsertStrategy(m.db, "Quick wins", "", false, true)
	m.loadStrategies()
	m.loadActiveStrats()
}

func key(r rune) tea.KeyMsg {
	return tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{r}})
}

func specialKey(kt tea.KeyType) tea.KeyMsg {
	return tea.KeyMsg(tea.Key{Type: kt})
}

func send(m tea.Model, msgs ...tea.Msg) tea.Model {
	for _, msg := range msgs {
		m, _ = m.Update(msg)
	}
	return m
}

func asModel(m tea.Model) tuiModel {
	return m.(tuiModel)
}

// --- Model-level unit tests ---

func TestTUIStartsOnEntries(t *testing.T) {
	m := testDB(t)
	if m.view != viewEntries {
		t.Errorf("expected viewEntries, got %d", m.view)
	}
	if m.topView() != "entries" {
		t.Errorf("expected entries top view, got %s", m.topView())
	}
}

func TestTUITabCycling(t *testing.T) {
	m := testDB(t)
	tab := specialKey(tea.KeyTab)

	// entries -> goals list
	r := asModel(send(m, tab))
	if r.view != viewGoalsList {
		t.Errorf("expected viewGoalsList, got %d", r.view)
	}

	// goals list -> strats list
	r = asModel(send(r, tab))
	if r.view != viewStratList {
		t.Errorf("expected viewStratList, got %d", r.view)
	}

	// strats -> entries
	r = asModel(send(r, tab))
	if r.view != viewEntries {
		t.Errorf("expected viewEntries, got %d", r.view)
	}
}

func TestTUIShiftTabCycling(t *testing.T) {
	m := testDB(t)
	tab := specialKey(tea.KeyTab)
	stab := specialKey(tea.KeyShiftTab)

	// entries -> shift+tab -> strategies
	r := asModel(send(m, stab))
	if r.view != viewStratList {
		t.Errorf("expected viewStratList, got %d", r.view)
	}

	// strategies -> shift+tab -> goals
	r = asModel(send(r, stab))
	if r.view != viewGoalsList {
		t.Errorf("expected viewGoalsList, got %d", r.view)
	}

	// goals -> shift+tab -> entries
	r = asModel(send(r, stab))
	if r.view != viewEntries {
		t.Errorf("expected viewEntries, got %d", r.view)
	}

	// tab forward then shift+tab back should return to same place
	r = asModel(send(m, tab, stab))
	if r.view != viewEntries {
		t.Errorf("expected viewEntries after tab+shift+tab, got %d", r.view)
	}
}

func TestTUISubViewToggle(t *testing.T) {
	m := testDB(t)
	tab := specialKey(tea.KeyTab)

	// Switch to goals, then toggle to matrix
	r := asModel(send(m, tab))
	if r.view != viewGoalsList {
		t.Fatalf("expected viewGoalsList, got %d", r.view)
	}
	r = asModel(send(r, key('m')))
	if r.view != viewGoalsMatrix {
		t.Errorf("expected viewGoalsMatrix, got %d", r.view)
	}

	// Toggle back to list
	r = asModel(send(r, key('m')))
	if r.view != viewGoalsList {
		t.Errorf("expected viewGoalsList, got %d", r.view)
	}

	// Switch to strategies, toggle matrix
	r = asModel(send(r, tab))
	if r.view != viewStratList {
		t.Fatalf("expected viewStratList, got %d", r.view)
	}
	r = asModel(send(r, key('m')))
	if r.view != viewStratMatrix {
		t.Errorf("expected viewStratMatrix, got %d", r.view)
	}
}

func TestTUISubViewPreference(t *testing.T) {
	m := testDB(t)
	tab := specialKey(tea.KeyTab)

	// Go to goals, switch to matrix
	r := asModel(send(m, tab, key('m')))
	if r.view != viewGoalsMatrix {
		t.Fatalf("expected viewGoalsMatrix, got %d", r.view)
	}

	// Tab to strats and back to entries
	r = asModel(send(r, tab, tab))
	if r.view != viewEntries {
		t.Fatalf("expected viewEntries, got %d", r.view)
	}

	// Tab to goals again -- should remember matrix preference
	r = asModel(send(r, tab))
	if r.view != viewGoalsMatrix {
		t.Errorf("expected goals matrix preference to be remembered, got %d", r.view)
	}
}

func TestTUIEntryCursorNavigation(t *testing.T) {
	m := testDB(t)
	seedEntries(t, m)

	if m.cursor != 0 {
		t.Fatalf("cursor should start at 0, got %d", m.cursor)
	}

	// j moves down
	r := asModel(send(m, key('j')))
	if r.cursor != 1 {
		t.Errorf("cursor should be 1 after j, got %d", r.cursor)
	}

	// j again
	r = asModel(send(r, key('j')))
	if r.cursor != 2 {
		t.Errorf("cursor should be 2, got %d", r.cursor)
	}

	// j at bottom doesn't overflow
	r = asModel(send(r, key('j')))
	if r.cursor != 2 {
		t.Errorf("cursor should stay at 2, got %d", r.cursor)
	}

	// k moves up
	r = asModel(send(r, key('k')))
	if r.cursor != 1 {
		t.Errorf("cursor should be 1 after k, got %d", r.cursor)
	}

	// g goes to top
	r = asModel(send(r, key('g')))
	if r.cursor != 0 {
		t.Errorf("cursor should be 0 after g, got %d", r.cursor)
	}

	// G goes to bottom
	r = asModel(send(r, key('G')))
	if r.cursor != 2 {
		t.Errorf("cursor should be 2 after G, got %d", r.cursor)
	}
}

func TestTUIGoalCursorNavigation(t *testing.T) {
	m := testDB(t)
	seedGoals(t, m)
	tab := specialKey(tea.KeyTab)

	r := asModel(send(m, tab)) // switch to goals
	if r.gCursor != 0 {
		t.Fatalf("goal cursor should start at 0")
	}

	r = asModel(send(r, key('j'), key('j')))
	if r.gCursor != 2 {
		t.Errorf("goal cursor should be 2, got %d", r.gCursor)
	}

	r = asModel(send(r, key('k')))
	if r.gCursor != 1 {
		t.Errorf("goal cursor should be 1, got %d", r.gCursor)
	}
}

func TestTUIGoalCompletion(t *testing.T) {
	m := testDB(t)
	seedGoals(t, m)
	tab := specialKey(tea.KeyTab)

	// Switch to goals, complete the first one (no entries to link)
	r := asModel(send(m, tab, key('x')))

	if !r.goals[0].Completed {
		t.Error("goal should be completed")
	}

	// Undo completion
	r = asModel(send(r, key('x')))
	if r.goals[0].Completed {
		t.Error("goal should be uncompleted after undo")
	}
}

func TestTUIGoalCompletionWithLinking(t *testing.T) {
	m := testDB(t)
	seedEntries(t, m)
	seedGoals(t, m)
	tab := specialKey(tea.KeyTab)

	// Switch to goals, press x -- should enter linking mode since entries exist
	r := asModel(send(m, tab, key('x')))
	if !r.linking {
		t.Fatal("should be in linking mode")
	}

	// Select first entry with space, confirm with enter
	r = asModel(send(r, specialKey(tea.KeySpace), specialKey(tea.KeyEnter)))
	if r.linking {
		t.Error("should have exited linking mode")
	}
	if !r.goals[0].Completed {
		t.Error("goal should be completed after linking")
	}
}

func TestTUIGoalCompletionSkipLinking(t *testing.T) {
	m := testDB(t)
	seedEntries(t, m)
	seedGoals(t, m)
	tab := specialKey(tea.KeyTab)

	// Enter linking mode then press esc to skip
	r := asModel(send(m, tab, key('x'), specialKey(tea.KeyEsc)))
	if r.linking {
		t.Error("should have exited linking mode")
	}
	if !r.goals[0].Completed {
		t.Error("goal should be completed even after skip")
	}
}

func TestTUIToggleImportance(t *testing.T) {
	m := testDB(t)
	seedGoals(t, m)
	tab := specialKey(tea.KeyTab)

	// Goals[1] starts important=true. Toggle it off.
	r := asModel(send(m, tab, key('j'), key('i')))
	if r.goals[1].Important {
		t.Error("goal should no longer be important")
	}
	// Cursor should stay on goal 1 after reload
	if r.gCursor != 1 {
		t.Errorf("cursor should stay at 1, got %d", r.gCursor)
	}

	// Toggle it back on (cursor preserved, no need to navigate)
	r = asModel(send(r, key('i')))
	if !r.goals[1].Important {
		t.Error("goal should be important again")
	}
}

func TestTUIToggleUrgency(t *testing.T) {
	m := testDB(t)
	seedGoals(t, m)
	tab := specialKey(tea.KeyTab)

	// Goals[0] starts urgent=true. Toggle it off.
	r := asModel(send(m, tab, key('u')))
	if r.goals[0].Urgent {
		t.Error("goal should no longer be urgent")
	}
}

func TestTUIDeleteGoal(t *testing.T) {
	m := testDB(t)
	seedGoals(t, m)
	tab := specialKey(tea.KeyTab)
	initialCount := len(m.goals)

	r := asModel(send(m, tab, key('d')))
	if len(r.goals) != initialCount-1 {
		t.Errorf("expected %d goals after delete, got %d", initialCount-1, len(r.goals))
	}
}

func TestTUIDeleteEntry(t *testing.T) {
	m := testDB(t)
	seedEntries(t, m)
	initialCount := len(m.entries)

	r := asModel(send(m, key('d')))
	if len(r.entries) != initialCount-1 {
		t.Errorf("expected %d entries after delete, got %d", initialCount-1, len(r.entries))
	}
}

func TestTUIStrategyNavigation(t *testing.T) {
	m := testDB(t)
	seedStrategies(t, m)
	tab := specialKey(tea.KeyTab)

	// entries -> goals -> strategies
	r := asModel(send(m, tab, tab))
	if r.view != viewStratList {
		t.Fatalf("expected viewStratList, got %d", r.view)
	}
	if r.sCursor != 0 {
		t.Fatalf("strategy cursor should start at 0")
	}

	r = asModel(send(r, key('j')))
	if r.sCursor != 1 {
		t.Errorf("strategy cursor should be 1, got %d", r.sCursor)
	}
}

func TestTUIStrategyTogglePriority(t *testing.T) {
	m := testDB(t)
	seedStrategies(t, m)
	tab := specialKey(tea.KeyTab)

	// Go to strategies, toggle importance on first one (starts important=true)
	r := asModel(send(m, tab, tab, key('i')))
	if r.strategies[0].Important {
		t.Error("strategy should no longer be important")
	}

	// Toggle urgency (starts urgent=true)
	r = asModel(send(r, key('u')))
	if r.strategies[0].Urgent {
		t.Error("strategy should no longer be urgent")
	}
}

func TestTUIGoalsMatrixPartitioning(t *testing.T) {
	m := testDB(t)
	seedGoals(t, m)

	quads := partitionGoals(m.activeGoals)

	// Q1: important + urgent = "Fix the bug"
	if len(quads[0]) != 1 || quads[0][0].Text != "Fix the bug" {
		t.Errorf("Q1 wrong: %v", quads[0])
	}
	// Q2: important only = "Write docs"
	if len(quads[1]) != 1 || quads[1][0].Text != "Write docs" {
		t.Errorf("Q2 wrong: %v", quads[1])
	}
	// Q3: urgent only = "Reply to email"
	if len(quads[2]) != 1 || quads[2][0].Text != "Reply to email" {
		t.Errorf("Q3 wrong: %v", quads[2])
	}
	// Q4: neither = "Tidy bookmarks"
	if len(quads[3]) != 1 || quads[3][0].Text != "Tidy bookmarks" {
		t.Errorf("Q4 wrong: %v", quads[3])
	}
}

func TestTUIStrategyMatrixPartitioning(t *testing.T) {
	m := testDB(t)
	seedStrategies(t, m)

	quads := partitionStrategies(m.activeStrats)

	if len(quads[0]) != 1 || quads[0][0].Title != "Ship v2" {
		t.Errorf("Q1 wrong: %v", quads[0])
	}
	if len(quads[1]) != 1 || quads[1][0].Title != "Improve docs" {
		t.Errorf("Q2 wrong: %v", quads[1])
	}
	if len(quads[2]) != 1 || quads[2][0].Title != "Quick wins" {
		t.Errorf("Q3 wrong: %v", quads[2])
	}
}

func TestTUIMatrixQuadrantNavigation(t *testing.T) {
	m := testDB(t)
	seedGoals(t, m)
	tab := specialKey(tea.KeyTab)
	right := specialKey(tea.KeyRight)
	left := specialKey(tea.KeyLeft)

	// Go to goals matrix
	r := asModel(send(m, tab, key('m')))
	if r.view != viewGoalsMatrix {
		t.Fatalf("expected viewGoalsMatrix, got %d", r.view)
	}
	if r.mQuadrant != 0 {
		t.Fatalf("should start in Q1 (quadrant 0)")
	}

	// right arrow moves to Q2
	r = asModel(send(r, right))
	if r.mQuadrant != 1 {
		t.Errorf("expected quadrant 1, got %d", r.mQuadrant)
	}

	// right from Q2 stays (no Q beyond right)
	r = asModel(send(r, right))
	if r.mQuadrant != 1 {
		t.Errorf("expected quadrant 1, got %d", r.mQuadrant)
	}

	// left arrow moves back to Q1
	r = asModel(send(r, left))
	if r.mQuadrant != 0 {
		t.Errorf("expected quadrant 0, got %d", r.mQuadrant)
	}

	// j from Q1 with 1 item moves to Q3
	r = asModel(send(r, key('j')))
	if r.mQuadrant != 2 {
		t.Errorf("expected quadrant 2 after j past single item, got %d", r.mQuadrant)
	}

	// k from Q3 moves back to Q1
	r = asModel(send(r, key('k')))
	if r.mQuadrant != 0 {
		t.Errorf("expected quadrant 0 after k from Q3, got %d", r.mQuadrant)
	}
}

func TestTUIMatrixToggleMovesBetweenQuadrants(t *testing.T) {
	m := testDB(t)
	seedGoals(t, m)
	tab := specialKey(tea.KeyTab)

	// Go to goals matrix, toggle importance on Q1 item (important+urgent -> urgent only = Q3)
	r := asModel(send(m, tab, key('m'), key('i')))
	quads := partitionGoals(r.goals)
	if len(quads[0]) != 0 {
		t.Errorf("Q1 should be empty after toggling importance, got %d items", len(quads[0]))
	}
	if len(quads[2]) != 2 {
		t.Errorf("Q3 should have 2 items (original + moved), got %d", len(quads[2]))
	}
}

func TestTUIMatrixDeleteGoal(t *testing.T) {
	m := testDB(t)
	seedGoals(t, m)
	tab := specialKey(tea.KeyTab)
	initialCount := len(m.goals)

	r := asModel(send(m, tab, key('m'), key('d')))
	if len(r.goals) != initialCount-1 {
		t.Errorf("expected %d goals after delete, got %d", initialCount-1, len(r.goals))
	}
}

func TestTUIMatrixCompletionWithLinking(t *testing.T) {
	m := testDB(t)
	seedEntries(t, m) // entries on 2026-05-06
	seedGoals(t, m)   // goals on 2026-05-06
	tab := specialKey(tea.KeyTab)

	// Go to goals matrix, press x on Q1 goal -- should enter linking mode
	r := asModel(send(m, tab, key('m'), key('x')))
	if !r.linking {
		t.Fatal("should be in linking mode from matrix view")
	}

	// Select first entry with space, confirm with enter
	r = asModel(send(r, specialKey(tea.KeySpace), specialKey(tea.KeyEnter)))
	if r.linking {
		t.Error("should have exited linking mode")
	}

	// The goal should be completed and removed from active goals
	g, _ := GetGoalByID(m.db, 1)
	if !g.Completed {
		t.Error("goal should be completed")
	}
	if len(g.EntryIDs) == 0 {
		t.Error("goal should have linked entries")
	}
}

func TestTUIEnrichedEntriesShowContext(t *testing.T) {
	m := testDB(t)
	seedEntries(t, m)

	// Create a strategy and goal, link an entry to the goal
	sid, _ := InsertStrategy(m.db, "Platform v2", "rewrite", false, false)
	sidp := &sid
	InsertGoal(m.db, "2026-05-06", "Ship auth system", sidp, false, false)
	LinkGoalEntries(m.db, 1, []int64{1}) // link Entry A to goal 1
	m.loadEntries()

	// Check that enriched entry has goal/strategy context
	if len(m.enriched) == 0 {
		t.Fatal("no enriched entries")
	}

	found := false
	for _, ee := range m.enriched {
		if ee.GoalText != nil && *ee.GoalText == "Ship auth system" {
			found = true
			if ee.StrategyTitle == nil || *ee.StrategyTitle != "Platform v2" {
				t.Errorf("expected strategy title 'Platform v2', got %v", ee.StrategyTitle)
			}
		}
	}
	if !found {
		t.Error("no enriched entry found with goal context")
	}

	// View should render the context line
	view := m.View()
	if !strings.Contains(view, "Ship auth system") {
		t.Errorf("view should show goal text in entry context")
	}
	if !strings.Contains(view, "Platform v2") {
		t.Errorf("view should show strategy title in entry context")
	}
}

func TestTUIDayNavigation(t *testing.T) {
	m := testDB(t)
	// Seed entries on two dates
	InsertEntry(m.db, &Entry{Date: "2026-05-05", Time: "09:00", Category: "dev", Title: "Yesterday", Bullets: []string{"y"}, HoursEst: 1})
	InsertEntry(m.db, &Entry{Date: "2026-05-06", Time: "09:00", Category: "dev", Title: "Today", Bullets: []string{"t"}, HoursEst: 1})
	m.loadDates()
	m.loadEntries()

	if m.date != "2026-05-06" {
		t.Fatalf("should start on 2026-05-06, got %s", m.date)
	}

	// h goes to older date
	r := asModel(send(m, key('h')))
	if r.date != "2026-05-05" {
		t.Errorf("expected 2026-05-05 after h, got %s", r.date)
	}

	// l goes back to newer
	r = asModel(send(r, key('l')))
	if r.date != "2026-05-06" {
		t.Errorf("expected 2026-05-06 after l, got %s", r.date)
	}
}

func TestTUIMatrixDayNavigation(t *testing.T) {
	m := testDB(t)
	InsertEntry(m.db, &Entry{Date: "2026-05-05", Time: "09:00", Category: "dev", Title: "X", Bullets: []string{"x"}, HoursEst: 1})
	InsertEntry(m.db, &Entry{Date: "2026-05-06", Time: "09:00", Category: "dev", Title: "Y", Bullets: []string{"y"}, HoursEst: 1})
	InsertGoal(m.db, "2026-05-05", "Old goal", nil, true, false)
	InsertGoal(m.db, "2026-05-06", "Today goal", nil, false, true)
	m.loadDates()
	m.loadEntries()
	m.loadGoals()
	tab := specialKey(tea.KeyTab)

	// Go to goals matrix -- should show today's goals
	r := asModel(send(m, tab, key('m')))
	if r.date != "2026-05-06" {
		t.Fatalf("should start on 2026-05-06, got %s", r.date)
	}

	// h navigates to older day
	r = asModel(send(r, key('h')))
	if r.date != "2026-05-05" {
		t.Errorf("expected 2026-05-05 after h, got %s", r.date)
	}

	// Matrix should now show the old day's goals
	quads := partitionGoals(r.goals)
	found := false
	for _, q := range quads {
		for _, g := range q {
			if g.Text == "Old goal" {
				found = true
			}
		}
	}
	if !found {
		t.Error("matrix should show 'Old goal' after navigating to 2026-05-05")
	}
}

func TestTUIViewRendering(t *testing.T) {
	m := testDB(t)
	seedEntries(t, m)
	seedGoals(t, m)
	seedStrategies(t, m)
	tab := specialKey(tea.KeyTab)

	// Entries view should contain entry titles
	view := m.View()
	if !strings.Contains(view, "Entry A") {
		t.Error("entries view missing 'Entry A'")
	}
	if !strings.Contains(view, "entries") {
		t.Error("entries view missing tab label")
	}

	// Goals view
	r := asModel(send(m, tab))
	view = r.View()
	if !strings.Contains(view, "Fix the bug") {
		t.Error("goals view missing 'Fix the bug'")
	}

	// Goals matrix view
	r = asModel(send(r, key('m')))
	view = r.View()
	if !strings.Contains(view, "Do First") {
		t.Error("matrix view missing 'Do First' quadrant label")
	}
	if !strings.Contains(view, "Schedule") {
		t.Error("matrix view missing 'Schedule' quadrant label")
	}

	// Strategies view
	r = asModel(send(r, tab))
	view = r.View()
	if !strings.Contains(view, "Ship v2") {
		t.Error("strategies view missing 'Ship v2'")
	}

	// Strategy matrix
	r = asModel(send(r, key('m')))
	view = r.View()
	if !strings.Contains(view, "Do First") {
		t.Error("strategy matrix missing quadrant labels")
	}
}

func TestTUIWindowResize(t *testing.T) {
	m := testDB(t)
	r := asModel(send(m, tea.WindowSizeMsg{Width: 200, Height: 50}))
	if r.width != 200 || r.height != 50 {
		t.Errorf("expected 200x50, got %dx%d", r.width, r.height)
	}
}

// --- Full program integration tests ---
//
// These use tea.NewProgram with Program.Send() to inject messages directly.
// This bypasses the input reader (which can hang with non-terminal I/O)
// while still exercising the full BubbleTea event loop, Init, Update, and View.

func runProgram(t *testing.T, m tuiModel, keys ...tea.Msg) tuiModel {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	p := tea.NewProgram(m,
		tea.WithInput(strings.NewReader("")),
		tea.WithOutput(&bytes.Buffer{}),
		tea.WithContext(ctx),
		tea.WithoutSignals(),
		tea.WithoutRenderer(),
	)

	go func() {
		// Small delay to let the program start its event loop
		time.Sleep(50 * time.Millisecond)
		for _, k := range keys {
			p.Send(k)
		}
		p.Send(tea.QuitMsg{})
	}()

	finalModel, err := p.Run()
	if err != nil {
		t.Fatalf("program error: %v", err)
	}
	return finalModel.(tuiModel)
}

func TestFullProgramTabCycleAndQuit(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	m := newTUIModel(db, "2026-05-06")

	// tab tab tab cycles all views
	fm := runProgram(t, m,
		specialKey(tea.KeyTab),
		specialKey(tea.KeyTab),
		specialKey(tea.KeyTab),
	)

	// After 3 tabs we should be back to entries
	if fm.view != viewEntries {
		t.Errorf("expected viewEntries after full cycle, got %d", fm.view)
	}
}

func TestFullProgramEntryDeletion(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	InsertEntry(db, &Entry{Date: "2026-05-06", Time: "09:00", Category: "dev", Title: "Temp", Bullets: []string{"x"}, HoursEst: 1})
	InsertEntry(db, &Entry{Date: "2026-05-06", Time: "10:00", Category: "dev", Title: "Keep", Bullets: []string{"y"}, HoursEst: 1})

	m := newTUIModel(db, "2026-05-06")
	fm := runProgram(t, m, key('d'))

	if len(fm.entries) != 1 {
		t.Errorf("expected 1 entry after delete, got %d", len(fm.entries))
	}
	if fm.entries[0].Title != "Keep" {
		t.Errorf("wrong entry remained: %s", fm.entries[0].Title)
	}
}

func TestFullProgramGoalPriorityToggle(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	InsertGoal(db, "2026-05-06", "Test goal", nil, false, false)
	m := newTUIModel(db, "2026-05-06")

	// tab to goals, toggle important, toggle urgent
	fm := runProgram(t, m,
		specialKey(tea.KeyTab),
		key('i'),
		key('u'),
	)

	if !fm.goals[0].Important {
		t.Error("goal should be important after toggle")
	}
	if !fm.goals[0].Urgent {
		t.Error("goal should be urgent after toggle")
	}

	// Verify persisted to DB
	g, _ := GetGoalByID(db, fm.goals[0].ID)
	if !g.Important || !g.Urgent {
		t.Error("priority not persisted to database")
	}
}

func TestFullProgramMatrixViewSwitch(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := OpenDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	InsertGoal(db, "2026-05-06", "Q1 goal", nil, true, true)
	InsertGoal(db, "2026-05-06", "Q4 goal", nil, false, false)

	m := newTUIModel(db, "2026-05-06")

	// tab to goals, m to matrix
	fm := runProgram(t, m,
		specialKey(tea.KeyTab),
		key('m'),
	)

	if fm.view != viewGoalsMatrix {
		t.Errorf("expected viewGoalsMatrix, got %d", fm.view)
	}
}
