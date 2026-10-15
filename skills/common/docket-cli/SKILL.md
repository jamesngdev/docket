---
name: docket-cli
description: How to read and write docket project state (questions, features, tickets, reviews, run completion) with the `docket` CLI. Use whenever you need to record output or finish your run.
---

# docket CLI

`DOCKET_PROJECT`, `DOCKET_RUN` and `DOCKET_DATA` are already set in your environment, so commands need no project flag.
For multi-line text, write it to a temp file and pass the `--*-file <path>` variant instead of the inline flag.

## Read
- `docket show questions` · `docket show features` · `docket show tickets` · `docket show ticket <id>`

## Write (TechLead)
- `docket question add --text "..." --source "docs/auth.md §3"`
- `docket question answer <id> --text "..."` (Advisor, only when the user asks to record a decision)
- `docket feature add --title "..." --description-file f.md --docs docs/a.md,docs/b.md` → prints the new id
- `docket feature outdated <id>`
- `docket ticket add --feature <id> --title "..." --requirements-file r.md --acceptance-file a.md --files a/,b.go --depends 3,4 [--model haiku|opus]` → prints id
  (every acceptance item must end with `— verify: <test or command>`; `--files` is required)
- `docket ticket outdated <id>`
- `docket ticket update <id> [--title] [--requirements-file] [--acceptance-file] [--depends] [--files] [--model]` (not for done tickets)
- `docket ticket redo <id> --reason-file why.md` (rework an in-flight ticket on its branch)
- `docket ticket merge <id>` (auto-merge only: squash-merge a Tester-approved PR) · `docket ticket flag <id> --reason "..."` (needs the user)

## Write (Developer)
- `docket ticket status <id> wait_review --pr <url>` — after the PR is open. Runs the gate (skill open-pr):
  use a 10-minute shell timeout; on failure fix the listed problems, push, retry.

## Write (any role)
- `docket note add --text "..."` — one-line repo fact for every later agent (appended to `.docket/REPO.md`).

## Write (Tester)
- `docket review submit <ticket-id> --result approve|request_changes --summary-file review.md`

## Finish — MANDATORY for every role
- `docket run done --summary "one line of what you did"`

This is the only signal that your run succeeded. If your session ends without it, the run is marked failed.
Call it last: your session is closed right after.
