# api reference

[← home](/)

---

default: `http://localhost:9746`

## POST /entries

create an entry.

```json
{
  "category": "dev",
  "title": "built auth flow",
  "bullets": ["oauth2 pkce", "token refresh"],
  "hours_est": 3,
  "date": "2026-04-13",
  "time": "14:30"
}
```

| field      | type     | required | default | validation              |
|------------|----------|----------|---------|-------------------------|
| category   | string   | yes      |         | non-empty               |
| title      | string   | yes      |         | non-empty               |
| bullets    | []string | yes      |         | at least one non-empty  |
| hours_est  | number   | no       | 0       | 0-24                    |
| date       | string   | no       | today   | YYYY-MM-DD              |
| time       | string   | no       | now     | HH:MM                   |

returns `201`:

```json
{"id": 42, "date": "2026-04-13"}
```

## PUT /entries/{id}

update an existing entry. accepts the same fields as POST.

```bash
curl -X PUT http://localhost:9746/entries/42 -d '{
  "category": "dev",
  "title": "updated title",
  "bullets": ["revised bullet"],
  "hours_est": 4,
  "date": "2026-04-13",
  "time": "14:30"
}'
```

returns `200` with the updated entry.

## GET /entries

list entries. supports three query modes:

```bash
# single date (default: today)
GET /entries?date=2026-04-13

# date range (inclusive)
GET /entries?from=2026-04-01&to=2026-04-15

# date range with category filter
GET /entries?from=2026-04-01&to=2026-04-15&category=dev
```

returns `200` with a json array of entries. returns `[]` if no matches.

## DELETE /entries/{id}

remove an entry by id.

returns `200`:

```json
{"deleted": 42}
```

## GET /schema

returns field names, types, and descriptions. agents can call this to
self-discover the api without hardcoded instructions in their prompts.

```json
{
  "endpoints": [
    {
      "method": "POST",
      "path": "/entries",
      "fields": [
        {"name": "category", "type": "string", "required": true, "description": "..."}
      ]
    },
    {"method": "PUT", "path": "/entries/{id}", "description": "..."},
    {"method": "GET", "path": "/entries", "description": "..."},
    {"method": "DELETE", "path": "/entries/{id}", "description": "..."}
  ]
}
```

## POST /goals

create a daily goal.

```json
{
  "text": "implement oauth2 pkce",
  "date": "2026-04-14",
  "strategy_id": 1
}
```

| field       | type   | required | default | description                    |
|-------------|--------|----------|---------|--------------------------------|
| text        | string | yes      |         | goal description               |
| date        | string | no       | today   | YYYY-MM-DD                     |
| strategy_id | number | no       |         | link to a strategic goal       |

returns `201`:

```json
{"id": 1, "date": "2026-04-14"}
```

## GET /goals

list goals for a date.

```bash
GET /goals?date=2026-04-14
```

returns `200` with a json array. each goal includes `entry_ids` if any entries
are linked.

## PUT /goals/{id}/done

mark a goal complete. optionally link entries.

```json
{"entry_ids": [41, 42]}
```

## PUT /goals/{id}/undo

reopen a completed goal.

## POST /goals/{id}/link

link entries to a goal.

```json
{"entry_ids": [41, 42]}
```

## DELETE /goals/{id}

delete a goal.

---

## POST /strategies

create a strategic goal.

```json
{
  "title": "ship v2 auth",
  "description": "oauth2, token refresh, integration tests"
}
```

| field       | type   | required | description                    |
|-------------|--------|----------|--------------------------------|
| title       | string | yes      | strategic goal title           |
| description | string | no       | longer description             |

returns `201`:

```json
{"id": 1, "title": "ship v2 auth"}
```

## GET /strategies

list strategies. filter by status.

```bash
GET /strategies?status=active
```

status values: `active`, `completed`, `archived`. omit for all.

## GET /strategies/{id}

strategy report with aggregated metrics.

```json
{
  "id": 1,
  "title": "ship v2 auth",
  "status": "active",
  "goals_done": 2,
  "goals_total": 5,
  "total_hours": 12.5
}
```

## PUT /strategies/{id}

update strategy status.

```json
{"status": "completed"}
```

## DELETE /strategies/{id}

delete a strategy. unlinks goals but does not delete them.

---

## GET /health

```json
{"status": "ok"}
```

---

[goals & strategies →](/goals)
