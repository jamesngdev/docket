---
name: open-pr
description: Push the ticket branch, open the GitHub pull request against the PR target branch, pass the wait_review gate and report it to docket.
---

# Open PR

1. `git push -u origin HEAD`
2. If a PR already exists for this branch (`gh pr view --json url -q .url`), push only, reuse its URL and update
   its body when needed (`gh pr edit <url> --body-file <file>`).
3. Otherwise `gh pr create --base <PR target from TICKET.md> --title "<ticket title> (#<id>)" --body-file <file>`.
   Write the body file to `$TMPDIR`, never inside the worktree (the gate requires a clean worktree).
   Body template (the gate rejects a body without the Acceptance and Tests sections):
   ```
   ## Summary
   <what changed and why, 2–5 lines>
   ## Acceptance criteria
   - [x] <criterion> — <evidence: test name / command output / file:line>
   ## Tests
   <commands run and their result lines>
   ## Notes
   <files outside "Planned files" and why · assumptions · team skills applied/skipped>
   ```
4. `docket ticket status <id> wait_review --pr <url>` — run it with a 10-minute shell timeout (600000 ms).
   This runs the **gate**: the branch must be committed and pushed, no `.docket/`, `.env` or secrets in the
   diff, no uncommitted or untracked files, the PR body follows the template, then the verify commands from
   `.docket/checks.yaml` run. A fake secret in a test fixture needs a `gate:allow-secret` comment on its line.
   If it fails, fix every listed problem, push, and run the same command again.
5. `docket run done --summary "PR <url>"`
