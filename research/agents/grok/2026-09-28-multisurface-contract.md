---
agent: grok
task_id: replay-multisurface-measurement-contract
turn_id: 2026-09-28-multisurface-contract
started_at: 2026-09-28T19:32:28Z
completed_at: 2026-09-28T19:35:10Z
status: COMPLETE
base_commit: 2974d1925465a15a7a1851cfe1e80360f700805e
head_commit: NOT_EMBEDDED

sources:
  - id: otel-genai-events
    kind: specification
    url: https://raw.githubusercontent.com/open-telemetry/semantic-conventions-genai/e57c543b4889619eb2a05702471937db5119165d/docs/gen-ai/gen-ai-events.md
    retrieved: 2026-09-28
    note: "Repository main at retrieval was e57c543 (2026-09-24). The opentelemetry.io GenAI pages visited the same day only say the documents moved."
  - id: otel-genai-moved
    kind: official-documentation
    url: https://opentelemetry.io/docs/specs/semconv/gen-ai/
    retrieved: 2026-09-28
  - id: openai-responses-usage
    kind: source-code
    url: https://raw.githubusercontent.com/openai/openai-python/main/src/openai/types/responses/response_usage.py
    retrieved: 2026-09-28
    note: "File header says it is generated from the OpenAPI spec. Not pinned to a release tag."
  - id: anthropic-messages
    kind: official-documentation
    url: https://platform.claude.com/docs/en/api/messages
    retrieved: 2026-09-28
  - id: anthropic-prompt-caching
    kind: official-documentation
    url: https://platform.claude.com/docs/en/build-with-claude/prompt-caching
    retrieved: 2026-09-28
  - id: deepseek-chat
    kind: official-documentation
    url: https://api-docs.deepseek.com/api/create-chat-completion/
    retrieved: 2026-09-28
  - id: deepseek-kv
    kind: official-documentation
    url: https://api-docs.deepseek.com/guides/kv_cache
    retrieved: 2026-09-28
  - id: deepseek-pricing
    kind: official-documentation
    url: https://api-docs.deepseek.com/quick_start/pricing/
    retrieved: 2026-09-28
  - id: gemini-tokens
    kind: official-documentation
    url: https://ai.google.dev/gemini-api/docs/generate-content/tokens
    retrieved: 2026-09-28
  - id: gemini-caching
    kind: official-documentation
    url: https://ai.google.dev/gemini-api/docs/caching
    retrieved: 2026-09-28
  - id: mistral-prompt-caching
    kind: official-documentation
    url: https://docs.mistral.ai/studio-api/conversations/advanced/prompt-caching
    retrieved: 2026-09-28
  - id: ollama-usage
    kind: official-documentation
    url: https://docs.ollama.com/api/usage
    retrieved: 2026-09-28
  - id: vllm-metrics
    kind: official-documentation
    url: https://docs.vllm.ai/en/latest/usage/metrics/
    retrieved: 2026-09-28
  - id: llamacpp-server
    kind: source-repository
    url: https://github.com/ggml-org/llama.cpp/blob/master/tools/server/README.md
    retrieved: 2026-09-28
    note: "Read via a public mirror of the server README. Not pinned to a llama.cpp release tag."
  - id: litellm-ui-logs
    kind: official-documentation
    url: https://docs.litellm.ai/docs/proxy/ui_logs
    retrieved: 2026-09-28
  - id: litellm-cost
    kind: official-documentation
    url: https://docs.litellm.ai/docs/proxy/cost_tracking
    retrieved: 2026-09-28
  - id: openrouter-usage
    kind: official-documentation
    url: https://openrouter.ai/docs/guides/administration/usage-accounting
    retrieved: 2026-09-28
  - id: open-responses
    kind: specification
    url: https://www.openresponses.org/specification
    retrieved: 2026-09-28
  - id: vllm-pr-40912
    kind: source-repository
    url: https://github.com/vllm-project/vllm/pull/40912
    retrieved: 2026-09-28
    note: "Merged 2026-06-20. Not executed against a vLLM binary in this turn."
  - id: replay-surfaces-doc
    kind: replay-source
    locator: docs/SURFACES.md
    commit: 2974d1925465a15a7a1851cfe1e80360f700805e

findings:
  - id: F1
    class: OBSERVED
    text: "Documented usage objects do not share one inclusion rule for input tokens."
  - id: F2
    class: OBSERVED
    text: "OpenTelemetry GenAI conventions are Development status and define input_tokens as inclusive of cached tokens."
  - id: F3
    class: OBSERVED
    text: "Hosted APIs document token usage. They do not document queue time, prefill time, or decode time. Local engines document those times and do not document a dollar price."
  - id: F4
    class: DERIVED
    text: "A single OpenAI-compatible usage adapter covers several named providers only for the fields that adapter actually receives. Absent keys are not zeroes."
  - id: F5
    class: DERIVED
    text: "Replay's public docs/SURFACES.md uses surface for the binary's filesystem and network touchpoints. The decision under review uses the same word for agents, providers, gateways, and product screens."
  - id: F6
    class: NOT_MEASURED
    text: "No fixture in this turn was executed against a live provider, gateway, or local runtime."

evidence:
  - "Anthropic documents total input as input_tokens + cache_creation_input_tokens + cache_read_input_tokens, and describes input_tokens as the uncached remainder."
  - "OpenAI Responses InputTokensDetails has cached_tokens and cache_write_tokens beside inclusive input_tokens. OutputTokensDetails.reasoning_tokens sits inside output_tokens."
  - "DeepSeek documents prompt_tokens = prompt_cache_hit_tokens + prompt_cache_miss_tokens. No cache-write field is documented."
  - "Gemini documents prompt_token_count as including cached content, thoughts_token_count separate from candidates_token_count, and total as the sum of prompt, candidates, tool-use prompt, and thoughts."
  - "Mistral documents cached_tokens as a subset of prompt_tokens, a 64-token cache block, and omission of cached_tokens when nothing was cached."
  - "OTel footnote 28, file cited above: input_tokens SHOULD include all input tokens, including cached tokens. Footnotes 23 and 24 say cache_read and cache_write SHOULD be included in input_tokens. The attribute name in that file is gen_ai.usage.cache_write.input_tokens, not cache_creation."
  - "Ollama documents prompt_eval_count, prompt_eval_cached_count, prompt_eval_duration, eval_count, eval_duration, load_duration, and total_duration, in nanoseconds, on the final chunk."
  - "vLLM's metrics page documents request_queue_time_seconds, request_prefill_time_seconds, request_decode_time_seconds, time_to_first_token_seconds, and e2e_request_latency_seconds as histograms, plus process counters for tokens."
  - "llama.cpp server README documents a per-response timings object (cache_n, prompt_n, prompt_ms, predicted_n, predicted_ms) and a separate /metrics scrape."
  - "LiteLLM documents that request and response content is not stored unless store_prompts_in_spend_logs is enabled, and that a computed cost is returned in x-litellm-response-cost."
  - "OpenRouter documents usage.cost as the amount charged to the OpenRouter account, with upstream_inference_cost only for bring-your-own-key."
  - "Replay docs/SURFACES.md at the base commit says message text is not written to the ledger, and that tool names, request path, session_id, and agent_id are written."

decisions:
  - "This turn records evidence and a derived recommendation. It does not change product code and it does not decide the roadmap."
  - "The measurement contract is the part the documents support. The named roster is not, by itself, a build order."
  - "Dollar cost is in scope for hosted calls only when a price source is named. It is out of scope for local runtimes."
  - "Optimization and verification are a separate log over measured requests. They are not fields a provider usage object contains."

uncertainties:
  - "xAI HTTP usage fields were not re-read from an xAI API reference in this turn. Treating that API as OpenAI-compatible is NOT_MEASURED here. The Grok CLI log is a different artifact and was not re-investigated."
  - "Whether a particular vLLM release omits Anthropic cache fields was not measured. PR 40912, merged 2026-06-20, claims to populate them. A June 2026 issue reported them absent. Current binaries were not run."
  - "Open Responses was read as a specification page. Which named providers pass its compliance suite was not measured."
  - "llama.cpp timings were taken from the server README on the default branch, not from a tagged release."
  - "No sample of production traffic was used. Nothing here estimates user counts, willingness to pay, or how often a follow-up session matches."

rejected_hypotheses:
  - id: H-single-latency
    status: NOT_SUPPORTED
    text: "One latency field is a sufficient performance model."
  - id: H-local-dollars
    status: NOT_SUPPORTED
    text: "Local runtimes can be given a dollar cost from the measurements they document."
  - id: H-otel-as-internal-store
    status: NOT_SUPPORTED
    text: "The current OTel GenAI attribute set can be the only stored form. It is Development, its input total is inclusive, and the public docs site for the old path no longer carries the normative text."
  - id: H-registry-equals-adapters
    status: NOT_SUPPORTED
    text: "Each name on the known list requires its own integration."
  - id: H-replay-knows-truer-usage
    status: NOT_SUPPORTED
    text: "After ingesting every named surface, Replay would know a token or dollar figure the serving system did not already report."

next_action:
  - "Human review of this record before any adapter work."
  - "If implementation starts, the first code change is a dialect fixture set, not a new provider client. That work is out of scope for this turn."
---

# Multi-surface measurement contract

## Question

The decision under review is: Replay should measure heterogeneous AI workloads through one evidence-aware contract, with usage, cost, performance, optimization, and verification as the major dimensions. The known names are agent CLIs (Claude Code, Codex, Grok/xAI logs), direct APIs (DeepSeek, Gemini, Mistral, xAI/Grok, OpenAI-compatible), gateways and apps (Open WebUI, OpenRouter, LiteLLM), local runtimes (Ollama, vLLM, llama.cpp), and Replay's own screens (CLI, statusline, web UI, MCP, export/API, proxy).

This turn does not add names to that set inside the findings. Candidates noticed while reading are listed separately.

## Method

Primary pages and one pinned specification file were read on 2026-09-28. Replay's own `docs/SURFACES.md` was read at `2974d192`. No provider was called. No runtime was started. No fixture was executed. Claims below are limited to what those documents say, plus derivations marked as such.

`SOURCE SAYS` is a statement that appears in a cited document. `GROK INTERPRETATION` is a conclusion from comparing those statements. `REPLAY IMPLICATION` is a consequence for the product if the interpretation is accepted. It is not a decision.

## Word collision inside Replay

SOURCE SAYS (`docs/SURFACES.md` at the base commit): "surface" means a filesystem or network touchpoint of the Replay binary. The ledger row is marked verified for one claim: message text is not written. The same row says records do contain the request path, `session_id`, `agent_id`, and tool names.

GROK INTERPRETATION: the architecture decision uses "surface" for a different partition (agent, provider, gateway, runtime, product screen). Using one registry for both meanings will schedule the wrong work.

REPLAY IMPLICATION: a workload registry needs its own name. It should not be appended to `docs/SURFACES.md` without saying the word has changed meaning.

## Inclusion rules that are actually documented

| Dialect | What the cited document says about input | Cache write | Reasoning |
|---|---|---|---|
| Anthropic Messages | Total input is `input_tokens + cache_creation_input_tokens + cache_read_input_tokens`. `input_tokens` is the uncached remainder. | `cache_creation_input_tokens`, with an optional 5-minute / 1-hour split | `output_tokens` is the billed output total. A details object may decompose it. |
| OpenAI Responses | `input_tokens` is the input total. `input_tokens_details.cached_tokens` and `cache_write_tokens` are details. | `cache_write_tokens` on the details object in the Python SDK model | `reasoning_tokens` is inside `output_tokens` |
| DeepSeek chat | `prompt_tokens = prompt_cache_hit_tokens + prompt_cache_miss_tokens` | Not documented as its own field. A miss is uncached input. | `completion_tokens_details.reasoning_tokens` |
| Gemini | `promptTokenCount` includes cached content. `cachedContentTokenCount` is the cached part. | Not documented | `thoughtsTokenCount` is separate from `candidatesTokenCount`. Documented total adds prompt, candidates, tool-use prompt, and thoughts. |
| Mistral chat | `prompt_tokens` includes cached tokens. Uncached input is `prompt_tokens - cached_tokens`. | Not documented | Not a separate usage counter in the caching page |
| OTel GenAI events, pinned file | `gen_ai.usage.input_tokens` SHOULD include cached tokens. Cache read and cache write SHOULD also be included in that total. | `gen_ai.usage.cache_write.input_tokens` | `gen_ai.usage.reasoning.output_tokens` SHOULD be included in output |

Class: OBSERVED for each cell that quotes a cited page. The table is a comparison, so the choice to place them side by side is DERIVED.

GROK INTERPRETATION: adding these numbers under one column named `input_tokens` without an inclusion flag double-counts or drops cache, depending on the row. That is the fact that makes a common contract useful. The contract's job is to record the conversion and the absences, not to pretend the vendors already agreed.

Streaming is part of the same hazard. Anthropic's message-delta usage is documented as cumulative. OpenAI and Mistral document that a stream carries usage on the final chunk only when usage is requested. Summing every event is not a documented operation for those APIs.

## What is not a usage field

SOURCE SAYS:

- Ollama's usage page lists `load_duration`, `prompt_eval_duration`, and `eval_duration` in nanoseconds, plus token counts and `prompt_eval_cached_count`. It does not list a price.
- vLLM's metrics document separates queue time, prefill time, decode time, time to first token, and end-to-end latency, as histograms, and token totals as counters. Those are process series with a model label, not a per-call invoice.
- The llama.cpp server README documents a per-response `timings` object and a Prometheus `/metrics` endpoint that is off unless `--metrics` is set. The README's context total is `prompt_n + cache_n + predicted_n`.
- Hosted usage pages cited above do not define queue time, prefill time, or decode time.

GROK INTERPRETATION: "latency" is not one comparable field. A minimum vocabulary that matches the documents is a set of optional durations, each with an observer: end-to-end (client or proxy), time to first token (streaming observer or a local histogram), queue (local engine only), prefill, decode, model load (Ollama), and tool time (agent log, not the provider). A vLLM or llama.cpp histogram must stay a fleet series. Writing it onto one request is an invention.

REPLAY IMPLICATION: cost and performance are different dimensions with different applicability. Hosted cost is possible when a price source is stored: a provider-returned cost (OpenRouter documents `usage.cost` as the account charge, and `upstream_inference_cost` only for bring-your-own-key), a gateway header (LiteLLM documents `x-litellm-response-cost`), or a named price row applied to normalized usage. DeepSeek's pricing page distinguishes peak and off-peak, so a row without a time basis does not determine a DeepSeek price. Local engines have no documented dollar. Inventing one from tokens or from GPU memory is not a measurement.

## Protocol leverage, and what it drops

SOURCE SAYS: LiteLLM documents an OpenAI-shaped usage object across providers and says content is not stored unless `store_prompts_in_spend_logs` is set. Open Responses describes a shared request and item schema based on the Responses API and says providers may extend it. Open WebUI is not a measurement specification; it is an application in front of a backend.

GROK INTERPRETATION: one OpenAI-compatible usage reader can ingest OpenAI, DeepSeek, and Mistral chat responses, OpenRouter's usage object, LiteLLM's response, and the `/v1` shims of Ollama, vLLM, and llama.cpp, for the keys that are present. Gemini's `usageMetadata` is not that object. Claude's Messages usage is not that object. Grok's CLI log was not shown to be that object.

What the abstraction drops is exactly the keys a particular response omits: TTL split, cache writes, reasoning, rate-limit headers, provider request id, and engine timings. Class of that loss list: DERIVED from the matrix above. It was not measured by diffing live responses.

vLLM PR 40912 says the Anthropic-compatible endpoint previously omitted cache fields and claims to populate them, with `cache_creation_input_tokens` reported as 0 when the engine only knows cache hits. Class: OBSERVED as a merged pull-request description. Whether a binary in the field does this is NOT_MEASURED. A research note must not say the omission is the current behavior, and must not say it is gone.

## Canonical record

GROK INTERPRETATION, offered as a recommendation rather than a finding of fact.

Belongs in one stored request:

- protocol, provider, model string as returned, agent name if a client log has one
- request id only if the wire or log contained one
- exclusive legs after a recorded conversion: uncached input, cache read, cache write, output, reasoning-as-subset-of-output
- a presence flag per leg: present, absent, or not applicable
- the raw usage object, or a hash of it, so the conversion can be checked
- optional durations with units and observers
- money only with currency and price source; absent money is unpriced, not zero

Does not belong in that record:

- prompt, completion, tool results, repository contents, secrets
- a single latency
- a synthesized session id labeled as the provider's
- finding, recommendation, intervention, or outcome

Retries and hops: the same call can appear in an agent log, a gateway spend row, and a provider response. SOURCE SAYS nothing in the cited vendor docs that joins those three. GROK INTERPRETATION: without a stored provider request id, they must not be added. Unjoined rows stay visible and out of the total. This was not tested on a trace.

Optimization. The chain observation, finding, recommendation, intervention, follow-up, outcome is a log shape. It is not present in any usage object read here. A recommendation is checkable later only when it names a knob the same evidence can see change (model id, a cache marker the client controls, a tool-set hash, an engine setting). Acceptance of a suggestion is not that change. A later session on another task is not a follow-up. A difference is not a cause. Class of this paragraph: GROK INTERPRETATION. Status of "verification is a measurement dimension comparable to usage": NOT_SUPPORTED by the documents reviewed.

## Privacy boundary for a later team view

SOURCE SAYS: Replay's ledger, per `docs/SURFACES.md`, already omits message text and already stores path, tool names, and session identifiers. LiteLLM's default is the same split for content: spend logs without bodies unless opted in.

REPLAY IMPLICATION: the smallest structured record that keeps a later aggregate possible without uploading prompts is the canonical request above, minus raw path strings. The current ledger is stricter about message text than about paths. This turn does not propose a sync feature. It records that any future export which includes `docs/SURFACES.md`'s path and tool-name fields is not "usage only."

## What Replay would know that the server does not

GROK INTERPRETATION: nothing about the token count or the hosted price that the serving response did not already carry, once the conversion is fixed. The serving system remains the source of those numbers.

The cross-surface product, if the contract is implemented, is the refusal to add incompatible counters, the explicit list of fields a hop dropped, and the separation of engine time from hosted usage. That is a property of the store. It is not a hidden measurement. Whether that property is distinctive relative to LiteLLM, OpenRouter, language-model observability products, or engine dashboards was not measured by using those products. Their documented scopes are different: LiteLLM and OpenRouter document cost for calls they proxy; vLLM documents engine histograms; none of the pages read here document a joined agent-log plus local-engine plus hosted-invoice record with a match grade on a later session.

Potential IP question, not a claim of novelty: whether a stored inclusion flag plus an absence flag, applied before any cross-vendor sum, is distinguishable from prior art that already normalizes usage (LiteLLM's OpenAI-shaped usage, OTel's inclusive `input_tokens` with cache breakdowns). This turn does not assert that it is.

## Candidates noticed, not added to the known set

DISCOVERED / CANDIDATE, from pages read while checking the known names. Not investigated as integrations.

- Azure OpenAI, Amazon Bedrock, and Google Vertex, mentioned by LiteLLM's cost-tracking page as tier-specific pricing.
- Further local metric families shaped like vLLM's. Not enumerated here.
- MCP, which the OTel GenAI repository lists as its own convention. That is a tool protocol, not one of the Replay screens in the decision, and not a model provider.

## Conformance, if implementation happens later

This is a REPLAY IMPLICATION, not a shipped ladder.

| Level | Pass condition grounded in this turn | Fail |
|---|---|---|
| Ingested | A row exists and the raw usage is kept | A vendor id was guessed and no usage object was kept |
| Measured | Exclusive legs filled and absences marked; stream events not summed; histograms not written as one request | A missing cache field stored as 0; inclusive input added to cache read |
| Priced | Hosted cost has a source. Local cost is absent. | Unpriced stored as 0, or a dollar on Ollama, vLLM, or llama.cpp |
| Deduped | A second hop points at the first or stays out of the total | Both rows are summed |
| Checked later | A later window is compared only when provider, model family, and agent match, and the grade is stored | The comparison is labeled an effect |

Optimization does not get a level until the knob it names is one of the measured fields.

## Hypothesis record

Observation. The cited usage documents disagree on whether "input" includes cache, and on whether cache write and reasoning exist.

Hypothesis. One evidence-aware contract can cover the known names without one integration project per name, and the five dimensions are equally part of that contract.

Prior art consulted. OTel GenAI events (pinned), Open Responses, LiteLLM spend logs, OpenRouter usage accounting, and the vendor pages in the source list. Not a full product evaluation.

Test. Document comparison only. No runtime test.

Result. The inclusion conflict is in the documents. Engine timings are in the local documents and absent from the hosted usage documents. The named list mixes agents, protocols, gateways, applications, engines, and Replay screens.

Interpretation. The contract is justified. Equal status for cost, a single latency, and causal verification is not. Most hosted names do not justify a private client if an OpenAI-compatible reader records absences. Gemini and Anthropic are different dialects. Grok CLI versus xAI HTTP was not established.

Status: PARTIALLY_SUPPORTED.

Rejected siblings are in the front matter.

## Out of scope

OPTIONAL / FUTURE: executing the dialect fixtures, pricing-table diffs, and any adapter. Closed durable-work-state mechanism research was not reopened. No Grok CLI ledger accounting was repeated.
