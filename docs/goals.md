# goals & strategies

[← home](/)

---

two layers: **daily goals** for what you want done today, and **strategies**
for things that span days or weeks.

goals link to entries (work you've logged) and to strategies. hours roll up
from entries through goals into strategy reports.

```
strategy: "ship v2 auth"
  ├── goal: "implement oauth2 pkce" (2026-04-14) → entries #41, #42
  ├── goal: "add token refresh" (2026-04-15) → entry #45
  └── goal: "write integration tests" (2026-04-16) → entry #48
```

---

## daily goals

### set goals for today

```bash
hrs goals add "implement oauth2 pkce"
hrs goals add "review deployment docs" -d 2026-04-18
hrs goals add "write migration scripts" -s 1
```

`-d` sets the date (defaults to today). `-s` links the goal to a strategy.

### list goals

```bash
hrs goals                    # today's goals
hrs goals -d 2026-04-18     # specific date
hrs goals --format json      # machine-readable
```

### complete a goal

```bash
hrs goals done 1             # mark goal #1 complete
hrs goals done 1 -e 41,42   # complete and link entries #41 and #42
```

linking entries connects the work you logged to the goal it achieved.

### link entries to a goal

```bash
hrs goals link 1 -e 41,42   # link entries without completing
```

### reopen or delete

```bash
hrs goals undo 1             # reopen a completed goal
hrs goals rm 1               # delete a goal
```

---

## strategies

strategies span multiple days. daily goals link up to them, and hrs
rolls up progress across all linked goals.

### create a strategy

```bash
hrs strategy add -t "ship v2 auth" -desc "oauth2, token refresh, integration tests"
```

title can also be passed as a positional argument:

```bash
hrs strategy add "ship v2 auth"
```

### list strategies

```bash
hrs strategy                         # all strategies
hrs strategy -status active          # filter by status
hrs strategy --format json           # machine-readable
```

### view a strategy report

```bash
hrs strategy 1                       # shorthand for report
hrs strategy report 1                # explicit
hrs strategy report 1 --format json  # machine-readable
```

the report shows goal completion (e.g. 3/5 done), total hours rolled up
from linked entries, and a list of all goals.

### manage lifecycle

```bash
hrs strategy done 1          # mark as completed
hrs strategy archive 1       # pause (archive)
hrs strategy reopen 1        # set back to active
hrs strategy edit 1 -t "new title" -desc "updated description"
hrs strategy rm 1            # delete (unlinks goals, doesn't delete them)
```

---

## linking it all together

the typical workflow:

1. create a strategy for a multi-day initiative
2. each morning, set daily goals linked to that strategy
3. as you complete work, log entries with `hrs log`
4. mark goals done and link the entries that achieved them
5. check strategy reports to see overall progress

```bash
# 1. create strategy
hrs strategy add -t "migrate to postgres"

# 2. set today's goals, linked to the strategy (-s flag)
hrs goals add "schema migration scripts" -s 1
hrs goals add "update connection pooling" -s 1

# 3. log work as you go
hrs log -c dev -t "wrote migration scripts" -b "up/down for users table;seed data" -e 2

# 4. complete goal, link the entry
hrs goals done 1 -e 50

# 5. check progress
hrs strategy 1
```

link goals to a strategy with `-s` on the CLI or `strategy_id` via HTTP.

---

## tui support

the tui has a dedicated goals view. press `tab` to switch between entries and
goals. see [tui explorer](/tui) for all keybindings.

---

[api reference →](/api)
