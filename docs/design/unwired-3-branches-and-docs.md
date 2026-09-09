# Built but unwired, part 3: branches that cannot be selected, and documents writing cheques

**Method.** Reachability from `main()` computed with
`golang.org/x/tools/cmd/deadcode` over `./cmd/replay` — an RTA call graph, not a
grep — cross-checked by hand for every finding kept. Documentation compared
against `/tmp/rp --help`, each subcommand's `--help`, and the dispatch table at
`cmd/replay/main.go:113-197`. Nothing in this tree was modified except this file.

Parts 1 and 2 covered config fields, flags and environment variables. This part
covers the two families they could not see: **arms of a `switch` or `if` that no
input can select**, and **prose that promises what the code does not do** — in
both directions, because a shipped capability nobody documented is the same
defect wearing the other hat.

---

## 1. Counts

| Verdict | Count | Where |
|---|---|---|
| **UNREACHABLE** | **12 findings, 49 identifiers and branches** — 8 findings with a real user consequence, 4 of superseded helpers and dead constants | §2, §3 |
| **OVERPROMISED** | **8** | §4 |
| **UNDOCUMENTED** | **4** | §5 |
| **REACHABLE-BY-DESIGN** | **5** | §6 |
| **REACHABLE** (checked and cleared) | every other enum family in the tree: the 9 `Cause*` values, 4 `Error*Class`, `Harm*`, `Why*`, `Reason*`, `Destination*`, `Trial*`, `Variant*`, `Tone*`, every advisor `Kind*`, every `reject*` | — |

**41 of the 49 unreachable items carry passing test references.** Green CI over a
branch no user can select is the pattern this audit exists to name, and it is
the overwhelming majority case here, not the exception. The eight with no test
are dead in both directions — nothing calls them and nothing checks them.

Two of the findings — the pre-flight guard (§3.7) and the four unwired packages
(§5.2) — are already in [UNWIRED-LOG](UNWIRED-LOG.md). They are restated only
where this audit adds a fact the log does not have: the exact line at which
`preflight.go` goes dead, and the discovery that `internal/quota` now sits behind
a shipped MCP tool.

---

## 2. The headline: the TUI's fallback screen can no longer be reached

The evidence note [tui-example-screens](../evidence/tui-example-screens-2026-09-08.md)
recorded that five of ten screens fell through the key dispatch to
`tui.Outcome()`, a canned illustration. All five were then wired. **What nobody
checked is what happened to the fallthrough itself.**

### The branch

```go
// cmd/replay/tui.go:191-192
        loop.SetRows(0)
        return tui.Frame{Key: k, Lines: tui.Outcome(k).Lines}
```

### The proof that no key reaches it

Three gates, and every one of them now closes:

1. **`--once --screen <name>`** is validated against the shortcut table and
   errors out on a miss — `cmd/replay/tui.go:69-78`. `key` is therefore always a
   real shortcut key or the command has already returned.
2. **The interactive loop** initialises the current screen to `Shortcuts()[0].Key`
   (`internal/tui/loop.go:143`) and only ever reassigns it inside
   `for _, s := range Shortcuts() { if s.Key == k { l.cur = k } }`
   (`internal/tui/loop.go:203-207`). An unknown keystroke changes nothing.
3. **All ten shortcut keys have an explicit case** in the source function:
   `d x g s m a c l w` and `shareK` at `cmd/replay/tui.go:142-186`, against the
   ten entries of `internal/tui/shortcuts.go:81-102`.

So the `default` arm is dead, and with it:

| Item | file:line | Tests | Verdict |
|---|---|---|---|
| `default` arm → `tui.Outcome(k)` | `cmd/replay/tui.go:191-192` | `internal/tui/outcomes_test.go` (4), `provenance_test.go` (6) | **UNREACHABLE** |
| `tui.Outcome`'s illustration switch | `internal/tui/outcomes.go:43-195` | same | **UNREACHABLE** |
| `Provenance.Example` — the only producer is `from := Example` | `internal/tui/provenance.go:25`; produced at `internal/tui/outcomes.go:180` | `provenance_test.go` | **UNREACHABLE** |
| `Banner(Example)` — the `[NOTE] example data` notice | `internal/tui/provenance.go:39-40` | `provenance_test.go` | **UNREACHABLE** |
| `Outcomes()` | `internal/tui/outcomes.go:209` | 7 test references, **zero production callers** | **UNREACHABLE** |

This is the *good* kind of dead code — it died because every screen became
measured. But it is dead, it is 200 lines, it holds an enum value the rest of
the program still branches on, and three documents still describe the notice it
renders as something a reader may meet. See §4.

---

## 3. Unreachable branches with a user consequence

Ranked by what a user loses.

### 3.1 `replay_quota` can only ever answer "no reading has been stored"

| Item | Kind | file:line | Reachable? | Tests | Verdict |
|---|---|---|---|---|---|
| `replay_quota` answer branch | branch | `cmd/replay/mcp.go:274-279` | no | `cmd/replay/mcp_test.go:115` asserts the tool is *listed* | **UNREACHABLE** |
| `saveQuota` | function | `cmd/replay/quotastore.go:37` | no production caller | `quotastore_test.go:23,57,91` | **UNREACHABLE** |
| `quotaReading.Age` | method | `cmd/replay/quotastore.go:31` | only from the dead branch | via the above | **UNREACHABLE** |

The chain, end to end. `loadQuota` (`cmd/replay/quotastore.go:81`) is called
exactly once in production, at `cmd/replay/mcp.go:269`. The only writer of that
file is `saveQuota`, and `deadcode` reports it unreachable from `main`;
`grep -rn saveQuota` returns three test call sites and nothing else. The file is
therefore never written, `q.RateLimits` is always nil, and `mcp.go:270-272`
returns unconditionally. Lines 274-279 — the actual answer, the window closest to
binding and its age — cannot execute.

The file's own header states the defect it was written to fix:

> `replay burn` reported "not reported" for the largest surface on the machine
> while the number arrived several times a second and was discarded.
> — `cmd/replay/quotastore.go:14-16`

`replay burn` still reports exactly that, and it is a string literal, not a
measurement:

```go
// cmd/replay/burn.go:243-246
    s := surfaceBurn{
        name: "claude-code", unit: "prompt, cache separate",
        quota: "not reported",
    }
```

This compounds finding #6 in [UNWIRED-LOG](UNWIRED-LOG.md), which recorded
`saveQuota` as unwired but not that an MCP tool advertised to agents depends on
it. See §4.3 for the documentation half.

### 3.2 The largest single waste cause has no surface at all

| Item | Kind | file:line | Reachable? | Tests | Verdict |
|---|---|---|---|---|---|
| `analysis.MeasureIdleRisk` | function | `internal/analysis/idle.go:58` | no | `internal/analysis/idle_test.go` — 6 tests | **UNREACHABLE** |
| `IdleRisk` struct and `Warn` field | type | `internal/analysis/idle.go:38-51` | never constructed in production | same | **UNREACHABLE** |
| `warnWithin`, `warnFloorTokens` | consts | `internal/analysis/idle.go:27,35` | read only by the above | same | **UNREACHABLE** |

Its own doc comment states the stakes:

> Measured across 1,506 transcripts: TTL expiry is 39 breaks and 10,586,000
> re-billed tokens, **33.9% of all waste**, at a mean of 271,436 tokens per event
> — ten times the next-largest cause. **It is also the only large cause where the
> expensive thing has NOT HAPPENED YET.**
> — `internal/analysis/idle.go:12-17`

A third of measured waste, uniquely still preventable, measured, tested, and
reachable from no command, no MCP tool, no TUI screen and no status line. The
`deadcode` run confirms no path from `main`.

### 3.3 Prefix-safe ordering: an analysis and a four-value enum nobody can select

| Item | Kind | file:line | Reachable? | Tests | Verdict |
|---|---|---|---|---|---|
| `analysis.OrderPlan` | function | `internal/analysis/order.go:90` | no | `internal/analysis/order_test.go` — 7 tests | **UNREACHABLE** |
| `Scope` enum: `ScopeSystem`, `ScopeTools`, `ScopeMemory` | enum | `internal/analysis/order.go:42-52` | never produced outside tests | same | **UNREACHABLE** |
| `Scope.invalidates` | method | `internal/analysis/order.go:56` | called only from dead code | same | **UNREACHABLE** |
| `countInvalidations`, `countInvalidatingActions` | funcs | `internal/analysis/order.go:151,166` | no | none | **UNREACHABLE** |
| `PlanOrder` result type | type | `internal/analysis/order.go:67-83` | never constructed in production | same | **UNREACHABLE** |

`ScopeAppend` is the zero value, so it is "produced" by every zero-valued
`Action` and nothing else. The other three scopes — the ones that carry the whole
insight, that editing `CLAUDE.md` or adding an MCP server rewrites the prefix —
have no producer at all. See §4.2: this is the capability the architecture
document sells hardest.

### 3.4 An entire error-screen catalogue that no user can see

| Item | Kind | file:line | Reachable? | Tests | Verdict |
|---|---|---|---|---|---|
| `tui.Problems()` — five refusal screens | handler | `internal/tui/errors.go:43` | no | `internal/tui/errors_test.go` — 6 tests | **UNREACHABLE** |
| `Problem.Render` | method | `internal/tui/errors.go:127` | no | same | **UNREACHABLE** |
| `Problem.Blocking` | method | `internal/tui/errors.go:150` | no | same | **UNREACHABLE** |

The file's design note explains what these are for:

> Whoever reads the message is stuck, and the message is the only thing standing
> between them and giving up. — `internal/tui/errors.go:9-10`

Five codes are declared "stable and greppable, so a user can search for it and an
agent can branch on it" (`errors.go:29-30`): `not-parsed`, `browser-request`,
`port-taken`, `consent-unreadable`, `cap-unenforceable`
(`errors.go:46,62,78,92,108`). A grep across all `*.go` and `*.md` finds four of
the five nowhere else in the repository at all; `not-parsed` appears once, as a
literal inside a storyboard fixture (`internal/tui/storyboard.go:192`). Nothing
emits them, nothing documents them, and no user has ever met the screen.

### 3.5 `replay advise` names a status no suggestion can ever carry

| Item | Kind | file:line | Reachable? | Tests | Verdict |
|---|---|---|---|---|---|
| `advisor.Applied` | enum value | `internal/advisor/advisor.go:71` | never produced | none reference it | **UNREACHABLE** |

`track()` is the only function that assigns a `Status`, and it returns exactly
four of the five: `AdviceOnly` (`internal/advisor/advisor.go:366`), `Pending`
(`:369`, `:374`), `Verified` (`:378`), `NotVerified` (`:380`). `Applied` is
declared and never returned.

The user is told otherwise, in the legend printed under every advice report:

```go
// cmd/replay/advise.go:118
        p.Printf("* = estimated via the byte-to-token fit. Statuses: pending, "+
                 "applied, verified, not verified, advice only.\n")
```

`grep -rn '\bApplied\b' --include="*.go" .` returns the declaration plus eleven
hits on `policy.Applied` — a different type in a different package, used by the
proxy's context-edit path. Nothing produces `advisor.Applied`.

**Verdict: UNREACHABLE**, and the legend at `advise.go:118` is **OVERPROMISED**:
it advertises a state of the tracking lifecycle that the tracking code cannot
reach. A user watching for a suggestion to move to "applied" waits forever, and
the tool told them to watch.

### 3.6 The proxy's second listener never says where it bound

| Item | Kind | file:line | Reachable? | Tests | Verdict |
|---|---|---|---|---|---|
| `Server.MetricsAddr` | method | `internal/proxy/metrics_listener.go:91` | no production caller | 7 test references | **UNREACHABLE** |

`--metrics-listen` (`cmd/replay/serve.go:52`) accepts an address, and the
listener binds and resolves it, including the socket-to-absolute-path case
(`internal/proxy/server.go:286-297`). The startup banner
(`cmd/replay/serve.go:167-197`) prints the proxy address, the upstream and the
ledger path, and never `MetricsAddr()`. An operator who passes `:0`, or who wants
to confirm the second listener came up at all, has no way to find out from the
tool.

### 3.7 The pre-flight guard (already recorded — confirmed, with the branch boundary)

[UNWIRED-LOG](UNWIRED-LOG.md) #7 and
[surface-taxonomy-1-request.md:34-45](surface-taxonomy-1-request.md) have this.
Stated here only to fix the exact line at which the code goes dead, which the
earlier notes leave implicit:

`Config.PreFlight` is assigned nowhere outside
`internal/proxy/preflight_test.go:21,226`; the `proxy.Config` literal at
`cmd/replay/serve.go:136-157` omits it. The zero value has `OptInActive: false`,
and `EvaluatePreFlightPolicy` returns `false` immediately on that
(`internal/analysis/predictor.go:46-48`). So `d.WouldRefuse(policy)` is always
false and **`internal/proxy/preflight.go:92` always returns**. Everything from
line 94 down is unreachable: the message, `Straddles`, the
`x-replay-warning: preflight:` header, the override path, and
`refusalPreFlight` (`internal/proxy/server.go:1017`). Eight tests cover it
(`preflight_test.go`), all calling `s.preFlight` directly.

`PreFlightDeficit.Fitted` (`internal/analysis/predictor.go:76-79`) is a further
degree of dead: `grep -rn '\.Fitted'` across the module returns **zero readers**,
production or test. Its own comment says *"the caller must be able to tell the
difference"*; no caller can, because no caller exists.

### 3.8 The four repaint cadences are one cadence

| Item | Kind | file:line | Reachable? | Tests | Verdict |
|---|---|---|---|---|---|
| `TickTraffic` | const | `internal/tui/refresh.go:39` | zero readers | ordering assertions only | **UNREACHABLE** |
| `TickTotals` | const | `internal/tui/refresh.go:44` | zero readers | same | **UNREACHABLE** |
| `TickAmbient` | const | `internal/tui/refresh.go:50` | zero readers | same | **UNREACHABLE** |
| `Attention` enum: `Calm`, `Notice`, `Act`; `Glance()` | enum + func | `internal/tui/refresh.go:59,61,63,75` | `Glance` has no production caller | 5 refs | **UNREACHABLE** |

```text
$ grep -rn "TickTraffic\|TickTotals\|TickAmbient" --include="*.go" . | grep -v _test.go
internal/tui/refresh.go:36,39,41,44,46,50     (declarations and their comments only)
```

`internal/tui/loop.go:145` starts one ticker, on `TickLiveness`, and
`paint()` (`loop.go:217-227`) rebuilds and diffs the entire frame on every one of
those ticks. The `tick` counter is passed to the source but read only by
`Pinwheel` and `Heartbeat` (`internal/tui/liveness.go:25,36`) — that is, for
animation, never to gate a redraw.

So the loop's own design comment is not true of the loop:

> The ticker is the liveness cadence, **the fastest of the four**, because a loop
> that woke on the slowest could not advance the cue. **Everything slower is
> derived from the tick count rather than from its own timer**, which keeps one
> clock in the program and makes the relationship between the rates something a
> test can assert instead of something four tickers agree on by luck.
> — `internal/tui/loop.go:133-137`

Nothing is derived from the tick count. There is one rate, 250 ms, applied to
everything, and the three constants that name the other three rates — each with a
paragraph of reasoning about why a spend total redrawn every frame is a number
nobody can read (`refresh.go:41-43`) — are read by nothing.
`docs/DASHBOARD-DESIGN.md:116` says repaint is *"at a fixed cadence"*, singular,
which is the accurate description; the code comment is the one that overstates.

The `Attention` triple is the same shape: `Glance()` renders the
`[ OK ]` / `[NOTE]` / `[CRIT]` marker that *"sits on the same row on every screen
and never moves"* (`refresh.go:67-69`), and no screen calls it.

### 3.9 Superseded helpers and dead constants — unreachable, low consequence

Kept for completeness; none of these costs a user anything, because a live code
path does the same job.

| Item | file:line | Superseded by | Tests | Verdict |
|---|---|---|---|---|
| `masking.Find` | `internal/masking/patterns.go:59` | unexported `find` (`:66`) | 6 refs | UNREACHABLE |
| `masking.LooksLikeHexSecret` | `internal/masking/entropy.go:177` | the `credential-assignment` pattern (`patterns.go:46-49`) covers the same case | 3 refs | UNREACHABLE |
| `masking.NewWithVault` | `internal/masking/mask.go:46` | — | 3 refs | UNREACHABLE |
| `learn.LoadSelected` | `internal/learn/learn.go:470` | `LoadFile` + `SelectionFor`, used at `internal/proxy/server.go:914` | 5 refs | UNREACHABLE |
| `chooseTTL` | `cmd/replay/apply.go:119` | `chooseTTLWithCoverage` (`:133`) | 4 refs | UNREACHABLE |
| `tipLineFor` | `cmd/replay/tipline.go:72` | `tipLineArm` with a real variant | 7 refs | UNREACHABLE |
| `consent.ReadUpdateConsent` | `internal/consent/consent.go:96` | nothing — the update-consent flow is unbuilt (§4.2) | 7 refs | UNREACHABLE |
| `analysis.ReadProvenance`, `SessionReadPaths`, `Provenance.Warning`, `pathsFromBlocks` | `internal/analysis/provenance.go:59,126,97,144` | — | 6 / 0 / 6 / 2 | UNREACHABLE |
| `cachemodel.ExportedAt`, `Incoherent` | `internal/cachemodel/export.go:61`, `measure.go:119` | — | 0 / 1 | UNREACHABLE |
| `money.Display.Line`, `Display.Amount` | `internal/money/money.go:134,156` | — | 16 / 3 | UNREACHABLE |
| `probe.Runner.Overhead`, `Search.AtMost` | `internal/probe/run.go:122`, `probe.go:430` | — | 2 / 26 | UNREACHABLE |
| `transcript.OllamaRequest.CacheHitRate` | `internal/transcript/ollama.go:71` | the two-count report at `cmd/replay/burn.go:233-238` | 3 refs | UNREACHABLE |
| `tui.AdviceRowsFit` | `internal/tui/advise.go:167` | — its comment says *"Exported so the caller can assert it on real data"*; there is no caller | 0 refs | UNREACHABLE |
| `proxy.stats.costs`, `unparsedTotal`, `setLaneErrors` | `internal/proxy/state.go:874,897,338` | — | 55 / 2 / 6 | UNREACHABLE |
| `tui.Marked`, `Loop.Painted`, `Start`, `Meter`, `SafeInAFrame`, `Glance`, `Since`, `Anchored`, `Hints`, `Heartbeat`, `LiveRow`, `LiveScenes`, `PaletteCodes`, `Active`, `StripSGR` | `internal/tui/*` | — | mixed | UNREACHABLE |

Constants and enum values with no producer, same class:

| Item | file:line | Proof | Tests | Verdict |
|---|---|---|---|---|
| `TierFrame`, `TierMeaning`, `TierFlourish` | `internal/tui/layout.go:58,61,65` | `grep` finds the declarations and one prose mention (`color.go:8`); no code reads them | 0 | UNREACHABLE |
| `minPathsForProvenance` | `internal/analysis/provenance.go:36` | read only at `:98`, inside the unreachable `Provenance.Warning` | 6 | UNREACHABLE |
| `betaHeader = "anthropic-beta"` | `internal/policy/contextedit.go:41` | `grep -rn betaHeader --include="*.go" .` → one line, the declaration | 0 | UNREACHABLE |
| `observation.BasisAccount` | `internal/observation/observation.go:50` | produced only by `AccountTag()` (`:106`), which has no production caller; `cmd/replay/contribute.go:74` calls `LocalTag` | 1 | UNREACHABLE — every corpus submission carries `basis: "local"`. Note `Basis` is a JSON field validated at `:157`, so a hand-written submission could still carry the value. |
| `learn.FamilyAsRun` | `internal/learn/learn.go:63` | `Catalog()` emits only `FamilyTTL` and `FamilyContextEdit`; the sole use is a map key at `:449` | 0 | UNREACHABLE in-tool. `Candidate.Family` is unmarshalled from `~/.replay/policy.json`, which only Replay writes. |
| `usage.MechanismRentedCache`, `MechanismImplicitPrefix`, `MechanismExplicitBreakpoint` | `internal/usage/usage.go:35,39,42` | the whole package has zero importers (§5.2) | pkg-local | UNREACHABLE |
| `(*Rules).PriceTier` | `internal/cachemodel/rules.go:176` | no production caller | 3 | UNREACHABLE — the *value* arrives from an installed rules document, so it is externally producible; nothing displays it. |

---

## 4. Documentation writing cheques

### 4.1 The worst one: `replay advise --guards` tells you to pass flags that do not exist

This is the only finding here where the tool hands the user a command line and
the same binary then rejects it.

> `replay advise <dir> --guards` suggests spend caps from your own session
> spread using Tukey's upper fence, `Q3 + 1.5*IQR`.
> — `docs/guide/commands.md:689` (also `docs/CLI.md:340`, `docs/guide/alerting.md:88`)

What the code prints:

```go
// cmd/replay/advise_guards.go:32,37
        fmt.Sprintf("  --spend-session-usd %.2f", usd.Upper),
        fmt.Sprintf("  --spend-session-tokens %.0f", tok.Upper),
...
// cmd/replay/advise_guards.go:44
        "Nothing is written: pass these yourself if you want them.")
```

What happens when a user does exactly that:

```text
$ /tmp/rp serve --spend-session-usd 2.08
flag provided but not defined: -spend-session-usd
Usage of serve:
```

The real flags are `-max-session-usd` (`cmd/replay/serve.go:58`) and
`-max-session-tokens` (`cmd/replay/serve.go:56`). The output names neither.

**Verdict: OVERPROMISED.** `--guards` is documented in three places as producing
caps you can use, its own closing line instructs the reader to pass them, and
both flags it emits are rejected by the command it emits them for. Everything
upstream works — the fence, the quartiles, the ten-session floor — and the last
line hands the user a dead command. `cmd/replay/advise_guards_test.go` asserts
the arithmetic and the wording; nothing asserts that the flag names exist.

### 4.2 `docs/architecture/mcp-server.md` describes a different program

`replay mcp` ships and serves seven tools. Part 2 §6b already recorded that this
document opens by denying the command exists. What it did not record is the
*positive* half, which is larger: **the document's tool table names ten tools,
and not one of them exists.**

> | Group | Tools | Annotation |
> |---|---|---|
> | 1. Measure | `replay_tool_cost`, `replay_blame`, `replay_diff`, `replay_context` | `readOnlyHint: true` |
> | 2. Cost | `replay_session_cost`, `replay_what_changed` | `readOnlyHint: true` |
> | 3. Plan | `replay_order_plan` | `readOnlyHint: true` |
> | 4. Advise | `replay_advise` (read), `replay_learn` | read-only; apply requires `--allow-write` |
> | 5. Health | `replay_health` | `readOnlyHint: true` |
>
> — `docs/architecture/mcp-server.md:163-169`

What the code actually does:

```text
$ grep -rn '"replay_tool_cost"\|"replay_blame"\|"replay_order_plan"\|"replay_health"\|
            "replay_session_cost"\|"replay_what_changed"\|"replay_advise"\|"replay_learn"\|
            "replay_context"\|"replay_diff"' --include="*.go" .
(no output — zero occurrences of any of the ten, in any Go file)
```

The seven tools the server does register — `replay_surfaces`,
`replay_price_check`, `replay_rules_free`, `replay_mcp_overhead`,
`replay_rules_latest`, `replay_installer_release`, `replay_quota`
(`cmd/replay/mcp.go:76,83,92,99,111,118,125`) — appear **nowhere in this
document**. The overlap between the designed surface and the shipped surface is
empty.

Two further claims in the same file rest on the same gap:

| Documentation | doc:line | What the code does |
|---|---|---|
| `replay_order_plan`, backed by *"`internal/analysis/order.go`, built 2026-09-05 and **the one piece of this design that exists**"* | `mcp-server.md:96-99` | `OrderPlan` is unreachable from `main` (§3.3). It is the one piece that exists and the one piece nobody can call. |
| *"MCP is the right place to surface the question, because `elicitation/create` lets the server ask the human directly"*, consent read by `internal/consent` | `mcp-server.md:230,251-253` | `grep -rn elicitation --include="*.go" .` → **zero hits**. `consent.ReadUpdateConsent` (`internal/consent/consent.go:96`) is unreachable from `main`. |

**Verdict: OVERPROMISED.** A reader who opens `docs/architecture/` to find out
what the MCP server does learns the names of ten tools that do not exist, is told
the server does not exist, and is not told the names of the seven that do.

### 4.3 `replay_quota` is documented as answering a question it cannot answer

> | `replay_quota` | the rate-limit window closest to binding, and time to reset | no |
> — `docs/guide/commands.md:1061`

The tool's own description repeats it to every connected agent:

> "The rate-limit window closest to binding, and how long until it resets, from
> the last reading the status line stored. Reports the reading's age, and says so
> plainly when there is no reading rather than reporting a full window."
> — `cmd/replay/mcp.go:126-128`

What the code does: returns the "no reading" sentence, every time, for the reason
in §3.1. The description's final clause is the only part that can ever run.

**Verdict: OVERPROMISED.** The honest wording today is "reports that no reading
has been stored".

### 4.4 The TUI's own help overlay advertises a flag the TUI does not have

```go
// internal/tui/keys.go:79
        Binding{"--json", "the same answer for a machine, no screen at all", L3},
```

`Help()` renders every L3 binding (`internal/tui/keys.go:162-165`), so this line
is on the screen a user reaches by pressing `?` — the one place progressive
disclosure sends someone who is looking for the ceiling of the vocabulary.

```text
$ /tmp/rp tui --json
flag provided but not defined: -json
Usage of tui:
  -color string ...
  -once ...
  -screen string ...
```

`runTUI` declares exactly three flags (`cmd/replay/tui.go:38-47`). No document
mentions `replay tui --json` either — `grep -rn 'tui.*--json' --include="*.md" .`
returns nothing — so the only place this flag is promised is inside the running
program.

**Verdict: OVERPROMISED**, and the most user-visible of the five, because the
promise is made by the binary itself at the moment someone asks for help.

### 4.5 The CHANGELOG announces a guard that has never fired

> **A pre-flight deficit warning**, off by default. Warns before a changed
> prefix is re-billed and refuses only against a ceiling the operator set.
> Consent is config, never a request header. A ceiling inside the estimate's own
> ±15% band suppresses the refusal rather than deciding it.
> — `CHANGELOG.md:357-360`

What the code does: nothing, in any configuration. There is no config path that
sets `Config.PreFlight`, so `internal/proxy/preflight.go:92` returns before the
warning, the refusal and the band check (§3.6). "Off by default" implies an on;
there is none. **Verdict: OVERPROMISED.**

### 4.6 Three documents describe an on-screen notice that can no longer render

| Documentation | doc:line | What the code does |
|---|---|---|
| *"The other five carry a notice saying they are example data and describe nobody, and they say it on screen rather than in a footnote. They are being wired one at a time"* | `docs/guide/commands.md:953-956` | All ten screens are wired. `Provenance.Example` has no producer (§2), so `Banner(Example)` cannot render. |
| *"the archetype renderings that do appear … are drawn from example data and say so"* | `docs/TUI-FLAG-SURFACE.md:6-9` (and *"nine of them now, and four read this machine"*, `:3-5`) | Ten screens, all measured. |
| *"Four of eight TUI screens still carry example data."* | `docs/GAP-ANALYSIS-2026-09-07.md:62` | Superseded by [tui-example-screens](../evidence/tui-example-screens-2026-09-08.md); not corrected in place. |

These understate rather than oversell, so they are not OVERPROMISED in the usual
direction — but the behaviour they describe is **UNREACHABLE**, and a reader
auditing the tool's honesty machinery is told to look for a banner that cannot
appear. `commands.md:953` in particular tells a user that half the surface is
fiction when it is not, which is the more damaging error of the three.

### 4.7 `docs/CLI.md` contradicts itself about `--json`, in the same file

> - **`--json` is not universal.** `cost`, `context`, `route`, `trim` and
>   `advise --apply` emit it; the rest print for people.
> — `docs/CLI.md:359-360`

Two flag tables earlier in the same document say otherwise:

> `| -json | bool | emit the finding as JSON for a CI step to act on |` — `docs/CLI.md:310` (`prefix`)
>
> `| -json | bool | emit the artefact as JSON, for committing and for the gate to read |` — `docs/CLI.md:326` (`budget`)

Both are real: `cmd/replay/prefix.go:112` and `cmd/replay/budget.go:90`. The
flag tables in `CLI.md` are generated from the binary and are correct; the
hand-written note beneath them is not. The two commands the note omits are the
two built specifically to be read by a CI gate, so an agent that trusts the note
will not pass `--json` to exactly the commands where it matters most.
**Verdict: OVERPROMISED in reverse** — the doc under-reports a shipped
capability, which is the same defect as §4.6 and costs a machine reader more.

### 4.8 Three command and flag counts are wrong

| Documentation | doc:line | Actual |
|---|---|---|
| *"`replay --help` lists all twenty"* | `README.md:158` | `printUsage` (`cmd/replay/main.go:552-601`) lists 25 |
| *"Replay has **80 flags across 14 commands**. `serve` carries 29 of them"* | `docs/TUI-FLAG-SURFACE.md:11` | *"25 commands, 90 flags, read from the binary"* — `docs/CLI.md:368`, which is generated. `cmd/replay/serve.go` declares 30. |
| *"Replay has 75 flags across 13 commands"* | `docs/AGENT-SURFACE.md:69` | as above |
| *"four of the nine read this machine: cost, why, doctor and share. The other five carry an example-data notice."* | `docs/AGENT-SURFACE.md:71-73` | ten screens, all measured; the notice cannot render (§2) |

`TUI-FLAG-SURFACE.md:19-20` asserts *"Every flag below was extracted from
`cmd/replay/*.go`, not from documentation. All 80 are classified; none are left
over."* Ten flags have since been added and none were classified, so the
completeness claim is the part that broke, not the arithmetic.
**Verdict: OVERPROMISED** — a stated-complete classification that is no longer
complete is worse than an undated count, because the sentence tells the reader
not to check.

---

## 5. Shipped and documented nowhere

### 5.1 `replay tui --screen live` works; the binary says there are nine screens

```text
$ /tmp/rp tui --once --screen live -color never
  no proxy answered at 127.0.0.1:4000
  ...
  live   ? keys  esc back  q quit
```

`live` is the tenth entry in `internal/tui/shortcuts.go:98-99` and has a case in
the dispatch at `cmd/replay/tui.go:175-177`. But the flag's own help string
omits it:

```go
// cmd/replay/tui.go:38-39
    screen := fs.String("screen", "cost", "which question to open on: "+
        "cost, why, context, advise, guards, model, safe, doctor, share")
```

and so does the error a user gets for a typo:

```text
$ /tmp/rp tui --once --screen bogus
replay: no screen called "bogus". The nine are: cost, why, context, advise,
guards, model, safe, doctor, share: invalid usage
```

Ten screens, and both of the binary's own lists say nine and name the same nine.
`docs/guide/commands.md:949` gets it right and lists all ten, so a user who reads
the guide can discover a screen that `--help` will then deny.

**Verdict: UNDOCUMENTED** (in the binary's own surface), and a live drift between
`cmd/replay/tui.go:39` and `docs/guide/commands.md:949`.

### 5.2 Four whole packages ship no behaviour — and one is now an MCP dependency

`internal/feed` (403 lines), `internal/otlp` (505), `internal/quota` (995),
`internal/usage` (427) are absent from `go list -deps ./cmd/replay`. This is
[UNWIRED-LOG](UNWIRED-LOG.md) #5, #8 and #9 and is already guarded by
`internal/regression/unwired_packages_test.go:34`.

The new fact is the one in §3.1: `internal/quota` is not only unimported, it is
the package that would supply the answer for a tool the shipped MCP server
already advertises to every connected agent. The gap is no longer internal.

Test counts over the four: 19 (`quota`), 11 (`usage`), 8 (`feed`), 7 (`otlp`) —
**45 passing tests over code that is not in the binary.**

### 5.3 `replay help` is a real subcommand and appears in no document

`cmd/replay/main.go:126` dispatches `help` alongside `--help` and `-h`.

```text
$ grep -rn "replay help" --include="*.md" .
(no output)
```

It is also the one dispatch case missing from `printUsage` itself
(`cmd/replay/main.go:552-601` lists every other command). Small, but it is the
bare word a person types first on an unfamiliar CLI, and nothing tells them it
works. **Verdict: UNDOCUMENTED.**

### 5.4 `replay mcp --install` is handled outside the flag system

`cmd/replay/main.go:139` intercepts `--install` / `-install` before `runMCP`,
so it appears in no `FlagSet` and in no `--help` output. It is documented at
`docs/guide/getting-started.md:149`. Recorded in part 2 §4; noted here only
because it is the one subcommand argument that `docs_drift_test.go` structurally
cannot see.

---

## 6. Defensive by design — not defects

Reported so that a later reader does not re-file them.

| Item | file:line | Why it is meant never to fire |
|---|---|---|
| `EvaluatePreFlightPolicy`'s `if !optInActive { return false }` | `internal/analysis/predictor.go:46-48` | This is the ADR-0011 consent gate, and it is *correct* that it refuses without opt-in. The defect is upstream — nothing can grant the opt-in — not here. **REACHABLE-BY-DESIGN.** |
| `pickVariant`'s `default:` error arm | `cmd/replay/sharepng.go:95-97` | Fires on user typo. **REACHABLE.** |
| `Banner`'s `default: return ""` | `internal/tui/provenance.go:47-48` | The `Measured` case, which is now every screen. **REACHABLE.** |
| `internal/regression`, `internal/mutation` | whole packages | Test-only harnesses; `mutation` is build-tag gated (`go test -tags mutation`). Correctly absent from the binary. **REACHABLE-BY-DESIGN.** |
| `internal/tui/storyboard.go` (312 lines, unreachable from `main`) | `internal/tui/storyboard.go` | It is a specification rendered by the real formatter, so a state cannot be drawn that the formatter would not produce (`:9-15`). Consumed by `internal/tui` tests and `scripts/tui-audit`. **REACHABLE-BY-DESIGN**, though it is worth noting that `Header`, `Traffic`, `Empty`, `Storyboard`, `Render`, `kv`, `stat` and `note2` are all dead to the binary and four of them have no test reference either. |

---

## 7. Ranked by what a user loses

1. **`replay advise --guards` prints a command the same binary rejects** (§4.1).
   Everything upstream of the last line is correct; the last line is unusable.
   The fix is two string literals.
2. **A third of measured waste has no surface.** `MeasureIdleRisk` (§3.2) is the
   only large break cause a user can still act on, and there is no way to ask for
   it. Cheapest large fix in this document: one MCP tool or one `doctor` line.
3. **An agent is told `replay_quota` reports a window and gets a refusal every
   time** (§3.1, §4.3). The fix is one call to `saveQuota` in `runStatusline`.
4. **`replay advise` names a status no suggestion can carry** (§3.5). A user
   waiting for "applied" waits forever, and the legend told them to wait.
5. **A reader of `docs/architecture/mcp-server.md` learns ten tool names that do
   not exist and none of the seven that do** (§4.2).
6. **Five error screens written for the moment a user is stuck are unreachable**
   (§3.4). The messages the user gets instead are whatever the call site happened
   to write.
7. **`replay tui`'s `?` overlay promises `--json`** (§4.4). The one flag a user
   discovers from inside the program is the one that does not exist.
8. **An agent following `docs/CLI.md:359` will not pass `--json` to `prefix` or
   `budget`** (§4.7) — the two commands built to be read by a CI gate.
9. **`--screen live` works and the binary denies it** (§5.1).
10. **The four repaint cadences are one cadence** (§3.8), so a spend total
    redraws four times a second — the thing the constant's own comment says
    makes a number unreadable.
11. **`--metrics-listen` never reports where it bound** (§3.6).

---

## 8. Why the existing guards miss all of this

`internal/regression/unwired_packages_test.go` compares packages against
`go list -deps ./cmd/replay`. Every finding in §2 and §3 lives **inside a package
that is in the binary** — `internal/tui`, `internal/analysis`, `internal/proxy`,
`cmd/replay` — so the guard cannot see any of them by construction. It answers
"does this package ship?", not "can this line run?".

`cmd/replay/docs_drift_test.go:19` asserts every command in `--help` has a
documentation entry. It checks one direction, at command granularity: it cannot
see a screen name missing from a flag's help string (§5.1), a flag promised only
by an in-program overlay (§4.3), or a branch that no input selects.

And the doc-guard suite is currently **red**, which is consistent with none of
the above being caught:

```text
$ go test ./cmd/replay/ -run 'Doc|Drift'
--- FAIL: TestNoOrphanedDocuments (0.02s)
    docs_drift_test.go:217: documents nothing links to, so nobody will find them:
          docs/design/surface-taxonomy-2-response.md
          docs/design/surface-taxonomy-3-providers.md
          docs/design/surface-taxonomy-4-rewriting.md
          docs/design/unwired-1-symbols.md
          docs/design/unwired-2-config.md
          docs/design/unwired-3-branches-and-docs.md
```

Six audit documents, this one included, are unlinked from `docs/design/README.md`,
whose table still lists one entry. This audit did not fix it, because the brief
was to modify nothing but its own output — but the guard is doing its job and is
being ignored, which is the same failure mode one level up.

The check that would catch the code family is the one used to produce this
report: `deadcode ./cmd/replay` with a reviewed baseline, in the same shape as
`unwired_packages_test.go` — an allowlist with a reason per entry, failing when a
new unreachable function appears or a listed one is deleted rather than wired.
It would have caught 11 of the 12 findings in §2 and §3 on the day each landed.

The check that would catch §4.1 is narrower and cheaper: a test asserting that
every `--flag` appearing in any string this binary prints is defined by some
`FlagSet` in `cmd/replay`. One test, and it goes red today.

---

[Design](README.md) · [Unwired log](UNWIRED-LOG.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
