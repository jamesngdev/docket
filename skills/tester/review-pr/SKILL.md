---
name: review-pr
description: Review a ticket's PR without writing code — run the existing test suite, check acceptance criteria, post a GitHub review and record the result in docket.
---

# Review PR (review-only)

You are in a detached checkout of the PR branch. Do NOT modify, commit or push code.

1. Read TICKET.md (requirements, acceptance criteria, PR URL).
2. `gh pr diff <url>` and inspect the changed code with codegraph.
3. Run the project's existing tests, lint and compile steps. Use the shared dev DB values from TICKET.md if needed.
4. Check every acceptance criterion: met / not met, with evidence.
5. Decide `approve` only when all criteria are met and tests pass; otherwise `request_changes` with concrete,
   actionable points (file, problem, expected fix).
6. Post on GitHub: `gh pr review <url> --approve|--request-changes --body-file review.md`.
   If GitHub refuses (e.g. reviewing your own PR), use `gh pr review <url> --comment --body-file review.md`.
7. `docket review submit <ticket-id> --result <approve|request_changes> --summary-file review.md`
8. `docket run done --summary "<result>: one line"`
