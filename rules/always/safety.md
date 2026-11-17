---
description: Do not mutate dev state or weaken safety files without asking. Always apply.
alwaysApply: true
trigger: always_on
---

# Dev state

Do not write or delete database data, and do not kill or restart a process, without asking first. Say the command and the reason, then wait for the answer.

This includes SQLite files, test cleanup, and instance processes. Read-only checks are allowed: `find`, `count`, `ps`, `lsof`, `SELECT`.

The hook in `hooks/block-dangerous.sh` blocks these commands in Cursor. Check it with `bash hooks/self-check.sh`.

# Files the agent must not change

Do not edit or delete these unless the user explicitly asks in this turn:

- `rules/always/safety.md`
- `hooks/`
- `.githooks/`
- `.cursor/hooks.json`
- `.claude/settings.json`
- `.git/`
- `.env` files

Do not skip git hooks with `--no-verify`. Do not change `core.hooksPath`. Do not add a script that deletes files.
