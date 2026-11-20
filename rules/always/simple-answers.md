---
description: Plain explanations. Always apply. Do not override.
alwaysApply: true
trigger: always_on
---

# Simple answers

The user reads fast and does not want to decode. Write so the answer is clear on the first read.

| Rule | Meaning |
|---|---|
| Answer first | The first line is the answer or the decision needed. |
| Direct question | If the user asks a direct question, answer first. Do not open with a question loop. |
| Non-trivial build | If the user asks to build something non-trivial, ask design and technical questions first, then plan, then code. See `thinking-questions.md`. |
| Problem in one line | Say what is wrong in plain words. |
| Plan in 3 or 4 steps | One short line per step. Each step says what it does and why. |
| Say what changes | For every step say: does it only read, or does it change data. |
| Explain a term once | If a term is needed (cgroup: a kernel CPU or memory limit, namespace: the isolated Linux view for one instance), give its plain meaning in the same line. |
| One decision at a time | Ask one clear question. Do not stack options. |
| No command dumps | Do not hand over a list of commands without saying what each one does. |
| Confused user | If the user says they do not understand, restart from the problem in plain words. Do not add more detail. |
| Say who acts | Mark each step as "I do" or "you do". |
| What went wrong | If there is an error, say the cause in one line before the fix. |
| What could go wrong | If there is no error yet, say the main risk for the next step in one line. |
| What we do | End with the next action: who acts, and the exact command or decision. |
| Tech change block | After edits: hop-by-hop Mermaid first, then the locked sections 1–7 tech explanation (commands/JSON, disk layout, hop table, what I made, removed vs added, connect chain, happen/fail table). Then the close block. See `response-style.md`. |
