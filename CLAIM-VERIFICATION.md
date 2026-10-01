# Claim verification

**Campaign run 2026-09-30 against `2687d9e`. Reproduce with the command in the
last section.**

This is not a summary of what Replay does well. It is a register of what the
product asserts, what the evidence actually supports, and where the two differ.

---

## Where the authoritative status lives

**Per-claim status is NOT in this document.** It is generated from
`internal/claims` into [CLAIM-REGISTER.md](CLAIM-REGISTER.md), which cannot
drift from the code because it is rendered from it:

```sh
go run scripts/claim-register/main.go
```

This document holds the narrative findings only. Where the two differ, the
register is correct, and an earlier version of this file carried a per-claim
table that went stale within two attacks. It has been removed rather than
maintained in parallel.

## The load-bearing finding

> **No code path in the repository reaches a provider invoice, balance or
> usage-report endpoint. Every dollar figure Replay prints is reconstructed:
> provider-reported usage multiplied by a locally held price table.**

This is structural, not incidental, and it is the boundary most at risk of
being crossed by accident, because the README names a truth tier "measured".
That word is accurate about the **usage** and silent about the **price**.

The proof is the only kind available for an absence, so it is built to be
falsifiable at three points:

1. A static scan enumerates every outbound destination in non-test code and
   finds none matching an authoritative-billing shape.
2. The scan is shown to find the destinations that **are** there, so a clean
   result cannot come from a broken scanner.
3. The billing detector is shown to fire on a planted `…/v1/invoices` and
   **not** to fire on an ordinary `…/v1/messages`, so it is neither dead nor
   indiscriminate.

Then the positive demonstration. Holding provider-reported usage identical and
changing only the local price basis:

| Basis | Figure |
|---|---|
| List table | **$0.020400** |
| A negotiated table, 20% off | **$0.016320** |

**A figure that changes when the operator edits their own copy of the price
list is not a bill.** That is the whole claim, and it is now a test.

---

## What Replay can currently prove

Precise wording, because the precision is the product:

- **That a missing counter and a zero counter are different facts**, and that
  its own classifier reaches a different verdict for each. Absent classifies
  `III-no-observable`; present-and-zero classifies `undetermined`.
- **That it will refuse rather than guess** when a corpus is empty, or when
  zero write observations meet an unknown provider contract, and that the
  refusal carries a reason.
- **That an unsourced pricing assertion is rejected outright**, so the one
  input a corpus cannot check cannot enter as a fact.
- **That it does not add figures with incommensurable units** across surfaces.
- **That its cost arithmetic is correct**, against a reference implementation
  written independently of the production function.

## What Replay cannot currently prove

- **What anybody was actually billed.** No endpoint exists. NO_ENDPOINT, not
  NOT_MEASURED: the gap is structural and no amount of further work inside the
  current evidence model closes it.
- **That acting on its two highest-value findings changes anything.** The
  verifier returns `AdviceOnly` for `KindCacheBreaks` and `KindHotFile` before
  any scoring runs. The refusal is correct, and it means the two findings worth
  the most money are the two that cannot be scored.
- **That it improves agent task outcomes.** R10 stopped at a preregistered
  ceiling on 2026-09-30 and its treatment arm never ran.

## What is merely reconstructed

**Every dollar figure in the product**, including those obtained through
`replay serve`. The usage is provider-reported. The price is a local, dated,
editable table that declares its own version and staleness. Their product is a
reconstruction, and the "measured" tier covers only the first factor.

## What remains NOT_MEASURED

Realized savings. Intervention outcome. Task improvement. Error reduction.
Prevention as a verified outcome. In every case the product already declines to
assert it, which is why none of these is REFUTED.

---

## Claims that were narrowed or refuted

Current status for each is in the register. The narrative:

**RPL-C004 split into three claims**, because it carried three different proof
obligations: that a vocabulary exists (C027, UNRESOLVED), that every output
carries a status (C028, REFUTED), and that the status is correct (C029,
NO_ENDPOINT). See `docs/evidence/c004-tier-mechanism-2026-09-30.md`.

**RPL-C005 was refuted at the printed surface** and its internal half
registered separately as C026. See
`docs/evidence/c005-unpriced-granularity-2026-09-30.md`.

## Mutation results

Five mutations applied to production code, each run against the test that
should notice:

| Mutation | Outcome |
|---|---|
| Advisor scores `KindCacheBreaks` instead of refusing | **killed** |
| `NullsInterpretable` always returns true | **killed by an existing test**, not by the new one |
| Unsourced contract fact accepted | **killed** |
| Cache-read multiplier dropped from the cost legs | **killed** |
| Uncached leg zeroed | **killed** |

The second deserves its record. It initially read as a surviving mutant, which
would have meant the new test could not fail. It was mis-targeted: `Classify`
does not call `NullsInterpretable`, so the mutation never touched the branch
RPL-C008 exercises. Re-targeted at `o.WriteFieldPresent`, which that path does
use, it died. The repository's own `TestNullsNeedAPopulatedReadCounter` kills
the original.

**The honest reading is that the mutation was wrong, not the test** — and the
only reason that is knowable is that the survivor was investigated instead of
being written up as a weak spot.

---

## Oracle independence

Established three ways, in descending strength:

1. **Longhand reimplementation.** The cost oracle computes four legs from the
   published pricing definition and never imports `CostLegsUSD`. A defect
   shared between implementation and oracle cannot hide.
2. **Structural scans.** The billing and footprint claims are decided by
   scanning source text, which cannot share a bug with the runtime it inspects.
3. **Self-check on every scan.** Each scan asserts it found something before
   asserting what it did not find, so a broken walker fails loudly instead of
   passing vacuously. The register enforces this at the type level:
   `TestNothingIsEstablishedOnItsOwnAuthority` fails any ESTABLISHED claim
   whose oracle is the system under test.

---

## Known limitations of this campaign

Recorded rather than omitted, per the standard the campaign was run to.

1. ~~No true cross-session cross-wiring test exists.~~ **CLOSED 2026-09-30.**
   A two-session adversarial fixture now exists with an independent oracle
   (`oracleJoin`, which never calls `requestJoin`), five tests, and three
   mutations on the join that all kill. **No crossing was found.** One bound
   remains and is recorded on RPL-C020 rather than closed: see below.
2. **Adapter conformance is partial.** A shared suite exists for the economic
   classification layer. The `internal/transcript` parsers have per-surface
   tests and no shared contract suite.
3. **No property-based testing was added.**
4. **No realtime-versus-historical convergence test was added.**
5. **Temporal attacks were not systematically exercised.** Clock skew,
   late-arriving evidence and future-dated records have partial coverage in
   `internal/proxy/correlation_test.go` and were not extended.
6. **13 claims are registered, not the full surface.** The discovery sweep
   found roughly 64 assertion sites; they consolidate to more than 13 distinct
   claims, and the register covers the ones where over-claiming is most costly.
7. **The footprint claim is static only.** A dynamic proof needs a sandboxed
   run and was not attempted.

---

## Test inventory

| Category | Count |
|---|---|
| Register meta-tests | 5 |
| Repository-wide claim scans | 4 |
| Epistemic boundary and refusal | 3 |
| Cost and billing boundary | 3 |
| Verifier refusal | 2 |
| Unit commensurability | 1 |
| **New in this campaign** | **18** |
| Pre-existing tests reused by the register | 4 |

---

## Reproducibility

```sh
go test ./...                      # the complete suite, 37 packages
go test ./internal/claims/ -v      # the register and its meta-tests
go test ./internal/cachemodel/ -run TestC011 -v   # the billing boundary
go test ./internal/surface/   -run TestC00  -v    # absence, zero, refusal
go test ./internal/advisor/   -run TestC015 -v    # the verifier's own refusal
```

## Full existing-suite result

**37 packages, 0 failures**, at `87ca7c3`.

One failure was found at the start of the campaign and is reported rather than
omitted: `TestNoOrphanedDocuments` was red at `436276e` because an evidence file
had been committed without a link from its section index. The guard was correct
and the commit was the defect. Fixed at `2687d9e` by adding the link, **not** by
relaxing the guard.

A second guard, `TestNoNewlyUnwiredPackages`, went red when `internal/claims`
was added, correctly observing that a package absent from the binary's
dependency closure ships no behaviour. It is now registered as deliberately
unwired with a reason, as UNWIRED-LOG.md entry 16.

## The cross-wiring result

**Can an independent reviewer feed Replay evidence from two valid sessions and
make it tell a plausible but false story?**

**On every path tested, no.** Two sessions with distinct identities, request
ids, models, token counts and timestamps were built, written to disk, and read
back through the real cost report. Legitimate associations survived, nothing
crossed, and adding a foreign session changed nothing about the first.

The result is backed by an independent oracle that reimplements the join rule
and never calls `requestJoin`, and by three mutations, each of which kills the
suite: ignoring the provenance flag so synthesised ids join, never detecting
duplicates, and corrupting the join key.

**One limitation was found and is recorded rather than fixed.**

Two genuinely different requests, in different sessions, on different models,
with different token counts, are counted as one request when they share a
provider request id. **The join compares no other field before merging.**

This is not a wrong line of code. Provider request ids are globally unique in
practice, so the merge is correct under that assumption. It is a finding
because **the assumption is never verified and the join has no way to notice
when it fails**: its correctness rests on a property of the provider rather
than on anything Replay observes. `TestXW2` pins the behaviour and names the
assumption so a later reader meets it deliberately.

Corroborating on session id or model before merging would close it. That is a
production change, not a test, and this campaign does not make production
changes without a demonstrated defect.

## Recommended next experiments

Only those justified by a gap this campaign identified.

1. **Decide whether the join should corroborate before merging.** The only
   finding from the cross-wiring attack. A session-id or model check would
   close it; whether it is worth the cost depends on whether a provider id
   collision is considered reachable.
2. **Enumerate printed figures against their tiers**, to settle RPL-C004 in
   either direction rather than leaving it UNRESOLVED.
3. **A shared conformance suite for the `internal/transcript` parsers**, so a
   new adapter cannot inherit a capability it does not have.

**Not recommended:** anything that would require a provider billing endpoint.
That gap is structural and no experiment inside the current evidence model
closes it.
