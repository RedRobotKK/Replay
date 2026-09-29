# Provider evidence matrix

**What Replay can actually recover from each surface. Compiled 2026-09-29 from
readers in this repository and from measurements taken against local corpora.
This is the foundation for any future evidence ingestion.**

Three standings, never merged:

- **OBSERVED BY REPLAY** a field Replay reads from an artifact and can re-derive.
- **PROVIDER ASSERTION** a number the provider states that Replay cannot check.
- **NOT AVAILABLE** the field is absent, or present and never populated.

**A field existing on one surface says nothing about another.** That assumption
is the live bug class in this area, and it has cost this repository real
corrections.

## Per surface

| | Claude Code | Proxy ledger | Codex | Grok | Ollama | Cursor |
|---|---|---|---|---|---|---|
| Transcript | `~/.claude/projects/*/*.jsonl` | `~/.replay/ledger/*.jsonl` | `~/.codex/**/rollout-*.jsonl` | `~/.grok/sessions/*/*/updates.jsonl` | server log, text | `state.vscdb` + transcripts |
| Tool calls | OBSERVED | OBSERVED | OBSERVED | OBSERVED | NOT AVAILABLE | OBSERVED |
| Timestamps | OBSERVED | OBSERVED | OBSERVED | OBSERVED | OBSERVED | OBSERVED |
| Model identity | OBSERVED, **per request** | OBSERVED, per request | OBSERVED, **per session, last-wins** | OBSERVED, per turn and session | OBSERVED, per request | NOT ESTABLISHED |
| Counting convention | **EXCLUSIVE** | both, converted | **INCLUSIVE** | **INCLUSIVE** | neither; cached prefix excluded from total | n/a |
| Cache read | OBSERVED | OBSERVED | OBSERVED | OBSERVED | OBSERVED, measured-vs-unmeasured sentinel | NOT AVAILABLE |
| Cache write | OBSERVED, 99.9% populated | OBSERVED | **PRESENT, ALWAYS ZERO** | **PRESENT, ALWAYS ZERO** | NOT AVAILABLE | NOT AVAILABLE |
| Cache-miss cause | **OBSERVED, 6 classes** | OBSERVED | NOT AVAILABLE | NOT AVAILABLE | NOT AVAILABLE | NOT AVAILABLE |
| Cost | DERIVED from a dated table | DERIVED | DERIVED | **REFUSED** | localOnly, free | NOT AVAILABLE |
| Session id | OBSERVED | OBSERVED | OBSERVED | OBSERVED | **NOT AVAILABLE** | n/a |
| Raw provider payload | NOT AVAILABLE | **OBSERVED, verbatim** | NOT AVAILABLE | NOT AVAILABLE | NOT AVAILABLE | NOT AVAILABLE |
| **Task outcome** | **NOT AVAILABLE** | **NOT AVAILABLE** | **NOT AVAILABLE** | **NOT AVAILABLE** | **NOT AVAILABLE** | **NOT AVAILABLE** |

## Fields not in the table above, and why

| Field | Standing |
|---|---|
| Trace IDs | **NOT AVAILABLE** on transcript surfaces. On the proxy ledger the provider's own `request_id` is recorded verbatim and never synthesised: OBSERVED |
| Headers / transport metadata | **OBSERVED only on the proxy path**, where Replay is in the request path. NOT AVAILABLE from any transcript surface |
| Continuation identifiers | Claude Code `parentUuid` chains: OBSERVED. Codex rollout files are one session each: OBSERVED by construction. Grok: session directory only. Ollama: NOT AVAILABLE |
| Provider-specific context indicators | Claude Code compaction records: OBSERVED, with a documented defect (one record shows postTokens above preTokens, clamped rather than allowed to offset). `cache_miss_reason`: OBSERVED, six classes, Claude Code only |
| Provider-issued cost | **PROVIDER ASSERTION on Grok only** (`costUsdTicks`). Deliberately not decoded. Oracle exposes a `cost` field whose one observed record fails an internal consistency check |
| Billing evidence | **NOT AVAILABLE from any provider API examined.** The single reconciliation performed used an independently observed account balance, read by a human, not an API |

## The assertion boundary, stated as a rule

**A provider assertion may not be promoted to observed evidence by being
decoded.** Two applications of that rule are already load-bearing in the code:

- Grok's `costUsdTicks` is **not decoded into a typed field at all**. The vendor
  guide states 10^10 ticks per USD; no invoice has been reconciled against it.
  A conversion would require changing the type, not a format string.
- Codex and Grok cache-write counters are present and always zero. That is a
  **provider-side or client-side silence**, and for Codex it was shown to mask a
  billed quantity. Reporting it as a measured zero would convert an absence into
  a finding.

The one place an assertion was checked against something independent is the
money reconciliation: 575 responses, derived cost inside an observed balance
interval, reported as an interval because a difference of two cent-resolution
readings carries two quantisation errors.

## Sources and dates

Provider-documentation claims used here: OpenAI's published cache-write pricing
for GPT-5.6 and later at 1.25x uncached input, and the Grok 1.0.41 user guide's
tick scale. Both are **PROVIDER ASSERTION**, checked against provider
documentation on 2026-09-26 and 2026-09-27 respectively, neither reconciled
against a statement of account. Everything else in this document was measured
against local corpora by readers in this repository.

## The three traps, measured

**1. "Zero cache write" means three different things.** On DeepSeek and Gemini
and OpenAI at or below 5.5, zero is a true fact about the billing contract. On
Codex it is a **client-logging defect masking a billed quantity**: 117 of 117
turns on a 5.6-generation model carried uncached input, 79 exceeded the
1,024-token cache minimum, the tier is documented at a 1.25x write premium, and
0 of 117 logged a write. On Grok it is **unknown**: the field is present and was
never populated across a 48-session corpus, and the pricing contract was not
found. Any cross-surface cache metric that treats these as one zero ranks
whichever client logs worst.

**2. "Input tokens" is two different quantities.** Anthropic counts exclusively,
Codex and Grok inclusively. Applying the wrong assumption to "1,000 prompt, 800
cached" yields 1,800: a 1.94x overstatement, worst on the sessions that cache
best.

**3. Aggregation differs per surface.** Grok carries two disagreeing
representations of one session (sum of turns vs the vendor's stated total,
reconciled as MATCH / DIFFERS / UNAVAILABLE, never added). Codex carries `Billed`
against `Reported`. Ollama has no session concept at all.

## Cost standing, which is not uniform

| Surface | Standing |
|---|---|
| Claude Code, Codex | DERIVED from a hand-maintained dated table, reconciled once against an observed balance to within a stated interval |
| Grok | **REFUSED.** The guide states 10^10 ticks per USD; nobody has reconciled it against an invoice. `costUsdTicks` is not decoded into a typed field, so a conversion requires changing the type rather than a format string |
| Ollama | `localOnly`, a different cell from "unknown price" |
| DeepSeek | DERIVED; prefix cache geometry established in the sequential regime only |

## Retention and volatility

Transcripts persist until the user deletes them. The proxy ledger is written by
Replay and persists. Codex durable usage logging is **regressing**: 26/26
sessions carried `last_token_usage` in 2026-03 against 20/235 (9%) in 2026-09.
Grok's `usage.json` is live and grows with the session, so a golden test must
use a published snapshot rather than the live corpus.

## The finding that matters most for the research program

**Task outcome is NOT AVAILABLE on every surface examined.** That is not a gap in
Replay's readers; it is a property of what the surfaces record. Until it changes,
R10 is blocked and claim 3 of the research program stays unevidenced.
