---
name: answer-discord
description: Where project knowledge lives and which read-only commands Backend Support may run to answer a Discord question about docs, questions, features, tickets or source code.
---

# Answering a Discord question

## Where things are
- **Source code**: your working directory — a read-only checkout of the base branch (the latest integrated code).
  Search with Grep/Glob, read with Read. Work in progress lives on ticket branches/PRs, not here.
- **Context directory** (path in your prompt), refreshed for every message:
  - `PROJECT.md` — documents of the active phase, questions with their answers, features.
  - `TICKETS.md` — every ticket in full: status, requirements, acceptance criteria, branch/PR, review.
  - `GIT_LOG.md` — the last 50 commits of the base branch with the files they touched.
  - `3TH_DOCUMENT.md` — third-party docs (links, cached pages, notes).
- **Docs**: the docs directory named in your prompt (checkout of the docs repo, active phase branch).

You have no shell: only Read, Grep and Glob. Files outside the working directory, the context directory and the
docs directory are not readable.

## How to answer
1. Work out what is being asked; if it is ambiguous, answer the most likely reading and say which one you chose.
2. Find evidence first (code, docs, ticket), then answer. Prefer a few precise references over long quotes.
3. Keep code excerpts short (a few lines) and in fenced blocks.
4. For "is X done / who is working on Y", use ticket status in `TICKETS.md`; for "what changed recently", `GIT_LOG.md`.
5. Follow-ups (people @mention you again in the thread) continue this conversation — reuse what you already found.
