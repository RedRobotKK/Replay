#!/usr/bin/env bash
# Renders every TUI screen and checks it for layout artifacts.
#
# The screens are the product's whole argument, and until now nothing checked
# them mechanically: the committed SVGs pin what one width looks like, not
# whether the layout survives a different terminal or a different locale.
set -euo pipefail
cd "$(dirname "$0")/../.."
bin=$(mktemp -d)/replay
go build -o "$bin" ./cmd/replay
REPLAY_BIN="$bin" python3 scripts/tui-audit/audit.py
