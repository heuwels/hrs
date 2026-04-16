# tui explorer

[← home](/)

---

```bash
hrs tui
```

vim-style keyboard navigation because we're not animals.

## keybindings

| key     | action              |
|---------|---------------------|
| `j`/`k` | scroll entries      |
| `h`/`l` | previous/next day   |
| `g`/`G` | jump to top/bottom  |
| `t`     | jump to today       |
| `tab`   | switch entries/goals view |
| `r`     | refresh             |
| `d`     | delete entry        |
| `e`     | show edit command   |
| `q`     | quit                |

## goals view

press `tab` to switch to the goals view. goals show checkboxes, completion
status, and linked entry counts.

| key     | action              |
|---------|---------------------|
| `j`/`k` | scroll goals        |
| `c`     | complete goal       |
| `u`     | reopen goal         |
| `d`     | delete goal         |
| `l`     | enter linking mode  |

### linking mode

press `l` on a goal to enter linking mode. use `j`/`k` to scroll entries and
`space` to toggle selection. press `enter` to confirm the link.

## hours caveat

the `hours_est` field asks the ai to estimate how long the work would
take a competent developer without ai assistance. this is useful for
understanding throughput, but take it with a grain of salt.

ai agents tend to overstate the complexity of tasks they've completed.
a "~4h" estimate for something that took 3 minutes of wall clock time
is flattering but not always realistic. the daily summaries roll these
up into person-hours and person-days which makes for a nice story, just
don't use it to plan your next sprint.

---

*[github](https://github.com/heuwels/hrs) · mit licensed*
