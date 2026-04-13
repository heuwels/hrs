package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var headerRe = regexp.MustCompile(`^##\s+(\d{1,2}:\d{2})\s+-\s+\[([^\]]+)\]\s+(.+?)(?:\s+\(~([\d.]+)h\))?\s*$`)

func Migrate(db *sql.DB, dir string) (int, error) {
	matches, _ := filepath.Glob(filepath.Join(dir, "????-??-??.md"))
	sort.Strings(matches)
	total := 0
	for _, path := range matches {
		date := strings.TrimSuffix(filepath.Base(path), ".md")
		var n int
		db.QueryRow(`SELECT COUNT(*) FROM entries WHERE date=?`, date).Scan(&n)
		if n > 0 {
			fmt.Fprintf(os.Stderr, "  skip %s (%d exist)\n", date, n)
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		entries := parseMarkdown(string(data), date)
		for _, e := range entries {
			b, _ := json.Marshal(e.Bullets)
			db.Exec(`INSERT INTO entries (date,time,category,title,bullets,hours_est) VALUES(?,?,?,?,?,?)`,
				e.Date, e.Time, e.Category, e.Title, string(b), e.HoursEst)
		}
		total += len(entries)
		fmt.Fprintf(os.Stderr, "  %s: %d entries\n", date, len(entries))
	}
	return total, nil
}

func parseMarkdown(content, date string) []Entry {
	var entries []Entry
	var cur *Entry
	for _, line := range strings.Split(content, "\n") {
		if m := headerRe.FindStringSubmatch(line); m != nil {
			if cur != nil {
				entries = append(entries, *cur)
			}
			h, _ := strconv.ParseFloat(m[4], 64)
			cur = &Entry{Date: date, Time: m[1], Category: m[2], Title: m[3], HoursEst: h}
		} else if cur != nil && strings.HasPrefix(line, "- ") {
			cur.Bullets = append(cur.Bullets, line[2:])
		} else if strings.HasPrefix(line, "---") {
			if cur != nil {
				entries = append(entries, *cur)
				cur = nil
			}
			break
		}
	}
	if cur != nil {
		entries = append(entries, *cur)
	}
	return entries
}
