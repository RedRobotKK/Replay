# The v1.0 release readiness gate, 2026-10-09: NO-GO, and exactly why

**Candidate `df8e437`, CI run 37889625915, read job by job. Verdict NO-GO.
Nothing merged, tagged, published, deployed or announced. The Go toolchain
bump is prepared and evidenced here and was not executed.** The full package
(nine deliverables, including the twelve-seat simulated debate and the risk
register) is published as
[REPLAY-REL-03](https://claude.ai/artifact/LBqG5oSnbtCiM8j7RuUjVV); this file
keeps the repository-side facts where the next reader of
[RELEASE-CRITERIA.md](../../RELEASE-CRITERIA.md) will find them.

## 1. Baseline, re-verified rather than trusted

| Item | Verified |
|---|---|
| HEAD at gate start | `df8e43705753905495ad4c5fd92c3f786e53e46f`, equal to `origin/fix/compaction-observed-vs-inferred` |
| origin/main, tags | `5c87562` (v0.8.0), 16 tags v0.1.0 to v0.8.0, unchanged |
| Stash | 3 pre-existing entries, untouched |
| Protected TTL evidence | `ttl-register-2026-10-05.md` hashes to its `.sha256`; last commits touching the five files predate this work; `~/replay-experiment/` never written |
| CI run 37889625915 | 14 success: four Go legs (the latest-release leg on go1.27.2 with `-race`), frozen mutants (`ok internal/mutation 3927s`, 144/144; `ok internal/blackbox`), guard reachability (`242 guard(s), 5 survived (1 pre-existing, 4 introduced), 4 evidenced, 0 unexplained`), golangci-lint 0, cli blueprint, x402, ledger bench, installer drift, tui layout, surface drift, merge result. 2 failure: govulncheck (9 reachable stdlib advisories), Markdown lint (239 findings, all mechanical) |
| Local on df8e437 | `go build`, `go vet`, `gofmt -l` clean; `go test -count=1 ./...` 40 packages ok; golangci-lint 0; CLAIM-REGISTER.md byte-identical to its generator |

## 2. Why NO-GO

The project's own bar, unchanged since 2026-09-13: "Every gate below is
closed, and this is still not a 1.0." Of the three items that must land first,
the external security review is not commissioned (no engagement, no reviewer,
no budget authorized) and the second provider has rules and dispatch but no
calibration corpus. `docs/ROADMAP.md`'s definition, claims "checked by somebody
other than the person who wrote them", is not met. Independently, the
candidate's required checks are red on govulncheck and Markdown lint, and the
Release workflow refuses a tag that is not on main.

## 3. The govulncheck findings, dispositioned

Thirteen advisories, all published 2026-10-08, all "Fixed in go1.26.9" (and
go1.27.2); none has or will have a 1.25 fix, because Go 1.25 left upstream
support when 1.27.0 shipped. Two facts about this binary settle most rows:
the proxy's upstream `http.Transport` sets a custom `DialContext` and not
`ForceAttemptHTTP2` (`internal/proxy/server.go:215`), so the provider leg is
HTTP/1.1 only; the listener is plaintext HTTP/1.1 with no TLS and no h2c, so
there is no HTTP/2 or TLS server in the shipped binary.

| Advisory | Flaw | Disposition |
|---|---|---|
| GO-2026-6617, 6612, 6603 | HTTP/2 server crash, flow-control refund, trailer memory | Not applicable: no HTTP/2 server |
| GO-2026-6613 | 2xx to HTTP/1 CONNECT keeps reading | Not applicable, measured: the mux answers 404 (authority form) or 502 (path form); nothing reaches the upstream |
| GO-2026-6605 | Transport sends CONNECT with body, pooled-connection desync | Not applicable, measured: inbound CONNECT never forwarded; the only CONNECT the Transport sends is the HTTPS_PROXY tunnel, bodiless |
| GO-2026-6610 | HTTP/2 upstream to HTTP/1 client response smuggling in a reverse proxy | Not applicable: the upstream leg is HTTP/1.1; becomes applicable if `ForceAttemptHTTP2` is ever set (a pinning test is recommended) |
| GO-2026-6607 | crypto/tls ECH server memory exhaustion | Not applicable: no TLS server |
| GO-2026-6611 | HTTP/2 client and server CPU via SETTINGS | Applicable but mitigated: only the default-transport typed commands (`upgrade`, `rules --check-prices`, `probe --execute`) speak HTTP/2, to TLS peers the operator chose |
| GO-2026-6608 | textproto ReadContinuedLine memory-limit bypass | Applicable but mitigated: reached through response trailer parsing from the configured provider |
| GO-2026-6609, 6604, 6600, 6599 | ServeFile ranges, os.Root on Windows, html/template | Not applicable: symbols unreached; no such use in shipped code |

Zero confirmed material exposures. The material risk is the toolchain: a
pinned major that cannot receive a fix, behind a gate this repository made
blocking on purpose.

The measurement: a built binary with an isolated `HOME` and ledger, a fake
upstream that logs every byte, and two raw requests.

```text
CONNECT example.com:443 HTTP/1.1 + Content-Length: 5 + body   ->  404 page not found, upstream received nothing
CONNECT /v1/messages HTTP/1.1  + Content-Length: 5 + body      ->  502 upstream request failed: context canceled, upstream received nothing
```

## 4. The toolchain option, qualified and not executed

Without changing `go.mod` (`git status` stayed empty):

| Toolchain | build | vet | `go test -count=1 ./...` | govulncheck |
|---|---|---|---|---|
| go1.25.13 (pinned) | ok | ok | 40 ok (and CI) | 9 reachable, exit 1 |
| go1.26.9 | ok | ok | 40 ok | No vulnerabilities found |
| go1.27.2 | CI job 113687364668: `go test -race -count=1 ./...` green | | | No vulnerabilities found |

Recommendation to Daniel: `toolchain go1.27.2` (minimum `go1.26.9`), as one
line in `go.mod` plus its comment, landed as its own PR whose 16 jobs are the
qualification matrix; frozen mutants, guard reachability and the golangci-lint
v2.5 action have not yet run under 1.27 and must. Rollback is one revert; past
tags are immutable. **Not executed; awaiting explicit approval.**

## 5. Public surfaces, re-fetched 2026-10-09

- `replay.doctor` still carries the $22,000 week ($18,000 to $25,000) and the
  "Talk to Daniel" call to action that [ADR-0028](../adr/0028-replay-is-a-product-with-a-hosted-service.md)
  withdrew, and the $16,078.68 / $631.77 / 123 sessions / 2,176 lanes / 106.7M
  tokens / 2026-09-21 figure set that no evidence file carries. The prepared
  replacement copy is in [gtm-02-public-surface-retrofit-2026-10-08.md](gtm-02-public-surface-retrofit-2026-10-08.md).
- `redrobot.jp/replay` and `redrobot.jp/replay-doctor/` carry the same figure
  set; the first also says "v0.6.2 is tagged" and links this project's
  self-authored review as "Security review".
- **New: the `/replay.sh` installer path on redrobot.jp serves `install.sh` as of commit `b91018f`
  (2026-09-10).** `replay.doctor/replay.sh` serves the current file (sha256
  `74a13afd...`, identical to HEAD, main and v0.8.0); the redrobot.jp copy
  (`4b04bd28...`) differs in 24 lines: the domain strings and the Windows
  message this repository later replaced because it offered an archive that
  does not exist. The checksum and cosign sections are identical.
- Both names are live and are one product; `README.md`, `plugin.json` and
  `SKILL.md` already say so. GitHub About, `CITATION.cff` and
  `marketplace.json` do not yet. Recommendation unchanged from
  [naming-comparison-2026-10-08.md](naming-comparison-2026-10-08.md): keep the
  split and state it; the decision is Daniel's.

None of these surfaces is deployed from this repository.

## 6. What this pass changed, test first

- The README's govulncheck badge read "no known vulnerabilities" on a tree
  whose govulncheck job was red. It now names the check ("blocking in CI");
  `TestFCGV2_TheGovulncheckBadgeClaimsTheCheckNotTheResult` went red on the
  old text and green on the new.
- [security-review-2026-09-04.md](security-review-2026-09-04.md) now carries a
  dated provenance correction above its "An external reviewer" sentence, which
  is left standing; `TestDC5_TheSelfAuthoredReviewCarriesItsProvenanceCorrection`
  pins it.
- The duplicate `### Added` heading under Unreleased is merged (markdownlint
  MD024 at CHANGELOG.md:183 in the CI log; clean after).

Verified after the edits: `go test ./internal/regression -count=1` ok,
`gofmt -l .` empty, `go vet` ok, `golangci-lint run ./...` 0 issues,
markdownlint clean on every touched file. The Markdown job is still red
(237 findings locally after this pass); every remaining finding is mechanical
and sits in evidence, research or generated files, and is dispositioned in
the package rather than swept here.

## 7. Decisions that are Daniel's

1. Approve the toolchain version and let the bump land.
2. Commission the external review from the prepared brief in
   [security-review-scope-2026-10-08.md](security-review-scope-2026-10-08.md),
   or amend RELEASE-CRITERIA.md by a dated record. The gate follows the
   criteria as written.
3. Publish the prepared site copy; redirect or refresh the `/replay.sh` installer path on redrobot.jp.
4. Choose the Markdown policy: a reviewed whitespace-and-token sweep, or a
   config scope that keeps the rules on reader-facing documents.
5. Naming: state the split on the three remaining surfaces, or schedule a
   rename for after 1.0 under ADR-0024.
6. Accept ADR-0024 and name the compatibility surfaces before the tag.
7. Decide whether two hand-verified reproductions plus one on the 1.0
   candidate tag satisfy the reproducibility gate, or wire
   `scripts/reproduce-release.sh` into CI.
8. Fund the second-provider calibration, or amend that gate by dated record.

---

[Evidence index](README.md) · [Release criteria](../../RELEASE-CRITERIA.md) ·
[REPLAY-REL-03 package](https://claude.ai/artifact/LBqG5oSnbtCiM8j7RuUjVV)
