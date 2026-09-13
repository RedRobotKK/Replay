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
which is avoidable, and write down why in a form they can hand to finance.

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

Measured on this machine, 32 calendar days to 2026-09-12: **$12,575.91 at list
prices**, of which **$346.40 was avoidable, 2.8%**. That is **$393 per calendar
day for one operator**.

**Everything in this section is an extrapolation from one machine and is
labelled as such.** It is not a population. It is used only to answer "who could
possibly buy this", which is a question about orders of magnitude.

At that intensity one operator generates about **$143,000 a year** of agent
spend at list.

| Metered operators | Annual bill at list | A $20,000 audit is |
|---|---|---|
| 1 | $143,000 | **13.9%**, which nobody buys |
| 3 | $430,000 | 4.6% |
| 5 | $717,000 | 2.8% |
| 10 | $1,434,000 | 1.4% |
| 20 | $2,869,000 | 0.7% |

**This is the qualification criterion, and it came out of arithmetic rather than
opinion: the buyer needs roughly eight or more metered operators.** Below five,
the fee is a double-digit share of the thing it examines and the conversation is
over regardless of how good the finding is.

And the sentence that does not forecast anything:

> At ten operators, this audit costs **half of what you already spent twice this
> year**.

That is backward-looking, it is derived from a measured share, and it is the
only value sentence here that the deliverable rules permit.

**The load-bearing caveat: those dollars are list prices, and the corpus that
produced them was generated on a subscription.** For the fee to be a fraction of
a real invoice, the customer must be metered. A flat-seat buyer is not a
smaller version of this customer, they are not a customer, and the site already
says so.

## 7. What this does not settle

- **No market comparables in this file.** Cost to produce sets a floor, never a
  price. What similar specialist audits actually sell for is a separate input.
- **Every hour above is an estimate by the person who would work them**, and no
  week has been run. The first engagement measures this table, and the table is
  corrected afterwards with the original left standing.
- **Utilisation is assumed at 45%**, from nothing. It is the single figure here
  most likely to be wrong.
- **The termination rate is invented.** There is no history to draw it from.

---

[Design index](README.md) · [The deliverable](forensics-week-deliverable.md) ·
[The money path](../MONEY-PATH.md)
