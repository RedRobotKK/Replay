# Spike: what would it take to support Cursor

**2026-09-05.** Read-only inspection of a real Cursor install, plus a look at how
LiteLLM normalises usage across providers.

## The question

Cursor looked like a client problem: write a parser for its sessions the way
`ParseClaudeCodeFile` reads Claude Code's JSONL, and Replay works for Cursor
users. Two intake paths exist, so the spike asked which one Cursor fits.

## Finding 1: the transcript path cannot produce cache forensics

Cursor keeps conversations in SQLite, not JSONL:
`~/Library/Application Support/Cursor/User/globalStorage/state.vscdb`, 627 MB on
this machine, tables `ItemTable`, `cursorDiskKV`, `composerHeaders`. Messages are
`bubbleId:` rows, 29,665 of them here.

Every one carries usage, and the usage is this:

```text
tokenCount = { inputTokens, outputTokens }
```

That is the whole accounting. Searching the entire store for cache fields:

| field | records |
|---|---:|
| `tokenCount`, `inputTokens`, `outputTokens` | 29,665 |
| `cacheRead`, `cache_read`, `cache_creation`, `cachedTokens`, `cache_read_input_tokens` | **0** |

**Cursor records no cache accounting at all.** A parser for this store would
yield spend and nothing else: no cache reads, no writes, no breaks, no rebill.
Replay's entire subject is what caching cost you, so the transcript path for
Cursor produces a tool that cannot answer the question it exists to answer.
Reverse-engineering an undocumented 627 MB schema to get there makes it worse,
not better.

## Finding 2: the proxy path works, but it is not a client problem

Cursor does support custom endpoints. This machine's `settings.json` already
has them:

```json
{ "name": "deepseek-v4-flash", "provider": "openai-compatible",
  "baseUrl": "https://api.deepseek.com", "model": "deepseek-v4-flash" }
```

So Cursor can be pointed at `replay serve`. But `provider: "openai-compatible"`
means it speaks OpenAI's wire format, and the proxy is coupled to Anthropic's:
`isMessages()` matches `/v1/messages`, and the policy layer reads
`anthropic-beta`.

**Supporting Cursor is therefore not client work. It is provider work.** It
requires an OpenAI-compatible request path, and it lands Replay in the
implicit-prefix caching family rather than explicit breakpoints.

## Finding 3: the counting trap, from LiteLLM

LiteLLM normalises usage across providers and its type definitions show the
mapping it needs:

| provider field | LiteLLM normalised |
|---|---|
| `cache_read_input_tokens` (Anthropic) | `cached_tokens` |
| `cache_creation_input_tokens` (Anthropic) | `cache_write_tokens` |
| `prompt_cache_hit_tokens` (DeepSeek) | `cached_tokens` |

It keeps the provider's own payload too, in `_hidden_params` and `model_extra`,
for the same reason `internal/usage` keeps `Raw`.

The trap is underneath that table. **Anthropic counts exclusively**:
`input_tokens` is the uncached remainder, with the cache reported beside it.
**OpenAI counts inclusively**: `prompt_tokens` already contains `cached_tokens`.
The same 150 tokens are reported as `input=100, cache_read=50` by one and
`prompt=150, cached=50` by the other.

An adapter that copies the provider's "input" figure into a normalised `Fresh`
is correct for one and double-counts the cache for the other, and **the error is
largest on exactly the sessions that cache best**, which are the ones anyone
using this tool cares most about. Langfuse shipped this bug
(langfuse/langfuse#12306).

`FromInclusive` does the subtraction and `Validate` refuses a record whose parts
do not add up. Note what the identity does not catch: an adapter that also
derives `Prompt` as the sum is internally consistent and still wrong, so the
guard is against inconsistency, not a proof of correctness.

## Finding 4: the economics are a different shape, not a different number

Reported cache pricing differs by kind, not degree. Anthropic charges a premium
to write (1.25x, 2.0x for the longer TTL) and reads at 0.1x. OpenAI's caching is
automatic, with no separate write charge, and reads at roughly half price.

That inverts the trimming advice. The break-even share is

```text
gamma > (w - alpha) / (f + w - alpha)
```

and with no write penalty the numerator goes negative: there is nothing to win
back, so trimming is not a trade-off at all. The advice Replay gives today is
correct for a provider that charges for writes and wrong for one that does not.
This is the same point `architecture/multi-provider.md` makes about the three
families, arriving from the pricing side.

Figures above are what the sources say, not what this repo has measured. That
distinction is the reason for the rules-as-dated-document work: every published
number is a claim, and Replay is positioned to check them.

## Conclusion

Cursor is not the cheap win it looked like. It is the second provider wearing a
client's clothes, and it should be sequenced as such:

1. Rules as a dated document, so a provider's numbers stop being Go constants.
2. An OpenAI-compatible request path in the proxy.
3. Cursor, DeepSeek, Grok and OpenAI then arrive together, because they are one
   piece of work, not four.
   (Corrected 2026-09-06, left in place rather than rewritten: Grok is not on
   this wire and never was. Captured off a live session it posts to /responses
   at cli-chat-proxy.grok.com, which nothing in this build parses. It was on the
   list here by assumption, which is the thing this spike existed to avoid.)

What ships from this spike is the counting defence, which was cheap and is
needed by all of them.

---

[Evidence](README.md) · [Architecture](../architecture/multi-provider.md)
