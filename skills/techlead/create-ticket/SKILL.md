---
name: create-ticket
description: Live chat where the project owner requests new work; TechLead clarifies only what is essential, then creates well-formed ticket(s) via `docket ticket add` (backlog, awaiting the user's approval).
---

# Create ticket(s) from a user request (interactive)

The project owner is chatting with you to request new work. Reply in their language.

1. **Understand** the request. Check what already exists so you never duplicate:
   `docket show tickets`, `docket show features`, PROJECT.md, the docs in `$DOCKET_DATA/docs/`, and the code via
   codegraph (where it lives, what must change, what already works).
2. **Clarify only if essential**: ask at most 3 short questions, and only when the answer changes the ticket
   (scope, API contract, data model). Otherwise decide sensibly and state the assumption in the requirements.
3. **Create immediately** once you understand. Do not ask for a confirmation of the ticket text.
   - Feature: attach to the closest existing feature (`--feature <id>`). If none fits, create one first with
     `docket feature add --title .. --description-file ..` (it appears as a draft for the user) and use its id.
   - Split into several tickets when one PR would exceed roughly 400 changed lines or mix unrelated concerns;
     create them in dependency order and chain with `--depends`.
   - Each ticket: imperative `--title`; `--requirements-file` with what to change, touched modules/symbols,
     API/DB contract, edge cases, assumptions; `--acceptance-file` as a `- [ ]` checklist that is testable;
     `--depends` on existing unfinished tickets it needs. Requirements end with `Team skills: <names>` (or
     `Team skills: none`), following the team skills listed in your prompt.
   - Command: `docket ticket add --feature <id> --title ".." --requirements-file r.md --acceptance-file a.md [--depends 3,4]`
4. **Report**: list the created ids with titles and dependencies, and remind the user the tickets are in
   `backlog`: press `a` on a ticket (or `A` for all) in the Tickets view to approve and dispatch.
5. Stay in the chat for follow-up requests or corrections (use `docket ticket update <id> ..` for fixes to tickets
   you just created).

Rules
- Never edit code, branches or PRs. Never change existing tickets unless the user asks.
- Never call `docket run done` (this chat is not a tracked run). The user ends the chat with /exit.
