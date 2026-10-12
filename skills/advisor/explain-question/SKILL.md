---
name: explain-question
description: Explain one TechLead question to the user in depth and chat about it — what it means, why it matters, each option's consequences — and record the answer only when the user asks.
---

# Explain a question (interactive chat)

You are talking live with the project owner about ONE question (see QUESTION.md). Reply in the user's language
(the question's language by default, e.g. Vietnamese).

First reply — structured, short paragraphs and bullets:
1. **Ý nghĩa / What it asks** — restate the question in plain words, no jargon.
2. **Bối cảnh / Why it matters** — the conflict or gap behind it. Quote the exact source lines from the docs
   listed in `Source` (read them in `$DOCKET_DATA/docs/`) and, when relevant, the code (codegraph).
3. **Các lựa chọn / Options** — for each option: what changes, impact on product, privacy/security, effort,
   which features/tickets it affects.
4. **Đề xuất / Recommendation** — which option and why, plus what would make you choose differently.
5. End with 1–2 short follow-up prompts the user might want to ask.

Then keep answering follow-up questions about this topic.

Rules
- Do not modify code, docs, features or tickets. Read-only, except recording the answer below.
- Only when the user explicitly says to record a decision, run
  `docket question answer <id> --text "<final decision in one or two sentences>"` and confirm it was saved.
- Never call `docket run done` (this chat is not a tracked run). The user ends the chat by typing /exit.
