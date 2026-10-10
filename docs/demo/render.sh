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
# Frame one is the poster frame GitHub shows when it does not autoplay, so the shell
# lead-in must go. Its length is not constant (typing and TUI launch time vary from run
# to run: a fixed 150 frames left the typed command as the poster on a clean re-render
# on 2026-10-10), so find the first frame where the drawn content is tall: the shell
# lead-in is two lines of text, the advise screen is about twenty.
lead=$(ffmpeg -hide_banner -i docs/demo/triage.gif \
  -vf "crop=iw:ih-36:0:36,cropdetect=limit=40:round=2:reset=1" -f null - 2>&1 |
  awk '/cropdetect/ { for (i = 1; i <= NF; i++) if ($i ~ /^crop=/) { split(substr($i, 6), a, ":"); if (a[2] > 200) { print NR - 1; exit } } }')
lead="${DEMO_LEADIN_FRAMES:-$lead}"
[ -n "$lead" ] || { echo "render.sh: no populated advise frame found in the recording" >&2; exit 1; }
echo "lead-in frames dropped: $lead"
ffmpeg -v error -y -i docs/demo/triage.gif \
  -filter_complex "[0:v]select=gte(n\,$lead),setpts=N/FRAME_RATE/TB,split[a][b];[a]palettegen=stats_mode=diff[p];[b][p]paletteuse=dither=none" \
  -loop 0 docs/demo/triage.trimmed.gif
mv docs/demo/triage.trimmed.gif docs/demo/triage.gif
