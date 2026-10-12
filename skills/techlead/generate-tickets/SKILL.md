---
name: generate-tickets
description: Split approved features into small, dependency-ordered tickets with requirements and acceptance criteria via `docket ticket add`.
---

# Generate tickets

Only for features listed in PROJECT.md as approved without tickets.

Each ticket must be completable by one Developer in one PR (roughly 400 changed lines or fewer):
- `--title`: imperative ("Add POST /orders endpoint").
- `--requirements-file`: what to change, touched modules/symbols (from codegraph), API/DB contract, edge cases.
- `--acceptance-file`: Markdown checklist (`- [ ] ...`), each item verifiable by a test or a request.
  Include criteria demanded by the team skills listed in your prompt (e.g. security rules, docs/llms.txt sync).
- Requirements end with `Team skills: <names>` (or `Team skills: none`): the team skills the Developer must apply.
- `--depends`: ids of tickets that must merge first. Create tickets in dependency order so the ids exist.

Rules
- Schema/migration tickets come before the code that uses them.
- Avoid two parallel tickets editing the same files; chain them with `--depends` instead.
- Doc-update runs: mark superseded non-done tickets with `docket ticket outdated <id>`. Never touch done tickets.
