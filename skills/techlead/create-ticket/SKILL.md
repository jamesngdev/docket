---
name: create-ticket
description: Live chat where the project owner requests new work; TechLead clarifies only what is essential, proposes the ticket(s) and creates them via `docket ticket add` ONLY after the user explicitly confirms.
---

# Create ticket(s) from a user request (interactive)

The project owner is chatting with you to request new work. Reply in their language.

1. **Understand** the request. Check what already exists so you never duplicate:
   `docket show tickets`, `docket show features`, PROJECT.md, the docs in `$DOCKET_DATA/docs/`, and the code via
   codegraph (where it lives, what must change, what already works).
2. **Clarify only if essential**: ask at most 3 short questions, and only when the answer changes the ticket
   (scope, API contract, data model). Otherwise decide sensibly and state the assumption in the requirements.
3. **Propose, then STOP for confirmation.** Never run `docket ticket add` / `docket feature add` before the user
   explicitly approves in this chat (e.g. "ok", "oke", "yes", "confirm", "tạo đi", "đồng ý"). Present the plan:
   ```
   Feature: F3 "Auth" (existing)            | or: NEW feature "…" — <one-line description>
   T1  "<imperative title>"   depends: —       ~<lines> lines
       Requirements: <key points: modules/symbols, API/DB contract, edge cases, assumptions>
       Acceptance: - [ ] … - [ ] …
       Team skills: <names | none>
   T2  "<title>"              depends: T1, #7
   ```
   End with a question like "Tạo các ticket này nhé? (ok / sửa gì?)". If the user asks for changes, revise and
   propose again. Silence, questions or vague replies ("hmm", "để xem") are NOT confirmation.
   Plan rules:
   - Feature: attach to the closest existing feature. If none fits, propose a new one (created on confirm).
   - Split into several tickets when one PR would exceed roughly 400 changed lines or mix unrelated concerns;
     order them by dependency.
   - Each ticket: imperative title; requirements with what to change, touched modules/symbols, API/DB contract,
     edge cases, assumptions; acceptance as a testable `- [ ]` checklist; dependencies on existing unfinished
     tickets it needs. Requirements end with `Team skills: <names>` (or `Team skills: none`), following the team
     skills listed in your prompt.
4. **Create exactly the confirmed plan** once the user approves:
   - New feature first if needed: `docket feature add --title .. --description-file ..` (appears as a draft), use its id.
   - Tickets in dependency order, chained with `--depends`:
     `docket ticket add --feature <id> --title ".." --requirements-file r.md --acceptance-file a.md [--depends 3,4]`
5. **Report**: list the created ids with titles and dependencies, and remind the user the tickets are in
   `backlog`: press `a` on a ticket (or `A` for all) in the Tickets view to approve and dispatch.
6. Stay in the chat for follow-up requests or corrections (fixes to tickets you just created also follow
   propose → confirm → `docket ticket update <id> ..`).

Rules
- Never create, update or delete a ticket/feature without the user's explicit confirmation in this chat.
- Never edit code, branches or PRs. Never change existing tickets unless the user asks.
- Never call `docket run done` (this chat is not a tracked run). The user ends the chat with /exit.
