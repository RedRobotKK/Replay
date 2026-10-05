# 28. Replay is a product with a hosted service, and three closed capabilities are reopened

**Status:** Accepted
**Date:** 2026-10-05

## Context

This record reverses decisions that earlier records called permanent. The
operator's rule of 2026-09-12 is that reversing a permanent decision takes a
dated document. This is that document. It records the decision and its stated
reason; it does not re-argue the evidence those earlier records rest on, and it
does not edit them.

**The decision maker and the reason.** Daniel Saito, operator and sole
maintainer, on 2026-10-05, after reading the 1.0 product gate and a simulated
twelve-seat business review. The reason as stated: the project has to pay for
the person who maintains it, and a fixed-fee forensic engagement is not an offer
he is prepared to make. The direction is a software product sold as a service,
with engineering effort on capability rather than on evidence documents.

**What is being reversed, and where each decision is recorded.**

| Decision | Recorded in | Standing before this record |
|---|---|---|
| No centralised or shared deployment until a tenant dimension exists in the spend guard, session table, metrics surface and credential path | [ADR-0015](0015-single-tenant-state-is-a-boundary.md) | Accepted |
| Entitlement is a signed local document; no account, no licence server, no identifier leaving the machine | [ADR-0023](0023-entitlement-is-a-signed-document-not-an-account.md) | Proposed |
| No org or tenant features; no individual subscription surface; no new measurement campaign on the operator's corpus | [REVENUE-PIVOT, section 10](../research/REVENUE-PIVOT.md) | Operator memo, 2026-09-30 |
| Quota titration closed: one titration moved the utilisation counter zero steps over 3.09M tokens | [quota-titration-2026-09-06](../evidence/quota-titration-2026-09-06.md) | Null result, published |
| Prompt and context optimisation closed: prompt-structure effects reversed sign on both replications | [FINAL-CLOSEOUT](../research/FINAL-CLOSEOUT.md) | Refuted, twice |
| A compaction-approaching signal declined; compaction segments reported, nothing predicted | [compaction-panel-2026-10-05](../evidence/compaction-panel-2026-10-05.md) | Panel decision, same day |
| Replay is not a product; the asset is the operator; sell labour | [REVENUE-PIVOT, section 1](../research/REVENUE-PIVOT.md) | Operator memo, 2026-09-30 |

**What is not being reversed.** The evidence behind each row above is unchanged
and stays where it is. The null, the two refutations and the panel decision
remain the last measured word on their questions. Reopening a capability means
building toward it under the same evidence rules, not asserting that the earlier
result was wrong.

**A fact this record has to carry about its own inputs.** The 2026-09-30 memo
says the repository accumulated its evidence "across eighteen months". The
first commit is dated 2026-09-02. On the date of this record the repository is
33 days old, with 705 commits over 24 active days. Every commercial null in the
tree was measured inside the project's first five weeks, and the install-fetch
count that anchors the distribution arithmetic in
[MONEY-PATH 0.4](../MONEY-PATH.md) was read on day five. The earlier memo's
duration is a defect in that memo and is corrected here rather than carried.

## Decision

Replay becomes a software product with a hosted service. The local binary
remains the sensor and stays free; the hosted service holds accounts, settings,
aggregates and verdicts, and is the paid surface. Three capability lines are
reopened as product work, in this order: compaction management, prompt and
context optimisation, quota titration. The fixed-fee forensic engagement is not
offered and is not to be proposed again.

**Amendments to the standing rules, stated one by one.**

1. **ADR-0015 is amended, not discarded.** A hosted service may be built. The
   tenant dimension that ADR-0015 names is the first engineering unit of that
   service, not a reason to defer it. ADR-0015's refusal to present a shared
   deployment as a configuration of the local one still holds.
2. **ADR-0023 is superseded.** Accounts and a hosted entitlement check are
   permitted for the hosted service. The local binary keeps working with no
   account and no network call; the README's footprint promise applies to the
   binary, and the hosted service is a separate, opt-in surface.
3. **Transcript bodies stay on the machine by default.** The hosted service
   receives aggregates and settings. If a capability needs transcript content
   on the server, the upload is explicit, per upload, and the user sees what is
   sent before it goes. This replaces the unconditional ban with a consent
   rule, because the three reopened capabilities can be designed on aggregates
   and the ban was written for a product that had no server at all.
4. **Billing as a percentage of a saving stays banned, and so does invoicing
   from an estimate.** These were evidence rules, not commercial ones. Replay
   has no measured saving to take a percentage of, and the wording ban on
   forecast language in product output is unchanged.
5. **The SPONSORS promise is kept.** Nothing free today becomes paid. The paid
   capabilities are the reopened lines, none of which exists in a shipped
   release.
6. **The unit of sale is not decided here.** Per-seat was closed on arithmetic
   in [MONEY-PATH](../MONEY-PATH.md) and that arithmetic stands. The hosted
   service's unit is an open question for a later record with a buyer behind
   it.

## Consequences

Easier: product work on the three lines can start, each under the repository's
existing gates (failing test first, frozen mutant, wiring and production-grade
matrices, guard reachability). The evidence programme on the operator's own
corpus stops after the frozen TTL schedule completes; nothing new is started on
that machine. The E9 field study runs to its preregistered deadline untouched.

Harder, and accepted:

- **Compaction management** starts from a panel that declined a predictive
  signal because the context window tier is not derivable from the model id.
  The first unit reports what is recorded (distance to the ceiling observed for
  this model on this machine, what survived a boundary, what was re-read),
  labelled estimated, and does not predict.
- **Prompt and context optimisation** starts from two refutations whose shared
  cause was the absence of a task-outcome signal. The first unit is that
  signal, captured locally, so an optimisation can be scored on an outcome
  rather than on tokens. Shipping an optimisation before the signal exists
  repeats the refuted experiment.
- **Quota titration** starts from a null that needs a second account and a
  model-weighting pilot before it is measurable at all. That is a resource
  decision for the operator; no quota is spent on it without authorisation.
- **Public statements that become false when the hosted service ships** and
  must change on that day, not before: the README footprint line ("No account,
  no telemetry"), the MONEY-PATH enterprise section that sells "the absence",
  and ADR-0023's table of what can be hosted. Until the service ships, every
  one of those statements remains true of the shipped binary.
- **The security position changes.** A hosted service that holds settings for
  many users is a target the local binary never was. ADR-0015's three hard
  items (per-tenant accounting, authenticated metrics, a threat model for any
  remote policy channel) are the entry conditions for the service, in that
  order.

Risk accepted: the three reopened lines may each end where they ended before.
If they do, the result is recorded against the same evidence rules and this
record is superseded in turn.

## Alternatives considered

**Keep selling labour.** The 2026-09-30 memo's recommendation, with a
preregistered offer and thresholds. The operator declined it on 2026-10-05: he
is not prepared to make the offer, and an experiment the operator refuses to
run has no result. It loses on that fact, not on its design.

**Reopen the capabilities without a hosted service.** Build compaction
management, context optimisation and titration into the free local binary and
keep the ban list intact. It loses because the operator's stated requirement is
revenue, and the repository's own arithmetic says the free binary has no paid
line without a capability that does not exist today and a surface to sell it
on.

**Reverse everything, including the evidence rules.** Allow percentage billing
and forecast wording so the product can be sold on a saving. It loses because
no saving has been measured, and a product that says one would be making the
claim this repository was built to refuse. The evidence rules are the part of
the project a competitor cannot copy in a quarter; they stay.
