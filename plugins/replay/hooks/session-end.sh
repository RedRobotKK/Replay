#!/bin/sh
# Replay Doctor, SessionEnd hook.
#
# Reads the session Claude Code just closed and prints one line: what the
# prompt cache re-billed in that session, and the command that names the turn.
# That line is the whole distribution mechanism of this project, so it has to
# be true, quiet, and never in the way:
#
#   - SessionEnd only. Nothing here runs on a turn.
#   - If `replay` is not installed, print how to install it once, then nothing.
#   - If the transcript cannot be read, print nothing. A hook that fails loudly
#     at the end of every session is a hook people remove.
#   - Nothing is sent unless BOTH of these exist: ~/.config/replay/watch.toml
#     naming a repository and a key, and a record the binary agreed to write
#     (it refuses without the watch consent grant). Then, and only then, this
#     script posts that one file with curl. The binary itself never sends;
#     this script is the "something outside the package" that does.
#
# Input: Claude Code's SessionEnd JSON on stdin (session_id, transcript_path,
# cwd, reason). Only transcript_path is used.
set -u

input=$(cat 2>/dev/null || true)
transcript=$(printf '%s' "$input" | sed -n 's/.*"transcript_path"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)
[ -n "$transcript" ] && [ -r "$transcript" ] || exit 0

if ! command -v replay >/dev/null 2>&1; then
  printf 'replay: not installed. One line, read it first: curl -fsSL https://replay.doctor/replay.sh | less\n'
  exit 0
fi

# The tip line: what this session re-billed, and how to see the turn. Read from
# `cost --json` on the one transcript and reduced to a single line here, so the
# hook never prints the full report at the end of every session. If the read
# fails, the transcript is empty, or the figures cannot be found, print nothing.
summary=$(replay cost --json "$transcript" 2>/dev/null || true)
if [ -n "$summary" ]; then
  usd=$(printf '%s' "$summary" | tr -d '\n' | sed -n 's/.*"avoidableUsd"[[:space:]]*:[[:space:]]*\([0-9.]*\).*/\1/p' | head -1)
  tok=$(printf '%s' "$summary" | tr -d '\n' | sed -n 's/.*"avoidableTokens"[[:space:]]*:[[:space:]]*\([0-9]*\).*/\1/p' | head -1)
  if [ -n "$usd" ] && [ -n "$tok" ] && [ "$tok" != "0" ]; then
    printf 'replay: $%s at list billed twice in this session (%s tokens). Which turn: replay diff "%s"\n' "$usd" "$tok" "$transcript"
  fi
fi

# Optional watch. Off unless the operator wrote the config file.
cfg="${XDG_CONFIG_HOME:-$HOME/.config}/replay/watch.toml"
[ -r "$cfg" ] || exit 0
repo=$(sed -n 's/^[[:space:]]*repo[[:space:]]*=[[:space:]]*"\([^"]*\)".*/\1/p' "$cfg" | head -1)
key=$(sed -n 's/^[[:space:]]*key[[:space:]]*=[[:space:]]*"\([^"]*\)".*/\1/p' "$cfg" | head -1)
endpoint=$(sed -n 's/^[[:space:]]*endpoint[[:space:]]*=[[:space:]]*"\([^"]*\)".*/\1/p' "$cfg" | head -1)
[ -n "$repo" ] && [ -n "$key" ] || exit 0
endpoint=${endpoint:-https://replay.doctor/api/watch}

out=$(mktemp -d 2>/dev/null) || exit 0
trap 'rm -rf "$out"' EXIT

# The binary writes the record only if the watch consent grant exists; it
# refuses otherwise and this script has nothing to send. Counts only, no text.
replay watch emit --repo "$repo" --key "$key" --out "$out" "$transcript" >/dev/null 2>&1 || exit 0
rec=$(ls "$out"/replay-watch-*.json 2>/dev/null | head -1)
[ -n "$rec" ] || exit 0

# One typed-by-config POST. The reply names the digest; failures are silent
# here and visible on the repository page as a missing week, which is the
# honest place for them.
curl -sS --max-time 8 -X POST -H "Authorization: Bearer $key" --data-binary @"$rec" "$endpoint" >/dev/null 2>&1 || true
exit 0
