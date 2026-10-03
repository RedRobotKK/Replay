# LOCKED ROOM VI: denominator integrity

**2026-10-02. Ten adversarial passes. No production change. Nothing committed.**

The question: does `replay cost` publish a figure whose denominator can be
materially smaller than the observed evidence population, because the five
absence states on `transcript.Session` disappear at the reporting boundary?

Answer: **the mechanism is real and demonstrated; the incidence is zero.**

Every substantive statement is labelled **MEASURED**, **INFERRED** or
**NOT MEASURED**.

---

## 1. Executive verdict

The reporting gap exists and was reproduced: a synthetic ledger holding one
skipped record, one unknown kind, one refusal, two provider failures and one
schema mismatch publishes `unreadable=0, totalRequests=2, tasks=1` and says
nothing about any of the six excluded records, in JSON or in prose. But on the
only real corpora on this machine the gap carries almost nothing. Four of the
five states are zero across 3,179 transcripts and 17 ledger records, and the
fifth, `Skipped`, is **100% Claude Code housekeeping lines: 0 of 178,102 were
unparseable and 0 carried provider usage**, so the sentence the report already
prints about them is true. Separately, four of the five states hold records
that by their own branch conditions carry no usage, so their absence from a
cost denominator is correct rather than a loss. Room V's framing was wrong in
two ways this room corrected against source: the states do not "disappear at
the reporting boundary" uniformly, because four of them are never *created* on
the transcript path at all; and `UnknownKinds` is unreachable in production
because no writer sets `Record.Kind`. One genuine defect was found, outside the
five states: the triage gate `Usage == nil || len(Prompt.Messages) == 0`
diverts records that *do* carry usage, discarding measured tokens. It is
production-reachable in principle and has **0 incidence in 17 real ledger
records**. Verdict **B, diagnostic only**; the defect is registered as a frozen
candidate rather than repaired on zero evidence.

---

## 2. Ten-pass evidence table

| Pass | Question | Experiment | Result | Survives? |
|---|---|---|---|---|
| 0 | baseline and ownership | `go test ./...`, mtime boundary | 37 ok, 0 FAIL; branch `fix/compaction-observed-vs-inferred`, HEAD `dc4686b`, 61 dirty entries | n/a |
| 1 | what creates and consumes each state | source trace of every write and read | 4 of 5 written only by `internal/ledger/store.go`; `UnknownKinds` has **zero** production readers; `Session.Refusals` has zero production readers | **mechanism yes** |
| 2 | real-corpus incidence | census over 3,179 transcripts and 5 ledger files | `Skipped` 981 sessions / 178,102 lines; everything else 0 except `ProviderFailures`=1 | **incidence no** |
| 2b | is `Skipped` loss or routine | instrumented `readLines`, reverted | **0 unparseable, 178,102 housekeeping, 0 carrying usage** | **killed** |
| 3 | can a user tell exclusion from absence | compare populations the code supports | yes at file level (`unreadable` + a sentence), no at record level | **gap yes** |
| 4 | can the instrument produce a one | six synthetic positive controls | all six fire; all six invisible in `cost --json` and in prose | **gap confirmed** |
| 5 | does disclosure change a conclusion | branch conditions vs cost inputs | 4 of 5 states hold records with no usage, so they could not have moved any figure | **killed** |
| 6 | would a new test protect anything | three mutations | all three caught by shipped tests (4, 2 and 2 respectively) | **no new invariant** |
| 7 | attack the hypothesis | five attacks | four succeed; one found a real defect in the gate *above* the five states | **partially** |
| 8 | prior art / classification | classify the survivor | parser diagnostics plus accounting denominator disclosure | **ordinary** |
| 9 | product consequence | evaluate eight axes | diagnostic value only, on zero measured incidence | **no** |
| 10 | verdict | | **B, diagnostic only** | |

---

## 3. Five-state census

**MEASURED.** Corpora: `~/.claude/projects` (3,179 `.jsonl`, 3,159 parsed,
105,288 requests, 3,254 lanes) and `~/.replay/ledger` (5 `.jsonl`, 17 records,
15 requests). Read through the production readers via `loadSession`.

| State | Real count | Positive control | Reaches `cost` output? | Claim impact |
|---|---|---|---|---|
| **Skipped** | 981 sessions (31.05%), 178,102 lines; 1 ledger record | fires | **no** | **none.** 100% housekeeping, 0 unparseable, 0 with usage |
| **UnknownKinds** | **0**, and unreachable: no production writer sets `Record.Kind` | fires (hand-written JSON) | **no** | none; also **zero production readers** |
| **Refusals** | **0** on both corpora | fires | **no** | none; also **zero production readers** on `transcript.Session` |
| **ProviderFailures** | **1** record (HTTP 401), of 17 ledger records | fires, both arms | **no** | none: the branch requires `Usage == nil` |
| **SchemaMismatch** | **0** on both corpora | fires | **no** | the only state whose records' usage exists and is deliberately unread |

A sixth exclusion, not in the brief, **is** published: **20 of 3,179 files
(0.63%, 1,372,090 bytes) fail to parse entirely**, 19 with "no provider
requests found" and 1 with "no conversation lines found". `cost` publishes an
`unreadable` key and prints "Absent from every figure above, not zero in it."
Those files contain no provider request, so they could not have contributed.

**Reconciliation.** The census counted 177,954 skipped lines over the 3,159
sessions that parsed; the instrumented count was 178,102. The 148-line
difference is the lines inside the 20 files that were read and then failed.
The two agree.

**The `Skipped` composition, by the Claude Code line type that produced it:**

| | | | |
|---|---:|---|---:|
| `last-prompt` | 23,509 | `frame-link` | 10,352 |
| `atis-latch` | 19,654 | `agent-name` | 6,244 |
| `queue-operation` | 18,713 | `file-history-snapshot` | 4,567 |
| `ai-title` | 18,671 | `cost-state` | 879 |
| `mode` | 18,262 | `file-history-delta` | 746 |
| `permission-mode` | 18,220 | `artifact-autoreact-ledger` | 547 |
| `custom-title` | 13,959 | `artifact-comment-monitor` | 70 |
| `pr-link` | 12,428 | `result`, `started` | 24, 24 |
| `bridge-session` | 11,212 | `worktree-state`, `continued-in` | 20, 1 |

Not one is conversation content. Not one carries provider usage.

---

## 4. Denominator analysis

The populations the implementation actually supports, named as the code names
them rather than as the brief proposed:

```
files discovered              transcriptFiles(): every .jsonl under the root
  -> files that yielded a session       the complement is `unreadable`, PUBLISHED
    -> records admitted as requests     the complement is the five states, NOT PUBLISHED
      -> requests that reached a price lookup
        -> PricedRequests / UnpricedRequests        both PUBLISHED
          -> rows with Unpriced == false            the dollar statistics
```

**Can a user distinguish "nothing happened" from "evidence existed and was
excluded"?**

- **At file level: yes.** `unreadable` is a published JSON key and the human
  output names it in a sentence written for exactly this purpose.
- **At record level: no.** `totalRequests` counts records admitted as requests.
  Nothing published says how many records were read and not admitted.

**MEASURED, Pass 4.** A session holding two valid requests and six excluded
records publishes:

```
unpriced=0  unreadable=0  totalRequests=2  tasks=1
```

and the prose output mentions none of them. That is the gap, demonstrated.

**And it does not matter, for five of six.** Read the triage in
`internal/ledger/store.go`: every non-request branch sits under
`if rec.Response.Usage == nil || len(rec.Prompt.Messages) == 0`, and
`ProviderFailures.ByStatus` additionally re-checks `Usage == nil`. A record
with no usage contributes no tokens and no dollars, so excluding it from a cost
denominator is **correct, not lossy**. `SchemaMismatch` is the single exception,
because those records' usage was never read, and it is zero on both corpora and
zero by construction today: `SchemaVersion` has been 2 throughout and nothing
writes another value.

---

## 5. Mutation result

**MEASURED.** Three mutations, each applied to production code, run, and
reverted byte-identically against a backup held outside the repository.

| Mutation | What it destroys | Caught by |
|---|---|---|
| **M1** `b.session.SchemaMismatch = schemaMismatch` to `Skipped +=` | the intact-bytes / lost-bytes distinction | **4 shipped tests**: `TestSM6_TheSchemaMismatchNoteIsTruthful`, `TestSM6b_TheTwoPopulationsAreReportedApart`, `TestSM6d_TheNoteAgreesWithItsOwnCount`, `TestStoreRoundTripToSession` |
| **M2** `UnknownKinds++` to `Skipped++` | version gap reported as data loss | **2 shipped tests**: `TestUK1_AnUnknownEventKindIsNotReportedAsLostConversation`, `TestUK2_AnUnknownEventKindIsCountedSoItIsNotSilentlyDropped` |
| **M3** suppress `cost`'s `unreadable` counter | the one exclusion `cost` does publish | **2 shipped tests**: `TestCW1_TheFourStates`, `TestCU4_CostCountsUnreadableFilesEndToEnd` |

**Every mutation this room could construct was already caught.** No new
invariant is needed and no new test is justified. That is a result against
adding machinery, and it is the reason Pass 9 recommends nothing.

---

## 6. Strongest counterargument

> This is not a Replay problem. The states are zero, the one that is not is
> routine housekeeping that the report already describes correctly, every
> excluded record is one that could not have contributed to a cost figure
> anyway, the file-level exclusion that *can* matter is already published with
> a sentence saying it is absent rather than zero, and every way of breaking
> the distinction is already caught by a shipped test. You are proposing to
> publish five counters that would read `0, 0, 0, 0, 0` on every corpus that
> exists, in order to disclose evidence that by construction has no effect on
> the number being disclosed.

**Answered with evidence: the counterargument is essentially correct, and this
room accepts it.**

Each clause holds. Zero on both corpora for four states: **MEASURED**.
Housekeeping: **MEASURED**, 0 of 178,102 unparseable and 0 carrying usage.
Could-not-have-contributed: **MEASURED** from the branch conditions, with the
one exception in Pass 7. Already published at file level: **MEASURED**.
Already caught: **MEASURED**, three mutations, eight shipped tests.

The one clause that does **not** hold is "by construction has no effect". The
gate above the five states can divert a usage-bearing record (Pass 7). That is
a real defect and it is the only thing this room found worth keeping. Its
measured incidence is zero.

---

## 7. Product consequence

**None, on this evidence.** Evaluating the eight axes the brief asks for
honestly, where the answer is driven by the measured incidence rather than by
the mechanism:

| axis | verdict |
|---|---|
| auditability | a counter that reads 0 on every available corpus audits nothing |
| cost accuracy | **unchanged**: the excluded records carry no usage |
| trust | the file-level disclosure already carries the load-bearing sentence |
| debugging | marginal, and `replay replay` already prints four of the five per session |
| enterprise reporting | **NOT MEASURED**: no enterprise corpus exists |
| agent observability | **NOT MEASURED** |
| reconstruction | unaffected |
| commercial differentiation | no evidence, and none is claimed |

The smallest honest change, **not recommended on this evidence**, would be one
additional integer in the `cost --json` document summing records read and not
admitted. It is one field and no architecture. It should wait for a corpus in
which it is non-zero.

---

## 8. Prior-art and engineering classification

The surviving mechanism is **parser diagnostics plus accounting denominator
disclosure**. Both are ordinary engineering:

- Separating "unreadable" from "not applicable" is Codd's missing-but-applicable versus missing-and-inapplicable, proposed in the 1980s and declined by the ISO standard.
- Reporting an eligible-but-excluded population beside a rate is the AAPOR disposition convention, in production in survey methodology since long before this repository.
- Publishing a denominator's coverage beside the figure is accounting disclosure.

**No novelty language is used and none is justified.** Replay combining
familiar mechanisms is not an invention, and this room's own measurement is the
argument against treating it as one: the mechanism exists, and the evidence
does not force anyone to care.

---

## 9. Final classification

### **B — DIAGNOSTIC ONLY**

Non-zero states exist. Exposing them does not materially change any published
claim.

**Why not A.** A requires that the controls establish no meaningful reporting
gap. They establish one: six excluded records, zero disclosure, reproduced in
Pass 4. The gap is real; its contents are empty.

**Why not C.** C requires that the states materially affect interpretation.
They do not: four are zero, the fifth is 100% housekeeping, and the records
behind four of the five carry no usage by their own branch conditions.

**Why not D or E.** Pass 8. Nothing here is unavailable through ordinary
engineering, and nothing surprised the room in a way that resists reduction to
parser diagnostics and denominator disclosure.

---

## 10. Next action

**Record as diagnostic and move on. This line is closed.**

Two things carried out of the room, neither of them a research programme:

**(a) A frozen defect candidate, not repaired.** The triage gate in
`internal/ledger/store.go` is `Usage == nil || len(Prompt.Messages) == 0`. The
disjunction diverts records that carry provider usage, and neither the refusal
branch nor the final fall-through re-checks usage. **MEASURED:** three
synthetic routes, each discarding 53,000 measured tokens into a counter that
holds no token figure:

| record | lands in | tokens |
|---|---|---|
| refusal carrying usage | `Refusals` | 53,000 discarded |
| status 200, usage, no prompt messages | `Skipped` | 53,000 discarded |
| status 429, usage, no prompt messages | `Skipped` | 53,000 discarded |

The third is the sharpest, because `store.go`'s own comment says the
`Usage == nil` clause exists so that such a record "stays a usage-bearing
record instead" and keeps "its measured tokens in the session totals". The
outer `||` has already diverted it before that clause is reached, so **the
guard written to protect those tokens is unreachable for them.**

**Production reachability, INFERRED from source:** `passthrough.go:175-183`
sets `rec.RequestSummary` only when `summarize` succeeds; on error the request
is still forwarded, the response is still tapped, and the record is written
with real usage and empty `Prompt.Messages`.

**Measured incidence: 0 of 17 real ledger records.** Not repaired, because
repairing a production triage path on zero observed incidence is the thing this
campaign exists to avoid. It becomes live the moment any corpus shows a
usage-bearing record with no prompt messages.

**(b) Two dead fields.** `Session.UnknownKinds` and `Session.Refusals` have
**zero production readers**. `report.go:231` already documents the second as a
known precedent: "local refusals were moved out of Skipped and nothing ever
read the new field, so the misleading line stopped counting them and no line
replaced it." `UnknownKinds` is the same shape, added later, and additionally
unreachable because no writer sets `Record.Kind`. Noted; no action, because a
counter that cannot be incremented cannot mislead anyone.

---

## Instrument failures, recorded

Two of this room's own oracles were wrong before they were right. Both are
recorded because an oracle that invents or misses a finding is the failure mode
this campaign exists to catch.

1. **The first census grep missed the dominant writer.** Patterns `\.Skipped++`
   and `\.Skipped =` do not match the struct literal `&Session{Skipped: skipped}`
   at `claudecode.go:159`, which produces essentially all 178,102. The first
   instrumentation was therefore placed at `claudecode.go:235` and recorded
   **0 of 178,102**, a positive control that failed and correctly refused to
   report a number.
2. **The first census run measured the test harness.** `os.UserHomeDir()`
   returned the suite's scratch home, because `TestMain` pins `HOME`, so both
   real roots reported ABSENT and the run "passed". Fixed by passing the roots
   in explicitly.

---

## Closing state

**MEASURED.** `go test ./...`: **37 ok, 0 FAIL**, unchanged from baseline. All
throwaway probes deleted; no `zz_` file remains under the module. Three
production files were mutated and reverted, each verified **byte-identical**
against a backup held outside the repository: `cmd/replay/cost.go`,
`internal/ledger/store.go`, `internal/transcript/claudecode.go`. `gofmt` clean.

Ownership boundary intact. `docs/WORK-STATE.md`, `docs/design/UNWIRED-LOG.md`,
`internal/regression/unwired_packages_test.go`, the modes catalogue,
`internal/stateledger/` and `.claude/` all carry their pre-room mtimes and were
not read for content, modified or staged. Nothing committed.
