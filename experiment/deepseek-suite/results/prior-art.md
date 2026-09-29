# Section 10: Mechanism flagging and prior-art reconnaissance

Date: 2026-09-28. Method: WebSearch and WebFetch only. Zero DeepSeek API calls were made
in producing this document.

## Scope and disclaimer

This document makes **no patentability conclusion and no novelty claim**. It asks a narrower
question for each of six candidate mechanisms observed in this campaign:

1. Is the observed behaviour concrete and implementation-specific enough to justify spending
   time on a prior-art search at all?
2. What prior art was actually found, with URLs?

The working prior is that each candidate is already well covered by published work. The
reconnaissance below largely confirms that prior. Where a search failed to find something, the
document says NOT_VERIFIED rather than asserting absence of prior art. Absence of a found
reference is evidence about the search, not about the field.

## Correction to the campaign's framing, found during this search

The campaign background states that the vendor "documents fixed token intervals and gives no
interval". That is not accurate as written. DeepSeek's own launch announcement states
verbatim:

> "The cache system uses 64 tokens as a storage unit; content less than 64 tokens will not be
> cached."

Source: <https://api-docs.deepseek.com/news/news0802/>, fetched 2026-09-28.

The current guide page at <https://api-docs.deepseek.com/guides/kv_cache/> does use the softer
wording, saying cache prefix units are created "at the end position of the user input and the
end position of the model output" and "at fixed token intervals" for long inputs, plus
"A subsequent request can only hit the cache if it fully matches a cache prefix unit" and
"The cache system works on a 'best-effort' basis and does not guarantee a 100% cache hit rate."

So the vendor documents a 64-token unit in one place and an unnamed interval in another. The
campaign measured 128-token quantisation with the final block withheld, on deepseek-flash.
That is a factor-of-two discrepancy against a published vendor number, and it is the single
most defensible measurement in the set precisely because there is a published number to
contradict. It should be reported as a discrepancy against documentation, not as an
undocumented mechanism. Possible explanations not distinguished by current evidence:
per-model differences, a change since the 2024 announcement, or a two-unit write granularity.
NOT_VERIFIED which.

## Part 1 and Part 2: candidate records with prior art found

### (a) Cache-aware agent execution scheduling (warm-then-fan as a scheduler primitive)

**Observed mechanism.** Concurrently issued requests sharing a prefix all missed the prefix
cache (0% hit rate). Warming a single request to completion and then fanning out the remainder
produced approximately 86% hit rate. Parallel-only was 11.5x faster at 0.99x the cost.

**What is technically specific about it.** Only the measurement is specific: the 0% versus 86%
contrast on this provider, and the finding that the latency/cost tradeoff on this workload
favoured parallel-only because cache savings did not outweigh the serialisation cost. The
scheduling pattern itself is not specific.

**What generic prior art likely covers it.** Cache warming before fan-out is published,
named practice with implementations in the wild.

**Closest prior art actually found.**

- "Prompt Caching Works. Your Prompt Assembly Code Does Not." describes exactly this mechanism:
  "When two parallel requests arrive simultaneously against a prefix that has no existing cache
  entry, both trigger a cache write", and prescribes "a single synchronous call to establish the
  cache entry before the parallel batch is dispatched". It cites a 7% to 85% hit-rate
  improvement, closely mirroring the 0% to 86% observed here.
  <https://dev.to/parag_d/prompt-caching-works-your-prompt-assembly-code-does-not-5edc>
- "Pre-Warm Your Anthropic Prompt Cache Before the Traffic Hits" describes a `PromptWarmer`
  scheduler component with prefix-keyed dedup, a 10-second TTL, 300 ms debounce and
  single-threaded execution to prevent concurrent warming requests flushing slot caches.
  <https://dev.to/mukundakatta/pre-warm-your-anthropic-prompt-cache-before-the-traffic-hits-oj6>
- Cache-aware scheduling as a server-side scheduler primitive predates both: SGLang's
  RadixAttention with LPM and DFS_WEIGHT scheduling policies, and the vLLM production stack's
  cache-aware and load-aware routers.
  <https://docs.vllm.ai/projects/production-stack/en/latest/use_cases/loadaware-routing.html>
  <https://github.com/vllm-project/vllm/issues/11477>

**Plain statement: this is anticipated.** A named, implemented cache-warming scheduler
component with the same race-condition diagnosis and the same corrective pattern is published.

**What additional experiment would establish a stronger mechanism.** Nothing about the pattern.
The defensible contribution would be a decision rule: measure the crossover point at which
warm-then-fan beats parallel-only as a function of fan-out width, shared prefix length and the
cache-read discount, and show the rule picks correctly out of sample. The current data point
says warm-then-fan lost on this workload, which is the more interesting result.

**Next prior-art search.** Search for warm-then-fan crossover analyses specifically, and for
patent filings by inference-gateway vendors (Portkey, Helicone, Cloudflare AI Gateway,
LiteLLM) on cache pre-warming. NOT_VERIFIED: no patent database search was performed.

### (b) Evidence-backed model/reasoning routing by task class

**Observed mechanism.** Disabling reasoning is class-conditional. Lookup tasks scored 17/18
with reasoning disabled versus 18/18 enabled, at 19.5x fewer output tokens. Aggregation tasks
scored 7/24 disabled versus 24/24 enabled. Failures were confidently wrong, not refusals.

**What is technically specific about it.** The measurement, on this model family, with the
failure-mode characterisation (confident wrongness rather than refusal, which means a
refusal-detector-based cascade would not catch it). The routing idea is not specific.

**What generic prior art likely covers it.** The entire LLM routing and cascade literature.

**Closest prior art actually found.**

- FrugalGPT (Chen, Zaharia, Zou, 2023), the canonical cascade-by-cost-with-a-learned-scorer
  paper. <https://arxiv.org/abs/2305.05176>
- RouterBench, a benchmark of 405k precomputed inference outcomes across 11 models and 7 task
  families, explicitly built for cost/quality routing analysis. <https://arxiv.org/abs/2403.12031>
- Route-To-Reason, which routes over **both** model and reasoning strategy jointly under a
  budget. This is the closest match to the specific idea of routing the reasoning switch rather
  than the model. <https://arxiv.org/html/2505.19435>
- "Dynamic Model Routing and Cascading for Efficient LLM Inference: A Survey"
  <https://arxiv.org/pdf/2603.04445>

**Plain statement: this is anticipated.** Route-To-Reason routes reasoning strategy by task
difficulty under budget. The observation that reasoning helps on multi-step aggregation and not
on lookup is the standard result in that literature, not a new one.

**What additional experiment would establish a stronger mechanism.** The confidently-wrong
failure mode is the part that is not obviously in the cascade literature, because cascades
usually assume a scorer can detect a bad cheap answer. An experiment that shows standard
cascade scorers (self-reported confidence, verifier LLM) fail to detect these specific failures
would be a genuine negative result about cascade design.

**Next prior-art search.** Search for cascade scorer failure under confident hallucination, and
for reasoning-toggle routing on hybrid reasoning models specifically.

### (c) Runtime cost/correctness optimization using independently verified outcomes

**Observed mechanism.** Using externally graded correctness (not model self-report) as the
signal that drives a cost/quality decision at runtime.

**What is technically specific about it.** Nothing identified. As stated, this is the definition
of an online, feedback-driven router.

**What generic prior art likely covers it.** Contextual bandit and online model selection
literature, where learning from observed outcome of the chosen arm is the defining setup.

**Closest prior art actually found.**

- "Learning to Route LLMs from Bandit Feedback: One Policy, Many Trade-offs", which explicitly
  targets the deployment case where "only the outcome of the chosen model is observed".
  <https://arxiv.org/abs/2510.07429>
- "Online Multi-LLM Selection via Contextual Bandits under Unstructured Context Evolution"
  <https://arxiv.org/pdf/2506.17670>
- "Cost-Effective Online Multi-LLM Selection with Versatile Reward Models"
  <https://arxiv.org/pdf/2405.16587>
- FrugalGPT's learned scorer is the same construct in offline-trained form.
  <https://arxiv.org/abs/2305.05176>

**Plain statement: this is anticipated, and it is the weakest of the six.** "Optimise cost and
correctness at runtime using verified outcomes" is a restatement of the contextual bandit
formulation of LLM routing. There is no implementation-specific detail here to search against.

**What additional experiment would establish a stronger mechanism.** Nothing would rescue the
candidate at this level of abstraction. It would have to be narrowed to a specific verifier, a
specific reward shaping, and a demonstrated regret bound or measured out-of-sample win against a
bandit baseline.

**Next prior-art search.** Not recommended. Retire the candidate.

### (d) Reconstruction of provider cost from heterogeneous runtime evidence across differing usage dialects

**Observed mechanism.** 479 saved responses reconstructed to $0.22995 derived against an
observed balance delta of $0.23, using provider-reported usage fields.

**What is technically specific about it.** The reconciliation against an observed account
balance delta is the specific part, and it is a good verification practice. The dialect problem
(inclusive versus exclusive cached-token counting) is specific and real but well known.

**What generic prior art likely covers it.** Every LLM observability and gateway product
computes cost from usage fields against a pricing table, and the inclusive-versus-exclusive
normalisation is an explicitly documented contract in at least one of them.

**Closest prior art actually found.**

- Langfuse documents an explicit exclusive-buckets contract and normalises OpenAI-schema usage
  objects and OpenTelemetry `gen_ai.usage` attributes at ingestion. This directly anticipates the
  dialect-reconciliation part.
  <https://langfuse.com/docs/observability/features/token-and-cost-tracking>
  <https://langfuse.com/resources/engineering/llm-cost-management>
- LiteLLM returns `response_cost` on every call from a maintained pricing repository, with a
  `cost_per_token` primitive and custom pricing support.
  <https://docs.litellm.ai/docs/completion/token_usage>
  <https://docs.litellm.ai/docs/proxy/pricing_calculator>
- tokonomics and tokencost-family libraries do exactly this reconstruction from usage fields.
  <https://github.com/phil65/tokonomics>
- The inclusive-versus-exclusive dialect difference is documented as a live, filed bug class
  across multiple projects: OpenAI's `prompt_tokens` includes `prompt_tokens_details.cached_tokens`
  while Anthropic's `input_tokens` and `cache_read_input_tokens` are disjoint.
  <https://github.com/continuedev/continue/issues/13104>
  <https://www.prompthub.us/blog/prompt-caching-with-openai-anthropic-and-google-models>
- OpenTelemetry GenAI semantic conventions standardise `gen_ai.usage.input_tokens` and
  `gen_ai.usage.output_tokens`; cost is explicitly a derived quantity computed at emit time
  against a pricing table the user controls, not a standardised attribute.
  <https://opentelemetry.io/docs/specs/semconv/registry/attributes/gen-ai/>
  <https://opentelemetry.io/blog/2026/genai-observability/>

**Plain statement: the mechanism is anticipated; the verification is the contribution.** What
was not found in any of the above is a published closed-loop check of a reconstructed figure
against an observed provider balance delta. Langfuse's own material notes that cloud cost tools
reconcile an invoice after the fact whereas LLM cost is computed per request, which implies the
reconciliation is usually not performed. The $0.22995 against $0.23 agreement at n=479 is a
verification result about a standard mechanism. NOT_VERIFIED that no tool performs this check;
only that no such tool was found in this search.

**What additional experiment would establish a stronger mechanism.** Run the same reconciliation
across at least two providers with opposite usage dialects and show the reconstruction agrees
with each balance delta under one normalisation. That would make the dialect normalisation
falsifiable rather than assumed. Also: register the residual. $0.22995 versus $0.23 is agreement
to the precision of the reported balance, which may be rounded. State the balance's precision
before claiming agreement.

**Next prior-art search.** Search for published billing-reconciliation audits of LLM gateways,
and for FinOps FOCUS specification coverage of GenAI token cost. NOT_VERIFIED: not searched.

### (e) Intervention based on observed cache population state

**Observed mechanism.** Reading cache state at runtime (for example `prompt_cache_hit_tokens`)
and changing scheduling behaviour in response.

**What is technically specific about it.** Nothing beyond (a). The provider exposes the signal
directly as a documented usage field, so consuming it is the intended use.

**What generic prior art likely covers it.** Cache-aware routing, where the router tracks KV
cache state per replica and dispatches to maximise hit rate, is shipped product in several
stacks. Client-side, the signal is a documented response field.

**Closest prior art actually found.**

- DeepSeek documents `prompt_cache_hit_tokens` and `prompt_cache_miss_tokens` as response usage
  fields, so observing cache population state is the documented interface.
  <https://api-docs.deepseek.com/guides/kv_cache/>
- Ray Serve prefix-aware routing. <https://docs.ray.io/en/latest/serve/llm/user-guides/prefix-aware-routing.html>
- GKE Inference Gateway KV-cache-aware routing, and Alibaba's precise-mode prefix cache-aware
  routing, both of which dispatch on observed cache block state.
  <https://www.spheron.network/blog/gke-inference-gateway-kv-cache-aware-llm-routing/>
  <https://help.aliyun.com/en/cs/user-guide/kvcache-aware-load-balancing-using-intelligent-inference-routing>
- vLLM production stack load-aware routing weighs cache-hit benefit against live load with a
  tunable beta. <https://docs.vllm.ai/projects/production-stack/en/latest/use_cases/loadaware-routing.html>
- The timing-side-channel literature is the black-box version: inferring cache state from TTFT
  without any usage field. "Auditing Prompt Caching in Language Model APIs" audited 17 providers
  including DeepSeek and detected caching in 8, with global cross-user sharing in 7.
  <https://arxiv.org/abs/2502.07776>
  <https://arxiv.org/html/2409.20002v2>

**Plain statement: this is anticipated, and it is close to a duplicate of (a).** Acting on
observed cache state is the entire premise of cache-aware routing, which is shipped in at least
four independent serving stacks.

**What additional experiment would establish a stronger mechanism.** Merge with (a). The only
distinct thing here is client-side use of a provider-reported field to drive client-side
scheduling against a black-box endpoint, where server-side cache state is not visible. A
measurement of how well the reported field predicts the next request's hit rate would be the
thing to test.

**Next prior-art search.** Search for client-side cache-state-driven request scheduling against
third-party APIs specifically, as distinct from server-side routing.

### (f) Evidence-driven fan-out scheduling using a measured cache block size

**Observed mechanism.** cached = 128 * max(0, floor(shared_prefix_tokens/128) - 1), exact on
17 of 17 rungs. Combined with byte-exact cache identity: a single leading space dropped the hit
rate to zero, while temperature and max_tokens changes did not affect it.

**What is technically specific about it.** This is the most specific of the six. The closed-form
with the minus-one term (the final block is never served) is a concrete, falsifiable claim about
a black-box endpoint, it was measured exactly on 17 rungs, and it contradicts a published vendor
number of 64 tokens. Using a measured block size to size prompt padding or fan-out batches is a
concrete downstream use.

**What generic prior art likely covers it.** Two separate bodies of work, both of which cover
large parts of it.

**Closest prior art actually found.**

- **Block quantisation with the partial final block excluded is documented behaviour in vLLM**,
  the reference open-source implementation. vLLM's prefix caching design states that only full
  blocks are cached, that the default block size is 16 tokens, and that with a 17-token message
  "the last token might not be cached". It further states the last block of a request "must hash
  more tokens and is less likely to be reused" and "should be evicted first". The measured
  DeepSeek formula is the same structural behaviour at a different block size.
  <https://docs.vllm.ai/en/stable/design/prefix_caching/>
  <https://github.com/vllm-project/vllm/blob/main/docs/design/prefix_caching.md>
- **The vendor publishes a block size.** 64 tokens, minimum cacheable unit, see the correction
  section above. <https://api-docs.deepseek.com/news/news0802/>
- **Block-boundary-aware prompt construction is published practice**, including explicit
  proposals to pad prefixes to the next block boundary so short prompts can be cached, and
  aligning special tokens to block boundaries with padding tokens.
  <https://github.com/vllm-project/vllm/issues/40696>
  <https://arxiv.org/pdf/2511.02749>
- **Black-box measurement of remote cache behaviour is an established methodology.** "Auditing
  Prompt Caching in Language Model APIs" uses two-sample Kolmogorov-Smirnov tests on TTFT
  distributions with a `PrefixFraction` parameter to test prefix-only versus exact matching, and
  audited DeepSeek among 17 providers. <https://arxiv.org/abs/2502.07776>
  The `NumVictimRequests` parameter in that work (1, 5 or 25 consecutive requests before probing)
  is the same concern as the campaign's completion-dependent population finding in (a).
- CacheProbe audits prompt cache isolation in gateway APIs. <https://arxiv.org/pdf/2605.30613>
- InputSnatch and "The Early Bird Catches the Leak" recover prompt content by token-by-token
  TTFT probing, which requires resolving cache granularity in practice.
  <https://link.springer.com/chapter/10.1007/978-981-92-4805-6_32>
  <https://arxiv.org/pdf/2409.20002>

**Plain statement: the mechanism is anticipated; the specific measured value is a documentation
discrepancy worth reporting.** Block quantisation with the final block withheld is documented
vLLM behaviour. Block-boundary padding is proposed practice. Black-box cache auditing is a
published methodology applied to this exact provider. What is left is a number, 128, that
disagrees with the vendor's published 64 on the model tested. That is a bug report or a
measurement note, not a mechanism.

**What additional experiment would establish a stronger mechanism.** Three things, in order of
value:

1. Re-run the rung sweep on a second DeepSeek model to test whether 128 is model-specific or
   whether the published 64 is stale. This directly tests the discrepancy.
2. Register the falsifier before running: state what result would mean the 128 figure is an
   artefact of the tokeniser boundary or of the probe construction rather than of the cache.
3. Test whether padding the shared prefix to a 128 boundary recovers the withheld final block,
   which is the only part with a downstream engineering consequence.

**Next prior-art search.** Search for prior published measurements of DeepSeek cache granularity
specifically, and check whether the "Auditing Prompt Caching" artefacts include per-provider
granularity estimates. NOT_VERIFIED: the paper's appendix and released code were not examined.

## Part 3: honest assessment and ranking

Ranked by how much of the candidate appears already covered, most covered first.

| Rank | Candidate | Assessment |
|---|---|---|
| 1 | (c) runtime cost/correctness optimization using verified outcomes | Fully covered. This is the contextual-bandit formulation of LLM routing, restated. No implementation-specific content. Retire it. |
| 2 | (e) intervention on observed cache population state | Fully covered, and largely a duplicate of (a). Cache-aware routing on observed KV state ships in Ray Serve, GKE Inference Gateway, Alibaba ACS and the vLLM production stack. The provider exposes the signal as a documented field. |
| 3 | (a) warm-then-fan as a scheduler primitive | Covered. A named `PromptWarmer` scheduler component with the same race diagnosis and the same fix is published, with a 7% to 85% hit-rate figure against this campaign's 0% to 86%. |
| 4 | (b) evidence-backed reasoning routing by task class | Covered by Route-To-Reason, FrugalGPT and RouterBench. The class-conditional result is the expected result in that literature. The confidently-wrong failure mode is the one under-covered detail. |
| 5 | (d) cost reconstruction across usage dialects | Mechanism covered by Langfuse, LiteLLM and tokonomics; the inclusive-versus-exclusive dialect problem is a documented, actively-filed bug class. The closed-loop check against an observed balance delta was not found published. That is a verification contribution, not a mechanism. |
| 6 | (f) fan-out scheduling using a measured cache block size | Most specific of the six and still covered in mechanism: vLLM documents block quantisation with the partial final block excluded, and boundary padding is proposed practice. What is left is a measured value that contradicts the vendor's published 64 tokens. That is a documentation discrepancy, which is worth reporting on its own terms. |

### Which are standard practice with a new measurement attached, rather than a new mechanism

**All six.** Stated individually, because the distinction matters per candidate:

- (a), (e) and (c) are **standard practice, and the measurement attached is not new either**.
  Cache warming before fan-out, routing on observed cache state, and online outcome-driven
  routing are all shipped or published, with comparable numbers already reported.
- (b) is **standard practice with a measurement that is new only in its specifics**. The 7/24
  versus 24/24 aggregation result on this model family is a data point, not a mechanism. The
  failure-mode characterisation is the part that might not be covered.
- (d) is **standard practice with a genuinely useful new verification**. The reconstruction
  method is commodity. Checking it against an observed balance delta at n=479 is the part that
  earns its place, and it belongs in the results as a validation of the campaign's own cost
  accounting rather than as a mechanism claim.
- (f) is **documented serving behaviour with a measurement that contradicts vendor
  documentation**. The exactness on 17 of 17 rungs is good evidence. Its value is as a bug
  report against DeepSeek's published 64-token figure, not as a mechanism.

### Items that should not be carried forward as mechanism claims at all

- Observation 4, the cross-endpoint cache hit (`/v1/chat/completions` prefix served to
  `/anthropic/v1/messages`), was seen once and not replicated. n=1. It should not appear in any
  mechanism record until replicated. If it replicates it is interesting as a cache-isolation
  finding, and the relevant prior art is the gateway cache-isolation auditing line
  (CacheProbe, <https://arxiv.org/pdf/2605.30613>), not scheduling.
- Observation 2, byte-exact cache identity, matches the vendor's own documented rule that a
  request "can only hit the cache if it fully matches a cache prefix unit". It is a confirmation
  of documentation, not a finding.

### Recommendation

Report (f) as a discrepancy against DeepSeek's published 64-token storage unit, and (d) as a
validated cost-accounting figure. Drop (a), (c) and (e) as mechanism candidates; they are
covered and the numbers add nothing beyond what is already published. Keep (b) only if the
confidently-wrong failure mode is tested against a standard cascade scorer, which is the one
place a real negative result might be available.

## Sources

- [DeepSeek API introduces Context Caching on Disk](https://api-docs.deepseek.com/news/news0802/)
- [DeepSeek Context Caching guide](https://api-docs.deepseek.com/guides/kv_cache/)
- [vLLM Automatic Prefix Caching design](https://docs.vllm.ai/en/stable/design/prefix_caching/)
- [vLLM prefix_caching.md on GitHub](https://github.com/vllm-project/vllm/blob/main/docs/design/prefix_caching.md)
- [vLLM issue 40696: prefix caching ineffective when prompt < block_size](https://github.com/vllm-project/vllm/issues/40696)
- [vLLM issue 11477: prefix cache aware load balancing](https://github.com/vllm-project/vllm/issues/11477)
- [vLLM production stack: load aware routing](https://docs.vllm.ai/projects/production-stack/en/latest/use_cases/loadaware-routing.html)
- [Ray Serve prefix-aware routing](https://docs.ray.io/en/latest/serve/llm/user-guides/prefix-aware-routing.html)
- [GKE Inference Gateway KV-cache-aware LLM routing](https://www.spheron.network/blog/gke-inference-gateway-kv-cache-aware-llm-routing/)
- [Alibaba ACS prefix cache-aware routing in precise mode](https://help.aliyun.com/en/cs/user-guide/kvcache-aware-load-balancing-using-intelligent-inference-routing)
- [TrueFoundry: KV cache routing and why standard load balancers break prefix caching](https://www.truefoundry.com/blog/kv-cache-routing-why-standard-load-balancers-break-prefix-caching-and-how-to-fix-it)
- [Using Span Queries to Optimize for Cache and Attention Locality](https://arxiv.org/pdf/2511.02749)
- [Prompt Cache: Modular Attention Reuse for Low-Latency Inference](https://arxiv.org/pdf/2311.04934)
- [Auditing Prompt Caching in Language Model APIs (arXiv 2502.07776)](https://arxiv.org/abs/2502.07776)
- [Auditing Prompt Caching, HTML full text](https://arxiv.org/html/2502.07776v1)
- [The Early Bird Catches the Leak: Timing Side Channels in LLM Serving Systems](https://arxiv.org/html/2409.20002v2)
- [InputSnatch: Stealing Input in LLM Services via Cache-Sharing Timing Side-Channel Attacks](https://link.springer.com/chapter/10.1007/978-981-92-4805-6_32)
- [CacheProbe: Auditing Prompt Cache Isolation in Gateway APIs](https://arxiv.org/pdf/2605.30613)
- [Prompt Caching Works. Your Prompt Assembly Code Does Not.](https://dev.to/parag_d/prompt-caching-works-your-prompt-assembly-code-does-not-5edc)
- [Pre-Warm Your Anthropic Prompt Cache Before the Traffic Hits](https://dev.to/mukundakatta/pre-warm-your-anthropic-prompt-cache-before-the-traffic-hits-oj6)
- [FrugalGPT (arXiv 2305.05176)](https://arxiv.org/abs/2305.05176)
- [RouterBench (arXiv 2403.12031)](https://arxiv.org/abs/2403.12031)
- [Route to Reason: Adaptive Routing for LLM and Reasoning Strategy Selection](https://arxiv.org/html/2505.19435)
- [Dynamic Model Routing and Cascading for Efficient LLM Inference: A Survey](https://arxiv.org/pdf/2603.04445)
- [Learning to Route LLMs from Bandit Feedback (arXiv 2510.07429)](https://arxiv.org/abs/2510.07429)
- [Online Multi-LLM Selection via Contextual Bandits under Unstructured Context Evolution](https://arxiv.org/pdf/2506.17670)
- [Cost-Effective Online Multi-LLM Selection with Versatile Reward Models](https://arxiv.org/pdf/2405.16587)
- [Langfuse: Token and cost tracking](https://langfuse.com/docs/observability/features/token-and-cost-tracking)
- [Langfuse: LLM cost management](https://langfuse.com/resources/engineering/llm-cost-management)
- [LiteLLM: completion token usage and cost](https://docs.litellm.ai/docs/completion/token_usage)
- [LiteLLM: pricing calculator](https://docs.litellm.ai/docs/proxy/pricing_calculator)
- [tokonomics](https://github.com/phil65/tokonomics)
- [continue issue 13104: OpenAI cost calculation ignores cached input tokens](https://github.com/continuedev/continue/issues/13104)
- [PromptHub: prompt caching with OpenAI, Anthropic and Google models](https://www.prompthub.us/blog/prompt-caching-with-openai-anthropic-and-google-models)
- [OpenTelemetry Gen AI attribute registry](https://opentelemetry.io/docs/specs/semconv/registry/attributes/gen-ai/)
- [OpenTelemetry blog: Inside the LLM Call, GenAI Observability](https://opentelemetry.io/blog/2026/genai-observability/)
- [Locality-aware Fair Scheduling in LLM Serving](https://arxiv.org/pdf/2501.14312)
- [LLM Query Scheduling with Prefix Reuse and Latency Constraints](https://arxiv.org/pdf/2502.04677)
- [BatchLLM: Optimizing Large Batched LLM Inference with Global Prefix Sharing](https://arxiv.org/pdf/2412.03594)

### Verification status of sources

- Fetched and read directly: the two DeepSeek documentation pages, the arXiv 2502.07776 HTML
  full text, and the dev.to prompt-assembly article. Quotations from those four are verbatim
  from the fetched content.
- All other URLs were surfaced by search and their described content comes from search result
  summaries, not from a direct fetch. Treat their characterisations as NOT_VERIFIED at the level
  of exact wording, though the URLs themselves and the general subject matter are reliable.
- No patent database (USPTO, EPO, Google Patents) was searched. Every statement in this document
  concerns published literature, documentation and shipped software only. NOT_VERIFIED: patent
  landscape.
