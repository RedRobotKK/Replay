# Surface detection survey, 2026

Goal: extend `cmd/replay/othersurfaces.go` so Replay detects every agent surface
a reader might be spending on, and states honestly what it can and cannot read
from each.

## The rule every agent works under

Replay's whole claim is that it does not state what it has not measured. A
detector that invents a path, or claims a format it has not seen, is worse than
no detector: it sends a reader looking for a file that is not there and then
says their bill has no blind spot when it does.

So: **verify on this machine where possible. Mark everything else UNVERIFIED and
say what would verify it.** NOT RECORDED beats a guess.

## Existing surfaces (cmd/replay/othersurfaces.go)

| Surface | Status today |
|---|---|
| Claude Code | read fully |
| Codex | read fully (`replay codex`) |
| Ollama | detected; `replay burn` reads logs, refuses a cache hit rate, says why |
| Grok | detected, not read: usage IS on disk in updates.jsonl, cacheCreationTokens zero on every record (corrected by AGENT-5; the /responses claim was wrong) |
| Cursor | detected, not read: tokenCount present and zero in state.vscdb, and no usage field in 4,566 agent-transcript rows |
| AnythingLLM | detected, not read: metrics object carries prompt_tokens and completion_tokens and no cache field (added by AGENT-5) |
| OpenClaw | detected, not read: cacheWrite counter present and zero on every assistant row, beside non-zero cacheRead (added by AGENT-5) |
| Oracle | detected, not read: no cache field, and the one session with usage has totalTokens 6 against inputTokens 4256 (added by AGENT-5) |

## What each agent returns

Per surface, a row with:
1. `name`
2. `dir` candidate paths, each marked VERIFIED-PRESENT / VERIFIED-ABSENT / UNVERIFIED
3. local artefact format (jsonl, sqlite, md, none)
4. does it record token usage? cache read/write tokens?
5. does the billing route support prompt caching at all?
6. **PASS condition**: what must hold for Replay to price it
7. **FAIL condition**: what makes it unpriceable, and the exact `why:` string to print
8. draft Go `otherSurface{}` literal

## Claimed by

- AGENT-1 VS Code extension family
- AGENT-2 terminal agents
- AGENT-3 IDE native
- AGENT-4 cloud and enterprise routes (pricing correctness)

## Findings

(agents append below)

### AGENT-4

Scope: cloud and enterprise billing routes. Bedrock, Vertex, Azure OpenAI,
OpenRouter, LiteLLM. The question is not whether Replay can READ these routes.
It is whether Replay prices them against the wrong table when it can.

#### 1. How Replay decides which price table to apply

There is one table and no route dimension.

- `internal/cachemodel/anthropic.go:161-188` is the whole table. Rows are
  matched by substring on the model id.
- `internal/cachemodel/match.go:31-47` `matchesModel` does the matching. It
  skips a row only when the text after the match is a short version run. A
  vendor prefix, a region prefix, an ARN, a Vertex `@date` suffix and a
  `vendor/` slash prefix all still match.
- `internal/cachemodel/anthropic.go:246` `anthropicFamilies` is a family-name
  test, not a route test. Every route form of a Claude id contains `claude`,
  so `foreignModel` and `unpriceable` (`:265-282`) return false for all of them
  and full cache arithmetic is applied.
- `internal/cachemodel/rules.go:64-67` states the premise out loud: "A row's
  provider is not used to select it. Model ids are already distinct across
  providers and Match does the selecting." That premise is false for route
  forms of the same model, and the measurement below is what falsifies it.
- Write multipliers are compiled constants
  (`internal/cachemodel/anthropic.go:102-104`, applied at `:463-468`).
  `ModelRule` (`internal/cachemodel/rules.go:52-90`) has `readMult` and no
  `writeMult`, so a route whose write multiple differs cannot be expressed in a
  rules document at all.

MEASURED, not argued. A probe built against this tree in a scratchpad copy,
calling `cachemodel.PriceFor` / `CostUSD` / `ReadMultiplierFor` /
`MinCacheablePrefix` on the same fixed usage:

| model id | priced | in/MTok | readMult | minPrefix | cost |
|---|---|---|---|---|---|
| `claude-sonnet-4-5-20250929` | true | 3.00 | 0.100 | 1024 | 0.078000 |
| `anthropic.claude-sonnet-4-5-20250929-v1:0` | true | 3.00 | 0.100 | 1024 | 0.078000 |
| `us.anthropic.claude-sonnet-4-5-20250929-v1:0` | true | 3.00 | 0.100 | 1024 | 0.078000 |
| `arn:aws:bedrock:...:inference-profile/us.anthropic.claude-sonnet-4-5-...` | true | 3.00 | 0.100 | 1024 | 0.078000 |
| `claude-sonnet-4-5@20250929` | true | 3.00 | 0.100 | 1024 | 0.078000 |
| `projects/p/locations/us-east5/publishers/anthropic/models/claude-sonnet-4-5` | true | 3.00 | 0.100 | 1024 | 0.078000 |
| `anthropic/claude-sonnet-4.5` | true | 3.00 | 0.100 | 1024 | 0.078000 |
| `bedrock/anthropic.claude-sonnet-4-5-20250929-v1:0` | true | 3.00 | 0.100 | 1024 | 0.078000 |
| `vertex_ai/claude-sonnet-4-5@20250929` | true | 3.00 | 0.100 | 1024 | 0.078000 |
| `azure/gpt-5` | false | 0.00 | 0.100 | 1024 | 0.000000 |

Every cloud route form of a Claude id prices to the same cent as the
first-party id, with `priced: true` and no label.

#### 2. What already distinguishes a route, and what it is used for

`cmd/replay/namespace.go:34-54` `classifyRoute` ALREADY detects these routes
from the model id. MEASURED against the same ids:

| model id | classifyRoute |
|---|---|
| `claude-sonnet-4-5-20250929` | first-party API |
| `anthropic.claude-sonnet-4-5-20250929-v1:0` | Bedrock |
| `us.anthropic.claude-...-v1:0` | Bedrock |
| `arn:aws:bedrock:...` | Bedrock |
| `claude-sonnet-4-5@20250929` | Vertex |
| `projects/.../publishers/anthropic/models/...` | Vertex |
| `bedrock/anthropic.claude-...` | Bedrock |
| `vertex_ai/claude-sonnet-4-5@20250929` | Vertex |
| `anthropic/claude-sonnet-4.5` | **other** |
| `openrouter/anthropic/claude-sonnet-4.5` | **other** |

It reaches exactly one place: `routeLine` (`cmd/replay/namespace.go:61-95`),
wired at `cmd/replay/cost.go:356` and `cmd/replay/cost.go:780`, printed by
`cmd/replay/share.go:75-76` as "routed via Bedrock, metered". It never reaches
the price. So a share card can print "routed via Bedrock, metered" beside a
dollar figure computed at Anthropic first-party list price.

The two halves of `cachemodel` already disagree about this, and the checking
half is the one that is right. `bareClaudeName`
(`internal/cachemodel/pricecheck.go:136-147`) rejects any key containing
`/`, `.`, `@` or `:` with the comment "rejects reseller keys". That is exactly
the Bedrock, Vertex, OpenRouter and LiteLLM id forms. The price-CHECK path
refuses to compare them. The price-APPLY path matches them into first-party
rows.

#### 3. The proxy

The proxy is not where this bug is. MEASURED, path predicates
(`internal/proxy/passthrough.go:361-389`) against real route paths:

| path | isMessages | isChatCompletions |
|---|---|---|
| `/v1/messages` | true | false |
| `/model/us.anthropic.claude-opus-5-v1:0/invoke` | false | false |
| `/model/.../invoke-with-response-stream` | false | false |
| `/v1/projects/.../publishers/anthropic/models/...:rawPredict` | false | false |
| `/v1/projects/.../:streamRawPredict` | false | false |
| `/openai/deployments/gpt-5/chat/completions` | false | false |
| `/v1/chat/completions` | false | true |
| `/anthropic/v1/messages` | **true** | false |

Native Bedrock, native Vertex and Azure OpenAI paths fall to `noteUnparsed`
(`internal/proxy/passthrough.go:407-414`), which says plainly that no cap, no
budget, no loop detection and no masking apply. That is an honest gap and it is
correct.

The last row is the hole. A gateway that mounts the Anthropic wire under a
prefix is parsed in full as first-party. And the upstream is free:
`cmd/replay/serve.go:29,38,53,122,146` take `--upstream` / `REPLAY_UPSTREAM`
as any absolute URL. Nothing records it. `ledger.Record`
(`internal/ledger/record.go:58-184`) carries `Path` and `Model` and no upstream
host, so a ledger written against a LiteLLM gateway is byte-identical in shape
to one written against `api.anthropic.com`. `internal/proxy/hostguard.go:39-59`
checks the CLIENT's Host header only, never the upstream.

#### 4. Evidence on this machine

| looked for | result |
|---|---|
| `~/.aws/` | VERIFIED-ABSENT |
| `~/.azure/` | VERIFIED-ABSENT |
| `~/.config/gcloud/` | VERIFIED-PRESENT. ADC `type: authorized_user`, one project configured, last touched 2026-08-05. SDK at `~/google-cloud-sdk`, not on PATH |
| `aws`, `az`, `gcloud`, `litellm` binaries | VERIFIED-ABSENT from PATH |
| `AWS_*`, `AZURE_*`, `GOOGLE_*`, `GCP_*` env | VERIFIED-ABSENT |
| `ANTHROPIC_BASE_URL`, `OPENAI_BASE_URL` env | VERIFIED-ABSENT |
| litellm config (`~/.litellm`, `~/.config/litellm`, `*litellm*.yaml`) | VERIFIED-ABSENT |
| `env` block in `~/.claude/settings.json`, `settings.local.json`, repo `.claude/settings.json` | VERIFIED-ABSENT |

Model ids across all 1,910 local Claude Code transcripts, parsed rather than
grepped: 8 distinct ids, 151,019 occurrences, all first-party.
`claude-opus-5` 130,214, `claude-opus-4-8` 9,313, `claude-sonnet-5` 7,194,
`claude-fable-5-1` 2,065, `claude-haiku-4-5-20251001` 2,008,
`claude-sonnet-4-6` 183, `<synthetic>` 40, `claude-sonnet-4-5-20250929` 2.
**Zero route-shaped ids.** This reproduces ADR-0015's corpus result on a
larger, newer corpus.

The surface is nonetheless real and installed. The Claude Code binary on this
machine (`~/.local/share/claude/versions/2.1.269`, Mach-O) carries
`CLAUDE_CODE_USE_BEDROCK` (18), `CLAUDE_CODE_USE_VERTEX` (19),
`ANTHROPIC_BEDROCK_BASE_URL` (20), `ANTHROPIC_VERTEX_BASE_URL` (13),
`ANTHROPIC_VERTEX_PROJECT_ID` (10), `CLOUD_ML_REGION` (9),
`AWS_BEARER_TOKEN_BEDROCK` (18), a full map of `us.anthropic.claude-*`
inference profile ids and `claude-*@YYYYMMDD` Vertex ids, and the request paths
`/model/${id}/invoke`, `/model/${id}/invoke-with-response-stream`,
`:rawPredict` and `publishers/anthropic/models`. The same binary carries
`cache_control` (72), `ephemeral` (159), `cache_creation_input_tokens` (137)
and `cache_read_input_tokens` (140).

Note what that last sentence does and does not establish. One client ships the
cache vocabulary and the route id maps together, which makes the same cache API
likely on those routes. It does not verify a single price.

Note also that Replay's own refusal text
(`internal/proxy/hostguard.go:89-93`) tells the operator to
"Set ANTHROPIC_BASE_URL". A Bedrock or Vertex user sets
`ANTHROPIC_BEDROCK_BASE_URL` or `ANTHROPIC_VERTEX_BASE_URL`, and that advice
does not reach them.

#### 5. Route table

The multiplier and verdict columns here were written before the external check
in section 9. Section 9 supersedes them and is the one to read.

| route | how it is detectable | caching applies | multipliers differ | verdict |
|---|---|---|---|---|
| AWS Bedrock, native | model id (`anthropic.claude-*`, `us|eu|apac.anthropic.claude-*`, `arn:aws:bedrock:*`) VERIFIED against `classifyRoute`; request path `/model/{id}/invoke` VERIFIED from the installed client | same field names shipped by the same client, likely | UNVERIFIED | UNVERIFIED on price |
| Google Vertex AI | model id (`claude-*@YYYYMMDD`, `publishers/anthropic/models/*`) VERIFIED; path `:rawPredict` / `:streamRawPredict` VERIFIED from the installed client | same, likely | UNVERIFIED | UNVERIFIED on price |
| Azure OpenAI | path `/openai/deployments/{name}/chat/completions` VERIFIED not parsed by this build; id form `azure/*` prices false VERIFIED | automatic, no write signal, per ADR-0019's OpenAI row | UNVERIFIED, and rates are per deployment and per region | UNVERIFIED on price |
| OpenRouter | `anthropic/claude-*` classifies as **other** VERIFIED. No route signal reaches anything | per-model, and the serving provider can change without the id changing | UNVERIFIED | UNVERIFIED on price |
| LiteLLM proxy | `bedrock/*` and `vertex_ai/*` prefixed ids classify correctly VERIFIED. A bare id behind an Anthropic-wire mount does not classify at all. `/anthropic/v1/messages` parses as first-party VERIFIED | inherits the backend's | inherits the backend's | UNVERIFIED on price |

What would settle the UNVERIFIED cells, named rather than gestured at:
`aws.amazon.com/bedrock/pricing` and the Bedrock prompt-caching page under
`docs.aws.amazon.com/bedrock`; `cloud.google.com/vertex-ai/pricing` and the
Claude-on-Vertex page under `cloud.google.com/vertex-ai/generative-ai/docs`;
`azure.microsoft.com/pricing` for the deployment tiers;
`openrouter.ai/api/v1/models`, which is live JSON and already named in
`docs/TOKEN-PRICES.md:21` as carrying `input_cache_read`;
`BerriAI/litellm` `model_prices_and_context_window.json`, already fetched by
`replay rules --check-prices` and already carrying `bedrock/` and `vertex_ai/`
keyed rows that `bareClaudeName` currently discards.

#### 6. PASS and FAIL, per route

PASS is the same sentence in every row, so it is written once: **Replay may
price a request on one of these routes only when a dated rules document in
force carries a row for that route's own id form, published by that route's
own operator, with that route's read AND write multipliers.** No such document
exists today and the schema cannot hold the write multiplier, so today every
row FAILS.

FAIL strings, in the project's voice:

- Bedrock:
  `"Replay cannot price Bedrock traffic. The model id names an AWS inference profile, Bedrock sets its own per-token rates, and this build carries only Anthropic's."`
- Vertex:
  `"Replay cannot price Vertex traffic. The model id names a Google publisher path, Vertex sets its own per-token rates, and this build carries only Anthropic's."`
- Azure OpenAI:
  `"Replay cannot read Azure OpenAI. It posts to /openai/deployments/NAME/chat/completions, which this build does not parse, and its rates are set per deployment."`
- OpenRouter:
  `"Replay cannot price OpenRouter traffic. The id carries a vendor prefix, OpenRouter adds its own margin, and the provider serving a request can change without the id changing."`
- LiteLLM and any Anthropic-wire gateway:
  `"Replay cannot price this traffic. The upstream is HOST, not api.anthropic.com, so the wire is first-party and the bill is set by whoever runs that gateway."`

Token counts, cache hit rates, break causes and avoidable TOKENS stay fully
reportable in every one of these cases. Only the dollar column is withheld.
That is the existing `--max-session-tokens` argument from
`docs/TOKEN-PRICES.md`, one layer up: a token figure needs no price table and
cannot go stale.

#### 7. Architecture recommendation

**`internal/cachemodel`, as a route gate in front of the price table. Not
`othersurfaces.go`, not a second price table, not a proxy guard.**

Not `othersurfaces.go`: that file answers "is there a directory of records on
this machine". A route is a property of a REQUEST, not a home. Bedrock leaves
no `~/.bedrock`. Adding route rows there would put a detector in a file whose
whole contract (`dir` non-empty) they cannot satisfy.

Not the proxy: MEASURED above, the proxy already refuses to parse native
Bedrock, Vertex and Azure paths, so the bug is not there. The offline readers
are where a route id becomes a dollar figure. A proxy-only guard would leave
`replay cost` and `replay burn` wrong.

Not a second price table, yet. A second table needs numbers nobody in this
repository has read from a dated primary source, and `ModelRule` has no
`writeMult`, so today a second table would be silently wrong on every cache
write. Shipping a table populated from recollection is the exact failure this
project exists to refuse.

So: move `classifyRoute` down into `cachemodel` and make `PriceFor`,
`ReadMultiplierFor` and `MinCacheablePrefix` consult it before `lookup`. An id
that names a route and has no row published for that route returns
`priced: false`, which every caller already handles: the unpriced counter
(`internal/proxy/state.go:215-218,977`), `CapNotEnforced`
(`internal/proxy/guards.go:86-88`), the burn "could not be priced" line
(`cmd/replay/burn.go:96-111`). This converts a wrong number into a stated
absence using machinery that already exists, and it makes the apply path agree
with the check path that `bareClaudeName` already implements one file away.

Two things follow from it, both small:
1. The proxy must record the upstream host on the ledger record, and the
   offline readers must decline a first-party claim when it is not
   `api.anthropic.com`. Without it the gateway case at `/anthropic/v1/messages`
   stays undetectable from the id alone.
2. `classifyRoute` must learn `anthropic/*` and `openrouter/*`, which it
   MEASURABLY classifies as "other" today.

The second price table comes later, when a dated document has actually been
read and `writeMult` exists on the schema.

#### 8. Highest-severity correctness risk

A Bedrock or Vertex user runs `replay cost`. Replay reads their Claude Code
transcripts, matches `us.anthropic.claude-opus-5-v1:0` into the first-party
Opus 5 row by substring, prices it at $5/$25 per million with a 0.10 cache-read
multiple and a 1.25x cache-write multiple, prints a dollar total and an
avoidable-spend figure, and stamps both with `PriceTableVersion 2026-09-07` and
`RulesVersion anthropic-2026-09-01`. On the share card it prints "routed via
Bedrock, metered" directly beside that total. Every number is stated with the
confidence of a measurement, the provenance line names a publisher who did not
set those rates, and nothing anywhere says the table might be the wrong one.

ADR-0015 already wrote down that these routes have "different economics" and
that Replay "would be pricing traffic this project has never once observed".
The gap between that sentence and the code is the risk: the detector exists
(`classifyRoute`), the refusal precedent exists (`bareClaudeName`), and neither
is connected to the price.

### AGENT-1 — VS Code / editor extension family

Surveyed on this machine 2026-09-12. Evidence tiers used below:

- **VERIFIED-ABSENT** — probed this machine, not there.
- **VERIFIED-PRESENT** — probed this machine, there, with files.
- **VERIFIED-UPSTREAM** — read out of the extension's own source at HEAD today
  (file and line cited). Tells you what the code writes. Does **not** tell you
  that this machine has it, and does not prove the on-disk directory spelling.
- **UNVERIFIED** — neither. Says what would settle it.

#### Machine survey, first

There is no VS Code on this machine at all.

| Probe | Result |
|---|---|
| `~/Library/Application Support/Code` | VERIFIED-ABSENT |
| `~/Library/Application Support/Code - Insiders` | VERIFIED-ABSENT |
| `~/Library/Application Support/VSCodium` | VERIFIED-ABSENT |
| `~/Library/Application Support/Windsurf`, `/Trae`, `/Kiro`, `/Positron` | VERIFIED-ABSENT |
| `~/.vscode/extensions`, `~/.vscode-insiders` | VERIFIED-ABSENT |
| `~/.continue` | VERIFIED-ABSENT |
| `~/.copilot`, `~/.config/github-copilot` | VERIFIED-ABSENT |
| `~/.local/share/kilo`, `~/.local/share/opencode`, `~/.kilo`, `~/.kilocode` | VERIFIED-ABSENT |
| `~/.cline`, `~/.roo` | VERIFIED-ABSENT |
| `code` / `code-insiders` on PATH | VERIFIED-ABSENT |
| `continue` on PATH | resolves to the **zsh builtin**, not a Continue binary. A `command -v continue` check is a false positive and must not be used as a detector. |

Cursor is installed and is a VS Code fork, so it can host these same
extensions. It does not host any of them here:

- `~/.cursor/extensions` VERIFIED-PRESENT, contents are only
  `anysphere.remote-containers-*` and `anysphere.remote-ssh-*`.
- `~/Library/Application Support/Cursor/User/globalStorage` VERIFIED-PRESENT,
  contents are only `anysphere.*`, `state.vscdb`, `conversation-search.db`.
- `.../globalStorage/{saoudrizwan.claude-dev, rooveterinaryinc.roo-cline,
  kilocode.kilo-code, continue.continue, github.copilot-chat}` all
  VERIFIED-ABSENT.
- `.../workspaceStorage/*/chatSessions` VERIFIED-ABSENT (7 workspaces, none has one).

**So every directory in this section is VERIFIED-ABSENT on this machine.** No
format claim below was made by reading a file here. Each was read out of the
vendor's source, and is labelled that way. What would upgrade any of them to
VERIFIED-PRESENT: one machine with that extension installed and one task run.

One further thing this machine cannot settle: VS Code lowercases
`publisher.name` when it makes the globalStorage folder. Every extension id
below is derived from the vendor's `package.json` `publisher` and `name`
fields, lowercased by that rule. The rule is not verified here, because there
is no globalStorage directory here to check it against. Roo is the one that
matters, since its publisher is mixed case (`RooVeterinaryInc`).

#### Cline

- **dir candidates**
  - `~/Library/Application Support/Code/User/globalStorage/saoudrizwan.claude-dev/tasks/<taskId>/` — VERIFIED-ABSENT here. Extension id VERIFIED-UPSTREAM: `apps/vscode/package.json` has `"publisher": "saoudrizwan"`, `"name": "claude-dev"`. Task dir shape VERIFIED-UPSTREAM: `apps/vscode/src/core/storage/disk.ts:52` is `getGlobalStorageDir("tasks", taskId)`.
  - Same path under `Cursor/`, `Code - Insiders/`, `VSCodium/`, `Windsurf/` — VERIFIED-ABSENT here for Cursor, UNVERIFIED elsewhere.
  - Linux `~/.config/Code/User/globalStorage/...` — UNVERIFIED, this is a macOS machine.
- **artefact format**: JSON files per task, not jsonl. VERIFIED-UPSTREAM, `disk.ts:17-35`: `api_conversation_history.json`, `ui_messages.json`, `context_history.json`, `task_metadata.json`, and a per-task `settings.json` (`disk.ts:199`).
- **token usage recorded?** Yes. `ui_messages.json` carries `say: "api_req_started"` entries whose `text` is JSON with `tokensIn`, `tokensOut`, `cost`. VERIFIED-UPSTREAM: `apps/vscode/src/shared/ExtensionMessage.ts:369-378` (`ClineApiReqInfo`) and `apps/vscode/src/shared/getApiMetrics.ts`, which parses exactly those keys back out.
- **cache tokens recorded?** Yes, both directions. `cacheWrites` and `cacheReads` are separate fields on the same payload (`ExtensionMessage.ts:373-374`), and `getApiMetrics.ts` sums them into `totalCacheWrites` / `totalCacheReads`. Cline is one of the few surfaces that records a cache **write**, which is the field Anthropic's own wire does not always give back.
- **the obstacle**: `ClineApiReqInfo` has no model id. Per-task `modelId` and `apiProvider` exist on `HistoryItem` (`apps/vscode/src/shared/HistoryItem.ts`), but `taskHistory` is a VS Code **globalState** key (`apps/vscode/src/shared/storage/state-keys.ts:71`), which VS Code persists into `globalStorage/state.vscdb`, a SQLite file. Tokens are in a JSON file; the model that priced them is in a database.
- **prompt caching on the route?** Depends entirely on the configured provider, which is why the model id matters. Cline is BYOK plus a first-party Cline provider plus OpenRouter. The price table is not knowable from the token file alone. UNVERIFIED which routes support caching; settling it means reading Cline's provider list, not this machine.
- **PASS**: Replay can price Cline when, for a task directory, it can read `ui_messages.json` for the token and cache counts **and** obtain that task's `modelId` and `apiProvider` — either from the per-task `settings.json` (contents not read, this is the cheap thing to check first) or by opening `state.vscdb` and parsing the `taskHistory` value.
- **FAIL**: no model id reachable without SQLite, so the counts cannot select a price table.
- `why:` `Replay cannot read Cline yet, because its task files count cache tokens but never name the model, and the model id sits in the VS Code state database this build does not open`

#### Roo Code

- **dir candidates**
  - `~/Library/Application Support/Code/User/globalStorage/rooveterinaryinc.roo-cline/tasks/<taskId>/` — VERIFIED-ABSENT here. Id VERIFIED-UPSTREAM: `src/package.json` `"publisher": "RooVeterinaryInc"`, `"name": "roo-cline"`, v3.53.0. Lowercasing assumed, see caveat above.
  - **A user-set override exists and defeats a fixed path.** VERIFIED-UPSTREAM, `src/utils/storage.ts` `getStorageBasePath`: the `customStoragePath` setting relocates the whole tasks tree. A detector that probes only globalStorage reports VERIFIED-ABSENT for a user who moved it.
- **artefact format**: JSON per task. VERIFIED-UPSTREAM, `src/shared/globalFileNames.ts`: `api_conversation_history.json`, `ui_messages.json`, `task_metadata.json`, `history_item.json`, `_index.json`.
- **token usage recorded?** Yes. `history_item.json` is `historyItemSchema` (`packages/types/src/history.ts`) with required `tokensIn`, `tokensOut`, `totalCost`.
- **cache tokens recorded?** Yes, both directions, but **optional**. `cacheWrites` and `cacheReads` are `z.number().optional()` on the same schema, and `totalCacheWrites`/`totalCacheReads` are optional on `tokenUsageSchema` (`packages/types/src/message.ts:283-288`). Absent means the provider did not report it, which is not the same as zero, and Replay must not print it as zero.
- **the obstacle**: `historyItemSchema` has `apiConfigName`, a **provider profile name** the user chose. It has no model id. A profile called "work" does not select a price.
- **prompt caching on the route?** Route depends on the profile. `ApiMessage` in `src/core/task-persistence/apiMessages.ts` carries OpenRouter-shaped `reasoning_details`, so OpenRouter is a live route; BYOK Anthropic is another. UNVERIFIED which, per install.
- **PASS**: Replay can price Roo when it resolves `apiConfigName` to a concrete model id, via the settings directory (`getSettingsDirectoryPath`, contents not read), and when it treats absent `cacheReads`/`cacheWrites` as NOT RECORDED rather than 0.
- **FAIL**: profile name only, no model.
- `why:` `Replay cannot read Roo Code yet, because its task history records tokens under a provider profile name the user chose, and a profile name does not select a price table`

#### Kilo Code

**This one has changed shape and any 2025 memory of it is wrong.** Kilo Code
was a Roo fork with the same JSON task tree. At v7.6.2 it is built on
OpenCode: the repo root `package.json` runs `--cwd packages/opencode` with
`KILO_CLIENT=cli`, and the VS Code piece is `packages/kilo-vscode`. Storage is
now SQLite, not JSON task directories.

- **dir candidates**
  - `~/.local/share/kilo/kilo.db` (plus `kilo.db-wal`, `kilo.db-shm`) — VERIFIED-ABSENT here. VERIFIED-UPSTREAM: `packages/kilo-docs/pages/code-with-ai/agents/session-history.md` gives this as the macOS and Linux default and Windows as `%USERPROFILE%\.local\share\kilo\kilo.db`.
  - The same doc says the path moves under `KILO_DB`, `XDG_DATA_HOME`, a dev channel, or an isolated dev environment, and tells you to run `kilo db path` rather than assume. A hardcoded probe is wrong for those users.
  - Old Roo-shaped `globalStorage/kilocode.kilo-code/tasks/` — VERIFIED-ABSENT here, UNVERIFIED whether current versions still write it. Id VERIFIED-UPSTREAM from `packages/kilo-vscode/package.json`: `"publisher": "kilocode"`, `"name": "kilo-code"`.
- **artefact format**: SQLite. Tables `session`, `message`, `part`, with payloads in JSON columns read via `json_extract(..., '$.role')` etc. VERIFIED-UPSTREAM from the same doc's worked queries.
- **token usage recorded?** Yes, and this is the richest schema in this family. VERIFIED-UPSTREAM, `packages/opencode/src/session/message.ts:130-140`: message metadata carries `cost` and `tokens: { input, output, reasoning, cache: { read, write } }`, all `Schema.Finite`, i.e. **required**, not optional.
- **cache tokens recorded?** Yes, read and write, separately, required.
- **prompt caching on the route?** Kilo routes through its own gateway or BYOK. UNVERIFIED which price table applies per install; the model is in the row, so it is answerable once the DB is open.
- **PASS**: Replay can price Kilo when it opens SQLite and reads `message.data` metadata. Nothing is missing from the data; the only gap is the reader.
- **FAIL**: this build reads files, not databases.
- `why:` `Replay cannot read Kilo Code yet, because its sessions are rows in a SQLite database this build does not open, and the path moves with KILO_DB and XDG_DATA_HOME`

#### Continue.dev

- **dir candidates**
  - `~/.continue/sessions/<sessionId>.json` and `~/.continue/sessions/sessions.json` — VERIFIED-ABSENT here. VERIFIED-UPSTREAM: `core/util/paths.ts:78-107`.
  - `~/.continue/dev_data/<schemaVersion>/<eventName>.jsonl` — VERIFIED-ABSENT here. VERIFIED-UPSTREAM: `core/util/paths.ts:229-248`.
  - a dev-data SQLite (`getDevDataSqlitePath`, `paths.ts:236`) — VERIFIED-ABSENT here.
  - **Not** a VS Code globalStorage directory. Continue keeps its own home, so the globalStorage sweep misses it entirely.
- **artefact format**: three at once. Session JSON, event JSONL, and SQLite.
- **token usage recorded?** Yes in all three, but they do not record the same thing, and the difference decides whether a bill is possible.
  - `dev_data` `tokensGenerated` events carry exactly `model`, `provider`, `promptTokens`, `generatedTokens`. VERIFIED-UPSTREAM, `packages/config-yaml/src/schemas/data/tokensGenerated/index.ts`.
  - the dev-data SQLite table is `tokens_generated` with columns `tokens_prompt`, `tokens_generated` and nothing else. VERIFIED-UPSTREAM, `core/data/devdataSqlite.ts:16-49`.
  - session JSON has an **optional** `usage?: SessionUsage`, and `Usage` does carry `promptTokensDetails.cachedTokens` and `promptTokensDetails.cacheWriteTokens`, the latter commented in source as Anthropic-specific. VERIFIED-UPSTREAM, `core/index.d.ts:274-289` and `:404-421`.
- **cache tokens recorded?** Split answer, and this is the finding. **The `dev_data` logs and the `tokens_generated` table record no cache field at all.** Cache read and cache write exist only on the optional `usage` object inside the session JSON, only when the provider returned them. Priced off `dev_data` alone, every cached read is billed at full input rate, which for a caching Anthropic route overstates the bill several times over. That failure is silent: the numbers look complete.
- **prompt caching on the route?** Yes on some. Continue is BYOK across many providers; `core/llm/llms/Anthropic.ts` and `cacheBehavior` / `cacheSystemMessage` / `cacheConversation` config options exist (`core/index.d.ts:674, 1074-1075`). So caching is real on this surface and the cheap artefact is the one that omits it.
- **PASS**: Replay can price Continue when it reads `~/.continue/sessions/*.json`, finds `usage.promptTokensDetails`, and has a model id for the session (`chatModelTitle`, which is a title, not necessarily a model id — UNVERIFIED whether it resolves).
- **FAIL**: reading `dev_data` instead, because it has no cache column.
- `why:` `Replay cannot read Continue yet, because its dev_data logs record prompt and generated tokens with no cache column, so a cached read there is indistinguishable from a full-price one`

#### GitHub Copilot, agent mode in VS Code

- **dir candidates**
  - `~/Library/Application Support/Code/User/workspaceStorage/<workspaceId>/chatSessions/*.json` — VERIFIED-ABSENT here (no VS Code; and the 7 Cursor workspaces have no `chatSessions`). VERIFIED-UPSTREAM: `microsoft/vscode`, `src/vs/workbench/contrib/chat/common/model/chatSessionStore.ts:72`.
  - `.../globalStorage/emptyWindowChatSessions/` for windows with no folder open, and `.../globalStorage/transferredChatSessions/` — VERIFIED-UPSTREAM, same file, lines 72 and 79. Both VERIFIED-ABSENT here.
  - note `chatSessionStore.ts:152` accepts both `.json` and `.jsonl` children.
  - **These are written by VS Code core, not by the Copilot extension.** So the transcript is there for any chat participant, and probing `globalStorage/github.copilot-chat` is probing the wrong place.
- **artefact format**: JSON (or JSONL) per session, one directory per workspace hash.
- **token usage recorded?** Yes, more than expected. `ISerializableChatResponseData` (`src/vs/workbench/contrib/chat/common/model/chatModel.ts:1893-1916`) serializes `promptTokens`, `completionTokens`, `outputBuffer`, `copilotCredits`, `sessionCopilotCredits`, and `modelTotals`. `IChatUsageModelTotal` (`.../chatService/chatService.ts:177-182`) is `{ model, inputTokens, cachedTokens, outputTokens }`, so the model id is in the record.
- **cache tokens recorded?** **Read only.** `cachedTokens` exists; there is no cache-write field anywhere in `IChatUsage` or `IChatUsageModelTotal`. Priced against an Anthropic table, every cache write would have to be inferred, and inferring it is guessing.
- Every one of those fields is optional (`?`) for backward compatibility, so an older session file may carry a transcript and no counts at all. Presence must be tested per record, never assumed from one sample.
- **prompt caching on the route?** Caching is clearly happening upstream, since `cachedTokens` is reported. But it does not reach the bill. GitHub bills Copilot in **premium requests with a per-model multiplier**, not tokens (docs.github.com, copilot-requests: "Each model has a premium request multiplier"; Copilot Chat is one premium request per prompt times the model rate; Copilot code review is 13). `copilotCredits` in the session file is the field that corresponds to the actual bill.
- **PASS**: Replay can report Copilot when it sums `copilotCredits` per session and prints credits, naming them as credits. It can report tokens as a **context** figure. It must not multiply Copilot tokens by any vendor token price and call the result a bill.
- **FAIL**: pricing it by tokens at all, on the subscription route.
- `why:` `Replay cannot price GitHub Copilot yet, because the subscription bills premium requests with a per-model multiplier and not tokens, so the token counts in the session file are not the bill`

#### GitHub Copilot CLI

Sits between this scope and AGENT-2's. Recorded here because it shares
Copilot's billing model, which is the fact that decides it.

- **dir candidates**
  - `~/.copilot/` — VERIFIED-ABSENT here. VERIFIED-UPSTREAM from docs.github.com (cli-config-dir-reference): default `$HOME/.copilot`, overridden wholesale by **`COPILOT_HOME`**.
  - inside it: `session-state/` (session history and workspace data), `session-store.db` (SQLite, cross-session data), `logs/`, `command-history-state/`. Same doc. `history-session-state/` is named in secondary sources as the older layout — UNVERIFIED, not in the official reference I read.
  - per-session `events.jsonl` and `workspace.yaml` are UNVERIFIED. Reported by secondary sources only; the GitHub docs page on session data does not name any file or field. Settling it needs one machine with Copilot CLI and one session run.
- **artefact format**: mixed, JSONL plus SQLite. Partly UNVERIFIED as above.
- **token usage recorded?** UNVERIFIED. The docs mention a `/chronicle cost-tips` command that "analyzes your token usage", which implies token data exists locally, but no schema is published and I did not see a field name. Do not assume.
- **cache tokens recorded?** UNVERIFIED, same reason.
- **prompt caching on the route?** Same premium-request billing as above, unless the user configures BYOK, which `~/.copilot/providers.json` ("user-level BYOK provider and model registry") shows is supported. BYOK changes which price table applies and is the only route where a token price is the right one.
- **PASS**: same as Copilot agent mode. Credits, not tokens, unless a BYOK provider is configured in `providers.json`, in which case that provider's table applies.
- **FAIL**: claiming a token bill with no verified schema behind it.
- `why:` `Replay cannot price the GitHub Copilot CLI yet, because it bills premium requests rather than tokens and this build has not seen its session files to know what they record`

#### Draft literals

These do not drop into `knownSurfaces` as written. Three problems, stated
rather than papered over:

1. `otherSurface.dir` is one string and `firstWithEntries` takes one path.
   Every surface here has several real candidates (stable and Insiders and
   VSCodium and Cursor for the globalStorage family; the `customStoragePath`,
   `KILO_DB`, `XDG_DATA_HOME` and `COPILOT_HOME` overrides). A one-path probe
   returns VERIFIED-ABSENT for real users. `firstWithEntries` should take a
   variadic.
2. These are macOS paths. `~/Library/Application Support` is wrong on Linux
   (`~/.config`) and Windows (`%APPDATA%`). Replay needs a per-OS helper before
   any of these literals is honest on another platform.
3. `hasEntries` only counts non-directory children. `globalStorage/<id>/` for
   Cline and Roo contains the `tasks/` **directory** and may contain no loose
   file, so `hasEntries` on the extension root can answer false on an install
   that has real data. Probe the `tasks` directory, or teach `hasEntries` about
   subdirectories.

```go
// vscodeGlobalStorage is the macOS globalStorage root for a VS Code family
// editor. It is macOS-only on purpose: Linux uses ~/.config and Windows uses
// %APPDATA%, and a path that is wrong on two platforms is worse than one that
// says which platform it answers for.
func vscodeGlobalStorage(home, app string) string {
	return filepath.Join(home, "Library", "Application Support", app, "User", "globalStorage")
}

// Cline. The tokens are in ui_messages.json; the model that prices them is in
// state.vscdb. Probe tasks/, not the extension root: the root can hold no
// loose file on a live install.
{
	name: "Cline",
	dir:  firstWithEntries(filepath.Join(vscodeGlobalStorage(home, "Code"), "saoudrizwan.claude-dev", "tasks")),
	why:  "Replay cannot read Cline yet, because its task files count cache tokens but never name the model, and the model id sits in the VS Code state database this build does not open",
},

// Roo Code. customStoragePath relocates this whole tree, so an absent result
// here is not proof of an absent install.
{
	name: "Roo Code",
	dir:  firstWithEntries(filepath.Join(vscodeGlobalStorage(home, "Code"), "rooveterinaryinc.roo-cline", "tasks")),
	why:  "Replay cannot read Roo Code yet, because its task history records tokens under a provider profile name the user chose, and a profile name does not select a price table",
},

// Kilo Code. Built on OpenCode since v7; sessions are SQLite rows, and the
// message metadata carries input, output, reasoning, cache read and cache
// write as required fields. The data is complete. The reader is not.
{
	name: "Kilo Code",
	dir:  firstWithEntries(filepath.Join(home, ".local", "share", "kilo")),
	why:  "Replay cannot read Kilo Code yet, because its sessions are rows in a SQLite database this build does not open, and the path moves with KILO_DB and XDG_DATA_HOME",
},

// Continue. Keeps its own home, so the globalStorage sweep never sees it.
{
	name: "Continue",
	dir:  firstWithEntries(filepath.Join(home, ".continue", "sessions")),
	why:  "Replay cannot read Continue yet, because its dev_data logs record prompt and generated tokens with no cache column, so a cached read there is indistinguishable from a full-price one",
},

// GitHub Copilot in VS Code. The transcript is written by VS Code core into
// workspaceStorage/<id>/chatSessions, not by the extension, so there is no
// single directory to probe; this literal is a placeholder and the real probe
// needs a glob over workspaceStorage.
{
	name: "GitHub Copilot",
	dir:  firstWithEntries(filepath.Join(home, "Library", "Application Support", "Code", "User", "workspaceStorage")),
	why:  "Replay cannot price GitHub Copilot yet, because the subscription bills premium requests with a per-model multiplier and not tokens, so the token counts in the session file are not the bill",
},

// GitHub Copilot CLI. COPILOT_HOME moves this.
{
	name: "GitHub Copilot CLI",
	dir:  firstWithEntries(filepath.Join(home, ".copilot", "session-state")),
	why:  "Replay cannot price the GitHub Copilot CLI yet, because it bills premium requests rather than tokens and this build has not seen its session files to know what they record",
},
```

#### What AGENT-1 could not verify

Nothing in this section was read off this machine, because none of these
extensions is installed here and VS Code is not installed here. Directory
spellings are derived from vendor `package.json` files, not observed;
lowercasing of `publisher.name` is assumed and matters most for Roo. Copilot
CLI's per-session file names and fields are the weakest claim on the page and
are marked UNVERIFIED throughout. Each becomes VERIFIED-PRESENT the same way:
one machine with the extension installed and one completed task.

### AGENT-3

Scope: IDE-native agents. Surveyed 2026-09-12 on this machine (darwin 25.5.0).
Cursor 3.17.19 is the only IDE agent installed here. Every other surface below
is VERIFIED-ABSENT on this machine, which is not the same as a verified path:
absence proves only that nothing is at the path, never that the path is right.
Those rows are tagged accordingly and must not merge until someone confirms
them VERIFIED-PRESENT on a machine where the product actually runs.

Probe used for absence: `find ~ -maxdepth 4 -iname '*windsurf*' -o -iname
'*codeium*' -o -iname '*zed*' -o -iname '*jetbrains*' -o -iname '*junie*'`
returned no product directories, and /Applications holds Cursor.app alone.

#### CORRECTION: the shipped Cursor string is imprecise

`othersurfaces.go` currently prints:

    Replay cannot read Cursor yet — its local history carries structure but no
    token usage

"no token usage" reads as "there is no such field". That is not what is on
disk. Cursor writes a token usage field on every message and leaves it at zero.

Evidence, `~/Library/Application Support/Cursor/User/globalStorage/state.vscdb`,
657 MB SQLite, tables `ItemTable`, `cursorDiskKV`, `composerHeaders`:

  - `cursorDiskKV` key prefixes: `bubbleId:` 29665, `agentKv:` 30317,
    `composerData:` 234, `checkpointId:` 583, `ofsContent:` 653,
    `codeBlockPartialInlineDiffFates:` 658, `inlineDiff:` 29
  - `composerHeaders`: 218 rows, 2026-07-09 to 2026-08-12
  - all 29665 `bubbleId:` rows carry `"tokenCount": {"inputTokens": N,
    "outputTokens": N}`; 0 rows missing the field
  - rows with inputTokens > 0: **0** (sum 0, max 0)
  - rows with outputTokens > 0: **0** (sum 0, max 0)
  - no `cacheRead`, `cacheWrite` or `cachedTokens` key in any `bubbleId:`,
    `agentKv:` or `composerData:` row
  - `composerData` `usageData`: present on all 234 composers, value `{}` on all
    234
  - `composerData` `contextUsagePercent`: present and non-zero (7.06, 13.56,
    8.16), but a fraction of the context window, not a billed token count
  - `agentKv` "usage" substring hits are chat body text (UsageBar component
    source), not telemetry

Other Cursor artefacts, all checked, none carrying usage:

  - `~/.cursor/projects/*/agent-transcripts/<id>/<id>.jsonl`, 118 files, keys
    `role` `message` `type` `status` `error` only
  - `~/.cursor/ai-tracking/ai-code-tracking.db`, SQLite: `ai_code_hashes`,
    `scored_commits`, `conversation_summaries`, `tracked_file_content`,
    `ai_deleted_files`, `tracking_state`. Line counts, AI percentage and model
    name. No tokens.
  - `conversation-search.db`, SQLite FTS5 over titles and bodies, no usage
    columns
  - 7 `workspaceStorage/*/state.vscdb`: 0 rows matching inputTokens, cacheRead
    or tokenCount
  - `~/Library/Application Support/Cursor/logs`: no `cache_read_input_tokens`,
    `inputTokens` or `outputTokens`

Why the distinction is worth shipping: "no token usage" invites the fix "write
a parser". The counters are there and empty, so no parser recovers them. The
data was never written, and a reader who wants Cursor priced has to change what
Cursor records, not what Replay reads. Proposed replacement string in the row
below.

#### Rows

**Cursor** (existing row, correction only)
- dir: `~/.cursor` VERIFIED-PRESENT (5 top-level files);
  `~/Library/Application Support/Cursor/User/globalStorage` VERIFIED-PRESENT
  (11 top-level files, holds the actual chat records)
- format: SQLite. `state.vscdb`, table `cursorDiskKV`, key prefix `bubbleId:`,
  one JSON message per row
- token usage recorded? Field present on every row, value zero on every row.
  cache tokens? No such field at all.
- prompt caching on that route? Cursor bills its own plan units, not provider
  tokens; nothing on disk states a cache rate. NOT RECORDED.
- PASS: a `bubbleId:` row with a non-zero `tokenCount.inputTokens` on a machine
  running a later Cursor build. Then the field is live and worth parsing.
- FAIL (current state): the counters are zero, so nothing can be priced.
  why: `Replay cannot read Cursor yet: every message row in its state.vscdb carries a tokenCount field and every one of them is zero`

**Zed agent panel** (the one genuinely priceable surface in this scope)
- dir: `~/Library/Application Support/Zed/threads` VERIFIED-ABSENT here, path
  itself UNVERIFIED. Derived from source, not memory: `crates/agent/src/db.rs`
  opens `paths::data_dir().join("threads")` then `threads.db`, and
  `crates/paths/src/paths.rs` makes `data_dir()` on macOS
  `~/Library/Application Support/Zed`. Linux candidate `~/.local/share/zed/threads`
  UNVERIFIED.
- format: SQLite. Table `threads(id, summary, updated_at, data_type, data BLOB,
  parent_id, folder_paths, folder_paths_order, created_at)`. The `data` BLOB is
  the thread serialised to JSON then zstd-compressed, with `data_type` = `zstd`.
- token usage recorded? Yes. The persisted `DbThread` carries
  `cumulative_token_usage` and a per-message `request_token_usage` map.
  cache tokens? Yes, separately: `TokenUsage` has four fields, `input_tokens`,
  `output_tokens`, `cache_creation_input_tokens`, `cache_read_input_tokens`.
- prompt caching on that route? Yes. Those fields are populated from the
  provider response, Anthropic included, so a BYOK Anthropic thread persists
  real cache reads and writes.
- Trap for whoever writes the reader: the usage fields are
  `#[serde(default, skip_serializing_if = "is_default")]`, so a zero is omitted
  from the stored JSON rather than written as 0. A missing key means zero, not
  missing data, and a reader that treats absent as unknown will under-report.
  Second trap: `ZED_STATELESS` swaps the DB for an in-memory one and persists
  nothing.
- PASS: decompress the `data` BLOB, read `cumulative_token_usage`, treat absent
  fields as zero. Zed is then fully priceable including cache, which puts it
  level with Codex rather than with Cursor.
- FAIL (this build): no zstd decode in the binary.
  why: `Replay cannot read Zed yet: its thread database stores each thread as zstd-compressed JSON, which this build does not decompress`

**Windsurf / Codeium Cascade**
- dir: `~/.codeium/windsurf/cascade` VERIFIED-ABSENT here, path itself
  UNVERIFIED but first-party documented as holding conversation history;
  `~/.windsurf/transcripts` VERIFIED-ABSENT here, path itself UNVERIFIED,
  documented for hook transcripts
- format: `~/.windsurf/transcripts/{trajectory_id}.jsonl`, one JSON object per
  step with `type` and `status` plus step-specific data, mode 0600, only the
  100 most recent kept. The format inside `.codeium/windsurf/cascade` is NOT
  DOCUMENTED and nobody here has opened one.
- token usage recorded? NOT RECORDED in the documented transcript schema. Not
  verified either way for the cascade directory.
  cache tokens? NOT RECORDED.
- prompt caching on that route? Yes at the billing layer. The rate card lists
  input, output and cache costs per million per model, and local agents convert
  consumed tokens into Agent Compute Units at those rates. So cache is priced
  upstream, and the local artefact shows none of it.
- PASS: a `.jsonl` step object, or a file under `cascade`, carrying a token
  count. Nobody has seen one.
- FAIL: the transcript records what the agent did, not what it spent.
  why: `Replay cannot read Windsurf yet: its Cascade transcripts record a step type and status per line and never a token count`

**JetBrains AI Assistant** (in-IDE)
- dir: no candidate. `~/Library/Application Support/JetBrains` VERIFIED-ABSENT
  here, and more to the point JetBrains documents no on-disk location or format
  for AI Assistant chat history at all. Proposing a path would be inventing one.
- format: NOT DOCUMENTED
- token usage recorded? NOT DOCUMENTED locally. Quota is computed from tokens
  server side and shown in the account console. cache tokens? NOT DOCUMENTED.
- prompt caching on that route? NOT DOCUMENTED for the credit route. BYOK bills
  at the provider, where caching applies and Replay could price it if the IDE
  wrote the numbers down, which is not established.
- PASS: someone finds and names the file. Until then there is no row to ship.
- FAIL: nothing to point at.
  why: `Replay cannot read JetBrains AI yet: JetBrains documents no on-disk location for AI Assistant chat history`
- Recommendation: do not add a literal. A probe of a guessed path that finds
  nothing is exactly the failure this survey exists to prevent.

**JetBrains Junie**
- dir: `~/.junie` VERIFIED-ABSENT here, path itself UNVERIFIED but first-party
  documented (`~/.junie/config.json`, `~/.junie/trust`, `~/.junie/allowlist.json`)
- format: per session an `events.jsonl` event log beside a continuously updated
  `transcript.md`, subagent transcripts in a `subagents` folder. The parent
  directory of session folders is NOT DOCUMENTED, and the `events.jsonl` schema
  is not published.
- token usage recorded? Junie computes token usage and cost in-session and
  prints it via `/stats`. Whether those numbers reach `events.jsonl` is NOT
  DOCUMENTED and unverified. cache tokens? NOT DOCUMENTED.
- prompt caching on that route? On BYOK to Anthropic, yes upstream. Junie also
  offers a JetBrains API key on usage-based billing. Neither is confirmed to
  land in a local file.
- PASS: an `events.jsonl` line carrying a token count. One session on a machine
  with Junie installed settles it, and that is the cheapest open question here.
- FAIL: the log's schema is unpublished, so no field can be claimed.
  why: `Replay cannot read Junie yet: its session event log has no published schema, so this build cannot tell a token count from any other field`
- Overlap note for AGENT-2: Junie also ships a CLI. This row covers the
  artefact, which both surfaces share. Do not add a second Junie literal.

**Xcode 26 coding intelligence**
- dir: `~/Library/Developer/Xcode` VERIFIED-ABSENT here (Xcode is not installed
  on this machine), and the only documented `CodingAssistant` paths are agent
  config, not history: `~/Library/Developer/Xcode/CodingAssistant/codex` and
  `~/Library/Developer/Xcode/CodingAssistant/ClaudeAgentConfig`, which hold
  AGENTS.md / CLAUDE.md and skills. Path for history: none published.
- format: NOT DOCUMENTED. Apple documents conversation history only as a UI
  feature, a sidebar with a History slider that requires a Git repo.
- token usage recorded? NOT DOCUMENTED. Neither Xcode intelligence article
  mentions tokens, cost or metering. cache tokens? NOT DOCUMENTED.
- prompt caching on that route? Xcode takes an added chat provider over the
  Chat Completions API, and can host ACP agents. Caching then depends entirely
  on the provider, and Apple states nothing about it.
- Subscription note: for the built-in Claude and ChatGPT sign-in options Apple
  publishes no per-token billing terms. On that route there is nothing for
  Replay to price, and the honest output is to say so rather than imply a blind
  spot. The blind spot is real only for an added provider billing per token.
- PASS: a named history file carrying usage. Nothing published supports one.
- FAIL: config directory only.
  why: `Replay cannot read Xcode intelligence yet: Apple publishes a coding assistant config directory and no location for its conversation history`

**Visual Studio (Windows) Copilot agent mode**
- dir: none applicable. Visual Studio is Windows-only; there is no macOS
  install to probe and this build's home probes would never fire.
- format: NOT DOCUMENTED. Microsoft documents the chat history panel as UI and
  never a file location. The `%APPDATA%\Code\User\workspaceStorage\<hash>\
  chatSessions\*.json` path circulating online is VS Code, a different product,
  and belongs to AGENT-1, not here.
- token usage recorded? NOT DOCUMENTED as a file. Usage appears only in the
  Copilot Usage window and a context-window indicator that shows a percentage,
  not raw counts. cache tokens? NOT DOCUMENTED locally.
- prompt caching on that route? Yes upstream, and explicitly: Copilot usage is
  measured in AI Credits computed from input tokens, output tokens and cached
  tokens at the model's published rates. Cached tokens are named in the billing
  formula and absent from anything local.
- PASS: a documented on-disk session file. None exists.
- FAIL: nothing on disk, and nothing on this platform.
  why: `Replay cannot read Visual Studio yet: Microsoft documents its Copilot chat history as a panel and never as a file on disk`
- Recommendation: do not add a literal to a darwin and linux binary.

#### Draft literals

Match the current `firstWithEntries(dirs ...string)` signature. Only the Cursor
change is safe to merge today; the rest carry unverified paths and are marked.

```go
// SAFE TO MERGE: correction to an existing shipped string. The directory is
// unchanged; globalStorage is added first because it is where the chat records
// actually are, and ~/.cursor is the fallback for an install that has skills
// and transcripts but no conversation database yet.
{
	name: "Cursor",
	dir: firstWithEntries(
		filepath.Join(home, "Library", "Application Support", "Cursor", "User", "globalStorage"),
		filepath.Join(home, ".config", "Cursor", "User", "globalStorage"),
		filepath.Join(home, ".cursor"),
	),
	why: "Replay cannot read Cursor yet: every message row in its state.vscdb carries a tokenCount field and every one of them is zero",
},

// DO NOT MERGE UNTIL VERIFIED-PRESENT on a machine running Zed. The path is
// read out of zed's own source, not out of anyone's memory, and that is still
// not the same as having seen the directory.
{
	name: "Zed",
	dir: firstWithEntries(
		filepath.Join(home, "Library", "Application Support", "Zed", "threads"),
		filepath.Join(home, ".local", "share", "zed", "threads"),
	),
	why: "Replay cannot read Zed yet: its thread database stores each thread as zstd-compressed JSON, which this build does not decompress",
},

// DO NOT MERGE UNTIL VERIFIED-PRESENT on a machine running Windsurf. Note the
// order: transcripts is documented down to the line format, cascade is only
// documented as "conversation history", so the one we can describe comes first.
{
	name: "Windsurf",
	dir: firstWithEntries(
		filepath.Join(home, ".windsurf", "transcripts"),
		filepath.Join(home, ".codeium", "windsurf", "cascade"),
	),
	why: "Replay cannot read Windsurf yet: its Cascade transcripts record a step type and status per line and never a token count",
},

// DO NOT MERGE UNTIL VERIFIED-PRESENT on a machine running Junie.
{
	name: "Junie",
	dir:  firstWithEntries(filepath.Join(home, ".junie")),
	why:  "Replay cannot read Junie yet: its session event log has no published schema, so this build cannot tell a token count from any other field",
},

// DO NOT MERGE UNTIL VERIFIED-PRESENT on a machine running Xcode 26, and note
// this probe would name a config directory, not a record of spending. Naming a
// surface Replay cannot price is only worth it if the reader learns something,
// and here they learn that Apple keeps no usage record at all.
{
	name: "Xcode intelligence",
	dir:  firstWithEntries(filepath.Join(home, "Library", "Developer", "Xcode", "CodingAssistant")),
	why:  "Replay cannot read Xcode intelligence yet: Apple publishes a coding assistant config directory and no location for its conversation history",
},
```

No literal proposed for JetBrains AI Assistant (no documented path anywhere) or
Visual Studio (Windows-only, no documented file).

#### One defect in hasEntries that these rows expose

`hasEntries` counts only top-level regular files, so a surface that keeps its
records one level down reads as absent. `~/.codeium/windsurf/cascade` and
`~/Library/Developer/Xcode/CodingAssistant` are both plausibly directories of
directories. `~/Library/Application Support/Zed/threads` holds `threads.db`
directly and is fine. Whoever merges the unverified rows should confirm the
probe fires on a real install rather than assume it, because a probe that
silently never fires is the exact failure this survey is meant to prevent.

#### What would verify each UNVERIFIED row

One install each, and one command:

  - Zed: open the agent panel, send one message, then
    `ls ~/Library/Application\ Support/Zed/threads` and
    `sqlite3 threads.db "select id, data_type, length(data) from threads"`.
    Decompress one blob and confirm `cumulative_token_usage`.
  - Windsurf: run one Cascade conversation, then `ls ~/.windsurf/transcripts`
    and `ls -R ~/.codeium/windsurf/cascade`, and grep both for `token`.
  - Junie: run one session, find the session folder, and
    `grep -o '"[a-zA-Z_]*token[a-zA-Z_]*"' events.jsonl | sort -u`.
  - Xcode 26: run one intelligence conversation in a Git-backed project and
    `find ~/Library/Developer/Xcode -newermt '-10 minutes' -type f`.
  - Cursor (to retire the correction): on a build later than 3.17.19, rerun the
    zero-scan over `bubbleId:` rows and see whether any tokenCount is non-zero.

#### 9. External verification, and what it corrects in sections 5 and 8

Sections 1 to 4 are measurements against this tree and this machine and stand
unchanged. Sections 5 and 8 guessed at the DIRECTION of the pricing error
before the vendor documents had been read. They were wrong about the magnitude,
and the correction is recorded here rather than edited away.

What a primary-source pass established.

VERIFIED by fetch:

- **Bedrock minimum cacheable prefix matches first-party exactly.**
  `docs.aws.amazon.com/bedrock/latest/userguide/prompt-caching.html` carries the
  table: Opus 5 512, Sonnet 5 1024, Haiku 4.5 4096, Opus 4.8 1024,
  Opus 4.7/4.6/4.5 4096, Sonnet 4.6/4.5 1024. Same family tiers Replay compiles
  at `internal/cachemodel/anthropic.go:131-138`. Max 4 checkpoints, TTL 5m or
  1h. So section 5's UNVERIFIED on the Bedrock minimum prefix is now VERIFIED,
  and the answer is parity.
- **Bedrock reports cache tokens under different names on the Converse API**:
  `cacheReadInputTokens`, `cacheWriteInputTokens`, `cacheDetails`, with
  `total input = inputTokens + cacheReadInputTokens + cacheWriteInputTokens`.
  The InvokeModel path keeps the Anthropic names. Two shapes on one route.
- **OpenRouter is at full parity on the numbers.** `openrouter.ai/docs/features/prompt-caching`
  gives write 1.25x at 5m, 2x at 1h, read 0.1x, and the same family minimum
  prefixes. `openrouter.ai/anthropic` gives Opus 5 $5/$25, Sonnet 5 $2/$10,
  Haiku 4.5 $1/$5, Fable 5.1 $10/$50, which is Replay's table to the cent. The
  live `openrouter.ai/api/v1/models` row for `anthropic/claude-fable-5.1`
  carries `input_cache_read` 0.00000025 against `prompt` 0.00001, which is the
  0.025x read Replay already encodes as `readMultiplierNewest`.
  OpenRouter reports usage as `prompt_tokens_details.cached_tokens` and
  `cache_write_tokens`, OpenAI-style, not the Anthropic names.
- **"Azure OpenAI" is the wrong name for this route.** Claude on Azure is
  **Microsoft Foundry**, a different product from Azure OpenAI Service
  (`platform.claude.com/docs/en/build-with-claude/claude-in-microsoft-foundry`).
  Billing is pass-through: Anthropic rates the usage at standard per-model
  rates and converts to Claude Consumption Units at $0.01 per CCU. Model ids
  are **bare first-party strings** used as the deployment name, posted to
  `https://{resource}.services.ai.azure.com/anthropic/v1/messages`.
- **The one verified divergence is regional, not per-route.**
  `platform.claude.com/docs/en/about-claude/pricing` states that on Bedrock and
  Google Cloud, "regional and multi-region endpoints include a 10% premium over
  global endpoints", from Sonnet 4.5, Haiku 4.5 and Opus 4.5 onward. Foundry
  carries the same 1.1x on the US Data Zone Standard deployment type.

NOT VERIFIED, with the fetch failure named:

- **Bedrock current base prices.** `aws.amazon.com/bedrock/pricing/` was fetched
  three times and returned only a legacy Claude 3.5 Sonnet fragment each time,
  never the current rows. The page is JS-rendered. Settling source: that URL
  through a JS-capable fetch, or the AWS Price List API.
- **Vertex, essentially all of it.** `cloud.google.com/vertex-ai/generative-ai/pricing`
  and the Claude prompt-caching page under `.../partner-models/claude/` both
  returned navigation shell with no body, repeatedly, including through the
  `docs.cloud.google.com` redirect. Caching on Vertex with a TTL up to one hour
  is attested only by a Google blog post reached through search. Whether Vertex
  takes `cache_control` unchanged, its multipliers, its minimum prefixes and its
  response field names are all UNVERIFIED. Settling sources:
  `https://cloud.google.com/vertex-ai/generative-ai/pricing#claude-models` and
  `https://docs.cloud.google.com/vertex-ai/generative-ai/docs/partner-models/claude/prompt-caching`.

**So the corrected finding.** These routes are at or near price parity with
first-party on everything that has actually been read. Section 8 implied
Replay's Bedrock figure is wrong wholesale. On the evidence, it is not. What is
established is narrower and still disqualifying:

1. **Replay is wrong by a documented 10% on any Bedrock, Vertex or Foundry
   request served from a regional or multi-region endpoint**, and it has no
   field that can tell. The region is sometimes inside the model id
   (`us.anthropic.*`, `eu.anthropic.*`) and sometimes nowhere at all. Replay
   prints that figure to the cent with `PriceTableVersion 2026-09-07` on it.
2. **Foundry traffic is undetectable.** Its model ids are bare first-party
   strings, so `classifyRoute` returns "first-party API" and the share card
   prints that. Parity of price makes the number right by luck, not by
   measurement, and the route claim on the card is false.
3. **Bedrock and Vertex parity is unread, not established.** Nobody in this
   repository has fetched a current Bedrock or Vertex price. Section 5's
   parity-shaped cells rest on LiteLLM's static file and on search snippets,
   which is a second observer, not an authority, in exactly the sense
   `internal/cachemodel/anthropic.go:34-43` already defines.

The architecture recommendation in section 7 does not change, and the
correction strengthens one part of it. A second price table is now clearly the
wrong first move: the numbers it would hold are the numbers that turn out to
match. The thing Replay actually lacks is the ability to say which route and
which REGION a figure was computed for, and to decline the dollar column when
it cannot say. That is a gate and a recorded upstream, not a table.

Revised FAIL strings for the two rows the check changed:

- Microsoft Foundry:
  `"Replay cannot tell Foundry traffic from first-party. Foundry uses bare model ids and bills in Claude Consumption Units, so the route line on this card is a guess and is not printed."`
- Any Bedrock, Vertex or Foundry request:
  `"Replay prices this at the global endpoint rate. Regional and multi-region endpoints carry a 10% premium and the request does not record which one served it, so this figure is a lower bound."`

### AGENT-2 — terminal / CLI agents

Scope: Aider, Gemini CLI, Goose, OpenCode, Amp, Charm Crush, plus Qwen Code.
Claude Code and Codex excluded (already read).

**Machine result first: none of these are installed here.** `command -v` returns
nothing for aider, gemini, goose, opencode, amp, crush. Global npm carries only
`@anthropic-ai/claude-code`, `@openai/codex`, `openclaw`, and non-agent tools.
Homebrew and pipx carry none of them. A depth-4 sweep of `~/Development` and a
depth-3 sweep of `~` found no `.aider*`, `.crush`, `.opencode`, `.goose*`,
`.amp`, or `.gemini` anywhere, project-local included. Every dir below is
therefore VERIFIED-ABSENT: checked with `test -e`, not assumed.

Because nothing is installed, **no artefact on this machine was opened.** Format,
field names and token accounting below come from reading upstream source at a
pinned version, listed per surface. That is evidence, not recollection, but it is
not observation of a real file: tagged SOURCE-VERIFIED, and every one of them
needs one real run to confirm before Replay prints a number from it.

Two repos moved, which any hardcoded URL in this build will need to follow:
`sst/opencode` now redirects to `anomalyco/opencode` (branch `dev`), and
`block/goose` to `aaif-goose/goose`.

#### Summary

| Surface | Local artefact | Tokens? | Cache read / write? | Verdict |
|---|---|---|---|---|
| Goose | sqlite | yes | yes / yes | PASS, richest of the set |
| OpenCode | json per message | yes | yes / yes | PASS |
| Gemini CLI | jsonl | yes | read only | PASS on price, cache write not billable that way |
| Qwen Code | jsonl | yes | read only | PASS on price, same cache shape |
| Crush | sqlite | partial | no / no | FAIL, cache split never persisted |
| Aider | markdown prose | rounded | yes / yes, but rounded | FAIL, counts rounded to 1k |
| Amp | settings only | UNVERIFIED | UNVERIFIED | FAIL, nothing priceable found |

#### Aider

- `~/.aider/` VERIFIED-ABSENT. Created on **every** run regardless of opt-in:
  `Analytics.__init__` calls `get_or_create_uuid()`, which calls
  `get_data_file_path()`, which does `mkdir(parents=True, exist_ok=True)` and
  writes `analytics.json`. So it is a reliable detection marker.
- `~/.aider.chat.history.md` VERIFIED-ABSENT. Not a home path in practice:
  `args.py:275` defaults it to `os.path.join(git_root, ".aider.chat.history.md")`,
  falling back to cwd. Per repo, not per user.
- `~/.aider.input.history` VERIFIED-ABSENT, same per-repo rule (`args.py:272`).
- Artefact: markdown. `analytics.json` is json and holds only
  `uuid`, `permanently_disable`, `asked_opt_in`. No usage.
- Token usage recorded? **Yes, as rounded prose.** `base_coder.py:2023-2068`
  builds `Tokens: 34k sent, 2.1k cache write, 18k cache hit, 900 received.`
  and `io.py:995-999` sends every `tool_output` into the chat history as a
  blockquote. So cache read and cache write both reach disk by name.
- The rounding is the defect. `utils.py:276-282`: under 1000 exact, under 10000
  one decimal ("3.4k"), above that `round(count/1000)` ("34k"). A 34k figure is
  any value in a 1000-token band. That cannot be priced.
- Exact counts exist only as a remote event. `analytics.py:213-246` posts
  `message_send` with `prompt_tokens`, `completion_tokens`, `total_tokens`,
  `cost`, `total_cost` to PostHog (`us.i.posthog.com`). **Replay can never read
  that.** The local mirror is opt-in via `--analytics-log <path>`, at a path the
  user chooses, so Replay cannot find it; and even that event carries no cache
  split, only prompt/completion.
- Prompt caching on the route: yes. Aider reads both
  `cache_creation_input_tokens` and `cache_read_input_tokens` (`:2003-2006`).
- PASS: an `--analytics-log` file whose path the reader supplies, and only for
  input/output/cost, never the cache split.
- FAIL: the default install. Nothing on disk carries an exact token count.
- Source: aider-chat 0.86.2 sdist (PyPI).

#### Gemini CLI

- `~/.gemini/` VERIFIED-ABSENT. Sessions at
  `~/.gemini/tmp/<projectId>/chats/session-<YYYY-MM-DDTHH-MM>-<8char>.jsonl`.
  `GEMINI_CLI_HOME` overrides the home root.
- `projectId` is not a hash of cwd any more. `Storage.initialize()` resolves it
  through a `ProjectRegistry` at `~/.gemini/projects.json`; the sha256-of-path
  layout is migrated away from. A reader that hashes cwd will miss the directory.
- Artefact: jsonl, one record per line, first line metadata
  (`sessionId`, `projectHash`, `startTime`, `kind`).
- Token usage recorded? **Yes, per assistant message.** `recordMessageTokens`
  writes `tokens: {input, output, cached, thoughts, tool, total}` from
  `promptTokenCount`, `candidatesTokenCount`, `cachedContentTokenCount`,
  `thoughtsTokenCount`, `toolUsePromptTokenCount`, `totalTokenCount`.
- Cache: **read yes, write no.** There is no cache-creation token field, and
  that is not an omission in the CLI: the Gemini route bills cache storage by
  time held, not by tokens written. A cache-write column here would be invented.
- Model is recorded (`msg.model = message.model` in `recordMessage`), so a row
  can be priced.
- PASS: `~/.gemini/projects.json` resolves the project id, and the session line
  carries both `model` and `tokens`.
- FAIL: a reader that cannot resolve the project id finds no files at all.
- Source: @google/gemini-cli 0.59.0 (npm, bundled).

#### Goose (Block)

- `~/.local/share/goose/` VERIFIED-ABSENT; DB would be
  `~/.local/share/goose/sessions/sessions.db`.
- `~/Library/Application Support/Block/goose/` VERIFIED-ABSENT and **wrong for
  this version.** The in-repo comment at `crates/goose/src/config/paths.rs:19`
  still names it, but goose pins `etcetera 0.11.0`, whose
  `create_strategies!(Apple, Xdg)` makes `choose_app_strategy` return the **XDG**
  strategy on macOS, not Apple. So data_dir is `$XDG_DATA_HOME` or
  `~/.local/share`, joined with `unixy_name` = "goose". Trusting the comment
  would point a reader at a directory goose does not write.
- `$GOOSE_PATH_ROOT/data/sessions/sessions.db` UNVERIFIED, env-dependent.
- Artefact: sqlite.
- Token usage recorded? **Yes, and it is the most complete of any surface here.**
  `sessions` table columns: `total_tokens`, `input_tokens`, `output_tokens`,
  `cache_read_tokens`, `cache_write_tokens`, plus `accumulated_total_tokens`,
  `accumulated_input_tokens`, `accumulated_output_tokens`,
  `accumulated_cache_read_tokens`, `accumulated_cache_write_tokens`,
  `accumulated_cost`, `provider_name`, `model_config_json`.
- Cache read and cache write are both first-class columns. Granularity is the
  session, not the message: `messages` holds `content_json` with no token columns.
- PASS: the sessions row carries provider, model config, and both cache columns,
  so a session can be priced without touching the wire.
- FAIL: nothing structural. The only blocker is that this build has no sqlite
  reader for it.
- Source: aaif-goose/goose @ main, `crates/goose/src/session/session_manager.rs`.

#### OpenCode

- `~/.local/share/opencode/storage/` VERIFIED-ABSENT. Root is
  `xdgData/opencode` via `xdg-basedir`, which on macOS returns
  `~/.local/share` (no Apple special case), overridable by `XDG_DATA_HOME`.
- Layout: `storage/message/<sessionID>/<messageID>.json`,
  `storage/session/<projectID>/<sessionID>.json`, `storage/part/...`.
- Artefact: one json file per message.
- Token usage recorded? **Yes, per assistant message, with cost.** Schema:
  `metadata.assistant.tokens {input, output, reasoning, cache {read, write}}`
  alongside `cost`, `modelID`, `providerID`.
- Cache read and cache write are both present as named fields.
- PASS: `modelID` + `providerID` + the token struct are in the same file, so
  Replay can reprice rather than trust the stored `cost`.
- FAIL: nothing structural; no walker for the storage tree yet.
- Source: anomalyco/opencode @ dev, `packages/opencode/src/session/message.ts`,
  `packages/core/src/global.ts`, `packages/opencode/src/storage/storage.ts`.

#### Amp (Sourcegraph)

- `~/.config/amp/settings.json` VERIFIED-ABSENT. Vendor docs give this and
  workspace `.amp/settings.json`; managed settings at
  `/Library/Application Support/ampcode/managed-settings.json`.
- `~/.amp/` VERIFIED-ABSENT.
- **Thread storage: UNVERIFIED, and deliberately left so.** The npm package is a
  shim; the platform binary `@ampcode/cli-darwin-arm64` is a 75MB Bun executable
  whose JS payload is compressed. `strings` over it yields only Bun runtime text:
  no `.amp` path, no ampcode.com URL, no thread filename. The docs pages for the
  CLI and for settings do not state where threads are kept or whether any usage
  is written locally.
- Token usage recorded locally? **UNVERIFIED.** `amp.showCosts` controls display,
  which is not evidence of a file.
- What would verify it: install `@ampcode/cli`, run one thread, then diff
  `~/.config/amp`, `~/.local/share`, `~/Library/Application Support` and the
  workspace for new files, and open whatever appears. Nothing short of that
  should be written into a detector.
- PASS: unknown until the above is run.
- FAIL today: the only Amp file this survey can name is `settings.json`, and it
  carries no usage.

#### Charm Crush

- `<project>/.crush/crush.db` VERIFIED-ABSENT across `~/Development` (depth 4).
  `defaultDataDirectory = ".crush"` is **relative to the working directory**, so
  Crush leaves nothing under `$HOME` to detect. `~/.crush` and `~/.config/crush`
  VERIFIED-ABSENT; `~/.config/crush/` holds config only, never the db.
- Artefact: sqlite, `crush.db`.
- Token usage recorded? **Partially, and misleadingly.** `sessions` has
  `prompt_tokens`, `completion_tokens`, `cost`. `messages` has no token columns.
- Cache read / write: **not persisted.** Crush does track
  `CacheCreationTokens` and `CacheReadTokens` in memory and uses both to compute
  `cost` (`internal/agent/agent.go:1956-1959`), but they are folded away before
  the write.
- Worse, the persisted counts are not session totals. `updateSessionTokenCounters`
  **assigns**, it does not accumulate: `session.PromptTokens = usage.InputTokens
  + usage.CacheReadTokens` for the latest turn only. A second path,
  `UpdateTitleAndUsage`, writes `InputTokens + CacheCreationTokens` instead. So
  the column means a different thing depending on which path last ran, and in
  neither case is it a total. Only `cost` accumulates.
- PASS: none for repricing. Replay could report Crush's own `cost` figure, but
  that is quoting Crush, not measuring, and it silently returns 0 when
  `model.FlatRate` is set.
- FAIL: the cache split is gone and the token columns are a last-turn snapshot.
- Source: charmbracelet/crush @ main, `internal/db/migrations/20250424200609_initial.sql`,
  `internal/agent/agent.go`.

#### Qwen Code (extra, found while checking the Gemini fork line)

- `~/.qwen/` VERIFIED-ABSENT. Sessions at
  `~/.qwen/projects/<sanitized-cwd>/chats/<sessionId>.jsonl`, with a sibling
  `<sessionId>.runtime.json`. `QWEN_HOME` overrides the root.
- Note the divergence from its Gemini CLI ancestor: `projects/<sanitized-cwd>`,
  not `tmp/<projectId>`. A shared reader for both would find neither.
- Token usage recorded? **Yes.** Per event: `input`, `cached`, `output`,
  `thoughts`, derived from `promptTokenCount`, `cachedContentTokenCount`,
  `totalTokenCount`, `thoughtsTokenCount`.
- Cache: read only, same reason as Gemini CLI. No cache-write count.
- PASS: the chats jsonl carries the token split per event.
- FAIL: no reader, and the per-project directory is keyed by sanitized cwd.
- Source: @qwen-code/qwen-code 0.23.3 (npm, bundled).

#### Draft literals

`dir` is one string and `firstWithEntries` takes one path, but four of these
surfaces have a real env-var alternative root. These drafts assume a variadic
`firstWithEntries(paths ...string)`; without it, each surface silently goes blind
whenever `XDG_DATA_HOME`, `GOOSE_PATH_ROOT`, `GEMINI_CLI_HOME` or `QWEN_HOME` is
set. No Go source was modified by this agent.

```go
{
	name: "Aider",
	dir:  firstWithEntries(filepath.Join(home, ".aider")),
	why:  "Replay cannot read Aider yet, because its chat history rounds every token count to the nearest thousand and the exact counts go to PostHog rather than to a file",
},
{
	name: "Gemini CLI",
	dir:  firstWithEntries(filepath.Join(home, ".gemini", "tmp")),
	why:  "Replay cannot read Gemini CLI yet, because its sessions are keyed by a project id held in ~/.gemini/projects.json, which this build does not resolve",
},
{
	name: "Goose",
	dir:  firstWithEntries(filepath.Join(home, ".local", "share", "goose", "sessions")),
	why:  "Replay cannot read Goose yet, because its sessions live in a SQLite database this build has no reader for",
},
{
	name: "OpenCode",
	dir:  firstWithEntries(filepath.Join(home, ".local", "share", "opencode", "storage", "message")),
	why:  "Replay cannot read OpenCode yet, because its per-message JSON files sit under a storage tree this build does not walk",
},
{
	name: "Amp",
	dir:  firstWithEntries(filepath.Join(home, ".config", "amp")),
	why:  "Replay cannot read Amp, because the only Amp file this build can name is settings.json, which carries no usage",
},
{
	name: "Crush",
	dir:  firstWithEntries(filepath.Join(home, ".config", "crush")),
	why:  "Replay cannot price Crush yet, because its database keeps a cost column and the latest turn's token counts, never the cache split",
},
{
	name: "Qwen Code",
	dir:  firstWithEntries(filepath.Join(home, ".qwen", "projects")),
	why:  "Replay cannot read Qwen Code yet, because its transcripts sit under a per-project directory this build does not walk",
},
```

Two notes on the drafts, both of which need a decision rather than a guess:

1. **Crush cannot be detected from `$HOME` at all.** `.crush/` is written beside
   the repo. The literal above probes `~/.config/crush`, which proves only that
   Crush was configured, never that a session exists. Honest options are to drop
   Crush from `knownSurfaces` or to teach `findOtherSurfaces` a working-directory
   probe. Pointing at `~/.config/crush` and calling it "records are on this
   machine" would be the exact defect the file's own comment warns about.
2. **The existing two `why:` strings use an em-dash**; these do not, per the
   no-em-dash rule. If the file's punctuation is to stay uniform, the Grok and
   Cursor strings are the ones to change, not these.

### AGENT-5 — implementation pass, and what it declined to ship

Scope: turn the survey above into code. Surveyed independently on this machine
2026-09-12 (darwin 25.5.0) rather than trusting the rows already in this file,
because the rule says verify here, and two of the claims handed over did not
survive that. Owns `cmd/replay/othersurfaces.go`,
`cmd/replay/surfacepaths_test.go` and the new `*surface*_test.go` files.

**Headline: 9 surfaces were verified on this machine, 2 were added.** The gap is
not an oversight and is the substance of this section. Seven of the nine were
already shipped or already covered (Claude Code, Codex, Ollama, Grok, Cursor,
Claude Desktop via `desktoproot.go`, and Open WebUI which turns out to be
undetectable). Everything else the survey proposed is VERIFIED-ABSENT here, so
its directory spelling has never been observed and does not ship.

#### Shipped

**OpenClaw** (new row). VERIFIED-PRESENT, and the only candidate in this whole
survey whose artefact was opened on this machine and found to contain a usage
object.

- dir: `~/.openclaw/agents/<agentId>/sessions`, VERIFIED-PRESENT.
  `~/.openclaw` itself VERIFIED-PRESENT with 20 entries.
- format: jsonl, one JSON object per line, `type` in
  {session, model_change, thinking_level_change, custom, message, compaction}.
- MEASURED: 2 session files, 707 entries, 447 assistant messages. Every
  assistant message carries `api`, `provider`, `model` and `usage` =
  {input, output, cacheRead, cacheWrite, totalTokens, cost{...}}.
- token usage recorded? Yes. 229 rows non-zero `input`.
- cache tokens? **Read yes, write never.** 113 rows carry a non-zero
  `cacheRead` summing to 5,208,111 tokens. **0 rows carry a non-zero
  `cacheWrite`**, across both providers present
  (`openrouter`/`anthropic/claude-3.5-haiku`, 254 rows;
  `openclaw`/`delivery-mirror`, 193 rows).
- PASS: one assistant row with a non-zero `cacheWrite` on a later build. Then
  both halves of the cache trade exist and OpenClaw is priceable, because the
  model id and provider are already on the same row.
- FAIL (current): a cached read at 0.1x is only a saving against the 1.25x
  write that created it. With the write side never counted, the arithmetic has
  one side missing.
- `why:` `Replay cannot price OpenClaw yet: every assistant row in its session log carries a cacheWrite counter and every one of them is zero, beside cacheRead counters on the same rows that are not`
- Probe note: the agent id is a path segment the user chooses, so the detector
  globs `agents/*/sessions` rather than hardcoding `main`. It probes the
  sessions directory and NOT `~/.openclaw`, because the root holds
  openclaw.json, exec-approvals.json and update-check.json from install time
  and would report a corpus on a machine that has never run a session. Guarded
  by OS10.

**AnythingLLM** (new row). VERIFIED-PRESENT, AnythingLLM.app 1.15.0 in
/Applications.

- dir: `~/Library/Application Support/anythingllm-desktop/storage`,
  VERIFIED-PRESENT, holds `anythingllm.db` (749 KB SQLite).
  Linux `~/.config/anythingllm-desktop/storage` **UNVERIFIED and deliberately
  not shipped as a second candidate**: a path nobody has seen makes a probe
  look thorough while never firing.
- format: SQLite, table `workspace_chats`, 46 rows, `response` column is JSON.
- MEASURED: every row's response has keys text, sources, type, attachments,
  metrics. All 46 `metrics` objects carry exactly prompt_tokens,
  completion_tokens, total_tokens, outputTps, duration, model, provider,
  timestamp.
- cache tokens? **None, in any metrics object.**
- **Why this row's wording differs from the rest of its family.** Most desktop
  frontends fail because the stored message does not say which backend served
  it. AnythingLLM does not fail that way: `provider` and `model` are on every
  row. Its failure is the single narrower one, no cache field. The two failure
  modes stay in separate sentences because they decide who has to act.
- PASS: a cache field appearing in the metrics object on a later build.
- FAIL: no cache field, so a cached read cannot be told from a full-price one.
- `why:` `Replay cannot price AnythingLLM yet: every workspace_chats row records prompt_tokens and completion_tokens in its metrics object, and that object has no cache field, so a cached read there cannot be told from a full-price one`
- Detection is install level and cannot be narrowed: the record is rows inside
  a SQLite file this build does not open, and anythingllm.db is written at
  first boot. Same bargain the shipped Cursor and Grok rows already make.

**CORRECTION to the handover on AnythingLLM.** The claim was "zero
instrumentation hits for cache_read, cache_creation, cache_write or
cached_tokens" across the whole database. A sweep here found one:
`cached_tokens` occurs once, inside the `prompt` column of a workspace_chats
row, in a raw API response a user had pasted into a chat. That is content, not
instrumentation. Small, and it decides the wording: a `why:` string asserting
the key appears nowhere in the file would be false and falsifiable by grep. The
shipped string claims only what was enumerated, the metrics object.

**Cursor** (amended, not added). The shipped string cited `state.vscdb` alone,
which understated the search. Re-measured here: 118 JSONL files under
`~/.cursor/projects/*/agent-transcripts` across 31 session directories, 4,838
rows of which 4,566 are message rows. Complete top-level key set across all of
them: role, message, type, status, error. **Zero rows carry a usage or token
key at any level.** Both halves are now in the sentence because they fail
differently: in the database the counter exists and is zero, in the transcripts
there is no counter at all. A reader told only the first might reasonably go
looking in the other file.

(The handover said 43 session directories; measured here it is 31. The 4,566
message-row figure reproduces exactly.)

#### Verified and NOT shipped, with what would change that

**Open WebUI** (raised by the maintainer mid-task). VERIFIED-PRESENT on this
machine, and still not shippable. This is the most interesting negative here.

- `~/.open-webui`, `~/open-webui`, `~/.config/open-webui`,
  `~/.local/share/open-webui`, `~/Library/Application Support/open-webui`,
  `~/.openwebui`, `~/.ollama-webui`: **all VERIFIED-ABSENT.** There is no
  `$HOME`-relative Open WebUI directory on this machine and, per the installed
  source, there is not supposed to be one.
- **Actually installed at** `~/local_ai_stack/venv/lib/python3.12/
  site-packages/open_webui/`, version 0.11.0, with the store at
  `.../open_webui/data/webui.db` VERIFIED-PRESENT. Also
  `~/Applications/Chrome Apps.localized/Open WebUI.app` (a browser shortcut).
- **Why no detector can ship.** VERIFIED from the installed source,
  `open_webui/env.py:222,244`: `DATA_DIR = os.getenv('DATA_DIR',
  OPEN_WEBUI_DIR / 'data')` where `OPEN_WEBUI_DIR = ENV_FILE_PATH.parent`, the
  installed package directory; and `env.py:267`
  `DATABASE_URL = sqlite:///{DATA_DIR}/webui.db`. So the store lives wherever
  pip put the package, which is a virtualenv path the user chose. There is no
  home-relative default to probe. This is AGENT-2's Crush problem exactly, and
  the same conclusion follows: pointing at a guessed path and calling it
  "records are on this machine" is the defect this file exists to prevent.
- artefact: SQLite, table `chat`, column `chat` is JSON holding `messages` and
  `history.messages`. 3 chats, 33 messages, 15 with a `usage` object.
- token usage recorded? Yes: input_tokens, output_tokens, total_tokens,
  prompt_tokens, completion_tokens, prompt_eval_count, eval_count, plus
  timings and `completion_tokens_details`{reasoning_tokens,
  accepted_prediction_tokens, rejected_prediction_tokens}.
- cache tokens? **None.** A regex sweep over every text column of every table
  found cache-ish strings only in the `tool` and `config` tables (tool source
  and configuration), never inside a usage object.
- **Is the backend identifiable?** Partly, and this is the maintainer's
  question 3. A `model` id IS stored per message (`mistral:latest`,
  `qwen2.5-coder:7b`) and `chat.models` repeats it. But the id is bare: there
  is no connection or backend identifier, and the `model` table that would map
  an id to a configured endpoint has **0 rows**. So on this database the
  backend is inferable only because these happen to be Ollama tags. A chat
  against an OpenAI-compatible endpoint would store an equally bare id and
  nothing would distinguish it. The surface would be unpriceable on that
  ground even if a cache field appeared.
- **Overlap with the shipped Ollama row: real, and measured.** Both models used
  here are local Ollama models, and `~/.ollama/logs/server-5.log` and
  `server-2.log` name `mistral` (1,043 and 67 hits) and `qwen2.5-coder` (1,033
  and 39). The same inference is on disk twice. A reader tells them apart only
  by the model id being an Ollama tag; nothing in webui.db says "Ollama". If
  Open WebUI were ever added, it must not be summed with `replay burn`'s Ollama
  figures without deduplicating, because double counting a bill is worse than
  missing it. Here the spend is zero either way (local inference), which is
  what makes the trap cheap to walk into and expensive later.
- What would make it shippable: a home-relative default path in a future
  release, or an explicit `--dir` style argument the reader supplies. Not a
  guessed path.

**Hermes Agent.** NOT SHIPPED. VERIFIED-ABSENT here: `HERMES_HOME` unset, no
`hermes` on PATH, `~/.hermes` absent, and a depth-4 sweep of `~` for `*hermes*`
returns nothing. The schema in the handover is documented (hermes-agent.
nousresearch.com developer guide) and has never been observed by anyone in this
repository. Shipping a probe of a path nobody has seen is the exact failure the
rule at the top of this file names: it cannot be watched firing, and a probe
that silently never fires reports a clean bill to a reader who has a blind
spot. What would verify it: one machine with Hermes installed, one session run,
then `ls $HERMES_HOME` or `~/.hermes` and `sqlite3 state.db .schema`. The
proposed `why:` is recorded for whoever does that:
`Replay cannot read Hermes yet: its state.db records cache_read_tokens and cache_write_tokens per session and per model, but its messages table carries only a scalar token_count, so per-turn cache waste is not recoverable`

**VS Code native chat.** NOT SHIPPED, and the evidence is now much better than
it was, which is why this entry is longer than a refusal needs to be.

- There IS a real VS Code build on this disk:
  `~/Library/Application Support/asd/code-server/lib/vscode`, whose
  `package.json` reads version **1.109.5**, with a 120 MB `out/` tree.
- **CORRECTION to the handover.** The sweep was described as covering "the
  whole 120MB out/ tree" at `~/Library/Application Support/asd/code-server`.
  That path's own `out/` is 452 KB and contains only code-server's Node wrapper
  (browser/serviceWorker.js, common/, node/app.js). The workbench is one level
  down, in `lib/vscode/out`. Swept against the wrapper, `cacheReadTokens`
  returns 0 hits, and so do `promptTokens`, `completionTokens` and
  `chatSessions`. A sweep whose positive controls are also zero is not evidence
  of absence, it is evidence the wrong tree was read. This is the "a check that
  cannot fail is not evidence" shape, in a survey rather than a test.
- **Re-swept against `lib/vscode/out` (120 MB), the conclusion holds and is now
  load-bearing.** Positive controls fire: `chatSessions` 300 hits,
  `workspaceStorage` 149, `emptyWindowChatSessions` 2,
  `transferredChatSessions` 2, `promptTokens` 16, `completionTokens` 3.
  Negative results in the same sweep: `cacheReadTokens` 0,
  `cacheCreationTokens` 0, `cachedTokens` 0, `cacheWrite` 0, `cacheRead` 0.
  So VS Code 1.109.5 persists prompt and completion tokens and defines no cache
  field at all.
- This also **falsifies one of AGENT-1's claims** for this version. AGENT-1
  reported `IChatUsageModelTotal` as `{model, inputTokens, cachedTokens,
  outputTokens}`. In this build `inputTokens`, `outputTokens`, `cachedTokens`
  and `modelTotals` each return 0 hits, and `copilotCredits` returns 0 too.
  Whatever release those names came from, it is not 1.109.5.
- **Why it still does not ship.** The path spelling is now VERIFIED from a
  shipped build rather than derived from a package.json, which is a real
  upgrade. But the DIRECTORY is VERIFIED-ABSENT: stock VS Code is not
  installed, and this code-server tree has no `User/workspaceStorage` or any
  `chatSessions` directory anywhere. So the probe cannot be watched firing on
  this machine, and it needs a per-workspace glob it has never been run
  against. What would verify it: one machine with VS Code chat used once, then
  `ls ~/Library/Application\ Support/Code/User/workspaceStorage/*/chatSessions`.
  Proposed `why:` for whoever does that:
  `Replay cannot read VS Code chat yet: its persisted chatSessions records carry only promptTokens and completionTokens, and VS Code 1.109.5 defines no cache-read or cache-write field at all`

#### Hazard recorded in code: audit.jsonl beside the Claude Desktop sandboxes

Recorded in `cmd/replay/desktoproot.go` as a comment, because it is a trap that
looks like a find. The sandbox tree carries a second population of
usage-bearing JSONL named `audit.jsonl`, which is the same traffic logged
again. MEASURED here under
`~/Library/Application Support/Claude/local-agent-mode-sessions`:

| tree | files | usage rows | cache_read tokens |
|---|---|---|---|
| `audit.jsonl` | 540 | 41,004 | 5,566,921,095 |
| `.claude/projects/*.jsonl` | 787 | 74,021 | 7,150,518,356 |

The audit records use the same field names the transcript parser already
understands (`cache_read_input_tokens`, `cache_creation_input_tokens`,
`input_tokens`, `output_tokens`) plus `duration_ms`, `iterations`, `tool_uses`.
They will parse cleanly and produce plausible figures. **CORRECTION to the
handover**: the inflation was given as "roughly fifty percent" and measures
**78%** of the sandbox cache-read volume here. Direction right, magnitude not.
`desktopSandboxRoots` is safe today because it matches a `projects` directory
whose parent is `.claude`, never a file extension. Widening that match is the
failure mode, and it would not announce itself.

#### DO NOT ADD, so nobody re-adds them

- **Ollama desktop app** (`~/Library/Application Support/Ollama/db.sqlite`).
  VERIFIED-PRESENT but a second Ollama row would duplicate the shipped one, and
  its `messages` table has fourteen columns of which none is a token count;
  chats and messages are both empty.
- **Claude Desktop.** Already read by `desktopSandboxRoots` in
  `cmd/replay/desktoproot.go`. A row here would be a second detector for one
  surface.
- **Copilot, Copilot Chat, Windsurf, Amazon Q, Tabnine, Supermaven, Augment,
  Cody, JetBrains AI Assistant, Junie.** Flat-rate seats or credit pools. There
  is no per-token invoice for a cache miss to land on, so a detector would
  compute a number the reader cannot recover on any bill. This is the same
  finding AGENT-1 reached for Copilot by a different route (premium requests
  with a per-model multiplier), generalised.
- **LM Studio, Jan, Msty, GPT4All, ChatGPT Desktop, Zed, Void, LibreChat, and
  the whole JetBrains family.** All VERIFIED-ABSENT on this machine. One honest
  caveat: LibreChat normally runs as a Docker stack and the Docker daemon was
  down, so the filesystem alone cannot rule it out. Note also that `os/exec` is
  banned in the shipped binary, so any surface that needs `docker` to find it
  is a detection path Replay cannot use even when a shell can see it.
- **Everything in AGENT-1's, AGENT-2's and AGENT-3's draft-literal blocks**
  (Cline, Roo Code, Kilo Code, Continue, GitHub Copilot CLI, Zed, Windsurf,
  Junie, Xcode intelligence, Aider, Gemini CLI, Goose, OpenCode, Amp, Crush,
  Qwen Code). Re-probed independently here, every one VERIFIED-ABSENT. Their
  directory spellings come from vendor source, not observation, and the blocks
  say so themselves ("DO NOT MERGE UNTIL VERIFIED-PRESENT"). They stay
  unmerged.

#### Tests, and the mutations that prove they can fail

New: `cmd/replay/openclawsurface_test.go` (OS9, OS10, OS11) and
`cmd/replay/anythingllmsurface_test.go` (OS12, OS13, OS14).

Written before the implementation and confirmed failing first. Seven mutations
of the production code, each reverted after:

| mutation | result |
|---|---|
| M1 glob hardcoded to `agents/main/sessions` | OS11 red |
| M2 probe `~/.openclaw` root instead of `sessions` | OS9, OS10, OS11 red |
| M3 OpenClaw reason loses the cacheRead clause | OS9 red |
| M4 probe `anythingllm-desktop` root instead of `storage` | OS12 red |
| M5 AnythingLLM reason reverted to the whole-database overclaim | OS12 red |
| M6 Cursor reason loses the agent-transcript half | OS14 red |
| M7 AnythingLLM dir set without the entries check | OS13 red |

One defect was found in the tests themselves by running them. The first draft
named its test functions after the product; `t.TempDir()` builds its path from
the test name and the empty state prints the roots it looked in, so every
"is the surface named" assertion matched the harness's own temp path rather
than the detector's output, and the absence assertion could not pass at all.
Both were measuring the test runner. The function names now avoid the product
word, and the file says why.

`go build ./... && go test ./...` pass, `internal/prose` (no em-dash) and
`internal/regression` (FD-11 frozen claim) included.

#### Consistency with `scripts/surface-drift`

Checked rather than assumed. The harness runs every surface with
`HOME=<empty temp dir>` (`drift.py`, the `env` dict), so `findOtherSurfaces`
finds nothing there and no row added here can move a drift verdict. Run against
the new binary: 141 surfaces, 38 OK, 4 EXPECTED REFUSAL, 48 NEEDS ARGUMENT,
51 NOT RUN, and **zero DRIFT, FLAKY or TIMEOUT**. Diffed against the committed
`last-run.json`: no existing verdict changed. The five new rows
(`serve --mask-ttl`, `serve --freeze-prefix`, `mcp --install`, `probe --vary`,
`cost --usage`) are flags other agents added to `docs/CLI.md` concurrently and
are unrelated to this work.

### AGENT-5, second pass: the dir-of-dirs defect, two terminal agents, one wrong row

#### The hasEntries defect: confirmed, and it was not what the existing test said

Reproduced independently. `hasEntries` counted only non-directory DIRECT
children, so a root holding `project/chats/session.jsonl` and nothing loose
answered false. Every surface shipped before 2026-09-13 happens to keep a file
directly at its probe root (`~/.grok`, `~/.cursor`, `~/.ollama/logs`,
`~/.codex`), which is the only reason it never bit.

**Another agent landed the fix in `othersurfaces.go` while this pass was
running**, with a bounded recursive walk (`entryProbeDepth = 3`, direct children
checked before any descent) and `cmd/replay/hasentries_test.go` (HE1 to HE4).
That work is theirs, not double-done here. What this pass contributes is the
shipped-row control, OS15: Oracle is the first shipped surface whose probe root
holds only directories, so the row exercises the fix against a real layout
rather than a fixture.

**A finding about the test that was supposed to cover this.**
`TestADirectoryOfDirectoriesIsNotEvidence` in `surfacepaths_test.go` is named as
though it pinned this behaviour. It did not and could not: its fixture builds
only EMPTY subdirectories, so it passes identically before and after the change.
A mutation forcing the depth back to 0 (M9) leaves it green while HE1, OS15 and
OS16 all go red. The comment has been rewritten to say what the fixture actually
tests, and to say plainly that it never caught the defect its name implies. A
test whose name overstates its fixture is how a defect survives a green suite.

The opposite direction still holds and is still guarded: a tree of empty
directories is not a corpus, because several agents create their store eagerly
on install. OS13 and HE2 both pin it.

#### Oracle (new row, VERIFIED-PRESENT)

- Oracle 0.8.6. dir: `~/.oracle/sessions` VERIFIED-PRESENT, 3 session
  directories. `~/.oracle` holds nothing but `sessions`.
- format: `~/.oracle/sessions/<slug>/meta.json`. A dir-of-dirs root with no
  loose file, which is why this row could not have fired before the hasEntries
  fix.
- MEASURED: only 1 of the 3 sessions carries a `usage` object at all. The other
  two have keys through `status` and stop. The one that does reads in full:
  `{inputTokens: 4256, outputTokens: 0, reasoningTokens: 0, totalTokens: 6,
  cost: 0.089376}`.
- cache tokens? **None, of any kind.**
- PASS: a meta.json carrying a cache field and a self-consistent total.
- FAIL: no cache field, and the record does not agree with itself. A
  totalTokens of 6 against an inputTokens of 4256 is not rounding and not a
  unit mismatch, it is two numbers that cannot both be counting the same
  request. Repricing the input column alone would mean trusting one half of a
  record whose other half is visibly wrong.
- `why:` `Replay cannot price Oracle: its meta.json records inputTokens and outputTokens and no cache field of any kind, and on the one session here that has usage at all the totalTokens is 6 against an inputTokens of 4256`

#### Grok: the shipped row was wrong, and is corrected

The shipped string blamed the wire: "Replay cannot read Grok's wire yet: it
posts to /responses, which this build does not parse". **The wire is not the
blocker.** A reader acting on that sentence would write a /responses parser and
gain nothing, because the numbers are already on disk.

MEASURED here: `~/.grok/sessions/<urlencoded-cwd>/<uuid>/updates.jsonl`, 105
files, 33,929 JSON-RPC records shaped `{timestamp, method, params}`. 1,131 carry
`params.update.usage` with inputTokens, outputTokens, totalTokens,
cachedReadTokens, cacheCreationTokens, reasoningTokens, costUsdTicks,
modelCalls, apiDurationMs, numTurns, plus a per-model `modelUsage` breakdown
keyed by model id (`grok-4.6-build`). That per-model key is exactly the backend
attribution most rows in this file are missing.

**And one thing the handover did not say, which changes the verdict.** The
suggested replacement implied both cache fields carry data and only a reader is
missing. Measured: cachedReadTokens is non-zero on 1,120 of 1,131 records and
totals 762,715,904 tokens, while **cacheCreationTokens is present on all 1,131
and zero on every one**. Grok is the same shape as OpenClaw, not a surface one
reader away from a bill. Both facts are in the shipped sentence: naming the file
retires the wire claim, naming the zero stops the next person expecting a bill
at the end of the parser.

(The handover said 166 usage records; measured here it is 1,131.)

- `why:` `Replay cannot read Grok yet: its per-turn usage sits in ~/.grok/sessions/*/*/updates.jsonl with cachedReadTokens and a per-model breakdown that this build has no reader for, and the cacheCreationTokens counter beside them is zero on every record`
- The verb stays "cannot read" because OS5 in `otherSurfaces_test.go` asserts it
  for this row, and because it is the right word: reading is what fails first.

#### OpenClaw: corrections to the handover, and one addition

- **The probe was never hardcoded to the agent id.** It has globbed
  `agents/*/sessions` since it landed, and OS11 is the test that holds it there.
  The handover's concern was already addressed.
- **Added: `OPENCLAW_STATE_DIR`.** VERIFIED from the installed bundle, openclaw
  2026.2.15 at `/opt/homebrew/lib/node_modules/openclaw`:
  `NEW_STATE_DIRNAME = ".openclaw"`, and `OPENCLAW_STATE_DIR` occurs 217 times
  including in the CLI's own help text ("Write completion scripts to
  $OPENCLAW_STATE_DIR/completions"). The whole tree relocates with it, so a
  probe that knows only `~/.openclaw` goes blind for anyone who set it. Guarded
  by OS18, and the override is tried before the default.
- On `cacheWrite`: the handover reads the zero as a route property (the
  OpenRouter openai-completions route returns `cached_tokens` only) rather than
  a missing field. That is a reasonable reading and it is not verified here, so
  the shipped string states the measurement (the counter is zero on every row)
  and does not assert the cause. The `totalTokens == input+output+cacheRead+
  cacheWrite` identity holding on all 447 rows is confirmed.

#### Settled: the two repository renames

Left open in the handover, settled with one API call each rather than guessed:

- `gh api repos/block/goose` resolves to `{"full_name": "aaif-goose/goose",
  "archived": false, "fork": false}`.
- `gh api repos/sst/opencode` resolves to `{"full_name": "anomalyco/opencode",
  "archived": false, "fork": false}`.

Both renames are real and neither repository is archived or a fork. AGENT-2's
note was correct and the disagreeing agent was wrong. No URL is hardcoded in
this build either way, so nothing in the tree depended on the answer.

#### Still NOT shipped, and why the standard did not move

The handover marked Aider, Gemini CLI, Goose, OpenCode and Warp
SOURCE-VERIFIED and suggested detect-and-decline rows. **All five are
VERIFIED-ABSENT on this machine** (re-probed independently: no `~/.aider`,
`~/.gemini`, `~/.local/share/goose`, `~/.local/share/opencode`, and no Warp
directory). Their path spellings have never been observed by anyone here, so a
probe cannot be watched firing and a silent never-firing probe is the failure
the rule at the top of this file names. They stay out, exactly as Hermes and
VS Code chat do.

Two of the handover's sharper findings are recorded here so the next pass does
not have to rediscover them:

- **Aider**: the defect is not only rounding. The cache lines reach disk only
  under `--no-stream`, and streaming is the default, so on a normal install
  those two numbers are never written at all.
- **Goose**: its `usage_ledger` is per provider response, so it is per-call
  priceable. The earlier AGENT-2 draft in this file describes it as
  per-session, which understates it.
- **Gemini CLI**: probe `~/.gemini`, not `~/.gemini/tmp`, per the dir-of-dirs
  question above.
- **OpenCode**: storage is `opencode.db` (SQLite) now. The AGENT-2 draft above
  probes the legacy JSON tree.

HOLD, unchanged: Amp (schema unverified), Crush (writes nothing under `$HOME`;
a `~/.config/crush` probe proves configuration and never a session), Plandex
(records live on a Docker volume), Mentat (archived, caching never enabled),
OpenHands.

#### Mutations for this pass

| mutation | result |
|---|---|
| M9 hasEntries depth forced to 0 (the original defect) | HE1, OS15, OS16 red; `TestADirectoryOfDirectoriesIsNotEvidence` stays GREEN, which is the finding above |
| M10 Oracle probe pointed at a directory that does not exist | OS15, OS16 red |
| M11 Grok why reverted to the /responses wire claim | OS17 red |
| M12 OPENCLAW_STATE_DIR override removed | OS18 red |

New tests: `cmd/replay/terminalsurface_test.go` (OS15 to OS18). Written before
the implementation, confirmed failing first.

`gofmt`, `go vet`, `go build ./...` and `go test ./...` all clean. Drift re-run
against the new binary: 141 surfaces, 38 OK, 4 EXPECTED REFUSAL, 48 NEEDS
ARGUMENT, 51 NOT RUN, zero DRIFT/FLAKY/TIMEOUT, and no verdict changed.
