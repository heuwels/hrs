# agent integration

[← home](/)

---

## the problem

hrs started as a markdown file archive with a tui wrapper. three things
broke that approach:

1. **blind writes**: claude would attempt to write directly to the
   markdown file, fail, and waste an entire tool-use cycle retrying.
2. **file locking**: multiple agents running concurrently would clobber
   each other's writes to the same file.
3. **sandboxing**: once we added guardrails (like
   [toolgate](https://github.com/brycehans/toolgate)) that prevent
   writes outside the active project directory, agents couldn't reach a
   shared worklog at all.

the fix: a cli and http api backed by sqlite, so agents can log work
from anywhere without touching files directly.

## two ways to log

agents can use either the **CLI** or the **HTTP daemon**. both write
to the same sqlite database and produce the same markdown output.

### option a: CLI (`hrs log`)

the simplest option. no daemon needed, the cli writes directly to
the database.

```bash
hrs log -c dev -t "built auth flow" -b "oauth2 pkce;token refresh;tests" -e 3
```

this works if your agent is allowed to run arbitrary commands.

### option b: HTTP daemon (`hrs serve`)

if your agent is sandboxed and can't write outside its project
directory, run the daemon and have agents POST via http instead.

this is useful when you use tools like
[toolgate](https://github.com/brycehans/toolgate) to restrict file
writes to the project root. the agent can still log work via http
without needing write access to the hrs database.

```bash
# start the daemon (once)
hrs serve &

# agents post entries from any directory
curl -s -X POST http://localhost:9746/entries -d '{
  "category": "dev",
  "title": "built auth flow",
  "bullets": ["oauth2 pkce", "token refresh", "tests"],
  "hours_est": 3
}'
```

agents can self-discover fields via `GET /schema`.

### which to use?

| | CLI | HTTP |
|---|---|---|
| setup | none | run `hrs serve` |
| works sandboxed | no (needs db write) | yes |
| works without daemon | yes | no |
| output | same | same |

the cli is smart: it tries the http server first, then falls back to a
direct database write. so if the daemon is running, `hrs log` uses it
automatically.

## example: claude code

add to your `CLAUDE.md`:

````markdown
## work logging

after completing significant work, log it with `hrs log`.
run `hrs log -h` to discover the flags.

log proactively. don't wait to be asked.
````

if using toolgate or another sandbox that blocks writes outside the
project directory, use curl instead:

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

same pattern. any agent that can shell out can log:

```bash
# cli
hrs log -c dev -t "..." -b "..." -e 1

# or http
curl -s -X POST http://localhost:9746/entries \
  -H "Content-Type: application/json" \
  -d '{"category":"dev","title":"...","bullets":["..."],"hours_est":1}'
```

## goals for agents

agents can also set and complete daily goals. this pairs well with work logging.
set a goal before starting, log entries as you work, then mark the goal done and
link the entries.

```bash
# set a goal
hrs goals add "implement oauth2 pkce"

# ... do the work, log entries ...

# complete and link entries
hrs goals done 1 -e 41,42
```

via HTTP:

```bash
# create a goal
curl -s -X POST http://localhost:9746/goals -d '{"text": "implement oauth2 pkce"}'

# complete with linked entries
curl -s -X PUT http://localhost:9746/goals/1/done -d '{"entry_ids": [41, 42]}'
```

see [goals & strategies](/goals) for the full guide.

## tips

- **log during work, not after**: agents forget context fast
- **be generous with hours_est**: but read the [caveat](/tui#hours-caveat)
- **category matters**: helps filter in the tui later
- **bullets should be outcomes**: "deployed X" not "worked on X"

---

[tui explorer →](/tui)
