# Product Requirements: Project Buffy v5.0.0

The execution-ready requirements for Buffy, a local tool that shows developers where their coding agent's prompt cache broke, what it cost, what a different context layout would have saved, and then applies the better layout live. This version supersedes v4.0.0 and incorporates every constraint from the two adversarial reviews.

| | |
|---|---|
| **Version** | 5.0.0 |
| **Status** | Draft, awaiting owner approval |
| **Date** | 2026-09-02 |
| **Owner** | RedRobotKK (single maintainer) |
| **Supersedes** | `buffy-prd-v4.0.0.md` (kept as history, not edited) |
| **Inputs** | `../reviews/PRD-v4-adversarial-review.md`, `../reviews/solution-red-blue-review-2026-09-02.md` |
| **Decisions recorded** | ADR-0001 through ADR-0005 in `../adr/` |

---

## 1. Summary

Coding agents resend the entire conversation on every turn. The provider caches the unchanged prefix and charges a fraction for it, but the cache is an exact-byte prefix match, so one changed byte early in the prompt silently re-bills everything after it. Developers see the invoice, never the cause.

Every Claude Code session already sits on disk as a transcript with exact per-turn usage. Buffy reads those transcripts and reproduces the provider's caching behavior turn by turn. Once its model of the session matches what the provider actually charged, it can answer three questions nobody can answer today:

1. **Where did the cache break, and what did that turn cost?** (`buffy diff`)
2. **Which file, tool description, or instruction is eating the most tokens across all my sessions?** (`buffy blame`)
3. **What would a different context layout have cost on the sessions I already paid for?** (`buffy replay`)

Then, as a local proxy, it applies the best layout on new sessions using only mechanisms the provider itself sanctions, measures the result, and keeps improving from the user's own history. Everything runs on the developer's machine. Nothing learns across users. No API calls are spent on analysis.

**Positioning line:** Buffy shows you the turn your agent's cache broke, what it cost, and what a better layout would have saved, on sessions you have already paid for.

## 2. Problem and evidence

Three failures, all silent:

- **Cost is invisible until the invoice.** A single agent task can cost more than a day of a hosted CI runner, and nothing in the workflow says so at the time.
- **Cache breaks are silent.** A timestamp in a system prompt, a reordered tool list, a tool result cleared and re-added, or a breakpoint pushed out of the lookback window each doubles the bill with no error anywhere.
- **Nobody can test a context strategy.** Advice about context layout is folklore. There is no way to ask "what would this have cost under a different layout" without spending money to find out.

**Evidence requirement.** Before public announcement, this document is amended with three real session traces (redacted) showing per-turn usage, the cache-break turn, and the replayed saving. Until then, no percentage savings figure appears anywhere in the repository.

## 3. Goals and non-goals

### Goals

1. A developer runs one command on existing transcripts and learns something true and actionable about their own spend within thirty seconds.
2. Every number Buffy shows is either measured from the wire or labeled as an estimate, with the calibration that justifies it.
3. Installing the proxy changes nothing about agent behavior until the user turns a policy on.
4. No policy Buffy applies can trip a provider history-binding check or corrupt a session.
5. Secrets in prompts stay on the machine, and the mechanism that keeps them there cannot be turned into an exfiltration path.
6. The tool improves from the user's own history without spending API calls and without a network connection.

### Non-goals

- Translating between provider API shapes. Buffy never re-renders a prompt from one format to another.
- Replacing the provider's server-side compaction or context editing. Buffy configures them; it does not reimplement them.
- Multi-agent messaging, vector search, or a virtual filesystem. Removed from scope; see ADR-0001.
- Cross-user learning, hosted dashboards, or any server component.
- Supporting clients that offer no base URL override (Cursor's agent mode).

## 4. Users

| User | Situation | What they need from Buffy |
|------|-----------|---------------------------|
| Solo developer on Claude Code | Pays per token or holds a subscription; has weeks of transcripts | One command that explains their bill and one change that lowers it |
| Team lead with an agent budget | Ten developers, one invoice, no attribution | Per-project blame and a policy that is safe to roll out |
| Platform or security engineer | Must approve any tool that touches API traffic | A threat model, a data-handling statement, and a binary they can verify |

**Non-user:** anyone whose agent cannot be pointed at a local base URL and who does not keep local transcripts. Buffy has nothing to offer them and the README says so.

## 5. Product principles (non-negotiable)

These bind every requirement below and every future change. They are restated in the repository `CLAUDE.md` so agents and contributors work under the same rules.

1. **Transparent by default.** The proxy forwards bytes unchanged until a policy is explicitly enabled. One environment variable disables everything.
2. **Deterministic rendering.** Every transformation is a pure function of its input. The same client message renders to the same provider bytes for the life of a session.
3. **Append-only history.** Buffy never edits an earlier turn client-side. Context reduction is requested from the provider through request parameters, which the provider excludes from its history-binding check.
4. **Never touch thinking blocks, signatures, or cache markers the client placed.**
5. **Two tiers of truth.** Numbers derived from transcripts are labeled *estimated*. Numbers captured on the wire are labeled *measured*. They never share a table without the label.
6. **State the assumption.** Every replayed saving carries the note that it assumes unchanged agent behavior. Behavior effects are measured only in live trials.
7. **No credentials, no raw content.** Buffy forwards the client's authentication headers unchanged and never persists them. The ledger stores derived data unless the user opts into raw storage.
8. **Secrets stay local, and rehydration is scoped.** Placeholders never rehydrate into shell or network tool inputs by default.
9. **Learning never touches bytes.** Learning writes a policy file. The proxy reads it at session start. The policy is a pure function.
10. **Offline and free.** No analysis step calls the provider API. No telemetry leaves the machine, ever.
11. **Honest status.** The README describes what the code does today, not what is planned.

## 6. Product surface

Four commands. Errors, suggestions, and calibration are sections of their output, not additional commands.

| Command | Purpose | Tier |
|---------|---------|------|
| `buffy replay <path>` | Reproduce as-run caching for recorded sessions, then score alternative layout policies | Estimated from transcripts; measured once the proxy has recorded the session |
| `buffy blame <path>` | Rank the sources of prompt tokens across sessions: files, tool results, tool descriptions, instructions | Same |
| `buffy diff <session> [turn]` | Locate the turn and the content where the cached prefix diverged, and classify the cause | Same |
| `buffy serve` | Local proxy: passthrough, usage capture, policy application, dry-run, spend guards | Measured |

### 6.1 Output contract

Every table produced by `replay`, `blame`, and `diff` carries three lines that cannot be suppressed:

```text
Tier: estimated (transcripts only)  |  measured (proxy-recorded)
Calibration: reproduced provider cache reads on 298/312 turns (14 unexplained; listed below)
Assumption: replayed savings assume the agent would have behaved identically under the alternative layout
```

If calibration falls below the threshold in Section 8.2, `replay` prints the calibration report and refuses to score alternatives for that model.

### 6.2 Reference output

```text
$ buffy replay ~/.claude/projects/my-app/

Tier: estimated (transcripts only)
Calibration: reproduced provider cache reads on 312/312 turns
Assumption: replayed savings assume unchanged agent behavior
Prices: anthropic rules 2026-09-01, price table 2026-09-01

  policy                             prompt tokens   cached share   vs as-run   guardrail
  as-run                                 4.21M           61%            -
  freeze-system-prompt                   3.02M           78%         -28%      none
  + context-edit tool results (age>8)    2.31M           84%         -45%      re-read rate: unknown until live trial
  + breakpoint-every-12                  2.14M           86%         -49%      none

  cost of errors (as-run)                 480k tokens (11%)
  1. 14 failed edits, anchor not found    310k   -> 9 followed a stale file read
  2. 3 repeated identical commands        120k
  3. 2 context overflows                   50k
  provider retries: not visible in transcripts; run the proxy to capture

  top token sources
  1. tool result: Read src/legacy/schema.ts (x19)      estimated 610k  ±90k
  2. tool descriptions (38 tools, every turn)           estimated 480k  ±120k
  3. CLAUDE.md                                          estimated 210k  ±30k
```

Token counts, never dollars, are the primary unit. A dollar column is available behind a flag and always cites the price table date.

## 7. Architecture

Five layers. Only the last one touches bytes on the wire, and it never learns.

```text
 transcripts (client)  ──►  1. session records (ledger)  ◄──  proxy captures
                                      │
                                      ▼
                            2. replay engine  (rules per provider, calibration gate)
                                      │
                                      ▼
                            3. policy engine  (validation, session types, dry-run, trials)
                                      │
                                      ▼
                            4. advisor        (suggestions, predicted vs realized)
                                      │  writes policy file
                                      ▼
 agent ──► 5. proxy (serve) ──► provider       reads policy file at session start only
```

Single Go binary. No sidecar processes. Tree-sitter, if ever required for token attribution inside source files, is used through cgo; the sidecar design from v4 is withdrawn (ADR-0001).

## 8. Functional requirements

Each requirement has an identifier and an acceptance test. A requirement without a test is not done.

### 8.1 Session records (ledger)

| ID | Requirement | Acceptance |
|----|-------------|------------|
| LG-1 | Ingest Claude Code transcripts and proxy captures into one schema: per turn, the block structure, content hashes, byte lengths, labels (file path, tool name), timestamps, and the provider usage object. | Ingest 20 real sessions; every usage field round-trips. |
| LG-2 | Store derived data only by default: hashes, lengths, labels, usage. No message bodies, and no tool arguments in labels: tool names only, with file paths replaced by a keyed hash that keeps the extension. | Grep the ledger for a known string from a session, including one planted inside a command argument; zero hits. |
| LG-3 | Optional raw mode stores bodies encrypted at rest with a key from the OS keychain, with a retention period and a `buffy purge` command. | Purge removes all raw data; encrypted files are unreadable without the keychain. |
| LG-4 | Ledger files are owner-only permissions under the user's home directory. | Permission check in the test suite. |

### 8.2 Replay engine

| ID | Requirement | Acceptance |
|----|-------------|------------|
| RE-1 | A rules module per provider encodes: prefix matching, breakpoint semantics, minimum cacheable size per model, lookback window, TTL by timestamp, write and read multipliers. Rules and price tables are versioned and dated and printed in every output. | Rules module has a unit test per rule with the documented values. |
| RE-2 | Calibration: reproduce the provider's reported cache reads and cache writes for every turn of a recorded session before scoring alternatives. | Match on at least 95 percent of turns across a 20-session corpus; every mismatch is listed with a reason. |
| RE-3 | If calibration is below threshold for a model, refuse alternatives for that model and print the calibration report. | Test with a corrupted session. |
| RE-4 | Unknown prefix (system prompt, tool definitions, client markers not present in transcripts) is modeled as an opaque block whose size is inferred from turn-one usage, and every figure that depends on it is labeled estimated. | Estimated label present whenever transcript-only data is used. |
| RE-5 | Alternative policies are scored on prompt-side tokens only, with the assumption line. | Assumption line present on every table. |
| RE-6 | Anthropic rules first. The OpenAI-compatible rules module is a separate deliverable and the README says which providers are supported. | README provider table. |

### 8.3 Blame

| ID | Requirement | Acceptance |
|----|-------------|------------|
| BL-1 | Attribute prompt tokens per turn to blocks by proportional byte share scaled to the turn's reported total, and aggregate by label across sessions. | Sum of attributions equals reported totals per turn. |
| BL-2 | Estimated attributions carry an uncertainty range derived from the byte-to-token variance observed during calibration. | Range printed; widens for code and JSON blocks. |
| BL-3 | With proxy captures, attribution uses exact block boundaries and the measured label. | Measured label on proxy-recorded sessions. |

### 8.4 Diff

| ID | Requirement | Acceptance |
|----|-------------|------------|
| DF-1 | For consecutive requests, find the first position where the cached prefix diverged and print the block, its label, and the turn's cache-write cost. | Fixture sessions with planted breaks at known turns. |
| DF-2 | Classify the cause: changed system prompt, changed tool set, edited earlier turn, breakpoint outside lookback, TTL expiry, model change, parameter change (thinking, effort), unknown. | One fixture per class. |
| DF-3 | On transcripts alone, causes in the unseen prefix are reported as "system prompt or tools changed (not visible in transcripts)". | Fixture with a planted turn-one usage jump. |

### 8.5 Error cost ledger

| ID | Requirement | Acceptance |
|----|-------------|------------|
| ER-1 | Classify and price: tool results flagged as errors, failed edits (anchor not found), repeated identical tool calls, context overflow and compaction, provider errors (proxy only). | One fixture per class; totals match. |
| ER-2 | Link failed edits to a preceding stale read of the same file when present. | Fixture with a clear-then-edit sequence. |
| ER-3 | Errors appear as a section in `replay` and `blame` and as a guardrail column in policy tables. | Output contract test. |

### 8.6 Proxy

| ID | Requirement | Acceptance |
|----|-------------|------------|
| PX-1 | Loopback TCP listener. Exposes `/v1/messages*` including `count_tokens`, and `/v1/chat/completions`. A shared header token is optional: loopback binding, browser-origin rejection, and the absence of stored credentials are the default protection, because requiring the token would force every user to configure custom headers before first contact. | Integration test with recorded fixtures for each; token test when set. |
| PX-2 | Byte-for-byte passthrough of request and response, including SSE streaming with immediate flush, when no policy is enabled. | Hash of forwarded bytes equals hash of received bytes across the fixture corpus. |
| PX-3 | Authentication headers are forwarded unchanged and never persisted or logged, including in debug mode. | Log scan test with a canary header value. |
| PX-4 | Timeouts sized for multi-minute turns. Retries (only on rate limit, overload, server error, and connection failure, bounded, jittered, honoring retry-after; never on client errors; never after a response has started streaming) ship with the v0.3 guards; in v0.2 the client's own retry policy applies and every provider response passes through unchanged. | Fault-injection tests per case. |
| PX-5 | Fail open: any failure inside Buffy's own analysis or policy code results in the original bytes being forwarded. The spend cap is the single fail-closed exception. | Panic injection test. |
| PX-6 | One environment variable disables all policies; a second disables Buffy entirely with a clear client-visible error that names the bypass. | Documented and tested. |
| PX-7 | Session identity comes from the client's `x-claude-code-session-id` header when present (with `x-claude-code-agent-id` distinguishing sub-agents), and otherwise from a hash of the stable prefix. When identity is uncertain, no policy is applied. | Fixture with two interleaved sessions, with and without the header. |
| PX-8 | The policy for a session is chosen at its first request and pinned for the session's life, persisted to disk. **Status:** implemented (`--policy-file`, pins under the ledger directory); tested across a file rewrite and a proxy restart. | Change the policy file mid-session; the session continues under the pinned policy. |
| PX-9 | Client-set request parameters and cache markers are never overridden. Buffy adds only what the client did not set, within provider limits (marker count, TTL ordering). The `anthropic-version` and `anthropic-beta` headers are forwarded verbatim, never filtered. | Fixture with four client markers; Buffy adds none. Header round-trip test. |
| PX-10 | Every applied transformation is logged with the before and after content hashes, and `buffy diff` can show the exact bytes. | Log format test. |
| PX-11 | Server-side compaction is never enabled from the proxy. | Static check: parameter not present in the policy catalog. |

### 8.7 Policy catalog

A policy is admissible only if it is one of three kinds. Anything else is rejected at design time.

| Kind | Mechanism | Examples |
|------|-----------|----------|
| Request parameter | Set a provider parameter the client left unset | Context editing for tool results older than N; cache breakpoint on the last stable block; TTL choice for a breakpoint |
| Append-only addition | Add content after the cached prefix, never remove it later | Mid-conversation system message where the model supports it; turn-scoped reminders |
| Deterministic per-message transformation | A pure function applied identically to a message every time it is seen | Secret masking with HMAC-derived placeholders |

Initial catalog for v0.3: `freeze-system-prompt` (diagnostic only, reports when the client changes it), `context-edit-tool-results`, `breakpoint-on-stable-block`, `hold-parallel-siblings` (delay identical parallel prefixes until the first response begins streaming). Each entry documents its provider mechanism, its parameters, and its guardrail metric.

### 8.8 Dry-run

| ID | Requirement | Acceptance |
|----|-------------|------------|
| DR-1 | The proxy renders a candidate policy alongside the active one on every turn, sends only the active one, and scores the candidate against the observed usage. | Candidate never appears on the wire; scores recorded. |
| DR-2 | A candidate graduates only after a configurable number of sessions in which predicted savings held within a tolerance. | Graduation test with synthetic sessions. |

### 8.9 Learning

| ID | Requirement | Acceptance |
|----|-------------|------------|
| LN-1 | A nightly local job re-scores the catalog over all recorded sessions and writes a versioned, human-readable policy file. No network access. | Job runs with networking disabled. |
| LN-2 | Selection uses held-out sessions, a minimum sample size, and a required margin above noise; a win must repeat across sessions; ties go to the simpler policy. | Statistical test on synthetic corpora with a known best policy and a decoy. |
| LN-3 | Session types (exploration, edit-heavy, long-running, model) are classified from early-turn signals with one policy per type. **Status:** implemented on the two signals a proxy has at a first request, model family and first-prompt size; edit-heavy and long-running are not early signals and are not types. | Classifier fixtures. |
| LN-4 | Parameter grids are coarse and bounded; the sweep reports confidence, not a single point. | Output includes intervals. |
| LN-5 | Live trials of graduated policies run on a bounded share of sessions, never on sessions the user marked important, with automatic revert when a guardrail metric (failed-edit rate, re-read rate) exceeds its threshold. **Status:** implemented (`--trial-share`, `--guardrail-reread`, `--revert-after`) on the re-read rate; marking a session important has no client mechanism yet and is not built. | Revert test. |
| LN-6 | Primary success metric is the cached share of the prompt per turn, which is scale-free. Absolute totals are secondary. | Metric definition test. |

### 8.10 Advisor

| ID | Requirement | Acceptance |
|----|-------------|------------|
| AD-1 | Convert top token sources into concrete suggestions with a predicted saving: defer-load rarely used tools, add a summary header to a frequently read file, split an instruction file. **Status:** implemented (`buffy advise`), plus dominant tool inputs, oversized results, and cache breaks. | Fixture produces the three suggestion types. |
| AD-2 | Track each suggestion to closure: pending, applied (detected from subsequent sessions), verified or not verified against the prediction. **Status:** implemented for kinds whose target is structural; hot files and cache breaks are advice only. | State machine test. |
| AD-3 | Realized savings are reported on the scale-free metric first. **Status:** implemented; shares of prompt tokens lead, corpus tokens follow. | Output contract test. |

### 8.11 Secret masking

| ID | Requirement | Acceptance |
|----|-------------|------------|
| MK-1 | Detection uses a maintained pattern set for known key formats, user-defined patterns, and an optional entropy heuristic. The README names the patterns; it never says "all secrets". | Corpus test with precision and recall reported. |
| MK-2 | Placeholders are derived by HMAC of the secret under a per-project key, so the same secret always maps to the same placeholder. | Determinism test across sessions. |
| MK-3 | The vault persists encrypted at rest with a key from the OS keychain; a daemon restart loses nothing. | Restart test mid-session. |
| MK-4 | Rehydration works across SSE chunk boundaries and inside tool-call JSON with escaping awareness. | Fixtures with placeholders split across chunks and inside JSON strings. |
| MK-5 | Scoped rehydration: by default, placeholders rehydrate into assistant text and into file-edit tool inputs targeting paths inside the project, never into shell or network tool inputs. Scope is configurable per pattern. | Adversarial corpus: injected placeholders in tool results never reach a shell or network tool input. |
| MK-6 | Every rehydration is logged with its destination. Thinking blocks are never modified in either direction. | Log test; thinking passthrough test. |
| MK-7 | A per-session report lists what was masked so the user can verify coverage. | Report test. |

### 8.12 Spend and loop guards

| ID | Requirement | Acceptance |
|----|-------------|------------|
| SP-1 | Token and dollar caps per session and per day computed from provider usage fields; fail closed before the next request; never mid-stream; user override with a logged reason. **Status:** implemented: token caps (`--max-session-tokens`, `--max-day-tokens`) and list-price dollar caps (`--max-session-usd`, `--max-day-usd`) with `x-buffy-override`. | Cap test with streaming in progress. |
| SP-2 | Loop detection: the same tool call with the same input N times triggers a warning to the client and, above a second threshold, a block. **Status:** implemented (`--loop-warn`, `--loop-block`, `x-buffy-warning` response header). | Fixture. |
| SP-3 | Provider circuit breaker: sustained rate-limit or overload responses open the breaker for a cooling period so the agent stops burning retries. **Status:** implemented (`--breaker-failures`, `--breaker-cooldown`), answering locally with `Retry-After` and one probe after cooldown. | Fault-injection test. |
| SP-4 | An error budget per session trips before the dollar cap when error cost exceeds a share of spend. **Status:** implemented (`--error-budget <share>`), judged from the same error classification `replay` prints, on sessions above a minimum size, with the override header. | Fixture. |

### 8.13 Staleness detection

| ID | Requirement | Acceptance |
|----|-------------|------------|
| ST-1 | When calibration for a model drops below threshold on new sessions, Buffy reports that the provider's behavior changed, stops scoring alternatives for that model, and attempts to refit the parameters it can infer (minimum cacheable size, effective lookback). | Simulated rule change test. |
| ST-2 | Rules it cannot infer remain in the versioned rules file with a documented update process. | Documentation. |

## 9. Client and platform support

| Client | Hook | v0.1 (replay) | v0.2+ (proxy) |
|--------|------|---------------|---------------|
| Claude Code, API key | `ANTHROPIC_BASE_URL` | Yes (transcripts) | Yes |
| Claude Code, subscription | `ANTHROPIC_BASE_URL` without a gateway credential | Yes (transcripts) | Yes, documented (spike 3 passed) |
| Aider | Base URL variables | Transcript format pending | Yes |
| OpenAI Codex CLI | Base URL variable | No transcripts known | Chat completions passthrough; caching rules later |
| Cursor agent mode | None | No | No, and the README says so |

| Platform | Status |
|----------|--------|
| macOS (arm64, amd64) | Supported from v0.1 |
| Linux (amd64, arm64) | Supported from v0.1 |
| Windows | Replay from v0.1; proxy verified in CI from v0.2 |
| Devcontainers and WSL | Loopback TCP with token; documented networking pattern from v0.2 |

## 10. Security

### 10.1 Threat model

**Assets:** provider credentials in transit; conversation content, which includes source code and secrets; the masking vault; the ledger; the policy file; the binary itself.

**Adversaries:** untrusted content that the agent reads (files in a cloned repository, web pages, tool output) and that can instruct the model; a malicious web page on the same machine; a compromised dependency or release channel; another user on a shared machine. The provider is trusted with what the client already sends it; Buffy's job is to send it less, never more.

**Trust boundaries:** the loopback listener; the ledger on disk; the rehydration point where a placeholder becomes a secret again.

### 10.2 Controls

| Threat | Control | Requirement |
|--------|---------|-------------|
| Placeholder forgery leading to exfiltration | Scoped rehydration with default deny for shell and network tools; rehydration log | MK-5, MK-6 |
| Credential theft from Buffy | No credentials persisted or logged | PX-3 |
| Second copy of transcripts at rest | Derived-data ledger; raw mode opt-in, encrypted, purgeable | LG-2, LG-3 |
| Local web page reaching the listener | No stored credentials; token on every read; Host and Origin checks; loopback only | PX-1, dashboard requirements when one ships |
| Stored script in tool output rendered by a dashboard | Terminal-only output until a dashboard ships; then strict escaping and a content security policy with no inline script | Dashboard requirements |
| Supply chain | Minimal pinned dependencies with checksum verification; signed releases; software bill of materials; reproducible builds; no auto-update | Release requirements |
| Memory disclosure | No core dumps; no body logging; redacting debug logs | PX-3 |
| Shared machine | Owner-only files; loopback with token | LG-4 |

### 10.3 What Buffy never does

Stores or logs credentials. Sends anything anywhere except the provider request the client asked for. Learns across users. Edits an earlier turn. Touches thinking blocks or signatures. Enables server-side compaction on the client's behalf. Rehydrates a secret into a shell or network tool input by default. Auto-updates itself.

### 10.4 Disclosure

Coordinated disclosure per `SECURITY.md`. An external security review is scheduled before the first 1.0 release and its report is published in `docs/reviews/`.

## 11. Privacy and data handling

- All data stays on the machine. There is no telemetry, no crash reporting, and no update check. The README states this in its first screen.
- The ledger holds derived data by default. Raw content is opt-in, encrypted, retention-limited, and purgeable.
- Placeholders are keyed per project, which limits the provider's ability to correlate a secret across projects. This is documented as a known limitation, not hidden.
- A `buffy purge` command removes everything Buffy wrote.

## 12. Observability

Metrics are computed from the provider's usage object, never inferred from bytes, and exposed locally as a Prometheus text endpoint behind the token. **Status:** `/buffy/metrics` and `/buffy/status` are implemented for the request, token, cache-break, upstream-error, refusal, and latency metrics; policy and rehydration counters arrive with those features.

| Metric | Definition |
|--------|------------|
| `buffy_cached_share` | `cache_read / (input + cache_creation + cache_read)` per request |
| `buffy_prompt_tokens_total` | Sum of the three usage fields per session |
| `buffy_cache_break_total` | Count of requests where the diff classifier found a divergence, by cause |
| `buffy_error_cost_tokens` | Tokens attributed to the error classes in Section 8.5 |
| `buffy_added_latency_seconds` | Time between receiving the last request byte and forwarding it, p50 and p99 |
| `buffy_upstream_errors_total` | By status class |
| `buffy_policy_applied_total` | By policy name and session type |
| `buffy_rehydration_total` | By destination kind |

Logs redact bodies and headers. A debug mode that logs bodies requires an explicit flag, prints a warning, and still redacts credentials.

## 13. Verification

### 13.1 Gating spikes

No public claim is made until all five pass. Spike 3 runs first because its answer changes the plan most.

| Spike | Question | Pass condition |
|-------|----------|----------------|
| 1 | Do Claude Code transcripts carry per-message usage with cache read and write counts? | Present on 20 real sessions across two client versions. |
| 2 | Does the replay engine reproduce as-run cache reads and writes? | At least 95 percent of turns across 20 sessions, mismatches explained. |
| 3 | Does Claude Code honor a base URL override under subscription authentication, and is proxying that traffic within the provider's terms? | Documented answer with a source. **Passed:** the LLM gateway documentation describes exactly this configuration; see `../architecture/proxy-protocol.md`. |
| 4 | Does adding the context-editing parameter from a proxy leave Claude Code's behavior intact? | A ten-turn session completes with the parameter present. |
| 5 | Does scoped rehydration hold under adversarial content? | Adversarial corpus never reaches a shell or network tool input. |

### 13.2 Test strategy

- Unit tests for every rule in the rules module with the documented values.
- Fixture corpus of recorded requests, responses, and SSE streams; passthrough tests hash bytes in and out.
- Provider history-binding check run in CI in error mode against a policy-applied session, so any edit to earlier history fails the build.
- Fault injection for every retry, timeout, and fail-open path.
- Adversarial corpus for rehydration.
- Determinism test: render the same session twice from a cold start; outputs are byte-identical.

### 13.3 Benchmark

An A/B harness runs a fixed set of real agent tasks with Buffy off, on with passthrough, and on with a policy, and reports task success, failed-edit count, prompt tokens, cached share, and wall-clock. It runs on demand under a hard spend cap, never on a schedule that spends money unattended. Its results page is the only place a savings percentage may appear, and each figure links to its run.

## 14. Release plan

Sequenced so that the first release costs nothing to run, works for every user regardless of spike 3, and produces the screenshot that gets the project noticed.

| Release | Scope | Gate |
|---------|-------|------|
| v0.1 | `replay`, `blame`, `diff` on transcripts. Estimated tier only. Anthropic rules. macOS and Linux and Windows. | Spikes 1 and 2 pass; calibration line on every output; README shows real output from the maintainer's own sessions. |
| v0.2 | `serve` passthrough with usage capture; measured tier; live `diff`. | Spike 3 answered; passthrough hash test green on the fixture corpus; added latency p99 published. |
| v0.3 | Policy catalog, dry-run, spend and loop guards, error guards. | Spike 4 passes; history-binding check green in CI; guardrail revert tested. |
| v0.4 | Learning job, session types, live trials, advisor. | Synthetic-corpus selection test passes; policy file documented. |
| v0.5 | Secret masking with scoped rehydration and persistent vault. | Spike 5 passes; precision and recall published for the pattern set. |
| 1.0 | External security review published; signed reproducible releases; provider rules for a second provider. | Review findings closed. |

Masking ships last on purpose: it is the feature with the highest consequence of a mistake, and it depends on the deterministic rendering and history-binding test harness that v0.2 and v0.3 establish.

## 15. Repository and community practices

These are in force now and are described in `../HOUSEKEEPING.md` and `CONTRIBUTING.md`.

- `main` is protected: pull requests only, CI green, one review, linear history. Conventional Commits; squash merge; small single-purpose changes.
- CI runs vet, race tests, build on Linux, macOS, and Windows, golangci-lint, and markdownlint on every pull request.
- Every design change lands with an ADR. Every user-visible change lands with a changelog entry.
- Historical documents under `docs/prd/` and `docs/reviews/` are never edited; a new version is added instead.
- Issue and pull request templates, a canonical label set, weekly stale automation, Dependabot, CODEOWNERS.
- Security reports go through private advisories, never public issues.
- The README states status truthfully and names the maintainer's availability for work.

## 16. Licensing

Apache License 2.0, decided by the owner on 2026-09-02 and recorded in ADR-0005. Any future commercial tier is a separate work. The README calls the project open source, which is now true.

## 17. Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Transcripts lack the fields replay needs (spike 1 fails) | Low | High | Proxy-recorded sessions become the only measured source; v0.1 becomes a smaller diagnostic |
| Subscription Claude Code cannot be proxied (spike 3 fails) | Medium | High | v0.1 still serves everyone; proxy targets API-key users; say so plainly |
| Calibration cannot reach threshold because client breakpoint placement is unobservable | Medium | High | Fit breakpoint placement per client version during calibration; label estimated; prefer proxy data |
| Estimated figures debunked publicly | Medium | High | Two-tier labels, calibration line, no savings percentage outside the benchmark page |
| Provider changes caching rules | Certain over time | Medium | Versioned rules, staleness detection, refusal to score |
| Provider absorbs the diagnostic natively | Medium | Medium | Cross-client, local what-if and advisor remain; the goal is reach, not moat |
| Single maintainer availability | High | High | Small releases; every step leaves a usable tool; documentation good enough for a second contributor |
| Masking false negatives create false confidence | Medium | High | Named pattern set, per-session masked report, never claim completeness |

## 18. Decisions required from the owner

1. ~~License for the first public release.~~ Decided: Apache 2.0 (ADR-0005).
2. Approval of the release sequence in Section 14, in particular replay before proxy and masking last.
3. ~~A real transcript sample for spikes 1 and 2.~~ One session confirmed both; the 20-session corpus is still needed.
4. Whether the "looking for work" line appears in the README from day one.

## 19. Open questions

- Which Claude Code versions write which transcript fields, and how far back the format is stable.
- Whether Aider transcripts contain usage in a form replay can use.
- Exact behavior of the provider's context-editing parameter when the client already sends its own edits.
- Placement rules for the OpenAI-compatible caching module.

## 20. Glossary

| Term | Meaning |
|------|---------|
| Prefix | The rendered prompt bytes from the start of the request up to a cache breakpoint |
| Cache break | A request whose prefix diverged from the previous request's, causing a cache write instead of a read |
| Calibration | Reproducing the provider's reported cache reads and writes for a recorded session before scoring alternatives |
| Policy | A deterministic layout decision applied by the proxy through a provider-sanctioned mechanism |
| Estimated / measured | Truth tiers: derived from transcripts alone, or captured on the wire |
| Dry-run | Rendering a candidate policy alongside the active one without sending it |
| Placeholder | The HMAC-derived token that replaces a secret before egress |
| Rehydration | Replacing a placeholder with the original secret in the response, within scope |

## Appendix A. Provider facts this design depends on

Verified on the review date against current provider documentation. Rules are versioned in the rules module; this list is the human-readable summary.

- Prompt caching is an exact-byte prefix match up to each breakpoint. Render order is tools, then system, then messages.
- Maximum four breakpoints per request. Minimum cacheable prefix is model-dependent, from 512 to 4096 tokens. Shorter prefixes silently do not cache.
- Cache writes cost 1.25x base input for the 5-minute TTL and 2x for the 1-hour TTL. Reads cost about 0.1x, lower on some models.
- Each breakpoint looks back at most 20 positions for a prior cache entry.
- An entry becomes readable only after the first response begins streaming.
- Mid-conversation system messages append instructions after the cached prefix without invalidating it, on the models that support them.
- Editing an earlier turn invalidates every later thinking block on the newest models; organizations created on or after 2026-08-31 receive a 400. Server-side compaction and context editing are excluded from that check. Adding, moving, or removing cache markers is also excluded.
- Thinking blocks must be passed back unchanged.
- Token counts come from the provider's counting endpoint or from reported usage, never from byte-length heuristics presented as exact.
