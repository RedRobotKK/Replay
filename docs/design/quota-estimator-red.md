# Red team: the quota-window claim

**Claim under attack:** *"Replay shows you how much of your quota window you have
used, and how much of it went to content you had already sent — so you stop
getting cut off mid-project."*

**Verdict:** the first clause ships as a passthrough of a number Claude Code
already computes. The second clause does not ship, cannot be computed from
anything this repository has measured, and its supporting arithmetic divides by a
number this repository publicly retracted three days before the arithmetic was
written. Ranked below by expected damage.

Every finding cites `file:line` or a command and its output. Where a criticism is
answerable, the answer that would satisfy me is stated.

---

---

## R0 — the self-retraction inverts itself, and keeps the retracted half

**Filed after the fact.** While this review was in progress,
`docs/evidence/subscription-allowance-2026-09-09.md` gained a
"RETRACTED IN PART" banner (`:3-37`). Most of it is right, and it independently
confirms R5 — *"zero records carrying any `anthropic-ratelimit-*` header,
ever … The utilization reading exists **only as prose**"*. `:26-29` concedes
the product claim outright: *"any claim that re-billed tokens are known to
consume the allowance … is not merely imprecise; **it is unestablished**. The
honest status is NOT MEASURED."*

But `:21-24` gets the retraction backwards:

> **What survives.** The re-billed token count, 3,778,706, is corroborated
> independently in `lane-isolation-2026-09-06.md` … The arithmetic is sound.
> Its multiplicand is not evidence.

`lane-isolation-2026-09-06.md` does not corroborate 3,778,706. It **is the
retraction of it** — see R1. The document retracts the multiplicand (the 0.05
utilization movement, which nothing in the repository has ever challenged) and
preserves the multiplier (3,778,706, which the cited document explicitly voids at
`lane-isolation-2026-09-06.md:38-41`: *"its `broken` labels and its **deficit
figures** are themselves products of the session-wide comparison"*, with
3,317,247 of it named "Forged by the session-wide compare" at `:33`).

The banner cites the retraction document by name, in the same paragraph, and
states the opposite of what it says. Both terms of the product are unsound, for
two different and independently documented reasons, and the correction rescues
neither.

Two further problems with the banner as written:

1. **The body still asserts the retracted claim.** `:60` — *"It turns out to be
   real"*. `:82-84` — the section heading is **"The finding"**, followed by *"Re-billed
   tokens consume the 5h allowance."* `:99-101` — *"Roughly 0.45 of a full 5-hour
   Max window, spent re-sending content that had already been sent."* A reader
   who lands on the anchor, or a quoter who lifts the bolded headline, gets the
   retracted number with no banner attached. A partial retraction that leaves the
   assertive prose intact is a retraction the internet will not honour.
2. **R2, R3, R7, R8 and R9 are untouched by it.** The banner retracts the
   utilization input. It does not address the wrong denominator (R2), the unit
   that resets (R3), the one-endpoint error bar (R7), whether the counter is
   denominated in tokens at all (R8), or the untested linearity (R9). Even a
   correct utilization reading would not rescue the arithmetic.

**What would satisfy me:** swap which half is retracted, or retract both; and
strike the assertive sentences at `:60`, `:82-84` and `:99-101` rather than
banner them.

## Tier 1 — thread-defining

Five findings. Any one of them, posted as a top comment with the line numbers
attached, ends the discussion of the headline.

### R1. The numerator is a retracted number, and the retraction is in this repo

*Still live after the R0 banner, which explicitly preserves this number as "corroborated".*

`docs/evidence/subscription-allowance-2026-09-09.md:60` performs the load-bearing
division:

```text
33,778,322 / 3,778,706 = 8.94x the measured range
```

`3,778,706` is the deficit total of the 2026-09-06 30-lane trial. That trial's
ledger was retracted the same day, in this repository, in
`docs/evidence/lane-isolation-2026-09-06.md`:

- `lane-isolation-2026-09-06.md:15` — the published figure was "3,734,134 of
  3,778,706 re-billed tokens, **98.8%**".
- `lane-isolation-2026-09-06.md:31-33` — lane-correct re-read: 3 real events,
  416,887 deficit tokens. **"Forged by the session-wide compare | 31 |
  3,317,247"** — 87.8% of the numerator names events that did not happen.
- `lane-isolation-2026-09-06.md:38-41` — **"Both figures are retracted."** and,
  decisively: *"its `broken` labels and its **deficit figures** are themselves
  products of the session-wide comparison."* The 3,778,706 is not a survivor of
  the retraction; it is the thing retracted.

The repository applied that correction everywhere except one file.
`internal/proxy/preflight.go:22-34` carries it in full — *"That was the
instrument, not the world … 31 of those 34 events never happened."*
`README.md:220-228` carries it. `internal/analysis/predictor.go:5-11` does not,
and still reads:

```go
// Measured 2026-09-06 across a 30-lane randomized trial: 142 responses carried
// anthropic-ratelimit-unified-*, and 3,778,706 re-billed tokens moved the 5h
// utilization figure from 0.10 to 0.15.
```

`subscription-allowance-2026-09-09.md:107-108` names its source explicitly:
*"Trial figures are quoted from the header comment of
`internal/analysis/predictor.go`."* The 09-09 document sourced its numerator from
the single location in the repository where the 09-06 correction had not landed,
and it did so with `docs/evidence/lane-isolation-2026-09-06.md` sitting one
directory entry away and listed in `docs/evidence/README.md`.

Git makes the ordering explicit:

```text
$ git log --oneline --date=iso --pretty='%h %ad %s' -S"3,778,706" --all
65cf481 2026-09-06 10:18:08 -0700  Warn before a changed prefix is re-billed...   <- figure introduced
0f63d3a 2026-09-06 11:33:56 -0700  Evidence: lane isolation, and the headline
                                   retracted the same day                          <- figure retracted
```

Seventy-five minutes. Three days later it became a headline.

The contamination is not confined to one document. `docs/evidence/qm-budget-2026-09-08.md:36`
also cites `internal/analysis/predictor.go` as the authority for "Re-billed
tokens still consume the usage allowance". Two published documents rest on the
same retracted comment.

**What would satisfy me:** delete or annotate the `predictor.go:5-11` comment,
withdraw `subscription-allowance-2026-09-09.md`, and correct the `qm-budget`
citation. Re-derive from the corrected instrument if you want a figure at all
(`lane-isolation-2026-09-06.md:52` — 336,060 re-billed of 7,996,448 total).

### R2. The denominator is the wrong quantity, and it is off by roughly 24x

The trial's utilization moved 0.10 → 0.15. The document attributes that entire
movement to the re-billed subset and divides by it alone
(`subscription-allowance-2026-09-09.md:60`).

A rate limit is charged against everything the account sends, not against the
wasteful fraction of it. The correct denominator is the trial's **total** prompt
tokens. That total is stated nowhere in `predictor.go`, nowhere in
`subscription-allowance-2026-09-09.md`, and is not recoverable from any retained
artifact (see R5).

The only trial in the repository that reports both quantities gives the shape:
`lane-isolation-2026-09-06.md:51-53` — 336,060 re-billed against 7,996,448 total,
**4.2%**. Attributing 100% of counter movement to a 4.2% subset overstates the
per-token rate by roughly 24x. Applying that correction alone:

```text
0.45 windows / 23.8  =  0.019 windows
```

Under 2% of one five-hour window, across the entire corpus. The headline is a
factor-of-24 arithmetic error independent of R1, and the two compound.

**What would satisfy me:** state the trial's total prompt tokens and divide by
that. If the total is unrecoverable, the figure is unrecoverable.

### R3. The unit resets, so the headline is denominated in a currency that does not accumulate

"Roughly 0.45 of a full 5-hour Max window" (`subscription-allowance-2026-09-09.md:63`)
reads as *half a window you could get back*. It is not. A 5h window resets every
five hours; utilization is not a stock, and fractions of it from different windows
cannot be summed.

The corpus spans:

```text
$ find ~/.claude/projects -name '*.jsonl' -exec stat -f '%Sm' -t '%Y-%m-%d' {} \; | sort -u
2026-08-12 ... 2026-09-09
```

28 days = 672 hours = **134.4 five-hour windows**. Even taking the uncorrected
headline at face value:

```text
0.45 / 134.4 = 0.0033  =  0.33% of any given window
```

One third of one tick of the 0.01-resolution counter that measured it. With R2
applied it is 0.014% — four orders of magnitude below the instrument. The claim's
own promise, *"so you stop getting cut off mid-project"*, requires a per-window
effect. The per-window effect is below the noise floor of the only instrument
that has ever observed it, which `predictor.go:10-11` states in its own words:
*"A 140,000-token full-prefix re-lay does not move the counter by one tick."*

The document's limit #3 (`:79-82`) gestures at this but the headline at `:63`
still states the single-window form, and the headline is what gets quoted.
Limit #3 also says the corpus spans "months"; it spans four weeks.

**What would satisfy me:** never state the figure as a fraction of one window.
State it per window, with the count of windows, and let the reader see 0.33%.

### R4. The one controlled experiment in the repository contradicts the figure by at least 4x, and the repository already said so

`docs/evidence/quota-titration-2026-09-06.md` is the only matched-arm experiment
here. Its result (`:20-27`): 1,012,229 write tokens + 2,078,382 read tokens =
3.09M tokens, 5h utilization **0.25 → 0.25**, **0 steps**.

Three mutually exclusive step sizes now exist in this repository:

| Step size | Source | Status |
|---|---|---|
| ~475,000 tok/step | 4 probe requests | **Void** — `quota-titration-2026-09-06.md:45`: *"Any figure derived from that 475,000 calibration should be treated as void."* |
| ~755,741 tok/step | 3,778,706 / 5, `predictor.go:6-9` | **The one the headline uses.** Never written up as evidence. |
| >3,090,611 tok/step | `quota-titration-2026-09-06.md:20-27` | Lower bound from the only controlled run |

The headline picks the middle value — the only one that was never published as
evidence and the only one contradicted by the controlled experiment, by a factor
of at least 4.1x.

Worse, the titration names the exact mechanism that produces the middle value and
declares it invalid (`:38-43`): *"The counter is ACCOUNT-WIDE, and the same
account was running an interactive Claude Code session throughout … passive
attribution of an account-wide counter to individual requests does not work."*

The 30-lane trial ran on the same account, same machine, same day. That day was
not quiet:

```text
$ find ~/.claude/projects -name '*.jsonl' -exec stat -f '%Sm' -t '%Y-%m-%d' {} \; | sort | uniq -c | grep 09-06
 127 2026-09-06
```

127 transcript files carry a 2026-09-06 mtime. The five steps are shared with
unmeasured background traffic by exactly the mechanism the titration used to void
its predecessor. `predictor.go` repeats the voided method and the 09-09 document
promotes its output to a headline.

**What would satisfy me:** the paired trial the document itself specifies
(`subscription-allowance-2026-09-09.md:95-100`), on a confirmed-idle account, with
the idle confirmed rather than assumed.

### R5. The 142 readings do not exist anywhere, so nothing here is auditable — and on the author's own machine the feature has never captured a single header

The task asked how many local ledger records carry a `quota` field. The answer is
zero, of seventeen:

```text
$ cd ~/.replay/ledger && for f in *.jsonl; do
    echo "$f total=$(wc -l < $f) with_quota=$(grep -c '"quota"' $f)"; done
351d4e3d-...jsonl total=2  with_quota=0
7022b9f2-...jsonl total=10 with_quota=0
f11756ad-...jsonl total=2  with_quota=0
f79cae6d-...jsonl total=2  with_quota=0
session-91d4ea...jsonl total=1 with_quota=0
$ cat *.jsonl | grep -c quota
0
```

Not "few". The string `quota` appears zero times in the entire persistent ledger.
This is not a schema problem — all 17 records are `schema: 2`, current — nor a
failed-request problem: sixteen are HTTP 200 responses on `/v1/messages` to
`claude-opus-5`, each with a `request_id` and a populated `usage` object. The
capture path (`internal/proxy/server.go:595`, `rec.Quota = quotaFrom(tap.Header())`)
landed at commit `b76390e`, 2026-09-05 23:18. The sixteen successes are dated
2026-09-04 22:45. The single record after the capture code landed is a **401**
(2026-09-06 13:42), which carries no quota headers.

So the author's own machine holds no example of the feature working, and the
trial data is gone:

```text
$ grep -rl "anthropic-ratelimit-unified" ~/.replay ~/Development/Replay-clean 2>/dev/null
(no output)
```

Zero hits across the ledger, the repository, every fixture, and every `testdata`
directory. The 142 readings and the 0.10 → 0.15 movement exist as prose in a Go
comment and nowhere else. No third party — and no future maintainer — can
recompute, re-audit, or falsify the number the headline rests on. Combined with
R1, the position is: an unauditable figure, whose only written provenance is a
comment the repository itself corrected elsewhere.

**Now conceded.** The R0 banner confirms this independently
(`subscription-allowance-2026-09-09.md:9-14`): a census found *"zero records
carrying any `anthropic-ratelimit-*` header, ever — the key is absent, not empty,
and `retry-after` has never been observed once."*

**What would satisfy me:** publish the trial ledger, redacted through
`replay corpus`, alongside the document — or state plainly that no such ledger
exists and that the reading is unverifiable.

---

## Tier 2 — a careful reader will note these

### R6. The unmeasured parameter is not a sensitivity, it is the whole claim

`internal/quota/quota.go:4-7` states the open question: the provider bills a write
at 1.25x and a read at 0.1x, "Whether the SUBSCRIPTION limit counts the same way
is undocumented." `subscription-allowance-2026-09-09.md:86-90` concedes it is
**"not measured"** and is "the single question that would most change this
figure."

It changes it to zero. "Content you had already sent" is only *waste* to the
extent a re-send costs more against the allowance than a cache read of the same
content would have. The recoverable amount is `(1 − read_rate/write_rate)` of the
figure:

| Allowance ratio | Recoverable share | Headline becomes |
|---|---|---|
| 12.5 (bills like the invoice) | 92% | 0.41 windows |
| 3.0 | 67% | 0.30 windows |
| 1.0 (allowance is flat in tokens) | **0%** | **0.00 windows** |

`quota-titration-2026-09-06.md:88` is unambiguous about which of these is
supported: *"a ratio near 12.5 and a ratio near 1.0 remain equally consistent
with what has been measured."* The claim is a coin flip on an unmeasured binary,
and one face of the coin is zero.

**What would satisfy me:** measure the ratio, or drop the second clause.

### R7. The error bar is computed on one endpoint and is too narrow

`subscription-allowance-2026-09-09.md:76-78` derives ±0.01 on a 0.05 total, giving
0.04–0.06 and the quoted 0.36–0.54.

A difference of two quantized readings carries the quantization of **both**. An
observed 0.10 and an observed 0.15 are each ±1 count, so the true movement is
0.03–0.07 — ±40%, not ±20%. The honest range is **0.27–0.63**, and that is before
any of R1 through R4. A range whose endpoints differ by 2.3x is not a measurement
being reported; it is a direction.

The repository already knows this failure mode. `internal/quota/quota.go:218-231`
records that the first estimator "was not noisy, it was degenerate" precisely
because "the counter moves in whole hundredths." The 09-09 document applies
one-endpoint quantization to the same counter.

**What would satisfy me:** ±2 counts, and quote 0.27–0.63 if you quote anything.

### R8. Nobody has established that the denominator is tokens

`anthropic-ratelimit-unified-5h-utilization` is *unified*, and it ships with a
companion `-representative-claim` naming which window binds
(`internal/quota/quota.go:33-43`). Neither name suggests a token count. The whole
calculation assumes utilization is linear in prompt tokens, and nothing in the
repository tests that.

Three specific ways the unit could differ, none excluded:

1. **Output tokens.** `internal/quota/quota.go:187` computes the denominator as
   `u.Input + u.CacheCreation + u.CacheRead` — output tokens are excluded
   entirely. If the allowance weights output (as every published price schedule
   does, at 5x input or more), a whole term is missing.
2. **Model weighting.** `quota-titration-2026-09-06.md:81-82` flags this itself:
   *"If the limit is weighted by model - untested."* The titration ran haiku. The
   corpus being extrapolated to is predominantly opus (all 16 successful ledger
   records are `claude-opus-5`). Extrapolating a haiku-derived rate onto an opus
   corpus is not a linearity assumption, it is a unit change.
3. **Requests, or a composite.** "Unified" across dimensions is the ordinary
   reading of the name, and a composite need not be linear in any single input.

If the unit is not tokens, R2's division is not merely mis-scaled — it is
undefined.

**What would satisfy me:** vary one input at a time against a fixed everything
else and show the counter responds proportionally to prompt tokens.

### R9. Linearity across 8.94x is asserted, and the mechanism argues against it

`subscription-allowance-2026-09-09.md:71-75` concedes linearity is "assumed and
was not tested." It is worse than untested: the two candidate mechanisms both
break it.

- **Sub-linear.** If cache reads are discounted against the allowance (R6), then
  as a corpus grows the cached share grows with it — long sessions read more and
  write proportionally less — and the marginal cost per token falls. The document
  names this at `:73-75` and does not carry it through.
- **Super-linear.** Rate limits commonly apply burst penalties or tighten as a
  window fills. Under any such rule the tail of a window costs more than the head,
  and a rate fitted at 0.10–0.15 utilization understates behaviour at 0.80.

The claim's payload — *"so you stop getting cut off"* — is a statement about the
region **near the ceiling**, which is precisely the region where the linearity
assumption is least defensible and where no observation exists. The trial never
went above 0.15.

**What would satisfy me:** readings above 0.7 utilization, or an explicit refusal
to extrapolate past the observed range.

### R10. Removing the tokens does not return the window

Even granting every prior point, the claim's causal step is unsupported. "How much
of it went to content you had already sent" invites the reading that the quota is
recoverable. Three reasons it is not:

1. **The breaks are not Replay's to fix.** `lane-isolation-2026-09-06.md:77-88`:
   all three real breaks are MCP connector tool blocks arriving mid-session, and
   `:105-108` states the remedy — *"client-side sequencing — bind MCP tools before
   the first cached request, not after."* That is a change to the agent client,
   not a saving the observer produces.
2. **The content still had to be sent.** The counterfactual is not "zero tokens",
   it is "the same content at the cache-read rate." The recoverable delta is R6's
   unmeasured ratio, again.
3. **The tool refuses to act on it.** `internal/analysis/predictor.go:29-33`
   ships `OptInActive` defaulting false, and lines 18-20 explain: *"a refused
   request is indistinguishable from a network failure to the agent that sent
   it."* By default nothing is prevented. "So you stop getting cut off" describes
   a behaviour that is off unless the user turns it on and sets a ceiling — and
   `predictor.go:85-95` (`Straddles`) then suppresses the refusal whenever the
   ceiling lands inside a ±15% estimate band (`EstimateError`, `:63`).

**What would satisfy me:** rephrase to what is true — this is an observation, not
a saving.

### R11. n=142 is a biased sample, and the pairing logic is most fragile exactly there

`subscription-allowance-2026-09-09.md:91` — *"n = 142 responses carried the header
at all. Most responses did not."* That is offered as a modest caveat. It is a
selection problem.

`internal/quota/quota.go:151-198` pairs records globally by timestamp, and
`:165-168` breaks the chain on any record without a reading:

```go
// A request with no reading breaks the chain: pairing across it
// would silently fold two requests' spend into one sample.
havePrev = false
```

If most responses carry no header, the chain breaks constantly and the surviving
consecutive pairs are the bursts — where responses arrive close enough together
to both carry headers. Those are exactly the intervals with the most concurrent
account-wide traffic, which is R4's confound at its maximum. The sample is not
merely small; it is selected for the condition that biases it.

And the coverage gap is unexplained. Nothing in the repository establishes *why*
most responses lack the header, which means nothing establishes that the 142 are
representative of the 33.8M tokens they are being extrapolated onto.

**What would satisfy me:** report the denominator (142 of how many?) and the rule
governing which responses carry the header.

### R12. Scope: one account, one machine, one tier, one model class

`subscription-allowance-2026-09-09.md:83-85` concedes one machine, one account,
`serviceTier: standard`, and nothing generalising to Pro or Team. Add to that
list, because the document does not:

- **Model.** The titration ran haiku (`quota-titration-2026-09-06.md:30`); the
  corrected lane trial ran haiku (`lane-isolation-2026-09-06.md:112`). The corpus
  is opus. See R8.2.
- **Prompt shape.** `lane-isolation-2026-09-06.md:112` — "one synthetic fan-out
  prompt". `:113-115` — *"the real ledger sessions on hand carry zero sub-agent
  lanes, so they cannot speak to the fan-out case at all."*
- **Time.** All observations are from a single day, 2026-09-06. Rate-limit
  accounting is provider policy and changes without notice.

The claim is addressed to "Claude Pro/Max/Enterprise subscribers." The evidence
covers one Max seat on standard tier running haiku for one afternoon.

---

## Tier 3 — the claim describes a feature that is not built

Filed last because it is not a flaw in the measurement; it is a gap between the
marketing copy and the source tree. It is also the finding the author can verify
in ninety seconds, so it belongs on the record.

### R13. Clause one ships, as a passthrough of somebody else's number

`cmd/replay/statusline.go:93` calls `quotaLine`, which reads a `rate_limits`
object supplied on stdin **by Claude Code**, not measured by Replay
(`cmd/replay/quotaline.go:8-13`):

```go
// Claude Code hands a statusline script a `rate_limits` object on stdin,
// carrying used_percentage and resets_at for a five hour window, a seven day
// window, and a spend limit where the account has one.
```

Replay's genuine contribution here is real but narrow: it picks the *binding*
window rather than the first one (`quotaline.go:32-38`) and suppresses a window
whose reset has passed (`:40-45`). Both are good. Neither is "Replay shows you how
much of your quota window you have used" — Claude Code knows that number and hands
it over.

### R14. Clause two does not ship, and the code says why

The second half of the claim — attributing quota consumption to re-sent content —
is implemented nowhere. The repository states three times, independently, that it
is deliberately not implemented:

- `cmd/replay/quotaline.go:17-20`: *"this file does not try to convert between
  them: the titration that attempted to price a window in tokens returned a null
  result, and **inventing a rate here would be exactly the figure this project
  refuses to state**."*
- `cmd/replay/burn.go:243-245`: the `claude-code` surface hardcodes
  `quota: "not reported"`.
- `docs/requirements.md:406`: quota headers are *"deliberately not a metric"* and
  are kept off `/replay/status` and `/replay/metrics` *"because publishing a
  counter implies its movement means something and nothing has established what a
  movement here means."*

And the front door, `README.md:75-78`, tells the reader the opposite of the
marketing claim:

> Whether a break also burns a subscription quota the way it burns a bill is
> measured, unresolved, and [written up as null] rather than assumed in either
> direction.

The proposed announcement asserts as a shipped capability the precise figure that
the code comments, the requirements document, and the README each separately
refuse to state. The most damaging review of this claim is a `grep` of the
project's own README, and it takes one command.

---

## What is actually defensible

Stated because a red team that only destroys is half useful. All of the following
survive every objection above:

- **The instrument's honesty.** `internal/quota/quota.go:232-274` refuses to
  report a ratio and names which arm is short. `:218-231` documents a degenerate
  estimator caught by simulation before it shipped. That is better practice than
  the claim it is being used to sell.
- **The null.** `docs/evidence/quota-titration-2026-09-06.md` publishes a negative
  result and voids its own predecessor. `docs/evidence/lane-isolation-2026-09-06.md`
  retracts a headline within hours and explains that the 98.8% figure "was about
  to justify building a prefix compression subsystem." That is the strongest
  material in this repository.
- **The binding-window fix.** `quotaline.go:32-45` — reporting the window that
  binds, and treating an expired reading as no reading, are real improvements over
  displaying the first field in the payload.
- **The dollars-versus-tokens distinction.** `README.md:60-63` and the `replay
  cost` wording it quotes are careful and correct.

## Claims that can be made honestly today

1. *"Replay shows which of your rate-limit windows will stop you first, and hides
   a window that has already reset."* — `quotaline.go:32-45`. True, shipped,
   verifiable.
2. *"Replay measures how many of your prompt tokens were re-billed because a cache
   broke, and names what broke it."* — `lane-isolation-2026-09-06.md:52,63,80-84`.
   True, shipped, and the MCP-handshake finding is genuinely useful.
3. *"Whether re-billed tokens burn your subscription allowance is undocumented; we
   measured it and got a null. Here is the experiment."* — This is the honest and,
   for the intended audience, the more interesting story.

## The one sentence that must not be shipped

Any sentence quantifying how much of a quota window went to re-sent content. There
is no such measurement. The number that exists is an 8.94x extrapolation of a
retracted figure, divided by the wrong denominator, expressed in a unit that
resets, resting on a parameter whose plausible values include zero, from data that
no longer exists.
