# What Replay is measuring

Theory layer following the frozen E005 result. **No experiment was run for this
document.** It defines the observable model, isolates what is potentially new,
and specifies the next decisive test without running it.

## 1. The observable model

The intuitive chain is:

```text
execution state -> structural transition -> economic consequence -> Replay observation
```

**E005 shows this is not a chain.** The middle arrow is conditional, and that is
the whole result.

### Arrow 1, execution state to structural transition

**NOT OBSERVED.** E005 begins at the provider diagnostic and never sees what in
the execution produced the divergence. Nothing here licenses a claim about what
causes a tool set or system prompt to change.

### Arrow 2, structural transition to economic consequence

**MEASURED, AND FOUND CONDITIONAL.** Median change in cache-creation tokens:

| provider class | n | median Δwrite |
|---|---:|---:|
| tools_changed | 1,006 | **+5,005** |
| messages_changed | 591 | 0 |
| system_changed | 119 | 0 |
| model_changed | 45 | 0 |

**A structural transition does not imply an economic one.** Three of four
provider-reported divergence classes show no median cache-write movement. The
arrow holds for one class and fails for three.

This also breaks a tempting shortcut: `model_changed` carries the corpus's
largest median `cache_missed_input_tokens` (243,095) with the smallest observed
economic footprint. **The provider's missed-token field is not a materiality
signal.**

### Arrow 3, economic consequence to Replay observation

**MEASURED.** Replay recovers `tools_changed` blind at 58% to 93% (bounded
against 375 same-second collisions). The lower bound exceeds every other class's
upper bound, so the ordering is robust; the exact rate is not.

### What the model actually is

```text
structural transition ──(conditional, class-dependent)──> economic consequence
                                                                │
                                                                ▼
                                                      Replay observation
                                                      (58-93% on the one
                                                       class that is material)
```

Replay is positioned on the **economic** arm. The provider diagnostic is
positioned on the **structural** arm. E005 is the measurement that separates
them.

## 2. The novelty question

### What this is not

It is **not** "cache invalidation detection." The provider already detects and
reports invalidation, more completely than Replay does, and publishes the cause
class. Claiming that would be claiming a worse version of an existing feature.

It is **not** "economic-state observability for AI executions" as a general
capability. That phrase is broader than anything measured and would need to
survive comparison with tracing, provider dashboards and cost tooling. E005
tested none of that.

### The narrowest defensible statement

> **Provider diagnostics report which structural transition occurred. Provider
> billing reports what was spent in total. Neither reports which transitions
> were economically material. E005 shows those are different questions, because
> three of four reported divergence classes cost nothing measurable.**

The candidate position is the join: **per-transition materiality**, computed
from durable artifacts, for an execution that has already happened.

### What would distinguish it from adjacent tools

| tool | reports | does not report |
|---|---|---|
| provider billing | totals per period | which change caused what |
| provider diagnostic | structural cause class | whether it cost anything |
| tracing (OTel spans) | latency, call graph | cache economics |
| Replay | per-turn re-billed tokens with evidence | why the change was made |

**This is a measurement position, not a mechanism.** No part of it is claimed as
novel technology, and this document makes no IP claim.

## 3. The next decisive test, specified not run

E005 was **retrospective**: given a completed execution, recover what was
material. The open question is **prospective**.

> **Hypothesis H:** observable execution state at turn N predicts whether turn
> N+1 carries an economically consequential transition, without access to any
> post-hoc provider diagnostic.

**Observable inputs at turn N**, all available before N+1 exists: hash of the
tool-definition set; hash of the system prefix; elapsed time since the previous
request; lane/sub-agent structure; whether the client rewrote history.

**Independent outcome at N+1**, from provider usage only: cache-creation tokens,
compared against the prior turn.

**Prediction target:** binary, does N+1 show a cache rebuild.

**Control:** a base-rate predictor that knows only the class frequencies. In
this corpus `tools_changed` is 49% of structural events, so **a naive predictor
is already strong** and the bar is beating it, not beating chance.

**Falsifier:** the state-based predictor performs no better than base rate.
Then observable execution state carries no predictive information and Replay is
a retrospective instrument only. That is a clean, acceptable result.

**What would count as evidence:** a held-out improvement over base rate, on
sessions not used to build the predictor, with the diagnostic withheld
throughout.

**Deliberately excluded:** cost savings, agent behaviour, enforcement,
multi-model replication. **One question, one outcome.**

## 4. Product implication, conditional

If H is eventually validated, the concrete thing a developer gets that a billing
dashboard and a tracing UI do not:

> **Which of your changes cost money, and which were free.**

A dashboard says the month cost $X. A trace says turn 47 took 8 seconds. The
provider diagnostic says turn 47 was `tools_changed`. **None of them says that
turn 47's tool-set change rebuilt 5,005 tokens of prefix while turn 52's system
prompt edit cost nothing.**

E005's practical implication is that **the actionable set is small**. If roughly
three quarters of structural changes are economically inert, a developer
optimising prompt stability is mostly optimising things that do not matter, and
the value is in identifying the minority that do.

**Market need for this remains UNKNOWN**, unchanged by E005. Nothing here is
evidence of demand or willingness to pay.

## Epistemic status

### ESTABLISHED BY E005

- The corpus accounting: 2,064 events, 1,761 structural, 303 indeterminate.
- Median Δwrite +5,005 for `tools_changed`, 0 for the other three classes.
- Replay recovers `tools_changed` blind at 58-93%.
- `cache_missed_input_tokens` is not a materiality or billing proxy.
- Structural divergence and economic disruption are separable observables.

### SUPPORTED HYPOTHESIS

- Replay occupies an independent measurement position on the economic arm.
- Per-transition materiality is a question neither billing nor diagnostics answer.
- TTL-class events are a provider-side condition the diagnostic does not report.

### NOT YET TESTED

- **Prediction.** Every E005 measurement is retrospective.
- Any causal claim about what produces a structural transition.
- Generality beyond this corpus, this client, this provider.
- That materiality information changes what a developer does.
- Comparison against tracing or cost tooling on the same task.
- **Demand, willingness to pay, commercial value, or moat.** None demonstrated.

## Standing

E005 is frozen. This document is theory built on it and introduces no new
measurement. The conditions under which the observable model above can be
measured at all are worked out in
[Identification conditions for E005](THEORY-IDENTIFICATION-2026-09-26.md). **The next step is the prospective test in section 3, and it has
not been run.**
