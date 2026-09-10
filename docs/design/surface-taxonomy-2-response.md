# Surface taxonomy, part 2: the response path and the ledger

**Scope.** From the provider's first byte back, through parsing and
classification, to what lands on disk — plus the rescore that runs after the
response is delivered. The request path, storage/analysis consumers, and
per-provider differences are other agents' lanes.

**Framing.** "Byte-for-byte passthrough" is a *setting*, not an identity. On the
response path it is `Rehydrator == nil || NoPolicy` (`internal/proxy/server.go:558`).
Everything below is a place where Replay **reads**, **records**, **classifies**,
or **could alter** — 41 surfaces. Every row carries a `file:line`. Rows I could
not ground are not here.

**Method.** Read-only. Claims are from source at HEAD in
`/Users/daniel/Development/Replay-clean`. Where a pass condition has no test,
the row says **NO TEST** — that is the finding, not an omission.

---

## Summary

"Alterable" below means **without changing behaviour-defining code**: a
`Config` flag, a package-level `var`/`const`, or client/provider-controlled
input. Every other row is alterable only by editing the logic, which is a
weaker claim and is marked "code only".

| | Count |
|---|---:|
| Surfaces identified | **41** |
| Alterable by flag, constant, or controlled input | **18** |
| Alterable only by editing logic ("code only") | **23** |
| Pass conditions with **no** enforcing test | **16** |
| Surfaces that fail **silently** (no log, no ledger field, no counter) | **9** |
| Known defects reproduced | **2 of 2** |

The nine silent-failure surfaces are A5, B3, B5, C3, C5, D1, D5, F2, G5.

---

## A. Header capture

| Surface | Primitive | Default state | Pass condition | Fail condition | Alterable? | What alteration does | Optimisation potential |
|---|---|---|---|---|---|---|---|
| **A1. Status capture** | `responseTap.status` | on, unconditional | `rec.Status` is the status the client saw | a status that never reaches the ledger | code only | — | none; already exact |
| **A2. Provider request id capture** | `providerRequestID(tap.Header())` → `Record.RequestID`: `request-id`, else `x-request-id` (`quota.go:28-41`, `server.go`) | on, unconditional | the provider's own id lands so a support ticket can be joined to a record, on **both** wire families | id absent → `requestFromRecord` synthesises `ledger-<index>` and marks `Request.IDMeasured` false, so no consumer joins on it across files (`store.go`) | code only | dropping it makes ledger requests unjoinable to provider-side traces | none |
| **A2b. Lane correlation** | `stats.enterLane` / `Record.Correlation` (`state.go`, `server.go`) | on, unconditional | every record says whether another request of its lane was open at the same time: `lane-serial` or `lane-overlap`, absent when unmeasured | an overlapped pair classified against whichever response finished first, which is a reading of the race | code only | removing it returns per-event cause attribution to arrival order | none; this is the reading nothing else can take |
| **A3. Quota header allowlist** | `quotaFrom` prefix match `anthropic-ratelimit-`, `x-ratelimit-` + exact `retry-after` (`quota.go:16-26,46-75`) | on, unconditional | every quota header lands verbatim, keyed by its own lowercased name; **nothing else does** | a provider-controlled header (cookie, token echo) reaching an append-only file the user publishes from | **yes** — the two slices are package-level vars | adding a prefix widens what a provider can write into the ledger; removing one silently unmeasures a real limit | **yes**: a new limit dimension appears as a new header. Test: add a header under a new prefix to `quotaUpstream` and assert it lands. |
| **A4. Values stored unparsed** | `map[string]string`, no numeric coercion (`quota.go:29-40`) | on | a wrong parse fails visibly in analysis, not silently as a zero | a typed struct guessing one shape per field | code only | typing it converts "did not parse" into "provider reported none" | none; the unparsed form is the correct choice |
| **A5. Only the first value of a repeated header** | `values[0]` (`quota.go:63,69`) | on | single-valued headers survive | a repeated `retry-after` loses all but the first | code only | — | **NO TEST** |
| **A6. Quota is captured once, after retries** | `rec.Quota = quotaFrom(tap.Header())` in the bookkeeping defer (`server.go:595`) | on | the record carries the headers of the response the client saw | **the 429s that caused the retries are gone** — see **Defect 2** | **yes** (`Config.Retries`) | turning retries off makes every attempt its own record | **yes**, and it is the highest-value alteration in this lane |

**Tests.** A3/A4: `TestQ1_AnthropicQuotaHeadersAreCaptured`
(`internal/proxy/quota_test.go:31`), `TestQ2_OpenAIQuotaHeadersAreCaptured` (`:62`),
`TestQ3_OnlyQuotaHeadersAreCaptured` (`:84`), `TestQ4_NoQuotaHeadersYieldsNothing`
(`:111`), `TestQ5_RetryAfterIsCaptured` (`:130`). A6 wiring:
`TestQ6_QuotaReachesTheLedger` (`:159`) — drives a real request and reads the
file, which is the ADR-0014 shape. **A5 and the retry case have no test.**

---

## B. Body capture and decoding

| Surface | Primitive | Default state | Pass condition | Fail condition | Alterable? | What alteration does | Optimisation potential |
|---|---|---|---|---|---|---|---|
| **B1. Tap writes client-first** | `t.ResponseWriter.Write(p)` then parse `p[:n]` (`server.go:1163-1181`) | on | the tap never delays or truncates delivery | a parser error affecting the client's bytes | code only | — | none; this is the invariant |
| **B2. Flush passthrough** | `Flush()` delegated (`server.go:1184-1188`), `FlushInterval: -1` (`server.go:223`) | on | SSE frames reach the client as they arrive | buffering that batches a stream | code only | — | `TestStreamingIsFlushedIncrementally` (`server_test.go:310`) |
| **B3. Response buffer cap** | `MaxResponseBytes = 16 << 20` (`server.go:58`); over it `t.dropped = true` (`server.go:1177`) | 16 MB | an oversized body is forwarded intact and simply not parsed | **`result()` returns `ledger.Response{}` with no log, no counter, no field** (`server.go:1197-1199`) — the record looks like a response that carried no usage | **yes** — a constant | raising it costs memory per in-flight request; lowering it silently unmeasures more traffic | **yes**: record the drop as a field so "dropped" is distinguishable from "no usage". Test: write `MaxResponseBytes+1` bytes and assert the record says the body was dropped rather than reporting empty. **NO TEST today.** |
| **B4. gzip decode** | `t.gz` set only for `Content-Encoding: gzip` (`server.go:1152`), decoded in `result()` (`server.go:1201-1211`) | on | a gzipped response is forwarded compressed and still parsed | a decode error → `Response{}`, silent | code only | — | `TestGzipResponseIsForwardedCompressedAndParsed` (`server_test.go:350`) |
| **B5. Any other content coding** | no branch for `br`, `zstd`, `deflate` (`server.go:1152`) | unhandled | — | **compressed bytes are handed to `ParseResponse`, `json.Unmarshal` fails, `Response{}` returned silently**; usage, spend and the session request are all lost | **yes** — the client's `Accept-Encoding` decides, and the proxy forwards it (`DisableCompression: true`, `server.go:194`) | a client that negotiates `br` unmeasures its own traffic with no warning | **yes**: either decode them or emit the once-per-encoding warning `noteUnparsed` already models (`server.go:1240`). Test: serve `Content-Encoding: br` and assert a warning, not a silent empty record. **NO TEST.** |
| **B6. gzipped event stream** | buffered whole, then re-parsed by a fresh `StreamParser` (`server.go:1212-1217`) | on | a compressed SSE stream still yields usage | over `MaxResponseBytes` it falls into B3 | code only | — | **NO TEST** for the gzip+SSE combination specifically |
| **B7. Parser selection** | `tap.openai` set from the **request path**, not the body (`server.go:557`, `1126-1129`, `1153-1158`) | path-derived | the right parser runs without guessing from bytes | a body-sniffing heuristic where the path already knows | code only | — | `isChatCompletions` (`server.go:999`) |

---

## C. Streaming reassembly

| Surface | Primitive | Default state | Pass condition | Fail condition | Alterable? | What alteration does | Optimisation potential |
|---|---|---|---|---|---|---|---|
| **C1. Anthropic SSE line framing** | `StreamParser.Write` splits on `\n`, only `data:`  lines parsed (`ledger/response.go:80-110`) | on | usage and block structure accumulate as bytes pass | a frame split across chunks losing data | code only | — | `TestParseResponseAndStream` (`ledger_test.go:110`), `TestClaudeCodeWireStreams` (`anthropic_conformance_test.go:203`) |
| **C2. Unbounded-line guard (Anthropic)** | `maxPendingLine = 1 << 20`; over it `dropped = true`, buffer reset (`response.go:58,85-91`) | 1 MB | the parser stops growing rather than consuming memory without bound | everything after the cap is lost, and `Result()` reports no usage rather than partial | **yes** — a constant | — | `TestStreamParserStopsOnAnEndlessLine` (`ledger_test.go:303`) |
| **C3. Unbounded-line guard (OpenAI) — absent** | `OpenAIStreamParser.Write` has **no** equivalent cap (`openai_stream.go:33-45`) | none | — | a newline-free OpenAI-compatible stream grows `p.buf` without bound, in the response path of a live proxy | **yes** — add the cap | the Anthropic parser already carries the fix; this one does not | **yes**: port `maxPendingLine`. Test: feed 2 MB with no newline and assert the buffer stops growing, mirroring `TestStreamParserStopsOnAnEndlessLine`. **NO TEST.** |
| **C4. Usage merge across frames (Anthropic)** | `merge` takes each field **only when non-zero** (`response.go:168-187`) | on | `message_delta` refreshes input accounting without erasing `message_start` | a genuine zero in the final frame cannot lower an earlier non-zero — a structural bias toward the larger figure | code only | a strict last-frame-wins rule would trust a provider that sends partial finals | none I can test today |
| **C5. Block-index bounds** | `s.blocks[ev.Index]` guarded by `ev.Index < len(s.blocks)` (`response.go:152`) | on | an out-of-order delta cannot panic the response path | deltas for an unstarted block are silently discarded | code only | — | **NO TEST** for the out-of-range branch |
| **C6. Raw usage frame selection (Anthropic)** | last frame carrying usage wins; `message_start` lifted from under `message` (`response.go:141,161,215-222`) | on | `RawUsage` holds the final accounting | an early frame overwriting the final one | code only | — | `TestRawUsageCarriesUndeclaredFields` (`anthropic_conformance_test.go:178`) |
| **C7. OpenAI SSE, last frame wins** | `p.usage = frame.Usage` on every frame carrying one (`openai_stream.go:77-86`) | on | the final totals frame is the one recorded | an intermediate frame winning | code only | — | `TestOpenAIStreamParserReadsUsageFromTheFinalFrame` (`openai_stream_test.go:11`), `...SurvivesFrameSplitting` (`:37`) |
| **C8. Absent stream usage is *none*, not zero** | `if p.usage != nil` (`openai_stream.go:101`) | on | a stream without `include_usage` reports no usage | a zero would tell the spend cap the request was free | code only | — | `TestAStreamWithNoUsageReportsNoneNotZero` (`openai_stream_test.go:59`) |
| **C9. Streamed text is never kept** | `b.Text = ""` at `content_block_start` (`response.go:148`); byte counts only thereafter | on | the ledger holds sizes, never content | any assistant text reaching disk | code only | — | `TestLedgerHoldsNoArgumentContent` (`ledger_test.go:74`) |

---

## D. Usage parsing and the inclusive/exclusive conversion

| Surface | Primitive | Default state | Pass condition | Fail condition | Alterable? | What alteration does | Optimisation potential |
|---|---|---|---|---|---|---|---|
| **D1. Anthropic non-streaming parse** | `ParseResponse`, gated on `msg.Type == "message"` (`ledger/response.go:14-38`) | on | an error object yields an empty response and no error; the status carries the outcome | a 200 whose shape this build does not know is silently unmeasured | code only | — | `TestErrorResponsesPassThroughAndAreRecordedWithoutUsage` (`server_test.go:376`) |
| **D2. Anthropic wire → analysis usage** | `WireUsage.Usage()` (`transcript/wire.go:80-93`) — exclusive in, exclusive out, no arithmetic | on | `input_tokens` is copied, not derived | any conversion on a path that needs none | code only | — | `TestOutputIsCopiedNotDerived` (`provider_conformance_test.go:525`) |
| **D3. Inclusive → exclusive conversion (OpenAI family)** | `Input = PromptTokens - cached` (`transcript/openai.go:44-71`) | on | the cache is counted **once**; error grows with hit rate if wrong, so it is largest on exactly the sessions the tool exists for | copying `prompt_tokens` into `Input` double-counts the cache (the docstring names Langfuse's shipped bug, `openai.go:21`) | code only | — | `TestLive_CachedTokensSurviveTheInclusiveConversion` (`live_cached_provider_test.go:41`) |
| **D4. Clamp on impossible numbers** | `cached > PromptTokens → cached = PromptTokens`; `cached < 0 → 0` (`openai.go:56-61`) | on | a provider bug cannot produce a negative fresh count that reads downstream as a saving | an unclamped subtraction | code only | — | `TestClampOnImpossibleProviderNumbers` (`provider_conformance_test.go:439`) |
| **D5. All-zero usage is absence, not zero** | `raw.Usage = nil` when prompt and completion are both 0 (`ledger/openai.go:128-130`) | on, **OpenAI path only** | a gateway's `"usage": {}` on a refusal path does not enter averages as a free request | a zeroed record averaging in as free | code only | **the Anthropic path has no equivalent guard** (`ledger/response.go:31-35` keys only on `msg.Usage != nil`) | **yes**: apply the same rule on the primary path. Test: an Anthropic 200 with `"usage":{}` must yield no usage, mirroring `TestEmptyUsageObjectIsNotAZeroMeasurement` (`provider_conformance_test.go:481`). **NO TEST on the Anthropic side.** |
| **D6. Applied context edits** | `context_management.applied_edits` summed into `AppliedEdits`/`ClearedInputTokens` (`response.go:36,47-55,189-194`) | on | the applied policy's measured side reaches the record | a policy claimed and never measured | code only | — | `TestResponsesReportAppliedContextEdits` (`ledger_test.go:205`) |
| **D7. `RawUsage` verbatim capture** | the provider's `usage` object lifted unparsed (`ledger/openai.go:164-174`; Anthropic `response.go:34`) | on | a field nobody knew mattered survives for a later calibration | round-tripping through a typed struct drops every undeclared field — the DeepSeek `prompt_cache_hit_tokens` defect, fixed 2026-09-05 (`openai.go:136-141`) | code only | — | `TestOpenAIRawUsageKeepsUnknownFields` (`rawusage_test.go:17`), `TestRawUsageCarriesUndeclaredFields` |
| **D8. `RawUsage` is a passthrough, not an allowlist** | `rawUsageBytes` returns whatever `usage` holds, unbounded, unfiltered (`openai.go:164-174`) | on | — | **a provider-controlled JSON blob of arbitrary size and content enters an append-only file** — precisely the risk `quotaFrom`'s allowlist rationale names for headers (`quota.go:42-45`) | code only | a size cap or a key filter would trade calibration value for containment | **stateable test**: assert a bound on stored `RawUsage` length. Recording it here as a code-vs-code tension: two surfaces on the same response, opposite trust postures. **NO TEST.** |

---

## E. Cache classification (the live read)

| Surface | Primitive | Default state | Pass condition | Fail condition | Alterable? | What alteration does | Optimisation potential |
|---|---|---|---|---|---|---|---|
| **E1. Read classification** | `cachemodel.ClassifyRead(ln.last, cur)` → reproduced / exceeded / broken + expected (`state.go:261`, `cachemodel/anthropic.go:282-292`) | on when the lane has a previous request | a break is named against **this lane's** own previous request | comparing against the session's last request forges breaks under fan-out | code only | — | `TestObserve_ConcurrentLanesDoNotForgeCacheBreaks` (`lastlane_test.go:31`), `...AGenuineWithinLaneBreakIsStillCaught` (`:68`) |
| **E2. Prefix-hash comparison** | `prefixChanged := ln.seen && rec.PrefixHash != ln.prefixHash` (`state.go:256`), per lane, updated at `:274` | on | a changed prefix is a **certain** cause: the proxy hashed both requests | a first request in a lane read as a change | code only | — | `TestObserve_ConcurrentLanesDoNotForgeAPrefixChange` (`prefixlane_test.go:32`), `...OpeningALaneIsNotAChange` (`:94`) |
| **E3. Break-cause vocabulary** | `breakCause` → `BreakCause` constants (`state.go:436-449`, `cachemodel/anthropic.go:301-321`) | on | stays a **bounded** vocabulary, because it is emitted as `replay_cache_break_total{cause=...}` | tool names in a Prometheus label = unbounded cardinality | code only | — | `TestBreakCause_StaysABoundedVocabulary` (`causedetail_test.go:116`) |
| **E4. `CauseDetail` free text** | `diffPrefix` → added / removed / resized tool names, capped at `maxNamedTools = 6` (`causedetail.go:33,67-91,98-126`) | on | names what actually changed, for a person; never a metrics label | reporting "system prompt or tool definitions changed" while holding the tool list | **yes** — `maxNamedTools` | raising it makes a log line nobody reads | `TestBreakCause_NamesWhatChanged` (`causedetail_test.go:30`), `TestPrefixChangeIsNamedAsBreakCause` (`server_test.go:855`) |
| **E5. Fallback classification** | `ClassifyBreak` on usage + timing when the prefix is unchanged; `CauseUnknown` when only history can tell (`state.go:444-448`) | on | the proxy does not guess a cause the data cannot settle | naming a half at random (`causedetail.go:57-62`) | code only | — | `TestLiveCacheBreakIsLoggedRecordedAndCounted` (`server_test.go:634`) |

---

## F. What is written to the ledger, and what is deliberately dropped

**Kept** (`internal/ledger/record.go:31-174`): schema, timestamp, session id, agent
id, request id, path, model, effort, stream flag, prompt **structure** (system
bytes, tool bytes/count/names, cache-control count, per-message block kinds and
sizes), prefix hash, policy name, status, refusal + reason, body hashes before
and after any transform, mask counts, `mask_degraded`, rehydration counts and
denials, retries, held ms, latency ms, **quota**, response blocks, parsed usage,
**raw usage**, applied edits, cleared tokens, cache outcome + cause + detail.

**Deliberately dropped**: all message and block **text** (`stripText`,
`summarize.go:121-127`; `response.go:24,148`); tool **arguments** beyond a keyed
`CallKey` (`summarize.go:54-65`); file **paths**, replaced by an
HMAC over a per-ledger key (`summarize.go:41-49`, key at `store.go:67-80`);
request and response **headers** other than the quota allowlist and `request-id`;
credentials; `SessionHash` (`json:"-"`, `record.go:119`); **cache-control marker
positions** — `Prompt.CacheControlCount` is a count only, and `evidence.go:24-37`
names this as the single most valuable capture the prefix-evidence feed is
missing.

| Surface | Primitive | Default state | Pass condition | Fail condition | Alterable? | What alteration does | Optimisation potential |
|---|---|---|---|---|---|---|---|
| **F1. Record assembly** | the bookkeeping `defer` (`server.go:583-650`) | on | runs even when the client disconnects mid-stream — the provider billed that turn (`server.go:578-582`) | a panic path that skips the record | code only | — | `TestClientAbortMidStreamIsStillRecorded` (`server_test.go:550`) |
| **F2. Write gate** | `if readable && rec.SessionID != ""` (`server.go:606`) | on | only paths this build can read are recorded | **a readable request whose body would not summarise and that carried no session header produces no record at all** — `rec.SessionID` falls back to `SessionHash` only when summarising succeeded (`server.go:534-536`) | code only | — | `TestCountTokensIsForwardedButNotRecorded` (`server_test.go:437`); the unsummarised-and-unheadered case has **NO TEST** |
| **F3. Refusal record** | `recordRefusal` writes from its own path, deliberately not the defer (`server.go:1057-1081`) | on | a guard firing is analysable, not just a log line; and it never re-arms the breaker | routing refusals through the defer would keep an open circuit open forever (`server.go:1041-1044`) | `Store == nil` disables | — | `TestRefusalIsRecordedOnTheLedger` (`refusal_test.go:101`), `...RecordCarriesNoContent` (`:132`), `...IsNeverObservedAsAProviderFailure` (`:66`) |
| **F4. Schema gate on read** | `rec.Schema != SchemaVersion → skipped++` (`store.go:156-162`) | `SchemaVersion = 2` | an older record is counted as skipped, never misread as current | figures that look measured and are not | **yes** — bumping the constant discards every existing ledger | this is why `Refusal` was added as an optional field rather than a bump (`record.go:48-50`) | `TestStoreRoundTripToSession` (`ledger_test.go:147`) asserts `Skipped == 2` |
| **F5. Session file naming** | `sessionFileName` sanitises then appends 8 hex of SHA-256 when it changed anything (`store.go:110-120`) | on | two ids that sanitise alike never share a file | cross-session contamination in one file | code only | — | `TestSessionFileNamesNeverCollide` (`ledger_test.go:290`) |
| **F6. Session-derived reader mapping** | `requestFromRecord` (`store.go:249-286`) | on | every field a session reader needs survives the record→session conversion | **`Quota` is dropped** — see **Defect 1** | code only | — | **NO TEST** asserts field coverage |

---

## G. File permissions and write behaviour

| Surface | Primitive | Default state | Pass condition | Fail condition | Alterable? | What alteration does | Optimisation potential |
|---|---|---|---|---|---|---|---|
| **G1. Directory permission** | `dirPerm = 0o700`, `os.MkdirAll` (`store.go:24,52`) | 0700 | the ledger directory is the user's own and nobody else's | a group- or world-readable directory of derived traffic data | code only | — | **NO TEST.** `internal/proxy` tests 0600 on the *sockets* (`uds_test.go:183`, `metrics_listener_test.go:254`); nothing asserts it on the ledger. Effective mode is also umask-dependent — untested. |
| **G2. File permission** | `filePerm = 0o600` on session files, `.label-key`, `.pins`, `.revert` (`store.go:25,76,96,378,418`) | 0600 | same | same; the `.label-key` is what makes path hashes unmatchable (`summarize.go:35-37`) | code only | — | **NO TEST.** ADR-0010:46 states "`0600` in `0700`" — code agrees; **nothing enforces it**. Test: `os.Stat().Mode().Perm()` on each of the four files after `Open` + `Append`. |
| **G3. Append durability** | `os.OpenFile(O_CREATE\|O_APPEND\|O_WRONLY)`, one `Write`, `Close`; **no `Sync`** (`store.go:96-105`) | on | a crash loses at most the tail; earlier records stay readable, and a torn line is counted as `skipped` rather than corrupting the file (`store.go:156-162`) | a torn line reported as data | code only | adding `Sync()` per record costs a fsync on the response path | **code vs docs**: ADR-0010:46 says "Crash-safe by construction". That is true of *readability* and false of *durability* — there is no fsync, and the last record can be lost. Both citations recorded. **NO TEST.** |
| **G4. Cross-process append** | `s.mu` is a **process-local** mutex (`store.go:94-95`) | on | two goroutines never interleave a line | **two proxy processes sharing one ledger dir have no lock**; interleaving relies on O_APPEND alone, which is not guaranteed atomic for arbitrary record sizes | **yes** — one proxy per ledger dir is the assumption | ADR-0015 names single-tenant state as a boundary | **NO TEST** |
| **G5. `.revert` write is not atomic** | `os.WriteFile` (`store.go:378`) | on | a revert survives a restart | a crash mid-write leaves a truncated file; `loadRevert` then returns "no revert" (`store.go:388-398`) — **fails open, the policy stays on** | code only | — | **Inconsistent with the same repo**: `guards.go:469` and `masking/vault.go:144` both use temp-file + `os.Rename`, and `cmd/replay/quotastore.go:53-71` does too. `TestRevertPersistsAcrossReopen` (`ledger_test.go:266`) covers only the happy path. **Optimisation**: temp+rename. Test: write a truncated `.revert`, reopen, assert the revert either survives or is reported — not silently discarded. |
| **G6. `.pins` append** | append-only, last line for a session wins on reload (`store.go:408-431`, `loadPins` `:436-458`) | on | an unparseable pin is a decision to make again, not a refusal to start | a corrupt pin bricking the proxy | code only | — | `TestPinsPersistAcrossReopen` (`ledger_test.go:226`) |

---

## H. Rehydration — the one response-body **alteration** that ships

| Surface | Primitive | Default state | Pass condition | Fail condition | Alterable? | What alteration does | Optimisation potential |
|---|---|---|---|---|---|---|---|
| **H1. Rehydration switch** | `Rehydrator != nil && messages && !NoPolicy` (`server.go:558`) | **off** | with it off the response body is byte-for-byte | a body altered without the operator asking | **yes** — this *is* the setting that makes passthrough conditional | ADR-0004 | `TestRehydrationOffLeavesResponsesAlone` (`server_test.go:1932`) |
| **H2. Accept-Encoding suppression** | `r.Header.Del("Accept-Encoding")` when rehydrating (`server.go:561`) | off | a compressed body cannot be rewritten as it passes, so uncompressed is requested | rewriting compressed bytes | follows H1 | costs bandwidth on every rehydrated session | `server_test.go:1854` asserts it does not reach the provider |
| **H3. Stream rehydration** | `masking.NewTransformReader` + `Content-Length` deleted, `ContentLength = -1` (`server.go:712-718`) | off | a declared length cannot cut the client off | a stale `Content-Length` truncating the response | follows H1 | — | `TestRehydrateStreamAcrossChunksAndDeltas` (`masking/rehydrate_test.go:189`) |
| **H4. JSON rehydration** | body read to `MaxResponseBytes`, rewritten, `Content-Length` reset (`server.go:719-742`) | off | the rewritten length is declared correctly | a length mismatch | follows H1 | — | `TestRehydrateBodyRestoresWithinScope` (`masking/rehydrate_test.go:47`) |
| **H5. Skip reporting** | `h.skipped` for compressed / oversize / unread bodies (`server.go:706-709,722,727`), logged at `server.go:766-768` | off | a skipped rewrite is **announced**, not silent | placeholders reaching the client with no word said | follows H1 | — | `server_test.go:1904` asserts `"rehydration skipped session=... : compressed response"` |
| **H6. Rehydration counts to the ledger** | `rec.Rehydrated`, `rec.RehydrationDenied` by destination — never a value or a path (`server.go:762`) | off | counts only | a secret or a path in the ledger | follows H1 | — | `TestRehydrationRestoresPlaceholdersWithinScope` (`server_test.go:1812`) |

---

## I. What runs **after** the response is delivered

| Surface | Primitive | Default state | Pass condition | Fail condition | Alterable? | What alteration does | Optimisation potential |
|---|---|---|---|---|---|---|---|
| **I1. Breaker observation** | `Breaker.Observe(upstreamFailed \|\| IsRetryableStatus(status))` only when a status arrived (`server.go:585-590`) | on when `Breaker` set | a client that left before any response is **no observation of the provider** | a user's Ctrl-C tripping the circuit | **yes** (`Config.Breaker`) | — | `TestBreakerHoldsRequestsAfterProviderFailures` (`server_test.go:581`) |
| **I2. Spend recording** | `Spend.Record(session, tokens, listCost(usage, model))` (`server.go:598-600`) | on when `Spend` set | an unpriced model contributes nothing **and says so** via `CapNotEnforced` (`server.go:449`) | a cap that silently never fires | **yes** | — | `TestUnpricedTrafficDoesNotFabricateAZeroCost` (`daycost_test.go:87`), `TestDollarCapOnAnUnpricedModelIsReportedNotSilentlyIgnored` (`guards_test.go:190`) |
| **I3. Rescore / what-if** | `stats.rescore` — builds the session incrementally and runs `analysis.AnalyzeLane` (`state.go:457-547`), called at `server.go:614` | on for readable requests with usage | **runs after the response has been delivered** and takes only the session's own lock, so other sessions are never held up (`state.go:451-456`) | scoring on the request path, adding latency to the turn | code only | — | `TestWhatIfMatchesOfflineReplayAndStaysOffTheWire` (`server_test.go:793`), `TestRescore_SessionCostDoesNotGrowWorseThanQuadratic` (`rescore_bench_test.go:182`), `BenchmarkRescoreBySessionLength` (`:30`) |
| **I4. Rescore lane isolation** | `st.whatIf[rec.AgentID] = rows` replaces that lane's figure, never the session's (`state.go:507-527`); `covered` captured under `scoreMu` (`state.go:476-481`) | on | a sub-agent lane does not erase its siblings' context | the race CI's `-race` caught on ubuntu and 25 local runs did not (`state.go:477-480`) | code only | — | `TestRescore_ALaneDoesNotEraseItsSiblingsContext` (`contextlane_test.go:30`), `TestStatus_ReReadsAndWhatIfAreAlsoPerLane` (`:92`) |
| **I5. Trial guardrail** | `Trial.breached(rr)` → `noteBreach` writes `.revert` (`server.go:615-617`, `state.go` breach path) | on when `PolicyFile` + `Trial` set | evidence gathered against one trigger cannot revert a different one | one counter disarming the guardrail for every policy after it (`state.go:163-168`) | **yes** (`Config.Trial`) | — | `TestBreachesDoNotCarryAcrossPolicies` (`revertscope_test.go:34`), `TestGuardrailBreachesRevertThePolicyUntilANewerFile` (`server_test.go:1590`) |
| **I6. What-if log cadence** | one line every `whatIfLogEvery` requests (`state.go:530`) | periodic | a log a person reads | a line per request | **yes** — a constant | — | covered indirectly by `server_test.go:793` |
| **I7. Request log line** | one line per request, never headers, never bodies (`server.go:629`) | on | session ids shortened to 12 chars (`server.go:1107-1112`) | content in a log | `Logger` | — | `TestRefusalIsLoggedWithAttribution` (`refusal_test.go:24`) |
| **I8. Prefix evidence extraction** | `EvidenceFrom` — only `status == 200`, no refusal, `CacheCreation > 0`, **and `CacheRead == 0`** (`evidence.go:38-86`) | on where called | only a **cold** write measures the caching floor; a warm increment says nothing about it | reading a 118-token warm increment as a prefix size produced "opus-5 caches at 118, contradicting a documented 512" — a fabricated refutation from four real sessions (`evidence.go:57-68`) | code only | dropping the `CacheRead == 0` filter re-introduces exactly that | `TestL1_CacheWriteGivesAnExactUpperBound` (`evidence_test.go:44`), `TestL6_AWarmWriteIsNotFloorEvidence` (`:151`), `TestL2_NoLowerBoundIsInvented` (`:72`) |

---

## The two reported defects

### Defect 1 — `requestFromRecord` drops `Quota`: **REPRODUCED**, and it is worse than reported

`internal/ledger/store.go:249-259` constructs the `transcript.Request`:

```go
req := &transcript.Request{
    ID: rec.RequestID, Model: rec.Model, Effort: rec.Effort,
    Timestamp: rec.Timestamp, Usage: *rec.Response.Usage,
    AppliedEdits: rec.Response.AppliedEdits,
    ClearedTokens: rec.Response.ClearedInputTokens,
    Tools: rec.Prompt.Tools,
}
```

`rec.Quota` is not copied. **It cannot be**: `transcript.Request`
(`internal/transcript/types.go:122-139`) has no `Quota` field, nor does
`transcript.Lane` (`:151-155`) or `transcript.Session`. So the drop is structural,
not a missed line — any reader that goes `ledger.ReadFile` → `sessionFromRecords`
→ `requestFromRecord` (`store.go:172-192`) sees a session with no rate-limit data
at all, over a ledger that may be full of it.

`Add` also skips any record with `Response.Usage == nil` (`store.go:217-220`) — so
a **429 or a 401**, the records most likely to carry `retry-after` and the
tightest `remaining` values, never becomes a request in a session even in
principle.

Three additional findings the reporting agent did not have:

1. **The only consumer of `Record.Quota` is orphaned.** `internal/quota`
   (`quota.go:112-141`, `Samples` at `:151`) reads `r.Quota` off `ledger.Record`
   directly — correctly, bypassing the session layer. But
   `go list -deps ./cmd/replay` does not contain it. `grep -rn "internal/quota"`
   over all `.go` files returns nothing. **No shipped command reads captured
   quota headers by any route.**
2. **The shipped quota surface reads a different source entirely.** `replay`'s
   quota line comes from Claude Code's status-line stdin JSON
   (`cmd/replay/statusline.go:51`), persisted to `~/.replay/quota.json`
   (`cmd/replay/quotastore.go:37-71`) and read back by the MCP tool
   (`cmd/replay/mcp.go:268-274`) — never from the ledger.
3. **The population is empty anyway.** `docs/design/quota-data-census.md` records
   0 of 17 local ledger records carrying a `quota` object, because `quotaFrom`
   was wired in on 2026-09-05 and 16 of the 17 records predate it.

**No test asserts field coverage across `requestFromRecord`.** `TestStoreRoundTripToSession`
(`ledger_test.go:147-203`) round-trips a record and checks usage, context and
output — it never constructs a record with `Quota` set.

### Defect 2 — retries keep only the final attempt's headers: **REPRODUCED**

Three citations, in order:

1. `retryTransport.RoundTrip` (`internal/proxy/retry.go:67-101`) loops. On a
   retryable status it calls `drain(resp)` (`:82`), which reads up to 64 KB into
   `io.Discard` and closes the body (`:161-165`). The failed `*http.Response` —
   **headers included** — goes out of scope. Only the last `resp` is returned
   (`:73`).
2. The reverse proxy therefore only ever sees the final response, and writes only
   its headers onto the tap. `tap.Header()` delegates to the embedded
   `http.ResponseWriter` (`server.go:1130`).
3. `rec.Quota = quotaFrom(tap.Header())` runs **once**, in the bookkeeping defer
   (`server.go:595`), after `s.rp.ServeHTTP` has returned.

Consequence: a turn that hit `429, 429, 200` is recorded as one record with
`Retries = 2` (`server.go:592`) and the **200's** headers. The 429s' `retry-after`
and their `anthropic-ratelimit-*` values — the tightest, most informative
readings, and the only direct evidence of a lockout — are discarded. Observed
lockouts structurally undercount, and `retry-after`, which `quota.go:22-26` calls
"the lockout itself… for a flat seat that is the whole event", is the field most
likely to be lost, because it appears on exactly the responses the retry
transport eats.

The retry accounting that *does* survive is deliberate and tested — the count
(`record.go:82-84`), the log lines, the `replay_retries_total` metric. The
existing test drives `529, 429, 503, 200` with `Retry-After: 0` on the first
(`server_test.go:1060-1093`, upstream at `:1021-1048`) and asserts the count, the
status and the log. **It asserts nothing about headers, and no test in the repo
does.** The one test that would catch this — a 429 with rate-limit headers
followed by a clean 200, asserting those headers survive — does not exist.

Note the interaction with **F2**: a 429 whose body did summarise *is* recorded
with its headers when retries are **off** (`Config.Retries.Attempts == 0`,
`server.go:196-201`). So the defect is created by turning retries on, which is
also the setting that hides the event from the user.

---

## The one alteration I would build first

**Record every attempt's quota headers, not only the last.**

It is the smallest change that fixes the larger of the two defects, it targets
the only field on the record denominated in the currency a flat-seat subscriber
actually spends, and it is the one alteration here with a clean optimisation
story: with per-attempt `retry-after` and `remaining`, the proxy can back off
against the provider's own stated wait instead of jittered exponential backoff
(`retry.go:107-118` already parses `Retry-After` for the delay — it just throws
the header away afterwards), and a session can be told it is approaching a
lockout before it hits one.

**Shape.** Have `retryTransport` stash each drained response's allowlisted
headers on the `retryCounter` already travelling in the request context
(`retry.go:37-47`) — it is the existing per-request carrier, needs no new
plumbing, and `quotaFrom` is already a pure function over `http.Header`. Add
`Attempts []map[string]string` to the record beside `Retries` as an **optional**
field, so `SchemaVersion` need not move (the precedent is `Refusal`,
`record.go:48-50`).

**Test that makes it fail first.** Drive `flakyUpstream` with
`script: []string{"429"}`, `afters: []string{"1"}` and rate-limit headers on the
429, followed by a clean 200 carrying different values. Assert the ledger record
carries **both** the 429's `retry-after` and its
`anthropic-ratelimit-tokens-remaining` **and** the 200's. Today that test fails
on the first assertion — the 429's headers are not in the record at all. It is
the same shape as `TestQ6_QuotaReachesTheLedger` (`quota_test.go:159`), which is
the ADR-0014 pattern this repo already uses: drive a real request through the
proxy and read the file that results, so a pure function nothing calls cannot
pass for wiring.

**Second, cheaper, same lane:** give `requestFromRecord` a `Quota` field to copy
into (`transcript.Request`, `types.go:122`) and a test that a record with quota
survives `ReadFile`. Without it, fixing the capture only fills a file nothing
reads.
