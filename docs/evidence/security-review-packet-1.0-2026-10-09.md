# External security review packet for the v1.0 candidate, 2026-10-09

**What this is.** The reviewer-ready packet for condition C-1 of the v1.0
closure ledger: everything an independent reviewer needs to start, bound
to one commit, with the internal work and the unperformed external work
kept apart. It consolidates
[security-review-scope-2026-10-08.md](security-review-scope-2026-10-08.md)
(scope, threat model, brief, acceptance criteria) and phase 2 section 5
(funding eligibility, two uncontacted candidates, the request-for-quote
draft) and adds what a reviewer would otherwise have to dig out of the
tree.

**What this is not.** Nobody has been contacted. No review is scheduled.
No spend is authorized. Every security statement below is the
maintainer's own, or a tool's, and is labelled as such. C-1 is BLOCKED on
Daniel choosing a reviewer, authorizing the message in phase 2 section
5.3, and later authorizing a quoted spend.

## 1. Candidate and scope

| Item | Value |
|---|---|
| Candidate | `37c0d0a8116b88b8fe7bbf6b6bf0630a2f20bec7`, branch `fix/compaction-observed-vs-inferred`, PR #336; the merge commit on `main` supersedes it once Daniel merges |
| Language, size | Go, 41 packages, zero third-party Go dependencies (no `go.sum`, no `require` block), toolchain go1.27.2 |
| Licence | BUSL-1.1, converting to Apache 2.0 on 2029-09-06 (ADR-0005) |
| Scope items | the seven in security-review-scope section 3, in the priority order of phase 2 section 5.4: proxy network guards, masking end to end, ledger content-freedom, OpenAI-compatible path label accuracy, installer checksum verification, self-update fetch-verify-execute, provenance semantics of printed figures |
| Out of scope | the hosted service of ADR-0028 (not built), Windows (refused by design), the two websites (separate repositories), anything needing live OpenAI access |

## 2. Architecture and trust boundaries

Replay is a single static binary with two modes:

- **Offline reader.** Commands such as `cost`, `diff`, `blame`, `context`
  and `corpus` read agent transcripts already on disk under the operator's
  home directory and print figures. No network.
- **Local transparent proxy.** `replay serve` binds a loopback listener and
  forwards the operator's own API traffic to the provider, recording a
  content-free ledger record per turn, and masking secrets when
  `--mask` is set. One upstream leg, HTTP/1.1 only by construction
  (`internal/proxy/http2pin_test.go`).

Trust boundaries, from the README's Footprint section and
`docs/SURFACES.md`:

| Boundary | What crosses it | Guard |
|---|---|---|
| Local process to loopback listener | any HTTP request | loopback bind; refusal of `Origin` and `Sec-Fetch-Mode`; `Host` guard against DNS rebinding (`internal/proxy/hostguard.go`, finding 6); `/replay/healthz` ungated by design; `/replay/status` and `/replay/metrics` unauthenticated unless `--token` |
| Proxy to provider | the operator's own requests, masked on request | HTTP/1.1 upstream by construction; the typed fetches refuse plain HTTP sources and cleartext redirects (`TestRC1_APlainHTTPSourceIsNeverFetched`, `TestX402_RedirectToCleartextIsRefused`) |
| Binary to disk | ledger records, vault ciphertext, key files, config | `internal/ownerdir` verifies owner-only mode on every open and refuses otherwise |
| Binary to the internet, typed by the operator | `rules --check-prices`, `probe --execute`, `upgrade`, `rules --update <url>` | enumerated in the README; `upgrade` is the one path that executes what it fetched |
| Binary to the internet, not typed | `replay burn` probes `127.0.0.1:11434` for a local Ollama | never leaves the machine |

## 3. Threat model and assumptions

Verbatim intent of security-review-scope section 4: a trusted single
operator on a trusted host; a hostile local process or a hostile LAN peer
reaching the loopback listener is in scope; a compromised host is out of
scope by design and disclosed as such. The reviewer is asked to test the
second question and not the first.

## 4. Data flow and privacy boundaries

- The ledger stores block kinds, sizes, timings and usage counts, never
  message text. Tool names are kept in the clear; path arguments and
  tool-call inputs are HMAC'd under a machine-local key on both halves of a
  record since 2026-09-10 (`internal/ledger/responsecallkey_test.go`,
  `TestLedgerHoldsNoArgumentContent`, `TestR1_ASecretOnTheResponsesPathReachesNeitherLedgerNorLog`).
- Masking replaces detected secrets before egress and rehydrates on the
  way back; the vault holds the replaced bytes encrypted, with a TTL
  (`internal/masking`, `TestMaskingReplacesSecretsBeforeEgressAndKeepsThemLocal`,
  `TestEV1_AnExpiredSecretDoesNotRehydrate`, `TestMK1_VaultFailureNeverReturnsTheSecret`).
- No telemetry, no account, no first-run prompt. The tip-jar line is the
  only ask, at most every thirty days, and sends nothing.
- The pool contribution path (`replay corpus -contribute`, replay.doctor
  `/api/contribute`) sends a content-free calibration report under a
  contributor secret stored owner-only (`TestC5_TheContributorSecretIsStableAndOwnerOnly`,
  `TestCalibrationContributionRefusesASymlinkedSecret`).

## 5. Authentication and authorization paths

There is no account system. The only credentials the binary handles are
the operator's own provider keys, which it forwards and never stores, and
`--token` for the status and metrics endpoints. Masking never masks bare
hashes and does mask hex secrets end to end
(`TestBareHashesAreNeverMasked`, `TestHexSecretsAreMaskedEndToEnd`).

## 6. Provider integration surfaces

| Surface | State | Label |
|---|---|---|
| Anthropic Messages API | calibrated against real traffic, 116 sessions ([calibration-corpus-2026-09-10.md](calibration-corpus-2026-09-10.md)) | measured |
| OpenAI `/v1/chat/completions` | read, guarded and ledgered; verified against a stub and a local Ollama endpoint; no live OpenAI call ever made | EXPERIMENTAL, UNMASKED |
| OpenAI `/v1/responses` (Codex) | parsed from fixtures built to Codex's own parser ([responses-r1-2026-10-04.md](responses-r1-2026-10-04.md)); no live call ever made | fixtures, not captures |
| xAI (grok) | `replay grok` reads the ledger; no calibration | totals not measured |

## 7. Cryptographic and provenance assumptions

- Releases: GoReleaser builds with `-trimpath` and a commit-derived
  `mod_timestamp`; `checksums.txt` is signed with Sigstore keyless signing
  bound to the workflow identity; one SBOM per archive (`.goreleaser.yaml`).
  v0.8.0 rebuilt byte-identical on all four platforms
  ([reproducible-build-2026-10-08.md](reproducible-build-2026-10-08.md)).
- Self-update (`internal/selfupdate`): fetch the release index and
  archive, verify the archive against `checksums.txt`, verify the
  signature, then execute. Tests: `TestFetchRefusesAChecksumMismatch`,
  `TestFetchRefusesWhenChecksumsAreMissing`, `TestFD1_AGoodSignatureVerifies`,
  `TestFD9_AWrongLengthSignatureIsNamedAsSuch`,
  `TestFCCOSIGN_EveryPackageReachingTheSignatureCheckPinsIt`.
- Installer (`install.sh`): resolves `releases/latest`, downloads the
  archive and `checksums.txt`, matches the archive against the anchored
  checksum line and dies otherwise, and refuses to build from source when
  it cannot verify (`TestFrozenFD8_TheInstallerPrintsOnlyFiguresTheEvidenceHolds`,
  `TestInstallLineHasNoShellMetacharacters`).
- Vault: AES encryption with the key file beside the ciphertext, bounded
  by a TTL. **This is open finding 3 of the 2026-09-04 review and is still
  open**: RELEASE-CRITERIA.md records that the gate closed by the TTL
  route and the key has not moved.
- Provenance of printed figures: every number carries one of measured,
  estimated, structural, not measured; the claim register
  (`internal/claims`, `CLAIM-REGISTER.md`) and `internal/regression` tests
  guard that no path upgrades an estimate to a measurement.

## 8. Dependency and supply-chain risks

- Zero third-party Go modules; the standard library is the whole
  dependency, which is why thirteen Go advisories moved the toolchain floor
  on 2026-10-09 and why govulncheck is a blocking CI job.
- GitHub Actions pinned by commit SHA; cosign v2.5.2 and syft v1.42.3
  pinned by version with the reason in the workflow comments; GoReleaser
  `~> v2`.
- npm and PyPI shims are published by `publish-shims.yml` after the
  Release workflow completes, with registry provenance; the shims download
  the same signed archives.
- Known residual: the CI canary job `Go (latest release)` runs on
  `stable` by design; a Go release that breaks the standard library is
  caught there before a tag.

## 9. Security tests and their results on the candidate

126 test functions whose names carry a security noun (loopback, origin,
vault, mask, secret, update, signature, checksum, install, cleartext, TLS,
HTTP/2, redact, leak, privacy, content) are in the tree; all pass under
`go test -race -count=1 ./...` locally and on CI run 37961162745 on
`37c0d0a`. Of particular interest to a reviewer:

| Area | Tests |
|---|---|
| Listener guards | `TestBrowserOriginAndTokenChecks`, `TestGG7_LoopbackIsDecidedByAddressNotSpelling`, `TestMT5_TheMetricsListenerIsLoopbackOnly`, `TestDP1_ANonLoopbackBaseIsNeverContacted` |
| Masking | `TestMaskChangesOnlyTheSecretBytes`, `TestMaskingIsDeterministicOnTheWire`, `TestMaskEntropyHeuristic`, `TestEV4_ARemaskedSecretGetsTheSamePlaceholderBack`, `TestO5_TheSecretNeverAppearsInThePayload` |
| Ledger | `TestLedgerHoldsNoArgumentContent`, `TestN5_TheRouteLineLeaksNoIdentifier`, `TestOU7_APrefixDoesNotLeakIntoTheNextRequest`, `TestE2E_Privacy`, `TestE2E_Redact` |
| Update and install | the five in section 7 and `TestIU1_TheInstallCommandIsTheSameEverywhere` |
| Frozen regressions | 144 catalogued defects in `internal/mutation/testdata/mutants.json`, each re-applied and killed on CI (ledger section 2) |

Automated tooling on the candidate: govulncheck v1.8.0, no
vulnerabilities; golangci-lint v2.14, 0 issues; guard reachability, 242
guards, 0 unexplained.

## 10. Known limitations, unresolved findings, accepted risks

| Item | State |
|---|---|
| Finding 3, vault key beside ciphertext | open; TTL bounds the window; disclosed in README and RELEASE-CRITERIA.md; a 1.0 needs the key elsewhere or the disclosure as the accepted risk, decided by Daniel |
| `/replay/status` and `/replay/metrics` unauthenticated without `--token` | accepted and disclosed in the README |
| Entropy heuristic false negatives | unmeasured against real secret shapes; the reviewer is asked to measure |
| Known mutant gap `metrics-shutdown-drain-not-awaited` | listed, not frozen; needs goroutine-leak detection the suite lacks |
| OpenAI-compatible path | unmasked and labelled so |

## 11. Reproducible commands and environment

```sh
git clone https://github.com/RedRobotKK/Replay && cd Replay
git checkout 37c0d0a8116b88b8fe7bbf6b6bf0630a2f20bec7   # or the merge commit
go version                      # go1.27.2 is selected by go.mod
go vet ./... && go test -race -count=1 ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...
go test -tags mutation -timeout 90m -count=1 ./internal/mutation/   # about 66 minutes
go test -tags mutation -timeout 30m -count=1 ./internal/blackbox/
make build && scripts/release-check.sh bin/replay
./scripts/reproduce-release.sh v0.8.0 linux amd64            # needs gh
```

Environment: macOS or Linux, Go 1.27.2 (downloaded by the go command
from `go.mod`), `gh` for the reproduction script, no credentials of any
kind, no network needed except for govulncheck's database and the
reproduction download.

## 12. Internal findings versus unperformed external work

| Kind | Exists | Where |
|---|---|---|
| Internally authored security analysis | yes | [security-review-2026-09-04.md](security-review-2026-09-04.md), written by the repository's own committer, with the 2026-10-08 provenance correction |
| Automated security tooling | yes | govulncheck, golangci-lint, guard reachability, in CI on every push |
| Internal adversarial testing | yes | the tests in section 9, the frozen mutant catalogue, the 2026-09-02 design review (the maintainer's own red-team pass) |
| Independent external review | **no** | nothing commissioned, scheduled, or contacted; acceptance criteria in security-review-scope section 6 |

## 13. What Daniel must do to move C-1

1. Choose a candidate from phase 2 section 5.2 (ADA Logics; Radically
   Open Security) or name another, and authorize sending the
   request-for-quote in phase 2 section 5.3 with this packet and the scope
   document attached.
2. On receipt of a written quote, authorize or decline the spend. The only
   sourced range is OSTIF's published $30,000 to $200,000 for audits; the
   phase 2 estimate for this scope is $25,000 to $60,000 and has no quote
   behind it.
3. On delivery, verify the five acceptance items, land fixes or dated
   accepted-risk decisions, and link the report from RELEASE-CRITERIA.md.

---

[Evidence index](README.md) · [Scope and brief](security-review-scope-2026-10-08.md) ·
[Closure ledger](closure-ledger-1.0-2026-10-09.md)
