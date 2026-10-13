# Discord Connect — Backend Support agent

Status: implemented (Discord side needs a live bot token to verify) · Branch: master (uncommitted)

## Decisions (confirmed with user)
- @mention bot in project channel → bot opens a thread; @mention again in that thread resumes the same Claude session. Non-mention messages are always ignored (no privileged Message Content intent needed).
- Anyone who can post in the channel may ask (Discord channel perms = access control).
- One bot token (global config), one channel per project (project settings).
- Backend Support is strictly read-only.

## Architecture
- `config.DiscordToken` (global, 0600) · `Settings.DiscordChannel` (per project) · `discord_threads` table (thread → claude session).
- Each project daemon keeps one gateway connection (discordgo) when token+channel set; messages filtered by channel / known thread. Token/channel changes picked up on daemon tick.
- `internal/support`: headless `claude -p --output-format json [--resume]` in isolated home (role `support`):
  - cwd = detached worktree `.docket/worktrees/support` of `origin/<base>` (tracked files only → no .env / .docket secrets reachable by Grep/Read)
  - `--tools Read,Grep,Glob,Skill` — NO Bash (review found `git diff --no-index`/`--output` escapes); default permission mode + deny secret reads; `--add-dir` docs + context (PROJECT.md, TICKETS.md, GIT_LOG.md, 3TH_DOCUMENT.md); minimal env.
- Replies chunked to Discord 2000-char limit, typing indicator; max 2 concurrent answers, 1 per thread, extra messages get a "busy" reply (no queue); Close() cancels running answers.
- UX: CLI `docket discord set-token|clear-token|channel <id>|off|status`, TUI `:discord <channel-id|off>`, header chip when enabled.

## Verified
- `claude -p` in default mode denies Read and Grep outside cwd/add-dirs (deterministic permission_denials), deny rules block Read of .env; Grep ignores Read deny rules → mitigated by the tracked-files-only worktree.
- Live test `SUPPORT_LIVE=1 go test ./internal/support`: answer with citations, resume, no secret leak, no writes.

## Out of scope
- codegraph MCP for support (index not present in the support worktree); DM support; slash commands.
