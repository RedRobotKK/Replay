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

Asserted by `internal/ledger/provider_conformance_test.go`, which runs in CI with no API
key and no network. The live equivalent is `scripts/verify-provider.sh`.
