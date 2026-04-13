# agent integration

[← home](/)

---

## the problem

ai coding agents are sandboxed to their working directory. they can't
write to a shared worklog at `~/worklogs/2026-04-13.md` if they're
running in `~/projects/my-app/`.

## the solution

worklog runs as a local daemon. agents post entries via http from
anywhere. the server writes the markdown files.

## example: claude code

add to your `CLAUDE.md`:

````markdown
## work logging

after completing significant work, log it:

```bash
curl -s -X POST http://localhost:9746/entries -d '{
  "category": "dev",
  "title": "Short description of work",
  "bullets": ["What was accomplished", "Key outcomes"],
  "hours_est": 2
}'
```

discover fields: `curl -s http://localhost:9746/schema`

log proactively. don't wait to be asked.
````

## example: cursor / other agents

same pattern — any agent that can shell out to `curl` can log:

```bash
curl -s -X POST http://localhost:9746/entries \
  -H "Content-Type: application/json" \
  -d '{"category":"dev","title":"...","bullets":["..."],"hours_est":1}'
```

## tips

- **log during work, not after** — agents forget context fast
- **be generous with hours_est** — but read the [caveat](/tui#hours-caveat)
- **category matters** — helps filter in the tui later
- **bullets should be outcomes** — "deployed X" not "worked on X"

---

[tui explorer →](/tui)
