# 1.0 claim closeout

**2026-10-02. Every registered 1.0 claim traced promise → claim → oracle →
evidence → surface, and given exactly one of four dispositions. No production
code changed. Nothing committed.**

The continuity line is frozen first, because the brief requires it and because
its verdict removes one claim from the live set.

---

## 0. Continuity result, frozen as negative evidence

| room | arms | fossilised | verdict |
|---|---|---|---|
| VIII | 8 | 0 of 8 | D, falsified |
| IX, explicit NOT-DETERMINABLE clause removed | 2 | **0 of 2** | closed |

**Work continuity: CLOSED / NEGATIVE.** Residual confound documented in
`panel-reader-discipline-2026-10-02.md`: the "check it against the files"
clause remained by the single-variable rule, so the Room IX null is weaker
than it reads. The result is two agents on one Go fixture and is not a claim
about all agents. No further continuity work is authorised.

---

## A. 1.0 CLAIM LEDGER

**Source of truth:** `internal/claims/claims.go`, 25 registered claims.
`CLAIM-REGISTER.md` is generated from it and was stale by one claim
(`RPL-C037` absent); regenerated this pass with
`go run scripts/claim-register/main.go`, idempotent on re-run, +56/−2 lines.
The suite had stayed green across that staleness: **nothing guards the
generated register against its source.** Recorded in the queue.

**Disposition rule, applied mechanically from the registered `Result`:**
ESTABLISHED → PROVEN if the surface says exactly that; BOUNDED → BOUNDED;
REFUTED → BLOCKED if any surface still makes the claim, else BOUNDED once the
promise is narrowed; NO_ENDPOINT, NOT_MEASURED, UNRESOLVED → POST-1.0 if no
surface promises it, BLOCKED if one does; DELIBERATE_NON_CLAIM → PROVEN when
its oracle holds.

### A.1 Registered claims

| ID | User-facing wording (exact, current) | Surface | Oracle | Result | Boundary | **1.0** | Required action |
|---|---|---|---|---|---|---|---|
| **C001** | "Replay Doctor reads the transcripts already on your disk and names the turn it broke on" | README:13; help text | `TestFixtureCorporaClassifyAsTheAuditFound` | BOUNDED | Claude Code, Codex, Grok, Ollama only; Cursor, AnythingLLM, OpenClaw refused | **BOUNDED** | none. Boundary is stated to users at `docs/SURFACES.md:275–277` |
| **C004** | *was* "Nothing prints without one" (a tier) | README:303 | `TestC004_*` ×4 | REFUTED | JSON carries no tier field; cost report labels dollars by dated basis, not tier words | **BOUNDED** — promise withdrawn this pass | README:303 sentence deleted. Was BLOCKED before this pass |
| **C025** | "Cost per task … at list prices dated 2026-09-07 (caching rules anthropic-2026-09-01)" | `replay cost` human header | `TestC004_TheCostReportStatesHowItsDollarsWereObtained` | BOUNDED | human output only; not JSON; not other reports | **BOUNDED** | register `Asserted` drifted: `cost.go:1057` → the sentence is now `cost.go:1261`. Queue |
| **C011** | "**measured** — Read from the provider's own usage counters" | README:299 | `TestC011_*` ×3 | BOUNDED | usage, not charge | **BOUNDED** | none |
| **C012** | none — no surface says what the provider billed | — | `TestC012_NoProviderBillingEndpointExists` | NO_ENDPOINT | — | **POST-1.0** | none; grep of README/CLI.md for a billing promise: 0 hits |
| **C024** | re-billed row in `blame`/`reconcile` | blame, reconcile | `TestConserve_*` ×2 | BOUNDED | those two outputs | **BOUNDED** | none |
| **C027** | README tier table names three tiers | README:297–301 | `TestC004_TheTierSpecificationIsInConflict` | UNRESOLVED | ADR-0002 names two; no enum, no enforcement | **POST-1.0** | the clause "enforced in code rather than promised in a README" was **false by this claim's own finding** and was removed this pass. Reconcile ADR-0002 vs README |
| **C028** | *was* "Nothing prints without one" | README:303 | `TestC004_OutputSurfaceInventory` +2 | REFUTED | `advise` emits 10% with no status; 5 of 14 surfaces unmeasured | **BOUNDED** — promise withdrawn this pass | same deletion as C004 |
| **C029** | none | — | none constructible | NO_ENDPOINT | — | **POST-1.0** | none |
| **C005** | *was* "Anything this tool cannot measure, it declines to print — and says why" | README:21–22 | `TestC005_*` ×4; `TestC005_AMixedTranscriptHidesItsUnpricedRecords` still pins `unpriced == 0` on a mixed transcript | REFUTED | a partly-priceable session prints a total over the priced subset | **BOUNDED** — promise narrowed this pass | README:21–22 now says what C030 + C035 establish. Was BLOCKED |
| **C026** | internal refusal with reason | `surface.Classify` | `TestC005_RefusalIsDistinguishableFromZero` +1 | BOUNDED | classifier only, not printed surface | **BOUNDED** | none |
| **C034** | "An unpriced model must never count as zero" | `docs/TOKEN-PRICES.md:53`; cost; since | `TestEC00_*`, `TestCW1–3`, `TestRB2`, `TestRB5` | BOUNDED | re-billed tokens in `cost` and worst-window in `since` only; 7 of 8 `PriceFor` consumers unmeasured at the user boundary | **BOUNDED** | none; boundary is in the registered scope verbatim |
| **C033** | TTL basis on cache-write dollars | `route` prints "Write penalty is …x at 5m and …x at 1h" | `TestEC1–4` in `internal/cachemodel` | BOUNDED | the register body: "declared in two code comments and in no output" | **BOUNDED** | the **Scope line** names `burn` and `cost --usage`, which print no TTL basis; the oracle drives `cachemodel`, not those surfaces. Narrow Scope. Queue |
| **C030** | "They are excluded rather than counted as free." + "N transcript(s) were excluded as unpriced" | `replay cost` human; JSON `unpriced` | `TestC005M_*` ×2 | BOUNDED | session granularity | **BOUNDED** | `Asserted` drifted: `cost.go:488` is now the median line; sentence is at `:641`. Queue |
| **C031** | none after this pass | — | `TestC005M_TheFixtureMatrix`, `TestC005_AMixedTranscript…` | REFUTED | record granularity; oracle pins the old `unpriced` key, not C035's `unpricedRequests` | **POST-1.0** | re-measure against the C035 sentence; do **not** convert C035's evidence into this claim's proof |
| **C032** | measured zero ≠ unmeasurable | `replay cost` | `TestC005M`, `TestCW1`, `TestRB6` | BOUNDED | session cost in `cost` | **BOUNDED** | none |
| **C037** | "The re-billed figure covers 33% of those tokens: 20000 of 60000 were priced, and 40000 ran on a model no price table carries" + JSON `unpricedRebilledTokens` | `replay cost` human + JSON | `TestC037_*` ×8, `TestC037R1–R7` | BOUNDED | break granularity in `cost`; `cost --usage` is a second path, registered NOT measured | **BOUNDED** | `Asserted` `cost.go:547/549` drifted. `cost --usage` path and corpus-payload coverage stay POST-1.0 **by the frozen decision**; not reopened |
| **C008** | absence, zero, unknown never collapsed | every cache counter | `TestC008_AbsentFieldAndZeroFieldDiffer` | ESTABLISHED | — | **PROVEN** | none |
| **C019** | none | — | `TestXW6_NoAccountIdentityExistsToCorrelateOn` | NO_ENDPOINT | — | **POST-1.0** | none; structurally unavailable |
| **C020** | `unjoinableRequests` disclosed | `cost` JSON; `overlap.go:23` | `TestXW1–5,7`, `TestRJ1–2` | BOUNDED | provider-id uniqueness not independently established | **BOUNDED** | none |
| **C021** | no cross-surface grand total | `replay burn` | `TestC021_NoCrossSurfaceTotalIsReachable` | ESTABLISHED | — | **PROVEN** | the registered Oracle says "type-level"; Room V measured it is a name lint. The **claim holds** (Room VIII census: 0 cross-surface reducers in shipped code); the oracle description overstates. Queue |
| **C013** | no savings claim anywhere; README:523 "not what you will save" | all output | `TestC013_NoSavingsClaimReachesTheUser` | DELIBERATE_NON_CLAIM | — | **PROVEN** | none |
| **C015** | none; `track` returns AdviceOnly | advisor | `TestC015_*` ×2 | NOT_MEASURED | — | **POST-1.0** | none |
| **C016** | none | — | `TestC016_NoTaskImprovementClaimIsAsserted` | NOT_MEASURED | — | **POST-1.0** | closed by Rooms VII–IX; no surface promises it |
| **C022** | "no network request except four commands the user types (`rules --check-prices`, `probe --execute`, `upgrade`, `rules --update`)" | SKILL.md:7; README:416–424 | `TestC022_OutboundDestinationsAreAnEnumeratedSet` | BOUNDED | the shipped binary; loopback Ollama probe is untyped and disclosed in README | **BOUNDED** | none. `contribute.go` makes no HTTP call; `replay.doctor`/`redrobot.jp` are printed strings, not fetches |

### A.2 PROOF-1.0 capability rows not carried as registered IDs

These are already written down as 1.0 claims in `docs/PROOF-1.0.md`; they are
inventoried, not invented.

| row | oracle | **1.0** |
|---|---|---|
| deficit cost is record-local (C036) | `TestC036_*` ×9, `TestFM*` ×4 | **PROVEN** |
| request-level coverage of the total (C035) | `TestPT0–2`, `TestRB1Pop_…`; sentence ships verbatim | **PROVEN** |
| gate PASS states what its figure excludes (SP-01) | `TestSP_*` ×3 | **PROVEN** |
| invoice-facing median excludes unpriceable rows (V1) | `TestV1_AnUnpriceableRowIsNotAFreeRow` | **PROVEN** |
| lane file names do not decide a session's cost (Q01) | `TestQ01_*` ×2 | **PROVEN** |
| absence ≠ zero ≠ unknown | ADR-0018; `TestGK5`, `TestRB6` | **PROVEN** |
| warm equals cold | `TestC037R5`, `TestC037R6` | **PROVEN** |
| reconstruction core is provider-independent | E7-01 | **PROVEN** |
| refusals are cheap and explicit | `internal/quota`, `internal/money` | **PROVEN** |
| action is not outcome, in the state model | `stateledger.Settle` — **UNWIRED** | **POST-1.0** |
| eleven work states expressible | E4-01 — **UNWIRED** | **POST-1.0** |

### A.3 Counts

| | registered | PROOF rows | **total** |
|---|---:|---:|---:|
| PROVEN | 3 | 9 | **12** |
| BOUNDED | 15 | 0 | **15** |
| POST-1.0 | 7 | 2 | **9** |
| BLOCKED | **0** | 0 | **0** |
| | 25 | 11 | **36** |

Before this pass's four wording edits, **three registered surfaces were
BLOCKED** (C004, C005, C028: refuted claims still promised verbatim on the
README). After: zero.

---

## B. 1.0 USER-FACING WORDING

Exact final text of every surface this pass changed, and the registered
finding each change is bounded to.

**README:21–22**, bounded to C030 + C035
> **The refusals are the feature.** Anything this tool cannot price, it leaves out of the figure and
> says how much it left out, in the place the number would have gone.

**README:24–25**, bounded to C025 + C035
> The cost report carries the population it was measured
> on and the date of the price table it used.

**README:295**, C027 found no enforcement
> This is the part that matters.

**README:303**, C004 and C028 refuted the universal; the sentence after it stands
> `replay route --to <model>` **refuses to give a dollar figure** for a
> model pair it has not measured, rather than guessing

Shipped surfaces inspected and left unchanged, with the sentence that ships:
`replay cost` header *"at list prices dated 2026-09-07 (caching rules
anthropic-2026-09-01)"* (C025); *"The total above covers 40% of the requests
read: 2 of 5 priced, 3 on a model no price table carries. Those 3 are not in
the figure and are not free"* (C035); *"The re-billed figure covers 33% of
those tokens … Those are not free, and what they cost is not established
here"* (C037); *"They are excluded rather than counted as free"* (C030); the
card's *"of priced spend, paid twice"* with the `>99%` ceiling; `docs/CLI.md`
*"An unpriced model is excluded, never counted as free. Figures say how many
transcripts were left out"* — generated from `scripts/cli-blueprint/gen.py:255`,
not hand-editable, and exactly C030's granularity.

---

## C. POST-1.0 QUEUE

A backlog, not a release blocker. **Production work required** is answered
per item.

| # | item | why outside 1.0 | evidence available | missing | production work |
|---|---|---|---|---|---|
| 1 | C012, C019, C029 | no endpoint exists to measure against | static scans | provider cooperation that no provider offers | **no** |
| 2 | C015, C016 | not measured; continuity closed negative in Rooms VII–IX | three rooms of negative results | — | **no** |
| 3 | C027 spec conflict | README names three tiers, ADR-0002 names two | `TestC004_TheTierSpecificationIsInConflict` | a decision | **no**, docs |
| 4 | C031 record-level disclosure | oracle pins the pre-C035 `unpriced` key | C035 ships `unpricedRequests` | re-measure C031 against the C035 sentence without converting C035's evidence | **no**, test |
| 5 | C037 `cost --usage` second path | registered NOT measured, NOT repaired; **frozen decision, not reopened** | `costusage.go:194/196/267` named in the register | a measurement | yes, if pursued |
| 6 | corpus-payload coverage (PROOF blocker 2b) | **frozen POST-1.0 by evidence, not reopened** | — | — | yes, if pursued |
| 7 | `stateledger` rows | UNWIRED; "that reader is the next decision and has not been taken" | E4-01 | a decision | yes, if pursued |
| 8 | C033 Scope line | names `burn` and `cost --usage`; oracle covers neither; body already admits "in no output" | this pass | narrow the Scope string in `claims.go`, regenerate | **no**, data |
| 9 | `Asserted` line drift | C025 `cost.go:1057`→`:1261`; C030 `:488`→`:641`; C037 `:547/549`→re-billed render; C022 `SKILL.md:9` is blank (sentence at `:7`); C020 `requestjoin_test.go:16` is a comment fragment | this pass | update refs in `claims.go`, regenerate | **no**, data |
| 10 | register drift guard | `CLAIM-REGISTER.md` was one claim stale and the suite stayed green | this pass | a test that regenerates to a buffer and diffs | **no**, test |
| 11 | C021 oracle | registered "type-level"; measured a name lint (Room V) | Room V, Room VIII census | behavioural oracle | **no**, test |
| 12 | **`p90Usd` under-reads at small n** | **no registered claim covers percentile correctness**, so not repaired under §7 | `percentile()`: `int(p*(n-1))` picks index 0 for n=2; per-task `[0.3568, 1.278]` prints median 0.8174, **p90 0.3568**; under-reads at n = 2–9 and 12 | — | **yes, one line**; promotion to BLOCKED is the maintainer's call. It ships on the human block and JSON |
| 13 | `lanes` counts a withheld session | header reads *"1 sessions (2 agent lanes)"* for two sessions, one unpriced; field doc says "read **and priced**" | this pass, mixed fixture | — | yes, one condition, or a doc fix |
| 14 | README:26 em-dash | house rule, not a claim | — | — | **no**, docs |

---

## D. RELEASE VERIFICATION

Recorded after the single full run; see the closing block.

**Release conclusion, mechanically from the counts:** 36 in-scope items;
**BLOCKED = 0**; every item is PROVEN, BOUNDED with its wording narrowed to
the demonstrated boundary, or explicitly deferred with its gap named.

### Closing block, measured after the single full run

| | |
|---|---|
| branch | `fix/compaction-observed-vs-inferred` |
| commit | `dc4686b` (HEAD unchanged; nothing committed, nothing staged) |
| production files changed (`cmd/`, `internal/`) | **0** |
| docs changed | `README.md` (4 sentences, +6/−6 lines), `CLAIM-REGISTER.md` (regenerated, +56/−2), `docs/evidence/README.md` (index rows), this document |
| `go test ./...` | **37 ok, 0 FAIL**, 1 package with no test files, exit 0 |
| gofmt | clean |
| claim count | **36** in scope: 25 registered + 11 PROOF-1.0 rows |
| PROVEN | **12** |
| BOUNDED | **15** |
| POST-1.0 | **9** |
| BLOCKED | **0** (3 on entry, all three README wording) |

**The release conclusion follows from the last four rows: every 1.0 promise is
supported at its registered boundary, bounded to it in words, or explicitly
deferred with its gap named. No blocker remains.**
