package main

import (
	"os"
	"path/filepath"
)

// DefaultDir returns ~/.hrs, creating it if needed.
func DefaultDir() string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".hrs")
	os.MkdirAll(dir, 0755)
	return dir
}

// DefaultDB returns ~/.hrs/hrs.db
func DefaultDB() string {
	return filepath.Join(DefaultDir(), "hrs.db")
}
