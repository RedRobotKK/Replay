#!/usr/bin/env bash
# Build the harness image and run the assertions. What CI invokes.
#
# Skips rather than fails when Docker is unavailable, so a developer without it
# is told why instead of being handed a broken build. CI sets X402_E2E_REQUIRED
# so the same absence is a failure there: a check that silently skips on the
# machine that matters is a check that cannot fail.
set -euo pipefail
cd "$(dirname "$0")/../.."

if ! docker info >/dev/null 2>&1; then
  if [ -n "${X402_E2E_REQUIRED:-}" ]; then
    echo "x402-e2e: docker is required here and is not available" >&2
    exit 1
  fi
  echo "x402-e2e: skipped, no docker daemon (set X402_E2E_REQUIRED to make this fatal)"
  exit 0
fi

docker build -q -f scripts/x402-e2e/Dockerfile -t replay-x402-e2e . >/dev/null
docker run --rm replay-x402-e2e
