# Two commercial hypotheses, pre-registered before either was run

**Written 2026-09-24, before any workload ran, before any reading was
interpreted, and before any offer was published. Results are appended below the
protocol, never edited into it.**

The research closure of 2026-09-24 froze seven statuses. Two of them are
commercial and both are UNKNOWN: market need and willingness to pay. This file
pre-registers the two smallest experiments that could move either, and fixes the
thresholds now so that a result cannot be reclassified after it is seen.

The two are independent. Neither outcome licenses a conclusion about the other.

> ## Provenance boundary, established 2026-09-27
>
> **Historical preregistration status: NOT VERIFIABLE from surviving evidence.**
> **Current document identity: pinned from 2026-09-27 onward.**
>
> The claim above about when this document was written is preserved exactly as
> it was made. It is **not** corroborated, and this note records why, so that a
> reader meets the gap deliberately rather than by accident.
>
> Three sources were checked and none establishes that the protocol existed in
> frozen form before results:
>
> - **Filesystem times say nothing.** This file's mtime is 2026-09-25 22:05,
>   and fourteen other files share that same minute, including files authored
>   by a different session. That timestamp records when this worktree was
>   materialised, not when text was written.
> - **Git says nothing.** The file has never been tracked, on any branch, so
>   there is no commit history to order.
> - **Session transcripts show repeated modification.** Fourteen operations modified this path between 2026-09-25 04:30 and 2026-09-27 04:33, including one whole-file rewrite and six edits after it. The earliest write found in any transcript is 2026-09-25 04:30, which is **after** the date in the heading above. Operation counts are
>   a heuristic over transcript payloads and are not exact authorship history;
>   what they establish is that the document changed repeatedly over days, not
>   which bytes changed.
>
> **What this pin does and does not do.** It fixes the identity of the text as
> it stands today, and nothing before today. It is forward-only. It does **not**
> retroactively establish preregistration status, and the document should not be
> cited as a preregistration on the strength of it.
>
> The protocol below is unmodified. Nothing was edited to make this note tidier.

## Protocol freeze

**Frozen at commit `6871022`, 2026-09-24.** The thresholds below are the
contract. A result that does not fit them is recorded against them, and they are
not moved to fit it.

## Experiment 1: does a cache break burn a subscription quota?

### Hypothesis

A re-billed token consumes measurably more of a Claude subscription's
rate-limit window than the cached read it replaced.

This re-opens [quota titration, 2026-09-06](quota-titration-2026-09-06.md),
which returned a null and recorded the instrument as insufficient rather than
the answer as settled.

### Why it is asked again

It is the precondition for an entire commercial framing. Most people who run
Replay hold a flat seat and spend no dollars, so every dollar figure the tool
prints is addressed to somebody else. If a broken cache costs a subscriber
window, Replay has an economic claim for that population. If it does not,
Replay has an accuracy claim and no economic one.

### The observable, named exactly

Claude Code hands a statusline script a `rate_limits` object. The observable is
these fields and no others:

| Field | Role |
|---|---|
| `rate_limits.five_hour.used_percentage` | primary observable |
| `rate_limits.seven_day.used_percentage` | secondary observable |
| `rate_limits.spend_limit.used_percentage` | recorded, never an outcome |
| `rate_limits.*.resets_at` | block validity: a reading taken after a reset is a new window and is never differenced against one taken before it |

**One step is 1% of a window**, and that is the quantisation floor of the entire
experiment.

**Instrument note, recorded 2026-09-24 from the first two live readings, and not
a result.** The 2026-09-06 file describes this field as carrying two decimals and
quotes it as `0.25`, a fraction. The payload observed today carries
`"five_hour":{"used_percentage":1}` and `"seven_day":{"used_percentage":26}`:
whole numbers, on a percentage scale rather than a fractional one. Both readings
imply the same one percentage point quantum, so the floor above is unchanged.
But the two representations differ by a factor of 100, and **anything that
differences a reading taken before this boundary against one taken after it is
wrong by 100x**. Readings are therefore stored raw, with their timestamp, and the
scale is resolved at analysis time rather than assumed at capture time. Two
readings is not enough to establish which representation is current, only enough
to establish that the question has to be asked.

Nothing else is an observable. In particular, **billing figures are not evidence
of quota consumption**, and a token being re-billed is not itself evidence that
it cost quota. Those are the two inferences this experiment exists to avoid
making.

### Scale identity: a pre-specified analysis rule

Added 2026-09-25, after the instrument note above and before any reading was
differenced. It exists because an API representation change must never be
allowed to present itself as a treatment effect.

**Raw quota values are immutable observations.** They are stored exactly as the
payload delivered them, with their timestamp, and are never rewritten in place.

**Any conversion to a normalised scale must be performed by a transformation
specified in advance and determined independently of the experimental outcome.**
A transformation chosen after seeing which direction it moves the result is not
a transformation, it is the result.

**Mixed-scale observations may not be differenced until scale identity is
established.**

The procedure for establishing it, fixed now:

1. Readings are partitioned into spans bounded by client version changes.
2. **A single reading strictly greater than 1.0 establishes the percentage scale
   for its whole span**, because a fraction of a window cannot exceed 1.0.
3. A span containing no reading above 1.0 has ambiguous scale, because `0.26`
   is consistent with both 26% and 0.26%.
4. Scale is assigned per span, with the specific reading that established it
   recorded alongside the assignment.

Applied to what exists today: the 2026-09-25 span carries `26`, so it is
**established as a percentage scale**. The 2026-09-06 file quotes `0.20`, `0.21`
and `0.25` and nothing above 1.0, so that span is **ambiguous** and its readings
may not be differenced against current ones. That is the rule working, and it
costs the earlier data rather than the later.

**This is not an exclusion rule and may not be used as one.** Observations whose
scale is unresolved are held, not dropped, and are reported with their count. If
scale identity is never established for a span, the affected period is reported
as unusable for differencing, with its size stated, and contributes to
INCONCLUSIVE. It never contributes to POSITIVE or NULL, and it is never silently
removed because it was inconvenient.

The measurement-validity question this raises is recorded as open: **can the
experiment establish a stable, version-independent interpretation of quota
utilisation before utilisation deltas are used as evidence?** If the answer turns
out to be no, that is itself the finding, and it is a finding about the
instrument rather than about the hypothesis.

### Starting state, verified 2026-09-24

- `replay ceiling --subscription` exists and reports tokens rather than dollars.
- `quotaline.go` already parses all three windows.
- `FS2` in `flatseat_test.go` fails the build if `replay cost` tells a
  subscriber what re-billed tokens cost them. **It stays untouched until this
  experiment resolves.**
- `README.md:243` records the question as an open null in both directions.
- **Nothing on this machine persisted a rate-limit reading.** The statusline at
  `~/.claude/statusline/render.py` did not read the field, the CLI exposes no
  usage subcommand, and no file under `~/.claude` stored one. The observable did
  not exist and had to be built before either arm could run.

### Baseline measurement

**Arm A baseline.** The first 14 days of sampled readings, with no intervention
of any kind, establish the ordinary distribution of window movement per hour
during normal work. No hypothesis is tested against those 14 days. They exist to
characterise drift.

**Arm B baseline.** Before each block, 30 minutes of confirmed account idle,
sampled throughout. The idle drift is recorded. **Any movement greater than one
step during the idle baseline voids that block**, because the account-wide
counter is then demonstrably carrying traffic the experiment did not produce.

This is the exact confound that voided the 2026-09-06 calibration, where a
475,000 token step was attributed to a probe while an interactive session ran
throughout. That calibration was withdrawn. Passive attribution of an
account-wide counter to individual requests does not work.

### Cold versus warm workload, defined

Both arms are produced by `claude -p` against a fixed prompt corpus: same model,
same prefix size, back to back, alternating order between blocks.

| Arm | Construction | What the provider does |
|---|---|---|
| COLD, cache-write | a fresh invocation with no resumable session | writes a ~159,000 token prefix at the 1.25x multiplier |
| WARM, cache-read | the same prompt via `--resume` against the session the cold arm just created | reads a ~159,000 token prefix at the 0.1x multiplier |

Same model, same size, same shape. Only warm against cold differs, so the
confound is removed by construction rather than by adjustment afterwards.

**Arm assignment rule, carried forward from the 2026-09-06 fix:** a request
joins an arm only when one kind of cached token holds 90% or more of its cached
total. Mixed requests are excluded from both arms. A request that is 40% write
and 60% read carries no clean contrast, and averaging it in is how a null gets
manufactured. In the 2026-09-06 data, 14 of 21 requests carried both kinds, and
the then-shipped rule scored every one of them as a write.

### Estimand

Steps moved per million tokens in the cold arm, divided by steps moved per
million tokens in the warm arm.

The metered bill charges a write at 1.25x and a read at 0.1x, a ratio of **12.5**.

| Observed ratio | Meaning |
|---|---|
| near 1.0 | the window counts tokens flat, and a break costs no extra quota |
| near 12.5 | the window is weighted like the bill, and a break costs quota |

The 2026-09-06 run left both equally consistent with the evidence. That is the
gap this is trying to close.

### Repetitions

**Arm B: five paired blocks**, cold and warm, order alternated between blocks so
that any monotonic drift across the session cancels rather than accumulating.

**A block is informative only if both of its arms move at least 10 steps.** At
the 2026-09-06 haiku volumes that is well over 30M tokens per arm, which is why
arm B carries the model-weighting pilot as a precondition rather than being
attempted directly.

Five blocks is a floor, not a target reached by stopping early. **Stopping once
the result looks clean is not permitted.** All five run, or the file reports how
many ran and why it halted.

### Acceptable measurement noise

- **Quantisation.** One step is 1% of a window. Any effect smaller than one step
  is invisible to this instrument and is reported as invisible, never as absent.
- **Idle drift.** More than one step during the 30-minute idle baseline voids the
  block. Voided blocks are discarded and re-run, never repaired.
- **Separation requirement.** The difference between arms must exceed **three
  times the standard deviation of the idle drift** measured across all blocks.
  Below that the arms are not distinguishable, and the result is INCONCLUSIVE
  regardless of which direction the point estimate leans.
- **Interval.** A bootstrap interval over the five block ratios. The thresholds
  below are stated in terms of what that interval excludes.

### Thresholds, fixed now

**POSITIVE.** The interval excludes 1.0, the separation requirement is met, and
no block was voided by idle drift.

Then document the observed relationship, the workload, the measurement, which
window moved, the limitations, and the reproduction steps. **Do not convert it
into a savings claim in the same document.** A demonstrated quota relationship is
a physical finding. What it is worth to anyone is a separate question this
experiment does not ask.

**NULL.** Both arms cleared the 10-step floor, the interval contains 1.0 and
excludes 12.5, and the separation requirement is met.

**INCONCLUSIVE.** The interval contains both 1.0 and 12.5; or the arms failed the
10-step floor; or the separation requirement was not met; or blocks were voided
and could not be re-run. A ratio of 1.0 and a ratio of 12.5 both remaining
consistent is exactly the 2026-09-06 outcome, and reproducing it is a legitimate
result rather than a failure to get one.

**The commercial attractiveness of POSITIVE is not evidence for it.** NULL is
cheaper to live with and harder to publish. That asymmetry is what these
thresholds exist to resist.

### The exact economic claim killed if NULL

On NULL these are removed permanently and may not be restated in any surface,
document, post, listing or conversation:

1. "Replay saves your Max quota."
2. "Replay gives you back X% of your five-hour or seven-day window."
3. Any dollar-equivalent framing of a subscription window, including "worth $N
   of tokens" applied to a flat seat.
4. "A broken cache costs a subscriber more than a cache read does."
5. Any claim that a subscriber recovers capacity, throughput, sessions per
   window, or work per window by reducing re-billed tokens.
6. Any use of the metered 12.5x multiplier as though it described a
   subscription.

The single surviving sentence is:

> A re-billed token is context the transcript shows was billed twice. Replay has
> not established that it consumes subscription quota.

`FS2` already enforces that boundary in the product. On NULL the boundary becomes
permanent rather than provisional, and the hypothesis does not reopen without new
evidence.

### Arm A: observational, zero marginal quota

1. A sampler appends one JSONL record per distinct reading to a local file,
   wrapped so that any failure cannot affect the status line. It sends nothing
   anywhere and costs no tokens, because the statusline already runs on every
   render.
2. It accumulates during ordinary work.
3. Each window movement is paired with the token flow Replay reads from the same
   period's transcripts, split into re-billed and ordinary.
4. Movement is regressed on the two token quantities separately.

**Arm A is immune to the account-wide confound** that voided the 2026-09-06
calibration, because it observes the whole account rather than attributing a
counter to individual requests.

**Arm A's own limitation, stated before it runs:** re-billed tokens are roughly
4% of the total on this corpus, so the two regressors are strongly collinear.
Separating their coefficients needs both volume and variance in the re-billed
fraction. **Minimum before any analysis: 30 days, at least 200 distinct readings,
and a re-billed fraction spanning at least a 2x range across the sampled
periods.** Short of that, the result is INCONCLUSIVE by collinearity and is
reported as such. It may never separate them.

### Arm B: interventional, not authorised, not run

The 2026-09-06 file costed this honestly: moving the counter 20 steps per arm at
haiku volumes needs well over 30M tokens per arm, a large share of a five-hour
window, risking locking the operator out of a working account in order to measure
lockout. Its verdict was that this **is not a reasonable experiment to run on a
working account**, and nothing since has changed that.

Arm B requires three things this session does not have:

- **A quiet account.** The largest error term in 2026-09-06 was an interactive
  Claude Code session running throughout. It is removable only by idling the
  account, and an agent session is itself the confound.
- **A quota budget the operator has agreed to spend**, with lockout accepted as a
  possible cost.
- **A model-weighting pilot first.** If the window is weighted by model, opus
  moves the counter far faster per token than haiku and the titration becomes
  affordable. If it is not, arm B is unaffordable on any working account and
  Experiment 1 is permanently blocked at this instrument. The pilot is far
  cheaper than the titration and settles whether the titration is worth running
  at all.

**Arm B is not executed in this session and nothing here claims it was.**

### Result

Arm A: instrument installed 2026-09-24, accumulating. Two live readings captured,
used only to characterise the field's representation as noted above. **No reading
has been interpreted against the hypothesis and no analysis has been run.** The
14-day baseline has not started to elapse.

Arm B: NOT RUN. Blocked on a quiet account, an authorised quota budget, and the
model-weighting pilot.

### Conclusion

Not yet reached. The 2026-09-06 null stands unchanged, and every claim it governs
stays exactly where it is.

## Experiment 2: will anyone pay for the forensic capability?

### Hypothesis

A real person or organisation will pay a fixed price for a scoped reconstruction
of what their agent transcripts show, delivered as a written report.

### What is explicitly not the signal

A download. A site visit. A profile impression. An inquiry. A conversation. A
positive reply. A proposal view. Someone saying it is interesting. A free CLI
run. Nine downloads of v0.6.1 is not a demand reading and was never treated as
one.

**Payment is the signal. Nothing upstream of payment is the signal.**

### Starting state, verified 2026-09-24

- The Upwork freelancer profile is live and reachable from the replay.doctor
  footer since 2026-09-12.
- It reads **$90.00/hr** under the title "Database and Data Architecture Review,
  Technical Due Diligence". The description predates Replay entirely.
- **Zero jobs, zero feedback, zero earnings.** The funnel starts empty, which is
  the cleanest possible baseline and also the hardest sell.

### The price, pre-registered

**The engagement price is a fixed $2,400. It is not hourly, and it is not $90/hr.**

This is registered explicitly because the profile's existing $90/hr rate would
otherwise become the experimental price by default, leaving a moving variable in
the middle of the experiment. The hourly rate is what the profile advertises for
a different service. The forensic engagement is a fixed-price product with a
fixed scope, and the two must not be allowed to blur.

Derivation, stated so it can be disagreed with: roughly 25 hours of scoped work
at the profile's existing nominal rate, rounded. It may not be changed after the
test window opens, because a price that moves during the test measures nothing.

A different price is a different experiment and gets its own registration.

### What the channel actually pays, read 2026-09-25

Checked before freezing the price, against fixed-price postings on the same
channel the experiment will use.

| Segment | n | median | mean | range |
|---|---|---|---|---|
| Cloud and infrastructure audit | 10 | $425 | $1,296 | $100 to $7,500 |
| Technical audit and due diligence | 4 | $2,250 | $2,500 | $500 to $5,000 |
| Pooled | 14 | $500 | $1,640 | $100 to $7,500 |

**$2,400 sits above 79% of the observed budgets.**

The two closest postings by intent, where a buyer is paying specifically to learn
where spend went:

- **$100** to identify what was causing an increase in AWS and RDS costs.
- **$250** for an enterprise cloud procurement advisor, posted by a company
  describing itself as scaling Bedrock Claude usage to $100k to $500k per month.

A search for fixed-price work naming LLM API cost analysis, token usage audit or
optimization returned **one posting**, a $40 chatbot build matching on keywords
only. **The category this offer sells into is not visibly present on the
channel**, which is a demand observation in its own right and is recorded here
rather than discovered later.

For context on the adjacent professional category: FinOps consulting engagements
typically target 20% to 40% addressable waste over four to eight weeks. Replay
measures 4.18% re-billed on the operator's own corpus. The established comparable
promises an order of magnitude more recoverable spend, whether or not it delivers
it.

**These are posted budgets, which are what buyers open with rather than what
sellers close at.** They bound the channel from below and are not a ceiling.

### The price and channel confound, recorded before launch

The figures above do not show that $2,400 is the wrong price. They show that
**$2,400 is an ordinary professional-services number and this channel clears
around $425**, which are two different markets.

The consequence is stated now rather than discovered in the result:

> **A zero at $2,400 on Upwork cannot distinguish a wrong price from a wrong
> channel from absent demand. All three are confounded in this configuration.**

The price is frozen at $2,400 anyway, deliberately. Dropping to the channel
median would likely produce a sale and would test whether somebody will buy a
cheap thing, which is a weak willingness-to-pay reading sitting next to the $100
and $250 comparables that appear to be the real market. **A cheap sale would not
move the UNKNOWN this experiment exists to move.**

What this costs: NULL becomes substantially less informative, and the honest
report on NULL names all three candidate causes without ranking them. That is
accepted as the price of testing a real number rather than a comfortable one.

### The deliverable, exactly

A written report, delivered as a document, containing:

1. **What the transcripts establish happened** over the period supplied.
2. **What changed between the two dates the buyer names**, stated as differences
   in observed quantities.
3. **Causes where the evidence supports them**, named as causes only where the
   evidence names them, and otherwise listed as candidates the evidence does not
   separate.
4. **An explicit separation of what the evidence establishes from what it
   cannot**, as its own section. That separation is the product, not a caveat
   attached to it.
5. **The commands and the corpus state** used to produce every figure, so the
   buyer can reproduce or dispute any of it.

### Input required from the buyer

- Agent transcript files for the period in question: Claude Code or Codex session
  directories, supplied as an archive.
- Two dates defining the comparison window.
- A statement of what they believe changed, recorded before the analysis, so the
  report can say where it agrees and where it does not.

**If the transcripts do not cover the window, the engagement does not start.**
That is stated up front rather than discovered halfway through.

### Turnaround

**Five working days** from receipt of a complete corpus and both dates. The clock
starts on receipt of the input, not on payment.

### What is promised, and what is not

Promised: the report above, the reproduction steps, and the separation of
established from unestablished.

**Not promised, and not to appear in any listing, message or conversation:**
savings, quota recovery, cost reduction, productivity improvement,
vendor-independent analysis, deterministic reconstruction, exhaustive
investigation, a causal explanation for an aggregate bill change, or any outcome
Replay has not measured. Every one of those was either falsified or left UNKNOWN
by the 2026-09-24 research closure.

The offer may ask: **what changed between these two dates, and what evidence
explains the observed changes?**

It may not ask: **why did your bill change?** unless the evidence in that
specific engagement establishes the causal link.

### Test window

**Start:** the date the offer is published. Publication requires the operator's
authorisation and has not happened. The start date is recorded here when it
occurs, and is never backdated.

**End:** 42 days after the start date.

**The duration is fixed at six weeks in advance**, so that a disappointing result
cannot be extended until it improves and an encouraging one cannot be cut short
to preserve it.

### What counts as a paid engagement

All four conditions together:

1. Money has actually transferred, cleared, and is not refunded.
2. It was paid for the forensic deliverable described above: not for adjacent
   consulting, not for an hourly arrangement, not for a different service.
3. The buyer is not the operator, not RedRobot, and not anyone with a personal
   relationship to either.
4. It was agreed at the pre-registered price, or at a price the buyer proposed,
   which is recorded as such.

Anything short of all four is recorded at whatever funnel stage it actually
reached, and is not counted as a sale.

### What constitutes repeat demand

**A second paid engagement, by the same buyer, commissioned after the first was
delivered, and paid for separately.**

Not a scope extension of the first. Not a positive comment. Not a statement of
intent to buy again. Not a referral, which is a distinct signal recorded
separately.

### Funnel, recorded as seven separate counts

```text
impression -> inquiry -> conversation -> proposal
           -> PAID ENGAGEMENT -> completed -> repeat
```

These are never summed and never collapsed into "traction". Each is reported as
its own integer.

### Thresholds, fixed now

**POSITIVE.** At least one engagement meeting all four conditions, paid and
completed within the window. Record the buyer type, the route in, the price
actually paid, and what they asked for rather than what was offered. **One paying
buyer settles willingness to pay at n=1 and settles nothing about volume.**

**NULL for this configuration.** Zero paid engagements across the 42 days. The
permitted statement is exactly:

> This GTM configuration generated zero paid engagements during the test period.

It does **not** license: no market exists, nobody needs this, Replay has no
value, or forensic analysis is the wrong product. Buyer, channel, positioning,
price, trust, timing and distribution are all uncontrolled here, and any one of
them alone explains a zero.

**One purchase without a return** is recorded as: one-time forensic demand
demonstrated, repeat demand UNKNOWN. **That is a result, not a failure.**

### Limitations, stated before the test

- n will be small. A zero and a one are both nearly uninformative about volume.
- The account has no history, so the seller is being read as unproven, and the
  offer is not the only variable under test.
- One channel, **and price is confounded with it**. A zero is a statement about
  Upwork at $2,400, not about demand in general and not about the price alone.
  The channel's observed median for comparable audits is $425, so the offer is
  priced roughly 5x above what this venue clears at, by choice.
- The category is not visibly present on the channel at all, so the offer is
  also being read by buyers who are not searching for it.
- The seller is the author of the tool, which is the weakest available evidence
  position for any claim about the tool's value.
- The price is a guess. It is registered so that it is a fixed guess rather than
  a drifting one.

### Result

Not launched. The offer is frozen above. Publication awaits the operator's
authorisation, because it puts a price on a public surface.

### Conclusion

Not yet reached.

## Dogfooding

Running Replay on the operator's own Max seat continues, because it is cheap and
produces the concrete material the offer needs.

It is **proof of capability and source material for the offer. It is not evidence
of willingness to pay**, and self-generated usage is never a proxy for customer
demand. Capability was already DEMONSTRATED before this file, so publishing more
of it answers a question that is already answered.

## Discipline

Every threshold above was written before the instrument produced a reading and
before the offer was published. If a result arrives that does not fit them, the
result is recorded against these thresholds, and the thresholds are not moved.
