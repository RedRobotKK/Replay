# Built-but-unwired audit, part 1 of 3: exported symbols with no production caller

Scope: every exported function, method, type, constant and variable declared in
`internal/` and `cmd/` — **798 symbols** — checked for a caller outside
`_test.go` that is transitively reachable from `cmd/replay`.

**Result: 111 UNWIRED exported symbols. 82 of them have tests.**

---

## Method, and how it can fail

Three independent passes, because a grep alone cannot see transitivity and a
call-graph alone cannot see types and constants.

1. **Enumeration.** `go/ast` parse of every non-`_test.go` file under
   `internal/` and `cmd/`, collecting `FuncDecl` (exported), `TypeSpec`,
   and exported `ValueSpec` names. Total 798. Cross-checked against an
   independent regex enumerator — the two sets are identical (0 symmetric
   difference), so the denominator is not guessed.
2. **Reachability for funcs and methods.** `golang.org/x/tools/cmd/deadcode`
   run as `deadcode -json ./cmd/replay`. This is rapid type analysis over the
   real call graph from `main`, so a function called only from another dead
   function is itself reported dead — the transitive resolution the brief asks
   for. It reported **64 dead functions across 13 packages**, plus the four
   packages that are not in the build graph at all.
3. **Reachability for types, constants and variables**, which `deadcode` does
   not model. For each symbol: package-qualified grep (`pkg.Symbol`) across
   every non-test file outside its own package, plus a bare-identifier grep
   inside its own package with line comments stripped. Each hit is then mapped
   to its enclosing function and discarded if that function is in the dead set.
   A symbol with no surviving reference is unwired.

**Where this can be wrong, stated plainly:**

- `deadcode` uses RTA. A function reached only by reflection would be
  misreported as dead. The only `reflect` use in the tree is
  `cmd/replay/costcache.go:167`, a `reflect.TypeOf(costUnit{})` on a local
  struct — no dynamic dispatch. So RTA is sound here.

- The run was on **darwin**. Files behind `//go:build linux`, `!unix`, or
  `windows` (`internal/tui/term_linux.go`, `term_other.go`,
  `isterm_other.go`, `winsize_other.go`, `internal/consent/ownership_windows.go`)
  were not analysed. No symbol in this report lives in one of those files, but
  a Linux- or Windows-only unwired symbol would not have been caught.

- **Interface satisfaction counts as wired** and is handled by `deadcode`, not
  by me. Methods listed here as UNWIRED are ones RTA found no dynamic call site
  for, given the concrete types the program actually instantiates.

- **Two-sided sanity check on the instrument.** `saveQuota`
  (`cmd/replay/quotastore.go:37`) appears in the dead set and grep independently
  confirms its only callers are `cmd/replay/quotastore_test.go`. Its neighbour
  `loadQuota` (`cmd/replay/quotastore.go:81`) does *not* appear in the dead set
  and grep independently finds a live caller at `cmd/replay/mcp.go:269`. The
  tool distinguishes the two correctly. The same two-sided check was repeated
  by hand for roughly twenty further symbols across `tui`, `analysis`,
  `masking`, `probe` and `money`.

- Where the answer is "the tool says dead but the mechanism is beyond RTA", the
  row says **not determined**. No row does, in this report.

---

## Ranked by consequence

Count is not the ranking. A dead helper is noise; a dead guard against a live
defect is the finding.

### 1. `internal/usage` — the guard against a live double-count (11 symbols, 9 with tests)

`internal/usage` has **zero importers anywhere in the tree, test files
included** — verified by `grep -rn --include="*.go" 'internal/usage"'`, which
returns nothing.

`FromInclusive` (`internal/usage/usage.go:179`) and `Record.Validate`
(`internal/usage/usage.go:204`) exist for exactly one reason, stated in their
own doc comments: providers disagree about whether the prompt figure already
contains the cached tokens. Anthropic counts exclusively; OpenAI counts
inclusively. `FromInclusive` derives `Fresh` by subtraction; `Validate` refuses
any record where `Fresh + CachedRead + CachedWrite != Prompt`.

The Codex reader does the copy the comment warns against:

```go
// internal/transcript/codex.go:150-155
return Usage{
    Input:          c.Input,     // OpenAI's inclusive prompt total
    CacheRead:      c.Cached,    // ...and the part of it served from cache
    Output:         c.Output,
    ThinkingTokens: c.Reasoning,
}, true
```

`c.Cached` is a subset of `c.Input` — the guard immediately above it at
`internal/transcript/codex.go:140` (`if c.Cached > c.Input || c.Reasoning > c.Output { return Usage{}, false }`)
proves the code knows that. So the cached tokens are counted once inside
`Input` and again as `CacheRead`.

**What the user is missing:** every Codex figure that divides by the prompt —
cached share, break-even threshold, cost — is computed on an inflated
denominator, and the error is largest on the best-cached sessions. `Validate`
would have refused those records loudly. It runs only in
`internal/usage/usage_test.go`.

### 2. `internal/masking.LooksLikeHexSecret` — an unwired secret detector (`internal/masking/entropy.go:177`)

Tested at `internal/masking/failclosed_test.go:99,108,118`. Called from nothing:
`grep -rn --include="*.go" 'LooksLikeHexSecret'` returns the declaration and
those three test lines only.

Its doc comment names its own reason for existing: "the entropy detector's blind
spot is exactly the hex with no envelope". That blind spot is real and
structural — `looksLikeCredential` (`internal/masking/entropy.go:87`) requires
`seen[classLower] && seen[classUpper] && seen[classDigit]`, so a lowercase hex
credential can never be flagged by the entropy path, at any length or entropy.

The pattern matcher does not close the gap either. `credential-assignment`
(`internal/masking/patterns.go:47-49`) requires a `[:=]` between the cue and the
value. Replaying that regex against a 32-char lowercase hex key:

| line | pattern matcher |
|---|---|
| `api_key = "0123456789abcdef0123456789abcdef"` | catches |
| `x-api-key: 0123456789abcdef0123456789abcdef` | catches |
| `Authorization: Bearer 0123456789abcdef0123456789abcdef` | **misses** |
| `curl --token 0123456789abcdef0123456789abcdef https://x` | **misses** |
| `auth token 0123456789abcdef0123456789abcdef` | **misses** |
| `the secret is 0123456789abcdef0123456789abcdef` | **misses** |

`LooksLikeHexSecret` catches all six — its window is 40 characters of preceding
text and requires no operator (`internal/masking/entropy.go:186`).

**What the user is missing:** a lowercase-hex credential presented without an
assignment operator — the `Bearer` header form most of all — passes through the
redaction proxy unmasked. Nothing else in the package can catch it.

### 3. `internal/quota` + `saveQuota` — the sensor that is never read from and never written to (11 symbols, 5 with tests)

`internal/quota` has zero importers anywhere (same grep, same empty result), and
is absent from `go list -deps ./cmd/replay`. That covers the header tables
(`RemainingHeaders` `quota.go:26`, `UtilizationHeaders` `quota.go:44`), the
subtraction measurement (`Samples` `quota.go:151`, `Compare` `quota.go:232`) and
the projection (`Forecast` `forecast.go:56`).

The companion defect is in `cmd/replay`, in unexported code: `saveQuota`
(`cmd/replay/quotastore.go:37`) has no production caller, while `loadQuota`
(`cmd/replay/quotastore.go:81`) is called at `cmd/replay/mcp.go:269`. The file
is read and never written.

**What the user is missing:** the flat-seat subscriber — the population
`internal/quota`'s package comment says the package exists for — gets no
rate-limit measurement and no "when does this seat run out" forecast, and the
quota line answers "no reading stored" permanently, because the reading arrives
several times a second from the status line and is discarded.

### 4. `internal/analysis/provenance.go` — an unshown correctness warning (4 symbols, 2 with tests)

`ReadProvenance` (`provenance.go:59`), `Provenance.Warning` (`provenance.go:97`),
`SessionReadPaths` (`provenance.go:126`) and the `Provenance` type
(`provenance.go:39`). Package-qualified grep for `analysis.Provenance`,
`analysis.ReadProvenance` and `analysis.SessionReadPaths` across the tree
returns nothing outside the package, and inside the package the only other
mention of the word is a string literal at `internal/analysis/report.go:192`.

**What the user is missing:** the warning at `internal/analysis/provenance.go:101-106` — that
every file a session read came from one directory, so "sources that share a
directory agree by construction" — is never printed. This is a
completeness-of-evidence caveat about the user's own session, and it is exactly
the class of caveat this codebase argues should never be silent.

### 5. `internal/analysis/order.go` and `idle.go` — two whole analyses (9 symbols, 7 with tests)

`OrderPlan` (`order.go:90`) computes a re-ordering of a plan's actions that
avoids cache invalidations, and reports `SavedTokens` / `SavedUSD`. Tested
across `order_test.go` (14 references to `ScopeAppend` alone). `MeasureIdleRisk`
(`idle.go:58`) computes what a session is about to lose when its cache expires,
with `Warn` gated at a 25,000-token floor. Tested in `idle_test.go`.

Neither type nor function is referenced outside its own file. **What the user is
missing:** two complete recommendations — "reorder these actions and save $X"
and "your cache expires in N minutes and that costs Y" — that exist, are
correct, and no command can reach.

### 6. `internal/feed` (8 symbols, 7 with tests) and `internal/otlp` (8 symbols, 7 with tests)

Both packages: zero importers, absent from `go list -deps ./cmd/replay`.

`internal/feed` is the ed25519-verified staleness feed. Its package comment
names the measured problem it solves — "the published v0.5.0 binary carried a
price table 75 days old". **What the user is missing:** no way to learn their
compiled price and rules tables have gone stale. `replay rules --update` still
works, but only if the user already knows to run it.

`internal/otlp` writes findings as OpenTelemetry spans to a file. **What the
user is missing:** the entire OTel export. There is no `replay` path that emits
a span.

### 7. `internal/cachemodel.Incoherent` (`internal/cachemodel/measure.go:119`)

Tested in `measure_test.go`; referenced elsewhere only in a comment at
`measure.go:47`. Its doc comment insists the contradiction "is surfaced rather
than smoothed away". **What the user is missing:** it is not surfaced. A model
whose caching-floor evidence is self-contradictory — a prompt cached below a
size at which another prompt failed to cache — is reported as though the
evidence were coherent.

### 8. The `internal/tui` design cluster (41 symbols, 30 with tests)

Five files form one mutually-referencing dead cluster: `storyboard.go`,
`errors.go`, `liveness.go`, `refresh.go`, `outcomes.go`, plus loose ends in
`layout.go`, `color.go`, `shortcuts.go`, `advise.go`, `provenance.go`,
`loop.go`. They reference each other (`Header()` and `Empty()` are called from
`liveness.go:89,90`, `Traffic()` from `storyboard.go:134`) and nothing outside
calls in. The live surface enters through `tui.StartWith`
(`cmd/replay/tui.go:233`), never `tui.Start` (`internal/tui/run.go:23`).

**What the user is missing: mostly nothing.** This reads as executable design
specification — `Storyboard()` generates the design's frames using the real
formatter precisely so a hand-drawn frame cannot disagree with the code, and
its tests are the point of it. Two exceptions worth a look:

- `Problems()` (`errors.go:43`) and `Problem.Render()` (`errors.go:127`) are a
  catalogue of real refusals with "what happened / why / what to do / what is
  unaffected" text — including a `chmod 600 ~/.config/replay/corpus-consent.toml`
  remedy at `errors.go:100`. If the running program does not emit these, users
  hitting those refusals get a lesser message.

- `Hints()` (`shortcuts.go:135`) is the per-screen keystroke discoverability
  line. Unreachable means the surface never tells users what else they can press.

### 9. Nothing is missing — redundant wrappers, superseded entry points, and enums with no product behind them

Named explicitly so a later reader does not re-litigate them.

| Symbol | file:line | Why nothing is missing |
|---|---|---|
| `consent.ReadUpdateConsent()`, `consent.FileName` | `internal/consent/consent.go:96,29` | There is deliberately no update check to gate: `cmd/replay/rules.go:145` states "no background refresh, no check-for-updates on startup". `ReadCorpusConsent` — the sibling with a product behind it — *is* wired at `cmd/replay/contribute.go:109`. |
| `tui.Start()` | `internal/tui/run.go:23` | Superseded by `tui.StartWith`, live at `cmd/replay/tui.go:233`. |
| `learn.LoadSelected()` | `internal/learn/learn.go:470` | Convenience wrapper; the proxy uses `LoadFile` + `SelectionFor` directly at `internal/proxy/server.go:914`, which is strictly more capable (it passes a session type). |
| `masking.Find()` | `internal/masking/patterns.go:59` | Exported wrapper over unexported `find`, which production uses at `internal/masking/mask.go:144`. |
| `masking.NewWithVault()` | `internal/masking/mask.go:46` | **TEST-ONLY BY DESIGN.** Its own doc says so: "Exists so the vault-failure path can be exercised; production uses New." Used at `failclosed_test.go:31,47`. |
| `proxy.Server.MetricsAddr()` | `internal/proxy/metrics_listener.go:91` | **TEST-ONLY BY DESIGN.** A bound-address accessor that blocks on `s.ready`; its only purpose is letting a test reach an ephemeral port (`metrics_listener_test.go:84,116,152,280`). Production is told the address by config. |
| `observation.AccountTag()` | `internal/observation/observation.go:104` | The privacy-preferring sibling `LocalTag` was chosen instead, at `cmd/replay/contribute.go:74`. Wiring `AccountTag` would make the tag linkable to a provider account. |
| `card.Variants()`, `card.Tones()` | `internal/card/card.go:30`, `tone.go:34` | The two arms are hard-coded at `cmd/replay/sharetui.go:43-50` and `sharepng.go:90`. A maintenance trap — a third variant would not be picked up — but nothing is absent today. |
| `advisor.Applied`, `tui.Pattern`, `tui.TierFrame/TierMeaning/TierFlourish` | `internal/advisor/advisor.go:71`, `internal/tui/layout.go:39,58,61,65` | Documentation constants and an unused enum member. |
| `money.Display.Line()`, `money.Display.Amount()` | `internal/money/money.go:134,156` | Cosmetic. The non-USD reader still gets converted figures via `Short()` (`cmd/replay/cost.go:731`) and the rate caveat via `Note()` (`cost.go:310`). What is absent is the standalone `"$12.00 (about ¥1,800 at 150/USD, 2026-09-01)"` form. |
| `transcript.SourceOllama`, `OllamaRequest.CacheHitRate()` | `internal/transcript/ollama.go:13,71` | The three-outcome Ollama cache-hit-rate accessor is never called; Ollama request records still parse. |
| `probe.Search.AtMost()`, `probe.Runner.Overhead()` | `internal/probe/probe.go:430`, `run.go:122` | Accessors superseded by struct fields the callers read directly (`internal/probe/reading.go:136`). Note the `AtMost` *field* on `Reading` is live and is a different symbol. |
| `cachemodel.ExportedAt()` | `internal/cachemodel/export.go:61` | Timestamp helper for a generated document; the export path stamps `PriceTableCheckedAt` instead. |

---

## Full table

Every UNWIRED symbol. "Non-test caller?" is "none" throughout — that is what
puts a row in this table. Each row was produced by the three-pass method above;
no row is here without both a dead-set entry (funcs/methods) or an empty
qualified grep (types/constants/vars).

### `internal/advisor` — 1 unwired, 0 with tests

| Symbol | Kind | file:line | Tests covering it | Non-test caller? | Verdict |
|---|---|---|---|---|---|
| `Applied` | const | `internal/advisor/advisor.go:71` | none | none | **UNWIRED** |

#### `internal/analysis` — 13 unwired, 9 with tests

| Symbol | Kind | file:line | Tests covering it | Non-test caller? | Verdict |
|---|---|---|---|---|---|
| `IdleRisk` | type | `internal/analysis/idle.go:39` | none | none | **UNWIRED** |
| `MeasureIdleRisk()` | func | `internal/analysis/idle.go:58` | idle_test.go | none | **UNWIRED** |
| `ScopeAppend` | const | `internal/analysis/order.go:43` | order_test.go | none | **UNWIRED** |
| `ScopeSystem` | const | `internal/analysis/order.go:45` | order_test.go | none | **UNWIRED** |
| `ScopeTools` | const | `internal/analysis/order.go:50` | order_test.go | none | **UNWIRED** |
| `ScopeMemory` | const | `internal/analysis/order.go:52` | order_test.go | none | **UNWIRED** |
| `Action` | type | `internal/analysis/order.go:61` | order_test.go | none | **UNWIRED** |
| `PlanOrder` | type | `internal/analysis/order.go:68` | none | none | **UNWIRED** |
| `OrderPlan()` | func | `internal/analysis/order.go:90` | order_test.go | none | **UNWIRED** |
| `Provenance` | type | `internal/analysis/provenance.go:39` | none | none | **UNWIRED** |
| `ReadProvenance()` | func | `internal/analysis/provenance.go:59` | provenance_test.go | none | **UNWIRED** |
| `Provenance.Warning()` | method | `internal/analysis/provenance.go:97` | provenance_test.go | none | **UNWIRED** |
| `SessionReadPaths()` | func | `internal/analysis/provenance.go:126` | none | none | **UNWIRED** |

#### `internal/cachemodel` — 2 unwired, 1 with tests

| Symbol | Kind | file:line | Tests covering it | Non-test caller? | Verdict |
|---|---|---|---|---|---|
| `ExportedAt()` | func | `internal/cachemodel/export.go:61` | none | none | **UNWIRED** |
| `Incoherent()` | func | `internal/cachemodel/measure.go:119` | measure_test.go | none | **UNWIRED** |

#### `internal/card` — 2 unwired, 2 with tests

| Symbol | Kind | file:line | Tests covering it | Non-test caller? | Verdict |
|---|---|---|---|---|---|
| `Variants()` | func | `internal/card/card.go:30` | card_test.go, tone_test.go, share_test.go | none | **UNWIRED** |
| `Tones()` | func | `internal/card/tone.go:34` | tone_test.go, share_test.go | none | **UNWIRED** |

#### `internal/consent` — 2 unwired, 2 with tests

| Symbol | Kind | file:line | Tests covering it | Non-test caller? | Verdict |
|---|---|---|---|---|---|
| `FileName` | const | `internal/consent/consent.go:29` | consent_test.go | none | **UNWIRED** |
| `ReadUpdateConsent()` | func | `internal/consent/consent.go:96` | consent_test.go | none | **UNWIRED** |

#### `internal/feed` — 8 unwired, 7 with tests

| Symbol | Kind | file:line | Tests covering it | Non-test caller? | Verdict |
|---|---|---|---|---|---|
| `VerifyFromVendor()` | func | `internal/feed/feed.go:70` | feed_test.go | none | **UNWIRED** |
| `HasVendorKey()` | func | `internal/feed/feed.go:76` | feed_test.go | none | **UNWIRED** |
| `MaxBundle` | const | `internal/feed/feed.go:83` | none | none | **UNWIRED** |
| `Bundle` | type | `internal/feed/feed.go:91` | feed_test.go | none | **UNWIRED** |
| `Verify()` | func | `internal/feed/feed.go:115` | feed_test.go | none | **UNWIRED** |
| `Bundle.NewerThan()` | method | `internal/feed/feed.go:141` | feed_test.go | none | **UNWIRED** |
| `CheckSource()` | func | `internal/feed/feed.go:150` | feed_test.go | none | **UNWIRED** |
| `Fetch()` | func | `internal/feed/feed.go:172` | feed_test.go | none | **UNWIRED** |

#### `internal/learn` — 1 unwired, 1 with tests

| Symbol | Kind | file:line | Tests covering it | Non-test caller? | Verdict |
|---|---|---|---|---|---|
| `LoadSelected()` | func | `internal/learn/learn.go:470` | learn_test.go | none | **UNWIRED** |

#### `internal/masking` — 3 unwired, 3 with tests

| Symbol | Kind | file:line | Tests covering it | Non-test caller? | Verdict |
|---|---|---|---|---|---|
| `LooksLikeHexSecret()` | func | `internal/masking/entropy.go:177` | failclosed_test.go | none | **UNWIRED** |
| `NewWithVault()` | func | `internal/masking/mask.go:46` | failclosed_test.go, maskfailclosed_test.go | none | **UNWIRED** |
| `Find()` | func | `internal/masking/patterns.go:59` | masking_test.go | none | **UNWIRED** |

#### `internal/money` — 2 unwired, 1 with tests

| Symbol | Kind | file:line | Tests covering it | Non-test caller? | Verdict |
|---|---|---|---|---|---|
| `Display.Line()` | method | `internal/money/money.go:134` | money_test.go | none | **UNWIRED** |
| `Display.Amount()` | method | `internal/money/money.go:156` | none | none | **UNWIRED** |

#### `internal/observation` — 1 unwired, 1 with tests

| Symbol | Kind | file:line | Tests covering it | Non-test caller? | Verdict |
|---|---|---|---|---|---|
| `AccountTag()` | func | `internal/observation/observation.go:104` | observation_test.go | none | **UNWIRED** |

#### `internal/otlp` — 8 unwired, 7 with tests

| Symbol | Kind | file:line | Tests covering it | Non-test caller? | Verdict |
|---|---|---|---|---|---|
| `Turn` | type | `internal/otlp/otlp.go:57` | otlp_test.go | none | **UNWIRED** |
| `Value` | type | `internal/otlp/otlp.go:79` | otlp_test.go | none | **UNWIRED** |
| `Attr` | type | `internal/otlp/otlp.go:85` | none | none | **UNWIRED** |
| `Span` | type | `internal/otlp/otlp.go:91` | otlp_test.go | none | **UNWIRED** |
| `Builder` | type | `internal/otlp/otlp.go:102` | otlp_test.go | none | **UNWIRED** |
| `New()` | func | `internal/otlp/otlp.go:106` | otlp_test.go | none | **UNWIRED** |
| `Builder.Turn()` | method | `internal/otlp/otlp.go:131` | otlp_test.go | none | **UNWIRED** |
| `Write()` | func | `internal/otlp/otlp.go:171` | otlp_test.go | none | **UNWIRED** |

#### `internal/probe` — 2 unwired, 2 with tests

| Symbol | Kind | file:line | Tests covering it | Non-test caller? | Verdict |
|---|---|---|---|---|---|
| `Search.AtMost()` | method | `internal/probe/probe.go:430` | probe_test.go, reading_test.go, trend_test.go | none | **UNWIRED** |
| `Runner.Overhead()` | method | `internal/probe/run.go:122` | run_test.go | none | **UNWIRED** |

#### `internal/proxy` — 1 unwired, 1 with tests

| Symbol | Kind | file:line | Tests covering it | Non-test caller? | Verdict |
|---|---|---|---|---|---|
| `Server.MetricsAddr()` | method | `internal/proxy/metrics_listener.go:91` | metrics_listener_test.go | none | **UNWIRED** |

#### `internal/quota` — 11 unwired, 5 with tests

| Symbol | Kind | file:line | Tests covering it | Non-test caller? | Verdict |
|---|---|---|---|---|---|
| `Reading` | type | `internal/quota/forecast.go:36` | forecast_test.go | none | **UNWIRED** |
| `Projection` | type | `internal/quota/forecast.go:44` | none | none | **UNWIRED** |
| `Forecast()` | func | `internal/quota/forecast.go:56` | forecast_test.go | none | **UNWIRED** |
| `RemainingHeaders` | var | `internal/quota/quota.go:26` | none | none | **UNWIRED** |
| `UtilizationHeaders` | var | `internal/quota/quota.go:44` | none | none | **UNWIRED** |
| `MinStepsPerArm` | const | `internal/quota/quota.go:56` | none | none | **UNWIRED** |
| `MinPerArm` | const | `internal/quota/quota.go:59` | quota_test.go | none | **UNWIRED** |
| `Sample` | type | `internal/quota/quota.go:95` | none | none | **UNWIRED** |
| `Samples()` | func | `internal/quota/quota.go:151` | quota_test.go | none | **UNWIRED** |
| `Comparison` | type | `internal/quota/quota.go:201` | none | none | **UNWIRED** |
| `Compare()` | func | `internal/quota/quota.go:232` | quota_test.go | none | **UNWIRED** |

#### `internal/transcript` — 2 unwired, 1 with tests

| Symbol | Kind | file:line | Tests covering it | Non-test caller? | Verdict |
|---|---|---|---|---|---|
| `SourceOllama` | const | `internal/transcript/ollama.go:13` | none | none | **UNWIRED** |
| `OllamaRequest.CacheHitRate()` | method | `internal/transcript/ollama.go:71` | ollamaunmeasured_test.go | none | **UNWIRED** |

#### `internal/tui` — 41 unwired, 30 with tests

| Symbol | Kind | file:line | Tests covering it | Non-test caller? | Verdict |
|---|---|---|---|---|---|
| `AdviceRowsFit()` | func | `internal/tui/advise.go:167` | none | none | **UNWIRED** |
| `PaletteCodes()` | func | `internal/tui/color.go:82` | tui_color_test.go | none | **UNWIRED** |
| `Painter.On()` | method | `internal/tui/color.go:131` | none | none | **UNWIRED** |
| `Active()` | func | `internal/tui/color.go:227` | none | none | **UNWIRED** |
| `Problem` | type | `internal/tui/errors.go:28` | none | none | **UNWIRED** |
| `Problems()` | func | `internal/tui/errors.go:43` | errors_test.go | none | **UNWIRED** |
| `Problem.Render()` | method | `internal/tui/errors.go:127` | errors_test.go, liveness_test.go | none | **UNWIRED** |
| `Problem.Blocking()` | method | `internal/tui/errors.go:150` | errors_test.go | none | **UNWIRED** |
| `Pattern` | const | `internal/tui/layout.go:39` | none | none | **UNWIRED** |
| `TierFrame` | const | `internal/tui/layout.go:58` | none | none | **UNWIRED** |
| `TierMeaning` | const | `internal/tui/layout.go:61` | none | none | **UNWIRED** |
| `TierFlourish` | const | `internal/tui/layout.go:65` | none | none | **UNWIRED** |
| `Meter()` | func | `internal/tui/layout.go:73` | keys_test.go | none | **UNWIRED** |
| `GlyphSafety` | var | `internal/tui/layout.go:108` | keys_test.go | none | **UNWIRED** |
| `SafeInAFrame()` | func | `internal/tui/layout.go:123` | keys_test.go | none | **UNWIRED** |
| `Heartbeat()` | func | `internal/tui/liveness.go:36` | liveness_test.go | none | **UNWIRED** |
| `LiveRow()` | func | `internal/tui/liveness.go:62` | liveness_test.go | none | **UNWIRED** |
| `LiveScene` | type | `internal/tui/liveness.go:71` | none | none | **UNWIRED** |
| `LiveScenes()` | func | `internal/tui/liveness.go:79` | liveness_test.go | none | **UNWIRED** |
| `Loop.Painted()` | method | `internal/tui/loop.go:272` | loop_test.go | none | **UNWIRED** |
| `Outcomes()` | func | `internal/tui/outcomes.go:209` | outcomes_test.go, provenance_test.go | none | **UNWIRED** |
| `Screen.String()` | method | `internal/tui/outcomes.go:218` | artifact_test.go, loop_test.go | none | **UNWIRED** |
| `Marked()` | func | `internal/tui/provenance.go:73` | share_tui_test.go, measured_test.go, provenance_test.go, share_test.go | none | **UNWIRED** |
| `TickTraffic` | const | `internal/tui/refresh.go:39` | refresh_test.go | none | **UNWIRED** |
| `TickTotals` | const | `internal/tui/refresh.go:44` | refresh_test.go | none | **UNWIRED** |
| `TickAmbient` | const | `internal/tui/refresh.go:50` | refresh_test.go | none | **UNWIRED** |
| `Attention` | type | `internal/tui/refresh.go:54` | refresh_test.go | none | **UNWIRED** |
| `Calm` | const | `internal/tui/refresh.go:59` | refresh_test.go | none | **UNWIRED** |
| `Notice` | const | `internal/tui/refresh.go:61` | refresh_test.go | none | **UNWIRED** |
| `Act` | const | `internal/tui/refresh.go:63` | refresh_test.go | none | **UNWIRED** |
| `Glance()` | func | `internal/tui/refresh.go:75` | refresh_test.go | none | **UNWIRED** |
| `Since()` | func | `internal/tui/refresh.go:102` | refresh_test.go | none | **UNWIRED** |
| `Anchored()` | func | `internal/tui/refresh.go:134` | refresh_test.go | none | **UNWIRED** |
| `Start()` | func | `internal/tui/run.go:23` | none | none | **UNWIRED** |
| `Hints()` | func | `internal/tui/shortcuts.go:135` | shortcuts_test.go | none | **UNWIRED** |
| `Scene` | type | `internal/tui/storyboard.go:33` | none | none | **UNWIRED** |
| `Header()` | func | `internal/tui/storyboard.go:40` | storyboard_test.go | none | **UNWIRED** |
| `Traffic()` | func | `internal/tui/storyboard.go:51` | refresh_test.go | none | **UNWIRED** |
| `Empty()` | func | `internal/tui/storyboard.go:56` | refresh_test.go | none | **UNWIRED** |
| `Storyboard()` | func | `internal/tui/storyboard.go:101` | storyboard_test.go | none | **UNWIRED** |
| `Render()` | func | `internal/tui/storyboard.go:301` | errors_test.go, liveness_test.go | none | **UNWIRED** |

#### `internal/usage` — 11 unwired, 9 with tests

| Symbol | Kind | file:line | Tests covering it | Non-test caller? | Verdict |
|---|---|---|---|---|---|
| `MechanismExplicitBreakpoint` | const | `internal/usage/usage.go:35` | usage_test.go | none | **UNWIRED** |
| `MechanismImplicitPrefix` | const | `internal/usage/usage.go:39` | usage_test.go | none | **UNWIRED** |
| `MechanismRentedCache` | const | `internal/usage/usage.go:42` | none | none | **UNWIRED** |
| `ProviderAnthropic` | const | `internal/usage/usage.go:47` | usage_test.go | none | **UNWIRED** |
| `Record` | type | `internal/usage/usage.go:55` | usage_test.go | none | **UNWIRED** |
| `FromAnthropic()` | func | `internal/usage/usage.go:108` | usage_test.go | none | **UNWIRED** |
| `Record.ToAnthropic()` | method | `internal/usage/usage.go:131` | usage_test.go | none | **UNWIRED** |
| `Record.CachedShare()` | method | `internal/usage/usage.go:145` | none | none | **UNWIRED** |
| `InclusiveCounts` | type | `internal/usage/usage.go:154` | usage_test.go | none | **UNWIRED** |
| `FromInclusive()` | func | `internal/usage/usage.go:179` | usage_test.go | none | **UNWIRED** |
| `Record.Validate()` | method | `internal/usage/usage.go:204` | usage_test.go | none | **UNWIRED** |

---

## Annex: unexported dead functions in `cmd/replay`

Out of scope (the brief is exported symbols) but reported by the same
`deadcode` run, and one of them is the documented `saveQuota` case.

| Symbol | file:line | Consequence |
|---|---|---|
| `saveQuota` | `cmd/replay/quotastore.go:37` | The status-line quota reading is never persisted, so `loadQuota` at `cmd/replay/mcp.go:269` always finds nothing. Ranked #3 above. |
| `chooseTTL` | `cmd/replay/apply.go:119` | None. A one-line wrapper over `chooseTTLWithCoverage`, which is live and used at `apply.go:236`. |
| `tipLineFor` | `cmd/replay/tipline.go:72` | None. Wrapper over `tipLineArm`, which is live. |
| `valueCommands` | `cmd/replay/support.go:33` | Its own comment says it exists "so a command added later is covered by the test the day it exists" — but only `support_test.go:37` iterates it, so the support line's command list is not in fact derived from it in production. |

---

## Cheapest to wire

`internal/masking.LooksLikeHexSecret` (`internal/masking/entropy.go:177`).

It is a pure function of `(line, start, length)` with no dependencies, already
tested, and the call site is one condition inside the entropy pass that
`internal/masking/mask.go:145` already gates on `m.Entropy`. Wiring it costs one
branch and closes a demonstrated leak of `Bearer <hex>` credentials through the
redaction proxy.

The runner-up for value-per-line is `usage.Record.Validate`
(`internal/usage/usage.go:204`), but that one needs `internal/transcript` to
adopt `usage.Record` first, which is a real refactor rather than a wire-up.
