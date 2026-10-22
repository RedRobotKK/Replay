# External security review packet, 2026-10-20

BOUND-TO-COMMIT: d96a95c6c5d1e04fcea89b48e2e9cbff1da56fe7

**This packet is not a review, and no review has occurred.** No external reviewer has been
contacted, engaged, scheduled or paid. Every security statement here is the maintainer's own or a
tool's output, labelled as such. The packet exists so that a reviewer, once Daniel commissions one,
can start from one commit with the scope, the network surface and the verification steps already
written down.

**What it supersedes.** [security-review-packet-1.0-2026-10-09.md](security-review-packet-1.0-2026-10-09.md)
was bound to `37c0d0a`, nineteen commits behind `main` when this file was written (`fe4369d`), and
its scope (inherited from [security-review-scope-2026-10-08.md](security-review-scope-2026-10-08.md)
section 3) omitted three things: the x402 path on `rules --update`, the release pipeline
(`release.yml`, `.goreleaser.yaml`, `publish-shims.yml`), and an explicit statement of what leaves
the machine. It also carried two statements this file corrects:

- Its section 4 said the pool contribution path "sends a content-free calibration report" to
  replay.doctor `/api/contribute`. The binary sends nothing on that path: every `--contribute` flag
  writes a file, `cmd/replay/contribute.go` imports no network package, and `internal/observation`
  carries an import ban. Submitting the file is something the operator does.
- It relied on SECURITY.md, which until 2026-10-20 said Replay "sends nothing anywhere". That
  sentence is corrected in SECURITY.md and now held to the code by
  `internal/regression/security_outbound_test.go`.

The 2026-10-09 and 2026-10-08 files are left as written, with a dated pointer at the top of the
packet. The 2026-10-08 scope document's provenance correction (section 1) and its acceptance
criteria (section 6) still stand and are restated in section 10 below.

## 1. Scope

In priority order. Each item names the code a reviewer reads first.

| # | Item | Code | Why it is in scope |
|---|---|---|---|
| 1 | Self-update fetch, signature and tag verification | `internal/selfupdate/fetch.go`, `verify.go`, `cmd/replay/upgrade.go` | The one path where the compromise model is "the attacker chooses what runs": it downloads a binary, verifies it, executes it once, and replaces the running one |
| 2 | Installer | `install.sh` | Same model as item 1 for a first install, run as `curl ... \| sh`; checksum anchoring, cosign identity, https-only fetch flags, `--no-verify` bootstrap behaviour |
| 3 | Proxy listener and upstream leg | `internal/proxy` (`server.go`, `hostguard.go`, `passthrough.go`, `retry.go`, `uds.go`, `metrics_listener.go`) | Holds the operator's provider key in transit; loopback bind, Host / Origin / Sec-Fetch-Mode guards, `--token`, header stripping, `HTTPS_PROXY` inheritance, `http://` upstream accepted |
| 4 | Masking and the vault key boundary | `internal/masking` (`vault.go`), `internal/proxy/masking.go` | The only store holding secrets; AES-256-GCM with `.vault-key` beside the ciphertext, 24h TTL; `/v1/chat/completions` unmasked by design |
| 5 | The x402 path | `cmd/replay/rules.go` (`fetchRules`, `paymentRequiredError`), `internal/cachemodel/x402.go`, `cmd/replay/x402_test.go` | A remote server's 402 body is parsed and printed; the claim is "reports, never pays, cannot sign". Terminal-escape handling of hostile terms is in scope |
| 6 | Release supply chain | `.github/workflows/release.yml`, `.goreleaser.yaml`, `.github/workflows/publish-shims.yml`, `packaging/npm`, `packaging/pypi` | What a user's verification in items 1 and 2 ultimately trusts: the workflow identity, main-ancestry gate, pinned actions, signing, SBOMs, npm and PyPI provenance |
| 7 | What leaves the machine | section 4 below; `cmd/replay/outbound_drift_test.go`, `internal/regression/security_outbound_test.go` | Whether the published network statement (SECURITY.md) is complete and true. The reviewer is asked to find a sender this list misses |
| 8 | Ledger content-freedom | `internal/ledger`, `internal/ownerdir` | That no message content reaches disk and that HMAC'd fields are not reversible offline |

**Out of scope:** the hosted service of ADR-0028 (not built), Windows (refused by design), the
replay.doctor and redrobot.jp sites (other repositories), anything needing live OpenAI access, and a
compromised host (section 2).

## 2. Threat model summary

Replay is built to be safe for **one trusted operator on a trusted host**, running a loopback
listener. In scope: a hostile local process or browser page reaching the loopback listener (DNS
rebinding, cross-origin requests); a hostile or compromised network path or release origin during
`upgrade`, `install.sh` or `rules --update`; a hostile rules feed (including its 402 body); a
supply-chain compromise between the tagged commit and the published archive. Out of scope by
design and disclosed: a compromised host. Anyone who can read `~/.replay` can read the vault
within its TTL, because the key sits beside it.

## 3. Sensitive data boundaries and auth assumptions

| Data | Where it lives | Where it goes |
|---|---|---|
| Provider API key (`x-api-key` / `Authorization`) | the client's request; in memory in the proxy | forwarded to the upstream, never written by Replay. `probe --execute` reads `ANTHROPIC_API_KEY` from the environment, never a flag |
| Secrets inside prompts | in memory; with `--mask`, the real value in `~/.replay/vault` (encrypted, 24h TTL) | with `--mask` on `/v1/messages` or `/v1/responses`, a placeholder goes upstream; otherwise the value goes upstream as typed |
| Message content | never stored | forwarded upstream only |
| Ledger records | `~/.replay/ledger`, owner-only | nowhere |
| Contributor secret | `~/.replay/contributor-secret`, owner-only | nowhere; it derives a tag written into a file the operator may submit |
| `--token` value | flag or `REPLAY_TOKEN` | stripped before the upstream leg |

Auth assumptions: there is no account system. The listener trusts loopback plus the Host and
Origin guards; `/replay/status` and `/replay/metrics` are unauthenticated unless `--token` is set;
`/replay/healthz` is never token-gated (by design, so `doctor` can probe it). Release authenticity
rests on GitHub's OIDC issuer and the exact identity
`https://github.com/RedRobotKK/Replay/.github/workflows/release.yml@refs/tags/<tag>`.

## 4. Network surfaces

Derived from the code at `fe4369d` by grep of every non-test sender shape and every package
importing `net`, `net/http` or `net/url`. Held to SECURITY.md by `TestSEC1` to `TestSEC3`.

| # | Surface | Trigger | Destination | Data sent | Controls | Tests |
|---|---|---|---|---|---|---|
| 1 | Proxy upstream | operator runs `replay serve`; then every client request, with no per-request prompt | `--upstream` / `REPLAY_UPSTREAM`, default `https://api.anthropic.com`; `http://` accepted; `HTTPS_PROXY` honoured | the client's request including its API key, minus `x-replay-token` and `x-replay-tenant-id`; masked on `/v1/messages` and `/v1/responses` when `--mask` | loopback listen, Host/Origin guards, HTTP/1.1 upstream, caps, breaker | `TestPassthroughIsByteExactAndRecordsUsage`, `TestBrowserOriginAndTokenChecks`, `TestR1_ValidTenantStillResolvesAndIsStrippedOutbound`, `TestGV6610_TheUpstreamLegIsHTTP1OnlyByConstruction`, `TestRefusesNonLoopbackListen` |
| 2 | `rules --check-prices` | typed | `raw.githubusercontent.com` (LiteLLM table) | one GET | 30s, 16 MiB cap, nothing installed | none exercises the fetch; parser tests in `internal/cachemodel` |
| 3 | `rules --update <url>` and x402 | typed | any host named | one GET | https only, cleartext redirect refused, 1 MiB cap, validated before install, 402 reported and not paid | `TestRC1_APlainHTTPSourceIsNeverFetched`, `TestX402_RedirectToCleartextIsRefused`, `TestX402_InstallsNothing`, `TestX402_NoSigningCapability`, `TestX402_HostileTermsCannotPaintTheTerminal` |
| 4 | `probe --execute` | typed, plan printed, confirmation unless `--yes` | `$ANTHROPIC_BASE_URL` or `api.anthropic.com` | synthetic billable requests on the operator's key | refuses without key or confirmation | `TestC8_AnUnconfirmedExecuteSendsNothing`, `TestProbeVaryCLI_PlansWithoutExecuteAndSendsNothing` |
| 5 | `upgrade` | typed | `github.com/RedRobotKK/Replay/releases/...` | GETs, no identifier | tag syntax first, sha256, cosign exact identity when present, symlink member refused, staged binary run once | `TestSIG9_FetchRefusesATagThatIsNotAReleaseTagBeforeAnyRequest`, `TestFetchRefusesAChecksumMismatch`, `TestIP1_SignatureIdentityIsPinnedToTheReleaseWorkflowAtTheTag`, `TestSIG2_CosignPresentAndNoSignaturePublishedRefuses`, `TestX402_SelfUpdateExecIsNotArbitrary`, `TestE2E_Upgrade` |
| 6 | `doctor` / `tui` probe | typed | `$ANTHROPIC_BASE_URL/replay/healthz` or `/replay/status`, loopback only | GET, no credential | non-loopback refused before the request | `TestDP1_ANonLoopbackBaseIsNeverContacted`, `TestDP2_ALoopbackProxyIsStillFound`, `TestDP3_WhatCountsAsLoopback` |
| 7 | `burn` Ollama probe | every `replay burn` | `127.0.0.1:11434/api/version` | GET | hard-coded loopback, 2s | no dedicated test |
| 8 | `internal/feed` | unwired: no caller | the base passed in | GET bundle and `.sig` | vendor host allow-list, ed25519 verify, never signs | `TestFeedVerifiesAndNeverSigns` |

Not outbound, checked: `--contribute` (writes a file), the plugin SessionEnd hook (executes no
request since the Watch send path was removed; `TestFCWK`, `TestFCWK2`), `mcp` (stdio only),
`internal/proxy/uds.go` (dials the operator's own socket path). Outside the binary: `install.sh`
(`api.github.com`, `github.com`, https-only flags), the npm launcher (fetches only when its
platform package is missing and `REPLAY_DOCTOR_ALLOW_FETCH=1`, checksum only, no signature check).

Fully automatic requests to another host: none found. The reviewer is asked to find one.

## 5. Known limitations

| Item | State |
|---|---|
| Vault key beside ciphertext (2026-09-04 finding 3) | open; TTL bounds the window; Daniel decides key move or accepted risk |
| `/replay/status`, `/replay/metrics` unauthenticated without `--token` | accepted, disclosed |
| `http://` upstream accepted by `serve` | the key then travels in cleartext, and to any `HTTP_PROXY`; disclosed in docs/SURFACES.md, not refused |
| `/v1/chat/completions` unmasked | by design, labelled at runtime |
| npm launcher fallback verifies checksums only | no signature check in the launcher |
| cosign's own network use during `upgrade` and `install.sh` | not measured; cosign defaults consult the Sigstore transparency log |
| GoReleaser pinned to `~> v2`, not an exact version | residual supply-chain drift |
| Entropy heuristic false-negative rate | unmeasured on real secret shapes |
| `rules --check-prices` fetch and `burn` probe | no test exercises the request itself |
| `cmd/replay/outbound_drift_test.go` | lists `internal/ledger` as a sender package although it imports no network package; the stale check does not cover that map |

## 6. Reproducible verification steps

Run at the bound commit, macOS or Linux, Go selected by `go.mod` (`toolchain go1.27.2`), no
credentials. Expected results are what the maintainer observed at `fe4369d` plus this change; a
reviewer is asked to re-run, not to trust them.

```sh
git clone https://github.com/RedRobotKK/Replay && cd Replay
git checkout d96a95c6c5d1e04fcea89b48e2e9cbff1da56fe7
go vet ./...                                                   # expect: no output, exit 0
go test -count=1 -run 'TestSEC' -v ./internal/regression/       # expect: SEC1, SEC2, SEC3 PASS
go test -count=1 -run 'TestOutboundSurfacesAreAllDocumented|TestX402_|TestRC1_|TestDP[123]_|TestC8_' ./cmd/replay/
                                                               # expect: ok
go test -count=1 ./internal/selfupdate/ ./internal/proxy/ ./internal/masking/ ./internal/feed/
                                                               # expect: ok for each
go test -race -count=1 ./...                                   # expect: ok for every package
grep -rnE 'http\.(NewRequest|DefaultClient|Client\{)|httputil\.ReverseProxy|net\.Dial|exec\.Command' \
  --include='*.go' cmd internal | grep -v _test.go             # expect: only files listed in section 4
sh scripts/installer-telemetry/check.sh                        # at fe4369d: T1, T3, T4 ok; T2 FAILS (see below)
```

`scripts/installer-telemetry/check.sh` T2 fails at `fe4369d` on `replay.doctor`. The hits are the
one-liner printed in the help text and the Windows refusal (install.sh:72, :144), which no command
fetches; the check's allow-list still names `redrobot.jp`, the host before the move. This is a defect
in the check, recorded here and not fixed in this change. A reviewer should confirm by reading
install.sh that no fetch goes to `replay.doctor`.

Mutation check for the network statement: put the sentence "Replay runs on your machine and sends
nothing anywhere." back into SECURITY.md and re-run `TestSEC3`; expect FAIL naming SECURITY.md.
Remove a file from one entry of `outboundSurfaces` in `security_outbound_test.go`; expect `TestSEC1`
FAIL naming that file.

Release verification for a published tag `vX.Y.Z` (needs network and cosign):

```sh
cosign verify-blob --certificate checksums.txt.pem --signature checksums.txt.sig \
  --certificate-identity https://github.com/RedRobotKK/Replay/.github/workflows/release.yml@refs/tags/vX.Y.Z \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com checksums.txt   # expect: Verified OK
sha256sum --check --ignore-missing checksums.txt                                         # expect: OK
./scripts/reproduce-release.sh vX.Y.Z linux amd64                                        # expect: byte-identical
```

## 7. External-reviewer handoff

**Required expertise.** Go network services and `net/http` / `httputil.ReverseProxy` internals;
local-service attack classes (DNS rebinding, CSRF against loopback, socket permissions); secure
update design (TUF-style threats, Sigstore keyless signing, GitHub Actions OIDC); shell installer
review; CI supply chain (GitHub Actions, GoReleaser, npm and PyPI provenance). Applied cryptography
at the level of reviewing AES-GCM use and HMAC keying, not designing primitives.

**Questions to answer.**

1. Can a party other than this repository's release workflow at the requested tag get a binary
   executed through `replay upgrade` or `install.sh`, with or without cosign installed?
2. Can a local process or web page cause the proxy to forward a request, read `/replay/status`, or
   learn the provider key, beyond what section 3 states?
3. Does any path send data off the machine that section 4 omits, including through `HTTPS_PROXY`,
   redirects, retries, or a subprocess?
4. Can a hostile rules feed or 402 body install rules, alter the terminal, or cause any payment or
   signing action?
5. Can a secret reach the ledger, a log, or the upstream unmasked on `/v1/messages` or `/v1/responses` with `--mask`
   set?
6. Do `release.yml`, `.goreleaser.yaml` and `publish-shims.yml` let a non-main commit, a moved tag,
   or a third-party action change what is signed or published?

**Artifacts provided.** This file; SECURITY.md; docs/SURFACES.md;
[security-review-scope-2026-10-08.md](security-review-scope-2026-10-08.md) (provenance correction
and brief); [security-review-2026-09-04.md](security-review-2026-09-04.md) (the maintainer's own
analysis, not external evidence); the repository at the bound commit; published release assets for
the most recent tag.

**Acceptance criteria for "published".** The RELEASE-CRITERIA.md external-review gate closes only
when all of these hold (restated from the 2026-10-08 scope document, section 6):

1. A report at a stable URL, in the reviewer's or firm's own name, not paraphrased into this
   repository's voice.
2. The reviewer is identifiable and is not Daniel Saito, RedRobot K.K., or an account either
   controls.
3. The report states what was tested, not merely read, and states the commit it tested, which
   matches `BOUND-TO-COMMIT` above or names the difference.
4. Each finding has a severity and either a fix with a commit reference or a dated accepted-risk
   decision.
5. RELEASE-CRITERIA.md links the report, with the prior "Not commissioned" status kept visible.

Until then this packet is preparation, and the gate stays open.

---

[Evidence index](README.md) · [Scope and brief](security-review-scope-2026-10-08.md) ·
[Previous packet](security-review-packet-1.0-2026-10-09.md)
