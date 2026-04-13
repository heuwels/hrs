# getting started

[← home](/)

---

## install

```bash
go install github.com/kollwitz-owen/hrs@latest
```

or build from source:

```bash
git clone https://github.com/kollwitz-owen/hrs
cd hrs
go build -o hrs .
```

## start the server

```bash
hrs serve --db ~/hrs.db --dir ~/worklogs/
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
hrs log -db ~/hrs.db -c dev -t "first entry" -b "hello world" -e 0.1
```

## view your logs

```bash
# print today's log
hrs ls -db ~/hrs.db

# launch the tui
hrs tui -db ~/hrs.db
```

## migrate existing markdown

if you have existing `YYYY-MM-DD.md` worklog files:

```bash
hrs migrate --db ~/hrs.db --dir ~/worklogs/
```

---

[api reference →](/api)
