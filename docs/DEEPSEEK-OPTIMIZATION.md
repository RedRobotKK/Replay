# DeepSeek: measured controls and optimization history

**The DeepSeek research campaign is CLOSED.** The authoritative operating
document is `DEEPSEEK-OPERATING-CONTRACT.md`. This page records what the harness
enforces today and what the experiments actually observed, with the scope of
each finding attached.

An earlier version of this page opened with "four measured interventions"
applying to give "6.5x cheaper and 6.9x faster". That headline is **superseded**;
see [Superseded claims](#superseded-claims).

## Current status

| | |
| --- | --- |
| research campaign | **CLOSED** |
| prompt optimization | **NULL under the tested design** |
| new API spend | **not authorised** |
| what is implemented | request-construction controls in `experiment/harness/policy.py` and `session.py` |
| what is not implemented | any prompt optimizer, rewriter or compressor |

The controls below are **implementation behaviour**, enforced by tests. They are
not a claim that applying them saves money on your workload. Tests establish
what the code does; only an experiment establishes economic benefit, and the
scope of the experiments that exist is stated under each finding.

## Enforced controls

What `make harness-test` guarantees, inside `make ci`.

| control | what it enforces | where |
| --- | --- | --- |
| stable-prefix assembly | the shared block precedes the variable part, and a prefix whose leading bytes vary is refused | `policy.assemble` |
| task-class reasoning policy | a class decides the reasoning setting; **an unclassified task gets reasoning ON** | `policy.config_for` |
| reasoning invariant | a request that disabled reasoning and gets reasoning tokens back **raises** | `policy.check_reasoning_honoured` |
| warm-or-fan decision | warming is deliberate and is skipped below the measured floor | `policy.should_warm` |
| concurrency bound | worker count is validated against the published limit at construction | `session.Run.__init__` |
| budget gates dispatch | spend is reserved before a request is sent, never accounted after | `fanout.Budget`, integer nano-USD |
| outcome separation | `ANSWERED`, `TRUNCATED`, `REFUSED_BUDGET` and an undecided checker stay four distinct facts | `session.Run` |
| checker isolation | a checker that raises leaves the answer undecided, never scored wrong | `session.Run.ask` |
| kwargs propagation | `extra_kwargs` reaches every call in a fan-out, warm and fanned alike | `session.Run.fan` |
| identity capture | requested model, returned model, system fingerprint, finish reason, reasoning tokens | `adapter.Measurement` |
| single entry point | a module that builds a provider adapter directly fails a test | `TestNoScriptBypassesTheRunner` |

The **fail-safe direction is deliberate**. An unclassified task takes the
expensive, correct setting rather than the cheap one, because reasoning-off did
not degrade gracefully where it was measured: it returned confident wrong values
with nothing in the response marking them wrong.

`system_fingerprint` identifies a **serving configuration, not model weights**.
It supports no claim about routing.

## Measured findings

Each with its scope. None is a provider contract.

### Prefix ordering

Placing the variable part after the shared block measured **4.6x cheaper at no
latency cost** on one benchmark. Cache matching is byte-exact: a single leading
space dropped the hit rate to zero, while `temperature` and `max_tokens` changes
did not affect it. Matching is prefix-wise and block-granular, so a change late
in a prefix still served the blocks before it.

*Scope: `deepseek-flash`, `/v1/chat/completions`, 12 questions over one ~3,400
token document, single run.*

### Warm before fan-out

Fanning out cold measured **11.5x faster at 0.99x the cost** with a 0% hit rate,
because every request began before any had completed. Warming one request first
then fanning measured **4.4x cheaper and 5.1x faster** than the cold baseline.

*Scope: as above. Warming is not universally cheaper; it is a tested behaviour
on a workload with a substantial shared prefix.*

### Reasoning configuration, class-conditional

| task class | reasoning ON | reasoning OFF |
| --- | ---: | ---: |
| lookup | 18/18 | **17/18**, far fewer output tokens |
| aggregative / counting | 24/24 | **7/24** |

*Scope: `deepseek-flash`, one corpus, n=18 and n=24, one run. Three further task
classes were preregistered and **never executed**; they are NOT_OBSERVED and are
not reported here as results.*

Once a shared prefix is cached, input dominates: the same intervention cut output
tokens about 39x and total cost only about **1.48x**.

### Cache geometry

Cached counts followed `128 * max(0, floor(n/128) - 1)` across 35 rungs, so a
shared prefix below 256 tokens cached nothing.

**Scoped to the sequential regime it was measured in.** A cached count of 13,563,
not a multiple of 128, was later observed once under concurrency. The harness
uses 128 as a **measured policy assumption**, not a provider guarantee.

### Billing reconstruction

575 provider responses reconstructed to a derived figure lying inside the
interval the account balance can distinguish, with four conservation identities
and zero violations. **Agreement is consistency, not proof of the provider's
internal implementation.**

## Evidence scope

Everything above is `deepseek-flash` on `/v1/chat/completions` unless stated,
one repository as corpus, single runs at the stated n. `deepseek-v4-pro` appears
only in a cross-model cache test. No finding is established as
model-independent, endpoint-independent or workload-independent.

## Prompt optimization

**NULL under the tested design.** Seven prompt conditions over 18 machine-checked
tasks, 396 trials, model and reasoning configuration held constant.

Two candidate effects appeared and **both reversed sign on independent
replication**. Output-token variability exceeded anything the conditions
produced: 44% median within-task spread for an identical condition and task, and
the control drifted 13% against itself with no treatment applied.

**No prompt optimizer, rewriter, compressor or "optimized prompt" mode is
implemented, and none should be.** The experiment is retained as evidence in
`experiment/prompt-opt/`; it is not a feature.

## Provider-contract boundary

| the vendor documents | Replay observed |
| --- | --- |
| pricing, including peak windows | matches `pricing.py` exactly |
| caching at "fixed token intervals", interval unstated | 128 tokens, sequential regime |
| a 64-token storage unit, 2024-08-02, for V2/MLA at $0.014/M | does not predict flash behaviour in 2026-09 |
| caching is "best-effort" | so the block model describes behaviour, not a contract |
| concurrency 2,500 flash / 500 v4-pro, HTTP 429 beyond | fan-out defaults sit far below |
| **only the enabling direction of reasoning** | the disabling parameter works and is **undocumented**, hence the runtime invariant check |

## Superseded claims

| withdrawn | why |
| --- | --- |
| "Four measured interventions, 6.5x cheaper and 6.9x faster" as a page headline | measured against a naive baseline and bundling three interventions on one benchmark; presented as a general result when it is one workload |
| reasoning-off as a general optimization | class-conditional; 7/24 on aggregation |
| reasoning-off as a ~20x cost lever | that is the output-token ratio, not the cost ratio; cost is about 1.48x once cached |
| unconditional 128-token quantization | scoped to the sequential regime after a concurrent counterexample |
| any prompt-structure cost benefit | NULL, both candidates reversed on replication |
| any universal statement about flash versus v4-pro | one task, one corpus |

Historical documents keep their original text. `experiment/deepseek/` and
`experiment/deepseek-suite/` are the record of what was run.

## Reproduction

| | |
| --- | --- |
| operating contract | `DEEPSEEK-OPERATING-CONTRACT.md` |
| cache and billing package | `experiment/deepseek-suite/FINAL-REPORT.md` |
| prompt-optimization null | `experiment/prompt-opt/FINAL-REPORT.md` |
| claim ledgers | `experiment/deepseek-suite/ledger.json`, `experiment/prompt-opt/derived/ledger.json` |
| primary evidence | `experiment/deepseek-suite/raw/usage-extract.json`, 575 usage blocks with a sha256 over the source |
| deterministic reconstruction | `experiment/harness/reconstruct.py`, makes no API call |
| tests | `make harness-test`, inside `make ci` |
