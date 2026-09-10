# Surface taxonomy, part 3: what differs per provider, 2026-09-09

**What this is:** every surface where Replay reads or could alter traffic, set
against every provider the code actually handles, with a citation in each cell
and a mark saying what kind of claim it is.

**The framing this part serves:** byte-for-byte passthrough is a setting, not an
identity. Part 3 asks the narrower question — where does one provider's shape
get assumed for another, and what does that cost.

## How to read a cell

Four marks, and the distinction between them is the point of the file.

| Mark | Means |
|---|---|
| **[code]** | A fact about **Replay**, read from the cited `file:line`. Checkable by opening the file. Says nothing about the provider. |
| **[measured]** | A fact about the **provider**, measured against the real thing, with a dated evidence file. |
| **[doc]** | A fact about the provider **read from its documentation** and never measured here. |
| **[nd]** | Not determined. |

The third and fourth are the ones this repository keeps having to correct.
`cache-accounting-shapes-2026-09-07.md` marks two of its four rows as read
rather than measured; `surface-census-2026-09-08.md` exists because a claim
about a disk was inferred from a network capture. So a **[code]** cell says
Replay does X — never that the provider deserves X.

## The providers, as the code actually distinguishes them

**Provider detection has exactly one signal: the URL path suffix.** There is no
Host dispatch, no base-URL inference, and one global upstream per `replay
serve` process (`internal/proxy/server.go:204-206`, default
`https://api.anthropic.com` at `cmd/replay/serve.go:29`). Two paths are
readable — `/v1/messages` (`internal/proxy/server.go:976`, matched at `:979`)
and `/v1/chat/completions` (`:977`, matched at `:1000`) — and `readable :=
messages || openai` at `:475`. Everything else is forwarded byte-for-byte and
counted once per path (`:476-482`, `:1240-1247`).

So the columns below are **wires, not vendors**, because that is the only
distinction the code can make. No ledger record carries a vendor field at all
(`internal/ledger/record.go:31-102`); `Path` is the whole of a record's
provenance.

| Col | Column | Wire | Registry status |
|---|---|---|---|
| **A** | Anthropic — Claude Code | `/v1/messages` | LIVE, fixture `testdata/anthropic` (10 files) |
| **D** | DeepSeek | `/v1/chat/completions` | LIVE, fixture `testdata/deepseek` (31 files) |
| **O** | Cursor / other OpenAI-compatible | `/v1/chat/completions` | STUB — same code path as D, no live capture |
| **X** | Codex | rollout JSONL on disk, **no wire** | not in the registry at all |
| **L** | Ollama / llama.cpp | server log on disk; `/api/chat` unhandled | not in the registry at all |
| **G** | Grok | `/responses` | FORWARDED |
| **?** | Anything else | any other POST | FORWARDED |

Registry: `internal/ledger/surface_registry_test.go:73-127`. Note it lives in a
**test file** and omits Codex and Ollama, which are the two surfaces Replay
reads most data from. That is a gap in the registry, not in the readers.

---

## 1. Counting and cache accounting

| Surface | A — Anthropic | D — DeepSeek | O — other OpenAI | X — Codex | L — Ollama | G — Grok | ? |
|---|---|---|---|---|---|---|---|
| **Headline input field names** | the **remainder** after the last breakpoint **[doc]** `cache-accounting-shapes-2026-09-07.md` (Anthropic's own identity, quoted, not measured) | the **whole prompt**, cache nested inside **[measured]** live capture 2026-09-05, `surface_registry_test.go:88-91` | assumed same as D **[code]** one parser, `internal/transcript/openai.go:44` | the **whole prompt**, cache inside **[measured]** `total_tokens == input+output` on 6,751 records, `cache-accounting-shapes-2026-09-07.md` | **work performed**, resident prefix absent entirely **[measured]** `ollama-cache-ceiling-2026-09-08.md`; type doc `internal/transcript/ollama.go:17-38` | **[nd]** never parsed | **[nd]** |
| **Which counting family that is** | exclusive | inclusive | inclusive (assumed) | inclusive | **a fourth kind** — neither | — | — |
| **Adapter converts to Replay's exclusive shape?** | n/a, native **[code]** `internal/transcript/wire.go:80` | **yes**, subtraction at `internal/transcript/openai.go:63` **[code]** | same code **[code]** | **NO** — `Input: c.Input, CacheRead: c.Cached` at `internal/transcript/codex.go:151-152` after proving `Cached ⊆ Input` at `:140` **[code]** | never becomes a `transcript.Usage`; own type **[code]** `ollama.go:26-38` | — | — |
| **Cache-read field read** | `cache_read_input_tokens` **[code]** `wire.go:67` | `prompt_tokens_details.cached_tokens` **[code]** `openai.go:34` | same **[code]** | `cached_input_tokens` **[code]** `codex.go:108` | none exists; recovered from `n_past` **[measured]** `ollama-cache-ceiling-2026-09-08.md` | — | — |
| **`prompt_cache_hit_tokens` / `_miss_tokens`** | n/a | **sent, and not parsed** — kept only in `RawUsage` **[code]** `internal/ledger/openai.go:137-143`, fixture `rawusage_test.go:20-21` **[measured]** | **[nd]** whether any other vendor sends these without `prompt_tokens_details` | n/a | n/a | — | — |
| **Cache-write reporting** | `cache_creation_input_tokens` + 5m/1h split **[code]** `wire.go:66,73-74` | **none exists** — implicit-prefix family **[code]** `openai.go:30-33` | none | none | none | — | — |
| **Reasoning tokens** | `output_tokens_details.thinking_tokens` **[code]** `wire.go:70` | `completion_tokens_details.reasoning_tokens` **[code]** `openai.go:37` | same | `reasoning_output_tokens` **[code]** `codex.go:111` | not parsed **[code]** | — | — |
| **Raw provider usage kept verbatim** | yes, since 2026-09-05 **[code]** `internal/ledger/response.go:33` | yes **[code]** `openai.go:143` | yes | **no** — no `RawUsage` on the Codex path **[code]** `codex.go:150-155` | no | — | — |
| **Records refused as impossible** | none | clamps `cached > prompt` **[code]** `openai.go:57-62` | same | refuses `Cached>Input`, negatives, and total-without-breakdown **[code]** `codex.go:138-148` | drops partial log blocks **[code]** `ollama.go:100-104` | — | — |
| **Cache-break detection** | inferred: prefix hash diff **[code]** `internal/proxy/causedetail.go:49-63` | same, but `PrefixHash` is over block **structure** not content **[code]** `ledger/openai.go:57-71` | same | **stated** by the provider each turn **[code]** `codex.go:276-290`, thresholds 0.50/0.10 | duration-based, 273x cold/warm separation **[measured]** `ollama-cache-observable-2026-09-09.md` | — | — |

**The fourth counting convention.** `cache-accounting-shapes-2026-09-07.md` is
titled "three ways to count a cached token". With Ollama it is four: Anthropic
partitions the cache **out** of the headline, OpenAI and Codex nest it **in**,
DeepSeek partitions it **within**, and Ollama reports **the work it did** —
a resident prefix is not re-evaluated and never appears in `Total` at all
(`internal/transcript/ollama.go:17-38`, ceiling measured at
`ollama-cache-ceiling-2026-09-08.md`). That is not a fourth spelling of the
same quantity. It is a different quantity.

---

## 2. Streaming shape

| Surface | A | D / O | X | L | G / ? |
|---|---|---|---|---|---|
| **SSE parser** | `internal/ledger/response.go:60-166` **[code]** | `internal/ledger/openai_stream.go:22-107` **[code]** | none — offline JSONL, `codex.go:180-217` **[code]** | none — `/api/chat` NDJSON unhandled anywhere **[code]** | none |
| **Data prefix accepted** | `"data: "` **with the space** **[code]** `response.go:104` | `"data:"` **without** **[code]** `openai_stream.go:48` | — | — | — |
| **Events read** | `message_start`, `content_block_start`, `content_block_delta`, `message_delta` **[code]** `response.go:137-166` | `choices[].delta.content`, `.reasoning_content`, terminal `usage` **[code]** `openai_stream.go:56-86` | `session_meta`, `event_msg`/`token_count` **[code]** `codex.go:208-213` | log lines `n_past`, `prompt eval time`, `eval time`, `total time` **[code]** `ollama.go:81-86` | — |
| **`stream_options.include_usage`** | n/a | **injected by Replay** when the client omitted it **[code]** `server.go:1278-1296`, recorded as policy `openai-include-usage` at `:544-553` | — | — | — |
| **Usage merge across frames** | field-wise merge, non-zero wins **[code]** `response.go:155-187` | last frame wins **[code]** `openai_stream.go:77-86` | per-turn delta accumulation **[code]** `codex.go:236-241` | — | — |
| **Zero-usage guard** | — | **non-stream only** — `"usage":{}` nilled at `openai.go:128-130`, **no equivalent on the stream path** **[code]** | absent-breakdown refused **[code]** `codex.go:143-147` | — | — |
| **Unfinished-line cap** | 1 MB then stop parsing **[code]** `response.go:57,85-91` | **none — buffer grows unbounded** **[code]** `openai_stream.go:33-45` | 16 MB scanner **[code]** `codex.go:184` | 4 MB scanner **[code]** `ollama.go:108` | — |
| **Streamed tool calls counted** | yes **[code]** `response.go:145` | **no** — stream reads only content and reasoning, non-stream does emit tool blocks **[code]** `openai_stream.go:56-76` vs `openai.go:154-157` | n/a | n/a | — |
| **Streaming fixture on disk** | 4 `.sse` **[measured]** `testdata/anthropic/stream-*.sse` | 2 + 20 `.sse` **[measured]** `testdata/deepseek/` | **none** | none | none |

---

## 3. Passthrough, and where Replay stops being a mirror

| Surface | A | D / O | X | L | G / ? |
|---|---|---|---|---|---|
| **Response tap** | tee — client written **first**, parser second **[code]** `server.go:1163-1181` | same tap **[code]** | n/a, offline | n/a | tap allocated and never read **[code]** `server.go:1153-1159` |
| **Request bytes altered by default** | no | **yes if streaming** — `stream_options` injected **[code]** `server.go:1278-1296` | n/a | n/a | no |
| **`--mask` covers it** | yes **[code]** `server.go:515-522` | **no**, warned once per path **[code]** `server.go:1249-1263` | n/a | n/a | no |
| **Rehydration rewrites the stream** | yes — holds tool-input deltas and **synthesises** a replacement `content_block_delta` **[code]** `internal/masking/stream.go:184-233,354-367` | not installed **[code]** `server.go:558` gates on `messages` | n/a | n/a | no |
| **Request-parameter policy applied** | yes **[code]** `server.go:541-543` | no | n/a | n/a | no |
| **Body hashes bracket any transform** | yes **[code]** `record.go:63-65` | yes, but nothing writes them for the `include_usage` injection **[nd]** | n/a | n/a | n/a |

The honest statement of the passthrough claim, per column: **A** is byte-for-byte
unless `--mask` or a policy is on, both opt-in, both logged. **X** and **L**
touch no wire. **G/?** are untouched.

**D/O is not byte-for-byte, by default, and the deviation is larger than the
field it adds.** `withUsageReporting` (`internal/proxy/server.go:1278-1295`)
decodes the body into a `map[string]json.RawMessage`, inserts
`stream_options`, and **re-marshals the whole map**. Go's `encoding/json`
serialises a map in sorted key order, so a streaming OpenAI-compatible request
reaches the provider with its top-level keys reordered and its whitespace
normalised, not merely with one member added. That is a defensible trade — the
alternative is no usage measurement at all on a streamed response — but it is
the one column where the README's claim needs the "unless" clause that ADR-0011
requires, and it is currently recorded only as `rec.Policy =
"openai-include-usage"` (`server.go:544-553`) rather than bracketed by
`BodyHashBefore`/`BodyHashAfter`, which is the mechanism the repository built
precisely to make this checkable (`internal/ledger/record.go:63-65`).

---

## 4. Identity

| Surface | A | D / O | X | L | G / ? |
|---|---|---|---|---|---|
| **Session id source** | `x-claude-code-session-id` **[code]** `server.go:64,483` | **always synthesised** — hash of block *kind/bytes/name*, never text **[code]** `ledger/openai.go:63-71,76-93` | real, `session_meta.id` **[code]** `codex.go:187-189` | **none** — only `slot`/`task` ints **[code]** `ollama.go:26-38,85` | none, no record written |
| **Collision mode** | none | two sessions matching in kind and byte size collide; stated in-tree **[code]** `ledger/openai.go:57-61` | none | n/a | n/a |
| **Sub-agent / lane id** | `x-claude-code-agent-id` **[code]** `server.go:65` | **can never be populated** — all traffic collapses to lane `""` **[code]** | none | none | none |
| **Provider request id** | `request-id` **[code]** `providerRequestID`, `quota.go` | **`x-request-id`** — captured since the request-id change; was empty, and every record on this family fell back to a name synthesised from its position in the file **[code]** | n/a | n/a | n/a |
| **On-disk discovery** | 4 roots + `REPLAY_TRANSCRIPTS` **[code]** `cmd/replay/defaultroot.go:54-79` | none | `~/.codex/{sessions,archived_sessions}`, `rollout-*.jsonl` **[code]** `cmd/replay/codex.go:23-48` | `~/.ollama/logs/server*.log` **[code]** `cmd/replay/burn.go:184-186` | Grok: 3.8 GB present and **not read** **[measured]** `surface-census-2026-09-08.md` |
| **`transcript.Source` tier** | `SourceTranscript` / `SourceLedger` **[code]** `types.go:162-165` | ledger | `SourceCodex` falls through the else-branch to "estimated" **[code]** `types.go:167-180` | `SourceOllama`, same fall-through | — |

---

## 5. Quota, rate limits, errors, retry

| Surface | A | D / O | X | L | G | ? |
|---|---|---|---|---|---|---|
| **Header allowlist captured** | `anthropic-ratelimit-*` **[code]** `internal/proxy/quota.go:17` | `x-ratelimit-*` **[code]** `quota.go:18` | n/a | n/a | `x-ratelimit-*` if it were readable | `retry-after` on all |
| **Quota data actually on this machine** | **zero** ledger records carry any quota key **[measured]** `quota-data-census.md:67-91,439-452` | zero **[measured]** same census | **6,871 events / 147 files**, primary 0–52%, secondary 0–**89% [measured]** `quota-data-census.md:279-333`, `codex-quota-2026-09-07.md:22-28` | none exists **[code]** `burn.go:182` | headers present, **never moved** across 8 calls / 940 KB **[measured]** `surface_registry_test.go:113-118` | — |
| **Quota read without a proxy** | no | no | **yes** — the client writes it to disk **[code]** `codex.go:213-232` | n/a | no | — |
| **Titration result** | 3.09M tokens moved the counter **zero** steps **[measured]** `quota-titration-2026-09-06.md:22-30` | — | 3.11M tokens moves primary by 1% **[measured]** `codex-quota-2026-09-07.md:16-20` | — | — | — |
| **Token weighting for the window** | **[nd]** | — | **underdetermined** — four weightings fit equally; an earlier draft publishing one was retracted **[measured]** `codex-quota-2026-09-07.md:57-68` | — | — | — |
| **Error body parsed** | **no** — non-`"message"` yields empty `Response{}` **[code]** `ledger/response.go:14-23` | **no** — only `choices`/`usage` decoded **[code]** `openai.go:118-158` | n/a | `{"error":"..."}` not decoded anywhere **[code]** | n/a | n/a |
| **Error shape Replay *emits* on refusal** | Anthropic envelope **[code]** `server.go:1090` | **Anthropic envelope** — wrong shape for the client **[code]** same line | n/a | n/a | n/a | Anthropic envelope |
| **Retry trigger** | `429 \|\| 5xx`, one policy for all **[code]** `guards.go:405-407` | same | n/a | n/a | same (body buffered) | same |
| **529 / `overloaded_error` handled distinctly** | **no** — caught only by the 5xx range; the constant appears in no non-test code **[code]** | n/a | n/a | n/a | n/a | n/a |
| **`Retry-After` honoured** | yes; a value above `MaxDelay` **stops** retrying **[code]** `retry.go:107-118,121-132` | yes | n/a | n/a | yes | yes |
| **Tool-result `IsError` populated** | yes **[code]** `wire.go:59,140` | **no** — so `ErrorBudget` (`guards.go:240-262`) is inert, **silently** | no | no | no | no |
| **Retries counted per session** | **no** — per request, plus one process-global counter **[code]** `retry.go:37-47`, `state.go:234` | same | n/a | n/a | same | same |

---

## 6. Downstream: pricing, advice, export

| Surface | A | D | O | X | L |
|---|---|---|---|---|---|
| **Priced by `cachemodel`** | yes **[code]** `anthropic.go:188-198` | no — `deepseek-*` fails the family test **[code]** | **depends on the model *string*, not the provider** — a gateway serving `claude-sonnet-*` over `/v1/chat/completions` passes **[code]** `anthropic.go:191-198` | no | no bill exists; unit is seconds **[measured]** `ollama-cache-observable-2026-09-09.md` |
| **Write premium (1.25x / 2.0x) can fire** | yes **[code]** `anthropic.go:103-104,357-361` | **never** — no write is reported, so `writeEquivalent` is always 0 | never | never | n/a |
| **`internal/analysis` / `internal/advisor` branch on provider** | **no. Neither package has a provider dimension at all** **[code]** — `advisor.go:134`, `analysis/replay.go:48,150,191`, `fit.go:212-214`, `blame.go:74`, `context.go:328-331`, `report.go:84`, `learn/types.go:34`, `proxy/state.go:280`, `cachemodel/anthropic.go:246` all call `Usage.PromptTotal()` on an untagged struct | | | | |
| **`internal/usage` normaliser (`FromInclusive`, `Validate`)** | **imported by nothing** — verified, zero import lines repo-wide **[code]** `internal/usage/usage.go:179,208` | | | | |
| **OTLP `gen_ai.system`** | `"anthropic"`, **hardcoded for every span** **[code]** `internal/otlp/otlp.go:140` — and `internal/otlp` is imported by nothing | | | | |

**Cell count, counted rather than estimated.** Six matrices, **48 surface rows**
against the seven provider columns; tables 2–6 collapse columns that behave
identically, so the grid is not a full 48x7. **116 cells inside tables 1–6
carry an explicit mark**: **93 [code]**, **17 [measured]**, **1 [doc]**,
**5 [nd]**. The unmarked cells are one-word restatements of the row above them
("inclusive", "never", "none"), `n/a`, or inherit the mark of the cell they
qualify. One further [nd] appears in the prose below, in S4.

That ratio is the honest headline, and it is uncomfortable in the right
direction: **93 against 17**. This repository knows a great deal about what its
own parsers do, and comparatively little, measured, about the providers they
parse. Marking those two the same colour is how a matrix like this becomes
decoration.

---

## Part 1 — where a shared code path assumes something only one provider does

Ranked by blast radius: how much of the tool's output goes wrong, times how
silently.

## S1. The Codex reader fills an exclusive-counting struct with inclusive counts

`internal/transcript/codex.go:151-152` writes `Input: c.Input, CacheRead:
c.Cached` — eleven lines after proving at `:140` that `c.Cached <= c.Input`,
i.e. that the cache is **inside** the input. `transcript.Usage.PromptTotal()`
(`internal/transcript/types.go:116`) is `Input + CacheCreation + CacheRead`. So
`PromptTotal()` on a Codex record returns the prompt **plus the cached share
again**.

On the local Codex corpus the cached share is **483,512,448 of 516,531,547
attributed tokens** (`codex-quota-2026-09-07.md:70-75`) — 94%. `PromptTotal()`
would report roughly **1.94x** the truth, and the error is largest on exactly
the well-cached sessions the tool exists for. This is the Langfuse bug that
`internal/transcript/openai.go:16-21` names and that `usage.FromInclusive`
(`internal/usage/usage.go:166-197`) was written to prevent.

**Blast radius today: zero.** Every current Codex consumer calls `.Total()`
(`= Input + Output`, `codex.go:84`), which is correct under inclusive counting:
`cmd/replay/codex.go:109-110`, `cmd/replay/burn.go:156`. Nothing calls
`PromptTotal()` on a Codex record.

**Blast radius on the next commit that routes Codex into the analysis packages:
total.** Twenty-plus call sites take `PromptTotal()` on an untagged
`transcript.Usage` — `internal/advisor/advisor.go:134`,
`internal/analysis/replay.go:48,150,191`, `fit.go:212-214`, `blame.go:74`,
`context.go:328-331`, `report.go:84`, `internal/learn/types.go:34`,
`internal/proxy/state.go:280`, `internal/cachemodel/anthropic.go:246`. None
knows which provider produced the struct. Every prompt total, cached share,
break deficit, effective-token figure and dollar figure on the **largest
corpus on this machine** would be wrong, and would look right.

**Rank 1** because the guard already exists and is unplugged: `internal/usage`
is imported by nothing, verified by `go list` over the whole module and by grep
(`grep -rn 'RedRobotKK/Replay/internal/usage"'` → 0 lines). `Validate()` at
`usage.go:208` would refuse this record on sight. `RELEASE-CRITERIA`-shaped fix:
route Codex through `usage.FromInclusive` before it becomes a `transcript.Usage`,
or give `Usage` a mechanism tag and make `PromptTotal()` refuse an untagged one.

## S2. The OpenAI-compatible path is not byte-for-byte, by default

`withUsageReporting` (`internal/proxy/server.go:1278-1295`) is called on every
`/v1/chat/completions` request (`server.go:544-553`). When the body says
`"stream": true` and carries no `stream_options`, it decodes the body into a
`map[string]json.RawMessage`, adds `{"include_usage":true}`, and **re-marshals
the map**. Go serialises map keys in sorted order, so the request that reaches
the provider is not the client's bytes with one field appended — it is a
re-serialisation with the top-level keys reordered and whitespace normalised.

The feature is right: without `include_usage` a streamed OpenAI-compatible
response reports no usage at all (`internal/ledger/openai_stream.go:19-21`), so
this buys the entire measurement on that column. Three things about it are
wrong.

1. It is **on by default**, and ADR-0011's first constraint is "off by default,
   at every stage".
2. It is recorded as `rec.Policy = "openai-include-usage"` and **not** bracketed
   by `BodyHashBefore`/`BodyHashAfter` — the mechanism this repository built to
   make "forwards bytes unchanged" checkable rather than promised
   (`internal/ledger/record.go:58-65`).
3. The README's byte-for-byte claim needs the same explicit "unless" clause
   that `--mask` and `--context-edit-trigger` already carry.

**Rank 2** because it is live, on by default, affects the whole of the second
readable wire, and touches the claim the project's first ADR is named after.
Nothing here computes a wrong number; what it does is make a headline promise
untrue on one column without saying so.

## S3. Replay answers every provider with Anthropic's error envelope

`internal/proxy/server.go:1090` writes `{"type":"error","error":{"type":...,
"message":...}}` for every locally generated refusal — circuit open, spend cap,
loop, error budget, preflight deficit (`:1013-1018`) — on every path, including
`/v1/chat/completions`. The comment at `:1035-1036` calls it "the provider's
error shape", singular.

An OpenAI-compatible client receives a 4xx/503 whose body its SDK cannot decode
as an error. **User-visible on every guard refusal on the OpenAI path**, which
is the path most likely to hit a guard because it is the one with no masking,
no policy and the most stub-shaped support. Transport failures are worse still:
`server.go:230-236` returns **plain text** to everyone.

**Rank 3**: smaller numerically than S1 but live today, not latent.

## S4. Gzipped event streams are always parsed with the Anthropic parser

`internal/proxy/server.go:1212-1217` constructs `&ledger.StreamParser{}`
unconditionally and ignores `t.openai`. The live tap is skipped for gzip at
`:1153` (`IsEventStream(ct) && !t.gz`), so a `Content-Encoding: gzip` SSE from
an OpenAI-compatible provider is buffered and then run through Anthropic's
`message_start`/`message_delta` parser, which finds nothing. The record is
written with `Usage: nil` — a request that silently costs nothing.

Compounding it, the two parsers disagree on the SSE data prefix: Anthropic
requires `"data: "` with the space (`ledger/response.go:104`), OpenAI accepts
`"data:"` (`openai_stream.go:48`). An Anthropic-compatible gateway emitting
`data:{...}` parses as nothing on the primary path.

**[nd]** whether any provider in use sends gzipped SSE. Rank 4 because the
failure mode is silent under-counting, which is the one this repository treats
as worst, but the trigger is unmeasured.

## S5. `x-claude-code-session-id` is the universal identity, and the fallback hashes shape

`internal/proxy/server.go:64-65,483` reads Claude Code's private headers on
every path. Every other client falls back to `SessionHash`
(`server.go:534-536`), which for the OpenAI family is a hash of the first two
blocks' **kind, byte size and tool name — never their text**
(`ledger/openai.go:63-93`). Two unrelated sessions whose system prompt and
first message match in kind and byte size to the byte share a session file.
The file itself says so at `openai.go:57-61`, and calls it the smaller error
than writing nothing — which it is, and it is still a collision.

`AgentID` can never be populated off the Anthropic path, so **every sub-agent
and lane analysis in the tool is Anthropic-only** and reports one lane `""` for
everyone else without saying so.

## S6. `ErrorBudget` is inert off the Anthropic wire, and does not say so

`transcript.Block.IsError` is set only from Anthropic's `is_error`
(`internal/transcript/wire.go:59,140`). No other decoder sets it. So the
`ErrorBudget` guard (`internal/proxy/guards.go:240-262`) can never fire on
OpenAI-compatible traffic, and `analysis/errors.go:115-130` falls back to text
markers.

The defect is not that it does not work; it is that **it does not warn**.
Masking has `noteUnmasked` (`server.go:1249-1263`); the error budget has
nothing. A guard that cannot fire is indistinguishable from a guard that found
nothing — the exact standard ADR-0014 sets.

## S7. `cachemodel` gates on the model *string*, not the provider

`foreignModel` (`internal/cachemodel/anthropic.go:188-198`) matches the
substrings `claude opus sonnet haiku fable mythos`. It is the only guard
standing between one provider's arithmetic and another's record, and it is a
substring test on a client-supplied `model` field.

Two consequences. A gateway serving `claude-sonnet-*` over
`/v1/chat/completions` — OpenRouter, LiteLLM, Cursor — passes the family test
and is priced with Anthropic's table. And on the whole OpenAI family the write
premium (`WriteMultiplierShort = 1.25`, `Long = 2.0`, `anthropic.go:103-104`)
**can never fire**, because that family reports no write at all
(`transcript/openai.go:30-33`): `writeEquivalent` (`anthropic.go:357-361`) is
structurally zero. The single largest lever in the cost model silently
evaluates to "no waste found" for every provider that cannot report a write.

## S8. `internal/otlp` labels every span `anthropic`, and is reachable from nothing

`internal/otlp/otlp.go:140` hardcodes `gen_ai.system: "anthropic"` on every
span. The package is imported by nothing (verified, zero import lines).

This matters twice. First, `cache-accounting-shapes-2026-09-07.md` states
"Replay emits it that way as of this file's date" — the corrected
`gen_ai.usage.input_tokens` sum at `otlp.go:148-149`. The emitter is real and
correct and **not wired into any binary**, so the claim is true of the source
tree and not of the tool. Second, the day it is wired, every DeepSeek turn is
exported as `gen_ai.system=anthropic`.

The same dead-code finding covers `internal/quota` (`quota.go`, `forecast.go`),
`internal/feed`, `internal/mutation` and `internal/regression`: **zero import
lines each**. `docs/design/quota-estimator-blue.md:35-37` already says
`Samples`, `Compare` and `Forecast` are "reachable by nothing"; this confirms
it and extends it. `saveQuota` (`cmd/replay/quotastore.go:37`) likewise has no
production caller, so the `replay_quota` MCP tool
(`cmd/replay/mcp.go:269`) returns "no quota reading has been stored" on every
machine.

## S9. One provider, two of its own surfaces, opposite meanings — and the comment picks wrong

`internal/transcript/ollama.go:33-34` and `cmd/replay/burn.go:~196` both state
that "Ollama's `prompt_eval_count` excludes the reused prefix."

That is **measured true** of the llama.cpp server-log line `prompt eval time =
39.10 ms / 1 tokens` on a fully resident prompt
(`ollama-cache-ceiling-2026-09-08.md`) — and **measured false** of the
`/api/chat` response field literally named `prompt_eval_count`, which reported
2823 cold and 2823 warm (`ollama-cache-observable-2026-09-09.md`). Same name,
two surfaces of one provider, opposite semantics.

Nothing computes a wrong number from this today, because the log parser only
ever reads the log. It is on the list because it is precisely the error class
`cache-accounting-shapes-2026-09-07.md` was written to name, reproduced inside
this repository's own comments, and it will mislead the first person who wires
`/api/chat` up.

## S10 (low). `loadSession` sends any `.jsonl` to the Claude Code parser

`cmd/replay/main.go:438-443` tries `ledger.IsLedgerFile` and otherwise falls
through to `ParseClaudeCodeFile`; `transcriptFiles` (`:448-500`) walks any
directory collecting `*.jsonl`. Codex rollouts are `.jsonl`, so `replay cost
~/.codex/sessions` routes them to the wrong parser.

**Ranked last on purpose: it fails loudly.** Codex rollout lines carry no
top-level `uuid` (`internal/transcript/codexdata/*.jsonl`), so `readLines`
(`claudecode.go:157-180`) skips every line and `ParseClaudeCode` returns "no
conversation lines found". Wrong error message, no wrong number. Fixing it is
a nicety; leaving it is not a correctness risk.

---

## Part 2 — per provider: what is alterable, and what altering it buys

Staged as ADR-0011 stages them: **stage 1** offline, sends nothing, no risk;
**stage 2** live, deterministic, semantics-preserving, off by default, fails
open; **stage 3** riskier, gated on evidence stage 2 produced.

## Anthropic — `/v1/messages`

| Alterable | Stage | Buys |
|---|---|---|
| `cache_control` breakpoint placement (`breakpoint-on-stable-block`) | **2** — metadata only, provably identical content | The only family where placement is a lever at all, because it is the only one where the client marks the cache. Named in ADR-0011 and unbuilt. |
| `stable-tool-order`, `hoist-stable-prefix` | **2** | Break causes here are measured as instability, not wording; `causedetail.go:49-63` already names tools-changed and system-changed as distinct causes, so the transform has a measurement to be judged against. |
| Write TTL (5m vs 1h) selection | **2**, evidence-gated | 1.25x against 2.0x is the largest single multiplier in the cost model (`anthropic.go:103-104`) and the split is already reported per request (`wire.go:73-74`), so the counterfactual is scorable offline first. |
| Error envelope on refusal | **already correct here** — S3 is a bug on the *other* columns, not this one | |
| Response rehydration | **already stage 3 in effect** — it synthesises delta events (`masking/stream.go:354-367`). Opt-in, fails open to raw bytes on overflow. | The precedent that stage 2 must match, and the proof that the pattern exists. |

## OpenAI-compatible — DeepSeek (live), Cursor and others (stub)

| Alterable | Stage | Buys |
|---|---|---|
| `stream_options.include_usage` injection | **shipping, and mislabelled** — it is a live request-body mutation on by default (`server.go:1278-1296`) | Without it there is no usage on a streamed response at all, so it buys the entire measurement. It should be bracketed by `BodyHashBefore`/`After` like every other transform, and named in `doctor`, per ADR-0011's own constraint. |
| Parsing `prompt_cache_hit_tokens` / `_miss_tokens` | **1** — read-only, the bytes are already in `RawUsage` | DeepSeek's partition is derived separately from `cached_tokens` (`provider_conformance_test.go:72-84`), so it is a free cross-check on the subtraction at `openai.go:63`. Costs one parser and no risk. |
| A vendor tag on the record | **1** | Nothing distinguishes DeepSeek from Cursor from a local llama.cpp today (`record.go:31-102`). Without it, no per-provider claim in this file can be checked against a live ledger. |
| Prefix stability transforms | **2** | Implicit-prefix family: you cannot lose money, only fail to save it, and the only lever is not perturbing the prefix (`usage.go:36-38`). So stage-2 transforms are the *whole* opportunity here — there is no placement lever and no write to optimise. |
| Emitting the OpenAI error envelope on refusal | **2** — deterministic, keyed on `t.openai` which already exists | Fixes S3. Cheap. |

## Codex

| Alterable | Stage | Buys |
|---|---|---|
| Nothing on the wire — `/v1/responses` is not parsed and Codex writes its own log | **1 only, by construction** | |
| Route the reader through `usage.FromInclusive` + `Validate` | **1** | Fixes S1 before it can fire. The code exists (`usage.go:179,208`) and is unimported. |
| Quota forecasting from the local rollout stream | **1** | The only moving quota corpus that exists: 6,871 events, secondary window observed to 89% (`quota-data-census.md:311-333`). `forecast.go:56` is written, tested, and reachable by nothing. Wiring it is pure stage 1 — reads a file, prints a projection, sends nothing. |
| Publishing a per-token window weighting | **not yet** | Four weightings fit the data equally and a draft that published one was retracted (`codex-quota-2026-09-07.md:57-68`). Report the band, not the number. |

## Ollama / llama.cpp

| Alterable | Stage | Buys |
|---|---|---|
| `cold_rate` calibration, then cache recovery from `prompt_eval_duration` | **1** — local, free, no provider call | Turns an unmeasurable surface into a rankable one. 273x cold/warm separation with no overlap, and it survives 4-way concurrency because the provider reports evaluation work, not wall clock (`ollama-cache-observable-2026-09-09.md`). Recovery within 8pp. |
| Report the unit as **seconds, not dollars** | **1** | 11.9 s of recomputation against 43 ms warm on a 2,823-token prefix. There is no bill; a dollar figure here would be invented. |
| Parsing `/api/chat` NDJSON in the proxy | **1→2** | Currently unhandled anywhere (`server.go:975-1001`). Would make Ollama a live surface rather than a log-reading one — and must not reuse either SSE parser (S4). |
| Server-side knobs (`num_ctx`, `cache_reuse`, `kv_cache_type`) | **3** — these change model behaviour, not framing | `facts.go:75-140` already pins what the defaults are and how each was read. `cache_reuse` defaults to 0 and Ollama never passes the flag, so there is a real lever here — and it changes outputs, so it is not semantics-preserving and cannot be stage 2. |

## Grok, and anything else

| Alterable | Stage | Buys |
|---|---|---|
| Nothing. Forwarded, unread, warned once per path (`server.go:1240-1247`) | — | **Forwarded is the correct state**, and the registry's promotion path for it is honest: capture a payload, land a fixture, write the surface's own conformance condition. |
| Reading `~/.grok/sessions` | **1** | 3.8 GB, 6,787 files, and **zero** usage fields at any depth across 13,444 walked events (`surface-census-2026-09-08.md`). It is a conversation surface, not a spend surface. Buys prompts and tool calls; buys no cost figure, and a claim otherwise was already withdrawn once. |

---

## Part 3 — which provider can actually be tested today

## Yes, today, with no provider call and no money

**Codex.** 150 rollouts, 42 MB, 6,751 usage records, 6,871 rate-limit events
across 147 files, spanning 2026-03-19 to 2026-09-06
(`surface-census-2026-09-08.md`, `quota-data-census.md:279-291`). Four fixtures
in-tree (`internal/transcript/codexdata/`) covering the interesting refusals:
absent breakdown, break, compaction, impossible counts. It is the only surface
where a quota signal **moves** (0→52% primary, 0→89% secondary) and the only
one that needs no proxy, because the client writes it to disk itself. S1 can be
given a red test this afternoon: assert `PromptTotal()` on a parsed rollout
equals `total_tokens`, and watch it fail by the cached share.

**Ollama.** 42 MB of logs, 3,161 requests, 586 carrying `n_past`, plus a live
local server for new measurements at zero cost. The 2026-09-09 work already
established the estimator and its concurrency behaviour. Anything here is
free to re-run, which is the property no other column has.

**Anthropic.** 1,681 transcript files, 1.5 GB, 10 in-tree fixtures including 4
streaming. Strong for *transcript*-tier work. **Weak where it matters most for
this file:** the ledger holds **17 records**, of which **zero** carry any quota
key (`quota-data-census.md:67-91`) — 16 predate `quotaFrom` being wired on
2026-09-05 and the 17th is a 401. So every Anthropic row in table 5 above is a
statement about code, not about captured data, and the census is careful to
call that a coverage gap rather than a demonstrated absence.

**DeepSeek.** 31 fixtures including 22 streaming, captured live 2026-09-05, and
the only OpenAI-family evidence that is not a stub. Enough to test the parser,
the inclusive→exclusive subtraction and the raw-usage retention. Not enough for
anything requiring a *fresh* call, which costs money.

## No

**Cursor.** Measured and killed on the transcript side — 29,665 rows, zero cache
fields. On the wire it shares DeepSeek's parser with **no fixture of its own**,
which is exactly the STUB status the registry gives it and exactly why
RELEASE-CRITERIA gates it at v1.0.

**Grok.** 3.8 GB on disk and no usage anywhere in it; `x-ratelimit-*` headers
that did not move across 8 calls. Testable as a conversation store; untestable
as a spend or quota surface.

**OpenAI proper, Google.** No corpus, no fixture, no capture. The Google row in
`cache-accounting-shapes-2026-09-07.md` is marked read-from-documentation and
stays that way here.

## Which I would build for first

**Codex**, and the reason is not that it is the biggest.

It is the only column where the three things this file cares about are all
present at once: a real session id the provider itself assigns, a usage record
per turn, and a **quota counter that moves**. Everywhere else at least one is
missing — Anthropic has the sessions and no quota, Ollama has the cache signal
and no bill and no session, Grok has volume and nothing else, DeepSeek has a
parser and 31 files.

And it is the column carrying the highest-ranked latent bug, with the fix
already written and unplugged. Building for Codex means wiring
`usage.FromInclusive` and `internal/quota`, which fixes S1, revives ~380 lines
of tested dead code, and produces the only forecast on this machine that has
data behind it — all of it stage 1, offline, sending nothing.

The second choice is Ollama, because it is free to measure and the estimator is
already validated. The one to avoid first is Anthropic: it looks like the
best-supported surface and is, for cache forensics, but its ledger is 17
records deep and the quota row is blank.

---

## Scope, honestly

One machine, one operator, 2026-09-09, and the versions installed on it. Every
**[code]** cell is checkable by opening the cited line and is a claim about
Replay only. Every **[measured]** cell names a dated evidence file whose own
scope caveats apply unchanged. The single **[doc]** cell is inherited, and is
marked because conflating them with measurement is the error this repository
keeps correcting.

Nothing in this file was changed in the source tree. The six **[nd]** cells are
left empty rather than guessed, and the largest of them is the same one part 3
cannot close on its own: no live OpenAI-compatible provider other than DeepSeek
has ever answered this proxy.

---

[Design notes](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
