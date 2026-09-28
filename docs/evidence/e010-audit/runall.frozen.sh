#!/bin/bash
# E010: 10 rounds, arms interleaved within each round, exactly as preregistered.
set -uo pipefail
OUT="$CLAUDE_JOB_DIR/tmp/e010"
LOCK="$OUT/.runlock"
if ! mkdir "$LOCK" 2>/dev/null; then echo "REFUSING: another E010 run holds $LOCK"; exit 9; fi
trap 'rmdir "$LOCK" 2>/dev/null' EXIT
if pgrep -f "e010/trial.sh" >/dev/null 2>&1; then echo "REFUSING: a trial.sh is already running"; exit 9; fi
echo "E010 start $(date -u +%FT%TZ)" >> "$OUT/ledger.txt"
for r in 1 2 3 4 5 6 7 8 9 10; do
  for arm in N Pfact Pinert Prose M; do
    "$OUT/trial.sh" "$arm" "$r" || echo "$arm $r LAUNCH_FAIL" >> "$OUT/ledger.txt"
  done
  echo "--- round $r complete $(date -u +%FT%TZ)" >> "$OUT/ledger.txt"
done
echo "E010 done $(date -u +%FT%TZ)" >> "$OUT/ledger.txt"
