#!/bin/sh
# Regenerates the SIGR fixtures. Run from this directory; needs openssl 3.
#
# The fixtures are generated rather than built in the test because the test
# must not import crypto/ecdsa or crypto/elliptic: TestX402_NoSigningCapability
# keeps signing out of every file in the module, tests included.
#
# Every private key is made in a temporary directory and deleted on exit, so
# nothing here can sign anything again. Certificates are valid for 100 years
# because cosign checks validity against the clock when the transparency log is
# skipped, and a fixture that expires is a test that starts failing on a date.
set -eu
REPO=RedRobotKK/Replay
TAG=v9.9.9
GH=https://token.actions.githubusercontent.com
k=$(mktemp -d)
trap 'rm -rf "$k"' EXIT

printf '%s\n' "0000000000000000000000000000000000000000000000000000000000000000  replay_9.9.9_linux_amd64.tar.gz" > checksums.txt

openssl ecparam -name prime256v1 -genkey -noout -out "$k/ca.key"
openssl req -x509 -new -key "$k/ca.key" -days 36500 -subj "/O=replay test/CN=sigr test root" \
  -addext "basicConstraints=critical,CA:TRUE" -addext "keyUsage=critical,keyCertSign,cRLSign" -out root.crt

n=1
leaf() { # name subject issuer
  hex=$(printf '%s' "$3" | od -An -tx1 | tr -d ' \n')
  cat > "$k/ext.cnf" <<CNF
[ext]
keyUsage = critical,digitalSignature
extendedKeyUsage = codeSigning
subjectAltName = critical,URI:$2
1.3.6.1.4.1.57264.1.1 = DER:$hex
1.3.6.1.4.1.57264.1.8 = ASN1:UTF8String:$3
CNF
  openssl ecparam -name prime256v1 -genkey -noout -out "$k/$1.key"
  openssl req -new -key "$k/$1.key" -subj "/O=replay test" -out "$k/$1.csr"
  n=$((n + 1))
  openssl x509 -req -in "$k/$1.csr" -CA root.crt -CAkey "$k/ca.key" -set_serial "$n" -days 36500 \
    -extfile "$k/ext.cnf" -extensions ext -out "$1.crt" 2>/dev/null
  openssl dgst -sha256 -sign "$k/$1.key" checksums.txt | openssl base64 -A > "$1.sig"
}

R=https://github.com/$REPO/.github/workflows
leaf control     "$R/release.yml@refs/tags/$TAG" "$GH"
leaf ci          "$R/ci.yml@refs/tags/$TAG" "$GH"
leaf main        "$R/release.yml@refs/heads/main" "$GH"
leaf othertag    "$R/release.yml@refs/tags/v0.8.0" "$GH"
leaf fork        "https://github.com/attacker/Replay/.github/workflows/release.yml@refs/tags/$TAG" "$GH"
leaf contains    "https://attacker.example/$R/release.yml@refs/tags/$TAG" "$GH"
leaf wrongissuer "$R/release.yml@refs/tags/$TAG" "https://accounts.google.com"
# Certificates are .crt because .gitignore drops *.pem (and *.key) repo-wide as
# possible secrets; a .pem fixture would be silently left out of every commit.
