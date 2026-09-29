# Anthropic efficiency: test matrix

What would have to be true for each hypothesis to be testable, written before
any experiment runs. A row with no observable is not an experiment, it is a
preference.

Companion to `ANTHROPIC-EFFICIENCY-CONTRACT.md` and
`ANTHROPIC-EFFICIENCY-SURFACE.md`.

## How to read the control-point column

| class | meaning |
| --- | --- |
| **OBSERVABLE ONLY** | Replay can see it happened, and cannot quantify or change it |
| **OBSERVABLE + MEASURABLE** | can be quantified from evidence Replay already holds |
| **MEASURABLE + CONTROLLABLE** | the user or repository can vary it deliberately |
| **SAFE TO AUTOMATE** | reserved: nothing qualifies until a tested contract exists |
| **REQUIRES USER POLICY** | a human decision, Replay can only inform it |
| **REQUIRES ADMIN POLICY** | an organization setting |
| **PROVIDER CONTROLLED** | not ours to change |
| **UNKNOWN** | not established |

**Nothing is classified SAFE TO AUTOMATE in this document.** That class exists
so a later change has to argue its way in.

## The matrix

| # | Surface | Hypothesis under test | Observable | Control | Test shape | Control point |
| --- | --- | --- | --- | --- | --- | --- |
| T1 | model | model choice changes task-level economics, not only per-token price | model id in transcript; per-lane usage | `/model` | paired, same task set, both directions | MEASURABLE + CONTROLLABLE |
| T2 | effort | effort changes the ratio of verified work to resource | usage and outcome | effort setting | paired by task class | UNKNOWN until the surface map says whether effort is observable |
| T3 | context | conversation length increases usage per unit of work | context size, per-turn usage | `/clear`, `/compact` | paired, same task from a long and a short context | OBSERVABLE + MEASURABLE |
| T4 | compaction | compaction preserves the work needed to continue | continuation behaviour after the event | `/compact` | recovery test: same next task, compacted and not | OBSERVABLE + MEASURABLE |
| T5 | compaction cost | compaction is not free | usage attributable to the summarisation turn | `/compact` | before/after accounting on one session | UNKNOWN: depends on whether the turn is attributable |
| T6 | prompt | prompt structure changes behaviour at constant semantics | trajectory, usage, outcome | prompt text | paired, replicated, criterion fixed first | MEASURABLE + CONTROLLABLE |
| T7 | tools | dependency-aware parallelism reduces wall-clock without raising cost | timestamps, tool-call count, usage | execution shape | concurrency test | OBSERVABLE + MEASURABLE |
| T8 | tool output | bounding tool output preserves task outcome | tool-result tokens, outcome | output caps | paired, per task class | MEASURABLE + CONTROLLABLE |
| T9 | handoff | structured work-state changes what a fresh agent retrieves | retrieval events in the trajectory | handoff format | blinded, four arms, see T13 | MEASURABLE + CONTROLLABLE |
| T10 | cache | a stable prefix improves reuse on this provider | cache-read telemetry | prefix construction | paired | OBSERVABLE + MEASURABLE |
| T11 | project | project material changes efficiency | usage, context | project contents | paired | UNKNOWN: depends on whether project content is in-context |
| T12 | subagent | delegation improves net verified work per unit resource | per-lane usage, parent plus children | delegation choice | paired, whole-task accounting | OBSERVABLE + MEASURABLE |
| T13 | quota | workflow shape changes consumption against a reset window | entitlement state | workflow | controlled, one regime at a time | UNKNOWN: depends on whether entitlement state is readable |

## The negative controls, which matter more than the positives

| # | Negative control | What it rules out |
| --- | --- | --- |
| N1 | same condition twice, no change | establishes the noise floor before any effect is claimed |
| N2 | a verbose variant of the winning structure | separates a structure effect from a length effect |
| N3 | an inert configuration change | rules out that any change at all moves the metric |
| N4 | state file present but never read | separates presence of durable state from its use |
| N5 | a wrong evidence anchor | separates "an anchor changes retrieval" from "a correct anchor does" |

N1 is not optional. A DeepSeek campaign spent its budget discovering that the
control drifted 13% against itself, which was larger than the effect being
chased, and only a preregistered replication caught it.

## Required test kinds, per the repository's own methodology

For every control that reaches implementation:

| kind | what it establishes |
| --- | --- |
| positive | the control does the thing |
| mutation | the test can fail; break the production code and watch it go red |
| fail-safe | the conservative path is taken when evidence is missing |
| missing telemetry | an absent field is NOT_OBSERVED, never zero |
| malformed configuration | fails closed with a diagnostic, never into a cheaper path |
| model/version mismatch | a policy measured on one model does not silently apply to another |
| entitlement mismatch | a policy measured on one regime does not silently apply to another |

The last two have no analogue in the DeepSeek work and are the additions this
surface needs, because Anthropic's economics differ by product and by
entitlement in a way a single provider endpoint did not.

## What blocks nearly everything

Four rows above are UNKNOWN for the same reason: **it is not yet established
what entitlement state, effort configuration, compaction accounting and project
context are visible to an external tool.** The surface map is what answers that.
Until it does, T2, T5, T11 and T13 cannot be designed, only described.
