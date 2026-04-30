# ReTrace — Roadmap

> A screentime & usage insights tool for Linux/Windows, built with Go, SQLite, Cobra, and BubbleTea.

---

## Phase 1 — Foundation
> Goal: Bare minimum project structure with a working database layer.

- [x] Initialize Go project (`go mod init`)
- [x] Set up project folder structure (`/cmd`, `/internal`, `/db`, `/ui`)
- [x] Design SQLite schema — `sessions` table
- [x] Implement DB connection + auto-migration on startup
- [x] Write basic session CRUD functions

**Schema (sessions):**
```sql
CREATE TABLE sessions (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    app       TEXT NOT NULL,
    title     TEXT,
    started_at DATETIME NOT NULL,
    ended_at  DATETIME NOT NULL,
    duration  INTEGER NOT NULL  -- seconds
);
```

---

## Phase 2 — Data Collection
> Goal: A background daemon that polls the active window and writes sessions to SQLite.

- [x] Active window polling loop (configurable interval, default 2s)
  - [x] Linux: via `xdotool getactivewindow getwindowname` + `getwindowpid`
  - [x] Windows: via `user32.dll` — `GetForegroundWindow`, `GetWindowText`
- [x] Session aggregation — coalesce continuous polling events into sessions
- [x] Write completed sessions to SQLite
- [x] Daemon lifecycle — start, stop, status (PID file based)

---

## Phase 3 — CLI with Cobra
> Goal: A usable command-line interface to query your data.

- [ ] `retrace daemon start` / `stop` / `status`
- [ ] `retrace today` — app usage breakdown for today
- [ ] `retrace week` — weekly summary
- [ ] `retrace app <name>` — stats for a specific app
- [ ] `retrace stats` — top apps, total screen time, most active hour

---

## Phase 4 — Insights Engine
> Goal: Surface patterns beyond raw time totals.

- [ ] Peak usage hour per app ("You use VSCode most at 11pm")
- [ ] Daily & weekly usage trends per app
- [ ] Most context-switched hours of the day
- [ ] Longest single session per app
- [ ] Daily focus score (% of time on categorized "work" apps)

---

## Phase 5 — BubbleTea TUI
> Goal: A full terminal dashboard replacing the plain CLI output.

- [ ] Dashboard view — today's top apps with usage bars
- [ ] Hourly heatmap view
- [ ] Insights panel
- [ ] App detail view (select app → see its trends)
- [ ] Keyboard navigation between views
- [ ] Color-coded app categories

---

## Phase 6 — Polish
> Goal: Make it configurable, portable, and shareable.

- [ ] Config file (`~/.config/trace/config.toml`) — poll interval, ignored apps, categories
- [ ] App categorization (work / creative / social / entertainment / system)
- [ ] Export data to CSV or JSON (`trace export`)
- [ ] Cross-platform install script / packaging
- [ ] README with demo screenshot/gif

---

## Stack

| Layer | Tool |
|---|---|
| Language | Go |
| Database | SQLite (`modernc.org/sqlite`) |
| CLI | Cobra |
| TUI | BubbleTea + Lipgloss |
| Window tracking (Linux) | `xdotool` |
| Window tracking (Windows) | `user32.dll` via `syscall` |

---

## Milestone Timeline

| Milestone | Target |
|---|---|
| Phase 1 — Foundation | Week 1 |
| Phase 2 — Data Collection | Week 1–2 |
| Phase 3 — CLI | Week 2 |
| Phase 4 — Insights | Week 3 |
| Phase 5 — TUI | Week 3–4 |
| Phase 6 — Polish | Week 4 |
