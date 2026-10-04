# R-1: the Responses API read through the proxy, wire to ledger to economics

**2026-10-04. Authorised after the transport probe in
[activation-path-2026-10-04.md](activation-path-2026-10-04.md) showed Codex CLI
0.154.0 reaches an ordinary HTTP proxy over SSE. No telemetry, no recruitment,
no change to Experiment 9 or 10, no change to QT-7a, QT-7b or QT-7c (verified
against `1175e41`). No live OpenAI endpoint was called.**

## The chain, as built

```text
Codex -> POST /v1/responses -> replay serve
  -> SummarizeResponsesRequest (structure, tool-call keys, session from prompt_cache_key)
  -> spend cap, error budget, loop detector         (existing guards, unchanged)
  -> masker                                         (already ran here before R-1)
  -> upstream
  -> responseTap: ParseResponsesResponse | ResponsesStreamParser (chosen by route)
  -> transcript.ResponsesUsage.Usage()              (one normalisation, no parallel accounting)
  -> ledger record, SpendGuard.Record, stats tally  (existing)
  -> replay cost / replay replay / replay simulate  (existing, read the same files)
```

Routing is one predicate added to the readable set. The tap gains one parser
pair selected by the route, as the chat-completions pair is. The normalised
`Usage` is the existing type; pricing, caps and the ledger were not touched.

## RED

Fifteen proxy tests were written against the build that forwarded the path
unread and all fifteen failed for that reason: no ledger record; the cap and
the loop guard blind; `NOT PARSED /v1/responses` still printed; the unparsed
counter still counting. The parser tests were red by missing API. RES4, the
one assertion R-1 was authorised to flip, now asserts the path is ledgered and
no longer announced as NOT PARSED; RES5's two assertions are unchanged and its
precondition is re-anchored from the NOT PARSED line to the READ line that
replaced it, since the old line would be false. RES1, RES2, RES3 and RES6 are
untouched and pass.

## Fixtures: built, not captured

`internal/ledger/testdata/openai-responses/` holds a Codex-shaped request, four
non-streaming replies and three streams. They are built to two checkable
sources: Codex CLI's own parser (`codex-rs/codex-api/src/sse/responses.rs`,
which matches events on the `type` field inside the data JSON and reads
`response.usage` on `response.completed` and `response.incomplete`) and its
own test helper (`codex-rs/core/tests/common/responses.rs`, `event:` then
`data:` framing). A live capture needs OpenAI spend that was not authorised.
Classification: the field names and event names are OBSERVED in the consumer's
source; the bodies are ESTIMATED shapes; a real endpoint's behaviour is NOT
MEASURED. The README in that folder says the same.

## Responses: what is mapped, and how

| Wire | Normalised | Note |
|---|---|---|
| `usage.input_tokens` minus cached minus write | `Input` | inclusive on the wire, exclusive here, as on chat completions |
| `usage.input_tokens_details.cached_tokens` | `CacheRead` | clamped to the input figure |
| `usage.input_tokens_details.cache_write_tokens` | `CacheCreation` | only when the field is present (GPT-5.6 and later per the surface contract); zero otherwise, never reconstructed |
| `usage.output_tokens` | `Output` | |
| `usage.output_tokens_details.reasoning_tokens` | `ThinkingTokens` | |
| request `model` | `Model` | the reply's model is not consulted |
| request `prompt_cache_key` | session, hashed | Codex sets it to its conversation id; structural fallback when absent |
| `instructions` | `Prompt.SystemBytes` | the shape's system prefix |
| input items | `Prompt.Messages` | message, function_call (keyed), function_call_output, reasoning, other by size |
| `"usage": {}` or absent | no usage | counted under `responses_without_usage` (QT-7a) |
| not JSON, another object, error behind 200 | `Unparsed` | counted under `responses_unparsed` (QT-7c) |

One departure from the Phase-0 note, stated rather than hidden: the Phase-0
summary said CacheCreation is "not applicable". The repository already records
that GPT-5.6 and later report `cache_write_tokens` and bill it
(`internal/surface/contracts.go`, `internal/probe/openai.go`, and the Codex
rollout reader's `cache_write_input_tokens`). Mapping a field the provider
sends is not inventing economics; inferring one it does not send would be.
So the write is carried when present and zero when absent, and
`TestResponses_CacheWriteTokensAreAWriteOnlyWhenSent` holds both halves.

**NOT MEASURED, by design.** Cache-break classification. The live classifier
expects the previous prompt total as this request's cache read, which is
Anthropic's documented prefix rule. OpenAI's cache is addressed by the client's
`prompt_cache_key` and reported back as `cached_tokens`; nothing on the wire
establishes the same expectation, and the AstraRules in `cachemodel` are not
wired into the live classifier on any path. So `stats.observe` skips the
classifier for records on this path: no outcome on the record, no break
counted, no cause named, while the lane state and the cost tallies update.
The proxy prints this once per path, on the READ line, and the serve banner
says it before any traffic. Also not measured: GPT-6 Astra's own transport,
Codex under ChatGPT login, Codex versions other than 0.154.0.

**Session identity.** OBSERVED on the wire by the 2026-09-06 capture of a
Responses client and by Codex's `ResponsesApiRequest`: `prompt_cache_key` is a
body field, present on the model calls, absent on some. Used when present,
hashed with a domain prefix, never written raw. Absent, the structural
fallback the chat-completions path uses, with its stated collision limit.
Four requests, two keys and one without, produce three sessions; the ledger
file holds neither the key nor any prompt text (checked on disk).

## Streaming

Its own parser, fed line by line as bytes pass, never buffering text. Usage
from `response.completed` and `response.incomplete`; text from
`response.output_text.delta`; reasoning from the two reasoning delta events;
the tool call from `response.output_item.done`. The result is the same whether
the stream arrives whole, one byte at a time, or in 7-byte chunks. A stream cut
before its terminal event records no usage and counts under QT-7a. A gzip
stream, which skips the incremental parsers, is re-read by route and yields the
same usage. The client receives the stream byte for byte.

## Security

- Masking ran on this path before R-1 (RES1) and still does; the record counts
  the masked secrets and the ledger file and the log hold neither the canary
  nor the raw cache key.
- The body hash bracket is taken on this path as on the other two; masking is
  the only rewrite, recorded on the Masked field.
- No `stream_options` injection (RES3), no membership of the chat-completions
  family (RES6), a clean body forwarded byte for byte (RES2).

## Mutation

Ten hand mutants, each applied to production code, built, run against its
named killer and restored byte-identically (hashes checked), then frozen as
M127 to M136. M125, the banner mutant from the activation gate, was re-anchored
to the new banner line with its intent inverted: it now restores the pre-R-1
"forwarded unread" claim.

| Id | Deciding branch | Mutation | Killed by |
|---|---|---|---|
| M127 | readable set | drop `responses` | TestR1_ANonStreamingResponsesReplyIsLedgeredWithNormalisedUsage |
| M128 | tap parser by route | never pick the Responses stream parser | TestR1_AStreamedResponsesReplyIsLedgeredFromTheCompletedEvent |
| M129 | normalisation | copy inclusive input into Input | TestResponsesUsage_NormalisesInclusiveCounts |
| M130 | stream terminal event | wait for an event that never comes | TestResponses_StreamUsageComesFromTheTerminalEvent |
| M131 | empty usage | record `{}` as a measured zero | TestResponses_NoUsageAndEmptyUsageAreAbsence |
| M132 | session identity | ignore prompt_cache_key | TestResponses_SessionHashIsThePromptCacheKey |
| M133 | cache classification | classify by the Messages rule | TestR1_NoCacheBreakIsClassifiedOnTheResponsesPath |
| M134 | unreadable body | drop the Unparsed mark | TestResponses_UnreadableBodiesAreMarkedUnparsed |
| M135 | reasoning tokens | drop them | TestResponsesUsage_NormalisesInclusiveCounts |
| M136 | summariser selection | never select the Responses summariser | TestR1_TheLoopGuardSeesRepeatedResponsesToolCalls |

All ten killed by hand; the registered harness result is recorded in the
Regression section below.

## Black box, on the shipped binary

`TestBB_ResponsesPathReachesTheLedgerAndTheEconomics` builds the production
binary, installs the OpenAI rules document the way an operator does
(`replay rules --update docs/rules/openai-2026-09-15.json`), starts
`replay serve --max-session-usd 0.05` as a child process against a fake
Responses upstream, and posts the Codex-shaped request.

| Step | Observed |
|---|---|
| three non-streaming requests at $0.021 (500 uncached, 1,000 cached at a tenth, 300 output on gpt-6-astra) | 200, 200, 200 |
| the fourth, same `prompt_cache_key` | 400 `replay_spend_cap`, "$0.06 of $0.05 at list price", issued by the binary, not forwarded |
| one streamed request on another key | 200, the fixture delivered byte for byte |
| ledger files on disk | 5 records: 4 provider requests with input 500, cache read 1,000, output 300, thinking 200, raw usage kept, no cache classification, no write invented; 1 spend_cap refusal; 2 sessions; no prompt text and no raw key |
| `replay cost --json` on the same files | 4 priced requests, $0.084 |

Pricing requires the OpenAI rules document. Without it the compiled table has
Anthropic rows only, so an OpenAI model is charged at the dearest known row as
an upper bound and the status flags the cap total as an over-estimate: the
existing QT-7b rule, pinned on this path by
`TestR1_AnUnpricedResponsesModelIsBoundedNotFree`. The Codex default models
(`gpt-5-codex` and the codex-mini variants) have no row in that document; they
are bounded, not free, until a dated row with a published source is added.

## Production wiring

The path is reached by `cmd/replay` through `serve` with no new flag:
`proxy.Server.handle` computes `isResponses` on every request. The serve
banner lists the path among the read ones and names the unmeasured figure
(`TestServeBannerNamesEveryClientAndEveryReadPath`, and the black-box twin
reads it from the built binary). The wiring and production-grade matrices
were regenerated; no surface was added, so neither changed.

## Regression

Measured on the working tree before commit, with the machine's memory limit
in force (`GOFLAGS=-p=2`):

| Check | Result |
|---|---|
| `go test ./...` | 38 packages ok, 0 failed |
| `go test -race ./...` | 38 ok, 0 races |
| RES1 to RES6, Messages, chat completions (OAE1 to OAE3), tap, QT-7a and QT-7c proxy tests | in the suite above, green |
| pricing (`internal/cachemodel`), caps and ledger (`internal/proxy`, `internal/ledger`) | green |
| registered harness, M125 and M127 to M136 | 135 catalogued, 11 mutants: 11 killed, 0 survived, 0 stillborn |
| `internal/blackbox` under the mutation tag, whole package | ok |
| vet, with and without the mutation tag; gofmt; `git diff --check` | clean |
| `golangci-lint run --new-from-rev=580ecc6` | 0 issues |
| wiring matrix and production-grade matrix regenerated | unchanged |
| QT-7a, QT-7b, QT-7c evidence and tests against `1175e41` | unchanged |
| docs guards and `internal/regression` | green |

## Decision

**R-1 PRODUCTION-READY**, with the three limits above stated on the surface
rather than in this file alone: fixtures built not captured; cache-break
classification not measured on this path; OpenAI pricing requires the installed
rules document and the Codex default models are bounded until a row exists.
