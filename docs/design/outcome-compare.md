# `replay outcome-compare`: the comparison contract

Written 2026-10-23, before the code. It wires `outcome.Compare` (requirement OC-3), which until now no command called. It compares two arms of scored run directories by counts. It estimates no effect.

## What a comparison may claim, and what it may not

A `COMPARABLE` result means only this: each arm has at least five eligible runs of the pinned task, the control arm is not at a ceiling, and here are the counts. It is a description of the runs supplied. It is not a statistical test. It is not evidence that a difference would hold on other runs, other tasks or other machines, and it is not evidence of any saving in cost, time or tokens.

The minimum of five eligible runs per arm was decided by the owner on 2026-10-23 as a predeclared operational floor. It is not a significance threshold. Below it, the result is `INSUFFICIENT`, whatever the counts are.

## Predeclared, before any run is seen

| | |
|---|---|
| Task | The one pinned in the binary, `count-b-tests-v1`. No other task can be compared |
| Metric | PASS count over eligible runs, per arm. Nothing else is compared |
| Direction | More PASS is better. The command reports counts and does not say which arm is better |
| Baseline | The arm named by `--control` |
| Arm of a run | The `arm` field of its `run.json`. That field is supplied by whoever produced the run; `replay outcome-record` labels it `operator_attested` |
| Eligible run | One `replay outcome` scores `PASS` or `FAIL`. An `UNAVAILABLE` run is counted apart and never in n |
| Minimum | 5 eligible runs in each arm |
| Ceiling | A control arm at 0 of n or n of n PASS is refused (R10's stop rule, unchanged). A ceiling is not evidence of no effect |

## What happens to every run

Each run directory supplied is scored by the unchanged scorer and listed in the output with its verdict and reasons. It then goes to exactly one of three places:

- `control`: its arm is the `--control` label. Counted in that arm: PASS and FAIL in n, UNAVAILABLE apart.
- `treatment`: its arm is the `--treatment` label. Counted the same way.
- `excluded`, with a reason: `arm_not_compared` when the run names another arm, `arm_unknown` when no arm could be read, because the run record is missing or malformed.

Nothing is dropped silently: an excluded run is still in the output, with why. The same run directory named twice is refused before anything is scored, as in `replay outcome`.

## Not measured

Cost, wall-clock time, token use and any quality beyond the pinned task are not compared. The output says `NOT MEASURED` for each. `run.json` carries usage figures, but they are operator-attested, so they are not compared or summed.

## Command

```text
replay outcome-compare --control <arm> --treatment <arm> [--json] <run-dir...>
```

Both labels are required and must differ. It exits 0 whenever every directory was scored, whatever the status; read the status from the output, as with `replay outcome`. It reads only, and writes, runs and reaches nothing.

## Test matrix

| Case | Expected |
|---|---|
| 5 control (3 PASS, 2 FAIL), 5 treatment (4 PASS, 1 FAIL), one run of another arm, one UNAVAILABLE control | `COMPARABLE`; control 3 of 5 with 1 unavailable, treatment 4 of 5; the other arm excluded as `arm_not_compared` |
| 4 eligible control runs plus 3 UNAVAILABLE | `INSUFFICIENT`: unavailable runs do not reach the minimum |
| 5 control all FAIL | `REFUSED_CEILING` |
| A missing run directory | excluded as `arm_unknown`, listed |
| The same directory twice | refused, exit 1 |
| No `--control`, no `--treatment`, the same label for both, no run directory | refused, exit 1 |
| Output | names the minimum, says no effect is estimated, and lists cost, time and tokens as `NOT MEASURED` |
