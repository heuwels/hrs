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

const siteCSS = `
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }

:root {
  --bg: #fafafa; --bg-alt: #f1f1f1; --text: #1a1a1a; --text-muted: #636363;
  --accent: #10b981; --accent-hover: #059669; --border: #e2e2e2;
  --code-bg: #1e1e2e; --code-text: #cdd6f4; --code-comment: #6c7086;
  --card-bg: #ffffff; --card-shadow: 0 1px 2px rgba(0,0,0,0.04);
  --hero-bg: #f5f5f5; --link: #10b981;
  --term-bg: #0d1117; --term-text: #c9d1d9; --term-green: #3fb950;
  --term-blue: #58a6ff; --term-yellow: #d29922; --term-dim: #484f58;
  --term-magenta: #bc8cff; --term-cyan: #39d2e0;
}

[data-theme="dark"] {
  --bg: #111111; --bg-alt: #1a1a1a; --text: #e4e4e4; --text-muted: #999999;
  --accent: #34d399; --accent-hover: #6ee7b7; --border: #2a2a2a;
  --code-bg: #0a0a0a; --code-text: #cdd6f4;
  --card-bg: #1a1a1a; --card-shadow: 0 1px 2px rgba(0,0,0,0.3);
  --hero-bg: #141414; --link: #34d399;
}

html { scroll-behavior: smooth; font-size: 16px; }
body {
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  background: var(--bg); color: var(--text); line-height: 1.7;
  -webkit-font-smoothing: antialiased;
}
a { color: var(--link); text-decoration: none; }
a:hover { text-decoration: underline; }
code, pre { font-family: "SF Mono", "Fira Code", "JetBrains Mono", Menlo, monospace; }
img { max-width: 100%; height: auto; display: block; }
.container { max-width: 860px; margin: 0 auto; padding: 0 1.5rem; }

/* Theme toggle */
.theme-toggle {
  position: fixed; bottom: 1.5rem; right: 1.5rem;
  background: var(--card-bg); border: 1px solid var(--border); border-radius: 50%;
  width: 42px; height: 42px; cursor: pointer;
  display: flex; align-items: center; justify-content: center;
  box-shadow: var(--card-shadow); z-index: 100; font-size: 1.1rem;
}
.theme-toggle:hover { background: var(--bg-alt); }
.theme-toggle .icon-sun, .theme-toggle .icon-moon { display: none; }
[data-theme="dark"] .theme-toggle .icon-sun { display: inline; }
[data-theme="light"] .theme-toggle .icon-moon { display: inline; }
:root:not([data-theme]) .theme-toggle .icon-moon { display: inline; }

/* Hero */
.hero {
  background: var(--hero-bg); padding: 3rem 0 5rem; text-align: center;
  border-bottom: 1px solid var(--border); overflow: visible; margin-bottom: 0;
}
.hero h1 { font-size: 3.5rem; font-weight: 800; letter-spacing: -0.03em; margin-bottom: 0; }
.hero .tagline {
  font-size: 1.15rem; color: var(--text-muted); margin: 0.5rem auto 2rem;
  max-width: 460px;
}
.hero-nav {
  display: flex; align-items: center; justify-content: center;
  gap: 0.5rem; flex-wrap: wrap; margin-bottom: 3rem;
}
.hero-nav a { color: var(--text-muted); font-size: 0.95rem; padding: 0.25rem; }
.hero-nav a:hover { color: var(--text); text-decoration: none; }
.hero-nav__primary {
  background: var(--accent); color: #fff !important;
  padding: 0.5rem 1.5rem !important; border-radius: 4px; font-weight: 600;
}
.hero-nav__primary:hover { background: var(--accent-hover); }
.hero-nav__sep { color: var(--text-muted); user-select: none; }

/* Terminal mockup */
.terminal-wrap { max-width: 640px; margin: 0 auto; }
.terminal {
  background: var(--term-bg); border-radius: 10px; overflow: hidden;
  box-shadow: 0 16px 48px rgba(0,0,0,0.15), 0 0 0 1px rgba(255,255,255,0.05);
  text-align: left;
}
[data-theme="dark"] .terminal { box-shadow: 0 16px 48px rgba(0,0,0,0.4), 0 0 0 1px rgba(255,255,255,0.06); }
.terminal-bar {
  display: flex; gap: 6px; padding: 12px 16px;
  background: #161b22; border-bottom: 1px solid #21262d;
}
.terminal-bar span { width: 12px; height: 12px; border-radius: 50%; }
.terminal-bar span:nth-child(1) { background: #ff5f57; }
.terminal-bar span:nth-child(2) { background: #febc2e; }
.terminal-bar span:nth-child(3) { background: #28c840; }
.terminal-bar::after {
  content: "hrs"; color: #484f58; font-size: 0.8rem; margin-left: auto;
  font-family: "SF Mono", "Fira Code", monospace;
}
.terminal-body {
  padding: 1.25rem 1.5rem; font-family: "SF Mono", "Fira Code", "JetBrains Mono", Menlo, monospace;
  font-size: 0.82rem; line-height: 1.6; color: var(--term-text); white-space: pre; overflow-x: auto;
}
.t-prompt { color: var(--term-green); }
.t-cmd { color: #e6edf3; font-weight: 600; }
.t-h1 { color: var(--term-cyan); font-weight: 700; }
.t-cat { color: var(--term-magenta); }
.t-title { color: var(--term-cyan); font-weight: 600; }
.t-hours { color: var(--term-dim); }
.t-bullet { color: #8b949e; }
.t-sel { color: var(--term-green); font-weight: 700; }
.t-dim { color: var(--term-dim); }
.t-summary { color: var(--term-dim); font-style: italic; }
.t-blink { animation: blink 1s step-end infinite; }
@keyframes blink { 50% { opacity: 0; } }

.section-rule { border: none; border-top: 1px solid var(--border); margin: 0; }

/* Pitch */
.pitch { padding: 4rem 0; }
.pitch h2 { font-size: 1.75rem; font-weight: 700; margin-bottom: 1rem; }
.accent {
  display: inline-block; background: var(--accent); color: #fff;
  padding: 0.05em 0.4em; border-radius: 4px; transform: rotate(-1.5deg);
}
[data-theme="dark"] .accent { color: #111; }
.pitch-intro { font-size: 1.1rem; color: var(--text-muted); max-width: 600px; margin-bottom: 2.5rem; }

/* Features */
.features-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); gap: 1.25rem; }
.feature {
  background: var(--card-bg); border: 1px solid var(--border);
  border-radius: 6px; padding: 1.5rem;
}
.feature-icon { font-size: 1.5rem; margin-bottom: 0.5rem; }
.feature h3 { font-size: 1rem; font-weight: 600; margin-bottom: 0.35rem; }
.feature p { font-size: 0.9rem; color: var(--text-muted); line-height: 1.5; }

/* Install */
.install { padding: 4rem 0; }
.install h2 { font-size: 1.75rem; font-weight: 700; margin-bottom: 1.5rem; }
.install pre {
  background: var(--code-bg); color: var(--code-text);
  padding: 1.25rem 1.5rem; border-radius: 8px; overflow-x: auto;
  font-size: 0.85rem; line-height: 1.6; margin-bottom: 1rem;
}
.install pre .comment { color: var(--code-comment); }
.install p { color: var(--text-muted); font-size: 0.95rem; margin-bottom: 1rem; }

/* Footer */
footer { border-top: 1px solid var(--border); padding: 2rem 0; text-align: center; }
.footer-nav {
  display: flex; align-items: center; justify-content: center;
  gap: 0.5rem; margin-bottom: 0.75rem; flex-wrap: wrap;
}
.footer-nav a { color: var(--text-muted); font-size: 0.9rem; }
.footer-nav a:hover { color: var(--text); }
.footer-sep { color: var(--text-muted); user-select: none; }
.footer-copy { color: var(--text-muted); font-size: 0.8rem; }

/* Docs pages */
.docs-header {
  background: var(--hero-bg); padding: 2rem 0 1.5rem;
  border-bottom: 1px solid var(--border);
}
.docs-header h1 { font-size: 1.75rem; font-weight: 700; }
.docs-header .breadcrumb { font-size: 0.85rem; color: var(--text-muted); margin-bottom: 0.5rem; }
.docs-header .breadcrumb a { color: var(--text-muted); }
.docs-body { padding: 3rem 0 4rem; }
.docs-body h2 {
  font-size: 1.5rem; font-weight: 700; margin-top: 3rem; margin-bottom: 1rem;
  padding-top: 1rem; border-top: 1px solid var(--border);
}
.docs-body h2:first-child { margin-top: 0; padding-top: 0; border-top: none; }
.docs-body h3 { font-size: 1.15rem; font-weight: 600; margin-top: 2rem; margin-bottom: 0.75rem; }
.docs-body p { margin-bottom: 1rem; color: var(--text); }
.docs-body ul, .docs-body ol { margin-bottom: 1rem; padding-left: 1.5rem; }
.docs-body li { margin-bottom: 0.35rem; }
.docs-body pre {
  background: var(--code-bg); color: var(--code-text);
  padding: 1.25rem 1.5rem; border-radius: 8px; overflow-x: auto;
  font-size: 0.85rem; line-height: 1.6; margin-bottom: 1.5rem;
}
.docs-body code { background: var(--bg-alt); padding: 0.15rem 0.4rem; border-radius: 4px; font-size: 0.85em; }
.docs-body pre code { background: none; padding: 0; border-radius: 0; font-size: inherit; }

@media (max-width: 640px) {
  .hero { padding: 2rem 0 3rem; }
  .hero h1 { font-size: 2.5rem; }
  .features-grid { grid-template-columns: 1fr; }
  .terminal-body { font-size: 0.72rem; padding: 1rem; }
}
`

const homeTmpl = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>hrs — timesheets for your agent</title>
<meta name="description" content="A tiny HTTP server backed by SQLite that AI agents can post work entries to from any directory. Renders markdown for humans.">
<style>` + siteCSS + `</style>
<script>
(function(){
  var t=localStorage.getItem('theme');
  if(t) document.documentElement.setAttribute('data-theme',t);
  else if(window.matchMedia('(prefers-color-scheme:dark)').matches) document.documentElement.setAttribute('data-theme','dark');
  else document.documentElement.setAttribute('data-theme','light');
})();
</script>
</head>
<body>

<header class="hero">
  <div class="container">
    <h1>hrs</h1>
    <p class="tagline">Timesheets for your agent. A tiny HTTP + SQLite server that every AI agent can POST work entries to.</p>
    <nav class="hero-nav">
      <a href="/docs/getting-started" class="hero-nav__primary">Get Started</a>
      <span class="hero-nav__sep">&middot;</span>
      <a href="/docs/">Docs</a>
      <span class="hero-nav__sep">&middot;</span>
      <a href="/docs/api">API</a>
      <span class="hero-nav__sep">&middot;</span>
      <a href="https://github.com/heuwels/hrs" target="_blank" rel="noopener noreferrer">
        <svg viewBox="0 0 16 16" width="16" height="16" fill="currentColor" style="vertical-align:-2px"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27s1.36.09 2 .27c1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/></svg>
      </a>
    </nav>

    <div class="terminal-wrap">
      <div class="terminal">
        <div class="terminal-bar"><span></span><span></span><span></span></div>
        <div class="terminal-body"><span class="t-prompt">~</span> <span class="t-cmd">hrs ls</span>

 <span class="t-h1">hrs</span>  <span style="color:#d29922;font-weight:700">2026-04-14</span> (today)

<span class="t-sel">&gt; </span><span class="t-sel">[dev]</span> <span class="t-title">hrs v1.0.0 released</span> <span class="t-hours">(~6h)</span>
    <span class="t-bullet">- Built Go timesheet server for AI agents</span>
    <span class="t-bullet">- CLI subcommands: serve/log/ls/tui/migrate/docs</span>
    <span class="t-bullet">- Smart log: server-first with direct DB fallback</span>

  <span class="t-cat">[security]</span> <span class="t-title">Audit CloudFront signed URLs</span> <span class="t-hours">(~2h)</span>
    <span class="t-bullet">- Reviewed token expiry and key rotation</span>
    <span class="t-bullet">- Updated IAM policy for least privilege</span>

  <span class="t-cat">[admin]</span> <span class="t-title">Xero bill entry via MCP</span> <span class="t-hours">(~0.5h)</span>
    <span class="t-bullet">- Created 6 supplier bills via API</span>

  <span class="t-dim">  &#x25BC; 4 more below</span>

  <span class="t-summary">  10 entries  ~18.5h  2.3d</span>
  <span class="t-dim">  j/k scroll  h/l day  g/G top/bottom  t today  q quit</span></div>
      </div>
    </div>
  </div>
</header>

<section class="pitch">
  <div class="container">
    <h2>The <span class="accent">problem</span></h2>
    <p class="pitch-intro">
      You run a dozen AI agents across different repos. At 5pm you have no idea what happened today. Each agent is sandboxed to its own working directory and can't write to a shared worklog.
    </p>
    <h2>The <span class="accent">fix</span></h2>
    <p class="pitch-intro">
      <strong>hrs</strong> is a local daemon that gives every agent a single HTTP endpoint to push structured work entries to. You get markdown files on disk and a terminal UI to see what got done.
    </p>
  </div>
</section>

<hr class="section-rule">

<section class="pitch" id="features">
  <div class="container">
    <h2>How it <span class="accent">works</span></h2>
    <div class="features-grid">
      <div class="feature">
        <div class="feature-icon">&#x1F4E1;</div>
        <h3>HTTP + CLI</h3>
        <p>Agents POST JSON to <code>localhost:9746/entries</code>. Humans use <code>hrs log</code>. Both work, same database.</p>
      </div>
      <div class="feature">
        <div class="feature-icon">&#x1F50D;</div>
        <h3>Self-discovery</h3>
        <p>Agents call <code>GET /schema</code> to learn the API. No hardcoded formats in your prompts.</p>
      </div>
      <div class="feature">
        <div class="feature-icon">&#x1F4C4;</div>
        <h3>Markdown on disk</h3>
        <p>Every entry lands in SQLite and renders to daily markdown files. Human-readable, git-friendly, grep-able.</p>
      </div>
      <div class="feature">
        <div class="feature-icon">&#x1F5A5;</div>
        <h3>Terminal UI</h3>
        <p>Vim-style TUI to browse entries by day. Scroll with j/k, switch days with h/l. All in your terminal.</p>
      </div>
      <div class="feature">
        <div class="feature-icon">&#x26A1;</div>
        <h3>Smart fallback</h3>
        <p>The CLI tries the server first, then writes directly to SQLite. Works whether the daemon is running or not.</p>
      </div>
      <div class="feature">
        <div class="feature-icon">&#x1F4E6;</div>
        <h3>Single binary</h3>
        <p>One Go binary. No runtime deps. <code>go install</code> and you're done. DB auto-creates at <code>~/.hrs/hrs.db</code>.</p>
      </div>
    </div>
  </div>
</section>

<hr class="section-rule">

<section class="install" id="install">
  <div class="container">
    <h2>Get <span class="accent">started</span></h2>
    <pre><code><span class="comment"># Install</span>
go install github.com/heuwels/hrs@latest

<span class="comment"># Start the daemon (optional — CLI works without it)</span>
hrs serve &amp;

<span class="comment"># Log from any agent or terminal</span>
hrs log -c dev -t "built auth flow" -b "oauth2 pkce,token refresh,tests" -e 3

<span class="comment"># See what happened today</span>
hrs ls

<span class="comment"># Or browse interactively</span>
hrs tui</code></pre>
    <p>See the <a href="/docs/getting-started">full docs</a> for agent integration, API reference, and migration from existing worklogs.</p>
  </div>
</section>

<footer>
  <div class="container">
    <nav class="footer-nav">
      <a href="/docs/">Docs</a>
      <span class="footer-sep">&middot;</span>
      <a href="/docs/api">API</a>
      <span class="footer-sep">&middot;</span>
      <a href="/docs/agent-integration">Agent Integration</a>
      <span class="footer-sep">&middot;</span>
      <a href="https://github.com/heuwels/hrs" target="_blank" rel="noopener noreferrer">GitHub</a>
    </nav>
    <p class="footer-copy"><em>hrs</em> &middot; timesheets for your agent &middot; a <a href="https://github.com/heuwels">heuwels</a> project &middot; MIT licensed</p>
  </div>
</footer>

<button type="button" class="theme-toggle" onclick="toggleTheme()" aria-label="Toggle dark mode">
  <span class="icon-sun">&#x2600;</span>
  <span class="icon-moon">&#x1F319;</span>
</button>

<script>
function toggleTheme(){
  var c=document.documentElement.getAttribute('data-theme');
  var n=c==='dark'?'light':'dark';
  document.documentElement.setAttribute('data-theme',n);
  localStorage.setItem('theme',n);
}
</script>
</body>
</html>`

const docsTmpl = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Title}} — hrs docs</title>
<style>` + siteCSS + `</style>
<script>
(function(){
  var t=localStorage.getItem('theme');
  if(t) document.documentElement.setAttribute('data-theme',t);
  else if(window.matchMedia('(prefers-color-scheme:dark)').matches) document.documentElement.setAttribute('data-theme','dark');
  else document.documentElement.setAttribute('data-theme','light');
})();
</script>
</head>
<body>

<header class="docs-header">
  <div class="container">
    <div class="breadcrumb"><a href="/">hrs</a> / <a href="/docs/">docs</a></div>
    <h1>{{.Title}}</h1>
  </div>
</header>

<section class="docs-body">
  <div class="container">
    {{.Body}}
  </div>
</section>

<footer>
  <div class="container">
    <nav class="footer-nav">
      <a href="/">Home</a>
      <span class="footer-sep">&middot;</span>
      <a href="/docs/">Docs</a>
      <span class="footer-sep">&middot;</span>
      <a href="https://github.com/heuwels/hrs" target="_blank" rel="noopener noreferrer">GitHub</a>
    </nav>
    <p class="footer-copy"><em>hrs</em> &middot; a <a href="https://github.com/heuwels">heuwels</a> project</p>
  </div>
</footer>

<button type="button" class="theme-toggle" onclick="toggleTheme()" aria-label="Toggle dark mode">
  <span class="icon-sun">&#x2600;</span>
  <span class="icon-moon">&#x1F319;</span>
</button>

<script>
function toggleTheme(){
  var c=document.documentElement.getAttribute('data-theme');
  var n=c==='dark'?'light':'dark';
  document.documentElement.setAttribute('data-theme',n);
  localStorage.setItem('theme',n);
}
</script>
</body>
</html>`

var (
	homeTemplate = template.Must(template.New("home").Parse(homeTmpl))
	docsTemplate = template.Must(template.New("docs").Parse(docsTmpl))
)

func renderMD(src string) (title, body string) {
	if m := mdHeading.FindStringSubmatch(src); m != nil && m[1] == "#" {
		title = m[2]
	}
	if title == "" {
		title = "hrs"
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

	s = mdHR.ReplaceAllString(s, "<hr>")

	// Lists
	s = mdUL.ReplaceAllString(s, "<li>$1</li>")
	s = strings.ReplaceAll(s, "<li>", "<ul><li>")
	s = strings.ReplaceAll(s, "</li>\n<ul><li>", "</li>\n<li>")
	s = strings.ReplaceAll(s, "</li>\n", "</li></ul>\n")
	s = strings.ReplaceAll(s, "</ul></ul>", "</ul>")

	// Links
	s = mdLink.ReplaceAllString(s, `<a href="$2">$1</a>`)

	// Bold
	s = mdBold.ReplaceAllString(s, "<strong>$1</strong>")

	// Italics
	s = regexp.MustCompile(`\*([^*]+)\*`).ReplaceAllString(s, "<em>$1</em>")

	// Paragraphs
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

func homeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	homeTemplate.Execute(w, nil)
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
	docsTemplate.Execute(w, struct {
		Title string
		Body  template.HTML
	}{title, template.HTML(body)})
}

// siteHandler serves the full microsite: homepage at /, docs at /docs/*
func siteHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", homeHandler)
	mux.HandleFunc("GET /docs/", http.StripPrefix("/docs", http.HandlerFunc(docsHandler)).ServeHTTP)
	mux.HandleFunc("GET /docs", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/docs/", http.StatusMovedPermanently)
	})
	return mux
}
