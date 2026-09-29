# Anthropic efficiency: the contract

Invariants that bind any Replay claim about Claude workload efficiency. Written
2026-09-29, **before** any Anthropic optimization work, so that the rules exist
before the results they will have to survive.

Most of these were not reasoned out in advance. They are what a closed DeepSeek
campaign cost to learn, restated so the next campaign does not pay again. Each
one names the mistake it prevents.

## A1. No quota claim without measuring the quota surface

Replay must not claim a change improves quota efficiency unless it measured the
entitlement surface that change is supposed to affect.

*Prevents:* asserting a saving on an axis nobody instrumented.

## A2. Usage is not context

A reduction in context-window occupancy must not be described as a quota saving.
They are different quantities and the relationship between them is **UNKNOWN**
until measured on the specific product surface.

*Prevents:* the most attractive available confusion, since context is easy to
observe and quota is not.

## A3. Cost is not quota

API token cost and subscription quota consumption stay separate metrics and are
never summed, averaged, or substituted for one another.

*Earned:* the repo already states that on a subscription seat the dollar figures
are list prices for somebody else, and that whether a re-billed token draws down
a rate-limit window was measured as a null result. That null is exactly why
these two must not be collapsed.

## A4. Product surfaces differ until shown otherwise

A policy validated on Claude Code must not be applied to Claude.ai, Cowork,
Team, Enterprise, Bedrock, Vertex or Foundry without its own evidence.

*Earned:* a DeepSeek result measured on one endpoint did not hold on another in
the same account, and the mechanism was a request-body difference nobody had
thought to vary.

## A5. An optimization names its model regime

Every claim states the model family and version it was measured on. A result on
one model is a result on that model.

## A6. An optimization names its reasoning configuration

Every claim states the effort or thinking configuration in force.

*Earned:* a DeepSeek reasoning parameter turned out to participate in cache
identity, so a run that left it unstated could not be compared with one that
set it.

## A7. Tool usage is part of the workload

Where tools materially drive the work, a claim accounts for tool calls and
tool-result tokens, not only model tokens.

## A8. Compaction is an operation, not a free reset

Compaction is treated as having measurable context and usage consequences. Its
cost is measured, not assumed to be zero, and its benefit is measured, not
assumed to be positive.

## A9. Durable state is judged on continuation, not existence

If Replay claims durable work-state improves continuation, it tests continuation
behaviour. The presence of a state file is not evidence that anything was
retrieved or used.

*Earned:* `replay-work-state-relay-2026-09-27.md` already records the reading
that had to be retracted, that agents "saw the contradiction and missed it",
when the transcripts showed the fact never entered context at all.

## A10. A prompt change needs replication before it becomes policy

One winning experiment is a lead. Policy requires an independent replication
against a criterion fixed before the result is seen.

*Earned:* two DeepSeek prompt effects each looked strong and each reversed sign
on replication. Both would have shipped.

## A11. Provider internals stay hypotheses

Undocumented quota, cache and compaction mechanics are **UNKNOWN**. A mechanism
that explains an observation is a candidate mechanism, not a finding, until an
experiment discriminates it from its alternatives.

*Earned:* a published vendor figure for cache block size did not predict the
measured behaviour of a current model, and the campaign could not establish
whether the difference was generational or a stale document.

## A12. The metric is verified work, not fewer tokens

```text
              VERIFIED USEFUL WORK
    ----------------------------------------
      QUOTA  or  COST  or  TIME, per regime
```

The denominator depends on the entitlement regime and the three are not
interchangeable. Token minimisation is not the objective and a cheap wrong
answer is not an optimization.

*Earned:* a DeepSeek reasoning setting cut output tokens by a factor of 39 and
total cost by 1.48, while dropping accuracy on one task class from 24/24 to
7/24, with the wrong answers arriving as confident numbers.

## A13. Absent is not zero

A telemetry field that did not arrive is NOT_OBSERVED. It is never read as zero,
never as false, and never as evidence that the underlying thing did not happen.

*Earned:* enough times to be its own rule.

## A14. Fail toward the expensive, correct setting

Where a policy cannot establish that a cheaper configuration is valid for the
current task, model and evidence regime, it selects the conservative one. A
policy may under-optimise safely; it may not silently produce wrong work.

## A15. Provider guidance is a hypothesis

Anthropic's own guidance about which model suits which work, or which commands
manage usage, enters this repository as a hypothesis with a source and a date.
It is not adopted as Replay policy without measurement.

## What this contract does not do

It does not forbid acting on provider guidance. It forbids **claiming** that
Replay measured something it did not. Following a documented recommendation and
saying so is fine; presenting it as a Replay finding is not.
