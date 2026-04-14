# tui explorer

[← home](/)

---

```bash
hrs tui -db ~/hrs.db
```

vim-style keyboard navigation because we're not animals.

## keybindings

| key     | action              |
|---------|---------------------|
| `j`/`k` | scroll entries      |
| `h`/`l` | previous/next day   |
| `g`/`G` | jump to top/bottom  |
| `t`     | jump to today       |
| `r`     | refresh             |
| `q`     | quit                |

## hours caveat

the `hours_est` field asks the ai to estimate how long the work would
take a competent developer without ai assistance. this is useful for
understanding throughput, but take it with a grain of salt.

ai agents tend to overstate the complexity of tasks they've completed.
a "~4h" estimate for something that took 3 minutes of wall clock time
is flattering but not always realistic. the daily summaries roll these
up into "person-days saved" which makes for a nice story — just don't
use it to plan your next sprint.

---

*[github](https://github.com/heuwels/hrs) · mit licensed*
