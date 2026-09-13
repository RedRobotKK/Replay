#!/bin/sh
# Tests for the plugin that need no Claude Code: the manifests parse, the hook
# is wired to SessionEnd and nothing else, the script exits 0 and prints
# nothing when it has nothing to say, and it sends nothing without a config.
#
#   sh plugins/replay/test.sh
set -u
here=$(cd "$(dirname "$0")" && pwd)
fail=0
say() { printf '%s\n' "$*"; }
bad() { say "FAIL: $*"; fail=1; }

python3 - "$here" <<'EOF' || fail=1
import json, sys, os
here = sys.argv[1]
root = os.path.dirname(os.path.dirname(here))
m = json.load(open(os.path.join(root, '.claude-plugin', 'marketplace.json')))
assert m['plugins'][0]['source'] == './plugins/replay', 'marketplace does not point at plugins/replay'
p = json.load(open(os.path.join(here, '.claude-plugin', 'plugin.json')))
assert p['name'] == 'replay' and p['version'] == m['plugins'][0]['version'], 'plugin and marketplace versions differ'
h = json.load(open(os.path.join(here, 'hooks', 'hooks.json')))
events = list(h['hooks'].keys())
assert events == ['SessionEnd'], f'the hook is wired to {events}; only SessionEnd may run, so no turn ever waits on it'
cmd = h['hooks']['SessionEnd'][0]['hooks'][0]
assert cmd['type'] == 'command' and 'session-end.sh' in cmd['command'], 'SessionEnd does not run the script'
assert cmd.get('timeout', 999) <= 30, 'the hook timeout must be short; a slow hook is a hook people remove'
for f in ('skills/replay-doctor/SKILL.md', 'commands/replay.md', 'hooks/session-end.sh'):
    t = open(os.path.join(here, f), encoding='utf-8').read()
    assert '—' not in t, f'{f} contains an em-dash'
    assert ' save ' not in t.lower() or 'never say "save"' in t.lower() or 'the word "save"' in t.lower(), f'{f} uses the forecast word save'
print('manifests ok')
EOF

[ -x "$here/hooks/session-end.sh" ] || bad "hooks/session-end.sh is not executable (chmod +x)"

# No stdin, no transcript: exit 0, print nothing.
out=$(printf '' | sh "$here/hooks/session-end.sh" 2>&1); rc=$?
[ "$rc" -eq 0 ] || bad "empty input exited $rc"
[ -z "$out" ] || bad "empty input printed: $out"

# A transcript path that does not exist: exit 0, print nothing.
out=$(printf '{"session_id":"x","transcript_path":"/nonexistent/x.jsonl"}' | sh "$here/hooks/session-end.sh" 2>&1); rc=$?
[ "$rc" -eq 0 ] || bad "missing transcript exited $rc"
[ -z "$out" ] || bad "missing transcript printed: $out"

# A readable transcript but no replay on PATH: exactly the install hint, nothing sent.
tmp=$(mktemp -d); printf '{}\n' > "$tmp/s.jsonl"
out=$(printf '{"transcript_path":"%s"}' "$tmp/s.jsonl" | PATH=/usr/bin:/bin HOME="$tmp" sh "$here/hooks/session-end.sh" 2>&1); rc=$?
[ "$rc" -eq 0 ] || bad "no-binary case exited $rc"
case "$out" in *"replay.doctor/replay.sh"*) ;; *) bad "no-binary case did not print the install hint: $out";; esac
rm -rf "$tmp"

# The dollar figure is printed to two decimal places.
#
# cost --json carries the unrounded float, so the line read "$1.89229 at list
# billed twice" until 2026-09-13: a number nobody says out loud, and false
# precision on a figure that rests on a byte-to-token fit.
#
# ASSERTED BY EQUALITY, NOT BY SUBSTRING, because "$1.89" is a substring of
# "$1.89229" and a contains-check would have passed against the bug it exists
# to catch.
if command -v replay >/dev/null 2>&1; then
  fixture="$here/../../internal/transcript/testdata/session-redacted.jsonl"
  if [ -r "$fixture" ]; then
    out=$(printf '{"transcript_path":"%s"}' "$fixture" | sh "$here/hooks/session-end.sh" 2>&1)
    # Pull the figure back out and compare it to its own two-decimal rounding.
    got=$(printf '%s' "$out" | sed -n 's/^replay: \$\([0-9.]*\) .*/\1/p')
    [ -n "$got" ] || bad "the tip line printed no dollar figure: $out"
    want=$(printf '%s' "$got" | LC_ALL=C awk '{ printf "%.2f", $1 }')
    [ "$got" = "$want" ] || bad "the tip line printed $got, which is not $want to the cent"
  fi
fi

[ "$fail" -eq 0 ] && say "plugin ok" || exit 1
