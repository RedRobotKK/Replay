# Register: cache-TTL setting, written before any write

**Status: REGISTERED, NOT STARTED, PREPARED (second amendment, the same
day, still before any observation).** The control boundary is established
from the client's own resolver
([ttl-control-boundary-2026-10-05.md](ttl-control-boundary-2026-10-05.md)):
the setting governs main-thread requests only. The eligible population is
main-thread lanes. Both predictors are corrected and tested. The design
below is executable by a person; nothing in it starts on its own.

**Final design.**

- Treatment: `promptCacheTtl=5m` written by `advise --apply --yes` or by
  hand, with its record in `interventions.jsonl`. Control: the setting
  absent, which on this subscription resolves to 1h while not in overage.
- Unit of assignment: a block of consecutive main-thread sessions; blocks
  alternate by a coin-flip order recorded here before the first block;
  at least 6 blocks, at least 30 sessions per arm, at least 2 repositories.
- Arm membership is OBSERVED, not assumed: a treatment session must show
  5-minute cache writes only, a control session 1-hour writes only, in its
  own usage breakdown; a session showing the other arm's writes, or mixed
  writes (overage, a client update, an environment variable), is excluded
  and counted.
- Sub-agent transcripts are excluded from both arms: the setting does not
  reach them.
- Primary metric, bill-weighted: effective tokens at list prices pooled
  over the arm, divided by new input tokens pooled over the arm. Secondary,
  session-weighted: the mean per-session cost per new input token, with
  interval.
- Predictions under test, both SIMULATED on this corpus: the apply
  predictor says 1h beats 5m on the bill; `learn` says 5m lowers the
  typical session's cost by 26.3%. Under the primary metric the treatment
  is therefore predicted to cost more, and under the secondary to cost
  less. The experiment tests both numbers; it does not choose between them.
- Quality guard: QUALITY_OUTCOME=UNAVAILABLE. Error share and re-reads
  after a clear are reported per arm as the weak proxies `learn.Graduate`
  already uses, labelled as such, and decide nothing.
- Rollback: restore the backup `plan.write` made, or delete the key; the
  restore is itself recorded. Rollback condition: any block in which
  treatment sessions show cache writes the setting did not request.
- Stopping: at the pre-registered n, reported per block.
- Falsifier: a realized bill-weighted difference whose interval includes
  the prediction's sign reversal; a realized session-weighted difference
  under half of 26.3%.
- Positive control before the first report: invert the realized-difference
  computation and watch the report go red.
- Authorization: running this changes the operator's own configuration
  for the duration of the blocks and uses the operator's subscription, no
  API spend. It starts only when the operator says so.

The earlier text below is kept as written.

**Earlier status: REGISTERED, NOT STARTED, NOT YET JUSTIFIED (first
amendment).** Nothing has been written to any settings file. This register exists
so that the measurement cannot be designed after the result is known. It
follows the panel of [control-panel-2026-10-05.md](control-panel-2026-10-05.md).

**Amendment, 2026-10-05, before any observation.** The hypothesis below
rests on `learn`'s 28.0%, which [intervention-truth-2026-10-05.md](intervention-truth-2026-10-05.md)
shows is a session-weighted mean over a population that mixes main-thread
sessions (1-hour writes) with sub-agent sessions (5-minute writes). On the
simulator's own bill-weighted terms, 5m sits 0.2% below as-run and the
costliest tenth of sessions prefers 1h by 3.4%; the apply path refuses on
this machine for that reason. Before any block runs: the primary metric
must be bill-weighted (effective tokens over the arm, cost per new input
token pooled over the arm), with the session-weighted mean secondary; the
expected effect must be restated from the bill-weighted simulation, which
is about zero; sub-agent transcripts must be excluded from both arms unless
the setting is shown to govern them; and the predictor whose number is
tested must be named. Until those are settled the experiment is not
justified, and nothing starts.

## Hypothesis

Setting `promptCacheTtl=5m` in Claude Code lowers cost per new input token
at list prices on this operator's sessions, against the unset behaviour,
which on this machine is OBSERVED to be 1-hour cache writes (976 of 981
sessions; 89.5% of cache-write tokens).

Simulated figure, for comparison only: 28.0% (interval 27.4 to 28.7;
held-out split 27.8%) on 755 sessions, `replay learn`, rules
anthropic-2026-09-05. Known weakness of the simulator: it scores the
client's own 1-hour policy at minus 35.1% against the as-run measurement.

## Metric

Cost per new (uncached) input token at list prices, per session, from the
session's own usage. Reported beside it as a mechanism check: cached share,
and cache-write tokens by TTL.

## Arms and design

Interleaved, not a single changepoint. Blocks alternate between the setting
written (5m) and the setting restored from its backup (unset), in an order
fixed by coin flips recorded here before the first block. Minimum 6 blocks.
Minimum 30 sessions per arm in total, across at least 2 repositories.

A session belongs to an arm only if its start timestamp is strictly after
the block's write instant and the settings file carried the arm's value for
the session's whole span, checked by file hash at both ends. Sessions
straddling a write or restore are excluded and counted.

## Confounds, named in the output

Client version; model mix (stratify by model id; exclude or label the
sessions that switch model mid-session, 10 of 908 here); repository and
workload mix; session-length distribution; provider-side cache pricing or
TTL behaviour changes; operator awareness (no blinding is possible, said
outright); concurrent changes to other Replay guards; time of day and day
of week.

## Stopping rule

Stop at the pre-registered n (30 per arm, 6 blocks), reported per block so
no single block carries the claim. Not when the number looks good.

## Falsifier

If the realized difference is at or below zero, or its interval includes
zero, the result is published with the same prominence as a positive, and
the simulator's 28.0% is recorded as not reproduced.

## Positive control

Before the first comparison is reported, mutate the realized-difference
computation (invert its sign, or zero one arm) and confirm the report goes
red. A measurement that cannot fail is not a measurement.

## Must not be claimed

Everything in the panel's list: no causation, no future tense, no
generalisation, no quality claim, no point estimate without interval and
arm sizes, no dollars, no "validated", no "replicated" from the holdout,
no reading of a null as "the setting does nothing", no control-plane
vocabulary.

## What starts it

A person decides to run the blocks. Replay does not write the setting on
its own, and nothing in this register authorises it to.
