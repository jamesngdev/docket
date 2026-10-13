---
name: merge-prs
description: Auto-merge gate — review each Tester-approved PR listed in MERGE.md and decide to squash-merge it, send it back to the Developer, or flag it for the user.
---

# Auto-merge review (TechLead)

Auto-merge is on for this project. For EVERY ticket in `MERGE.md` make exactly one decision. You are the last
gate before code lands on the PR target branch, so be strict: when in doubt, do not merge.

## Inspect each PR
1. `gh pr view <url> --json title,body,state,mergeable,reviewDecision,statusCheckRollup,files`
2. `gh pr checks <url>` — CI status.
3. `gh pr diff <url>` — read the actual change (use codegraph for surrounding code when needed).
4. Compare with the ticket's requirements, acceptance criteria and the Tester review in `MERGE.md`.

## Decide
- **Merge** when ALL hold: PR is open and mergeable; every required check passed (no failing or pending checks;
  a repo with no checks is fine); the diff implements the requirements and meets every acceptance criterion; no
  unrelated changes, debug code, secrets or disabled tests; the Tester review raised nothing still open.
  → `docket ticket merge <id>` (squash-merge + delete branch; the ticket becomes done automatically).
- **Send back** when the code is wrong or incomplete (missed acceptance criterion, bug, unrelated change):
  write precisely what must change to a file, then `docket ticket redo <id> --reason-file why.md`.
- **Flag for the user** when it needs a human: failing CI you cannot attribute, risky change (migrations that drop
  data, auth/security, breaking API), product decision, or anything ambiguous:
  `docket ticket flag <id> --reason "<one line why>"`.
- **Leave it** only when checks are still running: do nothing for that ticket — it is offered again in ~15 minutes.

Never merge a PR that is not in `MERGE.md`, never use `gh pr merge` directly, never push to any branch.

## Finish
`docket run done --summary "merged #a #b; sent back #c; flagged #d; waiting #e"`
