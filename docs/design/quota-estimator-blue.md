# `replay quota`: what an honest estimator can tell a flat-seat subscriber

**Status: open.** A design, not an implementation. Nothing here has been built,
and two of the figures it would most like to print are refused rather than
estimated.

**The question it opens:** a Pro, Max or Enterprise subscriber spends no dollars
per token. Their constraint is a window, their fear is being cut off mid-project,
and every headline figure this tool prints is denominated in a currency they do
not spend. `internal/ledger/record.go:88-96` already says where their real budget
is visible. This document works out how much of it can be read back honestly, and
names the parts that cannot.

---

## 0. What exists today

The capture path is complete and the read path is empty. Precisely:

| Piece | Where | State |
|---|---|---|
| Header capture, verbatim, allowlisted by prefix | `internal/proxy/quota.go:16-76` | Shipped |
| Written onto every proxied response | `internal/proxy/server.go:595` | Shipped |
| Stored unparsed on the record | `internal/ledger/record.go:88-96` | Shipped |
| Raw records readable, `Quota` intact | `internal/ledger/store.go:142-168` | Shipped |
| Subtraction estimator over records | `internal/quota/quota.go:112-196`, `232-278` | Shipped, **no importer** |
| Forecast over readings | `internal/quota/forecast.go:36-107` | Shipped, **no importer** |
| Status-line window renderer | `cmd/replay/quotaline.go:47-101` | Shipped |
| Status-line reading persisted to disk | `cmd/replay/quotastore.go:37-72` | Shipped, **no production caller** |
| MCP tool that reads that store | `cmd/replay/mcp.go:268-280` | Shipped |

Three facts fall out of that table and they shape everything below.

**`internal/quota` is dead code.** No file outside the package imports it.
`Samples`, `Compare` and `Forecast` are written, tested, and reachable by
nothing. `replay quota` is mostly a wiring job over an estimator that already
exists and already refuses correctly.

**`ledger.ReadFile` drops the reading.** It converts records into a
`transcript.Session` (`internal/ledger/store.go:172-181`) and
`requestFromRecord` (`:249`) carries no `Quota`. Every existing command reaches
the ledger through `forEachSession` (`cmd/replay/main.go:359`), which is the
session path. `replay quota` must use `ledger.ReadRecords` directly. Reusing the
session helper would silently produce a command that reports "no headers" on a
ledger full of them.

**`saveQuota` is never called.** Its only callers are in
`cmd/replay/quotastore_test.go`. `runStatusline` renders the window
(`cmd/replay/statusline.go:93`) and discards it — the exact defect
`cmd/replay/quotastore.go:10-19` was written to fix, fixed in the writer and not
in the caller. So `replay_quota` (`cmd/replay/mcp.go:269-272`) answers "No quota
reading has been stored" on every machine, always, and
`cmd/replay/burn.go:245` still hardcodes `quota: "not reported"` for
claude-code. This is the cheapest repair in the document and it is a
prerequisite for the entire no-proxy path in section D.

### The two sources are different instruments

They are not interchangeable and the design must never average them.

| | Proxy headers | Status line |
|---|---|---|
| Reaches the ledger via | `replay serve` only | a Claude Code status-line hook |
| Names | `anthropic-ratelimit-unified-5h-utilization`, `-7d-utilization`, `-representative-claim`, `-status` | `rate_limits.{five_hour,seven_day,spend_limit}` |
| Scale | fraction, `0.00`–`1.00`, two decimals (`internal/quota/quota.go:41-46`) | percent, `0`–`100` (`cmd/replay/quotaline.go:80`, `:104-119`) |
| Reset instant | **not present** — see A2 | `resets_at`, unix seconds |
| History | one row per request, in the ledger | one file, overwritten (`cmd/replay/quotastore.go:37`) |
| Names the binding window | yes, `-representative-claim` | no; `quotaLine` picks the max instead |

A 100x unit error between those two columns is the most likely single bug in any
implementation of this command. It belongs in a test before it belongs in a
function.

---

## A. What can be computed honestly

Every row states its formula, its inputs, its truth tier per
[ADR-0002](../adr/0002-replay-engine-and-truth-tiers.md), its error bar, and the
condition under which it refuses. *Structural* is used as
`internal/analysis/order.go:76-92` uses it: arithmetic over supplied counts,
neither measured on the wire nor fitted.

### A1. Current window utilisation — **measured**

**Formula.** Take the newest record whose `Quota` carries any member of
`internal/quota/quota.go:44-47`; `ParseFloat` its value. The binding window is
the value of `anthropic-ratelimit-unified-representative-claim`, not the larger
of the two. `internal/quota/forecast.go:15-18` is right about this and
`cmd/replay/quotaline.go:33-40` is doing the best it can without it: picking the
higher figure is a reasonable heuristic for a payload that does not say, and the
wrong choice for a header family that does.

**Inputs.** `[]ledger.Record` from `ledger.ReadRecords`; each record's
`Timestamp` for the age.

**Error bar.** ±0.005, one half of the counter's published two-decimal
resolution (`internal/quota/forecast.go:30-33`). Plus staleness: the reading is
as old as the request that carried it. Age is not a footnote here — print it
next to the figure, per `cmd/replay/quotastore.go:26-31`: *"a reading from three
hours ago renders identically to a live one, and a person deciding whether to
start a long run acts on it."*

**Refuses when.** No record carries a utilisation or remaining header
(→ section D); or the reading's window has already reset. The status-line
precedent at `cmd/replay/quotaline.go:63-68` is the rule to copy — an expired
window is *no reading*, never a full one.

**Never.** Do not reconcile the header reading against the status-line reading.
If both exist, print both rows with their own ages and sources. They are
measurements by different instruments at different instants, and one number
split from two is a figure neither instrument produced.

### A2. Time to reset — **reported**, and only from the status line

**This is the honest surprise in the header family.** Enumerating every
`anthropic-ratelimit-*` string in this repository — source, tests, fixtures and
prose — yields exactly five names: `-unified-5h-utilization`,
`-unified-7d-utilization`, `-unified-representative-claim`, `-unified-status`,
and the API-key family `-tokens-remaining` / `-tokens-limit` / `-tokens-reset` /
`-requests-remaining` / `-requests-limit`. **There is no observed reset field in
the unified family**, and a subscription session returns none of the API-key
family at all (`internal/quota/quota.go:33-42`; `docs/guide/commands.md:503-507`).

So for the population this command exists for, the proxy headers give a level
and no deadline. Time-to-reset comes from `rate_limits.*.resets_at` or it does
not come at all.

**Formula.** `time.Unix(resets_at, 0).Sub(now)`, from the stored reading.

**Error bar.** Small and of an unusual kind: `resets_at` is an *absolute*
instant, so unlike A1 it does not decay with the age of the reading. A
four-hour-old reading still names the right reset time while its utilisation
figure is worthless. Say so; it is the one place a stale reading is still good.

**Refuses when.** No stored reading, or `resets_at <= 0`, or the instant has
passed.

**Not derivable.** The window's *start* cannot be inferred from a utilisation
figure, so "you are 40 minutes into a five-hour window" is not available from
headers alone. NOT MEASURED.

### A3. Share of the consumed window that went to re-billed tokens — **refused**

This is the figure the user most wants and it is the one this design will not
print. The reasons are cumulative and each is sufficient.

**1. The token→window conversion is null.**
`docs/evidence/quota-titration-2026-09-06.md` ran matched arms — ten cold writes
and ten warm reads of a ~159,000-token prefix, same model, same size — and moved
**3.09M tokens through a counter that did not advance one step**. `Compare`
correctly refuses on that data and names the short arm
(`internal/quota/quota.go:262-270`). A ratio near 12.5 and a ratio near 1.0
remain equally consistent with what has been measured.

**2. The counter is account-wide, so attribution does not work.** The titration
is explicit: an earlier "one step ≈ 475,000 tokens" calibration was **voided**
because an interactive Claude Code session was running on the same account
throughout. Its conclusion — *"passive attribution of an account-wide counter to
individual requests does not work"* — applies to any derivation of "your cache
breaks consumed X% of your window", including the one this command would be
asked for.

**3. The two 2026-09-06 records disagree in exactly the direction attribution
needs.** The header comment at `internal/analysis/predictor.go:5-11` records
3,778,706 re-billed tokens co-occurring with 5h utilisation moving 0.10 → 0.15.
The titration records 3.09M matched tokens co-occurring with zero movement.
Neither is a contradiction, because **neither is an attribution** — both are
co-occurrence on a shared counter. `docs/evidence/subscription-allowance-2026-09-09.md`
scales the first of them by 8.94x to reach "~0.36–0.54 of a five-hour window",
and its own limits 1 and 5 already flag the linearity assumption and the unknown
accounting rule. This design adds a third: the confound the titration named in
the same week is inherited by that extrapolation, and 0.45 windows is a
co-occurrence scaled, not a cost measured. It is a fine number for an evidence
document that states its limits in full. It is not a number to print next to a
live window as though it were this user's.

**What the command prints instead.** The token count, on its own terms, with the
refusal beside it:

- avoidable (re-billed) tokens over the ledger, and their share of total tokens
  — **structural**, arithmetic over `Response.Usage` counts that the existing
  cost path already produces and `cmd/replay/since.go:172-176` already consumes;
- one sentence saying that the weight those tokens carry against the window is
  not established, with the citation.

**The one path to a real figure**, and it is per-user rather than universal: run
`quota.Samples` + `quota.Compare` over *this user's own* ledger
(`internal/quota/quota.go:151-278`). On a large enough corpus with enough
counter movement on both arms it becomes `Reportable` and the ratio is theirs,
measured, on their account. Until then the JSON field carries
`{"status": "NOT MEASURED", "why": <Comparison.Why verbatim>}`. `Compare`
already writes a better refusal string than a UI would.

### A4. Observed lockouts and the wait each named — **measured, with a stated undercount**

**Formula.** A lockout is a record where `Status == 429`, `Refusal == ""`, and
`Quota["retry-after"]` is present. Parse it by the two-form rule already written
at `internal/proxy/retry.go:121-133`: an integer is seconds; otherwise an HTTP
date.

**Two details that are easy to get wrong.**

*The date form must be resolved against the record's own timestamp*, not
`time.Now()`. `retryAfter` takes `now` as a parameter because it runs live; an
offline reader passing the current clock would report a wildly negative wait for
every historical lockout in the ledger.

*The `Refusal` term is belt-and-braces and should stay.* Replay's own local
refusals do set a `Retry-After` on the wire to the client
(`internal/proxy/server.go:1084-1090`, and the breaker at
`internal/proxy/guards.go:323-326`), but `recordRefusal`
(`internal/proxy/server.go:1068-1081`) writes a record with **no `Quota` map at
all**, so today the header test alone is sufficient. Keeping the `Refusal == ""`
term means a future guard that starts copying headers cannot silently turn
Replay's own back-pressure into "the provider locked you out" — which is the
worst false positive this command could produce.

**Error bar: a structural undercount, and it must be printed.** 429 is retryable
(`internal/proxy/guards.go:405-407`) and `retryTransport` waits out the
provider's own `Retry-After` when it fits under the cap
(`internal/proxy/retry.go:107-118`). The ledger records only the **final**
attempt's headers (`internal/proxy/server.go:594-595`). A 429 the proxy absorbed
therefore leaves `Retries > 0`, `Status 200`, and no `retry-after` anywhere. The
honest report is two non-additive lines: *N lockouts reached the client*, and
separately *M requests were retried, some of which were 429s the proxy absorbed;
their waits are not in the ledger*. Summing them would be inventing events.

**Rate.** Any per-hour figure states the span it was observed over, per
`cmd/replay/burn.go:41-53`. A corpus spanning an afternoon says nothing about a
month.

### A5. Retry pressure — **measured**

`sum(rec.Retries)` and the count of records with `Retries > 0`. Cheap, always
available with a ledger, and a leading indicator of nothing on its own: retries
cover 5xx too. Report it as instrument context for A4, never as a quota signal.

### A6. Header coverage — **measured**, and mandatory

`coverage = records carrying any quota header / records read`, with the family
named (`anthropic-ratelimit-unified`, `anthropic-ratelimit-tokens`, or
`x-ratelimit`).

This is the denominator for everything else and it must be on the screen. The
trial at `internal/analysis/predictor.go:5-7` saw the unified family on 142
responses; `docs/evidence/subscription-allowance-2026-09-09.md` limit 6 records
that most responses carried no header at all. A utilisation figure derived from
one record in four thousand is a true reading and a thin one, and the reader
cannot tell without the count.

The family name is also the most useful single thing the command can tell a
confused user: the unified family means a subscription seat, the `tokens-*`
family means an API key, and which one they have changes which of these rows can
ever be filled.

### A7. Deliberately not computed

- **Tokens remaining in the window.** Requires the window's size in tokens. It
  is in no observed header and nowhere in this repository. NOT MEASURED. What
  would supply it: a `-limit` companion to the unified family, or a lockout
  observed at a known cumulative token count on a quiet account.
- **Any dollar figure.** The command must not print one. That is the entire
  premise: `replay cost` already says the dollars are list price for somebody
  else.
- **A single "you are fine / at risk" verdict.** `internal/quota/quota.go:11-14`
  refuses to put thresholds and risk levels in the package for a stated reason,
  and a colour in the CLI is the same claim with a different renderer. The
  80% red at `cmd/replay/statusline.go:94-96` is defensible for a status line
  because it colours a *provider-reported percentage of a provider-defined
  window* and asserts nothing about tokens. Reuse that bound and nothing beyond it.

---

## B. Can time-to-lockout be forecast?

**Yes. `predictor.go` does not forbid it, and the repository already contains
the forecaster.**

### The distinction the predictor is actually drawing

`internal/analysis/predictor.go:5-11` concludes: *"A 140,000-token full-prefix
re-lay does not move the counter by one tick, so the header can be a denominator
and never a trigger."* That is a statement about **resolution at request
scale**. One request cannot be decided on, because one request is smaller than
one tick. It rules out a per-request gate. It says nothing about the second
derivative of a level over an afternoon.

Forecasting asks a different question and needs none of the machinery the null
denied. `internal/quota/forecast.go:9-14` states it exactly: *"A forecast needs
no token model, because the provider reports the LEVEL directly — what is needed
is its rate of change over TIME, and two readings an hour apart give that
without any conversion."*

### The confound cuts one way

This is the design's central claim and it is worth stating flatly. The
account-wide nature of the counter is **fatal to attribution and correct for
projection**.

The titration voided a calibration because other traffic on the same account
moved the counter. Attribution needs to subtract that traffic and cannot. But
the thing that locks the user out *is* the account-wide window — including the
interactive session in their other terminal, their phone, and their teammate on
an Enterprise seat. A projection of the account-wide level is a projection of
the actual event the user fears. The other traffic is not noise here. It is part
of the signal.

So: **the share question (A3) is refused and the forecast question is
answerable, for the same reason.**

### The model

Implemented at `internal/quota/forecast.go:56-107`.

- Segment the readings at the last reset — any reading below its predecessor
  (`:66-73`). Fitting across a rollover yields a negative rate and a forecast of
  "never" on a seat that is filling normally.
- `rate = (util_last − util_first) / span_hours` over the post-reset segment.
- `time_to_limit = (1 − util_last) / rate`.

It refuses on four conditions, each with a written reason: fewer than three
readings since the reset (`minReadings`, `:28`), a rise below the counter's own
0.01 resolution (`:83-87` — *"that is quantization, not consumption"*), a
non-positive rate, and a zero span.

### Its assumptions, stated

1. **Constant rate.** False. Agent work is bursty, and a secant across the whole
   post-reset span is a mean. It under-forecasts inside a burst and
   over-forecasts while idle — and the burst is precisely when the user asks.
   *Proposed extension, not currently in `forecast.go`:* also compute the rate
   over the most recent adjacent interval that clears the resolution floor, and
   report the pair as a band rather than a point.
2. **The window resets stepwise.** The reset detector is a fall in utilisation
   (`:66-73`). If the 5h window is a *rolling* window that decays continuously
   rather than resetting, the detector fires on ordinary decay and the fit
   restarts constantly. **Which of the two it is has not been established
   anywhere in this repository.** It is the second-most consequential unknown
   after A3's accounting rule.
3. **One account, one series.** Correct by construction here, and the same
   reason `Samples` pairs globally by timestamp rather than per session
   (`internal/quota/quota.go:145-150`).

### Its error bar

Quantization dominates. Each endpoint carries ±0.01, so the relative error on
the rate is about `0.02 / rise`. Over a rise of 0.05 — five steps, which is what
the whole 30-lane trial produced — that is **±40%**, before any assumption above
is questioned. A point estimate of "3h27m" from that is a lie told to three
significant figures.

Therefore: the forecast is printed as a **band**, from the rate at
`rise + 0.02` and at `rise − 0.02`, and it declines to print at all when the
band is wider than the window it is forecasting. That last bound is a *declared*
threshold in the sense `internal/quota/quota.go:49-56` means it — *"it bounds
embarrassment, not error"* — and the design should say so rather than pretend it
was measured.

### What would make it materially better

- **A finer header.** Three decimals turns a ±40% band into ±4%. Nothing else on
  this list comes close. It costs the provider one character.
- **A reset instant in the unified family.** Without it (A2), the proxy path can
  say "filling at 0.11/hour" and cannot say whether the window resets before the
  projection lands. That is the difference between a forecast and a warning.
- **A client-side counter — which Replay already has, honestly bounded.** The
  ledger knows tokens per minute exactly. It cannot be converted into window
  units, because the titration returned null and that conversion is the null.
  But it can be used *without* a conversion, as a shape prior: compare the
  current minute's token rate against the post-reset mean, and if it is far
  above, mark the forecast unstable rather than adjusting it. That uses an
  uncalibrated signal for the one thing it can support — "the assumption behind
  this number is currently false" — and for nothing more.
- **More readings, which is free.** The status line ticks on a 300ms debounce.
  Three readings is a floor and a thin one (`:24-28`); an hour of work could
  supply hundreds. See the ring-buffer change in D.

---

## C. The output

House style taken from `cmd/replay/budget.go`, `cmd/replay/since.go` and
`cmd/replay/burn.go`: two-space indent, a headline sentence, a labelled column
per figure, prose where prose is more honest than a number, and a `ran` trailer.

### C1. Human view

Values below are **layout placeholders in angle brackets**; the only literal
figures are the two that are cited. A real run prints only what its own ledger
supports.

```text
  Your window, and when it stops you

    binding window    5h              [measured]  named by the provider:
                                                  unified-representative-claim
    utilisation       <0.NN>          [measured]  +/-0.005, read <t> ago
    resets in         <NhNNm>         [reported]  status line, resets_at <local>
    filling at        <0.NN>/hour     [estimated] <k> readings over <span> since reset
    runs out in       <NhNNm - NhNNm> [estimated] band at the counter's own 0.01 resolution

    lockouts          <n>             [measured]  provider 429s that reached the client
      <time>   waited <n>s
    retried           <m> request(s)  [measured]  not addable to the line above: some were
                                                  429s the proxy absorbed, and their waits
                                                  are not in the ledger

    header coverage   <n> of <N> responses carried a rate-limit header   [measured]
    header family     anthropic-ratelimit-unified  (a subscription seat)

  What your cache breaks cost this window is NOT MEASURED.

    <n> tokens were re-billed in this ledger [structural, from usage counts], but
    the weight a re-billed token carries against the window is not established:
    a matched-arm titration moved 3.09M tokens through the counter and it did not
    advance one step. The counter is account-wide, so movement cannot be attributed
    to your requests either. docs/evidence/quota-titration-2026-09-06.md

  ran   replay quota
```

Rules the sketch is enforcing:

- **Every figure carries a tier.** `[measured]` = read off the wire by the proxy.
  `[reported]` = handed to us by the client, second-hand but not derived.
  `[estimated]` = fitted, per ADR-0002. `[structural]` = arithmetic over counts.
- **The refusal gets more space than the figures**, because it is the question
  the user came with and the honest answer is long.
- **No colour above 80%**, and none below it (A7).
- **No total.** Nothing on this screen adds to anything else on it — the
  precedent is `cmd/replay/burn.go:83-87`, whose columns are deliberately not
  addable and which says so on screen.

### C2. `--json`

Schema `replay.quota.v1`, versioned for the same reason `BudgetSchema`
(`cmd/replay/budget.go:52-55`) is: a file outlives the binary that wrote it.

```json
{
  "schema": "replay.quota.v1",
  "generated": "2026-09-09T12:00:00Z",
  "coverage": {
    "records": 0, "with_headers": 0,
    "family": "anthropic-ratelimit-unified",
    "span": {"from": null, "to": null},
    "tier": "measured"
  },
  "windows": [{
    "name": "5h",
    "binding": true,
    "binding_source": "anthropic-ratelimit-unified-representative-claim",
    "utilization": {"value": 0.0, "resolution": 0.01, "read_at": null,
                    "age_seconds": 0, "source": "proxy", "tier": "measured"},
    "resets_at": {"status": "NOT MEASURED",
                  "why": "no reset field is present in the unified header family; resets_at comes from the status line and none is stored"},
    "forecast": {"status": "NOT MEASURED", "why": "<Projection.Why, verbatim>"}
  }],
  "lockouts": {
    "count": 0, "tier": "measured", "events": [],
    "undercount": {"reason": "the proxy retries 429s and records only the final attempt's headers",
                   "retried_requests": 0}
  },
  "avoidable": {
    "tokens": 0, "share_of_tokens": 0.0, "tier": "structural",
    "window_share": {"status": "NOT MEASURED", "why": "<Comparison.Why, verbatim>"}
  }
}
```

Three contracts:

1. **A quantity that cannot be computed is an object with `status` and `why`,
   never `0` and never `null`.** A consumer that sums `utilization` across
   machines must be unable to accidentally add an absence. This is the JSON form
   of the rule `cmd/replay/quotastore.go:74-80` states for the reader: *"neither
   is a zero: a zeroed window would tell a reader they have a full allowance,
   which is the most expensive possible way to be wrong here."*
2. **`why` strings are the estimator's own, verbatim.** `Projection.Why` and
   `Comparison.Why` are already better written than a UI layer would manage
   (`internal/quota/quota.go:263-272`, `internal/quota/forecast.go:76-88`).
   Passing them through also means the refusal cannot drift from the condition
   that produced it.
3. **Exit code 0 even with nothing to report.** This is a report, not a gate.
   `replay cost-gate` is the gate. A CI job that fails because a developer has
   no quota headers has failed for the wrong reason.

---

## D. Degradation: the common case is no headers at all

Most users will run this with an empty `Quota` on every record, because the
headers only arrive through `replay serve`, and because `saveQuota` currently has
no production caller so the status-line path is empty on every machine too.

The command therefore has to be *good at having nothing*. Five states, each with
its own message and its own next step:

**0 — no ledger.** Follow `cmd/replay/budget.go:110-115` almost verbatim, with
the right noun:

> NOT MEASURED: no ledger found under ~/.replay/ledger. `replay serve` writes
> one, and the provider's rate-limit headers only arrive through it.

**1 — a ledger, and zero quota headers.** The important case, and the one where
the wrong output is actively harmful:

> NOT MEASURED: <N> requests read, none carried a rate-limit header.
>
> This is not "you have used none of your quota". It is no reading at all. The
> provider sends these headers on some responses and not others, and they only
> reach this ledger through `replay serve`.
>
> To get them:  export ANTHROPIC_BASE_URL=`http://127.0.0.1:4000`  and run
> `replay serve`. Then come back after some work.

The phrasing must never permit "0%", "0 of your quota", an empty progress bar, or
a full one. The distinction between *no reading* and *a reading of zero* is the
whole reason `statusInput.RateLimits` is a pointer
(`cmd/replay/statusline.go:48-51`) and the whole reason `since.go:151-155`
refuses to print `$0.00` for a quiet window.

**2 — headers present, forecast not supported.** Surface `Projection.Why`
unchanged. It already distinguishes "fewer than three readings since the reset"
from "the counter moved 0.004 over 40 minutes, below its own 0.01 resolution:
that is quantization, not consumption". Both are useful and they suggest
different next steps.

**3 — status-line reading only, no proxy.** Utilisation and reset time are
available; the forecast is not, because `saveQuota` keeps exactly one reading
and `Forecast` needs at least three. **The change this design asks for:** make
the store an append-only, size-capped JSONL of `{util, window, resets_at, at}`
beside the ledger rather than a single overwritten object, and call it from
`runStatusline`. That is a small, self-contained change that turns the most
widely available signal on the machine — a reading every 300ms of active work,
free, no proxy — into the input `Forecast` was written for. It is also the only
path by which a user who never runs `replay serve` gets a forecast at all.

**4 — a reading exists but its window has reset.** Reuse the wording already
written at `cmd/replay/mcp.go:274-277`: *"A reading exists but every window in it
has already reset, so it describes a period that is over."* Never age it forward.

---

## E. Figures this design needs and the repository does not have

Named, so that none of them gets quietly filled in with a plausible value.

1. **The allowance weight of a cache read versus a cache write.** The titration
   is null; `Compare` refuses. This is limit 5 of
   `docs/evidence/subscription-allowance-2026-09-09.md` and the single parameter
   everything in A3 is sensitive to.
2. **The window's size in tokens.** Absent from every observed header. Without
   it, "tokens remaining" cannot exist.
3. **Whether the 5h window resets stepwise or rolls continuously.** Decides
   whether `Forecast`'s reset detector is correct or pathological (B, assumption 2).
4. **The counter's true resolution.** Two decimals is what the wire carries; that
   is a floor on the resolution, not a measurement of it.
5. **Whether the limit is weighted by model.** Raised as an untested route in the
   titration's "what would be needed" and never tested.
6. **Whether Team and Enterprise seats return the same header family.** All trial
   readings were `serviceTier: standard`, one account, one machine — limit 4 of
   the allowance evidence. This command is specified for Enterprise and nothing
   in the repository establishes that Enterprise looks like this.
7. **The resolution of the status line's `used_percentage`.** It arrives as a
   float; whether it carries more than integer precision is unestablished, which
   makes the quantization term for the status-line forecast unknown rather than
   0.01.

---

## F. The three questions that decide this

Following the form of [P3-VISIBILITY-OPTIONS](P3-VISIBILITY-OPTIONS.md): each is
checkable, and each changes what gets built.

1. **Does the status-line ring buffer produce a forecast that survives contact
   with a real day's work?** Cheap to answer: persist the readings, run
   `Forecast` at the end of a working day, and compare its 10:00 projection
   against what actually happened by 15:00. If the burst assumption breaks it,
   the band in B is not enough and the model needs a recency weight.
2. **Does `Compare` ever become `Reportable` on a heavy user's own ledger?** If
   yes, A3 turns from a refusal into a per-user measured figure and this command
   gains its headline. If no user's corpus can carry it, the refusal is
   permanent and should be documented as such rather than as "not yet".
3. **Is the unified 5h window step-reset or rolling?** One quiet account, one
   reading per minute across a reset boundary, no traffic. It costs nothing and
   it decides whether `forecast.go:66-73` is a correctness feature or a bug.

---

## Method and provenance

Every claim about existing behaviour cites `file:line` against the working tree
at `~/Development/Replay-clean` on 2026-09-09. Trial figures are quoted from
`internal/analysis/predictor.go:5-11` and
`docs/evidence/quota-titration-2026-09-06.md`; corpus figures from
`docs/evidence/subscription-allowance-2026-09-09.md`. The header-name
enumeration in A2 is a full-tree scan for `anthropic-ratelimit-[a-z0-9-]*` across
source, tests, fixtures and documentation. **No new provider requests were made
for this document, and no production code was written or modified.**

---

[Documentation index](../README.md) · [Repository README](../../README.md)
