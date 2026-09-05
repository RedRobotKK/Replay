#!/bin/sh
# Pre-flight for a public launch.
#
# Every defect found on 2026-09-05 had the same shape: a check that reported
# success because it never ran. A vacuous allowlist test. A CI job asserting a
# file's absence in a directory it never built. An installer declaring success
# without executing the binary. A latency figure that was the difference of two
# noise distributions. A ledger path exercised only by a stub that sent a header
# no real client sends. A dollar total citing the wrong document's date.
#
# A checklist of good intentions is that same failure one level up. So this is a
# script. It fails loudly, names the fix, and runs before anything is posted.
#
#   sh scripts/preflight.sh          full run
#   sh scripts/preflight.sh --quick  skip the slow gates
#
# Exit 0 = clear. Anything else = do not post.

set -u
cd "$(dirname "$0")/.." || exit 2

QUICK=0
[ "${1:-}" = "--quick" ] && QUICK=1

PASS=0; FAIL=0; WARN=0
red=''; grn=''; ylw=''; dim=''; off=''
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
  red=$(printf '\033[31m'); grn=$(printf '\033[32m'); ylw=$(printf '\033[33m')
  dim=$(printf '\033[2m'); off=$(printf '\033[0m')
fi

ok()   { PASS=$((PASS+1)); printf '  %sPASS%s  %s\n' "$grn" "$off" "$1"; }
bad()  { FAIL=$((FAIL+1)); printf '  %sFAIL%s  %s\n' "$red" "$off" "$1"
         [ $# -gt 1 ] && printf '        %s%s%s\n' "$dim" "$2" "$off"; return 0; }
warn() { WARN=$((WARN+1)); printf '  %sWARN%s  %s\n' "$ylw" "$off" "$1"
         [ $# -gt 1 ] && printf '        %s%s%s\n' "$dim" "$2" "$off"; return 0; }
group(){ printf '\n%s\n' "$1"; }

TMP=$(mktemp -d) || exit 2
trap 'rm -rf "$TMP"' EXIT INT TERM

group "1. Build and suite"

if go vet ./... >"$TMP/vet" 2>&1; then ok "go vet"; else bad "go vet" "$(head -3 "$TMP/vet")"; fi
if [ -z "$(gofmt -l . 2>/dev/null)" ]; then ok "gofmt"
else bad "gofmt" "unformatted: $(gofmt -l . | tr '\n' ' ')"; fi
if go test -count=1 ./... >"$TMP/test" 2>&1; then
  ok "go test ($(grep -c '^ok' "$TMP/test") packages)"
else
  bad "go test" "$(grep -E '^(FAIL|---)' "$TMP/test" | head -3)"
fi

# The zero-dependency claim is a headline and a badge. It is one grep.
if [ -f go.sum ]; then
  bad "zero dependencies" "go.sum exists; README and badge both say none"
elif grep -q '^require' go.mod 2>/dev/null; then
  bad "zero dependencies" "go.mod has a require block"
else
  ok "zero dependencies (go.mod $(wc -c < go.mod | tr -d ' ') bytes, no go.sum)"
fi

group "2. Published numbers against what the tool prints"

go build -o "$TMP/replay" ./cmd/replay 2>"$TMP/build" || bad "build binary" "$(head -2 "$TMP/build")"

# Anchor on the constant's own name. Taking the first date in the file grabbed
# RulesVersion, which is declared above it — the precise confusion this whole
# section exists to catch, reproduced inside the checker.
ptv=$(grep -E '^const PriceTableVersion' internal/cachemodel/anthropic.go \
        | grep -oE '[0-9]{4}-[0-9]{2}-[0-9]{2}')

if [ -x "$TMP/replay" ] && [ -d "$HOME/.claude/projects" ]; then
  "$TMP/replay" cost "$HOME/.claude/projects/" >"$TMP/cost" 2>&1
  missing=""
  for pat in "median task" "p90 task" "avoidable"; do
    grep -q "$pat" "$TMP/cost" || missing="$missing '$pat'"
  done
  [ -z "$missing" ] && ok "cost prints every line the README quotes" \
                    || bad "cost output changed shape" "missing:$missing"

  if grep -q "list prices dated $ptv" "$TMP/cost"; then
    ok "dollar figures cite the price table ($ptv)"
  else
    bad "dollars cite the wrong document" "$(head -1 "$TMP/cost")"
  fi

  if [ -n "$ptv" ]; then
    tsec=$(date -u -j -f "%Y-%m-%d" "$ptv" +%s 2>/dev/null || date -u -d "$ptv" +%s 2>/dev/null)
    if [ -n "$tsec" ]; then
      days=$(( ( $(date -u +%s) - tsec ) / 86400 ))
      [ "$days" -gt 60 ] \
        && warn "price table is $days days old" "verify against current rates before quoting dollars in public" \
        || ok "price table is $days days old"
    fi
  fi
else
  warn "no corpus to check against" "run on the machine with ~/.claude/projects"
fi

# A withdrawn number must not survive anywhere. This is the check that would
# have caught 48µs sitting in six files after being corrected in one.
strays=$(grep -rl "48µs" README.md docs/ 2>/dev/null | grep -v "proxy-latency-2026-09-03.md" | tr '\n' ' ')
if [ -n "$(echo "$strays" | tr -d ' ')" ]; then
  for f in $strays; do
    # A withdrawn number may appear where it is being withdrawn. Anything else
    # is the old claim still standing.
    grep -qE "was published here first|was noise|corrected|withdrawn" "$f" \
      || bad "withdrawn figure still stated as current in $f" "48µs was corrected on 2026-09-05"
  done
  [ "$FAIL" -eq 0 ] && ok "withdrawn latency figure appears only in its correction note"
else
  ok "no stray withdrawn latency figures"
fi

group "3. Installer, end to end"

if [ -f install.sh ]; then
  sh -n install.sh 2>"$TMP/sh" && ok "install.sh parses" || bad "install.sh parses" "$(head -2 "$TMP/sh")"
  grep -q 'version >/dev/null' install.sh \
    && ok "installer runs the binary before declaring success" \
    || bad "installer never executes what it installed" "the exact defect this file exists for"
  grep -q 'buymeacoffee' install.sh \
    && ok "funding line on the success path" \
    || warn "no funding line" "the success path is the only moment a nag-free tool gets to ask"
else
  bad "install.sh missing"
fi

VEND=../RedRobot.jp/src/vendor/replay-install.sh
if [ -f "$VEND" ]; then
  if [ "$(shasum -a 256 install.sh | cut -d' ' -f1)" = "$(shasum -a 256 "$VEND" | cut -d' ' -f1)" ]; then
    ok "vendored fallback matches upstream byte for byte"
  else
    bad "vendored installer has drifted" "(cd ../RedRobot.jp && npm run vendor:installer)"
  fi
else
  warn "site checkout not found" "cannot compare the vendored installer"
fi

group "4. Licence and distribution"

for f in LICENSE NOTICE CLA.md CONTRIBUTING.md CITATION.cff; do
  [ -f "$f" ] && ok "$f" || bad "$f missing" "Apache 2.0 s4 requires LICENSE and NOTICE to ship with the binary"
done
grep -q "Apache-2.0" CITATION.cff 2>/dev/null \
  && ok "CITATION.cff agrees with LICENSE" \
  || warn "CITATION.cff licence field" "must not disagree with LICENSE"

group "5. Live proxy — the path no stub can verify"

if [ "$QUICK" -eq 1 ]; then
  warn "live proxy skipped (--quick)" "run the full pre-flight before posting anywhere"
elif curl -s -m 2 -o /dev/null "http://127.0.0.1:11434/v1/models" 2>/dev/null; then
  "$TMP/replay" serve -upstream http://127.0.0.1:11434 -listen 127.0.0.1:4199 \
    -ledger "$TMP/ledger" >"$TMP/serve.log" 2>&1 &
  SRV=$!
  i=0; while [ "$i" -lt 30 ]; do
    curl -s -m 1 -o /dev/null "http://127.0.0.1:4199/replay/status" 2>/dev/null && break
    i=$((i+1)); sleep 0.3
  done
  model=$(curl -s -m 3 http://127.0.0.1:11434/v1/models | sed -n 's/.*"id":"\([^"]*\)".*/\1/p' | head -1)
  # Deliberately NO session header: every OpenAI-compatible client omits the
  # Claude Code one, and that omission silently dropped every ledger record
  # until 80b3651.
  curl -s -m 120 -o /dev/null -X POST http://127.0.0.1:4199/v1/chat/completions \
    -H 'content-type: application/json' \
    -d "{\"model\":\"$model\",\"messages\":[{\"role\":\"system\",\"content\":\"terse\"},{\"role\":\"user\",\"content\":\"say OK\"}],\"max_tokens\":8}" 2>/dev/null
  sleep 1
  if [ -n "$(find "$TMP/ledger" -name '*.jsonl' 2>/dev/null)" ]; then
    ok "OpenAI-compatible request reaches the ledger with no session header"
  else
    bad "ledger wrote nothing" "regression of the 80b3651 fix: a real client sends no session header"
  fi
  kill "$SRV" 2>/dev/null; wait "$SRV" 2>/dev/null
else
  warn "no local OpenAI-compatible server on :11434" "start ollama, or this path is stub-only again"
fi

printf '\n%s\n' "----------------------------------------------------------------"
printf '  %s%d passed%s   %s%d failed%s   %s%d warnings%s\n' \
  "$grn" "$PASS" "$off" "$red" "$FAIL" "$off" "$ylw" "$WARN" "$off"
if [ "$FAIL" -gt 0 ]; then
  printf '\n  %sDo not post.%s Fix the failures and run again.\n\n' "$red" "$off"; exit 1
fi
if [ "$WARN" -gt 0 ]; then
  printf '\n  %sClear, with warnings.%s Read each and decide deliberately.\n\n' "$ylw" "$off"; exit 0
fi
printf '\n  %sClear to launch.%s\n\n' "$grn" "$off"
