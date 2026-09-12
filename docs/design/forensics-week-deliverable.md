# The forensics week: what the customer actually receives

**2026-09-12.** The week is the first thing Replay sells
([`MONEY-PATH.md`](../MONEY-PATH.md) rules out the per-seat subscription; the unit
is a repository). Nobody has bought one yet, so this is the shape proposed before
the first sale rather than the shape observed after it. It is in `design/`
because the question is open: the parts marked **UNDECIDED** are the ones a first
customer will settle.

## The problem this document exists to solve

A week of consulting with no defined deliverable becomes a week of whatever the
operator found interesting. The customer cannot tell in advance what they are
buying, cannot tell afterwards whether they got it, and cannot compare the second
week to the first.

Worse for this project specifically: the one thing being sold is that the numbers
are checkable. A deliverable that arrives as a slide deck of headline percentages
is the opposite of the product. So the shape is constrained by the same rules the
tool follows — every figure carries its population and its date, the three
verdicts stay separate, and a refusal is a result.

## What is handed over

Four things. Nothing else, and nothing verbal that is not also written.

### 1. The finding

One dated Markdown document, in the shape of the evidence cards this repository
already publishes — see [lane isolation](../evidence/lane-isolation-2026-09-06.md)
for the format, including the part where it retracts its own headline.

Structure, fixed:

```text
# <What was measured>, <date>

**What this measures:** one paragraph, in the customer's terms.
**Corpus:** N sessions / M transcript files / D days, read on <date>,
            at list prices dated <date> (caching rules <ruleset>).
**Billing basis:** metered | flat-seat | mixed, and what that means for
                   every dollar figure below.

## What it cost
## What was avoidable
## What was a trade
## What is not measured
## What to change, and what it does not fix
## How to check every figure here yourself
```

The section order is deliberate. **What is not measured** comes before the
recommendations, not in an appendix, because the limits decide how much weight
the recommendations carry.

### 2. The raw material

Their own outputs, so nothing in the finding is taken on trust:

- the ledger or transcripts the finding was computed from — **their file, on
  their machine**, never copied to ours;
- the exact command line for every figure, runnable by them;
- the version of `replay` and the rules document that produced it.

### 3. The check-it-yourself script

A short shell script that regenerates every number in the finding from their
corpus. If it does not reproduce, the finding is wrong and we say so.

This is the part that makes the week different from consulting. The deliverable
is not our authority, it is their measurement, which we took first.

### 4. One readout, recorded

Sixty minutes, their engineers, questions answered live. The recording is theirs.
**UNDECIDED:** whether the readout is included or a separate line item.

## What the finding may and may not claim

The three verdicts, kept apart in the document exactly as the tool keeps them
apart on screen:

| Verdict | Means | Example |
|---|---|---|
| **Avoidable** | Re-billed for a cause that had no benefit | A tool block arriving mid-session, breaking a warm prefix |
| **Trade** | Real spend bought something real | Priced fan-out: the parallelism was the point |
| **Not measured** | The instrument cannot tell | A session with no correlation data; a model with no published price |

**"Waste" does not appear in the document.** Neither does any figure derived by
multiplying avoidable tokens by a rate the customer does not pay.

Hard rules, carried over from the tool and non-negotiable in the write-up:

- **List price is not an invoice.** If the customer is on flat seats, the dollar
  figures are what the traffic would cost *someone billed per token*, and the
  document says so at the top rather than in a footnote. The tokens are still
  theirs; the dollars may not be.
- **n=1 stays visible.** One session is one session. A finding from a single
  corpus says so in the sentence that states it, not in the methodology.
- **No savings forecast.** The avoidable figure is what was *already spent
  twice*, never a prediction of what a change will recover. A recommendation may
  say what it removes; it may not multiply that by a month.
- **A retraction is a delivery.** If a figure is later found wrong, the
  correction goes to the customer as a new dated file and the original stays
  readable. That is the same rule the repository follows on itself.

## The week

**UNDECIDED: whether this is five consecutive days or five days inside a
fortnight.** A fortnight is likely better — their redaction takes wall-clock time
we do not control — but no one has run it.

| Day | Shape |
|---|---|
| 0 | They run `replay redact` themselves. We receive nothing until this is done. |
| 1 | Read the corpus. `cost`, `burn`, `doctor`. Establish billing basis and whether the corpus can answer anything at all. |
| 2 | `diff`, `blame`, `context`. Locate breaks, attach causes. |
| 3 | `advise`, `replay`, `route`, `trim`. What changes, what it costs, what it does not fix. |
| 4 | Write the finding. Run the check-it-yourself script against their corpus as if we were them. |
| 5 | Readout. |

**Day 1 can end the engagement.** If the corpus cannot answer the question — no
usage data, a billing basis where nothing is recoverable, a fortnight of
transcripts where a month was needed — that is the finding, it is delivered, and
the remaining days are not billed. **UNDECIDED:** the refund mechanics.

## The sentence that makes this not a subscription

The finding must end by saying what recurs and what does not.

`MONEY-PATH.md` already measured this: the cleanest result in the corpus traced
every break to an MCP connector's tool block arriving mid-session, and the fix is
client-side sequencing — **a free configuration change that takes one afternoon
and does not come back.** A week that finds that and then pitches a monthly
subscription is selling something the customer should cancel in month two.

So the finding names, explicitly:

- **what this week fixed, permanently** — the one-time part, which is most of it;
- **what will drift again** — configuration as the team adds servers and
  instructions, and provider prices and cache floors changing on someone else's
  schedule;
- **what, if anything, is worth watching for that drift** — and if the honest
  answer is "your own CI running `replay prefix`, free, and you do not need us",
  the document says that.

The second week is sold by the first week being true, or it is not sold.

## Open questions a first customer settles

1. Does the customer want the finding as a document or as a PR against their own
   repository? A PR is more actionable and harder to circulate internally.
2. Is the readout included or separate?
3. Five consecutive days, or five days across a fortnight?
4. What happens when day 1 ends it — refund, credit, or a reduced fee?
5. Does anyone actually want the check-it-yourself script, or is it something we
   need for our own integrity and they ignore? Worth knowing, because it is the
   most expensive part to produce.

## What this document does not cover

Price. The band lives with the commercial decisions, and nobody has paid one, so
any number here would be an opening position wearing the clothes of a rate card.
