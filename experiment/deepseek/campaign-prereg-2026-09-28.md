# DeepSeek campaign preregistration

**Written before any campaign trial runs.** Frozen figures below are from probes
already executed and recorded in `surface-map-2026-09-28.md` and
`gap-analysis-2026-09-28.md`.

## Question

> Can Replay systematically discover DeepSeek-specific interventions that reduce
> cost, latency or context consumption **while preserving the task outcome**?

The last clause is the one that makes this hard, and it is where the existing
prefix-ordering result is weakest: it measured a 98.3% token reduction and left
outcome `NOT_MEASURED`. No campaign result may repeat that.

## Established before starting (OBSERVED)

| fact | value |
|---|---|
| models | `deepseek-flash`, `deepseek-v4-pro` only |
| endpoints | `/v1/chat/completions`, `/v1/responses`, `/anthropic/v1/messages`, `/beta/chat/completions`, `/user/balance`, `/files` |
| usage dialects | chat + responses **inclusive**; anthropic **exclusive** |
| concurrency | 64 parallel requests, all 200, median latency fell from 662 ms to 575 ms. **No throttling observed at 64.** |
| rate-limit headers | none exposed. Limits `NOT_MEASURED` |
| balance resolution | two requests moved it $0.0000; floor near $0.01 |
| read multipliers | flash 0.0200, v4-pro 0.0333 |
| cache-write charge | **not published** |

## Budget

Whole campaign, flash, off-peak, cold worst case: **~$2.09**, about 4.5% of a
$46.89 balance. Peak costs exactly double. Off-peak windows in LA local time are
everything except 18:00-21:00 and 23:00-03:00 Sun-Thu.

## Outcome measurement, decided before running

Every intervention experiment uses a task with a **machine-checkable answer**, so
outcome is measured rather than asserted:

- extraction: pull a named field from a fixed document, exact-match scored
- counting: count occurrences of a token in supplied text, numeric match
- ordering: sort supplied items by a stated key, sequence match

An intervention that reduces cost and drops the checker's pass rate is a
**regression**, not an optimization. Arms are compared on **both** axes and a
result is reported only when both are measured.

## Experiments

Each is baseline first, one variable, n stated, cold readings on unique prefixes.

**D1 Cache mechanics.** Minimum cacheable prefix by binary search; cache TTL by
delayed re-request; what invalidates a prefix (whitespace, system/user placement,
tool definitions, temperature, `max_tokens`); whether a prefix cached on one
endpoint warms another.
*Falsifier:* if the minimum prefix is 0 and nothing invalidates, there is no
cache structure to optimize against.

**D2 Dialect conformance.** Confirm inclusive/exclusive on all three endpoints at
n=20 each, streaming against non-streaming, and that streaming totals equal
non-streaming for identical input.
*Falsifier:* a dialect that changes under load makes normalisation unsafe.

**D3 Reasoning.** On `deepseek-v4-pro`: whether `reasoning_content` is billed as
output, whether reasoning length is controllable, and reasoning tokens against
checker pass rate.
*Falsifier:* if reasoning tokens do not vary with the task, there is nothing to tune.

**D4 Performance.** Latency against prompt size (prefill) and against output size
(generation), separately. Throughput at concurrency 1, 8, 32, 64.
**TTFT requires a streaming client and is `NOT_MEASURED` until one exists**; no
latency result may be attributed to prefill without it.

**D5 Limits and failure.** Max context by binary search; the error taxonomy;
behaviour at the concurrency ceiling; retry and idempotency.
*Falsifier:* none — this is descriptive.

**D6 Prompt interventions.** Six arms, n=100, each against the checker:
prefix ordering (replication of the n=4 result at n=100 **with outcome measured**),
system vs user placement, tool-definition stability, history pruning,
structured-output cost, instruction compression.

**D7 Model selection.** flash against v4-pro on identical tasks: cost, latency,
checker pass rate. The question is where pro's 4.4x input price buys outcome.

## Controls, carried from prior work

- **Cold means cold.** Unique prefix per cold reading. A "cold" call on a recently
  used prefix returns warm; that error was found in the 2026-09-05 fixtures and
  again in this session's first Anthropic-path pair.
- **First call of each arm excluded** from steady-state figures.
- **Report the spread, not only the mean.**
- **Positive controls where the provider states a figure** it can be checked against.
- **Dollars are DERIVED** from the published table, never OBSERVED, unless a
  balance delta exceeds the ~$0.01 floor.
- **Peak/off-peak recorded per run**, since the same workload costs 2x across the
  boundary and a cost comparison spanning it is invalid.
- **Raw responses preserved** per trial; aggregates only in the repository.

## Stop conditions

Stop and report if the checker cannot distinguish arms, if a dialect proves
unstable, if spend exceeds $5, or if an arm's cost reduction comes with a
pass-rate drop — that last is a finding, not a failure.

## Not claimed

No scalar efficiency score. No causal claim from a before/after pair without an
arm comparison. No saving asserted without both axes measured. No inference about
provider internals from timing alone.
