# getting started

[← home](/)

---

## install

### download a binary

grab the latest release for your platform from
[github releases](https://github.com/heuwels/hrs/releases/latest).

```bash
# example: macos arm64
curl -L https://github.com/heuwels/hrs/releases/latest/download/hrs_darwin_arm64.tar.gz | tar xz
sudo mv hrs /usr/local/bin/
```

available builds: `darwin_amd64`, `darwin_arm64`, `linux_amd64`, `linux_arm64`.

### go install

```bash
go install github.com/heuwels/hrs@latest
```

### build from source

```bash
git clone https://github.com/heuwels/hrs
cd hrs
go build -o hrs .
```

no cgo — a plain `go build` works on any platform with no C compiler.

---

## configuration

hrs stores data in `~/.hrs/hrs.db` by default. override with env vars:

| variable | default          | description             |
|----------|------------------|-------------------------|
| HRS_DB   | ~/.hrs/hrs.db    | sqlite database path    |
| HRS_DIR  | ~/.hrs/          | markdown output dir     |

or pass `--db` / `--dir` flags to any command.

---

## quick start

### log an entry

```bash
hrs log -c dev -t "built auth flow" -b "oauth2 pkce;token refresh;tests" -e 3
```

bullets are separated by semicolons. `-e` is your estimate of how long
this would take a competent developer without ai assistance.

### view today's log

```bash
hrs ls
```

color output in the terminal, markdown when piped. add `--format json`
for structured output.

### browse with the tui

```bash
hrs tui
```

vim keys: `j/k` scroll entries, `h/l` switch days, `d` delete, `t` jump to today.

---

## cli reference

| command                  | description                          |
|--------------------------|--------------------------------------|
| `hrs serve`              | start the http api server            |
| `hrs log -c -t -b -e`   | log an entry                         |
| `hrs ls [date]`          | list entries (today if omitted)      |
| `hrs ls --from --to`     | list a date range                    |
| `hrs tui [date]`         | interactive terminal explorer        |
| `hrs edit <id> [flags]`  | update an entry                      |
| `hrs rm <id>`            | delete an entry                      |
| `hrs export`             | export entries as json or csv        |
| `hrs categories`         | list all categories                  |
| `hrs migrate`            | import existing markdown worklogs    |
| `hrs version`            | print version                        |

### hrs ls

```bash
hrs ls                                  # today, markdown/color
hrs ls 2026-04-14                       # specific date
hrs ls --format json                    # json output
hrs ls --from 2026-04-01 --to 2026-04-15  # date range
hrs ls --from 2026-04-01 --category dev   # filter by category
```

### hrs edit

```bash
hrs edit 42 -t "new title"              # update title
hrs edit 42 -e 2 -c admin              # update hours and category
hrs edit 42 -b "point one;point two"   # replace bullets
```

### hrs export

```bash
hrs export --format json                # all entries as json
hrs export --format csv --from 2026-04-01 --to 2026-04-30
hrs export --category dev               # filter by category
```

---

## run as a service

### macos (launchd)

```bash
cp contrib/hrs.plist ~/Library/LaunchAgents/
launchctl load ~/Library/LaunchAgents/hrs.plist
```

### linux (systemd)

```bash
cp contrib/hrs.service ~/.config/systemd/user/
systemctl --user enable --now hrs
```

---

## shell completions

installed automatically via homebrew. for manual setup:

```bash
# bash
source <(cat completions/hrs.bash)

# zsh — add to fpath
cp completions/hrs.zsh ~/.zsh/completions/_hrs

# fish
cp completions/hrs.fish ~/.config/fish/completions/
```

---

[api reference →](/api)
