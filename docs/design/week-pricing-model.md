# What the forensics week costs to produce, and what that makes it worth

**2026-09-13.** Built bottom-up because the existing band, $8,000 to $18,000,
has no derivation written down anywhere. A price with no arithmetic behind it
cannot be defended in the room where somebody asks why, and this is the one
project that does not get to answer a "why" with a feeling.

Every figure here is either measured on this machine today, taken from a named
file in this repository, or labelled as an assumption with its value stated so a
reader can substitute their own. **Nothing here is a savings forecast**, which
the deliverable rules forbid; the value argument is built on cost to produce and
on what the fee is a fraction of, never on what a customer might recover.

## 1. The week is not compute, and that is worth knowing first

Every command in the day table, run today against the real calibration corpus:

| | |
|---|---|
| Corpus | 1,950 transcript files, 2.0 GB, 121 sessions, 1,937 lanes, 72,439 requests |
| Span | 2026-08-12 to 2026-09-12, 32 calendar days, 27 of them active |
| `replay doctor` | 0.1 s |
| `replay cost` | 3.7 s |
| `replay burn` | 12.6 s |
| **Total machine time** | **about 16 seconds** |

So the buyer is not paying for processing. **They are paying for attribution and
for a written finding that survives their own engineers reading it.** That is
the honest frame and it is also the stronger one: 16 seconds of compute cannot
be marked up, and a defensible document can.

It also kills one objection before it is made. "Can't we just run the tool
ourselves?" Yes, and they should, and it is free, and it takes 16 seconds. What
they cannot do in 16 seconds is decide which of those numbers is a trade and
which is re-billed, and write down why in a form they can hand to finance.

## 2. The hours

Delivery, from the day table in
[the deliverable spec](forensics-week-deliverable.md):

| Day | Hours |
|---|---|
| 1. Read the corpus. `cost`, `burn`, `doctor`. Establish billing basis | 6 |
| 2. `diff`, `blame`, `context`. Locate breaks, attach causes | 8 |
| 3. `advise`, `route`, `trim`. What changes, what it does not fix | 7 |
| 4. Write the finding. Run the check-it-yourself script as if we were them | 8 |
| 5. Readout preparation, the recorded 60 minutes, written follow-up | 5 |
| **Delivery** | **34** |

The hours nobody counts, which are the ones that make consulting businesses
fail:

| | Hours |
|---|---|
| Discovery and qualification: can their corpus answer anything at all | 3 to 5 |
| Contracting: SOW, security questionnaire, DPA, liability cap | 3 to 8 |
| Day 0 support while **they** run `replay redact` on their own machines | 2 to 4 |
| The correction obligation. "A retraction is a delivery" is a real tail | 2 to 4 |
| Watch set up on their repositories at the end | 1 to 2 |
| **Uncounted** | **11 to 23** |

**Loaded total: 45 to 57 hours per engagement, against 34 billed. A ratio of
1.32x to 1.68x.** Any price quoted against 34 hours is being quoted against
two thirds of the work.

## 3. The floor: what a week must clear before anyone is paid

From [MONEY-PATH](../MONEY-PATH.md) section 0.2: the solo business floor is
**$25k to $45k a year**, and professional liability at **$4k to $10k** is
triggered specifically by a paid week carrying a contractual deliverable. Call
the relevant floor $29k to $55k.

| Weeks sold per year | Per week, to exist, at zero income |
|---|---|
| 6 | $4,833 to $9,167 |
| 10 | $2,900 to $5,500 |
| 15 | $1,933 to $3,667 |
| 20 | $1,450 to $2,750 |

**The direction of that table is the finding.** Fixed costs spread over fewer
engagements, so a year with few sales needs a *higher* price per sale, not a
lower one. The instinct to discount the first customer is arithmetically
backwards, and year one is the year with the fewest customers.

## 4. The ceiling: how many of these one person can actually deliver

46 working weeks, at 45% of time on delivery because the same person also sells,
maintains the price tables (MONEY-PATH calls that "the forever cost"), ships the
product and answers support:

**828 hours, which is 14.5 to 18.4 engagements a year at full utilisation.**

At 30%, during a launch or a build sprint, it is 9.7 to 12.3.

That is the ceiling, and it is not the constraint. **The constraint is demand:
zero weeks have been sold.** Capacity only starts to matter after the first two.

## 5. The risk the price has to carry

The deliverable spec says **Day 1 can end the engagement** and the remaining
days are not billed. That is the right policy and it is not free:

| Terminate at Day 1 | Realised per sold engagement |
|---|---|
| 15% | 88% of list |
| 25% | 80% of list |
| 35% | 72% of list |

A 25% termination rate takes a $20,000 list price to $16,000 realised. The list
price has to be set knowing that, or the policy quietly becomes a discount.

## 6. What the fee is a fraction of, and who the buyer therefore is

**This section was written once, was wrong by more than an order of magnitude,
and the wrong version is described here rather than deleted.**

The first version sized the buyer from this machine: $12,575.91 over 32 calendar
days is $393 per calendar day, which annualises to about $143,000 per operator,
which made eight metered operators enough to carry a $20,000 fee at a sane
percentage. That arithmetic is correct and the input is not representative.

**$393 per calendar day is about $11,950 per developer per month.** Against the
published figures:

| Source | Per developer per month | This corpus is |
|---|---|---|
| Anthropic's own Claude Code cost documentation, enterprise deployments | $150 to $250 | **48x to 80x** |
| Gartner, June 2026, the band 23 to 25% of technology leaders report | $200 to $500 | 24x to 60x |
| DX, blended seat plus token | $200 to $600 | 20x to 60x |
| Gartner, the heavy tail, about 6% of organisations | $2,000 to $4,000 | 3x to 6x |

The calibration corpus is one founder running agents at maximum intensity
through a launch sprint. **It is an outlier by one to two orders of magnitude
and cannot be used to size a buyer.** It is exactly the error this project
publishes retractions about, committed inside a document arguing for a price,
which is the worst available place for it.

### On published figures instead

A $20,000 audit, as a share of an organisation's annual agent bill:

| Developers | At Anthropic's published $150-250 | At Gartner's modal $200-500 | At the heavy tail $2,000-4,000 |
|---|---|---|---|
| 20 | 33% to 56% | 17% to 42% | **2.1% to 4.2%** |
| 50 | 13% to 22% | 6.7% to 17% | 0.8% to 1.7% |
| 100 | 6.7% to 11% | 3.3% to 8.3% | 0.4% to 0.8% |
| 200 | 3.3% to 5.6% | 1.7% to 4.2% | 0.2% to 0.4% |

The comparables in section 7 land a one-off audit at roughly **1% to 5%** of the
spend it examines. Taking 5% as the ceiling:

| Buyer profile | Developers needed to carry $20,000 |
|---|---|
| Typical spend, Anthropic's own figure | **135** |
| Typical spend, Gartner's modal band | **70** |
| Typical spend, DX blended | **60** |
| **The heavy tail, above $2,000 per developer per month** | **10** |

**So there are two buyers and they are not the same company.** Either an
organisation of roughly 60 to 135 developers at ordinary spend, or **a team of
ten in the heavy-usage tail**, which Gartner puts at about 6% of organisations.

The second is the target, and the reason is in this document's own mistake: the
outlier corpus that broke the first draft is precisely that profile. **The buyer
looks like this machine.** A team whose per-developer spend is 10x the published
median already knows it has a problem, has already been asked about it, and is
the 6% for whom the fee is 2% rather than 40%.

That is a sharper targeting statement than the first draft produced, and it came
from the figure that was wrong.

## 7. What the market pays for this shape

Normalised to a five-day single-specialist engagement whose deliverable is a
written report. Full provenance in the research note; tier is marked.

| Comparable | Normalised | Note |
|---|---|---|
| **Percona Database Health Audit** | **$11,400** | Published list price. Report in 5 to 7 business days plus a live rundown. **The closest shape of anything found**: fixed scope, written findings ranked by impact and effort, live Q&A |
| Trail of Bits, OpenZeppelin | $25,000 | Per engineer-week, from public Arbitrum and Venus procurement filings |
| Runtime Verification | $20,000 | Published rate card, per week |
| Dedaub | $17,500 | $3,500 per engineer-day published, times five |
| Spearbit | $9,500 to $16,000 | $1,900 to $3,200 per researcher-day, times five |
| Deloitte, G-Cloud 14 SFIA L6 to L7 | $12,000 to $15,250 | £1,925 to £2,450 per day at ~1.25. **The card itself says fixed-price deliverables may carry a premium** |
| Revenant Systems PostgreSQL audit | $6,200 | £4,950, two-week turnaround, effort days not stated |

Security-audit cluster mean: **$18,000 to $19,625**. Median of all midpoints:
$13,625.

Two cautions on this table. The security figures come from a market where funds
at risk inflate willingness to pay, so they are a ceiling rather than a
midpoint. And Percona's $11,400 is the most honest single comparable here
precisely because it is the least exciting one.

**Utilisation, checked rather than assumed.** Section 4 guessed 45%. The only
real survey found is SPI's 2026 Professional Services Maturity Benchmark,
n=509: **66.4% billable utilisation in 2025, an all-time low**, and that
measures staffed consultants inside firms who carry no sales or admin load of
their own. It is a ceiling for a solo operator who also builds the product, not
a comparable. 45% survives as an assumption; it is not contradicted, and it is
not confirmed either. **No citable survey of solo independent utilisation
exists**; the 120-to-160-billable-days figures in circulation are unsourced blog
assertions.

## 8. Where the two methods meet

| | Range |
|---|---|
| Market comparables for this shape | $11,400 to $25,000, clustering $17,500 to $25,000 |
| Cost floor at realistic year-one volume (6 to 10 weeks, $120k to $180k compensation) | $14,900 to $29,167 |
| **Overlap** | **$17,500 to $25,000** |

Bottom-up cost and top-down market agree, which is the only reason to trust
either. Then the termination policy is applied, because a list price that
ignores it is quoting a number nobody will realise:

| List | Realised at 15% to 25% Day-1 termination |
|---|---|
| $18,000 | $14,400 to $15,840 |
| $20,000 | $16,000 to $17,600 |
| **$22,000** | **$17,600 to $19,360** |
| $25,000 | $20,000 to $22,000 |

**The recommendation is $22,000 flat, quoted as $18,000 to $25,000 by scope.**
$22,000 is the list price whose realised revenue lands inside the overlap rather
than below it.

**The current $8,000 low end should go.** It is below every published comparable
except one whose "week" is calendar rather than effort, and it is below the cost
floor at any volume this business will see in year one. Section 3's table is the
argument: fixed costs over fewer engagements means year one is the year to
charge *more*, and year one is now.

## 9. What this does not settle

- **The comparables are mostly from adjacent markets.** No published rate card
  from anyone selling an AI-spend audit was found, because the category does not
  have public comparables yet. Percona and the security firms are the closest
  shapes available, not the same product.
- **Every hour above is an estimate by the person who would work them**, and no
  week has been run. The first engagement measures this table, and the table is
  corrected afterwards with the original left standing.
- **Utilisation is still assumed at 45%.** The only real survey measures
  staffed consultants in firms (66.4%, n=509) and is a ceiling, not a
  comparable. It remains the single figure here most likely to be wrong.
- **The termination rate is invented.** There is no history to draw it from.

---

[Design index](README.md) · [The deliverable](forensics-week-deliverable.md) ·
[The money path](../MONEY-PATH.md)
