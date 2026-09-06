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
#                       pass it. It writes a file and sends nothing, ever, and
#                       no command in this release sends it either: sharing is
#                       designed (ADR-0007, ADR-0008) and not built.
#   --help              this text
#
# Environment: REPLAY_VERSION, REPLAY_BIN_DIR, NO_COLOR.

set -eu

REPO="RedRobotKK/Replay"
COFFEE="https://buymeacoffee.com/saitodaniel"
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
    --version)       [ $# -ge 2 ] || die "--version needs a value."; VERSION="$2"; shift 2 ;;
    --version=*)      VERSION="${1#*=}"; shift ;;
    --bin-dir)       [ $# -ge 2 ] || die "--bin-dir needs a value."; BIN_DIR="$2"; shift 2 ;;
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
RESOLVED=unknown
if [ -z "$VERSION" ]; then
  step "Resolving the latest release"
  # Distinguish "there is no release" from "we could not ask". Swallowing that
  # difference meant one rate-limited API call (the unauthenticated GitHub limit
  # is 60/hr per IP, routinely hit on CI and behind shared NAT) silently dropped
  # every verification below, and said something untrue while doing it.
  if api=$(fetch "https://api.github.com/repos/$REPO/releases/latest" 2>/dev/null); then
    RESOLVED=ok
    VERSION=$(printf '%s' "$api" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1)
  else
    RESOLVED=failed
  fi
fi

# ----------------------------------------------------- no release yet: source
if [ -z "${VERSION:-}" ] && [ "$RESOLVED" = failed ]; then
  die "Could not reach the GitHub releases API, so the latest version is unknown.
     This script will not quietly fall back to an unverified source build: that
     would skip the checksum and signature checks you ran it for.
     Retry, or pass --version <tag> to install a specific release."
fi

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
          elif [ "$ALLOW_UNVERIFIED" -eq 1 ]; then
            warn "No signature published for ${VERSION}. Continuing because --no-verify was passed."
          else
            die "No Sigstore signature was published for ${VERSION}, and cosign is
     installed to check one. Every release this project's CI builds is signed, so
     a missing signature means these assets are not the ones CI produced.
     Nothing was installed. Pass --no-verify to install without this check."
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

    # Extract only the binary, and only if the archive's entry for it is a
    # regular file. A member that is a symlink named `replay` would otherwise be
    # followed by the `-f` test below and installed as a 0755 executable holding
    # whatever it pointed at on this machine.
    tar -xzf "$tmp/$archive" -C "$tmp" "$BIN" || die "Could not unpack ${BIN} from ${archive}."
    [ ! -L "$tmp/$BIN" ] || die "${archive} contains a symlink where ${BIN} should be. Nothing was installed."
    [ -f "$tmp/$BIN" ] || die "The archive did not contain a ${BIN} binary."
    # The guidance below is only reachable if mkdir is allowed to fail. Under
    # `set -e` a bare `mkdir -p` on an unwritable parent kills the script here,
    # and the user gets `Permission denied` instead of the sentence written to
    # tell them what to do about it.
    mkdir -p "$BIN_DIR" 2>/dev/null \
      || die "Could not create ${BIN_DIR}. Re-run with --bin-dir <somewhere writable>."
    # Land beside the destination, not on it.
    #
    # Whatever is already at $BIN_DIR/$BIN is, as far as this script knows, a
    # working install. Overwriting it before the new binary has been proved to
    # run means a failed upgrade takes the user's working copy with it: a
    # wrong-architecture download, an interrupted copy, a full disk, and they
    # are left with nothing where they started with something. Removing the
    # broken file afterwards is not a fix — it just makes the loss tidy.
    #
    # So install to a sibling, run it there, and only then move it into place.
    # The final mv is atomic within a filesystem, so there is no window where
    # the destination holds a half-written file.
    staged="$BIN_DIR/.$BIN.new.$$"
    install -m 0755 "$tmp/$BIN" "$staged" 2>/dev/null \
      || { cp "$tmp/$BIN" "$staged" && chmod 0755 "$staged"; } \
      || die "Could not write to ${BIN_DIR}. Re-run with --bin-dir <somewhere writable>."
    # Run it once before claiming success. Everything above verifies the bytes
    # that arrived; none of it proves the result executes here. A binary for the
    # wrong architecture, a chmod that did not take, a libc mismatch the platform
    # check missed: each installs cleanly, fails on first use, and would have
    # been reported as "Installed" either way.
    if ! "$staged" version >/dev/null 2>&1; then
      # It does not run. Discard the staged copy and leave whatever was already
      # installed exactly as it was — the user keeps the working binary they
      # had, and nothing that cannot run reaches their PATH.
      why=$("$staged" version 2>&1 | head -3 || true)
      rm -f "$staged"
      [ -n "$why" ] && info "$why"
      if [ -x "$BIN_DIR/$BIN" ]; then
        die "Downloaded ${BIN} ${VERSION}, but it does not run on this machine. Nothing was changed; your existing ${BIN_DIR}/${BIN} is untouched."
      fi
      die "Downloaded ${BIN} ${VERSION}, but it does not run on this machine. Nothing was installed."
    fi
    mv -f "$staged" "$BIN_DIR/$BIN" \
      || { rm -f "$staged"; die "Could not move ${BIN} into ${BIN_DIR}. Nothing was changed."; }

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
  # Refuse to follow a symlink here. The reasoning below about not truncating
  # an existing config does not survive one: a link planted at this path would
  # redirect the write to whatever it points at.
  if [ -L "$cfg_dir/corpus-consent.toml" ]; then
    die "${cfg_dir}/corpus-consent.toml is a symlink. Nothing was written."
  fi
  printf 'corpus_opt_in = true\n# Written by install.sh --corpus-opt-in on %s\n# Delete this file to withdraw. Nothing is sent: no command in this release transmits it.\n' \
    "$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "$cfg_dir/corpus-consent.toml"
  ok "Corpus contribution enabled in ${cfg_dir}/corpus-consent.toml"
  info "Nothing is sent: no command in this release transmits it."
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
# Discovery, not consent. Naming the local report is the installer's job. There
# is no submission path to point at: the binary sends nothing anywhere.
if [ "$CORPUS_OPT_IN" -eq 0 ]; then
  printf '\n%sReplay originates no network request you did not type. The proxy forwards\nyour own traffic; %s rules --check-prices and %s probe --execute are the two\nthat reach out, and only when you run them.%s\n' \
    "$C_DIM" "$BIN" "$BIN" "$C_0" >&2
  printf '%sTo see how well it is calibrated on your own traffic:%s %s%s corpus%s%s shows what\nyour sessions look like, on this machine, and sends nothing anywhere.%s\n' \
    "$C_DIM" "$C_0" "$C_B" "$BIN" "$C_0" "$C_DIM" "$C_0" >&2
fi

# Said once, at the only moment the person is definitely reading, and never
# again: Replay has no account and phones nothing home, so there is no second
# opportunity and no nag. Tied to a real number rather than a general appeal,
# because the corpus that makes the tool's figures measured rather than guessed
# was paid for out of pocket.
printf '\n%sReplay is free, Apache 2.0, and funded by nobody. The corpus behind its\nnumbers is 78 sessions across 1,450 transcripts, on one machine.%s\n' \
  "$C_DIM" "$C_0" >&2
printf '%sIf it saves you more than it cost you to install:%s %s%s%s\n' \
  "$C_DIM" "$C_0" "$C_ACCENT$C_B" "$COFFEE" "$C_0" >&2

printf '\n%sDocs%s https://github.com/%s#readme   %sUninstall%s rm %s/%s\n' \
  "$C_DIM" "$C_0" "$REPO" "$C_DIM" "$C_0" "$BIN_DIR" "$BIN" >&2
