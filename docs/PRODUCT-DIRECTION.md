# The measurement layer for context engineering

**2026-09-04.** A product direction, written after auditing what Replay already computes. It proposes
almost no new measurement: the argument is that the most valuable thing here is already being
calculated and is being reported as something smaller than it is.

## The observation

Everyone measures **tokens**. Dashboards, billing pages, a dozen wrappers. Tokens are the input to
the question nobody answers, which is: **how much of that bought nothing?**

Replay already computes the answer and calls it cache diagnostics.

| Signal in the code today | What it actually measures |
|---|---|
| `error_share` | The share of a session's spend that went to failed tools, failed edits and repeated identical calls. **Money that bought nothing** |
| `re_reads` | File reads that repeated a path already in context. **The agent forgot something you had already paid to tell it** |
| `cache_breaks`, `prefix_changes` | Re-billing caused by a layout that shifted. **Mechanical waste** |
| `unused-tools` | Tool definitions carried on every single request and never once called. **Rent on capability you did not use** |
| `hot-file` | The same file read over and over |
| `first-turn-content` | Content loaded at turn one and never referenced |
| `large-results` | Tool results that dwarf what was done with them |
| `held_ms` | Parallel sub-agents each paying the cache write for the same prefix. **The cost of fanning out** |

**That is a waste taxonomy, and it is already implemented.** What is missing is that it is presented
as eight unrelated diagnostics rather than one number with a breakdown.

## The direction

**Stop reporting what a session cost. Report what it wasted, and in which of five ways.**

1. **Mechanical** — the cache broke. Fix the layout. Replay already advises here.
2. **Rework** — the agent failed and retried. `error_share`.
3. **Forgetting** — it re-read what it already had. `re_reads`.
4. **Over-provisioning** — tools and context carried and never used. `unused-tools`,
   `first-turn-content`.
5. **Fan-out** — parallel siblings each paying to write the same cache. `held_ms`.

Each has a different fix, and that is the whole point. A cost dashboard tells you the number is big.
**A waste breakdown tells you which of five things to do about it**, and four of those five have
nothing to do with caching.

## Why this is the differentiator, in one line

**Nobody else can compute it.** A billing page sees totals. A wrapper sees its own calls. Replay sees
the **turn-by-turn reconstruction of a real session, with the provider's own usage numbers attached
and a calibration figure saying how much to trust them.** Rework, forgetting and fan-out waste are
only visible from there.

The field has moved from prompt engineering to context engineering in about eighteen months, and
**there is no measurement layer for it.** People are making context decisions — what to load, when to
compact, when to start fresh, how many sub-agents to run — on instinct, because nothing tells them
what the last decision cost. That is the gap.

## What to build, in order

**1. One number, with a breakdown.** `replay` already prints a policy table. Add a line above it:

```text
Session cost $12.40, of which $3.10 (25%) bought nothing:
  rework            $1.60   failed tools and repeated calls
  cache breaks      $0.90   3 breaks, largest at turn 32
  forgetting        $0.40   11 re-reads of 4 files
  unused tools      $0.20   9 of 23 definitions never called
```

Every figure here already exists. This is presentation, not new measurement, and it is the single
highest-leverage change available.

**Update, 2026-09-06: the headline half of this shipped, the breakdown half did not.** `replay cost`
now leads with one number — the avoidable amount, as a share of the total — and bare `replay` prints
it with no arguments at all. It also does something this section did not anticipate: it states the
same finding in **tokens** as well as dollars, and names who the dollars are for. The reason is in the
next paragraph but one — most readers hold a flat seat, and a dollar-only waste figure is addressed to
a minority. What is still missing is the five-way split. Today the report says how much was avoidable
and `replay diff` says which causes produced it; nothing puts the two on one screen.

**2. The compaction question, which nobody has data for.** Replay can see the turn where a session
stopped benefiting from its own history: where re-reads climb, where the cached share stops paying
for the prefix it carries. **"You should have started a fresh session around turn 40"** is a claim
only this tool can make, and it is the decision agent users make most often and most blindly.

**Update, 2026-09-06: the data now exists, and the advice still does not.** Claude Code records each
compaction with the prompt size before and after it, and nothing here was reading that field. It is
read now. Across this corpus the client wrote 39 such records, at a median of 999,029 tokens before
against 23,218 after — a **retention of 2.55%**, which is to say a compaction throws away
ninety-seven percent of the context and the session keeps going.

The first thing that bought was a correction rather than a feature: `replay context` was ranking
everything that had ever entered a session as though it were still there, which describes a context
that no longer exists. It now says by how much its own answer overstates. "Start fresh around turn 40"
is still not a claim this tool can make — knowing what a compaction dropped is not knowing whether the
session would have gone better without it — but the measurement that claim would have to rest on is
now taken instead of assumed.

**3. Fan-out economics.** `--hold-siblings` exists because parallel sub-agents all pay the cache
write. The cost is real. The *finding* was not, and this section is kept as written plus this
correction rather than quietly rewritten, because the mistake is more instructive than the claim.

The premium separates into two factors, `[ sum(P_i) / (k * P_max) ]` and
`[ 1.25k / (1.25 + 0.1(k-1)) ]`. The second holds no data - it is fixed by the group size and the
provider's own multipliers, and exceeds 1 for every `k > 1`. So no corpus can produce a premium
below 1, and the measured 1.68x / 2.56x / 3.34x sit at 88-99% of that ceiling. The only empirical
quantity is how equal sibling prompts are, about 0.9 and flat. See
[the correction](evidence/fan-out-premium-2026-09-06.md).

The "4.2x" above was never measured. It was written as an example of a publishable *sentence*, and
it survived several readings as though it were a number. Note what it is: the arithmetic ceiling for
six lanes is `1.25*6 / (1.25 + 0.1*5)` = **4.29**. An invented plausible figure landed on the
identity, which is exactly why an identity is so easy to mistake for a discovery.

The publishable sentence, honestly: **"Six sub-agents are billed about 4.3x one agent, and that
number is the price list rather than a property of your workload."**

**4. Cross-session, not per-session.** `advise` already aggregates. The habits are the target, not
the session: the file you always load and never use, the tool nobody calls, the instruction block
that breaks the cache every Monday because it has a date in it.

## What not to build

**Not a dashboard.** The audience runs a terminal and the data is local. A web UI adds a server, a
privacy question and a support burden, in exchange for charts nobody needs.

**Not autonomous optimisation.** `replay learn` already selects policies, and it refuses when
calibration is weak. That restraint is the product's credibility. **Advice a person acts on beats
changes a tool makes quietly**, especially in a request path.

**Not a score out of 100.** A single quality grade invites gaming and hides the breakdown, and the
breakdown is the useful part.

## The honest constraint

All five waste categories are computed from **78 sessions on one machine, one account and one
operator** — the figure this document originally gave as eleven, and later as 1363, which was a count
of transcript *files* rather than of independent draws
([correction](evidence/calibration-corpus-2026-09-06.md)). More files did not make it a wider sample.
The taxonomy is sound; the thresholds that turn a measurement into advice are not calibrated. **Ship
the breakdown before the advice.** Showing someone where their money went is defensible on 78
sessions from one operator. Telling them what to change is not.

The break-cause study on 2026-09-06 is the sharpest available demonstration of why. Run over the 40
largest sessions it said TTL expiry was 75.2% of re-billed tokens and prefix layout was not worth
building. Run over all 1,506 transcripts it said layout-addressable causes were 63.6% and TTL 33.9% —
the opposite conclusion, from the same tool on the same machine, five seconds apart
([evidence](evidence/break-causes-2026-09-06.md)). If a change of sample can invert the answer within
one operator's data, a threshold fitted to that data is not a threshold.

There is a second, quieter constraint that governs how any of this is worded. For a subscriber a
broken cache costs no dollars, and whether it costs *anything* — rate-limit budget, in practice — was
measured and came back null ([titration](evidence/quota-titration-2026-09-06.md)). So the waste
breakdown has to be stated in tokens as well as money, which `replay cost` now does. A breakdown
denominated only in dollars is a breakdown most of the audience will read as somebody else's problem.

---

[Documentation index](README.md) · [Repository README](../README.md)
