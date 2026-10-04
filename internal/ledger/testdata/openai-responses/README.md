# OpenAI Responses API wire fixtures

**Built 2026-10-04, NOT captured.** No live Responses endpoint was called:
that needs OpenAI spend, which was not authorised. These bodies are built to
two sources that a reader can check:

- the consumer: Codex CLI's own parser, `codex-rs/codex-api/src/sse/responses.rs`
  at openai/codex main on 2026-10-04, which matches events on the `type` field
  inside the data JSON and reads `response.usage` on `response.completed` and
  `response.incomplete`, with `input_tokens`, `input_tokens_details.cached_tokens`,
  `input_tokens_details.cache_write_tokens`, `output_tokens`,
  `output_tokens_details.reasoning_tokens` and `total_tokens`;
- the consumer's own fake: `codex-rs/core/tests/common/responses.rs`, whose
  helper emits `event: <kind>` then `data: <json>` and a blank line, with the
  usage object in exactly the shape above.

The request fixture carries what the transport probe of 2026-10-04 saw Codex
0.154.0 send (method, path, headers) with a body shaped to Codex's
`ResponsesApiRequest`: `instructions`, `input` items, `tools`, `store: false`,
`stream: true`, `include`, `prompt_cache_key`.

| File | Parsed by the tests through | What it establishes |
|---|---|---|
| `request-codex.json` | `SummarizeResponsesRequest` | structure, tool call keys, `prompt_cache_key` as session identity |
| `nonstream-cached-reasoning.json` | `ParseResponsesResponse` | inclusive input is split into uncached and cached; reasoning tokens |
| `nonstream-cache-write.json` | `ParseResponsesResponse` | `cache_write_tokens`, when the provider sends it, is a write |
| `nonstream-no-usage.json` | `ParseResponsesResponse` | a parsed reply with no usage is absence, not zero |
| `nonstream-empty-usage.json` | `ParseResponsesResponse` | `"usage": {}` is absence, not zero |
| `nonstream-error-200.json` | `ParseResponsesResponse` | an error object behind a 200 is unparsed, not "without usage" |
| `stream-completed.sse` | `ResponsesStreamParser` | usage from `response.completed`; text and reasoning bytes from deltas; the tool call from `output_item.done` |
| `stream-incomplete.sse` | `ResponsesStreamParser` | `response.incomplete` carries usage and it is read |
| `stream-cut.sse` | `ResponsesStreamParser` | a stream with no terminal event has no usage |

A live capture replaces these the day one is authorised; until then every
figure read from them is a test of the parser against the published shape,
not an observation of OpenAI.
