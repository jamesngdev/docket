# docket desktop (macOS) — plan

Target user: TechLead-level devs who know code + Claude. Stack: **Wails v2 + React + TS + xterm.js**, Orca-like UI.
TUI stays; desktop is a second client of the same engine (SQLite + daemon + CLI). No engine fork.

## Architecture
- `desktop/` = Wails app (package main) in the same Go module; reuses `internal/*`.
- One binary, two modes: no args → GUI; args → `cli.Run` (so `docket daemon` / agent callbacks work from the .app).
- Shared service layer `internal/appsvc` (snapshot, pipeline counts, next step, approve rules) used by TUI + desktop.
- Agents keep running in tmux; GUI never owns agent processes.
- Fixes needed for a GUI-launched engine: login-shell PATH (Finder apps get a minimal PATH), a `docket` shim in
  `.docket/bin` for agents, daemon version check = "newer binary wins" (TUI and app binaries coexist).

## Phases
| # | Phase | Status |
|---|-------|--------|
| 1 | Shell: projects + create wizard, health/onboarding, dashboard stepper, questions (option buttons), features, kanban (drag backlog→pending), agents list, docs + phase switch, chats/attach via Terminal.app | done |
| 2 | Live agents: PTY `tmux attach` → xterm.js panes, agents grid, chats in side panel | todo |
| 3 | PR review: diff viewer, line comments → `ticket redo`, CI/conflict panel | todo |
| 4 | Polish/ship: notifications, menu bar, dependency graph, usage meter, signing/notarization, brew cask | todo |

## Risks
- Wails v2 needs build tags; `go vet ./...` must keep passing without them.
- Unsigned app → Gatekeeper warning until a Developer ID is set up (phase 4).
