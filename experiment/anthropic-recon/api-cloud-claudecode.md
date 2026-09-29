# Anthropic API, Cloud Reseller and Claude Code Control Surfaces

Reconnaissance map. Documentation research only. No API calls were made in producing this
document.

- Fetch date for every DOCUMENTED row: 2026-09-29.
- Local inspection date for every OBSERVED row: 2026-09-29.
- Host note: `docs.claude.com/en/...` now returns `301 Moved Permanently` to
  `platform.claude.com/docs/en/...`. All citations use the canonical platform host.

## Evidence classes

| Class | Meaning |
| --- | --- |
| DOCUMENTED | Stated in vendor documentation. A URL is given. |
| OBSERVED | Visible on this machine right now. No vendor claim attached. |
| DERIVED | Computed from DOCUMENTED or OBSERVED rows. The inputs are named. |
| HYPOTHESIS | Plausible and untested. Carries a falsifier where one exists. |
| UNKNOWN | The documentation does not say. Not inferred. |

Provider internals are not inferred anywhere in this document. Where parity between the
first-party API and a reseller is not documented, the row reads UNKNOWN rather than
"same".

---

# O. API surfaces

## O.1 Rate limit dimensions

DOCUMENTED. Source: [Rate limits](https://platform.claude.com/docs/en/api/rate-limits).
Verbatim:

> The rate limits for the Messages API are measured in requests per minute (RPM), input
> tokens per minute (ITPM), and output tokens per minute (OTPM) for each model class. If
> you exceed any of the rate limits you will get a 429 error describing which rate limit
> was exceeded, along with a `retry-after` header indicating how long to wait.

The campaign brief's premise is confirmed. RPM, ITPM and OTPM are the three Messages API
dimensions. Two further facts qualify them.

| Fact | Class | Detail |
| --- | --- | --- |
| Limits are per model, not shared | DOCUMENTED | "Rate limits are applied separately for each model; therefore you can use different models up to their respective limits simultaneously." |
| Enforcement is token bucket, not fixed window | DOCUMENTED | "The API uses the token bucket algorithm to do rate limiting. This means that your capacity is continuously replenished up to your maximum limit, rather than being reset at fixed intervals." |
| Sub-minute bursts can trip a per-minute limit | DOCUMENTED | "a rate of 60 requests per minute (RPM) might be enforced as 1 request per second. Short bursts of requests can exceed the limit and trigger rate limit errors." |
| A separate acceleration limit exists | DOCUMENTED | "You might also encounter 429 errors because of acceleration limits on the API if your organization has a sharp increase in usage." The threshold is not published. |
| Acceleration limit threshold | UNKNOWN | Not published. |
| Concurrency limit (parallel in-flight requests) | UNKNOWN | No concurrency dimension is documented for the Messages API. RPM is the only request-side dimension. The Batch API has a queue-depth limit, which is the nearest documented analogue. |

### Cache-aware ITPM

DOCUMENTED, same page. This is the single most consequential detail for cost and
throughput modelling.

> **For most Claude models, only uncached input tokens count toward your ITPM rate
> limits.**

| Usage field | Counts toward ITPM? | Class |
| --- | --- | --- |
| `input_tokens` (tokens after the last cache breakpoint) | Yes | DOCUMENTED |
| `cache_creation_input_tokens` | Yes | DOCUMENTED |
| `cache_read_input_tokens` | No, for most models | DOCUMENTED |
| `cache_read_input_tokens` on Claude Haiku 3.5 | Yes. Footnote 4: "Limit counts `cache_read_input_tokens` toward ITPM usage." | DOCUMENTED |

Two traps the documentation names explicitly:

- `input_tokens` is not total input. "The `input_tokens` field only represents tokens
  that appear **after your last cache breakpoint**, not all input tokens in your request."
  Total is `cache_read_input_tokens + cache_creation_input_tokens + input_tokens`. Any
  instrument that reads `input_tokens` as total input under-reports whenever caching is
  active.
- `max_tokens` does not reserve OTPM. "OTPM rate limits are evaluated in real time as
  output tokens are produced, counting only the actual tokens generated. The `max_tokens`
  parameter does not factor into OTPM rate limit calculations, so there is no rate limit
  downside to setting a higher `max_tokens` value."

ITPM is estimated at request start and corrected during the request: "ITPM rate limits are
estimated at the beginning of each request, and the estimate is adjusted during the
request to reflect the actual number of input tokens used." DERIVED from that sentence:
an external observer sampling remaining-token headers mid-flight sees an estimate, not a
settled figure. Treat a single header read as provisional.

## O.2 Rate-limit response headers

DOCUMENTED. Source: [Rate limits, Response
headers](https://platform.claude.com/docs/en/api/rate-limits). These are the exact header
names returned.

| Header | Exposes | Class |
| --- | --- | --- |
| `retry-after` | Seconds to wait. "Earlier retries will fail." | DOCUMENTED |
| `anthropic-ratelimit-requests-limit` | RPM ceiling | DOCUMENTED |
| `anthropic-ratelimit-requests-remaining` | RPM headroom | DOCUMENTED |
| `anthropic-ratelimit-requests-reset` | RFC 3339 replenish time | DOCUMENTED |
| `anthropic-ratelimit-input-tokens-limit` | ITPM ceiling | DOCUMENTED |
| `anthropic-ratelimit-input-tokens-remaining` | ITPM headroom, "rounded to the nearest thousand" | DOCUMENTED |
| `anthropic-ratelimit-input-tokens-reset` | RFC 3339 | DOCUMENTED |
| `anthropic-ratelimit-output-tokens-limit` | OTPM ceiling | DOCUMENTED |
| `anthropic-ratelimit-output-tokens-remaining` | OTPM headroom, rounded to nearest thousand | DOCUMENTED |
| `anthropic-ratelimit-output-tokens-reset` | RFC 3339 | DOCUMENTED |
| `anthropic-ratelimit-tokens-limit` / `-remaining` / `-reset` | Combined. "display the values for the most restrictive limit currently in effect" | DOCUMENTED |
| `anthropic-priority-input-tokens-limit` / `-remaining` / `-reset` | Priority Tier only | DOCUMENTED |
| `anthropic-priority-output-tokens-limit` / `-remaining` / `-reset` | Priority Tier only | DOCUMENTED |
| `anthropic-workspace-id` | Which workspace the request counted against | DOCUMENTED |
| `anthropic-fast-*` | Fast mode rate limit status | DOCUMENTED, names not enumerated on this page |

So all three dimensions are individually exposed. RPM, ITPM and OTPM each have a
`-limit` / `-remaining` / `-reset` triple. This makes first-party rate-limit state
externally measurable without a paid call beyond the one that carries the headers.

Two measurement caveats, both DOCUMENTED:

- Token remaining values are rounded to the nearest thousand. Sub-1000-token resolution is
  not available from headers.
- The generic `anthropic-ratelimit-tokens-*` triple is not a fourth dimension. It reports
  whichever limit currently binds, and reports input plus output summed when no
  workspace limit applies. An instrument that reads it as "total tokens" will silently
  change meaning when a workspace limit is added.

### retry-after semantics

| Condition | HTTP | `retry-after` sent? | Distinguishing field | Class |
| --- | --- | --- | --- | --- |
| Rate limit exceeded (RPM/ITPM/OTPM) | 429 | Yes | error type `rate_limit_error` | DOCUMENTED |
| Fast mode limit exceeded | 429 | Yes | `anthropic-fast-*` headers | DOCUMENTED |
| Tier monthly spend cap reached | 429 | **No** | `error.details.error_code` = `enforced_spend_limit_reached` | DOCUMENTED |
| Self-set spend limit reached | 400 | n/a | `invalid_request_error`, message begins "You have reached your specified API usage limits" | DOCUMENTED |
| Claude Code workspace over its limit | 429 | Yes | "Claude Code requests over that workspace's limit can instead receive a 429 that carries a `retry-after` header" | DOCUMENTED |

The spend-cap 429 is the trap. It shares the `rate_limit_error` type with a real rate
limit but carries no `retry-after`, and the documentation is explicit that retrying does
not help: "Retrying, including the SDKs' automatic retries, fails until access resumes."
An external monitor that classifies by HTTP status alone will read a month-long billing
stop as transient throttling. Classify on `error.details.error_code`.

## O.3 Usage tiers and movement between them

DOCUMENTED. Tier names are Evaluation, Start, Build, Scale, Custom.

> Limits are defined by **usage tier**. Organizations are placed on a tier automatically
> based on usage history and account standing and can move to a higher tier over time as
> they use the API.

> New organizations and organizations with limited usage history may start in the
> Evaluation tier, with limits below the standard limits shown on this page while account
> history is established. These starting limits are part of how Anthropic prevents fraud
> and abuse, and they increase automatically as your organization builds usage history.

Note what changed against older public understanding: advancement is described as
automatic on usage history and account standing. A deposit-threshold table is **not**
present on the current page. Recording that as a correction, not a gap.

| Item | Value | Class |
| --- | --- | --- |
| Tier assignment mechanism | Automatic, on usage history and account standing | DOCUMENTED |
| Explicit deposit amount per tier | Not stated on the current page | UNKNOWN |
| Evaluation tier numeric limits | "below the standard limits shown on this page". Numbers not given. | UNKNOWN |
| Manual escalation path | "Request rate limit increase" on the Console Rate limits page | DOCUMENTED |
| Limits scope | Organization level | DOCUMENTED |
| Programmatic read of configured limits | [Rate Limits API](https://platform.claude.com/docs/en/manage-claude/rate-limits-api) | DOCUMENTED |

### Standard Messages API limits by tier

DOCUMENTED. Representative rows. Opus 5 and Sonnet 5 carry identical numbers at every
tier.

| Tier | Model | RPM | ITPM | OTPM |
| --- | --- | --- | --- | --- |
| Start | Opus 5 / Opus 5.5 / Sonnet 5 / Sonnet 5.5 / Haiku 4.5 | 1,000 | 2,000,000 | 400,000 |
| Start | Fable 5.x (combined 5.1 + 5) | 1,000 | 500,000 | 100,000 |
| Build | Opus 5 / Sonnet 5 / Haiku 4.5 | 5,000 | 5,000,000 | 1,000,000 |
| Build | Fable 5.x | 2,000 | 1,500,000 | 300,000 |
| Scale | Opus 5 / Sonnet 5 / Haiku 4.5 | 10,000 | 10,000,000 | 2,000,000 |
| Scale | Fable 5.x | 4,000 | 4,000,000 | 800,000 |
| Custom | above Scale | by arrangement | by arrangement | by arrangement |

Bucket-sharing footnotes matter for any measurement that attributes throttling to a model:

- Fable 5.1 and Fable 5 share one combined limit. Mythos 5.1 and Mythos 5 share a separate
  combined limit.
- Opus 4.8, 4.7, 4.6 and 4.5 share one combined bucket. Opus 5.5 and Opus 5 each have a
  separate limit and are **not** in that bucket.
- Sonnet 4.6 and 4.5 share one bucket. Sonnet 5.5 and Sonnet 5 each have their own.

DOCUMENTED and worth flagging: `inference_geo` does not partition the pool. "Rate limits
are currently shared across all `inference_geo` values. Requests with `inference_geo: 'us'`
and `inference_geo: 'global'` draw from the same rate limit pool."

## O.4 Spend controls and budget controls

| Control | Detail | Class |
| --- | --- | --- |
| Tier monthly spend cap, Start | $500 USD | DOCUMENTED |
| Tier monthly spend cap, Build | $1,000 USD | DOCUMENTED |
| Tier monthly spend cap, Scale | $200,000 USD | DOCUMENTED |
| Custom tier | "no monthly spend cap; limits are arranged with their account team" | DOCUMENTED |
| Cap reset | "API usage pauses until 00:00 UTC on the first day of the next month" | DOCUMENTED |
| Self-set spend limit | Console, Settings > Billing, "Adjust limit". "Your spend limit cannot exceed your current tier's cap." | DOCUMENTED |
| Per-workspace spend limit | Supported | DOCUMENTED |
| Per-workspace rate limit | Supported, "set per limiter type (such as requests per minute, input tokens per minute, or output tokens per minute)" | DOCUMENTED |
| Default workspace | "You can't set limits on the default Workspace." | DOCUMENTED |
| Workspace limit interaction | "Organization-wide limits always apply, even if Workspace limits add up to more." | DOCUMENTED |
| Unset workspace limit | "If not set, Workspace limits match the Organization's limit." | DOCUMENTED |

## O.5 Batch API

DOCUMENTED. Separate limits, shared across all models.

| Tier | RPM (all endpoints) | Max batch requests in processing queue | Max batch requests per batch |
| --- | --- | --- | --- |
| Start | 1,000 | 200,000 | 100,000 |
| Build | 2,000 | 300,000 | 100,000 |
| Scale | 4,000 | 500,000 | 100,000 |

Pricing: "The Batch API allows asynchronous processing of large volumes of requests with a
50% discount on both input and output tokens."
([Pricing](https://platform.claude.com/docs/en/about-claude/pricing)). The batch discount
stacks with prompt caching: "Batch API and prompt caching discounts can be combined."

The queue-depth limit is the closest documented thing to a concurrency ceiling anywhere in
the API surface.

## O.6 Prompt caching pricing

DOCUMENTED. Source:
[Pricing, Prompt caching](https://platform.claude.com/docs/en/about-claude/pricing).

| Cache operation | Multiplier on base input | Duration |
| --- | --- | --- |
| 5-minute cache write | 1.25x | 5 minutes |
| 1-hour cache write | 2x | 1 hour |
| Cache read (hit) | 0.1x standard. 0.025x on Fable 5.1 and Mythos 5.1. 0.05x on Opus 5.5. | Same duration as the preceding write |

Break-even, quoted verbatim: "caching pays off after one cache read for the 5-minute
duration (1.25x write), or after two cache reads for the 1-hour duration (2x write)."

Two enablement modes are documented, and the distinction matters for reseller parity
below:

- **Automatic caching**: "Add a single `cache_control` field at the top level of your
  request. The system automatically manages cache breakpoints as conversations grow."
- **Explicit cache breakpoints**: "Place `cache_control` directly on individual content
  blocks."

DERIVED from O.1 plus O.6: prompt caching acts on two independent axes at once. It cuts
billed cost via the 0.1x read multiplier, and it cuts ITPM consumption to zero for cache
reads on most models. The documentation gives the worked throughput case: "With a
2,000,000 ITPM limit and an 80% cache hit rate, you could effectively process 10,000,000
total input tokens per minute." Cost and rate-limit headroom are two instruments, and a
caching figure that names only one is incomplete.

## O.7 Model pricing

DOCUMENTED, USD per million tokens (MTok).

| Model | Base input | 5m cache write | 1h cache write | Cache hit | Output |
| --- | --- | --- | --- | --- | --- |
| Claude Fable 5.1 | $10 | $12.50 | $20 | $0.25 | $50 |
| Claude Fable 5 | $10 | $12.50 | $20 | $1 | $50 |
| Claude Opus 5.5 | $4 | $5 | $8 | $0.20 | $20 |
| Claude Opus 5 | $5 | $6.25 | $10 | $0.50 | $25 |
| Claude Opus 4.8 / 4.7 / 4.6 / 4.5 | $5 | $6.25 | $10 | $0.50 | $25 |
| Claude Sonnet 5.5 | $2 | $2.50 | $4 | $0.20 | $10 |
| Claude Sonnet 5 | $2 | $2.50 | $4 | $0.20 | $10 |
| Claude Sonnet 4.6 / 4.5 | $3 | $3.75 | $6 | $0.30 | $15 |
| Claude Haiku 4.5 | $1 | $1.25 | $2 | $0.10 | $5 |

A tokenizer change confounds every cross-version cost comparison. DOCUMENTED:

> Claude 4.7 and later models and Claude Mythos Preview use a newer tokenizer that
> contributes to their improved performance on a wide range of tasks. This tokenizer
> produces approximately 30% more tokens for the same text.

DERIVED: a per-token price cut between a 4.6-or-earlier model and a 4.7-or-later model
does not translate to a per-task cost cut at the same ratio. Any cost figure comparing
across that boundary must state which side of the tokenizer change each build sits on, or
it is not comparable. Sonnet 4.6 and earlier use the previous tokenizer.

Other pricing modifiers, all DOCUMENTED:

| Modifier | Effect |
| --- | --- |
| `inference_geo: "us"` (Claude 4.6 and later) | 1.1x on all token categories including cache writes and reads |
| Long context, 1M window, Claude 4.6 and later | No premium. "A 900k-token request is billed at the same per-token rate as a 9k-token request." |
| Fast mode, Opus 5.5 | $8 input / $40 output. First-party only. |
| Fast mode, Opus 5 and 4.8 | $10 input / $50 output. Not available with the Batch API. |
| Web search | $10 per 1,000 searches |
| Web fetch | No additional charge beyond tokens |
| Code execution | 1,550 free hours per org per month, then $0.05 per hour per container. Free when used with web search or web fetch. |
| Managed Agents session runtime | $0.08 per session-hour, `running` status only |

Tool use carries a fixed system-prompt token cost per request. Opus 5.5 and Sonnet 5.5:
286 tokens with `tool_choice` auto or none. Opus 5: 286 auto/none, 406 any/tool. Opus 4.7:
675 auto/none, 804 any/tool. This is a floor on any tool-enabled request and belongs in
any per-call cost model.

## O.8 Other API surfaces

| Surface | Status | Class |
| --- | --- | --- |
| Streaming | Supported first-party. SSE. | DOCUMENTED |
| Tool use | Supported, including Bash, Computer use, Memory, Text editor tools | DOCUMENTED |
| Token counting endpoint | [Token counting](https://platform.claude.com/docs/en/build-with-claude/token-counting). Per-platform availability in section C.4. | DOCUMENTED |
| Managed Agents rate limits | Create endpoints 300 rpm, read endpoints 1,200 rpm. Separate from Messages API. | DOCUMENTED |
| Files API rate limits | Separate per-organization limit shared across upload, list, retrieve, download, delete | DOCUMENTED |
| API key management | Console. Keys resolve to a workspace, reported in `anthropic-workspace-id`. | DOCUMENTED |
| Claude Code workspace | A distinct workspace whose limits "are checked separately" | DOCUMENTED |
| Rate Limits API | Reads configured org and workspace limits programmatically | DOCUMENTED |

---

# C. Cloud resellers

Scope note. The documentation now distinguishes **partner-operated** platforms, where the
cloud provider invoices you, from **Anthropic-operated** platforms billed through a
marketplace. Amazon Bedrock and Google Cloud are partner-operated. Claude Platform on AWS
and Claude in Microsoft Foundry are Anthropic-operated with marketplace billing. This
distinction is the spine of the whole section and it does not match the naive "three
resellers" framing in the brief.

## C.1 Microsoft Foundry

The decisive finding in this whole section.

**DOCUMENTED, verbatim**, from
[Claude in Microsoft Foundry](https://platform.claude.com/docs/en/build-with-claude/claude-in-microsoft-foundry):

> Foundry does not include Anthropic's standard rate limit headers
> (`anthropic-ratelimit-tokens-limit`, `anthropic-ratelimit-tokens-remaining`,
> `anthropic-ratelimit-tokens-reset`, `anthropic-ratelimit-input-tokens-limit`,
> `anthropic-ratelimit-input-tokens-remaining`, `anthropic-ratelimit-input-tokens-reset`,
> `anthropic-ratelimit-output-tokens-limit`, `anthropic-ratelimit-output-tokens-remaining`,
> and `anthropic-ratelimit-output-tokens-reset`) in responses. Manage rate limiting through
> Azure's monitoring tools instead.

That is a named, enumerated observability gap, not an inference. The entire header-based
measurement approach in O.2 does not transfer to Foundry.

| Question | Answer | Class |
| --- | --- | --- |
| Who owns quota | Microsoft. "Consider requesting rate limit increases through the Azure portal or Azure support." | DOCUMENTED |
| Rate-limit headers | Not returned. Enumerated above. | DOCUMENTED |
| Numeric RPM/ITPM/OTPM values on Foundry | Not published by Anthropic | UNKNOWN |
| Metering | Claude Consumption Units, "metered hourly, and invoiced monthly in arrears on your Azure bill. CCUs are not prepaid credits." | DOCUMENTED |
| CCU conversion | "$0.01 per CCU". Token usage rated in USD at standard per-model rates, then converted. | DOCUMENTED |
| Per-token rates | "same as Claude API pricing" | DOCUMENTED |
| Rate-limit partitioning lever | "You can create multiple deployments of the same model with different names to manage separate configurations or rate limits." | DOCUMENTED |
| Debug headers that DO exist | `request-id` and `apim-request-id` (Azure API Management) | DOCUMENTED |
| `usage` object parity | "The `usage` object is consistent across all platforms (Claude API, Amazon Bedrock, Claude Platform on AWS, Foundry, and Google Cloud)." | DOCUMENTED |
| Prompt caching supported | Yes, not on the not-supported list | DOCUMENTED |
| Prompt caching identical in every respect | Not stated | UNKNOWN |

Foundry has two hosting options and they are not feature-equivalent. This is a second
axis of difference inside one reseller.

Not supported on Foundry at all: Admin API, Advisor tool, Claude Managed Agents,
Compliance API, Models API, **Message Batches API**, server-side fallback, and the
`computer_toolset_20260801` / `browser_toolset_20260801` toolsets.

Additionally not supported **when hosted on Azure** (available when hosted on Anthropic):
Code execution, web search and web fetch versions later than `web_search_20250305` and
`web_fetch_20250910`, Agent Skills, programmatic tool calling, and the Files API.
DOCUMENTED: "Requests that use these features against a deployment hosted on Azure return
a `400 Bad Request` error by design. Claude Code detects deployments hosted on Azure and
automatically adapts its feature set."

Data residency, DOCUMENTED: "For deployments hosted on Azure, prompts and completions
remain within Azure. Only usage metadata and content flagged by Anthropic's safety systems
egress to Anthropic." US Data Zone Standard is "equivalent to `inference_geo: 'us'`" and
"applies the same 1.1x pricing multiplier".

## C.2 Amazon Bedrock

Bedrock has two integrations and they are not the same surface. The legacy integration
uses `InvokeModel` and `Converse` with ARN-versioned model IDs. The current integration is
the Messages API at `/anthropic/v1/messages` with SSE streaming. Newer models (Fable 5.x,
Opus 5.5, Opus 5, Sonnet 5.5, Sonnet 5, Opus 4.8, Opus 4.7) are reachable through
`InvokeModel` but "are omitted from the model table on this page because they do not have
ARN-versioned model IDs", and are served by the same infrastructure as the current
endpoint.

| Question | Answer | Class |
| --- | --- | --- |
| Billing owner | AWS. "Bedrock pricing: Amazon Bedrock pricing page". Partner-operated, the provider invoices you. | DOCUMENTED |
| Model lifecycle owner | AWS. "Lifecycle dates on partner-operated platforms are set by the partner and can differ from the Claude API schedule." | DOCUMENTED |
| Quota ownership | **Split.** Anthropic gates token-per-minute quota, AWS enforces RPM. Full quote in C.5. | DOCUMENTED |
| Default token quota | 2,000,000 input TPM. Up to 5,000,000 input TPM and 500,000 output TPM available "without additional Anthropic approval". | DOCUMENTED |
| RPM adjustments | "contact AWS support for RPM adjustments" | DOCUMENTED |
| `anthropic-ratelimit-*` headers | Page is silent | UNKNOWN |
| Prompt caching supported | Yes, listed under supported feature highlights | DOCUMENTED |
| **Automatic** prompt caching | **Pages disagree.** Legacy page says not supported, Features overview matrix says supported. See C.6. | contradiction, see C.6 |
| Message Batches API | Not supported | DOCUMENTED |
| Files API | Not supported | DOCUMENTED |
| Payload ceiling | "Bedrock limits request payloads to 20 MB." | DOCUMENTED |
| Regional premium | "Regional endpoints include a 10% pricing premium over global endpoints." | DOCUMENTED |

Not supported on Bedrock: URL sources for images and documents, Files API, server-side
tools (code execution, web search, web fetch, advisor), Agent Skills, MCP connector,
programmatic tool calling, Message Batches, Models, Admin, Compliance, Usage and Cost
endpoints, Claude Managed Agents, server-side fallback, automatic prompt caching, and the
`computer_toolset_20260801` / `browser_toolset_20260801` toolsets.

The automatic-caching gap is the one most likely to be missed. Caching works on Bedrock,
but the ergonomics differ: a first-party client that relies on the top-level
`cache_control` field gets no caching at all on Bedrock until it is rewritten to place
breakpoints on content blocks. Same feature name, different integration contract.

## C.3 Google Cloud (Vertex AI / Agent Platform)

The product is now documented as "Claude on Google Cloud" and the surface is called Agent
Platform. Two request-shape differences from the Messages API are DOCUMENTED:

> On Agent Platform, `model` is not passed in the request body. Instead, it is specified in
> the Google Cloud endpoint URL.

> On Agent Platform, `anthropic_version` is passed in the request body (rather than as a
> header), and must be set to the value `vertex-2023-10-16`.

| Question | Answer | Class |
| --- | --- | --- |
| Billing owner | Google Cloud. Partner-operated. | DOCUMENTED |
| Data handling owner | "Data handling for this offering is governed by Google Cloud." | DOCUMENTED |
| Model lifecycle owner | Google. "Lifecycle dates on partner-operated platforms are set by the partner" | DOCUMENTED |
| Quota ownership | Google Cloud. Anthropic's page never uses the word "quota". QPM and TPM in the Google console. | DOCUMENTED-BUT-UNVERIFIED, see C.5 |
| `anthropic-ratelimit-*` headers | Page is silent | UNKNOWN |
| Prompt caching supported | Yes | DOCUMENTED |
| Automatic prompt caching | Marked available in the Features overview matrix | DOCUMENTED |
| Message Batches API | Not supported | DOCUMENTED |
| Files API | Not supported | DOCUMENTED |
| Web search tool | Supported, unlike Bedrock | DOCUMENTED |
| Browser use tool | Supported, unlike Bedrock | DOCUMENTED |
| Payload ceiling | "Agent Platform limits request payloads to 30 MB." | DOCUMENTED |
| Endpoint types | Three: global, multi-region (`us`, `eu`), regional. Regional and multi-region carry a 10% premium. | DOCUMENTED |
| Provisioned throughput | "provisioned throughput requires regional endpoints". Global and multi-region are "pay-as-you-go traffic" only. | DOCUMENTED |

Not supported on Google Cloud: URL sources for images and documents, Files API, code
execution, web fetch, advisor, Agent Skills, MCP connector, programmatic tool calling,
Message Batches, Models, Admin, Compliance, Usage and Cost endpoints, Claude Managed
Agents, server-side fallback.

## C.4 Cross-platform difference summary

The first-party-only capabilities, DOCUMENTED:

| Capability | First-party API | Bedrock | Google Cloud | Foundry | Claude Platform on AWS |
| --- | --- | --- | --- | --- | --- |
| `anthropic-ratelimit-*` headers | Yes | UNKNOWN | UNKNOWN | **No, explicitly** | UNKNOWN |
| Quota owner | Anthropic | Split: Anthropic TPM, AWS RPM | Google Cloud | Microsoft | Anthropic, "not through AWS quota systems" |
| Token counting endpoint | GA | GA | GA | GA | GA |
| Message Batches API | Yes | No | No | No | Yes |
| Files API | Yes | No | No | Anthropic-hosted only | Yes |
| Prompt caching, 5m and 1h | Yes | Yes | Yes | Yes | Yes |
| Automatic prompt caching | Yes | **pages disagree, see C.6** | Yes | Yes | Yes |
| Citations | Yes | Yes | Yes | Yes | Yes |
| Thinking / adaptive thinking | Yes | Yes | Yes | Yes | Yes |
| Bash / Memory / Text editor tools | Yes | Yes | Yes | Yes | Yes |
| Computer use | Yes | Beta | Yes | Beta | Beta |
| Browser use | Yes | No | Yes | No | No |
| Claude Managed Agents | Yes | No | No | No | No |
| Admin / Compliance / Models / Usage and Cost APIs | Yes | No | No | No | partial |
| Server-side fallback (`fallbacks`) | Yes | No | No | No | UNKNOWN |
| Fast mode | Yes, "first-party only" | No | No | No | No |
| Agent Skills | Yes | No | No | Anthropic-hosted only | UNKNOWN |
| MCP connector | Yes | No | No | UNKNOWN | UNKNOWN |
| Streaming | Yes | SSE, per the Bedrock page | UNKNOWN | UNKNOWN | UNKNOWN |
| `usage` response object | Yes | Yes, "consistent across all platforms" | Yes | Yes | Yes |

Two rows carry an evidence note. The Features overview matrix has **no row for Streaming
and no row for Fast mode**. Streaming on Bedrock is DOCUMENTED on the Bedrock page itself
("uses standard SSE streaming") and fast mode is DOCUMENTED as first-party only on the
pricing page, but the remaining streaming cells are UNKNOWN rather than assumed universal.
Absence from a matrix is not a negative finding, and it is not a positive one either.

Metering and billing ownership:

| Platform | Operated by | Invoiced by | Unit |
| --- | --- | --- | --- |
| Claude API | Anthropic | Anthropic | MTok, USD |
| Amazon Bedrock | AWS (partner-operated) | AWS | AWS Bedrock pricing |
| Google Cloud | Google (partner-operated) | Google Cloud | Google Cloud pricing |
| Claude Platform on AWS | Anthropic | AWS Marketplace | CCU at $0.01 |
| Microsoft Foundry | Anthropic | Azure Marketplace | CCU at $0.01 |

DOCUMENTED for Claude Platform on AWS, which is the interesting hybrid: "Organizations on
Claude Platform on AWS are placed on the Start tier and can move to a higher tier
automatically as they build a history of paid AWS Marketplace invoices." Anthropic's tier
model applies, but tier movement is driven by AWS invoice history. Also: "Per-workspace
rate limit configuration and fast mode are not available on Claude Platform on AWS," and
the "Request rate limit increase" flow is not available there.

## C.5 Quota ownership, verified per platform

This is the section the brief asked for directly. Quota ownership is **not** uniform across
resellers, and the documentation is explicit for three of the four.

| Platform | Who owns the quota | Verbatim basis | Class |
| --- | --- | --- | --- |
| Claude API | Anthropic | Org-level tiers, Console | DOCUMENTED |
| **Amazon Bedrock** | **Split** | "Default quota is 2 million input tokens per minute (TPM). You can request up to 5 million input TPM and 500,000 output TPM without additional Anthropic approval. AWS enforces requests-per-minute (RPM) limits on the Bedrock side; contact AWS support for RPM adjustments." | DOCUMENTED |
| Google Cloud | Google Cloud | Anthropic's page is silent. Google's console owns QPM and TPM quota. | see below |
| Microsoft Foundry | Microsoft | "requesting rate limit increases through the Azure portal or Azure support" | DOCUMENTED |
| Claude Platform on AWS | Anthropic outright | "Organizations on Claude Platform on AWS are placed on the Start tier. **Anthropic manages rate limits directly, not through AWS quota systems.**" | DOCUMENTED |

Bedrock is the interesting case and it corrects a natural assumption. Quota there is split
down the middle of the three dimensions from O.1. The **token** dimensions are
Anthropic-gated, with a documented default of 2M input TPM and a ceiling of 5M input TPM
and 500k output TPM reachable "without additional Anthropic approval". The **request**
dimension is AWS-enforced and only AWS can raise it. Source:
[Claude in Amazon Bedrock](https://platform.claude.com/docs/en/build-with-claude/claude-in-amazon-bedrock),
Quotas section.

Google Cloud, with a flag on the evidence. Anthropic's own page does not use the word
"quota" at all. Google's canonical page is
`docs.cloud.google.com/gemini-enterprise-agent-platform/models/partner-models/claude/quotas`,
which is JavaScript-rendered and returned only a navigation shell to a direct fetch. The
following came from Google's indexed page text via search, not a verified fetch, and is
therefore recorded as **DOCUMENTED-BUT-UNVERIFIED**. Re-check in a browser before quoting
it anywhere public:

- Quota is expressed in queries per minute (QPM) and tokens per minute (TPM), and TPM
  counts input and output tokens **together**. That is a different shape from the
  first-party split of ITPM and OTPM.
- Maximum quotas "might vary by account and, in some cases, access might be restricted".
  Quotas are viewed on the Google Cloud console Quotas and System Limits page, so the
  quota object is Google's, not Anthropic's.
- Models launched before 2026-05-26 have quotas keyed to endpoint type. Models after that
  date use shared model-lineage quotas across versions per location.
- The count-tokens endpoint quota is 2,000 requests per minute by default, which is
  Google-set and differs from Anthropic's own tiered count-tokens limits in C.7.

### Are `anthropic-ratelimit-*` headers returned on Bedrock and Google Cloud?

**UNKNOWN.** Neither partner page mentions response headers at all. Foundry is the only
platform that makes an explicit statement, and that statement is negative. Asserting either
parity or absence for Bedrock and Google Cloud would require an empirical check against a
live response, which this campaign's no-paid-calls constraint puts out of scope. The row
stays UNKNOWN.

DERIVED, and this is the operational consequence: the header-based measurement plan from
O.2 is confirmed only for the first-party API, confirmed absent on Foundry, and unverified
on the two partner-operated platforms. Any cross-platform instrument must not assume the
headers exist.

## C.6 A documented contradiction: automatic prompt caching on Bedrock

Recording this as an anomaly with its evidence rather than resolving it by preference.

Two Anthropic pages disagree.

- The legacy Bedrock page lists under "Features not supported": "Automatic prompt caching
  (the top-level `cache_control` field); use explicit cache breakpoints instead."
  ([legacy Bedrock page](https://platform.claude.com/docs/en/build-with-claude/claude-on-amazon-bedrock-legacy))
- The Features overview matrix marks "Automatic prompt caching" as available on Bedrock.
  ([Features overview](https://platform.claude.com/docs/en/build-with-claude/overview))

The most likely reconciliation is that the two pages describe different integrations: the
legacy page covers `InvokeModel` and `Converse` with ARN-versioned model IDs, while the
matrix describes the current Messages API endpoint at `/anthropic/v1/messages`. That
reading is **HYPOTHESIS**, not documentation. Neither page says so.

Practical handling: on Bedrock, treat automatic caching as integration-dependent and use
explicit cache breakpoints, which are documented as working on both integrations. The
falsifier is cheap and does not need a generation call to design: send a top-level
`cache_control` request on each integration and compare `cache_creation_input_tokens` in
the `usage` object against zero.

## C.7 Token counting endpoint across platforms

DOCUMENTED. Source:
[Token counting](https://platform.claude.com/docs/en/build-with-claude/token-counting).
This is the most useful endpoint for this campaign, because it is the one instrument that
prices content without a generation call.

| Property | Value | Class |
| --- | --- | --- |
| Claude API | GA | DOCUMENTED |
| Claude Platform on AWS | GA | DOCUMENTED |
| Amazon Bedrock | GA | DOCUMENTED |
| Google Cloud | GA | DOCUMENTED |
| Microsoft Foundry | GA | DOCUMENTED |
| Cost | "Token counting is **free to use** but subject to requests per minute rate limits based on your usage tier." | DOCUMENTED |
| Rate limits | Start 5,000 RPM, Build 10,000 RPM, Scale 20,000 RPM | DOCUMENTED |
| Interaction with Messages limits | "Token counting and message creation have separate and independent rate limits. Usage of one does not count against the limits of the other." | DOCUMENTED |
| Accuracy | "The token count is an **estimate**" | DOCUMENTED |
| Google-side count-tokens quota | 2,000 RPM default, Google-set | DOCUMENTED-BUT-UNVERIFIED |

Full platform availability at GA, free, and on an independent limiter. That combination
makes it the correct instrument for the un-itemisable context costs identified in M.3:
CLAUDE.md size, MCP tool-definition overhead, and skill definitions.

The tokenizer caveat repeats here and matters for any counted figure: "Claude 4.7 and
later models and Claude Mythos Preview use a newer tokenizer. The same input text produces
approximately 30 percent more tokens than on earlier models." A token count is only
comparable against another count taken on the same side of that boundary.

## C.8 Claude Platform on AWS spend behaviour

DOCUMENTED, and it contains a measurement trap worth its own line.

> Spend is calculated at list prices and can take about 2 hours to reflect recent usage, so
> usage can exceed the cap or a limit before requests start failing. The overshoot is
> billed.

A spend cap on this platform is not a hard stop. There is roughly a two-hour lag, overshoot
happens, and the overshoot is billed. Any budget control built on it must assume it leaks.

Also DOCUMENTED there: tiers are "fixed steps: each tier pairs rate limits with a monthly
spend cap, and moving to a higher tier raises both", self-set limits require at least one
Email recipients entry on the Billing page before they can be set, breach of a self-set
limit returns HTTP 400, and "Role-based recipients, such as all admins, aren't available on
Claude Platform on AWS."

---

# M. Claude Code control surfaces

## M.0 Documentation host correction

Recording this because it invalidates a lot of older bookmarks and any scraper pointed at
the previous path.

Claude Code documentation no longer lives under the API docs host. `platform.claude.com/docs/en/docs/claude-code/*`
**404s**. The live base is `https://code.claude.com/docs/en/`, and
`docs.claude.com/en/docs/claude-code/<page>` 301-redirects there. Page slugs lost the
`claude-code/` segment as well: the cost page is now `/docs/en/costs`, permissions split
across `/docs/en/permissions` and `/docs/en/permission-modes`, and `/docs/en/slash-commands`
redirects to the skills page because custom commands were merged into skills. A machine
index is published at `https://code.claude.com/docs/llms.txt`.

## M.1 Local configuration in force (OBSERVED)

Inspected on this machine, 2026-09-29. Shape only. No value that could be a credential was
read or is reproduced here. The settings file was parsed for key paths with `jq`, and
values were never printed.

Claude Code version OBSERVED: `2.1.278`.

### `~/.claude/CLAUDE.md`

| Property | Value | Class |
| --- | --- | --- |
| Exists | Yes | OBSERVED |
| Size | 6,835 bytes, 81 lines | OBSERVED |
| Last modified | 2026-08-28 | OBSERVED |

Against the provider's "keep CLAUDE.md lean" guidance (see M.2), this file is roughly 6.8
KB and is loaded into every session in every project. DERIVED: at a nominal 4 characters
per token that is on the order of 1,700 tokens per session, before any project-level
CLAUDE.md. The figure is an estimate from a character count, not a measurement. The
token-counting endpoint would settle it without a paid generation call.

### `~/.claude/settings.json`

Top-level keys set. Eight keys. No value is reproduced.

| Key | Sub-structure OBSERVED | Note |
| --- | --- | --- |
| `agentPushNotifEnabled` | scalar | |
| `autoMode` | `autoMode.environment` array, 24 entries | Environment entries not printed |
| `cleanupPeriodDays` | scalar | Governs retention of local transcripts, which are the measurement substrate in M.3 |
| `modelSettings` | `modelSettings.claude-opus-5.effortLevel` | Effort is configured per model id |
| `outputStyle` | scalar | |
| `statusLine` | `statusLine.type`, `statusLine.command` | Custom command-based status line |
| `theme` | scalar | |
| `tui` | scalar | |

No credential-shaped key is present in this file. `permissions`, `hooks`, `env`, `model`
and `apiKeyHelper` are all absent at the user level.

Load-bearing for this campaign: `modelSettings.<model-id>.effortLevel` exists as a real,
persisted, per-model setting. Effort selection is therefore configuration, not only a
runtime toggle.

### `~/.claude/hooks/`

OBSERVED: the directory **does not exist**. `settings.json` has no `hooks` key. No hooks
are in force on this machine at the user level.

### Agents

| Location | Contents | Class |
| --- | --- | --- |
| `~/.claude/agents/` | One file, `replay-bakeoff.md`, 4,523 bytes | OBSERVED |
| `/Users/daniel/Development/.claude/agents/` | Does not exist | OBSERVED |

The session also surfaces built-in and plugin-provided agent types (`claude`,
`claude-code-guide`, `Explore`, `general-purpose`, `Plan`, `statusline-setup`) alongside
the one user-defined agent. DERIVED: the effective agent roster is larger than the
on-disk `agents/` directory, so counting files in that directory under-reports what is
available.

### Repository-level configuration

| Path | Contents | Class |
| --- | --- | --- |
| `/Users/daniel/Development/.claude/settings.local.json` | One key: `permissions`, containing `permissions.allow` with a single entry | OBSERVED |
| `/Users/daniel/Development/.claude/scheduled_tasks.lock` | Present | OBSERVED |
| `/Users/daniel/Development/.mcp.json` | Does not exist | OBSERVED |
| Working directory is a git repository | No | OBSERVED |

### MCP servers

Names only. No token, URL credential or header was read or is reproduced.

Connected and active this session: `claude-in-chrome`, `claude.ai Calendly`,
`claude.ai Claude Docs`, `claude.ai Cloudflare Developer Platform`, `claude.ai Gmail`,
`claude.ai Google Calendar`, `claude.ai Google Drive`, `claude.ai HubSpot`,
`claude.ai LunarCrush`, `claude.ai Monte Carlo`, `claude.ai Otter.ai`,
`claude.ai Stack Overflow`, `claude.ai Stripe`, `claude.ai Superhuman Mail`,
`claude.ai Upwork`, `claude.ai Wolfram`, `claude.ai dot.`, `claude.ai v0`.

Failed to connect this session: `claude.ai Unblocked`. OBSERVED error class: the account
is not part of an organization. This is a connection failure, not an absent capability.

`~/.claude/mcp-needs-auth-cache.json` lists 48 server names in a pending-auth state, of
which 44 are plugin-scoped (`plugin:<category>:<server>`, across brand-voice, data,
design, engineering, legal, marketing, product-management, productivity and sales
categories). No configuration file at `~/.claude.json` or `.mcp.json` declares an
`mcpServers` block on this machine, so these servers arrive through the plugin and
connector layer rather than local JSON.

Roughly 18 MCP servers are live in this session against a task that needs only web fetch,
web search and local file reads. The intuitive conclusion is that the other sixteen are
pure context weight. **That conclusion does not survive contact with the documentation.**
Claude Code from v2.1.221 uses tool search, which "discovers and loads only the tools
Claude needs, rather than loading all tools upfront" (see M.4). OBSERVED corroboration:
this session was handed a `ToolSearch` tool and a reminder listing deferred tool names
only, without their schemas.

So the per-connector overhead on this version is the name-level listing plus whatever the
search mechanism costs, not the full tool definitions. The honest statement is that the
cost is unknown and bounded above by the naive figure. The token-counting endpoint prices
a tool-definition block for free and without a generation call, which makes both the upper
bound and the actual figure measurable.

### Skills, commands, plugins

| Surface | OBSERVED |
| --- | --- |
| `~/.claude/skills/` | 12 entries, predominantly Cloudflare-domain skills plus a `synced` directory |
| `~/.claude/commands/` | One file, `agmsg.md`, 17,597 bytes |
| `~/.claude/plugins/` | `installed_plugins.json`, `known_marketplaces.json`, `plugin-catalog-cache.json`, `blocklist.json`, `marketplaces/`, `cache/`, `data/`, `synced/` |
| `~/.claude/statusline/` | `render.py`, `test_render.py`, `DESIGN.md`, `ratelimits.jsonl` |
| `~/.claude/telemetry/` | 6 `1p_failed_events.*.json` files |

## M.2 Provider guidance, recorded as hypothesis

The brief is correct that Anthropic's documentation names /clear, /compact, model
selection, a lean CLAUDE.md, file-path references and planning as usage-management levers.

**This section records that guidance as PROVIDER GUIDANCE. It is not validated optimal
policy.** Every item below is a hypothesis about cost or context behaviour until this
campaign measures it. The guidance is the vendor's, the burden of proof is not discharged
by the vendor stating it, and several of these levers have plausible failure modes that
the guidance does not mention.

The guidance lives under the heading "Reduce token usage" at
[code.claude.com/docs/en/costs](https://code.claude.com/docs/en/costs). Opening line,
verbatim: "Token costs scale with context size: the more context Claude processes, the more
tokens you use."

Verbatim, each item PROVIDER GUIDANCE:

| Lever | Documented wording |
| --- | --- |
| `/clear` | "**Clear between tasks**: Use `/clear` to start fresh when switching to unrelated work. Stale context wastes tokens on every subsequent message." Also "When you want a fresh start instead of continuity, `/clear` costs nothing." |
| `/compact` | "**Add custom compaction instructions**: `/compact Focus on code samples and API usage` tells Claude what to preserve during summarization." And, notably, "`/compact` reads the conversation it summarizes, so compacting a large context is itself a large request." |
| Model selection | "Sonnet handles most coding tasks well and costs less than Opus. Reserve Opus for complex architectural decisions or multi-step reasoning. Use `/model` to switch models mid-session, or set a default in `/config`. ... For simple subagent tasks, specify `model: haiku` in your subagent configuration." |
| Lean CLAUDE.md | "Your CLAUDE.md file is loaded into context at session start. ... Skills load on-demand only when invoked, so moving specialized instructions into skills keeps your base context smaller. **Aim to keep CLAUDE.md under 200 lines by including only essentials.**" The memory page repeats: "target under 200 lines per CLAUDE.md file. Longer files consume more context and reduce adherence." |
| Specific file paths | "**Write specific prompts**: Vague requests like 'improve this codebase' trigger broad scanning. Specific requests like 'add input validation to the login function in auth.ts' let Claude work efficiently with minimal file reads." |
| Plan Mode | "**Use plan mode for complex tasks**: Press Shift+Tab to cycle to plan mode before implementation. Claude explores the codebase and proposes an approach for your approval, preventing expensive re-work when the initial direction is wrong." |

Additional levers documented on the same page and not named in the brief: prefer CLI tools
over MCP servers, "Disable unused servers", use `/context`, use hooks that pre-filter tool
output ("reducing context from tens of thousands of tokens to hundreds"), delegate verbose
operations to subagents, and tune extended thinking via `/effort` or `MAX_THINKING_TOKENS`.

The page also publishes figures. These are vendor claims about a vendor population, not
measurements of this user's workload, and they are recorded as such:

- "average cost is around $13 per developer per active day and $150-250 per developer per
  month"
- "Agent teams use approximately 7x more tokens than standard sessions when teammates run
  in plan mode"
- background and idle usage "typically under $0.04 per session"
- "Unexpectedly high spend ... usually traces back to long sessions that were never cleared
  or to Opus left as the default model."

Note that the documentation itself already concedes the central objection to `/compact`:
the compaction turn reads the whole conversation and is therefore itself expensive. That
sentence is the vendor supplying its own falsifier, and it is the strongest single reason
to measure rather than adopt.

Falsifiers worth registering before measuring, so the result cannot be read backwards
into whatever the numbers say:

| Lever | Provider claim, in outline | Registered falsifier |
| --- | --- | --- |
| `/compact` | Compaction reduces context and therefore cost | If a compaction turn's own input tokens plus the post-compaction re-read exceed the tokens saved before the next natural boundary, compaction is net negative on that session shape. Measure the compaction turn itself, not only the context size after it. |
| `/clear` | Clearing resets context and therefore cost | A clear destroys the prompt cache prefix. If the next turn re-reads the same files, the cache-write multiplier (1.25x or 2x) is paid again on content that would have been a 0.1x read. Clearing can raise cost on a resumed task. |
| Model selection | Cheaper model for mechanical work | Per-token price is not per-task cost. A cheaper model that needs more turns, or that sits on the newer 30%-more-tokens tokenizer, can cost more per completed task. Compare completed tasks, not tokens. |
| Lean CLAUDE.md | Smaller memory file costs less | CLAUDE.md sits in the cached prefix. At a 0.1x read multiplier its marginal per-turn cost is an order of magnitude below its raw token count, and trimming it may buy far less than the character count suggests. |
| File-path references | Naming a file beats naming a directory | Already measured favourably in this user's own prior work on bounded location tasks. That result was for locating, and was explicitly not generalised to why-questions. Do not extend it without measuring. |
| Planning / Plan Mode | Planning first reduces rework | Plan Mode adds a planning turn. The saving is in avoided rework, which is only visible across whole tasks. A per-turn instrument cannot see it, and will read Plan Mode as pure overhead. |

The general defect to guard against: all six levers are plausible, all six are stated
without a published measurement, and measuring them on a handful of sessions will produce
six confident numbers that do not replicate. Register n before starting.

## M.3 Control surfaces: controllable, observable, measurable

Three distinct properties, and the campaign needs all three separated.

- **User-controllable**: the operator can set it.
- **Externally observable**: a tool outside the Claude Code process can see the state.
- **Externally measurable**: a tool outside the process can attach a number to its effect.

The substrate for both observability and measurability on this machine is OBSERVED, and it
is strong. Session transcripts at `~/.claude/projects/<project>/<session>.jsonl` carry one
record per turn, and the records expose control state and usage directly.

Control-state fields OBSERVED as top-level record keys in a live transcript:
`effort`, `perTurnEffort`, `permissionMode`, `mode`, `isCompactSummary`,
`compactMetadata`, `isSidechain`, `model` (under `message`), `sessionId`, `requestId`,
`cwd`, `gitBranch`, `version`, `entrypoint`, `sessionKind`, `userType`, `timestamp`.

Usage fields OBSERVED under `message.usage`:
`input_tokens`, `output_tokens`, `cache_creation_input_tokens`, `cache_read_input_tokens`,
`cache_creation.ephemeral_5m_input_tokens`, `cache_creation.ephemeral_1h_input_tokens`,
`output_tokens_details.thinking_tokens`, `server_tool_use`, `service_tier`, `speed`,
`inference_geo`, and an `iterations` array repeating the token fields per iteration.

That is the full first-party `usage` shape from O.1 and O.6, per turn, on local disk. The
5m and 1h cache-write buckets are separated, which means the 1.25x and 2x multipliers can
be applied correctly rather than averaged.

Sample values OBSERVED in the current session, confirming the fields are populated and not
merely present: `effort` = `high`, `permissionMode` = `auto`, `message.model` =
`claude-opus-5`, and at least one record with `isCompactSummary` = `true`.

Subscription-side limits are also observable locally. The status line receives a stdin
JSON payload, and `~/.claude/statusline/render.py` reads three windows from it:
`five_hour`, `seven_day` and `spend_limit`, each with `used_percentage` and `resets_at`.
`~/.claude/statusline/ratelimits.jsonl` holds 156 appended readings with the shape
`ts`, `session_id`, `model`, `rate_limits.five_hour.used_percentage`,
`rate_limits.five_hour.resets_at`, `rate_limits.seven_day.used_percentage`,
`rate_limits.seven_day.resets_at`.

This is a significant finding in its own right. **The subscription surface is metered on
different dimensions from the API surface.** The API exposes RPM, ITPM and OTPM with
per-minute reset times. The subscription exposes five-hour and seven-day rolling windows
as a used-percentage. They are not the same instrument, and a percentage is not a token
count. Any attempt to convert the subscription percentage into tokens per minute is
unwarranted from what is documented or observed.

| Control | User-controllable | Externally observable | Externally measurable | Basis |
| --- | --- | --- | --- | --- |
| Model selection (`/model`) | Yes | Yes, `message.model` per record | **Yes**, per-turn usage joined to published per-model prices | OBSERVED + DOCUMENTED |
| Effort selection | Yes, `modelSettings.<id>.effortLevel` and per-turn | Yes, `effort` and `perTurnEffort` | **Yes**, `output_tokens_details.thinking_tokens` per turn | OBSERVED |
| `/compact` | Yes | Yes, `isCompactSummary` and `compactMetadata` | **Yes**, token delta across the compaction boundary from adjacent records | OBSERVED |
| `/clear` | Yes | Yes, a new `sessionId` begins | **Yes**, cache-write vs cache-read split on the next turn shows the prefix loss | OBSERVED |
| Prompt caching behaviour | Indirectly | Yes | **Yes**, 5m and 1h write buckets and read tokens are separated per turn | OBSERVED |
| Permission mode / Plan Mode | Yes | Yes, `permissionMode` and `mode` | **Yes**, per-turn usage segmented by mode | OBSERVED |
| Repository state | Yes | Yes, `gitBranch` and `cwd` per record | Yes, usage attributable to branch | OBSERVED |
| Claude Code version | Partly | Yes, `version` per record | Yes, cost compared across versions | OBSERVED |
| Project memory (CLAUDE.md) | Yes | Yes, file on disk with a byte count | **Partly.** Its tokens sit inside the cached prefix and are not itemised per turn. The token-counting endpoint prices the file without a generation call. | OBSERVED + DERIVED |
| Subagents | Yes | Yes, `isSidechain` marks subagent records | Yes, subagent turns separable from main-thread turns | OBSERVED |
| MCP server roster | Yes | Yes, connected names visible | **Partly.** Tool-definition tokens are not itemised per turn. Token counting endpoint gives the per-definition cost. | OBSERVED + DERIVED |
| Skills roster | Yes | Yes, files on disk | Partly, same itemisation gap as MCP | OBSERVED |
| Hooks | Yes | Yes, `settings.json` and `hooks/` | Yes for their side effects. A hook's own execution is outside the model loop and costs no tokens. | OBSERVED |
| Permissions rules | Yes | Yes, `settings.local.json` | Indirect. Effect is on which tools run, not on tokens directly. | OBSERVED |
| `/cost` | Yes, read-only | It reports what the transcript already carries | Yes, but it is a convenience view over the same substrate | OBSERVED |
| `/context` | Yes, read-only | Reports live context composition | Partly. The reading is not persisted to the transcript, so it is not retrospectively auditable. | OBSERVED |
| API key vs subscription auth | Yes | Partly. `service_tier` appears in usage. | **Asymmetric.** API-key usage is measurable in tokens and dollars. Subscription usage is measurable only as a percentage of five-hour and seven-day windows. | OBSERVED |
| IDE integration | Yes | `entrypoint` distinguishes it | Yes, usage segmented by entrypoint | OBSERVED |

### The measurable set

DERIVED from the table. Controls that are both user-controllable and externally measurable
in tokens and dollars, with no instrumentation beyond reading the transcript JSONL:

model selection, effort selection, `/compact`, `/clear`, prompt-caching behaviour,
permission mode including Plan Mode, subagent use, repository state, and Claude Code
version.

Controls that are user-controllable and observable but **not** cleanly measurable per turn:
CLAUDE.md size, the MCP server roster and the skills roster. All three contribute to the
cached prompt prefix and none is itemised in the per-turn `usage` object. The token
counting endpoint is the right instrument for these, because it prices a block of content
without a generation call, and this campaign forbids paid calls.

### Instrument warnings

- Reading `input_tokens` as total input is wrong whenever caching is active. See O.1.
  Total is `cache_read_input_tokens + cache_creation_input_tokens + input_tokens`.
- A subscription `used_percentage` is not a token count and must not be converted into one.
- `/context` output is not persisted to the transcript. A claim resting on it is not
  auditable after the fact unless it is captured at the time.
- Transcript retention is bounded by `cleanupPeriodDays` in settings.json. A longitudinal
  measurement can lose its own history. Copy the transcripts before the window closes.
- A control whose effect cannot be made to move the number is not being measured. Change
  the setting, confirm the metric moves, and only then trust the instrument.

## M.4 Control surface documentation

All DOCUMENTED, all fetched 2026-09-29 from `code.claude.com/docs/en/`.

### Slash commands

The commands are documented at
[/docs/en/commands](https://code.claude.com/docs/en/commands). The ones named in the brief,
with their documented behaviour:

| Command | Documented wording |
| --- | --- |
| `/clear [name]` | "Start a new conversation with empty context. ... To free up context while continuing the same conversation, use `/compact` instead." |
| `/compact [instructions]` | "Free up context by summarizing the conversation so far." |
| `/context [all]` | "Visualize current context usage as a colored grid. Shows optimization suggestions for context-heavy tools, memory bloat, and capacity warnings." |
| `/cost` | "Alias for `/usage`." |
| `/usage` | "Show session cost, plan usage limits, and activity stats... `/cost` and `/stats` are aliases." |
| `/model [model]` | "Switch the AI model and save it as your default for new sessions." |
| `/effort [level]` | Levels low through xhigh, plus `max` and `auto` |
| `/permissions` | "Manage allow, ask, and deny rules for tool permissions." |
| `/plan`, `/config`, `/mcp`, `/agents`, `/insights`, `/rewind` | documented |

Structural change worth noting: custom commands and skills have merged. "A file at
`.claude/commands/deploy.md` and a skill at `.claude/skills/deploy/SKILL.md` both create
`/deploy`." OBSERVED corroboration: this machine has one file in `~/.claude/commands/`
and twelve entries in `~/.claude/skills/`, and both surfaces are live.

### settings.json

[/docs/en/settings](https://code.claude.com/docs/en/settings) and
[/docs/en/settings-reference](https://code.claude.com/docs/en/settings-reference).
Approximately 200 top-level keys are documented. Precedence, highest to lowest: managed
settings, then `claude --settings`, then `.claude/settings.local.json`, then
`.claude/settings.json`, then `~/.claude/settings.json`.

Keys directly relevant to context and cost control: `model`, `modelOverrides`,
`modelSettings`, `modelPricing`, `availableModels`, `fallbackModel`, `effortLevel`,
`maxEffortLevel`, `ultracode`, `alwaysThinkingEnabled`, `autoCompactEnabled`,
`autoCompactWindow`, **`promptCacheTtl`**, **`subagentPromptCacheTtl`**, `permissions`,
`hooks`, `disableAllHooks`, `env`, `apiKeyHelper`, `claudeMd`, `claudeMdExcludes`,
`autoMemoryEnabled`, `cleanupPeriodDays`, `enabledPlugins`, `teammateDefaultModel`,
`skillOverrides`, `disableBundledSkills`, `autoContinueAtUsageLimit`.

`promptCacheTtl` and `subagentPromptCacheTtl` are the ones to notice. The 1.25x versus 2x
cache-write multiplier from O.6 is under direct operator control from settings.json, which
makes cache TTL a first-class experimental variable rather than a fixed property.

Environment variables of interest: `MAX_THINKING_TOKENS`, `CLAUDE_CODE_SUBAGENT_MODEL` and
`CLAUDE_CODE_SUBAGENT_MODEL_FORCE`, `CLAUDE_CODE_ENABLE_TELEMETRY`, `ANTHROPIC_API_KEY`,
`ANTHROPIC_AUTH_TOKEN`, `CLAUDE_CODE_OAUTH_TOKEN`, `CLAUDE_CONFIG_DIR`, `ENABLE_TOOL_SEARCH`,
`MAX_MCP_OUTPUT_TOKENS` (default 25,000).

UNKNOWN: there is no key named `autoCompactThreshold`. The nearest real key is
`autoCompactWindow`.

### Hooks

[/docs/en/hooks](https://code.claude.com/docs/en/hooks). **33 events** are documented:
SessionStart, Setup, UserPromptSubmit, UserPromptExpansion, PreToolUse, PermissionRequest,
PermissionDenied, PostToolUse, PostToolUseFailure, PostToolBatch, Notification,
MessageDisplay, SubagentStart, SubagentStop, TaskCreated, TaskCompleted, Stop, StopFailure,
TeammateIdle, PreCompact, PostCompact, PreModelSwitch, PostModelSwitch, CwdChanged,
DirectoryAdded, FileChanged, WorktreeCreate, WorktreeRemove, ConfigChange, Elicitation,
ElicitationResult, SessionEnd, InstructionsLoaded.

A hook receives on stdin: `session_id`, `prompt_id`, `transcript_path`, `cwd`,
`scratchpad_dir`, `permission_mode`, `effort`, `hook_event_name`, `agent_id`, `agent_type`,
plus event-specific fields. Exit 0 is success, exit 2 is a blocking error, anything else is
non-blocking. JSON output can carry `permissionDecision`, `updatedInput`,
`updatedToolOutput` and `additionalContext`.

This is the single most important finding for external measurability. `PreCompact` and
`PostCompact` bracket a compaction, `PreModelSwitch` and `PostModelSwitch` bracket a model
change, and every hook is handed `transcript_path`, `permission_mode` and `effort`. An
external measurement harness can therefore mark its own boundaries in real time without
polling, and without any change to Claude Code itself. Note OBSERVED: no hooks are
currently configured on this machine, so this capability is available and unused.

### Subagents

[/docs/en/sub-agents](https://code.claude.com/docs/en/sub-agents). Markdown files with YAML
frontmatter. Resolution order: managed settings, `--agents` flag, `.claude/agents/`,
`~/.claude/agents/`, plugin `agents/`.

Frontmatter fields: `name` and `description` required, then `tools`, `disallowedTools`,
`model` (`sonnet` / `opus` / `haiku` / `fable`, a full model ID, or `inherit`),
`permissionMode`, `maxTurns`, `skills`, `mcpServers`, `hooks`, `memory`, `background`,
`omitClaudeMd`, `effort`, `isolation`, `color`, `initialPrompt`, and `experimental` with
`cacheTtl` of `5m` or `1h`.

Subagent model resolution: per-invocation override, then frontmatter, then
`CLAUDE_CODE_SUBAGENT_MODEL`, then the main conversation model. `omitClaudeMd` is a direct
lever on the CLAUDE.md cost question from M.2.

### MCP

[/docs/en/mcp](https://code.claude.com/docs/en/mcp). Scopes: **local** in `~/.claude.json`
and the default, **project** in `.mcp.json` and version-controlled, **user** in
`~/.claude.json` across all projects. Transports: `http` (recommended, alias
`streamable-http`), `sse` (deprecated), `stdio`, `ws`.

Directly relevant to the connector-overhead question in M.1, DOCUMENTED: "Claude Code uses
tool search ... discovers and loads only the tools Claude needs, rather than loading all
tools upfront". Default on from v2.1.221 with Claude 4.5-generation models, disabled with
`ENABLE_TOOL_SEARCH=false`.

OBSERVED corroboration: this session's tool list is deferred, and the harness surfaced a
`ToolSearch` tool plus a system reminder naming deferred tools by name only. So the naive
"every connected server loads its full tool definitions" model is **wrong on this version**,
and the DERIVED cost estimate in M.1 should be read as an upper bound, not a figure. What
tool search actually costs per session is a measurement this campaign should make rather
than assume in either direction.

### Permissions

[/docs/en/permission-modes](https://code.claude.com/docs/en/permission-modes) and
[/docs/en/permissions](https://code.claude.com/docs/en/permissions). Six modes: `default`
(labelled Manual, alias `manual`), `acceptEdits`, `plan`, `auto`, `dontAsk`,
`bypassPermissions`.

DOCUMENTED: "With Claude Code v2.1.283 or later, auto mode is the built-in starting
permission mode for interactive terminal and VS Code sessions." OBSERVED: this machine runs
2.1.278, has `autoMode` configured in settings.json, and the live transcript records
`permissionMode` = `auto`.

Rule evaluation, DOCUMENTED verbatim: "Rules are evaluated in order: deny, then ask, then
allow. The first match in that order determines the outcome, and rule specificity doesn't
change the order." A bare tool name in `deny` removes the tool from context entirely, which
makes `deny` a context-size lever as well as a safety one. `auto` and `bypassPermissions`
cannot be set from `.claude/settings.json` or `.claude/settings.local.json`.

### API key versus subscription authentication

[/docs/en/iam](https://code.claude.com/docs/en/iam). Credential precedence, highest first:
cloud provider variables, `ANTHROPIC_AUTH_TOKEN`, `ANTHROPIC_API_KEY`, `apiKeyHelper`,
`CLAUDE_CODE_OAUTH_TOKEN`, Anthropic profile or federation, then "Subscription OAuth
credentials from `/login`. This is the default for Claude Pro, Max, Team, and Enterprise
users."

DOCUMENTED: "If you have an active Claude subscription but also have `ANTHROPIC_API_KEY` set
in your environment, Claude Code uses the API key once you approve it." An operator who
believes they are on a subscription can be silently billing an API key. `unset
ANTHROPIC_API_KEY` and `/status` are the documented remedy and check.

The two authentication paths are metered on genuinely different instruments:

| | Subscription | API key / Console |
| --- | --- | --- |
| Meter | Per-seat allowance on "a rolling five-hour window and a weekly window" | Per-token billing |
| Shared with | Claude chat and Cowork | Workspace spend limits |
| Model switching as an escape | No. The seat limit is "shared across all models, so the developer can't restore access by switching models with `/model`" | n/a |
| Rate limit interaction | n/a | "Claude Code traffic in this workspace counts toward your organization's overall API rate limits" |
| Published sizing | n/a | 1-5 users: 200k-300k TPM. 500+ users: 10k-15k TPM. |
| **Prompt cache TTL** | **1 hour**, dropping to 5 minutes once drawing on usage credits | **5 minutes** by default |
| `/usage-credits` | Available | "isn't available with API key authentication" |
| `/usage` session cost | "isn't relevant for billing purposes" for Max and Pro | "intended for API users" |

The cache TTL row is the sharpest asymmetry, and it is easy to miss. DOCUMENTED verbatim:
"The lifetime is an hour on a subscription and drops to five minutes once you're drawing on
usage credits; on an API key or cloud provider, it's five minutes by default." An identical
workload has a different cache hit profile on a subscription than on an API key, which
means any cost or context experiment must hold the authentication method fixed or it is
measuring the auth method rather than the variable under test.

This also corroborates the OBSERVED local finding in M.3 exactly. The documentation names a
rolling five-hour window and a weekly window. `~/.claude/statusline/ratelimits.jsonl` on
this machine records `five_hour` and `seven_day`. Documentation and local observation agree
on the shape of the subscription meter, independently.

### OpenTelemetry: the external measurement channel

[/docs/en/monitoring-usage](https://code.claude.com/docs/en/monitoring-usage). DOCUMENTED
verbatim from the costs page: "OpenTelemetry export works on every setup and is the only
option that streams per-user token and cost metrics into your own observability stack in
near real time."

Enable with `CLAUDE_CODE_ENABLE_TELEMETRY=1` plus `OTEL_METRICS_EXPORTER`
(otlp / prometheus / console / none), `OTEL_LOGS_EXPORTER`, `OTEL_TRACES_EXPORTER` (beta),
and the standard `OTEL_EXPORTER_OTLP_*` variables. Default export interval 60,000 ms.

Metrics: `claude_code.session.count`, `claude_code.lines_of_code.count`,
`claude_code.pull_request.count`, `claude_code.commit.count`, **`claude_code.cost.usage`**
(USD), **`claude_code.token.usage`**, `claude_code.code_edit_tool.decision`,
`claude_code.active_time.total`.

Events include `claude_code.user_prompt`, `.assistant_response`, `.tool_result`,
`.api_request`, `.api_error`, `.api_refusal`, `.tool_decision`,
**`.permission_mode_changed`**, `.auth`, `.mcp_server_connection`, `.skill_activated`,
**`.compaction`**, **`.subagent_completed`**, `.hook_execution_start`,
`.hook_execution_complete`, `.api_retries_exhausted`.

Attributes: `session.id`, `app.version`, `app.entrypoint`, `organization.id`,
`user.account_uuid`, `user.id`, `user.email`, `terminal.type`, `vcs.*`.

Content is redacted by default. User agents, skills and plugins are reported as `"custom"`
unless `OTEL_LOG_TOOL_DETAILS=1`, which is a real limitation for anyone measuring the cost
of their own custom agents. Content cap is 61,440 units.

DERIVED: the events list closes the measurability question for the levers in M.2. A
`.compaction` event, a `.permission_mode_changed` event and a `.subagent_completed` event,
each carrying `session.id`, joined against `claude_code.token.usage` and
`claude_code.cost.usage`, measure `/compact`, Plan Mode and subagent delegation without
touching the model. This is a second, independent channel to the transcript JSONL in M.3,
which matters because two instruments that disagree are how a false green gets caught.

### Instrument caveat on `/usage` and `/cost`

DOCUMENTED, and it disqualifies these commands as the primary meter:

- Cost figures are "computed locally from token counts at list price" and are "an estimate",
  not an invoice.
- The plan breakdown is "approximate and computed from local session history on this
  machine". Other devices and claude.ai usage are excluded.
- The `modelPricing` setting, available in managed settings only, "changes reporting, not
  billing". An operator can therefore make `/cost` report any number they like without
  changing a cent of actual spend.

That last point is the one to carry forward. `/cost` is a locally computed, locally
configurable estimate. It is a convenience view, not evidence. Any figure this campaign
publishes should come from the transcript `usage` object or from OpenTelemetry, with the
model's published prices applied explicitly, so the arithmetic is auditable.

---

# Sources

All fetched 2026-09-29. Zero API calls were made against any Anthropic or DeepSeek
endpoint in producing this document.

### Fetched and read (API and cloud platforms)

- [Rate limits](https://platform.claude.com/docs/en/api/rate-limits)
- [Pricing](https://platform.claude.com/docs/en/about-claude/pricing)
- [Claude in Microsoft Foundry](https://platform.claude.com/docs/en/build-with-claude/claude-in-microsoft-foundry)
- [Claude in Amazon Bedrock (current Messages API integration)](https://platform.claude.com/docs/en/build-with-claude/claude-in-amazon-bedrock)
- [Claude on Amazon Bedrock (Opus 4.6 and earlier, legacy integration)](https://platform.claude.com/docs/en/build-with-claude/claude-on-amazon-bedrock-legacy)
- [Claude on Google Cloud (Vertex AI / Agent Platform)](https://platform.claude.com/docs/en/build-with-claude/claude-on-vertex-ai)
- [Claude Platform on AWS](https://platform.claude.com/docs/en/build-with-claude/claude-platform-on-aws)
- [Features overview](https://platform.claude.com/docs/en/build-with-claude/overview)
- [Token counting](https://platform.claude.com/docs/en/build-with-claude/token-counting)

### Fetched and read (Claude Code)

Host note: these live at `code.claude.com`, not `platform.claude.com`. See M.0.

- [Costs and usage management](https://code.claude.com/docs/en/costs)
- [Commands](https://code.claude.com/docs/en/commands)
- [Settings](https://code.claude.com/docs/en/settings)
- [Settings reference](https://code.claude.com/docs/en/settings-reference)
- [Hooks](https://code.claude.com/docs/en/hooks)
- [Memory](https://code.claude.com/docs/en/memory)
- [Subagents](https://code.claude.com/docs/en/sub-agents)
- [MCP](https://code.claude.com/docs/en/mcp)
- [IAM and authentication](https://code.claude.com/docs/en/iam)
- [Permissions](https://code.claude.com/docs/en/permissions)
- [Permission modes](https://code.claude.com/docs/en/permission-modes)
- [Monitoring usage (OpenTelemetry)](https://code.claude.com/docs/en/monitoring-usage)
- [Machine-readable doc index](https://code.claude.com/docs/llms.txt)

### Attempted and failed

- `https://platform.claude.com/docs/en/api/claude-on-microsoft-foundry` returned HTTP 404.
  Correct path found by search and recorded above.
- `https://platform.claude.com/docs/en/docs/claude-code/*` returns HTTP 404 for every
  Claude Code page. See M.0.
- `https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/partner-models/claude/quotas`
  returned only a JavaScript navigation shell. Its content is recorded in C.5 as
  DOCUMENTED-BUT-UNVERIFIED, sourced from indexed page text rather than a verified fetch.
- `https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/partner-models/claude/claude-quotas`
  returned HTTP 404.

### Cited in-line, referenced by the pages above, not independently fetched

- [Rate Limits API](https://platform.claude.com/docs/en/manage-claude/rate-limits-api)
- [Prompt caching](https://platform.claude.com/docs/en/build-with-claude/prompt-caching)
- [Batch processing](https://platform.claude.com/docs/en/build-with-claude/batch-processing)
- [Fast mode](https://platform.claude.com/docs/en/build-with-claude/fast-mode)
- [Data residency](https://platform.claude.com/docs/en/manage-claude/data-residency)
- [Workspaces](https://platform.claude.com/docs/en/manage-claude/workspaces)
- [Model deprecations](https://platform.claude.com/docs/en/about-claude/model-deprecations)
- [Errors](https://platform.claude.com/docs/en/api/errors)
- [Amazon Bedrock pricing (AWS)](https://aws.amazon.com/bedrock/pricing/)
- [Google Cloud generative AI pricing](https://cloud.google.com/vertex-ai/generative-ai/pricing#claude-models)

### Local inspection

Shape only. No credential, token, API key or configuration value that could carry a secret
was read into this document or printed to any log. `settings.json` was parsed for key
paths with `jq`; values were never emitted. MCP servers are listed by name only.

- `~/.claude/CLAUDE.md`
- `~/.claude/settings.json` (key paths only)
- `~/.claude/agents/`
- `~/.claude/commands/`, `~/.claude/skills/`, `~/.claude/plugins/`
- `~/.claude/statusline/render.py` and `~/.claude/statusline/ratelimits.jsonl` (field names only)
- `~/.claude/mcp-needs-auth-cache.json` (server names only)
- `~/.claude/projects/<project>/<session>.jsonl` (field names and control-state values only)
- `/Users/daniel/Development/.claude/settings.local.json` (key paths only)
