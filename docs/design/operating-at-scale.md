# Operating 500 to 1,000 repositories, as one person

**2026-09-13.** [`../MONEY-PATH.md`](../MONEY-PATH.md) section 0.3 says $500,000
post-tax needs roughly **469 paying repositories** at the $199 figure, and the
enterprise blend needs fewer customers but larger ones. It sizes the revenue and
stops there. This asks the next question, which nobody has asked: **what does it
take to run that, and what breaks first?**

**Everything here is modelled, not measured.** There are zero customers. Rates
taken from adjacent categories are labelled where they are used, and the point
of writing it now is that the failure modes are cheaper to see than to discover.

## The short answer

Nothing about 500 repositories is hard because of volume. Every hard thing in
this document is hard because of a design decision already taken, and most of
them are decisions worth keeping.

**The binding constraint is not support and it is not infrastructure. It is that
the PAID product does not exist.** `replay gate` is unbuilt and there is no
payment rail. The two things this file originally also listed as missing were
not: `replay budget` ships, and the `crypto/ed25519` allowlist narrowing that
entitlement needs landed on 2026-09-07. See gap 1. At 500 repositories the operational load is real but survivable;
at zero repositories the operational load is irrelevant, and that is where the
business is today.

So this is a plan for a problem worth having, written so that the shape of it is
known before the first customer rather than after the fiftieth.

## Gap 1: the paid capability does not exist

| | |
|---|---|
| **Gap** | `cmd/replay/gate*.go` does not exist |
| **Blocks** | Everything. There is nothing to sell, so there is nothing to operate |
| **Size** | Step 5 of `MONEY-PATH.md` section 5. Steps 3 and 4 are done |

**Corrected 2026-09-13, hours after this file was written.** The line above
originally said "Neither does `replay budget`, the free artefact it compares
against." That was wrong. `cmd/replay/budget.go` is 300 lines, dispatched at
`main.go:191`, with `--json` and roughly 530 lines of tests across three files.
Step 4 of the money path was already done when this document said it was not.

**And the second correction matters more than the first.** A free,
build-failing CI gate already ships: `replay cost --max-avoidable-usd`
(`cmd/replay/costgate.go`) exits non-zero when measured avoidable spend crosses
a ceiling. So the question is not whether a gate exists. It is what the PAID
gate can be that the free one is not, and `SPONSORS.md` binds the answer:
nothing free today ever becomes paid, so `replay gate` must be a demonstrably
different capability, standing cost against a committed artefact, rather than a
better version of something already given away. If that distinction cannot be
stated in one sentence a customer would accept, the paid capability does not
exist yet regardless of how much code is written.

Both errors came from reading `MONEY-PATH.md` rather than the tree. The money
path was written before those commands landed and was never re-read against the
code.

The boundary that survives all of this is the one worth keeping: the gate reads
Replay's own artefact and never the repository's agent configuration, because
`.mcp.json` can hold credentials and this tool reads no configuration it was not
explicitly pointed at.

**One thing the artefact is missing, and it is the same defect v0.6.0 just
fixed.** `budgetFile` carries `{schema, generated, standing, servers, measured}`
and no `binaryVersion`, `commit` or `pricingDigest`. Those three fields were
added to corpus submissions this week because two builds priced one corpus at
$4,088.49 and $11,969.37 under a single rules label. The budget artefact is that
defect re-committed, in the file whose job will be to fail a stranger's build,
and the gate has no other oracle: standing cost is computed by the same code
that computed the budget, so if the cost model drifts they drift together and
the gate silently never fires. The digest is what makes the gate refusable
rather than quietly wrong, and it belongs in the artefact before the gate exists.

## Gap 2: entitlement issuance is a job nobody has costed

[ADR-0023](../adr/0023-entitlement-is-a-signed-document-not-an-account.md) is
right and it has an operational shape nobody has drawn. An entitlement is a
signed file, verified offline, expiring on a date read from the local clock.
There is no licence server, which is the point, and there is therefore **no
remote revocation**, which the ADR states plainly as the accepted cost.

At one customer that is a non-issue. At 750 it is an engine:

| Entitlement term | Documents to sign per year | Per working day |
|---|---|---|
| Annual | 750 | 3 |
| Quarterly | 3,000 | 12 |
| Monthly | 9,000 | 36 |

**The term is not a billing detail, it is the whole operational design.** Monthly
billing with an annual entitlement means a customer who stops paying in month two
keeps a working binary for ten more months, because nothing can reach out and
turn it off. Monthly entitlements close that and create 36 signings a day, which
is a robot, and that robot holds the signing key.

The signing key lives off the build machine and off CI, which is correct, and a
robot that signs 9,000 documents a year is a machine that must hold it. **That
tension is unresolved and it is the single most important unaddressed design
question in the commercial path.** The likely answer is annual terms with monthly
invoicing and an accepted revocation lag, priced in, and that should be a written
decision rather than a default.

## Gap 3: the unit is self-reported and unverifiable

A repository count is a number the customer tells you. Replay does not phone
home, cannot enumerate a customer's repositories, and by ADR-0015 has no
central view of anything.

**That is not fixable without breaking the product's core promise, so it should
be a stated policy rather than an unspoken hole.** The honest version is an
honour system with the arithmetic visible, which is a model that works in
developer tooling more often than the fear of it suggests. Section 0.6 of the
money path already moves the unit to a CI pipeline for a different reason, and
that helps here too: a pipeline is more visible to the buyer's own finance
process than a repository count is.

## Gap 4: tax and invoicing at 500 customers is a second job

A Japanese K.K. selling software to businesses in the US and EU:

- **EU B2B** is reverse charge with a valid VAT number, so the obligation is
  collecting and validating VAT numbers, not remitting. **EU B2C** is a VAT
  registration and quarterly filing.
- **US** has no federal sales tax and roughly twenty states treat SaaS as
  taxable, each with its own nexus threshold.
- **Japan** has consumption tax and the qualified invoice system.

At five customers this is a spreadsheet. At 500 it is either a merchant of
record, which takes a percentage and assumes the liability, or an accountant and
a filing calendar. **The percentage is the cheaper answer at this size and it
should be modelled into the price rather than discovered after it**, because
`MONEY-PATH.md` section 0.3 currently assumes 3% payment fees and a merchant of
record is typically 5% to 8%.

This is also the gap that most argues for the enterprise shape. Twelve invoices
by bank transfer is a morning. Five hundred self-serve subscriptions across
thirty tax jurisdictions is a function.

## Gap 5: support has a published promise and no measured rate

`SUPPORT.md` promises triage within two working days and `SECURITY.md` an
acknowledgement within three. Both now carry a launch-week caveat, which was the
right first move and is not a capacity plan.

**Modelled**, using a 3% to 8% monthly contact rate, which is the range adjacent
developer tools report and which this project has never measured:

| Repositories | Contacts per month | Per working day |
|---|---|---|
| 500 | 15 to 40 | 1 to 2 |
| 1,000 | 30 to 80 | 1.5 to 4 |

That is survivable solo only while each contact is short. Two things make them
short and both are already true of this codebase: refusals name their own
reason, and the evidence files answer the question behind most questions. Two
things make them long and neither is solved: there is no status page, and there
is no way for a customer to see their own entitlement state without asking.

The honest reading is that **one person can support 500 repositories and cannot
support 1,000 while also building**, which is why the money path's own blend
assumes one hire before the top of this range.

## Gap 6: the aggregation every customer will ask for is refused

[ADR-0015](../adr/0015-single-tenant-state-is-a-boundary.md) refuses a central
dashboard until a tenant dimension exists in the spend guard, the session table,
the metrics surface and the credential path. It is correct: a shared deployment
today would turn one team's day cap into an organisation-wide denial of service,
and `requirements.md` already carries SP-6's acceptance test, written before the
feature precisely because it fails against the current guard.

It is also the single most requested thing in this category. **At 500
repositories, "show me all of them in one view" stops being a feature request
and becomes the reason a buyer chooses a competitor.** The gap is not that the
refusal is wrong. It is that nothing has costed the tenant dimension, and it sits
on the critical path to the enterprise tier that the $500k blend depends on.

## Gap 7: the free tier is the support surface, and it is unbounded

Every paying repository implies some number of free users, and free users are the
distribution channel that makes the paid tier possible. They also open issues.

**This is the gap with the least available leverage and it is worth naming
anyway**, because the obvious lever is the wrong one: a project whose
differentiator is that it publishes its own corrections cannot start triaging by
who paid. `SPONSORS.md` already forbids it in writing, and that promise is worth
more than the hours it costs.

## What breaks first, in order

1. **Entitlement issuance**, at roughly 50 customers, if the term is shorter
   than annual. It is manual today and there is no design for making it not
   manual that keeps the signing key off a robot.
2. **Tax compliance**, at the first EU consumer or the first US state threshold,
   whichever arrives sooner. This one has a legal deadline attached rather than
   a degradation curve.
3. **The aggregation refusal**, at the first customer with more than about twenty
   repositories, who will ask for one view and be told no.
4. **Support**, somewhere between 500 and 1,000, and later than instinct
   suggests because the product answers most of its own questions.
5. **Infrastructure**, last and by a wide margin. The rules feed is a static
   document and the pool endpoint takes one small write per contribution. Neither
   scales with paying customers.

## The recommendation this produces

**Do not build for 500 self-serve repositories.** The money path's own blend
reaches $500k post-tax through 10 to 14 organisation agreements plus a forensics
week, and every gap above is smaller at 14 customers than at 500: entitlement is
14 signatures, invoicing is bank transfer, tax is 14 reverse-charge lines, and
aggregation is a conversation rather than a product gap.

Self-serve should exist because it is the funnel and because it is how the first
corpus that is not the maintainer's laptop arrives. It should not be the thing
the revenue plan depends on, and the reason is not pricing. **It is that one
person can run fourteen relationships and cannot run five hundred, and the number
that decides which is 469 rather than 14.**

---

[The money path](../MONEY-PATH.md) ·
[ADR-0015](../adr/0015-single-tenant-state-is-a-boundary.md) ·
[ADR-0023](../adr/0023-entitlement-is-a-signed-document-not-an-account.md) ·
[Roadmap](../ROADMAP.md)
