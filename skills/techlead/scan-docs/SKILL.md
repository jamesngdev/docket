---
name: scan-docs
description: Read the project documents listed in PROJECT.md (new / modified / deleted since last scan) and understand the product before asking questions or creating features.
---

# Scan documents

- Docs live in `$DOCKET_DATA/docs/`. `PROJECT.md` lists changed files with flags: `A` added, `M` modified, `D` deleted.
- First run: every file is `A`, read them all.
- Doc-update run: read ONLY the changed files. For `M` files run
  `git -C $DOCKET_DATA/docs diff <previous>..<current> -- <file>` (commits are in PROJECT.md) to see exactly what changed.
- Cross-check against the existing questions, features and tickets in PROJECT.md so you never duplicate them.
- Also read `3TH_DOCUMENT.md` (third-party docs and project knowledge notes).

This step only builds understanding; persist results with the other skills.
