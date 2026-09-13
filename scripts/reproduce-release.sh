#!/bin/sh
# Rebuild a published release and compare it to the bytes on the releases page.
#
# RELEASE-CRITERIA.md's second 1.0 gate read "Reproducible is unverified.
# Nobody has rebuilt a published tag and compared the bytes, and a
# reproducibility claim nobody has tried to falsify is the class of claim this
# project refuses elsewhere." This is the falsification attempt, written so
# anybody can repeat it.
#
#   ./scripts/reproduce-release.sh v0.5.4 darwin arm64
#
# Exit 0 means the bytes match. Anything else means they do not, and the
# difference is printed rather than summarised.
set -eu

TAG="${1:?usage: reproduce-release.sh <tag> <goos> <goarch>}"
GOOS_IN="${2:?}"
GOARCH_IN="${3:?}"
VERSION="${TAG#v}"
REPO="RedRobotKK/Replay"
NAME="replay_${VERSION}_${GOOS_IN}_${GOARCH_IN}.tar.gz"

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

echo "Downloading $NAME and checksums.txt"
gh release download "$TAG" --repo "$REPO" -p "$NAME" -p checksums.txt -D "$work" >/dev/null
(cd "$work" && tar xzf "$NAME")
published=$(shasum -a 256 "$work/replay" | cut -d' ' -f1)

# THE TOOLCHAIN IS READ FROM THE BINARY, not assumed. A Go version that is one
# patch different produces different bytes, so guessing it is the difference
# between a reproduction and a disagreement nobody can explain.
#
# Each value is the part AFTER the "=", and that is not a detail. The first
# version of this script took $NF for the time and passed
# `Date=vcs.time=2026-09-09T00:49:33Z` into the ldflags. The rebuild then
# differed from the published bytes by exactly that string, both binaries were
# 8,364,258 bytes, and the script reported DOES NOT REPRODUCE about its own
# parsing. A reproducibility check that fails on its own bug is worse than none,
# because it retires a true claim.
meta=$(go version -m "$work/replay")
toolchain=$(printf '%s\n' "$meta" | head -1 | awk '{print $2}')
commit=$(printf '%s\n' "$meta" | tr '\t' ' ' | awk '/ vcs.revision=/{sub(/.*vcs.revision=/,""); print $1}')
vcstime=$(printf '%s\n' "$meta" | tr '\t' ' ' | awk '/ vcs.time=/{sub(/.*vcs.time=/,""); print $1}')
short=$(printf '%s' "$commit" | cut -c1-7)
[ -n "$toolchain" ] && [ -n "$commit" ] && [ -n "$vcstime" ] || {
  echo "could not read the build stamps out of the published binary" >&2; exit 2; }

echo "  published sha256: $published"
echo "  built by:         $toolchain"
echo "  commit:           $short"
echo "  commit time:      $vcstime"

# A CLONE, NOT A WORKTREE, and this is the part that took an hour to find.
#
# A linked git worktree stamps the main module as (devel) under go1.24 and the
# bytes differ, while the identical command in a real clone stamps the tag and
# reproduces exactly. The compiled code is the same either way: both binaries
# were 8,364,258 bytes and only the embedded build-info blob differed. So a
# reproduction attempt run in a worktree fails for a reason that has nothing to
# do with the code, which is exactly the sort of false negative that gets a
# real reproducibility claim abandoned.
src="$work/src"
echo "Cloning $REPO at $TAG into a fresh clone"
git clone -q --no-checkout "https://github.com/$REPO" "$src" 2>/dev/null ||
  git clone -q --no-checkout "$(git rev-parse --show-toplevel)" "$src"
(cd "$src" && git checkout -q "$TAG")

echo "Rebuilding with GOTOOLCHAIN=$toolchain"
( cd "$src" && GOTOOLCHAIN="$toolchain" CGO_ENABLED=0 GOOS="$GOOS_IN" GOARCH="$GOARCH_IN" \
  go build -trimpath \
    -ldflags "-s -w \
      -X github.com/RedRobotKK/Replay/internal/version.Version=$VERSION \
      -X github.com/RedRobotKK/Replay/internal/version.Commit=$short \
      -X github.com/RedRobotKK/Replay/internal/version.Date=$vcstime" \
    -o "$work/rebuilt" ./cmd/replay )

rebuilt=$(shasum -a 256 "$work/rebuilt" | cut -d' ' -f1)
echo "  rebuilt sha256:   $rebuilt"

if [ "$published" = "$rebuilt" ]; then
  echo "REPRODUCES: $TAG $GOOS_IN/$GOARCH_IN is byte-identical."
  exit 0
fi

echo "DOES NOT REPRODUCE. The difference, rather than a summary of it:"
echo "  published size: $(wc -c < "$work/replay")"
echo "  rebuilt size:   $(wc -c < "$work/rebuilt")"
echo "--- published build info ---"
go version -m "$work/replay" | sed 's/^/  /'
echo "--- rebuilt build info ---"
go version -m "$work/rebuilt" | sed 's/^/  /'
exit 1
