---
name: implement-ticket
description: Implement a docket ticket inside the prepared git worktree, following TICKET.md requirements and acceptance criteria, with tests.
---

# Implement ticket

You are already inside the ticket's worktree on branch `ticket/<id>-<slug>`.

1. Read `TICKET.md` fully (requirements, acceptance criteria, review feedback when this is a re-run) and `3TH_DOCUMENT.md`.
2. Explore the affected code with codegraph before editing. Follow the repo's existing patterns and conventions.
3. Implement the smallest change that satisfies every acceptance criterion.
4. Add or adjust tests proving each criterion. Run the project's test, lint and compile commands until green.
5. If TICKET.md has a review section, address every point and mention how in a PR comment.
6. Commit with conventional commit messages (1–3 commits). No secrets, no `.docket/` files.

If requirements are impossible or contradictory, do not guess silently: open the PR as a draft explaining the blocker.
