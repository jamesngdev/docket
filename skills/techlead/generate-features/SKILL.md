---
name: generate-features
description: Create product features from documents + codebase (codegraph) + answered questions with `docket feature add`.
---

# Generate features

Inputs: documents, answered questions (for skipped ones decide sensibly and state the assumption), codebase via codegraph.

A feature is a user-visible capability a reviewer can accept as a whole (e.g. "Payment checkout").
- `--title`: short noun phrase.
- `--description-file`: what it does, scope, out of scope, assumptions, how it fits existing code (modules/symbols).
- `--docs`: comma-separated related doc paths.

Rules
- Skip what the codebase already implements fully; describe partial work as the remaining delta.
- Doc-update runs: create features only for new scope; mark obsolete ones with `docket feature outdated <id>`.
- Never create tickets in this run; the user approves features first.
