# Release evidence packet, 2026-10-09: candidate 2152c08, read from the logs

**What this answers.** Whether candidate `2152c083d4549721bfb131d1f107d7fd7cf32912`
(branch `fix/compaction-observed-vs-inferred`, PR #336) has a defensible
engineering verdict, a defensible review verdict, and a v1.0 verdict, each
stated separately and each tied to the run, job or file it rests on. It
supersedes nothing: the [closure ledger](closure-ledger-1.0-2026-10-09.md)
of the same day remains the record of candidate `37c0d0a`; this packet is
the record of the three commits on top of it and of the CI run that covers
the exact head.

**Verdicts, stated first.**

| Question | Verdict | Rests on |
|---|---|---|
| Engineering CI on `2152c08` | **PASS** | run 37984135531, 16 of 16 success, read job by job (section 2) |
| PR #336 review | **READY FOR DANIEL'S MERGE DECISION** | sections 4 and 5; no release-blocking finding; a merge-ready verdict is not merge authorization |
| v1.0 release | **NO-GO** | C-1, C-2 and C-5 remain blocked (section 6); none is waived here |
| v0.9.0 release | **Recommended as a separate decision, after the merge** | section 8; no tag is created by this packet |

Nothing in this pass was merged, tagged, published, deployed, contacted or
paid for. One push was made, to the candidate branch, carrying this packet
and its index row; the run it started is recorded in section 2.4 as it
stood when this file was last written.

## 1. Candidate identity, verified 2026-10-09 20:46 to 20:58 UTC

| Item | Value |
|---|---|
| Repository | `RedRobotKK/Replay`, worktree on branch `fix/compaction-observed-vs-inferred` |
| HEAD at packet start | `2152c083d4549721bfb131d1f107d7fd7cf32912`, equal to `origin/fix/compaction-observed-vs-inferred` |
| Commits on top of the ledger's `37c0d0a` | `97f2c20` (evidence index repair, `TestEI1`), `e852135` (four records and their index rows), `2152c08` (one source comment in `cmd/replay/main.go`); 66 commits ahead of `origin/main`, 0 behind |
| `origin/main` | `5c8756260ca0dcf28ee566be1c11172147338c2e`, which `v0.8.0` resolves to |
| Tags | 16, `v0.1.0` to `v0.8.0`, unchanged |
| Working tree, stash | clean; 3 pre-existing stash entries, untouched |
| PR #336 | OPEN, draft, base `main`, `mergeable: MERGEABLE`, `mergeStateStatus: BLOCKED` (draft, and no review), 0 reviews, head `2152c08` |
| Ruleset 23126317 on `main` | required checks Go (ubuntu-latest), Go (macos-latest), golangci-lint, guard reachability, frozen mutants, merge result; required signatures; linear history; no deletion; no force push |
| `go.mod` | `go 1.24`, `toolchain go1.27.2`; the authorized baseline, unchanged by this pass |
| Catalogue | `internal/mutation/testdata/mutants.json`, sha256 `bba10c8c5ff026f92eb3f5970a888020f19617b3dfb4a8bbb32e5bca26080dac`, 144 entries, ids M1 to M145 with M71 absent by retirement, one named known gap not counted |
| Diff against `origin/main` | 173 files, +16154 / -758; 44 non-test Go files (+3296 / -367), 70 test files, 50 Markdown files; non-Go, non-Markdown: `ci.yml`, `.markdownlint-cli2.yaml`, `go.mod`, `CITATION.cff`, two plugin manifests, the three TTL evidence data files, `guard-evidence.json`, `mutants.json`, one Codex fixture |

## 2. CI evidence

### 2.1 The run that covers the exact head

CI run 37984135531, `pull_request` on #336, head `2152c08`, created
20:01:04 UTC, status `completed`, conclusion `success`, 16 of 16 jobs
success, last job finished 20:55:37 UTC. Read with `gh run view --json`
and then per job from the job logs, not from the check rollup.

| Job | Job id | Conclusion | Evidence line, from the log |
|---|---|---|---|
| frozen mutants | 114001737101 | success | `go version go1.27.2 linux/amd64`; `go test -tags mutation -timeout 90m -count=1 ./internal/mutation/` reported `ok internal/mutation 3180.820s`; `go test -tags mutation -timeout 30m -count=1 ./internal/blackbox/` reported `ok internal/blackbox 71.559s`; ran 20:01:06 to 20:55:37; no `-short` |
| guard reachability | 114001737176 | success | `guard-reachability: 242 guard(s), 5 survived (1 pre-existing, 4 introduced), 4 evidenced, 0 unexplained, 0 unchecked, 0 not built here`; the four introduced survivors carry per-guard evidence in `internal/guardcheck/testdata/guard-evidence.json`; ran 20:01:06 to 20:16:18 |
| Go (ubuntu-latest) | 114001737236 | success | coverage gate, tidy, vet |
| Go (macos-latest) | 114001737168 | success | race suite |
| Go (windows-latest) | 114001737416 | success | the refusal path |
| Go (latest release) | 114001737404 | success | `stable` canary |
| golangci-lint | 114001737046 | success | v2.14 |
| govulncheck | 114001737371 | success | v1.8.0 |
| merge result | 114001737273 | success | the merge compiles |
| Markdown lint | 114001737473 | success | |
| surface drift, installer drift, cli blueprint, tui layout audit, x402 end to end, ledger bench | 114001737288, 114001737401, 114001737083, 114001737197, 114001737480, 114001737228 | success | |

The run before it, 37961162745 on `37c0d0a`, is recorded in the closure
ledger section 3 with its coverage line (`89.2%`, floor `85%`), its lint
line (`0 issues.`) and its govulncheck line (`No vulnerabilities found.`).
The three commits between the two heads touch no Go source outside one
comment and one regression test, so those figures are not expected to have
moved; they were re-read locally on `2152c08` in section 2.3 rather than
assumed.

### 2.2 Mutation outcome counts, and what the CI log does and does not say

The CI log says `ok`. It does not print the per-mutant counts: the harness
prints `N catalogued, k mutants: killed, survived, stillborn` with
`t.Logf`, which `go test` shows only under `-v` or on failure, and the
workflow runs without `-v`. So the separate counts below come from two
things, named:

1. The harness semantics audited in the ledger section 2:
   `TestFrozenMutantsStillDie` calls `t.Fatalf` on a stillborn mutant and
   on a red baseline, `t.Errorf` on a survivor, and a timeout fails the
   job. An `ok` therefore means every catalogued mutant was applied,
   compiled, and killed by its named test, and none was skipped.
2. A bounded local run on this exact SHA with `-v`, so the count line is
   visible: section 2.3.

Counts for `2152c08`: catalogued 144, killed 144, survived 0, stillborn 0,
timed out 0, skipped 0, known gap 1 (`metrics-shutdown-drain-not-awaited`,
listed in `knownSurvivors`, not in the 144). The 135 of earlier runs and
the 144 of this one are two versions of one catalogue; the reconciliation
in the ledger section 2 was re-derived here from `git show` at `9694f1e`
(115), `3903712` (135) and `0320747` onward (144), and nothing was
substituted.

**One finding on the catalogue's own prose, not fixed here:** its `note`
field still says "the denominator is 72 hand-chosen edits". The number is
stale twice over (135, then 144). Changing the file changes the catalogue
hash the frozen-mutants job is identified by, so the repair belongs in a
commit of its own after the merge, not inside the candidate.

### 2.3 Local gates on `2152c08`, this host (darwin/arm64, go1.27.2)

Log: session scratch `closure4/gates-local-2152c08.log` and
`closure4/mutant-sample-2152c08.log`.

| Check | Exit | Output |
|---|---|---|
| `gofmt -l .` | 0 | nothing listed |
| `go vet ./...` | 0 | |
| `go build ./...` | 0 | |
| `go mod tidy -diff` | 0 | no diff; no `go.sum`, no `require` block |
| `go test -race -count=1 -coverprofile ./...` | 0 | 40 packages `ok`, 0 `FAIL`; total `89.2%` |
| `scripts/coverage-gate.sh` | 0 | `coverage: 89.2%  floor: 85%  headroom: +4.2` |
| `go run scripts/claim-register/main.go` then `cmp` | 0 | `wrote CLAIM-REGISTER.md: 27 claims`; byte-identical to the committed file |
| `npx --no-install markdownlint-cli2` | 0 | `Linting: 241 files`, `Summary: 0 issues in 0 files` (before this packet was added; the packet and its index row were linted separately before the commit) |
| `go test -tags mutation -count=1 -v -run 'TestFrozenMutantsStillDie/^M14[0-5]_' ./internal/mutation/` | 0 | `144 catalogued, 6 mutants: 6 killed, 0 survived, 0 stillborn`; `ok internal/mutation 115.957s`; catalogue `bba10c8c…` |

The full catalogue was not re-run locally; the documented workflow had
already produced the result on this exact SHA in the job above, and the
bounded run exists to make the count line visible, not to replace it.

### 2.4 The run this packet's own push starts

This packet is committed before it is pushed, so the run its push starts
cannot be named inside it. Its id, its terminal state and its
frozen-mutants log line are recorded in the handback of this pass, and
the next record in this directory should carry them. Until that run is
terminal, the engineering verdict above rests on `2152c08`; the commit
that adds this packet and its index row changes no Go source, no test, no
workflow and no catalogue.

## 3. Integrity and supply chain

| Item | Result |
|---|---|
| Toolchain | `go 1.24` / `toolchain go1.27.2` in `go.mod`; `go.sum` absent (zero third-party Go dependencies); the mutants job log prints `go1.27.2 linux/amd64`; the `Go (latest release)` canary is the one job on `stable`, by design |
| Protected TTL evidence | all six files under `docs/evidence/ttl-*` have the same blob id at HEAD and at `0ea7370`: `ttl-register-2026-10-05.md` `762ecb91`, `.sha256` `5dbf89d6`, `ttl-schedule-2026-10-05.json` `9588c22e`, `ttl-blocks-2026-10-05.jsonl` `db1ed7a6`, `ttl-amendment-2-2026-10-07.md` `b1a125d4`, `ttl-control-boundary-2026-10-05.md` `3a1bcbe5`; `shasum -a 256 -c ttl-register-2026-10-05.sha256` reports OK |
| Other protected paths | `~/replay-experiment/` not opened; `internal/claims/claims.go` not edited by this pass (the candidate's own changes to it, RPL-C038 and RPL-C039, are part of the reviewed diff); `CLAIM-REGISTER.md` regenerated and compared in section 2.3; `.markdownlint-cli2.yaml` not edited by this pass (the candidate adds 33 by-name exemptions for frozen records, listed with the reason) |
| Secrets sweep of the diff | no key-shaped string, no bearer token, no private-key block in any added line; three e-mail addresses, all public: a firm's published contact address inside one closure record, the repository's git remote, and a GitHub noreply address |
| Dependency and vulnerability evidence | govulncheck v1.8.0 green on the run; golangci-lint v2.14 green; both pins moved in `8d79db9` for the reason the workflow comment gives |
| Generated artifacts | `CLAIM-REGISTER.md` byte-identical to its generator's output; `production-grade-matrix.md` regenerated by `TestBB_Matrix` in the suite; no coverage, log or binary file in the diff |
| Credentials, spend, contact | no provider credential exists in the environment; no paid API was called; no message was sent; production configuration untouched; the site's production `.env` was not read |

## 4. PR review of the exact diff

Reviewed from `git diff origin/main...HEAD`, the 44 production Go files in
full (5059 diff lines), the workflow and lint-config diffs, the three
newest commits line by line, and the Markdown by grep for figures, names
and phrases. Nothing was taken from commit messages alone.

### 4.1 What the candidate changes

- **Tenancy (SP-5 to SP-10).** `internal/tenancy` is new: a validated
  `TenantID`, `ResolveTenant("")` returning `LocalTenant` so every existing
  install is unaffected, a length-prefixed scoped key. The proxy resolves
  `x-replay-tenant-id` before the breaker and refuses an unresolvable value
  (`replay_tenant_unresolved`), strips the header before forwarding, and
  nests the spend guard's session and day accumulators, the session table
  and the ledger's policy pins by tenant. `spend-day.json` gains a
  `tenants` map and still reads the pre-tenancy flat shape. `SpendGuard`
  gains a bounded retired table (SP-8) and refuses a never-seen session
  once both tables are full, fail closed.
- **Compaction (M138, M139, M140).** A lane rooted at a compaction is a
  continuation segment, not a sub-agent; `replay context` prints one line
  per boundary with the client's own sizes and the first prompt after; an
  observed-ceiling table is built per model and withheld under the panel's
  gate; `advise` sets aside a session attributed above its bill.
- **Interventions (M141, M144).** `advise --apply --yes` records each
  request to `~/.replay/interventions.jsonl` as refused, attempted or
  verified by read-back; `--json` says `applied` only on `VERIFIED`.
- **TTL scoring (M142, M143).** The tie floor compares a share to a share;
  the TTL family is scored on main-thread lanes only.
- **Codex (M137).** A credits-based limit with no window is printed as
  such; the reset instant is shown.
- **Purge (M145).** `--older-than` acts only on directories the registry
  names as ledger stores.
- **Second provider (RPL-C038, RPL-C039).** `RulesForModel` dispatches the
  Astra tier at the three production classification sites; the
  calibration claim is registered as not measured.
- **Tooling.** Guard reachability routes packages absent from the base ref
  out of the base test batch and still fails on a broken checkout; the
  claim register renders from one function; `scripts/ttl-block` is the
  field-study tool.
- **Supply chain and docs.** go1.27.2; lint and govulncheck pins; the 33
  by-name lint exemptions; the compatibility list in the roadmap; naming
  note in CITATION.cff and the manifests; the withdrawn offer removed from
  README.md; the two self-authored reviews relabelled; the evidence index
  repaired; the four 2026-10-09 records.

### 4.2 Findings, by severity

No finding below blocks the merge. Each names the file and the smallest
remedy.

| # | Severity | Finding | Where | Remedy |
|---|---|---|---|---|
| F1 | Low, correctness | `purge --older-than` decides by directory basename (`ledger`, `ledger-<name>`, `archive`), not by location under `~/.replay`. A directory named `ledger` anywhere is accepted. The refusal protects `~/.replay` itself, which is the defect M145 froze; it does not protect an unrelated directory that happens to share the name | `cmd/replay/purge.go` `isLedgerDir` | state the rule in the command's help text, or additionally require the parent to be the registry root; either after the merge |
| F2 | Low, doc accuracy | the catalogue's `note` says 72 hand-chosen edits; the catalogue holds 144 | `internal/mutation/testdata/mutants.json` | separate commit after the merge (section 2.2) |
| F3 | Low, doc accuracy | `ResolveTenant`'s doc comment says "This unit does not wire a live request path to it", while the package comment above it and `passthrough.go` say it is wired | `internal/tenancy/tenancy.go` | one-line comment fix after the merge |
| F4 | Low, doc accuracy | `dayLeader`'s first paragraph still says eviction "discards that session's spend"; SP-8 below it says eviction demotes to the retired table | `internal/proxy/guards.go` | comment fix after the merge |
| F5 | Low, consistency | `advise` prints `%d calibrated, %d set aside` where the calibrated figure includes the set-aside sessions, while the advice file's `sessions` field excludes them | `cmd/replay/advise.go` | decide which the header means and say so; after the merge |
| F6 | Low, convention | the candidate adds em-dashes to living public documents (README.md 2, `docs/SURFACES.md` 2, `docs/requirements.md` 5, `docs/guide/commands.md` 1, `RELEASE-CRITERIA.md` 1, `docs/design/UNWIRED-LOG.md` 2, the evidence index 2) and to Go comments. The README already carried dozens before the candidate, so this is a standing convention gap and not a regression the candidate introduced; the site replaces them on pull and prints the count. No test in this repository holds documents to the rule; `internal/prose` holds only what the binary prints | listed files | a repository-wide sweep with a test, as its own change; not inside the candidate |
| F7 | Low, cost | `replay context` parses every Claude Code transcript twice, once to build the ceiling table and once in `forEachSession`; the comment accepts the cost | `cmd/replay/context.go` | none required; noted |
| F8 | Informational | `Session` gains two JSON-tagged fields (`client_versions`, `repository_id`) on a struct whose other fields carry no tags. Nothing in the tree serialises `Session` as JSON, so no output shape changes today | `internal/transcript/types.go` | none required |
| F9 | Informational, re-pin | at a site pin past `5c87562`, two site tests fail that the runbook section 6 does not name: the vendored-installer check (named in the ledger, clears after the merge) and `tests/timeline.test.mjs` "the timeline was derived from the fixture the captures manifest hashed, at the pinned commit", which needs the captures manifest regenerated for the new pin | site repository | add the regeneration step to runbook section 6 before the re-pin |

### 4.3 What was checked and found clean

Production behaviour changes are each backed by a test the changelog names
and, for the ones that froze a mutant, by a catalogue entry that CI
re-applies. The tenant header is validated by a bounded regex, reserved
words are refused, and the header is deleted before the upstream leg. No
new network surface. No new outbound path. No test was weakened; no
threshold, floor or lint rule was loosened; the only lint-config change
adds by-name exemptions for frozen records with the reason beside them.
The toolchain line moves once, in the authorized direction. The three
newest commits are what their messages say: an index repair with a test
that fails on the old file, four records plus four rows, one comment.
Merge conditions: the six required checks are green on the head; the PR is
a draft with no review, which is the state Daniel's decision changes.

## 5. Public evidence and B-22

### 5.1 What a site re-pin would newly publish

Dated evidence files at `2152c08` absent from `5c87562`: 21 (the 17 the
ledger counted at `37c0d0a`, plus the four records in `e852135`). Of
those, six quote the withdrawn fixed-fee figure or the retired call to
action as defects, or name two uncontacted security-review firms:

- `gtm-02-public-surface-retrofit-2026-10-08.md`
- `release-gate-1.0-2026-10-09.md`
- `closure-council-1.0-2026-10-09.md`
- `closure-phase2-2026-10-09.md` (also a firm's contact address)
- `closure-ledger-1.0-2026-10-09.md`
- `security-review-packet-1.0-2026-10-09.md`

Two residuals sit outside the evidence index and are named for a
decision, not withheld: the Unreleased changelog entry that quotes the
withdrawn figure in its removal context (the live `/docs/changelog/` page
would carry it after a re-pin), and `launch-prediction-2026-09-14.md`,
which has been live on the site with the figure since before the
withdrawal and is unchanged by the candidate.

### 5.2 The exclusion, prepared and not published

Site repository `RedRobotKK/replay.doctor`, local branch
`fix/evidence-index-exclusion-2026-10-09` from `origin/main` (`9da68d6`),
one commit `8a39d7b6a941d063cff9591958bb822f5518f1b8`, **not pushed, not
deployed**. `replay.ref` stays at `5c87562`; nothing live changes.

- `scripts/lib/withheld-evidence.mjs`: the six names, each with its
  reason; `partitionEvidence`, `withheldReason`, `dropWithheldRows`.
- `scripts/pull-replay-docs.mjs`: a withheld record is not written, gets
  no route (so every page that cites it links to GitHub, as an undated
  file already does), and its row is dropped from the generated index;
  each exclusion is printed by name beside the undated ones.
- `tests/docs.test.mjs`: one test over a fixture listing that carries all
  six names (non-vacuous at any pin); one test over the built site (no
  page, no content file, no index mention, no withdrawn figure or retired
  phrase on the index) that prints whether the pinned tree exercised it.

Verification, with `REPLAY_REPO` at this worktree:

| Pin | Build | Result |
|---|---|---|
| `2152c08` (test only, not committed) | exit 0 | `wrote 5 guides, 33 command pages, 113 evidence pages (2 undated and 6 withheld linked to GitHub instead), 27 decisions`; 115 index rows of 121; none of the six in `dist/` or the content tree; zero firm names anywhere in `dist/`; the rendered index has no raw pipe text (B-24 clears with the re-pin) |
| `2152c08`, full suite | exit 1 | the two new tests pass, printing `withheld records in the pinned tree at 2152c08: 6 of 6`; the two failures are the re-pin consequences in F9, unrelated to the exclusion |
| `2152c08`, three mutation controls | each red | index rows kept: "is listed on the evidence index"; records published: "was pulled into the content directory"; row filter disabled: "a withheld row survived"; restored to 8 of 8 |
| `5c87562` (committed pin) | exit 0 | 98 evidence pages, 0 withheld; `npm test` 356 of 356; `check:surfaces`, `check:docs`, `check:installer` exit 0, with the standing `external-redrobot-jp` NOT_MEASURED row the ledger already records |

Sanitization requirements, for whichever records Daniel later wants
published: replace the quoted figure and the quoted call to action with a
reference to ADR-0028's withdrawal, and replace the firm names and the
contact address with "two uncontacted candidates, named in the
procurement record". The records themselves are not edited by this pass.

## 6. Release blockers

Status is PASS, BLOCKED, NOT RUN or STALE; the SHA is the commit the
evidence was produced on.

| ID | Requirement | Status | Evidence, SHA | Owner | Next action |
|---|---|---|---|---|---|
| C-1 | External security review, published | BLOCKED | no reviewer engaged, nothing sent; packet at `2152c08`, RFQ internal draft in the session scratch (section 7) | Daniel, then an external reviewer | approvals A1 to A3 of the RFQ draft |
| C-2 | Second-provider rules and calibration corpus | BLOCKED | rules dispatched (RPL-C038), calibration not measured (RPL-C039), no corpus, no credential, `2152c08` | Daniel (option A or B of the calibration runbook), then a provider | choose A or B |
| C-3 | Reproducibility on the 1.0 tag | NOT RUN | no tag exists; v0.8.0 reproduces on four platforms under go1.25.13 | engineering, after a tag | not before a tag is authorized |
| C-4 | govulncheck green | PASS | run 37984135531 job 114001737371, `2152c08` | | none |
| C-5 | Candidate on `main` with required checks green | BLOCKED | `origin/main` = `5c87562`; six required checks green on `2152c08`; PR draft, unreviewed | Daniel | mark ready, review, merge |
| C-6 | Withdrawn offer absent from every public surface | PASS | re-fetched 20:47 UTC: replay.doctor `/`, `/docs/`, `/docs/trust/evidence/`, `/docs/changelog/`, `/docs/trust/`, `llms.txt`, `llms-full.txt`; redrobot.jp `/`, `/Replay/`, `llms.txt`; the only occurrences are the withdrawal notice itself on the landing page and in `llms-full.txt` | | none |
| C-7 | No public surface claims an external review happened | PASS | same fetches, the retired two-word external-review label absent everywhere; the pinned docs index at `5c87562` is B-20 | | none |
| C-8 | Compatibility surfaces named before 1.0 | PASS, with B-09 | roadmap list verified at `db0781e`; ADR-0024 still Proposed (section 8) | Daniel | accept or amend ADR-0024 |
| B-09 | ADR-0024 Proposed | BLOCKED, not release-blocking by the criteria | section 8 | Daniel | decide |
| B-20 | Six site docs pages carry `5c87562` wording | STALE | clears with the re-pin after the merge; add F9's regeneration step to the runbook | Daniel (merge, then deploy) | after the merge |
| B-22 | Re-pin publishes records that quote the withdrawn figure or name firms | PREPARED, not published | section 5 | Daniel | authorize the push and merge of `8a39d7b` on the site, then the re-pin |
| B-23 | Unreleased holds 22 entries against a rule of about twenty | PASS, cadence note | count re-run on `2152c08` | Daniel | cut a release (section 8) |
| B-24 | Evidence index rendered as pipe text | PASS in the tree, STALE live | `97f2c20` with `TestEI1`; the site build at `2152c08` renders one table | Daniel (re-pin) | with B-20 |
| New | Two re-pin tests the runbook does not name (F9) | OPEN | site suite at the `2152c08` pin | engineering | runbook section 6 |
| G-mut, G-guard, G-cov, G-claims, G-surfaces | | PASS | sections 2 and 3 | | none |

No historical entry was erased; the ledger's rows are carried with their
resolution.

## 7. Decision log

**Daniel's four binding decisions, verbatim in substance:**

1. PR #336: wait for final CI and review, then report and ask before
   merging. No merge authorization.
2. B-22: exclude the named records from the public evidence index pending
   sanitization; preserve the underlying evidence; do not publish or
   deploy the exclusion without authorization.
3. External security review: prepare the RFQ and supporting materials
   only; no contact, no sending, no engagement, no spend.
4. v0.9.0: a separate release decision after the merge decision and the
   ADR-0024 review; no tag.

| Kind | Items |
|---|---|
| Completed | CI run 37984135531 read to terminal state job by job; the exact diff reviewed; local gates re-run on `2152c08`; protected blobs re-verified; production surfaces re-fetched; B-22 implemented, tested, mutation-checked and committed locally on the site; this packet and its index row committed and pushed to the candidate branch |
| Prepared, not executed | the B-22 site commit `8a39d7b` (unpushed); the RFQ internal draft (session scratch `closure4/rfq-security-review-internal-draft-2026-10-09.md`); the ADR-0024 decision note and the v0.9.0 recommendation (section 8) |
| Awaiting Daniel | mark PR #336 ready and merge, or not; push and merge the site exclusion; accept or amend ADR-0024; choose a reviewer and authorize contact and spend; choose C-2 option A or B; decide the two B-22 residuals; decide v0.9.0 after the merge |
| External dependencies | a reviewer's report (C-1); a second provider's traffic or a dated amendment (C-2) |
| Prohibited and not done | merge; tag; publish; deploy either site; contact anyone; send the RFQ; spend; change the toolchain; waive or re-scope a criterion |

## 8. ADR-0024 decision note and the v0.9.0 question

### 8.1 ADR-0024, read in full at `2152c08`

Status **Proposed**, dated 2026-09-13, index row in `docs/adr/README.md`
agrees. The decision it asks for: a published surface is removed only
after a full minor release in which it still works and warns on stderr,
then removed with a changelog entry, then exits 1 naming its replacement
for one further minor release; `--json` fields may be added at any time
and never removed or repurposed without the same clock; adding a verb
needs a written argument for why it is not a flag.

| Question | Finding |
|---|---|
| Does the recorded decision match the tree? | The procedure matches nothing yet because nothing has been deprecated; no deprecation code path exists in `cmd/replay`, which is consistent with the ADR. The counts do not match: the ADR says 31 verbs, the roadmap's list (verified from the tree on 2026-10-09) names 34 dispatch surfaces, and `production-grade-matrix.md` holds 34 rows. The ADR's consequence "the surface should be reduced before 1.0" has run the other way: 30, then 31, now 34 |
| Alternatives | (a) Accept as written with the count amended to 34 and the growth acknowledged in a dated note; (b) accept and also act on the consequence by naming the verbs to retire before 1.0, which then needs its own minor release under the ADR's own clock; (c) leave Proposed and tag 1.0 anyway, which the ADR's own text says cannot be repaired afterwards |
| Consequences | (a) binds the maintainer to the two-release clock on 34 surfaces from the next tag; (b) delays 1.0 by at least one minor release; (c) converts 34 verbs into an open-ended promise with no exit |
| Evidence required before acceptance | the count corrected against `cmd/replay/main.go` and the matrix; a statement that the list in the roadmap and in the ADR are the same list (they are, as of `db0781e`); Daniel's signature on the status line. No test is needed for a policy ADR, but `command_table_test.go` already pins that every verb has a guide section, which is half the obligation |
| Release consequence | not a 1.0 gate under RELEASE-CRITERIA.md; a 1.0 gate under the ADR's own argument. The roadmap's 1.0 definition ("checked by somebody other than the person who wrote them") does not depend on it. Recommendation: (a) before any 1.0 tag; irrelevant to a 0.9.0 |

Not marked Accepted by this pass.

### 8.2 The 22 Unreleased entries, reviewed for accuracy and user value

Each entry was read against the diff. Every behavioural claim names its
test or its frozen mutant and matches the code reviewed in section 4;
the figures quoted (thirteen advisories, nine reachable, 237 lint
findings, 33 exempted files, 34 surfaces, 2,223 sub-agent transcripts,
46 compactions, 155%) are the ones the evidence records carry. Two
entries are notes rather than changes (the naming recommendation, the
commissioning options) and say so. One entry (the README offer removal)
quotes the withdrawn figure in its removal context, which is the first
B-22 residual.

User value, grouped: **security** (the shipped v0.8.0 binaries were built
by go1.25.13 and carry the nine reachable advisories; a release built by
go1.27.2 is the only way a user gets the fix); **correctness of printed
figures** (M137 to M145: a wrong quota window, a wrong lane count, a 155%
share, a dropped tie population, a sub-agent scored under a setting that
cannot reach it, a purge that could remove evidence, an `applied` that
meant `requested`); **tenancy** (invisible to a solo install by
construction, every pre-tenancy file still reads); **docs and claims**.

### 8.3 Recommendation on v0.9.0

**Worth releasing, as 0.9.0 and not 1.0, after and only after PR #336 is
merged.** The security reason alone justifies a tag: the published
binaries carry a standard library with nine reachable advisories and the
fix is in the candidate. The release workflow's ancestry gate
(`git merge-base --is-ancestor "$GITHUB_SHA" origin/main`) refuses a tag
off `main`, so the order is merge, then tag. No tag is created by this
pass; the runbook's three separate authorizations stand.

---

[Evidence index](README.md) · [Closure ledger](closure-ledger-1.0-2026-10-09.md) ·
[Release runbook](release-runbook-1.0-2026-10-09.md) · [Release criteria](../../RELEASE-CRITERIA.md)
