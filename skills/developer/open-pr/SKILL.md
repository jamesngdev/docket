---
name: open-pr
description: Push the ticket branch, open the GitHub pull request against the PR target branch, and report it to docket.
---

# Open PR

1. `git push -u origin HEAD`
2. If a PR already exists for this branch (`gh pr view --json url -q .url`), push only and reuse its URL.
3. Otherwise `gh pr create --base <PR target from TICKET.md> --title "<ticket title> (#<id>)" --body-file <file>`.
   Body: summary, how each acceptance criterion is met, test commands run and their results.
4. `docket ticket status <id> wait_review --pr <url>`
5. `docket run done --summary "PR <url>"`
