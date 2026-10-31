#!/usr/bin/env bash
# Runs the proxy measurement harness and keeps everything a reader needs to
# trust or discount its figures: the machine's load and busiest processes
# before and after, the exact command, and its exit status.
#
#   scripts/replay-bench/run.sh                          # full run, 10 repeats
#   scripts/replay-bench/run.sh -noisy-label "reason"    # label a known-noisy run
#   OUT=docs/evidence/data scripts/replay-bench/run.sh   # choose the results dir
#
# Arguments are passed to the Go harness unchanged. Results go to $OUT
# (default bench-results/) under timestamped names that are never reused.
#
# Go cannot read another process's CPU use without os/exec, which this module
# confines to the mutation harness (cmd/replay/x402_test.go X6c), so the ps
# and uptime capture lives here, in the shell, and is handed to the harness as
# a sidecar file. Home-directory paths are stripped before anything is kept.
set -euo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$root"
out="${OUT:-bench-results}"
mkdir -p "$out"
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# TOP_PROCESSES is how many of the busiest processes are recorded.
TOP_PROCESSES=10

top_cpu() {
  # pcpu and the command's basename only: no arguments, no paths. awk reads
  # all of ps's output rather than `head` closing the pipe early, which under
  # pipefail turned ps's SIGPIPE into this script's exit status (141).
  if [ "$(uname -s)" = Darwin ]; then
    ps -Ao pcpu=,comm= -r
  else
    ps -eo pcpu=,comm= --sort=-pcpu
  fi | awk -v top="$TOP_PROCESSES" 'NR <= top {cpu=$1; $1=""; sub(/^ /,""); n=split($0,a,"/"); printf "%6s%%  %s\n", cpu, a[n]}'
}

total_cpu() {
  ps -Ao pcpu= 2>/dev/null | awk '{s+=$1} END {printf "%.1f%%\n", s}'
}

snapshot() { # label
  echo "== $1 $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "uptime: $(uptime | sed "s|$HOME|~|g")"
  echo "sum of %CPU over all processes (100% = one core): $(total_cpu)"
  echo "top $TOP_PROCESSES by %CPU:"
  top_cpu | sed "s|$HOME|~|g"
}

sha="$(git rev-parse HEAD 2>/dev/null || echo unknown)"
if ! git diff --quiet HEAD -- 2>/dev/null; then
  sha="$sha-dirty"
fi

# Build first, so compiling is not part of the measured run's load.
bin="$tmp/replay-bench"
go build -o "$bin" ./scripts/replay-bench

snapshot before >"$tmp/sidecar.txt"
cmd=("$bin" -out "$out" -git-sha "$sha" -sidecar "$tmp/sidecar.txt" "$@")
status=0
"${cmd[@]}" | tee "$tmp/stdout.txt" || status=$?

log="$out/replay-bench-run-$stamp.txt"
{
  echo "command: go build -o replay-bench ./scripts/replay-bench && replay-bench -out $out -git-sha $sha -sidecar <before> $*"
  echo "exit status: $status"
  echo "go: $(go version)"
  echo
  cat "$tmp/sidecar.txt"
  echo
  snapshot after
  echo
  echo "== stdout"
  cat "$tmp/stdout.txt"
} >"$log"
echo "run log: $log (exit status $status)"
exit "$status"
