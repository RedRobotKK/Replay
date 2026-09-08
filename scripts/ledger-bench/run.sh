#!/usr/bin/env bash
# Pins the benchmark's output so a change to the scoring is visible in CI.
#
# The numbers below are properties of a committed fixture, not of any machine, so
# a difference here means the scoring rule moved. That is the only thing this
# script is for: the benchmark is a claim about how four products account for
# tokens, and a claim nobody re-checks is the defect this repository exists to
# find.
#
# If a figure legitimately changes — a price table update, a corrected multiplier
# — update the expectations here in the same commit, and say why in the message.
set -euo pipefail

cd "$(dirname "$0")"
out=$(python3 bench.py --json)

fail=0
check() { # name expected actual
  if [ "$2" = "$3" ]; then
    printf '  ok    %-22s %s\n' "$1" "$3"
  else
    printf '  FAIL  %-22s want %s, got %s\n' "$1" "$2" "$3"
    fail=1
  fi
}

get() { printf '%s' "$out" | python3 -c "import json,sys;print(json.load(sys.stdin)$1)"; }

echo "ledger-bench, against the committed fixture"

check "prompt tokens"   "6638500" "$(get "['corpus']['prompt_tokens']")"
check "models"          "3"       "$(get "['corpus']['models']")"
check "cache share"     "0.9475"  "$(get "['corpus']['cache_share']")"
check "QM charge"       "33.19"   "$(get "['qm_charge_usd']")"
check "true cost"       "5.72"    "$(get "['true_cost_usd']")"
check "ratio"           "5.801"   "$(get "['ratio']")"
check "cached tokens"   "6290000" "$(get "['memorable_tokens']['cached_input_tokens']")"

# NOT MEASURED must never become a number. A product that cannot be scored from
# outside reports that it cannot, and the moment one of these turns into a zero
# the benchmark has started lying in the direction that flatters it.
for p in "River AI" "GBrain"; do
  got=$(printf '%s' "$out" | python3 -c "
import json,sys
rows=json.load(sys.stdin)['rows']
r=[x for x in rows if x['product']=='$p'][0]
print('yes' if 'NOT ' in r['after'] else 'NUMERIC:'+r['after'])")
  check "$p unmeasured" "yes" "$got"
done

echo
if [ "$fail" -ne 0 ]; then
  echo "ledger-bench: the scoring moved. Update the expectations deliberately or fix the cause." >&2
  exit 1
fi
echo "ledger-bench: all expectations hold."
