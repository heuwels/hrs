package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	port := flag.Int("port", 9746, "listen port")
	dbPath := flag.String("db", "worklog.db", "sqlite database path")
	logDir := flag.String("dir", ".", "directory for rendered markdown files")
	migrate := flag.Bool("migrate", false, "import existing markdown files into db")
	flag.Parse()

	db, err := OpenDB(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if *migrate {
		n, err := Migrate(db, *logDir)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("imported %d entries\n", n)
		return
	}

	s := &Server{db: db, logDir: *logDir}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.Health)
	mux.HandleFunc("GET /entries", s.ListEntries)
	mux.HandleFunc("POST /entries", s.CreateEntry)
	mux.HandleFunc("DELETE /entries/{id}", s.DeleteEntry)

	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	srv := &http.Server{Addr: addr, Handler: mux}

	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		<-ch
		srv.Close()
	}()

	fmt.Fprintf(os.Stderr, "worklogd listening on http://%s\n", addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
