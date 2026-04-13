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

| field      | type     | required | default |
|------------|----------|----------|---------|
| category   | string   | yes      |         |
| title      | string   | yes      |         |
| bullets    | []string | yes      |         |
| hours_est  | number   | no       | 0       |
| date       | string   | no       | today   |
| time       | string   | no       | now     |

returns `201`:

```json
{"id": 42, "date": "2026-04-13"}
```

## GET /entries?date=YYYY-MM-DD

list entries for a date. defaults to today.

returns `200` with a json array of entries.

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
  "endpoint": "POST /entries",
  "fields": [
    {"name": "category", "type": "string", "required": true, "description": "..."},
    ...
  ]
}
```

## GET /health

```json
{"status": "ok"}
```

---

[agent integration →](/agent-integration)
