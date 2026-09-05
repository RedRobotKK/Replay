# What counts as waste, and who actually cares

**2026-09-04.** Two questions that the product direction assumed and did not answer. Working them
through changes the taxonomy and changes the pitch.

## Part 1: the definition, and what it disqualifies

Replay cannot see outcomes. It cannot tell a finished session from an abandoned one, so **"spend that
did not contribute to the result" is unmeasurable from where it sits.** Any definition that depends
on whether the work succeeded is a definition Replay cannot apply.

What it *can* compute is a counterfactual on mechanics:

> **Waste is spend that a different mechanical choice would have avoided, with the work held
> constant.**

Every term does something. *Mechanical* excludes judgement. *Work held constant* means the same
messages, same tools, same result. *Would have avoided* means the counterfactual is computable, which
is exactly what the replay engine does.

### Applying that honestly breaks the five categories

I had five and called them all waste. **Only two survive.**

| Category | Counterfactual | Verdict |
|---|---|---|
| **Cache break** | Same messages, different layout, fewer tokens billed, **nothing else changes** | **Waste.** Strictly better on every axis |
| **Re-read** | The agent read a file already in context. Same work, fewer tokens | **Waste, usually.** Not if the file changed between reads |
| **Unused tools** | Nine definitions carried and never called | **Not waste. Insurance.** You cannot know in advance which tools a task needs. Paying to carry them is a hedge, and the right question is its price, not its existence |
| **Fan-out** | Six sub-agents each paying the cache write; sequencing avoids it | **Not waste. A trade.** It buys wall-clock time with money. Someone who fanned out on purpose bought exactly what they wanted |
| **Rework** | Failed tools, repeated calls | **Mostly not waste.** A failed call that told the agent something is exploration. Only a *repeated identical* call with an identical result is unambiguous |

**So the honest split is three tiers, not one bucket:**

- **Avoidable** — cache breaks, true duplicate re-reads, identical repeated calls. Strictly better if
  removed. **This is the only tier that should ever be called waste.**
- **Priced trades** — fan-out, unused tools, retries. Cost bought something: latency, flexibility,
  resilience. Report the price, never the verdict.
- **Unknowable** — whether an exploration was worth it. Replay must not have an opinion, and today it
  implicitly does by counting `error_share` as waste.

**`error_share` as currently defined is not a waste metric.** It is a *friction* metric, and calling
it waste is the kind of overclaim the rest of this project exists to avoid.

## Part 2: nobody cares about money they cannot see

**The harder question, and the honest answer is that for most users the cost framing does not work.**

A developer on a flat monthly subscription has **no marginal cost signal at all**. Waste is both
invisible and free to them. Telling that person they wasted $3.10 is telling them about someone
else's money. **The cost pitch lands only with usage-based API billing, and that is not where most
coding-agent users are.**

### But waste is not really about money

The same tokens buy something scarcer than dollars. **A context window is a fixed budget per
session**, and everything wasted inside it is capacity that the actual work does not get.

| What waste actually costs a subscription user | Why they feel it |
|---|---|
| **The session ends sooner** | Re-reads and unused definitions fill the window. Hitting the limit is felt immediately; the money is not |
| **The agent gets worse as it goes** | A window filled with stale re-reads and never-called tools is a window with less room for the problem |
| **Everything is slower** | Every wasted token is a token transmitted and processed on every subsequent turn |
| **Rate limits arrive earlier** | The ceiling most subscription users actually hit |

**So the pitch is not "you wasted $3.10". It is: "a quarter of your context went on things that did
nothing, so your agent hit its limit a quarter earlier than it needed to."**

That is the same measurement with a consequence the user has already experienced. **Nobody has
noticed their bill. Everyone has noticed the agent getting worse near the end of a long session and
not known why.**

### Who cares about the money version, and it is not most people

- **API-billed users**, who see a per-token number.
- **Teams**, where somebody in finance asks what the line item is.
- **Anyone running agents in CI or at scale**, where it is compute, not a subscription.

That is a real market and a smaller one. **It should not be the headline.**

## What this changes

**In the tool.** Report avoidable spend and priced trades in separate columns, never one total. A
number that mixes "you could have had this for free" with "you bought latency" is not a measurement,
it is an opinion with a decimal point.

**In the pitch.** Lead with the context window, not the invoice. Dollars stay available, as a column,
for the people whose problem they are.

**In what gets claimed.** `error_share` gets renamed and demoted. It measures friction, it is useful,
and it is not waste.

## The uncomfortable version of the answer

**"How does anyone care if they did not know they were wasting?"** is not only a marketing question.
It is a demand question, and the honest answer is that **most people will not care, because the thing
they cannot see costs them nothing they can feel.**

Replay's job is therefore not to tell people they wasted money. **It is to explain something they
have already noticed** — the long session that got vague and slow near the end — and attach a
mechanism to it. **A tool that explains a symptom someone already has is a product. A tool that
reports a cost they never noticed is a curiosity.**

---

[Documentation index](README.md) · [Repository README](../README.md)
