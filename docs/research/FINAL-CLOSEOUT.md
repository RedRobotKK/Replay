# Final closeout: what the research campaign discovered

**2026-09-29. Read-only synthesis. No API call, no agent run and no experiment
was performed to produce this document.** Every figure below is restated from an
artifact that already existed, under the scope it was measured in. Where this
document differs from an earlier artifact, the difference is a documentation
correction and is named as one in section 1.4. No observation was altered.

This supersedes nothing. `../RESEARCH-CLOSEOUT-2026-09-29.md` remains the
canonical closeout for the behavioural and evidence-anchor line. This document
covers the whole corpus and answers one question:

> **What did we actually discover, what did we actually rule out, and is there
> now a concrete technical mechanism worth taking into dedicated prior-art
> investigation?**

---

## 1. The freeze

### 1.1 Repository

| | |
|---|---|
| Branch | `fix/compaction-observed-vs-inferred` |
| HEAD | `8e871bf` pool: stop publishing a provider-account claim nothing can support |
| Campaign commits | `4c69268` closeout, `bda9dec` Grok reconciliation, `0e06db0` / `213e653` / `631698b` / `8e871bf` contribution boundary |
| Test status | **36 packages ok, 0 failures**, `go test ./...`, re-run for this synthesis |
| Build | clean |

One test failed when this closeout began: `TestNoOrphanedDocuments` named six
`docs/research/a2/` documents nothing linked to. That is a real documentation
defect and it was fixed by adding `a2/README.md` as the section index. No
production code and no experimental data was touched. The suite is green.

### 1.2 Uncommitted research artifacts

`docs/research/` is untracked in its entirety: nine programme documents, seven
A2 documents, and this file. Also untracked: `docs/evidence/modes-catalogue-2026-09-25.md`,
`internal/stateledger/`, `.claude/`. Modified and uncommitted: `docs/README.md`,
`docs/WORK-STATE.md`, `docs/design/UNWIRED-LOG.md`,
`internal/regression/unwired_packages_test.go`.

### 1.3 Frozen manifests and raw observations

All live in the session scratchpad, not the repository. All re-verified for this
closeout.

| Corpus | Contents | Manifest | Verified |
|---|---|---|---|
| Gate A, 80 trials | 4 arms x 20 runs | `anchor-exp/LOCK/raw-runs.sha256` | **81/81 OK**, run directory read-only |
| A2, all attempts | 20 trajectories, 3 scorers, 3 test suites, 2 prompts, 4 protocol docs | `a2/LOCK/A2_CLOSURE.sha256` | **73/73 OK** |
| DeepSeek suite | 9 raw JSON observation sets, 3 derived sets, 9 result and ledger documents | `deepseek-suite/LOCK/DEEPSEEK_SUITE.sha256` | **25/25 OK** |
| M3 discriminator (experiment 1) | prereg, script, raw result, 24 request and response artifacts | `m3/M3_LOCK.sha256` | **28/28 OK**, raw read-only |

The DeepSeek suite had no manifest before this closeout. One was created by
hashing the directory as found; no file was modified to produce it.

| Manifest | SHA-256 (first 12) |
|---|---|
| `a2/LOCK/A2_CLOSURE.sha256` | `cbdb40e6fb2a` |
| `deepseek-suite/LOCK/DEEPSEEK_SUITE.sha256` | `23082a90cdbf` |
| `docs/research/FINAL-CLOSEOUT.md` (this file, before this table was added) | see git |

Invalidated datasets, preserved and never deleted: `a2/runs` (original 8),
`a2/fresh` (fresh 8), `a2/v3/pilot` (4), plus the four entries in
`deepseek-suite/invalid/register.md` (DS-SURF-PO, DS-F2, DS-F4, and the voided
HH-02 mutation sweep).

### 1.4 Documentation corrections made here

Five inconsistencies survived into the artifacts. Each is corrected in this
document only; no underlying observation changed.

1. **Two billing figures, two scopes, stated as if one.** `$0.229947` is the
   479-response corpus in the 03:26 to 03:42 window. `$0.248764` is the
   575-response corpus in the 03:26 to 05:45 window. Both are defensible. They
   are not the same quantity and neither supersedes the other. Track D separately
   found three further totals in circulation ($0.18, about $0.86, $0.23); only
   the two scoped figures above are supported.
2. **Two reconciliation bands, two balance readings.** `($0.2200, $0.2400)` pairs
   with the 479 corpus and the $45.78 reading. `($0.2300, $0.2500)` pairs with
   the 575 corpus and the $45.77 reading. This document uses the second
   throughout, matching the final reconstruction.
3. **"17/17 rungs exact" is not supported by its own source** and appears in three
   places. The supported statements are: 12/12 on refit against observed prompt
   tokens, 5/5 fully observed with predictions printed before each warm call, and
   18/18 at prospectively discriminating rungs. The "17/17" phrasing is retired.
4. **"No artifact carries a per-call UTC timestamp" is too broad.** Chat-dialect
   response bodies carry a provider `created` field and are priced at it. The 11
   anthropic-dialect responses carry none, and `reconstruct.py` substitutes the
   corpus's latest timestamp, which is a documented silent promotion. Track D's
   statement is true of the arms-harness summary artifacts, not of saved bodies.
5. **The cross-dialect observation was recorded as "n=1, do not carry forward."**
   That was correct when written on 2026-09-28. A9 subsequently reproduced both
   outcomes and explained the difference. The retirement is lifted and replaced
   by F-07 below.

---

## 2. Evidence ledger

Evidence classes are used strictly. **OBSERVED**: a value literally present in a
saved provider response, balance response or run artifact. **DERIVED**:
arithmetic over OBSERVED values and the published rate table. **RECONSTRUCTED**:
assembled from multiple OBSERVED records by a documented procedure.
**ASSUMED**: a premise this campaign did not establish. **NOT_VERIFIED**: a
question that was not asked. No scalar confidence scores appear anywhere.

### 2.1 DeepSeek cache geometry and identity

| ID | Observation | Class | Establishes | Does NOT establish | Replication | Artifact | Survives |
|---|---|---|---|---|---|---|---|
| **F-01** | Cached tokens follow `128 * max(0, floor(n/128) - 1)` across 35 rungs; every cached count a multiple of 128 | OBSERVED (counts), DERIVED (relation) | A count relation on `deepseek-flash` in 2026-09 | The implementation. A block-commit policy and a deliberate one-block margin are indistinguishable from counts alone | 17 retrospective, then 18 prospective at discriminating rungs | `raw/track-a.json` | **yes** |
| **F-02** | At discriminating rungs: H-B 18/18, H-A 0/18, H-D1 0/18, H-D2 4/22 | OBSERVED | That the block model out-predicts three rivals at points chosen to separate them | Anything about rival formulas nobody published. They were constructed here | one prospective run, rungs chosen to discriminate | `raw/track-a.json` | **yes** |
| **F-03** | The vendor's 2024-08-02 announcement states a 64-token storage unit | OBSERVED (the document), REFUTED as a predictor of flash | A factor-of-two discrepancy against published vendor documentation | That the vendor document is wrong about its own subject (V2/MLA, quoted at $0.014/M) | two independent rung sets | `results/prior-art.md` | **yes** |
| **F-04** | One leading space dropped reuse from 2048 to 0; a trailing space did not; a mid-prefix change hit 896 = 7x128 | OBSERVED | Matching is byte-exact and prefix-wise, not semantic and not whole-string | Generality beyond one prefix length and one corpus | single run, 6 arms plus positive control | `raw/track-a.json` | **yes**, and it confirms vendor documentation rather than extending it |
| **F-05** | Sequential B hit 3200; concurrent B hit 0, with B dispatched before A returned on wall-clock | OBSERVED | Reuse requires the populating request to have completed | The commit point. Completion and first-token are indistinguishable at these latencies | A4 plus the independent all-parallel arm | `raw/track-a2.json` | **yes** |
| **F-06** | temperature, max_tokens, top_p, frequency_penalty, presence_penalty and stop all hit 3200 against a 3200 control | OBSERVED | Sampling parameters are not part of cache identity | Anything about `reasoning_effort`, which was held constant across all six arms | single run, 6 arms | `raw/track-a2.json` | **yes**, scope limited |
| **F-07** | Cross-dialect reuse: chat prefix served to `/anthropic/v1/messages` at 3200 when `reasoning_effort` absent from both bodies, 0 when present | OBSERVED | The cache is not endpoint-scoped, and the earlier apparent contradiction is explained by request-body shape | Byte-identity of the serialised prompt across dialects, which is ASSUMED | two arms, one run; supersedes both earlier readings | `raw/a9-crossdialect.json` | **yes** |
| **F-08** | Adding an unrecognised junk key to the body: 3200, against a 3200 control | OBSERVED | The cache key is not the raw serialised request body | That no other key matters. One key, one endpoint | single arm | `raw/a10-cachekey.json` | **yes**, and it is the load-bearing negative control for F-09 |
| **F-09** | Adding `reasoning_effort` to a body that lacked it: 0. Removing it from a body that had it: 0. Both against 3200 controls | OBSERVED | That `reasoning_effort` participates in cache identity, symmetrically | **The mechanism.** Key participation and prompt-assembly change are not separated | two directions, one run each | `raw/a10-cachekey.json` | **yes**, as an effect; the mechanism does not |
| **F-10** | flash to v4-pro 0, v4-pro to flash 0, against working controls both ways | OBSERVED | The cache is model-scoped, not account-global | Generality beyond two models, one endpoint, one prefix length | both directions | `raw/track-a.json` | **yes** |
| **F-11** | Populate, then query at +0.0s, +0.5s, +2.0s: 3200 at every delay | OBSERVED | There is no warm-up window to wait out | Cache TTL, which is NOT_VERIFIED at the long end | single run | `cache-characterisation-2026-09-29.md` | **yes** |
| **F-31** | Prefix populated under `reasoning_effort:"none"`, then queried with `thinking:{"type":"disabled"}`: **3200**, against a 3200 same-shape control | OBSERVED | That the **literal** reasoning parameter does not participate in cache identity | Which of candidate 1 or candidate 3 is correct. Both predict this hit | n=1, but mirrored by F-32 | `m3/raw/m3-discriminator.json` T1 | **yes** |
| **F-32** | The mirror: populated under `thinking:{"type":"disabled"}`, queried with `reasoning_effort:"none"`: **3200** against a 3200 control | OBSERVED | Symmetry. The reuse is not an artifact of one direction | Same limit as F-31 | n=1 per direction, two directions agree | `m3/raw/m3-discriminator.json` T2 | **yes** |
| **F-33** | Adding `response_format:{"type":"text"}`, a recognised parameter at its default value, to a plain body: **3200** against a 3200 control | OBSERVED | That a recognised named parameter that changes no prompt content does not bust reuse | That no other recognised parameter does. One parameter, one value | single arm | `m3/raw/m3-discriminator.json` T3 | **yes** |
| **F-34** | Junk-key arm repeated: **3200** against a 3200 control | OBSERVED | That the instrument reproduces A10 arm C, so experiment 1 is not void | Anything new. It is a positive control | replicates F-08 | `m3/raw/m3-discriminator.json` T4 | **yes**, as the run's validity control |
| **F-35** | On reasoning-off calls DeepSeek **omits** `completion_tokens_details.reasoning_tokens` rather than reporting 0; reasoning-on calls reported 8 | OBSERVED | That absence and zero are distinct in this provider's usage schema | That the field is absent for any other reason than reasoning being off | 6 calls off, 6 calls on, consistent | `m3/raw/m3-discriminator.json` | **yes**, and it invalidated a precondition I had worded as `== 0` |

### 2.2 Billing reconstruction

| ID | Observation | Class | Establishes | Does NOT establish | Replication | Artifact | Survives |
|---|---|---|---|---|---|---|---|
| **F-12** | 575 responses all carry a usage block; four conservation identities hold with zero violations and zero errors | OBSERVED | That the provider's usage reporting is internally consistent across the corpus | That the fields are complete or correct, only that they are consistent | whole corpus | `derived/final-reconstruction.json` | **yes** |
| **F-13** | Reconstructed cost $0.248764126 over 575 responses | DERIVED | A reconstruction from usage fields and the dated rate table | A total. It is a **lower bound**: 5 calls are NOT_OBSERVED | two independent paths agreed to nine decimals on the 479-response subset ($0.229946832) | `derived/final-reconstruction.json` | **yes, as a lower bound** |
| **F-14** | Balance $46.01 to $45.77; difference bounded by the open interval ($0.230, $0.250) | OBSERVED (readings), DERIVED (interval) | What the balance endpoint can distinguish, at two quantisation errors and $0.02 wide | A tighter band. The earlier $0.01-wide interval was an error, corrected | one full-session test | `results/DEFECT-interval-width.md` | **yes** |
| **F-15** | The derived figure lies inside that interval | DERIVED | **Consistency**, in the sense of reconciliation | That the provider computes cost by this path. A systematic error under about 8% would be invisible at this scale | one reconciliation | `FINAL-REPORT.md` 5.2 | **yes, as consistency only** |
| **F-16** | The conservation verifier is mutation-checked: injecting seven tokens into one response turns it red and names the file | OBSERVED | That the check can fail, so a pass is evidence | Coverage of error classes other than token discrepancy | one mutation | `FINAL-REPORT.md` 5.1 | **yes** |
| **F-17** | Balance moved $45.83 to $45.78 in a window containing no billable call; then 13 polls over 24.1 minutes all read $45.78 | OBSERVED | Settlement lag exists, and settlement converges and then stops | The lag duration, which is bounded from below only | one window each | `ledger.json` DS-B-06, DS-B-07 | **yes** |
| **F-18** | Max per-request cost $0.0056997; 0 of 479 requests reach $0.01 | OBSERVED | **REFUTES** per-request cent truncation as the explanation for the balance movement | The provider's actual rounding rule, which is NOT_VERIFIED | arithmetic over the corpus | `ledger.json` DS-B-05 | **yes, as a refutation** |

### 2.3 Reasoning configuration, cost and accuracy

| ID | Observation | Class | Establishes | Does NOT establish | Replication | Artifact | Survives |
|---|---|---|---|---|---|---|---|
| **F-19** | Mechanical/lookup: 18/18 reasoning on, 17/18 off. Aggregative: 24/24 on, 7/24 off. Zero truncated, zero undecided | OBSERVED | The lever is class-conditional: near-free on lookup, destructive on aggregation | A point estimate. The 94% rests on a single failure and supports "no large cost on lookups", not a 6-point gap | one run per cell, n=18 and n=24 | `reasoning-scope-2026-09-29.md` | **yes, for the two tested classes** |
| **F-20** | Aggregative failures are confident wrong numbers, not refusals or near-misses: counted 67 against a truth of 39, 22 against 17, 1 against 14, 6 against 2 | OBSERVED | That the degradation carries no runtime signal that it is wrong | That a cascade scorer would fail to catch it. That was not tested | four questions, one run | `reasoning-scope-2026-09-29.md` | **yes**, and it is the most under-covered observation in the set |
| **F-21** | With a warm shared prefix, disabling reasoning reduced output tokens about 39x and total cost about **1.48x** | OBSERVED (tokens, cost), DERIVED (ratios) | That input and cache economics dominate on a document-bearing prompt | Generality. The ratio is a function of prefix size and output length | one run, arms A3 and A4 | `ledger.json` DS-O-01 | **yes, for the tested shape** |
| **F-22** | Cheapest possible LLM checker costs $1.637e-4; break-even budgets are $5.49e-5 (LOOKUP) and $9.34e-5 (COUNTING) | DERIVED | That an LLM checker is 3.0x and 1.8x over budget respectively. **The LLM-checker route is closed on price** | Anything about a program checker, which costs no API dollars | analytic, from measured rates | `results/track-c-prereg.md` C5 | **yes** |
| **F-23** | For counting and deterministic transformation, a program that verifies the answer is a program that computes it | DERIVED | That those classes close by **redundancy**, not by price: the checker replaces the model | That LOOKUP and MULTI_STEP close. Those remain open with a non-LLM checker | analytic | `results/track-c-prereg.md` C5 | **yes** |

### 2.4 Durable work state and calibration

| ID | Observation | Class | Establishes | Does NOT establish | Replication | Artifact | Survives |
|---|---|---|---|---|---|---|---|
| **F-24** | Gate A, 80 trials: C0 20/20, T1 20/20, T2 20/20, T3 17/20. Fisher exact p=1.0000, risk difference 0%, bootstrap 95% CI [0%, 0%] | OBSERVED | That the design had **no headroom**. The frozen rule was applied verbatim and the campaign stopped | That the mechanism is false. A ceiling cannot show a reduction | 80 trials, preregistered | `anchor-exp/` | **yes, as NOT REPLICATED** |
| **F-25** | Three A2 calibration attempts produced no valid prospective dataset: retrospective re-scoring, then 7/8 scorer agreement, then 3/4 parser agreement | OBSERVED | That the observation interface failed three times, and candidate A is rejected | **No detection rate.** Each dataset is invalid for a reason that would bias an estimate | three attempts, $2.69 measured | `a2/A2_CLOSURE.md` | **yes, as a methodological result** |
| **F-26** | A structured verdict contract removed parsing ambiguity and introduced compliance as a separate failure: 1 run in 4 detected the contradiction and emitted no verdict block | OBSERVED | That reliable calibration needs both a deterministic observation interface and a protocol whose compliance characteristics are controlled | A compliance rate. n=4 | one pilot | `a2/A2_CLOSURE.md` 3 | **yes**, and it is the most transferable finding in the A2 line |
| **F-27** | A structured claims table changed what a fresh agent retrieved: 51/60 against 0/12, p=1.9e-08; the identical claim in prose 0/4 | OBSERVED | A **retrieval** effect: the agent's search went elsewhere and the fact entered context | Recognition, reasoning or task outcome. Arrival and naming matched exactly, so recognition-given-arrival has an empty cell | two independent sets | `../RESEARCH-CLOSEOUT-2026-09-29.md` 1.5 | **yes, as retrieval only** |

### 2.5 Instrumentation

| ID | Observation | Class | Establishes | Does NOT establish | Replication | Artifact | Survives |
|---|---|---|---|---|---|---|---|
| **F-28** | A `cp`-based restore can leave CPython executing a mutant's cached bytecode, silently voiding a mutation sweep | OBSERVED | That a mutation sweep needs `PYTHONDONTWRITEBYTECODE=1` and an emptied `__pycache__` | Generality beyond CPython | **20 of 20 trials**, reproduced by an independent track | `results/track-d.md` D1 | **yes** |
| **F-29** | Experiment classification: 8 VALID, 12 LIMITED, 3 INVALID, 1 designed but never run. Nothing discarded | RECONSTRUCTED | That the campaign's self-correction found its own invalid work before the audit did | Independence. The classification rests largely on the project's own disclosures | one audit pass | `results/track-d.md` D4 | **yes, with that caveat stated** |
| **F-30** | No patent database was searched at any point in the campaign | NOT_VERIFIED | Nothing | Nothing. It is the absence of a search, not a finding about the field | n/a | `ledger.json` DS-P-01 | **yes** |

### 2.6 Separation of kinds

The campaign died five times where two of these were treated as one.

| | |
|---|---|
| **Observation** | A value read off an artifact. F-01 counts, F-09 hit figures, F-14 balance readings |
| **Interpretation** | What the observation is taken to mean. "The cache is quantised to 128-token blocks" is an interpretation of F-01 |
| **Mechanism** | How the system produces the observation. **Every mechanism in this corpus is NOT_OBSERVED.** M1, M2 and M3 each have two live rival accounts |
| **Hypothesis** | A proposition with a declared falsifier, registered before the data. The block model was one. The weak-anchor harm question still is |
| **Prior-art conclusion** | What published work already covers. Established for six candidates, **not performed** for the one candidate that matters most (section 7) |

---

## 3. DeepSeek cache mechanism: A1 to A10 reconciliation

The six questions this section must answer map to its subsections: concrete
mechanism candidate (3.2), exact conditions tested (3.3), alternatives actually
eliminated (3.4), what remains unresolved (3.5), the prior-art question that
must be answered before an IP gate could be crossed (3.6), and the watchlist
decision (3.7).

### 3.1 The narrowest claim the evidence supports

> On `deepseek-flash` through the DeepSeek API on 2026-09-29, with a fixed
> 3,200-token shared prompt prefix, the presence or absence of the
> `reasoning_effort` key in the request body determined whether a subsequent
> request reused that prefix's cache entry. Toggling the key in either direction
> reduced reported `prompt_cache_hit_tokens` from 3,200 to 0 against same-shape
> controls that reported 3,200, while adding an unrecognised key to the body did
> not.

That is the whole claim. It is an **effect**, stated at the level of the
request body and the reported hit count. It is not a statement about cache-key
construction, and the phrase "part of the cache key" is avoided deliberately,
because the evidence does not reach it.

**Experiment 1 narrows it by one clause.** The determining property is not the
literal key `reasoning_effort`: a body naming `thinking:{"type":"disabled"}`
instead reused the same prefix, in both directions (F-31, F-32). So the
narrowest claim is now about the **reasoning setting the body requests**, not
about which parameter name requests it. Whether that setting acts through the
assembled prompt or through cache identity is still not separated.

### 3.2 Concrete mechanism candidates, after experiment 1

**Updated 2026-09-29 by the authorized M3 discriminator (section 12).** The
candidate set below is the state after that run, not before it. The pre-run
reasoning is preserved in 3.7 because it is what justified the experiment.

**Candidate 1, prompt assembly.** `reasoning_effort` changes the prompt the
server assembles, for example by inserting or removing a thinking directive.
The token stream differs, so the prefix differs, so byte-exact prefix matching
(F-04) correctly reports a miss. **Under this account there is no new mechanism
at all**: the behaviour reduces to prefix matching, which the vendor documents
and which F-04 merely confirms.

**Candidate 2, literal parameter participation. ELIMINATED.** The account was
that `reasoning_effort` is itself a component of cache identity, separate from
the prompt, while unrecognised keys are stripped before keying. **Experiment 1
eliminated it** (F-31, F-32): a prefix populated under `reasoning_effort:"none"`
was reused by a body naming `thinking:{"type":"disabled"}` instead, and the
mirror direction reused it too. If the literal parameter participated, neither
could have hit.

**Candidate 3, normalised reasoning mode. NOT ELIMINATED, and new.** Cache
identity includes the **reasoning mode the request resolves to**, however that
mode is spelled. This was not in the frozen candidate set; it was found by
experiment 1.

The junk-key arm (F-08, replicated as F-34) rules out a fourth account, "the key
is the raw serialised body", which would have predicted a miss and observed a
hit.

**Candidates 1 and 3 are observationally identical across the entire corpus.**
Both predict every observation in A9, A10 and experiment 1. Check candidate 3
against them: in A10, adding `reasoning_effort` to a plain body changes the
resolved mode, so it misses, while the junk key changes no mode, so it hits; in
A9, cross-dialect reuse occurs exactly when the mode matches; in experiment 1,
both disabling spellings resolve to mode "off" and therefore share an entry.

They differ in kind, which is why the distinction is not academic. Candidate 1
is vendor-documented prefix matching and **is not a mechanism**. Candidate 3 is
parameter participation at the level of normalised semantics and **would be**.

### 3.3 Exact conditions tested

| | |
|---|---|
| Model | `deepseek-flash` only. F-10 shows v4-pro does not share this cache; nothing about its geometry was measured |
| Endpoints | `/v1/chat/completions` throughout; `/anthropic/v1/messages` in A9 only |
| Shared prefix | 3,200 tokens in A9 and A10; 2,048 in A1; 35 rungs from roughly 200 to 4,750 tokens in A2 |
| Pricing regime | PEAK |
| Date | 2026-09-29, single session |
| Sample | **One run per arm.** 9 observations in A10 (three triplets), 4 in A9 (two arms of two) |
| Dispatch | Sequential, with the populating call awaited, except in A4 where concurrency was the variable |

### 3.4 Alternative explanations actually eliminated

| Alternative | Status | By what |
|---|---|---|
| Any difference in the request body busts reuse | **ELIMINATED** | F-08, junk key hit 3200 |
| Sampling parameters participate in cache identity | **ELIMINATED** | F-06, six parameters, all 3200 |
| The cross-dialect result was a one-off that should not be carried | **ELIMINATED** | F-07 reproduced both outcomes and attributed the difference to body shape |
| The cache is endpoint-scoped | **ELIMINATED** | F-07, reuse across dialects when the parameter is absent from both |
| The cache is account-global across models | **ELIMINATED** | F-10, zero both directions with working controls |
| Reuse needs a warm-up delay | **ELIMINATED** | F-11, 3200 at +0.0s |
| Matching is semantic or whole-string | **ELIMINATED** | F-04, leading space 0, trailing space 2048, mid-prefix 896 |
| The **literal** `reasoning_effort` key participates in cache identity (candidate 2 as frozen) | **ELIMINATED** | F-31 and F-32, experiment 1. Two different reasoning-parameter spellings reused each other's prefix, both directions |
| A recognised, named inference parameter at its default value busts reuse | **ELIMINATED** | F-33, `response_format:{"type":"text"}` hit 3200 |
| **Prompt assembly explains the whole effect (candidate 1)** | **NOT ELIMINATED** | Experiment 1 is consistent with it but cannot separate it from candidate 3 |
| **Normalised reasoning mode participates in identity (candidate 3)** | **NOT ELIMINATED** | Observationally identical to candidate 1 across A9, A10 and experiment 1 |

### 3.5 What remains unresolved

1. **Candidate 1 against candidate 3.** Now the single most consequential open
   question in the corpus. Candidate 1 against candidate 2 was the previous
   entry here and experiment 1 settled it.
2. **Whether any other named parameter behaves like `reasoning_effort`.**
   Experiment 1 added one data point: `response_format` at its default value
   does not bust reuse (F-33). That is one recognised parameter that behaves
   like the junk key, and it does not characterise the subset.
3. **Replication.** Every A9 and A10 arm is n=1.
4. **Byte-identity of the serialised prompt across dialects** is ASSUMED, and
   F-07 depends on it.
5. **Cache TTL**, **v4-pro geometry**, and **cache-hit pricing against money**
   are all NOT_VERIFIED. Every cache run was below the balance endpoint's $0.01
   resolution, so none of the cache work is reconciled against money.
6. **The block-model implementation.** Partial-block commit and a deliberate
   one-block margin remain indistinguishable (M1).

### 3.6 The prior-art question that must be answered first

> **Is it already disclosed, implemented, patented or otherwise established that
> request-level inference-control parameters such as `reasoning_effort`
> participate in prefix or KV-cache identity or reuse?**

Four sub-questions, each searchable:

1. Do open serving stacks (vLLM, SGLang, TensorRT-LLM, Dynamo) include
   sampling or reasoning-control parameters in the prefix-cache hash key, or
   only token ids and LoRA/multimodal identity?
2. Do servers for hybrid-reasoning models maintain separate cache populations
   per reasoning mode, and is that documented?
3. Do the published cache-auditing works (Auditing Prompt Caching, arXiv
   2502.07776; CacheProbe, arXiv 2605.30613) test parameter sensitivity of cache
   identity, as distinct from prefix sensitivity?
4. Is there patent activity from inference-gateway or model vendors on
   parameter-scoped cache partitioning?

**None of this has been searched.** The existing prior-art document
(`prior-art.md`, 2026-09-28) predates A9 and A10 and covers six other
candidates. It does not address this one at all.

### 3.7 Watchlist decision

Does this qualify as a concrete mechanism beyond generic agent memory or session
continuity, for the internal IP watchlist?

**Yes.** It is not an agent-memory claim and is not adjacent to one. It is a
specific, falsifiable, implementation-level behaviour of a named provider's
inference cache; it was observed symmetrically in both directions; it carries a
discriminating negative control that rules out the obvious deflationary account
of body hashing; and it has a direct economic consequence, namely that mixing
reasoning-on and reasoning-off calls over one shared prefix maintains two cache
populations and that toggling mid-workload costs a full prefix repopulation.

**IP WATCHLIST: TRIGGERED — INVESTIGATE**

That was the decision as of the closeout, and it was conditional: the first
investigative step was to be the $0.05 discriminator, not a prior-art search,
because if candidate 1 held the observation reduced to vendor-documented
matching and the entry would close without a search.

### 3.7a The decision after experiment 1

**The discriminator ran on 2026-09-29 under an authorization for M3 only. The
frozen stop condition fired. The entry is CLOSED and no prior-art search was
performed.** Full record in section 12. The trigger above is preserved exactly
as written because it is the reasoning that justified the experiment, and
because a closeout that edits its own superseded decisions is not auditable.

**So the answer to the watchlist question today is no, with the reason stated
precisely.** Not because the effect went away, and not because it was shown to
be generic. Candidate 2 as framed was eliminated, and what replaced it,
candidate 3, is **observationally identical to the deflationary account**. A
watchlist entry cannot be carried on a candidate that no observation in the
corpus distinguishes from vendor-documented behaviour.

**The evidence that is still missing**, stated as the prompt requires:

> A measurement that separates candidate 1 from candidate 3, by holding the
> server-assembled prompt constant while varying the resolved reasoning mode, or
> by varying the assembled prompt while holding the mode constant. As far as the
> public interface permits, since neither quantity is directly observable.

Only that measurement could re-open the entry. Until it exists, **candidate 3 is
recorded, not pursued.** Whether to open it as a separate entry is Daniel's
decision and is listed in section 12; the default is that it is not.

Nothing in this section is a novelty claim or a patentability claim, and no such
claim is available from this evidence.

---

## 4. Billing reconstruction, final

### 4.1 The result

| Quantity | Value | Class |
|---|---|---|
| Responses | 575, all carrying a usage block | OBSERVED |
| Conservation identities | four, **zero violations**, zero errors | OBSERVED |
| Aggregate identity | `5,885,824 + 402,429 == 6,288,253` (re-verified for this closeout) | OBSERVED |
| Independent reconstruction paths | two, agreeing to nine decimals on the 479-response subset at $0.229946832 | DERIVED |
| Mutation check on the verifier | injecting seven tokens into one response turns it red and names the file | OBSERVED |
| Balance before | $46.01 | OBSERVED |
| Balance after | $45.77 | OBSERVED |
| Reconciliation band | **($0.230, $0.250)**, open both ends | DERIVED |
| **Derived cost** | **$0.248764126** | DERIVED, **lower bound** |
| Verdict | derived lies inside the band: **consistent** | DERIVED |

The band is two cent-resolution readings differenced, so it carries two
quantisation errors and is $0.02 wide. The earlier $0.01-wide interval was an
error that flattered every reconciliation by halving the band a derived figure
had to land in, and it is not resurrected here.

### 4.2 The four quantities kept apart

1. **Derived request-level cost**: $0.248764126, arithmetic over provider usage
   fields and the dated rate table. Never OBSERVED.
2. **Observed balance**: $46.01 and $45.77, read from `/user/balance`, reported
   to the cent. The only OBSERVED cost source in the campaign. The $46.01 anchor
   survives only as a transcribed number with no persisted raw body, which is a
   preservation defect in the evidence the reconciliation depends on.
3. **Settlement lag**: OBSERVED to exist (F-17, the balance moved in a window
   with no billable call) and OBSERVED to converge and stop (13 polls, 24.1
   minutes, no change). Duration bounded from below only.
4. **Unresolved**: 5 calls are NOT_OBSERVED. They are DS-F2b reasoning-parameter
   probes sent by raw curl without `-o`; no body was written and none can be
   recovered. Their output tokens survive only as a transcription (89 tokens,
   an output-side floor of $0.0001068). **Their input tokens are NOT_OBSERVED
   entirely and are not zero.**

### 4.3 The margin, stated because it is thin

Re-derived for this closeout:

```
band upper (open)          $0.250000000
derived (lower bound)      $0.248764126
headroom                   $0.001235874
```

The 5 NOT_OBSERVED calls must together cost strictly less than $0.001236 for the
reconciliation to hold. The maximum observed single-request cost in the corpus
is $0.0056997, which is **4.6x** that headroom on its own.

The reconciliation is therefore consistent **conditional on those five probe
calls having been cheap on the input side**. They are described as trivial
prompts, which is a qualitative statement and not a measurement. This does not
overturn F-15; it states its precision correctly. A systematic error under about
8% would be invisible at this scale in any case, which is the more fundamental
limit.

**How this must be cited.** A **CONDITIONAL RECONCILIATION**, never an
unconditional economic verification. The permitted form names the condition:

> The reconstructed figure is consistent with the observed balance delta,
> conditional on five NOT_OBSERVED probe calls costing less than $0.001236 in
> total on the input side, which was not measured.

Dropping the conditional clause converts a bounded result into a claim the
evidence does not support. The condition is resolvable by measurement, and doing
so is the second authorized step in section 10.

### 4.4 What the reconciliation is

Reconciliation in the strict sense: two independently produced quantities
compared within a stated band. It reaches verification for **resource facts
only**. It does not establish that the provider computes cost by this path, and
it must not be read as an audit.

---

## 5. Track C: reasoning configuration economics

**Classification: clarifying / negative optimization result. Not a product
claim.**

The original headline, that disabling reasoning saves roughly 20x or 6.5x, is
retired. It was a claim about prompts with no document in them.

### 5.1 What was measured

| | |
|---|---|
| Output volume | changed dramatically: about **39x** fewer output tokens with reasoning off on the warm-prefix arms; 19.5x on lookup and 9.6x on aggregation in the task study |
| **Total cost** | changed only **1.48x** in the warm/fan comparison |
| Why | input and cache economics dominate. With a 16,000-token shared corpus in every call, input is 70% of the reasoning-on cost and **98%** of the reasoning-off cost |
| Accuracy | degraded materially by class: LOOKUP 93%, AGGREGATION 44%, TRANSFORMATION 33%, COUNTING 29%, MULTI_STEP 11%, against 100% with reasoning on |
| Failure shape | confidently wrong values, not refusals or near-misses. Counted 67 against a truth of 39; 1 against 14 |

### 5.2 The checker economics

An LLM checker over the same corpus cannot cost less than the input price,
$1.637e-4, even at a perfect cache hit with zero output. Break-even budgets are
$5.49e-5 for LOOKUP and $9.34e-5 for COUNTING. **The LLM checker is 3.0x and
1.8x over budget respectively, so that route is closed on price.**

A program checker costs no API dollars and clears both budgets, and for counting
and deterministic transformation that closes the question for a different
reason: a program that can verify a count is a program that computes the count.
Once it exists, it is the answer and the model call is redundant.

> **Programmatic verification can replace model checking for deterministic
> transformations and counting. The checker economics therefore do not justify
> an LLM checker.**

### 5.3 What this does not support

It does not support a routing product. The COUNTING class clears its own
economic floor by seven points on n=24, and one more failure in that cell would
have put it under. The checker recall is ASSUMED to be 1.0 throughout, and no
checker was measured. A live contradiction remains unreconciled: DS-F1 labels a
counting task "mechanical" and finds reasoning-off **better** (18/20 against
9/20) while DS-RS classifies counting as aggregative and finds it catastrophic
at 29%. Both cannot be right, and deciding it needs a re-run.

---

## 6. A2 and the durable work-state line

| | |
|---|---|
| Candidate A | **REJECTED AS AN A2 CALIBRATION TASK** |
| Mechanism | **NOT ESTABLISHED / NOT REPLICATED** |
| IP gate | **NOT CROSSED** |
| Treatment arms run | **none, at any point, against any A2 candidate** |

That last line is the one most easily lost. A2 was control-arm only by design.
No result in the A2 line can be read as evidence about the treatment it was
being built to test, in either direction.

### 6.1 The methodological result

- **Retrospective re-scoring was invalid.** The scorer was repaired after the
  trajectories existed and every repair was informed by reading them, so the
  rate was a fit to the data it was computed from.
- **Fresh calibration still lacked acceptable scorer agreement**: 7 of 8 against
  manual adjudication, on a disagreement the operator's own prompt had solicited.
- **Structured verdicts introduced compliance as a separate measurement
  variable.** Moving the verdict into a machine-readable contract eliminated
  semantic parsing ambiguity and exposed a different failure: one run in four
  detected the contradiction and emitted no verdict block. Invalid runs are not
  missing at random, so dropping them biases the rate down and counting them as
  non-detections biases it up.
- **Candidate B failed objective scorability.** It is an absence claim, and the
  adversarial review had preregistered withdrawal rather than repair.
- **Candidate C failed the preregistered baseline-headroom requirement.** The
  guard sits one line above a greppable assignment, which is easier than
  candidate A, which was already rejected for insufficient headroom.
- **The existing repository does not contain the required enumeration
  substrate.** Measured, not assumed: `deepseek-cursor-proxy@ea3da01` has exactly
  two function families of N>=4 and no call-site family of N>=5 with a lone
  deviation.
- **Therefore the next experiment, if ever authorized, requires a new
  substrate.** It is not authorized and it is not run here.

The transferable finding, which generalises past this programme:

> Reliable calibration requires both a deterministic observation interface and a
> protocol whose compliance characteristics are themselves controlled. A
> structured output contract converts an ambiguity problem into a compliance
> problem, which is progress only once the compliance rate is measured.

Detail in [a2/A2_CLOSURE.md](a2/A2_CLOSURE.md) and
[a2/A2_GATE_REVIEW.md](a2/A2_GATE_REVIEW.md).

---

## 7. Prior art

### A. Established by the campaign

Searched, with URLs recorded in `deepseek-suite/results/prior-art.md`. Six
candidates, all found anticipated in mechanism.

| Candidate | Finding |
|---|---|
| Warm-then-fan as a scheduler primitive | Anticipated. A named `PromptWarmer` component with the same race diagnosis and the same fix is published, reporting 7% to 85% against this campaign's 0% to 86% |
| Block quantisation with the final block withheld | Anticipated. vLLM documents caching only full blocks, default block 16, with an explicit note that a 17-token message may not cache its last token. Same structure, different block size |
| Byte-exact prefix identity | Confirms vendor documentation. Not a finding |
| Reasoning routing by task class | Anticipated by Route-To-Reason, FrugalGPT and RouterBench. The class-conditional result is the expected result in that literature |
| Runtime cost/correctness optimization on verified outcomes | Fully anticipated. This is the contextual-bandit formulation of routing, restated. Retired |
| Cost reconstruction across usage dialects | Mechanism anticipated by Langfuse, LiteLLM and tokonomics; the inclusive-versus-exclusive dialect split is a documented, actively filed bug class |

Two things survive that pass as contributions of a different kind, neither a
mechanism:

- **A documentation discrepancy.** The vendor publishes a 64-token storage unit;
  `deepseek-flash` measured at 128 with the final block withheld. Worth
  reporting as a discrepancy, not as an undocumented mechanism.
- **A verification that was not found published.** No closed-loop check of a
  reconstructed LLM cost figure against an observed provider balance delta was
  found. Langfuse's own material notes that cloud cost tools reconcile an
  invoice after the fact whereas LLM cost is computed per request, which implies
  the check is usually not performed.

### B. Reconnaissance that is incomplete

- Only four sources were fetched and read directly. Every other URL comes from
  search-result summaries; their exact characterisations are NOT_VERIFIED.
- The appendix and released code of Auditing Prompt Caching (arXiv 2502.07776)
  were not examined, and that work audited DeepSeek specifically.
- No search was performed for warm-then-fan crossover analyses, for cascade
  scorer failure under confident hallucination, for client-side cache-state
  scheduling against third-party APIs, or for published measurements of DeepSeek
  cache granularity.

### C. Claims for which no patent-database search has been performed

**All of them.** No USPTO, EPO or Google Patents search was run at any point in
the campaign. Every statement in the prior-art work concerns published
literature, documentation and shipped software only.

This is the absence of a search. It is evidence about what was done, not about
what exists. **"No prior art exists" is not stated anywhere and is not
supportable from this corpus.** What is supportable is "no matching capability
was found in the sources examined."

### D. The one prior-art question, and why it is not being asked

Stated in full in section 3.6:

> Whether request-level parameters such as `reasoning_effort` participating in
> prefix or KV-cache identity or reuse is already disclosed, implemented,
> patented or otherwise established.

**This question has never been asked, and on current evidence it is not
warranted.** It was raised by the A9 and A10 observations, which the existing
prior-art document predates. The discriminator that section 3.7 placed ahead of
any search then ran and closed the entry (3.7a).

**No prior-art investigation is warranted from the current evidence**, because
the surviving account, candidate 3, is observationally identical to
vendor-documented behaviour. Searching now would be searching against a
candidate no observation distinguishes from documented prefix matching.

The question is retained, unanswered, because it becomes the right question the
moment candidate 3 is established and not before. Establishing it requires the
candidate 1 against candidate 3 measurement, which is **not authorized**.

**This is a closed gate, not a cleared field.** Per part C, no patent database
has ever been searched, for this or any other candidate, and nothing here says
or implies that no prior art exists.

---

## 8. Replay product relevance

Classified rather than forced. Several of these belong nowhere near the product.

| Finding | Classification | Why |
|---|---|---|
| Billing reconstruction and balance reconciliation (F-12 to F-18) | **Core Replay** | This is the product's demonstrated primitive: read usage from artifacts, reconstruct deterministically, reconcile against independently observed state, report an interval |
| Inclusive vs exclusive token dialects, `usage.FromInclusive` | **Core Replay** | Shipped normalisation; the dialect split is a live bug class across the ecosystem |
| Byte-exact prefix identity, leading-space cache destruction (F-04) | **Supporting Replay evidence** | Explains cache-break causes Replay already reports. Confirms vendor documentation rather than extending it |
| Completion-dependent population, concurrent miss (F-05) | **Supporting Replay evidence** | Explains an observable pattern in real traces. The remedy is published practice |
| Block model, 128 with final block withheld (F-01 to F-03) | **Potential future feature** | Would let Replay tell an operator the minimum prefix worth warming. Requires per-provider measurement and goes stale silently |
| Reasoning-setting cache identity (F-07 to F-09, F-31 to F-34) | **Research-only** | Candidate 2 eliminated. It becomes Supporting Replay evidence only if candidate 3 is established, and collapses to F-04 if candidate 1 holds. Neither has happened |
| Reasoning class-conditionality and confident wrongness (F-19, F-20) | **Research-only** | A real measured result about a model, not about Replay. No product surface consumes it |
| Checker economics (F-22, F-23) | **Dead end**, deliberately | It closes a route. Recording a closed route is worth more than the route was |
| Contribution leaderboard and ranking | **Dead end** | Blocked on a missing outcome signal and on k>=20 contributors |
| Distinct-provider-account claims | **Dead end** | Structurally unavailable until an AI provider issues an account-scoped credential |
| A2 calibration substrate | **Research-only** | No treatment arm ever ran |
| Compliance as a measurement variable (F-26) | **Supporting Replay evidence** | Directly applicable to any Replay measurement that parses agent output |

### Does the DeepSeek finding strengthen Replay's broader thesis?

The thesis is that an independent evidence layer can reconstruct relationships
among agent behaviour, request construction, provider runtime behaviour and
observed economic outcome.

**It strengthens two of the three links, and it does not establish the chain.**

- **Request construction to provider runtime behaviour: strengthened, and this is
  the real contribution.** F-04, F-06, F-08 and F-09 together show that a
  property of the request body, visible to an observer who holds the artifact,
  determines a runtime behaviour reported back in the response. The junk-key
  control matters here: it shows the relationship is **selective**, so the
  mapping is not trivially "any change matters" and an evidence layer that
  reconstructs it is doing real work rather than restating a tautology.
- **Provider runtime behaviour to economic outcome: strengthened.** Cache hit and
  miss counts feed the reconstruction that reconciles against the balance.
- **Agent behaviour to request construction: not established here.** Nothing in
  the DeepSeek work involves an agent. Every request was harness-constructed.

Two limits on how far this can be taken. The economic link is established only
for **resource** outcomes; the campaign's central and repeated finding is that
no task-outcome signal exists anywhere in the corpus, so "economic outcome" here
means what was consumed, never whether the work succeeded. And none of the cache
work is reconciled against money at all: every cache run sat below the balance
endpoint's $0.01 resolution.

**No causal claim beyond what was measured.** The observations are associations
between a request-body property and a reported counter, under single-run
conditions on one model in one session.

---

## 9. IP and mechanism gate

### Mechanism candidates

#### M1: block-quantised prefix cache with the final block withheld

| | |
|---|---|
| **Observation** | `cached = 128 * max(0, floor(n/128) - 1)` across 35 rungs; every cached count a multiple of 128; nothing cached below 256 tokens |
| **Hypothesis** | The cache is quantised to 128-token blocks and the trailing partial block is never served |
| **Mechanism** | KV state is stored in fixed blocks and only complete ones are committed, so the minus-one term is an artifact of never having a complete final block. **Rival: a deliberate one-block safety margin produces identical numbers** |
| **Implementation surface** | Client-side prefix sizing and fan-out batching against a measured block size |
| **Evidence** | 17 retrospective plus 18 prospective discriminating rungs; H-B 18/18, H-A 0/18, H-D1 0/18, H-D2 4/22 |
| **Prior art status** | **Anticipated.** vLLM documents full-block-only caching with the partial final block excluded. Boundary padding is proposed practice |
| **Replication status** | Formula replicated prospectively. Mechanism NOT_OBSERVED. One model |
| **IP status** | **NOT A MECHANISM YET.** What remains is a measured value that contradicts the vendor's published 64 |
| **Next required evidence** | Re-run the rung sweep on a second model to test whether 128 is model-specific or the published 64 is stale |

#### M2: completion-dependent cache population

| | |
|---|---|
| **Observation** | Sequential B hit 3,200; concurrent B hit 0, with B dispatched before A returned on wall-clock |
| **Hypothesis** | Reuse requires the populating request to have completed |
| **Mechanism** | The entry is committed on completion. **Rival: commit at first token, indistinguishable at these latencies** |
| **Implementation surface** | Warm-then-fan scheduling |
| **Evidence** | A4 with a wall-clock timeline, plus an independent all-parallel arm at 0% |
| **Prior art status** | **Anticipated.** A named scheduler component with the same race diagnosis and the same fix is published, with comparable numbers |
| **Replication status** | Effect replicated across two independent runs. Commit point NOT_OBSERVED |
| **IP status** | **NOT A MECHANISM YET.** Rediscovery with a measurement attached |
| **Next required evidence** | Stagger B's dispatch across A's response window and locate where the hit appears |

#### M3: the requested reasoning setting affects prefix-cache reuse

| | |
|---|---|
| **Observation** | Adding the key: 0. Removing it: 0. Both against 3,200 controls. An unrecognised junk key: 3,200. Sampling parameters: 3,200. `response_format` at its default: 3,200. Cross-dialect reuse only when the key is absent from both bodies. **Two different reasoning-parameter spellings reused each other's prefix, both directions: 3,200** |
| **Hypothesis** | Reuse is governed by the reasoning setting the request resolves to, not by which parameter name requests it, and not by the presence of arbitrary recognised or unrecognised keys |
| **Mechanism** | **NOT_OBSERVED, two live rivals, and they are observationally identical.** Candidate 1: the setting changes the server-assembled prompt, so the prefix differs and this reduces to vendor-documented byte-exact matching. Candidate 3: the normalised reasoning mode is a component of cache identity. **Candidate 2, literal parameter participation, was ELIMINATED by experiment 1** |
| **Implementation surface** | An evidence layer that predicts, from a saved request body alone, whether a subsequent request will reuse a prefix. Operationally: mixing reasoning-on and reasoning-off calls over one shared prefix maintains two cache populations, and toggling mid-workload costs a full repopulation |
| **Evidence** | 9 A10 observations in three controlled triplets; 4 A9 observations in two arms; **12 experiment-1 observations in four controlled triplets** (F-31 to F-35). The junk-key arm is the load-bearing control, run twice, and rules out raw-body hashing |
| **Prior art status** | **NOT SEARCHED.** The existing prior-art document predates these observations and does not address this candidate. Question stated in 3.6 |
| **Replication status** | **n=1 per arm throughout.** The junk-key control is the one arm run twice (A10 arm C, then F-34) and it reproduced. The spelling-independence result is mirrored across two directions, which is symmetry rather than replication |
| **IP status** | **CLOSED 2026-09-29.** The discriminator ran and the frozen stop condition fired. Candidate 2 as stated is eliminated. See section 12 for the residual, candidate 3 |
| **Next required evidence** | **Separate candidate 1 from candidate 3**: hold the server-assembled prompt constant while varying the resolved reasoning mode, or vary the prompt while holding the mode constant, as far as the public interface permits. Nothing else moves this candidate, because every other observation is predicted equally by both. Replication at n>=3 remains outstanding and is a separate need |

#### M4: provider cost is reconstructible from response usage alone

| | |
|---|---|
| **Observation** | 575 responses, four conservation identities, zero violations, two independent paths agreeing to nine decimals, landing inside the band the balance can distinguish |
| **Hypothesis** | Usage fields are complete and the published rate card is applied as written, so the response body is sufficient evidence of cost |
| **Mechanism** | Deterministic arithmetic over reported usage at each call's own timestamp |
| **Implementation surface** | Shipped. This is Replay's demonstrated primitive |
| **Evidence** | F-12 to F-18, plus a mutation check proving the verifier can fail |
| **Prior art status** | **Mechanism anticipated** by Langfuse, LiteLLM and tokonomics. The closed-loop check against an observed balance delta was **not found published**, which is a verification contribution rather than a mechanism |
| **Replication status** | One reconciliation, one provider. Agreement within a $0.02 band on a $0.25 total is a weak constraint: a systematic error under about 8% would be invisible |
| **IP status** | **NOT A MECHANISM YET** |
| **Next required evidence** | The same reconciliation across two providers with opposite usage dialects, at an order of magnitude more spend so the band is a smaller fraction of the total |

#### M5: weak-anchor harm to contradiction detection

| | |
|---|---|
| **Observation** | Gate A: C0 20/20, T1 20/20, T2 20/20, T3 17/20. Fisher p=1.0000, risk difference 0%, CI [0%, 0%] |
| **Hypothesis** | An executable but nondispositive verification path reduces contradiction detection relative to no path |
| **Mechanism** | None proposed on evidence |
| **Implementation surface** | None |
| **Evidence** | 80 preregistered trials at ceiling; three failed attempts to build a calibrated substrate |
| **Prior art status** | Evidence-conditioned action gating is published with measured results (ECLoop). That establishes relevant prior art and decides nothing about this question |
| **Replication status** | **NOT ESTABLISHED / NOT REPLICATED.** No treatment arm has ever run on a calibrated task |
| **IP status** | **NOT A MECHANISM YET** |
| **Next required evidence** | A calibrated substrate. Section 6 explains why none exists and why the next attempt needs a new repository |

### Gate summary

| Candidate | Verdict |
|---|---|
| M1 block quantisation | NOT A MECHANISM YET |
| M2 completion-dependent population | NOT A MECHANISM YET |
| **M3 requested reasoning setting affects reuse** | **NOT A MECHANISM YET.** Watchlist closed 2026-09-29 by the discriminator; candidate 3 recorded, not pursued |
| M4 cost reconstruction | NOT A MECHANISM YET |
| M5 weak-anchor harm | NOT A MECHANISM YET |

**Zero candidates of five now stand as triggered.** The one trigger was closed
by the experiment written to close it, budgeted at $0.05 and costing $0.0024,
which is the outcome that design predicted as more likely. It closed on a
narrower basis than the frozen rule claimed, and that is recorded in 3.7a
rather than smoothed over. No novelty claim and no patentability claim was ever
made, and none is available from this evidence.

---

## 10. Final status

| Area | Final status | What survives |
|---|---|---|
| DeepSeek cache granularity | **MEASURED, mechanism NOT_OBSERVED, anticipated in prior art** | `cached = 128 * max(0, floor(n/128) - 1)` on `deepseek-flash`, 18/18 at discriminating rungs, and a factor-of-two discrepancy against the vendor's published 64-token unit |
| DeepSeek request-shape mechanism | **EFFECT OBSERVED, watchlist CLOSED, prior art NOT SEARCHED and not needed** | The effect stands. The discriminator eliminated parameter-presence participation: two bodies naming different reasoning parameters reused each other's prefix in both directions. A third account, normalised reasoning mode, is observationally identical to prompt assembly and is recorded, not pursued |
| Billing reconstruction | **CONDITIONALLY CONSISTENT, as a lower bound.** Not an unconditional economic verification | $0.248764126 derived inside ($0.230, $0.250), four conservation identities with zero violations, two paths to nine decimals, a mutation-checked verifier, and a $0.001236 headroom that 5 NOT_OBSERVED calls sit against |
| Track C economics and accuracy | **CLARIFYING / NEGATIVE OPTIMIZATION RESULT** | Output volume moves about 39x while total cost moves 1.48x because input and cache dominate; accuracy degrades by class into confident wrong answers; the LLM checker is closed on price and counting and transformation close by redundancy |
| A2 calibration | **CANDIDATE SET EXHAUSTED WITHOUT A CALIBRATED TASK** | Candidate A rejected; B fails scorability; C fails headroom; the substrate repository is measurably too small. No treatment arm ever ran |
| Durable work-state mechanism | **NOT ESTABLISHED / NOT REPLICATED** | A retrieval effect (51/60 against 0/12) that is not recognition and not task outcome |
| Prior art | **PARTIAL.** Six candidates searched in literature; the seventh was never searched and no longer needs to be unless candidate 3 is established; **zero patent-database searches, ever** | Six candidates found anticipated in mechanism; two non-mechanism contributions survive, a documentation discrepancy and an unpublished verification |
| IP gate | **NOT CROSSED**, and now with no open trigger | The single trigger was closed by the experiment designed to close it. Nothing in the corpus is an established mechanism |
| Replay product relevance | **ONE CORE, TWO SUPPORTING, ONE FUTURE, THREE DEAD ENDS** | Billing reconstruction and dialect normalisation are Core. Cache-break causes and compliance-as-a-variable are Supporting. Block-size advice is Future. Ranking, account claims and the LLM checker are Dead |

### WHAT WE KNOW

1. Replay can reconstruct what an AI workload consumed from response artifacts
   alone and reconcile it against the provider's own balance, with four
   conservation identities holding across 575 responses, two independent paths
   agreeing to nine decimals, and a verifier proven able to fail.
2. `deepseek-flash` serves prefix cache in 128-token blocks and never serves the
   final block, so nothing under 256 tokens caches. This contradicts a published
   vendor figure of 64.
3. Cache matching is byte-exact and prefix-wise. One leading space destroys
   reuse; a trailing change does not.
4. Reuse requires the populating request to have completed, so concurrent cold
   requests do not share a miss.
5. Sampling parameters, unrecognised body keys, and a recognised parameter at
   its default value are all irrelevant to cache identity. **The requested
   reasoning setting is not**, in both directions, and that asymmetry against
   the junk key is the single most specific observation in the corpus. That
   effect is governed by the **setting**, not by the parameter name:
   `reasoning_effort:"none"` and `thinking:{"type":"disabled"}` reused each
   other's prefix in both directions.
6. The cache is model-scoped, not account-global, and not endpoint-scoped.
7. Disabling reasoning moves output volume by roughly an order of magnitude and
   total cost by about half that, because input and cache dominate a
   document-bearing prompt.
8. Reasoning-off degrades into confident wrong answers, not refusals, and the
   degradation is class-conditional.
9. An LLM checker cannot pay for itself at these rates, and for counting and
   deterministic transformation a program checker replaces the model rather than
   rescuing it.
10. Deterministic scoring of unconstrained agent prose fails repeatedly, and
    moving to a structured contract trades ambiguity for compliance.

### WHAT WE DO NOT KNOW

1. **Whether M3 is a mechanism at all**, or just byte-exact prefix matching seen
   through a server-side prompt change. Experiment 1 eliminated the literal
   parameter account and left two candidates that no observation in the corpus
   distinguishes: the assembled prompt, and a normalised reasoning mode in cache
   identity. The first is not a mechanism; the second would be.
2. Whether any A9, A10 or experiment-1 result replicates. Every arm is n=1
   except the junk-key control, which is the one arm run twice and reproduced.
3. Whether any other named inference parameter behaves like a reasoning
   control. One more was tested, `response_format` at its default, and it
   behaved like the junk key.
4. What the 5 NOT_OBSERVED calls cost on the input side, against a $0.001236
   headroom.
5. Whether the reconstruction survives at a scale where the balance band is a
   small fraction of the total, or across a second provider with the opposite
   usage dialect.
6. Cache TTL, `deepseek-v4-pro` cache geometry, and cache-hit pricing against
   money. All three are below or beyond what was instrumented.
7. Whether the DS-F1 and DS-RS counting results can be reconciled. They
   currently contradict.
8. Whether a contradiction-detection task with 40 to 70 percent baseline headroom
   can be built at all, on any substrate.
9. **The entire patent landscape**, for every candidate.
10. Whether an independently produced, machine-checkable task-outcome signal can
    be joined to a Replay resource trace. This is the missing primitive the whole
    campaign converged on from four directions.

### WHAT WOULD CHANGE THE CONCLUSION

- **Reuse tracking the resolved reasoning mode while the assembled prompt is
  held constant** would establish candidate 3, make M3 a parameter-participation
  mechanism, and re-open the watchlist entry closed in 3.7a. The reverse result
  closes it permanently. This replaces the no-op-value test, which ran and
  eliminated the literal-parameter account instead.
- **Any A9, A10 or experiment-1 arm failing to reproduce** would remove the
  effect, not just the mechanism.
- **A single NOT_OBSERVED probe call costing more than about $0.0012** would push
  the derived lower bound past the reconciliation band's upper edge and make the
  reconciliation inconsistent rather than consistent.
- **A second provider reconciling under one normalisation** would turn the
  dialect handling from assumed to falsifiable, and would be the strongest
  available support for the Core Replay position.
- **A `deepseek-v4-pro` rung sweep returning 64** would make the block-size
  discrepancy model-specific and reduce it to a documentation note.
- **A patent search returning parameter-scoped cache partitioning** would close
  M3's IP question on prior art regardless of which rival mechanism holds.
- **An independently produced task-outcome signal** would reopen the entire
  behavioural line, which is currently paused rather than refuted.

### RECOMMENDED NEXT EXPERIMENT

Three, in a **fixed order**, all forward-looking. **None is authorized and none
has been run.** The former experiment 1, the M3 discriminator, was authorized
separately and is complete; its result is section 12 and it is not repeated
here.

Two rules bind the sequence, and they are the reason the order is fixed rather
than advisory:

1. **Do not combine them into one experiment.** Each answers a different
   question and each has its own stop condition. A combined run cannot stop at
   the first answer.
2. **Do not let evidence from one answer a different question.** Calls made for
   a cache question are not billing evidence and the reverse. Reusing a
   trajectory across questions is how the A2 line produced three invalid
   datasets, and experiment 1 held this boundary.

#### 1. Measure the five missing input-side costs

Re-send the five DS-F2b reasoning-parameter probe bodies, preserved by
transcription in `optimization-arms-2026-09-29.md`, this time saving the
response body so the input token counts are OBSERVED rather than NOT_OBSERVED.

- **Why first**: it resolves the section 4.3 condition, which is the only thing
  standing between a conditional reconciliation and an unconditional one, on the
  one result classified **Core Replay**. It is also the only open question in
  the corpus whose answer is a measurement rather than a judgement.
- **Stop condition**: if the five together cost less than $0.001236, the
  condition on F-15 is discharged and the reconciliation stands unconditionally
  for this corpus. If they cost more, the derived lower bound exceeds the band's
  open upper end and **the reconciliation is inconsistent**: record it, do not
  widen the band, do not re-derive the interval to accommodate it.
- **Spend**: under **$0.01**, 5 calls.
- **Boundary**: billing evidence only. These calls toggle reasoning parameters
  and must still not be read as cache evidence, because they are not run against
  a controlled shared prefix.

#### 2. Separate candidate 1 from candidate 3

Hold the server-assembled prompt constant while varying the resolved reasoning
mode, or vary the prompt while holding the mode constant, as far as the public
interface permits.

- **Why second**: it is the **only** measurement that can move M3. Every other
  observation in the corpus is predicted equally well by both candidates, so
  every other cache arm is uninformative about this question. It is also the
  only thing that could re-open the watchlist entry closed in 3.7a.
- **Stop condition**: if reuse tracks the assembled prompt, candidate 1 holds,
  M3 is vendor-documented behaviour and the entry stays closed permanently. If
  reuse tracks the mode independently of the prompt, candidate 3 holds, and
  **that** is the point at which the section 3.6 prior-art search becomes worth
  its cost, as a separate authorization.
- **Spend**: **not estimated.** The design is not fixed, and estimating spend
  for an unfixed design is how a budget becomes a commitment. Neither quantity
  is directly observable through the public interface, so the design may turn
  out to be infeasible, which is itself a reportable outcome.
- **Precondition**: preregister the falsifier before the run, and check whether
  the provider emits a value or omits the field for every quantity the
  preconditions rely on. Experiment 1 was written against a
  `reasoning_tokens == 0` precondition the provider can never satisfy (F-35).

#### 3. Replication at n>=3

Re-run every A9, A10 and experiment-1 arm three times, with each arm's result
written to disk before the next is dispatched.

- **Why third**: the entire cache line is n=1 per arm. The one arm ever run
  twice, the junk-key control, reproduced. Replication is worth more than any
  extension of the line, but it is third because it cannot change a verdict that
  experiments 1 and 2 have not already settled.
- **Stop condition**: any arm that does not reproduce its original outcome means
  that effect is not established. Report and stop; do not extend, and do not run
  a third round to break a tie.
- **Spend**: about **$0.15**, roughly 75 calls.

Deferred, and named so it does not vanish into implied authorization: the
`deepseek-v4-pro` rung sweep that would settle whether the measured 128-token
block is model-specific or the vendor's published 64 is stale, about $0.30. It
is worth doing and it is not next, because it settles a documentation
discrepancy rather than a live candidate.

**Total for steps 1 and 3: about $0.16.** Step 2 is unpriced by design.

## 11. Standing constraints on any continuation

Ratified 2026-09-29 alongside this closeout. These bind until explicitly lifted.

### 11.1 The four-way distinction is the product of this closeout

Its value is that these boundaries are explicit. Collapsing any two of them
recovers exactly the errors the campaign spent itself discovering.

| | Example in this corpus | The error if collapsed |
|---|---|---|
| **Observed effect** | Toggling `reasoning_effort` moved reported reuse from 3,200 to 0 | Reading it as a cache-key finding. Two mechanisms remain live |
| **Derived quantity** | $0.248764126, arithmetic over usage fields and a dated rate table | Reading it as an observed cost. The only OBSERVED cost source is the balance endpoint |
| **Conditional reconciliation** | Derived lies inside ($0.230, $0.250), conditional on five unmeasured probe calls | Reading it as unconditional economic verification. See 4.3 |
| **Established mechanism** | **None in this corpus.** M1, M2 and M3 each carry two live rival accounts | Reading a supported effect as an explained one |

### 11.2 What must not happen

- **Do not reopen frozen evidence.** Every manifest in 1.3 verifies. A frozen
  artifact is corrected by an amendment that sits beside it, never by editing it.
- **Do not resurrect rejected candidates.** A2 candidate A is rejected, B fails
  objective scorability, C fails baseline headroom. The prompt-optimization and
  compaction hypotheses are REFUTED and stay refuted.
- **Do not imply a treatment arm ran in A2.** None ever did, against any
  candidate.
- **Do not convert Track C into a positive claim.** It is a clarifying negative
  optimization result: it closes the LLM-checker route and says what does not
  work.
- **Do not state or imply novelty, patentability or freedom to operate.** M3 is
  a watchlist trigger, which is an instruction to investigate. No patent database
  has been searched, for any candidate.
- **Do not say "no prior art exists."** The supportable form is "no matching
  capability was found in the sources examined."
- **Do not broaden the research.** Continuation means the fixed sequence in
  section 10 and nothing adjacent to it.

### 11.3 Authorization state

| | |
|---|---|
| Closeout | **RATIFIED**, read-only, evidence baseline auditable |
| Experiment 1, M3 discriminator | **AUTHORIZED AND RUN 2026-09-29. Complete.** See section 12 |
| Next 1, five missing input-side costs | specified, stop condition fixed, **NOT AUTHORIZED** |
| Next 2, separate candidate 1 from candidate 3 | design not fixed, **NOT AUTHORIZED** |
| Next 3, replication at n>=3 | specified, stop condition fixed, **NOT AUTHORIZED** |
| `deepseek-v4-pro` rung sweep | deferred, **NOT AUTHORIZED** |
| Prior-art search | gated behind Next 2 returning candidate 3, **NOT AUTHORIZED** |
| Opening a watchlist entry for candidate 3 | Daniel's decision, **NOT TAKEN** |
| Everything else | **NOT AUTHORIZED** |

Experiment 1 was authorized for M3 only and is complete. **No further API spend
is authorized.** Each step requires its own authorization, and authorization of
one is not authorization of the next. Nothing proceeded to experiment 2 or 3.

---

## 12. Experiment 1 result: the M3 discriminator

**Run 2026-09-29 under an authorization for M3 only. 12 calls, `deepseek-flash`,
DERIVED spend $0.0023568 against a $0.05 ceiling, zero errors.** Preregistration
hashed `dcea16afe273` before the first call; full record in
`$CLAUDE_JOB_DIR/tmp/m3/M3_RESULT.md`, manifest `M3_LOCK.sha256`, 28/28 verifying,
raw and request artifacts set read-only.

### What ran

Four triplets, each populate then same-shape control then test arm, distinct
filler seed, 3,200-token shared prefix. All 12 request bodies were verified on
the wire from saved request artifacts.

| Triplet | Populate | control | TEST arm | result |
|---|---|---:|---|---:|
| T1 | `reasoning_effort: "none"` | 3200 | `thinking: {"type":"disabled"}` | **3200 HIT** |
| T2 | `thinking: {"type":"disabled"}` | 3200 | `reasoning_effort: "none"` | **3200 HIT** |
| T3 | plain | 3200 | `response_format: {"type":"text"}` | **3200 HIT** |
| T4 | plain | 3200 | unrecognised junk key | **3200 HIT** |

T4 reproduces A10 arm C, so the instrument behaves as it did when the original
observation was made. Every cold call populated from 0. Every control hit.

### The stop condition fired

Reuse survived the no-op value, in both directions. Applying the frozen rule
without modification:

> **Candidate 1 holds. IP WATCHLIST: CLOSED. No prior-art search was performed.**

Candidate 2 **as the frozen design stated it**, the literal parameter's presence
participating in cache identity, is **eliminated**: two bodies naming different
reasoning parameters reused each other's prefix in both directions.

### Two things the run did not establish

**A defect in my own preregistration.** It required `reasoning_tokens == 0` on
the populate calls. DeepSeek **omits** that field when reasoning is off rather
than reporting zero, so as worded the precondition was unsatisfiable by a call
behaving exactly as intended. Zero is not absent. The substantive question was
answered yes by three independent signals: the field absent on all six
reasoning-off calls against 8 on all six reasoning-on calls, warm output of 1
token (`ok`) against 8 exhausted, and `policy.check_reasoning_honoured` passing
on every ANSWERED call. T1 and T2 stand, and the wording defect is recorded
rather than reinterpreted.

**A third candidate the frozen design did not contain.**

> **Candidate 3: a normalised reasoning MODE participates in cache identity,
> however that mode is spelled in the request body.**

Candidate 3 predicts every one of the twelve observations here, and every
observation in A9 and A10, exactly as well as candidate 1 does. The two are
**observationally identical across the whole corpus**. They differ in kind:
candidate 1 is vendor-documented prefix matching and is not a mechanism;
candidate 3 is parameter participation at the level of normalised semantics and
would be.

So the frozen rule's **action** fired cleanly, and its **stated reason**, that
M3 "collapses into documented byte-exact matching", is stronger than the
evidence supports. Both facts are recorded. The action stands, because a rule
written in advance that fires is honoured, not relitigated after seeing the
result.

### Open for Daniel, not acted on

Three options. **Option 1 is the default and is what stands unless Daniel says
otherwise.**

1. **Accept the closure as frozen.** The rule fired; honouring it is the
   discipline this closeout was built on.
2. **Close M3 as specified and open a separate watchlist entry for candidate 3**,
   which is a new candidate found by an authorized run rather than a reopening
   of M3.
3. **Authorize experiment 3** to separate candidates 1 and 3 before deciding.

---

## Closing

The campaign produced one demonstrated primitive, one measured discrepancy
against vendor documentation, one specific unexplained effect, three dead ends
recorded as dead, and a great deal of evidence that the instruments needed
checking before the subjects did.

> **Replay can verify what was consumed. It cannot yet verify what was
> accomplished.** That boundary was reached from four directions and is not
> softened here.

The one candidate that earns further work is M3, and the first thing to do with
it is try to kill it for five cents.
