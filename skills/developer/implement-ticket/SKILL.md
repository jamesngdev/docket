---
name: implement-ticket
description: Implement a docket ticket inside the prepared git worktree, following TICKET.md requirements and acceptance criteria, with tests.
---

# Implement ticket

You are already inside the ticket's worktree on branch `ticket/<id>-<slug>`.

1. Read `TICKET.md` fully (requirements, acceptance criteria, planned files, repo notes, review feedback when
   this is a re-run) and `3TH_DOCUMENT.md`. The "Repo notes" section has the compile/test commands and
   conventions: use them, do not re-discover them.
2. Explore only the affected code with codegraph before editing. Follow the repo's existing patterns.
3. Implement the smallest change that satisfies every acceptance criterion. Stay inside "Planned files";
   if you truly must touch another file, keep it minimal and say why in the PR body.
4. Add or adjust tests proving each criterion (the `verify:` part of each item names the test or command).
   Run the repo-notes commands until green.
5. If TICKET.md has a review or change-request section, address every point and mention how in a PR comment.
6. Commit with conventional commit messages (1–3 commits). No secrets, no `.env`, no `.docket/` files.
7. Learned something non-obvious (needed env var, flaky test, hidden convention)? `docket note add --text "..."`.

If requirements are impossible or contradictory, do not guess silently: open the PR as a draft explaining the blocker.
