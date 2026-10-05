# Register: cache-TTL setting, written before any write

**Status: REGISTERED, NOT STARTED, FROZEN (third amendment, 2026-10-05 UTC,
before any observation). The hash of this file as frozen is recorded in
`ttl-register-2026-10-05.sha256` beside it; the block order derived from
that hash is `ttl-schedule-2026-10-05.json`; blocks are recorded in
`ttl-blocks-2026-10-05.jsonl`. Any edit to this file after freezing
invalidates the schedule, and `scripts/ttl-block` refuses to run against
an edited register until the schedule is re-derived and refrozen.**

## Third amendment: the treatment mechanism and its provenance

**Why.** The second amendment named `advise --apply --yes` as a way to write
the treatment. Executable falsification on 2026-10-05 UTC, against this
corpus: `replay advise ~/.claude/projects --apply --json` proposes
`(unset) -> 1h`, the automatic default here, and would never write 5m. So
that path cannot start the treatment arm, and a hand edit leaves no record.
Neither the hypotheses, the metrics, the analysis, the sample sizes nor the
eligibility rules change below; only how an arm is put in place and how
that is recorded.

**Treatment mechanism.** `go run ./scripts/ttl-block start -block N -arm
treatment` writes `"promptCacheTtl": "5m"` into the Claude Code settings
file (`$CLAUDE_CONFIG_DIR/settings.json`, else `~/.claude/settings.json`),
after copying the existing file to a timestamped backup beside it and
keeping every other key. The settings file is the mechanism, not an
environment variable, because the client reads the file on every launch
whatever shell or editor starts it, and the environment variable would take
precedence over the file if one were set (the resolver is read in
[ttl-control-boundary-2026-10-05.md](ttl-control-boundary-2026-10-05.md)).
A researcher reproduces it from the frozen register, the schedule and the
tool at the commit named in the block log.

**Control mechanism.** `go run ./scripts/ttl-block start -block N -arm
control` removes the key, so the client's automatic default applies: 1h on
a subscription not in overage, which is what this machine ran before the
study. Writing an explicit `1h` would hold 1h through overage and would
not be the baseline the hypothesis names, so the key is removed.

**Block start.** A block starts at the instant the tool writes the file. The
tool refuses to start a block whose arm the schedule does not name, a block
already started, or any block while an earlier one is open.

**Provenance, three states kept apart.** Requested: the arm and value the
tool was asked for (`requested_value`). Configured: the value read back
from the file after the write (`configured_value`), with the file's SHA-256
and the backup path. Verified actual state: per session, the TTL the
client billed its cache writes at, in the transcript's own usage breakdown
(`ephemeral_5m_input_tokens`, `ephemeral_1h_input_tokens`), which is the
arm-membership rule already in force below. The tool records the first two;
the third is observed later from the transcripts and is not the tool's to
claim. A `BLOCK_STARTED` record carries `state_change: VERIFIED` only when
the read-back equals the request; otherwise it is `BLOCK_NOT_STARTED`,
`UNVERIFIED`, the tool exits non-zero, and the block has not begun.

**Block end.** `go run ./scripts/ttl-block end -block N` records the end
instant and whether the settings file still hashes to what the block
started with (`span_intact`). A changed file does not void the record; the
sessions after the change fall under the span rule below and are excluded.

**Valid exposed block.** `BLOCK_STARTED` with `VERIFIED`, a `BLOCK_ENDED`
with `span_intact: true`, and at least one eligible session whose writes
show the block's arm. A block that fails any of the three is reported as
such and contributes no sessions; it is not re-run under a different
number.

**If treatment verification fails.** The block is recorded as not started,
the backup is restored by hand, the restore is recorded as its own event,
and the cause is written down before any further block. Two consecutive
failures to verify abort the study until the cause is understood.

**Abort conditions, executable.** A block in which treatment sessions show
cache writes the arm did not request (the rollback condition below); a
client update that changes the resolver during the study; an edit to this
register after freezing; the operator's decision to stop. Each is recorded
in the block log as it happens.

**Randomization.** The block order is a function of the frozen register:
with the register's SHA-256 digest, block 1 is the treatment when the first
byte is even and the control when it is odd, and the arms alternate. Six
blocks. The order is generated once by `go run ./scripts/ttl-block
schedule` and frozen as `ttl-schedule-2026-10-05.json`, which carries the
register hash it was derived from. Nobody chooses the first block.

**Every record** in the block log carries the register hash and the
schedule hash it ran under, so a later reader can check that the blocks
were run against the design that was frozen.

The second amendment's text follows unchanged.

**Earlier status: REGISTERED, NOT STARTED, PREPARED (second amendment, the same
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
  (Superseded by the third amendment: the apply path proposes 1h on this
  corpus, so the treatment is written by `scripts/ttl-block` and recorded
  in the block log. The arms themselves are unchanged.)
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
