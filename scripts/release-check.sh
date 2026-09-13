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

# 6. The wire contract this release is named for.
#
# v0.6.0 renames avoidableUsd to rebilledUsd across the payloads and bumps the
# corpus and pool schemas to v2. Nothing here could see that: every assertion
# above reads human prose, and a binary shipping the old field names would have
# passed all five.
#
# THE SEVERITY IS WHY THIS IS HERE RATHER THAN IN A UNIT TEST. This script runs
# AFTER goreleaser uploads, on purpose, because the artifact is what has to be
# right. So a wrong field name discovered here means a public release to pull.
# A unit test on the tree would not have caught a build that shipped from the
# wrong commit, which is the whole reason this file inspects the binary.
json="$("$BIN" cost --json "$work/corpus" 2>&1 || true)"
case "$json" in
  *'"rebilledUsd"'*) note "the payload uses the v0.6.0 field names" "rebilledUsd present" ;;
  *) bad "the payload uses the v0.6.0 field names" "no rebilledUsd in cost --json" ;;
esac
case "$json" in
  *'"avoidableUsd"'*|*'"avoidableShare"'*|*'"avoidableTokens"'*)
    bad "the retired field names are gone" "the binary still emits an avoidable* key" ;;
  *) note "the retired field names are gone" "no avoidable* key in cost --json" ;;
esac

# 7. The flag the CI gate is documented under still exists.
#
# --max-avoidable-usd became --max-rebilled-usd in the same release. Anyone
# whose pipeline calls the old name gets "flag provided but not defined" and
# exit 1, which docs/guide/commands.md tells CI authors means replay itself
# failed. Checking the new one exists is the half this script can prove.
# Captured first, then matched. This was a pipeline with grep, and `cost --help`
# exits non-zero the way usage output does, which `set -o pipefail` at the top of
# this script turned into a failed assertion about a flag that was present all
# along. An assertion that fails for a reason unrelated to what it asserts is
# the exact defect this file exists to catch, committed inside it.
help_out="$("$BIN" cost --help 2>&1 || true)"
case "$help_out" in
  *-max-rebilled-usd*) note "the CI gate flag is present" "--max-rebilled-usd" ;;
  *) bad "the CI gate flag is present" "--max-rebilled-usd is not in cost --help" ;;
esac

if [ "$fail" -ne 0 ]; then
  echo "release-check: FAILED" >&2
  exit 1
fi
echo "release-check: ok"
