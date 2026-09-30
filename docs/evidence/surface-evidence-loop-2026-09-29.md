# Closing the evidence loop on three unwired surfaces

**2026-09-29.** Read-only. No adapter was written, no API call was made, no
money was spent, and no surface changed verdict. This file records what the
artifacts on one machine do and do not establish, so the next person does not
re-derive it or, worse, over-read it.

**Scope: n=1 machine.** Every count below is from this machine's stores. None
of it generalises without a second corpus, and where a figure is one afternoon
of one model it says so.

---

## 1. OpenClaw

### 1.1 The conservation result, traced end to end

Corpus: `~/.openclaw/agents/main/sessions`, 3 files, of which one is a
soft-deleted `*.jsonl.deleted.2026-02-17T18-03-57.399Z` and one is
`sessions.json` carrying no usage.

**The corpus boundary changes the counts, so both are stated.**

| | all files | `*.jsonl` only, which is what `surface.Probe` reads |
|---|---:|---:|
| usage blocks | **451** | **447** |
| non-zero `cacheRead` | **116** | **113** |
| non-zero `cacheWrite` | **0** | **0** |
| all-zero blocks | **218** | **218** |

The 4-block difference is entirely the soft-deleted file. `Probe` filters on a
`.jsonl` suffix, so it skips that file **silently**, and nobody decided that.
It is recorded here rather than left to be rediscovered.

**The 218 all-zero blocks decompose identically on both corpora**, because all
of them are in the live file:

| | blocks |
|---|---:|
| `openclaw` / `delivery-mirror`, an internal row | **193** |
| `openrouter` / `anthropic/claude-3.5-haiku`, real blocks carrying no usage | **25** |
| **total all-zero** | **218** |

An all-zero usage object is **ABSENT, not a free request**. That is not a new
rule; it is the rule the DeepSeek surface already holds in
`internal/ledger/openai.go`, where `"usage": {}` is treated as the absence of a
measurement rather than a measurement of nothing.

**The identity, checked per block and not merely in aggregate:**

```
input 7,895,540 + output 95,077 + cacheRead 5,239,589 = 13,230,206
sum(totalTokens)                                      = 13,230,206
blocks whose own parts disagree with their own totalTokens: 0 of 451
```

### 1.2 What this establishes

**OBSERVED, artifact fact.** `cacheRead` is **disjoint** from `input`. If reads
were nested inside the input count, the left side would exceed `totalTokens` by
5,239,589; it does not, on any block. So OpenClaw counts **reads
exclusively**, in the Anthropic family, and **not** inclusively as Codex and
Grok do. An adapter, if one is ever written, must construct with the exclusive
shape and must not call `usage.FromInclusive`.

### 1.3 What this does NOT establish, stated because the identity invites the error

**The counting convention is established for READS ONLY.** `cacheWrite` is zero
on every one of the 451 blocks, so it is **not a term in the sum**. The identity
therefore cannot separate two incompatible worlds:

- a provider that performed no billable cache write, and
- a client that dropped a counter the provider billed for.

This is exactly the Codex case that `internal/surface/contracts.go` exists to
stop anyone resolving from a corpus. **Nothing in these bytes decides it.**

**Who sets the applicable price is not established either.** The artifact names
`openrouter` as the provider on all 258 blocks that carry any usage, and
`anthropic/claude-3.5-haiku` as the model. That is an **OBSERVED artifact
fact**: a string in a log. Every step from there to a bill is **INFERRED**:

| Step | Class |
|---|---|
| the log names provider `openrouter` | **OBSERVED** |
| the log carries a `cost` object | **OBSERVED** |
| that `cost` object reproduces a published rate card | **DERIVED**, by arithmetic over the log's own numbers |
| therefore OpenRouter's rate card applies to this account | **INFERRED**, not established |
| therefore OpenRouter invoiced these amounts | **NOT ESTABLISHED.** No invoice was seen |
| therefore OpenRouter charges a cache-write premium | **NOT ESTABLISHED.** No authoritative source was accepted |

**The `cost` object is DERIVED and must never be a reconciliation target.** It
is the client's own arithmetic over a rate table, so it is a second
reconstruction by the same procedure as Replay's. Agreement between the two
would prove that both read the same rate card, not that either matches a bill.
This is the opposite of Grok's `usage.json`, which is the vendor's own ledger
and is therefore a legitimate thing to reconcile against.

**No contract row was added.** A bake-off panel proposed an
`OpenRouterResoldAnthropicContract` asserting `ContractPricedDistinctly` on the
strength of a reseller's published documentation page. It was declined. The
page is a marketing surface rather than a billing contract, the fetched render
carried unresolved template variables, and the corroboration offered was
circular in the direction that matters: the read multiple agreeing between the
page and the log proves the client reads that rate card, which is a fact about
the client's code and not about an invoice. **The write premium is precisely
the number absent from the corpus, so the corpus cannot corroborate it.**

`ContractFact.Validate` refuses an unsourced assertion, which is the right
guard, and the correct state here is to not make the assertion at all.

**What would establish it:** an OpenRouter invoice or billing export for this
account covering a period in this corpus, or an authoritative pricing document
accepted as an N1a input with its date and its limits recorded. Until then the
class stays `ClassUndetermined` and this surface stays BLOCKED-EVIDENCE.

---

## 2. Oracle

### 2.1 Exactly which fields carry message text

Corpus: `~/.oracle/sessions`, 3 sessions, each with a `meta.json`.

| Field path | Type | Measured content length |
|---|---|---|
| `/promptPreview` | string | 6, 6 and 61 characters |
| `/options/prompt` | string | 6, 6 and 61 characters |

Both hold **the user's prompt verbatim**, not a hash, not a length, not a
redaction. They were measured by length and path; **the content was not read
into this document and is not reproduced anywhere**.

### 2.2 Why reading `meta.json` would violate the standing privacy contract

Replay's ledger promise, recorded in `docs/SURFACES.md` and verified against a
request stuffed with secrets, is that **message text is never written into
Replay state**. The proxy path keeps that promise by never holding the thing:
`transcript.OpenAIRequest` decodes messages only as far as the guards need and
its `Blocks` carry kind, size and tool name with `Text` deliberately never set.

A `meta.json` reader breaks that at the first line. The file cannot be opened
and parsed for `inputTokens` without the prompt passing through the process,
and any error path, log line, panic dump or future field-dump walks over it. A
reader would be walking past user content on **every run**, for a corpus of
three smoke pokes, two of which errored.

**Therefore: no `meta.json` reader should be built.** This is not a cost or
effort judgement and it does not become acceptable if the corpus grows.

Independently disqualifying, so the privacy verdict does not rest alone: the one
session carrying usage reports `inputTokens` 4256, `outputTokens` 0,
`reasoningTokens` 0 and **`totalTokens` 6**. That fails the class of
conservation identity `usage.Record.Validate()` already enforces, so the
existing normaliser would reject the record without anyone writing a new rule.

### 2.3 Reachability, recorded separately, and it is not authorization

All three sessions record `options.mode` = `api` and
`options.baseUrl` = `http://127.0.0.1:18999/v1`.

**The artifact fact:** Oracle is an OpenAI-compatible client with a
configurable base URL, already pointed at a loopback endpoint. The
`openai:/v1/chat/completions` intake in `replay serve` is PRODUCTION-WIRED, so
Oracle's traffic is **reachable by an intake that already exists**, with no
adapter and no new code.

**What that does not mean.** Reachability is a property of the topology. It is
not permission to send anything, not permission to interrogate the endpoint,
and not permission to make a live call to capture a fixture. Capturing Oracle
through the proxy would be a live call against a paid model and **is not
authorized**. It is recorded here as an option with its cost named, and it
stays unexercised.

Were it ever authorized, the prize is larger than an adapter: it would close
the matrix's STUB row, which states that no non-DeepSeek OpenAI-compatible
client has ever been captured.

---

## 3. AnythingLLM

### 3.1 The dependency-free invariant, which is enforced and not aspirational

`cmd/replay/x402_test.go`, `TestX402_NoSigningCapability`:

- fails if `go.mod` contains the string `require`
- fails if `go.sum` exists at all

Verified on this tree: **0 `require` lines, no `go.sum`.** The module is
genuinely dependency-free today.

There is no SQLite in the Go standard library. So reading `anythingllm.db`
requires one of: a third-party driver, which fails the test outright; a
hand-written page-and-B-tree parser inside a tool whose promise is that it is
small and touches nothing; or shelling out to an external `sqlite3` binary,
which is a runtime dependency in a tool that promises to be self-contained.

**The blocking gate is DEPENDENCY, not INGESTION.** The evidence is present and
readable in principle; the repository's own enforced contract excludes the
means of reading it. That is a different sentence and it points at a different
decision-maker.

**No adapter was designed.** Spending design effort on a surface the enforced
contract excludes would be work that cannot land.

### 3.2 The verified live artifacts

`~/Library/Application Support/anythingllm-desktop/storage/anythingllm.db`,
749,568 bytes, opened read-only.

| | |
|---|---|
| `workspace_chats` rows | **46** |
| `response` keys, every row | `text, sources, type, attachments, metrics` |
| `metrics` keys, every row | `prompt_tokens, completion_tokens, total_tokens, outputTps, duration, model, provider, timestamp` |
| cache keys in `metrics` | **0** |
| distinct `provider` values | **1**, `pd` |
| distinct `model` values | **1**, `qwen3-vl:4b-instruct` |
| window | 2026-08-09T19:10:37Z to 2026-08-10T01:58:51Z, a single **6h 48m** span |
| `prompt_tokens` total | 242,566 |
| `completion_tokens` total | 18,647 |
| `.env` beside the database | **present**, 552 bytes |

Two consequences worth stating even though the surface is blocked:

- The one real advantage of this surface, a `provider` and `model` stamped on
  every row so the price table is never in doubt, **currently selects a table
  that does not exist**: no price row for `qwen3-vl:4b-instruct` exists in
  `internal/cachemodel`, and a 4B open-weights model is local inference.
- Any walk over that storage root walks a directory holding **provider
  credentials**. A reader would need to exclude it explicitly.

**What would change the picture:** a second AnythingLLM corpus, on a different
machine, where `provider` names a remote billed vendor. That is the only thing
that makes the routing advantage point at a real price table. It does not
address the dependency gate, which would still stand.

---

## 4. The probe fix, verified for scope

`FieldSpec.Exclude` was added at `089ec44` after a dollar figure was found being
counted as a token counter. This section verifies it fixes **only** that.

### 4.1 Measured against the real corpus, with and without

| | records | read obs | read non-zero | write obs | write non-zero | write field present | records with read |
|---|---:|---:|---:|---:|---:|---:|---:|
| **without** `Exclude` | 707 | 894 | 226 | 894 | 0 | true | 447 |
| **with** `Exclude: ["cost"]` | 707 | **447** | **113** | **447** | 0 | true | 447 |

Every observation count **halves exactly**, which is what must happen when each
counter appeared exactly twice per record, once in tokens and once in dollars.
Nothing else moves: the record count is unchanged, `WriteFieldPresent` stays
true, `WriteNonZero` stays 0, and `RecordsWithRead` was already 447 in both
because per-record presence was never the inflated quantity.

**Legitimate token fields remain observable.** After the fix the probe still
reports 447 read observations and 113 non-zero ones, which are the true
readings for the `.jsonl` corpus.

### 4.2 The RED test and the mutation evidence are kept

`internal/surface/probeunits_test.go`, `TestPU1`. It failed before the fix with
`ReadObservations = 2, want 1`. It pins **both halves**: with `Exclude` the
nested cost object is not counted, and **without** it the same record still
double counts, so the hazard is visible at the call site and a future default
change has to break that line on purpose.

Mutation: removing the loop that populates the skip set turns `TestPU1` red at
`ReadObservations = 2, want 1`.

`Exclude` is **opt-in by design**. Skipping every sub-object named `cost` on
every surface would be the normalisation this package refuses elsewhere: the
spelling is the evidence, and a surface genuinely reporting tokens under that
name would go silently unread.

---

## 5. Final evidence table

Statuses are the five permitted values and nothing else. No ranking is implied
and the order is the matrix's.

| Vendor | Surface | Status | Precise blocking reason |
|---|---|---|---|
| Anthropic | Claude Code transcripts | **SUPPORTED** | none |
| Anthropic | `/v1/messages` via `replay serve` | **SUPPORTED** | none |
| OpenAI | Codex rollouts | **SUPPORTED** | none |
| Ollama | `server*.log` | **SUPPORTED** | none |
| DeepSeek | `/v1/chat/completions` via `replay serve` | **SUPPORTED** | none |
| xAI | `~/.grok/sessions` local store | **SUPPORTED** | none |
| OpenRouter (reselling Anthropic) | OpenClaw session log | **BLOCKED-EVIDENCE** | `cacheWrite` is 0 on all 451 blocks and the identity has no write term, so the corpus cannot separate "no write billed" from "counter dropped". Resolving it needs an authoritative pricing source for **OpenRouter**, which is the biller of record and not Anthropic. A reseller's published page was found and declined as insufficient |
| Oracle | `meta.json` | **BLOCKED-PRIVACY** | `/promptPreview` and `/options/prompt` hold the user's prompt verbatim, and the ledger promise is that message text never enters Replay state. Independently, `totalTokens` 6 against `inputTokens` 4256 fails the conservation identity `usage.Record.Validate()` already enforces |
| Oracle | `http://127.0.0.1:18999/v1` via the wired proxy intake | **STUB-REACHABLE** | Reachable by an intake that is already PRODUCTION-WIRED, with no adapter. Capturing it is a live paid call and **is not authorized**. Reachability is not authorization |
| Mintplex | AnythingLLM `anythingllm.db` | **BLOCKED-DEPENDENCY** | Reading SQLite needs a dependency and `TestX402_NoSigningCapability` fails on one `require` line or on `go.sum` existing. Separately, no cache field exists and all 46 rows are one local 4B model with no price row |
| xAI | `openai:/responses` | **BLOCKED-EVIDENCE** | The only capture was shape-only, "keys and types recorded, values never", and no usage was ever parsed from the SSE stream. No response body exists |
| Anysphere | Cursor | **BLOCKED-EVIDENCE** | `tokenCount` present and zero on every row; transcripts carry no usage key. Re-verified 2026-09-29: 118 files, 4,838 rows, key set `role/message/type/status/error` |
| (any) | generic OpenAI-compatible **client** | **STUB-REACHABLE** | The wire is SUPPORTED. No non-DeepSeek client has been captured, so the generalisation to an arbitrary client is unproven |

**Six SUPPORTED. Two STUB-REACHABLE. One BLOCKED-PRIVACY. One
BLOCKED-DEPENDENCY. Three BLOCKED-EVIDENCE.**

---

## 6. What this pass did not produce

**No new evidence about any provider's billing.** Everything here is a
re-reading of artifacts already on disk, plus one scope verification of a fix
committed earlier the same day. No experiment was run and none was designed.

**No contract row, no adapter, no fixture, no verdict change.** Three surfaces
that were below the gate are still below it, with their blocking reasons now
stated precisely enough to act on or to disagree with.

**The frozen research corpus was not touched** and nothing here derives from
it.
