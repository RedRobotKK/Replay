# Research architecture after E005

What a skeptical panel would need before believing the E005 phenomenon is
general. **No experiment run. E005 evidence not reopened.**

The governing constraint is discovered rather than chosen, and it decides most
of what follows.

> **CORRECTED 2026-09-26** by `docs/evidence/cross-surface-observability-2026-09-26.md`.
> Two claims below were wrong, both understating what is reachable. The Codex row
> said the write field was absent; it is present on 11,776 records, uniformly
> zero, and demonstrably a client logging gap. And a second priced cache-write
> contract does exist, on OpenAI GPT-5.6+, reachable at the API seam through
> `replay serve` rather than from transcripts. The corrected text is marked
> inline. Items 5, 10 and 12 are affected. **E005 itself is unchanged.**

## The constraint: the instrument does not exist elsewhere

E005 compared two observables. Their availability, measured on this machine:

| surface | economic observable (Δ cache-creation) | structural oracle (`cache_miss_reason`) |
|---|---|---|
| Claude Code | **present, priced, separate** | **present** |
| Codex | field present, **zero on all 11,776 records** (client logging gap) | absent |
| Grok | field present, **zero on every record** | absent |
| Ollama | no bill, no cache economics | absent |
| Cursor | 118 transcripts, **no usage fields** | absent |
| AnythingLLM / OpenClaw / Oracle | no cache-write field or zero | absent |

Two consequences the panel cannot design around:

**1. No other provider publishes a structural oracle.** `cache_miss_reason` is
an Anthropic field. **E005's design is unrepeatable on any other provider**, not
for want of effort but because the ground truth does not exist. "Replicate E005
on OpenAI" is not an experiment that can be specified.

**2. The economic observable is unmeasurable on every other local surface.**
Codex reports reads with no writes, so Δwrite cannot be computed at all. Grok's
write field is uniformly zero. Cursor has no usage.

**Claude Code is the only surface where both halves of E005 can be measured, and
the only surface where either can be measured from durable artifacts on this
machine.** ~~the only surface where either half can be measured~~ **CORRECTED:** a
second priced cache-write contract exists on OpenAI GPT-5.6+, which reports
`input_tokens_details.cache_write_tokens` and bills writes at 1.25x. It is
unreachable from Codex transcripts, which zero the field, and reachable at the
API seam through `replay serve`.

## The fifteen questions

**1. Strongest conclusion E005 legitimately supports.** Within one provider,
one client, one corpus: structural divergence and economic disruption are
separable observables, and the provider's structural class predicts materiality
poorly for three of four classes. Nothing about AI execution in general.

**2. Stronger conclusions plausibly reachable.** That materiality is
class-dependent across cache implementations; that a client-side instrument can
recover materiality without provider cooperation; that `cache_missed_input_tokens`
is non-diagnostic of cost generally.

**3. Evidence missing for each.** For the first, a second cache implementation
with measurable Δwrite. For the second, a surface where the provider publishes
no diagnostic and the instrument still works ,  **which is every surface except
Anthropic, and on none of them is Δwrite computable.** For the third, an
independent billing record to check against.

**4. Alternative explanations to eliminate.** That `tools_changed` is material
because tool definitions sit at the prefix head, making this a fact about
Anthropic's breakpoint placement rather than about execution. That the corpus is
one operator's workload and class frequencies reflect this machine's habits, not
agent execution. That Replay's detector and the diagnostic correlate through a
shared dependence on request size. **None is eliminated by E005.**

**5. Genuinely necessary surfaces.** One: a second prefix-cache implementation
that reports cache writes separately. **No locally stored transcript surface
qualifies.** **CORRECTED:** OpenAI GPT-5.6+ does qualify on the economic arm, but
only when captured at the API seam. The blocker is spend and a key, which is
task #30, not the absence of an instrument.

**6. Redundant surfaces.** Cursor (no usage), Ollama (no cache economics),
AnythingLLM, OpenClaw, Oracle. **Codex transcripts are redundant for this
question** because the write field, though present, is never populated, and
September's builds dropped usage logging from 26/26 sessions to 20/235. It
remains a spend surface. **JEV is not relevant**: it carries attempts, answers and tokens, with
no prefix-cache contract to invalidate. Adding these surfaces would grow the
sample and answer nothing.

**7. Does the universe need the listed surfaces?** No. **The list conflates
"agent surfaces that exist" with "surfaces carrying the observable."** Seven of
eight fail the second test.

**8. Correct unit of analysis.** **REFINED** by
[Identification conditions for E005](THEORY-IDENTIFICATION-2026-09-26.md): the
unit is the **(surface, measurement boundary) pair**, since Codex fails at its
transcript and passes at the API seam with no change to provider or traffic.
Within a boundary the unit is the **prefix-cache contract**: the triple of caching scheme, breakpoint
placement and client rendering behaviour. Two models behind one provider share
a contract; one model reached through two clients may not. **E005 is n=1 in
contracts, not n=2,064 in events.**

**9. Should there be 100 experiments?** No, and the number is unsupportable from
either direction. The program is bounded above by contracts carrying the
observable, currently **one**. A hundred experiments on one contract measures
one contract a hundred times. If three more contracts became measurable, a
defensible program is roughly **6 to 12 experiments**, not 100.

**10. Minimum program to distinguish provider artifact from general property.**
**CORRECTED:** it can be run, at a cost. Two contracts with separately reported
cache writes exist, Anthropic and OpenAI GPT-5.6+. The second is reachable only
through `replay serve`, carries no provider cause classes, and so supports the
economic arm alone. **The question is blocked on spend and a key, not on
instrumentation**, and the resulting design is weaker than E005 and must not be
reported as an E005 replication.

**11. Most falsifying experiments.** Two, both currently blocked. (a) A second
contract where materiality is class-independent would falsify the general claim
immediately. (b) A workload from a different operator on the same contract: if
class frequencies shift substantially, E005 measured a habit rather than a
property.

**12. Independent vs sequential.** Two are independent. (b) above needs only a
second operator's Claude Code corpus and **no new instrument**, which makes it
the cheapest real test available and still the first choice. **CORRECTED:** (a) is
no longer blocked either, but needs OpenAI spend and delivers the economic arm
only.

**13. Replication and external validation.** Replication means a second operator
and a second corpus on the same contract. External validation means an
independent party running the blind against their own transcripts and reporting
the class table. Neither has occurred. **E005 is one operator, one machine,
self-measured.**

**14. Measurements that would strengthen the economic claim.** A provider
invoice reconciled against Replay's re-billed figure for one period, which is
the only way to close the gap between "cache-creation tokens moved" and "money
moved." **This is already recorded as blocked in task #30.**

**15. Theoretical results obtainable without another experiment.** One is
available and worth stating: **the class table itself implies an upper bound on
what any prefix-stability intervention can save.** If three of four divergence
classes carry zero median Δwrite, then interventions targeting them have no
economic headroom, whatever their frequency. That is derivable from frozen E005
numbers and needs no new data.

## Where the panel disagrees

**Systems and observability** read the constraint as fatal: one contract, one
operator, self-measured, no independent oracle anywhere else. Their position is
that E005 is a well-executed measurement of a single vendor's caching behaviour
and should be published as such, with no generality claim.

**Methodology** notes the cheap test in (12) is available now and unblocks the
generality question partially: a second operator's corpus costs nothing but
access and would separate "property" from "habit."

**Statistics** flags that 2,064 events from 14 sessions is **not 2,064
independent observations**; clustering within session and within operator makes
the effective n far smaller than the headline, and the confidence one should
attach to the class table is correspondingly lower.

**Causal inference** notes that nothing in E005 is interventional. Every number
is observational, and the class differences could reflect what this operator
changes rather than what changing it costs.

## What the panel concludes

**The honest state is that E005 measured something real about one cache
contract, and that no second contract publishes the structural oracle E005 was
built around.**

**CORRECTED:** the original text said the instruments do not exist outside that
contract. On the economic arm they do. OpenAI GPT-5.6+ prices cache writes at
1.25x and reports them, and `replay serve` already captures at the seam where
they are visible. What does not exist anywhere else is the structural oracle, and
that is a fact about what providers publish.

The next step is still not an experiment. In order of cost it is: a second
operator's Claude Code corpus, which tests habit against property for the price
of access; or a `replay serve` capture against OpenAI GPT-5.6+, which needs spend
and delivers the economic arm alone; or accepting that Replay is an instrument
for one contract and saying so plainly.

**Neither path requires the 100-experiment program, and the panel finds no
evidence that would justify one.**
