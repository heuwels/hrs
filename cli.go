package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
)

func cmdServe(args []string) error {
	fs := flag.NewFlagSet("hrs serve", flag.ExitOnError)
	port := fs.Int("port", 9746, "listen port")
	dbPath := fs.String("db", "hrs.db", "sqlite database path")
	logDir := fs.String("dir", ".", "markdown output directory")
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
	dbPath := fs.String("db", "hrs.db", "sqlite database path")
	logDir := fs.String("dir", ".", "markdown output directory")
	category := fs.String("c", "", "category (e.g. dev, admin, security)")
	title := fs.String("t", "", "title")
	bullets := fs.String("b", "", "bullets (comma-separated)")
	hours := fs.Float64("e", 0, "estimated person-hours")
	date := fs.String("d", "", "date (YYYY-MM-DD, default: today)")
	time := fs.String("T", "", "time (HH:MM, default: now)")
	fs.Parse(args)

	if *category == "" || *title == "" || *bullets == "" {
		return fmt.Errorf("required: -c category -t title -b bullets")
	}

	db, err := OpenDB(*dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	parts := strings.Split(*bullets, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}

	e := &Entry{
		Date:     *date,
		Time:     *time,
		Category: *category,
		Title:    *title,
		Bullets:  parts,
		HoursEst: *hours,
	}

	id, err := InsertEntry(db, e)
	if err != nil {
		return err
	}

	// Sync markdown
	entries, _ := GetEntries(db, e.Date)
	if md := RenderMarkdown(entries); md != "" {
		os.WriteFile(fmt.Sprintf("%s/%s.md", *logDir, e.Date), []byte(md), 0644)
	}

	out, _ := json.Marshal(map[string]any{"id": id, "date": e.Date})
	fmt.Println(string(out))
	return nil
}

func cmdLs(args []string) error {
	fs := flag.NewFlagSet("hrs ls", flag.ExitOnError)
	dbPath := fs.String("db", "hrs.db", "sqlite database path")
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

	entries, err := GetEntries(db, date)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		fmt.Printf("No entries for %s\n", date)
		return nil
	}

	fmt.Print(RenderMarkdown(entries))
	return nil
}

func cmdTUI(args []string) error {
	fs := flag.NewFlagSet("hrs tui", flag.ExitOnError)
	dbPath := fs.String("db", "hrs.db", "sqlite database path")
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
	dbPath := fs.String("db", "hrs.db", "sqlite database path")
	logDir := fs.String("dir", ".", "directory with existing markdown files")
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
