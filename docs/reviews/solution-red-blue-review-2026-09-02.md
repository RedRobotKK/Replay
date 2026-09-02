# Buffy Solution Design: Red / Blue Team Review

Adversarial review of the full Buffy design as it stands after the PRD review: offline replay and what-if engine, blame and diff, live transparent proxy with policy application, error cost ledger, secret masking, spend controls, and the self-improving policy layer. The question under test is whether the idea is solid and secure enough to build and announce.

**Review date:** 2026-09-02
**Scope:** the design described in `docs/ROADMAP.md`, ADR-0001, and the design discussion of 2026-09-02 (replay-first, three commands, policy learning). Facts about provider caching and history binding are the same ones verified in `PRD-v4-adversarial-review.md`, Appendix A.

---

## 0. Verdict

**The idea is solid and can be made secure, under five constraints.** Without them it is a debunking thread waiting to happen. With them it is a defensible, honest tool.

1. **Two tiers of truth.** Numbers derived from transcripts alone are labeled *estimated*. Numbers captured by the proxy on the wire are labeled *measured*. The two are never mixed in one table without the label.
2. **Live policies use only provider-sanctioned mechanisms.** Request parameters (context editing, cache breakpoint placement, TTL), append-only additions (mid-conversation system messages), and deterministic per-message masking. Buffy never edits an earlier turn client-side.
3. **Replay reports prompt-side savings under the stated assumption that the agent's behavior is unchanged.** Behavior effects (re-reads, failed edits) are only measured in bounded live trials with automatic revert.
4. **Buffy stores no provider credentials and, by default, no raw conversation content.** The ledger holds derived data: hashes, token counts, structure, labels.
5. **Secret rehydration is scoped and logged.** Placeholders are never rehydrated into inputs of network-capable tools by default.

Two findings are severe enough to block a public launch until closed: placeholder forgery as an exfiltration path (S1) and the OAuth base-URL question that decides whether the proxy reaches most Claude Code users at all (P1). Both have concrete resolutions below.

---

## 1. Red team: the replay and what-if engine

| # | Attack | Severity | Holds? |
|---|--------|----------|--------|
| R1 | Transcripts do not contain the system prompt, tool definitions, or cache breakpoint markers. The prefix that caching keys on starts with exactly those. Replay cannot see the first and most expensive part of the prompt. | High | Partially. Mitigated by tiering (Section 3). |
| R2 | Per-block token attribution needs a tokenizer. The provider's tokenizer is not public; the counting endpoint needs network and a key. Offline blame is a proportional estimate. | High | Partially. Estimates must carry error bars and the *estimated* label. |
| R3 | Calibration can pass by coincidence for a wrong model of where the client places breakpoints, and the what-ifs built on it are then wrong. | Medium | Mitigated: calibration must match both cache reads and cache writes across many sessions, not one. |
| R4 | Every what-if assumes the model would have produced the same outputs under a different prompt layout. That is false in general: clear old tool results and the agent re-reads files. Savings are an upper bound. | High | Accepted and disclosed. The output states the assumption on every table. Live trials are the only measure of behavior change. |
| R5 | Provider rules drift: TTL, minimum cacheable size, lookback window, prices, new models. A silent price change keeps calibration passing while every saving figure is wrong. | Medium | Mitigated: versioned, dated rules and price tables shown in the output; calibration drop refuses what-ifs. |
| R6 | TTL simulation needs request start times. Transcripts carry message timestamps, which approximate response end. Long generations shift the window. | Low | Accepted. Error is bounded and documented. |
| R7 | The OpenAI-compatible side has different caching rules (automatic prefix caching, no breakpoints, a different minimum, `cached_tokens` in usage). One simulator does not fit both. | Medium | Scope: Anthropic rules first, a second rules module later. Say so in the README. |

**Red team conclusion.** Offline replay is honest only as a *diagnostic with estimates*. It becomes a *measurement* once the proxy is in the path and sees the full prefix, the markers, and the exact bytes. The design must make that distinction impossible to miss.

## 2. Red team: the live proxy and policy application

| # | Attack | Severity | Holds? |
|---|--------|----------|--------|
| L1 | Any policy that edits an earlier turn client-side ("clear tool results older than N") is a history edit. On the newest models it returns a 400 for organizations created after 2026-08-31; on every model it invalidates the cache from that point, which defeats the purpose. | Critical | Closed by constraint 2: such policies are applied by setting the provider's own context-editing parameter, which is applied server-side per request and is excluded from the history check. |
| L2 | Server-side compaction returns blocks the client must echo back. Claude Code does not know Buffy enabled it. Enabling compaction from a proxy corrupts the next turn. | High | Closed: compaction is never enabled by the proxy. Context editing is safe because it is stateless per request; compaction is not. |
| L3 | Adding cache breakpoints can exceed the four-marker limit or violate TTL ordering when the client already placed its own. | Medium | Closed: count client markers first; never add beyond the limit; never place a 1-hour marker after a 5-minute one. |
| L4 | The client updates and starts doing what a policy does (its own context editing). Two layers fighting produce nonsense. | Medium | Closed: never override a parameter the client set explicitly. Key policies by client user-agent version. |
| L5 | The nightly job changes the chosen policy while a session is live. Turn 12 renders differently from turn 11. | High | Closed: policy is pinned per session at the first request and persisted; changes apply to new sessions only. |
| L6 | Session identity. The proxy must know which requests belong to one session to pin a policy. Misidentification mixes policies within a session. | Medium | Mitigated: key on a hash of the stable prefix (system prompt plus first user turn). Conservative fallback: no policy. |
| L7 | Streaming fidelity: flush behavior, HTTP/2, multi-minute turns, backpressure, mid-stream disconnects. A hang here loses trust permanently. | High | Mitigated by requirements: reverse proxy with immediate flush, timeouts sized for multi-minute turns, recorded SSE fixtures in tests, fault-injection tests. |
| L8 | Parallel subagents send identical prefixes at once; the cache is readable only after the first response begins streaming, so all of them miss. | Low | Opportunity, not a defect: an optional policy holds siblings until the first response starts. Evaluate with replay before shipping. |
| L9 | Transparency claim versus policy application. Once a policy is on, bytes are not transparent by definition. | Low | Closed by wording: transparent by default; when a policy is on, every transformation is logged and the before and after bytes are diffable; one variable turns it all off. |

## 3. Red team: the learning layer

| # | Attack | Severity | Holds? |
|---|--------|----------|--------|
| M1 | Small samples. A single developer produces tens of sessions a week. Split by session type, the buckets are tiny. Score many policies and parameters nightly and the winner is the lucky one. | High | Mitigated: few policies, coarse parameter grids, held-out sessions, a win must repeat across sessions, ties go to the simpler policy. |
| M2 | A policy can cut tokens and raise errors. Replay cannot see the errors it causes, because it assumes unchanged behavior. | High | Mitigated: guardrail metrics in live trial (failed-edit rate, re-read rate) with automatic revert. Replay never graduates a policy on its own. |
| M3 | "Verified saving" is confounded by the user simply working less that week. | Medium | Closed: primary metric is the cached share of the prompt per turn, which is scale-free; absolute totals are shown second. |
| M4 | Session-type misclassification in the first turns picks a suboptimal policy. | Low | Accepted: all policies in the catalog are safe by construction, so misclassification costs efficiency, never correctness. |
| M5 | The learning state itself becomes a hidden source of non-determinism. | Medium | Closed: learning writes a versioned policy file; the proxy reads it only at session start; the file is human-readable and diffable. |

## 4. Red team: security

| # | Attack | Severity | Holds? |
|---|--------|----------|--------|
| S1 | **Placeholder forgery.** Masking replaces a secret with a placeholder the model can see. Untrusted content in a tool result (a file in a cloned repository) instructs the model to write that placeholder into a shell command. Buffy rehydrates the real secret into the command, which then sends it to the attacker. | Critical | Open in the current design. Resolution: rehydration is scoped. By default, placeholders rehydrate into assistant text and into file-edit tool inputs targeting paths inside the project, never into shell or network tool inputs. Every rehydration is logged with its destination. Users can widen the scope per pattern. |
| S2 | Masking false negatives. Two regex patterns give a false sense of safety. | High | Mitigated: a maintained pattern set for known key formats, user-defined patterns, an optional entropy heuristic, and a per-session report of what was masked so the user can check. The README never says "all secrets"; it says which patterns. |
| S3 | Buffy becomes a second copy of every transcript, retained longer than the client's own. Code and secrets at rest, again. | High | Closed by constraint 4: the ledger stores hashes, token counts, structure, and labels such as file paths. Raw content storage is opt-in, encrypted at rest, with a retention setting and a purge command. |
| S4 | Provider credentials. If Buffy stores or caches the API key or OAuth token, it becomes the most valuable file on the machine. | High | Closed: Buffy forwards the client's authentication headers unchanged and never persists them. Headers are excluded from every log path, including debug. |
| S5 | Local dashboard renders tool output. A file in a malicious repository contains script; it executes in the dashboard with access to Buffy's data. | High | Closed: terminal-only output in v0.1. When a web dashboard ships: strict escaping, a content security policy with no inline script, loopback binding, Host header check, token on every read. |
| S6 | Loopback listener reachable from a browser page on the same machine through a cross-origin request. | Medium | Closed: no stored credentials means a forged request carries no key. Dashboard reads require the token. Origin and Host checks reject browser-originated calls. |
| S7 | Placeholder correlation. The same secret maps to the same placeholder, so the provider can tell two sessions share a secret. | Low | Accepted and documented. Keying per project rather than globally limits correlation. |
| S8 | Supply chain. A tool that sits in front of API keys is a target for a poisoned dependency or a tampered release. | High | Mitigated: minimal dependencies, pinned modules with checksum verification, signed release binaries, a software bill of materials, reproducible builds, no auto-update mechanism. |
| S9 | Memory hygiene. Request bodies with code and secrets live in process memory; a garbage-collected runtime cannot zero them. | Medium | Accepted and documented: no core dumps, no body logging, debug logs redact. |
| S10 | Multi-user machines. | Low | Closed: loopback with token; ledger under the user's home with owner-only permissions. |

## 5. Red team: product and platform

| # | Attack | Severity | Holds? |
|---|--------|----------|--------|
| P1 | **Subscription-authenticated Claude Code may not honor a base URL override, or proxying that traffic may be outside the provider's terms.** If so, the live proxy reaches only API-key users, a minority of Claude Code users. | Critical to plan | Open. Must be verified in a spike before any launch claim. Offline replay works for everyone regardless, which is a second reason to ship replay first. |
| P2 | The provider ships cache diagnostics and cost reporting natively; a third-party tool already reports Claude Code session costs. | Medium | Accepted. The defensible parts are cross-client operation, local what-if analysis, and the advisor loop. For the stated goal (being noticed) this is sufficient; it is not a moat and should not be described as one. |
| P3 | Estimated numbers presented as measured get debunked publicly. | High | Closed by constraint 1 and the calibration line on every output. |
| P4 | Provider terms on modifying requests. | Low | Buffy acts inside the user's own account on the user's own machine and adds only documented parameters. Document this and link the relevant terms. |

---

## 6. Blue team: what holds and why

**The core insight survives every attack.** Prompt caching is prefix-based and priced; the provider tells you exactly how much was cached on every response; transcripts and the wire together contain enough to reconstruct why. Nobody else turns that into "here is the turn where it broke and here is what a different layout would have cost." The attacks above narrow the claims; none of them removes the value.

**The constraints make the design stronger, not weaker.**

- Restricting live policies to provider-sanctioned mechanisms (L1, L2) removes the entire class of history-editing bugs and means Buffy can never trip the binding check. It also means the policy catalog is small and each entry is auditable.
- Tiering estimated versus measured (R1, R2) turns the biggest credibility risk into a feature: the proxy's value proposition becomes "upgrade your estimates to measurements."
- Storing derived data only (S3) makes the ledger boring to steal and easy to explain to a security team.
- Forwarding credentials unchanged (S4) means Buffy has nothing to leak that the client did not already have.
- Pinning policy per session (L5) makes every session reproducible from its policy file and its transcript, which is also what makes bug reports answerable.

**The learning layer is safe because it never touches bytes.** It writes a policy file; the proxy reads it at session start; the policy is a pure function. Learning can be wrong and the worst outcome is a suboptimal but valid session.

**The masking layer is worth shipping only with scoped rehydration.** With S1 closed it delivers what it promises: the provider does not see the secret, and the secret cannot be routed to the network by content the model read. Without S1 closed it should not ship.

---

## 7. Validation spikes that gate any announcement

Each spike is a day or less and settles a question the design depends on.

| Spike | Question | Pass condition |
|-------|----------|----------------|
| 1 | Do Claude Code transcripts carry per-message usage with cache read and write counts? | Fields present on a sample of 20 real sessions across two client versions. |
| 2 | Does the replay engine reproduce as-run cache reads and writes from transcripts? | Match on at least 95 percent of turns across 20 sessions, with the mismatches explained. |
| 3 | Does Claude Code honor a base URL override under subscription authentication, and is proxying that traffic within terms? | Documented answer with a source. If no: replay-first is the whole v0.1 and the proxy targets API-key users only. |
| 4 | Does adding the context-editing parameter from a proxy leave Claude Code's behavior intact? | A ten-turn session completes with the parameter present and the response fields it adds ignored by the client. |
| 5 | Does scoped rehydration hold under adversarial content? | A test corpus of injected placeholders in tool results never reaches a shell or network tool input. |

---

## 8. Required design changes before building

1. Add the two-tier truth model to the output format specification: every table carries *estimated* or *measured*, plus the calibration line and the "assumes unchanged agent behavior" note.
2. Rewrite the policy catalog so every live policy maps to a request parameter, an append-only addition, or a deterministic per-message transformation. Delete any policy that edits earlier turns client-side.
3. Specify the ledger schema as derived data only, with the opt-in raw mode, encryption, retention, and purge.
4. Specify scoped rehydration with a default deny for shell and network tool inputs, and a rehydration log.
5. Add the five spikes to the roadmap as the gate for v0.1, with spike 3 first because it changes the plan most.
6. Record the above as ADR-0002 (replay engine and truth tiers), ADR-0003 (policy application constraints), and ADR-0004 (masking and rehydration scope).
