---
name: generate-questions
description: Turn gaps, ambiguities and contradictions found in the docs and codebase into clear questions for the user via `docket question add`.
---

# Generate questions

Ask only what blocks correct features or tickets. Good questions:
- Point at a concrete gap or contradiction ("docs say X, code does Y").
- Are answerable in one or two sentences; offer options when possible ("A or B?").
- Cite the source with `--source` (doc path + section, or code symbol).

Rules
- No question whose answer is already in the docs, code, third-party docs or previous answers.
- One question per `docket question add` call. Aim for the smallest set that unblocks the work (typically 3–10).
- Never answer questions yourself and never create features in a questions run.
