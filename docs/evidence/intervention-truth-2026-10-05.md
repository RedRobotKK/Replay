# Intervention truth, 2026-10-05

**Question.** Can Replay truthfully distinguish a recommendation from an
intent to apply, an apply attempt, an actual state change, and a measured
outcome? The cache-TTL setting is the test case, because it is the only
change Replay makes to a user's configuration itself.

## Intervention truth

**What `advise --apply` does today.** `applySettings` reads
`promptCacheTtl` from the Claude Code settings file, builds a plan from
`ttlPlan` over the calibrated lane reports, and calls `plan.write`. Without
`--yes` it prints the diff. With `--yes` it backs the file up, rewrites it
with the one key changed, and until today printed "set" when the write
call returned without error. Nothing recorded the write, and nothing read
the value back.

**Why it does not write on this machine.** Traced CLI to filesystem: the
plan is refused by the predictor before any write. `chooseTTLWithCoverage`
finds 5m cheaper in total and 1h cheaper on the costliest tenth of
sessions, and its rule is that a split verdict is not a finding. The reason
printed is "no single setting is right for this corpus". Classification:
intentional non-writing behaviour of the predictor. Not a dry run, not a
missing configuration, not a path or permission problem, not a dead path:
the write path is reachable and tested, and the refusal is the product
rule working. On this machine the truthful record of the intervention is
therefore a refusal, and it is now written as one.

**The state-transition contract shipped today.** One line per `--apply
--yes` in `~/.replay/interventions.jsonl`, schema `replay.intervention.v1`:

| Field | Meaning | Epistemic state |
|---|---|---|
| event | INTERVENTION_APPLIED, APPLY_ATTEMPTED, INTERVENTION_REFUSED | as it ended |
| apply_requested | always true: a dry run writes no record | fact |
| apply_attempted | false for a refusal | fact |
| prior_value | what the file held, or UNSET | read before the write |
| intended_value | the plan's value, or UNDECIDED on a refusal | decision |
| applied_value | what was written, or NONE | fact |
| actual_value | what the file holds after, read back; UNREADABLE when it could not be read | independent read |
| state_change | VERIFIED only when actual_value equals intended_value on read-back; UNVERIFIED otherwise; NOT_ATTEMPTED on a refusal | verification |
| reason | the predictor's refusal reason | decision |
| predicted_effect | the predictor's margin as a fraction | SIMULATED |
| predicted_effect_basis | SIMULATOR | basis |
| realized_effect | UNAVAILABLE | never filled by the record |
| outcome | NOT_YET_MEASURED | never filled by the record |
| quality_outcome | UNAVAILABLE | no task-outcome signal exists |
| block, model | UNAVAILABLE | no experiment exists |

The same transition reaches the machine-readable surface: the
`replay.apply.v1` entry carries `event`, `state_change`, `actual`, and
`applied` true only with `state_change: VERIFIED`; a dry run says
`NOT_REQUESTED`. Until later on 2026-10-05 it said `applied: true` from the
`--yes` flag before the write ran (M144).

The invariant: INTERVENTION_APPLIED is permitted only when the value was
read back from the file. A write that returns without error and a file
that then reads something else is APPLY_ATTEMPTED, UNVERIFIED, and the
reader is told "the change was not confirmed".

## Predictor truth

Two predictors, two populations, two weightings. Measured 2026-10-05.

| Component | Decision | Objective | Inputs | Population | Weighting | Output meaning |
|---|---|---|---|---|---|---|
| `learn` (ttl family) | select ttl-5m, 28.0% | mean per-session share of as-run effective tokens avoided, candidate against as-run | simulated candidate vs observed usage per session | every transcript under the projects root, 3,216 files, of which 2,223 are nested sub-agent transcripts; calibrated sessions, 30% holdout, ties excluded | session-equal | "on sessions where it differs, 5m avoids 28% of that session's as-run effective tokens, on average" |
| `advise --apply` (`ttlPlan`) | refuse | token-weighted margin between simulated 5m and simulated 1h, with a top-decile veto and a coverage floor | the two simulated policies per session, no as-run reference | same files, sessions at or above 95% match rate with both policies priced | by effective tokens | "the cheaper TTL avoids this share of the dearer TTL's total, unless the costliest tenth disagrees" |

What the two populations actually ran (OBSERVED from `cache_creation` in
usage): the 981 main-thread transcripts wrote 1-hour caches in 976, mixed
in 3, 5-minute in 2; all 2,216 nested sub-agent transcripts with cache
writes wrote 5-minute caches only. The client runs two policies at once
with the setting unset.

Consequences, all CALCULATED from the simulator on this machine:

- On main-thread sessions only, simulated 1h is within 10% of as-run on 898
  of 900: the simulator reproduces the policy the client ran.
- Over the recursive population, simulated 1h is more than 10% from as-run
  on 1,694 of 2,647 sessions, almost all sub-agents that ran 5m. The
  "ttl-1h minus 35%" is sub-agents scored under a policy they never ran.
- Per session, the two predictors never disagree: 2,300 of 2,300 sessions
  scored by both prefer the same TTL, and 2,271 prefer 5m.
- Bill-weighted over the sessions `ttlPlan` scores: simulated 5m is 0.2%
  below as-run, simulated 1h 11.8% above it; on the costliest tenth, 1h is
  3.4% cheaper than 5m.

So the 28.0% is not a saving on the bill. It is a session-weighted mean
over the sessions where the candidate differs from as-run, and the sessions
that carry the bill prefer the other direction. Whether `promptCacheTtl`
governs sub-agent requests at all is UNAVAILABLE from the record. The
relationship between the two predictors is explained, not resolved:
UNRESOLVED, pending a decision on which objective the intervention serves
and on the setting's scope.

## Registry and purge truth

The store registry (`homeStores`) describes `~/.replay`. `apply` modifies
`~/.claude/settings.json`, which is outside it; the registry now names the
intervention log as a store no retention window removes. `replay purge
<dir> --older-than` enumerates `.jsonl` files directly inside whatever
directory it is given and removes those older than the window; it does not
consult the registry. Pointing it at `~/.replay` itself would remove
`measurements.jsonl` and `interventions.jsonl`. Pre-existing, unrelated to
the contract, recorded here and left alone.

## Test truth

RED, proven before GREEN: no record (5 tests), no margin carried, no store
registered; then no refusal record, no read-back, no explicit states (4
tests). GREEN: the targeted suite passes. Mutation: 8 hand mutations on the
first half and 10 on the contract, all killed, each restored; M141 frozen
and killed through the harness. The margin's definition is pinned by a
hand-computable test: (loser minus winner) over loser, never negative, a
tie refused before any margin exists, token-weighted. The production path is
proven to record under the user's home by a test of `settingsPlan`.

## Experiment readiness: NOT YET JUSTIFIED (superseded the same day)

Later on 2026-10-05 the control boundary was established from the
client's own resolver and both predictors were corrected and tested; see
[ttl-control-boundary-2026-10-05.md](ttl-control-boundary-2026-10-05.md)
and the second amendment of the register. The verdict below is kept as it
was written.

1. The intervention can be performed: yes, by `plan.write`, and verified by
   read-back: yes, as of today.
2. The predictor semantics are understood: yes, and they say the
   registered hypothesis (28% lower cost per new input token) is a
   session-weighted artifact over a mixed population; the bill-weighted
   simulated effect of the setting is about zero, split by session size.
3. The apply path refuses on this machine, correctly.
4. Therefore running the interleaved experiment now would test a
   prediction nobody holds at the bill level. The register is amended to
   say so and stays NOT STARTED.

Decision among the five: A, fix the predictor semantics first. Not C.

The final question: can Replay now say "I recommended X, predicted Y,
changed state X, verified it, and measured what happened next"? It can say
the first four truthfully, with the record to prove each, and on this
machine the truthful first four are "recommended nothing, predicted a
split, refused, verified nothing changed". The missing link is the fifth,
measurement, and before it the predictor's objective must be one the
measurement can test.
