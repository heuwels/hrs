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

## GET /health

```json
{"status": "ok"}
```

---

[agent integration →](/agent-integration)
