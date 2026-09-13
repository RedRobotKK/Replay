# Built but unwired, part 2: configuration that nothing ever sets

**Date:** 2026-09-09
**Scope:** struct fields on configuration types; command-line flags in `cmd/replay/`;
environment variables read in `internal/` and `cmd/`, checked in both directions
against `README.md`, `docs/` and `install.sh`.
**Method:** every claim below cites `file:line`. Every DEAD or PHANTOM verdict is
backed by a search that returned nothing; those searches are reproduced inline.
No file was modified except this one.

---

## 1. Counts

| Verdict | Count | Family |
|---|---|---|
| **DEAD** | 3 findings (1 field + 2 fields beneath it, 1 flag on 2 of 3 commands) | field, flag |
| **PHANTOM** | 8 variables + 2 flags | env, flag |
| **UNDOCUMENTED** | 4 variables | env |
| **False negative claims** (docs deny a wired capability) | 2 | env, command |
| **WIRED** | 87 of 88 flags; every config-type field except `PreFlight`; 14 of 18 env reads | all |

The headline number: **1,124 struct fields were scanned across the whole repository.
Exactly one field on a configuration type has no production writer.**

---

## 2. The reproduction: `proxy.Config.PreFlight`

**Confirmed.** The finding reproduces exactly as described, and it is already on
record at `docs/design/surface-taxonomy-1-request.md:34-45` as finding F1.

| Setting | Kind | Declared at | Read at | Set in production? | Documented? | Verdict |
|---|---|---|---|---|---|---|
| `proxy.Config.PreFlight` | field | `internal/proxy/server.go:103` | `internal/proxy/preflight.go:62` | **No** | No | **DEAD** |
| `analysis.PolicyState.OptInActive` | field | `internal/analysis/predictor.go:32` | `internal/analysis/predictor.go:46`, `:91` | **No** | No | **DEAD** |
| `analysis.PolicyState.CeilingTokens` | field | `internal/analysis/predictor.go:30` | `internal/analysis/predictor.go:49`, `:94` | **No** | No | **DEAD** |

### The search that found no setter

```text
$ grep -rn 'PreFlight:' --include='*.go' .
internal/proxy/preflight_test.go:21
internal/proxy/preflight_test.go:226

$ grep -rn 'PolicyState{' --include='*.go' . | grep -v _test.go
(no output)
```

Both writers are in `_test.go`. **A field set only in tests is DEAD for this
audit's purposes**, and both of these are.

`cmd/replay/serve.go:136-155` is the only `proxy.Config` literal in the shipped
binary. It sets eighteen fields with keyed syntax and omits `PreFlight`. There is
no `-preflight-*` flag among the thirty `serve` declares (`serve.go:51-86`), no
environment variable, and no mention in `docs/` outside the taxonomy note that
already records the gap.

### What is unreachable as a result

`analysis.EvaluatePreFlightPolicy` short-circuits on the first line
(`internal/analysis/predictor.go:46-48`):

```go
if !optInActive {
    return false
}
```

With `OptInActive` false forever, `d.WouldRefuse(policy)` is always false, so
`preflight.go:91-93` returns `true` on every request and nothing below it runs:

- all 120 lines of `internal/proxy/preflight.go`
- the refusal kind `replay_preflight_deficit` / `preflight_deficit`
  (`internal/proxy/server.go:1017`), which is a key in the `Refusals` map on the
  ledger record (`internal/proxy/state.go:641`) that can never appear

- the straddle warning on `HeaderWarning` (`preflight.go:104`)
- the override path (`preflight.go:112-115`)

The guard **is** on the request path — `server.go:970` calls `s.preFlight` on
every request. It simply cannot ever act.

### Tests cover it, and they pass

```text
$ go test ./internal/proxy/ -run 'TestPreFlight' -v
--- PASS: TestPreFlight_RefusesAChangedPrefixOverTheCeiling
--- PASS: TestPreFlight_AMatchingPrefixIsNeverRefused
--- PASS: TestPreFlight_TheFirstRequestOfASessionPasses
--- PASS: TestPreFlight_TheDefaultNeitherRefusesNorWarns
--- PASS: TestPreFlight_AStraddledCeilingWarnsAndForwards
--- PASS: TestPreFlight_OverrideProceedsOnceAndIsLogged
--- PASS: TestPreFlight_ASiblingLaneDoesNotTriggerARefusal
--- PASS: TestPreFlight_DoesNotRaceOrCreateSessions
ok  github.com/RedRobotKK/Replay/internal/proxy  0.399s
```

Eight tests, eight passes, all calling `s.preFlight` directly with a hand-built
`Config`. **None goes through HTTP, so none would notice that `serve` never sets
the field.** This is the dangerous shape: the suite is green about a guard the
binary cannot run.

---

## 3. Configuration struct fields — the full family

Every configuration type in the repository, and whether each of its fields has a
production writer.

| Type | Declared at | Fields | Production writer | Verdict |
|---|---|---|---|---|
| `proxy.Config` | `internal/proxy/server.go:70` | 19 | `cmd/replay/serve.go:136-155` — 18 of 19 | 18 **WIRED**, `PreFlight` **DEAD** |
| `proxy.SpendLimits` | `internal/proxy/guards.go:23` | 4 | `cmd/replay/serve.go:133` | **WIRED** |
| `proxy.LoopLimits` | `internal/proxy/guards.go:267` | 2 | `cmd/replay/serve.go:144` | **WIRED** |
| `proxy.BreakerSettings` | `internal/proxy/guards.go:317` | 2 | `cmd/replay/serve.go:145` | **WIRED** |
| `proxy.ErrorBudget` | `internal/proxy/guards.go:241` | 1 | `cmd/replay/serve.go:154` | **WIRED** |
| `proxy.TrialSettings` | `internal/proxy/trial.go:16` | 3 | `cmd/replay/serve.go:151` | **WIRED** |
| `proxy.SiblingSettings` | `internal/proxy/siblings.go:17` | 1 | `cmd/replay/serve.go:152` | **WIRED** |
| `proxy.RetrySettings` | `internal/proxy/retry.go:17` | 3 | `cmd/replay/serve.go:153` | **WIRED** |
| `analysis.PolicyState` | `internal/analysis/predictor.go:26` | 2 | **none** | **DEAD** (both fields) |
| `learn.Options` | `internal/learn/learn.go:194` | 1 | `cmd/replay/learn.go:72` | **WIRED** |
| `probe.Config` | `internal/probe/probe.go:26` | 8 | `cmd/replay/probe.go:99-108` — all 8 | **WIRED** |

### Coverage claim, and its limit

A mechanical scan extracted all 1,124 exported struct fields in the repository
and searched for any write outside `_test.go` (keyed composite literal `Field:`
or assignment `.Field =`). 120 fields had no production writer. Reviewing all
120 by hand, every one except `Config.PreFlight` and the two `PolicyState`
fields falls into one of three benign classes:

1. **JSON decode targets** — populated by `encoding/json` reflection, not
   assignment. E.g. `cachemodel.PaymentTerms` (`internal/cachemodel/x402.go:27`,
   filled by `ParsePaymentRequired` at `:52`), `transcript.OpenAIUsage`,
   `cmd/replay/statusInput`.
2. **Unkeyed composite literals** — the scan's known blind spot. E.g.
   `tui.Shortcut` is built positionally at `internal/tui/shortcuts.go:81-90`.
3. **A disclosed placeholder** — `ledger.Record.Trimmed` / `.BodyHashBefore` /
   `.BodyHashAfter` (`internal/ledger/record.go:63-65`). The code says so itself:
   *"Nothing writes it yet: the live trimmer does not ship."* Honest, and no test
   claims otherwise.

The blind spot does not affect this audit's conclusion: **every configuration
type above is constructed with keyed literals only**, verified at
`cmd/replay/serve.go:133-154`, `learn.go:72` and `probe.go:99`. `PolicyState` is
never constructed in production in any syntax.

---

## 4. Command-line flags

88 flags across 20 `flag.FlagSet` declarations. **87 fully WIRED, 1 partially DEAD.**

### The structural reason there are so few

Every flag binds to a **function-local** variable (`x := fs.String(...)`). Go
refuses to compile an unused local, so *"declared and parsed but never read"* is
structurally impossible in this codebase — the compiler already enforces one
read. The only surviving failure modes are (a) read into a struct field nothing
consumes, and (b) read on an unreachable path. Both were checked for all 88.

Parsing is real on every path: every subcommand routes through `parseArgs`
(`cmd/replay/help.go:44`), which calls `fs.Parse`, and every `run*` is dispatched
from `cmd/replay/main.go:129-191`.

### The one finding: `--dollars` is a silent no-op on two of its three commands

| Setting | Kind | Declared at | Read at | Reaches behaviour? | Documented? | Verdict |
|---|---|---|---|---|---|---|
| `--dollars` | flag | `cmd/replay/main.go:279` | `cmd/replay/main.go:307` → `analysis.LaneReport.Dollars` | `replay` / bare path: yes. `blame`, `diff`: **no** | yes | **WIRED** for `replay`; **DEAD** for `blame` and `diff` |

`runReport` backs three commands — `replay` (`main.go:129`), `blame` (`:131`),
`diff` (`:133`), plus the bare-path form (`:191`). The flag is stored on the
report at `main.go:307`. The search for readers:

```text
$ grep -rn '\.Dollars\b' internal/analysis/*.go cmd/replay/*.go | grep -v _test
internal/analysis/report.go:207:   priced := r.Dollars && base.CostUSD > 0
internal/analysis/report.go:213:   } else if r.Dollars {
cmd/replay/main.go:307:            rep.Dollars = *dollars
```

Both readers sit inside `WriteReplay` (`internal/analysis/report.go:197-246`).
`WriteBlame` (`:260`) and `WriteDiff` (`:270`) never consult it.

**What a user cannot do:** see dollar figures from `replay blame --dollars` or
`replay diff --dollars`. The flag is accepted, hoisted, parsed and stored, then
silently ignored — no error, no warning, and output identical to omitting it.
The user has no way to learn the flag did nothing.

### Everything else

All WIRED, grouped by file: `serve.go` (30), `probe.go` (16), `cost.go` (10),
`rules.go` (6), `advise.go` (5), `prefix.go` (3), `tui.go` (3), `context.go` (2),
`learn.go` (2), `route.go` (2), `statusline.go` (2), `trim.go` (2),
`agents.go` (1), `budget.go` (1), `burn.go` (1), `since.go` (1).
`codex.go:53`, `doctor.go:32` and `corpus.go:67` build a FlagSet and declare zero
flags — they take arguments only.

**No flag is read only to print itself back in help text.** Several flag names
appear inside *other* flags' description strings (`rules.go:69,72`;
`cost.go:368`), which inflates a naive grep, but each also has a behavioural read.

Two deliberate short-circuits that are not defects: `cost.go:586` — `--share`
returns before the `--json` branch at `:614`; `cost.go:387-391` — `--png` without
`--share` is rejected with an explicit error rather than ignored.

Note: `replay mcp --install` is handled by hand at `cmd/replay/main.go:139`,
outside any FlagSet. It works (verified), and is documented at
`docs/guide/getting-started.md:149`.

---

## 5. Environment variables

### 5a. WIRED — code reads it, docs mention it

| Variable | Read at | Documented at |
|---|---|---|
| `REPLAY_DISABLED` | `cmd/replay/serve.go:100`, `doctor.go:89` (const `serve.go:36`) | `docs/guide/troubleshooting.md:69`, `docs/SURFACES.md:136` |
| `REPLAY_TOKEN` | `cmd/replay/serve.go:113` (const `serve.go:37`) | `docs/CLI.md:139`, `docs/SURFACES.md:136` |
| `REPLAY_UPSTREAM` | `cmd/replay/serve.go:53` via `envOr` (`serve.go:269-274`), `doctor.go:96` | `docs/SURFACES.md:73,136` |
| `REPLAY_NO_POLICY` | `cmd/replay/serve.go:88` (const `serve.go:42`) | `docs/guide/commands.md:1208`, `docs/SURFACES.md:137` |
| `REPLAY_TRANSCRIPTS` | `cmd/replay/defaultroot.go:28` (const `defaultroot.go:16`) | `docs/CLI.md:358` |
| `REPLAY_FX_<CODE>` | `internal/money/money.go:85`, entered from `cost.go:294`, `tui.go:67` | `docs/guide/commands.md:73` |
| `ANTHROPIC_BASE_URL` | `doctor.go:68`, `probe.go:110`, `tui.go:707` | `docs/guide/getting-started.md:105`, `docs/SURFACES.md:83` |
| `ANTHROPIC_API_KEY` | `cmd/replay/probe.go:114` | `docs/guide/commands.md:765` |
| `CLAUDE_CONFIG_DIR` | `cmd/replay/doctor.go:239` | `docs/SURFACES.md:16,22,136` |
| `LC_ALL` / `LC_MONETARY` / `LANG` | `internal/money/money.go:121` | `docs/guide/commands.md:57-59` |
| `NO_COLOR` | `internal/tui/color.go:113`, `statusline.go:205` | `docs/CLI.md:149` — **but denied by `docs/SURFACES.md:141`** |
| `XDG_CONFIG_HOME` | `cmd/replay/defaultroot.go:63`, `contribute.go:101` | `docs/SURFACES.md:30` — **but denied by `docs/SURFACES.md:141`** |
| `HTTP_PROXY` / `HTTPS_PROXY` / `NO_PROXY` | `internal/proxy/server.go:186` (`http.ProxyFromEnvironment`) | `docs/SURFACES.md:66-74` |
| `HOME` | `os.UserHomeDir`, e.g. `cmd/replay/serve.go:262` | `docs/SURFACES.md:137` |

Not an environment variable despite the shape: `REPLAY_SECRET_`
(`internal/masking/vault.go:22`) is the vault **placeholder prefix**, never read
from the environment.

### 5b. UNDOCUMENTED — works, but no user could discover it

| Variable | Read at | What it controls | Verdict |
|---|---|---|---|
| `TERM` | `internal/tui/color.go:116`, `cmd/replay/tipline.go:59` (twice) | `dumb` disables colour; `dumb` **or empty** disables OSC 8 hyperlinks | **UNDOCUMENTED** |
| `COLUMNS` | `internal/tui/cols.go:48` | Terminal width — the **highest-precedence** input, ahead of `TIOCGWINSZ` and the 80 default | **UNDOCUMENTED** (changelog-only, `CHANGELOG.md:71`) |
| `KUBERNETES_SERVICE_HOST` | `cmd/replay/defaultroot.go:153` | Presence flips `env.containerized`, changing the guidance `doctor` and first-run print | **UNDOCUMENTED** |
| `PATH` | `cmd/replay/defaultroot.go:168` | Scanned with `os.Stat` to decide whether `claude` is installed | **UNDOCUMENTED** (plumbing; low severity) |

`TERM` is the sharpest of these. `docs/CLI.md:149` and `docs/guide/commands.md:951`
document the colour precedence as flag plus `NO_COLOR` only. A user on `TERM=dumb`
gets behaviour the documented precedence does not predict, and any launcher that
does not export `TERM` silently loses hyperlinks with nothing to explain it.

`COLUMNS` is the highest-value one to document: it is the override a user would
reach for when the TUI mis-renders, and `docs/TUI-FLAG-SURFACE.md` — the document
whose entire subject is the TUI's controls — contains no environment-variable
mentions at all.

### 5c. PHANTOM — documentation promises it, no production code reads it

**Proof search (returns nothing, exit 1):**

```text
$ grep -rnE 'X402_PAY_TO|X402_NETWORK|X402_ASSET|X402_FACILITATOR|X402_PRICE_ATOMIC|ENABLE_TOOL_SEARCH|OPENAI_BASE_URL|CLAUDE_JOB_DIR' cmd/ internal/
exit=1
```

Zero hits, `_test.go` included. `internal/feed` and `internal/cachemodel` contain
no `Getenv`/`LookupEnv` at all.

#### PHANTOM 1 — the five x402 seller variables. **The worst finding in this section.**

`docs/adr/0013-x402-rules-feed.md`, an ADR marked **`Status: Accepted`** (line 3),
states at lines 137-145, in the present tense:

> **Configuration.** The seller reads five values from the environment, and quotes
> nothing unless all five are present:
>
> | `X402_PAY_TO` | `0xa5dB841b59cFac070d78C51eCaf86dADf0509b5E` |
> | `X402_NETWORK` | `base` |
> | `X402_ASSET` | USDC on Base, `0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913` |
> | `X402_FACILITATOR` | facilitator base URL |
> | `X402_PRICE_ATOMIC` | price per fetch, in atomic units |

Together with the fallback promised at `:132-134`: *"A half-configured seller is
not a seller. Any missing setting answers `503`."*

**Nothing reads any of the five, anywhere in the repository.** The only
occurrences in the whole tree are the three lines of that table itself:

```text
$ grep -rn 'X402_PAY_TO\|X402_FACILITATOR\|X402_PRICE_ATOMIC' --exclude-dir=.git .
docs/adr/0013-x402-rules-feed.md:141
docs/adr/0013-x402-rules-feed.md:144
docs/adr/0013-x402-rules-feed.md:145
```

Even `scripts/x402-e2e/seller.go` — the one thing in the repo that answers `402` —
reads none of them. The **buyer** side is real and tested
(`internal/cachemodel/x402.go:52` `ParsePaymentRequired`, consumed at
`cmd/replay/rules.go:254`, covered by `cmd/replay/x402_test.go`). The **seller**
side is a design.

**What is unreachable:** the paid rules feed cannot be configured or operated.
An operator following an Accepted ADR to stand up the seller would set five
variables and get no quoting, no `503`, and no diagnostic — because no code path
consults them. **This is a money path described as configuration and shipped as
prose.** No test covers it, which is the one mercy: there is no green suite
asserting a seller exists.

#### PHANTOM 2 — `OPENAI_BASE_URL`

`docs/architecture/README.md:27`, inside the **"Current state"** diagram:

> `│  ANTHROPIC_BASE_URL / OPENAI_BASE_URL -> http://127.0.0.1:4000`

Presented as a supported entry path beside the real one. Replay never reads it,
and the same section states `serve` "is implemented for the Anthropic Messages
API". A user pointing an OpenAI client at Replay via this variable gets nothing.

#### PHANTOM 3 — `ENABLE_TOOL_SEARCH`

`docs/architecture/proxy-protocol.md:10`:

> "`ENABLE_TOOL_SEARCH=true` restores it when the gateway forwards
> `tool_reference` blocks unchanged. Replay's passthrough does, **and the README
> will say so.**"

The variable is the client's, so no Replay read is expected — but the promise
about the README is unkept. **`README.md` contains zero environment-variable
mentions of any kind** (verified: no `ANTHROPIC`, no `REPLAY_*`, no `export`).

#### PHANTOM 4 — `CLAUDE_JOB_DIR`

`docs/evidence/wire-families-2026-09-06.md:49` cites
`$CLAUDE_JOB_DIR/tmp/grok-shapes.jsonl` as a capture path. A harness variable in
an evidence file; unreproducible for a reader, but not a product setting. Low.

**Correctly framed, listed for completeness, not defects:**
`ANTHROPIC_AUTH_TOKEN` (`docs/architecture/proxy-protocol.md:8`) and
`ANTHROPIC_CUSTOM_HEADERS` (`:22`) are both explicitly the client's variables.
`REPLAY_VERSION`, `REPLAY_BIN_DIR`, `REPLAY_NO_OPEN` are installer-only
(`install.sh:31,32,39`) and documented as such.

### 5d. PHANTOM flags — documented, never declared

Diffing every `--flag` token in `README.md` and `docs/` against the 80 declared
flag names:

| Flag | Documented at | Declared? | Verdict |
|---|---|---|---|
| `replay corpus --submit` | `docs/adr/0008-corpus-at-launch.md:98` ("What ships at v0.1, concretely"), `:54`, `:58` | **No** — `corpus.go:67` declares zero flags | **PHANTOM** |
| `replay corpus --show-aggregate` | `docs/adr/0008-corpus-at-launch.md:99,107,130` | **No** | **PHANTOM** |

`replay corpus --submit` exits with `flag provided but not defined: -submit`.
ADR-0008:54 and :58 assert it *"is still the only thing that transmits"* and
*"The only way a byte leaves is that someone typed `replay corpus --submit`"* —
statements about a flag that does not exist.

**Partially mitigated, and worth crediting:** `docs/adr/0007-federated-calibration-corpus.md:135-149`
carries a dated correction — *"Nothing in this record was built... The released
binary makes no network request at all"* — and `docs/SURFACES.md:93` marks the
submission path **"Verified absent"**. ADR-0008's own "What ships at v0.1"
section was never corrected to match.

**Design-record only, correctly marked, NOT findings:** `--ledger-retention`
(`docs/adr/0010`), `--rewrite` / `--rewrite-command` (`docs/adr/0011`, **Status:
Proposed**), `--allow-write` (`docs/architecture/mcp-server.md:168`, a document
whose line 5 says *"Nothing here is built"*). `install.sh --corpus-opt-in` is a
real installer flag, implemented at `install.sh:93,396-407`. **`README.md` is
clean: every flag it names exists.**

---

## 6. The inverse defect: documentation that denies a wired capability

This is the same shape as PHANTOM with the sign flipped — the doc makes a
falsifiable negative claim, and the claim is false. It is arguably worse, because
a negative claim is what a reader trusts when auditing what the tool does.

### 6a. `docs/SURFACES.md:141-142` — a "verified" claim refuted by one grep

> **Read by `install.sh` only, and by no Go file:** `NO_COLOR`, `XDG_CONFIG_HOME`,
> `REPLAY_VERSION`, `REPLAY_BIN_DIR`. **Verified: zero occurrences of any of the
> four in `cmd/` or `internal/`.**

Two of the four are read by the binary:

```text
$ grep -rn 'NO_COLOR\|XDG_CONFIG_HOME' cmd/ internal/ | grep -v _test.go
cmd/replay/statusline.go:205:  colour := !*noColour && os.Getenv("NO_COLOR") == ""
cmd/replay/defaultroot.go:63:  if x := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); x != "" {
cmd/replay/contribute.go:101:  dir := os.Getenv("XDG_CONFIG_HOME")
internal/tui/color.go:113:     if _, set := os.LookupEnv("NO_COLOR"); set {
```

Four production reads. The section contradicts the project's own docs in two
directions at once: `docs/CLI.md:149` and `docs/guide/commands.md:951` both tell
users the binary honours `NO_COLOR`, and `scripts/surface-drift/drift.py:77` sets
`NO_COLOR="1"` when invoking the binary — the drift harness **depends on** the
behaviour this section says does not exist.

The passage opens with *"A second correction: the first version of this section
conflated the binary and the installer"* (`:133`). The correction is itself wrong.

Two further accuracy notes in the same file:

- `docs/SURFACES.md:67` cites `internal/proxy/server.go:170` for
  `http.ProxyFromEnvironment`; the line is **186**
  (`docs/design/surface-taxonomy-1-request.md:217` has it right).

- The "Read by the binary" list (`:136-138`) names 8 variables and omits 10 real
  reads: `ANTHROPIC_API_KEY`, `REPLAY_TRANSCRIPTS`, `REPLAY_FX_<CODE>`,
  `NO_COLOR`, `XDG_CONFIG_HOME`, `TERM`, `COLUMNS`, `LC_ALL`/`LC_MONETARY`/`LANG`,
  `KUBERNETES_SERVICE_HOST`, `PATH`.

### 6b. Three documents deny a shipped command

> **"Nothing here is built. There is no `replay mcp` command."**
> — `docs/architecture/mcp-server.md:5`

Repeated at `docs/architecture/README.md:17` and
`docs/design/P3-VISIBILITY-OPTIONS.md:285`.

`replay mcp` ships. It is dispatched at `cmd/replay/main.go:138,147`, listed in
`--help` line 23, and it answers:

```text
$ ./replay mcp <<< '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
{"jsonrpc":"2.0","id":1,"result":{"tools":[{"name":"replay_surfaces",...
```

The capability is reachable and *is* documented for users
(`docs/CLI.md:9,56,354`, `docs/guide/commands.md:1046`,
`docs/guide/getting-started.md:149`), so this is a stale-document defect rather
than an unreachable capability — but a reader who starts at `docs/architecture/`
is told a working feature does not exist.

---

## 7. Why the existing drift tests do not catch any of this

`cmd/replay/docs_drift_test.go:19` asserts that every command in `--help` has a
section in `docs/guide/commands.md`. It operates at **command** granularity only.
It cannot see:

- a config field with no setter (`PreFlight`)
- a flag documented but never declared (`--submit`)
- a flag declared but inert on one of its commands (`--dollars` on `blame`)
- an environment variable read but undocumented (`COLUMNS`, `TERM`)
- a documentation claim that is factually false (`SURFACES.md:141`)

Per ADR-0014, *checks must be able to fail*. Each of the five gaps above is a
check that could be written and would go red today. The `NO_COLOR` case is the
cheapest: a test that greps `cmd/` and `internal/` for every variable
`docs/SURFACES.md` claims is absent would have failed the day that line was written.

---

## 8. Ranked by consequence — what a user cannot do

1. **Cannot cap pre-flight cache-break spend at all.** `proxy.Config.PreFlight`
   (`internal/proxy/server.go:103`). An entire refusal guard, its error kind and
   its counter are unreachable, with eight green tests over them. *DEAD.*
2. **Cannot operate the paid rules feed.** The five `X402_*` variables
   (`docs/adr/0013-x402-rules-feed.md:141-145`) are promised by an **Accepted**
   ADR in the present tense and read by nothing. A money path documented as
   configuration. *PHANTOM.*
3. **Cannot trust the privacy surface inventory.** `docs/SURFACES.md:141-142`
   asserts, as *verified*, that four variables are read by no Go file; two of
   them are read in four places. *False negative claim.*
4. **Cannot submit to the corpus, though three ADR passages say they can.**
   `replay corpus --submit` / `--show-aggregate`
   (`docs/adr/0008-corpus-at-launch.md:54,58,98,99`) do not exist; `corpus.go:67`
   declares zero flags. Partly corrected in ADR-0007, never in ADR-0008. *PHANTOM.*
5. **Cannot get dollar figures from `blame` or `diff`.** `--dollars`
   (`cmd/replay/main.go:279`) is accepted and silently ignored by
   `WriteBlame`/`WriteDiff` (`internal/analysis/report.go:260,270`). *DEAD on 2 of 3 commands.*
6. **Cannot discover `COLUMNS` or `TERM`.** Both change rendering, neither appears
   in any user-facing document — `COLUMNS` outranks terminal detection
   (`internal/tui/cols.go:48`). *UNDOCUMENTED.*
7. **Cannot point an OpenAI client at Replay** despite `OPENAI_BASE_URL` appearing
   in the "Current state" diagram (`docs/architecture/README.md:27`). *PHANTOM.*

---

## 9. Cheapest thing to wire

**`proxy.Config.PreFlight`.** Everything downstream already exists, is tested and
is on the request path (`internal/proxy/server.go:970`). The gap is one flag and
one field:

```go
// cmd/replay/serve.go, beside the other guards at :51-86
preflightCeiling := fs.Int64("preflight-ceiling", 0,
    "refuse a request whose changed prefix would re-lay more than this many "+
    "tokens (0 = off)")

// cmd/replay/serve.go, in the proxy.Config literal at :136
PreFlight: analysis.PolicyState{
    CeilingTokens: *preflightCeiling,
    OptInActive:   *preflightCeiling > 0,
},
```

Roughly five lines. `OptInActive: ceiling > 0` preserves the ADR-0011 consent
constraint and the documented zero-value behaviour that
`TestPreFlight_TheDefaultNeitherRefusesNorWarns`
(`internal/proxy/preflight_test.go:116`) already asserts.

Per the TDD guardrail, the honest order is red first: **there is currently no HTTP-path
test for pre-flight at all** — all eight go straight to `s.preFlight`. Write the
missing one through `httptest` against a server built by `proxy.New`, watch it
fail against today's binary, then add the flag. That test is the thing that would
have caught this in the first place, and it is the reason the eight passing tests
were not evidence.

---

*Audited 2026-09-09. No file modified except this one. Prior art: this reproduces
and extends finding F1 of `docs/design/surface-taxonomy-1-request.md:34-45`.*
