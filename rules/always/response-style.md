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
| Close every reply | End with a short "What went wrong / what could go wrong" and "What we do" block. |

Apply this to every response: code, explanation, and discussion.

# Close every reply

Every chat reply ends with these two short parts (2 to 4 lines total):

| Part | Meaning |
|---|---|
| What went wrong / what could go wrong | If something failed, name the real cause in plain words. If nothing failed yet, name the main risk for the next step. |
| What we do | The next action. Mark **I do** or **you do**. One clear command or decision. |

Do not skip this block on success replies. On success, "what could go wrong" is the risk to watch, not a fake failure.
