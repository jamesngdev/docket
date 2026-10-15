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
| Pencil.app | optional: agents read/edit `.pen` design files through the Pencil MCP (app must be open) |

The TUI header shows which of these are missing.

## Install

One command (macOS / Linux, amd64 / arm64) — downloads the latest release binary to `~/.local/bin/docket`:

```bash
curl -fsSL https://raw.githubusercontent.com/jamesngdev/docket/master/install.sh | sh
```

Pin a version or change the location with `DOCKET_VERSION=v0.1.0` / `DOCKET_INSTALL_DIR=/usr/local/bin` (set on `sh`).
Re-run the same command to upgrade. Check with `docket version`.

From source (Go 1.26.4+):

```bash
git clone git@github.com:jamesngdev/docket.git
cd docket
make install          # go install + symlink ~/.local/bin/docket
```

Make sure `~/.local/bin` is on your `PATH`.

### Releasing

Push a `v*` tag; the `release` GitHub Action runs tests and publishes binaries via GoReleaser:

```bash
git tag v0.1.0 && git push origin v0.1.0
```

## Quick start

```bash
docket                # open the TUI (project picker)
docket .              # open the project in this folder directly (or the new-project form, prefilled)
docket ~/code/my-api  # same, for any path inside a project
```

1. Press `n` to create a project: name, GitHub repo, local checkout path, docs repo and docs phase (branch),
   base / PR branch (default `develop`), parallel agents and models per role.
2. The daemon clones the docs, indexes the code and starts **TechLead**, which writes a shared
   `docker-compose.yml` for the dev database and asks its first questions.
3. Follow the Dashboard stepper (`Enter` or `n` runs the next step):
   answer questions → approve features → approve tickets.
4. Approved tickets are dispatched to **Developers**. Watch them in **Agents**, attach with `Enter`
   (detach with `Ctrl-D`).
5. Review and merge PRs on GitHub. docket notices the merge, marks the ticket done and unblocks its dependents.
   Or turn on **auto-merge** (`:automerge on`, `docket automerge on`, or "Auto merge: y" in the new-project form):
   once the Tester approves a PR and GitHub reports no conflicts, a TechLead run reads the diff, CI checks and
   acceptance criteria and then squash-merges it (deleting the branch), sends it back to the Developer, or flags it
   for you. PRs it leaves open while CI is still running are offered again after 15 minutes.

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
# discord_token: ...         # Discord bot token for Backend Support (set with `docket discord set-token`)
```

Per-project settings (parallel agents, base/PR branch, review rounds, models) are chosen when creating the
project and stored in its database.

## Discord Connect (Backend Support)

Teammates can ask a **read-only Backend Support agent** about the project from Discord. `@mention` the bot in the
project's channel → it opens a thread and answers there; @mention it again in that thread to follow up (it
remembers the conversation). Messages that don't mention the bot are ignored. It can read the docs, questions, features, tickets and the source code of the base
branch — nothing else, and it never writes anything.

Setup (once per machine, then once per project):

1. [Discord Developer Portal](https://discord.com/developers/applications) → New Application → **Bot** →
   Reset Token (copy it). No privileged intents are needed.
2. OAuth2 → URL Generator → scope `bot`, permissions *View Channels, Send Messages, Create Public Threads,
   Send Messages in Threads, Read Message History* → open the URL and add the bot to your server.
3. `docket discord set-token` (paste the token when asked; stored in `~/.config/docket/config.yaml`, shared by all projects).
4. In Discord enable Developer Mode, right-click the channel → Copy Channel ID, then in the project:
   `docket discord channel <channel-id>` or `:discord <channel-id>` in the TUI (`off` disables).

The project daemon connects within seconds (header shows `◉ discord`, Dashboard activity shows the connection
state). The daemon must be running — answers come from your machine. `docket discord status` shows the config.

**Dev database (optional).** Let Backend Support look at real dev data ("why can't user X log in?", "how many
orders are PENDING?"):

```bash
docket discord db                      # paste postgres://user:pass@host:5432/db or mongodb://host:27017/db
docket discord db off                  # remove it
```

The URL is checked by connecting before it is saved (project settings; `status` shows it with the password hidden).
When the answer depends on data, the agent queries it on its own through docket's built-in MCP server
(`docket support-db-mcp`), which reads the URL itself — the agent never sees it. Read-only is enforced by the server:
Postgres runs one prepared statement in a `READ ONLY` transaction that is always rolled back (20s timeout, ≤100 rows);
MongoDB exposes only find / count / aggregate (`$out`, `$merge` rejected). Point it at a **dev** database, ideally
with a read-only user — answers (and any data they quote) are visible to the whole channel.

How it stays read-only: each answer is a headless `claude -p` call (resumed per thread) in an isolated config dir
with **no shell** — only Read, Grep, Glob (plus the read-only dev-database tools when one is set). It runs in a detached worktree of `origin/<base branch>` (tracked files
only — no `.env`, no `.docket/` data); reads outside that worktree, the docs and its context folder are denied, and
project state (questions, features, full tickets, recent git log) is pre-rendered into the context folder. At most
2 answers run at once per project and one per thread; extra messages get a "busy" reply instead of queueing.
Anyone who can post in the channel can ask, so pick a channel whose members may see the code.

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
- **Agents** — `claude` in tmux with `--setting-sources user`, a strict MCP config (codegraph, plus Pencil when installed) and an
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
- Agents push branches and open PRs but never merge — merging is yours, unless you turn on auto-merge for a
  project (off by default): then TechLead squash-merges Tester-approved PRs it judges complete.
- Backend Support (Discord) is the exception: no shell, read-only tools, default permission mode (anything
  outside its folders is denied). Its answers are visible to everyone in the channel.
