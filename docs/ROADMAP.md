# Roadmap

What ships in each release, the gate for each, and what is deliberately deferred. The requirements behind every line are in [`prd/buffy-prd-v5.0.0.md`](prd/buffy-prd-v5.0.0.md); the decisions are in [`adr/`](adr/).

The sequence is chosen so the first release costs nothing to run, works for every user regardless of how they authenticate, and produces the output that gets the project noticed. Items are **proposed** until the owner approves PRD v5.

## Gating spikes (before any public claim)

| Spike | Question | Pass condition |
|-------|----------|----------------|
| 1 | Do Claude Code transcripts carry per-message usage with cache read and write counts? | Present on 20 real sessions across two client versions |
| 2 | Does the replay engine reproduce as-run cache reads and writes? | At least 95 percent of turns across 20 sessions, mismatches explained |
| 3 | Does Claude Code honor a base URL override under subscription authentication, within the provider's terms? | Documented answer with a source |
| 4 | Does adding the context-editing parameter from a proxy leave Claude Code's behavior intact? | A ten-turn session completes with the parameter present |
| 5 | Does scoped rehydration hold under adversarial content? | Adversarial corpus never reaches a shell or network tool input |

## v0.1: replay, blame, diff (offline)

Read Claude Code transcripts. Reproduce the provider's caching turn by turn, print the calibration line, then score alternative layouts and rank token sources. Estimated tier only; Anthropic rules; macOS, Linux, Windows.

**Gate:** spikes 1 and 2 pass; every output carries tier, calibration, and assumption lines; README shows real output from the maintainer's own sessions.

## v0.2: transparent proxy

`buffy serve`: loopback listener, byte-for-byte passthrough for the Anthropic Messages API and OpenAI chat completions with streaming, usage capture, measured tier, live `diff`. No policies yet.

**Gate:** spike 3 answered; passthrough hash test green on the fixture corpus; added latency p99 published.

## v0.3: policies, dry-run, guards

Policy catalog using only provider-sanctioned mechanisms (ADR-0003), dry-run scoring of candidates, spend and loop guards, provider circuit breaker, error budget.

**Gate:** spike 4 passes; provider history-binding check green in CI against a policy-applied session; guardrail revert tested.

## v0.4: learning and advisor

Nightly local re-scoring, held-out validation, session types, bounded live trials with automatic revert, suggestions tracked from prediction to verified saving.

**Gate:** synthetic-corpus selection test passes; policy file format documented.

## v0.5: secret masking

Named pattern set, HMAC-derived placeholders, persistent encrypted vault, scoped rehydration (ADR-0004), per-session masked report.

**Gate:** spike 5 passes; precision and recall published for the pattern set.

## 1.0

External security review published, signed reproducible releases, caching rules for a second provider.

## Deferred until a user asks

Vector store, agent-to-agent messaging, virtual filesystem, Rust sidecar, web dashboard. Padded cache slots are rejected outright (ADR-0001).

## Not goals

Translating between provider API shapes. Replacing server-side compaction or context editing. Any server component. Cursor agent mode.
