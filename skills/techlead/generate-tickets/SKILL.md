---
name: generate-tickets
description: Split approved features into small, dependency-ordered tickets with requirements and acceptance criteria via `docket ticket add`.
---

# Generate tickets

Only for features listed in PROJECT.md as approved without tickets.

Each ticket must be completable by one Developer in one PR (roughly 400 changed lines or fewer):
- `--title`: imperative ("Add POST /orders endpoint").
- `--requirements-file`: what to change, touched modules/symbols (from codegraph), API/DB contract, edge cases.
- `--acceptance-file`: Markdown checklist; EVERY item ends with how it is verified, e.g.
  `- [ ] POST /orders returns 201 with the order id — verify: test TestCreateOrder` or
  `— verify: curl -X POST localhost:3000/orders …`. The CLI rejects items without `verify:`.
  Include criteria demanded by the team skills listed in your prompt (e.g. security rules, docs/llms.txt sync).
- `--files`: comma-separated paths/dirs/globs the ticket will change (`internal/orders/,migrations/*.sql`).
  Required. Changes outside them are flagged as out of scope (tests and lockfiles are exempt), so be complete
  but tight.
- `--model` (optional): `haiku` for trivial mechanical tickets (rename, config, copy change), `opus` for
  cross-cutting or subtle ones (concurrency, migrations with data, security). Omit for the project default.
- Requirements end with `Team skills: <names>` (or `Team skills: none`): the team skills the Developer must apply.
- `--depends`: ids of tickets that must merge first. Create tickets in dependency order so the ids exist.

Rules
- Schema/migration tickets come before the code that uses them.
- Avoid two parallel tickets editing the same files; chain them with `--depends` instead.
- Doc-update runs: mark superseded non-done tickets with `docket ticket outdated <id>`. Never touch done tickets.
