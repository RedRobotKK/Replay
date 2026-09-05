# ADR-0007: Improving the cache model from many machines

- **Status:** Proposed
- **Date:** 2026-09-04
- **Supersedes:** nothing
- **Related:** ADR-0002 (replay engine and truth tiers), ADR-0006 (learning selection)
- **Amended by:** ADR-0008 (corpus at launch), which resolves the circularity in "no aggregation before the twenty-session gate": a public release is the only way to reach twenty sessions
- **Partly corrected by:** [`architecture/multi-provider.md`](../architecture/multi-provider.md), which shows this ADR assumed one learning problem where there are three

## Context

The cache model in `internal/cachemodel` is calibrated against sessions on one machine. The corpus of
2026-09-03 is honest about what that is worth: **11 sessions, one project, one machine, the
development sessions for this repository itself.** The roadmap gate asks for twenty independent
sessions and we do not have them.

Volume is not a vanity number here. That corpus already produced a finding the rules file
contradicts:

| Model | Rules say minimum cacheable prefix | Observed upper bound |
|---|---:|---:|
| claude-fable-5-1 | 512 | **at most 40,563** |
| claude-haiku-4-5-20251001 | 4,096 | **at most 14,873** |

Nothing uncached was ever seen below those bounds, so the true minimum sits somewhere in a very wide
interval and the published number may be wrong, stale, or right but untestable from this sample.
**One more machine narrows that interval. A hundred machines closes it.** The same is true of the
`fit tokens/byte` parameter, which ranges from 0.445 to 0.795 across eleven sessions of one person's
work and is currently a guess with error bars of up to ±159%.

So the question is how to learn from deployed instances without becoming the thing this tool exists
to avoid.

## Decision

**Aggregate observations about the provider's behaviour. Never collect anything about the user's
work.** Four rules, in priority order.

### 1. The unit of contribution already exists and does not change

`replay corpus` emits session id prefixes, client version, request counts, match rates, fit
parameters, prefix bounds and break causes. **No paths, no project names, no message content, no
prompts.** That was deliberate and it stays the contract. A contribution is that report and nothing
else.

**What this measures is the provider's caching behaviour, not the contributor.** That distinction is
the whole basis for doing this at all, and any field that cannot be defended on it does not go in.

### 2. Contribution is opt-in, visible, and reviewable before it is sent

`replay corpus --submit` prints the exact payload, asks for confirmation, and only then sends. There
is no background telemetry, no first-run prompt that defaults to yes, and no `--submit` implied by
any other flag. **A tool that sits in your request path does not get to phone home quietly.**

Session id prefixes come out entirely on submission. They exist so a maintainer can debug their own
report; they have no purpose in an aggregate and they are the only field that could ever correlate
two submissions.

### 3. Submissions are untrusted input, and the statistics have to survive that

Anyone can POST a fabricated corpus. The aggregate is therefore built from **robust statistics, not
means**: trimmed medians for fit parameters, interval intersection for prefix bounds, and a cap on
how much any one contributor can move any one figure.

Two thresholds before a number is published or acted on:

- **k-anonymity.** No per-model figure is published until at least **k independent contributors**
  have reported that model. Below k the row reads "insufficient data", not a number with a wide
  error bar.
- **Corroboration.** A rule only changes when independent contributors agree. One machine reporting
  a 40,563-token prefix bound is a curiosity; twenty machines agreeing is a fact.

An adversary who wants to poison the model has to run many independent machines producing plausible
traffic over time, and the payoff is making a local cost estimate slightly wrong. **The attack is
expensive and the prize is small, which is the right shape.**

### 4. A learned rule is a dated file and a pull request, never a silent update

The rules are already versioned by date, `anthropic-2026-09-01`. Aggregation produces a **candidate**
`anthropic-YYYY-MM-DD`, as a pull request, with the evidence table attached and a diff a person can
read. **Nothing auto-updates on a user's machine**, and `replay learn` keeps refusing to score
against a model whose calibration looks unreliable.

This is the same discipline as the model gate: a checkpoint that scores worse than the incumbent is
refused rather than shipped. A rules file learned from the corpus is a checkpoint.

## What "a better formula" concretely means

Three things get better with more machines, and they are not the same problem.

**The minimum cacheable prefix is a bounds problem, not a fitting problem.** Each session gives an
upper bound (the smallest cached prefix) and a lower bound (the largest uncached prompt). Intersect
those intervals across contributors and the interval shrinks monotonically. **This needs breadth, not
depth: many machines each contributing a little is strictly better than one machine contributing a
lot**, which is exactly what the current corpus lacks.

**The tokens-per-byte fit is a regression with an obvious confound.** It varies 0.445 to 0.795 here,
and the likely cause is content: code, prose and tool output tokenise differently. With enough
sessions the fit can be conditioned on observable, non-content features already in the ledger, block
kind and size distribution, rather than being one global constant with ±159% error bars.

**Break causes are a classifier that is currently starved.** All four breaks in the corpus landed in
one bucket, "prefix diverged inside the message history at an unknown block". **Unknown is the
interesting part.** More sessions across more clients turn that bucket into named causes, and a named
cause is actionable where an unknown one is not.

## What we are not doing

**No model training on anyone's code or prompts.** Not anonymised, not tokenised, not hashed. The
corpus contains no content and gains nothing from it, because the thing being modelled is a caching
rule, not a language.

**No per-user identity, no persistent installation id, no cohort tracking.** There is no product
question here that needs to know a machine came back.

**No aggregation before the twenty-session gate is met honestly.** Building a submission pipeline is
easier than getting twenty real sessions, and shipping the pipeline first would let us report a large
number that means nothing. The gate is about independence, not volume.

## Consequences

**Good.** The rules stop being one person's guess. The prefix-bound interval closes. Break causes get
names. A provider behaviour change shows up across many machines at once instead of being mistaken
for local drift, which is what the per-model staleness split already prepares for.

**Costs.** Someone has to run and defend an endpoint that receives untrusted input. The k-anonymity
threshold means the first models take a long time to publish. And **the honest failure mode is that
nobody submits**, in which case the corpus stays small and the rules file keeps saying so, which is
worse for the tool but not dishonest.

**Open.** The value of k. Whether contribution belongs in the CLI at all, or should stay a pull
request against `docs/evidence/` where the review is a human reading a diff, which is slower and
harder to game.

---

[Decision records](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
