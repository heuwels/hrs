package main

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"regexp"
	"strings"
)

//go:embed docs/*.md
var docsFS embed.FS

var (
	mdHeading  = regexp.MustCompile(`(?m)^(#{1,6})\s+(.+)$`)
	mdCode     = regexp.MustCompile("(?s)````?([a-z]*)\\n(.*?)````?")
	mdTable    = regexp.MustCompile(`(?m)^\|(.+)\|$`)
	mdTableSep = regexp.MustCompile(`(?m)^\|[-| ]+\|$`)
	mdLink     = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	mdBold     = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	mdInline   = regexp.MustCompile("`([^`]+)`")
	mdHR       = regexp.MustCompile(`(?m)^---+$`)
	mdUL       = regexp.MustCompile(`(?m)^- (.+)$`)
)

const pageTmpl = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Title}} — worklog</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:'SF Mono','Cascadia Code','Fira Code',monospace;
  font-size:15px;line-height:1.7;color:#c9d1d9;background:#0d1117;
  max-width:680px;margin:0 auto;padding:2rem 1.5rem}
h1,h2,h3{color:#e6edf3;margin:1.5em 0 0.5em;font-weight:600}
h1{font-size:1.4em;border-bottom:1px solid #21262d;padding-bottom:0.3em}
h2{font-size:1.1em}
h3{font-size:1em}
p{margin:0.5em 0}
a{color:#58a6ff;text-decoration:none}
a:hover{text-decoration:underline}
hr{border:none;border-top:1px solid #21262d;margin:1.5em 0}
code{background:#161b22;padding:0.15em 0.4em;border-radius:3px;font-size:0.9em;color:#e6edf3}
pre{background:#161b22;padding:1em;border-radius:6px;overflow-x:auto;margin:0.8em 0;border:1px solid #21262d}
pre code{background:none;padding:0}
ul{margin:0.5em 0;padding-left:1.5em}
li{margin:0.2em 0}
table{border-collapse:collapse;margin:0.8em 0;width:100%}
th,td{border:1px solid #21262d;padding:0.4em 0.8em;text-align:left}
th{background:#161b22;color:#e6edf3;font-weight:600}
em{color:#8b949e}
.ascii{color:#58a6ff;text-align:center;margin:1em 0}
</style>
</head>
<body>
{{.Body}}
</body>
</html>`

var pageTemplate = template.Must(template.New("page").Parse(pageTmpl))

func renderMD(src string) (title, body string) {
	// Extract title from first h1
	if m := mdHeading.FindStringSubmatch(src); m != nil && m[1] == "#" {
		title = m[2]
	}
	if title == "" {
		title = "worklog"
	}

	s := src

	// Code blocks first (protect from other transforms)
	type codeBlock struct {
		placeholder string
		html        string
	}
	var blocks []codeBlock
	s = mdCode.ReplaceAllStringFunc(s, func(m string) string {
		sub := mdCode.FindStringSubmatch(m)
		ph := fmt.Sprintf("§CODE%d§", len(blocks))
		escaped := template.HTMLEscapeString(sub[2])
		blocks = append(blocks, codeBlock{ph, "<pre><code>" + escaped + "</code></pre>"})
		return ph
	})

	// Inline code (protect from other transforms)
	var inlines []codeBlock
	s = mdInline.ReplaceAllStringFunc(s, func(m string) string {
		sub := mdInline.FindStringSubmatch(m)
		ph := fmt.Sprintf("§INLINE%d§", len(inlines))
		escaped := template.HTMLEscapeString(sub[1])
		inlines = append(inlines, codeBlock{ph, "<code>" + escaped + "</code>"})
		return ph
	})

	// ASCII art blocks (``` with no language)
	s = regexp.MustCompile("(?s)```\\n(.*?)```").ReplaceAllString(s, `<div class="ascii"><pre>$1</pre></div>`)

	// Tables
	var tableLines []string
	lines := strings.Split(s, "\n")
	var out []string
	inTable := false
	for _, line := range lines {
		if mdTable.MatchString(line) && !mdTableSep.MatchString(line) {
			tableLines = append(tableLines, line)
			if !inTable {
				inTable = true
			}
		} else if mdTableSep.MatchString(line) {
			continue
		} else {
			if inTable {
				out = append(out, buildTable(tableLines))
				tableLines = nil
				inTable = false
			}
			out = append(out, line)
		}
	}
	if inTable {
		out = append(out, buildTable(tableLines))
	}
	s = strings.Join(out, "\n")

	// Headings
	s = mdHeading.ReplaceAllStringFunc(s, func(m string) string {
		sub := mdHeading.FindStringSubmatch(m)
		level := len(sub[1])
		return fmt.Sprintf("<h%d>%s</h%d>", level, sub[2], level)
	})

	// HRs
	s = mdHR.ReplaceAllString(s, "<hr>")

	// Lists
	s = mdUL.ReplaceAllString(s, "<li>$1</li>")
	s = strings.ReplaceAll(s, "<li>", "<ul><li>")
	s = strings.ReplaceAll(s, "</li>\n<ul><li>", "</li>\n<li>")
	// Close unclosed uls
	s = strings.ReplaceAll(s, "</li>\n", "</li></ul>\n")
	// Fix double-close
	s = strings.ReplaceAll(s, "</ul></ul>", "</ul>")

	// Links
	s = mdLink.ReplaceAllString(s, `<a href="$2">$1</a>`)

	// Bold
	s = mdBold.ReplaceAllString(s, "<strong>$1</strong>")

	// Italics
	s = regexp.MustCompile(`\*([^*]+)\*`).ReplaceAllString(s, "<em>$1</em>")

	// Paragraphs (loose lines)
	pLines := strings.Split(s, "\n")
	var pOut []string
	for _, l := range pLines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" {
			pOut = append(pOut, "")
		} else if strings.HasPrefix(trimmed, "<") || strings.HasPrefix(trimmed, "§") {
			pOut = append(pOut, l)
		} else {
			pOut = append(pOut, "<p>"+l+"</p>")
		}
	}
	s = strings.Join(pOut, "\n")

	// Restore code blocks
	for _, b := range blocks {
		s = strings.ReplaceAll(s, b.placeholder, b.html)
	}
	for _, b := range inlines {
		s = strings.ReplaceAll(s, b.placeholder, b.html)
	}

	return title, s
}

func buildTable(rows []string) string {
	if len(rows) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("<table>")
	for i, row := range rows {
		cells := strings.Split(strings.Trim(row, "|"), "|")
		tag := "td"
		if i == 0 {
			tag = "th"
		}
		b.WriteString("<tr>")
		for _, c := range cells {
			fmt.Fprintf(&b, "<%s>%s</%s>", tag, strings.TrimSpace(c), tag)
		}
		b.WriteString("</tr>")
	}
	b.WriteString("</table>")
	return b.String()
}

func docsHandler(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "/" || path == "" {
		path = "/index"
	}
	path = strings.TrimPrefix(path, "/")

	data, err := docsFS.ReadFile("docs/" + path + ".md")
	if err != nil {
		http.NotFound(w, r)
		return
	}

	title, body := renderMD(string(data))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	pageTemplate.Execute(w, struct {
		Title string
		Body  template.HTML
	}{title, template.HTML(body)})
}
