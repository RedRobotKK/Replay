# ADR-0009: Crowdsource the waste taxonomy, not the cache model

- **Status:** Proposed
- **Date:** 2026-09-04
- **Amends:** ADR-0007 (federated calibration corpus), ADR-0008 (corpus at launch)
- **Related:** [`PRODUCT-DIRECTION.md`](../PRODUCT-DIRECTION.md)

## Context

ADR-0007 proposed crowdsourcing **cache model calibration**: minimum prefix bounds, tokens-per-byte,
break causes. That is worth doing and it is the smaller half. It improves accuracy in a tool that is
already accurate to 99% on the sessions it has.

Two facts sit next to each other and had not been connected.

**First: every operational guard ships turned off, because nobody knows what number to put in it.**
`--error-budget`, `--loop-warn`, `--loop-block`, `--breaker-failures`, `--max-session-*` all default
to `0`, meaning disabled. The only threshold that appears anywhere is `e.g. 0.3` in the flag help,
and `errorBudgetMinPromptTokens = 10_000` is likewise a chosen round number. **Not one guard
threshold in this tool is derived from evidence.** They are all defensible guesses, and a guess is
why they are off by default.

**Second: Replay already computes a waste taxonomy** — rework, forgetting, mechanical,
over-provisioning, fan-out — and reports it as eight unrelated diagnostics.

## Decision

**The corpus should carry the waste distribution, and the guards should be calibrated from it.**

### 1. Waste metrics are a better thing to crowdsource than cache rules

They are **content-free by construction**. `error_share` is a ratio. `re_reads` is a count.
`cache_breaks` is an integer. There is no path from these numbers back to anyone's code, prompts or
project. Cache calibration needs prefix sizes and token fits, which are closer to the shape of the
work; waste metrics are further from it.

They are also **useful to the contributor immediately**, which ADR-0008 could not honestly claim.
"Your engine reproduces 99% of turns" is a fact about the tool. **"Your error share is 22%, the
median is 4%, and you are in the worst decile" is a fact about your afternoon**, and it is worth
running the command for on its own.

### 2. This turns operational error management from reactive to predictive

Today a guard refuses the next request **after** a threshold you invented is crossed. That is a
circuit breaker: useful, dumb, and off by default.

With a distribution across many machines, the same signals answer a different question. **Not "have
you crossed 0.3" but "what usually happens next to sessions shaped like this one."**

- **Calibrated defaults.** A guard can ship *on*, with a threshold at a real percentile rather than a
  round number somebody liked.
- **Early warning before the money is spent.** Error share climbing at turn 12 in the pattern that
  usually precedes abandonment is worth saying out loud at turn 12, not refusing at turn 40.
- **Failure signatures.** The characteristic shape of a session that ends in rework, told apart from
  one that is merely long. That needs many sessions and cannot be derived from one machine.
- **Attribution.** Which client versions, which tool sets, which session shapes waste most. A user
  cannot see this; a corpus can.

### 3. This is the moat, and the cache model is not

**Anyone can re-derive a provider's cache rules.** They are published, and a competitor with traffic
can measure them in a week.

**A distribution of how real agent sessions actually fail cannot be re-derived from anything public.**
It exists only where someone has been reconstructing sessions turn by turn, across many machines,
with the provider's own usage attached. That is a position, not a feature.

## What changes in ADR-0007 and ADR-0008

Everything about consent stays: a command a person types, the payload shown as bytes, the aggregate
published back, k-anonymity, robust statistics, no learned rule without a pull request.

**What changes is the payload and the reciprocity.** `replay corpus` should carry the waste
distribution alongside the calibration table, and `--show-aggregate` should return percentiles a
contributor can place themselves in. That is the reason to submit, and ADR-0008 did not have one.

## The risk this introduces, which is new

**`unused-tools` names tools.** A waste report saying "9 of 23 tool definitions were never called"
is harmless. One that names them **discloses the contributor's tool inventory**, and that is exactly
the leak already found in `internal/transcript/testdata/session-redacted.jsonl`, where paths and
bodies were hashed and tool names were not.

**So the corpus carries counts and never names**, and the same fix belongs in `replay redact`. This
is the one place where the waste taxonomy is less safe than the cache table, and it is the same bug
twice, which is how it should be read.

## Sequencing

**None of this ships before the breakdown does.** `PRODUCT-DIRECTION.md` argues for showing a user
their own waste first, on eleven sessions, because that is defensible. Prediction needs far more data
than description: a percentile over twenty machines is a curiosity, and a claim about what "usually
happens next" needs enough sessions that survivorship bias is visible rather than invisible.

**People who submit a corpus are people whose sessions worked well enough to finish.** That skew is
inherent, it is not fixable by more data, and any predictive claim has to be stated in front of it
rather than behind it.

---

[Decision records](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
