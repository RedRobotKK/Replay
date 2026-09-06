# 0012. Consider dual licensing, decline it, keep the option

**Status:** Accepted
**Date:** 2026-09-05

## Context

[ADR-0005](0005-apache-2-license.md) settled the licence. It did not settle the question that
keeps returning: whether Replay should be relicensed under a copyleft licence for open-source
users and sold under commercial terms to everyone else, on the MySQL and Qt model.

The question was raised again on 2026-09-05, from the position that the project has enough
value that a fork could out-earn its author. It was examined twice, independently, and both
examinations reached the same answer for the same primary reason.

**Copyleft leverage requires a trigger, and Replay does not offer one.** GPL obligations attach
on *distribution*. Replay is a standalone binary and a local proxy: `defaultListen` is
`127.0.0.1:4000` and nobody links it into a product, so nobody ever distributes it. A company
running it on a hundred laptops distributes nothing and would owe nothing. AGPL §13 attaches
instead on *remote network interaction*, which a single-user loopback proxy is not, so it does
not bite either. **A commercial licence nobody is legally obliged to buy is a price list, not
a business.**

Two secondary arguments were offered and are recorded here as weaker than they look:

- *That gating would erode the tool's trust model.* It would not, necessarily. Dual licensing
  is a legal mechanism, not a technical one, and it does not imply licence keys, phone-home,
  or usage tracking; Sidekiq Pro ships as a private gem with none of them. The
  "[no account, no telemetry](../../README.md)" guarantee and the licence are independent, and
  conflating them would wrongly rule out ever selling anything.
- *That a CLA suppresses contributions.* Real, but it is an argument against a CLA, not against
  dual licensing, and the two are not the same decision. See below.

## Decision

**Replay stays Apache 2.0, single-licensed.** No copyleft tier, no commercial tier, no
relicensing.

**The CLA stays.** [CLA.md](../../CLA.md) is required for contributions and grants the
maintainer the right to relicense. It is retained not because a relicence is planned — the CLA
says in its own text that none is — but because the two decisions have different reversal
costs. Declining to dual-license today costs nothing and can be undone tomorrow. Failing to
collect relicensing rights cannot be undone at all: the first contribution that lands without
them fixes the project's licensing permanently, and it does so silently, at a moment nobody
notices.

**The asymmetry is the entire argument.** Cost of holding the option: one line in a first pull
request. Cost of discovering it expired: total, and discovered late.

## Consequences

- The commercial question is deferred, not closed, and the deferral is cheap to maintain.
- Any future revenue must come from a **separate work** — a hosted service, an aggregation
  layer, an enterprise deployment — because ADR-0005 already guarantees this code cannot be
  retroactively restricted. That constraint is a feature; it is what makes the Apache grant
  credible to an adopter.
- **Zero dependencies keeps the choice free.** `go.mod` is 45 bytes with no `require` block, so
  no inbound licence constrains the outbound one. Any future licensing decision stays
  unencumbered as long as that holds, which is one more reason it should.
- The CLA's contributor cost is real and is paid at contribution number one. At the time of
  writing there are none, and the requirement is disclosed before anyone invests work.

## Amended 2026-09-06: ADR-0015 makes this deferral more durable than it knew

This ADR defers open core and says to revisit "when an enterprise surface exists and someone has
asked for one". [ADR-0015](0015-single-tenant-state-is-a-boundary.md) landed the following day and
changed the standing of that condition. Every piece of shared mutable state in the proxy is scoped
to one human and carries no tenant dimension, and 0015 records that as an **architectural
boundary** rather than an unbuilt feature. The centralised, many-developer instance is therefore
not a gap in the roadmap waiting to be filled; it is a different work, declined on purpose.

That matters here because it is the one deployment shape whose absence this ADR rests on. The
Context above declines copyleft on the grounds that a loopback proxy triggers neither distribution
nor AGPL §13 network interaction. 0015 says the architecture will not grow the shape that would
supply the trigger. **The two arguments are independent and point the same way**, so the deferral
does not quietly expire the first time someone builds a dashboard: it would take a deliberate
reversal of 0015, which is itself a recorded decision. Re-verified on the same day: `go.mod` is
still 45 bytes with no `require` block, no `go.sum` and no `vendor/`, so nothing inbound
constrains the outbound licence.

## Alternatives considered

**AGPL plus a commercial licence.** Rejected: no trigger, per the Context. Note the one
deployment shape that *would* trigger AGPL — a centralised instance serving many developers —
is a component that does not exist. If it is ever built, it is a separate work and this ADR
does not bind it.

**Open core.** Not rejected, not adopted; it is premature. It needs an enterprise surface to
withhold, and every candidate — a fleet dashboard, aggregate reporting — is explicitly a
[non-goal](../../README.md) today. Revisit when one exists and someone has asked for it.

**Drop the CLA to lower the contribution barrier.** Rejected. It optimises for a contributor
who has not yet appeared, at the cost of an option that cannot be recovered once lost.

---

[Decision records](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
