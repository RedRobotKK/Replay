# Benefit gap analysis

**Date:** 2026-09-09. **Tree:** `1b82dc7` (`advisor: stop inferring that advice was applied from
the number it measures`). **Scope:** read-only audit. No production code was changed.

Everything in this repository tests whether the code does what it says. This file maps what is not
tested: whether the tool helps. It is written to be hostile to itself, and every number in it is
either a `file:line` or a command whose real output is pasted.

A note on the tree. `1b82dc7` landed **during** this analysis and fixes the circularity described in
§1.2 — independently, and its commit message reaches the same diagnosis this file does. The finding
that prompted the audit was therefore already half-known. §1.3 shows what the fix did not fix, and
that the repaired path is currently unreachable and red.

---

## 0. Reproduction

`~/.replay/advice.json`, generated `2026-09-09T19:29:20.913543Z`, 1246 sessions / 1738 transcripts,
140 suggestions.

```text
$ python3 -c "..." # status counts
'advice only'        117
'not verified'       20
'applied'              1
'pending'              1
'verified'             1

# kind counts
'hot-file'           116
'large-results'       19
'tool-inputs'          3
'first-turn-content'   1
'cache-breaks'         1
```

Twenty-one carry a `realized_share`. Sorted by realized/predicted:

```text
target                           sess  share%  pred%  real%  ratio   2s/N
mcp__claude-in-chrome__javascri     1   12.24   6.12   0.01  0.002  0.002
Edit                                1   10.07   5.03   0.01  0.002  0.002
mcp__a42b3efe-...                   1   22.69  11.35   0.02  0.002  0.002
StructuredOutput                    1   11.74   5.87   0.01  0.002  0.002
mcp__claude_ai_Upwork__upwork__     2   41.36  20.68   0.07  0.003  0.003
Write                               3   11.36   5.68   0.03  0.005  0.005
Artifact                            4   16.32   8.16   0.05  0.006  0.006
mcp__claude-in-chrome__navigate     7   12.18   6.09   0.07  0.011  0.011
Write                               7   14.96   7.48   0.08  0.011  0.011
Bash                                8   17.90   8.95   0.12  0.013  0.013
mcp__claude-in-chrome__read_pag    12   23.63  11.81   0.23  0.019  0.019
mcp__claude-in-chrome__get_page    17   12.39   6.19   0.17  0.027  0.027
mcp__claude-in-chrome__browser_    27   30.69  15.35   0.67  0.043  0.043
WebSearch                          41   20.92  10.46   0.69  0.066  0.066
mcp__claude_ai_Gmail__get_threa    48   25.10  12.55   0.97  0.077  0.077
mcp__claude-in-chrome__computer    57   19.07   9.53   0.87  0.092  0.091
mcp__claude-in-chrome__javascri    64   19.39   9.69   1.00  0.103  0.103
Read                              107   30.30  15.15   2.61  0.172  0.172
WebFetch                          121   23.87  11.93   2.32  0.195  0.194
ToolSearch                        132   11.81   5.91   1.25  0.212  0.212
mcp__claude_ai_Gmail__search_th   398   26.53  13.27   8.49  0.640  0.639
```

The finding reproduces. It is also, immediately, not what it looks like — see the last two columns.

---

## 1. The verifier

### 1.1 `realized_share` is prevalence, not a saving

The two rightmost columns are `realized/predicted` and `2 × sessions / 1246`. They agree to three
decimal places on all 21 rows. Tested directly:

```text
model: realized_share == sessions * share / N
max abs error over all 21 rows: 0.000136
```

The mechanism is a denominator mismatch, and it is two lines apart in one function.

- `internal/advisor/advisor.go:333` — `a.shares = append(a.shares, ob.targets[k].share)` runs inside
  a loop over **every** observation, so `len(shares) == N`, the whole corpus, with a zero wherever
  the target did not appear. The field comment says so plainly:
  `internal/advisor/advisor.go:299` — `shares []float64 // per session in time order, zero when absent`.
- `internal/advisor/advisor.go:343-346` — `s.Share` sums `a.evidence` and divides by
  `len(a.evidence)`: the mean **conditional on the target appearing**.
- `internal/advisor/advisor.go:407-412` — `before, after := mean(earlier), mean(recent)` over the
  padded slice: the mean **unconditional across all sessions**.

`PredictedShare` derives from the conditional mean (`advisor.go:358`, `trimShare * s.Share`).
`RealizedShare` derives from the unconditional one. They are not the same quantity and are never on
the same scale. Since `after` was zero for all 21 rows (the target simply did not recur in the last
two sessions), the arithmetic collapses to

```text
realized = (sessions × share) / N
realized / predicted = 2 × sessions / N
```

so `Verified` fires precisely when `sessions ≥ N/4`. Gmail's `search_threads` appears in 398 of 1246
sessions; 398 ≥ 311.5, so it verified. Nothing else in the corpus clears the bar. **The single
"verified" suggestion was verified for being frequent.** No test would have caught this: `track()`
is exercised only by six hand-written `[]float64` literals in `internal/advisor/track_test.go:34-110`,
none of which contains a zero for an absent session.

**This defect survives `1b82dc7` untouched.** Lines 333, 346 and 407 are unchanged.

### 1.2 The circularity (now fixed)

Before `1b82dc7`, "applied" was inferred from `after > before*(1-appliedDrop)` and the size of that
same drop then decided `Verified`. The commit's own comment
(`internal/advisor/advisor.go:381-400`) states the problem correctly and concludes: *"It is not
evidence the advice fails. It is evidence the verifier was measuring corpus drift and could not tell
which way it was being fooled."* That is right, and §1.1 says why it was worse than drift: it was
prevalence.

### 1.3 What the fix broke

`track()` now gates on a caller-supplied `applied` flag (`internal/advisor/advisor.go:401`). The
flag is declared (`advisor.go:295`), documented as *"set by the caller from the reader's own
decision"* (`advisor.go:293`), and read (`advisor.go:366`).

**It is never written.** `Suggest(obs []Observation)` (`advisor.go:308`) has no parameter that could
carry it, and both production callers pass only observations:

```text
cmd/replay/advise.go:102   suggestions := advisor.Suggest(obs)
cmd/replay/tui.go:563      return adviceRows(advisor.Suggest(obs)), sessions
```

So `applied` is always false, `track()` always returns `Pending`, and `Verified` / `NotVerified` are
unreachable through the public API. The suite says so:

```text
$ go test ./internal/advisor/
--- FAIL: TestSuggestionsAreTrackedToClosure (0.00s)
    advisor_test.go:159: a large drop must verify: {... Status:pending RealizedShare:0 ...}
FAIL    github.com/RedRobotKK/Replay/internal/advisor    0.324s
```

This is the current tree, red, at the time of writing.

### 1.4 The reader's decision does not survive

`cmd/replay/triage.go:23` `markAdvice` writes a status into `advice.json` through a temp-and-rename,
with a comment about exactly this failure mode: *"A silent success is how somebody comes to trust a
screen that is recording nothing."*

`cmd/replay/advise.go:160` then serialises `adviceFile{... Suggestions: suggestions}` — the fresh
output of `Suggest()` — and overwrites the file. There is no read-back and no merge; grep for the
filename constant finds only the writer and two readers:

```text
cmd/replay/advise.go:155   path = filepath.Join(home, ".replay", adviceFileName)
cmd/replay/triage.go:24    path := filepath.Join(tipStateDir(), adviceFileName)
cmd/replay/tui.go:483      b, err := os.ReadFile(filepath.Join(tipStateDir(), adviceFileName))
```

Every triage decision — `Applied`, `Dismissed` — is destroyed by the next `replay advise`. The one
`applied` row in the current file is a manual TUI mark (`track()` has never been able to return
`Applied`) and will not survive the next run. The comment in `triage.go` describes the bug the file
still has.

### 1.5 A perfect application cannot realize the prediction

Independent of everything above. `PredictedShare = 0.5 × Share` is *"the share of prompt tokens the
suggestion expects to remove per session"* (`advisor.go:97`), and against the **original** prompt
total that is correct. But `realized = before − after` compares two shares, and `after` is measured
against the **new, smaller** total.

Halve a target of share `s` and change nothing else:

```text
after   = (s/2) / (1 − s/2) = s / (2 − s)
realized = s − s/(2−s) = s(1−s)/(2−s)
realized / predicted = 2(1−s)/(2−s)
```

A flawless application registers as less than the prediction, by a knowable amount:

| target | share | best-case realized/predicted |
|---|---:|---:|
| `mcp__claude_ai_Upwork__upwork__*` | 41.4% | 0.74 |
| `mcp__claude-in-chrome__browser_batch` | 30.7% | 0.82 |
| `Read` | 30.3% | 0.82 |
| `Bash` | 30.0% | 0.82 |

On today's corpus the ceiling stays above `verifyShare = 0.5`, so nothing is wrongly failed *yet*.
Above `s = 0.5` it does: the currently-failing test's own fixture has `Share: 0.5648`, ceiling 0.60,
and at `s = 0.9` a perfect halving scores 0.18 and is stamped **not verified**. Choosing a share as
"the scale-free metric" (`advisor.go:4-5`) is what causes this: a share cannot cleanly measure a
change that shrinks its own denominator. Nothing in the repo states the correction and no test
asserts it.

---

## 2. Section A — every benefit claim, and whether anything checks it

A *benefit* claim asserts the tool produces a good outcome. A *measurement* claim asserts a number is
faithful. The repository audits the second class seriously and the first class not at all. That
asymmetry is the single structural finding of this file, and `docs/evidence/` proves it: **26 dated
evidence files, and not one of them studies whether acting on the output changes anything.** They
are calibration corpora, break-cause censuses, cache-accounting shapes, quota titrations, latency,
wire families, probe results — all fidelity, no benefit.

Legend: **T** = the number is tested as *computed*; **G** = tested as *true* against an independent
authority; **—** = neither.

### 2.1 `replay advise` — the command that exists to help

| # | Claim | Where | Falsifier | Checked |
|---|---|---|---|---|
| A1 | "predicted saving N% of prompt tokens per session (M tokens across the corpus)" | `cmd/replay/advise.go:125` | Apply the change; per-session prompt tokens do not fall by N% ± the fit's error | **—** No test, no evidence file, no trial. See §3.1 |
| A2 | "realized N%" printed beside A1 as an outcome | `cmd/replay/advise.go:126-127` | Show the figure moves when nothing was applied | **—** Refuted in §1.1: it is `sessions × share / N` |
| A3 | Status vocabulary "pending, applied, verified, not verified, advice only" implies progression | `cmd/replay/advise.go:132`; `advisor.go:66-81` | Find a status the pipeline cannot produce | **—** Refuted: `Applied` unreachable from `track()`; `Verified`/`NotVerified` unreachable at all on this tree (§1.3) |
| A4 | "Predictions assume the target is halved" | `cmd/replay/advise.go:114`; `trimShare = 0.5` `advisor.go:43` | Any measurement of what a real user achieves on a real target | **—** `0.5` is asserted. Nothing in the repo or the evidence directory measures an achievable trim |
| A5 | "The ranked list above this section is the same advice, **measured against your own sessions** rather than asserted." | `cmd/replay/advise.go:258` | Count suggestions that are never measured against anything | **—** 117 of 140 are `advice only` by construction (`advisor.go:378-380`): `KindHotFile` and `KindCacheBreaks` return early. The sentence is false for 84% of the list |
| A6 | hot-file: "repeats are N% of prompt tokens", predicted saving = **100%** of the repeats (no trim) | `advisor.go:348-351`, `describe()` `advisor.go:412-413` | Show a user cannot remove all repeat reads | **—** The most aggressive prediction in the tool sits on the one kind that is never verified |
| A7 | Per-suggestion "M tokens across the corpus" reads as additive | `cmd/replay/advise.go:125` | Sum them and compare to the corpus | **—** Sum = **1,255,321,841 tokens** across 140 suggestions. They double-count — see §2.2 |
| A8 | unused-tools: "N definitions … are X% of prompt tokens" | `advisor.go:285-287` | Compare `EstimateTokens(schema bytes)` to a tokenizer | **—** The fit is deliberately not fitted on schema bytes and is then applied to them; §4.3 |

### 2.2 The double count behind A7

`internal/advisor/advisor.go:151-161`: one tool-result blame entry for a file read is noted **twice**
— into `KindHotFile` via `noteReads` (line 154) and into `byTool`, which becomes `KindLargeResults`
(lines 156-160). Two suggestions, two predicted savings, the same tokens.

Measured on the live file:

| tool | hot-file suggestions | hot-file prompt tokens | hot-file predicted | large-results prompt tokens | large-results predicted |
|---|---:|---:|---:|---:|---:|
| `Read` | 116 | 76,091,308 | 63,865,820 | 61,600,300 | 30,800,150 |

The hot-file rows carry *more* prompt tokens than the single `large-results Read` row that overlaps
them, and 94.7M tokens of savings are predicted across the two. Nothing in the output says the rows
are not additive.

Related, unchecked: `fileTarget` (`advisor.go:180-183`) keys hot files on `path.Base`, so
`advisor.go` in two repositories is one target and its `reads` and `sessions` are the sum. `minReads`
(`advisor.go:49`) gates on that summed count.

### 2.3 The tool contradicts its own disclaimer

- `README.md:240` — "**You want a savings forecast.** Replay reports what was already spent, not
  what you will save." (listed under *Who should not use this yet*)
- `cmd/replay/cost.go:313` — "It is not a forecast of savings, it is what was already spent twice."
- `cmd/replay/share.go:79` — "Not a forecast of savings."
- `cmd/replay/advise.go:125` — "**predicted saving** 15% of prompt tokens per session (492M tokens
  across the corpus)."

Three surfaces of one binary disclaim forecasting and a fourth forecasts. A hostile reader finds this
by running `replay` twice.

### 2.4 "Sessions" means two things, 10.8× apart

Same machine, same corpus, same day:

```text
$ replay cost ~/.claude/projects
Cost per task, across 115 sessions (1729 agent lanes) ...

$ head ~/.replay/advice.json
"sessions": 1246, "transcripts": 1738
```

`advise` calls a calibrated lane a session; `cost` calls a lane a lane and something else a session.
Every share in `advice.json` is normalised by the first count. `README.md:80-84` explains the
lane/file distinction carefully for `transcripts` and does not mention that `sessions` also differs.

### 2.5 The largest unchecked benefit claim: what waste costs a flat-seat user

`docs/WHAT-YOU-GET.md:13-14` concedes the tool *"does not pay for itself in money on a flat seat"*,
and `docs/WASTE-DEFINITION.md:50-52` concedes that flat seats are *"where most coding-agent users
are."* So for the majority audience the entire value proposition rests on a second claim, made in
four places:

- `docs/WASTE-DEFINITION.md:56-57` — "**A context window is a fixed budget per session**, and
  everything wasted inside it is capacity that the actual work does not get."
- `docs/WASTE-DEFINITION.md:60-64` — four causal consequences, tabulated: *the session ends sooner*,
  *the agent gets worse as it goes*, *everything is slower*, *rate limits arrive earlier* ("the
  ceiling most subscription users actually hit").
- `docs/WASTE-DEFINITION.md:66-67` — quantified: *"a quarter of your context went on things that did
  nothing, so your agent hit its limit a quarter earlier than it needed to."*
- Shipped, unhedged, on the default screen — `internal/tui/measured.go:360-361`: *"N tokens is what
  the waste cost you: context the work did not get."* And in the default command,
  `cmd/replay/cost.go:316-318`: *"The tokens are still yours. They are context the work did not get,
  and **rate-limit budget spent on nothing**."*

**None of the four consequences is measured anywhere in the repository.** No test, no evidence file,
no trial. And the one adjacent measurement the project did run points the other way:
`README.md:228-235` reports matched cold-write and warm-read arms over **3.09M tokens** in which
*"the utilisation counter moved **zero** steps … That is a null result and it is published as one."*

That null is evidence *against* the "rate-limit budget spent on nothing" half of the sentence
`cmd/replay/cost.go:318` prints on every run. The repository published a measurement, called it a
null, and left the claim it undercuts in the default output. This is the sharpest instance of the
rigor asymmetry in the whole audit: the measurement was done honestly and never propagated to the
sentence it bears on.

Falsification conditions, all runnable by an operator with one subscription account:

- **"Rate limits arrive earlier" is refuted** if a session deliberately padded with 500k tokens of
  re-billed prefix reaches the 5h limit no earlier than a matched unpadded session. The instrument
  already exists (`replay probe`), and the earlier titration is the pilot.
- **"The session ends sooner" is refuted** if compaction fires at the same turn index in both arms —
  compaction thresholds are what actually end sessions, and `cmd/replay/trim.go:139-142` already
  admits trimming does not move them.
- **"The agent gets worse as it goes" cannot be tested by this tool at all.** It needs a per-session
  outcome, and `docs/design/surface-taxonomy-4-rewriting.md` (§D) explains why Replay is built never
  to see one. See §5.

### 2.6 Cross-file contradictions, verified by hand

Each of these is a falsifier already sitting inside the repository. Every quote below was read at the
cited line on this tree.

| # | One place says | The other says |
|---|---|---|
| X1 | `internal/advisor/advisor.go:354-356` — *"A break that does not happen re-bills nothing"*, so `KindCacheBreaks` predicts **100%** of observed re-billed tokens (`PredictedTokens = a.tokens`) | `docs/WHAT-YOU-GET.md:51-53` — *"5% is what was detected, not what is recoverable … A realistic recovery rate puts the true figure nearer **2 to 3 percent**"* |
| X2 | `internal/advisor/advisor.go:454` — *"disable this server if the work does not need it"*; `cmd/replay/mcp.go:322` — *"A tool never called is the whole figure wasted"* | `docs/WASTE-DEFINITION.md:29` — *"**Not waste. Insurance.**"*; `docs/WHAT-YOU-GET.md:153-154` — *"It is not proof you will not need it tomorrow, and that is insurance, not waste"* |
| X3 | `cmd/replay/cost.go:318` — re-billed tokens are *"rate-limit budget spent on nothing"* | `README.md:228-235` — the titration moved the utilisation counter *"**zero** steps"*, published as a null |
| X4 | `docs/PRODUCT-DIRECTION.md:16` — `error_share` is *"Money that bought nothing"* | `docs/WASTE-DEFINITION.md:42-43` — *"**`error_share` as currently defined is not a waste metric.** It is a *friction* metric, and calling it waste is the kind of overclaim the rest of this project exists to avoid"* |
| X5 | `cmd/replay/advise.go:125` — *"predicted saving"* | `README.md:240`; `cmd/replay/cost.go:313`; `cmd/replay/share.go:79` — *"not a forecast of savings"* (§2.3) |

X1 is the most consequential: the single `cache-breaks` suggestion in `advice.json` predicts
**23,387,943 tokens**, and its status is `advice only`, so the 100%-recovery assumption the repo's own
prose puts at 2–3% is never checked against anything.

X2 has an operational edge: `advise` tells the reader to disable a server, and the README's own
break-cause table (`README.md:188-191`) identifies mid-session tool-definition changes as a break
cause carrying ~361k tokens each. The advice and the diagnosis point in opposite directions and
nothing reconciles them.

### 2.7 A CI gate that is silently permissive

`cmd/replay/costgate.go:47-61` — on **failure**, the gate prints *"%d transcript(s) were excluded as
unpriced, so the real figure is higher"*, with a comment explaining that *"Excluded is not free."*

`cmd/replay/costgate.go:63-64` — on **pass**, it prints only *"avoidable spend $X is within the $Y
ceiling"* and **no unpriced caveat at all**.

The caveat is attached to the outcome that does not need it. A build passes a spend ceiling on a
corpus with unpriced holes and says nothing. Falsifier: run `--max-avoidable-usd` over a corpus where
the unpriced transcripts alone exceed the ceiling; the gate returns 0.

### 2.8 Measurement claims — for contrast, these *are* audited

| # | Claim | Where | Checked |
|---|---|---|---|
| M1 | "reproduces the provider's own cache reads on **97.46%** of compared turns" | `README.md:201` | **G** `internal/analysis/analysis_test.go:26` predicts each turn's read from prior usage and compares to the provider's `cache_read_input_tokens` on a real redacted transcript |
| M2 | Attribution sums to what the provider billed | `README.md`; blame output | **G** for the total: `internal/analysis/reconcile_test.go:40`, `analysis_test.go:70`. Explicitly **not** for the split — `internal/analysis/conservation_test.go:15`: *"Conservation is necessary and not sufficient: it pins the total and says nothing about the split."* The split is where every estimate lives |
| M3 | Break-cause distribution (50.8% re-render, 33.9% TTL) | `README.md:94-98` | **T**, with an unusually honest sampling caveat at `README.md:105`. My corpus run gives 77.8% / 6.2% — see §4.4 |
| M4 | Sample is 78 sessions, not 1450 files | `README.md:203-206` | Self-correcting; the retraction is published |
| M5 | List prices dated `2026-09-07` | `internal/cachemodel/anthropic.go:24` `PriceTableVersion` | **—** No test fetches a published sheet; `internal/cachemodel/pricecheck_test.go:16` cross-checks against an inline JSON literal. The real comparison happens only when a human runs `rules --check-prices` |

M1 and M2 are the only two places in 201 test files where the tool predicts a number and an
independent authority says whether it was right. Both concern the cache model. Neither concerns the
advice.

### 2.10 The count

A systematic sweep of `README.md`, `docs/WHAT-YOU-GET.md`, `docs/PRODUCT-DIRECTION.md`,
`docs/WASTE-DEFINITION.md` and every user-facing string in `cmd/replay/` and `internal/tui/`
enumerated **223 distinct claims**, each with a `file:line`. Classified:

| class | count | tested as *true* |
|---|---:|---|
| (d) coverage / completeness — "it names every break", "it reads no config" | ~95 | Several. The no-network and no-config promises have real guard tests; the completeness promises do not |
| (a) accuracy of a number | ~50 | Two — M1 and M2 — both about the cache model |
| **(b) savings / benefit if you act** | **~65** | **Zero** |
| **(c) comparative — "cheaper", "worth it", "nobody else can"** | **~13** | **Zero** |

**About 78 benefit-and-comparative claims, and not one has a test that could falsify it.** The
strongest confirmation is structural and negative: `docs/evidence/` holds 26 dated studies, and none
of them is a benefit study.

The claims cited in §2.1–§2.9, §3 and §4 were each read at the cited line on this tree. The
classification counts above come from the sweep and are approximate at the (b)/(d) margin.

Further (b)/(c) claims that ship in the binary and have no check:

- `cmd/replay/route.go:307-309` — *"At $N saved per turn it repays on turn K, inside the M turns
  measured. **Switching is worth it for work this long or longer.**"* The most direct act-on-this
  instruction in the tool. Falsifier: switch, run work of that length, fail to recover the switch
  cost — or lose output quality, which `route` cannot see and `docs/WHAT-YOU-GET.md:147-150` says it
  must not judge.
- `cmd/replay/apply.go:189-190` — the evidence sentence behind the one setting the tool will actually
  write to disk. Its designed falsifier (`replay verify`, `cost --compare`) exists and has never been
  run to a conclusion.
- `internal/advisor/advisor.go:439-440` — *"truncate outputs before they enter the conversation"*.
  Falsifier: truncation causes re-runs costing more than the tokens saved.
  `internal/analysis/trim.go:96` concedes its harm probe is *"a LOWER BOUND on harm"*, and
  `trim.go:101-103` documents a case the probe scores **backwards** — removing test failures produces
  fewer edits, *"which this counts as a saving rather than as damage."*
- `internal/advisor/advisor.go:445-446` — *"split instruction files … move the rest to on-demand
  files"*. Falsifier: the cold writes for on-demand loading exceed the always-on carry — exactly the
  trade `docs/WHAT-YOU-GET.md:68-71` warns about, applied by the advice in the opposite direction.
- `internal/advisor/advisor.go:456-457` — *"defer-load tools the session does not use"*. Falsifier:
  defer-loading binds tools mid-session, which is the break cause `README.md:188-191` singles out as
  the largest by size.
- `cmd/replay/tipvariant.go:66` — *"Replay just found $N you had already paid for once."* On a flat
  seat no dollars were paid; `cmd/replay/cost.go:315` says exactly that three lines earlier in the
  same output.

---

## 3. Section B — the four benefit tests nobody has run

### 3.1 B1 — Apply one suggestion deliberately and measure

**What it settles:** whether `trimShare = 0.5` describes anything real, and whether acting on the
top suggestion moves prompt tokens at all.

**Cannot currently be run through the tool.** §1.3 and §1.4: the `applied` flag is unwired and a
triage mark is destroyed by the next `advise`. It must be run manually against frozen corpora.

**Procedure.** Target `large-results` / `Bash` (share 30.0%, predicted 15.0%, 662 sessions — the
largest row in the file).

1. Freeze: `cp -R ~/.claude/projects /tmp/corpus-before`. Record
   `replay advise /tmp/corpus-before --out /tmp/before.json`.
2. Intervene: wrap `Bash` so every result is truncated to 2 KB (`head -c 2048`), and change nothing
   else — same repositories, same task mix, same model.
3. Run ≥ 20 new sessions. Collect only those into `/tmp/corpus-after`.
4. `replay advise /tmp/corpus-after --out /tmp/after.json` and compare.

**Primary endpoint must be absolute prompt tokens per session, not share.** §1.5 is the reason: the
intervention shrinks the denominator, so a share understates a real saving by a computable factor.
Report both.

**Falsification conditions**, stated in advance:

- **The prediction is refuted** if mean prompt tokens per session in the *after* set does not fall,
  or if the `Bash` tool-result share falls to more than 25% (predicted post-state is 17.6%; see
  §1.5) — i.e. the halving assumption did not survive contact with the work.
- **The metric is refuted** if absolute Bash result tokens per session halve while `RealizedShare`
  reports anything other than ≈ 12.4 points (§1.5's `s(1−s)/(2−s)` for `s = 0.30`).
- **The whole model is refuted** if total prompt tokens per session are flat or rise while Bash
  tokens fall — meaning the agent compensated (re-running truncated commands, reading files instead),
  which is the substitution effect the tool has no way to see.

**Cost:** ~20 sessions of real work, days to weeks of wall clock. **Confounds that must be held:**
task mix, repository, model, and the `--append-system-prompt`/tool set, because all of them move
prompt tokens more than the intervention does. Not runnable retrospectively from data on this
machine: no historical session recorded whether the operator was trying.

### 3.2 B2 — Does the offline estimate agree with the proxy's measured tier?

**Status: I ran it. It fails, 4 of 4.**

Four ledger sessions have a matching Claude Code transcript by session UUID:

```text
$ for f in 351d4e3d... 7022b9f2... f11756ad... f79cae6d...; do find ~/.claude/projects -name "$f.jsonl"; done
/Users/daniel/.claude/projects/-Users-daniel-Development-Replay-clean/351d4e3d-9908-4577-aa11-fdd093c0b21f.jsonl
/Users/daniel/.claude/projects/-Users-daniel-Development-Replay-clean/7022b9f2-59f7-4356-9a44-f13b211512b7.jsonl
/Users/daniel/.claude/projects/-Users-daniel-Development-Replay-clean/f11756ad-8ad7-43f2-b88a-ce8076218921.jsonl
/Users/daniel/.claude/projects/-Users-daniel-Development-Replay-clean/f79cae6d-0d11-4033-ac6f-15f25a5503d5.jsonl
```

`replay context` on both recordings of the same session:

| session | total | ledger (measured) | transcript (estimated) |
|---|---|---|---|
| `7022b9f2` | 205k / 205k | tool **91.8%**, user 5.3%, system 2.7%, Bash 0.3% (558) | system **89.1%**, Bash 10.9% (22k), user 0.0% (58) |
| `351d4e3d` | 106k / 106k | tool **86.3%**, user 9.1%, system 4.5% | system **100.0%** |
| `f11756ad` | 203k / 203k | tool **92.0%**, user 5.5%, system 2.5% | system **100.0%** |
| `f79cae6d` | 37k / 37k | tool **66.7%**, user 23.7%, system 9.6% | system **100.0%** |

**The totals agree exactly and the attribution does not.** Both paths anchor to provider usage, so
the total is conserved (that is M2). The split — which is the entire product, and the sole input to
`advisor.Observe` (`advisor.go:147-170` iterates `rep.Blame`) — is contradictory. `Bash` on
`7022b9f2` is 558 tokens on one path and 22k on the other, a factor of 40.

`replay blame` on the same session localises the disagreement:

```text
LEDGER      2. tool definitions (224 tools)      x1  21k once   167k in prompts (±167k)
TRANSCRIPT  2. tool result: Bash echo one        x1  21k once   171k in prompts (±171k)
```

The same ~21k block is attributed to a tool-definition re-lay by one path and to a Bash result by the
other. One of them is wrong, and the ledger — which saw the wire — is the one to believe. The
transcript path pushes unattributable mass into the estimated unseen prefix
(`internal/analysis/fit.go:203-219`). The request counts also differ: 10 vs 9.

**Honest limits.** These four are short, near-synthetic sessions (`Bash echo one` … `echo eight`),
zero fitted turns, so the *magnitude* here will not generalise. The *direction* is systematic and
follows from the code. n = 4 is all the paired data on this machine.

**The existing test does not cover this.** `internal/proxy/server_test.go:793`
`TestWhatIfMatchesOfflineReplayAndStaysOffTheWire` does diff a live and an offline path, but its own
header says why that is not validation — `server_test.go:790-792`: *"since they come from the same
simulator over the same records"* — and its upstream is `&invariantUpstream{perTurn: 800}`
(`server_test.go:794`), a fake whose usage is constructed to satisfy the invariant. Both sides
consume the same synthetic numbers through the same code. It guards against divergence; it is not
evidence either side is right.

**To close it properly:** 20–30 real sessions driven through `replay serve` while Claude Code also
writes its transcript, then a per-label diff of `blame` output. **Falsification:** for each label
present in both, the transcript-path token figure must lie within the ledger figure ± the reported
`±` band. On `7022b9f2` the `Bash` label misses by 40×, and the band on the transcript side is ±100%
— still not wide enough. Cost: a few days of ordinary work with the proxy in front. This is the
cheapest of the four and it is already failing.

### 3.3 B3 — Is the byte-to-token fit any good?

**What it settles:** the credibility of every figure marked `*`, of `KindUnusedTools` entirely, and
of 91.6% of cache-break causes (§4.4).

**Nothing tests it today.**

```text
$ grep -rn "EstimateTokens" --include="*_test.go" .
(no results)
```

`TokenFit.EstimateTokens` (`internal/analysis/fit.go:239`) is the function every estimated token
figure flows through and no test calls it. The only check anywhere is a plausibility band —
`internal/analysis/analysis_test.go:61`: `if fit.TokensPerByte <= 0 || fit.TokensPerByte > 2` — which
admits essentially every value the code can emit. `TokensPerByte` appears elsewhere in tests only as
a hand-supplied struct literal (`internal/analysis/route_test.go:117-118`,
`cmd/replay/route_test.go:17-18`): fed in, never checked.

No tokenizer exists in the tree. `go.mod` is three lines with zero requires; there is no `go.sum` and
no `vendor/`. The only `count_tokens` caller is `internal/probe/run.go:230`, reached only by
`replay probe --execute`; in tests it is a stub that divides by a constant
(`internal/probe/run_test.go:48`), and the stub says so at `run_test.go:33`.

**How bad is the error?** Measured across the whole corpus by parsing the `Rules:` line
(`internal/analysis/report.go:151`) from `replay diff ~/.claude/projects`, 1734 lanes:

```text
median relative error   49%
p75                     80%
p90                    100%
> 50%                  842 lanes (48.6%)
= 100%                 315 lanes (18.2%)
   of which 0 fitted turns:  97
   of which 1 fitted turn:   81
```

The `±100%` figures are not measurements. `weightedSpread` returns `1` whenever
`len(samples) < 2 || mean == 0` (`internal/analysis/fit.go:220-222`) and `Fit` sets
`RelativeError = 1` when `sumBytes == 0` (`fit.go:197-199`). So at least 178 lanes print an error bar
that is a placeholder, and the report renders it identically to a measured one. On `7022b9f2` this
produces a headline row reading `1.50M in prompts (±1.50M)` — an uncertainty equal to the value,
printed as the #1 top token source.

`docs/design/surface-taxonomy-4-rewriting.md:313` already reaches the right conclusion for a
different feature — *"the fitted ratio's relative error is a byte-weighted standard deviation that
reaches ±159% across sessions … a deadline rule built on that estimate would straddle its own
threshold on most requests"* — and the straddle doctrine it invokes
(`internal/analysis/preflight.go:99-110`, `PreFlightDeficit.Straddles`) is applied to pre-flight
refusals and **not** to advice.

**The self-contradiction.** `Fit` deliberately excludes prefix-changed turns because *"its write
covers tool definitions, which are denser than prose and would drag the fit"*
(`internal/analysis/fit.go:178-182`). `internal/proxy/preflight.go:41-48` repeats the point: *"System
prompts and tool definitions are JSON schemas, which are denser than prose, so the prose default
understates them."* Then `internal/advisor/advisor.go:285` calls
`fit.EstimateTokens(b.bytes)` on **exactly** tool-definition bytes to size a `KindUnusedTools`
suggestion. The code states the ratio does not describe schemas and then uses it on schemas.

**Test, with its falsification condition.** Take the 1.4 MB real transcript already in the tree
(`internal/transcript/testdata/session-redacted.jsonl`), extract the user-side content blocks of each
fitted turn, count them with an authority — Anthropic's `count_tokens` endpoint, or an offline
`cl100k`/Claude BPE table — and compare per turn.

- **The fit is adequate** if the per-turn tokenizer count lies inside `EstimateTokens(bytes) ×
  (1 ± RelativeError)` for ≥ 90% of turns.
- **The fit is refuted** if the *median* absolute relative deviation exceeds the reported
  `RelativeError`, i.e. the error bar understates the error.
- **The schema application (A8) is refuted separately** if tokens-per-byte measured on tool-definition
  JSON differs from the session's fitted prose ratio by more than that ratio's own error band. The
  code predicts it will; nobody has shown it.

**Cost:** low. A day. Either offline (a BPE table, vendored under a build tag or kept out of the
module) or a few dollars of `count_tokens`. **What is blocked on this machine:** a *Claude* tokenizer
is not public; `count_tokens` needs a key and network. `cl100k` would establish the shape of the error
but not its exact size for this provider. That gap is real and should be stated in whatever evidence
file results.

### 3.4 B4 — Would a human agree a cache break is a break?

**What it settles:** whether `replay diff`, the command in the tool's one-line description
(*"see where your coding agent's prompt cache broke"*), attaches the right cause.

**No ground truth exists.** Exactly one test labels a real break:
`internal/analysis/analysis_test.go:38 TestBreakIsClassifiedAsRerender`, a single human label on one
transcript. `internal/analysis/analysis_test.go:33` asserts `cal.Broken != 1` on the same file. That
is n = 1.

**How much rides on the untested fit.** From my run of `replay diff ~/.claude/projects` (763
classified breaks over 1734 lanes):

```text
594  77.8%  client re-rendered history after the system prefix
105  13.8%  prefix diverged inside the message history at an unknown block
 47   6.2%  cache expired (gap longer than the TTL)
 11   1.4%  system prompt or tool definitions changed
  6   0.8%  model changed between requests
```

Only the bottom three — 64 breaks, **8.4%** — are decided from usage, model id and wall-clock gap
alone (`internal/analysis/diff.go:50-61`, delegating to `cachemodel.ClassifyBreak`). Those are
independently checkable and are the ones the existing evidence covers.

The top two, **91.6%**, ride on the fit:

- `CauseRerendered` (`internal/analysis/diff.go:73-80`) fires when
  `math.Abs(float64(t.Actual) - unseen) <= unseen * rerenderTolerance`, with
  `rerenderTolerance = 0.10` (`diff.go:30`) and `unseen = fit.UnseenPrefix.Total()` — itself an
  estimate produced at `fit.go:203-219`. **A ±10% acceptance band is applied to a quantity whose own
  median relative error is 49%.** The comment at `diff.go:27-30` concedes the estimate is *"coarse by
  nature"* and picks 10% anyway.
- `CauseUnknown`'s reported position comes from `locateByTokens` (`diff.go:107-119`) walking the same
  fit, and the printed evidence says so: *"position is an estimate from the byte-to-token fit"*.

Because the tolerance band is much narrower than the estimate's error, `CauseRerendered` is
effectively a coin-flip dressed as a classification, and it is the modal answer.

`README.md:94-98` publishes 50.8% re-render / 33.9% TTL; my run gives 77.8% / 6.2%. `README.md:105`
already warns the ranking is sample-dependent, which is to the repository's credit and does not
substitute for a labelled set.

**Test, with its falsification condition.** Build a labelled set of 100 breaks. For the ledger tier
the label is derivable without a human: the proxy sees the actual bytes of consecutive requests, so
"the prefix changed" and "the history was edited" are *observable facts*, not inferences. Label those
mechanically from the wire, then run the transcript-tier classifier over the same sessions blind.

- **The classifier is adequate** if it agrees with the wire-derived label on ≥ 85% of breaks and its
  `CauseUnknown` rate is below 15%.
- **`CauseRerendered` is refuted** if, on the subset where the wire shows the prefix did *not* change,
  the classifier still says "re-rendered" more than 15% of the time.
- **The tolerance is refuted** if perturbing `rerenderTolerance` from 0.10 to 0.15 moves more than
  10% of breaks between causes — that would show the band, not the evidence, is choosing the answer.
  **This one is runnable today** on the existing corpus, offline, in an afternoon, and needs no new
  data at all.

**Cost:** the perturbation arm is hours. The labelled arm needs 20–30 ledger sessions containing
real breaks; only 4 ledger sessions exist. A human-only variant (show an engineer the two requests
and ask "did the prefix change?") is possible but slow and unnecessary for the ledger tier, where
the answer is on the wire.

---

## 4. Section C — gaps beyond the four, ranked by what a hostile reader finds first

**C1. The suite is red on the current tree** (§1.3). `go test ./internal/advisor/` fails. Whatever
else is true, a reader who runs the tests first sees this first.

**C2. `advise` forecasts savings while three other surfaces of the same binary say it does not**
(§2.3). `README.md:240` vs `cmd/replay/advise.go:125`. Found by reading the README and running the
tool.

**C3. "measured against your own sessions rather than asserted" is false for 84% of the list**
(`cmd/replay/advise.go:258`; `advisor.go:378-380`). 117 of 140 return `AdviceOnly` before any
measurement happens.

**C4. Predicted savings sum to 1.26 billion tokens and double-count** (§2.2). 116 hot-file `Read`
rows (63.9M predicted) overlap one `large-results Read` row (30.8M predicted) by construction at
`advisor.go:151-161`. No note tells the reader the rows are not additive.

**C5. Single-session suggestions carry corpus-scale predictions.**
`tool-inputs / mcp__claude-in-chrome__javascript_tool`: `sessions = 1`, `predicted_tokens =
30,059,333`. `PredictedTokens` is `0.5 × a.tokens` (`advisor.go:359`) where `a.tokens` already counts
each block once per prompt that carried it, so a single long session's carry cost is presented as a
corpus-wide saving. Three more single-session rows exceed 1M predicted tokens.

**C6. "Sessions" means 1246 in one command and 115 in another** (§2.4).

**C7. The one properly-designed benefit experiment in the repo has never returned a result.**
`internal/learn/graduate.go:60-130` implements a real treated/control trial with confidence intervals
and a held-out check, and `cmd/replay/learn.go:97` prints *"realized saving N%, predicted N%"* from
it. `DefaultTrialMinSessions = 5` (`graduate.go:19`). Current state, `~/.replay/policy.json`
(2026-09-09T18:22): `"sessions": {"found": 1, "calibrated": 1, "holdout": 1}` and every candidate
`"rejected: fewer than 5 sessions with evidence"`. The design is right and aimed at the wrong
subject: it tests *policies the proxy can change*, not *advice a human must act on*. It is starved
because only 4 ledger sessions exist against 1738 transcripts.

**C8. `PriceTableVersion` is a compiled constant no test validates against a published sheet**
(`internal/cachemodel/anthropic.go:24`; `internal/cachemodel/pricecheck_test.go:16` compares against
an inline JSON literal). Every dollar figure in every report depends on it, and the check runs only
when a human types `rules --check-prices`. Falsifier: a published price changes and CI stays green.

**C9. `RelativeError = 1` is a sentinel rendered as a measurement** (§3.3). 18.2% of lanes print
`±100%` that means "no data", indistinguishable in the output from a measured 100% spread.
`FitNote` (`fit.go:266-272`) distinguishes *zero* fitted turns in prose but not the one-turn case,
which is 81 of those lanes.

**C10. Cost and quota have no invoice check.** `cmd/replay/verify.go:11-23` states this itself:
*"Everything else available today either measures the engine against its own model, or asks people
what they believe. Neither is evidence."* Its six tests (`verify_test.go:20-114`) exercise the
before/after median arithmetic on hand-built slices. No test compares a total to a provider invoice
or a subscription usage page. Falsifier: a month's `replay cost` total differs from the Anthropic
console by more than the reported band. **Closable on this machine** by anyone with console access.

**C7b. The repository contradicts itself in five places on what the tool is for** (§2.6). X1 — the
advisor predicts 100% recovery of cache-break tokens while `docs/WHAT-YOU-GET.md:51-53` puts
realistic recovery at 2–3%. X2 — `advise` and the MCP surface tell the reader to disable unused
servers while `docs/WASTE-DEFINITION.md:29` says that is insurance and must not be recommended. X3 —
`cost.go:318` claims re-billed tokens are rate-limit budget spent while the repo's own titration
returned a null. X4 — `error_share` is "money that bought nothing" in one doc and explicitly retracted
as an overclaim in the next. X5 — three surfaces disclaim forecasting and a fourth forecasts.

**C7c. The `--max-avoidable-usd` CI gate prints its data-quality caveat only when it fails**
(§2.7, `cmd/replay/costgate.go:47-64`). A build passes a spend ceiling over a corpus with unpriced
holes and says nothing.

**C11. Substitution is invisible.** Every prediction assumes the agent behaves identically after the
change; the report says so once, as an assumption line
(`"replayed savings assume the agent would have behaved identically under the alternative layout"`).
If truncating `Bash` output makes the agent run three commands where it ran one, prompt tokens rise
and the tool records a success. Nothing measures re-issue rate, and B1 is the only test above that
could detect it. `docs/design/surface-taxonomy-4-rewriting.md` (§D) explains why the tool structurally
cannot see task outcomes: it never reads model output. That boundary is deliberate and correct, and
it means **no benefit claim about work quality can ever be checked from this data.**

**C12. `MinShare = 0.10` (`advisor.go:40`) is a selection filter with survivorship built in.** A
target only enters a session's evidence when it exceeds 10% *in that session*. So `Share` is a mean
over the sessions where the target was already large — a conditional mean presented as a typical
value. That is the same conditional/unconditional confusion as §1.1, one layer up, and it inflates
every headline percentage in the list. Falsifier: recompute `Share` over all sessions and compare;
if the unconditional mean is materially lower, the reported figure describes a selected subsample.
**Runnable today** from `advice.json` plus a corpus re-scan.

**C13. Hot-file targets collide on base name** (`advisor.go:180-183`). `minReads = 3` and the
`(reads−1)/reads` factor (`advisor.go:348`) both operate on a possibly-merged count.

**C14. `appliedDrop = 0.2`, `verifyShare = 0.5`, `recentSessions = 2`, `trimShare = 0.5`,
`rerenderTolerance = 0.10`, `MinShare = 0.10` are all asserted constants.** Not one has a sensitivity
analysis in `docs/evidence/`. C-level ranking aside, `recentSessions = 2` is the sharpest: two
sessions is a sample of two, and the whole verification verdict turns on it.

---

## 5. What cannot be closed with data on this machine

| Question | Blocked by | What would close it |
|---|---|---|
| Does acting on advice reduce spend? (B1) | No historical session recorded intent; needs a controlled intervention | ≥ 20 new sessions with one variable changed and the corpus otherwise held |
| Is the fit right for *Claude*? (B3) | No public Claude tokenizer; no network in tests | `count_tokens` against a key, or a published BPE table. `cl100k` gives shape, not size |
| Do break causes match the wire? (B4, labelled arm) | 4 ledger sessions, few real breaks | 20–30 sessions through `replay serve` in normal work |
| Do the two tiers agree in general? (B2) | 4 paired sessions, all short and near-synthetic | The same 20–30 paired sessions |
| Do the dollar totals match an invoice? (C10) | No invoice in the repo | The operator's own console export, one month |
| Does the advice improve *work outcomes*? | Structural: the tool never reads model output (`internal/analysis/summarize.go:114` `stripText`) | Nothing on this machine. It needs an external, machine-checkable per-session outcome — tests passing, a PR merged — that Replay is deliberately built not to see |

The B4 tolerance-perturbation arm and the C12 conditional-mean recomputation are the exceptions:
both run offline, today, on data already present.

---

## 6. Summary

- **Of 223 enumerated claims, about 78 are benefit or comparative claims, and none has a test that
  could falsify it** (§2.10). Two accuracy claims — M1 and M2 — have genuine ground truth, and both
  are about the cache model rather than the advice. `docs/evidence/` holds 26 dated studies and not
  one of them studies whether acting on the output changes anything.
- **The 1-of-21 result is a broken verifier, not failing advice.** `RealizedShare` is arithmetically
  `sessions × share / N` (max error 1.4×10⁻⁴ over all 21 rows) and `Verified` means only
  `sessions ≥ N/4`. It has never contained information about whether anyone applied anything.
  Commit `1b82dc7` reaches the same conclusion by a different route and removes the circularity; it
  does not remove the denominator mismatch (§1.1), the post-change-denominator error (§1.5), or the
  destruction of triage marks (§1.4), and it leaves `Verified` unreachable and the suite red (§1.3).
- **The most dangerous unverified claim is the flat-seat one** (§2.5): that re-billed tokens are
  *"context the work did not get, and rate-limit budget spent on nothing"*
  (`cmd/replay/cost.go:316-318`, `internal/tui/measured.go:360-361`), elaborated into four causal
  consequences and a proportionality at `docs/WASTE-DEFINITION.md:56-67`. It is the entire value
  proposition for the audience the project itself calls the majority; it ships unhedged in the
  default command and the default screen; nothing measures any of the four consequences; and the one
  measurement the project *did* run on the closest question — 3.09M tokens, utilisation counter moved
  zero steps (`README.md:228-235`) — is evidence against the rate-limit half. A published null result
  never reached the sentence it bears on.

  The runner-up is A5 — *"measured against your own sessions rather than asserted"*
  (`cmd/replay/advise.go:258`) — which converts an unmeasured list into an audited one in the
  reader's mind and is false for 117 of 140 rows by construction.
- **The cheapest decisive test is B3.** Everything estimated, 91.6% of break causes, and the entire
  `unused-tools` kind depend on `TokenFit.EstimateTokens`, a function with zero test coverage whose
  median relative error on this corpus is 49% and whose stated error bar is a sentinel on 18% of
  lanes. Run it first, offline, in a day: everything else in this file is easier to interpret once
  the size of that error is known.

  Two smaller arms are runnable today with no new data at all: perturbing `rerenderTolerance` from
  0.10 to 0.15 and counting how many of the 763 breaks change cause (§3.4), and recomputing `Share`
  unconditionally to size the `MinShare` selection effect (C12).
