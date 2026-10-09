# The v1.0 closure council, 2026-10-09: NO-GO until named conditions, and what this pass closed

**Candidate `e937dc6` on `fix/compaction-observed-vs-inferred`, CI run
37899158156 read job by job. Verdict: NO-GO until the conditions in section 2
are satisfied. Nothing merged, tagged, published, deployed or announced. The
Go toolchain line in `go.mod` was not changed.** This pass closed the Markdown
lint job, pinned the fact the HTTP/2 advisory dispositions rest on, identified
the deployment mechanism of both public sites, and corrected three premises
in the prior gate's record. The twelve seats below are simulated
perspectives reasoned by one agent; they are not external people and their
agreement is not an independent security review. The prior package is
[REPLAY-REL-03](https://claude.ai/artifact/LBqG5oSnbtCiM8j7RuUjVV) and its
repository record is
[release-gate-1.0-2026-10-09.md](release-gate-1.0-2026-10-09.md); this file
builds on both and overwrites neither.

## 1. Baseline, re-verified rather than trusted

| Item | Verified 2026-10-09 |
|---|---|
| HEAD at start | `e937dc6dcc66085bb94d69ca94a0a5b3b675a7b9`, equal to `origin/fix/compaction-observed-vs-inferred`; working tree clean |
| origin/main, tags | `5c87562` (v0.8.0); 16 tags v0.1.0 to v0.8.0, unchanged |
| Stash | 3 pre-existing entries (`9cea1dc`, `2fd184b`, `453041a`), untouched |
| `go.mod` | `go 1.24`, `toolchain go1.25.13`; not changed by this pass |
| Protected TTL evidence | `ttl-register-2026-10-05.md` hashes to its `.sha256` (`6fb642fb...`); the last commit touching any of the five files is `56d0645` (2026-10-07); `~/replay-experiment/` never read or written |
| CI run 37899158156 on `e937dc6` | 14 success (ledger bench, frozen mutants, x402, Go latest release on go1.27.2 with `-race`, golangci-lint, tui layout, cli blueprint, Go windows, guard reachability, Go macos, installer drift, merge result, Go ubuntu, surface drift); 2 failure (govulncheck, Markdown lint) |
| Required status checks on `main` (ruleset 23126317) | Go (ubuntu-latest), Go (macos-latest), golangci-lint, guard reachability, frozen mutants, merge result. **govulncheck and Markdown lint are not required checks.** All six required checks are green on the candidate |
| Release workflow | `release.yml` refuses a tag whose commit is not an ancestor of `origin/main`; the candidate is not |
| govulncheck, local, go1.25.13 | 9 reachable stdlib advisories, exit 1, identical to CI |
| govulncheck, local, go1.27.2 | No vulnerabilities found, exit 0 |
| Markdown lint, local | 237 findings in 43 files before this pass, identical to the CI count on `e937dc6`; 0 after |

### 1.1 Three premises corrected

These were stated as ground truth going into this pass and are wrong as
stated. They are corrected here and the conclusions that rested on them are
re-derived, which is the only honest way to keep a record a reader can trust.

1. **"go1.25.13 is the last 1.25 release that will ever ship."** False as a
   premise: `go.dev/dl` lists `go1.25.14` (stable, 2026-08-19). The
   conclusion survives for a different reason: the thirteen advisories were
   published 2026-10-08, each OSV record reads `introduced 0, fixed 1.26.9`
   and `introduced 1.27.0-0, fixed 1.27.2`, so every 1.25.x including 1.25.14
   is affected, and Go's policy ("each major Go release is supported until
   there are two newer major releases") means 1.25 left support when 1.27.0
   shipped. No 1.25 fix exists or will.
2. **"The figure set on replay.doctor has zero matching evidence file."**
   True of this repository, false of the product. The landing figures are
   rendered from `src/vendor/replay-corpus.json` in the private repository
   `RedRobotKK/replay.doctor`, written by `scripts/vendor-corpus.mjs` from
   `replay cost --json` on build 0.6.2, `takenAt` 2026-09-21, and the page
   names both the build and the date. The correct disposition is
   **provenanced and stale by two releases**, not unsupported; the correct
   repair is to re-vendor from v0.8.0, not to substitute the older 2026-09-17
   reading.
3. **"redrobot.jp serves the installer from commit b91018f."** The vendored
   metadata in `RedRobotKK/redrobot-jp` names commit `8946b63` (vendored
   2026-09-12); `install.sh` is byte-identical at `8946b63` and `b91018f`
   (both `4b04bd28...`), so the content attribution was right and the commit
   was not. The repository's current `install.sh` (`74a13afd...`, changed at
   `f72e3f8`, 2026-09-13) is what replay.doctor serves, vendored at
   `48c9974` on 2026-09-20.

## 2. Council verdict, binding

**NO-GO until the following are satisfied.** A majority of seats could not
override any of them, because each is a written criterion or an accepted
decision, and none was weakened to move faster.

| ID | Mandatory condition | Written source | Objective closure |
|---|---|---|---|
| C-1 | An external security review, published | RELEASE-CRITERIA.md "Where this stands today"; docs/ROADMAP.md "checked by somebody other than the person who wrote them" | The five items in security-review-scope-2026-10-08.md section 6 exist; RELEASE-CRITERIA.md links the report |
| C-2 | Second-provider caching rules **and** a calibration corpus against real traffic | RELEASE-CRITERIA.md second-provider row | A calibration record from real OpenAI traffic, or a dated amendment by Daniel |
| C-3 | Reproducibility verified on the 1.0 candidate tag | RELEASE-CRITERIA.md reproducible-releases row ("not a fact about any v1.0 tag") | `scripts/reproduce-release.sh` run against the 1.0 tag on all four platforms, or wired into CI |
| C-4 | govulncheck green on the candidate | ci.yml makes the job blocking on purpose; README badge says "blocking in CI" | The toolchain bump lands and the 16-job matrix including govulncheck is green on the merged commit |
| C-5 | Candidate on `main` with all required checks green | release.yml ancestry gate; ruleset 23126317 | Merge, then the Release workflow accepts the tag |
| C-6 | The withdrawn forensic week absent from every public surface | ADR-0028 (2026-10-05) | Section 6's four locations corrected and re-fetched |
| C-7 | No public surface claims an external review happened | RELEASE-CRITERIA.md measurement rule; the provenance correction in e937dc6 | redrobot.jp/replay's two "adversarial reviewer" sentences corrected and re-fetched |
| C-8 | Compatibility surfaces named before 1.0 | docs/ROADMAP.md v1.x "Before 1.0 ships, this file has to say which" | The list exists in ROADMAP.md or an ADR |

**Permissible deferrals** (not gates; recorded so they are not smuggled in):
the naming note on CITATION.cff and marketplace.json (option C); the
installer on redrobot.jp (P1, must precede any announcement that names that
URL, does not gate the tag); the stale corpus figures (provenanced, labelled
with build and date, to be re-vendored at release); Markdown lint (closed by
this pass pending CI confirmation, and not a required check either way).

**Authorization dependencies**, all Daniel's: the toolchain bump; the review
engagement and its budget; the site deployments; the naming note; any
criteria amendment.

## 3. Debate record

Each seat states: assessment of the NO-GO; top blocker; smallest path;
what can be deferred; strongest counterargument to its own proposal; evidence
that would change its conclusion.

**1. Release engineering chair.** NO-GO is correct. Top blocker: the
candidate is not on `main` and two blocking-by-intent jobs are red. Smallest
path: land the lint and generator fixes now, the toolchain bump as its own
commit on approval, merge once the matrix is green. Defer: naming, installer
redirect. Counter: the ruleset does not require govulncheck, so a merge is
mechanically possible today; rejected, because the repository made the job
blocking on purpose and the badge now says so. Would change: nothing short of
the conditions in section 2.

**2. Staff Go engineer.** NO-GO; the toolchain is the material risk, not any
one advisory. Top blocker: a pinned major that cannot receive a fix. Smallest
path: `toolchain go1.27.2` (floor 1.26.9), one line and a comment. Defer:
raising the `go 1.24` language directive. Counter: 1.26.9 is the smaller jump;
answered: 1.26 leaves support when 1.28 ships, 1.27.2 buys a full cycle, and
the latest-release job already ran 1.27.2 under `-race` green. Unverified under
1.27 and named as such: frozen mutants (3927 s), guard reachability, the
golangci-lint v2.5 action, Windows and macOS legs, GoReleaser. Note for the
record: `go.mod`'s own comment says the line is a floor, so a developer with
go1.27.1 installed (this machine) builds with a version the same advisories
affect; the bump fixes that too. Would change: a 1.25 fix, which cannot exist.

**3. Product security lead.** NO-GO on policy, not on exposure. Thirteen
advisories, zero confirmed material exposure, two applicable but mitigated
(GO-2026-6611 on the default-transport typed commands to TLS peers the
operator chose; GO-2026-6608 on a trusted provider's trailers), eleven not
applicable with evidence, two of those measured against the built binary.
Smallest path: pin the construction-time fact the HTTP/2 rows rest on (done
this pass, `TestGV6610_...`, watched red under `ForceAttemptHTTP2: true`).
Defer: nothing. Counter: "unreachable is not harmless"; agreed, which is why
the disposition names policy noncompliance and the fix is the toolchain, not
the argument. Would change: a reachable path to an HTTP/2 or TLS server in
the shipped binary.

**4. Independent assurance specialist.** NO-GO; C-1 is unconditional as
written and this council cannot substitute for it. Top blocker: no reviewer,
no engagement, no budget. Smallest path: run two channels in parallel, an
application to an OSS audit fund (OSTIF or the OpenSSF program) and two
quotes from named independent researchers with public audit records; take
whichever lands first that meets section 6 of the scope file. Defer: the
boutique-firm route unless the fund declines. Counter: a fund application has
no guaranteed acceptance; answered by the parallel quotes. Also raised, and
adopted as C-7: redrobot.jp/replay publicly says "checked by an adversarial
reviewer who ran the proxy against a fake upstream" and links the
self-authored file. That is a public statement of the independence 1.0
requires, made before it exists. Would change: a published report meeting the
five criteria.

**5. Testing and mutation specialist.** NO-GO stands; the test evidence on
the candidate is complete under 1.25.13 and partial under 1.27.2. Top
blocker: frozen mutants and guard reachability have not run under 1.27.
Smallest path: the bump commit's own CI run is the qualification. Defer:
nothing. Counter: the suite is toolchain-agnostic by design; answered: the
latest-release job exists because a stdlib change once broke this project,
so "agnostic" is a hypothesis the matrix tests. Would change: a green 16-job
run on the bump commit.

**6. SRE and operations lead.** NO-GO; both sites are out of step with the
tree and nothing in this repository deploys them. Finding: both sites are
Cloudflare Pages projects in private repositories (`RedRobotKK/replay.doctor`,
project `replay-doctor`, `npm run deploy`; `RedRobotKK/redrobot-jp`, project
`redrobot-jp`, `npm run deploy:cf`), each vendoring `install.sh` at a pinned
commit with a metadata file and a freshness check. Smallest path for the
installer: make `redrobot.jp/replay.sh` and `/Replay/install.sh` 301 to
`replay.doctor/replay.sh`, which the replay.doctor repository already calls
"the canonical origin for its installer"; one vendoring instead of two.
Defer: nothing else. Counter: a redirect changes the URL printed in old
installs; answered: `curl -fsSL` follows redirects and the content is
identical. Rollback: Pages keeps prior deployments; roll back from the
dashboard. Verification: `curl -fsSL <url> | shasum -a 256` equals the
repository's `install.sh` for both URLs. Would change: evidence that any
reader depends on redrobot.jp serving its own bytes.

**7. Product manager.** NO-GO; the scope of 1.0 is fine and is not the
problem. Top blocker: the three RELEASE-CRITERIA items that need another
party. Smallest path: do not add scope. Defer: `replay recall`, everything
ROADMAP already lists as not a gate. Counter: Markdown lint feels like a
release criterion because it is red; it is not one and must not be dressed as
one. Would change: a change to the written criteria.

**8. GTM and positioning strategist.** NO-GO; a public page sells an offer an
accepted ADR withdrew. Top blocker: the $22,000 week at
`replay.doctor` `src/pages/index.astro:755` and
`src/content/docs/vendor.md:19`, and its de-dollarized echo at `redrobot-jp`
`src/pages/replay-doctor.astro:56`. Smallest path: the corrected copy in
section 6, which is the GTM-02 copy for the offer and a different repair for
the figures (re-vendor, not substitute). Defer: the naming note. Counter: the
figure set should be removed outright; answered: it is provenanced in the
site repository and labelled with build and date, and Daniel's own rule is to
re-vendor every release, so the defect is staleness and the fix is the
vendoring step at release. Would change: a release that re-vendors.

**9. Documentation and developer-experience lead.** NO-GO; the lint job was
red on nothing a reader meets, and that is its own defect because a red job
nobody reads is a job that gets switched off. Smallest path: generator fixes
in the generators, mechanical fixes in living documents, a named exemption
for frozen records with the reason beside it, following the repository's own
precedent (`AGENTS_STATE.md`). Done this pass. Defer: the two "## 17."
headings in UNWIRED-LOG.md (a numbering defect, not a lint finding, recorded
here and not adopted). Counter: a 33-file exemption is a rug; answered in
Round 2. Would change: a reader-facing defect in an exempted file.

**10. Compliance and research-integrity specialist.** NO-GO; and three
premises in the prior record were wrong and are corrected above rather than
deleted. Top blocker: C-1, because an amendment route exists (a dated,
attributable note in RELEASE-CRITERIA.md) and is not recommended, since the
criterion was Daniel's own and nothing about the risk changed. Smallest path:
record, do not amend. Defer: nothing. Counter: the second-provider gate could
be amended on the ground that no OpenAI budget has ever existed; answered:
that is a reason the gate is hard, not a reason it is wrong. Would change: a
dated amendment by Daniel, recorded as a dissent.

**11. External customer advocate.** NO-GO; a stranger sees a $22,000 offer
with "Talk to Daniel" on the product site and "nothing is for sale" in the
README, and a Windows user who runs `redrobot.jp/replay.sh` is told about an
archive that does not exist. Top blocker: C-6 and C-7, because the second is
worse than the first: it claims a check that is the definition of 1.0. Smallest
path: the two deployments. Defer: naming, which confuses nobody who reads the
note the README already carries. Counter: the offer is unsold and harmless;
answered: an offer nobody can buy, beside a statement that nothing is for
sale, costs trust and sells nothing. Would change: re-fetched pages.

**12. Adversarial release auditor.** Round 2 is below. Assessment: NO-GO;
the GO case cannot be built while C-1 is open, whatever else closes.

### Round 1, challenges

- Toolchain required before release under current policy? **Yes.** The job
  is blocking by the repository's own stated intent, the badge now says so,
  and the pinned major cannot receive the fix. Seats 1, 2, 3, 5 agree; seat 7
  notes the ruleset does not require it and is answered.
- Confirmed exposure, policy noncompliance, or both? **Policy noncompliance
  with zero confirmed material exposure.** Seat 3's disposition stands and is
  now pinned by a test that goes red when the fact changes.
- Is the external review unconditional? **Yes, as written.** Seats 4 and 10;
  no seat found text that conditions it. The only permitted alternative is a
  dated amendment by Daniel, which no seat recommends.
- Are all live-site corrections mandatory before release? **C-6 and C-7
  yes; the installer and the figures no**, with the installer mandatory
  before any announcement names that URL.
- Lint: mechanical sweep or exemption? **Both, narrowly.** Generator defects
  in the generators, living documents fixed, frozen records exempted by name.
- Naming: ADR or Daniel? **Daniel.** No ADR records the decision
  (`grep -l "Replay Doctor" docs/adr/*.md` is empty); the three remaining
  surfaces wait on one line from him.
- Does the evidence prove all required checks passed? **The six required
  checks, yes; the sixteen-job matrix, no** (two red), and the matrix is the
  bar this repository set itself.
- What proceeds without further authorization? Lint, generators, the pin
  test, this record, the push to the feature branch. All done.

### Round 2, falsification

| Objection | Resolution |
|---|---|
| The exemption hides 219 findings; the job is green because it looks at less | Partly upheld, then answered: the list is explicit, 33 names and no glob, a new record is still linted, the list cannot grow without a reviewed change, and the one finding a reader could see (the five-column row in the evidence index) was fixed, not exempted. Residual: a future edit to an exempted file is unlinted; accepted |
| The pin test proves a construction-time fact, not runtime behaviour | Upheld as stated and sufficient: Go's documented rule is about construction, the test holds the three inputs to that rule and the listener's TLS config, and was watched red under the mutation. A runtime negotiation test would need a TLS upstream, which the shipped binary never configures |
| The figure set is still unsupported; a JSON in another repository is not an evidence file | Partly upheld: the JSON is machine output naming build, date and command, which is what the measurement rule demands, but no narrative records how it was taken. Disposition: provenanced and stale. Either way the repair is the same, re-vendor at release |
| The council is a self-review wearing twelve hats | Upheld without reservation: it is, the file says so in its first paragraph, and C-1 is open precisely because of it |
| The toolchain analysis is incomplete: nothing ran under 1.27 except one job | Upheld; that is why the bump lands as its own commit whose matrix is the qualification, with rollback by revert |
| A GO recommendation is being prepared under another name | Not upheld: the verdict is NO-GO and every condition is objective |

## 4. Criteria-to-evidence matrix

| Criterion | Evidence | State |
|---|---|---|
| Six required status checks | CI run 37899158156, jobs read individually | MET on `e937dc6` |
| Full 144-mutant sweep | frozen mutants job, `ok internal/mutation 3927s` | MET |
| Guard reachability, 0 unexplained | job log: 242 guards, 5 survived, 4 evidenced, 0 unexplained | MET |
| golangci-lint 0 issues | CI job and local run (v2.13.2) | MET |
| Four Go legs | ubuntu, macos, windows, latest release (go1.27.2, `-race`) | MET |
| govulncheck green | 9 reachable on 1.25.13; clean on 1.26.9 and 1.27.2 locally | NOT MET (C-4) |
| Markdown lint green | 237 to 0 locally; CI confirmation pending on the push | CLOSED LOCALLY |
| External review published | none commissioned | NOT MET (C-1) |
| Second-provider calibration corpus | rules and dispatch exist; no corpus; no OpenAI budget | NOT MET (C-2) |
| Reproducible on the 1.0 tag | verified on v0.5.4 and v0.8.0; no 1.0 tag | NOT MET (C-3) |
| Candidate on main | not merged | NOT MET (C-5) |
| ADR-0028 compliance on public surfaces | offer live at three locations | NOT MET (C-6) |
| No public claim of external review | redrobot.jp/replay lines 477 and 608 | NOT MET (C-7) |
| Compatibility surfaces named | ROADMAP v1.x gap still open | NOT MET (C-8) |

## 5. Technical and security report

**Candidate.** `e937dc6` plus this pass's commits on the same branch (SHAs in
section 11). Nothing in this pass touches production behaviour: two
generator functions, one test file in `internal/proxy`, one test file under
the `mutation` tag, documents and lint configuration.

**Vulnerability dispositions.** Unchanged from
[release-gate-1.0-2026-10-09.md](release-gate-1.0-2026-10-09.md) section 3,
re-verified: govulncheck on go1.25.13 reports the same nine reachable
advisories plus four unreached; each OSV record on `vuln.go.dev` carries
`fixed 1.26.9` and `fixed 1.27.2`; the proxy transport at
`internal/proxy/server.go:215` sets `DialContext` and not
`ForceAttemptHTTP2`, and `TestGV6610_TheUpstreamLegIsHTTP1OnlyByConstruction`
now holds that. Policy: noncompliant (a blocking job is red). Exposure: none
confirmed material. Residual uncertainty, stated: GO-2026-6611 remains
reachable through `upgrade`, `rules --check-prices` and `probe --execute`
to TLS peers the operator chose; GO-2026-6608 through the configured
provider's response trailers. Neither is harmless by being unreached; both
are fixed by the toolchain and by nothing else in this tree.

**Toolchain analysis, bounded.** Prepared change, not applied:

```diff
-// The number is 1.25.13 because govulncheck said so on its first run. The floor
+// The number was 1.25.13 because govulncheck said so on its first run. The floor
 ...
-toolchain go1.25.13
+// Raised to 1.27.2 on <date> for the thirteen advisories of 2026-10-08
+// (GO-2026-6599 to 6617), each fixed in 1.26.9 and 1.27.2 and in no 1.25.x,
+// because 1.25 left upstream support when 1.27.0 shipped. 1.26.9 is the
+// minimum that clears govulncheck; 1.27.2 is chosen because 1.26 leaves
+// support when 1.28 ships.
+toolchain go1.27.2
```

Evidence under the new version: go1.26.9 build, vet, full suite (40 packages)
and govulncheck clean, locally; go1.27.2 govulncheck clean locally and the
latest-release CI job green with `-race`. Not evidenced under 1.27 and named:
frozen mutants, guard reachability, golangci-lint v2.5 action, Windows and
macOS legs, GoReleaser (release.yml uses `go-version-file: go.mod`, so the
release binary's compiler changes with this line). Known risk: the pinned
golangci-lint action version may refuse a newer toolchain; if the lint job
fails on the bump commit, the fix is the action's `version:` input, not the
toolchain. Rollback: `git revert` of the one commit; all sixteen past tags
are immutable and unaffected. CI consequence: every job runs on 1.27.2, so
the bump commit's run is the qualification matrix.

## 6. GTM correction register

Both sites are Cloudflare Pages projects deployed from private repositories
by `wrangler pages deploy`; neither is deployed from this repository and
nothing here can change them. Each row names the file a deployer edits.

| ID | Surface | File and line | Current claim | Correction | Authorization | Verification |
|---|---|---|---|---|---|---|
| G-1 | replay.doctor | `RedRobotKK/replay.doctor` `src/pages/index.astro:755` | "As of 2026-09-13 the week is $22,000, quoted at $18,000 to $25,000 by scope... Talk to Daniel" | Replace the block with: "There is nothing paid today. Every command works, on every model, with no account and no key. If a paid capability is ever added it will be something that does not exist today." Remove the CTA | Daniel (deploy) | re-fetch; `grep -c "22,000"` is 0 |
| G-2 | replay.doctor | `src/content/docs/vendor.md:19` | "The week is $22,000 flat, quoted at $18,000 to $25,000 by scope" | Delete the sentence and its derivation pointer; add "Withdrawn 2026-10-05 (ADR-0028)" | Daniel (deploy) | re-fetch `/vendor/` |
| G-3 | replay.doctor | `src/vendor/replay-corpus.json` (`takenAt` 2026-09-21, `replayVersion` 0.6.2) | $16,078.68, $631.77, 123 sessions, 2,176 lanes, 106.7M tokens, "read by replay 0.6.2" | Provenanced, stale. `npm run vendor:corpus` against v0.8.0 on the operator's machine at release; the page renders whatever the file holds, build and date included | Daniel (corpus is his) | `replayVersion` on the page equals the release |
| G-4 | redrobot.jp/replay-doctor/ | `RedRobotKK/redrobot-jp` `src/pages/replay-doctor.astro:56` | "The one thing for sale is a week of one operator reading a team's corpus with them" | "Running the tool costs nothing and no entitlement check will enter the binary. Nothing is for sale today." | Daniel (deploy) | re-fetch; "week" absent |
| G-5 | redrobot.jp/replay | `src/pages/Replay/index.astro:134` | "v0.6.2 is tagged" from `CORPUS.version` | Re-vendor (G-3) or read the version from the release feed | Daniel | page shows the current tag |
| G-6 | redrobot.jp/replay | `src/pages/Replay/index.astro:477` and `:608` | "checked by an adversarial reviewer who ran the proxy against a fake upstream... The findings are published"; "The adversarial security review" | "checked by the maintainer against a fake upstream; the findings are published and the review is self-authored (an external review is not yet commissioned)"; list label "The maintainer's security review (self-authored)" | Daniel (deploy) | re-fetch; "adversarial reviewer" absent |
| G-7 | redrobot.jp/replay.sh and /Replay/install.sh | `src/pages/Replay/install.sh.ts`, `src/vendor/replay-install.sh` (commit `8946b63`, sha256 `4b04bd28...`) | Installer with the superseded Windows message (names an archive that does not exist and a `go install` the binary refuses) | Preferred: 301 both paths to `https://replay.doctor/replay.sh`. Alternative: `npm run vendor:installer` and deploy | Daniel (deploy) | `curl -fsSL` of both URLs hashes to `74a13afd...` (equal to repository `install.sh`) |
| G-8 | GitHub About | repository settings | "Replay Doctor names the turn..." with no note | Append: "(Replay on GitHub and as the CLI; Replay Doctor on npm, PyPI and replay.doctor; one binary.)" | Daniel (settings) | API `description` |
| G-9 | CITATION.cff | this repository | no note | Add to `abstract`: "Distributed as replay-doctor on npm and PyPI and at replay.doctor; same binary." | Daniel (naming decision, one line) | file |
| G-10 | .claude-plugin/marketplace.json | this repository | "Replay Doctor for Claude Code..." with no split stated | Prefix the metadata description with "Replay (published as Replay Doctor on npm, PyPI and replay.doctor; same binary): " | Daniel (naming decision) | file |

In-repository surfaces (README, CHANGELOG, plugin.json, SKILL.md, install.sh)
were corrected in the prior passes and re-checked: no `$22,000` outside dated
evidence, design and research records that describe the withdrawn offer as
history.

## 7. Operational checklist

1. **Toolchain** (on approval): one commit on this branch; wait for the full
   16-job run; if golangci-lint fails on version, fix the action input; if
   frozen mutants or guard reachability fail, revert and report.
2. **Merge**: PR from this branch; the six required checks are already
   green; squash is not required (linear history is).
3. **Tag 1.0** only after C-1 through C-8: the release workflow verifies
   ancestry, builds with `go-version-file: go.mod`, signs with cosign and
   emits SBOMs per archive; then run `scripts/reproduce-release.sh` against
   the tag on all four platforms (C-3).
4. **Sites**: apply G-1, G-2, G-4, G-6, G-7 first (they are decision
   compliance), G-3 and G-5 at release; deploy; re-fetch each URL and record
   the hashes in a dated evidence file.
5. **Post-release**: `scripts/installer-drift/check.sh` with
   `INSTALLER_DRIFT_STRICT=1`; npm and PyPI `latest` equal the tag; GitHub
   About carries the naming note.
6. **Rollback**: toolchain by revert; sites by redeploying the previous Pages
   deployment; a published tag is never deleted (ADR-0024 territory), a
   defective 1.0 is followed by 1.0.1.

## 8. Risk register

| ID | Risk | Severity | Likelihood | Evidence | Mitigation | Owner | Residual |
|---|---|---|---|---|---|---|---|
| R-A | 1.25 toolchain cannot receive the 2026-10-08 fixes | High | Certain | OSV ranges; Go policy | Bump (prepared) | Daniel | None after bump |
| R-B | Bump breaks a job not yet run under 1.27 | Medium | Low | latest-release job green | Own commit; revert | Agent on approval | Low |
| R-C | External review never commissioned | High | Open | no engagement | Section 9 memo | Daniel | 1.0 cannot ship |
| R-D | Public page claims an external review | High | Certain today | G-6 | Corrected copy | Daniel | None after deploy |
| R-E | Withdrawn offer live | High | Certain today | G-1, G-2, G-4 | Corrected copy | Daniel | None after deploy |
| R-F | Stale installer on redrobot.jp misleads Windows users | Medium | Certain today | G-7 | Redirect | Daniel | None after deploy |
| R-G | Exempted files drift unlinted | Low | Medium | 33 names | List is explicit and reviewed | Maintainer | Accepted |
| R-H | Figures stale against the shipping build | Medium | Certain today | G-3 | Re-vendor at release | Daniel | None after vendoring |
| R-I | Naming split confuses a reader on three surfaces | Low | Low | README note exists | G-8 to G-10 | Daniel | Low |
| R-J | Second-provider calibration never funded | Medium | Open | no OpenAI budget ever | Amendment or corpus | Daniel | 1.0 cannot ship |

## 9. Authorization memo for Daniel

Only decisions that need you. Each has a recommended choice and the
consequence of the alternative.

1. **Toolchain: approve `toolchain go1.27.2` in `go.mod`.** Recommended: yes,
   as one commit on this branch, evaluated by its own CI run. Alternative:
   1.26.9 (also clean, shorter support). Not doing it: govulncheck stays red
   forever on this major, and C-4 cannot close. Cost: one CI run (about 70
   minutes). Rollback: revert.
2. **External review: pick a channel and a ceiling.** Recommended: apply to
   an OSS audit fund and request two quotes from named independent
   researchers in parallel, using section 5 of
   [security-review-scope-2026-10-08.md](security-review-scope-2026-10-08.md)
   as the brief and its section 6 as acceptance; set a budget ceiling so the
   quotes can be accepted without a second round. Alternative: amend
   RELEASE-CRITERIA.md by dated record; not recommended and recorded as a
   dissent if taken. Not deciding: 1.0 does not ship.
3. **Sites: deploy G-1, G-2, G-4, G-6, G-7 now; G-3 and G-5 at release.**
   Recommended: the redirect for G-7. These are decision compliance (ADR-0028)
   and a public provenance statement, not polish.
4. **Naming: one line.** Recommended: option C (code identity "Replay",
   distribution identity "Replay Doctor", stated on every name-bearing
   surface); on your yes, G-9 and G-10 are applied here and G-8 is yours in
   settings. Alternative: a rename scheduled under ADR-0024 after 1.0.
5. **Second provider: fund a calibration corpus or amend by dated record.**
   Recommended: decide which before committing to a 1.0 date, because this is
   the gate with the longest unknown lead time.
6. **Compatibility surfaces: name them (C-8).** Recommended: the list of
   verbs and flags the 1.x promise covers, in ROADMAP.md or an ADR, before
   the tag.

Not requested: any spend, any merge, any tag, any deployment, any
announcement. This memo does not grant release authorization and neither
does the verdict.

## 10. Blocker register, single and authoritative

| ID | Criterion or risk | Verified state | Smallest remediation | Owner | Depends on | Authorization | Verification | Acceptance | Failure path | Status |
|---|---|---|---|---|---|---|---|---|---|---|
| B-01 | C-4 govulncheck | 9 reachable on 1.25.13 | `toolchain go1.27.2` | Daniel approves, agent applies | none | Toolchain | 16-job CI run | all 16 green | revert | BLOCKED (authorization) |
| B-02 | Markdown lint | 237 to 0 locally | this pass | agent | none | existing branch push | CI job on the pushed commit | job green | revert the config | IN PROGRESS (CI pending) |
| B-03 | C-1 external review | not commissioned | engage per memo item 2 | Daniel | budget | spend | scope section 6 | report linked from RELEASE-CRITERIA.md | amendment (dissent) | BLOCKED (authorization) |
| B-04 | C-2 second provider | rules and dispatch, no corpus | corpus or amendment | Daniel | OpenAI spend | spend | calibration record | record exists | amendment | BLOCKED (authorization) |
| B-05 | C-3 reproducibility on the 1.0 tag | verified on v0.8.0 | run script on the tag | release operator | tag exists | tag | four hashes equal | identical bytes | 1.0.1 | OPEN (sequenced after tag) |
| B-06 | C-5 candidate on main | branch only | merge | Daniel | B-01, B-02 | merge | ruleset | ancestor of main | revert merge | BLOCKED (authorization) |
| B-07 | C-6 withdrawn offer | live at three locations | G-1, G-2, G-4 | Daniel | none | deploy | re-fetch | strings absent | redeploy previous | BLOCKED (authorization) |
| B-08 | C-7 public review claim | live at two lines | G-6 | Daniel | none | deploy | re-fetch | phrase absent | redeploy previous | BLOCKED (authorization) |
| B-09 | C-8 compat surfaces | unnamed | write the list | Daniel | none | none (product decision) | file | list exists | n/a | OPEN (decision) |
| B-10 | Installer parity (P1) | `4b04bd28` on redrobot.jp | G-7 redirect | Daniel | none | deploy | hashes equal | equal | redeploy previous | BLOCKED (authorization) |
| B-11 | Stale figures (P2) | 0.6.2 vendored | G-3, G-5 at release | Daniel | release | corpus access | page version | equals tag | re-vendor | OPEN (sequenced) |
| B-12 | Naming note (P3) | three surfaces lack it | G-8 to G-10 | Daniel, then agent | decision | naming | files and API | note present | revert | BLOCKED (decision) |
| B-13 | HTTP/2 disposition pinned | pinned this pass | done | agent | none | none | test red under mutation | test in tree | n/a | VERIFIED CLOSED |
| B-14 | Generator artifacts | fixed in both generators | done | agent | none | none | regenerated files match | lint 0 | n/a | VERIFIED CLOSED |

## 11. Execution board and handoff

**Completed this pass, test first where code changed:**

- `internal/claims/render.go`: the register no longer ends with a blank line
  (`TestRenderEndsWithExactlyOneNewline`, red then green);
  `CLAIM-REGISTER.md` regenerated by `go run scripts/claim-register/main.go`
  (one line removed).
- `internal/blackbox/grade_test.go`: `entryLine` renders an empty argument
  as `""` (`TestBB_EntryLineQuotesAnEmptyArgument`, red then green);
  `docs/evidence/production-grade-matrix.md` regenerated by the full black
  box run (`ok internal/blackbox 73.688s`, one cell changed).
- `internal/proxy/http2pin_test.go`: the HTTP/2 disposition pin, green on the
  tree and red under `ForceAttemptHTTP2: true`, mutation reverted and the
  file verified clean.
- Living documents: `CAMPAIGN-BASELINE.md`, `docs/ASSIGNMENT-CHANGESET.md`,
  `docs/design/UNWIRED-LOG.md` (verbatim duplicate section removed),
  `docs/PROOF-1.0.md`, `docs/PRODUCTION-WIRING.md`, `docs/evidence/README.md`
  (escaped `||` that was splitting a row), `docs/research/README.md`,
  `docs/evidence/security-review-scope-2026-10-08.md` (fence language).
- `.markdownlint-cli2.yaml`: 33 frozen records exempted by name with the
  reason beside the list. `markdownlint-cli2`: 0 issues in 0 files.
- `CHANGELOG.md`: entries under Unreleased.
- Verified locally: `gofmt -l .` empty; `go vet ./...` and
  `go vet -tags mutation ./internal/blackbox` clean; `golangci-lint run`
  0 issues; full suite result recorded in the commit message of this pass.

**Not done, by rule:** the toolchain line; any site change; any spend; any
merge, tag, publish or announcement; any edit to the five TTL files or
`~/replay-experiment/`.

**Immediate next action:** Daniel answers memo items 1 and 3; on item 1 the
agent applies the one-line bump as its own commit and reports the 16-job
result; on item 3 the deployer applies section 6 and the agent re-fetches.

---

[Evidence index](README.md) · [Release criteria](../../RELEASE-CRITERIA.md) ·
[Prior gate](release-gate-1.0-2026-10-09.md) ·
[Reviewer brief](security-review-scope-2026-10-08.md) ·
[GTM-02](gtm-02-public-surface-retrofit-2026-10-08.md) ·
[ADR-0028](../adr/0028-replay-is-a-product-with-a-hosted-service.md)
