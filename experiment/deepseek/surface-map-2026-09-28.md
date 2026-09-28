# DeepSeek API surface map, and one measured prompt intervention

**Date** 2026-09-28 · **Account balance at start** 46.89 USD · No credential appears in this file.

## 1. Endpoint surface (OBSERVED, by probe)

| path | status | note |
|---|---|---|
| `/models`, `/v1/models` | 200 | model list |
| **`/user/balance`** | 200 | **returns real USD balance** |
| `/v1/chat/completions` | 200 | the path Replay already parses |
| **`/v1/responses`** | 200 | **Responses API works** |
| **`/anthropic/v1/messages`** | 200 | **Anthropic-compatible** |
| `/beta/chat/completions` | 200 | returns `reasoning_content` |
| `/files`, `/v1/files` | 200 | file API present |
| `/completions` | 405 | legacy, POST only |

Not found: embeddings, moderations, batches, fine-tuning, assistants, threads,
images, audio, rerank.

**Models actually available: `deepseek-flash` and `deepseek-v4-pro`.** Only two.
A captured 400 from 2026-09-05 named three including `deepseek-v4-flash-vision-exp`;
that model is gone and `deepseek-v4-flash` now normalises to `deepseek-flash`.

**No rate-limit headers are exposed.** No `x-ratelimit-*`, no `retry-after` on a
200. Rate limits and quotas are `NOT_MEASURED` from response metadata.

## 2. Three usage dialects, and why it matters

The same provider reports usage three different ways.

| path | fields | counting |
|---|---|---|
| `/v1/chat/completions` | `prompt_tokens`, `prompt_cache_hit_tokens`, `prompt_cache_miss_tokens`, `prompt_tokens_details.cached_tokens` | **INCLUSIVE** |
| `/v1/responses` | `input_tokens`, `input_tokens_details.cached_tokens`, `output_tokens_details.reasoning_tokens` | **INCLUSIVE** |
| `/anthropic/v1/messages` | `input_tokens`, `cache_creation_input_tokens`, `cache_read_input_tokens` | **EXCLUSIVE** |

Evidence for inclusive on `/v1/responses`: identical input, `input_tokens` stayed
1,435 while `cached_tokens` went 0 to 1,280.

Evidence for exclusive on `/anthropic/v1/messages`, on a guaranteed-cold unique
prefix: cold `input=9657, read=0`; warm `input=185, read=9472`. Both sum to 9,657.

**The hazard.** Replay's canonical ledger field is `Input int json:"input_tokens"`
and means the uncached remainder. The Responses API publishes a field with the
**same name** and the **opposite meaning**. Wiring that path by name-matching
would double-count the cache, and the error would be largest on exactly the
sessions that cache best. That is the defect the 2026-09-05 spike recorded
Langfuse shipping.

`/anthropic/v1/messages` needs no conversion: its three fields are Replay's three
fields, with the same semantics.

## 3. Measured intervention: prefix ordering

Same semantic task, same model, same `max_tokens`. Only the order of a static
block and a variable question changed. n=4 per arm, first call excluded from the
steady-state figures as cold.

| arm | fresh input | cache read | median wall |
|---|---:|---:|---:|
| **A** static block first, question last | **200** | **11,264** | 855 ms |
| **B** question first, static block last | **11,463** | **0** | 1,000 ms |

Arm A steady state: 199, 201, 199 fresh. Arm B: 11,462, 11,465, 11,461. Neither
is noise.

**OBSERVED: a 98.3% reduction in fresh input tokens from reordering alone.**

`NOT_MEASURED`:

- **Dollars.** `internal/cachemodel` has no DeepSeek entry. Separately, two
  requests moved the account balance by 0.0000 USD, so the balance endpoint has a
  resolution floor near 0.01 USD and cannot price small workloads.
- **Latency effect.** 855 ms against 1,000 ms median is suggestive, but the
  ranges overlap (735–1012 against 908–1196) at n=4. No speedup is claimed.
- **Outcome equivalence.** Both arms asked a trivially equivalent question and
  the replies were not compared. The token result is real; that the task outcome
  was preserved is asserted by construction, not measured.

## 4. What Replay supports today

The proxy routes `/v1/messages`, `/v1/chat/completions` and `/v1/responses`
(`internal/proxy/passthrough.go`). A live cold/warm pair through
`-upstream https://api.deepseek.com` was ingested and normalised correctly
(`experiment/deepseek/baseline/2026-09-28-cold-warm.md`).

Missing instrumentation, smallest first:

1. **No DeepSeek pricing entry**, so every dollar figure is `NOT_MEASURED`.
2. **Responses-API usage is not normalised**, and its field names collide with
   Replay's with inverted meaning.
3. `/user/balance` is not read by anything, though it is the only
   provider-reported economic figure available.

## 5. Controls required before a campaign

- Cold means cold: use a unique prefix per cold reading. A "cold" call against a
  recently used prefix returns warm and silently encodes a false baseline.
- Exclude the first call of each arm from steady-state figures.
- Hold model, `max_tokens` and task identical across arms; vary one thing.
- Compare outputs, not only usage, or record outcome as `NOT_MEASURED`.
- Balance deltas are only admissible above roughly 0.01 USD.
