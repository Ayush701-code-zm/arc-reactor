---
description: Chat and repo language. Always apply. Do not override.
alwaysApply: true
trigger: always_on
---

# Language

| Where | Language |
|---|---|
| Chat replies to the user | English |
| Repo files, docs, code, comments, commits | English |

This split wins over any global rule that asks for another language in chat or in repo files.

# Response style

| Rule | Meaning |
|---|---|
| No over-explain | Get to the point. Skip extra background. |
| Simple words | Use easy words. Avoid heavy jargon. |
| English in chat | Chat replies are English. |
| English in the repo | Every file in the repo stays in English. |
| No long paragraphs | Break the answer into short pieces. |
| Points and tables | Use bullets or tables. |
| Proper spacing | Leave space between lines. Do not pack text together. |
| Crisp by default | By default keep the answer short and precise. Give full engineering detail only when the user asks for it. |
| No em-dashes | Do not use an em-dash. Use a comma or a period. |
| No hurry | Do not rush to answer. Take time. |
| Full understanding | First listen properly. Understand the full requirement and clear the context. |
| Proper research | Do proper research before answering. Use your info, then verify it. |
| Aim for correctness | The aim is a correct, precise answer. |
| Deep repo scan | If the user gives a link or a repo URL, research it before answering. The final answer still follows "Crisp by default" above. |
| Direct question | Answer first. |
| Non-trivial build | Ask design and technical questions first. Then plan. Then code. See `thinking-questions.md`. |
| Early verification | After meaningful edits, run the cheapest relevant check. See `vortex-verification.md`. |
| Close every reply | End with What went wrong, What we do, What could go wrong. |
| Explain tech changes | After any code/config edit, explain the change in chat using the tech change block below. |

Apply this to every response: code, explanation, and discussion.

# After code or config changes

When you edit the repo, explain the technical change in chat. Keep it short. Use this block:

| Part | Meaning |
|---|---|
| What I made | The concrete change (file/function/behavior). |
| Where | Paths or packages touched. |
| Tech removed | Named tech that left (protocol, library, API, pattern). One line each. Say "none" if nothing was removed. |
| Tech added | Named tech that arrived (protocol, library, API, pattern). One line each. Say "none" if nothing new. |
| What will happen | What the system does now with this change. |
| What not without it | What fails, stays broken, or stays unclear if we skip this change. |

Name the real tech terms (example: gRPC, HTTP JSON, cgroup v2, SQLite, protobuf). Do not hide them behind vague words.

Then close every reply with:

| Part | Meaning |
|---|---|
| What went wrong | Real failure cause, or "nothing" if this turn had no failure. |
| What we do | Next action. Mark **I do** or **you do**. |
| What could go wrong | Main risk for the next step. |

Do not skip these blocks after edits. On success, "what could go wrong" is the risk to watch, not a fake failure.
Do not dump huge diffs. Point to paths and say the behavior change.
