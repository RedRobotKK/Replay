# Pre-registration: does a historical cap simulation change a user's cap?

**2026-10-03. FROZEN at Gate 1 of the post-P0 roadmap. No participant has been
enrolled. No observation exists. Nothing below may change after the first
observation is recorded.**

This is the authoritative specification of the commercial experiment that the
ten-pass board review pre-registered as experiments 9 and 10
(`docs/design/implementation-board-10-pass-2026-10-03.md`, section 8) and that
`simulate-p0-2026-10-03.md` section 5 restated. Those two documents fixed the
hypothesis, the population, the duration, the treatment, the measurement and the
threshold. This document fixes the seven elements they left open. It changes none
of theirs. The three decisions that resolved the open elements were made by the
gatekeeper on 2026-10-03 and are recorded verbatim in section 3.

---

## 1. The frozen hypothesis

**Pro hypothesis under test.** The board review names `replay simulate --policy`
"the only paid candidate": a developer who sees what an explicit alternative spend
cap would have done to their own recorded requests will act on it. The experiment
tests one behavioural consequence of that: whether seeing the simulation causes
the user to change their configured spend cap.

**Falsifier.** Fewer than three of ten participants change a cap because of a
simulation they were shown.

**What the experiment does not test.** Whether anyone pays. Whether the figure is
accurate. Whether a cap is a good idea. Whether the feature generalises beyond
these ten people. A survival is one hypothesis surviving one experiment, not
product-market fit; a kill is one hypothesis dying, not a verdict on Replay.

---

## 2. The fifteen elements

1. **Population.** Ten existing users. "Existing user" means a person who, before
   their enrolment, has run `replay serve` with a ledger directory that holds at
   least one session record with usage, on a machine they control, and who is not
   excluded under element 9. The ten are named on an enrolment list held outside
   this repository. The list is complete before the first enrolment and cannot
   grow or shrink afterwards.
2. **Duration.** Thirty days per participant, measured from that participant's
   start point (element 10), or a single common end date if the ten are run as one
   cohort (element 11). Which of the two applies is written on the enrolment list
   before the first enrolment.
3. **Treatment.** A historical simulation over the participant's own Replay
   ledger under an explicit alternative spend-cap policy: `replay simulate
   --policy <file> <their ledger directory>` as released. The participant chooses
   the alternative cap. No cap value is suggested, no benefit is described, and
   the invitation carries no saving, forecast or future-spend wording.
4. **Exposure.** A participant is exposed when they have actually been shown the
   result of a simulation over their own ledger: the printed report or its JSON,
   from a run that exited 0 and replayed at least one request. Running the
   command is not exposure. Being told about the command is not exposure. A run
   over an empty population (0 requests) is not exposure.
5. **Primary outcome.** The number of participants who explicitly report that
   they changed their spend cap because of the simulation they were shown. Both
   parts are required: a cap change under element 7, and the participant's own
   statement that the simulation was the reason.
6. **Denominator.** All ten enrolled participants, always. A participant who is
   never exposed, stops using Replay, cannot be reached, or answers UNKNOWN stays
   in the denominator and is not a qualifying participant. The denominator never
   shrinks and is never re-based on the exposed.
7. **Cap-change definition.** A change in the configured value of any of the four
   spend-cap settings `replay serve` accepts: `--max-session-tokens`,
   `--max-day-tokens`, `--max-session-usd`, `--max-day-usd`. The following count:
   one positive value to a different positive value; zero or absent (no cap) to a
   positive value; a positive value to zero or absent. The following do not count:
   running a simulation; viewing a result; changing any Replay setting that is not
   one of the four caps; a change on a machine other than the one whose ledger
   was simulated.
8. **Missing-data handling.** Each of the five recorded values (element 12) may be
   UNKNOWN. UNKNOWN is a third state, never a success and never inferred into a
   failure by any rule other than this one: a participant with an UNKNOWN
   anywhere in the chain that establishes the primary outcome (exposure, cap
   change, attribution) is not a qualifying participant for the count, and is
   reported as UNKNOWN alongside the count. Nothing is imputed from timestamps,
   ledger contents, refusal reasons, filesystem state or later commands.
9. **Exclusions.** Pre-treatment only, decided at enrolment, written on the
   enrolment list: the operator of this repository; anyone who has committed code
   to Replay; anyone who cannot be given a release binary carrying the command.
   There are no post-treatment exclusions. A participant who turns out to have
   been ineligible under this rule after enrolment is reported as UNKNOWN, not
   removed.
10. **Start condition.** A participant's experiment starts when both have
    happened: they are on the enrolment list, and they have first been exposed
    under element 4. The exposure date is the start point. A participant who is
    enrolled and never exposed has no start point and stays in the denominator.
11. **End condition.** Per participant, thirty days after the start point. For a
    common cohort, the pre-registered end date written on the enrolment list,
    which is at least thirty days after the last possible enrolment date. The
    end-of-window question (element 12) is asked within three days after the end;
    an answer that arrives later is recorded as UNKNOWN.
12. **Decision procedure.** After the observation window closes for every
    participant, a dataset of exactly ten rows is frozen as a file with its
    digest recorded before any count is made. Each row holds, as reported by the
    participant: cap before the simulation; alternative cap shown; whether they
    changed the cap afterwards; cap after, if changed; whether the simulation
    influenced the change; the dates of exposure and of the answer. The count of
    qualifying participants under element 5 is then made once and the result is
    written next to the digest. No second count, no re-reading of answers.
13. **Kill criterion.** Fewer than three qualifying participants: PRO HYPOTHESIS
    KILLED / REDESIGN REQUIRED. Three or more: PRO HYPOTHESIS SURVIVES THIS
    EXPERIMENT. The count is the primary outcome of element 5, not cap changes in
    general and not exposures.
14. **No threshold adjustment.** The threshold of three, the denominator of ten
    and the thirty days are fixed by this document and are not changed after any
    observation is recorded, for any reason, including dropouts, late answers,
    a shortfall of participants, or an interim count. A shortfall of
    participants stops the experiment before it starts (roadmap Phase 3: WAITING
    FOR 10 EXISTING USERS); it does not shrink the ten.
15. **Attribution.** Participant-reported, never inferred by Replay. The
    participant states whether the simulation influenced the change. A cap change
    without that confirmation does not count toward the primary outcome. Replay
    does not infer causality from timestamps, ledger contents, refusal reasons,
    filesystem state or subsequent commands.

---

## 3. The gatekeeper's three decisions, verbatim

**Decision 1, what counts as a cap change.** "Count as a cap change: changing from
one positive cap to another positive cap; changing from zero/no cap to a positive
cap; changing from a positive cap to zero/no cap. Do NOT count merely running a
simulation. Do NOT count merely viewing a result. Do NOT count changing an
unrelated Replay setting. The participant must confirm that the change was made
because of the simulation for the primary outcome."

**Decision 2, cap observation.** "Do NOT build a new persistent cap-state system
for this experiment. The experiment does not require Replay to infer the user's
configured cap from the ledger. The participant records: cap before simulation;
alternative cap shown by simulation; whether they changed their cap afterward;
cap after, if changed; whether the simulation influenced the change. If a cap
cannot be determined reliably, mark it UNKNOWN rather than infer it. Do not modify
production spend-cap semantics."

**Decision 3, observation and attribution.** "Use explicit participant reporting
for the behavioral outcome. The participant must explicitly report whether the
simulation influenced the change. Do not infer causality from timestamps, ledger
contents, refusal reasons, filesystem state, or subsequent commands. A cap change
without confirmed simulation influence does NOT count toward the primary outcome."

---

## 4. What Replay records

Nothing new. The cap in force is held in `replay serve` flags and persisted
nowhere; `spend-day.json` holds the day's spend, not the limit; a threshold
reaches the ledger only inside a refusal reason, and only when a refusal happens.
This experiment does not read any of those as evidence of a cap change (element
15). Whether `replay simulate` should write a local exposure record is a Phase 2
question that this document does not decide and does not require: exposure under
element 4 is established by the participant showing the report they were shown.

---

## 5. Audit against the frozen documents

Checked line by line on 2026-10-03 before freezing.

| Source | Says | This document | Result |
|---|---|---|---|
| Board review, experiment 9 | "Offer `simulate --policy` to ten existing users for 30 days; kill if fewer than 3 change a cap because of it." | Elements 1, 2, 3, 5, 13 | CONSISTENT |
| Board review, experiment 10 | "Same ten users: if no one enables a cap after seeing a simulation, the paid tier is observability-only and the Pro hypothesis dies." | Enabling counts as a cap change (element 7). The count of enables is reported as a secondary figure next to the primary count. | CONTRADICTION, reported in section 6 |
| Board review, section 10 | "ten existing users, thirty days, the count who change or enable a cap because of a simulation they ran, with a kill threshold of three" | Elements 1, 2, 5, 7, 13; "a simulation they ran" is narrowed to "a simulation they were shown" (element 4), which is the stricter reading | CONSISTENT |
| `simulate-p0-2026-10-03.md` section 5 | "ten existing users, thirty days, `replay simulate --policy` over their own ledger; kill threshold: fewer than three change or enable a cap because of a simulation they ran" | As above | CONSISTENT |
| `simulate-p0-2026-10-03.md` section 5 | "the observation itself, which is a question asked of each user at day thirty, recorded by hand" | Element 12: participant-reported, recorded as a frozen dataset | CONSISTENT |
| `simulate-p0-2026-10-03.md` section 1 | the P0 claim: a deterministic re-run of recorded requests, SIMULATED, never predicted | Elements 3 and 4 use the command as released and claim nothing beyond it | CONSISTENT |
| Board review, "the only paid candidate" | `simulate --policy` is Pro and "untested with any buyer" | Section 1 tests one behavioural consequence, not payment | CONSISTENT, and narrower |
| Roadmap, Phase 2 scope | instrumentation "may record only what is necessary" | Section 4: nothing is required by this document; Decision 2 forbids a cap-state system | CONSISTENT |
| Roadmap, immutable baseline | P0 closed, not reopened | No file under `cmd/` or `internal/` changes | CONSISTENT |

---

## 6. The one contradiction, reported and not resolved here

Experiment 9 and experiment 10 in the board review are two kill rules over the
same ten users. Experiment 9 kills on fewer than three changes. Experiment 10
kills on zero enables. They disagree on one case: three or more participants
change an existing positive cap and none enables a cap where there was none.
Under experiment 9 the hypothesis survives; under experiment 10 "the paid tier is
observability-only and the Pro hypothesis dies".

Decision 1 fixes the primary rule as experiment 9 with enables counted as changes.
This document therefore pre-registers experiment 10's figure, the number of
qualifying participants whose change was from no cap to a positive cap, as a
**secondary observation that is reported and carries no decision**. Whether that
figure should also be a kill rule is a gatekeeper decision that was not made at
Gate 1. It must be made before the first observation is recorded or not at all;
making it afterwards would be the threshold adjustment element 14 forbids.

---

## 7. Status

Enrolment list: does not exist. Release carrying the command: does not exist (the
commit is on a branch, unreleased). Observations: none. The next roadmap gate,
Phase 3, stops at WAITING FOR 10 EXISTING USERS until both exist.

---

## 8. Resolution of section 6 (added 2026-10-03, before any enrolment)

The gatekeeper decided, after Gate 1 and before any observation exists: **"Do NOT
make 'enable a cap' a second kill criterion."** The experiment has exactly one
primary commercial decision rule, element 13: fewer than three of the ten enrolled
participants who were shown the simulation change their spend cap because of that
simulation, and the current Pro hypothesis is killed or redesigned.

The enable figure from experiment 10 is retained only as a secondary descriptive
observation. It must not create a second kill rule, override the primary outcome,
rescue a failed primary outcome, alter the denominator, alter the threshold, or
trigger a post-hoc reinterpretation. Reported separately, with no decision riding
on any of them: the number who had no cap and enabled one; the number who changed
an existing cap; the number who changed a cap but did not attribute it to the
simulation; the number with an UNKNOWN or missing observation.

Section 6 is therefore closed. Nothing in sections 1 to 5 changes.
