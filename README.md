# worklog

Timesheets for your agent. A tiny HTTP server backed by SQLite that AI agents can POST work entries to from any directory.

```
POST /entries → log work
GET  /entries → read back
```

## Why

I was tired of getting to the end of each day after context-switching between a dozen priorities and feeling like every day was an armed robbery where my time was the prize. I needed a way to recall what I actually worked on.

AI coding agents (Claude Code, Cursor, etc.) can't write files outside their working directory. This gives them a local API to push structured work logs to from wherever they're running, rendered as markdown for humans to read.

## Install

```bash
go install github.com/kollwitz-owen/worklog@latest
```

Or build from source:

```bash
git clone https://github.com/kollwitz-owen/worklog
cd worklog
go build -o worklog .
```

## Usage

```bash
# Start the server (default: localhost:9746)
worklog --db ~/worklog.db --dir ~/worklogs/

# Custom port
worklog --port 8080 --db ~/worklog.db --dir ~/worklogs/
```

The `--dir` flag controls where markdown files are rendered. Point it at a directory your TUI or editor watches.

## API

### `POST /entries`

```bash
curl -X POST http://localhost:9746/entries -d '{
  "category": "dev",
  "title": "Built authentication flow",
  "bullets": [
    "Implemented OAuth2 PKCE flow",
    "Added token refresh logic",
    "Wrote integration tests"
  ],
  "hours_est": 3
}'
```

`date` and `time` default to now if omitted.

### `GET /entries?date=2026-04-13`

Returns JSON array of entries. Defaults to today.

### `DELETE /entries/{id}`

Remove an entry by ID.

### `GET /schema`

Returns field names, types, and descriptions so agents can self-discover the API without hardcoded instructions:

```json
{
  "endpoint": "POST /entries",
  "fields": [
    {"name": "category", "type": "string", "required": true, "description": "Work category, e.g. dev, security, admin, docs, infra"},
    {"name": "title", "type": "string", "required": true, "description": "Concise label for the work performed"},
    {"name": "bullets", "type": "[]string", "required": true, "description": "Terse, outcome-focused bullet points"},
    {"name": "hours_est", "type": "number", "required": false, "description": "Estimated person-hours this would take without AI assistance"},
    {"name": "date", "type": "string", "required": false, "description": "YYYY-MM-DD, defaults to today"},
    {"name": "time", "type": "string", "required": false, "description": "HH:MM, defaults to now"}
  ]
}
```

### `GET /health`

Returns `{"status": "ok"}`.

## Markdown output

Each POST syncs a `YYYY-MM-DD.md` file in `--dir`:

```markdown
# Worklog — 2026-04-13

## 14:30 - [dev] Built authentication flow (~3h)
- Implemented OAuth2 PKCE flow
- Added token refresh logic
- Wrote integration tests

---
## Daily Summary
- Entries: 1
- Est. person-hours saved: 3h
- Est. person-days: 0.4d (assuming 8h/day)
```

## Migrate existing markdown

If you have existing `YYYY-MM-DD.md` worklog files, import them:

```bash
worklog --migrate --db ~/worklog.db --dir ~/worklogs/
```

## Agent integration

Add something like this to your `CLAUDE.md` (or equivalent agent instructions). Here's an abridged version of how we use it at KO:

````markdown
## Work Logging

All agent sessions MUST log significant work to the shared worklog.

### When to log:
- Any task that took more than 2-3 minutes
- Writing, modifying, or reviewing code
- Creating, updating, or reviewing PRs
- Bug fixes, security work, documentation
- Research that produced findings

**Rule of thumb:** Can you write 2+ bullet points? Then log it.

### How to log:

```bash
curl -s -X POST http://localhost:9746/entries -d '{
  "category": "dev",
  "title": "Short description of work",
  "bullets": ["What was accomplished", "Key outcomes or artifacts", "Files/systems touched"],
  "hours_est": 2
}'
```

### Fields:

Discover fields dynamically: `curl -s http://localhost:9746/schema`

### Important:
- **Log proactively** — don't wait to be asked
- **Log during work** — not just at the end of the session
- **Be generous** — when in doubt, log it
````

## A note on hours estimates

The `hours_est` field asks the AI to estimate how long the work would take a competent developer without AI assistance. This is useful for understanding throughput, but take it with a grain of salt — AI agents tend to overstate the complexity of tasks they've completed. A "~4h" estimate for something that took 3 minutes of wall clock time is flattering but not always realistic. The daily summaries roll these up into "person-days saved" which makes for a nice story, just don't use it to plan your next sprint.

## Design

- Single static binary, zero runtime dependencies
- SQLite with WAL mode — handles concurrent agent writes
- `net/http` stdlib server — one goroutine per request
- Markdown files are a rendered view, SQLite is the source of truth
- ~200 lines of Go

## License

MIT
