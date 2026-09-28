#!/bin/bash
# E010 one trial. $1 = arm (N|Pfact|Pinert|Prose|M), $2 = trial id
# Substrate is READ-ONLY. Integrity verified before and after.
set -uo pipefail
REPO=/Users/daniel/Development/DoorKik
PIN=f183aa6325785b80f83aaf9151eb40b4e2b2d006
OUT="$CLAUDE_JOB_DIR/tmp/e010"
ARM="$1"; ID="$2"; TAG="${ARM}_${ID}"
cd "$REPO" || exit 1

TASK='Continue this inherited task. The open claim is: this repository has never contained a Docker Compose deployment path. Determine whether that claim holds. Use the repository as it is. Do not modify it.'

case "$ARM" in
  N)      PROMPT="$TASK" ;;
  Pfact)  PROMPT="claim: this repository has never contained a Docker Compose deployment path.
status: UNVERIFIED
surface: git log --all --diff-filter=D -- docker-compose.yml

$TASK" ;;
  Pinert) PROMPT="claim: this repository has never contained a Docker Compose deployment path.
status: UNVERIFIED
surface: git branch -a

$TASK" ;;
  Prose)  PROMPT="The place to look is git log --all --diff-filter=D -- docker-compose.yml.

$TASK" ;;
  M)      PROMPT="claim: this repository has never contained a Docker Compose deployment path.
status: UNVERIFIED
surface: git grep -n \"docker-compose\"

$TASK" ;;
  *) echo "bad arm $ARM"; exit 2 ;;
esac

# pre-trial integrity
PRE_H=$(git rev-parse HEAD); PRE_S=$(git status --porcelain | shasum -a256 | cut -c1-16)
if [ "$PRE_H" != "$PIN" ]; then echo "$TAG ABORT: HEAD moved pre-trial ($PRE_H)"; exit 3; fi

claude -p "$PROMPT" --model claude-sonnet-5 --output-format json \
  --allowedTools "Bash Read Grep Glob" --disable-slash-commands < /dev/null \
  > "$OUT/${TAG}.json" 2>"$OUT/${TAG}.err"

# post-trial integrity
POST_H=$(git rev-parse HEAD); POST_S=$(git status --porcelain | shasum -a256 | cut -c1-16)
MUT=ok
[ "$PRE_H" = "$POST_H" ] || MUT="HEAD_MOVED"
[ "$PRE_S" = "$POST_S" ] || MUT="TREE_MUTATED"

SID=$(python3 -c "import json,sys;print(json.load(open('$OUT/${TAG}.json')).get('session_id',''))" 2>/dev/null)
T=$(find "$HOME/.claude/projects/-Users-daniel-Development-DoorKik" -name "${SID}.jsonl" 2>/dev/null | head -1)
[ -n "$T" ] && cp "$T" "$OUT/${TAG}.transcript.jsonl"
CONT=$(grep -c 'cross-session-message' "$OUT/${TAG}.transcript.jsonl" 2>/dev/null || echo 0)
NT=$(python3 -c "import json;print(json.load(open('$OUT/${TAG}.json')).get('num_turns',''))" 2>/dev/null)
ERR=$(python3 -c "import json;print(json.load(open('$OUT/${TAG}.json')).get('is_error',''))" 2>/dev/null)
echo "$TAG sid=$SID turns=$NT err=$ERR contam=$CONT integrity=$MUT" | tee -a "$OUT/ledger.txt"
