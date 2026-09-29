# A2 adversarial review, before calibration

**Conducted before any calibration run. One candidate was repaired and the
repair is recorded here rather than absorbed silently.**

## Flaw found in candidate A, and repaired

**Original claim:** "Setting `reasoning_effort: low` in the config reduces the
reasoning effort sent to the upstream API."

**Why it was invalid.** The config default is `max`, which normalises to `max`.
Setting `low` normalises to `high`. So relative to the default, setting `low`
genuinely *does* reduce the effort sent. The claim is arguably TRUE, and two
competent engineers could reasonably disagree about it. That triggers the
preregistered rejection criterion for ambiguous ground truth.

**Repaired claim:** "The `reasoning_effort` value from the config is passed
through to the upstream API unchanged."

This is a universal claim falsified by a counterexample: `low` and `medium` are
both rewritten to `high` by `EFFORT_ALIASES`. `max` and `high` do pass through,
which is why the claim looks plausible, and that is the point. The ground truth
is now unambiguous and the dispositive evidence is unchanged.

**This repair happened before any agent run and before any data existed.**

## Threats considered, per candidate

| Threat | Assessment |
|---|---|
| Could an agent succeed without detecting the contradiction? | The scorer requires a verdict **bound to C1 in the same sentence**, plus the mechanism, plus the dispositive file. A lucky guess would have to name the alias mapping |
| Is the contradiction real? | Independently re-derived from source: `EFFORT_ALIASES` maps `low -> high` and `medium -> high`. Verified by parsing the table, not by reading it |
| Is one artifact merely stale documentation? | No. Both sides are live operational code, not docs |
| Hidden lexical shortcut? | Grepping `reasoning_effort` returns the config default, the dataclass field and the function name. It does **not** return `EFFORT_ALIASES`, which is the dispositive artifact |
| Does the task tell the agent where to look? | No path, no command, no file is named anywhere in the prompt or the claims table |
| Would a generic repository search solve it trivially? | That is exactly what calibration measures. If C0 exceeds 70%, the candidate is rejected for that reason |
| Scorer false positives? | Found one in testing: an answer negating a different claim while confirming C1. Closed by binding the verdict to the claim identifier. 13 scorer tests pass |
| Scorer false negatives? | A correct detection phrased without naming the file fails. This is deliberate: citing the dispositive evidence is part of detection, and it is stated in the task |
| Unstated convention required? | The task explicitly asks for the claim id, what is true instead, and the file |
| Testing repository familiarity? | Every agent starts fresh on a repository none has seen |
| Difficulty from irrelevant complexity? | The repository is 39 Python files. Difficulty is compositional, not volumetric |
| Would an independent engineer agree on the ground truth? | For the repaired A and for C, yes: a mapping table and a conditional. For **B this is the live doubt**, see below |

## Candidate B carries a standing doubt

B is an **absence claim**: that no test verifies the configured value reaches the
upstream request. Absence is harder to score and harder to agree on. Two
engineers could disagree about whether a given test "counts" as verifying it.

B is retained in third position and **will be withdrawn rather than repaired**
if it is reached and the doubt materialises. It is declared here so that
withdrawing it later is not a post-hoc convenience.

## Distractors are true, deliberately

C2 and C3 in every candidate's claims table were independently verified as TRUE
against the repository. An agent that checks them will find them sound, which
means finding the C1 contradiction requires actual investigation rather than a
general suspicion that something in the table is wrong.
