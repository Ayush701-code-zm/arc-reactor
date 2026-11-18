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

When you edit the repo, explain the technical change in chat. Start with the tech explanation (not a status dump). Use this order:

1. **Flowchart first** (Mermaid). Prefer one big detailed chart when the change touches several paths.
   - Label nodes/subgraphs as **UNCHANGED** vs **UPDATED** (or show before → after on the updated edges).
   - Do not draw only the full system with no change markers.
   - Do not draw only a tiny “what changed” list with no path detail.
2. **Detailed tech table** (not one vague line per cell).

| Part | Meaning |
|---|---|
| What I made | Concrete change: functions, RPCs, files, status transitions. Several bullets if needed. |
| Where | Exact paths and symbols (package, file, function). |
| Tech removed | Named tech that left (protocol, library, API, pattern, old behavior). One line each. Say "none" if nothing was removed. |
| Tech added | Named tech that arrived. One line each with what it does. Say "none" if nothing new. |
| How they connect | Short prose or bullets tying the flowchart nodes: Go → gRPC → C++ → Linux (or similar). |
| What will happen | Runtime behavior now, including happy path and the main failure path. |
| What not without it | What fails, stays broken, or stays unclear if we skip this change. |

Name the real tech terms (example: gRPC, HTTP JSON, cgroup v2, SQLite, protobuf, SIGTERM, SIGKILL). Do not hide them behind vague words.
Do not compress the whole change into a single short sentence. Prefer tables + flowchart.

Then close every reply with:

| Part | Meaning |
|---|---|
| What went wrong | Real failure cause, or "nothing" if this turn had no failure. |
| What we do | Next action. Mark **I do** or **you do**. |
| What could go wrong | Main risk for the next step. |

Do not skip these blocks after edits. On success, "what could go wrong" is the risk to watch, not a fake failure.
Do not dump huge diffs. Point to paths and say the behavior change.
