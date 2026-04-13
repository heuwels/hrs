
```
            ┌─────────┐
            │   hrs   │
            │ ▓▓▓░░░  │
            │ ▓▓▓▓░░  │
            │ ▓▓▓▓▓░  │
            └─────────┘
```

# hrs

timesheets for your agent.

---

a tiny http server backed by sqlite that ai agents can post work entries
to from any directory. renders markdown for humans.

- [getting started](/getting-started)
- [api reference](/api)
- [agent integration](/agent-integration)
- [tui explorer](/tui)
- [github](https://github.com/kollwitz-owen/hrs)

---

## the pitch

you run a dozen ai agents across different repos. at 5pm you have no
idea what happened today. hrs gives every agent a single endpoint
to push structured work entries to. you get markdown files and a tui
to see what got done.

```
POST http://localhost:9746/entries

{
  "category": "dev",
  "title": "built auth flow",
  "bullets": ["oauth2 pkce", "token refresh", "tests"],
  "hours_est": 3
}
```

agents self-discover fields via `GET /schema`. no hardcoded formats
in your prompts.

---

*a [ko promotions](https://github.com/kollwitz-owen) project. mit licensed.*
