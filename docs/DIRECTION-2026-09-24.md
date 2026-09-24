# Product direction, 2026-09-24

**Written after a research closeout that falsified the previous differentiator. Grounded in an
inspection of the shipping product, not in the PRD that prompted it.**

Nothing here is implemented. Every proposed capability carries one of:
**BUILD NOW**, **BUILD NEXT**, **VALIDATE FIRST**, **FUTURE**, **DO NOT BUILD**.

---

## 1. Current-state assessment

Thirty subcommands ship. The substrate for this direction is further along than the PRD assumes, and
in three places it is already built.

| Capability the direction needs | What ships today | Gap |
| --- | --- | --- |
| **Compaction measurement** | `analysis.Compactions`, `CompactedTokens`. Distinguishes "compacted, size unknown" from "compacted, dropped nothing" | No per-session narrative, no before/after |
| **Optimize and measure loop** | `internal/learn`: `Catalog`, `Score`, `Select`, `Verdict` with `Sessions`, `HoldoutSessions`, `Mean`, `Interval`, `HoldoutMean`, `CachedShareDelta`, `Estimated` | **A train/holdout-validated intervention loop already exists.** It is not surfaced as a product workflow |
| **Weekly digest** | `replay since`: "what happened while you were away" | Not weekly, not behavioural, no change detection |
| **Longitudinal diff** | `replay budget --json`, documented as "a later run can tell you what grew" | Manual. No stored history |
| **Recommendations with decisions** | `advisor`: 6 Kinds, `Status`, `Decision`, schema 2, reader decisions persisted apart from computed status | 83.7% of advice is outside the verifier by construction (task #34) |
| **CI gate** | Exit-code contract: `exitGateBreached` vs `exitCannotEvaluate`, frozen, with `blocksAMerge` | No GitHub Action, no check output |
| **Evidence discipline** | 40+ dated files in `docs/evidence/`, retractions appended not deleted | Not a runtime surface |

**Not present, and must not be assumed:** any agent identity model across sessions, any stored
longitudinal history, any team or multi-user concept, any policy engine for gates, any aggregation
across users.

---

## 2. New product thesis

> Replay turns AI activity into an evidence-backed, measurable record that people can explore,
> evaluate, improve, and eventually supervise.

**This is a product thesis, not a moat.** The research closeout established that reconstruction is not
a differentiator and that the theoretical machinery is not novel. What survived is narrower: the
failure is at write time, and a system that records what an assertion rests on can enumerate its own
contradictions rather than leaving them to be discovered weeks later.

**North star:** Replay is where you understand what your AI is doing, what changed, what can actually
be established, what could be improved, whether the improvement worked, and eventually what your AI
is allowed to do.

---

## 3. Personas

One evidence model, several lenses. **Do not build a product per persona.**

| Persona | Question | Served by |
| --- | --- | --- |
| Developer | What changed in my agents? | `context`, `blame`, `diff` |
| Engineering lead | Better or worse over time? | Nothing yet. Needs history |
| Security | What are agents touching? | `agents`, `privacy`, `redact` |
| Product | Behaving as intended? | Nothing yet |
| Finance | Where is consumption increasing? | `cost`, `burn`, `budget` |
| Compliance | What can we establish? | The epistemic contract, §12 |
| Executive | What is happening across the estate? | **FUTURE.** Needs everything above |

---

## 4. Jobs to be done

1. **Keep my agent working longer before compaction.** The free-tier job.
2. **Tell me what my agent actually did, with evidence.**
3. **Tell me what changed since last week.**
4. **Tell me what to try, and then whether it worked.**
5. **Stop a change that breaks behaviour we rely on.**
6. **Let me show someone else what we established.**

Jobs 1, 2 and 4 have substrate today. Job 3 has a primitive. Jobs 5 and 6 do not.

---

## 5. Product principles

1. **Never collapse an epistemic category.** §12 is a trust contract, not labelling.
2. **A metric that cannot be computed is not displayed.** No placeholder figures to populate a screen.
3. **Refuse rather than estimate silently.** The exit-code contract already draws this line.
4. **A recommendation is not a finding.** Observation, hypothesis, recommendation and validation stay
   distinct.
5. **Read-only against every source, permanently.**
6. **Say what a change does not fix.**

---

## 6. Core product loop

```text
capture  ->  explain  ->  detect change  ->  inspect evidence
                                                   |
                          measure  <-  decide  <----+
                             |
                             +--> surface again next week
```

Every step except "detect change" and "surface again" has shipping substrate.

---

## 7. Free tier experience

The mental model is **"Replay helps my agent work longer before compaction"**, not "you are using an
observability platform".

**What can honestly be said today.** `analysis` counts compactions and the tokens they removed, and
already distinguishes a compaction whose size is unknown from one that dropped nothing. So Replay can
state: *this session compacted N times, removing M tokens where the client reported it*.

**What cannot be said.** "Compaction avoided", "session extended by X%", or any saving. There is no
counterfactual session to compare against. **Any such claim is NOT MEASURED and must be labelled so.**

| Capability | Class |
| --- | --- |
| Report compactions and compacted tokens per session, with the unknown-size case distinct | **BUILD NOW** |
| Name the largest context consumers before the next compaction, from existing `context` analysis | **BUILD NOW** |
| "You would have avoided a compaction" | **DO NOT BUILD.** Not measurable |
| Percentage session-extension claims | **DO NOT BUILD** unless a paired measurement exists |

---

## 8. Weekly AI review

`replay since` is the primitive and it is the wrong shape: it reports what happened, not what
**changed**.

Change detection requires stored history, which does not exist. `replay budget --json` documents the
manual version.

| Capability | Class |
| --- | --- |
| Store a dated weekly snapshot of computable figures | **BUILD NEXT** |
| Report week-over-week deltas on figures that already exist: sessions, compactions, cache traffic dollars, advice counts, coverage | **BUILD NEXT** |
| "3 agents changed materially", "17 new behavioural patterns", "2 policy exceptions", "98.7% evidence completeness" | **DO NOT BUILD YET.** None is computable. No agent identity model, no pattern model, no policy engine, no completeness metric |
| Evidence completeness as a metric | **VALIDATE FIRST.** Define it before displaying it |

**The AI Week mock in the PRD is roughly 20% computable today.** Building the screen before the
metrics exist is the failure mode this repository has catalogued repeatedly.

---

## 9. Explore and analytics

The Tableau analogy describes position, not architecture.

| Capability | Class |
| --- | --- |
| Existing per-session exploration via `context`, `blame`, `diff`, `burn` | ships |
| A canonical behavioural model across sessions and agents | **VALIDATE FIRST.** Requires an agent identity contract that does not exist, and identity was the hardest unsolved thing in the closed research |
| Free-form query | **DO NOT BUILD.** Becomes an observability platform |
| Ten declared questions of the "which agents changed this week" shape | **FUTURE**, and each needs its own computability check |

---

## 10. Optimization workflow

**This is the strongest surviving asset and it is underused.** `internal/learn` already produces a
`Verdict` with a training mean, a confidence interval, a **holdout mean on sessions selection never
saw**, and an `Estimated` flag.

That is Observe → Hypothesise → Experiment → Measure, with a holdout, already built.

| Capability | Class |
| --- | --- |
| Surface `learn` verdicts as a first-class workflow, with the four categories kept distinct | **BUILD NOW** |
| Show the holdout mean beside the training mean, always, never the training figure alone | **BUILD NOW** |
| Record which intervention the reader chose, reusing the `Decision` mechanism from schema 2 | **BUILD NEXT** |
| A general LLM recommendation engine | **DO NOT BUILD** |

---

## 11. Measurement and validation

The loop exists for cache and context-edit candidates. Extending it is the highest-value work.

**The binding constraint is task #34: 83.7% of advice sits outside the verifier by construction**,
because hot-file and cache-breaks return `AdviceOnly` before any other logic. A validation workflow
that can only ever judge 16% of its own advice should say so on the screen.

| Capability | Class |
| --- | --- |
| Print the coverage figure wherever advice is shown | **BUILD NOW.** The count already exists |
| Extend holdout validation to a second candidate family | **BUILD NEXT** |
| Claim an intervention worked without a holdout | **DO NOT BUILD** |

---

## 12. GitHub and CI

The exit-code contract is already frozen and already distinguishes a measured breach from an
inability to evaluate. That is the hard half.

| Capability | Class |
| --- | --- |
| A documented recipe using the existing exit codes in an Actions step | **BUILD NEXT** |
| A published Action that emits a check with the evidence | **VALIDATE FIRST.** Nobody has asked |
| "Does this change preserve the behaviour we require?" | **FUTURE.** Needs a policy primitive that does not exist |

---

## 13. Security sidecar

**Replay beside the security stack, never instead of it.**

| Capability | Class |
| --- | --- |
| Document the sidecar position honestly, with no claimed integration | **BUILD NOW**, one page |
| An AIRS integration | **DO NOT BUILD.** No access, no evidence anyone wants it. Fabricating one is the failure this session already committed once |

---

## 14. Supervision

Observe → Establish → Supervise → Gate.

Replay is at **Observe**, with the beginnings of Establish. Everything past that needs a policy
primitive, an identity model and organisational history, none of which exist.

**FUTURE**, entirely. Nothing in this section is buildable now and nothing should be claimed.

---

## 15. Pricing

The five-tier table is a hypothesis with **zero customer evidence**, and the existing recorded price
is a repository week at $22,000, quoted $18k to $25k, none sold.

**VALIDATE FIRST**, all of it. The progression Free-Extend / Pro-Understand / Team-Understand-ours /
Business-Operationalise / Enterprise-Supervise is a sensible narrative and is not a plan.

The arithmetic from the closed research still applies: recovered engineer time at plausible incident
rates lands near $7,000 a year, which does not clear the recorded price. **Either the value is not
recovered time, or the price is wrong.** Unresolved.

---

## 16. Data and privacy

| Layer | Position |
| --- | --- |
| Customer-private: prompts, code, identities, transcripts | Stays customer-controlled. Never silently enters generalised learning |
| Aggregate intelligence | **DO NOT BUILD.** No consent model, no legal model, and the existing contribution path already carries real disclosure risk documented in `docs/evidence/contribution-payload-2026-09-10.md` |

---

## 17. Moat hypotheses

Stated as hypotheses. **None is established.**

| Hypothesis | Assessment |
| --- | --- |
| Evidence model as a common semantic layer | Plausible. Not differentiated; prior art everywhere |
| Longitudinal history | **The strongest candidate.** Requires storing history, which is not built |
| Institutional memory: decisions become reusable | Plausible. The `Decision` mechanism exists at session scope only |
| Optimization history: interventions and measured outcomes | **Second strongest, and partly built.** `learn` records verdicts with holdouts |
| Behavioural baselines | Requires history and identity |
| Supervisory policy | Requires all of the above |
| Aggregate intelligence | Do not build |

**Strongest candidate:** Replay becomes the longitudinal intelligence and institutional memory layer
for an organisation's AI. **Unproven, and it requires storing history before anything can test it.**

---

## 18. The epistemic contract

Seven states, never collapsed: **OBSERVED, MEASURED, CORRELATED, HYPOTHESIS, RECOMMENDATION,
VALIDATED, UNKNOWN/NOT MEASURED**.

The product already has pieces: `Estimated` on suggestions and verdicts, `NOT MEASURED` in the exit
codes, `Status` distinct from `Decision`, retractions appended.

**BUILD NOW:** one internal vocabulary so these stop being per-surface conventions.

---

## 19. MVP scope

> The smallest shipping product that makes one user say "I need to open Replay again next week".

Four things, all grounded in existing substrate:

1. **Compaction reality** on the free path: compactions, compacted tokens, the unknown-size case kept
   distinct, and the largest context consumers. **BUILD NOW.**
2. **Advice coverage stated on screen**: the share of advice the verifier can never reach.
   **BUILD NOW.**
3. **Optimization verdicts surfaced** with the holdout mean always beside the training mean.
   **BUILD NOW.**
4. **A dated weekly snapshot** of figures that already compute, so week two has something to compare
   against. **BUILD NEXT**, and it is the one thing that creates a reason to return.

Nothing else. No agent model, no policy engine, no GitHub Action, no team features, no dashboard.

---

## 20. Phase 2 and 3

**Phase 2, after the MVP has returning users:** week-over-week deltas on stored snapshots; decisions
recorded against optimization verdicts; a documented CI recipe; a second validated candidate family.

**Phase 3, gated on Phase 2 retention:** an agent identity contract, which is the hardest unsolved
problem and was the hardest thing in the closed research; behavioural baselines; then, and only then,
any conversation about policy or supervision.

---

## 21. Metrics

Instrument only what exists. **Activation:** time to first useful result, first repeated use.
**Habit:** week-2 and week-4 return, weekly snapshot opens. **Value:** findings opened, interventions
tested, interventions validated on holdout. **Moat indicators:** weeks of history retained, decisions
referenced later, validated optimizations.

`replay advise` currently has **zero** telemetry on whether anybody opens a finding. That is the
first thing to know and it is unmeasured today.

---

## 22. Risks

1. **Building the AI Week screen before the metrics exist.** The single largest risk, and the
   repository has a documented history of exactly this.
2. **Agent identity.** Phase 3 depends on it and the research found it unsolved.
3. **Verifier coverage.** A measurement product that can judge 16% of its own advice is fragile.
4. **Price versus value.** The arithmetic does not currently clear.
5. **Incumbents.** Datadog ships hypothesis-driven investigation now.

---

## 23. Anti-goals

Not an observability dashboard, not an LLM tracing product, not a SIEM, not an AI security platform,
not an autonomous recommender, not a BI clone, not a warehouse before the value is proven. **No
free-form query, ever.**

---

## 24. Open questions

1. What is the agent identity contract? Everything past Phase 2 waits on it.
2. What is "evidence completeness" as a computable number?
3. Does anyone open a `replay advise` finding? Unmeasured.
4. Is the value recovered time, or something else? The arithmetic says not time.

---

## Next build sequence

1. One epistemic vocabulary, internal, replacing per-surface conventions. **BUILD NOW.**
2. Advice coverage printed wherever advice is shown. The count exists. **BUILD NOW.**
3. Compaction reality on the free path. **BUILD NOW.**
4. Optimization verdicts surfaced, holdout always beside training. **BUILD NOW.**
5. Dated weekly snapshot. **BUILD NEXT.**

Each is RED-first, bounded, and touches evidence the product already captures. **If a step needs
evidence that is not captured, it stops and the dependency is recorded rather than fabricated.**

---

[Documentation index](README.md) · [Repository README](../README.md)
