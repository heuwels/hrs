# Comprehensive Refactor — Context

## What changed

### Core (db.go)
- Switched from CGo sqlite (mattn/go-sqlite3) to pure Go (ncruces/go-sqlite3) — eliminates C compiler requirement
- Added input validation: date (YYYY-MM-DD), time (HH:MM), hours_est (0-24), non-empty fields
- Added UpdateEntry, GetEntryByID, GetEntriesRange, GetCategories functions
- Fixed "person-hours saved" → "person-hours (without AI)" in markdown summary

### CLI (cli.go, main.go)
- Fixed bullet delimiter: comma → semicolon (avoids data corruption with commas in text)
- Added `hrs version` (set via ldflags at build time)
- Added `hrs rm <id>` — delete entry by ID
- Added `hrs edit <id>` — update entry with optional flag overrides
- Added `hrs ls --format json|md --from --to --category` — date ranges and JSON output
- Added `hrs export --format json|csv --from --to --category` — bulk data export
- Added `hrs categories` — list all unique categories
- Added color output for `hrs ls` when stdout is a TTY
- Added env var support: HRS_DB, HRS_DIR (no more repeating --db/--dir flags)
- Fixed filepath.Join consistency (was using fmt.Sprintf with /)

### Defaults (defaults.go)
- Removed os.MkdirAll side effect from DefaultDir() — deferred to actual writes
- Added HRS_DB and HRS_DIR env var checks

### Server (server.go)
- Added PUT /entries/{id} endpoint for updating entries
- Added from/to/category query params to GET /entries (backward compatible with date param)
- Updated schema to document all endpoints

### TUI (tui.go)
- Added d key to delete selected entry
- Added e key to show "use: hrs edit <id>" hint
- Updated help footer

### Tests (server_test.go)
- Added TestUpdateEntry, TestDateRangeQuery, TestCategoryFilter, TestInputValidation
- Updated testMux with PUT route
- Fixed testServer to use file-based temp DBs (ncruces requirement)

### Build Infrastructure (new files)
- .goreleaser.yml — CGO_ENABLED=0, cross-platform (darwin/linux, amd64/arm64), Homebrew tap
- .github/workflows/ci.yml — unit tests + e2e binary tests on ubuntu + macos
- .github/workflows/release.yml — tests gate before GoReleaser publish
- completions/ — bash, zsh, fish shell completions
- contrib/ — macOS launchd plist + Linux systemd service

## What's left
- Final e2e verification of all new commands
- Create PR or push to main
- Publish first tagged release (v0.2.0?)
