# Project Buffy PRD v4.0.0 — Adversarial Review (Red / Blue / Adopter)

**Reviewed document:** `projectbuffyprd.md`, version 4.0.0-PROD_FINAL, dated 2026-09-02
**Review date:** 2026-09-02
**Review posture:** Senior systems engineer + security reviewer + prospective enterprise adopter. Every claim below was checked against current Anthropic API caching and history-binding semantics (see Appendix A for the facts used).

---

## 0. Verdict

**Not ready for execution. Status should be reverted from "Approved for Production" to "Draft / Discovery".**

The PRD reads well, but four of its load-bearing mechanisms are either technically incorrect or contradict each other, and the code samples would not connect to a single IDE listed in the vision statement. The strongest, most defensible product inside this document is buried in Sections 2.4 and 3: **a transparent local proxy that gives developers cache diagnostics, spend controls, and secret masking**. The headline features (byte-padded cache slots, AST pruning of tool output, a virtual filesystem, a vector store, an agent mesh bus) are either wrong, unproven, or scope creep that will sink a small team.

Blocking findings, in order of severity:

| # | Finding | Type | Severity |
|---|---------|------|----------|
| 1 | "Byte-perfect padded slots" do not affect prompt caching. Caching is an exact-byte prefix match; length alignment buys nothing and the padding costs tokens. The `CompileAnchorPaddedSlot` function also silently truncates real context. | Correctness | **P0** |
| 2 | The `/v1/chat/completions` facade cannot serve Claude Code, which speaks the Anthropic Messages API (`/v1/messages`, `count_tokens`, SSE streaming, thinking blocks, its own `cache_control`). Cursor does not route its agent through a user proxy at all. "Drop-in" is false for two of the three named clients. | Product | **P0** |
| 3 | Rewriting conversation history mid-flight (pruning tool results, re-masking) collides with Anthropic's preserved-thinking history-binding check. On Claude Fable 5.1, organizations created on or after 2026-08-31 receive a **400** when an earlier turn changes. Buffy's users on new orgs will be enforced before Buffy is. | Correctness | **P0** |
| 4 | AST-pruning tool results changes what the agent believes is in the file. Coding agents edit by exact string match against the tool output they just read. Pruned bodies mean failed edits, retries, and hallucinated code. No mitigation is specified. | Product / Safety | **P0** |
| 5 | The transport in the code (Unix socket + 32-byte raw pre-HTTP handshake) cannot be reached by any HTTP client. The diagram says `localhost:4000`; the code listens on a Unix socket. | Correctness | **P1** |
| 6 | The masking vault lives only in RAM with random placeholder IDs. A daemon restart orphans every placeholder in the client's stored transcript, and non-deterministic placeholders break both caching and history binding. | Security / Data loss | **P1** |
| 7 | No Windows story. Abstract Unix sockets are also invisible from devcontainers, which is where a large share of Claude Code users run. | Adoption | **P1** |
| 8 | The Rust/IPC split is justified by cgo overhead, but a socket round-trip costs far more than a cgo call. The justification is backwards, and the sample Rust daemon has no message framing. | Architecture | **P1** |
| 9 | "Open-source" plus BSL 1.1 is a contradiction that enterprise legal will catch on day one. | Adoption | **P2** |
| 10 | No MVP scope, phases, acceptance criteria, owners, non-goals, or risk register. The document is a vision plus code, not an execution plan. | Execution | **P1** |

A recommended re-scope is in Section 6.

---

## 1. Red Team: Attacks on the Core Claims

Each item: the claim as written, why it fails, the concrete failure scenario, and the fix.

### 1.1 "100% cache-hit rate via character-perfect slot alignment" (Sections 1.2, 2.1, 4.1)

**Claim.** Padding context blocks to fixed byte lengths keeps the provider's prompt cache valid when block contents change.

**Why it fails.** Provider prompt caching keys on the exact bytes of the rendered prompt up to each `cache_control` breakpoint. Any changed byte at position N invalidates every breakpoint at or after N. Length is irrelevant. A slot whose content changed but whose length stayed constant is a cache miss exactly like one whose length changed. There is no "slot" concept on the provider side, only prefix.

**Failure scenario.** L1 summary is regenerated at turn 12. It is padded to the same 8,000 bytes. Every breakpoint after the system prompt misses. The user paid for the padding tokens on every one of the previous 11 turns for zero benefit.

**Secondary defects in the sample code.**

- `rawContext[:targetTokens*4]` truncates real context when it exceeds the slot. This silently drops code from the prompt. In a coding agent that is a correctness bug, not a degradation.
- `len(rawContext) / 4` is not a token count. Token density varies by a factor of two or more between prose, code, JSON, and non-Latin text. The API has a `count_tokens` endpoint for this.
- The filler `[{"buffy_anchor_zone":"0x4F1A","fill":"....` is placed inside the prompt where the model reads it. It is not free. Runs of dots tokenize unpredictably; measure it before claiming "minimal tokens".
- The provider minimum cacheable prefix is model-dependent (512 to 4096 tokens). A padded slot below that threshold silently does not cache at all.
- Maximum 4 `cache_control` breakpoints per request. Claude Code already places its own. Buffy injecting more produces a 400 error.
- Render order is `tools` then `system` then `messages`. Tools come first in the prefix. If the client's tool list changes (lazily loaded MCP servers), everything behind it misses regardless of what Buffy does. The PRD never mentions tools.
- L0 "appended directly to every active upstream execution turn" containing "global system state metrics" is by definition volatile. If it lands in the system prompt, it invalidates the whole cache every turn. It must go after the last breakpoint (as a mid-conversation `role: "system"` message where the model supports it, or in the user turn).

**Fix.** Delete the padding concept entirely. Replace with: (a) freeze everything ahead of the first breakpoint; (b) place breakpoints at stability boundaries; (c) put per-turn volatile content after the last breakpoint; (d) verify with `usage.cache_read_input_tokens` on every request and alert when it collapses. That last item is a real product: a local "your cache broke at turn 7 because the tool list changed" diagnostic is something nobody ships today.

### 1.2 "Drop-in middleware for Cursor, Claude Code, Aider" via `/v1/chat/completions` (Sections 1.1, 2.1)

**Claim.** An OpenAI-compatible endpoint is sufficient to sit in front of the named agents.

**Why it fails.**

- Claude Code uses the Anthropic Messages API through `ANTHROPIC_BASE_URL`. It calls `/v1/messages` and `/v1/messages/count_tokens`, sends `anthropic-beta` headers, streams SSE, sets its own `cache_control` breakpoints, and round-trips `thinking` blocks with signatures. None of that maps onto chat completions. A proxy that only exposes `/v1/chat/completions` is invisible to Claude Code.
- Cursor's agent runs through Cursor's own backend. A user-configured base URL only applies to a narrow custom-model path and disables most agent features. There is no proxy-insertion point for the product's main use case.
- Aider works with both API shapes via base URL overrides. It is the only one of the three where "drop-in" is currently true.
- Any translation layer between API shapes (OpenAI format in, Anthropic format out) forces Buffy to re-render the prompt. Re-rendering is precisely what destroys byte-stable prefixes.

**Fix.** Ship two transparent passthrough listeners: `/v1/messages*` (Anthropic-native, byte-preserving) and `/v1/chat/completions` (OpenAI-native, byte-preserving). Never translate between them in v1. Publish a client compatibility matrix with the exact environment variable per client and a "works / partial / unsupported" status.

### 1.3 History mutation vs. preserved thinking (Sections 2.2, 3.1)

**Claim.** Buffy can prune, compact, and mask message content before forwarding, and the provider will not notice.

**Why it fails.** On Claude Fable 5.1 (and enforced more widely on future models), each `thinking` block's signature binds the conversation prefix that produced it: the top-level system prompt, the tool set, and every message before the block. When the transcript comes back, the API checks that prefix is unchanged. Organizations created on or after 2026-08-31 get a 400 on mismatch. Older orgs record the mismatch and will be enforced on later models.

Buffy's design mutates history in at least three ways:

1. **Pruning drift.** File X is "non-target" at turn 3 and is pruned to signatures. At turn 9 the agent starts editing X, so it becomes "target" and the tool result from turn 3 is re-expanded. That is an edit to an earlier turn. Result: 400 on enforced orgs, full cache miss on everyone else.
2. **Non-deterministic placeholders.** `[SECURE_ASSET_4F1E]` looks random. If the same secret gets a different tag on the next request, every earlier message containing it changes.
3. **Any compaction.** Buffy-side summarization of earlier turns is a middle-of-history edit.

**Fix.**

- Every transformation must be a pure, deterministic function of the input bytes so that a given client message always renders to the same provider bytes for the life of the session. No cross-turn state may influence how an earlier message is rendered.
- Placeholders must be keyed by HMAC of the secret value under a per-session key, so the same secret always maps to the same tag.
- Adopt the three-step compatibility check from the provider migration guide and run it in CI with `prefix_mismatch_behavior: "error"`.
- Prefer server-side compaction and context editing over client-side rewriting; the provider excludes those from the history check.

### 1.4 AST pruning of tool output (Section 2.2)

**Claim.** Stripping function bodies from "non-target" source frames reduces tokens without harming the agent.

**Why it fails.** Coding agents edit files by exact string replacement against content they just read via a tool. If Buffy shows the model a skeleton and the model then issues an edit, the anchor string does not exist in the real file. The edit fails, the agent re-reads (full cost again, now uncached), and in the worst case it "fixes" the file by rewriting it from the skeleton, deleting the real bodies. The PRD's SQLite "source map" only helps Buffy, not the agent, because the agent's edit tool talks to the real filesystem, not to Buffy.

There is also no definition of "target" vs. "non-target". That classifier is the entire feature and is absent.

**Fix.** Do not prune tool results in v1. When it is introduced, make it opt-in per glob, never touch the file the agent last read or edited, and gate release on an A/B benchmark: task success rate, edit-failure count, cost, and wall-clock on a fixed set of real agent tasks with Buffy on vs. off. A `≥4.0x` reduction target as an SLO incentivizes the failure mode; the right SLO is "zero regression in task success at N% cost reduction".

### 1.5 Transport and authentication (Sections 2.1, 2.4, 4.1)

- Diagram: `localhost:4000`. Code: Unix domain socket. Pick one. HTTP clients configured with a base URL need TCP.
- `AuthenticateClient` reads 32 raw bytes before any HTTP. No IDE can prepend a raw token to an HTTP connection. Use a bearer header or an `x-buffy-token` header instead.
- Linux abstract-namespace sockets have **no filesystem permission checks**. Any local UID can connect. The PRD calls this "isolated within memory space"; it is less isolated than a `0700` directory socket. The 32-byte token becomes the only guard, which is fine only if it is actually enforced on every path (it is not; `InterceptStream` never calls `AuthenticateClient`).
- Abstract sockets are scoped to the network namespace. A Claude Code session inside a devcontainer or Docker cannot reach the host daemon. That is a large fraction of the target audience.
- The PRD text says the abstract prefix is a space character; the code uses `\x00`. Editorial, but it signals the text and code were not reviewed together.
- Windows is absent. Named pipes or `AF_UNIX` on Windows 10+ exist, but nothing in the design addresses them.
- "Non-Blocking TCP Window Throttling Guard" sets `SetReadBuffer` on a Unix socket. Unix sockets have no TCP window. The busy-wait loop at 5 ms when memory is capped is a spin, not backpressure, and `ActiveMemory` is never incremented anywhere, so the guard never engages.

### 1.6 The masking vault (Section 3.1)

- **RAM-only with a random per-process key.** Restart the daemon and every placeholder already stored in the IDE's transcript is unrecoverable. The user's session is permanently corrupted. Persist the map, encrypted at rest with a key from the OS keychain (macOS Keychain, libsecret, DPAPI), scoped per session.
- **"Encrypted in host RAM" is theater against the stated threat.** The AES key is in the same process memory as the ciphertext. A memory dump gets both. State the real threat model: the provider must not see secrets; local processes running as the same user are trusted.
- **"Neutralizing cryptographic timing side-channels" via an async queue** is not a thing. Go's AES-GCM is constant-time on supported hardware. Delete the claim; it invites ridicule from security reviewers.
- **Regex coverage is two patterns** (`sk-`, `ghp_`) while the prose promises credentials, JWTs, and PII. Either scope the claim down to "API-key patterns from a maintained list" or specify the detection engine, its false-positive budget, and how users add rules.
- **Rehydration must cover `tool_use.input` JSON and streamed SSE deltas.** The model writes the secret into a file via a tool call; the placeholder arrives inside a JSON string, possibly split across chunks. Buffy must buffer to placeholder boundaries and rehydrate with JSON escaping awareness. None of this is specified.
- **Secrets inside thinking blocks.** If a placeholder appears in a thinking block and Buffy rehydrates it, the client stores the modified block and sends it back. The signature no longer matches. Thinking blocks must pass through untouched in both directions.
- **The proxy holds the crown jewels**: the provider API key, the session token, the vault key, and plaintext of every masked secret. A compromised Buffy is strictly worse than no Buffy. The PRD needs a hardening section (memory locking, no swap, no crash dumps, secure delete, audit log).

### 1.7 Go/Rust IPC split (Sections 2.2, 4.2)

- The justification is cgo overhead. A cgo call is on the order of 100 nanoseconds. A Unix-socket round-trip with serialization is on the order of tens of microseconds. The PRD's own target (`<1.5 ms` round-trip) is three orders of magnitude above the cost it claims to avoid. Tree-sitter has mature cgo bindings. The sidecar adds a second build toolchain, a second release artifact, a lifecycle to supervise, and a socket to secure, for a negative performance return.
- "Zero-allocation IPC" is contradicted by the sample, which allocates a `String`, a `format!` buffer, and a thread per connection.
- The Rust daemon does one `read()` into a 32 KB buffer and treats the result as the whole message. There is no length prefix or delimiter. Any tool output over 32 KB (common for build logs, which is the stated use case) is truncated silently. `from_utf8_lossy` corrupts binary bytes.
- One `incoming()` error breaks the accept loop and the daemon exits.
- "Read-Only SQLite Ephemeral Memory Space" that is written to on every request is a contradiction in terms.

**Fix.** Single Go binary. Tree-sitter via cgo, or a pure-Go grammar subset. Re-evaluate a sidecar only if profiling shows parsing on the hot path, and by then the design should be pruning offline anyway.

### 1.8 Virtual filesystem, vectors, mesh bus (Sections 2.3, 2.4)

- "Accessible to the LLM via standard system commands": a shell cannot open `buffy://`. The only mechanisms are a FUSE mount (macFUSE kernel extension on macOS, a non-starter for enterprise laptops) or MCP resources. The MCP path is correct and should be the only one.
- "AAK Shorthand Dialect" is undefined. A format the model must read and that affects cache bytes cannot be a placeholder name.
- LanceDB is Rust/Python-native; Go bindings are immature. More importantly, **which embedding model?** A local one adds a runtime dependency (ONNX or similar) and hundreds of MB. A cloud one sends the very code the privacy shield is masking to a third party. The PRD is silent. This is a privacy-claim contradiction until answered.
- The A2A/MCP "mesh bus", "AgMsg", and "microsecond-latency" claims have no requirements behind them. Nothing in the vision statement needs multi-agent messaging.
- Token circuit breaker "immediately kills the transaction" mid-stream. The agent receives a truncated response and often retries, spending more. Fail closed *before* the next request instead, based on accumulated `usage` fields, denominated in dollars, with per-session and per-day caps and a user-visible override.

### 1.9 Metrics (Section 3.2)

- OpenTelemetry and Prometheus are different systems; pick the export path.
- `buffy_cache_alignment_coefficient` cannot be computed from Buffy's side of the wire. Define it as `cache_read_input_tokens / (input_tokens + cache_creation_input_tokens + cache_read_input_tokens)` from the provider's `usage` object, per request and per session.
- No baseline exists for the "90%" or "4.0x" numbers. Targets without a measurement method are marketing.
- Missing operational metrics: added latency p50/p99, provider error rate by status, 429 retry count, masking false-positive count, edit-failure delta, daemon memory, vault size.

### 1.10 Licensing and positioning (Sections 1.2, 6)

- BSL 1.1 is not an OSI-approved open-source license and is not "permissive". Calling the project open-source while shipping BSL will be the first thing enterprise legal flags, and the first thing Hacker News flags. Say "source-available, converts to Apache 2.0 on date X". State the Additional Use Grant explicitly; without it, the license text is incomplete.
- Section 6 ("unassailable tech moat", "venture capital or strategic acquisition") does not belong in a PRD that adopters will read. It signals that the document is a pitch, not a spec.
- "MySQL equivalent for AI state" is a positioning line, not a requirement. Remove from the technical sections.

---

## 2. Red Team: Line-Level Code Review

The code is labeled "Production-Grade Implementation Blueprints". It would not pass a first-round review.

### Go (`core`)

- Every error from `MkdirAll`, `WriteFile`, `Chmod`, `Remove`, `ReadFull(cryptoKey)`, `aes.NewCipher`, `cipher.NewGCM` is discarded. A failed vault write leaves the daemon with a token no client can read. A failed key read leaves an all-zero AES key.
- `NewHardenedPlatformGate` writes the session token to the real `$HOME/.buffy/.session_vault` and, on macOS, deletes and rebinds the real socket path. Running the test suite overwrites the live daemon's credentials and socket. Tests must use a temp directory and injected paths.
- `AuthenticateClient` is never invoked by `InterceptStream`. The gate is unauthenticated by default.
- `InterceptStream` spins at 5 ms intervals when `ActiveMemory >= MaxMemoryCap`, but nothing ever increments `ActiveMemory`.
- `scrubRegex` is created and never used. No masking code exists.
- `InMemCryptoVault.mu` and `encryptedMap` are declared and never used. No vault code exists.
- GCM nonce generation and storage is not shown. Nonce reuse under a fixed key is a total break.
- `CompileAnchorPaddedSlot`: silent truncation (see 1.1), returns un-padded input when the gap is under 64 bytes (so the "exact length" guarantee the test asserts is false in that band), and pads with dots inside a JSON string the model will read.
- No HTTP server, no SSE, no upstream client, no retry, no timeout, no 429 handling, no context propagation to upstream. Multi-minute turns on frontier models need explicit timeout design.

### Rust

- No framing; single read; truncation over 32 KB; lossy UTF-8; thread-per-connection with no limit; accept-loop error exits the daemon; socket file permissions not set; no cleanup on exit.
- "Tree-Sitter AST Compactor" is a string prefix. There is no parser.

### Test

- Asserts `len(payload) == targetTokens*4` for a 21-byte input, which passes, but does not test the truncation or the under-64-byte branches where the guarantee fails.
- Races the server goroutine's `Accept` against the client `Dial` with no synchronization. Flaky on loaded CI.
- Uses the real home directory (see above).
- No test for masking, rehydration, streaming, cache-prefix stability, or any of the value propositions.

---

## 3. Blue Team: What Is Worth Defending

Steelmanning the document: there is a real product here, and it is narrower and better than the one described.

### Defensible theses

1. **Developers cannot see why their cache breaks.** Cache misses are silent, and the bill is the only symptom. A local proxy that diffs adjacent request payloads, detects the first divergent byte inside the overlap, and names the cause (tool list changed, timestamp in system prompt, non-sorted JSON, breakpoint pushed out of the 20-position lookback) is genuinely valuable and does not exist as a desktop tool. The provider's cache-diagnostics beta helps but requires code changes in the client; Buffy could do it for any client.
2. **Local secret masking before egress** is a real enterprise requirement, provided it is deterministic, persistent, and honest about its threat model.
3. **Spend circuit breakers per session and per day**, denominated in dollars from real `usage` fields, protect against runaway loops. Fail-closed on the next request, never mid-stream.
4. **Per-session cost and token observability** with a local dashboard. Most developers have no idea what a single Claude Code task costs.

### Design principles that make those four safe

- **Byte transparency by default.** Buffy forwards `/v1/messages` and `/v1/chat/completions` exactly as received unless a feature is explicitly enabled. Off-switch is one environment variable. Every transformation is a pure function of the input, deterministic across turns, and logged.
- **Never touch thinking blocks, signatures, or `cache_control` markers.**
- **Never rewrite an earlier turn differently than it was rewritten before.** Enforce this with a per-session render cache keyed by message hash, and an integration test that runs the provider's prefix-binding check in error mode.
- **Persist what the client depends on.** Placeholders are HMAC-derived; the vault survives restarts; the key lives in the OS keychain.
- **Measure outcomes, not tokens.** The release gate for any transformation is an A/B on real agent tasks: success rate, edit failures, cost, latency.

**Reframed positioning.** "A local, transparent proxy that shows you what your coding agent is spending, keeps your secrets on your machine, and stops runaway loops." That is a product a developer installs in five minutes and a security team can approve. The CxOS vision can stay as a north star in an appendix.

---

## 4. Adopter Lens

Written as the questions a platform-engineering lead at a 200-person company asks before allowing this on developer laptops.

| Question | PRD answer today | What is needed |
|----------|------------------|----------------|
| Does it work with Claude Code out of the box? | No (chat-completions only). | `/v1/messages` passthrough, `ANTHROPIC_BASE_URL` one-liner, `count_tokens` forwarded. |
| Does it work with Cursor? | No, and cannot for agent mode. | Say so. Do not list it. |
| Windows? | Not mentioned. | Support matrix with dates, or state "macOS and Linux only" in the first paragraph. |
| Devcontainers / Docker / WSL? | Abstract sockets make it unreachable. | TCP on loopback with token auth; document the container networking pattern. |
| Can it break my agent's edits? | Yes, by design (pruning), with no mitigation. | Pruning off by default; opt-in per glob; benchmark evidence. |
| Can it lose my data? | Yes (RAM-only vault). | Persistent encrypted vault; documented recovery. |
| What does it add to latency? | Not measured. | p50/p99 added latency per request, published. |
| What happens when it crashes mid-stream? | Undefined. | Supervisor, restart semantics, client-visible error, no orphaned placeholders. |
| What can it see and store? | Everything, unstated. | Data-handling document: what is logged, where, retention, how to purge. |
| How do I turn it off? | Not stated. | One env var; uninstall leaves nothing behind. |
| Is it open source? | Claims yes; license says no. | "Source-available under BSL 1.1; Apache 2.0 on YYYY-MM-DD; Additional Use Grant: ..." |
| Who supports it, what is the release cadence? | Not stated. | Owners, versioning policy, security-disclosure process. |
| Why not just use the provider's built-in compaction, context editing, and caching? | Not addressed. | A clear comparison. The provider's server-side compaction and context editing are free and do not trip the history check; Buffy's value is what they cannot do (local masking, spend caps, cross-client diagnostics). |
| Why not LiteLLM or another proxy? | Not addressed. | Competitive comparison focused on byte transparency, cache diagnostics, and local secret handling. |
| Show me the savings. | "Up to 90%" with no method. | A reproducible benchmark script and a dashboard showing `cache_read_input_tokens` and dollars per session, before and after. |

**Five-minute install test.** An adopter should be able to run one command, set one environment variable, run a normal Claude Code task, and open a local page that shows cost, cache hit ratio, and masked-secret count for that task. Nothing in the current PRD gets to that screen.

---

## 5. Execution Gaps

What a PRD needs for a team to build without guessing, and whether this one has it.

| Element | Present? | Notes |
|---------|----------|-------|
| Problem statement with evidence | Partial | Token bleed is asserted, not measured. Add three real session traces with cost breakdowns. |
| Target user and non-user | No | Solo developer? Platform team? Both have different install and policy needs. |
| MVP scope and non-goals | No | Everything is in scope. Nothing is deferred. |
| Client compatibility matrix | No | See Section 4. |
| Platform matrix (macOS, Linux, Windows, containers) | No | |
| Threat model | No | "Zero-trust" is used as an adjective. Write the actual model: assets, adversaries, trust boundaries. |
| Data handling and retention | No | |
| Acceptance criteria per feature | No | Metrics have targets but no measurement method or baseline. |
| Benchmark methodology | No | Required to make any savings claim. |
| Phases, milestones, owners, dates | No | Four titled authors, zero owners. |
| Risk register | No | This review is a starting point. |
| Dependencies and their licenses | No | tree-sitter grammars, LanceDB, embedding model. |
| Failure modes and recovery | No | Crash, restart, upgrade, vault loss, provider 429/5xx, provider API change. |
| Observability plan | Partial | Three metrics named; export path ambiguous; no logs or traces. |
| Release, versioning, upgrade path | No | |
| Open questions | No | A "PROD_FINAL" document with no open questions is itself a warning sign. |

---

## 6. Recommended Re-scope

### v0.1 — Transparent proxy with visibility (4 to 6 weeks, one Go engineer)

- Single Go binary. TCP loopback listener with header-token auth. `/v1/messages*` and `/v1/chat/completions` byte-transparent passthrough with SSE streaming.
- Per-request capture of `usage` fields. Local dashboard: cost, tokens, cache read ratio per session.
- Cache-break detector: diff adjacent payload prefixes, report the first divergence and a likely cause.
- One env var to disable. Clean uninstall.
- Acceptance: zero behavior change on a fixed suite of Claude Code and Aider tasks; added latency p99 under a stated budget; cache ratio matches direct-to-provider within measurement noise.

### v0.2 — Secret masking and spend control

- Deterministic HMAC placeholders; encrypted persistent vault; OS keychain key; rehydration across SSE chunk boundaries and inside tool-call JSON; thinking blocks untouched.
- Dollar-denominated circuit breaker, fail-closed before the next request, per-session and per-day, with override.
- Acceptance: provider prefix-binding check passes in error mode across a 50-turn masked session; restart mid-session loses nothing; documented false-positive rate on a corpus.

### v0.3 — Opt-in context tools

- MCP server exposing session history and summaries as resources (this is the honest version of `buffy://`).
- Pruning as an opt-in per-glob feature, never on the last-read or last-edited file, gated on an A/B benchmark.

**Deferred indefinitely until a user asks:** vector store, A2A mesh bus, Rust sidecar, padded slots (never).

---

## 7. Required Edits Before the Next Revision

1. Status: "Approved for Production" to "Draft".
2. Remove: padded-slot mechanism and `CompileAnchorPaddedSlot`; "100% cache-hit"; "up to 90%" until benchmarked; "zero-trust" as an adjective; timing side-channel claim; "MySQL for AI state"; "unassailable moat"; all of Section 6.
3. Replace the diagram so transport, endpoints, and auth match the code.
4. Add: client matrix, platform matrix, threat model, data-handling policy, MVP/non-goals, phases with owners, benchmark methodology, risk register, open questions.
5. Define or delete: "AAK Shorthand Dialect", "AgMsg", "target vs non-target" classifier, embedding model choice.
6. Fix the license paragraph.
7. Move the code samples to a `spikes/` appendix labeled as such, or delete them; a PRD should specify behavior, not ship stubs.

---

## Appendix A — Facts Used in This Review

Verified against current Anthropic API documentation on the review date.

- Prompt caching is an exact-byte prefix match up to each `cache_control` breakpoint. Render order is `tools`, then `system`, then `messages`. Any byte change at position N invalidates every breakpoint at or after N.
- Maximum 4 breakpoints per request. Minimum cacheable prefix is model-dependent: 512 tokens on the newest models, up to 4096 on some others. Shorter prefixes silently do not cache.
- Cache writes cost 1.25x base input (5-minute TTL) or 2x (1-hour TTL). Reads cost about 0.1x base, and 0.025x on Claude Fable 5.1.
- Each breakpoint looks back at most 20 positions for a prior cache entry.
- A cache entry becomes readable only after the first response begins streaming; parallel identical requests all pay full price.
- Mid-conversation `role: "system"` messages (supported on Opus 5, Opus 4.8, Fable 5/5.1) let an operator inject instructions after the cached prefix without invalidating it. Turn-scoped messages with `clear_at: "next_user_message"` exist in beta for per-turn reminders and must never be deleted from history.
- Preserved thinking: on Claude Fable 5.1, a thinking block's signature binds the conversation prefix that produced it. Editing an earlier turn invalidates every later block. Organizations created on or after 2026-08-31 receive a 400 on mismatch; older organizations record the mismatch and are enforced on future models. Server-side compaction and context editing do not count as edits. Tools and frameworks that run under users' own keys are advised to test with `prefix_mismatch_behavior` set, because their users on new organizations are enforced first.
- Thinking blocks must be passed back unchanged; stripping or modifying them can trigger signature errors.
- Token counts should come from the `count_tokens` endpoint, not from byte-length heuristics.
- The correct cache-efficiency measure is derived from `usage`: `cache_read_input_tokens / (input_tokens + cache_creation_input_tokens + cache_read_input_tokens)`.
