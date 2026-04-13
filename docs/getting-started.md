# getting started

[← home](/)

---

## install

```bash
go install github.com/kollwitz-owen/worklog@latest
```

or build from source:

```bash
git clone https://github.com/kollwitz-owen/worklog
cd worklog
go build -o worklog .
```

## start the server

```bash
worklog serve --db ~/worklog.db --dir ~/worklogs/
```

`--dir` is where markdown files get rendered. point it at a directory
your editor or tui watches.

## log something

```bash
# via curl
curl -X POST http://localhost:9746/entries -d '{
  "category": "dev",
  "title": "first entry",
  "bullets": ["hello world"],
  "hours_est": 0.1
}'

# via cli
worklog log -db ~/worklog.db -c dev -t "first entry" -b "hello world" -e 0.1
```

## view your logs

```bash
# print today's log
worklog ls -db ~/worklog.db

# launch the tui
worklog tui -db ~/worklog.db
```

## migrate existing markdown

if you have existing `YYYY-MM-DD.md` worklog files:

```bash
worklog migrate --db ~/worklog.db --dir ~/worklogs/
```

---

[api reference →](/api)
