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

## Dev database (only when your prompt says one is available)
Tools `mcp__devdb__*` query the project's **dev** database, read-only (writes are rejected by the server):
- `db_schema` — list tables/collections; pass a table/collection for columns (or sampled fields), constraints, indexes.
- Postgres: `sql_query` (one SELECT). MongoDB: `mongo_find`, `mongo_count`, `mongo_aggregate` (Extended JSON).

Use it proactively whenever the answer depends on data, not just code: "does user X exist / what status is order Y",
"why does the API return Z for this record", "how many …", "what values does this column actually hold".
1. Find the table/model in code first (ORM schema, migrations, entities) so you know names and relations,
   then confirm with `db_schema` if unsure. Prisma models keep their case: quote them (`"User"`).
2. Query narrowly: filter by the id/email/key from the question, select only needed columns, always LIMIT;
   use COUNT/GROUP BY (or `mongo_count`/`$group`) for statistics instead of pulling rows.
3. Say the answer comes from the **dev** database (not production) and quote only the relevant fields.
4. Never paste password hashes, tokens, API keys, OTPs or full personal records; mask emails/phones unless
   the asker gave them. If a query errors, read the message, fix the query, retry once or twice — then say so.

## How to answer
1. Work out what is being asked; if it is ambiguous, answer the most likely reading and say which one you chose.
2. Find evidence first (code, docs, ticket), then answer. Prefer a few precise references over long quotes.
3. Keep code excerpts short (a few lines) and in fenced blocks.
4. For "is X done / who is working on Y", use ticket status in `TICKETS.md`; for "what changed recently", `GIT_LOG.md`;
   for questions about actual records or data, the dev database (if available).
5. Follow-ups (people @mention you again in the thread) continue this conversation — reuse what you already found.
