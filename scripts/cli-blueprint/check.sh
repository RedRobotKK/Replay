#!/usr/bin/env bash
# Fails if docs/CLI.md no longer matches the binary.
#
# The reference is generated, so the only way it goes wrong is by not being
# regenerated. This is the thing that notices.
set -euo pipefail
cd "$(dirname "$0")/../.."

bin=$(mktemp -d)/replay
go build -o "$bin" ./cmd/replay
python3 scripts/cli-blueprint/gen.py --bin "$bin" > "$bin.md"

if diff -u docs/CLI.md "$bin.md"; then
  echo "cli-blueprint: docs/CLI.md matches the binary."
else
  echo >&2
  echo "cli-blueprint: docs/CLI.md is out of date with the binary." >&2
  echo "Regenerate it in the same commit as the flag change:" >&2
  echo "  go build -o /tmp/replay ./cmd/replay && python3 scripts/cli-blueprint/gen.py --bin /tmp/replay > docs/CLI.md" >&2
  exit 1
fi
