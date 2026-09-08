# hrs macOS GUI

Native SwiftUI companion for [hrs](https://github.com/heuwels/hrs). It draws
the link between **strategic** goals and **tactical** (daily) goals so unaligned
work is impossible to ignore.

The rule the graph is built around: only work on a tactical goal if it informs
a strategic one. Drag a goal onto a strategy to link. Drag it into the
Unaligned column to unlink.

## Run

`hrs serve` must be listening on `127.0.0.1:9746` (the default).

```bash
# from the hrs repo root
make gui
```

Or:

```bash
cd gui-macos
swift run HrsGUI
```

The base URL is `http://127.0.0.1:9746` by default (`HRS_URL` overrides it, then
the inspector footer). The app never opens the SQLite file; every read and write
goes through the REST API.

Drag-to-unlink and "Show completed goals" need the API additions on this branch
(`PUT /goals/{id}` with `strategy_id: null`, and `GET /goals?all=1`). Linking
and the default open-goals graph already work against hrs 1.7.0.

## Layout

| Column | What |
|---|---|
| Strategic | Multi-day strategies. Drop a goal anywhere on a lane to link it. |
| Tactical | Open goals already linked to that strategy. Lines are the `strategy_id` relation. |
| Unaligned | Open goals with no strategic parent. Drop here to unlink. |

Completed strategies stay hidden unless you turn them on, but a completed
strategy that still has open goals remains visible so those goals do not look
orphaned. Close or archive a strategy from the inspector (or the card's
context menu). Linked tactical goals stay put; Reopen brings the strategy
back.

## API surface

- `GET /strategies`
- `GET /strategies/{id}` (hours / goal counts on the cards)
- `GET /goals/active` (default)
- `GET /goals?all=1` (when "Show completed goals" is on)
- `PUT /goals/{id}` with `{"strategy_id": N}` or `{"strategy_id": null}`
- `PUT /goals/{id}/done` and `/undo`
