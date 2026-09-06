# Roadmap

What ships in each release, the gate for each, and what is deliberately deferred. The requirements behind every line are in [`requirements.md`](requirements.md); the decisions are in [`adr/`](adr/README.md).

The sequence is chosen so the first release costs nothing to run, works for every user regardless of how they authenticate, and produces the output that gets the project noticed. Items are **proposed** until the owner approves PRD v5.

## Gating spikes (before any public claim)

| Spike | Question | Pass condition |
|-------|----------|----------------|
| 1 | Do Claude Code transcripts carry per-message usage with cache read and write counts? | Present on 20 real sessions across two client versions. **Status:** confirmed on **78 distinct sessions**, read across 1450 transcript files ([corpus](evidence/calibration-corpus-2026-09-06.md)). Until 2026-09-06 this row said "1363 real sessions", which was the file count from the [2026-09-05 corpus](evidence/calibration-corpus-2026-09-05.md); a session writes one transcript per agent lane, and one session supplied 1020 of those rows. The gate is still met — 78 against 20 — but by four times over rather than seventy, and independence is not met at all, because every session comes from one machine |
| 2 | Does the replay engine reproduce as-run cache reads and writes? | At least 95 percent of turns across 20 sessions, mismatches explained. **Status:** 97.46% (28,135/28,868 turns) across 1450 transcripts, from **78 distinct sessions**, on one machine ([corpus](evidence/calibration-corpus-2026-09-06.md)); the 450 transcripts below the threshold are listed rather than dropped. Until 2026-09-06 this said "1363 sessions" and was counting files. The session count is still met — 78 against a gate of 20 — but by four times over rather than seventy, and independence is not met at all, because every session comes from one machine, one account and one operator |
| 3 | Does Claude Code honor a base URL override under subscription authentication, within the provider's terms? | Documented answer with a source. **Status: passed.** The LLM gateway docs state that `ANTHROPIC_BASE_URL` without a gateway credential keeps the subscription login active and routes requests through the gateway; a local gateway is a documented configuration. See [`architecture/proxy-protocol.md`](architecture/proxy-protocol.md) |
| 4 | Does adding the context-editing parameter from a proxy leave Claude Code's behavior intact? | A ten-turn session completes with the parameter present. **Status: passed 2026-09-05** ([evidence](evidence/spike-4-real-provider-2026-09-05.md)); the provider applied zero edits on that session, so the parameter is accepted and not yet shown to do anything |
| 5 | Does scoped rehydration hold under adversarial content? | Adversarial corpus never reaches a shell or network tool input |

## v0.1: replay, blame, diff (offline)

Read Claude Code transcripts. Reproduce the provider's caching turn by turn, print the calibration line, then score alternative layouts and rank token sources. Estimated tier only; Anthropic rules; macOS, Linux, Windows.

**Status:** shipped, and the offline path is where most of 2026-09-06 went. Bare `replay` leads with the cost report over the transcript root it discovers, rather than a list of sixteen commands with the one that reports money eleventh; `cost` and `corpus` take that same root as their default; and `--help` is grouped and ranked by value. The cost report states the avoidable figure in tokens as well as dollars and names who the dollars are for, because most readers hold a flat seat and a dollar-only finding is addressed to a minority. It discloses transcript overlap instead of implying the total is exact, and it indexes rather than reparsing — the report over 1,483 transcripts cost 6.3s wall and 19.6s CPU, nearly all of it reparsing unchanged files. Compaction is parsed, so `replay context` now reports by how much its own attribution overstates a session that compacted; across this corpus the client wrote 39 such records at a median retention of 2.55%. `replay route --to` charges for the switch itself rather than comparing two per-turn rates. What breaks the cache is now measured across the whole corpus rather than a sample ([break causes](evidence/break-causes-2026-09-06.md)).

**Gate:** spikes 1 and 2 pass; every output carries tier, calibration, and assumption lines; README shows real output from the maintainer's own sessions.

## v0.2: transparent proxy

`replay serve`: loopback listener, byte-for-byte passthrough for the Anthropic Messages API with streaming, usage capture into a derived-data ledger, measured tier for `replay`, `blame`, and `diff`. No policies yet.

**Status: shipped in v0.2.0.** Implemented for the Anthropic Messages API and **verified against the real provider** on 2026-09-05: a ten-turn session, all 200, 1,816,417 prompt tokens captured from provider usage, zero credential strings and zero message content in the ledger ([spike 4](evidence/spike-4-real-provider-2026-09-05.md)). OpenAI chat completions are now read rather than only forwarded: request summarised, usage converted out of inclusive counting, streaming included, guards and ledger applied. **That path is verified against a test stub only, never against a live OpenAI-compatible provider**, and secret masking does not cover it; the proxy warns at runtime on both counts ([spike](evidence/spike-openai-compatible-2026-09-05.md), [surfaces](SURFACES.md)).

**Gate: met.** Spike 3 answered; passthrough hash test green; added latency p99 published (98µs, see [`evidence/proxy-latency-2026-09-03.md`](evidence/proxy-latency-2026-09-03.md)); a real session recorded through the proxy at the measured tier (spike 4, 2026-09-05).

## v0.3: policies, dry-run, guards

Policy catalog using only provider-sanctioned mechanisms (ADR-0003), dry-run scoring of candidates, spend and loop guards, provider circuit breaker, error budget.

**Status:** guards are implemented and tested against a fake provider: spend caps per session and per day (fail closed before the next request, override logged), loop detection (warn or refuse), and the circuit breaker. Dry-run scoring (DR-1) is implemented: the proxy re-scores TTL and context-editing candidates after every turn from measured usage, publishes them on `/replay/status`, and sends nothing extra to the provider; the live figures equal an offline replay of the same ledger by test. `freeze-system-prompt` is implemented as the prefix hash on every ledger record and the certain break cause it gives. `context-edit-tool-results` is implemented as an opt-in flag that adds the provider's `context_management` parameter to admissible requests, pinned per session, logged with hashes, and measured through the provider's applied-edits report on the ledger; it is off by default and stays marked experimental for a measured reason rather than an absence of measurement: spike 4 ran it against the real provider on 2026-09-05 and the provider applied **zero** context edits on a session whose largest prompt was ten times the configured trigger, so the parameter is accepted without breaking anything and is not yet shown to do anything. Retries are implemented as PX-4 specifies: bounded, jittered, honoring `Retry-After`, only on retryable failures, only before any response byte reached the client, off by default. The error budget (SP-4) and list-price dollar caps (SP-1) are implemented. `hold-parallel-siblings` is implemented as `--hold-siblings`: a request whose prefix is in flight and not yet cached waits, bounded, until the first response begins, with the wait recorded per request. Graduation (DR-2) is implemented in `replay learn`. `breakpoint-on-stable-block` is not built. SP-7 has landed: a daily spend-cap refusal names the session that spent the budget, attributed on today's spend rather than the lifetime total, and says the attribution is partial when eviction means the largest survivor cannot be the largest spender. The proxy also records each response's rate-limit headers verbatim on the ledger, which is the only spend figure a flat-seat user has. SP-5, SP-6 and SP-8 — a resolved tenant key, per-tenant caps, and accounting eviction cannot widen — are specified and not built, gated on [ADR-0015](adr/0015-single-tenant-state-is-a-boundary.md): every piece of shared mutable state here is scoped to one human, so a centralised deployment would turn the day cap into an organisation-wide denial of service.

**Gate:** spike 4 passes (done, 2026-09-05); provider history-binding check green in CI against a policy-applied session; guardrail revert tested (done: breaches and the revert flag are now keyed per policy, so evidence gathered against one trigger cannot revert another and reverting one policy no longer disarms the guardrail for the next).

## v0.4: learning and advisor

Nightly local re-scoring, held-out validation, session types, bounded live trials with automatic revert, suggestions tracked from prediction to verified saving.

**Status:** the learning job is implemented as `replay learn` (LN-1, LN-2, LN-4, LN-6 with the metric decision in ADR-0006): catalog re-scoring over all sessions, held-out validation, minimum evidence, margin above noise, paired ties to the simpler policy, intervals in the output, a documented policy file. The proxy reads the policy file at each session's first request and pins the decision on disk (PX-8, `serve --policy-file`). The advisor is implemented as `replay advise` (AD-1 to AD-3): suggestions from the largest token sources with predicted savings on the share of prompt tokens, tracked to closure across sessions. Live trials with automatic revert (LN-5) are implemented on the re-read guardrail: a stable share of new sessions is treated, the rest are controls, and enough breaches revert the policy for new sessions until a newer learning result. Session types (LN-3) are implemented in `learn` on model family and first-prompt size, with one selection per type in the policy file, and the proxy picks by type at a session's first request. Graduation from trial results (DR-2) is implemented in `learn` from the treated and control sessions the trial recorded. Staleness detection (ST-1) is implemented offline: calibration is judged per model with the newest sessions on their own, a model whose newest sessions stopped calibrating is reported as changed provider behavior and has no alternatives scored, and the minimum cacheable prefix is bounded from usage; the lookback window is not refit (ST-2).

**Gate:** synthetic-corpus selection test passes (done); policy file format documented (done: [`architecture/policy-file.md`](architecture/policy-file.md)).

## v0.5: secret masking

Named pattern set, HMAC-derived placeholders, persistent encrypted vault, scoped rehydration (ADR-0004), per-session masked report.

**Status:** implemented as an opt-in flag: the named pattern set with user patterns and an opt-in entropy heuristic (MK-1), HMAC placeholders under a per-install vault key (MK-2, keyed per install rather than per project until a project identity exists), a vault encrypted at rest under an owner-only key file rather than the operating system keychain (MK-3, partial), scoped rehydration across stream chunks and inside tool-call JSON with per-pattern scope and a logged destination for every placeholder (MK-4 to MK-6), and the per-request masked report on the ledger and status (MK-7). Precision and recall are reported by the corpus test (1.00 and 1.00 on 15 positives and 15 negatives); the adversarial corpus test holds shell, network, unknown tools, and paths outside the project closed. Spike 5 is answered by that test on fixtures, not yet by traffic from a real agent.

**Gate:** spike 5 passes; precision and recall published for the pattern set (done for the repository corpus).

## Open, and measured rather than assumed

**Does a cache break cost a flat seat anything?** It decides whether Replay has a case to make to the majority of the people who run it, and no provider documents it. It was measured on 2026-09-06 with matched arms — ten cold writes and ten warm reads of a ~159,000 token prefix, same model, same size, only warm against cold differing — and 3.09M tokens moved the 5h utilisation counter by **zero** steps ([titration](evidence/quota-titration-2026-09-06.md)). A ratio near 12.5 and a ratio near 1.0 remain equally consistent with what has been measured, and the tool says so instead of picking one.

Two things came out of the attempt that outlive the null. The instrument now refuses rather than reports, naming which arm is short: the first estimator took the median of non-zero deltas, and simulation showed it returning exactly 1.00 whether the truth was 12.5 or 1.0, because the counter moves in whole hundredths. And live traffic settled a documentation question — a subscription session returns none of the documented `tokens-remaining` headers, but does return `anthropic-ratelimit-unified-5h-utilization`, `-7d-utilization` and `-representative-claim`.

Closing it needs a quiet account rather than a bigger budget: the account-wide counter is the largest error term and it is removable, not reducible. Until then, the report states the waste in tokens as well as dollars, which is a statement that holds either way.

## 1.0

External security review published, signed reproducible releases, caching rules for a second provider.

## Deferred until a user asks

Vector store, agent-to-agent messaging, virtual filesystem, Rust sidecar, web dashboard. Padded cache slots are rejected outright (ADR-0001).

## Not goals

Translating between provider API shapes. Replacing server-side compaction or context editing. Any server component. Cursor agent mode.

---

[Documentation index](README.md) · [Repository README](../README.md)
