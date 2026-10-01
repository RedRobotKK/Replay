# R10 objective task outcome: does durable work state improve task outcomes?

**Verdict: INVALID EXPERIMENT. Stopped at Stage 0 on a preregistered ceiling
condition. The treatment arm was never run, and the question is exactly as open
as it was before.**

Protocol, substrate, oracle, runner and raw runs:
`/Users/daniel/Development/r10-trial/`, hashes in `FROZEN.sha256`.

## What was asked

> Does providing durable work state cause an agent to resolve a seeded defect
> more successfully than the corresponding control, rather than merely retrieve
> the relevant information more often?

This is **R10** from `../research/DURABLE_WORK_STATE_RESEARCH.md`, previously
**BLOCKED** for want of a benchmark with hidden ground truth. The benchmark was
built. It did not work.

## The three claims, kept separate

| | Claim | Status after this trial |
|---|---|---|
| 1 | Structured representation changes what an agent **retrieves** | **SUPPORTED at origin**, unchanged: R1, 51/60 vs 0/12, p=1.9e-08 |
| 2 | An anchor's **capability** changes retrieval | Supported at n=10, unchanged |
| 3 | Any of this causes **better work** | **STILL NOT ESTABLISHED.** This trial produced no evidence either way |

**51/60 vs 0/12 is retrieval evidence and is not an outcome result.** It was
cited for lineage only and this trial does not extend it.

## Design, frozen before any run

Information held constant, representation varied, which is R1's independent
variable carried to an outcome endpoint. Identical working copy, prompt, model,
tools, permissions and turn budget in both arms. **One file differed in content
under an identical filename**, `docs/NOTES.md`: prose in control, a structured
work-state table in treatment.

Primary endpoint was **binary task outcome from a hidden oracle**. Retrieval,
naming the defect, and whether the agent opened the notes were explicitly not
endpoints and were not scored.

The seeded defect admitted two repairs, one correct (`rate_for`, exact
millicents) and one wrong (`_legacy_rate × 100`, where rounding has already
destroyed the precision). The oracle was shown able to **fail** on the
unmodified substrate, **pass** on the correct repair, and **fail** on the naive
repair, before freezing.

## Result

| Valid control runs | PASS |
|---|---|
| 5 | **5** |

Seven runs total on `claude-haiku-4-5-20251001`, two void under invalid-run
rule 1. 36 to 51 seconds each, no infrastructure failures. Well inside the
30-run ceiling.

The protocol said: *0/5 or 5/5: STOP. INVALID EXPERIMENT.* It fired.

## Why, and this is the transferable part

**All seven runs chose the correct repair. Not one chose the naive repair the
task was built to catch.**

The substrate answered its own question. `_legacy_rate` contains a literal
`round()` visible in 25 lines; `rate_for` is named canonically and documented
"Exact"; the whole repository is two source files an agent can read
exhaustively. There was no retrieval problem to have, and a representation
effect needs one.

**R1 measured prose at 0/4. This trial measured prose at 5/5.** The
representations are comparable and the substrates are not. R1's effect lives
where the control representation fails; this substrate was built where it does
not.

## This is the second substrate in this programme to die at ceiling

A2 candidate A was rejected for the same reason, and Gate A's 80 trials
returned NOT REPLICATED because C0 and T2 both reached 100%.

**Three times now, the binding constraint has been task headroom rather than
the mechanism.** The repository's standing note that "a ceiling erases the
observable treatment difference" is the most replicated finding in this
programme, and it has now been reproduced on an outcome endpoint as well as a
retrieval one.

## Incidental, not an endpoint

**2 of 7 runs, 29%, edited a test file after being told not to.** Both still
repaired correctly, so no outcome changed. Recorded because future trials here
should budget an invalid-run rate near 30% rather than assume compliance.

## What this does not say

It is **not** evidence that durable work state fails to improve outcomes. A
ceiling cannot observe an effect in either direction. No treatment run exists.

**No follow-on experiment is specified**, because the protocol licenses one
only on a positive result.
