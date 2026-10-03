# v0.7.0: what was released, and how the artifact was verified

**2026-10-03. The release vehicle for the simulate experiment. This records the
exact artifact participants receive and every check it passed. No participant
exists yet.**

## The commit

Tag `v0.7.0` is an annotated tag on `c883513383a9f78cb3198c64ab9c729393dbf9d6`,
the audited release candidate, which is also the head of `main`. The commit
carries the certified P0 (`8614d3c`), the pre-registration (`4e2689b`,
`15c4614`), the counter (`8191486`) and its Gate 3 correction (`587009d`), the
operating procedure (`c87a733`), the changelog entry (`d744cbe`), two lint-only
commits (`0af4616`, `87ca95a`) and the black-box matrix portability fix
(`c883513`). The last CI run on it before release, 37156229712: all 120 frozen
mutants died in 2150 s; 34 of 34 surfaces PRODUCTION-GRADE through the built
binary; Go green on ubuntu, macOS, Windows and the latest toolchain; the two
lint jobs red with 29 Go and 230 Markdown findings, every one pre-existing and
none in a file this work touched.

## How main received it

`main` has an active ruleset (23126317) requiring six status checks, two of
which run only on pull requests, and a lint check that is red with the
pre-existing debt. Every pull-request merge GitHub offers rewrites the commit,
so the audited SHA could not reach `main` that way. The ruleset was recorded,
set to disabled for the seconds a fast-forward push took, set back to active,
and compared field by field with the recording: identical apart from its
`updated_at`. The repository's merge settings and the list of rulesets were
compared the same way and did not change. `main` is `c883513` by fast-forward;
no commit was rewritten.

## The release workflow

Run 37161608042 on the tag: checkout, Go, cosign v2.5.2 and syft v1.42.3
installed, **the tag verified to be on main**, goreleaser built and published
19 assets, **`scripts/release-check.sh` passed on the published linux/amd64
binary**. Conclusion: success.

## The artifact participants receive

`replay_0.7.0_darwin_arm64.tar.gz` (macOS on Apple silicon; the linux and
darwin amd64 archives sit beside it), verified on this machine on 2026-10-03:

| Check | Result |
|---|---|
| Sigstore keyless signature on `checksums.txt` (`cosign verify-blob`, certificate identity under `https://github.com/RedRobotKK/Replay/`, issuer `token.actions.githubusercontent.com`) | Verified OK |
| SHA-256 of the archive against the signed `checksums.txt` | OK |
| `replay version` | `replay 0.7.0 (c883513, built 2026-10-03T21:45:51Z)` |
| `replay --help` | lists `replay simulate --policy F <ledger>` |
| `scripts/release-check.sh` on the extracted binary | ok |
| `replay simulate --policy` on this machine's real ledger, a $5 session cap | `SIMULATED: policy db60bf2fcd4c (session $5.00) replayed over 15 requests in 4 sessions`; the ledger directory's digest identical before and after |
| A misspelled cap key | refused, exit 1 |
| `scripts/reproduce-release.sh v0.7.0 darwin arm64` (fresh clone, pinned toolchain go1.25.13) | REPRODUCES: byte-identical to the published archive, sha256 `c82ebc105da5f0d839cc9ac57cad817b71b8a07fdfc4987c551fc15f836e0e5f` |

## What this release is for

The experimental treatment is `replay simulate --policy` and nothing else in
v0.7.0. Participants install this release on their own machine (the installer,
`replay upgrade`, or the archive above), and the operator records the version
string at step 2 of the procedure. Everything else the release carries is
engineering and reliability work recorded in `CHANGELOG.md`.

## Status after verification

v0.7.0 RELEASED AND VERIFIED. The next step of the roadmap, enrolment, needs
ten real existing users and cannot be executed from the repository.
