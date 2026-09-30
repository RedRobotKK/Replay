# Production wiring: which vendor surfaces Replay actually reads

**2026-09-29.** The authoritative support matrix. It answers one question per
row, and the question is not "does Replay support this vendor".

> **The unit of support is vendor to product to surface to version or protocol
> where observable.** A vendor is never wired. A surface is.

Three vendors appear here with **six** distinct surfaces between them. Two of
the six are production-wired against every gate, two are wired against the
reporting path but not the ledger, and two are below the line with the reason
stated. Claiming any of the three vendors as "supported" would be false for at
least one of its own surfaces.

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

| Vendor | Product | Surface | Discovery | Maturity | Evidence | Canonical state | Unknowns | Tests | Status |
|---|---|---|---|---|---|---|---|---|---|
| Anthropic | Claude Code | `~/.claude/projects/**/*.jsonl` transcripts (`$CLAUDE_CONFIG_DIR` honoured) | **automatic** | PRODUCTION-WIRED | prompt, cache read, cache write with 5m/1h TTL split, output, thinking, tools, model, timestamps | `transcript.Session` via `usage.FromAnthropic`, exclusive counting | none material on this surface | `claudecode_test.go`, `burn*_test.go`, conservation and reconcile suites | **PRODUCTION-WIRED** |
| Anthropic | any client on the proxy | `anthropic:/v1/messages` via `replay serve` | **explicit** (configure the proxy) | PRODUCTION-WIRED | same fields, off the wire | `ledger.Record` to `~/.replay/ledger/<session>.jsonl` | none material | `testdata/anthropic` live fixtures, `provider_conformance_test.go`, `openaistream_e2e_test.go` | **PRODUCTION-WIRED** |
| DeepSeek | DeepSeek CLI and any OpenAI-compatible client | `openai:/v1/chat/completions` via `replay serve` | **explicit** (configure the proxy) | PRODUCTION-WIRED | `prompt_tokens` (inclusive), `prompt_tokens_details.cached_tokens`, `prompt_cache_hit_tokens`, `prompt_cache_miss_tokens`, `completion_tokens_details.reasoning_tokens`, raw usage verbatim | `ledger.Record` via `usage.FromInclusive`, converted not copied | cache **write** is `NOT_APPLICABLE`: `DeepSeekContract` records a miss billing as ordinary input, sourced | `testdata/deepseek` live fixtures captured 2026-09-05, `provider_conformance_test.go`, `model_matrix_test.go`, `rawusage_test.go` | **PRODUCTION-WIRED** |
| xAI | Grok CLI | `~/.grok/sessions/*/updates.jsonl` + `usage.json` local store | **automatic** | PRODUCTION-WIRED | `inputTokens` (inclusive), `cachedReadTokens`, `cacheCreationTokens`, `outputTokens`, `reasoningTokens`, `modelCalls`, `turnCount`, per-model breakdown, vendor ledger | `usage.Record` via `usage.FromInclusive`; reconciled against the vendor ledger and never merged with it | cache **write** is `UNKNOWN`: `XAIContract` is deliberately `ContractUnknown`. Cost is `UNKNOWN`: the tick scale is unreconciled and no xAI rate table exists | `grok_test.go` (GK1 to GK14), `burngrok_test.go` (GW1 to GW6), `burnroster_test.go` | **PRODUCTION-WIRED** |
| xAI | Grok CLI | `openai:/responses` at `cli-chat-proxy.grok.com` via `replay serve` | explicit | **DISCOVERED** | none read. The proxy forwards and warns once per path | none | everything | `wire-families-2026-09-06.md` capture only | **NOT PRODUCTION-WIRED** |
| Anysphere | Cursor | `cursor:sqlite` transcript store | n/a | **REFUSED** | 29,665 message rows and **zero** cache fields | none possible | all cache forensics | `spike-cursor-2026-09-05.md` | **NOT PRODUCTION-WIRED** |

**Not a vendor claim.** Anthropic is wired on two surfaces of its own products
and says nothing about other Anthropic clients. xAI is wired on one surface and
**not** on the other. DeepSeek is wired on the one surface it has here, and the
generic OpenAI-compatible **client** half remains unproven: the DeepSeek
capture proves the parser against real provider bytes and proves nothing about
what an unseen client sends.

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

```
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

```
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

```
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

### xAI / Grok CLI / `openai:/responses` via the proxy

```
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

Reason: **the surface is not parsed at all.** To promote it, capture a
`/responses` payload from a live authenticated session, land it as a fixture,
and write the surface's own condition in `provider_conformance_test.go`. Note
that the quota headers are not a route to a live guard here: no live quota
state was found on any endpoint, `/settings` included.

### Anysphere / Cursor / `cursor:sqlite`

```
FINAL: NOT PRODUCTION-WIRED. Maturity REFUSED.
```

Reason: measured and found unmeasurable. 29,665 message rows carry **zero**
cache fields, so the transcript path cannot produce cache forensics at any
level of effort. This is a kill with a measurement behind it, recorded so the
week is not spent rediscovering it.

---

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

- `TestPW1` pins the roster of surfaces reaching the cross-surface report
  against the rows above. Adding or renaming a surface without updating this
  file fails.
- `TestPW2` requires every surface in that report to declare its token unit and
  its quota standing, so no column reads as addable or as "nothing used" when
  it means "cannot answer".
