package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
)

func logViaServer(port int, e *Entry) (string, error) {
	body, _ := json.Marshal(e)
	url := fmt.Sprintf("http://127.0.0.1:%d/entries", port)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("server returned %d", resp.StatusCode)
	}
	out, _ := io.ReadAll(resp.Body)
	return strings.TrimSpace(string(out)), nil
}

func cmdServe(args []string) error {
	fs := flag.NewFlagSet("hrs serve", flag.ExitOnError)
	port := fs.Int("port", 9746, "listen port")
	dbPath := fs.String("db", DefaultDB(), "sqlite database path")
	logDir := fs.String("dir", DefaultDir(), "markdown output directory")
	fs.Parse(args)

	db, err := OpenDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	s := &Server{db: db, logDir: *logDir}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /schema", s.Schema)
	mux.HandleFunc("GET /health", s.Health)
	mux.HandleFunc("GET /entries", s.ListEntries)
	mux.HandleFunc("POST /entries", s.CreateEntry)
	mux.HandleFunc("PUT /entries/{id}", s.UpdateEntryHandler)
	mux.HandleFunc("DELETE /entries/{id}", s.DeleteEntry)
	mux.HandleFunc("GET /goals", s.ListGoals)
	mux.HandleFunc("GET /goals/active", s.ListActiveGoals)
	mux.HandleFunc("POST /goals", s.CreateGoal)
	mux.HandleFunc("PUT /goals/{id}", s.UpdateGoalHandler)
	mux.HandleFunc("PUT /goals/{id}/done", s.CompleteGoalHandler)
	mux.HandleFunc("PUT /goals/{id}/undo", s.UncompleteGoalHandler)
	mux.HandleFunc("POST /goals/{id}/link", s.LinkGoalEntriesHandler)
	mux.HandleFunc("DELETE /goals/{id}", s.DeleteGoalHandler)
	mux.HandleFunc("GET /strategies", s.ListStrategies)
	mux.HandleFunc("POST /strategies", s.CreateStrategy)
	mux.HandleFunc("GET /strategies/{id}", s.GetStrategyReportHandler)
	mux.HandleFunc("PUT /strategies/{id}", s.UpdateStrategyStatusHandler)
	mux.HandleFunc("DELETE /strategies/{id}", s.DeleteStrategyHandler)
	mux.HandleFunc("GET /docs/", http.StripPrefix("/docs", http.HandlerFunc(docsHandler)).ServeHTTP)
	mux.HandleFunc("GET /docs", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/docs/", http.StatusMovedPermanently)
	})

	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	srv := &http.Server{Addr: addr, Handler: mux}

	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		srv.Close()
	}()

	fmt.Fprintf(os.Stderr, "hrs listening on http://%s\n", addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

func cmdLog(args []string) error {
	fs := flag.NewFlagSet("hrs log", flag.ExitOnError)
	dbPath := fs.String("db", DefaultDB(), "sqlite database path")
	logDir := fs.String("dir", DefaultDir(), "markdown output directory")
	port := fs.Int("port", 9746, "server port to try before direct DB write")
	category := fs.String("c", "", "category (e.g. dev, admin, security)")
	title := fs.String("t", "", "title")
	bullets := fs.String("b", "", "bullets (semicolon-separated)")
	hours := fs.Float64("e", 0, "estimated person-hours")
	date := fs.String("d", "", "date (YYYY-MM-DD, default: today)")
	timeFlag := fs.String("T", "", "time (HH:MM, default: now)")
	fs.Parse(args)

	if *category == "" || *title == "" || *bullets == "" {
		return fmt.Errorf("required: -c category -t title -b bullets")
	}

	parts := strings.Split(*bullets, ";")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}

	e := &Entry{
		Date:     *date,
		Time:     *timeFlag,
		Category: *category,
		Title:    *title,
		Bullets:  parts,
		HoursEst: *hours,
	}

	// Try the server first — avoids SQLite lock contention with concurrent agents
	if resp, err := logViaServer(*port, e); err == nil {
		fmt.Println(resp)
		return nil
	}

	// Fall back to direct DB write
	os.MkdirAll(*logDir, 0755)
	db, err := OpenDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	id, err := InsertEntry(db, e)
	if err != nil {
		return err
	}

	// Sync markdown
	entries, _ := GetEntries(db, e.Date)
	if md := RenderMarkdown(entries); md != "" {
		os.WriteFile(filepath.Join(*logDir, e.Date+".md"), []byte(md), 0644)
	}

	out, _ := json.Marshal(map[string]any{"id": id, "date": e.Date})
	fmt.Println(string(out))
	return nil
}

func cmdLs(args []string) error {
	fs := flag.NewFlagSet("hrs ls", flag.ExitOnError)
	dbPath := fs.String("db", DefaultDB(), "sqlite database path")
	format := fs.String("format", "md", "output format (md|json)")
	from := fs.String("from", "", "start date (YYYY-MM-DD)")
	to := fs.String("to", "", "end date (YYYY-MM-DD)")
	category := fs.String("category", "", "filter by category")
	ticket := fs.String("ticket", "", "filter by goal/strategy ticket prefix (e.g. PROMO)")
	fs.Parse(args)

	date := now().Format("2006-01-02")
	if fs.NArg() > 0 {
		date = fs.Arg(0)
	}

	db, err := OpenDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	isTTY := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())

	// Ticket filter implies a date range query (a single day rarely contains
	// the work you want for an R&D claim).
	if *ticket != "" {
		if *from == "" {
			*from = "2000-01-01"
		}
		if *to == "" {
			*to = now().Format("2006-01-02")
		}
		entries, err := GetEntriesByTicket(db, *from, *to, ticketLike(*ticket), *category)
		if err != nil {
			return err
		}
		if *format == "json" {
			if entries == nil {
				entries = []Entry{}
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(entries)
		}
		if len(entries) == 0 {
			fmt.Printf("No entries matching ticket %q from %s to %s\n", *ticket, *from, *to)
			return nil
		}
		groups := groupByDate(entries)
		for i, g := range groups {
			if i > 0 {
				fmt.Println()
			}
			if isTTY {
				renderColorLs(g)
			} else {
				fmt.Print(RenderMarkdown(g))
			}
		}
		return nil
	}

	// For single-date TTY display, use enriched entries to show goal/strategy context
	if *from == "" && isTTY {
		enriched, err := GetEnrichedEntries(db, date)
		if err != nil {
			return err
		}
		if len(enriched) == 0 {
			fmt.Printf("No entries for %s\n", date)
			return nil
		}
		renderColorLsEnriched(enriched)
		return nil
	}

	var entries []Entry
	if *from != "" {
		if *to == "" {
			*to = now().Format("2006-01-02")
		}
		entries, err = GetEntriesRange(db, *from, *to, *category)
	} else {
		entries, err = GetEntries(db, date)
	}
	if err != nil {
		return err
	}

	if *format == "json" {
		if entries == nil {
			entries = []Entry{}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}

	if len(entries) == 0 {
		if *from != "" {
			fmt.Printf("No entries from %s to %s\n", *from, *to)
		} else {
			fmt.Printf("No entries for %s\n", date)
		}
		return nil
	}

	// Group by date for range queries, render each group
	groups := groupByDate(entries)
	for i, g := range groups {
		if i > 0 {
			fmt.Println()
		}
		if isTTY {
			renderColorLs(g)
		} else {
			fmt.Print(RenderMarkdown(g))
		}
	}
	return nil
}

func groupByDate(entries []Entry) [][]Entry {
	var groups [][]Entry
	var cur []Entry
	var curDate string
	for _, e := range entries {
		if e.Date != curDate {
			if len(cur) > 0 {
				groups = append(groups, cur)
			}
			cur = nil
			curDate = e.Date
		}
		cur = append(cur, e)
	}
	if len(cur) > 0 {
		groups = append(groups, cur)
	}
	return groups
}

func renderColorLs(entries []Entry) {
	hdrStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	catSt := lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	titleSt := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	hoursSt := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	bulletSt := lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	sumSt := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)

	fmt.Println(hdrStyle.Render(fmt.Sprintf("# Worklog — %s", entries[0].Date)))
	fmt.Println()
	var total float64
	for _, e := range entries {
		total += e.HoursEst
		hours := ""
		if e.HoursEst > 0 {
			hours = hoursSt.Render(fmt.Sprintf(" (~%gh)", e.HoursEst))
		}
		fmt.Printf("%s %s%s\n", catSt.Render(fmt.Sprintf("[%s]", e.Category)), titleSt.Render(e.Title), hours)
		for _, bullet := range e.Bullets {
			fmt.Printf("  %s\n", bulletSt.Render("- "+bullet))
		}
		fmt.Println()
	}
	fmt.Println(sumSt.Render(fmt.Sprintf("%d entries  ~%gh  %.1fd", len(entries), total, total/8)))
}

func renderColorLsEnriched(entries []EnrichedEntry) {
	hdrStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	catSt := lipgloss.NewStyle().Foreground(lipgloss.Color("5"))
	titleSt := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
	hoursSt := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	bulletSt := lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	sumSt := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
	ctxSt := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)

	fmt.Println(hdrStyle.Render(fmt.Sprintf("# Worklog — %s", entries[0].Date)))
	fmt.Println()
	var total float64
	for _, e := range entries {
		total += e.HoursEst
		hours := ""
		if e.HoursEst > 0 {
			hours = hoursSt.Render(fmt.Sprintf(" (~%gh)", e.HoursEst))
		}
		fmt.Printf("%s %s%s\n", catSt.Render(fmt.Sprintf("[%s]", e.Category)), titleSt.Render(e.Title), hours)
		for _, bullet := range e.Bullets {
			fmt.Printf("  %s\n", bulletSt.Render("- "+bullet))
		}
		if e.GoalText != nil {
			ctx := fmt.Sprintf("  -> %q", *e.GoalText)
			if e.StrategyTitle != nil {
				ctx += fmt.Sprintf(" | s: %s", *e.StrategyTitle)
			}
			if t := entryTicket(e); t != "" {
				ctx += fmt.Sprintf(" [%s]", t)
			}
			fmt.Println(ctxSt.Render(ctx))
		}
		fmt.Println()
	}
	fmt.Println(sumSt.Render(fmt.Sprintf("%d entries  ~%gh  %.1fd", len(entries), total, total/8)))
}

func cmdTUI(args []string) error {
	fs := flag.NewFlagSet("hrs tui", flag.ExitOnError)
	dbPath := fs.String("db", DefaultDB(), "sqlite database path")
	fs.Parse(args)

	date := now().Format("2006-01-02")
	if fs.NArg() > 0 {
		date = fs.Arg(0)
	}

	db, err := OpenDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	m := newTUIModel(db, date)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

func cmdDocs(args []string) error {
	fs := flag.NewFlagSet("hrs docs", flag.ExitOnError)
	port := fs.Int("port", 9747, "listen port")
	fs.Parse(args)

	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	srv := &http.Server{Addr: addr, Handler: http.HandlerFunc(docsHandler)}

	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		srv.Close()
	}()

	fmt.Fprintf(os.Stderr, "docs at http://%s\n", addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

func cmdMigrate(args []string) error {
	fs := flag.NewFlagSet("hrs migrate", flag.ExitOnError)
	dbPath := fs.String("db", DefaultDB(), "sqlite database path")
	logDir := fs.String("dir", DefaultDir(), "directory with existing markdown files")
	fs.Parse(args)

	db, err := OpenDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	n, err := Migrate(db, *logDir)
	if err != nil {
		return err
	}
	fmt.Printf("imported %d entries\n", n)
	return nil
}

func cmdRm(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: hrs rm <id>")
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid id: %s", args[0])
	}

	fs := flag.NewFlagSet("hrs rm", flag.ExitOnError)
	dbPath := fs.String("db", DefaultDB(), "sqlite database path")
	fs.Parse(args[1:])

	db, err := OpenDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = DeleteEntryByID(db, id)
	if err != nil {
		return err
	}

	out, _ := json.Marshal(map[string]any{"deleted": id})
	fmt.Println(string(out))
	return nil
}

func cmdEdit(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: hrs edit <id> [flags]")
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid id: %s", args[0])
	}

	fs := flag.NewFlagSet("hrs edit", flag.ExitOnError)
	dbPath := fs.String("db", DefaultDB(), "sqlite database path")
	category := fs.String("c", "", "category")
	title := fs.String("t", "", "title")
	bullets := fs.String("b", "", "bullets (semicolon-separated)")
	hours := fs.Float64("e", -1, "estimated person-hours")
	date := fs.String("d", "", "date (YYYY-MM-DD)")
	timeFlag := fs.String("T", "", "time (HH:MM)")
	fs.Parse(args[1:])

	db, err := OpenDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	e, err := GetEntryByID(db, id)
	if err != nil {
		return fmt.Errorf("entry %d not found", id)
	}

	// Merge provided flags over existing values
	if *category != "" {
		e.Category = *category
	}
	if *title != "" {
		e.Title = *title
	}
	if *bullets != "" {
		parts := strings.Split(*bullets, ";")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		e.Bullets = parts
	}
	if *hours >= 0 {
		e.HoursEst = *hours
	}
	if *date != "" {
		e.Date = *date
	}
	if *timeFlag != "" {
		e.Time = *timeFlag
	}

	if err := UpdateEntry(db, id, e); err != nil {
		return err
	}

	out, _ := json.Marshal(e)
	fmt.Println(string(out))
	return nil
}

func cmdExport(args []string) error {
	fs := flag.NewFlagSet("hrs export", flag.ExitOnError)
	dbPath := fs.String("db", DefaultDB(), "sqlite database path")
	from := fs.String("from", "", "start date (YYYY-MM-DD)")
	to := fs.String("to", "", "end date (YYYY-MM-DD)")
	category := fs.String("category", "", "filter by category")
	ticket := fs.String("ticket", "", "filter by goal/strategy ticket prefix (e.g. PROMO)")
	format := fs.String("format", "json", "output format (json|csv)")
	fs.Parse(args)

	if *from == "" {
		*from = "2000-01-01"
	}
	if *to == "" {
		*to = "2099-12-31"
	}

	db, err := OpenDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	var entries []Entry
	if *ticket != "" {
		entries, err = GetEntriesByTicket(db, *from, *to, ticketLike(*ticket), *category)
	} else {
		entries, err = GetEntriesRange(db, *from, *to, *category)
	}
	if err != nil {
		return err
	}
	if entries == nil {
		entries = []Entry{}
	}

	switch *format {
	case "csv":
		fmt.Println("id,date,time,category,title,bullets,hours_est")
		for _, e := range entries {
			bulletStr := strings.Join(e.Bullets, ";")
			// Quote fields that might contain commas
			fmt.Printf("%d,%s,%s,%s,%q,%q,%g\n",
				e.ID, e.Date, e.Time, e.Category,
				e.Title, bulletStr, e.HoursEst)
		}
	default:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}
	return nil
}

func cmdGoals(args []string) error {
	action := ""
	if len(args) > 0 {
		switch args[0] {
		case "add", "done", "undo", "rm", "link", "edit":
			action = args[0]
			args = args[1:]
		}
	}

	switch action {
	case "add":
		return cmdGoalsAdd(args)
	case "done":
		return cmdGoalsDone(args)
	case "undo":
		return cmdGoalsUndo(args)
	case "rm":
		return cmdGoalsRm(args)
	case "link":
		return cmdGoalsLink(args)
	case "edit":
		return cmdGoalsEdit(args)
	default:
		return cmdGoalsList(args)
	}
}

const goalsUsage = `hrs goals - manage daily goals

usage:
  hrs goals [-d date]                       list goals (default: today)
  hrs goals add "goal text" [-d ..] [-i -u] add a goal
  hrs goals edit <id> [-t text] [-i -u]     edit a goal
  hrs goals done <id> [-e entry_ids]        mark a goal complete
  hrs goals undo <id>                       reopen a completed goal
  hrs goals rm <id>                         delete a goal
  hrs goals link <id> -e entry_ids          link entries to a goal
`

func cmdGoalsList(args []string) error {
	fs := flag.NewFlagSet("hrs goals", flag.ExitOnError)
	dbPath := fs.String("db", DefaultDB(), "sqlite database path")
	date := fs.String("d", "", "date (YYYY-MM-DD, default: today)")
	format := fs.String("format", "", "output format (json for machine-readable)")
	fs.Parse(args)

	if *date == "" {
		*date = now().Format("2006-01-02")
	}

	db, err := OpenDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	goals, err := GetGoals(db, *date)
	if err != nil {
		return err
	}

	if *format == "json" {
		if goals == nil {
			goals = []Goal{}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(goals)
	}

	if len(goals) == 0 {
		fmt.Printf("No goals for %s\n", *date)
		return nil
	}

	isTTY := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())
	done := 0
	for _, g := range goals {
		if g.Completed {
			done++
		}
	}

	if isTTY {
		hdr := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
		dim := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		check := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
		open := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
		impSt := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1"))
		urgSt := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
		fmt.Println(hdr.Render(fmt.Sprintf("Goals — %s  (%d/%d)", *date, done, len(goals))))
		fmt.Println()
		for _, g := range goals {
			stratTag := ""
			if g.StrategyID != nil {
				stratTag = dim.Render(fmt.Sprintf(" s#%d", *g.StrategyID))
			}
			prio := ""
			if g.Important {
				prio += impSt.Render("!")
			}
			if g.Urgent {
				prio += urgSt.Render("^")
			}
			if prio != "" {
				prio += " "
			}
			if g.Completed {
				fmt.Printf("  %s %s%s", check.Render("[x]"), prio, dim.Render(g.Text))
				if len(g.EntryIDs) > 0 {
					fmt.Printf(" %s", dim.Render(fmt.Sprintf("(entries: %s)", formatIDs(g.EntryIDs))))
				}
				fmt.Printf("%s\n", stratTag)
			} else {
				fmt.Printf("  %s %s%s %s%s\n", open.Render("[ ]"), prio, g.Text, dim.Render(fmt.Sprintf("#%d", g.ID)), stratTag)
			}
		}
	} else {
		fmt.Printf("Goals — %s  (%d/%d)\n\n", *date, done, len(goals))
		for _, g := range goals {
			mark := "[ ]"
			if g.Completed {
				mark = "[x]"
			}
			fmt.Printf("  %s %s (#%d)\n", mark, g.Text, g.ID)
		}
	}
	return nil
}

func formatIDs(ids []int64) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.FormatInt(id, 10)
	}
	return strings.Join(parts, ",")
}

func cmdGoalsAdd(args []string) error {
	fs := flag.NewFlagSet("hrs goals add", flag.ExitOnError)
	dbPath := fs.String("db", DefaultDB(), "sqlite database path")
	date := fs.String("d", "", "date (YYYY-MM-DD, default: today)")
	strategyFlag := fs.Int64("s", 0, "link to strategy ID")
	importantFlag := fs.Bool("i", false, "mark as important")
	urgentFlag := fs.Bool("u", false, "mark as urgent")
	ticketFlag := fs.String("ticket", "", "ticket reference (e.g. PROMO-123, ENG-456, GH-org/repo#12)")
	fs.Parse(args)

	text := strings.Join(fs.Args(), " ")
	if text == "" {
		return fmt.Errorf("usage: hrs goals add \"goal text\"")
	}
	if *date == "" {
		*date = now().Format("2006-01-02")
	}

	db, err := OpenDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	var sid *int64
	if *strategyFlag != 0 {
		sid = strategyFlag
	}

	var ticket *string
	if *ticketFlag != "" {
		ticket = ticketFlag
	}

	id, err := InsertGoal(db, *date, text, sid, *importantFlag, *urgentFlag, ticket)
	if err != nil {
		return err
	}
	resp := map[string]any{"id": id, "date": *date, "text": text}
	if sid != nil {
		resp["strategy_id"] = *sid
	}
	if ticket != nil {
		resp["ticket_ref"] = *ticket
	}
	out, _ := json.Marshal(resp)
	fmt.Println(string(out))
	return nil
}

func cmdGoalsDone(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: hrs goals done <id> [-e entry_ids]")
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid goal id: %s", args[0])
	}

	fs := flag.NewFlagSet("hrs goals done", flag.ExitOnError)
	dbPath := fs.String("db", DefaultDB(), "sqlite database path")
	entryFlag := fs.String("e", "", "linked entry IDs (comma-separated)")
	fs.Parse(args[1:])

	db, err := OpenDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	var entryIDs []int64
	if *entryFlag != "" {
		for _, s := range strings.Split(*entryFlag, ",") {
			eid, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
			if err != nil {
				return fmt.Errorf("invalid entry id: %s", s)
			}
			entryIDs = append(entryIDs, eid)
		}
	}

	if err := CompleteGoal(db, id, entryIDs); err != nil {
		return err
	}
	out, _ := json.Marshal(map[string]any{"completed": id, "entry_ids": entryIDs})
	fmt.Println(string(out))
	return nil
}

func cmdGoalsUndo(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: hrs goals undo <id>")
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid goal id: %s", args[0])
	}

	fs := flag.NewFlagSet("hrs goals undo", flag.ExitOnError)
	dbPath := fs.String("db", DefaultDB(), "sqlite database path")
	fs.Parse(args[1:])

	db, err := OpenDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := UncompleteGoal(db, id); err != nil {
		return err
	}
	out, _ := json.Marshal(map[string]any{"reopened": id})
	fmt.Println(string(out))
	return nil
}

func cmdGoalsRm(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: hrs goals rm <id>")
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid goal id: %s", args[0])
	}

	fs := flag.NewFlagSet("hrs goals rm", flag.ExitOnError)
	dbPath := fs.String("db", DefaultDB(), "sqlite database path")
	fs.Parse(args[1:])

	db, err := OpenDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := DeleteGoal(db, id); err != nil {
		return err
	}
	out, _ := json.Marshal(map[string]any{"deleted": id})
	fmt.Println(string(out))
	return nil
}

func cmdGoalsLink(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: hrs goals link <id> -e entry_ids")
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid goal id: %s", args[0])
	}

	fs := flag.NewFlagSet("hrs goals link", flag.ExitOnError)
	dbPath := fs.String("db", DefaultDB(), "sqlite database path")
	entryFlag := fs.String("e", "", "entry IDs to link (comma-separated)")
	fs.Parse(args[1:])

	if *entryFlag == "" {
		return fmt.Errorf("required: -e entry_ids")
	}

	db, err := OpenDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	var entryIDs []int64
	for _, s := range strings.Split(*entryFlag, ",") {
		eid, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		if err != nil {
			return fmt.Errorf("invalid entry id: %s", s)
		}
		entryIDs = append(entryIDs, eid)
	}

	if err := LinkGoalEntries(db, id, entryIDs); err != nil {
		return err
	}
	out, _ := json.Marshal(map[string]any{"goal_id": id, "linked_entries": entryIDs})
	fmt.Println(string(out))
	return nil
}

func cmdGoalsEdit(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: hrs goals edit <id> [-t text] [-s strategy_id] [-i] [-u]")
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid goal id: %s", args[0])
	}

	fs := flag.NewFlagSet("hrs goals edit", flag.ExitOnError)
	dbPath := fs.String("db", DefaultDB(), "sqlite database path")
	text := fs.String("t", "", "text")
	strategyFlag := fs.Int64("s", -1, "link to strategy ID (0 to unlink)")
	importantFlag := fs.String("i", "", "important (true|false)")
	urgentFlag := fs.String("u", "", "urgent (true|false)")
	ticketFlag := fs.String("ticket", "", "ticket reference (empty string clears)")
	fs.Parse(args[1:])

	db, err := OpenDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	g, err := GetGoalByID(db, id)
	if err != nil {
		return fmt.Errorf("goal %d not found", id)
	}

	if *text != "" {
		g.Text = *text
	}
	if *strategyFlag == 0 {
		g.StrategyID = nil
	} else if *strategyFlag > 0 {
		g.StrategyID = strategyFlag
	}
	if *importantFlag != "" {
		g.Important = *importantFlag == "true" || *importantFlag == "1"
	}
	if *urgentFlag != "" {
		g.Urgent = *urgentFlag == "true" || *urgentFlag == "1"
	}
	if ticketWasSet(fs, "ticket") {
		if *ticketFlag == "" {
			g.TicketRef = nil
		} else {
			v := *ticketFlag
			g.TicketRef = &v
		}
	}

	if err := UpdateGoal(db, id, g.Text, g.StrategyID, g.Important, g.Urgent, g.TicketRef); err != nil {
		return err
	}
	out, _ := json.Marshal(g)
	fmt.Println(string(out))
	return nil
}

// ticketLike normalises a user-supplied ticket filter into a SQL LIKE pattern.
// "PROMO"     -> "PROMO%"     (prefix match — common case)
// "%PROMO%"   -> "%PROMO%"    (explicit wildcards pass through)
// "PROMO-123" -> "PROMO-123%" (still a prefix; tickets with longer suffixes match)
func ticketLike(s string) string {
	if strings.ContainsAny(s, "%_") {
		return s
	}
	return s + "%"
}

// ticketWasSet reports whether the named flag was explicitly passed on the
// command line. Used to distinguish "not provided" from "provided as empty".
func ticketWasSet(fs *flag.FlagSet, name string) bool {
	set := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})
	return set
}

func cmdStrategy(args []string) error {
	action := ""
	if len(args) > 0 {
		switch args[0] {
		case "add", "done", "archive", "reopen", "rm", "edit", "report":
			action = args[0]
			args = args[1:]
		}
	}

	switch action {
	case "add":
		return cmdStrategyAdd(args)
	case "done":
		return cmdStrategyStatus(args, "completed")
	case "archive":
		return cmdStrategyStatus(args, "archived")
	case "reopen":
		return cmdStrategyStatus(args, "active")
	case "rm":
		return cmdStrategyRm(args)
	case "edit":
		return cmdStrategyEdit(args)
	case "report":
		return cmdStrategyReport(args)
	default:
		// Default: if arg looks like an ID, show report; otherwise list
		if len(args) > 0 {
			if _, err := strconv.ParseInt(args[0], 10, 64); err == nil {
				return cmdStrategyReport(args)
			}
		}
		return cmdStrategyList(args)
	}
}

func cmdStrategyList(args []string) error {
	fs := flag.NewFlagSet("hrs strategy", flag.ExitOnError)
	dbPath := fs.String("db", DefaultDB(), "sqlite database path")
	status := fs.String("status", "", "filter by status (active|completed|archived)")
	format := fs.String("format", "", "output format (json for machine-readable)")
	fs.Parse(args)

	db, err := OpenDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	strategies, err := GetStrategies(db, *status)
	if err != nil {
		return err
	}

	if *format == "json" {
		if strategies == nil {
			strategies = []Strategy{}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(strategies)
	}

	if len(strategies) == 0 {
		fmt.Println("No strategies found.")
		return nil
	}

	isTTY := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())

	if isTTY {
		hdr := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
		dim := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		activeSt := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
		completedSt := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		archivedSt := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
		impSt := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("1"))
		urgSt := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
		fmt.Println(hdr.Render("Strategies"))
		fmt.Println()
		for _, s := range strategies {
			var st lipgloss.Style
			switch s.Status {
			case "completed":
				st = completedSt
			case "archived":
				st = archivedSt
			default:
				st = activeSt
			}
			badge := st.Render(fmt.Sprintf("[%s]", s.Status))
			prio := ""
			if s.Important {
				prio += impSt.Render("!")
			}
			if s.Urgent {
				prio += urgSt.Render("^")
			}
			if prio != "" {
				prio += " "
			}
			fmt.Printf("  %s %s %s%s", dim.Render(fmt.Sprintf("#%d", s.ID)), badge, prio, s.Title)
			if s.Description != "" {
				fmt.Printf(" %s", dim.Render("- "+s.Description))
			}
			fmt.Println()
		}
	} else {
		for _, s := range strategies {
			fmt.Printf("#%d [%s] %s\n", s.ID, s.Status, s.Title)
		}
	}
	return nil
}

func cmdStrategyAdd(args []string) error {
	fs := flag.NewFlagSet("hrs strategy add", flag.ExitOnError)
	dbPath := fs.String("db", DefaultDB(), "sqlite database path")
	title := fs.String("t", "", "title")
	desc := fs.String("desc", "", "description")
	importantFlag := fs.Bool("i", false, "mark as important")
	urgentFlag := fs.Bool("u", false, "mark as urgent")
	ticketFlag := fs.String("ticket", "", "ticket reference (e.g. PROMO-123, ENG-456, GH-org/repo#12)")
	fs.Parse(args)

	// Allow title as positional arg or -t flag
	if *title == "" {
		*title = strings.Join(fs.Args(), " ")
	}
	if *title == "" {
		return fmt.Errorf("usage: hrs strategy add -t \"title\" [-desc \"description\"]")
	}

	db, err := OpenDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	var ticket *string
	if *ticketFlag != "" {
		ticket = ticketFlag
	}

	id, err := InsertStrategy(db, *title, *desc, *importantFlag, *urgentFlag, ticket)
	if err != nil {
		return err
	}
	resp := map[string]any{"id": id, "title": *title}
	if ticket != nil {
		resp["ticket_ref"] = *ticket
	}
	out, _ := json.Marshal(resp)
	fmt.Println(string(out))
	return nil
}

func cmdStrategyStatus(args []string, status string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: hrs strategy %s <id>", status)
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid strategy id: %s", args[0])
	}

	fs := flag.NewFlagSet("hrs strategy "+status, flag.ExitOnError)
	dbPath := fs.String("db", DefaultDB(), "sqlite database path")
	fs.Parse(args[1:])

	db, err := OpenDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := UpdateStrategyStatus(db, id, status); err != nil {
		return err
	}
	out, _ := json.Marshal(map[string]any{"id": id, "status": status})
	fmt.Println(string(out))
	return nil
}

func cmdStrategyEdit(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: hrs strategy edit <id> [-t title] [-desc description] [-i true|false] [-u true|false]")
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid strategy id: %s", args[0])
	}

	fs := flag.NewFlagSet("hrs strategy edit", flag.ExitOnError)
	dbPath := fs.String("db", DefaultDB(), "sqlite database path")
	title := fs.String("t", "", "title")
	desc := fs.String("desc", "", "description")
	importantFlag := fs.String("i", "", "important (true|false)")
	urgentFlag := fs.String("u", "", "urgent (true|false)")
	ticketFlag := fs.String("ticket", "", "ticket reference (empty string clears)")
	fs.Parse(args[1:])

	db, err := OpenDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	s, err := GetStrategyByID(db, id)
	if err != nil {
		return fmt.Errorf("strategy %d not found", id)
	}
	if *title != "" {
		s.Title = *title
	}
	if *desc != "" {
		s.Description = *desc
	}
	if *importantFlag != "" {
		s.Important = *importantFlag == "true" || *importantFlag == "1"
	}
	if *urgentFlag != "" {
		s.Urgent = *urgentFlag == "true" || *urgentFlag == "1"
	}
	if ticketWasSet(fs, "ticket") {
		if *ticketFlag == "" {
			s.TicketRef = nil
		} else {
			v := *ticketFlag
			s.TicketRef = &v
		}
	}
	if err := UpdateStrategy(db, id, s.Title, s.Description, s.Important, s.Urgent, s.TicketRef); err != nil {
		return err
	}
	out, _ := json.Marshal(s)
	fmt.Println(string(out))
	return nil
}

func cmdStrategyRm(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: hrs strategy rm <id>")
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid strategy id: %s", args[0])
	}

	fs := flag.NewFlagSet("hrs strategy rm", flag.ExitOnError)
	dbPath := fs.String("db", DefaultDB(), "sqlite database path")
	fs.Parse(args[1:])

	db, err := OpenDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := DeleteStrategy(db, id); err != nil {
		return err
	}
	out, _ := json.Marshal(map[string]any{"deleted": id})
	fmt.Println(string(out))
	return nil
}

func cmdStrategyReport(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: hrs strategy report <id>")
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("invalid strategy id: %s", args[0])
	}

	fs := flag.NewFlagSet("hrs strategy report", flag.ExitOnError)
	dbPath := fs.String("db", DefaultDB(), "sqlite database path")
	format := fs.String("format", "", "output format (json for machine-readable)")
	fs.Parse(args[1:])

	db, err := OpenDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	r, err := GetStrategyReport(db, id)
	if err != nil {
		return err
	}

	if *format == "json" {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(r)
	}

	goals, _ := GetStrategyGoals(db, id)

	isTTY := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())

	if isTTY {
		hdr := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6"))
		dim := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
		activeSt := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
		check := lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
		open := lipgloss.NewStyle().Foreground(lipgloss.Color("3"))

		badge := activeSt.Render(fmt.Sprintf("[%s]", r.Status))
		fmt.Printf("%s %s\n", hdr.Render(fmt.Sprintf("Strategy #%d: %s", r.ID, r.Title)), badge)
		if r.Description != "" {
			fmt.Printf("%s\n", dim.Render(r.Description))
		}
		fmt.Printf("%s\n\n", dim.Render("Created: "+r.CreatedAt))

		fmt.Printf("  Goals:  %d/%d done\n", r.GoalsDone, r.GoalsTotal)
		fmt.Printf("  Hours:  %.1fh (%.1fd)\n\n", r.TotalHours, r.TotalHours/8)

		if len(goals) > 0 {
			fmt.Println(hdr.Render("  Linked goals:"))
			for _, g := range goals {
				if g.Completed {
					fmt.Printf("    %s %s %s\n", check.Render("[x]"), dim.Render(g.Text), dim.Render(g.Date))
				} else {
					fmt.Printf("    %s %s %s\n", open.Render("[ ]"), g.Text, dim.Render(g.Date))
				}
			}
		}
	} else {
		fmt.Printf("Strategy #%d: %s [%s]\n", r.ID, r.Title, r.Status)
		if r.Description != "" {
			fmt.Printf("%s\n", r.Description)
		}
		fmt.Printf("Created: %s\n\n", r.CreatedAt)
		fmt.Printf("Goals: %d/%d done\n", r.GoalsDone, r.GoalsTotal)
		fmt.Printf("Hours: %.1fh (%.1fd)\n\n", r.TotalHours, r.TotalHours/8)
		for _, g := range goals {
			mark := "[ ]"
			if g.Completed {
				mark = "[x]"
			}
			fmt.Printf("  %s %s (%s)\n", mark, g.Text, g.Date)
		}
	}
	return nil
}

func cmdCategories(args []string) error {
	fs := flag.NewFlagSet("hrs categories", flag.ExitOnError)
	dbPath := fs.String("db", DefaultDB(), "sqlite database path")
	fs.Parse(args)

	db, err := OpenDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	cats, err := GetCategories(db)
	if err != nil {
		return err
	}
	for _, c := range cats {
		fmt.Println(c)
	}
	return nil
}
