#!/bin/sh
# Verify Replay against a live OpenAI-compatible provider that actually caches.
#
# This exists because usage.FromInclusive — the code that stops OpenAI's
# inclusive prompt_tokens being double-counted — has never run with anything
# cached. Ollama reports no cached tokens at all, so the local run could not
# exercise it. The error that code prevents grows with hit rate, so it is
# largest on exactly the sessions Replay exists for.
#
#   DEEPSEEK_API_KEY=... sh scripts/verify-provider.sh deepseek
#   OPENAI_API_KEY=...   sh scripts/verify-provider.sh openai
#
# Costs a few cents. Sends a synthetic prompt with no real content.
#
# It does not assume field names. Replay keeps the provider's usage object
# verbatim in raw_usage, so this diffs what the provider SENT against what
# Replay PARSED, and reports any cache-shaped field Replay ignored. That is the
# whole reason raw_usage is kept.

set -eu
cd "$(dirname "$0")/.."

PROVIDER="${1:-deepseek}"
case "$PROVIDER" in
  deepseek) BASE="https://api.deepseek.com"; MODEL="deepseek-chat"; KEY="${DEEPSEEK_API_KEY:-}" ;;
  openai)   BASE="https://api.openai.com";   MODEL="gpt-4o-mini";   KEY="${OPENAI_API_KEY:-}" ;;
  *) echo "usage: $0 [deepseek|openai]" >&2; exit 2 ;;
esac
[ -n "$KEY" ] || { echo "no API key in the environment for $PROVIDER" >&2; exit 2; }

TMP=$(mktemp -d); trap 'rm -rf "$TMP"' EXIT INT TERM
go build -o "$TMP/replay" ./cmd/replay

"$TMP/replay" serve -upstream "$BASE" -listen 127.0.0.1:4198 -ledger "$TMP/ledger" \
  >"$TMP/serve.log" 2>&1 &
SRV=$!
trap 'kill $SRV 2>/dev/null; rm -rf "$TMP"' EXIT INT TERM
i=0; while [ "$i" -lt 40 ]; do
  curl -s -m 1 -o /dev/null http://127.0.0.1:4198/replay/status 2>/dev/null && break
  i=$((i+1)); sleep 0.25
done

# A prefix long enough to be cacheable, identical across both calls. Providers
# that cache implicitly need the SECOND request to hit, so we send two.
PREFIX=$(awk 'BEGIN{for(i=0;i<1400;i++) printf "The quick brown fox jumps over the lazy dog. "}')
python3 - "$PREFIX" >"$TMP/body.json" <<'PY'
import json, sys
print(json.dumps({
    "model": "__MODEL__",
    "messages": [
        {"role": "system", "content": sys.argv[1]},
        {"role": "user", "content": "Reply with exactly: OK"}],
    "max_tokens": 8,
}))
PY
sed -i.bak "s/__MODEL__/$MODEL/" "$TMP/body.json"

echo "Sending two identical requests through the proxy (the second should hit cache)."
for n in 1 2; do
  curl -s -m 120 -X POST http://127.0.0.1:4198/v1/chat/completions \
    -H 'content-type: application/json' \
    -H "authorization: Bearer $KEY" \
    --data-binary "@$TMP/body.json" >"$TMP/resp$n.json"
  echo "  request $n: $(python3 -c "
import json,sys
print(json.dumps(json.load(open('$TMP/resp$n.json')).get('usage',{})))" 2>/dev/null || echo 'unparseable')"
  sleep 2
done

sleep 1
LEDGER=$(find "$TMP/ledger" -name '*.jsonl' | head -1)
[ -n "$LEDGER" ] || { echo; echo "FAIL: no ledger record was written at all."; exit 1; }

echo
python3 - "$LEDGER" <<'PY'
import json, sys

# Any usage field whose name suggests caching. Deliberately not a fixed list:
# the point is to find the ones we do not know about.
def cacheish(name):
    n = name.lower()
    return "cache" in n or "cached" in n

rows = [json.loads(l) for l in open(sys.argv[1]) if l.strip()]
print(f"{len(rows)} ledger record(s)\n")

problems = []
for i, r in enumerate(rows, 1):
    resp = r.get("response", {})
    raw, parsed = resp.get("raw_usage") or {}, resp.get("usage") or {}
    print(f"--- request {i} ---")
    print(f"  provider sent : {json.dumps(raw)}")
    print(f"  replay parsed : {json.dumps(parsed)}")

    # Flatten one level so prompt_tokens_details.cached_tokens is seen too.
    flat = {}
    for k, v in raw.items():
        if isinstance(v, dict):
            for k2, v2 in v.items():
                flat[f"{k}.{k2}"] = v2
        else:
            flat[k] = v

    sent = {k: v for k, v in flat.items() if cacheish(k) and isinstance(v, int) and v > 0}
    got = (parsed.get("cache_read_input_tokens", 0) or 0) + \
          (parsed.get("cache_creation_input_tokens", 0) or 0)
    if sent and got == 0:
        problems.append(
            f"request {i}: provider reported {sent} but Replay recorded zero cache "
            f"activity. The adapter is not reading this provider's field names.")

    # The engine's own invariant: prompt == fresh + read + write.
    p = parsed.get("input_tokens", 0) or 0
    if raw.get("prompt_tokens") is not None:
        total = p + (parsed.get("cache_read_input_tokens", 0) or 0) + \
                    (parsed.get("cache_creation_input_tokens", 0) or 0)
        if total != raw["prompt_tokens"]:
            problems.append(
                f"request {i}: parts do not add up. provider prompt_tokens="
                f"{raw['prompt_tokens']}, replay fresh+read+write={total}. "
                f"This is the inclusive/exclusive double-count.")
    print()

if problems:
    print("PROBLEMS FOUND\n")
    for p in problems:
        print("  " + p)
    sys.exit(1)
print("No discrepancy between what the provider sent and what Replay recorded.")
PY
