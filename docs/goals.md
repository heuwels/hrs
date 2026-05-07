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
hrs goals add -d 2026-04-18 "review deployment docs"
hrs goals add -s 1 "write migration scripts"
hrs goals add -s 1 --ticket PROMO-123 "ship pat creation"
```

`-d` sets the date (defaults to today). `-s` links the goal to a strategy.
`--ticket` attaches an external ticket reference (see [ticket
references](#ticket-references)). Place flags before the goal text — Go's
flag parser stops at the first positional argument.

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
hrs strategy add -t "ship pat creation" --ticket PROMO-150
```

title can also be passed as a positional argument:

```bash
hrs strategy add "ship v2 auth"
```

`--ticket` attaches an external ticket reference (see [ticket
references](#ticket-references)).

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

## ticket references

both goals and strategies can carry an optional `ticket_ref` — a free-form
string that points at an external work item. anything works:

- promotheus: `PROMO-123`
- jira: `PROJ-456`
- linear: `ENG-789`
- github: `org/repo#42` or a full url

```bash
hrs strategy add -t "ship pat creation" --ticket PROMO-150
hrs goals add -s 1 --ticket PROMO-151 "filament ui"
hrs goals edit 5 --ticket ""    # clear
```

set the ticket on the strategy, the goal, or both — the filter ORs across
them, so a goal under a `PROMO-150` strategy still matches `--ticket PROMO`
even when the goal itself carries no ticket.

### filtering entries by ticket

`hrs ls` and `hrs export` accept `--ticket` to find every entry linked
(through a goal) to a goal or strategy whose ticket matches the prefix:

```bash
# every entry tagged to a promotheus ticket — for an R&D claim
hrs export --ticket PROMO --from 2026-01-01 --to 2026-12-31 --format csv

# narrow to one ticket
hrs ls --ticket PROMO-150 --from 2026-04-01

# combine with category
hrs export --ticket PROMO --category dev --format json
```

the match is a SQL `LIKE` prefix by default — `--ticket PROMO` becomes
`PROMO%`. include `%` or `_` in your value to override.

ticket filtering only sees entries that are linked to a goal. if you want
work to count for a ticket-scoped report, complete the goal with `-e
<entry_ids>` (or `hrs goals link`) so the link is recorded.

---

## tui support

the tui has a dedicated goals view. press `tab` to switch between entries and
goals. see [tui explorer](/tui) for all keybindings.

---

[api reference →](/api)
