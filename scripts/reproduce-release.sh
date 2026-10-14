#!/bin/sh
# Rebuild a published release and compare it to the bytes on the releases page.
#
# RELEASE-CRITERIA.md's second 1.0 gate read "Reproducible is unverified.
# Nobody has rebuilt a published tag and compared the bytes, and a
# reproducibility claim nobody has tried to falsify is the class of claim this
# project refuses elsewhere." This is the falsification attempt, written so
# anybody can repeat it. release.yml's `reproduce` job runs it for every
# platform of every tag.
#
#   ./scripts/reproduce-release.sh v0.5.4 darwin arm64
#
# Exit 0: the bytes match. Exit 1: they do not, and the difference is printed
# rather than summarised. Exit 2: nothing was compared, because an input was
# missing, ambiguous or disagreed with the tag. 2 is never a pass.
#
# THE TAG IS THE CLAIM AND THE ARTIFACT IS WHAT IS CHECKED. Until 2026-10-13
# this read the toolchain and the commit out of the published binary and
# rebuilt from those, so a binary built from the wrong commit was rebuilt from
# the same wrong commit and "reproduced": the question answered was whether
# the artifact reproduces itself. Every build input now comes from the tag in
# a fresh clone, and the artifact's stamps must agree with it before anything
# is built. It also accepted an archive whatever checksums.txt said about it.
#
# TRACING OFF BEFORE ANYTHING ELSE. bash as sh enables the options a caller's
# SHELLOPTS lists before this line runs, and with xtrace it expands the
# caller's PS4 before every command: PS4='$((clean=1,path_seen=1))' rewrote the
# allow-list verdict below, and the trace printed GH_TOKEN (R1, 2026-10-14).
# set is a special builtin, which no function replaces when bash runs as sh.
# PS4 is still expanded once, for this line, which is why a SHELLOPTS or
# BASHOPTS in the environment is refused below rather than re-run: whatever
# that one expansion did, the run does not pass.
set +vx
set -eu

# THE WHOLE SCRIPT RUNS UNDER AN ALLOW-LIST, not only go. On 2026-10-13 a
# caller's GIT_CONFIG_COUNT/KEY/VALUE insteadOf rule sent the clone to a decoy
# repository whose tag rebuilt to the published bytes, and the gate printed
# REPRODUCES about a binary the upstream tag never produced; GREP_OPTIONS=-v
# inverted every grep test here; GIT_DIR, GH_HOST, TAR_OPTIONS and the locale
# can each change what is fetched, read or matched. So if the environment holds
# anything not named below, the script runs itself again under exactly these:
#   PATH      finds sh, git, gh, tar, go. It is the one input not narrowed: a
#             caller who controls PATH controls every tool, and that is stated
#             rather than pretended away. In CI the runner sets it.
#   HOME      gh's stored login when run by hand; go and git are pinned apart
#             from it below and in pinned_go.
#   GH_TOKEN  the read-only token release.yml passes for `gh release download`.
#   LC_ALL=C  byte-wise matching for grep and awk, whatever the caller's locale.
# The check is a shell `case`, so no tool's own environment options can answer
# it. REPRO_SANITIZED only stops a loop: if it is set and the environment is
# still not clean, the script refuses rather than trusting it.
#
# UNTIL THE RE-RUN, ONLY RESERVED WORDS, SPECIAL BUILTINS AND PATHS. Where sh
# is bash (macOS /bin/sh is bash 3.2), bash imports functions the caller
# exported, and a function shadows any command or regular builtin of its
# name: an exported `env` answered this check with a clean listing, and
# `read` or `[` could end it early, so the re-run was skipped and a hostile
# GIT_CONFIG_* reached gh and git (F2, 2026-10-14). bash cannot import a
# function whose name holds a slash, so env is /usr/bin/env, the one path
# POSIX systems keep for it (macOS, Debian and Ubuntu, Alpine, NixOS), and it
# also runs printf, which NixOS keeps nowhere else. Where /usr/bin/env is
# missing the command substitution lists nothing, PATH is never seen, and the
# exec below fails, so nothing runs. for and case are reserved words, and set,
# unset, exit and exec are special builtins, which a function does not replace
# when bash runs as sh. The re-run starts under env -i, with no exported
# functions at all. dash imports none, but runs this same path.
#
# Refused outright, not re-run: SHELLOPTS and BASHOPTS (see set +vx above).
# Two cases no line here can reach: SHELLOPTS=noexec and SHELLOPTS=onecmd make
# bash as sh exit 0 having run nothing; onecmd reads one line, the #! line
# (measured 2026-10-14; dash ignores both).
# A pass is exit 0 AND the REPRODUCES line; exit 0 alone is not one.
#
# Only when sh runs it. Under `bash reproduce-release.sh`, outside posix mode,
# a function can replace even set and exit, and nothing in a script can
# defend itself from that. release.yml and the documentation run it as sh.
set -f
IFS='
'
clean=1
path_seen=0
opts=0
for line in $(/usr/bin/env); do
  case $line in
    PATH=*) path_seen=1 ;;
    SHELLOPTS=* | BASHOPTS=*) opts=1 clean=0 ;;
    HOME=* | GH_TOKEN=* | LC_ALL=C | PWD=* | OLDPWD=* | SHLVL=* | _=* | REPRO_SANITIZED=1) ;;
    *) clean=0 ;;
  esac
done
set +f
# unset is a special builtin, and an unset IFS splits as the default does.
unset IFS
case $opts in
  1)
    /usr/bin/env printf '%s\n' "NOT COMPARED: SHELLOPTS or BASHOPTS is set, and sh applies it before this script can check anything; unset it" >&2
    exit 2
    ;;
esac
case $clean$path_seen in
  11) ;;
  *)
    case ${REPRO_SANITIZED:-} in
      1)
        /usr/bin/env printf '%s\n' "NOT COMPARED: the environment is still not the allow-list after re-running under it" >&2
        exit 2
        ;;
    esac
    exec /usr/bin/env -i PATH="$PATH" HOME="${HOME:-/nonexistent}" ${GH_TOKEN:+"GH_TOKEN=$GH_TOKEN"} \
      LC_ALL=C REPRO_SANITIZED=1 sh "$0" "$@"
    ;;
esac
# git reads no system or user configuration: no insteadOf, no hooks template,
# no credential helper from outside this script.
GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null GIT_TERMINAL_PROMPT=0
export GIT_CONFIG_NOSYSTEM GIT_CONFIG_GLOBAL GIT_TERMINAL_PROMPT

TAG="${1:?usage: reproduce-release.sh <tag> <goos> <goarch>}"
GOOS_IN="${2:?}"
GOARCH_IN="${3:?}"
VERSION="${TAG#v}"
REPO="RedRobotKK/Replay"
NAME="replay_${VERSION}_${GOOS_IN}_${GOARCH_IN}.tar.gz"

refuse() {
  echo "NOT COMPARED: $*" >&2
  exit 2
}

# Arguments are names, never options or paths. A tag reaches git and gh, and a
# platform reaches a file name, so anything else is refused before either runs.
printf '%s\n' "$TAG" | grep -Eqx 'v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?' ||
  refuse "$TAG is not a release tag of the form vX.Y.Z"
printf '%s\n' "$GOOS_IN" | grep -Eqx '[a-z0-9]+' || refuse "$GOOS_IN is not a GOOS"
printf '%s\n' "$GOARCH_IN" | grep -Eqx '[a-z0-9]+' || refuse "$GOARCH_IN is not a GOARCH"

work=$(mktemp -d)
# The module cache under $work is written read-only by go; make it removable.
trap 'chmod -R u+w "$work" 2>/dev/null; rm -rf "$work"' EXIT

# THE GO ENVIRONMENT IS STATED, NOT INHERITED, for every go command here.
# GOFLAGS and GOAMD64 each change the bytes of an identical source (measured
# 2026-10-13), and so can a go env file, a go.work above the clone, or
# GOEXPERIMENT; each is set to what goreleaser builds with. The checksum
# database is enforced and its bypasses cleared, because GOTOOLCHAIN can
# download the declared toolchain. The build cache is new for this run: a
# cache entry is trusted by its key, and a poisoned one could answer for the
# rebuild.
#
# AN ALLOW-LIST, NOT A DENY-LIST. The first version set eleven variables and
# inherited the rest, and GOCACHEPROG, GO111MODULE, GOROOT and GOTMPDIR each
# still reached the build (2026-10-13): an external cache program can answer
# for compiled packages whatever GOCACHE says. env -i passes PATH, to find go
# and the git that stamps the build, and HOME; everything else is stated here,
# so a variable Go adds later is dropped too. GOPROXY is left at Go's default.
pinned_go() {
  /usr/bin/env -i PATH="$PATH" HOME="${HOME:-$work}" LC_ALL=C \
    GIT_CONFIG_NOSYSTEM=1 GIT_CONFIG_GLOBAL=/dev/null GIT_TERMINAL_PROMPT=0 \
    GOENV=off GOFLAGS= GOWORK=off GOEXPERIMENT= GOAMD64=v1 GOARM64=v8.0 GO111MODULE=on \
    GOSUMDB=sum.golang.org GONOSUMDB= GONOSUMCHECK= GOPRIVATE= GONOPROXY= GOINSECURE= \
    GOCACHE="$work/gocache" GOPATH="$work/gopath" "$@"
}

echo "Downloading $NAME and checksums.txt"
gh release download "$TAG" --repo "github.com/$REPO" -p "$NAME" -p checksums.txt -D "$work" >/dev/null ||
  refuse "could not download $NAME and checksums.txt for $TAG"
[ -s "$work/$NAME" ] || refuse "$NAME is missing or empty"
[ -s "$work/checksums.txt" ] || refuse "checksums.txt is missing or empty"

# The archive is bound to checksums.txt before anything is read out of it.
# checksums.txt is the file release.yml signs; an archive it does not vouch
# for is not the release, whatever it rebuilds to.
listed=$(awk -v n="$NAME" '$2 == n { print $1 }' "$work/checksums.txt")
[ -n "$listed" ] || refuse "checksums.txt has no entry for $NAME"
[ "$(printf '%s\n' "$listed" | wc -l | tr -d ' ')" = 1 ] || refuse "checksums.txt lists $NAME more than once"
archive_sum=$(shasum -a 256 "$work/$NAME" | cut -d' ' -f1)
[ "$archive_sum" = "$listed" ] ||
  refuse "$NAME hashes to $archive_sum but checksums.txt says $listed"

# THE MEMBER COMPARED IS THE MEMBER A USER GETS. `replay upgrade` installs the
# first regular member whose base name is replay, and install.sh extracts the
# member named exactly replay, the later of two. An archive that held a second
# one, under another directory or the same name, passed this check while
# upgrade installed the other (found 2026-10-13). So the archive must hold
# exactly one member whose base name is replay, named exactly replay, and
# then the two readers cannot disagree.
members=$(tar tzf "$work/$NAME") || refuse "$NAME is not a readable tar.gz"
# Counted by Go's rule, filepath.Base, which drops trailing slashes: a
# regular file named "replay/" is a replay to `replay upgrade` and was not one
# to `awk -F/ '$NF == "replay"'`, so a planted "replay/" ahead of the genuine
# member passed here while upgrade installed it (found 2026-10-13).
count=$(printf '%s\n' "$members" | awk '{ sub(/\/+$/, ""); n = split($0, p, "/"); if (p[n] == "replay") c++ } END { print c + 0 }')
[ "$count" = 1 ] && printf '%s\n' "$members" | grep -qx replay ||
  refuse "$NAME must hold exactly one member whose base name is replay, named exactly replay; it holds $count"

# THE BYTES COMPARED ARE THE BYTES `replay upgrade` INSTALLS. The member is
# selected by selfupdate.Unpack, the code upgrade runs, through
# scripts/archive-member from this script's own tree. Counting names with tar,
# above, was not enough: on 2026-10-13 a planted member whose PAX header said
# path=zzz and whose GNU long name said replay was "zzz" to bsdtar and "replay"
# to Go, so the gate compared the genuine member while upgrade installed the
# plant. tar must also extract byte-identical content, because install.sh uses
# tar; the two readers agreeing is the condition, not either one alone.
#
# Neither extracted file is executed: `go version -m` reads build info as data.
root=$(cd "$(dirname "$0")/.." && pwd)
(cd "$root" && pinned_go GOTOOLCHAIN=local go run ./scripts/archive-member "$work/$NAME" "$work/replay.upgrade" "$work/members.go") ||
  refuse "$NAME is not in the plain form goreleaser writes, or replay upgrade would not install a binary from it"
# tar and Go must list the same members in the same order: the plain form
# leaves nothing for two readers to resolve differently, and this checks that
# they did not.
[ "$members" = "$(cat "$work/members.go")" ] ||
  refuse "tar and Go list the members of $NAME differently"
(cd "$work" && tar xzf "$NAME" replay) || refuse "$NAME has no replay member"
[ -f "$work/replay" ] && [ ! -L "$work/replay" ] || refuse "replay in $NAME is not a regular file"
cmp -s "$work/replay" "$work/replay.upgrade" ||
  refuse "tar and replay upgrade select different bytes for replay in $NAME"
published=$(shasum -a 256 "$work/replay.upgrade" | cut -d' ' -f1)

# A CLONE, NOT A WORKTREE, and this is the part that took an hour to find.
#
# A linked git worktree stamps the main module as (devel) under go1.24 and the
# bytes differ, while the identical command in a real clone stamps the tag and
# reproduces exactly. The compiled code is the same either way: both binaries
# were 8,364,258 bytes and only the embedded build-info blob differed. So a
# reproduction attempt run in a worktree fails for a reason that has nothing to
# do with the code, which is exactly the sort of false negative that gets a
# real reproducibility claim abandoned.
#
# There is no fallback to a local repository. It used to clone the checkout
# the script ran from when GitHub failed, which is a different source from the
# one the tag names.
src="$work/src"
echo "Cloning $REPO at $TAG into a fresh clone"
git clone -q --no-checkout "https://github.com/$REPO" "$src" || refuse "could not clone $REPO"
# By its full tag ref, peeled to the commit, and checked out detached: the name
# alone would also match a branch, and the tree built is the commit compared.
commit=$(git -C "$src" rev-parse --verify -q "refs/tags/$TAG^{commit}") || refuse "$TAG is not a tag in $REPO"
git -C "$src" checkout -q --detach "$commit" || refuse "could not check out $commit"
# goreleaser's ShortCommit is `git rev-parse --short`, whose length grows with
# the repository. Cutting to seven characters would one day disagree with it.
short=$(git -C "$src" rev-parse --short "$commit")
# goreleaser's CommitDate: the committer date, in UTC.
vcstime=$(cd "$src" && TZ=UTC git log -1 --format=%cd --date=format-local:%Y-%m-%dT%H:%M:%SZ "$commit")

# THE TOOLCHAIN IS READ FROM go.mod AT THE TAG. A Go version one patch
# different produces different bytes, so it has to be named exactly. release.yml
# builds with the go.mod toolchain, and a tag with no toolchain line does not
# say which one that was.
toolchain=$(awk '$1 == "toolchain" { print $2 }' "$src/go.mod")
[ -n "$toolchain" ] || refuse "go.mod at $TAG has no toolchain line, so the toolchain that built it is ambiguous"
# Releases before v0.6.0 have no toolchain line and are refused here. v0.5.4
# was reproduced on 2026-09-13 by this script's earlier version, which took the
# toolchain from the binary; that result stands, and is not repeatable with
# this one.
[ "$(printf '%s\n' "$toolchain" | wc -l | tr -d ' ')" = 1 ] &&
  printf '%s\n' "$toolchain" | grep -Eqx 'go[0-9]+\.[0-9]+(\.[0-9]+|rc[0-9]+)?' ||
  refuse "go.mod at $TAG does not name exactly one toolchain of the form goN.N.N"

# What the artifact says about itself, which must agree with the tag.
#
# Each value is the part AFTER the "=", and that is not a detail. The first
# version of this script took $NF for the time and passed
# `Date=vcs.time=2026-09-09T00:49:33Z` into the ldflags. The rebuild then
# differed from the published bytes by exactly that string, both binaries were
# 8,364,258 bytes, and the script reported DOES NOT REPRODUCE about its own
# parsing. A reproducibility check that fails on its own bug is worse than none,
# because it retires a true claim.
meta=$(pinned_go GOTOOLCHAIN=local go version -m "$work/replay.upgrade") || refuse "replay in $NAME has no Go build info"
stamp() { printf '%s\n' "$meta" | tr '\t' ' ' | awk -v k=" $1=" 'index($0, k) { sub(".*" k, ""); print $1 }'; }
built_by=$(printf '%s\n' "$meta" | head -1 | awk '{print $2}')
built_from=$(stamp vcs.revision)
built_time=$(stamp vcs.time)
built_dirty=$(stamp vcs.modified)

echo "  tag:              $TAG"
echo "  commit:           $commit ($short)"
echo "  commit time:      $vcstime"
echo "  toolchain:        $toolchain (go.mod at the tag)"
echo "  archive sha256:   $archive_sum (matches checksums.txt)"
echo "  published sha256: $published"

[ "$built_from" = "$commit" ] ||
  refuse "the published binary was built from ${built_from:-an unstamped tree}, but $TAG is $commit"
[ "$built_dirty" = false ] ||
  refuse "the published binary was built from a modified tree (vcs.modified=${built_dirty:-absent})"
[ "$built_by" = "$toolchain" ] ||
  refuse "the published binary was built by ${built_by:-an unknown toolchain}, but go.mod at $TAG names toolchain $toolchain"
[ "$built_time" = "$vcstime" ] ||
  refuse "the published binary carries commit time ${built_time:-none}, but $TAG's commit time is $vcstime"

echo "Rebuilding with GOTOOLCHAIN=$toolchain"
( cd "$src" && pinned_go GOTOOLCHAIN="$toolchain" CGO_ENABLED=0 GOOS="$GOOS_IN" GOARCH="$GOARCH_IN" \
  go build -trimpath \
    -ldflags "-s -w \
      -X github.com/RedRobotKK/Replay/internal/version.Version=$VERSION \
      -X github.com/RedRobotKK/Replay/internal/version.Commit=$short \
      -X github.com/RedRobotKK/Replay/internal/version.Date=$vcstime" \
    -o "$work/rebuilt" ./cmd/replay ) || refuse "the rebuild of $TAG failed, so nothing was compared"
[ -s "$work/rebuilt" ] || refuse "the rebuild produced no binary"

rebuilt=$(shasum -a 256 "$work/rebuilt" | cut -d' ' -f1)
echo "  rebuilt sha256:   $rebuilt"

if [ "${#published}" = 64 ] && [ "$published" = "$rebuilt" ]; then
  echo "REPRODUCES: $TAG $GOOS_IN/$GOARCH_IN is byte-identical."
  exit 0
fi

echo "DOES NOT REPRODUCE. The difference, rather than a summary of it:"
echo "  published size: $(wc -c < "$work/replay.upgrade")"
echo "  rebuilt size:   $(wc -c < "$work/rebuilt")"
echo "--- published build info ---"
pinned_go GOTOOLCHAIN=local go version -m "$work/replay.upgrade" | sed 's/^/  /'
echo "--- rebuilt build info ---"
pinned_go GOTOOLCHAIN=local go version -m "$work/rebuilt" | sed 's/^/  /'
exit 1
