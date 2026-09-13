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
#   - NOTHING IS SENT. This hook makes no network call of any kind, and the
#     v0.6.0 plugin has no code that could. That is a deliberate build-order
#     decision taken on 2026-09-12, not an oversight: Replay Watch, the paid
#     per-repository service, does not merge ahead of the first sold forensics
#     week. The sending half arrives with the first engagement that pays for
#     it, and until then this hook prints a line and stops.
#
#     What was here until today: a curl POST to replay.doctor/api/watch, gated
#     on a config file, calling `replay watch emit`. That verb does not exist,
#     so the block could not have worked, and every failure in it was swallowed
#     by `2>/dev/null || true`. A silent no-op shipped to everyone who installs
#     is worse than an absent feature, because the absent one is honest.
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
  usd=$(printf '%s' "$summary" | tr -d '\n' | sed -n 's/.*"rebilledUsd"[[:space:]]*:[[:space:]]*\([0-9.]*\).*/\1/p' | head -1)
  tok=$(printf '%s' "$summary" | tr -d '\n' | sed -n 's/.*"rebilledTokens"[[:space:]]*:[[:space:]]*\([0-9]*\).*/\1/p' | head -1)
  if [ -n "$usd" ] && [ -n "$tok" ] && [ "$tok" != "0" ]; then
    printf 'replay: $%s at list billed twice in this session (%s tokens). Which turn: replay diff "%s"\n' "$usd" "$tok" "$transcript"
  fi
fi

# The hook ends here, and ending here is the feature.
#
# A reader who installs this can confirm in one screen that it reads one file,
# prints at most one line, and opens no socket. That is the claim the project
# is sold on, and it is worth more before a launch than a send path with no
# customer behind it.
exit 0
