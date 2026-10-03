#!/usr/bin/env bash
# SessionStart hook: point a fresh or compacted session at the durable cursor.
#
# Fires on startup, resume, clear and compact. The compact matcher is the reason
# this hook exists at all: compaction is lossy, there is no PreCompact hook, and
# SessionStart(compact) is the only native re-injection point for custom state.
#
# It injects a POINTER, not the file.
#
# docs/WORK-STATE.md is ~6KB, about 1,500 tokens. Injecting it on every session
# start and after every compaction would add that to the fixed prefix forever.
# This repository measured, on 2026-09-25, that 73-90% of an optimised run is
# already fixed prefix and that the remaining headroom in tool output is 10-27%
# (docs/evidence/modes-catalogue-2026-09-25.md). Growing the prefix to save
# tool-output tokens would be the wrong direction by its own evidence.
#
# So: ~90 tokens that say where the cursor is and that it must be reconciled.
# The agent reads the file when it needs it, which is once, deliberately.
set -uo pipefail

STATE="docs/WORK-STATE.md"
[ -f "$STATE" ] || exit 0

# Anchors the agent can check cheaply without opening anything.
BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "?")
COMMIT=$(git log -1 --format=%h 2>/dev/null || echo "?")
DIRTY=$(git status --porcelain 2>/dev/null | wc -l | tr -d ' ')
AGE=$(( ( $(date +%s) - $(stat -f %m "$STATE" 2>/dev/null || echo 0) ) / 86400 ))

printf '%s' "{\"hookSpecificOutput\":{\"hookEventName\":\"SessionStart\",\"additionalContext\":\
\"Durable work state for this repository is docs/WORK-STATE.md (updated ${AGE}d ago). \
Repository right now: branch ${BRANCH}, commit ${COMMIT}, ${DIRTY} uncommitted path(s). \
Before acting on prior work, read that file and reconcile it against the repository; \
where they disagree the repository wins and the file is repaired first. \
Established facts live in docs/evidence/ and decisions in docs/adr/ - the cursor points at them rather than copying them.\"}}"
exit 0
