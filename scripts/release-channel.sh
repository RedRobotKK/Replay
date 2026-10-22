#!/bin/sh
# release-channel.sh TAG
#
# Prints `stable` or `prerelease` for a release tag, or refuses it: a message
# on stderr, nothing on stdout, exit 2.
#
# The grammar is the one every release has, and the one selfupdate.ValidTag
# and install.sh apply: ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?$. A tag with
# a -suffix is a prerelease; goreleaser marks the same tags as GitHub
# prereleases (`prerelease: auto` in .goreleaser.yaml).
#
# Every job that publishes to a package registry runs this before its first
# publishing command and publishes only on `stable`. A prerelease never reaches
# npm or PyPI. That is the policy, not an accident of this script: npm publish
# without --tag puts a version under the dist-tag `latest`, so v1.0.0-rc.1
# would become what `npx replay-doctor` installs for everybody.
#
# internal/regression/release_channel_test.go runs this under sh, dash and
# bash --posix, checks it against selfupdate.ValidTag, and checks that the
# workflows call it before they publish.

set -u

refuse() {
	printf 'release-channel: %s\n' "$1" >&2
	exit 2
}

[ "$#" -eq 1 ] || refuse "usage: release-channel.sh TAG (got $# arguments)"
tag=$1

# grep reads lines, so a tag holding a newline would be judged one line at a
# time and pass on its first. Refuse any control character before grep sees it.
case $tag in
*[[:cntrl:]]*) refuse "tag holds a control character" ;;
esac

printf '%s\n' "$tag" | grep -Eqx 'v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.]+)?' ||
	refuse "tag '$tag' is not vMAJOR.MINOR.PATCH with an optional -prerelease"

case $tag in
*-*) echo prerelease ;;
*) echo stable ;;
esac
