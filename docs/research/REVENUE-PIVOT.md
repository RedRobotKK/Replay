# Replay as work sample: evidence to revenue

**2026-09-30.** An operator's decision memo. No experiment was run to produce
it, no API call was made, no feature was proposed, and no experimental
conclusion was rewritten.

---

## 1. Executive verdict

**Replay is not a product. It is the most credible work sample in the
portfolio, and the asset being sold is the operator.**

The individual $25/month question is closed on the repository's own
arithmetic, and the last open route closed on 2026-09-30 when the
observed-field count returned **0 of 95**. Nothing in this document reopens it.

What survives is harder to see because it is not a feature: across eighteen
months this repository accumulated an unusual quantity of evidence that one
person can **characterise an opaque system from the outside, instrument it,
measure its economics, falsify his own attractive hypotheses, and stop.** That
capability has buyers. The software does not.

The recommended action is a fixed-scope paid engagement at a price an order of
magnitude below what has already failed to sell, because the open question is
not what the capability is worth. It is whether anyone will pay for it at all.

---

## 2. Frozen evidence boundary

Carried forward verbatim in substance. None of this is renegotiated by this
document.

- Replay measures **resource and economic facts**.
- Replay reconstructs and verifies **evidence-backed claims about resources**.
- Replay **does not establish that any work succeeded.** No task-outcome signal
  exists anywhere in the corpus, and four independent lines of enquiry stopped
  at that same absence.
- Replay **does not claim savings because spend was observed.** Prompt-structure
  savings are REFUTED, twice, with both replications reversing sign. Compaction
  work loss is REFUTED. `replay advise` reports **0 verified**.
- **The $25 individual subscription thesis is closed.** 2.75% rebilled share
  means roughly $909/month of spend at 100% recovery, and recovery is unproven.
  Flat-seat users recover $0, with a null result behind it.
- The paid gate is dead. The paid rules feed is not being built.
- **No new product wedge is manufactured from this evidence below.**

---

## 3. Demonstrated capabilities

Every row names an artifact. Nothing here is a claim about aptitude.

### Demonstrated

| Capability | Artifact that proves it |
|---|---|
| Characterise a black-box system's internals from its outputs alone | `cached = 128 * max(0, floor(n/128) - 1)`, exact across 35 rungs, 18/18 at prospectively discriminating rungs against 0/18 for the rival |
| Preregister a falsifier and honour it when it fires | `cache-model-prereg-2026-09-29.md` written before the round; Gate A's 80 trials returning NOT REPLICATED and the campaign stopping on the frozen rule |
| Find a defect in a vendor's published documentation | measured 128-token block against DeepSeek's published 64-token storage unit |
| Build a verifier and prove it can fail | four conservation identities, zero violations over 575 responses, the verifier mutation-checked by injecting seven tokens and watching it go red |
| Reconcile a derived figure against an independent record | derived $0.248764126 inside an observed balance interval, reported as an interval rather than a point |
| Detect and correct a defect that flattered his own result | `DEFECT-interval-width.md`: the reconciliation band was half its true width, found by the author, corrected against himself |
| Withdraw a published figure | the $406.07 "only measured dollar figure on the machine" withdrawn after the files were opened; a 98.8% that became 4.2%, still on the page |
| Distinguish absence from zero, in code | `ContractUnknown` as the default; `usage.Record.Validate()` refusing records that do not add up; `cachemodel.CountersWriteMissing` naming the third state |
| Recognise an instrument failing and stop | three scorers over one task, each failing differently on real data; candidate A rejected; the campaign terminated rather than continued |
| Normalise a live cross-vendor bug class | inclusive vs exclusive token counting, `usage.FromInclusive` plus a conservation check, against live captured fixtures |
| Ship a proxy into a credential path without holding secrets | `replay serve`, byte-for-byte passthrough, message text never written, enforced by test |
| Maintain a dependency-free binary under pressure | `TestX402_NoSigningCapability`: zero `require` lines, no `go.sum`, verified |

### Plausible, not demonstrated

- That these methods transfer to a codebase he did not write. Every artifact is
  his own repository or a black-box provider, never a client's system.
- That he can do this under a deadline with someone else's priorities.
- Team leadership on technical work. The LinkedIn record after 2008 is
  commercial; the engineering record before it is not on the profile at all.

### Unsupported

- That any of it improves an outcome. Zero interventions verified.
- That the approach scales past one operator. One person, one machine, one
  account, stated in the README.
- Any novelty or patentability claim. Six candidates examined, all found
  anticipated; zero patent-database searches ever run.

---

## 4. The three strongest work samples

### WS-1. Characterising a black-box prompt cache

| | |
|---|---|
| **Problem** | A provider exposes cache hit counts and nothing about how the cache works. Cost depends on behaviour nobody documents. |
| **Evidence available** | Provider-reported `prompt_cache_hit_tokens` only |
| **Investigation** | 35 prefix rungs; a model fitted on 12; a **preregistered** falsification round with predictions printed before each call |
| **Mechanism found** | 128-token block quantisation with the final block never served, so nothing under 256 tokens caches |
| **Verification** | 18/18 at rungs chosen to discriminate, against 0/18 for the operator's own prior favourite |
| **Retraction** | The preregistered round came back **4 of 5**, not 5 of 5. It was not quietly refit: the miss was traced to a word-to-token estimate, the estimate was removed, and the round was rerun with the estimate eliminated. Separately, a circulating "17/17 rungs exact" figure was found unsupported by its own source and retired |
| **Artifact** | `cache-characterisation-2026-09-29.md`, `cache-model-prereg-2026-09-29.md`, raw JSON, `mechanisms.md`, `claims.md` |
| **Why it demonstrates ability** | Almost nobody preregisters against a commercial API. The retraction is the strongest part: an attractive result was allowed to fail and was fixed by removing the weak instrument rather than by rescuing the number |
| **Who cares** | Inference providers, gateways, anyone whose margin depends on cache behaviour they have not measured |

### WS-2. Reconciling a cost figure against the provider's own record

| | |
|---|---|
| **Problem** | Every LLM cost tool multiplies tokens by a price table. None of the 21 examined checks the answer against anything |
| **Evidence available** | 575 saved responses and a balance endpoint reported to the cent |
| **Investigation** | Four conservation identities; two independent reconstruction paths; inclusive vs exclusive counting isolated as a live bug class and normalised |
| **Mechanism found** | Provider usage is internally consistent and the response body is sufficient evidence of cost, **for resources only** |
| **Verification** | Zero violations; paths agree to nine decimals; the verifier itself mutation-checked; derived figure inside an observed interval |
| **Retraction** | The reconciliation band was **half its true width**, because a difference of two cent-resolution readings carries two quantisation errors. Found by the author, against his own result, and recorded as a defect rather than quietly widened |
| **Artifact** | `results/track-b.md`, `DEFECT-interval-width.md`, `internal/usage`, `provider_conformance_test.go`, live DeepSeek fixtures |
| **Why it demonstrates ability** | The self-found defect is the evidence. Anyone can compute a number; this is someone who went looking for the way his own number was flattering him |
| **Who cares** | Observability vendors, gateways, FinOps teams, anyone who publishes a cost figure customers act on |

### WS-3. Killing his own hypothesis and stopping

| | |
|---|---|
| **Problem** | Does an executable but insufficient verification anchor reduce an agent's contradiction detection? |
| **Evidence available** | A preregistration, a frozen stopping rule, and a budget |
| **Investigation** | 80 trials across four arms. Then, when the design proved to have no headroom, three further attempts to build a calibrated task |
| **Mechanism found** | **None.** C0 20/20, T2 20/20, Fisher p=1.0000, risk difference 0%. Both arms at ceiling |
| **Verification** | The frozen rule was applied verbatim: "T2 not below C0: the mechanism is NOT established. Report and stop" |
| **Retraction** | Three consecutive scoring instruments each failed on real data in a different way. Each failure was recorded rather than patched over. Candidate A was rejected after four attempts and the whole line terminated |
| **Artifact** | `docs/research/a2/A2_CLOSURE.md`, `A2_GATE_REVIEW.md`, hashed trajectory manifests, three archived scorers |
| **Why it demonstrates ability** | This is the rarest of the three. The result was a null, the sunk cost was real, the incentive to rescue it was obvious, and it was reported as a null and stopped. A hiring manager for an eval or safety team is looking for exactly this and can almost never find evidence of it |
| **Who cares** | Model labs' eval teams, AI reliability and evaluation companies, anyone who has been burned by a benchmark that could not fail |

---

## 5. Buyer and market hypotheses

Assessed against the frozen boundary. **No category has positive demand
evidence.** Two carry recorded negative evidence.

### 5.1 Observability and gateway vendors

**Who has the pain.** Langfuse, Helicone, LiteLLM, Portkey, OpenRouter and
similar. Their cost figures are computed forward and never checked. The
inclusive-versus-exclusive dialect defect is a **publicly filed bug class**
(`continuedev/continue#13104`), which is pain someone has already written down.

**What they would pay for.** A provider conformance suite with live captured
fixtures, and correctness of the cost figure their customers act on.

**Why Daniel over another tool.** The tools are the thing that is wrong. He has
the fixtures, the normaliser, the conservation check, and a documented instance
of finding the defect.

**Artifact.** `provider_conformance_test.go`, `testdata/deepseek`, `internal/usage`.

**Smallest credible offer.** A fixed-scope conformance audit of one provider
adapter, delivered as tests plus fixtures they keep.

**Demand evidence.** None positive. The filed bug is evidence of the *problem*,
not of willingness to pay.

### 5.2 Enterprise AI FinOps and spend governance

**Who has the pain.** Platform and finance teams carrying a metered AI bill
nobody can attribute. The sharpest measured instance is the **8.1x** aggregate
error in `yc-software/qm`'s spend throttle over 52,511 real requests, where
collapsing four Anthropic usage fields into one accounts for 101% of the error
and the flat rate for -1.1%.

**What they would pay for.** An investigation that says what the bill was, where
the cache is breaking, and where the budget is unreachable.

**Why Daniel.** Reconciliation against the provider's own record was not found
in 21 products examined.

**Artifact.** The 575-response reconciliation; `replay ceiling`.

**Smallest credible offer.** A fixed-fee spend forensic on 30 days of traffic.

**Demand evidence.** **Negative and recorded.** A week was quoted at $22,000
(range $18-25k) and **none sold**. This category has been tested at that price
and failed.

### 5.3 AI reliability, evaluation and verification

**Who has the pain.** Teams whose benchmarks cannot fail, whose scorers agree
with themselves, and who discovered it late.

**What they would pay for.** Someone who builds the instrument, catches it
lying, and says so.

**Why Daniel.** WS-3 is a portfolio piece for precisely this and is unusually
hard to fake.

**Artifact.** Three archived scorers, mutation-proven suites, the compliance
finding, the A2 closure.

**Smallest credible offer.** An audit of one existing eval harness: what can it
fail on, and what would a passing run still leave unsettled.

**Demand evidence.** Hypothesis only. Never tested.

### 5.4 AI infrastructure and research engineering roles

**Who has the pain.** Teams hiring people who can characterise opaque systems
rather than call APIs.

**What they would pay for.** Salary.

**Why Daniel.** The artifacts are stronger than most interview loops produce.

**The problem, stated plainly.** His LinkedIn record from 2008 onward is
**commercial**: account manager, sales rep, director of sales, VP of sales and
marketing, four advisory seats. The engineering record that would support an IC
or staff role is entirely pre-2008 and **absent from the profile**. For this
category the credibility of proof is currently low despite the capability being
high, and that gap is a documentation problem rather than a capability one.

**Demand evidence.** He is looking for work, so the category is live. No
positive signal yet.

### 5.5 Specialised forensic and advisory engagements

**Who has the pain.** Whoever just received an AI bill they cannot explain, or
shipped an agent that quietly costs three times what was modelled.

**What they would pay for.** A bounded investigation with an artifact at the end.

**Why Daniel.** The whole repository is a demonstration of this exact motion.

**Artifact.** Every evidence file. The method is the product.

**Smallest credible offer.** See section 7.

**Demand evidence.** Negative at $22-25k. **Untested below that.**

---

## 6. Ranked targets

Ranked on demonstrated capability x buyer pain x ability to pay x credibility of
proof x probability of access. **Access is weighted heavily**, because the
recorded failure mode is not rejection, it is never reaching a buyer at all: 53
lifetime install fetches from 2 distinct IP addresses.

| Rank | Target | Why it ranks here |
|---|---|---|
| **1** | **Open-source observability and gateway maintainers** (Langfuse, Helicone, LiteLLM/BerriAI and similar) | The only category where **access is structurally open**: they accept contributions and talk to contributors. The defect is already filed publicly, he has the fix and the fixtures, and the conversation starts with work rather than with a pitch. Ability to pay is moderate, probability of access is the highest available |
| **2** | **Inference providers and gateways with cache-dependent margin** (OpenRouter-class, and inference startups) | Highest overlap with WS-1. Their economics depend on cache behaviour they usually have not characterised. Well funded. Access is cold but the artifact travels |
| **3** | **AI eval and reliability companies** | WS-3 is close to a perfect credential and there is no substitute for it. Smaller budgets, and demand is pure hypothesis |
| **4** | **Model labs' eval and infrastructure teams** | Highest pay, strongest fit for WS-3, **lowest probability of access** without a referral. Do not lead here |
| **5** | **Enterprise platform teams with a metered agent bill** | Highest measured pain (8.1x in a named third party's throttle) and real budget, but requires a warm introduction and has already failed at $22k |
| **Killed** | Individual developers | The subscription arithmetic is closed. Do not re-approach |
| **Killed** | Selling Replay as software to any of the above | The panel verdict stands: the paid column collapses, and the BUSL grant already permits the CI use a paid tier would charge for |

---

## 7. The smallest commercial experiment

**No software build. No research campaign. No API spend.**

| | |
|---|---|
| **Offer** | A fixed-scope **AI spend and cache forensic**. One codebase or one month of agent traffic. Deliverable in five working days |
| **Buyer** | A team running metered agent traffic at $2,000/month or above, reached through category 1 or 2 above |
| **Deliverable** | A written report leading with **where their budget ceiling actually halts execution against real spend**, since that is the largest measured effect; then what the traffic cost, reconciled against the provider's own record and reported as an interval; then every cache break with its cause and position; and an explicit list of what could not be determined and why. Plus the tooling, which is free and stays free |
| **Price hypothesis** | **$2,500 to $5,000.** Deliberately an order of magnitude below the $22,000 week that has already failed to sell. The question being tested is not what the capability is worth, it is **whether anyone will pay anything for it** |
| **Proof artifact** | WS-2 for the reconciliation, WS-1 for the cache work, and the retraction record for why the number can be trusted |
| **Outreach target** | Three to five named maintainers or engineering leads in categories 1 and 2. The opening is the filed dialect defect and the fixtures, not a service pitch |
| **Success criterion** | **One paid engagement completed and delivered.** Not a signed LOI, not interest, not a call that went well |
| **Falsification criterion** | **Five qualified conversations with a buyer who has the pain and the budget, and zero closes.** At that point the capability is not purchasable in this form and the correct next move is employment (category 4 or 5), not a smaller price |

The single highest-probability action inside this is specific and grounded, not
generic: **the inclusive-versus-exclusive cost defect is publicly filed in
open-source repositories, and he holds the fix, the normaliser, the conservation
check and live captured fixtures.** Contributing that is simultaneously a
technical act, a credential, and an introduction to the buyers in category 1.
It is recommended because the artifact exists and the bug is filed, not because
contributing to open source is generally advisable.

---

## 7a. Recoverable optimizations, by measured magnitude

Added after the question was put directly: is 2.75% really the whole prize? It
is not. But only one lever is large, and that one has an undetermined sign.

| Lever | Measured magnitude | Recoverable? | Status |
|---|---|---|---|
| **Cache-collapse in a third party's budget throttle** | **8.1x aggregate over 52,511 requests**, 16.3x on Sonnet, measured against `yc-software/qm`. Ignoring the cache is **101% of the error**; the flat rate is **-1.1%** | **Sign undetermined for the level; the misallocation is sign-independent.** See below | the only lever large enough to price against |
| Re-billed waste from cache breaks | 2.75% shipping build, 4.99% on v0.5.4, 4.2% on a fan-out session. Realistic recovery 2-3%, so **$60-90/month** on a $3,019 bill | yes, but small | one-time per cause |
| Reasoning-off by task class | **19.5x** fewer output tokens on lookup, but only **1.41-1.48x** total cost once a warm corpus is in the prompt, because input dominates at 70-98% | partly | class-conditional; 29% accuracy on aggregation, failing as confident wrong numbers |
| Configuration fixes (MCP bind order, warm-then-fan) | traced every break in one session to one connector binding mid-session | yes | **one afternoon, free, does not recur** |
| Prompt and prefix structure | **REFUTED.** Two replications, both reversing sign. The 98.8% that would have justified it was an instrument artifact, corrected to 4.2% the same morning | **no** | do not resurrect |

**PROVENANCE CORRECTION, 2026-09-30.** Earlier versions of this file cited
**"7.94x over 57,958 requests, halting at $62.94"** four times. **That figure has
no evidence file.** `7.94` appears in `MONEY-PATH.md`, this file,
`RESEARCH-INDEX.md`, `guide/commands.md`, and in
`internal/cachemodel/ceiling_test.go:281`, where it is a **synthetic fixture**:
`CeilingEffect{Requests: 10, CorrectUSD: 100, BlindUSD: 794}`, which is 7.94
because 794/100 is. `57,958` and `$62.94` appear in no evidence file at all.

The evidenced ancestor is `docs/evidence/qm-budget-2026-09-08.md`: **8.1x
aggregate over 52,511 requests**, scoped to `yc-software/qm`'s shipped code, on a
corpus with a 94-99% cache share. That file states its own limits, which every
downstream citation dropped: "The ratios are the result. The absolute totals are
not," and "a QM engineer defending the flat rate as an acceptable approximation
is **right**, and the finding survives them completely."

A figure in a business document with no provenance is the defect class this
repository exists to find. It is corrected here rather than carried.
`MONEY-PATH.md` lines 203 and 578 carry the same unevidenced figure and are
**not** edited by this pass; that is a separate ticket.

**The finding that matters, and why it is not yet a claim.** The measured error
is far larger than the waste figure, and MONEY-PATH 0.7 already states the
problem with it:

> Nobody has asked whether that is value to the buyer or a bill they were
> deliberately avoiding. If the cap was a guardrail against runaway agents, then
> correcting the arithmetic unlocks capacity they already paid for and the value
> is real. If the cap was a budget, correcting the arithmetic hands them a
> $13,000 a month increase, sold to the person who set the limit. **Those are
> opposite products and no measurement in this repository distinguishes them.**

No measurement on this machine can distinguish them, because the question is
about the buyer's intent rather than about the traffic. It is settled by asking,
not by measuring.

**What this changes in the offer, and what it does not.**

It does **not** license a savings claim. Prompt-structure optimization stays
refuted, the recoverable waste stays at 2.75%, and nothing here says an
intervention improves an outcome.

It **does** change what the engagement leads with. An offer that opens on "I
will find your waste" is selling $60-90 a month and correctly loses. An offer
that opens on "your budget ceiling is halting your agents at an eighth of what
you approved, and I will show you where" is selling something an order of
magnitude larger, and it is the same work on the same artifacts.

**And the engagement is the instrument.** Five conversations settle the sign
question, and MONEY-PATH says so. A paid forensic that reports the ceiling
finding to five buyers answers it as delivered work rather than as unfunded
research. That is the cheapest available route to the one figure that is both
load-bearing and untested, and it is the reason the experiment in section 7 is
worth running even if it converts poorly.

---

## 8. Offer hypothesis, stated so it can be wrong

> A team running metered agent traffic will pay $2,500 to $5,000 for a
> five-day investigation that shows them where their budget ceiling is halting
> their agents short of the capacity they approved, tells them what the traffic
> actually cost checked against the provider's own record rather than computed
> forward, and names every cache break with its cause.

**What makes it plausible:** the reconciliation practice was not found in 21
products examined; the cache-break attribution was not found either; and the
8.1x finding is a large, concrete, measured number, scoped to one named
codebase and one corpus.

**What makes it doubtful, recorded honestly:** the recoverable *waste* is 2.75%,
so on a $2,000/month bill the engagement costs more than a year of the waste it
finds. **The offer therefore cannot be sold on savings.** The throttle-error
finding is much larger and is the reason the offer is worth making at all, but it
is scoped to one named codebase, and the sign of any level change is
undetermined: it is either capacity a buyer already paid for, or a budget
increase sold to the person who set the budget. If it turns out to be the
second, this offer has no buyer and the experiment will show that quickly, which
is the point of running it small.

---

## 9. Success and falsification

| | |
|---|---|
| **Success** | One engagement paid and delivered. The capability is purchasable, and the next question is price and repeatability |
| **Partial** | Paid, but the buyer wanted the tooling rather than the investigation. That reopens the product question with real evidence for the first time, which is the only legitimate route back |
| **Falsified** | Five qualified conversations, zero closes. The capability is not purchasable as an engagement. Move to employment and stop selling |
| **Void** | Fewer than five qualified conversations reached. Nothing is learned about demand; the constraint was access, which is the failure mode already on record |

---

## 10. What not to build

- **No paid gate.** Ten panel seats converged, and the BUSL Additional Use Grant
  already permits CI use without restriction.
- **No paid rules feed.** The observed-field count returned 0 of 95.
- **No entitlement mechanism.** ADR-0023's own precondition, a symbol-table
  guard, is unbuilt, and no surviving paid item needs one.
- **No adapters** for AnythingLLM, OpenClaw, Oracle or Cursor. Blocked on
  dependency, privacy and source evidence respectively.
- **No `/responses` parser.** The only capture was shape-only.
- **No org or tenant features.** ADR-0015 blocks them and no buyer has asked.
- **No new measurement campaign** on this machine's corpus. It is one machine,
  and every remaining question needs a corpus that is not his.
- **No individual subscription surface of any kind.**

---

## 11. What would reopen Replay as a product

Four triggers. Any one is sufficient; none has occurred.

1. **A second corpus, from a machine that is not his**, showing rebilled share
   materially above 2.75%. Everything in the arithmetic rests on one machine.
2. **The observed-field count returning non-zero** on a corpus that includes
   traffic near the cache floor. Today it is 0 of 95, and ordinary sessions can
   never bound it because their prefixes are tens of thousands of tokens.
3. **A buyer asking, unprompted, for the enforcement rather than the
   investigation.** That is demand for a capability rather than for labour, and
   it is the only clean signal the gate question was closed prematurely.
4. **An organisation-shaped buyer appearing**, which reopens ADR-0015's tenant
   dimension as an architecture question with a customer behind it.

Absent those, the product question stays closed and the operator is the unit
being sold.
