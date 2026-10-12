---
name: use-shared-db
description: Connect to the project's shared dev database/services started by docket. Never start your own containers.
---

# Shared dev database

- Connection values are in the "Dev environment" section of TICKET.md (from `$DOCKET_DATA/dev-db.env`).
- Export them (or write a git-ignored local env file the app reads) before running the app or tests.
- The stack is shared with other agents working in parallel: prefix test data with your ticket (e.g. `t42_`)
  or use a dedicated schema when the test setup allows, and clean up what you create.
- Never run `docker compose up/down` or start containers. If the DB is unreachable, say so in the PR and still finish.
