# 0016. Relicense under the Business Source License 1.1

**Status:** Accepted
**Date:** 2026-09-06
**Supersedes:** [ADR-0005](0005-apache-2-license.md), and reverses the decision in
[ADR-0012](0012-dual-licensing-deferred.md)

## Context

ADR-0005 licensed the project Apache 2.0. ADR-0012 examined dual licensing, declined it, and
kept the option open by retaining the CLA. The owner reopened the question on 2026-09-06, while
staging the v0.5.0 release, and decided to change the licence.

**ADR-0012's central argument survives and is worth restating, because it rules out the option
most people reach for first.** Copyleft attaches to an event. GPL attaches on conveying, and a
company running Replay on a hundred laptops conveys nothing. AGPL §13 attaches on remote network
interaction, and a loopback proxy on `127.0.0.1:4000` is not that; ADR-0015 records that the
architecture will not grow the centralised shape that would supply the trigger. So AGPL would
have cost this project every legal department that bans it outright and bought no obligation in
return. **All of the cost, none of the leverage.** That option is closed, and 0012 closed it
correctly.

**Where 0012 is incomplete.** Its trigger analysis considers only the end-user shape: a developer
running the binary on a laptop. It disposes of the vendor shape in five words — "nobody links it
into a product" — which is asserted rather than argued. That is the shape that would actually
cost this project something: an observability or agent-tooling vendor embedding the attribution
engine and selling it. **That is distribution, and it would trigger.** Meanwhile 0012 celebrates
zero dependencies three times as keeping the licensing choice free, without noting that zero
dependencies also makes this engine maximally cheap to vendor into someone else's binary. The
same fact cuts both ways and only one edge was recorded.

The licences that address the vendor shape are not copyleft at all. They restrict by grant rather
than by trigger, which is why they need no distribution hook to work.

**The timing is the other half of the decision.** A licence binds a version, not a project.
Everything published under Apache 2.0 stays available under it forever, and that is by design —
ADR-0005 calls the irrevocability a feature, and it is. v0.4.0 is already out and stays Apache.
But v0.4.0 carries the `--check-prices` defect an outside contributor reported in #54, so it is
the release nobody wants to fork. **v0.5.0 is the first release worth taking.** The cost of
changing the licence is therefore at its minimum on the day before that tag is pushed, and rises
permanently the moment it is. The decision was made on that day, deliberately.

## Decision

**Replay is licensed under the Business Source License 1.1** (SPDX `BUSL-1.1`) from this commit
onward, with these parameters:

| | |
|---|---|
| Licensor | RedRobot KK |
| Change Date | **2029-09-06**, three years out |
| Change License | Apache License, Version 2.0 |
| Additional Use Grant | all production use except reselling Replay itself |

The Additional Use Grant is written to permit the thing this tool is for and forbid only the
thing that would take it. Running it on any number of machines inside your organisation,
commercially, in production, in CI, modified for internal use, and using its output for any
purpose, are all explicitly permitted. Offering it to third parties as a hosted service, or
embedding it in a product whose value derives substantially from its analysis of agent traffic,
is not.

**Three years, not the BUSL default of four.** The repository was originally scaffolded under
BUSL 1.1 with a three-year conversion, per the v4 PRD, before ADR-0005 replaced it. Returning to
three restores the original intent rather than inventing a new number.

**Trademarks are now reserved explicitly.** Apache 2.0 §6 already granted no trademark rights and
BUSL grants none either, but neither NOTICE nor README said so. A fork may take the code; it may
not take the name. This is the cheapest leverage available and it was being left on the floor.

## Consequences

- **This project is no longer open source under the OSI definition.** It is source-available. The
  README said "open source" and no longer does, because a claim that is not true does not get to
  stay for being flattering. `docs/requirements.md` is corrected in the same commit.
- **Homebrew core, and Linux distribution packaging, are now out of reach.** Both require an
  OSI-approved licence. Distribution stays the install script and GitHub releases.
- **Some corporate adopters will decline it**, and some will not read past "not open source". That
  is the price, and it is paid against an install base that is currently near zero, which is the
  cheapest moment to pay it.
- **The CLA was the mechanism that made this possible**, exactly as ADR-0012 predicted. It argued
  the CLA should be kept because "declining to dual-license today costs nothing and can be undone
  tomorrow", and tomorrow arrived. There are still no external code contributors — issue #54 was a
  report, not a patch — so no third party's copyright had to be cleared.
- **Zero dependencies is what made the change free.** `go.mod` has no `require` block, so no
  inbound licence constrains the outbound one. It is now load-bearing twice.
- The commercial offering ADR-0005 required to be a "separate work" no longer has to be. That
  constraint is lifted for v0.5.0 onward and still binds every earlier version.

## Alternatives considered

**Stay on Apache 2.0.** The strongest case against changing, and it is real: licensing leverage is
a tax on adoption, and you levy it when you have something to tax. Replay's binding constraint
today is that almost nobody has installed it, not that somebody might take it. Rejected because
the timing argument above cuts the other way — the option gets strictly more expensive to exercise
from the v0.5.0 tag onward, and never cheaper.

**AGPL 3.0, alone or dual.** Rejected on ADR-0012's reasoning, which is sound and is restated
above rather than repeated in summary. No trigger exists, so it is cost without leverage.

**Elastic License 2.0.** Very close, and simpler to read. Rejected only because it has no change
date: it never becomes open source. BUSL's conversion keeps a promise attached to the licence
itself rather than to the maintainer's continued goodwill, which suits a project whose whole
argument is that claims should be checkable.

**Open core.** Still premature, still needs an enterprise surface to withhold, and ADR-0015 says
the architecture declines to grow one. Unchanged by this decision.

---

[Decision records](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
