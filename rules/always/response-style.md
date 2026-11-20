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

1. **Flowchart first** (Mermaid). Teach the change hop by hop. Abstract box salads are not allowed.

## Flowchart style (locked)

| Must | Meaning |
|---|---|
| One real command path | Walk one concrete user action (example: `tiny instance create ...`) step by step |
| Hop labels | Each node says who acts and what data/protocol moves (HTTP JSON, gRPC, SQLite, signal, file path) |
| Change markers | Mark **UPDATED**, **UNCHANGED**, and **REMOVED / BEFORE** (old path no longer used) |
| Split charts if needed | Chart 1 = main request path. Chart 2 = other commands or smoke. Do not crush everything into one unreadable graph |
| Plain words after | Short table: Layer / Job / Changed? plus env defaults if relevant |

| Forbidden | Why |
|---|---|
| Only architecture overview boxes | User cannot see what this change did |
| Only a tiny “what changed” list with no path | Missing how pieces connect |
| Mixing overview + walkthrough with no labels | Hides UPDATED vs UNCHANGED |

Good pattern (copy this shape):

1. BEFORE/REMOVED subgraph (what the change took away)
2. UPDATED caller steps (CLI or API)
3. UPDATED/UNCHANGED middle hops (tinyd, service, store)
4. UNCHANGED backend (gRPC, C++, Linux) unless that layer changed
5. Arrows labeled with protocol (`HTTP JSON`, `gRPC protobuf`)
6. Optional second chart for sibling commands or smoke startup order
7. Plain-words table under the charts

2. **Detailed tech explanation** — use this exact section shape (same depth as the Phase 2 image writeup the user locked).

After the flowchart, write these sections in order. Do not collapse them into one short table only.

### 1) What you type (examples)
- Real commands (`tiny ...`)
- Example JSON response when useful
- One skip/fail command example

### 2) On-disk layout (example) — when files/paths change
- Directory tree
- What is UUID vs name vs env default

### 3) Hop table (examples)
| Hop | Example |
|---|---|
| HTTP / RPC / DB / Linux | concrete payload, path, or field |

### 4) What I made (broken down)
| Area | Concrete change | Example |
|---|---|---|
| package/file | what changed | symbol or behavior |

### 5) Tech removed vs added
| Removed | Example of old behavior |
|---|---|
| ... | ... |

| Added | Example |
|---|---|
| ... | ... |

### 6) How they connect
- One sentence path: CLI → … → Linux
- Numbered example chain for one concrete name/id

### 7) What will happen / what fails without it
| Situation | Result |
|---|---|
| happy path | ... |
| main failures | ... |

Name the real tech terms (gRPC, HTTP JSON, cgroup v2, SQLite, protobuf, SIGTERM, SIGKILL, `rootfs_path`, etc.).
Order is fixed: **hop-by-hop flowchart → sections 1–7 above → close block**.
Do not replace sections 1–7 with a single compressed “Part/Detail” mega-table.

Then close every reply with:

| Part | Meaning |
|---|---|
| What went wrong | Real failure cause, or "nothing" if this turn had no failure. |
| What we do | Next action. Mark **I do** or **you do**. |
| What could go wrong | Main risk for the next step. |

Do not skip these blocks after edits. On success, "what could go wrong" is the risk to watch, not a fake failure.
Do not dump huge diffs. Point to paths and say the behavior change.
