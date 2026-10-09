# Closure Council Phase 2: authorized remediation, 2026-10-09

**Status: NO-GO for v1.0.** Every blocker that Daniel's authorization of
2026-10-09 reached is closed on the feature branch or on a site feature
branch; the four that decide the verdict are outside it: no external review
is commissioned, the second-provider gate has rules and no corpus, the
candidate is not on `main`, and the corrected sites are not deployed. This
record follows [closure-council-1.0-2026-10-09.md](closure-council-1.0-2026-10-09.md)
and reuses its registers by identifier. One agent, no sub-agents; the two
panels in section 9 were reasoned by that agent and are labelled simulated.

## 1. Baseline, re-verified rather than trusted

| Fact | Verified |
|---|---|
| Starting HEAD | `0ea7370`, on `a921240`, `b01354e`, `e937dc6`; `origin/fix/compaction-observed-vs-inferred` equal |
| `origin/main` | `5c87562`, the v0.8.0 commit, unchanged at start and at the end of this pass |
| Tags | 16, unchanged |
| Shared stash | 3 entries (`9cea1dc`, `2fd184b`, `453041a`), untouched |
| `go.mod` at start | `go 1.24`, `toolchain go1.25.13` |
| Baseline CI run `37909480292` on `0ea7370` | 15 of 16 green; govulncheck red on 9 reachable stdlib advisories |
| Upstream Go releases (`go.dev/dl`, read 2026-10-09) | stable: go1.27.2, go1.27.1, go1.27.0, go1.26.9; go1.25.14 exists; 1.25 is two majors behind |
| go1.27.2 archives | present for linux, darwin and windows on amd64 and arm64, every platform the release builds |
| Private site repositories | both accessible by `gh repo view`; cloned into the job's scratch directory, never into the worktree |

## 2. Toolchain upgrade to go1.27.2

**Commits, both pushed to `fix/compaction-observed-vs-inferred`:**

- `bbee9f6` toolchain: the one line `toolchain go1.25.13` to
  `toolchain go1.27.2`, the comment paragraph the council prepared, dated,
  and a CHANGELOG entry under Security. `go 1.24` stays as the language
  floor.
- `8d79db9` ci: the two tool pins that checked the toolchain could not read
  go1.27.2. `govulncheck@v1.1.4` panicked before any verdict
  (`panic: unexpected expr: *ast.KeyValueExpr`, exit status 2, job
  113897437872); golangci-lint v2.5 refused the tree
  (`can't load config: the Go language version (go1.25) used to build
  golangci-lint is lower than the targeted Go version (1.27.2)`, job
  113897438297). Pins moved to `govulncheck@v1.8.0` (requires go 1.26.0) and
  golangci-lint `v2.14` (go1.27 support landed in v2.13.0). Both were run
  locally under go1.27.2 before the commit: "No vulnerabilities found", exit
  0; "0 issues", exit 0. Nothing excluded, no finding suppressed, no required
  check changed.

**Local matrix under go1.27.2 on `bbee9f6`, before the push:** `go build
./...` ok; `go vet ./...` ok; `gofmt -l .` empty; `go test ./...` 40
packages ok, 0 FAIL; govulncheck "No vulnerabilities found".

**Why `8d79db9` cancelled the first run.** `ci.yml` sets
`cancel-in-progress` for every non-`main` ref, so the push of the pin commit
cancelled run `37953323762` on `bbee9f6` while frozen mutants and guard
reachability were still running. The run on `8d79db9`, the same tree plus
two `ci.yml` lines, is the qualification matrix. The table says what each
run saw.

**CI comparison, job by job.** Baseline is run `37909480292` on `0ea7370`
(go1.25.13). Run 2 is `37953323762` on `bbee9f6`. Run 3 is `37954240783` on
`8d79db9`.

| Job | Baseline `0ea7370` | Run 2 `bbee9f6` | Run 3 `8d79db9` |
|---|---|---|---|
| Go (ubuntu-latest) | success | success | success |
| Go (macos-latest) | success | success | success |
| Go (windows-latest) | success | success | success |
| Go (latest release), -race | success | success | success |
| golangci-lint | success (v2.5 on go1.25.13) | failure: v2.5 refuses go1.27.2 | success: v2.14, "0 issues" |
| govulncheck | failure: 9 reachable advisories | failure: v1.1.4 panics, no verdict | success: v1.8.0, "No vulnerabilities found" |
| guard reachability | 242 guards, 0 unexplained | cancelled by the next push | success: 242 guards, 5 survived (1 pre-existing, 4 introduced), 4 evidenced, 0 unexplained, 0 unchecked |
| frozen mutants | success (`ok internal/mutation 3192s`, `ok internal/blackbox 70s`) | cancelled by the next push | in progress at the time of writing; the terminal result is appended in section 12 |
| Markdown lint | success | success | success |
| cli blueprint | success | success | success |
| installer drift | success | success | success |
| surface drift | success | success | success |
| x402 end to end | success | success | success |
| ledger bench | success | success | success |
| tui layout audit | success | success | success |
| merge result | success | success | success |

No job regressed against the baseline. The two red cells in run 2 are the
pins, not the code, and run 3 shows the same jobs green with the pins moved.
The release compiler changes with this line because `release.yml` reads
`go.mod`; no release is built or tagged here, and
`scripts/reproduce-release.sh` follows the same line.

## 3. Website remediation, prepared and pushed, not deployed

Neither site is deployed by anything in this repository, and no deploy
command, Cloudflare setting or DNS record was touched. Each repository got
a new feature branch from its `origin/main`; neither `main` moved.

### 3.1 RedRobotKK/replay.doctor, branch `fix/withdrawn-offer-and-corpus-0.8.0`, commit `42fb99b`

| Council item | File | Change |
|---|---|---|
| G-1 withdrawn offer | `src/pages/index.astro` | the $22,000 block and the "Talk to Daniel" button replaced by the prepared copy: nothing is paid today, the week was withdrawn 2026-10-05 by ADR-0028, none was ever sold |
| G-2 vendor packet | `src/content/docs/vendor.md` | price paragraph and buyer sizing removed; "What the engagement was, and its withdrawal" states "Withdrawn 2026-10-05 (ADR-0028)"; processing and invoicing terms kept for the record and marked as the withdrawn engagement's; the facts procurement still needs are unchanged |
| G-3 corpus | `src/vendor/replay-corpus.json` | re-vendored by `npm run vendor:corpus` from the published v0.8.0 darwin/arm64 binary, sha256 `4a7c2251...` verified against the release's `checksums.txt`; `replay 0.8.0 (5c87562)` |
| tests | `tests/prose.test.mjs`, `tests/vendor.test.mjs` | the three assertions that the offer was present now assert it is absent (red first, then green) |
| week report | `scripts/week-report.mjs` | renders the three summary fields 0.8.0 added (`pricedRequests`, `unpricedRequests`, `unpricedRebilledTokens`); the mutation test that requires every input to move the report caught them |

Build clean (126 pages); `npm test` 313 of 313 with `REPLAY_REPO` pointed
at a Replay checkout; `npm run check:installer` current (the vendored
installer at `48c9974` is byte-identical to `main`'s `install.sh`, sha256
`74a13afd...`). Built `index.html` and `vendor/index.html` contain no
`22,000`, `18,000` or `Talk to Daniel`.

### 3.2 RedRobotKK/redrobot-jp, branch `fix/replay-claims-and-corpus-0.8.0`, commits `003f711` and `aff1ef5`

| Council item | File | Change |
|---|---|---|
| G-4 week echo | `src/pages/replay-doctor.astro` | "Nothing is for sale today", the withdrawal named and dated, the rules feed's refusal to quote a price kept because the commercial-claims test forbids denying it |
| G-6 review claim (C-7) | `src/pages/Replay/index.astro` | "checked by the maintainer", "the review is self-authored: an external review is not yet commissioned"; list label "The maintainer's security review (self-authored)" |
| G-5 stale version | `src/vendor/replay-corpus.json` | byte-identical to the replay.doctor file so both sites publish one reading; the page now renders "v0.8.0 is tagged" from the file |
| briefs | `public/llms.txt`, `public/.well-known/llms.txt`, `public/ai-pack/knowledge/llms.txt` | the 0.8.0 reading with the 2026-09-21 reading recorded as superseded, in the form the coherence test and the briefs' own history use |
| G-7 installer | `src/vendor/replay-install.sh`, `.meta.json` | `npm run vendor:installer`: from `8946b63` (sha256 `4b04bd28...`) to `main` at `5c87562` (sha256 `74a13afd...`, equal to the repository's `install.sh` and to the replay.doctor copy); the superseded Windows message goes with it |
| script defect | `scripts/vendor-corpus.mjs` | read only the retired `avoidableUsd` in its summary line and crashed after writing the file under a current binary; reads either spelling now |
| tests | `tests/unit/commercial-claims.test.ts`, `tests/unit/replay-doctor.test.ts` | three new assertions, red first, then green |
| residue, second commit `aff1ef5` | `public/llms.txt` and its three mirrors | the orchestrating session's check of `003f711` found "Commands, as of v0.6.2" still in the three served copies and the `docs/ai-pack` copy untouched; corrected in the one source, mirrors regenerated by `ai-distribute.mjs` offline, the two verbs added since 0.6.2 (`simulate`, `grok`) named after diffing both binaries' help; a coherence test now requires every dated release claim in the five briefs to name the corpus build, red on the stale tree and red again under a one-mirror mutation |

Build clean; `vitest run` 404 of 404 after `aff1ef5`; `npm run check:replay` reports the
corpus current against release 0.8.0; `npm run check:installer` current at
`5c87562`. Playwright end-to-end was not run in this environment. Built
pages contain none of "adversarial reviewer", "week of one operator" or
"v0.6.2 is tagged".

### 3.3 The reading, and why the session count moved

| Reading | Build | Sessions | Lanes | Total at list | Re-billed | Tokens | Median | p90 |
|---|---|---|---|---|---|---|---|---|
| 2026-09-21 (was vendored) | 0.6.2 | 123 | 2,176 | $16,078.68 | $631.77 | 106.7M | $0.79 | $7.79 |
| 2026-10-09 (vendored now) | 0.8.0 | 982 | 3,890 | $20,148.49 | $711.26 | 148.6M | $0.32 | $0.82 |
| 2026-10-09 (control) | 0.6.2 | 982 | 3,245 | $20,146.69 | $838.74 | 148.2M | $0.32 | $0.82 |

The control row is the published v0.6.2 binary (sha256 `c042ca60...`
verified) reading today's corpus. Both builds count 982 sessions and the
same median, so the move from 123 is the corpus growing (2,206 transcript
files before 2026-09-22, 3,917 now), not a change of unit. The two builds
differ in lanes (0.8.0 names compaction segments) and in re-billed dollars,
which is the accounting that changed between the releases. The total moved
by a few cents between the dry run and the vendoring because the session
doing the work writes transcripts into the corpus it reads; the file
vendored into both repositories is the same bytes.

### 3.4 Deployment plan, awaiting authorization

Nothing below has been run. Each site is one command, each has a
verification that cannot pass on the old copy, and each has a rollback.

**replay.doctor.** On a checkout of `fix/withdrawn-offer-and-corpus-0.8.0`
(or of `main` after that branch is merged): `npm ci`; `REPLAY_REPO=<a Replay
checkout> npm run build`; `npm test`; `npm run deploy` (runs the D1
migrations, none new here, then `wrangler pages deploy dist --project-name
replay-doctor --branch main`). Verify: `curl -s https://replay.doctor/ |
grep -c '22,000'` is 0; the same for `/vendor/`; the landing meter foot
reads `REPLAY COST 0.8.0` and `READ 2026-10-09`; `curl -sS -D -
https://replay.doctor/replay.sh -o /tmp/i.sh | grep -i x-replay-installer`
and `shasum -a 256 /tmp/i.sh` both read `74a13afd...`. Rollback: redeploy
the previous `main`.

**redrobot-jp.** On a checkout of `fix/replay-claims-and-corpus-0.8.0` (or
of `main` after merge): `npm ci`; `npm run build`; `npm test`; `npm run
check:deploy`, which refuses unless `origin/main` and the live production
commit are both ancestors of the checkout; then `npm run deploy:cf`. The
branch descends from `origin/main` at `63cf4d6`, so the first question
passes; whether the live commit is on that lineage can only be answered by
the check itself, and if it refuses, the fix is to rebase the branch onto
the live lineage, never to skip the check. Verify: `/Replay/` carries no
"adversarial reviewer" and reads "v0.8.0 is tagged"; `/replay-doctor/`
carries no "week"; both installer paths on redrobot.jp hash to
`74a13afd...`. Rollback: redeploy the previous `main`.

**Hashes to re-check after deploy.** The three copies of the installer
(repository `install.sh`, replay.doctor, redrobot.jp) must all read
`74a13afd523dca3efffea203d18bc02c22e7a9d1233a64d8ee7219ffbff4d8a8`.

## 4. Protected paths and evidence integrity

| Check | Result |
|---|---|
| `git diff 0ea7370 HEAD` on the five TTL files | empty |
| `shasum -a 256 -c docs/evidence/ttl-register-2026-10-05.sha256` | OK |
| `~/replay-experiment/` | no file newer than this pass's first scratch file |
| `internal/claims/claims.go`, `CLAIM-REGISTER.md` | unchanged since `0ea7370`; the register was not regenerated because nothing it reads changed |
| `.markdownlint-cli2.yaml` | unchanged; exemptions by filename only |
| E9 and E10 records, frozen dated records | unchanged |
| Shared stash | 3 entries, untouched |
| Tags | 16, untouched |

Corrections in this pass are recorded here, in a new dated file, and in
the two commits' messages; no frozen record was rewritten.

## 5. External security review: procurement package, nothing sent

No email, form, application or post was sent; no party was contacted; no
cost was incurred. Every candidate below is uncontacted and every figure
is an estimate unless it names its source.

### 5.1 Funding programmes, eligibility read on 2026-10-09

| Programme | What the page says | Replay's fit |
|---|---|---|
| Sovereign Tech Agency, Sovereign Tech Fund | Code "must be licensed such that it may be freely reusable, changeable, and redistributable", "OSI-approved or FSF Free/Libre licenses"; security audits funded when necessary; activities must exceed EUR 50,000; about six months from submission to contract; "currently not looking for user-facing applications" | **Not eligible on licence.** Replay is BUSL-1.1 (`CITATION.cff`, `LICENSE`), which is neither OSI-approved nor FSF-free. Not worth an application |
| GitHub Secure Open Source Fund | "Clear open source license", "demonstrated community traction and adoption", $10,000 per project plus a three-week programme; rolling applications | **Not eligible on traction, and doubtful on licence.** Zero external users have ever been observed (this repository's own records); BUSL-1.1 is not what "open source license" ordinarily means. It funds training more than audits |
| OSTIF | No licence criterion on the intake page; "audit expenses typically range anywhere from $30k to $200k"; OSTIF "helped many projects raise funds and source sponsors"; process starts with a conversation through an intake form; 90-day disclosure | **The one programme route worth one message.** Whether a BUSL-1.1 single-maintainer tool qualifies is not stated and would be the first question. Uncontacted |
| Alpha-Omega | page returned no readable content in this environment | Not verified; historically funds through foundations rather than unsolicited single projects, which is an inference and labelled so |

**Finding.** The funded-programme route is closed on licence for the
largest fund and on traction for GitHub's; OSTIF is the only one whose
page does not exclude Replay on its face. A direct engagement is the
realistic path, and it costs money that is not authorized.

### 5.2 Two independent reviewer candidates, uncontacted

| Candidate | Why they fit section 3 of the scope | Independence check | Contact route on their site |
|---|---|---|---|
| ADA Logics | "find and fix vulnerabilities in critical software"; disclosed vulnerabilities in Kubernetes, Istio, Node.js, which are Go-heavy and network-facing; they audit through OSTIF and publish | no relationship with Daniel Saito or RedRobot K.K. known to this record; Daniel confirms in writing before any engagement (scope section 6.2) | contact form, "respond within one business day" |
| Radically Open Security | "Non-Profit Computer Security Consultancy"; code audits and penetration tests; publishes reports for open-source clients as a matter of model | same | `info@radicallyopensecurity.com` |

Both are categories 1 and 3 of the scope document's section 7. Neither
has been asked anything.

### 5.3 Request for quote, draft, one body for both addressees

> **Subject:** Request for quote: focused external security review of Replay
> (Go CLI and local proxy) before a 1.0 release
>
> **Who is asking.** RedRobot K.K., Tokyo (Daniel Saito, maintainer).
> Replay is a local Go tool, about 200 source files, zero third-party Go
> dependencies, BUSL-1.1, public at github.com/RedRobotKK/Replay. We are
> asking for a quote, not placing an order; no engagement exists until a
> written scope and price are accepted in writing.
>
> **Scope (section 3 of docs/evidence/security-review-scope-2026-10-08.md,
> which we will attach verbatim).** (1) The local HTTP proxy's
> network-facing guards: loopback binding, browser-origin refusal, the
> upstream leg being HTTP/1 only by construction, request and response
> handling against a hostile LAN peer or local process. (2) The
> secret-masking path end to end: detection, vault encryption, key storage,
> TTL and eviction, and what it cannot catch by shape. (3) The ledger's
> on-disk record format, content-free by design: try to recover message
> content from it. (4) The self-update path: fetch, checksum, Sigstore
> verification, execute. (5) The installer's checksum verification.
> (6) Provenance semantics: that every figure the tool prints is labelled
> measured, estimated, structural or not measured, and that no path can
> print a measured label on an estimate. Threat model: a trusted single
> operator on a trusted host; a hostile local process or LAN peer reaching
> the loopback listener; a compromised host is out of scope by design and
> disclosed as such.
>
> **Deliverables (section 6 of the same document).** A published report at a
> stable URL in your own name; your own statement of what was tested, not
> merely read; for each finding a severity and, once we respond, either a
> commit reference or a dated accepted-risk decision; a short closing
> statement if nothing material is found, because the negative result is
> what the release gate needs. We link the report from RELEASE-CRITERIA.md
> without editing its substance.
>
> **Timeline.** We would like a start within the next six weeks and a
> report within three weeks of start. Please state your earliest start,
> your effort estimate in person-days, and the review window you need.
>
> **Conflicts of interest.** Please confirm in writing that nobody on the
> engagement has a prior commercial or personal relationship with Daniel
> Saito or RedRobot K.K., and that payment will come from RedRobot K.K.
> alone, so the independence of the report is checkable later rather than
> asserted.
>
> **Cost.** Please quote a fixed price or a day rate with a cap, in USD or
> EUR, and say what a 20% scope reduction (dropping items 5 and 6) would
> change. For our own planning we are using OSTIF's published range for
> audits, $30,000 to $200,000, and expect a review of this size to sit at
> the low end; that is our estimate, not a budget, and no budget is
> approved yet.
>
> **Existing material.** docs/evidence/security-review-2026-09-04.md is a
> self-authored internal review and carries a provenance correction saying
> so; it is a map, not a substitute. Please re-verify rather than take it
> on trust, which is the standard this project holds itself to.

**Cost, labelled.** The only sourced figure is OSTIF's published $30,000
to $200,000 range for audits. This record's own estimate for a three-week
review of the six scope items by a boutique firm is $25,000 to $60,000;
that is an estimate with no quote behind it. No spend is authorized at any
amount.

### 5.4 Recommended review scope

Items 1 to 6 of the draft above, in that order of priority, with the
proxy's network guards and the masking path as the two that a 1.0 cannot
ship without. Item 6, provenance semantics, is the one no generic audit
would include and the one this product's claims rest on.

## 6. Second provider: recommendation

**State (RELEASE-CRITERIA.md, corrected 2026-10-08).** Typed,
mutation-tested `CacheRules` for OpenAI's published pricing exist
(`internal/cachemodel/openai.go`, `AstraRules()`); no non-test caller
selects them; the OpenAI-compatible proxy path is verified against a test
stub only and labelled `EXPERIMENTAL, UNMASKED`. The gate is rules **and** a
calibration corpus against real traffic; only the rules exist.

| Option | What it needs | What it buys | What it costs |
|---|---|---|---|
| A. Fund a calibration corpus | OpenAI API spend (none authorized, no credentials in this environment); real sessions through the proxy against the live provider; the provider's own usage fields to reconcile against; a machine that is not this one, or the independence problem of the first provider repeats | the gate as written: a dated calibration record with a match rate for the second provider, the counterpart of the 97.46% over 28,135 turns the first provider has | money, weeks, and a second corpus to keep current |
| B. Dated amendment | one record in `docs/evidence/`, one row correction in RELEASE-CRITERIA.md, one sentence in the README and docs where "second provider" is promised | an honest 1.0 whose claim is "one provider verified end to end, a second modelled and unverified"; the path keeps its EXPERIMENTAL label | nothing, and the gate as written is not met, so the criteria document must be narrowed rather than ticked |

**Recommendation: B for 1.0, A as the first 1.x gate.** The 1.0 definition
in the roadmap is claims checked by somebody other than the person who
wrote them; a calibration corpus produced on this machine with this
operator's key would not meet that definition even if it were funded, so A
does not close the gate alone. The amendment states the limitation the
product already states at runtime. **Minimum evidence for A, when it is
funded:** at least 20 sessions of real traffic through the proxy against the
live provider, the provider's usage fields reconciled turn by turn, the
match rate published with its denominator, and the record signed by
whoever ran it, on a machine that is not the maintainer's. This is a
recommendation; the choice is Daniel's and no amendment was written.

## 7. Naming, option C, applied on the branch

Commit `1ace6f6`: `CITATION.cff` abstract gains "Distributed as
replay-doctor on npm and PyPI and at replay.doctor; same binary."; the
marketplace description is prefixed "Replay (published as Replay Doctor on
npm, PyPI and replay.doctor; same binary): ". The exact prepared text from
the council record (G-9, G-10). README.md, the plugin manifest and the
skill already carried the note, so every name-bearing file in the tree now
says the same thing. This records a fact and decides nothing; no ADR
records a naming decision, the decision is Daniel's, and the commit reverts
in one command. GitHub About (G-8) is a repository setting and untouched.

## 8. Compatibility surfaces, named from the tree

Commit `db0781e`: `docs/ROADMAP.md`'s v1.x bullet now carries the list it
said it had to carry, each line verified on 2026-10-09: 34 dispatch
surfaces (`production-grade-matrix.md`), the exit codes frozen 2026-09-13
(`docs/guide/commands.md`), fourteen `--json` schema strings (every
`replay.*.v*` literal in non-test Go), the corpus, pool and watch records,
the ledger record and budget artefact (both schema 2), and the policy file,
which carries no schema string and is named so the absence is on record.
ADR-0024 carries the same list and stays **Status: Proposed**; accepting it
is Daniel's decision and this pass does not make it. Adjacent, not fixed:
`cmd/replay/main.go` says the exit codes are "Published in docs/CLI.md";
the table is in `docs/guide/commands.md`.

## 9. Two decisions taken in this pass, simulated panel, recorded

**Decision 1: push the pin commit while run 2 was still running, or wait.**
Evidence: the branch's concurrency policy cancels in-progress runs; frozen
mutants takes about an hour; the pin commit changes two `ci.yml` lines and
no Go. Alternatives: wait about 45 minutes for run 2's mutants result, then
push, doubling the wall time; or push and let run 3 be the matrix. Binding
recommendation: push, because the qualification evidence must be on the
final commit either way and run 3 contains the same tree. Dissent, noted:
a run 2 mutants result would have isolated the toolchain from the pins;
the loss is one data point, not a gap in the final evidence, since the pins
cannot affect what mutants do.

**Decision 2: apply the naming note and the compatibility list on the
branch, or hand them over as patches.** Evidence: three surfaces already
carry the naming note; ADR-0024 already names the surfaces; Daniel's
section 2 authorizes documentation changes where the council's
recommendations support a safe, unambiguous change, and section 6 says to
prepare the naming changes without touching GitHub About. Alternatives:
patch files in the scratch directory; commits on the branch. Binding
recommendation: commits on the branch, each reverting in one command, with
the decisions themselves (the name; ADR-0024's status) left explicitly
open. Dissent, noted: a product owner would want to state the name first
and have the files follow; the commits do not pre-empt that because they
state the fact of two names, which is true under every option.

## 10. Blocker board

| ID | Finding and source | Closure condition | Action and commit | Evidence | Independent verification | Remaining risk or dependency | Daniel's authorization still required |
|---|---|---|---|---|---|---|---|
| B-01 | govulncheck red, 9 reachable (council C-4) | 16-job run green on the new toolchain | `bbee9f6`, `8d79db9` | run 3: govulncheck "No vulnerabilities found"; 15 jobs green, mutants appended in section 12 | job logs read, not the summary | release compiler changes at the next tag | **no** for the branch; merge is separate |
| B-02 | Markdown lint | job green | done in the prior pass | green on runs 1 to 3 | run logs | none | no |
| B-03 | external review not commissioned (C-1) | published independent report | package prepared, section 5; nothing sent | n/a | n/a | lead time of weeks; money | **yes**: choose a candidate, authorize contact, authorize spend |
| B-04 | second provider, rules without corpus (C-2) | corpus or dated amendment | recommendation B, section 6; nothing written | n/a | n/a | the claim must be narrowed if B | **yes**: choose A or B |
| B-05 | reproducibility on the 1.0 tag (C-3) | four hashes equal on the tag | sequenced after the tag | v0.8.0 reproduced on 2026-10-05 | prior record | compiler is now go1.27.2 | after the tag |
| B-06 | candidate not on `main` (C-5) | merge | not done | `origin/main` = `5c87562` | `git rev-parse` | ruleset requires the six checks | **yes**: merge |
| B-07 | withdrawn offer live at three locations (C-6) | strings absent live | sites corrected on feature branches, `42fb99b`, `003f711` | built pages clean | grep of `dist/` | not deployed | **yes**: deploy |
| B-08 | public independent-review claim (C-7) | phrase absent live | `003f711` | built page clean | grep of `dist/` | not deployed | **yes**: deploy |
| B-09 | compatibility surfaces unnamed (C-8) | list exists | `db0781e` | ROADMAP carries the verified list | ADR-0024 Decision matches | ADR-0024 still Proposed | **yes**: accept ADR-0024 |
| B-10 | installer parity on redrobot.jp (P1) | both paths hash `74a13afd` | `003f711` vendors `main`'s installer | `check:installer` current | sha256 of the vendored file | not deployed | **yes**: deploy |
| B-11 | stale figures (P2) | page version equals the release | `42fb99b`, `003f711` re-vendor from v0.8.0 | `check:replay` current | v0.6.2 control reading | not deployed; moves again at the next release | **yes**: deploy |
| B-12 | naming note on three surfaces (P3) | note present | `1ace6f6` for G-9 and G-10 | files | diff | G-8 GitHub About untouched | **yes**: the name itself, and G-8 |
| B-13 | HTTP/2 disposition | pinned | prior pass | test in tree | passes under go1.27.2 | none | no |
| B-14 | generator artefacts | fixed | prior pass | lint 0 | runs 1 to 3 | none | no |
| B-15 (new) | tool pins unreadable under go1.27.2 | both jobs reach a verdict | `8d79db9` | run 3 green | job logs | none | no |
| B-16 (new) | `vendor-corpus.mjs` crash on current binaries (redrobot-jp) | script completes | `003f711` | script output | re-run | none | deploy only |
| B-17 (new, adjacent) | HubSpot deal builder prices a "Forensics week" at $22,000 (`functions/_lib/hubspot.js`, replay.doctor) | decision: remove or re-scope | not changed; backend, not public copy | test still asserts the amount | read | a confirmed waiting-list member who ticks the forensics box still creates a priced deal | **yes**: decide |
| B-18 (new, adjacent) | replay.doctor pulls docs at `replay.ref` = `48c9974` (0.6.2) | ref bumped and new command pages added | not changed | `tests/docs.test.mjs` would require pages for new verbs | read | docs on the site are two releases old | decide |

## 11. Minimum decisions needed from Daniel

1. **Merge** `fix/compaction-observed-vs-inferred` into `main` (B-06), after
   reading run 3's terminal result.
2. **Deploy** both site feature branches, or merge them to their `main`
   first and deploy from there (B-07, B-08, B-10, B-11).
3. **External review**: pick a candidate from section 5.2 or name another;
   authorize the message; later, authorize the spend against a written
   quote (B-03).
4. **Second provider**: A or B from section 6 (B-04).
5. **Naming**: confirm option C or choose otherwise, and set GitHub About
   (B-12, G-8).
6. **ADR-0024**: Proposed to Accepted, or amend the list (B-09).
7. **HubSpot deal builder**: remove, or state what it now represents (B-17).

## 12. Verdict

**NO-GO for v1.0, by the project's own criteria.** The decisive
conditions, each with its evidence:

| Condition | State | Evidence |
|---|---|---|
| External security review, published and independent | not commissioned | RELEASE-CRITERIA.md row; section 5 of this record; nothing sent |
| Second provider: rules and calibration corpus | rules only | RELEASE-CRITERIA.md corrected row; `AstraRules()` has no non-test caller |
| Candidate on `main` with the required checks | branch only | `origin/main` = `5c87562` |
| Public surfaces free of the withdrawn offer and the false review claim | corrected in source, live pages unchanged | section 3; no deploy |
| govulncheck green on the shipped toolchain | met | run 3, `8d79db9` |
| Frozen mutants, guard reachability, four Go legs, lint, blueprint, drift, x402, bench, tui, merge result | met except as appended below | run 3 |
| Compatibility surfaces named | met in ROADMAP; ADR-0024 still Proposed | `db0781e` |

Nothing here was merged, tagged, published, deployed or paid for.

**Frozen mutants on run 3, appended when the job reached a terminal
state:** still running when this record was last written (started 2026-10-09T15:46:57Z, job 113900673640); the terminal result is to be appended by whoever reads it, and the four local commits of this pass are not pushed until then, because a push to this branch cancels the in-flight run.
