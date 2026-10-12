# po-tui MVP — implementation plan

Source: `REQUIREMENT.md`, `docs/tui-ui-design.md`. Mode: cook --auto.

| # | Phase | Packages | Status |
|---|-------|----------|--------|
| 1 | Foundation: paths, global config, SQLite store, shell/git/gh/tmux helpers | `internal/{paths,config,store,sh,gitops,gh,tmux}` | done |
| 2 | Bundled skills + isolated claude-home builder | `skills/`, `internal/agent` | done |
| 3 | Agent spawn + context files (TICKET.md, PROJECT.md, 3TH_DOCUMENT.md) | `internal/agent` | done |
| 4 | Daemon: dispatch, run reaping, review loop, tester trigger, PR polling, techlead tasks, docs rescan, unix socket | `internal/daemon`, `internal/ipc` | done |
| 5 | Agent-facing CLI (`po-tui question/feature/ticket/review/run ...`) + project create | `internal/cli`, `internal/project` | done |
| 6 | TUI (Bubble Tea): picker, new-project form, dashboard stepper, questions, features, tickets, agents, docs, textarea, `:` commands | `internal/tui` | done |
| 7 | Tests + review + docs (8 review findings fixed) | `*_test.go`, `docs/` | done |

Key decisions beyond the spec
- Shared state lives in SQLite (WAL). TUI and CLI write data rows directly; the socket is used for control
  (techlead run, rescan, stop/restart, kick). Avoids duplicating every CRUD op as an RPC.
- Agents run interactive `claude` in tmux. `po-tui run done` = callback; daemon then captures the pane and kills the session.
- Stopping a developer run moves its ticket back to `backlog` so it is not instantly re-dispatched.
- In views, `h`/`l` drive the view (e.g. ticket status filter); views are switched with `Tab`/`Shift-Tab`/`1`-`6`.
