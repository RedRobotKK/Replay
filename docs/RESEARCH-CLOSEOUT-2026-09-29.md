# Research campaign closeout

**2026-09-29. Canonical record for the campaign that ran from the DeepSeek
optimization work through the evidence-anchor and seam-selection passes.**

**Status: PAUSED: RESEARCH FRONTIER IDENTIFIED.** Not failed, not abandoned, not
complete forever. The restart condition is in section 7.

No code changed to produce this memo. No experiment was run for it, and nothing
here is a new claim: every figure below was generated earlier in the campaign
and is restated under the scope it was measured in.

The question it answers is not what Replay is for. It is what Replay is
demonstrably good at today, on evidence already generated rather than on
intended architecture.

---

## 0. Distinctions this memo preserves

The campaign died five times at the boundary between two of these words being
treated as one. They are kept apart deliberately, and a reader who collapses
them will read this memo as stronger than it is.

| | |
|---|---|
| **Measurement** | A quantity was read off an artifact. |
| **Reconciliation** | Two independently produced quantities were compared and agreed within a stated band. |
| **Verification** | A property was checked by something that can fail, and was shown to fail when the property is broken. |
| **Intervention** | A change was made with the intent of moving an outcome. |
| **Outcome** | Whether the work succeeded. |

Four consequences, each of which constrains a section below.

1. **Resource verification is not task-success verification.** Replay reaches
   verification for what was consumed. It does not reach it for what was
   accomplished.
2. **Missing evidence is not negative evidence.** A hypothesis with no
   measurable endpoint is recorded as unmeasurable, never as false, and never
   converted into a proxy claim.
3. **Observed provider and account state is not inferred behaviour.** The
   balance readings are observations. Everything reconstructed from transcripts
   is derived, and the two are never summed into one number.
4. **A measurement boundary is not a product failure.** Five of this campaign's
   terminations are boundaries of the instrument, not defects in it.

A refuted hypothesis stays refuted. An unmeasurable one stays unmeasurable.

---

## 1. What Replay has proven

### 1.1 Cost reconstruction reconciles against provider state

| | |
|---|---|
| Responses | 575 |
| Derived cost | $0.248764 |
| Observed balance interval | ($0.230, $0.250) |
| Conservation identities | four |
| Violations | zero across the corpus |
| Independent reconstruction paths | two, agreeing to nine decimals |
| Mutation testing | injected token discrepancies were caught |

The interval is two cent-resolution balance observations differenced, so it
carries two quantisation errors and is $0.02 wide rather than $0.01. Derived
cost lands inside it. This is reconciliation, in the sense of section 0: two
independently produced quantities compared within a stated band.

### 1.2 Cache pricing is consistent with the published model under the tested conditions

A batch was constructed so that competing hit-rate and miss-rate hypotheses
differed by 29x. It resolved at a ratio of 1.023.

That is the whole claim. It is consistency with the published model under the
conditions tested, not an independent confirmation of provider billing in
general.

### 1.3 Prefix-cache geometry, in the tested sequential regime

`cached = 128 * max(0, floor(n/128) - 1)` was exact on 35 rungs, with 18 of 18
prospectively discriminating rungs separating it from three competing models.

**Scope.** Sequential dispatch, one provider, the shapes tested. This is not a
universal provider guarantee and must not be cited as one. A concurrent or
otherwise differently populated regime is outside what was measured, and
section 1.4 is the reason that qualifier is load-bearing rather than cautious.

### 1.4 Prefix matching is byte-exact in the tested regime, and population order matters

One leading space produced zero cached tokens. Concurrent dispatch produced a
0% hit rate where sequential dispatch on the same shapes produced 86%.

### 1.5 A structured claims table changes what a fresh agent retrieves

| | |
|---|---|
| With the claims table | 51/60 |
| Without it | 0/12 |
| p | 1.9e-08 |
| The identical claim in prose | 0/4 |

**This is a retrieval effect.** It says the agent's search went somewhere else
and the fact entered its context. It is not recognition, not reasoning, and not
a task-outcome improvement. The supporting transcript analysis is explicit that
recognition-given-arrival could not be compared at all, because in these trials
that cell is empty: arrival and naming matched exactly, so no trial exists in
which the fact arrived and was missed.

### 1.6 A resolvable destination improves evidence-supported resolution

5/25 rose to 25/25. A plausible wrong destination produced confident wrong
answers in 12 of 13 failures.

**Correction preserved.** The later reading of the anchor work is that a file
citation is a **locating** anchor, not a machine-established **dispositive**
one. Only executable anchors were dispositive. No arm in the anchor series
carried an anchor sufficient to settle the claim it pointed at, and that
remains unmeasured. Anchor sufficiency is not resurrected here as a product
primitive.

### 1.7 Failure persistence is structurally separable from retry

0 of 49 failure runs were identical-input retries.

This is an observed structural property of the failure corpus. It is not a
claim of causal recovery and not general failure-cause identification. See
section 2.7.

---

## 2. What Replay has not proven

1. **That any agent task succeeded.** No outcome signal exists anywhere in the
   corpus. This is the root of every other entry in this section.
2. **That an intervention improves an outcome.** Zero interventions were
   verified.
3. **That delegation is economically positive or negative.** Recorded as **NO
   MEASURABLE ENDPOINT**: parent-to-child identity and task outcome are both
   absent, so there is no unit to divide cost by. Economics must not be
   inferred from token volume, which measures dispatch and not value.
4. **That Replay's own advice works.** `replay advise` returns **126
   AdviceOnly, 12 pending, 0 verified**. The verifier never fires because the
   dominant advice kinds are `AdviceOnly` by construction. Per section 0 this
   is unverified, not refuted.
5. **That prompt structure reduces cost.** **REFUTED.** The candidate effects
   failed replication twice, with both replications reversing sign. Not to be
   resurrected under another name.
6. **That compaction causes sustained work loss.** **REFUTED.** The apparent
   short-window positive was an instrumentation artifact. That manual
   `/compact` was not separately tested is not grounds to reopen it.
7. **That failure cause is generally recoverable.** 69% of errors carry no
   recoverable predicate. Section 1.7 is a structural observation and does not
   extend to causal recovery.

---

## 3. Strongest demonstrated primitive

**Evidence, then reconstruction, then reconciliation, then a bounded claim.**

Replay reads provider-reported usage from response artifacts, deterministically
reconstructs cost from a dated rate table, and checks that reconstruction
against independently observed account state. The result is reported as an
interval rather than a falsely precise point.

Every step is machine-checkable, and the checks were themselves checked: the
conservation identities held across 575 responses, two independent
reconstruction paths agreed to nine decimals, and mutation testing caught
injected token discrepancies rather than passing regardless.

This is a **measurement and reconciliation primitive**. In the vocabulary of
section 0 it reaches verification, but only for resource facts:

> Replay can verify what was consumed. It cannot currently verify what was
> accomplished.

---

## 4. Missing primitive

**An independently observable task outcome.**

The ledger converges on this from four directions, which is why it is one gap
observed four times rather than four gaps.

| Termination | Where it stopped |
|---|---|
| Delegation economics | No joinable parent-to-child identity, and no success signal on the child |
| Advisor verification | The dominant advice kinds are `AdviceOnly` by construction, so the verifier fires zero times |
| Failure persistence | Nothing records whether a condition was actually resolved |
| Behavioural intervention | Every hypothesis terminates at the same missing endpoint |

Without an outcome signal: intervention verification has no after-state;
counterfactual optimization has no comparable outcome unit; delegation
economics has no completed-work denominator; and any behavioural-improvement
claim collapses into resource accounting. That collapse is exactly how the
prompt-optimization campaign produced a null.

The missing primitive is therefore not better prompting, not better memory and
not more telemetry. It is **a trustworthy outcome signal joinable to the
existing resource trace**. No proxy for it is proposed here, because a proxy
would convert an unmeasurable hypothesis into a claim, which section 0 forbids.

---

## 5. Honest product boundary

**TODAY.** Replay has demonstrated **resource reconstruction and reconciliation
for AI workloads**: what was consumed, how it was billed, where cache behaviour
changed, what that behaviour cost, and how the reconstruction compares with
independently observed provider state.

**NOT YET.** Replay has **not** demonstrated an optimizer, an intervention
system, a reliability system, a routing engine, or an outcome-improvement
system. The optimization hypotheses tested in this campaign either failed
replication or had no outcome endpoint.

**WHAT WOULD CHANGE IT.** An independently produced, machine-checkable
task-outcome signal that can be joined to an existing Replay resource trace at
the unit of work. The existing primitive then closes a loop it cannot close
today: predict, intervene, measure, verify.

This boundary is not to be softened with indirect success metrics. An indirect
metric here would be a proxy for the one thing the campaign established is
absent.

---

## 6. IP status

**NO CURRENT IP LEAD.**

**PATENT PRIOR ART SEARCH = NOT_VERIFIED.**

The prior-art work identified the closest conceptual neighbours and found the
evidence-sufficiency mechanism conventional. Evidence-carrying termination with
sufficiency as a stopping gate is published in the 2026 literature; the
distinction between relevance and sufficiency appears verbatim in several
contemporary sources and ships in at least one MCP server; and the conceptual
ancestry reaches FEVER, PCAOB AS 1105 and Toulmin.

The remaining potentially distinctive observation, reconstructing resource cost
and checking it against provider account state, is a measurement and
reconciliation schema rather than an established invention.

**This was not a comprehensive patent search, and nothing above should be read
as implying one was performed.** No novelty is claimed and no patentability is
claimed.

---

## 7. Research restart condition

The campaign is **PAUSED: RESEARCH FRONTIER IDENTIFIED**. The frontier is the
missing primitive in section 4.

Restart when a task-outcome signal becomes available that is:

1. **independently produced**, not emitted by the agent being measured;
2. **machine-checkable**;
3. **attachable** to a specific lane, episode or unit of work; and
4. **joinable** to an existing Replay resource trace.

All four conditions are required. Three of four yields another measurement
boundary, which this campaign produced five times.

Until then, further research on this frontier is expected to produce boundaries
rather than a closed-loop behavioural result. That expectation is why the
campaign is paused rather than continued, and it is not a judgement that the
frontier is unreachable.

---

## 8. Final thesis

> **Replay is demonstrably an instrument that reconstructs what an AI workload
> consumed and reconciles it against the provider's own state. Its evidence
> records do not yet contain a trustworthy, joinable signal showing whether the
> work succeeded.**
