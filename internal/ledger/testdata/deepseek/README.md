# DeepSeek conformance fixtures

**Captured from `api.deepseek.com` on 2026-09-05.** Every file is a complete response
body written straight to disk. None was typed from the API documentation, and none was
reconstructed from terminal output.

That distinction is the whole point. A first attempt at these fixtures *was* reconstructed
from console output truncated at 150 characters, and it was wrong in three ways that the
real captures exposed:

- the "cold" fixture was not cold — the prefix was already cached from an earlier run, so
  it came back with 13,952 cached tokens and would have encoded a false expectation
- the reasoner's cached figure was recorded as 0 and is actually 14,080; reasoning models
  cache like anything else
- the 400 body names the models that do exist, `deepseek-v4-pro`, `deepseek-v4-flash` and
  `deepseek-v4-flash-vision-exp`, which no amount of guessing would have produced

A fixture that looks like a capture but is a transcription is the same defect as a check
that cannot fail, and it was caught only because the API key was still live long enough to
take the real thing.

The cold/warm pair is genuine: `01` uses a nonce-prefixed system prompt no cache had seen
(9,630 prompt tokens, 0 cached), and `02` repeats it four seconds later (9,630 prompt
tokens, 9,600 cached). That pair is what proves the inclusive-to-exclusive conversion.

| File | Surface |
|---|---|
| `01-chat-cold.json` | chat, genuinely cold |
| `02-chat-warm.json` | chat, same prefix, cache hit |
| `03-chat-stream.sse` | chat, streaming, usage on the final chunk |
| `04-reasoner.json` | reasoner, with `completion_tokens_details.reasoning_tokens` |
| `05-tool-calls.json` | a reply carrying `tool_calls` |
| `06-error-400.json` | an error body, which must yield no usage at all |
| `07-reasoner-stream.sse` | reasoner, streaming |
| `09-request-tool-loop.json` | the agent loop request: tool call and tool result in history |
| `09-response-tool-loop.json` | its reply |
| `10-error-401.json` | an auth failure, the shape a rotated or wrong key produces |

There is no surface 8. The numbering follows the order the surfaces were probed
live and 8 was `/v1/models`, which the proxy passes through and deliberately does
not ledger, so it has no response fixture to keep.

Asserted by `internal/ledger/provider_conformance_test.go`, which runs in CI with no API
key and no network. The live equivalent is `scripts/verify-provider.sh`.

## What an audit found here, 2026-09-05

An adversarial audit planted 16 defects in production code. **Six survived**, and the
suite's stated structure turned out to be inverted from its real one.

- **The counting invariant could not fail.** `Usage()` sets `Input = prompt - cached`,
  `CacheRead = cached`, and never assigns `CacheCreation`, so `fresh + read + write`
  is identically `prompt_tokens` for every input including negative ones. 200,000
  random pairs produced zero violations. Every realistic defect — mis-splitting the
  prompt rather than breaking the sum — passed it.
  **Fixed** by checking against `prompt_cache_hit_tokens` and
  `prompt_cache_miss_tokens`, which the provider derives *separately* from
  `cached_tokens`. That cross-field redundancy is why these fixtures are worth
  keeping as bytes rather than as numbers.
- **The universal checks ran on four surfaces of nine**, streaming among the missing.
  Now they run on all of them.
- Two real production bugs surfaced once the tests were hardened: a streamed
  reasoning model recorded **zero response bytes**, because only `content` deltas
  were counted and a reasoner streams `reasoning_content`; and an empty
  `"usage": {}` object produced a **zeroed record**, which enters every average as
  a free request.

The fixtures themselves came through clean. All ten satisfy
`prompt_tokens == hit + miss`, `cached_tokens == hit`, and `total == prompt +
completion` — cross-field redundancy a hand-written fixture almost always gets
wrong. The one transcription artefact that did survive was a **comment**, not data:
it cited 14010 and 58, numbers matching no fixture here, sitting directly above the
line calling itself the most consequential assertion in the file.
