#!/bin/sh
# Replay installer.
#
#   curl -fsSL https://raw.githubusercontent.com/RedRobotKK/Replay/main/install.sh | sh
#
# Downloads the latest released binary for this platform, verifies it against the
# release checksums, and installs it. Falls back to `go install` when no release
# is published yet. Set REPLAY_BIN_DIR to choose where it lands.
set -eu

REPO="RedRobotKK/Replay"
BIN="replay"
BIN_DIR="${REPLAY_BIN_DIR:-}"

say() { printf '%s\n' "$*" >&2; }
die() { say "install: $*"; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || die "this installer needs $1"; }

need uname
if command -v curl >/dev/null 2>&1; then fetch() { curl -fsSL "$1"; }; dl() { curl -fsSL -o "$2" "$1"; }
elif command -v wget >/dev/null 2>&1; then fetch() { wget -qO- "$1"; }; dl() { wget -qO "$2" "$1"; }
else die "this installer needs curl or wget"; fi

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) die "unsupported architecture: $arch" ;;
esac
case "$os" in
  linux|darwin) ;;
  *) die "unsupported OS: $os. On Windows use the release archive or 'go install'." ;;
esac

if [ -z "$BIN_DIR" ]; then
  if [ -w /usr/local/bin ] 2>/dev/null; then BIN_DIR=/usr/local/bin
  else BIN_DIR="$HOME/.local/bin"; fi
fi
mkdir -p "$BIN_DIR"

tag=$(fetch "https://api.github.com/repos/$REPO/releases/latest" 2>/dev/null \
      | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1 || true)

if [ -z "${tag:-}" ]; then
  say "No published release yet."
  if command -v go >/dev/null 2>&1; then
    say "Building from source with go install."
    GOBIN="$BIN_DIR" go install "github.com/$REPO/cmd/$BIN@latest"
  else
    die "no release to download and Go is not installed. Install Go, or wait for the first tag."
  fi
else
  ver=${tag#v}
  archive="${BIN}_${ver}_${os}_${arch}.tar.gz"
  base="https://github.com/$REPO/releases/download/$tag"
  tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
  say "Downloading $BIN $tag for $os/$arch."
  dl "$base/$archive" "$tmp/$archive" || die "no build published for $os/$arch in $tag"

  if dl "$base/checksums.txt" "$tmp/checksums.txt" 2>/dev/null; then
    if command -v sha256sum >/dev/null 2>&1; then sum=$(sha256sum "$tmp/$archive" | cut -d' ' -f1)
    elif command -v shasum >/dev/null 2>&1; then sum=$(shasum -a 256 "$tmp/$archive" | cut -d' ' -f1)
    else sum=""; say "No sha256 tool found, skipping checksum verification."; fi
    if [ -n "$sum" ]; then
      grep -q "$sum" "$tmp/checksums.txt" || die "checksum mismatch for $archive. Not installing."
      say "Checksum verified."
    fi
  else
    say "Could not fetch checksums.txt, skipping verification."
  fi

  tar -xzf "$tmp/$archive" -C "$tmp"
  install -m 0755 "$tmp/$BIN" "$BIN_DIR/$BIN" 2>/dev/null || { cp "$tmp/$BIN" "$BIN_DIR/$BIN"; chmod 0755 "$BIN_DIR/$BIN"; }
fi

say ""
say "Installed to $BIN_DIR/$BIN"
case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) say "Add it to your PATH:  export PATH=\"$BIN_DIR:\$PATH\"" ;;
esac
say "Next:  $BIN doctor"
