# DeepSeek research campaign: final evidence package

**Frozen at** `d1810a7` (campaign) · this report adds the package only.
**Spend** $0.24 OBSERVED against a $3.00 ceiling. **Corpus** 575 request/response
pairs. **Status** closed.

Evidence classes are used strictly: OBSERVED, DERIVED, SUPPORTED_HYPOTHESIS,
NOT_OBSERVED, REFUTED, ASSUMED, NOT_VERIFIED. Nothing is promoted between them
anywhere in this document.

---

## 1. Executive summary

**What was actually measured.** Provider-reported token counts on 575 responses,
account balance readings before and after, and pass/fail against machine-computed
ground truth on two task classes. Everything else in this report is derived from
those.

**What survived.** A count relation for the prefix cache that predicted 18 of 18
discriminating rungs chosen in advance. Block-granular, prefix-wise matching.
Non-reuse when a request is issued before the populating request returns.
Model-scoped behaviour. `reasoning_effort` participating in cache identity while
an unrecognised key does not. A cost reconstruction that lands inside the
interval the balance can distinguish.

**What was refuted.** Per-request cent truncation. The operator's `128 *
floor(n/128)` form. The vendor's 2024 64-token unit as a predictor of current
flash behaviour. A universal cross-dialect cache claim, in both its earlier
directions.

**What is merely derived.** Every dollar figure. `$0.248764126` is arithmetic
over observed tokens and a published rate card, not a provider-reported charge.
Its agreement with the balance is CONSISTENCY, not proof that the provider
computes cost by this path.

**What matters for Replay.** One thing was demonstrated end to end and not found
published: reconstruct provider cost from response-level evidence, then reconcile
it against observed provider state. The cache and routing mechanisms this
campaign measured are, on the prior-art evidence, standard practice.

---

## 2. Campaign scope and budget

| | |
| --- | --- |
| Ceiling | $3.00, enforced in integer nano-USD |
| Spend | **$0.24 OBSERVED** (balance $46.01 to $45.77) |
| Reconstructed | **$0.248764126 DERIVED** |
| Billable calls | 575 saved, 5 NOT_OBSERVED |
| Models | `deepseek-flash` throughout; `deepseek-v4-pro` in A8 only |
| Endpoints | `/v1/chat/completions`, `/anthropic/v1/messages` |
| Agents | 4 on free tracks, all hard-scoped to zero spend |
| Replay baseline | $12,860.68 across 949 lanes, 4% re-billed, median task $775.87 |

Pricing provenance is in `pricing.md`. The holiday exclusion in the peak window
is ASSUMED from documentation and NOT_OBSERVED here.

---

## 3. Experimental design

Preregistration in `PLAN.md`, written before Track A executed, with four rival
formulas named in advance and falsifiers declared. Rungs were chosen to separate
the rivals rather than to confirm the incumbent. Every prediction was printed
before its observing call.

Token counts were never estimated from character counts. The warm prompt is a
strict extension of the cold prompt, so the shared prefix is exactly the cold
call's own provider-reported `prompt_tokens`. An earlier campaign mis-scored a
rung precisely because it estimated, and blamed the model for an instrument
error.

---

## 4. Cache findings

### 4.1 The count relation

Observed behaviour is consistent with a **128-token block-granular cache whose
effective cached prefix is one block behind the available shared prefix**:

```text
cached = 128 * max(0, floor(n / 128) - 1)
```

At rungs chosen to discriminate:

| hypothesis | fit |
| --- | ---: |
| **H-B** `128*(floor(n/128)-1)` | **18/18** |
| H-A `128*floor(n/128)` | 0/18 |
| H-D1 `64*floor(n/64)` | 0/18 |
| H-D2 `64*(floor(n/64)-1)` | 4/22 |

**BOUND ADDED 2026-09-29 by the prompt-optimization campaign.** All 35 rungs
above were sequential cold/warm pairs. Under **concurrent** load a cached count
of **13,563** was observed once in 251 trials, which is not a multiple of 128.
Its sibling trial, same condition and task, read 13,440 with an identical total
input of 13,697. The quantization claim is therefore scoped to the sequential
regime it was measured in; under concurrency a non-multiple occurs rarely. See
`experiment/prompt-opt/derived/ledger.json` PO-06.

**This establishes a COUNT RELATION, not an implementation.** A server that
commits only complete blocks and a server that holds a deliberate one-block
margin produce identical counts. Distinguishing them needs control over
tokenisation this campaign does not have. See `mechanisms.md` M1.

### 4.2 Prefix matching

| variant | cached |
| --- | ---: |
| identical (positive control) | 2,048 |
| trailing space appended | 2,048 |
| **leading space** | **0** |
| capitalisation changed at start | 0 |
| punctuation inserted at start | 0 |
| **one character changed mid-prefix** | **896 = 7 x 128** |

Matching is prefix-wise and block-granular, not whole-string equality. A change
late in the prefix still serves the blocks before it.

### 4.3 Population timing

Sequential B hit 3,200. Concurrent A and B both hit 0, with `B dispatched before
A returned` established from a wall-clock timeline rather than code ordering.

**NOT_OBSERVED: the exact cache-population commit point.** Completion and
first-token are indistinguishable at the observed latencies.

### 4.4 Cross-dialect behaviour, and the correction

| condition | cross-dialect reuse |
| --- | --- |
| `reasoning_effort` absent from both bodies | **3,200 observed** |
| `reasoning_effort` present in both bodies | **0** |
| unrecognised junk key added | cache still hit 3,200 |
| `reasoning_effort` toggled within one dialect | 0, in both directions |

So: **`reasoning_effort` participates in cache identity under the tested
conditions; an arbitrary request-body key does not.** The cache key is therefore
not the raw request body.

**NOT_OBSERVED:** whether this works through cache-key construction or through
another request-path transformation such as prompt assembly.

**Methodological lesson, preserved because it is part of the result.** A
replication disagreed with an earlier observation. The correct resolution came
from diffing the saved raw request bodies, which showed the earlier run carried
no `reasoning_effort` and the replication did. Choosing the newer, better
controlled result would have produced a true conclusion by luck and lost the
mechanism entirely.

### 4.5 Scope

Neither direction crossed between `deepseek-flash` and `deepseek-v4-pro`, with
working controls on both. Model-scoped behaviour was **observed for these two
models on one endpoint**. It is not generalised to all DeepSeek models or
endpoints. Block size on v4-pro is future work, not a current conclusion.

---

## 5. Billing reconstruction

### 5.1 What holds

575 responses, all carrying a usage block. Four conservation identities, **zero
violations**. Two independent reconstruction paths agreed to nine decimals. The
verifier is mutation-checked: injecting seven tokens into one response turns it
red and names the file.

### 5.2 The numbers, by class

| quantity | value | class |
| --- | ---: | --- |
| balance before | $46.01 | OBSERVED |
| balance after | $45.77 | OBSERVED |
| difference interval | **($0.230, $0.250)** | DERIVED, open both ends |
| reconstructed cost | **$0.248764126** | DERIVED |
| agreement | inside the interval | **SUPPORTED / CONSISTENT** |

Agreement is **not** proof that the provider computes cost by this path. At this
scale a systematic error under roughly 8% would be invisible.

The interval is open at both ends and four half-resolutions wide because a
difference of two cent-resolution readings carries two independent quantisation
errors.

### 5.3 Cent truncation: REFUTED

Maximum per-request cost is **$0.0056997**; **0 of 479** requests reach one cent.
Under per-request truncation every call bills zero and the session total would be
zero. The balance fell $0.24. Refuted from existing evidence, no spend.

### 5.4 Settlement lag

The balance changed during a window containing no billable call, and later held
at $45.78 across 13 polls over 24.1 minutes with zero inference. Settlement lag
explains the earlier apparent discrepancy. **This is not stated as a universal
provider billing rule**; it is one observed window and one observed convergence.

### 5.5 Evidence gap, not hidden

**The $46.01 anchor has no persisted raw body.** It was read inline. Later
readings persist their raw bodies. The reconstruction is also a **LOWER BOUND**:
five calls used raw curl without saving a response and are NOT_OBSERVED.

---

## 6. Optimization findings

### 6.1 Cost

With a warm shared prefix:

| arm | output tokens | total cost |
| --- | ---: | ---: |
| warm-then-fan, reasoning ON | 936 | $0.00313 |
| reasoning OFF | 24 | $0.00211 |
| ratio | **~39x** | **~1.48x** |

**Input and cache costs dominate once the prefix is warm.** The output-token
saving is real and mostly does not reach the bill.

### 6.2 Accuracy, with measured and predicted kept apart

**MEASURED, live, ground truth computed from the corpus:**

| class | reasoning ON | reasoning OFF | n |
| --- | ---: | ---: | ---: |
| lookup / mechanical | 18/18 (100%) | **17/18 (94%)** | 18 |
| aggregative / counting | 24/24 (100%) | **7/24 (29%)** | 24 |

**REGISTERED PREDICTIONS, NOT_OBSERVED.** Track C designed and preregistered
five classes and their expected cells. Three were never executed:

| class | predicted OFF | status |
| --- | ---: | --- |
| AGGREGATION | 44% (22-67) | **NOT_OBSERVED** |
| TRANSFORMATION | 33% (11-56) | **NOT_OBSERVED** |
| MULTI_STEP | 11% (0-33) | **NOT_OBSERVED** |

These are not measurements and are not reported as such. The task set, the
checkers and the predictions are committed, so the experiment is runnable.

**Conclusion, bounded.** The tested reasoning-off intervention produced modest
cost savings while introducing substantial task-dependent accuracy degradation on
the classes measured. This is not a universal recommendation and these are not
universal model accuracy rates.

Failures were confident wrong values, not refusals: asked to count lines
beginning with `func` plus a space it answered 67 where the truth was 39, and
counted 1 type declaration where there were 14. Nothing in the response marks it
wrong.

### 6.3 Checker economics

Break-even checker cost is $5.5e-5 (lookup) and $9.3e-5 (counting). The cheapest
conceivable LLM checker, with a perfect cache hit and zero output, is $1.64e-4.
A programmatic checker is therefore preferable **where deterministic verification
exists**.

Stronger, and bounded: **for counting and deterministic transformation, a
verifier that independently recomputes the answer may simply replace the model
computation.** This does not apply to all task classes.

---

## 7. Instrumentation and validity

`__pycache__` can leave CPython executing a mutant's bytecode after a `cp`
restore, silently voiding a mutation sweep. Reproduced **20 of 20 trials**, both
directions, in an isolated tree. The permanent guard is two lines in
`scripts/harness-test`, and the detection is the restore-and-run control.

Track D classification of prior experiments: **8 VALID, 12 LIMITED, 3 INVALID**,
one designed-never-run. Invalid experiments are preserved in `invalid/register.md`,
not deleted.

---

## 8. Methodological corrections

Every one of these changed a conclusion or a stated precision. They are results,
not footnotes.

1. **Cent truncation refuted.** The hypothesis was mine; the corpus falsified it
   without spending.
2. **The balance interval was understated by half.** I propagated one reading's
   quantisation error where a difference carries two. The band is $0.02, open at
   both ends. The old band flattered every reconciliation.
3. **Reasoning-off savings were overstated.** The "6.5x" headline bundled three
   interventions against a naive baseline. Decomposed, ordering plus warming is
   4.4x and reasoning-off adds 1.48x.
4. **The cross-dialect cache result was initially misinterpreted**, in both
   directions: first as universal reuse from an n=1 observation, then as no reuse
   from a better-controlled refutation.
5. **That contradiction was resolved by comparing raw request bodies**, not by
   preferring the newer experiment.
6. **`session.Run.fan` does not propagate `extra_kwargs`** although `ask` does,
   so Track C could only force its arms by passing a `task_class` it knew to be
   false. Recorded OPEN; the affected arms are the NOT_OBSERVED cells in 6.2.
7. **`git add -A` in a shared worktree swept up another agent's in-flight files**
   mid-edit. Per-path staging only, from here.
8. **A regex intended to fix one Markdown code span matched across adjacent
   spans** and corrupted 70 sites in an uncommitted agent report. The
   substitution was exactly invertible, was reversed, and the single genuine case
   was then fixed by hand. The file had no commit to fall back to.
9. **DS-F5's single-batch reconciliation is weakened**, not withdrawn. Given
   settlement lag, the $0.04 fall across that window cannot be cleanly attributed
   to that batch. The full-campaign reconstruction supersedes it.
10. **A6's parameter-invariance result does not cover `reasoning_effort`**,
    because all six arms shared one task class and held it constant.

---

## 9. Prior art

Reconnaissance only; not exhaustive.

- **vLLM** documents caching only full blocks, with an explicit note that a
  17-token message may not cache its last token. Structurally the same as the
  measured relation, at a different block size.
- The **concurrent cache-miss race** and the warm-then-fan remedy are published,
  with the fix stated as a synchronous call before dispatching the parallel batch.
- **Langfuse** already normalises inclusive and exclusive usage dialects at
  ingestion.
- **Verified-outcome routing** resembles a contextual-bandit formulation.
- **DeepSeek's 2024-08-02 documentation** states a 64-token storage unit. It
  describes V2/MLA at $0.014 per million. It is **not described as wrong**; it
  concerns a different model generation and pricing regime, and it does not
  predict currently measured flash behaviour.

**PATENT PRIOR ART SEARCH = NOT_VERIFIED.** No patent database was queried. No
novelty, patentability, infringement or freedom-to-operate conclusion is
available or implied.

---

## 10. Replay implications

### Directly demonstrated

- Response-level evidence reconstructs usage and cost relationships for this
  provider, with conservation identities holding across 575 responses.
- Closed-loop reconciliation against observed provider state was demonstrated
  **under the tested conditions**.
- Cache behaviour materially affects both cost reconstruction and optimisation.
- Reasoning mode changes both output volume and cache identity under the tested
  conditions.

### Supported product hypotheses

- A capability: evidence-backed provider-spend reconstruction and reconciliation.
- An architecture: collect response-level evidence, reconstruct usage and cost,
  reconcile against observed provider state, surface the discrepancy.

### Not demonstrated

Universal provider billing accuracy. Universal cross-endpoint cache behaviour.
Universal DeepSeek cache rules. Provider-independent cost reconstruction.
Production-scale reconciliation. Patentable novelty. General AI-agent
optimisation superiority.

---

## 11. Withdrawn and retired claims

Superseded, not erased. Historical artifacts keep their original text.

| retired claim | why | superseded by |
| --- | --- | --- |
| Per-request cent truncation explains the balance gap | max request $0.0056997, 0 of 479 reach a cent | DS-B-05 |
| Reasoning-off is a 6.5x cost lever | bundled three interventions against a naive baseline | DS-O-01 |
| Reasoning-off is a ~20x cost lever | that is the output-token ratio, not the cost ratio | DS-O-01 |
| The cache is shared across dialects (universal) | conditional on `reasoning_effort` absence | DS-C-10 |
| The cache is never shared across dialects | same | DS-C-10 |
| The 2024 64-token figure describes current flash behaviour | 0/18 and 4/22 at discriminating rungs | DS-C-03 |
| The vendor documents no cache interval | it does, on a 2024 page I had not read | DS-C-03 |
| This campaign discovered the cache mechanisms | vLLM and published scheduling work cover them | section 9 |
| Any patentability implication from those mechanisms | no search performed | DS-P-01 |
| Session cost does not reconcile (factor 1.9) | settlement lag, resolved | DS-B-04 |

---

## 12. Evidence gaps

No experiment is launched to fill these. None blocks a claim in this report.

| gap | why it matters | blocks a claim? | next-experiment cost |
| --- | --- | --- | --- |
| Exact cache commit point | separates completion from first-token | no, DS-C-06 records it NOT_OBSERVED | ~$0.01 |
| `reasoning_effort` mechanism | cache-key vs request-path transformation | no, DS-C-09 is scoped to the effect | ~$0.005 |
| v4-pro block size | tests whether 128 is model or infrastructure | no | ~$0.05 |
| Cache TTL | the only vendor claim still unmeasured | no | time, not money |
| Larger-scale reconciliation | a $0.02 band hides systematic error under ~8% | no, DS-B-04 states the limit | ~$2.00 |
| Endpoint and model generality | all cache geometry is flash on one endpoint | no, scope is stated | unknown |
| Production-scale reliability | nothing here ran at production volume | no, listed as not demonstrated | unknown |
| Patent prior-art search | not performed | no, DS-P-01 is NOT_VERIFIED | n/a |
| The three NOT_OBSERVED accuracy cells | Track C preregistered, never ran | no, they are not reported as measured | ~$0.05 |

---

## 13. Final claims matrix

Only claims that survived the audit. Full records in `ledger.json`.

| Claim | Evidence class | Evidence | Scope | Status |
| --- | --- | --- | --- | --- |
| Cached counts follow `128*max(0,floor(n/128)-1)` | OBSERVED counts, SUPPORTED_HYPOTHESIS relation | 35 rungs | flash | holds |
| H-B 18/18, H-A 0/18, H-D1 0/18, H-D2 4/22 | OBSERVED | prospective rungs | flash | holds |
| The 2024 64-token figure does not predict flash | REFUTED as predictor | two rung sets | flash, 2026-09 | holds |
| Matching is prefix-wise and block-granular | OBSERVED | A1, 6 arms + control | flash | holds |
| A request issued before population returned did not reuse | OBSERVED | A4 + arms run | flash | holds |
| Exact population commit point | NOT_OBSERVED | n/a | n/a | open |
| Six sampling parameters do not affect cache identity | OBSERVED | A6 | flash, chat | holds |
| An unrecognised body key does not affect cache identity | OBSERVED | A10 arm C | flash, chat | holds |
| `reasoning_effort` participates in cache identity | OBSERVED | A10 arms A and B | flash | holds |
| `reasoning_effort` mechanism | NOT_OBSERVED | n/a | n/a | open |
| Cross-dialect reuse is conditional on its absence | OBSERVED | A9 two arms | flash | holds |
| No cross-model reuse, flash and v4-pro | OBSERVED | A8 both directions | 2 models, 1 endpoint | holds |
| 575 responses, 4 identities, 0 violations | OBSERVED | usage-extract.json | campaign | holds |
| Reconstructed cost $0.248764126 | DERIVED | extract x pricing | campaign, LOWER BOUND | holds |
| Balance moved $46.01 to $45.77 | OBSERVED | raw bodies from the second read on | campaign | holds |
| Reconstruction lies inside ($0.230, $0.250) | SUPPORTED_HYPOTHESIS | consistency only | campaign | holds |
| Cent truncation cannot explain the movement | REFUTED | corpus arithmetic | campaign | holds |
| Balance changed with no billable call in window | OBSERVED | poll + corpus mtimes | one window | holds |
| Balance stopped moving, 13 polls over 24.1 min | OBSERVED | balance_poll.jsonl | one window | holds |
| Reasoning-off: ~39x output, ~1.48x cost | OBSERVED tokens, DERIVED ratio | arms A3 vs A4 | one shape | holds |
| Lookup OFF 17/18; aggregative OFF 7/24 | OBSERVED | reasoning-scope | registered task set | holds |
| Three further accuracy classes | NOT_OBSERVED | preregistered only | n/a | open |
| Programmatic checker preferred where deterministic | DERIVED | break-even analysis | 2 classes | holds |
| `cp` restore can void a mutation sweep | OBSERVED | 20/20 trials | CPython | holds |
| Patent prior art | NOT_VERIFIED | none | n/a | open |

---

## 14. Recommended next actions

1. **Nothing, for the research.** The campaign is closed. The gaps in section 12
   are future work and none blocks a claim here.
2. **Fix `session.Run.fan` to propagate `extra_kwargs`** before any further Track
   C work. The three NOT_OBSERVED cells cannot be measured faithfully until it is.
3. **Persist raw bodies for every balance read.** The $46.01 anchor lacks one.
4. **Rotate the credential.** Outstanding since before this campaign; it is a
   human action in the provider console.
5. **If one experiment is run, run the v4-pro rung sweep** at roughly $0.05. It is
   the only gap that tests whether a headline result is model-specific or
   infrastructural.

---

## Artifact manifest

| path | purpose |
| --- | --- |
| `README.md` | evidence classes and frozen campaign state |
| `PLAN.md` | preregistration, written before Track A ran |
| `pricing.md` | rate table with provenance; holiday rule marked ASSUMED |
| `FINAL-REPORT.md` | this document |
| `ledger.json` | machine-readable claim ledger, 23 records |
| `claims.md` | per-claim matrix with confounds and falsifiers |
| `mechanisms.md` | observation, mechanism, alternative, discriminating experiment |
| `raw/usage-extract.json` | **primary evidence.** 575 provider usage blocks, verbatim, with a sha256 over the source corpus |
| `raw/track-a.json`, `track-a2.json`, `track-a-scope.json` | A1, A2, A4, A6, A7, A8 records |
| `raw/a9-crossdialect.json`, `a10-cachekey.json` | the correction and its resolution |
| `raw/balance-pre-trackA.json`, `balance-post-trackA.json` | balance readings with raw bodies |
| `derived/final-reconstruction.json` | the reconstruction |
| `derived/track-b.json`, `track-d.json` | agent track outputs |
| `results/*.md` | per-track reports, including `DEFECT-interval-width.md` |
| `invalid/register.md` | experiments whose instrumentation failed, preserved |
| `../harness/reconstruct.py` | deterministic re-runnable reconstruction, no API call |

The 43 MB raw corpus lives outside the repository in an ephemeral job directory
and will not survive it. `raw/usage-extract.json` carries every field the
reconstruction uses, verbatim, and reproduces the same figure to nine decimals.
