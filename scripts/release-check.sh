#!/usr/bin/env bash
# Assert what a built binary actually prints, not what the source tree says.
#
# Why this exists. On 2026-09-08 the published v0.5.0 asset was downloaded and
# run against the same corpus as the local build. It printed:
#
#   Cost per task, across 1666 transcripts at list prices dated 2026-06-24
#
# while the tree printed "115 sessions" at prices dated 2026-09-07. The
# transcript-count denominator is the one docs/evidence/calibration-corpus
# identifies as overstating the independent sample by roughly twenty times,
# because a session writes one transcript per lane. It had been retracted in
# the tree and was still shipping in the artifact a stranger downloads.
#
# Every check in this repository ran against the source tree. None ran against
# the release asset. That gap is the same shape as the defects the tool exists
# to find: the thing nobody checked is the thing that reached the user.
#
# Usage: scripts/release-check.sh path/to/replay
set -euo pipefail

BIN="${1:?usage: release-check.sh <path-to-replay-binary>}"
[ -x "$BIN" ] || { echo "not executable: $BIN" >&2; exit 2; }

fail=0
note() { printf '  %-46s %s\n' "$1" "$2"; }
bad() { printf '  %-46s FAIL: %s\n' "$1" "$2" >&2; fail=1; }

echo "release-check: $BIN"

# The price table date the binary was compiled with. Read from the tree so the
# check compares the artifact against the source it claims to be built from,
# which is the comparison that was missing.
want_price="$(sed -n 's/^const PriceTableVersion = "\(.*\)"$/\1/p' internal/cachemodel/anthropic.go)"
[ -n "$want_price" ] || { echo "could not read PriceTableVersion from the tree" >&2; exit 2; }

# 1. The version is real. A binary reporting "dev" has not been stamped, which
#    is how a 0.5.0 build went on reporting 0.4.0 from a literal.
ver="$("$BIN" version 2>&1 || true)"
# A binary for another platform produces "Exec format error" from the shell,
# and every assertion below then grades that message instead of the binary.
# The v0.5.1 release run did exactly this against a darwin binary on a Linux
# runner and printed two content failures, which reads as "the artifact is
# wrong" when it means "nothing was checked". Refuse instead.
case "$ver" in
  *"Exec format error"*|*"cannot execute"*|*"Permission denied"*)
    echo "  cannot run this binary on this machine: $ver" >&2
    echo "release-check: NOT RUN (wrong platform or not executable)" >&2
    exit 2
    ;;
esac
case "$ver" in
  *dev*|"") bad "version is stamped" "reports '$ver'" ;;
  *) note "version is stamped" "$ver" ;;
esac

# 2. Run the binary on a real fixture and read what it says about it.
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/corpus/proj/subagents"
# The repository's own redacted session, copied to a main lane and a sub-agent
# lane of the same session. Two files, one session: exactly the case where a
# denominator counting files and one counting sessions disagree, which is the
# framing this check exists to pin. Hand-rolled JSONL was tried first and the
# binary correctly refused to price it, which is a fair answer to a fixture
# that was not a transcript.
fixture="internal/transcript/testdata/session-redacted.jsonl"
[ -f "$fixture" ] || { echo "fixture missing: $fixture" >&2; exit 2; }
cp "$fixture" "$work/corpus/proj/main.jsonl"
cp "$fixture" "$work/corpus/proj/subagents/lane.jsonl"

out="$("$BIN" cost "$work/corpus" 2>&1 || true)"

# 3. The sample size is stated in sessions.
#
# "N transcripts" as the denominator is the retracted framing: it counts lanes
# as independent draws. The header must say sessions. It may also report lanes,
# as long as it names both for what they are.
head_line="$(printf '%s\n' "$out" | head -1)"
case "$head_line" in
  *session*) note "denominator is stated in sessions" "$head_line" ;;
  *transcript*) bad "denominator is stated in sessions" "header counts transcripts: $head_line" ;;
  *) bad "denominator is stated in sessions" "no sample size in: $head_line" ;;
esac

# 4. The price date the artifact prints is the one it was built from.
#
# This is the assertion that would have caught v0.5.0: the shipped binary said
# 2026-06-24 while the tree said 2026-09-07, and nothing compared them.
case "$out" in
  *"$want_price"*) note "price table matches the tree" "$want_price" ;;
  *) bad "price table matches the tree" "tree says $want_price; binary printed $(printf '%s\n' "$out" | sed -n 's/.*prices dated \([0-9-]*\).*/\1/p' | head -1)" ;;
esac

# 5. An absent measurement is not reported as a good one.
#
# `replay corpus` published a `<synthetic>` row at 100.0% "calibrated" for a
# model whose turns never reached the API and so were never compared. If the
# per-model table is present, a row with no evidence must say so.
corpus_out="$("$BIN" corpus "$work/corpus" 2>&1 || true)"
if printf '%s\n' "$corpus_out" | grep -q '^| Model '; then
  if printf '%s\n' "$corpus_out" | grep -qE '^\| *<synthetic>.*\| *(100\.0%|[0-9.]+%) *\|.*calibrated'; then
    bad "unmeasured models are not 'calibrated'" "a <synthetic> row reports a percentage and a verdict of calibrated"
  else
    note "unmeasured models are not 'calibrated'" "checked"
  fi
else
  note "unmeasured models are not 'calibrated'" "no per-model table in this run"
fi

if [ "$fail" -ne 0 ]; then
  echo "release-check: FAILED" >&2
  exit 1
fi
echo "release-check: ok"
