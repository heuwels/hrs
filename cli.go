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

	isTTY := isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd())

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

	entries, err := GetEntriesRange(db, *from, *to, *category)
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
