# docket

**Docs in, pull requests out.** docket is a terminal UI that reads your project documentation, asks you the
questions that matter, plans features and tickets, and then orchestrates a team of isolated
[Claude Code](https://claude.com/claude-code) agents that implement, review and ship those tickets as GitHub PRs.

```
docs repo ──► TechLead ──► questions ──► features ──► tickets ──► Developers ──► PRs ──► Tester ──► you merge
               (you answer)  (you approve)  (you approve)   (parallel, per worktree)   (review)
```

You stay in control at three gates — answering questions, approving features, approving tickets — and on every
pull request. Everything else runs in the background.

---

## Features

- **Doc-driven planning** — TechLead scans a Markdown docs repo, asks targeted questions, then writes features
  and dependency-ordered tickets with testable acceptance criteria. Code is read through
  codegraph.
- **Phases** — each branch of the docs repo is a phase. Switch phases and TechLead only plans what changed
  compared to the previous phase.
- **Parallel developers** — tickets are dispatched as soon as their dependencies are merged, up to N agents at
  once, each in its own git worktree and branch, sharing one docker-compose dev database.
- **Review loop** — a Tester agent runs the tests and reviews every PR; requested changes send the ticket back to
  its Developer automatically (bounded rounds).
- **Merge conflicts handled** — open PRs are checked continuously; a conflicting PR is rebased and resolved by a
  Developer using the project context.
- **Chat with the agents** — ask an Advisor to explain any question, chat with TechLead to change a ticket (and
  its related tickets), or describe new work and let TechLead create the tickets.
- **Isolated agents** — agents never see your personal `~/.claude` skills, commands, hooks or MCP servers. They
  get docket's bundled skills plus your team skill repos, pulled fresh on every run, while reusing your normal
  `claude` login.
- **Background daemon** — dispatching, PR polling and reviews keep going after you close the TUI. Agents run in
  tmux, so you can attach to any of them and watch or intervene.

## Requirements

| Tool | Why |
| ---- | --- |
| Go 1.26.4+ | install docket |
| `claude` (logged in) | the agents |
| `git`, `gh` (authenticated) | worktrees, PRs, reviews |
| `tmux` | hosts every agent session |
| `docker` + `docker compose` | shared dev database |
| `codegraph` | code navigation for agents (optional but recommended) |

The TUI header shows which of these are missing.

## Install

```bash
git clone git@github.com:jamesngdev/docket.git
cd docket
make install          # go install + symlink ~/.local/bin/docket
```

Make sure `~/.local/bin` is on your `PATH`.

## Quick start

```bash
docket                # open the TUI
```

1. Press `n` to create a project: name, GitHub repo, local checkout path, docs repo and docs phase (branch),
   base / PR branch (default `develop`), parallel agents and models per role.
2. The daemon clones the docs, indexes the code and starts **TechLead**, which writes a shared
   `docker-compose.yml` for the dev database and asks its first questions.
3. Follow the Dashboard stepper (`Enter` or `n` runs the next step):
   answer questions → approve features → approve tickets.
4. Approved tickets are dispatched to **Developers**. Watch them in **Agents**, attach with `Enter`
   (detach with `Ctrl-b d`).
5. Review and merge PRs on GitHub. docket notices the merge, marks the ticket done and unblocks its dependents.

## The TUI

Sidebar views (`Tab` / `1`–`6`): Dashboard · Questions · Features · Tickets · Agents · Docs.

| Key | Action |
| --- | ------ |
| `j` `k` `g` `G` | move · top · bottom |
| `Enter` | primary action (answer, attach agent, switch phase, preview) |
| `n` | run the next pipeline step |
| `a` / `A` | approve / approve all |
| `e` `t` `C` `d` | edit requirements/description · title · acceptance · dependencies |
| `x` | reject / stop agent / mark outdated |
| `c` / `X` | chat with an agent about the selected question or ticket / end that chat |
| `N` | (Tickets) describe new work to TechLead, which creates the ticket(s) |
| `h` `l` | (Tickets) status filter — `wait_review` also triggers a PR/conflict check |
| `o` / `L` | open PR in browser / open agent log |
| `:` | commands: `:phase <branch>` `:rescan` `:dispatch` `:techlead <task>` `:compose` `:daemon stop` `:project` `:q` |
| `?` / `q` | help / back to projects |

Ticket statuses: `backlog` (needs approval) → `pending` → `processing` → `wait_review` → `done`
(plus `outdated`). Markers: `⚠` needs you · `⚔` PR conflict · `💬` chat running.

## CLI

The same binary is the CLI that agents (and you) use. Inside a project checkout, or with `--project <repo>`:

```bash
docket phase list | phase set <docs-branch>
docket show questions|features|tickets|ticket <id>
docket ticket update <id> [--title ..] [--requirements-file f] [--acceptance-file f] [--depends 1,2]
docket ticket redo <id> --reason "..."      # rework an in-flight ticket on its branch
docket daemon status|stop
docket auth set-token <sk-ant-…> | auth clear-token
docket help
```

## Configuration

`~/.config/docket/config.yaml`

```yaml
projects:                    # registered project checkouts (managed by the TUI)
  - /path/to/repo
team_skill_repos:            # skills given to Developers and TechLead, pulled on every run
  - https://github.com/jamesngdev/be-skills
# oauth_token: sk-ant-...    # optional dedicated agent token; default reuses your `claude` login
```

Per-project settings (parallel agents, base/PR branch, review rounds, models) are chosen when creating the
project and stored in its database.

## How it works

```
<repo>/.docket/                 (excluded from git automatically)
  docket.sqlite                 project state: phases, docs, questions, features, tickets, runs, activity
  docs/                         clone of the docs repo (checked out at the active phase)
  worktrees/<ticket>/           one git worktree per ticket branch
  claude-home/<role>/           isolated CLAUDE_CONFIG_DIR per role (skills, settings, CLAUDE.md)
  context/<run>/                PROJECT.md / TICKET.md / 3TH_DOCUMENT.md handed to each agent
  docker-compose.yml, dev-db.env shared dev database written by TechLead
  logs/                         daemon log + full tmux output of every agent run
```

- **Daemon** (`docket daemon`, auto-started) — every 5 s: reap finished runs, run the review loop, start testers,
  dispatch developers; every 60 s: poll PRs (merged / closed / conflicting). The TUI and agents talk to it over a
  unix socket; state lives in SQLite, so restarts resume cleanly.
- **Agents** — `claude` in tmux with `--setting-sources user`, a strict MCP config (codegraph only) and an
  isolated config dir. They report back through the `docket` CLI and finish with `docket run done`.
- **Roles** — TechLead (planning, ticket chat, new-ticket chat), Developer (implement, rework, resolve conflicts),
  Tester (review-only), Advisor (explains questions).

Details: [`REQUIREMENT.md`](REQUIREMENT.md), [`docs/tui-ui-design.md`](docs/tui-ui-design.md),
[`docs/codebase-summary.md`](docs/codebase-summary.md).

## Development

```bash
make test        # go vet + go test
make install     # rebuild and relink; running daemons restart on the new binary automatically
```

Bundled agent skills live in [`skills/`](skills) (embedded in the binary); `skills/manifest.yaml` decides which role
gets which skill.

## Safety notes

- Agents run with `--dangerously-skip-permissions` inside their worktrees. Only use docket on repositories and
  machines where that is acceptable.
- Agents push branches and open PRs but never merge — merging is always yours.
