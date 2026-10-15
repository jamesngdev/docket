---
name: review-pr
description: Review a ticket's PR without writing code — check acceptance criteria against the diff, run tests only where the gate and CI do not already prove them, post a GitHub review and record the result in docket.
---

# Review PR (review-only)

You are in a detached checkout of the PR branch. Do NOT modify, commit or push code.

1. Read TICKET.md (requirements, acceptance criteria, planned files, gate line, repo notes, PR URL).
2. `gh pr diff <url>` and inspect the changed code with codegraph. `gh pr checks <url>` for CI.
3. Tests — avoid duplicate work:
   - The "Gate" line in TICKET.md means the verify commands of `.docket/checks.yaml` already passed on that
     commit. If it matches the PR head (`gh pr view <url> --json headRefOid -q .headRefOid`), do NOT re-run the
     full suite; run only the specific tests/commands named in the `verify:` parts of the criteria.
   - No gate line, a different head, or CI failing: run the repo-notes test commands yourself.
4. Check every acceptance criterion: met / not met, with evidence. Also check: changes stay within the planned
   files (or the PR explains why), no debug code, no disabled or deleted tests, conventions from repo notes.
5. Decide `approve` only when all criteria are met and tests pass; otherwise `request_changes` with concrete,
   actionable points (file:line, problem, expected fix). On a re-review, check each earlier point explicitly.
6. Post on GitHub: `gh pr review <url> --approve|--request-changes --body-file review.md`.
   If GitHub refuses (e.g. reviewing your own PR), use `gh pr review <url> --comment --body-file review.md`.
7. `docket review submit <ticket-id> --result <approve|request_changes> --summary-file review.md`
8. `docket run done --summary "<result>: one line"`
