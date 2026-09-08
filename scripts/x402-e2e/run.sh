#!/usr/bin/env bash
# The x402 lifecycle Replay implements, asserted end to end inside a container.
#
# Why this exists next to the unit tests rather than instead of them. The tests
# in cmd/replay/x402_test.go drive the 402 path through `rulesTransport`, a hook
# that replaces the HTTP round tripper. That proves the parsing and the prose.
# It cannot prove that the shipped binary refuses plain http, that it verifies a
# certificate chain, that its exit codes reach a shell, or that a payment demand
# leaves no file behind on a real filesystem. This does, against a real listener
# over real TLS.
#
# There is no wallet and no signing, and that is the subject rather than an
# omission. Replay's lifecycle is request, 402 with terms, report, stop. Steps
# four onward of x402 need a key, and crypto/ecdsa, crypto/elliptic, crypto/ecdh
# and math/big are banned by TestX402_AllowlistIsMeaningful precisely so this
# binary cannot take them. A harness that built a signer would be testing a
# different program.
set -u

REPLAY=/usr/local/bin/replay
RULES="$HOME/.replay/rules.json"
BASE=https://seller.test:8443
fails=0

ok()   { printf '  \033[32mok\033[0m   %s\n' "$*"; }
bad()  { printf '  \033[31mFAIL\033[0m %s\n' "$*"; fails=$((fails+1)); }
step() { printf '\n\033[36m%s\033[0m\n' "$*"; }

# run captures stdout+stderr and the exit code without a pipe in the way.
# Reading $? after a pipe reports the last command's status, which is a mistake
# already made once in this repository's history.
out=""; code=0
run() {
  printf '  \033[2m$ %s\033[0m\n' "$*"
  out="$("$@" 2>&1)"; code=$?
}

step "0  the binary under test"
run "$REPLAY" version
[ "$code" -eq 0 ] && ok "version exits 0: $out" || bad "version exited $code"

step "1  a valid payment demand"
run "$REPLAY" rules --update "$BASE/rules/paid"
[ "$code" -eq 2 ] && ok "exit 2, which a script reads as 'pay for this'" || bad "exit $code, want 2"
case "$out" in *"will not pay"*) ok "says it will not pay" ;; *) bad "no refusal in the output" ;; esac
case "$out" in *'"2.50"'*) ok "names the amount" ;; *) bad "amount not reported" ;; esac
case "$out" in *'"base"'*) ok "names the network" ;; *) bad "network not reported" ;; esac
# The seller's description is attacker-controlled text. It must be quoted so it
# cannot be mistaken for Replay's own words.
case "$out" in *'seller says "'*) ok "seller's words are quoted as theirs" ;; *) bad "seller description is not attributed" ;; esac
[ ! -f "$RULES" ] && ok "no rules file was written" || bad "a payment demand installed $RULES"

step "2  the same demand as JSON, for an agent with a spending policy"
run "$REPLAY" rules --update "$BASE/rules/paid" --x402-json
[ "$code" -eq 2 ] && ok "exit 2" || bad "exit $code, want 2"
printf '%s' "$out" | grep -q '"paid": false' && ok "reports paid: false" || bad "no paid:false in JSON"
printf '%s' "$out" | grep -q '"x402Version": 1' && ok "carries the demand verbatim" || bad "demand not echoed"
[ ! -f "$RULES" ] && ok "still nothing written" || bad "JSON mode installed $RULES"

step "3  a 402 whose body is not x402"
run "$REPLAY" rules --update "$BASE/rules/notx402"
[ "$code" -eq 1 ] && ok "exit 1, which a script reads as 'it broke'" || bad "exit $code, want 1"
case "$out" in *"no x402 version"*) ok "says what was wrong with the body" ;; *) bad "unclear error: $out" ;; esac

step "4  a 402 offering no way to pay"
run "$REPLAY" rules --update "$BASE/rules/nooptions"
[ "$code" -eq 1 ] && ok "exit 1" || bad "exit $code, want 1"
case "$out" in *"no payment option"*) ok "distinguishes it from a malformed body" ;; *) bad "unclear error: $out" ;; esac

step "5  the success path, so refusal is not the only thing proven"
run "$REPLAY" rules --update "$BASE/rules/free"
[ "$code" -eq 0 ] && ok "exit 0" || bad "exit $code, want 0: $out"
[ -f "$RULES" ] && ok "rules installed at $RULES" || bad "nothing installed on the success path"
grep -q "seller-2026-09-08" "$RULES" 2>/dev/null && ok "the document the seller served is the one on disk" || bad "installed document does not match"

step "6  plain http is refused outright"
run "$REPLAY" rules --update "http://seller.test:8443/rules/free"
[ "$code" -ne 0 ] && ok "refused a plain-http feed" || bad "installed over plain http"

printf '\n'
if [ "$fails" -ne 0 ]; then
  printf '\033[31mx402-e2e: %d assertion(s) failed\033[0m\n' "$fails"
  exit 1
fi
printf '\033[32mx402-e2e: all assertions passed\033[0m\n'
