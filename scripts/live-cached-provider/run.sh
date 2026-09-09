#!/bin/sh
# READINESS failure mode 1, in one command.
#
# The OpenAI-compatible path's inclusive-to-exclusive conversion has never run
# against a provider that reports a cache. Every test to date used Ollama, which
# reports none — re-verified 2026-09-09, where two identical requests returned
# identical usage and no prompt_tokens_details at all.
#
# This costs a few cents and is the last unmet abort criterion in READINESS.md.
#
#   REPLAY_LIVE_KEY=sk-... sh scripts/live-cached-provider/run.sh
#
# Defaults to DeepSeek because it is the cheap option and the interesting one:
# it sends both cache vocabularies, and whether they agree has never been
# checked. Override for any OpenAI-compatible provider that reports a cache.
set -eu
: "${REPLAY_LIVE_KEY:?set REPLAY_LIVE_KEY to a provider key. Nothing was run and nothing was billed.}"
export REPLAY_LIVE_BASE_URL="${REPLAY_LIVE_BASE_URL:-https://api.deepseek.com}"
export REPLAY_LIVE_MODEL="${REPLAY_LIVE_MODEL:-deepseek-chat}"

printf 'Two billable requests to %s (%s), a few cents.\n' \
  "$REPLAY_LIVE_BASE_URL" "$REPLAY_LIVE_MODEL"
cd "$(CDPATH= cd -- "$(dirname -- "$0")/../.." && pwd)"
go test ./internal/ledger/ -run TestLive_CachedTokensSurviveTheInclusiveConversion -count=1 -v
