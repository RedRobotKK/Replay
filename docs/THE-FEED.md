# The feed worth publishing is not the price feed

**2026-09-04.** "We need an API feed." Yes — but not the one it looks like, and checking the
incumbent first is what shows why.

## The price feed already exists, and it is better than anything we would maintain

**LiteLLM's `model_prices_and_context_window.json`: 3,561 models, 27 of them Anthropic, free, no key,
community-maintained, versioned in git.** One Anthropic row carries:

```
input_cost_per_token, output_cost_per_token,
cache_creation_input_token_cost, cache_creation_input_token_cost_above_1hr,
cache_read_input_token_cost, prompt_cache_min_tokens,
max_input_tokens, max_output_tokens, deprecation_date,
supports_prompt_caching, supports_prompt_cache_breakpoint, …
```

**It already publishes the minimum cacheable prefix.** `prompt_cache_min_tokens: 4096` for
haiku-4-5, which is the same number Replay hardcodes.

**So building a price feed would be re-solving a solved problem against an incumbent with
distribution.** Do not.

## What this fixes immediately, for free

Replay should **consume** that file instead of compiling a table into the binary. Three problems
disappear at once:

- **The stale table.** 27 Anthropic models maintained by people who watch this daily, against 15 rows
  dated 72 days ago.
- **The unknown-model-is-free bug**, because coverage stops being the constraint.
- **The multi-provider rules problem** from `architecture/multi-provider.md`, since the same file
  covers 3,561 models across every provider.

Refreshed **in CI as a pull request that bumps a vendored copy and its date**, never fetched at
runtime, per `docs/TOKEN-PRICES.md`. A human reads the diff, the binary still talks only to your
provider.

## The gap nobody fills, and it is the interesting one

**Every field in LiteLLM is transcribed from vendor documentation. Nobody measures whether any of it
is true.**

`prompt_cache_min_tokens: 4096` is what the vendor says. **Replay's own corpus could not confirm it:
across 11 sessions the smallest cached prefix was 14,873 tokens and no uncached prompt was ever seen
below that**, so the published floor is untested rather than wrong. For fable-5.1 the rules say 512
and observation bounds it at 40,563.

**That is not a contradiction yet. It is an unverified claim, and unverified is a status nobody
publishes.**

## So the feed to publish is the verification layer

Not prices. **Observations against published claims**, per model per field:

```json
{
  "model": "claude-haiku-4-5",
  "field": "prompt_cache_min_tokens",
  "documented": 4096,
  "documented_source": "litellm@<commit>",
  "observed": { "upper_bound": 14873, "lower_bound": null,
                "sessions": 11, "machines": 1 },
  "status": "unverified",
  "as_of": "2026-09-03"
}
```

**`status` is the product.** `consistent`, `unverified`, `contradicted`. **No vendor dashboard will
ever show you the third one**, and no aggregator can compute it, because computing it requires
replaying real traffic against the provider's own usage numbers turn by turn. **That is precisely
what Replay does and precisely what LiteLLM cannot.**

## Why this is a better position than owning the prices

**Owning a price feed means owning other people's billing.** If a number is wrong, somebody's budget
guard fails and it is your fault. A solo maintainer should not take that liability to compete with an
incumbent doing it well.

**Owning the verification layer means owning a claim nobody else can make.** It is additive rather
than competing: LiteLLM stays the source for what vendors say, and this says whether it holds. **The
natural end state is contributing corrections upstream**, which makes the aggregator better and makes
Replay the thing that found them.

And it is the same design already written down twice: the `documented` / `observed` / `status` shape
in `architecture/multi-provider.md`, fed by the corpus in ADR-0007 and ADR-0009.

## What to do

1. **Vendor LiteLLM's JSON**, refreshed by a CI pull request. Kills the stale table and the
   unknown-model-is-free bug this week.
2. **Emit the verification records** from `replay corpus`, which already computes the bounds.
3. **Publish them as a static JSON file**, in the repo, beside `docs/evidence/`. **No API, no server,
   no uptime obligation.** A file on a CDN is a feed.
4. **Open upstream issues when a field is `contradicted`**, with the evidence attached.

## The constraint that has not changed

**Eleven sessions on one machine.** Every observation is honest and none is yet strong enough to call
a vendor wrong. **Publish `unverified` freely and `contradicted` only with a corpus behind it**, or
the feed becomes exactly the unchecked assertion it exists to correct.
