# Release runbook and reproducibility manifest for the v1.0 candidate, 2026-10-09

**What this is.** The release, prepared and not performed: the manifest
that pins what would be built, the exact commands in order, who must
authorize each, what each prints when it succeeds, where to stop, and how
to roll back. Candidate `37c0d0a8116b88b8fe7bbf6b6bf0630a2f20bec7` on
`fix/compaction-observed-vs-inferred`; the merge commit on `main`
supersedes it as the SHA every later step names.

**What was executed in this pass:** nothing in sections 3 to 7. The
verification in section 2 ran. No tag was created or moved, no release
published, no shim pushed, no site deployed, no branch merged.

## 1. Reproducibility manifest

| Item | Value on the candidate |
|---|---|
| Source | `37c0d0a`, 63 commits ahead of `main` (`5c87562`, v0.8.0), 0 behind, tree clean |
| Compiler | go1.27.2, selected by `go.mod`'s `toolchain` line in `release.yml` (`go-version-file: go.mod`) |
| Language floor | `go 1.24` |
| Third-party modules | none; no `go.sum`; `go mod tidy -diff` empty |
| Build flags | `-trimpath`; ldflags set `internal/version.Version`, `.Commit`, `.Date` from the tag, short commit and commit date; `mod_timestamp` is the commit timestamp (`.goreleaser.yaml`) |
| Platforms | linux and darwin, amd64 and arm64; Windows excluded by design and refused at run time |
| Packaging | tar.gz archives; deb, rpm, apk and pkg.tar.zst for Linux; one SBOM per archive (the ten Linux packages carry none, stated in RELEASE-CRITERIA.md) |
| Signing | `checksums.txt` signed by cosign v2.5.2, Sigstore keyless, identity bound to `https://github.com/RedRobotKK/Replay/.*` and the GitHub OIDC issuer; syft v1.42.3 for SBOMs; both pinned with the reason in `release.yml` |
| Ancestry gate | `release.yml` runs `git merge-base --is-ancestor "$GITHUB_SHA" origin/main` and exits 1 for a tag that is not on `main` |
| Post-build check | `release.yml` runs `scripts/release-check.sh` against the published binary |
| Reproduction | `scripts/reproduce-release.sh <tag> <goos> <goarch>` reads the toolchain from the published binary, rebuilds and compares bytes; v0.8.0 reproduced on all four platforms on 2026-10-08 under go1.25.13; the next tag is the first built by go1.27.2 and must be reproduced again |
| Installer | `install.sh` in the tree, sha256 `74a13afd523dca3efffea203d18bc02c22e7a9d1233a64d8ee7219ffbff4d8a8`, 556 lines; identical bytes served at replay.doctor `/replay.sh` and both redrobot.jp paths on 2026-10-09 19:40 UTC; it resolves `releases/latest` and verifies the archive against `checksums.txt` |
| Frozen catalogue | `internal/mutation/testdata/mutants.json` sha256 `bba10c8c5ff026f92eb3f5970a888020f19617b3dfb4a8bbb32e5bca26080dac`, 144 entries |
| Claim register | `CLAIM-REGISTER.md` regenerates byte-identical on the candidate |
| Shims | npm and PyPI packages named `replay-doctor` are published by `publish-shims.yml` on `workflow_run` of the Release workflow (or by dispatch with a tag) and download the same signed archives |

Secrets and privacy in release outputs: the archives contain the binary
only; the SBOM lists the standard library; no path under a home
directory, no credential and no transcript enters any artifact.
`scripts/release-check.sh` asserts the binary's printed denominators and
field names against the tree.

## 2. Verified on the candidate before any release step

| Check | Result |
|---|---|
| CI run 37961162745 on `37c0d0a` | 16 of 16 success, including frozen mutants 144 of 144 and govulncheck clean (ledger section 3) |
| Local race suite, vet, gofmt, tidy, coverage gate, claim register, release-check, installer drift | all exit 0 (ledger section 3) |
| Protected evidence | five TTL files unchanged by blob id; `shasum -a 256 -c` OK |

## 3. Prerequisites that are Daniel's, in order

| Step | Action | Authorization | Stop if |
|---|---|---|---|
| P1 | Decide C-2: option A or B (runbook); if B, write the dated amendment and narrow the claim on README.md, docs/ROADMAP.md and the site docs | Daniel | undecided: the tag is not a 1.0 by the criteria |
| P2 | Commission C-1 and receive the published report; land fixes or dated accepted-risk decisions; link it from RELEASE-CRITERIA.md | Daniel, external reviewer | no report: the tag is not a 1.0 by the criteria |
| P3 | ADR-0024: Proposed to Accepted, or amend | Daniel | still Proposed: the deprecation promise is unmade |
| P4 | Name decision and GitHub About (B-12) | Daniel | optional for the tag |
| P5 | Mark PR #336 ready and merge it (ruleset requires the six checks, signatures, linear history) | Daniel | any required check not green on the head |

A release cut before P1 and P2 is a v0.9.x by the criteria, not v1.0, and
the runbook below works for either; only the version string differs.

## 4. The release commands, prepared, not run

Every command below is to be typed by Daniel on a checkout of `main` at
the merge commit; none was executed in this pass.

```sh
# 4.1 Confirm the SHA and that the required checks are green on it
git fetch origin && git checkout main && git pull --ff-only
git rev-parse HEAD                                   # record: the merge commit
gh run list --branch main --limit 3                  # the push run on the merge commit: success
gh api repos/RedRobotKK/Replay/commits/$(git rev-parse HEAD)/check-runs \
  --jq '.check_runs[] | "\(.name): \(.conclusion)"'  # six required contexts: success

# 4.2 Move Unreleased into a dated section (release PR, reviewed like code)
#     CHANGELOG.md: "## [Unreleased]" -> "## [1.0.0] - <date>" plus a fresh Unreleased header
#     Merge that PR; re-run 4.1 on the new merge commit.

# 4.3 Tag. Irreversible once pushed: the tag push is the trigger.
git tag -s v1.0.0 -m "v1.0.0"                        # signed; ruleset requires signatures on main
git tag -v v1.0.0
git push origin v1.0.0                               # STOP before this line without explicit authorization

# 4.4 Watch the Release workflow to a terminal state (bounded poll, not a single sleep)
gh run list --workflow Release --limit 1
gh run watch <run id> --exit-status

# 4.5 Verify the published assets
gh release view v1.0.0 --json assets --jq '.assets[].name'   # 19 names, as v0.8.0
cosign verify-blob --certificate checksums.txt.pem --signature checksums.txt.sig \
  --certificate-identity-regexp 'https://github.com/RedRobotKK/Replay/.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com checksums.txt
for p in "darwin arm64" "darwin amd64" "linux amd64" "linux arm64"; do
  ./scripts/reproduce-release.sh v1.0.0 $p            # each: REPRODUCES ... byte-identical, exit 0
done

# 4.6 Shims: publish-shims.yml runs on the completed Release workflow; confirm
gh run list --workflow "Publish the npm and PyPI shims" --limit 1
npm view replay-doctor version                        # 1.0.0
pip index versions replay-doctor                      # 1.0.0

# 4.7 Installer, live, after the release
curl -fsSL https://replay.doctor/replay.sh | sha256sum   # 74a13afd... unless install.sh changed
scripts/installer-drift/check.sh                         # matches
```

Expected outputs are the ones v0.8.0 produced on 2026-10-05 and recorded
in CHANGELOG.md and the reproducible-build record; a deviation at any
step is a stop.

Three separate authorizations, as the project's own procedure has held
since v0.8.0: one to merge (P5), one to open the release PR (4.2), one to
push the tag (4.3). The third is the irreversible and public one.

## 5. Release notes

GoReleaser generates the notes from commit subjects (fixes, documentation,
other) with the signing footer in `.goreleaser.yaml`. For a 1.0, the
CHANGELOG section moved in 4.2 is the authoritative text and must state,
as the tree states today:

- one provider calibrated against real traffic; the second provider with
  rules, dispatch and a labelled experimental path, calibration outstanding
  or amended per P1;
- the external security review's status per P2, never as completed unless
  the report is linked;
- the toolchain floor go1.27.2 and the thirteen advisories;
- the compatibility surfaces the version number covers (ROADMAP v1.x,
  ADR-0024);
- macOS and Linux only; Windows refused;
- figures labelled measured, estimated, structural, not measured, and no
  headline figure that the instrument has not tried to falsify.

## 6. The replay.doctor re-pin, after the merge (B-20)

Verified in a scratch clone on 2026-10-09 (ledger section 5); to be run
by Daniel on `RedRobotKK/replay.doctor` after P5, never before:

```sh
git checkout main && git pull --ff-only && git checkout -b docs/repin-<merge sha short>
echo <merge commit full sha> > replay.ref
npm run vendor:installer           # vendors main's HEAD install.sh and records its commit
REPLAY_REPO=<path to a Replay checkout> npm run build
REPLAY_REPO=<path to a Replay checkout> npm test      # expect 88 of 88 in the six files run today, and the full suite green
grep -rlE 'adversarial security review|an adversarial pass' src/content/docs/docs/index.md ; echo "exit $? (1 means absent)"
# open a PR; merge; Cloudflare Pages deploys main
```

Stop conditions: `npm test` not green; `check:surfaces:public` reporting
anything beyond the standing `external-redrobot-jp` NOT_MEASURED row; or
B-22 undecided (the re-pin publishes the closure records of 2026-10-08 and
2026-10-09 as evidence pages, which quote the withdrawn figure and name two
uncontacted firms). If Daniel decides those records stay off the site, the
pull script needs an exclusion list before the re-pin, which is a site
change outside this repository.

## 7. Rollback and recovery

| Failure | Action | Authority |
|---|---|---|
| Release workflow fails before publishing | nothing is public; fix on `main` by PR; delete the local tag; a new tag with a new patch number, never the same name | Daniel |
| Release published, defect found | `gh release edit v1.0.0 --prerelease` removes it from `releases/latest`, which is what the installer and `upgrade` resolve; or `gh release delete v1.0.0` after the assets are saved for the record. The tag stays: tags are not moved by policy. Ship v1.0.1 | Daniel |
| Shim published with a defect | `npm deprecate replay-doctor@1.0.0 "<reason>"`; PyPI: yank 1.0.0 in the project UI; both leave the version resolvable by pin and hidden from default installs | Daniel |
| Installer serving a stale copy | `scripts/installer-drift/check.sh` names the drifted host; redeploy the site from its vendored copy | Daniel, site deploy |
| A frozen mutant survives on `main` | the push run goes red; the merge is reverted by PR, not force-pushed (ruleset) | Daniel |
| Protected evidence touched | restore from `0ea7370` by blob id; the five ids are in the ledger, section 6 | engineering |

Tested to the extent possible without production actions: `gh release
view v0.8.0` lists 19 assets; the ruleset forbids deletion and non-fast-
forward on `main`; `install.sh` reads `tag_name` from `releases/latest`
(line 221), so a pre-release mark is sufficient to withdraw a build from
new installs. Not tested: the pre-release flip itself, the npm deprecate,
the PyPI yank.

---

[Evidence index](README.md) · [Closure ledger](closure-ledger-1.0-2026-10-09.md) ·
[Release criteria](../../RELEASE-CRITERIA.md) · [Housekeeping](../maintainers.md)
