# Product Requirements: Project Replay v5.0.0

The execution-ready requirements for Replay, a local tool that shows developers where their coding agent's prompt cache broke, what it cost, what a different context layout would have saved, and then applies the better layout live. This version supersedes v4.0.0 and incorporates every constraint from the two adversarial reviews.

| Field | Value |
|---|---|
| **Version** | 5.0.0 |
| **Status** | Current. Build status is recorded per requirement below |
| **Date** | 2026-09-02 |
| **Owner** | RedRobotKK (single maintainer) |
| **Supersedes** | `replay-prd-v4.0.0.md`, which is in the git history rather than the working tree, where a superseded document cannot be mistaken for a current one |
| **Inputs** | Two adversarial reviews, `PRD-v4-adversarial-review.md` and `solution-red-blue-review-2026-09-02.md`, both in the git history. The surviving one in the working tree is [Design review, 2026-09-02](design-review-2026-09-02.md) |
| **Decisions recorded** | ADR-0001 through ADR-0005 in [`adr/`](adr/README.md) |

---

## 1. Summary

Coding agents resend the entire conversation on every turn. The provider caches the unchanged prefix and charges a fraction for it, but the cache is an exact-byte prefix match, so one changed byte early in the prompt silently re-bills everything after it. Developers see the invoice, never the cause.

Every Claude Code session already sits on disk as a transcript with exact per-turn usage. Replay reads those transcripts and reproduces the provider's caching behavior turn by turn. Once its model of the session matches what the provider actually charged, it can answer three questions nobody can answer today:

1. **Where did the cache break, and what did that turn cost?** (`replay diff`)
2. **Which file, tool description, or instruction is eating the most tokens across all my sessions?** (`replay blame`)
3. **What would a different context layout have cost on the sessions I already paid for?** (`replay replay`)

Then, as a local proxy, it applies the best layout on new sessions using only mechanisms the provider itself sanctions, measures the result, and keeps improving from the user's own history. Everything runs on the developer's machine. Nothing learns across users. No API calls are spent on analysis.

**Positioning line:** Replay shows you the turn your agent's cache broke, what it cost, and what a better layout would have saved, on sessions you have already paid for.

## 2. Problem and evidence

Three failures, all silent:

- **Cost is invisible until the invoice.** A single agent task can cost more than a day of a hosted CI runner, and nothing in the workflow says so at the time.
- **Cache breaks are silent.** A timestamp in a system prompt, a reordered tool list, a tool result cleared and re-added, or a breakpoint pushed out of the lookback window each doubles the bill with no error anywhere.
- **Nobody can test a context strategy.** Advice about context layout is folklore. There is no way to ask "what would this have cost under a different layout" without spending money to find out.

**Evidence requirement.** Before public announcement, this document is amended with three real session traces (redacted) showing per-turn usage, the cache-break turn, and the replayed saving. Until then, no percentage savings figure appears anywhere in the repository.

## 3. Goals and non-goals

### Goals

1. A developer runs one command on existing transcripts and learns something true and actionable about their own spend within thirty seconds.
2. Every number Replay shows is either measured from the wire or labeled as an estimate, with the calibration that justifies it.
3. Installing the proxy changes nothing about agent behavior until the user turns a policy on.
4. No policy Replay applies can trip a provider history-binding check or corrupt a session.
5. Secrets in prompts stay on the machine, and the mechanism that keeps them there cannot be turned into an exfiltration path.
6. The tool improves from the user's own history without spending API calls and without a network connection.

### Non-goals

- Translating between provider API shapes. Replay never re-renders a prompt from one format to another.
- Replacing the provider's server-side compaction or context editing. Replay configures them; it does not reimplement them.
- Multi-agent messaging, vector search, or a virtual filesystem. Removed from scope; see ADR-0001.
- Cross-user learning, hosted dashboards, or any server component.
- Supporting clients that offer no base URL override (Cursor's agent mode).

## 4. Users

| User | Situation | What they need from Replay |
|------|-----------|---------------------------|
| Solo developer on Claude Code | Pays per token or holds a subscription; has weeks of transcripts | One command that explains their bill and one change that lowers it |
| Team lead with an agent budget | Ten developers, one invoice, no attribution | Per-project blame and a policy that is safe to roll out |
| Platform or security engineer | Must approve any tool that touches API traffic | A threat model, a data-handling statement, and a binary they can verify |
| Operator running inference against a live position | Trading, market-making, or any loop where the model's output is acted on for money | The **total** cost of inference for a window, not a rate — because it is subtracted from a P&L, not compared with a peer |

**Non-user:** anyone whose agent cannot be pointed at a local base URL and who does not keep local transcripts. Replay has nothing to offer them and the README says so.

### The last row wants the number the share card refuses

This is worth stating rather than smoothing over, because the two surfaces
answer opposite questions and the tension is by design.

`replay cost` reports the total. It runs locally, prints to that operator's own
terminal, and the total is the entire point: a trading loop's inference cost is
a line item against realised P&L, and a median task cost cannot be subtracted
from anything.

The share card omits the total on purpose, for the reason in
`cmd/replay/share.go`: it is built to be pasted in public, and a total tells a
reader the poster's burn rate. That reasoning does not weaken for a trader — it
strengthens. Position size is inferable from spend, and an operator who posts
their monthly inference bill has published something about the size of the book
behind it.

So the card carries the **route**, not the total: whether the traffic went
first-party, through Bedrock, or through Vertex. That is a category rather than
a quantity, it reveals nothing about the poster, and for the metered routes it
is the one billing fact a model id actually settles. See `cmd/replay/namespace.go`
for why the first-party case deliberately claims less than the other two.

## 5. Product principles (non-negotiable)

These bind every requirement below and every future change. They are restated in the repository `AGENTS.md` so agents and contributors work under the same rules.

1. **Transparent by default.** The proxy forwards bytes unchanged until a policy is explicitly enabled. One environment variable disables everything.
2. **Deterministic rendering.** Every transformation is a pure function of its input. The same client message renders to the same provider bytes for the life of a session.
3. **Append-only history.** Replay never edits an earlier turn client-side. Context reduction is requested from the provider through request parameters, which the provider excludes from its history-binding check.
4. **Never touch thinking blocks, signatures, or cache markers the client placed.**
5. **Two tiers of truth.** Numbers derived from transcripts are labeled *estimated*. Numbers captured on the wire are labeled *measured*. They never share a table without the label.
6. **State the assumption.** Every replayed saving carries the note that it assumes unchanged agent behavior. Behavior effects are measured only in live trials.
7. **No credentials, no raw content.** Replay forwards the client's authentication headers unchanged and never persists them. The ledger stores derived data unless the user opts into raw storage.
8. **Secrets stay local, and rehydration is scoped.** Placeholders never rehydrate into shell or network tool inputs by default.
9. **Learning never touches bytes.** Learning writes a policy file. The proxy reads it at session start. The policy is a pure function.
10. **Offline and free.** No analysis step calls the provider API. No telemetry leaves the machine, ever.
11. **Honest status.** The README describes what the code does today, not what is planned.

## 6. Product surface

Four commands. Errors, suggestions, and calibration are sections of their output, not additional commands.

**Status, 2026-09-06: the binary offers seventeen.** This section is left as written because it is the requirement, and the drift from four to seventeen is a fact about the build rather than a change of intent — `cost`, `context`, `route`, `trim`, `advise`, `learn`, `probe`, `corpus`, `rules`, `statusline`, `redact`, `doctor` and `version` all grew out of the four rather than replacing them. Two consequences are worth recording here rather than only in the guide. The bare `replay` form now runs the cost report over the transcript root it discovers, so the first thing a new reader sees is a finding rather than this list; and `replay --help` groups and ranks the seventeen by value, because a flat list put the specialist commands in front of the ones that answer the question people arrive with. [`guide/commands.md`](guide/commands.md) is the reference, and a test fails the build if it stops covering every command and every flag.

| Command | Purpose | Tier |
|---------|---------|------|
| `replay replay <path>` | Reproduce as-run caching for recorded sessions, then score alternative layout policies | Estimated from transcripts; measured once the proxy has recorded the session |
| `replay blame <path>` | Rank the sources of prompt tokens across sessions: files, tool results, tool descriptions, instructions | Same |
| `replay diff <session> [turn]` | Locate the turn and the content where the cached prefix diverged, and classify the cause | Same |
| `replay serve` | Local proxy: passthrough, usage capture, policy application, dry-run, spend guards | Measured |

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
$ replay replay ~/.claude/projects/my-app/

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
  3. AGENTS.md                                          estimated 210k  ±30k
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
| LG-3 | Optional raw mode stores bodies encrypted at rest with a key from the OS keychain, with a retention period and a `replay purge` command. | Purge removes all raw data; encrypted files are unreadable without the keychain. |
| LG-4 | Ledger files are owner-only permissions under the user's home directory. | Permission check in the test suite. |

### 8.2 Replay engine

| ID | Requirement | Acceptance |
|----|-------------|------------|
| RE-1 | A rules module per provider encodes: prefix matching, breakpoint semantics, minimum cacheable size per model, lookback window, TTL by timestamp, write and read multipliers. Rules and price tables are versioned and dated and printed in every output. | Rules module has a unit test per rule with the documented values. |
| RE-2 | Calibration: reproduce the provider's reported cache reads and cache writes for every turn of a recorded session before scoring alternatives. **Status:** 97.46% (28135/28868 turns) across 1450 transcripts, from **78 distinct sessions** ([corpus](evidence/calibration-corpus-2026-09-06.md)); 450 transcripts fall below the 95% threshold and are listed rather than dropped. The rate is lower than the 11-session figure it replaces because the sample is no longer only this repository's own development work. Until 2026-09-06 this cited "1363 sessions", which was a transcript count: a session writes one transcript per lane, so subagents multiplied the file count without adding an independent draw. The session count is met (78 against 20); independence is not, because all of them come from one machine. | Match on at least 95 percent of turns across a 20-session corpus; every mismatch is listed with a reason. |
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
| ER-1 | Classify and price: tool results flagged as errors, failed edits (anchor not found), repeated identical tool calls, context overflow and compaction, provider errors (proxy only). **Status:** the compaction half was the last to land, on 2026-09-06, and it arrived as a correction rather than a feature. Claude Code writes a record carrying `compactMetadata` with the prompt size before and after; nothing read it, so a session that had compacted was attributed as though everything it ever loaded were still in context. Measured across this corpus: 39 records, median 999,029 tokens before against 23,218 after — a retention of 2.55%, so the unsubtracted attribution described a context ninety-seven percent of which was gone. `replay context` now reports the count, the dropped tokens, and the resulting overstatement, and says the overstatement is unmeasurable when a compaction recorded no size. | One fixture per class; totals match. |
| ER-2 | Link failed edits to a preceding stale read of the same file when present. | Fixture with a clear-then-edit sequence. |
| ER-3 | Errors appear as a section in `replay` and `blame` and as a guardrail column in policy tables. | Output contract test. |

### 8.6 Proxy

| ID | Requirement | Acceptance |
|----|-------------|------------|
| PX-1 | Loopback TCP listener. Exposes `/v1/messages*` including `count_tokens`, and `/v1/chat/completions`. A shared header token is optional: loopback binding, browser-origin rejection, and the absence of stored credentials are the default protection, because requiring the token would force every user to configure custom headers before first contact. | Integration test with recorded fixtures for each; token test when set. |
| PX-2 | Byte-for-byte passthrough of request and response, including SSE streaming with immediate flush, when no policy is enabled. | Hash of forwarded bytes equals hash of received bytes across the fixture corpus. |
| PX-3 | Authentication headers are forwarded unchanged and never persisted or logged, including in debug mode. | Log scan test with a canary header value. |
| PX-4 | Timeouts sized for multi-minute turns. Retries (only on rate limit, overload, server error, and connection failure, bounded, jittered, honoring retry-after; never on client errors; never after a response has started streaming) ship with the v0.3 guards; in v0.2 the client's own retry policy applies and every provider response passes through unchanged. | Fault-injection tests per case. |
| PX-5 | Fail open: any failure inside Replay's own analysis or policy code results in the original bytes being forwarded. The spend cap is the single fail-closed exception. | Panic injection test. |
| PX-6 | One environment variable disables all policies; a second disables Replay entirely with a clear client-visible error that names the bypass. | Documented and tested. |
| PX-7 | Session identity comes from the client's `x-claude-code-session-id` header when present (with `x-claude-code-agent-id` distinguishing sub-agents), and otherwise from a hash of the stable prefix. When identity is uncertain, no policy is applied. | Fixture with two interleaved sessions, with and without the header. |
| PX-8 | The policy for a session is chosen at its first request and pinned for the session's life, persisted to disk. **Status:** implemented (`--policy-file`, pins under the ledger directory); tested across a file rewrite and a proxy restart. | Change the policy file mid-session; the session continues under the pinned policy. |
| PX-9 | Client-set request parameters and cache markers are never overridden. Replay adds only what the client did not set, within provider limits (marker count, TTL ordering). The `anthropic-version` and `anthropic-beta` headers are forwarded verbatim, never filtered. | Fixture with four client markers; Replay adds none. Header round-trip test. |
| PX-10 | Every applied transformation is logged with the before and after content hashes, and `replay diff` can show the exact bytes. | Log format test. |
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
| DR-2 | A candidate graduates only after a configurable number of sessions in which predicted savings held within a tolerance. **Status:** implemented in `learn` from treated and control sessions the trial recorded; tolerance is half the predicted saving. | Graduation test with synthetic sessions. |

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
| AD-1 | Convert top token sources into concrete suggestions with a predicted saving: defer-load rarely used tools, add a summary header to a frequently read file, split an instruction file. **Status:** implemented (`replay advise`), plus dominant tool inputs, oversized results, and cache breaks. | Fixture produces the three suggestion types. |
| AD-2 | Track each suggestion to closure: pending, applied (detected from subsequent sessions), verified or not verified against the prediction. **Status:** implemented for kinds whose target is structural; hot files and cache breaks are advice only. | State machine test. |
| AD-3 | Realized savings are reported on the scale-free metric first. **Status:** implemented; shares of prompt tokens lead, corpus tokens follow. | Output contract test. |

### 8.11 Secret masking

| ID | Requirement | Acceptance |
|----|-------------|------------|
| MK-1 | Detection uses a maintained pattern set for known key formats, user-defined patterns, and an optional entropy heuristic. The README names the patterns; it never says "all secrets". **Status:** implemented: pattern set, user patterns, and the entropy heuristic behind `--mask-entropy`, each with a corpus. | Corpus test with precision and recall reported. |
| MK-2 | Placeholders are derived by HMAC of the secret under a per-project key, so the same secret always maps to the same placeholder. **Status:** implemented under a per-install vault key; per-project keys wait for a project identity the proxy can trust. | Determinism test across sessions. |
| MK-3 | The vault persists encrypted at rest with a key from the OS keychain; a daemon restart loses nothing. **Status:** encrypted at rest under an owner-only key file; the keychain is not integrated. | Restart test mid-session. |
| MK-4 | Rehydration works across SSE chunk boundaries and inside tool-call JSON with escaping awareness. **Status:** implemented; a streamed tool input is held until its block ends. | Fixtures with placeholders split across chunks and inside JSON strings. |
| MK-5 | Scoped rehydration: by default, placeholders rehydrate into assistant text and into file-edit tool inputs targeting paths inside the project, never into shell or network tool inputs. Scope is configurable per pattern. **Status:** implemented; unrecognized tools are denied too, and symbolic links are not resolved. | Adversarial corpus: injected placeholders in tool results never reach a shell or network tool input. |
| MK-6 | Every rehydration is logged with its destination. Thinking blocks are never modified in either direction. **Status:** implemented as counts by destination per response, on the log, ledger, status, and metrics. | Log test; thinking passthrough test. |
| MK-7 | A per-session report lists what was masked so the user can verify coverage. **Status:** implemented on ledger records, status, and metrics, by pattern name. | Report test. |

### 8.12 Spend and loop guards

| ID | Requirement | Acceptance |
|----|-------------|------------|
| SP-1 | Token and dollar caps per session and per day computed from provider usage fields; fail closed before the next request; never mid-stream; user override with a logged reason. **Status:** implemented: token caps (`--max-session-tokens`, `--max-day-tokens`) and list-price dollar caps (`--max-session-usd`, `--max-day-usd`) with `x-replay-override`. | Cap test with streaming in progress. |
| SP-2 | Loop detection: the same tool call with the same input N times triggers a warning to the client and, above a second threshold, a block. **Status:** implemented (`--loop-warn`, `--loop-block`, `x-replay-warning` response header). | Fixture. |
| SP-3 | Provider circuit breaker: sustained rate-limit or overload responses open the breaker for a cooling period so the agent stops burning retries. **Status:** implemented (`--breaker-failures`, `--breaker-cooldown`), answering locally with `Retry-After` and one probe after cooldown. | Fault-injection test. |
| SP-4 | An error budget per session trips before the dollar cap when error cost exceeds a share of spend. **Status:** implemented (`--error-budget <share>`), judged from the same error classification `replay` prints, on sessions above a minimum size, with the override header. | Fixture. |
| SP-5 | **Tenant identity is resolved before any cap is consulted.** Every guarded request carries a tenant key — the unit a budget belongs to — resolved at the proxy boundary and passed to the guard alongside the session id. A request whose tenant cannot be resolved is refused, never pooled into a shared bucket: pooling is how one team's spend silently becomes another's refusal. **Status:** specified, not implemented. Gated on ADR-0015. | A request with no resolvable tenant is refused with a distinct reason, and contributes to no accumulator. Asserted by mutation: removing the resolution step must not leave the suite green. |
| SP-6 | **Caps are scoped per tenant.** Session caps key on `{tenant, session}` and day caps on `{tenant, day}`. `SpendGuard.dayUsed` is today one accumulator and `spendState` one persisted record; both gain a tenant dimension. An organisation-wide ceiling remains available but is a separate, explicitly named limit — never the emergent behaviour of a per-developer flag. **Status:** specified, not implemented. Gated on ADR-0015. | Two tenants, one cap value: tenant A exhausting its day cap refuses A and **not** B. This is the acceptance test that fails against the current guard, which is why it is written before the feature. |
| SP-7 | **A refusal names the tenant that caused it.** `Check` returns a reason string carrying no identity, which is correct for one human and useless for fifty: an org-wide refusal that cannot say who spent the budget is an outage with no operator action attached. The reason names the tenant and the limit it hit, under the same rules as every other record — no keys, no paths, no raw content. **Status:** implemented at the granularity that exists today, which is the session — a lane, in a fan-out corpus — because there is no tenant key yet for it to name (SP-5). A daily cap refusal names the largest session accounted for and its share, attributed on the day's spend rather than the lifetime total, since a lane that ran all of yesterday may have touched nothing today. Where eviction means the largest survivor cannot be the largest spender, the refusal says the attribution is partial rather than naming a session it cannot stand behind; where no live session accounts for the spend, it says that. Re-read this row once SP-5 lands: naming a session is not naming a tenant. | A refusal record names the tenant and survives the ledger's existing leak assertions (`sk-ant`, `Bearer` prefix, `x-api-key`, home paths, `"messages":null`). |
| SP-8 | **Eviction may not widen a cap.** The guard's per-session table is bounded at `maxSpendSessions = 1024` and evicts least-recently-seen; eviction discards that session's accumulated spend, so a session that goes quiet and returns is handed a fresh session budget. For one developer the day cap backstops this and the headroom is wide. For fifty developers with parallel sub-agent lanes the table churns continuously and the session cap becomes advisory. Accounting must be durable for the life of the cap window, or eviction must fail closed. **Status:** specified, not implemented. Gated on ADR-0015. | A session evicted and then seen again is not granted a fresh session budget. |

### 8.13 Staleness detection

| ID | Requirement | Acceptance |
|----|-------------|------------|
| ST-1 | When calibration for a model drops below threshold on new sessions, Replay reports that the provider's behavior changed, stops scoring alternatives for that model, and attempts to refit the parameters it can infer (minimum cacheable size, effective lookback). **Status:** implemented offline in `replay corpus` and `replay learn`; the minimum cacheable size is bounded from usage, the lookback is not inferable from usage and is not refit. | Simulated rule change test. |
| ST-2 | Rules it cannot infer remain in the versioned rules file with a documented update process. **Status:** documented in `docs/architecture/replay-engine.md`. | Documentation. |

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
| Windows | **UNSUPPORTED, and never tested.** This row said "proxy verified in CI from v0.2". It was not: the Windows job failed at `go vet` before reaching its test step, so no test has ever run there. Corrected 2026-09-06, when fixing the compile error produced the first Windows test run and fourteen failed. See [RELEASE-CRITERIA.md](../RELEASE-CRITERIA.md) |
| Devcontainers and WSL | Loopback TCP with token; documented networking pattern from v0.2 |

## 10. Security

### 10.1 Threat model

**Assets:** provider credentials in transit; conversation content, which includes source code and secrets; the masking vault; the ledger; the policy file; the binary itself.

**Adversaries:** untrusted content that the agent reads (files in a cloned repository, web pages, tool output) and that can instruct the model; a malicious web page on the same machine; a compromised dependency or release channel; another user on a shared machine. The provider is trusted with what the client already sends it; Replay's job is to send it less, never more.

**Trust boundaries:** the loopback listener; the ledger on disk; the rehydration point where a placeholder becomes a secret again.

### 10.2 Controls

| Threat | Control | Requirement |
|--------|---------|-------------|
| Placeholder forgery leading to exfiltration | Scoped rehydration with default deny for shell and network tools; rehydration log | MK-5, MK-6 |
| Credential theft from Replay | No credentials persisted or logged | PX-3 |
| Second copy of transcripts at rest | Derived-data ledger; raw mode opt-in, encrypted, purgeable | LG-2, LG-3 |
| Local web page reaching the listener | No stored credentials; token on every read; Host and Origin checks; loopback only | PX-1, dashboard requirements when one ships |
| Stored script in tool output rendered by a dashboard | Terminal-only output until a dashboard ships; then strict escaping and a content security policy with no inline script | Dashboard requirements |
| Supply chain | Minimal pinned dependencies with checksum verification; signed releases; software bill of materials; reproducible builds; no auto-update | Release requirements |
| Memory disclosure | No core dumps; no body logging; redacting debug logs | PX-3 |
| Shared machine | Owner-only files; loopback with token | LG-4 |

### 10.3 What Replay never does

Stores or logs credentials. Sends anything anywhere except the provider request the client asked for. Learns across users. Edits an earlier turn. Touches thinking blocks or signatures. Enables server-side compaction on the client's behalf. Rehydrates a secret into a shell or network tool input by default. Auto-updates itself.

### 10.4 Disclosure

Coordinated disclosure per `SECURITY.md`. An external security review is scheduled before the first 1.0 release and its report is published in `docs/internal/reviews/`.

## 11. Privacy and data handling

- All data stays on the machine. There is no telemetry, no crash reporting, and no update check. The README states this in its first screen.
- The ledger holds derived data by default. Raw content is opt-in, encrypted, retention-limited, and purgeable.
- Placeholders are keyed per project, which limits the provider's ability to correlate a secret across projects. This is documented as a known limitation, not hidden.
- A `replay purge` command removes everything Replay wrote.

## 12. Observability

Metrics are computed from the provider's usage object, never inferred from bytes, and exposed locally as a Prometheus text endpoint behind the token. **Status, 2026-09-05:** twenty-one series are implemented, including the policy and rehydration counters this once deferred. Three names below drifted or were not built, and the table keeps the requirement's wording so the difference stays visible: `replay_rehydration_total` shipped as `replay_rehydrated_total`; `replay_error_cost_tokens` was never built; and `replay_added_latency_seconds` is **not** what `replay_request_latency_seconds` measures. The requirement asks for the proxy's own overhead, the byte-in to byte-out gap; the implemented summary covers the whole round trip including the provider, so it is dominated by network time. The overhead figure quoted elsewhere comes from `BenchmarkAddedLatency`, not from this metric.

| Metric | Definition |
|--------|------------|
| `replay_cached_share` | `cache_read / (input + cache_creation + cache_read)` per request |
| `replay_prompt_tokens_total` | Sum of the three usage fields per session |
| `replay_cache_break_total` | Count of requests where the diff classifier found a divergence, by cause |
| `replay_error_cost_tokens` | Tokens attributed to the error classes in Section 8.5 |
| `replay_added_latency_seconds` | Time between receiving the last request byte and forwarding it, p50 and p99 |
| `replay_upstream_errors_total` | By status class |
| `replay_policy_applied_total` | By policy name and session type |
| `replay_rehydration_total` | By destination kind |

Logs redact bodies and headers. A debug mode that logs bodies requires an explicit flag, prints a warning, and still redacts credentials.

**Added 2026-09-06, and deliberately not a metric.** Each response's rate-limit headers — the `anthropic-ratelimit-*` and `x-ratelimit-*` families, plus `retry-after` — are recorded on the ledger entry as the verbatim strings the provider sent. Verbatim because the formats disagree across providers and header families and a parser that guesses wrong fails silently; the ledger's job is to preserve the reading, not to interpret it. They are **not** on `/replay/status` or `/replay/metrics`, because publishing a counter implies its movement means something and nothing has established what a movement here means: [the titration](evidence/quota-titration-2026-09-06.md) moved 3.09M tokens through matched arms and shifted the utilisation counter zero steps. This is the only spend signal a flat-seat user has — there is no invoice to reconcile against — which is exactly why it is worth capturing before it is worth publishing. One fact from live traffic belongs in this record: a subscription session returns none of the documented `anthropic-ratelimit-tokens-remaining` headers, and returns `anthropic-ratelimit-unified-5h-utilization`, `-7d-utilization` and `-representative-claim` instead.

## 13. Verification

### 13.1 Gating spikes

No public claim is made until all five pass. Spike 3 runs first because its answer changes the plan most.

| Spike | Question | Pass condition |
|-------|----------|----------------|
| 1 | Do Claude Code transcripts carry per-message usage with cache read and write counts? | Present on 20 real sessions across two client versions. |
| 2 | Does the replay engine reproduce as-run cache reads and writes? | At least 95 percent of turns across 20 sessions, mismatches explained. |
| 3 | Does Claude Code honor a base URL override under subscription authentication, and is proxying that traffic within the provider's terms? | Documented answer with a source. **Passed:** the LLM gateway documentation describes exactly this configuration; see [`architecture/proxy-protocol.md`](architecture/proxy-protocol.md). |
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

An A/B harness runs a fixed set of real agent tasks with Replay off, on with passthrough, and on with a policy, and reports task success, failed-edit count, prompt tokens, cached share, and wall-clock. It runs on demand under a hard spend cap, never on a schedule that spends money unattended. Its results page is the only place a savings percentage may appear, and each figure links to its run.

## 14. Release plan

Sequenced so that the first release costs nothing to run, works for every user regardless of spike 3, and produces a first release that can be evaluated on its own output.

| Release | Scope | Gate |
|---------|-------|------|
| v0.1 | `replay`, `blame`, `diff` on transcripts. Estimated tier only. Anthropic rules. macOS and Linux. (Windows was listed here and was never tested; see section 9.) | Spikes 1 and 2 pass; calibration line on every output; README shows real output from the maintainer's own sessions. |
| v0.2 | `serve` passthrough with usage capture; measured tier; live `diff`. | Spike 3 answered; passthrough hash test green on the fixture corpus; added latency p99 published. |
| v0.3 | Policy catalog, dry-run, spend and loop guards, error guards. | Spike 4 passes; history-binding check green in CI; guardrail revert tested. |
| v0.4 | Learning job, session types, live trials, advisor. | Synthetic-corpus selection test passes; policy file documented. |
| v0.5 | Secret masking with scoped rehydration and persistent vault. | Spike 5 passes; precision and recall published for the pattern set. |
| 1.0 | External security review published; signed reproducible releases; provider rules for a second provider. | Review findings closed. |

Masking ships last on purpose: it is the feature with the highest consequence of a mistake, and it depends on the deterministic rendering and history-binding test harness that v0.2 and v0.3 establish.

## 15. Repository and community practices

These are in force now and are described in [`maintainers.md`](maintainers.md) and [`CONTRIBUTING.md`](../CONTRIBUTING.md).

- `main` is protected: pull requests only, CI green, one review, linear history. Conventional Commits; squash merge; small single-purpose changes.
- CI runs vet, race tests and build on Linux and macOS, plus golangci-lint and markdownlint, on every pull request. **A Windows job also runs and does not pass**, and as of 2026-09-06 the lint job does not either: 37 issues, all pre-existing. Both are listed as gates in [RELEASE-CRITERIA.md](../RELEASE-CRITERIA.md) rather than described here as passing. This line previously said the suite ran on Windows, which was true and read as a claim that it passed.
- Every design change lands with an ADR. Every user-visible change lands with a changelog entry.
- Historical documents are never edited; a new version is added instead, and superseded drafts live in the git history rather than the working tree.
- Issue and pull request templates, a canonical label set, weekly stale automation, Dependabot, CODEOWNERS.
- Security reports go through private advisories, never public issues.
- The README states status truthfully and names the maintainer.

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
| Provider absorbs the diagnostic natively | Medium | Medium | Cross-client, local what-if and advisor remain; the goal is a tool that is worth using |
| Single maintainer availability | High | High | Small releases; every step leaves a usable tool; documentation good enough for a second contributor |
| Masking false negatives create false confidence | Medium | High | Named pattern set, per-session masked report, never claim completeness |

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

---

[Documentation index](README.md) · [Repository README](../README.md)
