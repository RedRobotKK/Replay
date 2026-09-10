# Avoidable spend is a fan-out phenomenon, 2026-09-10

> **Correction, 2026-09-10 (later the same day).** Produced while `replay cost`
> priced one lane per transcript, dropping 36.2% of requests corpus-wide and
> concentrated in exactly the fan-out sessions this file is about. The direction
> of the finding is reinforced rather than withdrawn — the sessions holding the
> avoidable spend are the ones whose cost was most understated — but every
> absolute figure below is a floor, not a total. Corrected corpus total:
> **$10,503**; corrected avoidable: **$297.25** over 61.5M tokens.

**What this measures:** where the corpus-wide avoidable figure actually sits.
The headline is a single percentage over 116 sessions; this asks how it is
distributed, because a rate that is uniform and a rate that is four sessions
are different products.

## The finding

**Twenty-five sessions that fanned out to sub-agent lanes hold 98.8% of all
avoidable spend.** The ninety-one single-lane sessions hold 1.2%.

| | sessions | spend | avoidable | share of all avoidable |
|---|---:|---:|---:|---:|
| Fan-out (lanes > 1) | 25 | $3,259.06 | $160.79 | **98.8%** |
| Single lane | 91 | $165.27 | $2.03 | 1.2% |
| **All** | **116** | **$3,424.33** | **$162.82** | 100% |

Concentration inside the fan-out group is sharper still: the **top five sessions
carry 91%** of all avoidable spend.

| session | lanes | requests | cost | avoidable | of own spend | of all avoidable | breaks |
|---|---:|---:|---:|---:|---:|---:|---:|
| `facfd32e` | 1,014 | 14,070 | $1,056.14 | $61.03 | 5.8% | **37.5%** | 558 |
| `fee79714` | 53 | 3,026 | $405.83 | $34.11 | 8.4% | **20.9%** | 34 |
| `f191c330` | 158 | 6,586 | $505.12 | $29.79 | 5.9% | **18.3%** | 115 |
| `eec05948` | 297 | 8,162 | $728.30 | $15.90 | 2.2% | **9.8%** | 33 |
| `bad7cd55` | 2 | 694 | $215.53 | $7.33 | 3.4% | **4.5%** | 2 |

> **Column corrected 2026-09-10.** The two rightmost percentages were published
> as one column headed `share`, carrying the of-own-spend figures. The paragraph
> above the table claims the top five hold 91% of all avoidable spend, and that
> column sums to 25.7% — so a reader who added it up got a number contradicting
> the sentence two lines earlier. The values are unchanged and nothing was
> re-derived; the of-all-avoidable column is new, and it is the one the finding
> rests on. It sums to 91.0%.

## The two true sentences that point opposite ways

**96 of 116 sessions have zero avoidable spend.** By count, 83% of this corpus
is clean, and a per-session report would tell most readers there is nothing to
find.

**Those 96 sessions hold $76.81 — 2.2% of all spend.** By money, they are
rounding error.

Both are measured and neither is the whole picture. Quoting the first alone
makes the tool sound useless; quoting the second alone hides that most
individual sessions genuinely have nothing in them. The distribution is the
finding, not either summary of it.

## What this does to the "one-time fix" objection

[MONEY-PATH.md](../MONEY-PATH.md) states its own strongest objection: the
cleanest measurement in this repository traced every cache break in one fan-out
session to an MCP connector's tool block arriving mid-session, concluded the fix
is client-side sequencing, and therefore *"a subscription whose value is finding
that is a subscription for a fix you apply once."*

This measurement neither confirms nor refutes that, and the reason is worth
being precise about. It establishes **where** the money is — in fan-out — and
says nothing about **why** each break happened, because `replay cost` estimates
avoidable spend from the provider's usage fields rather than reading the wire.
Attributing 558 breaks in `facfd32e` to a cause needs the proxy, not this
command.

What it does establish is that the exposure is **recurring rather than
structural**: it appears per fan-out session, and this operator started 25 of
them. If the sequencing fix works it must be re-applied by every new project
that adds a connector, which is a different shape of problem from one afternoon
of work — and if it does not work, the exposure simply continues. Distinguishing
those two is a proxy measurement nobody has taken.

## What it does to the price

MONEY-PATH answers "is $25 the right price" with *"No for $25 per developer per
month. Yes for $25 per repository per month."* This corroborates that
conclusion from an independent direction.

A developer running single-lane sessions has **$2.03 of avoidable spend across
91 sessions** to find. There is no subscription in that. A repository whose work
fans out has **$160.79 across 25 fan-out sessions, 91% of it in five it could
not have picked in advance** — which is exactly the case for a standing
instrument rather than a one-off audit.

*This sentence read "a recurring five-figure-lane exposure" until 2026-09-10.
Nothing in the corpus is five figures: the largest lane count is 1,014 and the
avoidable total is $160.79. The measured figures replace it.*

The unit was argued to be the repository on other grounds. This says the money
agrees.

## Method

```sh
replay cost ~/.claude/projects --per-task --json
```

Sessions grouped by `lanes`, which `foldSessions` sets to the number of agent
transcripts folded into each row. Shares computed over `avoidableUsd` and
`costUsd` as reported; no figure here is re-derived.

## Limits

**One operator, one machine, and the corpus is live.** The total moved from
$3,411.92 to $3,424.33 during this session's own work — these figures are a
snapshot of a corpus that grows while it is read, and a re-run will not
reproduce them exactly.

**The measuring session is in the corpus.** `eec05948` in the table above is
the session that produced this document: 297 lanes, $728.30, ranked fourth by
avoidable spend. It is not excluded, because excluding the observer would be a
larger distortion than including it, but a reader should know the instrument is
inside its own sample.

**Avoidable spend here is estimated, not measured on the wire** — ADR-0002's
lower tier. It is derived from the provider's reported usage fields, so it
carries the model's assumptions about what a different layout would have read.

**This is one operator's fan-out habits.** Whether 98.8%-in-fan-out generalises
is exactly the question a pooled corpus would answer and a single machine
cannot. That is what `replay cost --contribute` was built for, and at the time
of writing the pool has one member.

---

[Evidence](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
