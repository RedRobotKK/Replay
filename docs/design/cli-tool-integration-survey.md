# The AI CLI tooling on this machine, and what Replay could measure of it

**2026-09-09. A survey, not a decision.** Every claim below carries the command
that produced it. Two prior claims are corrected: one about what is installed,
one about what Grok records on disk. The second reverses a retraction made in
`docs/evidence/surface-census-2026-09-08.md`.

## Summary

| Question | Answer |
|---|---|
| Of the ten tools previously reported on, how many were classified correctly? | **9 of 10** |
| What was wrong? | `whisper-cpp` was reported absent. It is installed via Homebrew under different binary names. |
| AI CLIs found that were not on the list at all | **5** — `codex`, `grok`, `cursor`, `oracle`, `openclaw` |
| Highest-value integration | **Grok**, which carries per-turn token counts *and* a cost field |

## Part 1: what is actually installed

The prior check appears to have run `command -v` only. That is why it missed
`whisper-cpp`, which Homebrew installs as `whisper-cli`, `whisper-server` and
seven siblings — never as `whisper-cpp`.

### The ten previously reported

| Tool | Reported | Actual | How it is reached | Evidence |
|---|---|---|---|---|
| `claude` | installed | **installed** | `/Users/daniel/.local/bin/claude` → `~/.local/share/claude/versions/2.1.260` | `claude --version` → `2.1.260 (Claude Code)` |
| `ollama` | installed | **installed** | `/opt/homebrew/bin/ollama` + `/Applications/Ollama.app` | `ollama --version` → `0.33.2` (client 0.33.3) |
| `whisper-cpp` | NOT installed | **installed — reported wrongly** | brew formula `whisper-cpp` 1.9.2; binaries `whisper-cli`, `whisper-server`, `whisper-stream`, `whisper-bench`, `whisper-lsp`, `whisper-talk-llama`, `whisper-command`, `whisper-quantize`, `whisper-vad-speech-segments`, plus `parakeet-cli` | `brew list whisper-cpp`; `whisper-cli --version` → `whisper.cpp version: 1.9.2` |
| `openai` | NOT installed | **absent, confirmed** | — | not in `brew list`, `npm ls -g`, `pipx list` (empty), `uv tool list` (empty); `python3 -m openai` → `ModuleNotFoundError` on python3, 3.11, 3.12, 3.14 |
| `aider` | NOT installed | **absent, confirmed** | — | absent from all bin dirs, npm globals, brew, pipx, uv; no `site-packages/aider*` found |
| `mentat` | NOT installed | **absent, confirmed** | — | same sweep |
| `shell-genie` | NOT installed | **absent, confirmed** | — | same sweep |
| `fabric` | NOT installed | **absent, confirmed** | — | same sweep; `~/go/bin` holds only `deadcode`, `goimports`, `golangci-lint` |
| `platypus` | NOT installed | **absent, confirmed** | — | same sweep |
| `diffusion-cli` | NOT installed | **absent, confirmed** | — | same sweep |

Bin directories swept: `/opt/homebrew/bin`, `/usr/local/bin`, `/usr/bin`,
`~/.local/bin`, `~/go/bin`, `~/.cargo/bin`, `~/.grok/bin`. Package managers
swept: `brew list --formula`/`--cask`, `npm ls -g --depth=0`, `pipx list`,
`uv tool list`, `python3 -m pip list` across four interpreters.

**A false positive worth naming.** A bare `command -v continue` returns
`continue`, because it is a shell builtin. Continue.dev is *not* installed. Any
checker that trusts `command -v` for that name will report it wrongly.

### The five the list did not mention

| Tool | Version | Path |
|---|---|---|
| **Codex** (OpenAI) | `codex-cli 0.153.4` | `/opt/homebrew/bin/codex` → npm `@openai/codex`; native binary at `@openai/codex-darwin-arm64/vendor/aarch64-apple-darwin/bin/codex` (220 MB Mach-O arm64) |
| **Grok** (xAI) | `grok 1.0.5 (5115b46bc909) [stable]` | `~/.grok/bin/grok` → `~/.grok/downloads/grok-1.0.5-macos-aarch64`; on `PATH` first |
| **Cursor** | `3.17.19` | `/usr/local/bin/cursor` + `/Applications/Cursor.app` |
| **Oracle** | `@steipete/oracle 0.8.6` | `/opt/homebrew/bin/oracle` (also `oracle-mcp`) — wrapper around the OpenAI Responses API |
| **OpenClaw** | `openclaw 2026.2.15` | `/opt/homebrew/bin/openclaw` — multi-provider agent gateway, configured for OpenRouter |

Also present but not model CLIs: `gh 2.96.0` (**no** Copilot extension —
`gh extension list` returns empty), `clawhub`, `mcporter`, `agmsg`.

Checked and confirmed absent beyond the original list: `llm`, `sgpt`,
`chatblade`, `mods`, `gptme`, `open-interpreter`, `continue`, `goose`, `crush`,
`opencode`, `gemini`, `amp`, `cline`, `aichat`, `tgpt`, `q`.

## Part 2: the four routes, per installed tool

### Claude Code — proxy + transcript. Already integrated

- **Proxy route: yes.** `ANTHROPIC_BASE_URL` is present in the binary alongside
  `ANTHROPIC_AUTH_TOKEN`, `ANTHROPIC_CUSTOM_HEADERS`, `ANTHROPIC_BEDROCK_BASE_URL`
  and `ANTHROPIC_API_HOST`. This is the route Replay already documents.
- **Transcript route: yes.** `~/.claude/projects/` — **1,735 `.jsonl` files, 1.6 GB**
  across 12 project directories.
- **Usage in transcripts: yes, richest of any surface.** A real record carries
  `input_tokens`, `cache_creation_input_tokens`, `cache_read_input_tokens`,
  `output_tokens`, `output_tokens_details.thinking_tokens`, `service_tier`,
  `cache_creation.ephemeral_1h/5m_input_tokens`, and a per-iteration breakdown.
- **Config route:** `~/.claude/settings.json` (no base URL set today).

Parsed by `internal/transcript/claudecode.go`.

### Codex — proxy + transcript. Already integrated; rate-limit data unused

- **Proxy route: yes, two ways.** The binary contains the literal
  `OPENAI_BASE_URL`, the config key `openai_base_url`, and a full
  `model_providers` table with `base_url`, `env_key` and `wire_api` fields — so
  a provider can be redirected without touching the environment. It also ships
  its own `network-proxy` with MITM support (`network.mitm`, `network.mitm_hooks`,
  `network.proxy_url`), which is a second, independent interception point.
- **Transcript route: yes.** `~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl`
  (**29 files, 25 MB**) and `~/.codex/archived_sessions` (**121 files, 17 MB**).
  Entry types: `session_meta`, `turn_context`, `response_item`, `event_msg`.
- **Usage: yes, and more than tokens.** `event_msg` of subtype `token_count`
  carries `total_token_usage` and `last_token_usage`
  (`input_tokens`, `cached_input_tokens`, `output_tokens`,
  `reasoning_output_tokens`, `total_tokens`), `model_context_window`, **and a
  `rate_limits` object** with `used_percent`, `window_minutes`, `resets_at` for
  primary and secondary windows plus `plan_type`. One 220 KB session held 28 such
  events.
- **Config route:** `~/.codex/config.toml`.
- **Unexplored:** `~/.codex/logs_1.sqlite` is **422 MB** with a single `logs`
  table. Not opened here.

Parsed by `internal/transcript/codex.go`. The `rate_limits` block is a
quota-exhaustion surface Replay does not currently expose.

### Grok — proxy + transcript. Not integrated. This is the finding

- **Proxy route: yes.** The binary contains `GROK_CLI_CHAT_PROXY_BASE_URL`
  (7 occurrences), `XAI_API_BASE_URL` (2), `GROK_CLI_BASE_URL` (1), plus
  `GROK_MODELS_BASE_URL`, `GROK_CONVERSATIONS_BASE_URL` and others. It contains
  **no** `ANTHROPIC_BASE_URL` and **no** `OPENAI_BASE_URL` — consistent with
  `wire-families-2026-09-06.md`: Grok is a third wire family, posting
  `POST /responses` to `cli-chat-proxy.grok.com`.
- **Transcript route: yes, and it is the largest on the machine.**
  `~/.grok/sessions/` — **6,787 files, 3.8 GB**, one directory per URL-encoded
  working directory, then one per session UUID. Each session holds
  `chat_history.jsonl`, `events.jsonl`, `updates.jsonl`, `summary.json`,
  `signals.json`, `prompt_context.json`, `rewind_points.jsonl`,
  `system_prompt.txt`. Plus `~/.grok/logs/unified.jsonl` (4.5 MB,
  2026-08-25 → 2026-09-06).

#### Correction: Grok does record token counts and cost

`docs/evidence/surface-census-2026-09-08.md` states that `inputTokens`,
`outputTokens`, `cachedReadTokens`, `cacheCreationTokens` and `costUsdTicks`
"appear in **zero files**" across `~/.grok`, and on that basis withdrew a
figure of $406.07 over 1,411 turns.

That search was wrong. Re-run today:

```text
inputTokens: 76 files      outputTokens: 77 files
cachedReadTokens: 72 files cacheCreationTokens: 72 files
costUsdTicks: 72 files
```

The fields live in **`updates.jsonl`**, which the census did not walk — it
walked `events.jsonl` (correctly finding only lifecycle events) and
`chat_history.jsonl` (correctly finding only `content`, `model_id`,
`model_fingerprint`, `reasoning_effort`). A real record:

```json
{"sessionUpdate":"turn_completed","prompt_id":"2554eb4f-...","stop_reason":"end_turn",
 "usage":{"inputTokens":17415,"outputTokens":35,"totalTokens":17450,
   "cachedReadTokens":6272,"cacheCreationTokens":0,"reasoningTokens":30,
   "modelCalls":1,"apiDurationMs":4927,"costUsdTicks":43574400,
   "modelUsage":{"grok-4.6-build":{...}},"numTurns":1}}
```

Aggregated over every `updates.jsonl` on the machine:

| | Turns | Model calls | inputTokens | cachedReadTokens | outputTokens | reasoningTokens | costUsdTicks |
|---|---|---|---|---|---|---|---|
| All | **1,411** | 5,251 | 1,113,399,118 | 1,065,998,336 | 2,670,238 | 1,841,562 | 4,060,673,077,000 |
| `grok-4.6-build` | — | 3,113 | 607,250,657 | 564,243,968 | 1,208,264 | — | 1,018,309,505,000 |
| `grok-4.5-build` | — | 2,138 | 506,148,461 | 501,754,368 | 1,461,974 | — | 3,042,363,572,000 |

**The 1,411 turns match the withdrawn figure exactly**, and at 1 tick = 1e-10 USD
the total is **$406.0673077** — the withdrawn number to seven decimal places.
The retraction was itself in error.

**What is still not verified.** The tick scale is not written down in any file.
1e-10 is inferred from reproducing the prior figure, and it yields
per-model rates that are internally plausible for a cached-read-dominated
workload — but it has **not** been checked against an xAI invoice. Treat
$406.07 as "measured tokens times an unverified scale factor", not as a billed
amount. The token counts themselves need no such caveat.

Separately, `~/.grok/logs/unified.jsonl` carries **192**
`shell.turn.inference_done` records with a *different*, snake_case shape —
`prompt_tokens`, `cached_prompt_tokens`, `completion_tokens`, `reasoning_tokens`,
`ttft_ms`, `itl_p50_ms`, `tokens_per_sec`, `model_elapsed_ms`, `attempts` — keyed
by `sid` to the session UUID. That is a latency surface Replay has no equivalent
of anywhere. `signals.json` adds per-session `contextTokensUsed` /
`contextWindowTokens` / `compactionCount`.

- **Config route:** `~/.grok/config.toml`.

### Cursor — transcript only. No usage

- **Proxy route: no.** No base-URL override; already excluded by
  `requirements.md` §9 for exactly this reason. Correct as documented.
- **Transcript route: yes.** `~/.cursor/projects/<encoded-cwd>/agent-transcripts/<uuid>/<uuid>.jsonl`
  — **118 files**, `~/.cursor/projects` totals 11 MB. Roles: `user`, `assistant`,
  and `error`/`status` entries. Sibling dirs: `agent-tools`, `terminals`,
  `canvases`, `mcps`, `assets`.
- **Usage: no.** Assistant entries have exactly `["message","role"]` and the
  message has only `["content"]`. No `usage`, no token or cost key at any depth.
  This confirms the census.
- **Other stores, not opened:** `~/.cursor/ai-tracking/ai-code-tracking.db`
  (6.7 MB; tables `ai_code_hashes`, `conversation_summaries`,
  `tracked_file_content`, `scored_commits`, `ai_deleted_files`, `tracking_state`)
  and `~/Library/Application Support/Cursor/User/globalStorage/state.vscdb`
  (**658 MB**). Whether either holds usage is undetermined — I did not query them.

### Ollama — proxy + log. Already integrated

- **Proxy route: yes.** `OLLAMA_HOST` and `OLLAMA_CLOUD_BASE_URL` are both in
  the binary. Ollama also serves an OpenAI-compatible `/v1/chat/completions`,
  which Replay's chat-completions path already parses.
- **Log route: yes, but thin.** `~/.ollama/logs/server.log` (3.0 MB) plus five
  rotated files, one of which is 41 MB. **126** `POST /api/chat` or
  `/api/generate` lines in the current log.
- **Usage: no.** `grep -c eval_count` over `server.log` returns **0**. The GIN
  access lines carry status, latency and endpoint only — the `eval_count` /
  `prompt_eval_count` fields exist in the API *response* but are never logged.
  Token counts for Ollama are obtainable only through the proxy.

### whisper-cpp — no route

Speech-to-text. `whisper-server` exposes `/inference` and `/load` — **not** an
OpenAI-compatible `/v1/audio/transcriptions` endpoint. No session log, no token
concept, no base-URL client env var. **No route.** It is not a measurement
surface and should not be treated as one.

### Oracle — proxy route, trivially

`@steipete/oracle` 0.8.6 reads **`OPENAI_BASE_URL`** and **`ANTHROPIC_BASE_URL`**
(both present in `dist/`, alongside `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`,
`GEMINI_API_KEY`, `OPENROUTER_API_KEY`, `AZURE_OPENAI_API_KEY`). No config or
log directory exists on this machine (`~/.oracle`, `~/.config/oracle`,
`~/Library/Application Support/oracle` all absent) — so **proxy route only, no
transcript route**. It targets the OpenAI **Responses** API, which Replay
forwards unparsed.

### OpenClaw — proxy + config route

`~/.openclaw/openclaw.json` configures an `openrouter:default` auth profile with
`openrouter/anthropic/claude-3.5-haiku` as primary. `dist/` contains
`DEFAULT_OPENAI_BASE_URL`, `DEFAULT_OLLAMA_BASE_URL`, `DEFAULT_GEMINI_BASE_URL`,
`DEFAULT_COPILOT_API_BASE_URL` and ten more — every provider has an overridable
base URL, so **proxy and config routes both apply**. Logs exist
(`~/.openclaw/logs/`, 2.1 MB: `gateway.log`, `gateway.err.log`,
`config-audit.jsonl`) but were not examined for token fields.

### Route summary

| Tool | Proxy | Transcript | Usage in transcript | Config | Replay today |
|---|---|---|---|---|---|
| Claude Code | ✅ `ANTHROPIC_BASE_URL` | ✅ 1.6 GB | ✅ richest | ✅ | **read** |
| Codex | ✅ `OPENAI_BASE_URL` + `model_providers` | ✅ 42 MB | ✅ + rate limits | ✅ | **read** (rate limits unused) |
| **Grok** | ✅ `GROK_CLI_CHAT_PROXY_BASE_URL` | ✅ **3.8 GB** | ✅ **tokens + cost** | ✅ | **not read** |
| Cursor | ❌ | ✅ 11 MB | ❌ none | ✅ | not read |
| Ollama | ✅ `OLLAMA_HOST` | ✅ logs | ❌ not logged | — | **read** |
| Oracle | ✅ `OPENAI_BASE_URL` / `ANTHROPIC_BASE_URL` | ❌ none | — | ❌ | not read |
| OpenClaw | ✅ per-provider | ⚠️ logs unexamined | undetermined | ✅ | not read |
| whisper-cpp | ❌ | ❌ | — | ❌ | **no route** |

## Part 3: ranking by new measurement surface per unit of work

| Rank | Tool | New surface | Work | Why |
|---|---|---|---|---|
| 1 | **Grok transcripts** | 1,411 turns, 5,251 model calls, tokens **and** cost, 3.8 GB | Small — one parser | Only unread surface on the machine with a cost field |
| 2 | **Codex `rate_limits`** | Quota windows, `resets_at`, `plan_type` | Very small — fields already in a parsed file | Data is already being read past |
| 3 | **Oracle / OpenClaw via proxy** | Live measured spend on a third and fourth client | Small — documentation, not code | Both honour `OPENAI_BASE_URL`; Replay's chat-completions path already exists |
| 4 | Grok `unified.jsonl` latency | TTFT, inter-token latency, tokens/sec | Small | No latency surface exists anywhere in Replay |
| 5 | Cursor transcripts | 118 conversations | Moderate | Conversation-only; **no** spend data. Do not build for cost. |
| — | whisper-cpp | none | — | No route. Excluded. |

### What the top three actually require

**1. Grok transcript reader.**

- `internal/transcript/grok.go` with `ParseGrokFile` over
  `~/.grok/sessions/<url-encoded-cwd>/<uuid>/updates.jsonl`, selecting
  `.params.update.sessionUpdate == "turn_completed"` and reading
  `.params.update.usage`. Session id is on `.params.sessionId`; cwd is
  URL-decoded from the parent directory name.
- Map to `transcript.Usage`: `inputTokens` → input, `cachedReadTokens` →
  cache-read, `cacheCreationTokens` → cache-creation, `outputTokens` → output,
  `reasoningTokens` → thinking. **Confirm inclusive vs exclusive counting before
  trusting the sum** — `inputTokens` here (17,415) exceeds `cachedReadTokens`
  (6,272), which is consistent with inclusive, the same convention
  `internal/transcript/openai.go` already converts.
- A new `Source` constant (`"grok-session-update"`), tier "estimated
  (transcripts only)".
- Cost: **do not** route through `internal/cachemodel`, which rejects any
  `Provider != "anthropic"`. Grok ships its own `costUsdTicks`, so the honest
  move is to carry the vendor's own number as a distinct field and record the
  tick scale as unverified — not to re-derive it from a rules file that does not
  exist for xAI.
- **First: correct `docs/evidence/surface-census-2026-09-08.md`.** It currently
  asserts the opposite, and Finding 4's retraction needs un-retracting.

**2. Codex rate limits.** `internal/transcript/codex.go` already defines
`CodexQuota`. Confirm it reads `event_msg.payload.rate_limits` (`used_percent`,
`window_minutes`, `resets_at`, `plan_type`) and surface it in `replay codex`.
Fields exist in files already on disk; no new I/O.

**3. Oracle and OpenClaw through the existing proxy.** Both already speak
OpenAI-compatible wire formats and already read `OPENAI_BASE_URL`. The gap is
that `/v1/chat/completions` is ledgered but **not masked and has no policy
applied** (`noteUnmasked`, `internal/proxy/server.go:999`). Closing that is the
work; adding the clients is documentation. Note Oracle targets `/v1/responses`,
which Replay forwards unparsed — so Oracle needs a Responses-API parser first,
while OpenClaw does not.

## What was not examined

Named rather than guessed at: `~/.codex/logs_1.sqlite` (422 MB, single `logs`
table); `~/.cursor/ai-tracking/ai-code-tracking.db` (6.7 MB);
`~/Library/Application Support/Cursor/User/globalStorage/state.vscdb` (658 MB);
`~/.openclaw/logs/` (2.1 MB); `~/.grok/memtrace/` (21 MB);
`~/.grok/sessions/session_search.sqlite` (4.7 MB). Whether any holds usage is
**undetermined**, not "no".

No tool was executed in a way that makes a paid API call. Nothing was installed
or modified. `~/.config/replay-metrics/posthog.env` contains a live PostHog
personal API key in plaintext; its value is deliberately not reproduced here.

---

[Documentation index](../README.md) · [Repository README](../../README.md)
