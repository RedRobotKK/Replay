# Replay 1.0 proof package

**2026-10-02. Assembled, not invented: every row points at an artifact that
already exists in this repository.**

This document says what Replay can establish, which artifact establishes it, and
what it cannot establish. It is the technical foundation of a 1.0 claim and it is
deliberately narrower than the architecture.

## The scoping decision this package rests on

**Fifteen packages have no non-test importer**, including `internal/claims`,
`internal/surface`, `internal/stateledger` and `internal/evidencepin`. They are
registered as unwired in `internal/regression/unwired_packages_test.go`.

The evidence/claim semantic machinery is therefore **research scaffolding, not
shipped product**. A 1.0 that claims to be "an evidence system for understanding
autonomous work" would be overclaiming on exactly the axis this repository
exists to police.

**What ships is a cache-and-cost forensics tool with unusually disciplined
evidence semantics.** That is what this package proves.

## Scale of the existing proof

| | |
|---|---:|
| test functions | **2,568** |
| packages green | **37 of 37** |
| evidence documents | **69** |
| registered claims, each with controls and a stated boundary | **25** |
| mutation suites enforced in CI | **5** |
| fixture corpora | **22** |
| TODO/FIXME in non-test code | **1** |

## What is demonstrated, and by what

| capability | status | artifact |
|---|---|---|
| **OBSERVE** four transcript surfaces, refuse three | ESTABLISHED | RPL-C001 BOUNDED; `TestFixtureCorporaClassifyAsTheAuditFound` |
| **absence is not zero is not unknown** | ESTABLISHED | ADR-0018; `TestGK5_AMissingVendorLedgerIsUnavailableNotZero`, `TestRB6_AGenuineZeroIsNotTheSameAsUnavailable`, `Coverage.BranchTaken` |
| **a deficit's cost is a property of the record, not of arrival order** | ESTABLISHED | RPL-C036; `claim_c036_oracle_test.go`, 7 mutations killed |
| **request-level pricing coverage of the total** | ESTABLISHED | RPL-C035 |
| **break-level coverage of the re-billed figure** | ESTABLISHED | RPL-C037 BOUNDED; `claim_c037_*_test.go`, 12 mutations killed |
| **a gate's PASS states what its figure excludes** | ESTABLISHED | SP-01; 4 mutations killed |
| **the invoice-facing median excludes rows nobody could price** | ESTABLISHED | V1; 4 mutations killed |
| **action is not outcome, inside the state model** | ESTABLISHED, UNWIRED | `stateledger.Settle` refuses a non-dispositive check |
| **eleven work states are expressible and distinct** | ESTABLISHED, UNWIRED | E4-01 |
| **the reconstruction core is provider-independent** | ESTABLISHED | E7-01: 0 non-comment provider references in `internal/analysis` |
| **warm equals cold, enforced by a derived schema key** | ESTABLISHED | `unitSchema()` reflects JSON tags into `costIndexKey()`; `TestC037R5`, `TestC037R6` |
| **refusals are cheap and explicit** | ESTABLISHED | `internal/money` entire, audited and bounded; the shipped refusal paths are held through the binary by `TestE2E_Cost` (unpriced sessions excluded and disclosed), `TestSP_TheGatePassMustStateWhatItsFigureExcludes`, `TestV1_AnUnpriceableRowIsNotAFreeRow` and `TestUnmeasuredSigmaSuppressesTheDollarFigure`. the quota package was cited here until 2026-10-02; it is not in the shipped binary and certifies nothing |

## What is NOT established, stated plainly

| | status | why |
|---|---|---|
| **work continuity** | **NOT MEASURED** | R10 stopped at a preregistered ceiling, control 5/5, treatment never ran |
| **durable scratch improves outcomes** | **NOT MEASURED** | same trial; no corpus examined carries a task-outcome signal |
| **the state model crossing a process boundary** | **NOT IMPLEMENTED** | `stateledger` has 0 JSON tags, no `Marshal`, no production caller |
| **temporal evidence frontier** | **NOT APPLICABLE to the wired product** | the analysis path is postmortem by construction; no wired surface justifies an earlier decision with later evidence. It applies only to the proxy guards, where the audited defect is the opposite: LRU eviction destroys the running total a cap is computed over |
| **influence or causality** | **NO ENDPOINT** | not attempted; no evidence supports a causal label |
| **quota titration benefit** | **RESEARCH** | `internal/quota` forecasts and never blocks, which is correct; no controller exists and none is justified |

## Release blockers: all five closed

| | blocker | resolution |
|---|---|---|
| 1 | **Q01** a session's cost depended on lane file names and on the cost index | **FIXED.** `foldSessions` recomputes both flags over every lane: `Unpriced` as AND, `MixedEpochs` as OR. Classified by tracing as a semantic attribution defect, a per-lane fact used as a session fact. `TestQ01_LaneFileNamesDoNotDecideASessionsCost` |
| 2 | **Card** said "of spend paid twice" over a share of PRICED spend, and had no ceiling guard | **FIXED.** PNG on 2026-10-01: "of priced spend, paid twice" and a measured 99.6% renders `>99%`. Text card on 2026-10-02: it had kept "of my agent spend" and rounded 99.6% to `100%`; now "of my priced agent spend" with the same ceiling. `TestShareCardNamesThePricedPopulation`, `TestShareCardDoesNotRoundToAHundredPercent`, both through `cost --share` |
| 2b | **Corpus payload** does not publish request- or break-level coverage | **POST-1.0, by evidence.** See below |
| 3 | **Proxy cap** disclosure could not fire while a comment asserted it did | **FIXED.** `listCost` returns `(cost, upperBound)`; the flag arms where the substitution happens |
| 4 | **Advisor** priced an unpriceable model at $0 and sorted recommendations on that key | **FIXED.** `cacheTrafficUSD` returns `(usd, priced)`; `Suggestion.UnpricedSessions` discloses it. The arithmetic is unchanged |
| 5 | **Published documents** read an absent `tasks`, `totalUsd` or `medianTaskUsd` as zero | **FIXED.** Every quantitative key is presence-checked. Presence, not value: a measured zero still pools |

## Scope boundary: request- and break-level coverage is NOT published

The corpus and roster publish coverage at **transcript granularity** and no finer.
They carry `unpriced` and render "N further transcripts were read by these
contributors and left out". The claim they make and the evidence they publish
are the same granularity.

Classified **POST-1.0** on three independent checks, not on implementation cost:

1. **RPL-C037's registered `Scope` is "break granularity within `replay cost`,
   human and JSON surfaces".** Its own `DoesNotEstablish` names "any surface
   other than `replay cost`" as not examined. The claim never extended here.
2. **No README or documentation file mentions `pricedRequests`,
   `unpricedRequests`, `unpricedRebilledTokens`, "request-level" or
   "break-level" coverage.** There is no public promise to substantiate.
3. The pooled figures are substantiated by the field they publish.

Adding those fields would publish evidence for a claim nobody makes, and would
change a schema with a strict allow-list, a version string and a test pinning
the field set. **The counters remain computed and are visible on `replay cost`,
where the claim that needs them lives.** Nothing was weakened to reach this.

## Known limitations

- The calibration corpus is one machine: 981 sessions, 3,153 lanes, 104,917
  requests. Nothing here generalises to a second operator.
- No authoritative billing endpoint exists anywhere in Replay. Every dollar is
  usage multiplied by a local price table.
- `go install` adoption is **UNMEASURED** and must never be inferred from clone
  counts.
- The four parsers return four different types with no normalizer. That is
  deliberate, and the cost is that disclosure discipline is re-implemented per
  provider and has drifted.

## How to reproduce

```
go test ./...                 # 37 packages
go test -race ./cmd/replay/   # concurrency
go test ./internal/claims/    # register guards: boundaries, controls, test existence
go test ./internal/regression/ # orphan links, dangling links, unwired registration
```

Mutation campaigns are recorded per claim in `docs/evidence/`, each naming its
control and every killed mutant.
