# docket — codebase summary

Go 1.26 · Bubble Tea · SQLite (modernc, pure Go) · tmux · `claude` CLI.

## Run

```bash
make install                         # go install + link ~/.local/bin/docket (on PATH)
docket auth set-token <token>        # once; token from `claude setup-token` (agents run with an isolated config dir)
docket                               # TUI: create/open a project
docket daemon status|stop            # inside a project checkout (or --project <repo>)
make test                            # vet + tests
```

## Layout

| Path | Responsibility |
| ---- | -------------- |
| `main.go` | No args → TUI, otherwise CLI subcommands |
| `internal/paths` | Every path under `<repo>/.docket/`, socket path, global config dir |
| `internal/config` | `~/.config/docket/config.yaml`: registered projects, OAuth token (0600) |
| `internal/usage` | Claude subscription usage (5h/weekly limits) from the OAuth usage API; reads the token, never refreshes it |
| `internal/store` | SQLite schema + CRUD: project, documents, questions, features, tickets, third_docs, runs, activity; `Dispatchable` |
| `internal/store/phases.go` | Phases = docs branches: active phase, per-phase docs scan + commits, phase-scoped queries |
| `internal/docscan` | Hash docs `*.md`, diff vs previous scan (A/M/D) |
| `internal/gitops`, `internal/gh`, `internal/tmux`, `internal/sh` | Thin wrappers over git, gh, tmux, exec |
| `skills/` | Embedded skill bundle + `manifest.yaml` (role → skills). The only skills agents ever get |
| `internal/agent` | Isolated `CLAUDE_CONFIG_DIR` per role, context files (PROJECT.md, TICKET.md, 3TH_DOCUMENT.md), kickoff prompts, tmux launch |
| `internal/discord` | Discord Connect: per-daemon gateway connection, @mention → thread, follow-ups resume the thread's session, chunked replies |
| `internal/support` | Read-only Backend Support: headless `claude -p` (+`--resume`), no shell (Read/Grep/Glob only) in a detached base-branch worktree; project state + git log pre-rendered into its context dir |
| `internal/daemon` | Loop every 5s + on kick: reap runs, review loop, start tester, dispatch developers, poll PRs (60s); TechLead tasks; docs rescan; compose up |
| `internal/ipc` | Newline-JSON over unix socket (ping, kick, techlead, rescan, stop, restart, compose, shutdown) |
| `internal/cli` | Agent-facing commands (`question/feature/ticket/review/run`), `show`, `daemon`, `auth` |
| `internal/project` | New project: clone, `.docket/` layout, git info/exclude, docs clone, DB, register |
| `internal/tui` | Picker, new-project form, workspace (dashboard stepper, questions, features, tickets, agents, docs), textarea editor, `:` commands |

## Run lifecycle

1. Daemon prepares context files and the role's isolated home (skills copied from the bundle), then starts
   `claude --setting-sources user --strict-mcp-config --mcp-config .docket/mcp.json --dangerously-skip-permissions`
   in tmux session `docket-<slug>-<role>-<ticket|run>`; pane output is piped to `.docket/logs/<run>.log`.
2. The agent records results through `docket …` (env `DOCKET_PROJECT`, `DOCKET_RUN`, `DOCKET_DATA`) and finishes with
   `docket run done`.
3. Next tick: callback seen → session killed, run `done`; session gone without callback → run `failed`.
4. `onFinished`: bootstrap → compose up; developer → unlock (or back to pending + `needs_user`); tester → remove review worktree.

## Known limits / to verify

- Isolation spike (REQUIREMENT §7): confirm `--setting-sources user` also blocks project-scope `.claude/` skills and CLAUDE.md.
- The TUI reloads the whole snapshot every 2s; fine for hundreds of tickets, not tuned beyond that.
