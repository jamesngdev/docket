# docket — TUI Design

Companion to `REQUIREMENT.md` §5. All screens are 80 columns wide in the mockups; the real
layout is responsive (sidebar collapses to icons under 100 cols).

## Global frame

```
┌ docket ─ <project-name> ─────────────────────────────── agents 2/3  gh ✓ docker ✓ ┐
│ SIDEBAR (14) │ MAIN PANE                                                          │
│              │                                                                    │
├──────────────┴────────────────────────────────────────────────────────────────────┤
│ :<command line>                          j/k move · Enter open · ? help · q back  │
└───────────────────────────────────────────────────────────────────────────────────┘
```

- **Header:** project name, active phase (docs branch), agent slots used, dependency health (`gh`, `docker`, `tmux`, `claude`, `codegraph`) and daemon state (`daemon ✓` / `daemon ✗`).
- **Sidebar:** `Dashboard` `Questions` `Features` `Tickets` `Agents` `Docs`. Badges show counts needing attention
  (open questions, unapproved items, running agents). `Tab`/`Shift-Tab` or `1`–`6` switch views. The sidebar never
  takes focus, so `h`/`l` are free for the view (ticket status filter).
- **`n` (global):** runs the next pipeline step shown in the status bar (scan docs, run TechLead, jump to the gate).
- **Status bar:** left = `:` command line (vim style); right = context keys for the focused widget.
- **Modals** (confirm, forms, textarea) overlay the main pane; `Esc` closes.

## Global keymap

| Key            | Action                                   |
| -------------- | ---------------------------------------- |
| `j` `k`        | move down / up                           |
| `h` `l`        | prev/next ticket status filter           |
| `Tab` `1`–`6`  | switch view                              |
| `n`            | run the next pipeline step               |
| `gg` `G`       | top / bottom                             |
| `Enter`        | open / primary action                    |
| `e`            | edit focused field (inline textarea)     |
| `a`            | approve (feature / ticket)               |
| `x`            | reject / stop (context dependent)        |
| `/`            | filter current list                      |
| `:`            | command line (`:project`, `:rescan`, `:dispatch`, `:q`) |
| `r`            | refresh / rescan                         |
| `?`            | help overlay                             |
| `q` `Esc`      | back / close                             |
| `Ctrl-S`       | save textarea                            |

## 1. Project picker (launch)

```
┌ docket ───────────────────────────────────────────────────────────────────────────┐
│                                                                                   │
│   PROJECTS                                                                        │
│                                                                                   │
│   ▸ reelme-backend      ~/Work/reelme-be         agents 2   tickets 4/12 done     │
│     shop-api            ~/Work/shop-api          agents 0   tickets 9/9  done     │
│     billing-service     ~/Work/billing           agents 1   ⚠ 3 questions open    │
│                                                                                   │
│   n  new project      Enter open      d delete      q quit                        │
│                                                                                   │
└───────────────────────────────────────────────────────────────────────────────────┘
```

`n` opens the **New project** form:

```
┌ New project ──────────────────────────────────────────────────────────────────────┐
│ Name          [ reelme-backend                                   ]               │
│ Description   [ Backend for ReelMe mobile app                    ]               │
│ GitHub repo   [ git@github.com:apero/reelme-be.git               ]               │
│ Local path    [ ~/Work/reelme-be                                 ]  (clone if missing) │
│ Docs repo     [ git@github.com:apero/reelme-docs.git             ]               │
│ Base branch   [ develop ]   PR target [ develop ]   Max agents [ 3 ]              │
│ Models        techlead [ opus ]  developer [ sonnet ]  tester [ sonnet ]          │
│                                                                                   │
│ Tab next field · Ctrl-S create · Esc cancel                                       │
└───────────────────────────────────────────────────────────────────────────────────┘
```

On create: clone repo + docs, `codegraph init`, add `.docket/` to `.git/info/exclude`, start the project daemon,
then jump to Dashboard with the stepper at **Scan**. TechLead's first run also generates the shared
`.docket/docker-compose.yml`, which the daemon brings up.

## 2. Dashboard

```
┌ docket ─ reelme-backend ─────────────────────────────── agents 2/3  gh ✓ docker ✓ ┐
│ ▸ Dashboard  │ PIPELINE                                                           │
│   Questions 3│                                                                    │
│   Features  2│  ✓ Scan ──── ✓ Questions ──── ● Features ──── ○ Tickets ──── ○ Run │
│   Tickets    │  12 docs      5/5 answered    2 of 4 to approve                    │
│   Agents    2│                                                                    │
│   Docs       │  ▶ Enter: review features (2 waiting approval)                     │
│              │                                                                    │
│              │ BOARD                                                              │
│              │  backlog 2   pending 3   processing 2   wait_review 1   done 4     │
│              │                                                                    │
│              │ ACTIVITY                                                           │
│              │  11:32  dev-#5   opened PR #42 "Order API"                         │
│              │  11:30  tester   approved PR #40                                   │
│              │  11:12  techlead generated 4 features from docs diff (3 files)     │
│              │  10:58  docs     pulled a1b2c3 → d4e5f6 (3 changed)                │
├──────────────┴────────────────────────────────────────────────────────────────────┤
│ :                                 Enter continue · r rescan docs · ? help         │
└───────────────────────────────────────────────────────────────────────────────────┘
```

Stepper states: `✓` done, `●` current gate (needs user), `◐` agent running, `○` not started.
`Enter` always jumps to the view that unblocks the current gate.

## 3. Questions

```
┌ docket ─ reelme-backend ─────────────────────────────── agents 2/3  gh ✓ docker ✓ ┐
│   Dashboard  │ QUESTIONS   3 open · 2 answered · 0 skipped          [/] filter    │
│ ▸ Questions 3│                                                                    │
│   Features   │ ▸ ○ Q3  Should refresh tokens be rotated on every use?             │
│   Tickets    │   ○ Q4  Which payment providers must v1 support?                   │
│   Agents    2│   ○ Q5  Is soft-delete required for user accounts?                 │
│   Docs       │   ✓ Q1  Auth method?                     → JWT + refresh token     │
│              │   ✓ Q2  Target DB?                        → PostgreSQL 16           │
│              ├────────────────────────────────────────────────────────────────────┤
│              │ Q3 · source: docs/auth.md §3, src/auth/token.go                     │
│              │ Should refresh tokens be rotated on every use? The doc mentions     │
│              │ "long-lived sessions" but the code has a 7-day fixed TTL.           │
│              │                                                                    │
│              │ ANSWER                                                             │
│              │ ┌────────────────────────────────────────────────────────────────┐ │
│              │ │ Yes, rotate on every refresh. Keep a 30-day absolute max.      │ │
│              │ │ █                                                              │ │
│              │ └────────────────────────────────────────────────────────────────┘ │
├──────────────┴────────────────────────────────────────────────────────────────────┤
│ -- INSERT --                        Ctrl-S save · Esc cancel · s skip question    │
└───────────────────────────────────────────────────────────────────────────────────┘
```

- `Enter` on a question focuses the textarea (insert mode). `s` marks skipped.
- Detail pane is structured: SOURCE (one bullet per `;`-separated item), QUESTION (inline options `(A)`, `(B)`…
  each on its own line, "Đề xuất/Recommend" highlighted), ANSWER.
- `c` opens a live chat with an **Advisor** agent about the selected question (interactive `claude` in tmux,
  isolated home, skill `explain-question`, context QUESTION.md + PROJECT.md). The TUI attaches; `Ctrl-b d`
  returns and keeps the chat alive (💬 marker); `c` again resumes it; `X` ends it. When asked, the advisor
  records the decision with `docket question answer <id> --text …`.
- When all are answered/skipped, status bar shows `▶ Enter: run TechLead (analyse code → features)`.

## 4. Features

```
┌ docket ─ reelme-backend ─────────────────────────────── agents 2/3  gh ✓ docker ✓ ┐
│   Dashboard  │ FEATURES    2 draft · 3 approved · 1 outdated                      │
│   Questions  │                                                                    │
│ ▸ Features  2│ ▸ ○ F5  Payment checkout (Stripe + VNPay)              draft       │
│   Tickets    │   ○ F6  Order history & export                        draft       │
│   Agents    2│   ✓ F1  Authentication                                approved    │
│   Docs       │   ✓ F2  User profile                                  approved    │
│              │   ✓ F3  Order API                                     approved    │
│              │   ⊘ F4  Email notifications                           outdated    │
│              ├────────────────────────────────────────────────────────────────────┤
│              │ F5 · Payment checkout                                              │
│              │ docs: docs/payment.md, docs/orders.md                              │
│              │                                                                    │
│              │ Users can pay for an order with Stripe (international) or VNPay    │
│              │ (domestic). Webhooks update order status. Refund within 7 days…    │
│              │                                                                    │
│              │ tickets: none yet (generated after approval)                       │
├──────────────┴────────────────────────────────────────────────────────────────────┤
│ :                 a approve · A approve all · e edit · x reject · Enter detail    │
└───────────────────────────────────────────────────────────────────────────────────┘
```

After the last draft is approved: `▶ Enter: run TechLead (generate tickets)`.

## 5. Tickets

List + detail. `h`/`l` cycles the status filter.

```
┌ docket ─ reelme-backend ─────────────────────────────── agents 2/3  gh ✓ docker ✓ ┐
│   Dashboard  │ TICKETS   [all] backlog pending processing wait_review done        │
│   Questions  │                                                                    │
│   Features   │   ○ #7  Stripe checkout session          F5   backlog   ⇠ #5       │
│ ▸ Tickets   2│   ○ #8  VNPay checkout                   F5   backlog   ⇠ #5       │
│   Agents    2│   ● #4  Add user auth middleware         F1   pending              │
│   Docs       │ ▸ ◐ #5  Order API                        F3   processing  dev-1    │
│              │   ◐ #6  Profile endpoints                F2   processing  dev-2    │
│              │   ◔ #3  Login page                       F1   wait_review PR#42 ⚠  │
│              │   ✓ #1  Setup DB schema                  F1   done        PR#40    │
│              ├────────────────────────────────────────────────────────────────────┤
│              │ #5 · Order API · F3 Order API · processing · dev-1 (12m)           │
│              │ branch ticket/5-order-api · worktree .docket/worktrees/5           │
│              │ depends on: #1 ✓      blocks: #7 #8                                │
│              │                                                                    │
│              │ REQUIREMENTS                                                       │
│              │  - POST /orders creates an order from cart …                       │
│              │ ACCEPTANCE CRITERIA                                                │
│              │  [ ] returns 201 with order id   [ ] rejects empty cart            │
├──────────────┴────────────────────────────────────────────────────────────────────┤
│ :   c chat · a approve · e edit · d deps · Enter agent · o PR · x stop · ? help  │
└───────────────────────────────────────────────────────────────────────────────────┘
```

- `⚠` on `wait_review` = Tester requested changes and max rounds reached (`needs_user`).
- `c` opens a live **TechLead chat** about the ticket (💬 marker while it runs; `c` again resumes, `X` ends).
  TechLead proposes changes to this and related tickets, applies them after you confirm, re-dispatches in-flight
  tickets on the same branch, and creates follow-up tickets for merged ones. Acceptance edit moved to `C`.
- `N` opens the **new-ticket chat**: describe the work to TechLead; it clarifies only what is essential, checks
  existing tickets/features/code, and creates the ticket(s) right away (linked feature, dependencies, acceptance
  criteria, `Team skills:` line) in `backlog` for your approval. One such chat per project; `N` resumes it.
- Detail pane shows the TechLead **CHANGE REQUEST** when a ticket is being reworked.
- `⚔ conflict` marks a PR that conflicts with its target; the detail shows the auto-resolve attempt (max 2).
  Switching the filter to `wait_review` triggers an immediate PR/conflict check.
- `d` opens a dependency editor (pick tickets with `Space`).
- `Enter` on a `processing` ticket attaches to its agent's tmux session.
- `:dispatch` forces a dispatch pass; normally automatic.

## 6. Agents

```
┌ docket ─ reelme-backend ─────────────────────────────── agents 2/3  gh ✓ docker ✓ ┐
│   Dashboard  │ AGENTS   running 2 · slots 3 · history 14                          │
│   Questions  │                                                                    │
│   Features   │ ▸ ◐ dev-1     Developer  #5 Order API       12m   tmux po-reelme-dev-5 │
│   Tickets    │   ◐ dev-2     Developer  #6 Profile         04m   tmux po-reelme-dev-6 │
│ ▸ Agents    2│   ○ techlead  idle                                                 │
│   Docs       │   ○ tester    idle                                                 │
│              │                                                                    │
│              │ HISTORY                                                            │
│              │   ✓ tester    #3 Login page    review: approve        2m           │
│              │   ✗ dev-1     #3 Login page    failed (no callback)   8m   log ▸   │
│              │   ✓ techlead  features from diff                      3m           │
│              ├────────────────────────────────────────────────────────────────────┤
│              │ dev-1 · last 5 lines of pane                                       │
│              │ > Running go test ./internal/order/...                             │
│              │ > ok   internal/order   0.41s                                      │
│              │ > git push -u origin ticket/5-order-api                            │
├──────────────┴────────────────────────────────────────────────────────────────────┤
│ :                       Enter attach (tmux) · x stop · r restart · L open log     │
└───────────────────────────────────────────────────────────────────────────────────┘
```

`Enter` suspends the TUI and runs `tmux attach -t <session>`; detaching (`Ctrl-b d`) returns to the TUI.

## 7. Docs

```
┌ docket ─ reelme-backend ─────────────────────────────── agents 2/3  gh ✓ docker ✓ ┐
│   Dashboard  │ DOCUMENTS   phase phase-2 @ d4e5f6 · 5 files · 3 third-party       │
│   Questions  │                                                                    │
│   Features   │ ▸ ○ phase-1                        phase                           │
│              │   ● phase-2                        phase (active)                  │
│              │   M docs/payment.md        scanned 10:58                           │
│   Tickets    │   A docs/refunds.md        scanned 10:58                           │
│   Agents    2│   M docs/orders.md         scanned 10:58                           │
│ ▸ Docs       │     docs/auth.md           scanned 09:10                           │
│              │     docs/profile.md        scanned 09:10                           │
│              │                                                                    │
│              │ 3RD-PARTY DOCS                                             n new   │
│              │   🔗 Stripe API reference   https://stripe.com/docs/api             │
│              │   📄 VNPay integration PDF  (cached 42 KB)                          │
│              │   📝 Env & secrets layout   (note)                                  │
├──────────────┴────────────────────────────────────────────────────────────────────┤
│ :                       r pull & rescan · Enter preview · i add 3rd doc · d del   │
└───────────────────────────────────────────────────────────────────────────────────┘
```

Phase rows list the docs repo's branches; `Enter` on one switches the active phase (also `:phase <branch>`).
The new phase is diffed against the previous one, so only its changes reach TechLead.
`r` pulls the docs repo, diffs it and (when files changed) starts TechLead on the diff automatically.
`i` opens a template (`type: link|cached|note`, title, url, note, `---`, content); `cached` fetches the URL on save.

## Inline textarea (shared widget)

Used for answers, feature description, ticket requirements / acceptance criteria, 3rd-party notes.
`Ctrl-S` saves, `Esc` cancels. Soft-wrap, no external editor.
