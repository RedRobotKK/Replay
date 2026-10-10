#!/usr/bin/env bash
# Renders docs/demo/triage.gif from the redacted fixture, not from a private corpus.
#   docs/demo/render.sh [path-to-replay-binary]
# With no argument it builds the binary from this tree. Pass the binary extracted
# from a release archive to record a released build.
set -euo pipefail
cd "$(dirname "$0")/../.."
export PATH="/opt/homebrew/bin:$PATH"
bin=docs/demo/.replay-demo-bin
if [ $# -ge 1 ]; then cp "$1" "$bin"; else go build -o "$bin" ./cmd/replay; fi
rm -rf docs/demo/.home
mkdir -p docs/demo/.home/.claude/projects/fixture-redacted
cp internal/transcript/testdata/session-redacted.jsonl docs/demo/.home/.claude/projects/fixture-redacted/
echo "fixture sha256: $(shasum -a 256 internal/transcript/testdata/session-redacted.jsonl | cut -d' ' -f1)"
echo "binary sha256:  $(shasum -a 256 "$bin" | cut -d' ' -f1)"
echo "binary version: $(HOME=$PWD/docs/demo/.home "$bin" version)"
HOME=$PWD/docs/demo/.home "$bin" advise docs/demo/.home/.claude/projects >/dev/null
vhs docs/demo/triage.tape
# Frame one is the poster frame GitHub shows when it does not autoplay; drop the shell lead-in.
lead="${DEMO_LEADIN_FRAMES:-150}"
ffmpeg -v error -y -i docs/demo/triage.gif \
  -filter_complex "[0:v]select=gte(n\,$lead),setpts=N/FRAME_RATE/TB,split[a][b];[a]palettegen=stats_mode=diff[p];[b][p]paletteuse=dither=none" \
  -loop 0 docs/demo/triage.trimmed.gif
mv docs/demo/triage.trimmed.gif docs/demo/triage.gif
