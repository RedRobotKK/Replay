# DeepSeek optimisation: the levers, and how they are enforced

Four measured interventions. Applying all of them made a benchmark **6.5x
cheaper and 6.9x faster at an unchanged 12/12 pass rate**. Each is enforced by
`experiment/harness/policy.py` and covered by `make harness-test`, which runs
inside `make ci`.

Nothing here is advice you have to remember. The rules are code, and the one
rule that depends on an undocumented provider parameter fails loudly when its
premise stops holding.

## The levers

### 1. Put variable content last

**4.6x cheaper, no latency cost.** The shared block leads, the query follows.

Prefix matching is byte-exact, so **one leading space drops the cache hit rate to
zero**. A timestamp, request id, or rotating system preamble placed before a
shared document is expensive in a way that is invisible at the call site.

`policy.assemble(shared, variable)` builds the prompt and refuses a prefix that
begins with whitespace.

### 2. Disable reasoning on lookups, never on aggregation

This lever is **class-conditional** and reversing it is costly:

| task class | reasoning on | reasoning off |
| --- | ---: | ---: |
| lookup (answer is verbatim in the input) | 18/18 | **17/18**, 19.5x fewer output tokens |
| aggregation (answer is built by a rule) | 24/24 | **7/24** |

Reasoning-off does not degrade gracefully. Asked to count lines beginning with
`func` plus a space it answered **67** where the truth is **39**, and counted
**1** type declaration where there are **14**. Nothing in those responses marks
them wrong.

`policy.config_for(task_class)` returns the right kwargs. An **unclassified**
task gets reasoning ON: the default must fail toward expensive and correct, never
toward cheap and silently wrong.

### 3. Warm the prefix, then fan out

**Never fan out cold.** Concurrency alone measured 11.5x faster at **0.99x the
cost** and a 0% hit rate, because every request starts before any has completed.

The cache is quantised to **128-token blocks** and the final block is never
served:

```text
cached_tokens = 128 * max(0, floor(shared_prefix_tokens / 128) - 1)
```

Exact on 17 of 17 rungs. So a shared prefix below **256 tokens caches nothing at
all** and warming it buys nothing. `policy.should_warm(prompts)` decides.

### 4. Prefer `deepseek-flash`

`deepseek-v4-pro` cost **3.5x more for an identical score** on mechanical
extraction. It is not established that this holds where reasoning is
load-bearing.

## What the provider does and does not promise

Read `experiment/deepseek/docs-reconciliation-2026-09-29.md` before extending
any of this. In short:

- **Pricing** matches `experiment/harness/pricing.py` exactly, peak windows
  included.
- **Concurrency** is published at 2,500 for flash and 500 for v4-pro, HTTP 429
  beyond. Our fan-out defaults sit far below that.
- **The 128-token block is undocumented.** The vendor states "fixed token
  intervals" and gives no interval, and calls caching **best-effort**. The block
  model describes current behaviour; it is not a contract.
- **Disabling reasoning is undocumented.** The guides show only
  `reasoning_effort: "high"` and `thinking: {"type": "enabled"}`.

## The guard that matters

Because the disabling parameter is undocumented, it can be dropped or renamed
without notice, and the failure is silent: reasoning resumes, lookup output rises
roughly 20x, nothing errors.

`policy.check_reasoning_honoured(measurement, sent_kwargs)` raises
`ReasoningRegression` when a request disabled reasoning and the response reports
reasoning tokens anyway. `reasoning_tokens` is already on every chat-dialect
response, so the check is free. **Call it on every such response.**

## How a spending experiment is written

Every experiment goes through `session.Run`, which applies the levers by
construction rather than by the author remembering them:

```text
run = session.Run(curlrc=..., tmp=..., ceiling_usd=0.50, label="my-experiment")
rows = run.fan(TaskClass.LOOKUP, shared_document, questions)
```

`task_class` is a **required** argument with no default, so the reasoning
setting is never chosen by accident. The shared block always leads, the worker
count is validated against the published limit at construction rather than
discovered as HTTP 429 mid-run, spend is reserved before dispatch, and every
response whose request disabled reasoning is checked.

`TestNoScriptBypassesTheRunner` enforces this against the source: a new script
either uses the runner or has to declare itself frozen in its own docstring.
`runner.py` is superseded and raises if imported; it had no ceiling and used
`ThreadPoolExecutor.map`, which discards paid-for results when any call raises.

The one-shot probes that produced the findings above are **frozen**. They
predate the runner and hand-roll their own budget, and rewriting them would
change what produced the numbers in the ledger.

## Running it

```text
make harness-test     # 43 fixture tests, no credential, no network, no spend
make ci               # includes the above
```

Before a run that spends money, pass the plan through `policy.review()`; it
reports cold fan-outs, reasoning set wrongly for the task class, and worker
counts above the published limit.

## Evidence

| finding | file |
| --- | --- |
| cache geometry, what busts it, availability | `experiment/deepseek/cache-characterisation-2026-09-29.md` |
| baseline and six arms | `experiment/deepseek/optimization-arms-2026-09-29.md` |
| where reasoning-off applies | `experiment/deepseek/reasoning-scope-2026-09-29.md` |
| what the vendor documents | `experiment/deepseek/docs-reconciliation-2026-09-29.md` |
| fan-out defects and their fixes | `experiment/deepseek/fanout-audit-2026-09-28.md` |

## Limits

One model family, one repository as corpus, `n` of 18 and 24 per cell in the
reasoning study. The 94% lookup figure rests on a single failure and supports
"no large cost on lookups" rather than a point estimate. Cache TTL is
undocumented beyond "hours to days" and unmeasured here.
