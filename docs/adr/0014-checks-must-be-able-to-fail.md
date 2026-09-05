# 14. A check must be able to fail, and we verify that mechanically

**Status:** Accepted
**Date:** 2026-09-05

## Context

On 2026-09-05 this project found and fixed roughly a dozen defects. Almost none
of them were missing tests. They were **checks that could not fail** — code that
looked like verification, passed review, and would have passed forever.

The list is worth keeping, because the shapes recur:

- **A grep standing in for a capability guarantee.** The promise that the binary
  cannot sign a transaction was enforced by searching source for eight literal
  strings. Two independent reviewers each wrote a working ECDSA signer that
  passed it: `crypto/ecdsa` is not needed to sign, `X-PAYMENT` is case-sensitive
  while Go canonicalises header names, and `PrivateKey` and `Sign(` are
  identifier names a rename removes.
- **A paywall no test imported.** Deleting the payment gate from a route handler
  left 25 of 25 tests green, because every test imported the library and nothing
  imported the route.
- **An assertion comparing a field to itself.** The export test checked
  `ReadMult` against the same raw struct field the exporter copied, so it could
  not fail for exactly the four rows where the value drifts.
- **A guard shadowed by an earlier one.** The payee check sat behind a
  differentiation check that always refused first. Disabling the payee check
  changed nothing observable. This happened twice.
- **An orphan-document checker satisfied by its own prose.** It counted the
  filename anywhere in the corpus, and a sentence naming the file met the bar.
- **A drift check reading a stale binary.** `[ -x ./replay ] || go build`
  short-circuits, so a binary from an earlier session vouched for a feed it
  predated.
- **A fixture that could not distinguish the bug.** Warm-write evidence was
  accepted because every test fixture had a zero cache read.
- **A skip reported as a pass.** A missing fixture called `t.Skipf`, and Go
  reports a skipped test as a passing package.

Several of these were written the same day, to catch the earlier ones.

The common property is not carelessness. Each check was written by someone
thinking about the right risk. What each lacked was any evidence that it could
distinguish the world it guarded against from the world it lived in.

## Decision

**A check is not evidence until it has been observed to fail.**

Three practices follow, in increasing order of how much they cost:

**1. Mutate before trusting.** Before a new test, gate or assertion is
believed, reintroduce the defect it exists to catch and watch it go red. Then
revert. Report the score — "16 mutants, 16 caught" is information; "tests pass"
is not.

**2. Decisions live in pure functions; shells are addresses.** The route
handlers in this project's sibling repository now contain no decisions. Every
refusal, every price, every guard lives in a function a test can call directly.
This is not tidiness: the untestable shape *was* the vulnerability, and a route
that cannot be imported is a route with no paywall.

**3. Reachability is asserted mechanically.**
`scripts/check-guard-reachability.mjs` neutralises each conditional in a
decision module and runs the tests that cover it. A conditional whose removal
changes nothing observable is reported: it is either unreachable, or untested.
Both matter.

That checker is the first thing here that attacks the class rather than the
instances, and it earned its place by finding the original bug from a green
suite — reconstructing the shadowed-payee condition, it named the exact line
with nobody knowing to look. On its first full run it found nine more gaps.

## Consequences

- **Guards are cheap to add and now cost something to keep.** Every new
  conditional in a decision module must be distinguishable by some test, or the
  build says so. That is the intended friction.
- **Two rules keep the checker from becoming what it hunts.** It refuses to run
  against a red baseline, because every mutant looks caught when the suite is
  already failing. And a conditional it cannot safely neutralise is reported as
  **unverified**, never skipped — which is how it disclosed its own blind spot
  with brace-less one-liners, behind which three real guards were hiding.
- **It proves reachability, not correctness.** A guard could flip an outcome
  that is wrong in both directions and still pass. This closes the "cannot
  fail" class and nothing more, and claiming otherwise would be the same
  over-reach the whole record is about.
- **Fixtures are part of the check.** Several defects survived because the test
  data could not express the failure: no cache read, equal token counts, no
  adjacent invalidations. A mutation that survives is as often a weak fixture as
  a missing test, and the fixture is where to look first.
- **This is why figures carry truth tiers.** `measured`, `estimated`,
  `structural` and now `declared` exist for the same reason as this record: a
  number that cannot be wrong in a visible way is not a measurement.

## Alternatives considered

**Code review.** Every defect above passed review, several of them mine. Review
is good at intent and bad at absence — nobody notices the input that was never
tested.

**Coverage thresholds.** Every one of these lines was covered. Coverage says a
line executed, not that anything depended on the result.

**A single catch-all check.** Considered and rejected on the evidence: every
defect here was caught by a specific check that could fail for a specific
reason. A general check that catches everything is a check that passes
everything, which is the failure class itself, one level up.

---

[Decision records](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
