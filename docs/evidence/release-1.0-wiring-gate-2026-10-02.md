# 1.0 runtime wiring gate

**2026-10-02. Every PROVEN and BOUNDED 1.0 claim traced through four gates to
the binary a user runs. No production change. Nothing committed.**

The claim closeout of the same day gave 27 items a PROVEN or BOUNDED
disposition on the strength of a registered oracle. This gate asks a narrower
question of each: is the thing the oracle proves the thing the shipped
`replay` binary does, on a real run, with a test that would go red if the
production code stopped doing it?

Labels: **OBSERVED** is what a command printed or a test did. **READING** is
what that permits. Status words are the four the brief allows and no others.

---

## 0. Answer

**Are all 1.0 claims actually wired into the shipped execution path?**

**No.** 21 of 27 are. One is UNWIRED (RPL-C026: its deciding code,
`surface.Classify`, is not in the shipped dependency closure and has no
production caller). Five are ORACLE-WEAK (C001, C008, C021, C025, and the
PROOF row "refusals are cheap and explicit"): the behaviour ships and was
observed end to end, but the oracle the register names either tests a package
the binary does not link or did not go red under a relevant mutation. One
PROOF-1.0 row marked FIXED is E2E-FAIL: the text share card at
`cmd/replay/share.go:48` still says "of my agent spend" over a priced-only
denominator, and no test pins it.

Nothing in the failing rows is a wording change away from PROVEN; each is named
below with its broken link, the smallest repair, and the tests that repair
would touch. None was applied.

---

## 1. Gates and status words

| Gate | Question | Evidence accepted |
|---|---|---|
| 1 claim → surface | Which printed surface carries the promise? | the sentence or JSON key, by file:line |
| 2 surface → production path | Which function decides it, and who calls that function in the shipped closure? | `go list -deps ./cmd/replay`; a non-test caller in that closure |
| 3 production path → oracle | Does a test go red when the deciding code is mutated? | a mutation at the deciding site, run against the named oracle or the suite, restored byte-identical |
| 4 claim → E2E observable | Does `go run ./cmd/replay …` print it on a fixture? | the printed line, quoted |

Status words, as used in the matrix:

- **WIRED**: all four gates passed.
- **UNWIRED**: gate 2 failed. The deciding code is not reachable from the binary.
- **ORACLE-WEAK**: gates 2 and 4 passed; gate 3 failed. Either the registered oracle lives in a package outside the shipped closure (so it cannot test the production path by construction), or a relevant mutation at the deciding site survived the oracle. Where a different, non-registered test killed the mutation, that test is named; the status is still ORACLE-WEAK because the register points at the wrong guard.
- **E2E-FAIL**: gate 4 produced output that contradicts the claim.

A row that was not mutated in this gate is not WIRED. Every WIRED row below
names the mutation and the test that failed.

---

## 2. The shipped closure

**OBSERVED.** `go list -deps ./cmd/replay` contains 21 `internal/` packages.
Absent from it: `internal/surface`, `internal/quota`, `internal/stateledger`,
`internal/claims`. Present: `internal/money`. `surface.Classify` has zero
non-test callers under `cmd/` and `internal/` (the one grep hit is the string
in `internal/claims/claims.go:209`, which is the claim's own Scope text).
`internal/regression/unwired_packages_test.go:62` already records
`internal/surface` as "OPEN, and deliberately so for now."

**READING.** Any oracle in `internal/surface` tests a harness, not the binary.
That is the mechanism behind three of the five ORACLE-WEAK rows.

---

## 3. Fixtures

Hand-built schema-2 ledgers under `$CLAUDE_JOB_DIR/tmp/e2e/` (not committed):

- `priced/priced.jsonl`: one session, two `claude-opus-5` requests, 5,000
  cache-creation tokens each, so the second is a cache break.
- `unpriced.jsonl`: one session on `totally-unknown-model-x9`, 900,000
  creation tokens, so the deficit is large and unpriceable.
- `q01/s.jsonl` + `q01/agent-0badf00d.jsonl`: one session id across two files,
  the lane file on the unknown model.
- `internal/ledger/testdata/agent-lanes.jsonl` for the lane count.
- `replay since` takes no directory (`Usage: replay since [--peek]`), so it was
  run under a fresh `HOME` holding only the two ledgers. Four earlier attempts
  that passed a directory and printed "Nothing since you last looked" were an
  instrument error, not a product one.

---

## 4. The matrix

Disposition is the closeout's. Mutation column: the site mutated → the test
that went red (or SURVIVED). E2E column: the printed line.

| Claim | Disposition | Surface | Renderer | Deciding code | Live caller | Mutation killed | E2E | Status |
|---|---|---|---|---|---|---|---|---|
| C001 reads transcripts on disk, names the turn | BOUNDED | `replay diff`, `cost` | `cmd/replay/diff` render | `internal/transcript` readers; `internal/analysis` break detection | `runCost`, `runDiff` | not mutated at a C001 site; registered oracle `TestFixtureCorporaClassifyAsTheAuditFound` is in `internal/surface/probe_test.go:79`, outside the closure | `diff priced.jsonl` → "turn 1 at 12:00:01 (+1s): read 0 of 5.0k expected, 5.0k re-billed / cause: system prompt or tool definitions changed" | **ORACLE-WEAK** |
| C004 dollars labelled by dated basis (tier promise withdrawn) | BOUNDED | `cost` human header | `costHeaderLine` `cost.go:1261` | same | `runCost` | shares C025's site; see C025 | "at list prices dated 2026-09-07 (caching rules anthropic-2026-09-05)" | WIRED |
| C025 header names price-table date and rules version | BOUNDED | `cost` human header | `cost.go:1261` | `cachemodel.PriceTableVersion`, `RulesVersionInEffect()` | `runCost` | `dated %s` → `%s` SURVIVED; `→ "undated"` SURVIVED (`TestC004_TheCostReportStatesHowItsDollarsWereObtained` accepts the rules date for its `\b20\d\d-\d\d-\d\d\b`) | header printed with both dates | **ORACLE-WEAK** |
| C011 "measured" = provider usage counters | BOUNDED | README:299; every dollar | `cachemodel/legs.go` | `in := p.InputPerMTok / tokensPerMillion` | `cost`, `since`, `route` | `* 2` → `TestC011_IndependentArithmeticAgreesWithProduction` | `since`: "2 cache break(s) $0.03 re-billed, 905,000 tokens" from the usage fields written | WIRED |
| C024 re-billed row in blame/reconcile | BOUNDED | `replay blame` | blame rows | `internal/analysis/blame.go` | `runBlame` | catalogue M2, M4, M5, M6, M9 all killed (`TestFrozenMutantsStillDie` 76/76) | blame rows rendered on the ledger fixture | WIRED |
| C028 route refuses a dollar figure for an unmeasured pair (print-everything promise withdrawn) | BOUNDED | README:303 | `route.go:358` | `route.go:267` `if r.Dilation.Measured && r.From.Known && r.To.Known` | `runRoute` | drop `r.Dilation.Measured &&` → `TestUnmeasuredSigmaSuppressesTheDollarFigure` ("a dollar figure appeared without a measured sigma") | route output carried TTL lines and no dollar line on the fixture pair | WIRED |
| C005 "leaves out of the figure and says how much it left out" (declines-to-print withdrawn) | BOUNDED | README:21–22 | C030's sentence | `cost.go` `if !priceKnown { unpriced++ }` | `runCost` | see C030 | "1 transcript(s) excluded: their model is not in the price table. They are left out rather than counted as free." | WIRED |
| C026 internal refusal with reason | BOUNDED | none printed (register: "classifier only") | — | `surface.Classify` | **none** in the closure | not applicable; `TestC005_RefusalIsDistinguishableFromZero` tests `internal/surface` | not observable | **UNWIRED** |
| C034 unpriced never counts as zero; worst window by tokens | BOUNDED | `cost` re-billed tokens; `since` largest | `since.go:102` `worstByRebilledTokens` | `deficit += br.Deficit` | `runCost`, `runSince` | `deficit += 0` → `TestRB2_AKnownDeficitIsReportedWhetherOrNotAPriceExists` | `since`: "largest: session unp at 22:01, 900,000 tokens re-billed" while the dollar line excludes it | WIRED |
| C033 TTL basis on cache-write dollars | BOUNDED | `route` "Write penalty is …x at 5m and …x at 1h" | `route.go:342` | `cachemodel/anthropic.go` `writeEquivalent` | `runRoute` | `WriteMultiplierLong` → `Short` → `TestEC4_TheTTLDistinctionSurvivesWhereTheProviderSuppliesIt` | the two-TTL line printed | WIRED |
| C030 excluded-not-free sentence; JSON `unpriced` | BOUNDED | `cost` human + JSON | `cost.go:641` | `unpriced++` at `:811` (warm) and `:845` (cold) | `runCost` | counter-only, whole suite → `TestAnUnpricedSessionIsExcludedAndCounted`, `TestCW1`, `TestCW2`, `TestPT1`; combined with C032 → `TestCW1`, `TestC005M` | "1 further transcripts were read but not priced … They are excluded rather than counted as free." JSON `unpriced: 1` | WIRED |
| C032 measured zero ≠ unmeasurable | BOUNDED | `cost` | `Unpriced: !priceKnown` | same | `runCost` | `Unpriced: false` → `TestAnUnpricedSessionIsExcludedAndCounted` | unpriced session excluded from `$0.08`, not printed as `$0.00` | WIRED |
| C037 coverage sentence; JSON `unpricedRebilledTokens` | BOUNDED | `cost` human + JSON | re-billed render | `unpricedDeficit += br.Deficit` | `runCost` | `+= 0` → `TestC037_P2_NoBreakCoverageIsDisclosed`, `TestC037R1_TheFourCoverageCases` | "The re-billed figure covers 50% of those tokens: 20000 of 40000 were priced" | WIRED |
| C020 unjoinable requests disclosed | BOUNDED | `cost` JSON `unjoinableRequests` | `overlap.go:23` | `overlap.go:62` `if !measured` | `runCost` → `requestJoin.add` | `if false` → `TestXW3`, `TestXW7`, `TestRJ1_SynthesisedIDsAreNotJoinedAcrossFiles`, `TestRJ5_CostReportsLedgerRequestsAsUnjoinable` (the loop-1 mutation at the `cost.go` counter survived; it was not the gate) | `unjoinableRequests: 1` for a record written without `request_id` | WIRED |
| C022 outbound destinations are four named commands | BOUNDED | SKILL.md:7; README:416–424 | — | absence of net calls in the closure | — | planted `https://evil.example.com/…` in `cost.go` → `TestC022_OutboundDestinationsAreAnEnumeratedSet` | static property; the scan is the observable and it covers the closure (it caught the plant) | WIRED |
| C008 absence ≠ zero ≠ unknown | PROVEN | every counter; JSON `omitempty` | — | `observation/corpus.go:255` presence check; `ledger/record.go` `omitempty` | `runCost` | `omitempty` dropped on `Correlation` SURVIVED registered oracle; `corpus.go:255` presence check → `TestB5_AnOmittedQuantitativeKeyIsRefusedNotReadAsZero` (not registered). Registered `TestC008_AbsentFieldAndZeroFieldDiffer` is in `internal/surface/claim_epistemic_test.go:15`, outside the closure | record without `request_id` → `unjoinableRequests: 1`, not joined on `""` | **ORACLE-WEAK** |
| C021 no cross-surface grand total | PROVEN | `replay burn` | per-surface rows | absence of a reducer | `runBurn` | planted `func rollup(bs []surfaceBurn) int` in `burn.go` SURVIVED `TestC021_NoCrossSurfaceTotalIsReachable` (a name lint) | burn printed per-surface rows and no total line | **ORACLE-WEAK** |
| C013 no savings claim reaches the user | PROVEN | README:523; all output | `cost.go` closing sentence | the sentence | `runCost` | "not a forecast of savings" → "a forecast of what you will save" → `TestC013_NoSavingsClaimReachesTheUser` | output carries the "already paid for" sentence, no saving figure | WIRED |
| C036 deficit cost is record-local | PROVEN | `cost` re-billed dollars | per-break pricing | `cachemodel.PriceForAt(br.Turn.Request.Model, br.Turn.Request.Timestamp)` | `runCost` | session-level `(model, at)` → `TestC036_A_PricedRecordWithDeficit`, `_B_UnpriceableRecordWithDeficit`, `_D_MixedSession` | mixed fixture priced the opus break and excluded the unknown-model break | WIRED |
| C035 request-level coverage of the total | PROVEN | `cost` "The total above covers N% of the requests read" | coverage sentence | `analysis/replay.go` `Tally.AddAt` `t.UnpricedRequests++` | `runCost` | counter removed → `TestPT1_DoesPartialSurviveSessionAggregation` | "The total above covers 50% of the requests read: 1 of 2 priced" | WIRED |
| SP-01 gate PASS states what its figure excludes | PROVEN | `cost --max-rebilled-usd` | `costgate.go:53` `rebilledFigureCaveats` | the `len(caveats) > 0` branch | `runCost` | condition inverted → `TestSP_TheGatePassMustStateWhatItsFigureExcludes` | "1 of 3 requests read priced nothing, so this figure covers part of the work." true exit 0 on PASS, 1 on FAIL | WIRED |
| V1 invoice-facing median excludes unpriceable rows | PROVEN | `replay verify --compare` | `verify.go:170` | `verify.go:90` `unpriceable` | `runVerify` | `return false` → `TestV1_AnUnpriceableRowIsNotAFreeRow` | "2 further tasks were read and priced nothing, so they are in neither median. They are excluded, not free." | WIRED |
| Q01 lane file names do not decide a session's cost | PROVEN | `cost` folded vs `--per-lane` | `foldSessions` | `s.Unpriced = s.Unpriced && u.Unpriced` | `runCost` | `_ = u.Unpriced` → `TestQ01_LaneFileNamesDoNotDecideASessionsCost` | two files, one session id, lane on an unknown model: folded `tasks 1, lanes 2, pricedRequests 1, unpricedRequests 1, totalUsd 0.04125`; the session is priced from its main lane, the lane is disclosed at request and transcript level | WIRED |
| absence ≠ zero ≠ unknown (ADR-0018) | PROVEN | unpriced disclosure vs `$0` | — | `corpus.go:255`; `Unpriced` flag | `runCost` | `corpus.go:255` → `TestB5`; `Unpriced: false` → `TestAnUnpricedSessionIsExcludedAndCounted` | "They are left out rather than counted as free." beside a dollar line that omits them | WIRED |
| warm equals cold | PROVEN | `cost` with and without the index | `costcache.go:171–196` | `costIndexKey()` ends `+ "/" + unitSchema()` | `runCost` | `unitSchema()` dropped from the key → `TestC037R6_TheIndexKeyCarriesTheCounter` | cold and warm `--json` byte-identical; "1 transcript(s) reused from the index, 0 re-read" | WIRED |
| reconstruction core is provider-independent (E7-01) | PROVEN | none printed; E7-01 | — | `internal/analysis` on every reader | `runCost` over claudecode/codex/grok/ollama readers | catalogue provider mutants killed, incl. `M56_grok-back-on-the-chat-completions-row` | this gate ran Anthropic-shaped ledgers only; the cross-provider observation is E7-01 of the same day, not re-run here | WIRED |
| refusals are cheap and explicit | PROVEN | README:21 | — | PROOF row cites `internal/quota`, `internal/money` | `internal/money` is in the closure; `internal/quota` is not (6 catalogue mutants in it are killed by its own tests, which test a dead package) | the shipped refusals are the C030, SP-01, V1 and C028 rows above, each killed | the four refusal lines above | **ORACLE-WEAK** |
| PROOF-1.0 blocker 2: card says what its denominator is | marked FIXED | `cost --share` text card; PNG tone | `share.go:48/51/54`; `card/tone.go:196` | `pct := s.RebilledShare * 100` where `RebilledShare` = rebilled ÷ **priced** spend (`card.go:53`) | `cost.go:1041` `shareCard(s, breaks)`; `sharetui.go:103` | zero tests pin the text headline (`grep 'paid twice' *_test.go`: one comment) | "37% of my agent spend was paid twice." on the mixed fixture, where 63% of the requests were never priced | **E2E-FAIL** |

**Counts over the 27 claim rows:** WIRED 21, ORACLE-WEAK 5, UNWIRED 1,
E2E-FAIL 0. The 28th row is a PROOF-1.0 blocker, not a registered claim, and
is the one E2E-FAIL.

---

## 5. The failing rows, as the brief asks

### 5.1 UNWIRED: RPL-C026

- **Claim.** "Internal refusal with reason." Register Scope: "surface.Classify
  only. NOT the printed surface, which RPL-C005 refutes."
- **Broken link.** Gate 2. `surface.Classify` is in `internal/surface`, which
  `go list -deps ./cmd/replay` does not contain, and nothing under `cmd/` calls
  it. The claim is wired into a harness that the binary never links.
- **Does release wording depend on it?** No. README:21 "The refusals are the
  feature" is carried at runtime by C030 (exclusion sentence), SP-01 (gate
  caveats), V1 (median exclusion) and C028 (route's dollar refusal), all WIRED.
  No README or guide sentence names `surface.Classify` or depends on its
  reason strings.
- **Smallest repair.** A register decision, not code: either re-scope C026 to
  POST-1.0 (its own Scope already disclaims the printed surface, and the
  shipped refusal-with-reason is C030's sentence plus the `Unpriced` flag), or
  wire `Classify` behind the cost reader, which is new production code and out
  of 1.0 scope. The register already lists `internal/surface` as deliberately
  unwired at `unwired_packages_test.go:62`.
- **Affected tests.** None change under the re-scope. Under the wiring option:
  `TestC005_RefusalIsDistinguishableFromZero` and its sibling in
  `internal/surface/claim_epistemic_test.go` would need a production-path
  twin in `cmd/replay`, and `unwired_packages_test.go` would drop its row.
- **Not applied.** Per the brief.

### 5.2 ORACLE-WEAK

| Claim | What the register names | What actually guards the shipped path | Smallest repair (not applied) |
|---|---|---|---|
| C001 | `TestFixtureCorporaClassifyAsTheAuditFound` (`internal/surface`) | the `diff`/`cost` fixture tests in `cmd/replay`; `diff` named turn 1 with cause on a two-request ledger | point `Tests` at a `cmd/replay` test that runs `diff` over a fixture and asserts the turn and cause |
| C008 | `TestC008_AbsentFieldAndZeroFieldDiffer` (`internal/surface`) | `TestB5_AnOmittedQuantitativeKeyIsRefusedNotReadAsZero` (`internal/observation`, in the closure) | add `TestB5` to C008's `Tests`; regenerate the register |
| C021 | `TestC021_NoCrossSurfaceTotalIsReachable`, registered as "type-level"; a planted `rollup` reducer survived it | nothing behavioural; Room VIII's census found 0 reducers by reading, not by test | an oracle that compiles a reducer over `[]surfaceBurn` in a test and asserts the burn renderer never calls one, or a render-level assertion that no line sums across surfaces |
| C025 | `TestC004_TheCostReportStatesHowItsDollarsWereObtained`; the header with the price-table date removed still passed because the rules version carries a date that satisfies the same regex | nothing pins `PriceTableVersion` in the header | assert the header contains `cachemodel.PriceTableVersion` literally, not any date |
| refusals are cheap and explicit | `internal/quota`, `internal/money` | C030, SP-01, V1, C028 rows | repoint the PROOF row's evidence at those tests; `internal/quota` is a dead package |

### 5.3 E2E-FAIL: PROOF-1.0 blocker 2

- **Row.** `docs/PROOF-1.0.md:69`: "**FIXED, wording only.** Now 'of priced
  spend, paid twice'".
- **OBSERVED.** `internal/card/tone.go:196` (the PNG) says "of priced spend,
  paid twice". `cmd/replay/share.go:48` (the text card, reached from
  `cost.go:1041` and `sharetui.go:103`) says "%.0f%% of my agent spend was paid
  twice." over `RebilledShare`, which `card.go:53` defines as the fraction of
  *priced* spend. On the mixed fixture it printed **"37% of my agent spend was
  paid twice."** with one priced request in three.
- **Why it matters for release wording.** README:24–25 (edited in the
  closeout) promises the cost report "carries the population it was measured
  on". The text card is the one surface designed to leave the machine, and it
  names the wrong population.
- **Smallest repair.** Three string edits at `share.go:48`, `:51`, `:54`:
  "of my agent spend" → "of my priced agent spend" (or match the PNG: "of
  priced spend"). RED first: a `cmd/replay` test that builds a `costSummary`
  with `RebilledShare: 0.37` and asserts the headline contains "priced"; it
  fails today because no test pins the headline.
- **Affected tests.** None existing (`grep -l 'paid twice' cmd/replay/*_test.go
  internal/card/*_test.go` returns only a comment in `tipline_test.go:11`).
  `docs/screens/share.svg` does not carry the headline. `PROOF-1.0.md:69`
  needs its row corrected to "PNG fixed; text card open".
- **Not applied.** Per the brief.

---

## 6. Mutation ledger

Every mutation was applied to the working tree, run, and restored from a
backup with `cmp` before the next. Final check: `git diff --stat cmd internal`
equals the pre-gate figure exactly (24 files changed, 597 insertions, 122
deletions), `gofmt -l cmd internal` empty.

**Loop 1, 18 single-anchor mutations, registered oracle only.** 12 killed, 6
survived: C025, C030, C032, C020, C008, C021. Loop 2 re-mutated the five
survivors whose first site was not the deciding site, or ran against the whole
suite:

| Claim | Loop-2 site | Result |
|---|---|---|
| C020 | `overlap.go:62` `if !measured` → `if false` | killed: `TestXW3`, `TestXW7`, `TestRJ1`, `TestRJ5` |
| C030 | counter only, whole suite | killed: `TestAnUnpricedSessionIsExcludedAndCounted`, `TestCW1`, `TestCW2`, `TestPT1` |
| C030 + C032 | both sites | killed: `TestCW1`, `TestC005M` |
| C032 | flag only | killed: `TestAnUnpricedSessionIsExcludedAndCounted` |
| C008 | `corpus.go:255` presence check | killed: `TestB5` (not the registered oracle) |
| C025 | header → `"undated"` | **SURVIVED** |
| C021 | `rollup` plant | **SURVIVED** (loop 1; not re-run, the oracle is a lint by construction) |

**Reruns in this pass, for the record:** C020 at the gate (above); C028 route
sigma gate → `TestUnmeasuredSigmaSuppressesTheDollarFigure`.

**CI catalogue.** `go test -tags mutation ./internal/mutation/`:
`TestFrozenMutantsStillDie` PASS, 76 of 76 frozen mutants killed (1,026.9 s).
The run was then killed by the operating system during `TestKillMatrix` (low
memory), so no package-level `ok` line exists for this run. `TestKillMatrix`
is the harness's own stillborn-baseline check, not a claim oracle; its last
verified result is the closeout's green suite. Not restarted in this pass.
Relevant to this matrix: `blame.go` M2/M4/M5/M6/M9 (C024), `cost.go` M35/M50,
`costcache.go` M52 (warm=cold), `share.go` M36 (route line on the card, not the
headline), and six `quota.go` mutants that test a package outside the closure.

---

## 7. E2E ledger

All runs via `go run ./cmd/replay` at `dc4686b` on the fixtures in §3. Exit
codes were captured to a file and echoed, after a pipeline masked them once.

| Run | Printed |
|---|---|
| `cost` on the mixed set | header with both dates; "The total above covers 50% of the requests read"; "excluded rather than counted as free"; JSON `unpriced: 1`, `unjoinableRequests: 1` |
| `cost --max-rebilled-usd` PASS / FAIL | caveat line on PASS; exit 0 / 1 |
| `verify --compare` | "2 further tasks were read and priced nothing … excluded, not free." |
| `cost --share` | "37% of my agent spend was paid twice." (§5.3) |
| `route --to` | "Write penalty is …x at 5m and …x at 1h"; no dollar line on an unmeasured pair |
| `burn` | per-surface rows, no total |
| `blame` | re-billed rows |
| `diff priced.jsonl` | "turn 1 at 12:00:01 (+1s): read 0 of 5.0k expected, 5.0k re-billed / cause: system prompt or tool definitions changed" |
| `since` under a fresh HOME | "2 session(s) 4 request(s) $0.08 / 2 cache break(s) $0.03 re-billed, 905,000 tokens / largest: session unp at 22:01, 900,000 tokens re-billed / 1 transcript(s) excluded: their model is not in the price table." |
| `cost` cold then warm | identical JSON; "1 transcript(s) reused from the index, 0 re-read" |
| `cost` / `cost --per-lane` on `q01/` | `tasks 1, lanes 2, pricedRequests 1, unpricedRequests 1, totalUsd 0.04125` both ways |

---

## 8. Observations outside the gate (no status, no action)

- The `cost` header printed `caching rules anthropic-2026-09-05` under the real
  `HOME` and `anthropic-2026-09-01` under a fresh one. Both are
  `RulesVersionInEffect()` (`rules.go:404`): a loaded rules document overrides
  the compiled constant. By design; recorded so the two figures in §7 do not
  read as a defect.
- On the `q01/` fixture the folded report says "across 1 sessions (2 agent
  lanes)" and then "1 further transcripts were read but not priced". The
  unpriced transcript is a lane of that one session, not a further session.
  This is the record-versus-session granularity the closeout already queued
  under C031 (POST-1.0); not reopened.
- `replay since` takes no directory argument. The guide should say so if it
  does not; not checked here.

---

## 9. Verification

**OBSERVED.** Branch `fix/compaction-observed-vs-inferred`, HEAD `dc4686b`.
Production files (`cmd/`, `internal/`) changed by this gate: **0**; the
`git diff --stat cmd internal` figure is byte-for-byte the pre-gate figure.
Staged: **0**. Committed: nothing. Full suite: not re-run; no production change
since the closeout's verified **37 ok, 0 FAIL** baseline, and every targeted
run in §6 was against an unmodified tree after restore. Mutation catalogue:
76/76 frozen mutants killed; the run was OS-killed during `TestKillMatrix`
afterwards (§6). Docs written by this gate: this file and one index row in
`docs/evidence/README.md`. Fixtures and backups live under
`$CLAUDE_JOB_DIR/tmp/` and are not part of the tree.
