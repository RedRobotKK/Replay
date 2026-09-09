# Vacuous tests: which checks would still pass if the thing they test were broken

**Opened 2026-09-09.** The three `unwired-*` audits asked *what is built and
unreachable*. This one asks the adjacent question, and it has a different
answer: **what is checked, and unfalsifiable.**

A test that cannot fail is worse than no test. No test is an admitted gap. A
green test is a claim, and every reader downstream — a reviewer, a release
gate, the next agent — spends that claim as if it were evidence.

Nothing in this tree was modified to produce this document except this file.
Every mutation reported below was applied to a scratch copy under `/tmp` and
reverted; the commands and their output are quoted.

---

## 1. Counts

| | |
|---|---|
| **Vacuous or near-vacuous tests** | **20 findings** (§3) |
| — of which assert *nothing at all*, on any machine | **3** |
| — of which assert nothing on CI, and only bite locally | **5** |
| — of which pass alone and **fail in their own package**, through state one test writes into `$HOME` for another to read | **2** |
| — of which never execute in `go test ./...` at all | **6** (one of them is a 72-mutant catalogue) |
| **Refusal sites never executed by any test** | **144 of 239** (60%), measured by cross-package coverage (§5) |
| — of which are *policy* refusals, not I/O wrappers | **71** |
| **Escaping mutations verified by running them** | **5 of 5 attempted** (§6) |
| **Mutations attempted that were caught** (reported, because a negative result is a result) | **2** (§6.6) |

**The tree is red at `744b0de`, before any mutation, on a completely fresh
`$HOME`** — so it is red on CI too, not only here:

```text
$ rm -rf /tmp/h6 && mkdir -p /tmp/h6
$ HOME=/tmp/h6 XDG_CONFIG_HOME=/tmp/h6/.config go test ./cmd/replay/ -count=1
--- FAIL: TestNoOrphanedDocuments   docs/design/benefit-gap-analysis.md is linked from nowhere
--- FAIL: TestScreenSVGs            the advise screen no longer matches its committed image
--- FAIL: TestTC1_AConvertedScreenIsStillASCIIAndFits   non-ASCII '▸' (U+25B8) under a ja_JP locale
--- FAIL: TestTW1                   non-ASCII '▸' (U+25B8) shifts width by locale
```

Two of those four are the subject of §3.2 and are the most interesting thing in
this document.

---

## 2. Method, and how it can be wrong

Four instruments, because no one of them sees the whole shape.

1. **Direct instrumentation.** For any test whose assertions sit behind a
   filter or a range, a throwaway `zz_probe_test.go` was dropped into the
   package, run, and deleted, printing how many items actually reached the
   assertion. A count of zero is proof, not inference. This is what produced
   §3.1.
2. **Differential execution.** Every test that renders a screen was run twice:
   once with `REPLAY_TRANSCRIPTS` pointed at an empty directory (the CI
   condition) and once against this machine's 1,744 transcripts. Where the two
   runs take 0.07s and 42s, they are not running the same code.
3. **Cross-package coverage.** `go test ./... -coverpkg=./... -coverprofile`
   with a fresh `$HOME`, then every `return … errors.New(…)` / `return …
   fmt.Errorf(…)` line matched against the profile. A refusal on a line with
   count 0 has never fired under test. This is §5's number.
4. **Mutation.** For each headline claim, a one-line edit to production code in
   a scratch tree, then the suite. §6.

**Where this is wrong, stated plainly:**

- **Coverage counts execution, not assertion.** A refusal line with a non-zero
  count was *reached*; it does not follow that anything checked what it said.
  The 144 is therefore a **floor** on untested refusals, not a ceiling. §5.3
  gives a worked example of a refusal that is covered and still unchecked.
- **The literal-text instrument was tried first and discarded.** Grepping each
  refusal message against every `_test.go` reported 217 of 223 untested, which
  is wrong: `internal/proxy/uds.go` and `internal/feed/feed.go` are covered
  thoroughly by tests that assert behaviour rather than message text. That
  method is recorded here only so nobody repeats it.
- **Darwin, `go1.26.0`, one machine.** Coverage of platform-gated files
  (`term_linux.go`, `ownership_windows.go`, `isterm_other.go`) is not measured.
  A refusal that only exists on Windows is not in the 144.
- **"Near-vacuous" is a judgement.** Where a test is vacuous but the behaviour
  it names is caught by a sibling, that is said so, and the mutation output
  showing the sibling catching it is quoted. §6.6.
- **One untracked file** (`internal/advisor/track_test.go`) was present in the
  working tree and was included in every run, since that is what a developer
  would actually be running.

---

## 3. A — vacuous and near-vacuous tests

### 3.1 The worst one: a filter that matches nothing

`internal/tui/storyboard_test.go:38` — `TestStoryboard_TrafficRowsAlignWithTheirHeader`

Its own doc comment states the stakes:

> A row that is one space short shears the column for every row after it, and
> it is invisible in review. **This is the defect the previous storyboard
> shipped with, so it is the one the storyboard itself is checked for.**

It checks nothing. The assertion sits inside a filter:

```go
for _, sc := range Storyboard() {
    for i, line := range sc.Lines {
        if !looksLikeTraffic(line) {
            continue
        }
        if got := columnStarts(line); !equal(got, want) { t.Errorf(...) }
    }
}
```

`looksLikeTraffic` (`storyboard_test.go:79`) ends with
`return len(columnStarts(line)) == 5`. `columnStarts`
(`storyboard_test.go:93-103`) has a trailing clause that prepends index `2` to
a list that already begins with `2` for any indented line — and every line in
this table is indented by two spaces. So it returns **six** starts for a
five-column row, the filter demands five, and **no row in the storyboard has
ever reached the assertion.**

Measured, not argued. A probe file was added, run, removed:

```text
$ go test ./internal/tui/ -run TestProbeFilterMatches -count=1 -v
    zz_probe_test.go:26: lines=166  looksLikeTraffic=0  looksLikeHeaderBlock=6
    zz_probe_test.go:29: candidate "  15:06:44  anthropic  api.anthropic.com        messages          parsed   "
                         cols=[2 2 12 23 48 66]
    zz_probe_test.go:31: Header()[0]="  time      surface    endpoint                 wire              status   "
                         cols=[2 2 12 23 48 66]
```

166 lines examined, **0 matched**. Note the sibling filter,
`looksLikeHeaderBlock`, matched 6 — so
`TestStoryboard_TheHeaderBlockHasOneGeometryEverywhere`
(`storyboard_test.go:125`) is live and is **not** reported here. Two tests
written the same day, the same shape, and only one of them works.

The escaping mutation is §6.1. `Traffic()` also feeds the live `serve` screen
at `internal/tui/liveness.go:104`, so the unguarded surface is not only the
storyboard.

### 3.2 The headline: one test writes to the real `$HOME`, and two others read it back

This is the largest cluster, and it is the `corpus()` defect wearing a new hat.

`cmd/replay/tui_currency_test.go:24-49` documents the hazard precisely:

> Without it these tests read whatever corpus the machine happens to have. That
> is how the first version of this file passed here and failed on CI in three
> jobs […] A test that only exercises the interesting path on the author's
> machine is the same defect this project has spent the day finding, wearing a
> different hat.

`corpus()` sets `REPLAY_TRANSCRIPTS`. **It does not isolate `$HOME`.** The
repository already has the helper for that — `isolateHome`, at
`cmd/replay/bare_test.go:65`, which sets `HOME`, `USERPROFILE` and
`CLAUDE_CONFIG_DIR` — and none of `tui_currency_test.go`,
`tui_color_test.go`, `tui_width_test.go` or `screens_svg_test.go` calls it.

So the suite writes into the developer's real home directory. Verified with a
fresh, empty `$HOME`:

```text
$ HOME=/tmp/h3 XDG_CONFIG_HOME=/tmp/h3/.config go test ./... -count=1 -coverprofile=/tmp/cover.out
$ find /tmp/h3 -maxdepth 3 -not -path '*/go-build*' -not -path '*/Library/*'
/tmp/h3
/tmp/h3/.replay
/tmp/h3/.replay/policy.json
/tmp/h3/.replay/cost-index.json
/tmp/h3/.replay/advice.json
/tmp/h3/.replay/tip.json
```

**The writer, bisected.** Each candidate run alone against its own fresh home,
then the home inspected:

```text
WRITES advice.json: TestS1_EveryResultCarriesTheAsk          cmd/replay/support_test.go:35
WRITES advice.json: TestS2_MachineReadableOutputIsNotPolluted  cmd/replay/support_test.go:59
clean:              TestS4_TheAskNamesWhatWasDelivered
clean:              TestAdviseOutDashStillApplies
clean:              TestAdviseJSONStdoutIsPureJSON
```

`support_test.go` runs `replay advise` and does not call `isolateHome`. Its two
neighbours in `advise_out_test.go` do, and are clean — the helper works; it is
simply not used here.

**The readers.** `TestTC1` and `TestTW1` render the `advise` screen, which
reads `~/.replay/advice.json` back and prints `▸` (U+25B8) and `—` (U+2014)
into a grid both tests require to be ASCII. So each of them **passes alone and
fails inside its own package**, with no code change between the two runs:

```text
$ HOME=/tmp/h5 go test ./cmd/replay/ -run 'TestTW1$' -count=1
ok      github.com/RedRobotKK/Replay/cmd/replay 0.294s

$ HOME=/tmp/h6 go test ./cmd/replay/ -count=1      # same fresh home, whole package
--- FAIL: TestTW1
    tui_width_test.go:49: screen advise line 5: non-ASCII '▸' (U+25B8) shifts width by locale:
            ▸ 1  Bash inputs are 28% of prompt tokens
--- FAIL: TestTC1_AConvertedScreenIsStillASCIIAndFits
    tui_currency_test.go:66: screen advise line 20: non-ASCII '—' (U+2014) under a ja_JP locale:
            as of 13:06 on 9 Sep — `replay advise` to refresh
```

**The product defect underneath is real**: the `advise` screen emits U+25B8 and
U+2014 into an 80-cell grid, and `tui_width_test.go:19-25` explains exactly why
that is not cosmetic — both are East Asian width class Ambiguous, so the row
measures one width where it was written and another in the operator's own
ja_JP terminal.

**Two corrections to what a first pass of this audit concluded**, both found by
running rather than reasoning, and both recorded because getting this wrong is
the same error the document is about:

- This is **not** corpus-dependence. It reproduces on an empty machine. The
  first pass assumed the `corpus()` shape because the file next door documents
  it; the bisect above says the medium is `$HOME`, not `REPLAY_TRANSCRIPTS`.
- `TestScreenSVGs` (`cmd/replay/screens_svg_test.go:75`) is **also** red, and is
  *not* a member of this cluster: it pins both `$HOME` (`:100-111`) and the
  corpus (`:76`), and it fails deterministically, twice in a row on an
  untouched tree, because the committed `docs/screens/advise.svg` is stale
  against the screen's current wording. A plain missing regeneration, not a
  vacuity finding — listed here only so nobody counts it twice.

Also worth recording as a containment issue in its own right: `go test ./...`
writes four files into whatever `$HOME` the developer happens to be running
under. On this machine that is the real `~/.replay/`.

### 3.3 The colour family: five tests that only ever see the empty screen

`cmd/replay/tui_color_test.go` has six tests. **One** calls `corpus(t)`.

| Test | line | `corpus(t)`? |
|---|---|---|
| `TestCL1_ColourChangesNoCell` | 54 | **yes**, with a comment explaining exactly why |
| `TestCL2_OnlySGRAndOnlyFromThePalette` | 87 | no |
| `TestCL3_TheDefaultOnAPipeIsColourless` | 116 | no |
| `TestCL4_NoColorBeatsTheFlag` | 132 | no |
| `TestCL5_ColourIsActuallyEmitted` | 147 | no |
| `TestCL6_ColoursDoNotNest` | 169 | no |

The five that do not pin a corpus read whatever the machine has. On a CI runner
that is nothing, and each screen returns its `Unavailable` branch. The two runs
are not close:

```text
$ REPLAY_TRANSCRIPTS=/tmp/nocorpus go test ./cmd/replay/ -run 'TestCL[1-6]' -count=1 -v
--- PASS: TestCL1_ColourChangesNoCell (0.57s)
--- PASS: TestCL2_OnlySGRAndOnlyFromThePalette (0.08s)
--- PASS: TestCL3_TheDefaultOnAPipeIsColourless (0.07s)
--- PASS: TestCL4_NoColorBeatsTheFlag (0.07s)
--- PASS: TestCL5_ColourIsActuallyEmitted (0.07s)
--- PASS: TestCL6_ColoursDoNotNest (0.07s)
```

against, on this machine:

```text
--- PASS: TestCL2_OnlySGRAndOnlyFromThePalette (35.00s)
--- PASS: TestCL3_TheDefaultOnAPipeIsColourless (32.35s)
--- PASS: TestCL4_NoColorBeatsTheFlag (37.53s)
--- PASS: TestCL5_ColourIsActuallyEmitted (42.43s)
--- PASS: TestCL6_ColoursDoNotNest (67.05s)
```

0.07s versus 35–67s is two different programs. What CI actually checks, counted
by SGR sequences emitted per screen with `-color always`:

| screen | no corpus | real corpus |
|---|---|---|
| cost | 2, codes `0,2` | **30**, codes `0,1,2,31,33,36` |
| context | 4, codes `0,33,36` | **28**, codes `0,1,2` |
| model | 4 | **18** |
| safe | 4 | **12**, codes `0,1,2,32` |
| guards | 4 | 2 |
| share | 2 | 4 |

On CI the `cost` screen emits two dim sequences. **The bold total, the red, the
yellow and the cyan are never rendered** — including `Alarm` on the avoidable
figure, which `internal/tui/measured.go:308` calls out as *"the only number
here that is money already spent twice"*.

`TestCL5` is the most exposed of the five, because it exists specifically to
stop the other four passing against a layer that paints nothing:

> Without this the four tests above are all satisfied by a colour layer that
> does nothing, which is the shape of check this project has now found seven
> times.

On CI it is satisfied by two dim sequences on an "I have nothing to show you"
banner. §6.2 has the mutation.

`TestCL6` is worth naming separately. Its comment cites the cursor row
"wrapping a row that already coloured its own break count by severity" — that
row is in the `Measured` branch of the `context` screen, which on CI emits
codes `0,33,36` from the banner and never renders at all.

### 3.4 An assertion satisfied by the URL, not the message

`cmd/replay/x402_test.go:721` — `TestX402_RedirectToCleartextIsRefused`

```go
if !strings.Contains(err.Error(), "http") {
    t.Errorf("the error should name the cleartext hop, got: %v", err)
}
```

`http.Client` wraps a `CheckRedirect` error in a `*url.Error` whose `Error()`
carries the original `https://…` URL. That URL contains the substring `http`.
The assertion therefore holds for **any** error at all. Verified in §6.3.

The other two assertions in the same test — that the fetch failed and that
nothing was installed — are live and load-bearing. Only the third is theatre.

### 3.5 A loop guard that cannot be satisfied

`internal/ledger/evidence_test.go:72` — `TestL2_NoLowerBoundIsInvented`, the
loop at `:80`

```go
for _, e := range ev {
    if !e.Wrote && e.Marked {
        t.Errorf("a non-writing record became lower-bound evidence: %+v", e)
    }
}
```

`EvidenceFrom` drops non-writing records before returning — which is `L3`'s
job, one test below. So `!e.Wrote` is unsatisfiable by construction and the
first assertion is a tautology. Probed:

```text
$ go test ./internal/ledger/ -run TestProbeL2 -count=1 -v
    zz_probe_test.go:14: len(ev)=1
    zz_probe_test.go:16: ev[0]={Model:claude-opus-5 PrefixTokens:512 Wrote:true Marked:true Session:s2 Machine:machine-a}
    zz_probe_test.go:21: Observed==nil: false
```

The second assertion (`o.LowerBound == nil`) **is** live — `Observed` is
non-nil. Half the test works. Reported at low severity, and reported at all
only because the fixture was checked rather than assumed.

### 3.6 Six tests that never run in `go test ./...`

| # | Test | file:line | Why it never runs |
|---|---|---|---|
| 1 | `TestFrozenMutantsStillDie` + `TestKillMatrix` | `internal/mutation/mutation_test.go:261,326` | `//go:build mutation`. `.github/workflows/ci.yml:42,95` and `Makefile:26` all run `go test -race -count=1 ./...` with **no `-tags mutation`**. **72 frozen mutants, none of them ever re-run.** |
| 2 | `TestOllamaConservationHoldsOnTheLocalLogs` | `internal/transcript/ollama_test.go:72` | skips without `~/.ollama/logs/server*.log` |
| 3 | `TestCodexConservationHoldsOnTheLocalCorpus` | `internal/transcript/codex_corpus_test.go:19` | skips without `~/.codex`; its own comment says "which is most machines and every CI runner" |
| 4 | `TestLive_CachedTokensSurviveTheInclusiveConversion` | `internal/ledger/live_cached_provider_test.go:41` | needs `REPLAY_LIVE_BASE_URL` + key. Self-labelled; the least dishonest of the six |
| 5 | the `ledger` sub-tests of the two reconcile tests | `internal/analysis/reconcile_test.go:69-91,183-189` | skip without `~/.replay/ledger`. The sibling `fixture` sub-tests are live, so the parent is not vacuous — only this arm |
| 6 | `TestRescore_SessionCostDoesNotGrowWorseThanQuadratic` | `internal/proxy/rescore_bench_test.go:184` | `t.Skip("timing test")` under `-short` — and it is **flaky** at full length: it failed once at 33.16x against a 32x bound and passed on immediate re-run, same tree, same binary |

**#1 deserves emphasis.** `internal/mutation` is the best-designed thing in this
tree. It refuses to run on a red baseline; it distinguishes killed from
survived from stillborn so a compiler refusal is never scored as a kill; it
fails rather than skips when a mutant's anchor has moved. Its doc comment is
right about why it exists — *"the mutant was the evidence and it was thrown
away."* And the catalogue is **not rotten**: every anchor and every named
killer test still resolves.

```text
$ python3 …   # anchors and killer-test names checked against the tree
mutants: 72
MISSING FILES: 0
MISSING ANCHORS: 0
MISSING KILLER TESTS: 0
```

It simply never runs. The instrument built to prove the tests can fail is
itself the test in this repository that has never been asked to.

Its coverage is also lopsided. 72 mutants, by package:

```text
19  internal/proxy      18  cmd/replay        10  internal/analysis
 6  internal/quota       4  internal/observation  3  internal/masking
 2  internal/regression  1  internal/consent   + 8 in docs/config/install.sh
```

Fifteen packages have **no mutant at all**: `advisor`, `cachemodel`, `card`,
`facts`, `feed`, `learn`, `ledger`, `money`, `otlp`, `policy`, `probe`,
`transcript`, `tui`, `usage`, `version`. Every finding in §3.1, §3.3 and §3.5
above is in one of them. That is not a coincidence — mutation testing is
exactly the instrument that would have found them, and it was pointed
elsewhere.

### 3.7 Near-vacuous: guarded only by a neighbour

**`internal/advisor/advisor_test.go:264` — `TestUnusedBuiltinsAreNotGivenAServer`.**
Every assertion is inside `for _, s := range Suggest(obs) { if s.Kind != KindUnusedTools { continue } … }`
with no count check. Its immediate sibling at `:193` does the same walk and
then `t.Fatalf`s on `len(servers) < 2`. One was written with the guard and one
without, ten lines apart. Suppress built-in suggestions and this test goes
green having asserted nothing — but two other tests catch it, so the behaviour
is not actually exposed. §6.6.

**`internal/money/money_test.go:177` — `TestMN10_TheDollarsAlwaysSurvive`**
ranges the production `rates` map directly with no `len(rates) == 0` guard.
The table is a large compiled ECB set, so this is latent rather than live.

### 3.8 Latent conditional skips

`internal/analysis/route_test.go:161,195` and
`internal/cachemodel/measure_test.go:118,191` skip on a property of the
*compiled pricing table* (`top.Known`, `DocumentedMinPrefix(...) == 0`).
Neither fires today — verified by running them — but a pricing-rules update
that drops `claude-opus-5` would silently disable four tests with a green
result. One line each fixes it: `t.Fatal` instead of `t.Skip`, because the
compiled table not pricing the model the test is written around *is* the
failure.

---

## 4. B — untested paths that matter, ranked by consequence

Ranked by what a user loses, not by count.

### B1. The `advise` screen's non-ASCII output

`internal/tui` renders `▸` (U+25B8) and `—` (U+2014) into the advise screen.
`TestTW1` and `TestTC1` both forbid it, and both are red today (§3.2). This is
the one item in this section that is not an untested path but an *untested-for-
years path that has just become tested by accident* — nothing deliberately
arranges for `~/.replay/advice.json` to exist when the screens are rendered; a
neighbour writes it. **The input**: any `$HOME` with `.replay/advice.json`
holding at least one suggestion. **What is missing**: a test that constructs
that state on purpose, rather than inheriting it.

### B2. The colour of every `Measured` screen

Ten screens, six of which paint differently once there is data (§3.3). **The
input**: `REPLAY_TRANSCRIPTS` pointed at a fixture — the repo already ships one
at `internal/transcript/testdata/session-redacted.jsonl`, and `corpus()`
already knows how to use it. **Why nothing does today**: five of six colour
tests do not call it. This is one line each.

### B3. `cmd/replay/tui.go:76` — the screen-name validation

Never executed by any test (coverage count 0). This matters beyond itself:
[unwired-3](unwired-3-branches-and-docs.md) §2 proves the TUI's `default` arm is
dead, and **gate 1 of that three-gate proof is this validation**. The proof that
`tui.Outcome()` is unreachable rests on a check nothing exercises. If it
regressed, the audit's conclusion would silently become false.

**The input**: `replay tui -once -screen nosuchscreen`. One test.

### B4. `cmd/replay/priceclient.go` — no test file exists

Three refusal sites (`:37`, `:41`, `:45`), zero coverage, and no
`priceclient_test.go` in the tree. This is the path that fetches the price
database — the source of every dollar figure the tool prints. **The input**: an
`httptest` server returning 500, then a truncated body. The repository already
has this pattern in `x402_test.go`.

### B5. `internal/proxy/server.go:172,175` — `New()`'s nil guards

Never executed (§6.4). These are the proxy's only defence against being
constructed without an upstream or without a ledger store. **The input**:
`proxy.New(proxy.Config{})`. Two lines.

### B6. The MCP tool surface — `cmd/replay/mcp.go:241,281,297`

`replay_price_check needs a model`, `no tool %q`,
`replay_mcp_overhead needs a model and a byte size`. All three uncovered. This
is the one interface an LLM agent drives directly and therefore the one where
malformed input is *most* likely, not least. **The input**: an MCP
`tools/call` with an empty argument object. `mcp_test.go` MC1–MC10 exercise
only success paths.

### B7. `internal/usage/usage.go:210` — the TTL-split invariant

`TTL split %d+%d exceeds the total write %d`. Uncovered, while its sibling
invariant at `:206` is tested (`TestValidateCatchesADoubleCountedCache`,
`usage_test.go:178`). Compounding: UNWIRED-LOG #8 records that this whole
package has **zero importers** while the ~1.94x double-count it guards sits
live in `transcript/codex.go:151`. So the guard is unwired *and* half of it is
unchecked.

### B8. `internal/ledger/store.go` — 17 uncovered refusal sites

The append-only cost ledger is the system of record every later number is
derived from, and every I/O failure path in it — including `:178`,
`no records in %s` — has never executed under test.

---

## 5. C — refusals never exercised

> A refusal that has never fired is a claim, not a behaviour.

### 5.1 The number

```text
$ HOME=/tmp/h4 go test ./... -count=1 -coverpkg=./... -coverprofile=/tmp/cover-all.out
$ python3 …   # every `return … errors.New/fmt.Errorf` line matched against the profile
CROSS-PACKAGE (-coverpkg=./...)
refusal return-sites: 239
never executed by ANY test in the repo: 144
of which policy-shaped (not an I/O wrapper): 71
```

**144 of 239, 60%.** Of those, 71 are the tool declining to answer — the
behaviour this project treats as its distinguishing feature — rather than a
wrapped `os` error.

By file, the untested refusals concentrate:

```text
17  internal/ledger/store.go       11  internal/masking/vault.go
10  cmd/replay/rules.go            10  internal/transcript/claudecode.go
 7  cmd/replay/advise.go            6  cmd/replay/serve.go
 6  internal/proxy/uds.go           6  internal/feed/feed.go
 6  internal/probe/run.go           5  cmd/replay/probe.go
```

### 5.2 The refusals that guard money or correctness and have never fired

| Refusal | file:line | What it protects |
|---|---|---|
| `refusing to fetch rules over plain http` | `cmd/replay/rules.go:214` | the pricing rules the whole cost model reads, over cleartext |
| `too many redirects` | `cmd/replay/rules.go:236` | the same fetch, against a redirect loop |
| `no cache writes … nothing measured to publish` | `cmd/replay/rules.go:379` | publishing a rules bundle with no evidence behind it |
| `no sessions with usable token fits were found` | `cmd/replay/route.go:60` | a model-migration recommendation built on nothing |
| `no session could be analyzed (%d failures)` | `cmd/replay/corpus.go:126` | the primary cost report's empty-corpus refusal |
| `no transcript could be analyzed` | `cmd/replay/main.go:317` | the same, for bare `replay` |
| `NOT MEASURED: no ledger found under %s` | `cmd/replay/budget.go:185` | a budget figure with no ledger |
| `NOT MEASURED: … no usable …` | `cmd/replay/budget.go:196` | the same |
| `not confirmed; nothing was sent` | `cmd/replay/probe.go:131` | **the guard in front of real, billable API traffic** |
| `a model is required` (×2) | `cmd/replay/probe.go:57,62` | the same command's argument gate |
| `upstream URL is required` / `ledger store is required` | `internal/proxy/server.go:172,175` | the proxy's construction-time safety net |
| `TTL split … exceeds the total write` | `internal/usage/usage.go:210` | double-counted cache-write billing |
| `the feed at %s is larger than %d bytes` | `internal/feed/feed.go:201` | an unbounded read from a remote feed |
| `the feed carries no version, so a rollback could not be detected` | `internal/feed/feed.go:131` | a silent downgrade of the rules |
| `no records in %s` | `internal/ledger/store.go:178` | an empty ledger read as a real one |
| `replay_price_check needs a model` etc. | `cmd/replay/mcp.go:241,281,297` | the agent-facing tool surface |
| `no screen called %q` | `cmd/replay/tui.go:76` | see B3 — an unwired-3 proof depends on it |
| all 11 vault failure paths | `internal/masking/vault.go:63…203` | the secret vault behind the masking proxy |

### 5.3 Covered is not checked — a worked example

`cmd/replay/rules.go:233` (`refusing a redirect to plain http`) **is** covered,
by `TestX402_RedirectToCleartextIsRefused`. It is still unchecked: the test's
assertion that the error "names the cleartext hop" is satisfied by the substring
`http` inside the URL, so the message could say anything. §6.3 proves it by
replacing the message with `"nope"`.

This is why the 144 is a floor.

### 5.4 Refusals that ARE well covered, so nobody re-audits them

Recorded so this section is not read as a blanket claim. All exercised, all
verified by running:

- **The five proxy refusal kinds** — `refusalCircuitOpen`, `refusalSpendCap`,
  `refusalLoop`, `refusalErrorBudget`, `refusalPreFlight` — every one has a
  named test (`internal/proxy/refusal_test.go`, `errorbudget_test.go`,
  `preflight_test.go`). Note that `refusalPreFlight` is tested and, per
  UNWIRED-LOG #7, cannot fire in production. Tested and unreachable are
  independent properties, and this is the pair that shows it.
- **`internal/proxy/uds.go`** — U2 through U10, nine named tests, and six
  frozen mutants (M16–M21).
- **`internal/feed/feed.go`** — FD1–FD8 cover every integrity and rollback
  guard except the size cap.
- **`internal/masking/scope.go`'s `Reason` enum** — all seven values,
  including `ReasonTooLarge`, which an earlier pass of this audit wrongly
  called untested. Corrected by running the mutation; see §6.6.
- **`cmd/replay/apply.go`'s six `plan.Reason` branches** — the auto-apply
  refusals that decide whether Replay edits a user's config. All six tested.
- **`internal/advisor/guards.go`'s three-way `UpperFence` refusal.**

---

## 6. D — the mutation that escapes

Five one-line changes to production code, each applied in a scratch copy under
`/tmp`, each run, each reverted. **The repository was not modified.**

### 6.1 The traffic table shears and the test written for it stays green

**Test:** `internal/tui/storyboard_test.go:38`, `TestStoryboard_TrafficRowsAlignWithTheirHeader`

**Mutation** — `internal/tui/storyboard.go:52`, total width preserved so the
grid-height guards do not notice:

```diff
 func Traffic(t, surface, endpoint, wire, status string) string {
-    return Row(trafficCols, t, surface, endpoint, wire, status)
+    return Row([]Column{{"time", 9}, {"surface", 9}, {"endpoint", 22}, {"wire", 16}, {"status", 9}}, t, surface, endpoint, wire, status)
 }
```

Every traffic row is now one cell right of its header:

```text
H|  time      surface    endpoint                 wire              status   |
R|  12:04:31   claude     /v1/messages            anthropic         200      |
             ^ one cell right, and so is everything after it
```

**Result — the entire package is green:**

```text
$ go test ./internal/tui/ -count=1
ok      github.com/RedRobotKK/Replay/internal/tui       0.495s
```

A first attempt that widened the row without compensating *was* caught — by
`refresh_test.go:127`, on grid height, not on alignment:

```text
--- FAIL: … the traffic window changed height when a request arrived …
    an occupied row is 76 cells and an empty one is 75.
```

So the only thing standing between this defect and a release is a test about
something else, and it is defeated by keeping the width constant — which is
what a real shear does.

### 6.2 The avoidable-money figure loses its alarm colour; ten colour and width tests stay green

**Tests:** all of `TestCL1`–`TestCL6`, `TestTC2`–`TestTC4`, `TestTW1`–`TestTW4`.

**Mutation** — `internal/tui/measured.go:309`:

```diff
-        paint(Alarm, money(m.AvoidableUSD))+" avoidable. List price, not your bill.",
+        money(m.AvoidableUSD)+" avoidable. List price, not your bill.",
```

`internal/tui/measured.go:308` says what this costs: *"Alarm on the avoidable
figure because it is the only number here that is money already spent twice."*

**Result, run against this machine's real 1,744-transcript corpus** — the
harder of the two conditions, since on CI the branch never renders at all:

```text
$ go test ./cmd/replay/ -run 'TestCL…|TestTC…|TestTW…' -count=1 -timeout 900s -v
--- PASS: TestCL1_ColourChangesNoCell (0.61s)
--- PASS: TestCL2_OnlySGRAndOnlyFromThePalette (35.00s)
--- PASS: TestCL3_TheDefaultOnAPipeIsColourless (32.35s)
--- PASS: TestCL4_NoColorBeatsTheFlag (37.53s)
--- PASS: TestCL5_ColourIsActuallyEmitted (42.43s)
--- PASS: TestCL6_ColoursDoNotNest (67.05s)
--- FAIL: TestTC1_AConvertedScreenIsStillASCIIAndFits (0.31s)   ← red on the pristine tree too, §3.2
--- PASS: TestTC2_TheLocalFigureIsShownAndNamed (0.08s)
--- PASS: TestTC3_ADollarReaderSeesNoChange (0.10s)
--- PASS: TestTC4_TheRateIsStatedOnScreen (0.09s)
--- PASS: TestTW1 (82.56s)
--- PASS: TestTW2 (85.04s)
--- PASS: TestTW3 (77.37s)
--- PASS: TestTW4 (0.00s)
```

`TestCL5` — *"colour actually appears when it is asked for"* — passes, because
other elements on the screen are still painted. It asserts that *some* colour
exists, not that the figure that carries the screen has any. On CI it would
pass in 0.07s without rendering the branch at all.

### 6.3 The cleartext refusal stops naming cleartext

**Test:** `cmd/replay/x402_test.go:721`, `TestX402_RedirectToCleartextIsRefused`

**Mutation** — `cmd/replay/rules.go:233`:

```diff
                 if req.URL.Scheme != "https" {
-                    return fmt.Errorf("refusing a redirect to plain http: %s", req.URL.Redacted())
+                    return errors.New("nope")
```

**Result:**

```text
$ go test ./cmd/replay/ -run 'TestX402_RedirectToCleartextIsRefused' -count=1 -v
--- PASS: TestX402_RedirectToCleartextIsRefused (0.01s)
```

The assertion `strings.Contains(err.Error(), "http")` is satisfied by the
`https://…` URL that `*url.Error` prepends, not by the message. The fix is one
character wider: assert `"plain http"` or `"cleartext"`.

### 6.4 The proxy accepts a nil ledger store

**Refusal:** `internal/proxy/server.go:175` — never executed (§5).

**Mutation:**

```diff
-    if cfg.Store == nil {
+    if false {
         return nil, errors.New("ledger store is required")
     }
```

**Result:**

```text
$ go test ./internal/proxy/ -count=1 -timeout 600s
ok      github.com/RedRobotKK/Replay/internal/proxy     2.671s
```

(The pristine tree in the same scratch run: `ok … 2.092s`. An earlier run
failed on `TestRescore_SessionCostDoesNotGrowWorseThanQuadratic` at 33.16x
against a 32x bound and passed on immediate re-run — that flake is §3.6 #6, not
this mutation.)

Nothing in the repository constructs a `proxy.Config` without a `Store`, so the
guard has never been asked a question.

### 6.5 A capability is unwired exactly the way `preflight.go` was, and the guard built to catch that stays green

**Test:** `internal/regression/unwired_packages_test.go:38`,
`TestNoNewlyUnwiredPackages` — written today, in response to the unwired audit.

Start with the fact that needs no mutation. The guard's own doc comment names
its motivating case:

> `internal/proxy/preflight.go` has eight tests and cannot fire in production,
> because `Config.PreFlight` is never assigned.

`cmd/replay/serve.go:136-155` still does not assign `PreFlight`. And:

```text
$ go test ./internal/regression/ -run 'TestNoNewlyUnwiredPackages' -count=1 -v
--- PASS: TestNoNewlyUnwiredPackages (0.02s)
```

**The guard is green on the exact defect it was written for.** It resolves
imports at *package* granularity (`reachable()`, `unwired_packages_test.go:109-172`),
so a dead file inside a live package is invisible to it — and `preflight.go`
lives in `internal/proxy`, which the binary certainly reaches.

**Mutation**, to show a *new* instance is equally invisible —
`cmd/replay/serve.go:150`, one line deleted:

```diff
         Masker:        masker,
-        Rehydrator:    rehydrator,
         Trial:         proxy.TrialSettings{…},
```

Every masked secret now stays a placeholder in responses to the user's tools.
`internal/masking` is still imported, so it is still "wired".

**Result:**

```text
$ go build ./... && go test ./internal/masking/ ./internal/regression/ -count=1
ok      github.com/RedRobotKK/Replay/internal/masking      0.625s
ok      github.com/RedRobotKK/Replay/internal/regression   2.338s
```

The masking package's own tests pass because they construct the rehydrator
directly — which is precisely the shape UNWIRED-LOG #7 describes for
`preflight.go`, reproduced on demand.

This is not an argument to delete the guard. It catches the case it was built
for — a whole package leaving the closure — and it has the anti-vacuity check
(`len(all) < 10`, `unwired_packages_test.go:54`) that most of the tests in §3 lack. It is an argument that
**package granularity is the wrong resolution for this defect**, and that the
register it belongs to is symbol-level.

### 6.6 Two mutations that were caught — reported because a negative result is a result

**Built-in unused-tool suggestions suppressed** (`internal/advisor/advisor.go:281`):

```diff
-        if b.bytes == 0 {
+        if b.bytes == 0 || target == builtinTools {
             continue
```

```text
--- FAIL: TestUnusedToolsAndHotFilesAcrossSessions (0.00s)
--- FAIL: TestSuggestionsAreTrackedToClosure (0.00s)
--- PASS: TestUnusedToolsAreAttributedToTheirServer (0.00s)
--- PASS: TestUnusedBuiltinsAreNotGivenAServer (0.00s)   ← vacuous, as §3.7 says
```

`TestUnusedBuiltinsAreNotGivenAServer` is vacuous, and the behaviour is
nonetheless guarded — by two tests that were not written for it. Worth fixing
(one `len()` check), not worth alarm.

**`ReasonTooLarge` relabelled** (`internal/masking/scope.go:26`):

```diff
-    ReasonTooLarge       = "too-large"
+    ReasonTooLarge       = "scope"
```

```text
--- FAIL: TestRehydrateStreamGivesUpOnOversizedInput (0.08s)
    rehydrate_test.go:338: report: {Restored:map[] Denied:map[edit:Edit/scope:1]}
```

Caught. An earlier pass of this audit had listed `ReasonTooLarge` as untested
on the strength of a name grep; the mutation refuted it. That is why §5 is
built on coverage and mutation rather than on grep, and why §2 records the
discarded instrument.

---

## 7. What to write first

**One test, and it is not in any of the packages above:**

```text
internal/regression/harness_test.go
    TestNoTestWritesToTheRealHome
```

Snapshot `$HOME` before and after a `go test ./cmd/replay/...` sub-invocation
and fail on any new path — or, cheaper and with no sub-process, parse
`cmd/replay/*_test.go` and fail any test that calls `run(...)` without first
calling `isolateHome`. **It is red today**, and it is the single mechanism
behind the largest cluster in this document: `TestS1` and `TestS2` write
`~/.replay/advice.json` and `TestTC1` and `TestTW1` read it back, which is why
each of those two passes alone and fails inside its own package.

It is worth writing before the more obvious candidates because of what it costs
to not have it. A suite whose result depends on the order its own tests ran in
is a suite whose green cannot be reasoned about at all — and every other
finding in this document was harder to establish than it needed to be for
exactly that reason. It is also the only finding here that reaches outside the
repository: the tests write into the developer's actual home directory.

Then, in order:

1. **`-tags mutation` in CI** (`.github/workflows/ci.yml`). Not a test — a
   config line. Seventy-two frozen mutants exist, their anchors all still
   resolve, and none of them has been re-run since the day it was written. This
   is the cheapest true statement about the suite available anywhere in the
   repository, and it costs one line to start telling it. Give it its own job
   with a longer timeout; it compiles the tree once per mutant.
2. **`isolateHome(t, t.TempDir())` in `support_test.go`**, and `corpus(t)` in
   the five colour tests, and `isolateHome` folded into `corpus()` itself.
   Seven lines. Turns §3.2 and §3.3 off at the source and makes the tree green.
   Regenerate `docs/screens/advise.svg` in the same change, and fix the
   `▸`/`—` the regenerated image will then contain — that is the real defect,
   and updating the image without fixing the screen would be the wrong half.
3. **A matched-count assertion in every filter-based test.** `TestStoryboard_TrafficRowsAlignWithTheirHeader`
   first, with `if matched == 0 { t.Fatal(…) }` — and fix `columnStarts`'
   duplicate index while there, since that is the actual bug.
4. **A mutant for each of §6.1–§6.5**, added to `testdata/mutants.json`, so
   these five findings become five permanent questions rather than one
   document. They meet the catalogue's own admission criteria: viable,
   observable, historical, and each names the specific check that must die.
5. **The fifteen packages with no mutant** — start with `internal/tui`,
   `internal/ledger` and `internal/advisor`, in that order, because that is
   where §3's findings clustered.

---

## 8. Loose ends recorded, not fixed

- `docs/design/benefit-gap-analysis.md` is already an orphan and
  `TestNoOrphanedDocuments` (`cmd/replay/docs_drift_test.go:217`) is red on it.
  **This file will be a second orphan until it is linked from
  [docs/design/README.md](README.md)**, which was out of scope for a read-only
  audit.
- **The working tree moved under this audit.** Every measurement above was taken
  against `744b0de` plus one untracked file. While it was being written,
  something else began editing the tree: `docs/screens/advise.svg` at 13:11,
  then `cmd/replay/tui.go`, `internal/tui/advise.go` and
  `internal/tui/advise_test.go` (+44 lines of test). That looks like the ▸/—
  defect of §3.2 being fixed, which is the right outcome — but it means the line
  numbers here should be re-resolved by symbol name rather than trusted, and
  nothing was reverted. No command run for this audit passed `-update` or
  touched a tracked file; all three scratch copies under `/tmp` still carry the
  `HEAD` bytes of `advise.svg`.
- `internal/advisor/track_test.go` is untracked in the working tree. It was
  included in every run above.
- `TestRescore_SessionCostDoesNotGrowWorseThanQuadratic` is flaky at a 32x
  bound (observed 33.16x, then green on re-run). A timing assertion that fails
  one run in some unknown number is a check nobody will believe on the day it
  matters.

---

[Design](README.md) · [Unwired log](UNWIRED-LOG.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
