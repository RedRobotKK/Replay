#!/bin/sh
# The installer sends nothing, asserted rather than promised.
#
# install.sh prints two sentences to the reader while it runs:
#
#   "Replay originates no network request you did not type."
#   "Free to run, BUSL 1.1, no account, no telemetry."
#
# On 2026-09-08 an opt-in install beacon was built here and reverted the next
# day. READINESS.md had already pre-registered the decision — "the zero-telemetry
# promise is the asset, and it is worth more than knowing the install count" —
# and the beacon would have been default-off, so it bought a signal near zero on
# the one day it was meant to measure while putting a phone-home in a `curl | sh`
# one-liner aimed at the audience most likely to read it.
#
# This is what survived, and it is worth more than the beacon was: the claim is
# now a check that fails rather than a sentence in a README. It works by putting
# a stub curl and wget on PATH and reading what the script would send, so it
# tests behaviour instead of grepping the source for words.
set -eu
root=$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)
work=$(mktemp -d); trap 'rm -rf "$work"' EXIT
fail=0
say() { printf '%s\n' "$*"; }

# T1: no analytics host appears anywhere in the installer.
#
# Cheap, and it is the check that would have caught the beacon on the way in.
# The pattern names endpoints, never the word "telemetry" on its own: the script
# prints "no account, no telemetry" as its closing line, and a check that fired
# on the claim it exists to protect is a check that has to be switched off.
ANALYTICS='posthog|mixpanel|amplitude|segment\.(io|com)|plausible\.io|/capture/|/collect|google-analytics'
if grep -nEi "$ANALYTICS" "$root/install.sh" >/dev/null 2>&1; then
  say "FAIL T1: install.sh names an analytics endpoint:"
  grep -nEi "$ANALYTICS" "$root/install.sh" | sed 's/^/      /'
  fail=1
else
  say "ok T1  no analytics endpoint in the installer"
fi

# T2: the only hosts it contacts are the ones distributing the software.
#
# github.com and api.github.com serve the release and its checksums, and
# SURFACES.md lists them as installer-only. Anything else is a new outbound
# surface that nothing documents.
# Only URLs handed to something that fetches. The first version of this check
# took every https:// string in the file and reported three failures that were
# a comment showing the one-liner, an error message pointing at go.dev, and
# cosign's OIDC issuer — which is an identity asserted about a certificate, not
# a host anything connects to. Treating "appears in the file" as "is contacted"
# is the same defect this repository keeps finding in its own measurements.
t2=0
hosts=$(grep -nE '(curl|wget|save)[^|]*https://' "$root/install.sh" \
        | grep -vE '^[0-9]+: *#' \
        | grep -oE 'https://[a-zA-Z0-9.-]+' | sed 's|https://||' | sort -u)
for h in $hosts; do
  case "$h" in
    github.com|api.github.com|objects.githubusercontent.com|raw.githubusercontent.com) ;;
    redrobot.jp) ;;  # narrowed below: only the documented install.sh path
    *) say "FAIL T2: fetches from an undocumented host: $h"; fail=1; t2=1 ;;
  esac
done
# redrobot.jp is allowed for exactly one path — the documented one-liner in the
# help text. Allowing the whole host was a real hole: the beacon reverted on
# 2026-09-09 posted to redrobot.jp/Replay/installed, so a host-level allowlist
# would have waved through the precise change this file exists to stop.
#
# The allowed path is the CANONICAL one, and this check spent a while demanding
# the other. internal/regression/install_url_test.go collapsed three working
# addresses onto https://redrobot.jp/replay.sh, install.sh moved with it, and
# this list did not, so T2 failed on the single URL the installer is supposed to
# print. Two guards wanting opposite things is the shape the FD8 comment in
# install.sh describes, and a check that fails on correct code is a check
# somebody switches off. The canonical address is asserted in that test; this
# file allows it and nothing else.
for u in $(grep -oE 'https://redrobot\.jp[^" ]*' "$root/install.sh" | sort -u); do
  case "$u" in
    https://redrobot.jp/replay.sh) ;;
    *) say "FAIL T2: an undocumented redrobot.jp URL: $u"; fail=1; t2=1 ;;
  esac
done
[ "$t2" = 0 ] && say "ok T2  fetches only from the release hosts ($(printf '%s' "$hosts" | tr '\n' ' '))"

# T3: the claim the script prints is still there.
#
# The inverse of the check this file used to hold. If someone removes the line
# in order to add a beacon, that is exactly the change that should be visible in
# a diff and argued for, rather than arrived at quietly.
if grep -q 'no account, no telemetry' "$root/install.sh"; then
  say "ok T3  the script still makes the claim"
else
  say "FAIL T3: install.sh no longer claims 'no account, no telemetry'"
  say "      If that was deliberate, READINESS.md item 4 is the argument to answer."
  fail=1
fi

# T4: the claim and the behaviour agree, observed on the wire.
#
# T1 and T2 read the file. This runs the script's own network helper against a
# stub curl and a stub wget, so a request assembled at runtime — a variable, an
# eval, a here-doc — is still caught.
mkdir -p "$work/bin"
for tool in curl wget; do
  cat > "$work/bin/$tool" <<'STUB'
#!/bin/sh
printf '%s\n' "$*" >> "$SENT"
STUB
  chmod +x "$work/bin/$tool"
done
: > "$work/sent"
PATH="$work/bin:$PATH" SENT="$work/sent" sh "$root/install.sh" --help >/dev/null 2>&1 || true
if [ -s "$work/sent" ]; then
  say "FAIL T4: --help reached the network:"; sed 's/^/      /' "$work/sent"; fail=1
else
  say "ok T4  --help sends nothing"
fi

[ "$fail" = 0 ] && say "PASS" || { say "FAILED"; exit 1; }
