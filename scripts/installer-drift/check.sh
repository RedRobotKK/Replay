#!/usr/bin/env bash
# Does the installer people actually run match the one in this repository?
#
# On 2026-09-09 it did not. install.sh here had --no-tui, merged in #91; the copy
# served from redrobot.jp was 28 lines behind and rejected the flag. Every reader
# who curls the one-liner gets the hosted file, so the repo's version is the one
# nobody runs.
#
# This is the same lesson as scripts/release-check.sh, which exists because a
# published binary was 75 days behind the tree: the artifact and the source are
# different things, and only one of them is what a user meets.
#
# Advisory by design. A deploy lag is a fact about the website, not a fault in
# the commit under test, and failing a pull request for it would put a red cross
# on work that did not cause it. It prints loudly and exits 0 unless
# INSTALLER_DRIFT_STRICT=1 says otherwise.
set -uo pipefail
cd "$(dirname "$0")/../.."

URLS="https://redrobot.jp/replay.sh https://redrobot.jp/Replay/install.sh"
repo_sha=$(shasum -a 256 install.sh | cut -d' ' -f1)
repo_lines=$(wc -l < install.sh | tr -d ' ')
drift=0

printf 'repo install.sh   %s  %s lines\n\n' "${repo_sha:0:12}" "$repo_lines"

for u in $URLS; do
  tmp=$(mktemp)
  if ! curl -fsSL --max-time 20 "$u" -o "$tmp" 2>/dev/null; then
    printf '  %-42s UNREACHABLE\n' "$u"
    rm -f "$tmp"; continue
  fi
  sha=$(shasum -a 256 "$tmp" | cut -d' ' -f1)
  lines=$(wc -l < "$tmp" | tr -d ' ')
  if [ "$sha" = "$repo_sha" ]; then
    printf '  %-42s matches\n' "$u"
  else
    drift=1
    printf '  %-42s DRIFTED  %s  %s lines (%+d)\n' "$u" "${sha:0:12}" "$lines" "$((lines - repo_lines))"
    # Flags are what a reader reaches for and what a script passes, so a
    # missing one is the difference that bites first.
    miss=$(comm -23 \
      <(grep -oE '^[[:space:]]*--[a-z-]+\)' install.sh | tr -d ' )' | sort -u) \
      <(grep -oE '^[[:space:]]*--[a-z-]+\)' "$tmp"     | tr -d ' )' | sort -u) | tr '\n' ' ')
    [ -n "${miss// /}" ] && printf '      flags the hosted copy does not accept: %s\n' "$miss"
  fi
  rm -f "$tmp"
done

echo
if [ "$drift" -eq 0 ]; then
  echo "installer-drift: the hosted installer matches this repository."
  exit 0
fi
echo "installer-drift: the hosted installer is NOT this file. Readers run the hosted one." >&2
[ "${INSTALLER_DRIFT_STRICT:-0}" = "1" ] && exit 1
exit 0
