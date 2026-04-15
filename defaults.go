package main

import (
	"os"
	"path/filepath"
)

// DefaultDir returns the hrs data directory.
// Checks HRS_DIR env var first, then defaults to ~/.hrs/.
// Does NOT create the directory — callers must ensure it exists before writing.
func DefaultDir() string {
	if dir := os.Getenv("HRS_DIR"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".hrs")
}

// DefaultDB returns the hrs database path.
// Checks HRS_DB env var first, then defaults to <DefaultDir>/hrs.db.
func DefaultDB() string {
	if db := os.Getenv("HRS_DB"); db != "" {
		return db
	}
	return filepath.Join(DefaultDir(), "hrs.db")
}
