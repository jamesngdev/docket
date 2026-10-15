---
name: write-repo-notes
description: Write .docket/REPO.md (how to build, test and navigate this repo) and .docket/checks.yaml (the verify commands every Developer change must pass) so agents never re-discover them.
---

# Write repo notes and verify checks

Every Developer, Tester and TechLead run gets REPO.md pasted into its context, and the `wait_review` gate runs
checks.yaml before a PR can enter review. Getting these right saves every later run time and tokens.

## 1. Discover (codegraph + config files, no guessing)
- Language, framework, package manager; how to install deps.
- Exact compile/typecheck, lint, unit-test and (if separate) integration-test commands, as CI runs them
  (`.github/workflows/*`, `Makefile`, `package.json` scripts, `pyproject.toml`, …).
- Which tests need the shared dev DB from `$DOCKET_DATA/dev-db.env`, and how they seed/clean data.
- Layout: the 5–15 directories that matter and what lives in each; where routes, services, models,
  migrations and tests go; how a new endpoint/module is usually wired.
- Conventions a reviewer would reject a PR for: naming, error handling, logging, DTO/validation style,
  migration naming, commit style.

## 2. Write `$DOCKET_DATA/REPO.md` (aim for 40–80 lines, facts only)
```
# Repo notes
## Commands
- install: …
- compile: …   - lint: …   - test: …   - one test: …
## Dev environment
- tests need: … (env vars from dev-db.env) · test data: …
## Layout
- path/ — what lives here
## Conventions
- …
## Learned notes
```
Agents append one-liners under "Learned notes" with `docket note add`. Keep that section; on later runs
merge duplicates and delete notes that are no longer true.

## 3. Write `$DOCKET_DATA/checks.yaml`
```yaml
checks:              # run in order from the ticket worktree root, with dev-db.env exported
  - name: compile
    run: go build ./...
  - name: lint
    run: go vet ./...
  - name: test
    run: go test ./...
    timeout: 8m      # optional per-check limit; all checks together get at most 9m
max_diff_lines: 600  # larger PRs are flagged for TechLead review
```
Rules: only commands that already pass on the base branch (run each once to confirm); non-interactive;
safe to run while other agents use the same shared dev DB (no global reset/drop; per-run data or schemas);
no `--fix`/auto-format flags (they would rewrite files); no `docker compose up`. If the full test suite is
slower than ~8 minutes, run the fast unit subset here and leave the rest to CI.
