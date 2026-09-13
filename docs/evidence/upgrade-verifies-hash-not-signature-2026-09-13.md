# `replay upgrade` verifies the hash, not the signature

**Read 2026-09-13.** Answers open question 4 of the PRD: *"Does `upgrade`
verify the signature? To read in `internal/selfupdate`; add if it only hashes."*

**It only hashes.**

## What is actually true

Releases **are** signed. `.github/workflows/release.yml` installs cosign v2.5.2
at a pinned SHA and publishes `checksums.txt.pem` and `checksums.txt.sig`
alongside `checksums.txt`, using Sigstore keyless signing under the workflow's
own OIDC identity.

`install.sh` **does** check it. If cosign is present on the machine and no
signature is found, it refuses to install rather than proceeding.

`internal/selfupdate` does **neither**. `fetch.go` fetches `checksums.txt` over
HTTPS, looks the archive up in it, compares sha256, and refuses on a mismatch
or on a checksums file it could not fetch. There is no `--no-verify` and that
is deliberate. But nothing reads `checksums.txt.pem` or `checksums.txt.sig`,
and no cosign verification happens anywhere in the upgrade path.

## Why this is worth a dated file rather than a quiet patch

**The two install routes make different promises and nobody said so.** A user
who runs `install.sh` with cosign installed gets signature verification. The
same user running `replay upgrade` the following week does not, and nothing in
either path tells them the guarantee changed. That asymmetry is the finding;
the missing feature is the smaller half of it.

**What the hash check does and does not cover.** It covers a corrupted or
truncated download, and a modified archive served against an unmodified
`checksums.txt`. It does not cover anything that can modify `checksums.txt`
itself, because the file that is trusted is the file that is fetched. The
signature exists precisely to close that, and it is published and unused.

## What was not done, and why it is not a one-line fix

Verifying a Sigstore bundle properly means a Fulcio certificate chain, a Rekor
inclusion proof, and a certificate identity check against the workflow that
signed it. This module has **zero third-party dependencies** by design, and
`sigstore-go` is not a small one. Shipping a partial check that parses a `.sig`
without validating the certificate identity would be worse than the current
state, because it would read as verification in the code and in the docs while
proving that somebody signed something.

So the state is recorded rather than half-closed. The options, none chosen here:

1. Take the dependency and verify properly, which ends the zero-dependency
   property that several of this project's other claims rest on.
2. Shell out to `cosign` when it is on PATH, exactly as `install.sh` does, and
   say plainly that upgrade is verified only where cosign is installed.
3. Leave it, and state on the surfaces page that `upgrade` is hash-verified and
   `install.sh` is signature-verified where cosign exists.

Option 2 is the one that makes the two routes agree, and it is the one this
file recommends. It is not launch-critical: it does not regress anything, and
the honest sentence costs nothing to publish today.

---

[Evidence index](README.md) · [Surfaces](../SURFACES.md)
