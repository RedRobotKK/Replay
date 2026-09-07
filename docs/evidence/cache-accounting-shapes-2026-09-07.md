# Four providers, three ways to count a cached token, 2026-09-07

**What this measures:** what each provider's usage fields mean arithmetically,
and whether a reader can add them up without knowing which provider produced
them.

## Summary

**No. And the reason is narrower than "providers differ": it is that the field
called input tokens names the whole prompt on three surfaces and a remainder on
the fourth.**

| Provider | Headline input field | What it names | Cache fields |
|---|---|---|---|
| Anthropic | `input_tokens` | **the remainder**, after the last cache breakpoint | `cache_read_input_tokens`, `cache_creation_input_tokens`, disjoint from it |
| OpenAI | `input_tokens` | the whole prompt | `cached_tokens`, a subset of it |
| Google | `promptTokenCount` | the whole prompt | `cachedContentTokenCount`, a subset of it |
| DeepSeek | `prompt_tokens` | the whole prompt | `prompt_cache_hit_tokens` + `prompt_cache_miss_tokens`, a partition of it |

Anthropic's own documentation states the identity:

```text
total_input_tokens = cache_read_input_tokens + cache_creation_input_tokens + input_tokens
```

and says of the last term, verbatim, that it "represents only the tokens that
come **after the last cache breakpoint** in your request, not all the input
tokens you sent."

Google's says the opposite of its own field, equally plainly: "When
`cachedContent` is set, this is still the total effective prompt size meaning
this includes the number of tokens in the cached content."

Two fields with the same role, the same obvious name, and opposite meanings.

## Why this is not a naming quibble

Sum `input_tokens` across a mixed corpus and the Anthropic rows are short by
every cached token they had. On the corpus behind this repository that is not a
rounding error: cached reads are the majority of prompt tokens in ordinary
agent work. A dashboard that adds the two vendors is not slightly wrong, it is
wrong by most of the traffic on one of them, and it looks right.

## What is measured here and what is read

**Measured**, on 148 Codex rollout files, 6,751 usage records, one machine:

```text
total_tokens == input_tokens + output_tokens            6,751 records
total_tokens == input + cached_input + output_tokens        0 records
```

So `cached_input_tokens` is inside `input_tokens` on the OpenAI surface, and is
not a separate bucket. This was checked rather than assumed because the engine
that reads it was written against Anthropic, where the opposite holds.

**Read from documentation**, not measured here: the Google and DeepSeek rows,
and Anthropic's partition identity. Each is cited above. This project has a
corpus for two of the four surfaces and says so rather than implying four.

## The consequence for the semantic conventions

[semantic-conventions-genai PR #440](https://github.com/open-telemetry/semantic-conventions-genai/pull/440),
merged 2026-08-20, specifies `gen_ai.usage.cache_read.input_tokens` and
`gen_ai.usage.cache_write.input_tokens`, and says both SHOULD be included in
`gen_ai.usage.input_tokens`.

That rule is correct for OpenAI, Google and DeepSeek and **incorrect for
Anthropic**, whose field of that name excludes them by construction. An
instrumentation that passes the Anthropic wire field straight through emits a
prompt total missing every cached token.

[Issue #487](https://github.com/open-telemetry/semantic-conventions-genai/issues/487),
opened 2026-09-01, asks how a consumer is meant to know which detail attributes
are additive and which are subsets. On the evidence above the useful answer is
not a per-attribute flag:

**Require the headline field to name the whole prompt, and make the breakdown
attributes parts of it.** Then a consumer never has to know the provider,
because the invariant is the same everywhere: the parts are contained by the
whole, and a producer whose wire format disagrees converts at the edge rather
than exporting its own arithmetic. That is one rule to state, one rule to test,
and it is testable from the emitted span alone.

Replay emits it that way as of this file's date: `gen_ai.usage.input_tokens`
carries the sum, and the two cache attributes carry their parts.

## Scope, honestly

Four providers, of which two were measured and two were read. Prices and field
names move on the provider's schedule; every claim here is dated and cited so a
reader can check whether it still holds rather than trusting that it does.

This file makes no claim about how any provider *bills* the fields it reports,
which is a separate question with at least four more dimensions.

---

[Evidence](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
