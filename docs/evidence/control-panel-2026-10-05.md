# Control-thesis panel, 2026-10-05

**Questions.** (A) Is "Replay as an AI workload control plane" supported by
the evidence as of today? (B) Should the next unit close the loop on the one
intervention Replay performs itself, the cache-TTL settings write: record
the application and measure the realized effect? (C) Should the
experimental learn, trial, graduate proxy loop be promoted so that
graduation changes behaviour, before it has run on live traffic?

**Evidence put to the panel.** The control-surface matrix in
[control-matrix-2026-10-05.md](control-matrix-2026-10-05.md): nineteen
in-path proxy controls, all off by default but three; no routing, no model
or host change, rate-limit headers captured and read by nothing that
decides; the one closed loop in code with `Graduated` unread; `learn`
selecting ttl-5m on this corpus (755 sessions, 28.0%, held-out 27.8%);
`advise --apply --yes` writing the setting with no record and no
measurement; waste incidence near zero here; the advisor share bound and
its positive control; the 2026-09-29 finding that no task-outcome signal
exists. Simulated from that evidence alone, twelve roles.

## Vote

| Role | A | B | C |
|---|---|---|---|
| Agent architecture | NOT SUPPORTED | MODIFY | DEFER |
| Inference and caching | PARTIAL | PROCEED | DEFER |
| Inference economics | NOT SUPPORTED | MODIFY | DEFER |
| Developer tools | PARTIAL | MODIFY | DEFER |
| Observability | NOT SUPPORTED | PROCEED | DEFER |
| Security and enterprise | PARTIAL | MODIFY | REJECT |
| HCI | PARTIAL | MODIFY | DEFER |
| Productivity research | PARTIAL | MODIFY | DEFER |
| Product-market fit | NOT SUPPORTED | MODIFY | DEFER |
| CFO and FinOps | NOT SUPPORTED | MODIFY | DEFER |
| Skeptical buyer | NOT SUPPORTED | DEFER | REJECT |
| Red team | NOT SUPPORTED | MODIFY | REJECT |

A: NOT SUPPORTED 7, PARTIAL 5, SUPPORTED 0. B: MODIFY 9, PROCEED 2, DEFER 1.
C: DEFER 9, REJECT 3.

## Consensus

**(A)** No role supports the thesis. What the evidence licenses: Replay
observes and diagnoses AI coding sessions from their own records, and
offers in-path guards (all but three off by default) plus one
machine-actionable client-setting write. Observe and diagnose are shipped.
Optimize is unlicensed vocabulary while no task-outcome signal exists.
Route and control do not exist in an executing form. Preserve and recover
was rejected three times. Verify improvement has never run. The minority of
five holds PARTIAL on the strength of tested in-path mechanisms: a control
surface, not a plane.

**(B)** Proceed only under conditions, in this order: a blocking pre-check
of whether the client's behaviour with the setting unset already matches a
5-minute TTL, in which case the write is a no-op and there is nothing to
measure; a register written to disk before any write (hypothesis, metric,
arms, minimum n, stopping rule, falsifier); an interleaved design rather
than one changepoint (at least 6 randomised blocks, at least 30 sessions
per arm, at least 2 repositories), with "after" defined as a session whose
whole span carried the value; confounds named in the output; a mandatory
positive control on the realized-saving computation; and the null published
with the same prominence as a positive. The red team's five attacks (vanity
loop, no-op, no blinding, continuity substrate returning, unbounded
collection) were accepted as conditions, not as a veto. The skeptical buyer
dissents: a one-operator before/after is not procurement evidence.

**(C)** Unanimous against promotion as asked, for two independent reasons:
the loop cannot be exercised here (no API spend, 18 ledger records), and
every context-edit candidate on this corpus was rejected, so graduation has
nothing to reach. The supported alternative is a reader of
`TrialReport.Graduated` that reports and exits, closing the observability
gap and leaving the act gap open and labelled EXPERIMENTAL. Three roles hold
REJECT: acting on graduation needs a reviewed change-management design.

## The blocking pre-check, measured the same day

OBSERVED on 981 sessions with a cache-write breakdown in their usage: 976
wrote 1-hour caches only, 3 mixed, 2 wrote 5-minute caches only; 89.5% of
all cache-write tokens were 1-hour writes, all under client 2.1 with
`promptCacheTtl` unset. The write to 5m is therefore not a no-op, and B is
not rejected on that ground.

A second fact the register must carry: `learn` scores the ttl-1h candidate
at minus 35.1% against the as-run baseline, and the as-run baseline on this
corpus mostly ran 1-hour writes. The simulator and the measurement disagree
by a third on the policy the client actually ran. The as-run figure is
OBSERVED (provider usage summed); every candidate figure, the 28.0%
included, is SIMULATED. Whichever way the realized measurement lands, the
first thing it tests is the simulator, not the setting, and the register
says so.

## Must not be claimed (binding on B)

That Replay caused a measured difference; "will save", "expected",
"forecast" or any future tense; generalisation beyond this operator,
machine or repositories; that work got better, faster or of higher quality;
a point estimate without its interval and arm sizes; invoice dollars (list
prices only, calculated); that 28.0% was validated if realized lands inside
its interval (consistency is not validation); that the 27.8% holdout is a
replication (same procedure, same corpus); that a null means the setting
does nothing (the write may be a no-op); control-plane, routing or
optimization vocabulary anywhere in the surface.

## Label wording for B, if it proceeds

At write time: "`promptCacheTtl=5m` applied by Replay at <instant> (prior
value: unset; backup: <path>). Calculated on the 755 sessions used for
selection: 28.0% lower cost per new input token (interval 27.4 to 28.7;
held-out split 27.8%), list prices. Realized effect after this instant: not
yet measured. Effect on task outcome: unavailable."

After measurement: "Observed: sessions after <instant> (n=__) versus
sessions before (n=755), cost per new input token at list prices: __%
difference (interval __ to __). Uncontrolled before/after on one operator;
not attributed to the setting. Calculated figure for comparison: 28.0%.
Effect on task outcome: unavailable."

## Conditions that would change the verdict

A to PARTIAL or SUPPORTED: a shipped reader of rate-limit headers that
gates a decision; any executing destination or model change; guards on by
default with a recorded reason. C to PROCEED: the loop run on live traffic
by an operator who is not the author, with at least two arms, a graduation
that fires and a revert exercised in production, plus a context-edit
candidate positive at n of at least 5. B to REJECT: evidence that unset
already yields 5-minute behaviour.
