# The v1.0 closure ledger, 2026-10-09: every condition, every gate, one SHA

**What this answers.** Whether candidate `37c0d0a8116b88b8fe7bbf6b6bf0630a2f20bec7`
(branch `fix/compaction-observed-vs-inferred`, PR #336) can be recommended
for a v1.0 release, judged against the project's own written criteria, with
every gate tied to the exact commit its evidence was produced on. It
reconciles the two earlier records of the same day
([closure-council-1.0-2026-10-09.md](closure-council-1.0-2026-10-09.md),
[closure-phase2-2026-10-09.md](closure-phase2-2026-10-09.md)) against the
repository and the live production surfaces as they stood at 19:15 to
20:00 UTC, rather than repeating them.

**Verdict, stated first: NO-GO for v1.0.** Every gate this environment can
run is green on the candidate. The three conditions that remain are the
ones the 1.0 definition in [docs/ROADMAP.md](../ROADMAP.md) puts outside the
maintainer's own hands: an external security review that nobody has
commissioned (C-1), a second-provider calibration corpus that does not
exist and an amendment that has not been written (C-2), and a candidate
that is not yet on `main` (C-5), which only Daniel merges. Nothing in this
pass was merged, tagged, published, deployed, contacted or paid for.

## 1. Source of truth, re-verified at 19:40 UTC

| Item | Value |
|---|---|
| Repository | `git@github.com:RedRobotKK/Replay.git` |
| Branch | `fix/compaction-observed-vs-inferred` |
| HEAD at ledger start | `37c0d0a8116b88b8fe7bbf6b6bf0630a2f20bec7`, equal to `origin/fix/compaction-observed-vs-inferred` |
| `origin/main` | `5c8756260ca0dcf28ee566be1c11172147338c2e`, which is the commit `v0.8.0` resolves to (tag object `1e63477`) |
| Merge base | `5c87562`; the branch is 63 commits ahead, 0 behind |
| Working tree | clean, 0 modified, 0 untracked, before this pass's commits |
| Stash | 3 pre-existing entries, untouched |
| Tags | 16, `v0.1.0` to `v0.8.0`, unchanged |
| PR | #336, OPEN, draft, base `main`, mergeable, head `37c0d0a` |
| `go.mod` | `go 1.24`, `toolchain go1.27.2` |
| Ruleset 23126317 on `main` | required checks: Go (ubuntu-latest), Go (macos-latest), golangci-lint, guard reachability, frozen mutants, merge result; required signatures; linear history; no deletion, no force push. Tags are not covered by any ruleset; their immutability is policy, not enforcement |

### 1.1 Correction to the mission brief: the toolchain baseline is go1.27.2

The brief named go1.25.13 as the approved baseline. That line was true
until 2026-10-09. Daniel authorized the upgrade that day (phase 2 record,
section 2); it landed as `bbee9f6` with the rationale in `go.mod` and
`CHANGELOG.md` (thirteen advisories GO-2026-6599 to GO-2026-6617, fixed
only in go1.26.9 and go1.27.2, nine of them reachable from this module, and
go1.25 out of upstream support), and `8d79db9` moved the two CI tool pins
that could not read go1.27.2 (govulncheck to v1.8.0, golangci-lint to v2.14).
It was qualified on CI run 37954240783 (`8d79db9`) and run 37961162745
(`37c0d0a`), both 16 of 16 success. This pass changed the toolchain in
neither direction.

Every job in `ci.yml` and `release.yml` that builds the product reads the
compiler from `go.mod` (`go-version-file: go.mod`); the mutants job log
prints `go version go1.27.2 linux/amd64`. One job deliberately selects
another toolchain: `Go (latest release)` uses `go-version: stable` and is
documented in the workflow as a canary for standard-library changes. It is
not silent and it does not build a shipped artifact.

## 2. Frozen mutants: the 135 and the 144 are two versions of one catalogue

| Commit | Date | Entries | Highest id | Known survivors |
|---|---|---|---|---|
| `580ecc6` | 2026-10-04 | 125 | M126 | 1 |
| `3903712`, `6bcbca1` | 2026-10-04 | 135 | M136 | 1 |
| `b29c925` | 2026-10-05 | 136 | M137 | 1 |
| `e9da6a8` | 2026-10-05 | 144 | M145 | 1 |
| `26a28c4`, `0320747`, `37c0d0a` | 2026-10-07 to 09 | 144 | M145 | 1 |

The 135-mutant run on `6bcbca1` (CI run 37255188761) was the catalogue of
its day. The nine entries M137 to M145 were frozen by eight commits on
2026-10-05 (`b29c925`, `a9d9906`, `85f4259`, `7843002`, `3020822`,
`db64ba9`, `1270ae9`, `e9da6a8`), each a fix commit that froze the defect it
fixed, and `0320747` re-anchored M70 on 2026-10-08 without changing the
count. The id M71 is absent by retirement, recorded in README.md line 567,
`internal/regression/mutation_population_test.go` and the sweep record of
2026-10-08. No entry was added or removed to reconcile anything.

**Catalogue identity on the candidate:** `internal/mutation/testdata/mutants.json`,
sha256 `bba10c8c5ff026f92eb3f5970a888020f19617b3dfb4a8bbb32e5bca26080dac`,
schema `replay.mutants.v1`, 144 mutants, one named known survivor
(`metrics-shutdown-drain-not-awaited`, listed as a gap and not counted).

**The run that applies to the candidate:** CI run 37961162745, job
`frozen mutants`, on `37c0d0a`, `go version go1.27.2 linux/amd64`,
`go test -tags mutation -timeout 90m -count=1 ./internal/mutation/` reported
`ok internal/mutation 3958.021s`, then `./internal/blackbox/` reported
`ok internal/blackbox 90.196s`; the job ran 16:43:30 to 17:51:20 UTC and
concluded success, inside the job's 120 minute limit. No `-short`, which is
the package's only skip path. The same job on `8d79db9` (run 37954240783)
reported `3171.258s` and `74.443s`.

**Why `ok` means killed and not merely finished.** `TestFrozenMutantsStillDie`
calls `t.Fatalf` on a mutant that does not compile (stillborn), `t.Errorf`
on one whose named killer passes (survived), and `t.Fatalf` if the baseline
suite is not green before any mutant is applied; `TestKillMatrix` is the
secondary permutation run; `TestTheStillbornCountIsWhatFailsTheRun` and
`TestTheClassifierTellsARefusalFromAFailure` prove the harness tells a
compiler refusal from a test failure. A timeout would have failed the job.

**Local confirmation on this host, bounded on purpose:** the six newest
entries, `go test -tags mutation -count=1 -run 'TestFrozenMutantsStillDie/^M14[0-5]_' ./internal/mutation/`,
exit 0, `144 catalogued, 6 mutants: 6 killed, 0 survived, 0 stillborn`,
104.5s; `go test -tags mutation -run Responses ./internal/blackbox/` ok.
The full catalogue was not re-run locally: the host had 1.4 GiB of swap
free and 12 GiB of disk, which is the same shape of host that killed the
2026-10-08 local attempt, and the documented CI workflow had already
produced the result on this exact SHA.

Counts for the handback: killed 144, survived 0, stillborn 0, timed out 0,
skipped 0, known gap 1 (not in the 144).

## 3. Gate matrix on `37c0d0a`

CI run 37961162745 (pull_request on #336, merge ref `e60c54a` of `37c0d0a`
into `5c87562`), 16 jobs, all success, started 16:43:30 UTC:

| Gate | Result | Evidence line | Duration |
|---|---|---|---|
| Go (ubuntu-latest) | success | `coverage: 89.2%  floor: 85%  headroom: +4.2`; go.mod tidy; vet | 3m42s |
| Go (macos-latest) | success | race suite | 2m46s |
| Go (windows-latest) | success | the refusal path, by design | 3m07s |
| Go (latest release) | success | `go test -race` on `stable` | 2m58s |
| golangci-lint v2.14 | success | `0 issues.` | 45s |
| govulncheck v1.8.0 | success | `No vulnerabilities found.` | 26s |
| guard reachability | success | `242 guard(s), 5 survived (1 pre-existing, 4 introduced), 4 evidenced, 0 unexplained, 0 unchecked, 0 not built here` | 13m59s |
| frozen mutants | success | section 2 | 67m50s |
| merge result | success | the merge compiles, not only the branch | 18s |
| Markdown lint | success | `Linting: 237 files`, `Summary: 0 issues in 0 files` | 25s |
| surface drift | success | job green | 32s |
| installer drift | success | job green | 5s |
| cli blueprint | success | job green | 31s |
| x402 end to end | success | `x402-e2e: all assertions passed` | 51s |
| ledger bench | success | `ledger-bench: all expectations hold.` | 7s |
| tui layout audit | success | job green | 27s |

Local, same SHA, this host (darwin/arm64, go1.27.2), 19:43 to 19:49 UTC,
log at the session scratch `closure3/gates-local-1.log` and `gates-local-2.log`:

| Check | Exit | Output |
|---|---|---|
| `gofmt -l .` | 0 | nothing listed |
| `go vet ./...` | 0 | |
| `go build ./...` | 0 | |
| `go mod tidy -diff` | 0 | no diff; no `go.sum`, no `require` block |
| `go test -race -count=1 -coverprofile ./...` | 0 | all packages ok; total 89.2% |
| `scripts/coverage-gate.sh` | 0 | 89.2% against the 85% floor |
| `go run scripts/claim-register/main.go` | 0 | `CLAIM-REGISTER.md` byte-identical to the committed file |
| `make build` then `scripts/release-check.sh bin/replay` | 0 | `replay v0.8.0-63-g37c0d0a`; every assertion `release-check: ok` |
| `scripts/installer-drift/check.sh` | 0 | both hosted copies match the repository `install.sh` (`74a13afd523d`, 556 lines) |
| golangci-lint (local v2.13.2) | 1 | one `typecheck` issue: the local tool cannot read go1.27.2's `internal/abi`. Tool older than the toolchain, the exact condition `8d79db9` fixed in CI by pinning v2.14; CI reports 0 issues. Not a defect in the tree |

No gate was invalidated by this pass: the commits below touch
`docs/evidence/*.md` and one Go source comment, and the CI run on the new
head is recorded in the handback once it reaches a terminal state.

## 4. Closure ledger

Columns: status is one of PASS, FAIL, BLOCKED, NOT RUN, STALE; owner is
engineering, internal review, external reviewer, external provider, or
Daniel; the SHA is the commit the evidence was produced on.

| ID | Requirement and source | Status | Severity | Evidence, SHA | Current | Owner | Next action | Completion criterion |
|---|---|---|---|---|---|---|---|---|
| C-1 | External security review, published; RELEASE-CRITERIA.md "Where this stands today", ROADMAP 1.0 definition | BLOCKED | release-blocking | [security-review-packet-1.0-2026-10-09.md](security-review-packet-1.0-2026-10-09.md), `37c0d0a`; no reviewer engaged, nothing sent | yes | Daniel, then external reviewer | choose a candidate, authorize contact and spend | the five items in security-review-scope section 6 exist and RELEASE-CRITERIA.md links the report |
| C-2 | Second-provider rules and a calibration corpus against real traffic; RELEASE-CRITERIA.md second-provider row | BLOCKED | release-blocking | [second-provider-calibration-runbook-2026-10-09.md](second-provider-calibration-runbook-2026-10-09.md); rules exist and are dispatched (`AstraRules`, `RulesForModel`); fixture pipeline validated on `37c0d0a` (136 tests, 6 packages, exit 0); no corpus; no credentials | yes | Daniel (A or B), then external provider | choose option A (funded corpus on another machine) or B (dated amendment) | A: the runbook's minimum evidence; B: the amendment and the narrowed claim on every surface that promises a second provider |
| C-3 | Reproducibility verified on the 1.0 tag; RELEASE-CRITERIA.md reproducible-releases row | NOT RUN | release-blocking after the tag | v0.8.0 reproduces on four platforms, go1.25.13 ([reproducible-build-2026-10-08.md](reproducible-build-2026-10-08.md)); the 1.0 tag does not exist | n/a | engineering, after Daniel tags | run `scripts/reproduce-release.sh v1.0.0 <goos> <goarch>` four times after the release workflow completes | four `REPRODUCES` lines on the 1.0 tag, built by go1.27.2 |
| C-4 | govulncheck green on the candidate | PASS | | run 37961162745, `No vulnerabilities found.`, `37c0d0a` | yes | | none | met |
| C-5 | Candidate on `main` with all required checks green | BLOCKED | release-blocking | `origin/main` = `5c87562`; PR #336 draft, mergeable; the six required checks are green on `37c0d0a` | yes | Daniel | mark ready and merge | `origin/main` equals the merge commit and the release workflow's ancestry gate accepts the tag |
| C-6 | Withdrawn forensic week absent from every public surface; ADR-0028 | PASS | | replay.doctor main `9da68d6` (Pages deployments 87a62b13 at 18:07 and 16529b59 at 19:13 UTC), redrobot-jp main `94a84b0` (aa88b7e4 at 18:19 UTC); fetched again 19:40 UTC: the only remaining occurrences are the withdrawal notice itself on the landing page and `llms-full.txt` ("was withdrawn on 2026-10-05 by ADR-0028; none was sold") and a corpus figure "re-billed $199.77" on redrobot.jp `llms.txt`, which is a measurement, not a price | yes | | none | met in production |
| C-7 | No public surface claims an external review happened | PASS | | same fetches: "adversarial reviewer" absent from every public page and llms file; the pinned docs index at `5c87562` is covered by B-20 | yes | | none | met in production, with B-20 as the residue |
| C-8 | Compatibility surfaces named before 1.0 | PASS | | `db0781e`, ROADMAP v1.x list; ADR-0024 carries the same list, Status: Proposed | yes | Daniel for ADR-0024 | accept or amend ADR-0024 | ROADMAP carries the list (met); ADR-0024 status is B-09 |
| B-01 | govulncheck red on nine reachable advisories | PASS | | `bbee9f6`, `8d79db9`; runs 3 and 4 | yes | | none | met |
| B-02 | Markdown lint red | PASS | | run 4 `0 issues` | yes | | none | met |
| B-03 | External review not commissioned | BLOCKED | release-blocking | same as C-1 | yes | Daniel | same as C-1 | same as C-1 |
| B-04 | Second provider, rules without corpus | BLOCKED | release-blocking | same as C-2 | yes | Daniel | same as C-2 | same as C-2 |
| B-05 | Reproducibility on the 1.0 tag | NOT RUN | after the tag | same as C-3 | n/a | engineering | same as C-3 | same as C-3 |
| B-06 | Candidate not on `main` | BLOCKED | release-blocking | same as C-5 | yes | Daniel | merge | same as C-5 |
| B-07 | Withdrawn offer live at three locations | PASS | | C-6 deployments | yes | | none | met |
| B-08 | Public independent-review claim | PASS | | C-7 fetches | yes | | none | met |
| B-09 | ADR-0024 still Proposed | BLOCKED | not release-blocking by the criteria; the ADR's own text says the policy must exist before 1.0 | `docs/adr/0024`, Status: Proposed | yes | Daniel | accept, or amend the list | Status: Accepted |
| B-10 | Installer parity on redrobot.jp | PASS | | both hosted paths and replay.doctor `/replay.sh` hash `74a13afd523dca3efffea203d18bc02c22e7a9d1233a64d8ee7219ffbff4d8a8`, fetched 19:40 UTC; `installer-drift` exit 0 locally and in CI | yes | | none | met |
| B-11 | Stale figures on the sites | PASS | | live pages name v0.8.0 as the release; `/release-0-8-0/` is 200 | yes | | moves again at the next release | met for 0.8.0 |
| B-12 | Naming note on three surfaces | PASS, decision open | | `1ace6f6`; GitHub About (G-8) untouched | yes | Daniel | state the name; set About | Daniel's choice recorded |
| B-13 | HTTP/2 disposition | PASS | | test in tree, green under go1.27.2 | yes | | none | met |
| B-14 | Generator artefacts | PASS | | lint 0 | yes | | none | met |
| B-15 | Tool pins unreadable under go1.27.2 | PASS | | `8d79db9`; runs 3 and 4 | yes | | none | met |
| B-16 | `vendor-corpus.mjs` crash (redrobot-jp) | PASS | | deployed in `94a84b0` | yes | | none | met |
| B-17 | HubSpot deal builder priced the week | PASS | | removed on the site branch `7ea203c`, merged in #16 `3be1508`, deployed | yes | | none | met; Daniel's read of the removal stands as his to revisit |
| B-18 | replay.doctor pinned to 0.6.2 | PASS | | re-pinned to `5c87562` in #16 | yes | | none | met; superseded by B-20 for the next pin |
| B-19 | `docs/README.md` carried the C-7 claim | PASS | | `ef88b40`; run 4 | yes | | none | met in the tree; public copy is B-20 |
| B-20 | Six replay.doctor `/docs/` pages carry `5c87562` wording | STALE | not release-blocking; public-claim hygiene | section 5 below: the re-pin is verified in a scratch clone and cannot complete before the merge, because `vendor:installer` reads `main` | yes | Daniel (merge, then deploy) | after the merge: re-pin, re-vendor, test, deploy | the six pages render the merged wording; `npm test` 88 of 88 |
| B-21 | Site narrative named 0.6.2 as the release | PASS | | `092ce39` in #16, deployed | yes | | none | met |
| B-22 (new) | Re-pinning the site publishes 17 new evidence pages, among them the closure records that quote the withdrawn figure, the banned phrases as defects, and two uncontacted reviewer firms by name | BLOCKED on a decision | public-surface | section 5 | yes | Daniel | decide whether closure records are published on the site or excluded by the pull script | decision recorded and applied in the re-pin commit |
| B-24 (new) | `docs/evidence/README.md` carried 22 index rows above its own `# Evidence` heading, because one earlier edit joined two rows and the heading with literal `\n` characters; the live `/docs/trust/evidence/` page at `5c87562` renders those rows as pipe text, confirmed by fetch at 19:58 UTC | PASS in the tree | public-claim hygiene; not release-blocking | fixed in this pass with a regression test in `internal/regression`; the public copy clears with B-20 | yes | Daniel (re-pin and deploy) | same as B-20 | the live index renders one table |
| B-23 (new) | `CHANGELOG.md` Unreleased holds 22 entries; the cadence rule in RELEASE-CRITERIA.md says do not let it run past roughly twenty | PASS, cadence note | not a gate | count by `awk` on `37c0d0a` | yes | Daniel | cut a release | an entry count under the rule after the next tag |
| G-mut | Frozen mutants on the candidate | PASS | | section 2 | yes | | none | met |
| G-guard | Guard reachability | PASS | | run 4 line in section 3 | yes | | none | met |
| G-cov | Coverage floor | PASS | | 89.2% against 85% | yes | | none | met |
| G-repro | Build reproducibility machinery | PASS for v0.8.0, NOT RUN for 1.0 | | C-3 | | | | |
| G-claims | Claim register current | PASS | | regeneration byte-identical | yes | | none | met |
| G-surfaces | Release check against the built binary | PASS | | `release-check: ok` | yes | | none | met |

## 5. B-20 and the site re-pin, verified without deploying

In a scratch shared clone of `RedRobotKK/replay.doctor` at `main`
(`9da68d6`), with `REPLAY_REPO` pointed at this worktree:

1. `replay.ref` left at `5c87562`: `pull:docs` wrote 98 evidence pages,
   `npm run build` exit 0, six test files 88 of 88, and
   `check-release-surfaces` prints the same standing `external-redrobot-jp`
   NOT_MEASURED row it prints today. That row is pre-existing and not
   introduced by any pin.
2. `replay.ref` set to `37c0d0a` as a stand-in for the merge commit:
   `pull:docs` wrote 115 evidence pages and 27 decisions; build exit 0;
   `docs.test.mjs` and `prose.test.mjs` green; the six pages named in
   phase 2 section 13.4 lose the old wording (the docs index drops both
   "adversarial" sentences; the 2026-09-04 review page keeps its frozen
   title and gains the provenance correction, which is the intended
   shape). One test fails: "the vendored installer is the one the pinned
   release ships", because `scripts/vendor-installer.mjs` always vendors
   `main`'s HEAD and compares its commit with `replay.ref`. It can only
   pass once the Replay branch is on `main`.
3. The prepared patch (`replay.ref` and `vendor/replay-install.meta.json`)
   is in the session scratch as `b20-repin-replay.doctor.patch`; the
   procedure that applies to the real merge commit is in the release
   runbook, section 6.

What the re-pin would newly publish, counted against the pages already
live at `5c87562`: six pages carrying the figure "22,000" in withdrawal
or removal context (the changelog, the evidence index, and the four
2026-10-08 and 2026-10-09 closure records), four carrying the banned
"Talk to Daniel" phrase as a quoted defect, two carrying "adversarial
reviewer" the same way, and one (phase 2) naming ADA Logics and
Radically Open Security as uncontacted candidates. The repository is
public, so none of this is new information; the site's own
public-surface rules are the question, and that is B-22, Daniel's.

## 6. Protected boundaries

| Item | Before | After |
|---|---|---|
| `ttl-register-2026-10-05.md` | blob `762ecb91502643e1f7300c20d587c5c04511f207`; `shasum -a 256 -c` OK | unchanged, re-checked at the end of the pass |
| `ttl-register-2026-10-05.sha256` | blob `5dbf89d647313ad65193175dc8303307f40c3280` | unchanged |
| `ttl-schedule-2026-10-05.json` | blob `9588c22e79a9862f91aa5818311d0cd71ac1b6dd` | unchanged |
| `ttl-blocks-2026-10-05.jsonl` | blob `db1ed7a61d0860df072c11bcc40b1cb629924a36` | unchanged |
| `ttl-amendment-2-2026-10-07.md` | blob `b1a125d475274c6e73f25fecbfc7a98007497c0a` | unchanged |

All five blob ids equal their ids at `0ea7370`. `~/replay-experiment/` was
not opened. `internal/claims/claims.go` was not edited. `CLAIM-REGISTER.md`
was regenerated and found byte-identical. `.markdownlint-cli2.yaml` was not
edited. No provider credential exists in the environment (zero matching
variable names; `~/.config/replay` holds the pool contributor secret and
no provider key, and nothing there was read). No request was sent to any
paid API. Production configuration was not touched. No message was sent.

## 7. Panel of twelve perspectives on the verdict

An internal analytical exercise, not a review by real external people, and
not a substitute for C-1. Each line is the strongest evidence-based
conclusion from that seat; the resolution rule is that a release-blocking
objection from any seat holds unless evidence on this ledger refutes it.

| Seat | Conclusion |
|---|---|
| 1. Release engineering | The candidate is the strongest the project has had: 16 of 16 on the exact SHA, reproducibility machinery proven on the previous tag. It is a release-ready branch; it is not a 1.0 by the criteria. Merge review first |
| 2. Test and mutation quality | 144 of 144 on the SHA, harness semantics audited (stillborn fatal, survivor error, baseline guarded). The one named gap is recorded, not hidden. No objection |
| 3. Security engineering | Internal analysis, automated tooling (govulncheck, guard reachability) and internal adversarial tests exist and are green; none is an external review. Finding 3 (vault key beside ciphertext, TTL-bounded) stays open and disclosed. Blocking on C-1 only |
| 4. Independent-audit standards | The 1.0 definition is "checked by somebody other than the person who wrote them". No artefact in this repository can satisfy that by construction. NO-GO until a third party reports |
| 5. Provider calibration and experimental design | One provider calibrated (Anthropic, 97.79% match on 116 sessions); the second has rules, dispatch and fixtures, no corpus. Fixture validation proves the pipeline parses, not that the provider behaves. Option B is honest; option A needs another machine and spend |
| 6. Data provenance | Every figure on this ledger names its run id or log; the mutant count history is reconstructed from `git log -p`, not from reports. No objection |
| 7. Privacy | Ledger is content-free by design and tested; the site re-pin would publish closure records naming third parties. Flag B-22 before any deploy |
| 8. Supply-chain integrity | Zero third-party Go dependencies, pinned actions by SHA, cosign and syft pinned by version, keyless signing bound to the workflow. The local lint mismatch is a developer-machine condition, not a supply-chain one. No objection |
| 9. API compatibility | Surfaces are named (C-8). ADR-0024 is still Proposed; a deprecation promise that is not accepted before 1.0 is the thing the ADR says cannot be made after. Accept or amend before tagging |
| 10. Product claims and documentation | Production surfaces are clean of the withdrawn offer and the false review claim. Six site docs pages lag the tree until the re-pin. The Unreleased section is over the cadence rule. Ship a release to drain it; do not call it 1.0 |
| 11. Operations and rollback | Rollback is bounded: a bad release can be marked pre-release (the installer reads `releases/latest`) or deleted, shims deprecated or yanked; tags are not moved by policy. Procedure in the runbook. No objection |
| 12. Skeptical external customer | "Verified by whom?" is answered honestly on every surface only because the surfaces say nobody external yet. That honesty is the product's claim. A 1.0 before an external report would contradict it |

Disagreement: seat 1 would call the branch GO for merge review; seats 4 and
12 hold NO-GO for 1.0. Both are right about different questions, and the
ledger records both: **GO for Daniel's merge review of PR #336 as a
post-0.8.0 release candidate; NO-GO for naming it v1.0.**

## 8. Changes made in this pass

Recorded commit by commit in the handback. Scope: this ledger, the
security-review packet, the second-provider runbook, the release runbook,
their index rows in [README.md](README.md), the repair of that index's
glued heading (B-24) with a regression test that fails on the old file,
and one stale source comment in `cmd/replay/main.go` that named
`docs/CLI.md` as the home of the exit-code table when the table is in
`docs/guide/commands.md`. No production code path changed; no test was
weakened; no gate threshold moved.

---

[Evidence index](README.md) · [Release criteria](../../RELEASE-CRITERIA.md) ·
[Phase 2 record](closure-phase2-2026-10-09.md)
