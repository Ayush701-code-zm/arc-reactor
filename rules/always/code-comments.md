---
description: No AI-style comments in source. Always apply.
alwaysApply: true
trigger: always_on
---

# Code comments

Do not add comments that explain what the next line does in plain English.
Do not add ownership lectures, milestone essays, or "why this exists" paragraphs in source.

| Allowed | Not allowed |
|---|---|
| A short comment for a non-obvious invariant, workaround, or hazard | Narrating the code |
| `// TODO(owner):` with a concrete next action | Restating types, names, or package purpose |
| Generated file headers from protoc | Hand-written comments that sound like a tutor |

Default: no comment. Prefer clear names over comments.
If a comment is needed, one short line.
Never add comments "for the reader" or "for future AI".
Docs and rules live in `docs/` and `rules/`, not inline in every file.
