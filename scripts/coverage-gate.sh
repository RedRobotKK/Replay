#!/usr/bin/env bash
# Fail when statement coverage drops below the floor in .coverage-floor.
#
# The CI step this replaces printed the total and moved on, which is a
# measurement nobody consumes. A number printed into a log that no step reads
# is the same shape as a test that cannot fail: it looks like a check and
# nothing depends on it.
#
# WHY THE FLOOR IS A FILE AND NOT A LITERAL HERE. The README states the floor
# in a badge, and internal/regression FC-CG fails if the two disagree. One
# number, two readers, and neither can drift from the other quietly.
#
# WHY THIS RUNS ON ONE RUNNER. Coverage is not comparable across the matrix:
# files tagged //go:build unix are compiled out on Windows, so the same tree
# reports a different total there. A single floor applied to three runners
# would either be too low to mean anything on Linux or wrong on Windows. The
# gate runs on ubuntu-latest and the other two still print their total.
set -euo pipefail

profile="${1:-coverage.out}"
floor_file="${2:-.coverage-floor}"

[ -f "$profile" ] || { echo "coverage-gate: no profile at $profile" >&2; exit 1; }
[ -f "$floor_file" ] || { echo "coverage-gate: no floor at $floor_file" >&2; exit 1; }

floor=$(tr -d '[:space:]' < "$floor_file")
line=$(go tool cover -func="$profile" | tail -1)
actual=$(printf '%s' "$line" | grep -oE '[0-9]+\.[0-9]+%$' | tr -d '%')

if [ -z "$actual" ]; then
  echo "coverage-gate: could not read a total from: $line" >&2
  echo "coverage-gate: refusing to pass on an unreadable profile" >&2
  exit 1
fi

awk -v a="$actual" -v f="$floor" '
  BEGIN {
    printf "coverage: %.1f%%  floor: %s%%  headroom: %+.1f\n", a, f, a - f
    if (a < f) {
      printf "\ncoverage-gate: %.1f%% is below the floor of %s%%.\n", a, f
      print  "Add tests for what this change touched, or argue the floor down in"
      print  ".coverage-floor with the reason in the commit message. Lowering it"
      print  "silently is the one thing this gate exists to prevent."
      exit 1
    }
    if (a - f >= 2) {
      printf "\ncoverage-gate: %.1f%% sits %.1f points above the floor.\n", a, a - f
      print  "Raise .coverage-floor (and the README badge) to lock the gain in;"
      print  "headroom nobody claims is headroom that erodes."
    }
  }'
