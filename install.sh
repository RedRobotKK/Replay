#!/bin/sh
# Replay installer.
#
#   curl -fsSL https://raw.githubusercontent.com/RedRobotKK/Replay/main/install.sh | sh
#
# Replay sits in the path between your agent and your model provider, so this
# script is written to be read before it is run. It is short on purpose. It
# downloads a released binary, verifies it against the release checksums, and
# refuses to install anything that does not match.
#
#   --version <tag>     install a specific release instead of the latest
#   --bin-dir <dir>     where the binary lands
#   --dry-run           print what would happen, change nothing
#   --no-modify-path    never mention or touch shell configuration
#   --no-verify         install even if the checksum cannot be verified. Off by
#                       default: an unverifiable download aborts.
#   --corpus-opt-in     agree now to share calibration reports. Off unless you
#                       pass it. It writes a file; it sends nothing, ever. You
#                       still run `replay corpus --submit` to send anything.
#   --help              this text
#
# Environment: REPLAY_VERSION, REPLAY_BIN_DIR, NO_COLOR.

set -eu

REPO="RedRobotKK/Replay"
BIN="replay"
VERSION="${REPLAY_VERSION:-}"
BIN_DIR="${REPLAY_BIN_DIR:-}"
DRY_RUN=0
MODIFY_PATH=1
CORPUS_OPT_IN=0
ALLOW_UNVERIFIED=0

# ---------------------------------------------------------------- presentation
# Colour only when stdout is a terminal that wants it. Piped into a file or a
# log, this prints clean text with no escape codes.
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ] && [ "${TERM:-dumb}" != "dumb" ]; then
  C_ACCENT=$(printf '\033[38;5;44m'); C_OK=$(printf '\033[38;5;71m')
  C_WARN=$(printf '\033[38;5;179m'); C_ERR=$(printf '\033[38;5;167m')
  C_DIM=$(printf '\033[2m');         C_B=$(printf '\033[1m'); C_0=$(printf '\033[0m')
else
  C_ACCENT=; C_OK=; C_WARN=; C_ERR=; C_DIM=; C_B=; C_0=
fi

step()  { printf '%s→%s %s\n'  "$C_ACCENT" "$C_0" "$*" >&2; }
ok()    { printf '%s✓%s %s\n'  "$C_OK"     "$C_0" "$*" >&2; }
info()  { printf '  %s%s%s\n'  "$C_DIM"    "$*"   "$C_0" >&2; }
warn()  { printf '%s!%s %s\n'  "$C_WARN"   "$C_0" "$*" >&2; }
die()   { printf '%s✗%s %s\n'  "$C_ERR"    "$C_0" "$*" >&2; exit 1; }
run()   { if [ "$DRY_RUN" -eq 1 ]; then info "would run: $*"; else "$@"; fi; }

usage() {
  # Not a self-read: under `curl | sh` there is no script file to read from.
  cat <<'USAGE'
Replay installer.

  curl -fsSL https://redrobot.jp/Replay/install.sh | sh

  --version <tag>     install a specific release instead of the latest
  --bin-dir <dir>     where the binary lands
  --dry-run           print what would happen, change nothing
  --no-modify-path    never mention or touch shell configuration
  --no-verify         install even if the download cannot be verified
  --corpus-opt-in     agree now to share calibration reports. Sends nothing.
  --help              this text

Environment: REPLAY_VERSION, REPLAY_BIN_DIR, NO_COLOR.
Source: https://github.com/RedRobotKK/Replay
USAGE
  exit 0
}

while [ $# -gt 0 ]; do
  case "$1" in
    --version)        VERSION="${2:-}"; shift 2 ;;
    --version=*)      VERSION="${1#*=}"; shift ;;
    --bin-dir)        BIN_DIR="${2:-}"; shift 2 ;;
    --bin-dir=*)      BIN_DIR="${1#*=}"; shift ;;
    --dry-run)        DRY_RUN=1; shift ;;
    --no-modify-path) MODIFY_PATH=0; shift ;;
    --corpus-opt-in)  CORPUS_OPT_IN=1; shift ;;
    --no-verify)      ALLOW_UNVERIFIED=1; shift ;;
    -h|--help)        usage ;;
    *)                die "unknown option: $1. Try --help." ;;
  esac
done

# ------------------------------------------------------------------- downloader
if command -v curl >/dev/null 2>&1; then
  fetch() { curl -fsSL --proto '=https' --proto-redir '=https' --tlsv1.2 "$1"; }
  save()  { curl -fsSL --proto '=https' --proto-redir '=https' --tlsv1.2 -o "$2" "$1"; }
elif command -v wget >/dev/null 2>&1; then
  fetch() { wget -q --https-only --secure-protocol=TLSv1_2 -O- "$1"; }
  save()  { wget -q --https-only --secure-protocol=TLSv1_2 -O "$2" "$1"; }
else
  die "curl or wget is required."
fi

# ------------------------------------------------------------------- platform
os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$arch" in
  x86_64|amd64)  arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) die "unsupported architecture: $arch. Open an issue and say what you are on." ;;
esac
case "$os" in
  linux)  ;;
  darwin) ;;
  msys*|mingw*|cygwin*)
    die "Windows is not handled by this script. Use the release archive, or: go install github.com/$REPO/cmd/$BIN@latest" ;;
  *) die "unsupported OS: $os." ;;
esac

# musl and glibc are not interchangeable, and a binary built for one fails on the
# other with an error that does not name the cause. uv and rustup both check this.
libc=""
if [ "$os" = linux ]; then
  if command -v ldd >/dev/null 2>&1 && ldd --version 2>&1 | grep -qi musl; then
    libc=musl
  else
    libc=gnu
  fi
fi

# A short decode of the wordmark: glyphs settle left to right into REPLAY.
# Terminal only. A pipe, a log, CI and NO_COLOR all get plain text and no
# delay, because an installer people are told to read should not also be a
# thing that misbehaves when it is not being watched.
banner() {
  { [ -t 1 ] && [ -z "${NO_COLOR:-}" ] && [ "${TERM:-dumb}" != dumb ]; } || return 0
  w=REPLAY; g='#%@&$*+=-<>/|01'; i=0
  while [ "$i" -le 6 ]; do
    out=$(printf '%.*s' "$i" "$w"); j=$i
    while [ "$j" -lt 6 ]; do
      k=$(( (i * 7 + j * 11 + 3) % 15 + 1 ))
      out="$out$(printf %s "$g" | cut -c"$k")"
      j=$((j + 1))
    done
    printf '\r  %s%s%s' "$C_ACCENT$C_B" "$out" "$C_0"
    sleep 0.04 2>/dev/null || true
    i=$((i + 1))
  done
  printf '   %sprompt cache, measured%s\n\n' "$C_DIM" "$C_0"
}

banner
step "Replay installer"
info "platform  ${os}/${arch}${libc:+ (${libc})}"

# ------------------------------------------------------------------ where to
if [ -z "$BIN_DIR" ]; then
  if [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then BIN_DIR=/usr/local/bin
  else BIN_DIR="$HOME/.local/bin"; fi
fi
info "install   ${BIN_DIR}/${BIN}"

# ------------------------------------------------------- what is already here
previous=""
if command -v "$BIN" >/dev/null 2>&1; then
  previous=$("$BIN" version 2>/dev/null | head -1 || echo "unknown version")
  info "existing  ${previous} at $(command -v "$BIN")"
fi

# ------------------------------------------------------------------- version
if [ -z "$VERSION" ]; then
  step "Resolving the latest release"
  VERSION=$(fetch "https://api.github.com/repos/$REPO/releases/latest" 2>/dev/null \
            | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1 || true)
fi

# ----------------------------------------------------- no release yet: source
if [ -z "${VERSION:-}" ]; then
  warn "No release is published yet."
  if command -v go >/dev/null 2>&1; then
    info "Building from source instead. This takes a minute."
    run mkdir -p "$BIN_DIR"
    if [ "$DRY_RUN" -eq 0 ]; then
      GOBIN="$BIN_DIR" go install "github.com/$REPO/cmd/$BIN@latest" \
        || die "go install failed. The error above is from Go, not from this script."
      ok "Built and installed to ${BIN_DIR}/${BIN}"
    else
      info "would run: GOBIN=$BIN_DIR go install github.com/$REPO/cmd/$BIN@latest"
    fi
  else
    die "There is no release to download and Go is not installed.
   Install Go from https://go.dev/dl/ and re-run, or wait for the first tagged release."
  fi
else
  ver=${VERSION#v}
  archive="${BIN}_${ver}_${os}_${arch}.tar.gz"
  base="https://github.com/$REPO/releases/download/$VERSION"
  step "Downloading ${BIN} ${VERSION}"

  # A private umask for the download: the archive lands here before it is
  # verified, and nothing else on the machine needs to read it.
  old_umask=$(umask); umask 077
  tmp=$(mktemp -d 2>/dev/null || mktemp -d -t replay)
  umask "$old_umask"
  trap 'rm -rf "$tmp"' EXIT INT TERM

  if [ "$DRY_RUN" -eq 1 ]; then
    info "would download ${base}/${archive}"
    info "would verify against ${base}/checksums.txt"
    info "would install to ${BIN_DIR}/${BIN}"
  else
    save "$base/$archive" "$tmp/$archive" \
      || die "No build for ${os}/${arch} in ${VERSION}.
   Check https://github.com/$REPO/releases/tag/$VERSION for what was published."

    # A checksum that is merely "usually checked" is not checked. If the file is
    # reachable and a hashing tool exists, a mismatch stops the install.
    if save "$base/checksums.txt" "$tmp/checksums.txt" 2>/dev/null; then
      if command -v sha256sum >/dev/null 2>&1; then sum=$(sha256sum "$tmp/$archive" | cut -d' ' -f1)
      elif command -v shasum >/dev/null 2>&1; then sum=$(shasum -a 256 "$tmp/$archive" | cut -d' ' -f1)
      else sum=""; fi
      if [ -n "$sum" ]; then
        # Bind the digest to THIS filename. An unanchored grep would accept the
        # hash of any file listed in checksums.txt.
        want=$(awk -v f="$archive" '$2 == f || $2 == "*" f { print $1; exit }' "$tmp/checksums.txt")
        [ -n "$want" ] || die "checksums.txt does not list ${archive}. Nothing was installed."
        [ "$want" = "$sum" ] || die "Checksum mismatch for ${archive}. Nothing was installed.
   checksums.txt says ${want}, the download hashes to ${sum}."
        ok "Checksum verified"

        # The release signs checksums.txt with Sigstore keyless signing. If
        # cosign is here, check it: a checksum fetched from the same origin as
        # the archive proves the download is intact, not that it is genuine.
        if command -v cosign >/dev/null 2>&1; then
          if save "$base/checksums.txt.pem" "$tmp/checksums.txt.pem" 2>/dev/null &&
             save "$base/checksums.txt.sig" "$tmp/checksums.txt.sig" 2>/dev/null; then
            if cosign verify-blob \
                 --certificate "$tmp/checksums.txt.pem" \
                 --signature "$tmp/checksums.txt.sig" \
                 --certificate-identity-regexp "https://github.com/$REPO/.*" \
                 --certificate-oidc-issuer https://token.actions.githubusercontent.com \
                 "$tmp/checksums.txt" >/dev/null 2>&1; then
              ok "Signature verified (Sigstore, built by CI from the tag)"
            elif [ "$ALLOW_UNVERIFIED" -eq 1 ]; then
              warn "Signature did NOT verify. Continuing because --no-verify was passed."
            else
              die "Signature verification FAILED for checksums.txt. Nothing was installed.
   The checksums may be intact but they were not signed by this project's CI."
            fi
          else
            info "no signature published for ${VERSION}; checksum only"
          fi
        else
          info "cosign not installed, so the signature was not checked. Checksums only."
          info "To verify provenance: https://github.com/${REPO}#install"
        fi
      elif [ "$ALLOW_UNVERIFIED" -eq 1 ]; then
        warn "No sha256 tool found. Installing unverified because --no-verify was passed."
      else
        die "No sha256 tool (sha256sum or shasum) was found, so the download cannot be
   verified. Install one, or re-run with --no-verify to accept an unverified binary."
      fi
    elif [ "$ALLOW_UNVERIFIED" -eq 1 ]; then
      warn "checksums.txt could not be fetched. Installing unverified because --no-verify was passed."
    else
      die "checksums.txt could not be fetched from ${base}, so the download cannot be verified.
   Nothing was installed. Re-run with --no-verify to accept an unverified binary."
    fi

    tar -xzf "$tmp/$archive" -C "$tmp" || die "Could not unpack ${archive}."
    [ -f "$tmp/$BIN" ] || die "The archive did not contain a ${BIN} binary."
    mkdir -p "$BIN_DIR"
    install -m 0755 "$tmp/$BIN" "$BIN_DIR/$BIN" 2>/dev/null \
      || { cp "$tmp/$BIN" "$BIN_DIR/$BIN" && chmod 0755 "$BIN_DIR/$BIN"; } \
      || die "Could not write to ${BIN_DIR}. Re-run with --bin-dir <somewhere writable>."
    ok "Installed ${BIN} ${VERSION}"
  fi
fi

[ "$DRY_RUN" -eq 1 ] && { info "Dry run. Nothing was changed."; exit 0; }

# --------------------------------------------------------------------- PATH
# Say the truth about PATH rather than editing a shell rc behind someone's back.
on_path=0
case ":$PATH:" in *":$BIN_DIR:"*) on_path=1 ;; esac
if [ "$on_path" -eq 0 ] && [ "$MODIFY_PATH" -eq 1 ]; then
  case "${SHELL##*/}" in
    zsh)  rc="~/.zshrc" ;;
    bash) rc="~/.bashrc" ;;
    fish) rc="~/.config/fish/config.fish" ;;
    *)    rc="your shell profile" ;;
  esac
  warn "${BIN_DIR} is not on your PATH, so the ${BIN} command will not be found yet."
  if [ "${SHELL##*/}" = fish ]; then
    printf '  %sfish_add_path %s%s\n' "$C_B" "$BIN_DIR" "$C_0" >&2
  else
    printf '  %secho '\''export PATH="%s:$PATH"'\'' >> %s%s\n' "$C_B" "$BIN_DIR" "$rc" "$C_0" >&2
  fi
  info "then open a new shell, or: export PATH=\"$BIN_DIR:\$PATH\""
fi

# ------------------------------------------------------------------- corpus
# There is no prompt here on purpose. `curl | sh` has no terminal to answer
# one, and consent given before the tool has ever run is consent to a payload
# the person has not seen. This flag exists for people who have already read
# the docs and want to say yes once, non-interactively, for a fleet.
if [ "$CORPUS_OPT_IN" -eq 1 ]; then
  cfg_dir="${XDG_CONFIG_HOME:-$HOME/.config}/replay"
  mkdir -p "$cfg_dir"
  # A dedicated file, because '>' on config.toml would silently truncate any
  # other settings the user already had there.
  printf 'corpus_opt_in = true\n# Written by install.sh --corpus-opt-in on %s\n# Delete this file to withdraw. Nothing is sent until you run: replay corpus --submit\n' \
    "$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "$cfg_dir/corpus-consent.toml"
  ok "Corpus contribution enabled in ${cfg_dir}/corpus-consent.toml"
  info "Still nothing is sent until you run: ${BIN} corpus --submit"
fi

# -------------------------------------------------------------------- finish
printf '\n' >&2
if [ -n "$previous" ]; then
  info "replaced  ${previous}"
fi
printf '%sNext:%s  %s%s doctor%s   %s# what Replay can see on this machine%s\n' \
  "$C_B" "$C_0" "$C_ACCENT$C_B" "$BIN" "$C_0" "$C_DIM" "$C_0" >&2
printf '        %s%s ~/.claude/projects/<project>/%s   %s# read a session you already paid for%s\n' \
  "$C_ACCENT$C_B" "$BIN" "$C_0" "$C_DIM" "$C_0" >&2
# Discovery, not consent. Naming the command is the installer's job; deciding
# is the user's, later, with the report in front of them.
if [ "$CORPUS_OPT_IN" -eq 0 ]; then
  printf '\n%sReplay makes no network request except to the provider you configured.%s\n' \
    "$C_DIM" "$C_0" >&2
  printf '%sTo help calibrate it against real traffic:%s %s%s corpus%s%s shows what your own\nsessions look like and sends nothing.%s %s%s corpus --submit%s%s offers to.%s\n' \
    "$C_DIM" "$C_0" "$C_B" "$BIN" "$C_0" "$C_DIM" "$C_0" "$C_B" "$BIN" "$C_0" "$C_DIM" "$C_0" >&2
fi

printf '\n%sDocs%s https://github.com/%s#readme   %sUninstall%s rm %s/%s\n' \
  "$C_DIM" "$C_0" "$REPO" "$C_DIM" "$C_0" "$BIN_DIR" "$BIN" >&2
