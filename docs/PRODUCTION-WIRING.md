# Production wiring: which vendor surfaces Replay actually reads

**2026-09-29.** The authoritative support matrix. It answers one question per
row, and the question is not "does Replay support this vendor".

> **The unit of support is vendor to product to surface to version or protocol
> where observable.** A vendor is never wired. A surface is.

**Twelve surfaces across eight vendors, plus one first-party artifact. Six are
production-wired.** Claiming a vendor as "supported" would be false for xAI,
which is wired on its local store and not on its wire, and misleading for the
OpenAI-compatible family, where the wire is proven and the client half is not.

## The gate

A surface is **PRODUCTION-WIRED** only when a supported real-world artifact
from that specific surface traverses Replay's normal production path into
canonical state, with provenance-preserving normalization, explicit
observed/derived/inferred/unknown semantics, durable persistence,
production-visible reporting, deterministic failure handling and automated
regression coverage, **without developer-only code paths or fabricated
telemetry**. Twelve gates, all required. One failure and the surface stays
below the line.

Automatic discovery is **not** a gate. A surface may be production-wired with
explicit or configured selection. Discovery mode is recorded separately so the
limitation is visible rather than hidden.

## Maturity

`DISCOVERED` to `PARSED` to `NORMALIZED` to `PRODUCTION-WIRED` to `VERIFIED`.
These do not collapse. **A parser passing a fixture is not production wiring.**
`VERIFIED` additionally requires sustained real-world validation beyond the
gate and no surface here claims it.

---

## The matrix

Twelve surfaces across eight vendors, plus one first-party artifact. **Six are
production-wired. Six are not**, each with the gate that blocks it and the
evidence that would unblock it.

| Vendor | Product | Surface | Discovery | Maturity | Evidence | Canonical state | Unknowns | Tests | Production status | Blocking gate |
|---|---|---|---|---|---|---|---|---|---|---|
| Anthropic | Claude Code | `~/.claude/projects/**/*.jsonl` transcripts (`$CLAUDE_CONFIG_DIR` honoured). Burn name `claude-code` | **automatic** | PRODUCTION-WIRED | prompt, cache read, cache write with 5m/1h TTL split, output, thinking, tools, model, timestamps | `transcript.Session` via `usage.FromAnthropic`, exclusive counting | none material | parser, burn, conservation, reconcile, schema-mismatch suites | **PRODUCTION-WIRED** | none |
| Anthropic | any client on the proxy | `anthropic:/v1/messages` via `replay serve` | explicit | PRODUCTION-WIRED | same fields, off the wire | `ledger.Record` to `~/.replay/ledger/<session>.jsonl` | none material | `testdata/anthropic` live fixtures, `provider_conformance_test.go` | **PRODUCTION-WIRED** | none |
| OpenAI | Codex CLI | `~/.codex/sessions` and `archived_sessions` rollout JSONL. Burn name `codex` | **automatic** | PRODUCTION-WIRED | per-turn usage, cached reads, rate-limit events, compaction rebases | `transcript.CodexSession`, inclusive counting | cache **write** is `UNKNOWN` where reads are reported and the write counter is absent: `cachemodel.CountersWriteMissing` names that state rather than reading it as zero | `codex_test.go` x2, `codexdata` fixtures incl. `absent-breakdown`, `break`, `compacted`, `impossible`, `burncodexprice_test.go` | **PRODUCTION-WIRED** | none |
| Ollama | Ollama server | `~/.ollama/logs/server*.log`. Burn name `ollama` | **automatic** | PRODUCTION-WIRED | prompt sizes, eval durations, `n_past` context reuse | request counts and token totals; local, so no billing record | cached **share** is deliberately never reported: `n_past` appears only when the whole prompt was cached, so a share over those requests measures a population selected for having been cached | `ollama_test.go`, `ollamadata/server.log`, `burnollamashare_test.go`, `adviseollama_test.go` | **PRODUCTION-WIRED** | none |
| DeepSeek | DeepSeek CLI | `openai:/v1/chat/completions` via `replay serve` | explicit | PRODUCTION-WIRED | `prompt_tokens` (inclusive), `prompt_tokens_details.cached_tokens`, `prompt_cache_hit_tokens`, `prompt_cache_miss_tokens`, `completion_tokens_details.reasoning_tokens`, raw usage verbatim | `ledger.Record` via `usage.FromInclusive`, converted not copied | cache **write** is `NOT_APPLICABLE` on a sourced contract | `testdata/deepseek` live fixtures captured 2026-09-05, conformance, model matrix, raw-usage, stream e2e | **PRODUCTION-WIRED** | none |
| xAI | Grok CLI | `~/.grok/sessions/*/updates.jsonl` + `usage.json`. Burn name `grok` | **automatic** | PRODUCTION-WIRED | `inputTokens` (inclusive), `cachedReadTokens`, `cacheCreationTokens`, `outputTokens`, `reasoningTokens`, `modelCalls`, `turnCount`, per-model breakdown, vendor ledger | `usage.Record` via `usage.FromInclusive`; reconciled against the vendor ledger and never merged | cache **write** `UNKNOWN` (`XAIContract` is `ContractUnknown`); **cost** `UNKNOWN` | GK1-GK14, GW1-GW6, PW1-PW2 | **PRODUCTION-WIRED** | none |
| xAI | Grok CLI | `openai:/responses` at `cli-chat-proxy.grok.com` via `replay serve` | explicit | **DISCOVERED** | request-body **key names only**. No response body, no usage values | none | everything | capture notes only, no fixture | **NOT PRODUCTION-WIRED** | **INGESTION** |
| Anysphere | Cursor | `state.vscdb` + `~/.cursor/projects/*/agent-transcripts/*.jsonl` | automatic (detected, not read) | **REFUSED** | `tokenCount` present and **zero on every row**; transcripts carry no usage key at all | none possible | all of it | refusal re-verified 2026-09-29 | **NOT PRODUCTION-WIRED** | **INGESTION** |
| Mintplex | AnythingLLM | `anythingllm.db` (SQLite), `workspace_chats.metrics` | automatic (detected, not read) | **DISCOVERED** | 46 rows, all one provider `pd` and one model `qwen3-vl:4b-instruct`, in a single 6h48m window. `prompt_tokens`, `completion_tokens`, `total_tokens`, `provider`, `model`. **No cache field of any kind** | none | cache read and write entirely; and no price row exists for that model | `anythingllmsurface_test.go` | **NOT PRODUCTION-WIRED** | **DEPENDENCY.** Reading it needs SQLite and `TestX402_NoSigningCapability` forbids any dependency |
| OpenRouter (reselling Anthropic) | OpenClaw | `~/.openclaw/agents/*/sessions/*.jsonl` | automatic (detected, not read) | **DISCOVERED** | 451 usage blocks, 258 carrying any usage; 116 non-zero `cacheRead` summing 5,239,589 tokens; **0 non-zero `cacheWrite`**; conservation closes exactly at 13,230,206 so it counts **exclusively** like Anthropic | none | whether the zero write is real or dropped. The deciding vendor is **OpenRouter**, not Anthropic | `openclawsurface_test.go`, `probeunits_test.go` | **NOT PRODUCTION-WIRED** | **INGESTION** (no adapter), and the contract input is a reseller's page rather than an invoice |
| Oracle | Oracle agent | `meta.json` | automatic (detected, not read) | **DISCOVERED** | 3 sessions, 2 errored with no usage key; the third reports `totalTokens` 6 against `inputTokens` 4256. `promptPreview` and `options.prompt` hold the user's message text verbatim | none | cache entirely, and the totals contradict themselves | registry entry with the measurement | **NOT PRODUCTION-WIRED, and no `meta.json` reader should ever be built** | **PRIVACY**, plus the source fails the conservation identity `usage.Record.Validate()` already enforces |
| (any) | any OpenAI-compatible CLI | `openai:/v1/chat/completions`, **client half** | explicit | **STUB** | the wire is proven by the DeepSeek row. **No non-DeepSeek client has ever been captured** | n/a, the wire row carries it | what an unseen client sends | none for the client half | **NOT A SURFACE CLAIM** | see note |
| Replay | Replay itself | Jev capture stream (`replay jev <file>`) | **manual** (path argument) | PRODUCTION-WIRED for its own purpose | Replay's own evaluation records, contract-versioned | Jev evaluations | n/a | `jev_test.go`, `jev_contract_test.go`, `jevrefusal_test.go`, `jevdata/valid.jsonl` | **not a vendor surface** | n/a |

**The generic OpenAI-compatible row is not a surface, it is a generalisation.**
The wire is production-wired and the DeepSeek row holds the live bytes that
prove it. What is unproven is that an arbitrary unseen client sends a
conforming body. Replay reads the wire, not the client, so a new conforming
client needs no code; a non-conforming one would be forwarded and warned about
like any other unknown shape. The row exists so nobody reads the DeepSeek
capture as a claim about Cursor.

---

## Evidence semantics, per surface

`OBSERVED` is a value present in the artifact. `DERIVED` is arithmetic over
observed values and a documented table. `INFERRED` is a model-based
interpretation. `UNKNOWN` is absent and must never be read as zero.
`NOT_APPLICABLE` is a field the provider's contract says cannot exist.

| Field | Claude Code transcripts | DeepSeek `/v1/chat/completions` | Grok local store |
|---|---|---|---|
| prompt tokens | OBSERVED, exclusive of cache | OBSERVED, **inclusive** | OBSERVED, **inclusive** |
| fresh tokens | OBSERVED | **DERIVED** by subtraction, `Validate` refuses a record that does not add up | **DERIVED** by subtraction, same check |
| cache read | OBSERVED | OBSERVED | OBSERVED |
| cache write | OBSERVED, with TTL split | **NOT_APPLICABLE**, sourced contract | **UNKNOWN**, contract not found |
| output | OBSERVED | OBSERVED | OBSERVED |
| reasoning | OBSERVED | OBSERVED where reported. **ASSUMED zero where `completion_tokens_details` is absent**, which it is on every non-reasoner call. See the gap note below | OBSERVED |
| cost | DERIVED from a dated rate table | DERIVED from a dated rate table | **UNKNOWN**: no rate table, and the vendor's own tick scale is unreconciled against any invoice |
| vendor reconciliation | n/a | n/a | OBSERVED as `MATCH` / `DIFFERS` / `UNAVAILABLE`. UNAVAILABLE is a third value, not zero |
| surface kind | local transcript store | wire, `/v1/chat/completions` | local session store, **not a wire**. Grok's wire is `/responses` at `cli-chat-proxy.grok.com` and is a separate, unwired row |

**Gap, stated rather than smoothed.** DeepSeek's reasoning figure is the one
field in this table whose class is ASSUMED. The parser maps an absent
`completion_tokens_details` to zero, and for this field that is very probably
the true value, because a model that did not reason produced no reasoning
tokens. It is still an assumption. It costs nothing either way, because
reasoning is a **share of** `completion_tokens`, which is observed and billed
on its own, so no figure a reader acts on moves. The same shape on a cache
write would be the Codex defect and would be money. Pinned by
`TestDS_AbsentReasoningReadsAsZeroAndThatIsAnAssumption`, which fails if the
absent case ever starts carrying a fabricated figure.

**The one rule behind the whole column.** A provider that does not report a
field has not reported zero. `XAIContract` is `ContractUnknown` because Grok's
`cacheCreationTokens` is zero on every record observed, and a column of zeros
is observationally identical to a client dropping a counter the provider bills
for. That is the Codex defect, and it is why the contracts table takes the
provider's published pricing as a **second, independent input** rather than
reading the corpus twice.

---

## Gate results

### Anthropic / Claude Code / `~/.claude/projects/**/*.jsonl`

```text
IDENTITY                PASS   vendor, product and store path identified; JSONL schema versioned by the records themselves
PRODUCTION REACHABILITY PASS   bare `replay`, `replay burn`, `replay advise`, `replay cost`
INGESTION               PASS   transcript.ParseClaudeCodeFile over automatically discovered paths
CANONICALIZATION        PASS   transcript.Session, usage.FromAnthropic
PROVENANCE              PASS   every record traces to its file, line and request id
SEMANTICS               PASS   exclusive counting preserved; TTL split carried
PERSISTENCE             PASS   the transcript is the durable artifact and is never modified
REPORTING               PASS   burnClaudeCode, analysis.AnalyzeLane, advise, cost
FAILURE HANDLING        PASS   an unparsable file is counted nowhere rather than counted as zero
SCHEMA/DRIFT            PASS   unknown record kinds are skipped, not coerced
AUTOMATED TESTS         PASS   parser, burn, conservation, reconcile, schema-mismatch suites
SECURITY/PRIVACY        PASS   read-only; message text never copied into Replay state
FINAL: PRODUCTION-WIRED
```

### DeepSeek / CLI and OpenAI-compatible clients / `openai:/v1/chat/completions`

```text
IDENTITY                PASS   vendor, wire and endpoint identified
PRODUCTION REACHABILITY PASS   `replay serve`, the shipped proxy; no developer-only route
INGESTION               PASS   real bytes captured from api.deepseek.com 2026-09-05
CANONICALIZATION        PASS   ledger.Record via usage.FromInclusive
PROVENANCE              PASS   RawUsage stores the provider's usage object verbatim beside the parsed one
SEMANTICS               PASS   inclusive-to-exclusive conversion is a subtraction Validate refuses if it does not add up; cache write NOT_APPLICABLE on a sourced contract; reasoning ASSUMED zero when absent, classified and pinned, and it moves no billed figure
PERSISTENCE             PASS   ~/.replay/ledger/<session>.jsonl, 0600
REPORTING               PASS   ledger.IsLedgerFile -> ReadFile -> Session -> analysis, advisor and cost
FAILURE HANDLING        PASS   an all-zero usage object is treated as absent, not as a free request; 400 and 401 fixtures covered
SCHEMA/DRIFT            PASS   a field this build does not declare survives in RawUsage instead of being dropped
AUTOMATED TESTS         PASS   live fixtures, conformance invariants, model matrix, stream e2e through the server
SECURITY/PRIVACY        PASS   message text is never written; the proxy prints EXPERIMENTAL, UNMASKED once per path
FINAL: PRODUCTION-WIRED
```

Caveat carried, not hidden: the **client** half of this wire is STUB. No
generic OpenAI-compatible CLI has been pointed at `replay serve` and captured.

### xAI / Grok CLI / `~/.grok/sessions` local store

```text
IDENTITY                PASS   vendor, product, store layout; Grok 1.0.41 guide names the tick scale
PRODUCTION REACHABILITY PASS   `replay grok`, and since this pass `replay burn`; `replay doctor` discovers the store
INGESTION               PASS   real session artifacts; fixtures carry the authentic params.update.usage envelope
CANONICALIZATION        PASS   usage.Record via usage.FromInclusive, per turn
PROVENANCE              PASS   reconstruction and the vendor ledger are reported side by side and never summed
SEMANTICS               PASS   cache write UNKNOWN not zero (GK7); absent ledger UNAVAILABLE not zero (GK5); dollars refused (GK6)
PERSISTENCE             PASS   the session store is the durable artifact and is read, never written
REPORTING               PASS   burnGrok, added by this pass. Previously this gate FAILED
FAILURE HANDLING        PASS   a malformed line does not discard the good ones (GK8); an inconsistent turn is excluded and counted (GK12); an absent root reads as empty (GK9)
SCHEMA/DRIFT            PASS   costUsdTicks is deliberately not decoded into a typed field, pinned by GK14
AUTOMATED TESTS         PASS   GK1 to GK14, GW1 to GW6, PW1 to PW2; BG tests mutation-proven
SECURITY/PRIVACY        PASS   read-only; no message text enters Replay state
FINAL: PRODUCTION-WIRED
```

### OpenAI / Codex CLI / `~/.codex/sessions` and `archived_sessions`

```text
IDENTITY                PASS   vendor, product, both roots. `codex archive` moves a session between them without changing its format, and both are scanned
PRODUCTION REACHABILITY PASS   `replay codex`, `replay burn`, `replay doctor`
INGESTION               PASS   real rollout JSONL; fixtures cover break, compaction, absent breakdown and impossible counters
CANONICALIZATION        PASS   transcript.CodexSession, inclusive counting
PROVENANCE              PASS   per-turn records trace to file and turn index
SEMANTICS               PASS   cachemodel.ClassifyCounters names CountersWriteMissing when reads are reported beside an absent write, rather than reading the absence as zero
PERSISTENCE             PASS   the rollout is the durable artifact and is never modified
REPORTING               PASS   burnCodex, and a live quota reading, the only surface that has one
FAILURE HANDLING        PASS   an unparsable rollout is counted nowhere; a compaction rebase is reported rather than summed through
SCHEMA/DRIFT            PASS   an impossible counter combination is classified, not coerced
AUTOMATED TESTS         PASS   12 tests across transcript and cmd, plus burn pricing
SECURITY/PRIVACY        PASS   read-only, no message text copied
FINAL: PRODUCTION-WIRED
```

### Ollama / Ollama server / `~/.ollama/logs/server*.log`

```text
IDENTITY                PASS   product and log glob; `server*.log` matches the reader, `app*.log` carries no usage and is excluded
PRODUCTION REACHABILITY PASS   `replay burn`, `replay advise`, `replay doctor`
INGESTION               PASS   real server logs; ollamadata/server.log fixture
CANONICALIZATION        PASS   per-request counts with the reused prefix excluded, which is this surface's own counting convention
PROVENANCE              PASS   each request traces to its log and block
SEMANTICS               PASS   a block without an n_past line has an UNKNOWN prefix, not a zero one, and the cached share is never reported at all because the population that carries n_past is selected for having been cached
PERSISTENCE             PASS   the log is the durable artifact and is never modified
REPORTING               PASS   burnOllama, marked localOnly because nobody invoices for a local model
FAILURE HANDLING        PASS   an unreadable log is skipped; a non-Ollama file parses to nothing and is counted nowhere
SCHEMA/DRIFT            PASS   counted by parsing rather than by filename, so a renamed or foreign file cannot inflate the count
AUTOMATED TESTS         PASS   parser, burn share suppression, advise
SECURITY/PRIVACY        PASS   read-only, local only, no network
FINAL: PRODUCTION-WIRED
```

### xAI / Grok CLI / `openai:/responses` via the proxy

```text
IDENTITY                PASS   endpoint captured off the wire
PRODUCTION REACHABILITY PASS   traffic does reach `replay serve`
INGESTION               FAIL   /responses is not parsed. The proxy forwards it unchanged and warns
CANONICALIZATION        FAIL   no canonical record is produced
PROVENANCE              FAIL   nothing is stored to trace
SEMANTICS               FAIL   no fields are classified because none are read
PERSISTENCE             FAIL   no ledger record
REPORTING               FAIL   absent from every report
FAILURE HANDLING        PASS   forwarding unchanged and warning once per path is deterministic and honest
SCHEMA/DRIFT            PASS   nothing is fabricated, which is the correct behaviour for an unparsed surface
AUTOMATED TESTS         FAIL   no fixture exists
SECURITY/PRIVACY        PASS   forwarded, not stored; masking does not cover this path and the proxy says so
FINAL: NOT PRODUCTION-WIRED. Maturity DISCOVERED.
```

Reason: **the surface is not parsed, and the repository does not contain the
evidence needed to parse it.** This was checked rather than assumed during this
pass.

What the repository holds is a **shape-only** capture:
`wire-families-2026-09-06.md` records that the method was "keys and types
recorded, values never", and states in its own scope paragraph that **"no cache
hit was observed, no usage was parsed out of the SSE stream"**. The request
body's key set is known. **No response body was retained, and no usage object
from this path exists anywhere in the repository.**

**Exact missing evidence**, all three required:

1. A real `/responses` **response**, specifically the SSE event carrying usage.
   The transport is `stream: true` on every call, so usage arrives in stream
   events and a non-streamed body will not do.
2. The **field names and nesting** of that usage object. It cannot be assumed
   to match `/v1/chat/completions`: the request body already differs, carrying
   `input`, `prompt_cache_key` and `max_output_tokens` where chat completions
   carry `messages` and `max_tokens`.
3. Whether a cache **write** is reported at all on this path, which decides
   between `UNKNOWN` and `NOT_APPLICABLE` for that field.

Obtaining it needs a live authenticated session against
`cli-chat-proxy.grok.com`. **No attempt was made to fabricate it**, and
inventing the schema from the chat-completions family is the specific
assumption this project already made once and had to retract.

The quota headers are not a route around this: no live quota state was found on
any endpoint, `/settings` included, and the `x-ratelimit-*` headers did not move
across 8 model calls and roughly 940KB of responses.

**xAI's other surface is unaffected.** `~/.grok/sessions` is production-wired
and carries the usage this path does not expose.

### Anysphere / Cursor / `state.vscdb` and agent transcripts

```text
IDENTITY                PASS   both stores located and enumerated
PRODUCTION REACHABILITY PASS   detected by knownSurfaces and named in the empty-state report
INGESTION               FAIL   there is no usage evidence to ingest
CANONICALIZATION        FAIL   nothing to canonicalize
PROVENANCE              FAIL   nothing to trace
SEMANTICS               FAIL   no field to classify
PERSISTENCE             FAIL   no record
REPORTING               FAIL   absent from every report; the registry says why instead
FAILURE HANDLING        PASS   the refusal is explicit and carries its measurement
SCHEMA/DRIFT            PASS   nothing is fabricated
AUTOMATED TESTS         PASS   for the refusal, not for a reader
SECURITY/PRIVACY        PASS   detected by path, never opened for content
FINAL: NOT PRODUCTION-WIRED. Maturity REFUSED.
```

**Refusal re-verified 2026-09-29 on the live store, not carried forward on
trust.** The agent transcripts were re-enumerated today: **118 files, 4,838
rows, 4,566 of them message rows**, and the complete top-level key set across
every one of them is `role`, `message`, `type`, `status`, `error`. Those
numbers reproduce the 2026-09-12 measurement exactly. A deep scan for any key
matching token, usage, cache or cost found **one** occurrence at any depth, and
it sits inside message content rather than instrumentation, which is the same
pasted-API-response artefact the AnythingLLM sweep recorded.

The two halves fail differently and both are load-bearing. In `state.vscdb` the
counter **exists and is zero**, so Cursor did not record it and no parser
recovers it. In the transcripts there is **no counter at all**. A reader told
only the first would reasonably go looking in the second.

**What would unblock it:** Cursor recording a non-zero `tokenCount`, or any
usage key appearing in the transcripts. Nothing Replay can write changes this.

---

## Truth statement

> **Replay currently has 6 production-wired surfaces across 5 vendors.**

1. Anthropic, Claude Code, `~/.claude/projects/**/*.jsonl` transcripts
2. Anthropic, any client, `anthropic:/v1/messages` via `replay serve`
3. OpenAI, Codex CLI, `~/.codex/sessions` and `archived_sessions`
4. Ollama, Ollama server, `~/.ollama/logs/server*.log`
5. DeepSeek, DeepSeek CLI, `openai:/v1/chat/completions` via `replay serve`
6. xAI, Grok CLI, `~/.grok/sessions` local store. **This is the local store,
   not a wire.** Grok's wire is `/responses` at `cli-chat-proxy.grok.com` and
   it is not wired; see the table below

> **The following surfaces remain below PRODUCTION-WIRED:**

| Surface | Maturity | Blocking gate | What would unblock it |
|---|---|---|---|
| xAI, Grok CLI, `openai:/responses` | DISCOVERED | INGESTION | A real SSE usage event from a live authenticated session. The capture is shape-only and no response body exists |
| Anysphere, Cursor, `state.vscdb` and agent transcripts | REFUSED | INGESTION | Cursor recording a non-zero `tokenCount`, or any usage key in the transcripts. Re-verified today; nothing Replay can write changes it |
| Mintplex, AnythingLLM, `anythingllm.db` | DISCOVERED | **DEPENDENCY** | A way to read SQLite without a dependency, which `TestX402_NoSigningCapability` forbids outright. Even then: no cache field exists, and all 46 rows are one local 4B model with no price row, so a reader would print "46 requests, no price" |
| OpenClaw, session log | DISCOVERED | INGESTION | A sourced contract fact for **OpenRouter**, which the measurement shows is the biller of record rather than Anthropic. That is the Codex question and it needs a vendor pricing document, not more corpus. A reseller's published price page is the only source found, and whether that counts as an N1a input is Daniel's call |
| Oracle, `meta.json` | DISCOVERED | PRIVACY, and the source contradicts itself | Nothing. **Do not build this reader.** Oracle's `options.baseUrl` is `http://127.0.0.1:18999/v1` and its mode is `api`, so it is an OpenAI-compatible client already reachable by the PRODUCTION-WIRED proxy intake with no adapter at all. Capturing it there would also close the STUB row, and it needs a live call, so it needs spend authorisation |
| any OpenAI-compatible CLI, client half | STUB | not a surface | A capture from a non-DeepSeek client. The **wire** is already wired |

**This is not 100% coverage and the repository does not support claiming it.**
Four of the six unwired surfaces are unwired because the source does not carry
the evidence, which is not a Replay defect and cannot be closed by writing
code.

## What is deliberately not claimed

- **No vendor is production-wired.** Six surfaces, four wired, two not.
- **`VERIFIED` is claimed nowhere.** No surface here has sustained real-world
  validation beyond the gate.
- **Grok carries no dollar figure**, on any surface, in any report. The tick
  scale in Grok's own records is stated by its guide at 10^10 ticks per USD and
  has never been reconciled against a statement of account, and no xAI rate
  table is installed. The surface is UNPRICED, which is a different cell from
  the local-only one Ollama occupies: xAI does bill for this.
- **Nothing here rests on the research corpus.** The frozen research in
  `docs/research/` is evidence about what was measured under experimental
  conditions. It is not a production source and confers no production claim.
  In particular the DeepSeek cache work establishes **no** production cache
  mechanism, and no production telemetry was derived from it.

## Regression guards

Two, both mutation-proven, so this document cannot drift from the binary:

- `TestPW1` pins the roster of surfaces reaching the cross-surface report.
- `TestPW2` requires every surface in that report to declare its token unit and
  its quota standing, so no column reads as addable or as "nothing used" when
  it means "cannot answer".
- `TestWM1` requires every surface `replay burn` reports to carry a
  PRODUCTION-WIRED row here, matched on its burn name. The first version of
  this guard scanned the whole document and did **not** fail when a row was
  deleted to test it, because the name still appeared in prose; it now scans
  verdict rows only.
- `TestWM2` fails if a surface `knownSurfaces` refuses with a measured reason
  appears on a PRODUCTION-WIRED row. This is the guard for a **stale refusal**,
  which is the failure that actually happened: the 2026-09-06 evidence recorded
  Grok's local store as carrying no token counts, and 130 files on this machine
  now do.
- `TestWM3` requires every vendor row to reach a verdict.
