# DeepSeek gap analysis: pricing, and the telemetry a prompt optimizer needs

**Date** 2026-09-28. Prices read from DeepSeek's published table on this date.
Telemetry compared against the OpenTelemetry GenAI semantic conventions.

## Part A — Pricing gap

### A1. DeepSeek has no entry at all

`internal/cachemodel` carries Anthropic and OpenAI rules. It has no DeepSeek row,
so every request Replay ingests from this provider is counted `unpriced` and every
dollar figure is `NOT_MEASURED`. `replay cost` over a live DeepSeek ledger returned
`unpriced: 1`, `totalUsd: 0`, where 0 means unpriced rather than free.

### A2. Published prices, USD per 1M tokens

| model | cache hit | cache miss | output |
|---|---:|---:|---:|
| `deepseek-flash` off-peak | 0.003 | 0.15 | 0.60 |
| `deepseek-flash` peak | 0.006 | 0.30 | 1.20 |
| `deepseek-v4-pro` off-peak | 0.022 | 0.66 | 1.98 |
| `deepseek-v4-pro` peak | 0.044 | 1.32 | 3.96 |

Derived read multipliers: **flash 0.0200**, **v4-pro 0.0333**. Anthropic's is
0.1, so a DeepSeek cache read is proportionally about five times cheaper than an
Anthropic one. **No cache-write charge is published.**

### A3. The structural mismatch

`cachemodel.Price` is `{InputPerMTok, OutputPerMTok, ReadMult}`. Three problems:

1. **Time-of-day pricing is not expressible.** DeepSeek charges peak rates 01:00
   to 04:00 and 06:00 to 10:00 UTC Monday to Friday, with off-peak at half.
   `PriceForAt` resolves a price by the *date a table was published*, which is a
   different axis from the *hour a request was made*. Pricing a DeepSeek request
   needs the request's UTC timestamp, which the ledger records but the price
   lookup does not consume.
2. **No cache-write price.** The struct has no write field because Anthropic's
   write is a multiple of input. DeepSeek publishes no write charge at all, which
   is a third state: not zero-cost by assumption, but unpublished.
3. **Two-model table only.** `deepseek-flash` and `deepseek-v4-pro` are the only
   models the API returns. A 2026-09-05 capture named a third,
   `deepseek-v4-flash-vision-exp`, which no longer exists.

### A4. Consequence for advice, measured

The prefix-ordering intervention measured at n=4 per arm
(`surface-map-2026-09-28.md`) priced at the off-peak flash rate:

| arm | fresh | cache read | input cost/call | per 1,000 calls |
|---|---:|---:|---:|---:|
| static block first | 200 | 11,264 | $0.00006379 | $0.0638 |
| variable question first | 11,463 | 0 | $0.00171945 | $1.7194 |

**DERIVED: 96.3% lower input cost**, $1.6557 saved per 1,000 calls. This is
DERIVED from a published table, not OBSERVED: no invoice or balance movement
confirms it. Two live requests moved the account balance by $0.0000, so
`/user/balance` has a resolution floor near one cent and cannot confirm figures
this small.

The absent write charge also inverts Replay's trimming advice for this provider.
Replay's break-even assumes a write premium to win back; with no write charge
there is nothing to recover, so "trim to avoid a re-write" is not a trade-off
here. That is the same point the 2026-09-05 spike reached from the pricing side.

## Part B — Telemetry gap

### B1. What Replay emits today

`internal/otlp` emits, to a file and never to a network:

```
gen_ai.operation.name   gen_ai.request.model   gen_ai.system
gen_ai.usage.input_tokens        gen_ai.usage.output_tokens
gen_ai.usage.cache_read.input_tokens   gen_ai.usage.cache_write.input_tokens
replay.cache.outcome   replay.cache.prefix_id   replay.calibration.tier   replay.turn
```

The `replay.*` attributes are the differentiated part: no third-party convention
carries a cache outcome or a prefix identity.

### B2. What the OpenTelemetry GenAI conventions define

Metrics: `gen_ai.client.operation.duration`,
`gen_ai.client.operation.time_to_first_chunk`,
`gen_ai.client.operation.time_per_output_chunk`,
`gen_ai.server.request.duration`, `gen_ai.server.time_to_first_token`,
`gen_ai.server.time_per_output_token`, `gen_ai.invoke_agent.duration`,
`gen_ai.invoke_agent.inference_calls`, `gen_ai.invoke_agent.tool_calls`,
`gen_ai.execute_tool.duration`, `gen_ai.invoke_workflow.duration`.

Required attributes: `gen_ai.operation.name`, `gen_ai.provider.name`,
`gen_ai.tool.name`. Conditionally required: `error.type`, `gen_ai.request.model`,
`gen_ai.agent.name`, `gen_ai.tool.type`. Recommended: `gen_ai.response.model`,
`server.address`.

### B3. Gaps that matter for optimizing prompts

| gap | why it matters | evidence today |
|---|---|---|
| **No duration metric emitted** | `latency_ms` is in `ledger.Record` and reaches no span | `NOT_MEASURED` in OTLP output |
| **No TTFT / time-per-output-token** | the only way to separate prefill from generation, which is exactly what a prefix change affects | never captured; needs a streaming client |
| **`gen_ai.provider.name` absent** | Replay emits `gen_ai.system`, an older name; a DeepSeek run is not attributable by the current convention | one-line fix |
| **`gen_ai.response.model` absent** | DeepSeek rewrites the requested id: `deepseek-v4-flash` comes back as `deepseek-flash`. Only recording the request model mislabels the row | OBSERVED this session |
| **No tool-call or agent metrics** | agentic workloads cannot be attributed to tools | not applicable to the current proxy |
| **No cost attribute** | there is no standard one; cost stays derived from usage and a dated table | by design |

### B4. What no third-party convention gives us

The conventions above measure *what a call did*. They carry nothing for **why a
cached prefix broke**, which is the fact a prompt optimizer acts on.
`replay.cache.outcome` and `replay.cache.prefix_id` have no equivalent in the
GenAI semconv, in LiteLLM's normalisation, or in the usage shapes any provider
publishes. Adopting the standard's names should not mean dropping these.

## Part C — Smallest changes, in order

1. **Add a dated DeepSeek row to `cachemodel`**, with the four prices and both
   read multipliers. Unblocks every dollar figure for this provider.
2. **Make the price lookup accept the request hour**, or record DeepSeek prices as
   peak-only and flag off-peak requests as over-stated. Either is honest; silently
   pricing everything at one rate is not.
3. **Emit `latency_ms` as `gen_ai.client.operation.duration`.** The value already
   exists in the ledger and reaches nothing.
4. **Emit `gen_ai.response.model` alongside the request model**, because DeepSeek
   demonstrably rewrites it.
5. **Rename `gen_ai.system` to `gen_ai.provider.name`**, keeping the old key until
   consumers migrate.

TTFT requires a streaming client and is the largest of these. It is the one that
would let a prefix intervention be attributed to prefill rather than to total
latency, and it stays `NOT_MEASURED` until then.
