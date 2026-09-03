# Roadmap

What ships in each release, the gate for each, and what is deliberately deferred. The requirements behind every line are in [`prd/buffy-prd-v5.0.0.md`](prd/buffy-prd-v5.0.0.md); the decisions are in [`adr/`](adr/).

The sequence is chosen so the first release costs nothing to run, works for every user regardless of how they authenticate, and produces the output that gets the project noticed. Items are **proposed** until the owner approves PRD v5.

## Gating spikes (before any public claim)

| Spike | Question | Pass condition |
|-------|----------|----------------|
| 1 | Do Claude Code transcripts carry per-message usage with cache read and write counts? | Present on 20 real sessions across two client versions. **Status:** confirmed on 1 session, client 2.1.258; corpus pending |
| 2 | Does the replay engine reproduce as-run cache reads and writes? | At least 95 percent of turns across 20 sessions, mismatches explained. **Status:** 55/56 turns on 1 session, the miss explained; corpus pending |
| 3 | Does Claude Code honor a base URL override under subscription authentication, within the provider's terms? | Documented answer with a source. **Status: passed.** The LLM gateway docs state that `ANTHROPIC_BASE_URL` without a gateway credential keeps the subscription login active and routes requests through the gateway; a local gateway is a documented configuration. See `architecture/proxy-protocol.md` |
| 4 | Does adding the context-editing parameter from a proxy leave Claude Code's behavior intact? | A ten-turn session completes with the parameter present |
| 5 | Does scoped rehydration hold under adversarial content? | Adversarial corpus never reaches a shell or network tool input |

## v0.1: replay, blame, diff (offline)

Read Claude Code transcripts. Reproduce the provider's caching turn by turn, print the calibration line, then score alternative layouts and rank token sources. Estimated tier only; Anthropic rules; macOS, Linux, Windows.

**Gate:** spikes 1 and 2 pass; every output carries tier, calibration, and assumption lines; README shows real output from the maintainer's own sessions.

## v0.2: transparent proxy

`buffy serve`: loopback listener, byte-for-byte passthrough for the Anthropic Messages API with streaming, usage capture into a derived-data ledger, measured tier for `replay`, `blame`, and `diff`. No policies yet.

**Status:** implemented for the Anthropic Messages API and tested against a fake provider (byte-exact request and response, incremental flushing, gzip, error passthrough, origin and token checks, no credential in ledger or logs). Not yet exercised against the real provider or a real client session. OpenAI chat completions passthrough works as bytes but its responses are not summarized; retries and provider-error handling are v0.3 with the guards.

**Gate:** spike 3 answered (done); passthrough hash test green (done); added latency p99 published (done: 98µs, see `reviews/proxy-latency-2026-09-03.md`); a real Claude Code session recorded through the proxy with calibration at the measured tier (pending, needs a person at a machine).

## v0.3: policies, dry-run, guards

Policy catalog using only provider-sanctioned mechanisms (ADR-0003), dry-run scoring of candidates, spend and loop guards, provider circuit breaker, error budget.

**Status:** guards are implemented and tested against a fake provider: spend caps per session and per day (fail closed before the next request, override logged), loop detection (warn or refuse), and the circuit breaker. Dry-run scoring (DR-1) is implemented: the proxy re-scores TTL and context-editing candidates after every turn from measured usage, publishes them on `/buffy/status`, and sends nothing extra to the provider; the live figures equal an offline replay of the same ledger by test. `freeze-system-prompt` is implemented as the prefix hash on every ledger record and the certain break cause it gives. `context-edit-tool-results` is implemented as an opt-in flag that adds the provider's `context_management` parameter to admissible requests, pinned per session, logged with hashes, and measured through the provider's applied-edits report on the ledger; it is off by default and marked experimental until spike 4 runs it against the real provider. Retries are implemented as PX-4 specifies: bounded, jittered, honoring `Retry-After`, only on retryable failures, only before any response byte reached the client, off by default. The error budget (SP-4) and list-price dollar caps (SP-1) are implemented. Graduation (DR-2), `breakpoint-on-stable-block`, and `hold-parallel-siblings` are not built.

**Gate:** spike 4 passes; provider history-binding check green in CI against a policy-applied session; guardrail revert tested.

## v0.4: learning and advisor

Nightly local re-scoring, held-out validation, session types, bounded live trials with automatic revert, suggestions tracked from prediction to verified saving.

**Status:** the learning job is implemented as `buffy learn` (LN-1, LN-2, LN-4, LN-6 with the metric decision in ADR-0006): catalog re-scoring over all sessions, held-out validation, minimum evidence, margin above noise, paired ties to the simpler policy, intervals in the output, a documented policy file. The proxy reads the policy file at each session's first request and pins the decision on disk (PX-8, `serve --policy-file`). The advisor is implemented as `buffy advise` (AD-1 to AD-3): suggestions from the largest token sources with predicted savings on the share of prompt tokens, tracked to closure across sessions. Live trials with automatic revert (LN-5) are implemented on the re-read guardrail: a stable share of new sessions is treated, the rest are controls, and enough breaches revert the policy for new sessions until a newer learning result. Session types (LN-3) are implemented in `learn` on model family and first-prompt size, with one selection per type in the policy file; the proxy does not yet pick by type. Graduation from trial results (DR-2) is not built.

**Gate:** synthetic-corpus selection test passes (done); policy file format documented (done: `architecture/policy-file.md`).

## v0.5: secret masking

Named pattern set, HMAC-derived placeholders, persistent encrypted vault, scoped rehydration (ADR-0004), per-session masked report.

**Gate:** spike 5 passes; precision and recall published for the pattern set.

## 1.0

External security review published, signed reproducible releases, caching rules for a second provider.

## Deferred until a user asks

Vector store, agent-to-agent messaging, virtual filesystem, Rust sidecar, web dashboard. Padded cache slots are rejected outright (ADR-0001).

## Not goals

Translating between provider API shapes. Replacing server-side compaction or context editing. Any server component. Cursor agent mode.
