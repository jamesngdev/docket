---
name: resolve-conflicts
description: Resolve merge conflicts of a ticket's open PR against its target branch using project context — rebase, keep both sides' intent, test, force-push with lease, verify mergeable.
---

# Resolve PR merge conflicts

You are in the ticket's worktree. The PR target branch is in TICKET.md ("PR target").

1. Sync: `git fetch origin`, `git checkout <ticket branch>`, `git reset --hard origin/<ticket branch>`
   (the remote branch is the source of truth for the PR).
2. Rebase onto the target: `git rebase origin/<PR target>`. Never merge the target into the branch.
3. For every conflicted file:
   - Understand BOTH sides: this ticket (TICKET.md requirements/acceptance) and the incoming change
     (`git log -p origin/<PR target> -- <file>`, "Recently merged tickets" in TICKET.md, codegraph).
   - Keep both intents. Never drop already-merged behaviour to make this ticket pass; adapt this ticket's code
     to the new base instead (renamed symbols, moved files, new signatures, migration order).
   - Migrations / lockfiles: regenerate rather than hand-merge when the tool supports it; renumber migrations
     so they come after the merged ones.
   - `git add <file>` then `git rebase --continue`.
4. Run the project's compile, lint and test commands. Fix only what the rebase broke.
5. `git push --force-with-lease origin HEAD:<ticket branch>`
6. Verify `gh pr view <url> --json mergeable -q .mergeable` is not CONFLICTING (UNKNOWN: wait a few seconds, retry).
7. Comment on the PR what you resolved and why: `gh pr comment <url> --body-file notes.md`.
8. `docket ticket status <id> wait_review --pr <url>` (10-minute shell timeout: it runs the gate, see skill
   open-pr; fix and retry until it passes) then `docket run done --summary "conflicts resolved"`.

If a conflict needs a product decision: `git rebase --abort`, explain the exact conflict in a PR comment, and finish
with `docket run done --summary "unresolved: <reason>"` WITHOUT setting the status (the ticket is flagged for the user).
