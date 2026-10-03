# Operating the simulate experiment: eligibility, enrolment, procedure, dataset

**2026-10-03. Prepared at Gate 3 of the post-P0 roadmap. No participant is
enrolled. No simulation has been shown to anyone. The thirty-day clock has not
started. Nothing here changes the pre-registration
(`simulate-experiment-prereg-2026-10-03.md`, sections 1 to 8) or the counter
(`simulate-experiment-instrument-2026-10-03.md`); where this document and the
pre-registration could be read differently, the pre-registration wins.**

The experiment needs two things the repository cannot supply: a release carrying
`replay simulate --policy`, and ten real existing users. This document is what
the operator does once both exist, written down so that ten enrolments are run
the same way and so that nothing about the outcome depends on who ran them.

---

## 1. Eligibility, decided before enrolment

A person is eligible when every line below is true on the day they are
enrolled. Each check is objective and is recorded with its evidence in the
enrolment record (section 2). There are no post-enrolment exclusions
(pre-registration element 9).

| | Check | Evidence the operator records |
|---|---|---|
| E1 | Has run `replay serve` before enrolment, on a machine they control, and that machine has a ledger directory with at least one session file holding a record with usage | The participant names the ledger directory (default `~/.replay/ledger`) and the count of `*.jsonl` files in it. The operator does not read the ledger. If the participant cannot name a directory with at least one file, E1 is not met. |
| E2 | Is not the operator of this repository | By identity |
| E3 | Has not committed code to Replay | The operator checks the author list of the repository (`git log --format=%an` and `%ae`) against the person; no match |
| E4 | Can run the release binary: macOS or Linux (Windows is not built) | The participant states the platform |
| E5 | Is not already on the enrolment list | The list |

The enrolment list is complete, with exactly ten eligible people, before the
first of them is exposed. It cannot grow or shrink afterwards. A shortfall
stops the experiment before it starts (WAITING FOR 10 EXISTING USERS); it never
shrinks the denominator.

**Window mode, decided by the gatekeeper at Gate 3: PER-PARTICIPANT.** Each
participant's thirty-day clock begins at the first qualifying showing of the
simulation and ends thirty days later. A clock never starts at enrolment unless
enrolment and exposure happen at the same sitting. The line "window mode:
per-participant" is written at the top of the enrolment list.

---

## 2. The enrolment record

One record per participant, held **outside this repository**, in the evidence
folder of section 5. It is the only place a participant's identity and their
`pNN` label are joined. The analytical dataset (section 4) carries only the
label.

```
participant id        pNN (assigned in enrolment order, p01 to p10)
name / contact        (outside the repository only)
eligibility           E1 yes/no + evidence, E2, E3, E4, E5 likewise
enrolment date        YYYY-MM-DD
machine               how the participant names the machine whose ledger is simulated
ledger directory      as the participant names it
window mode           per-participant (decided at Gate 3)
experiment start      YYYY-MM-DD = the date the simulation was SHOWN (exposure), or "not yet"
experiment end        YYYY-MM-DD = start + 30 days
exposure status       not yet | shown on YYYY-MM-DD | UNKNOWN
cap before            the four serve flags as the participant states them, or UNKNOWN
alternative cap       the policy the participant chose
policy file sha256
simulate.txt sha256, simulate.json sha256, policy hash the command printed
day-30 question asked YYYY-MM-DD
answer received       YYYY-MM-DD | UNKNOWN
Q1 changed            yes | no | UNKNOWN   (verbatim answer kept)
Q2 cap after          the four flags as stated | UNKNOWN
Q3 influenced         yes | no | UNKNOWN   (verbatim answer kept)
observation status    complete | not exposed | no answer | answer late | UNKNOWN
notes                 operational failures only, dated; never an interpretation
```

"Experiment start" is exposure, not enrolment (element 10). A participant who is
enrolled and never shown a simulation has no start, stays on the list, and
appears in the dataset as not exposed.

---

## 3. The operator procedure, per participant

The operator's job is to show one simulation and write down what the participant
says. The fixed wording below is used as written. Anything in the forbidden
list is not said, in any phrasing.

**Forbidden at every step:** suggesting a cap value; saying what the participant
should do; any sentence containing "save", "saving", "would have saved",
"forecast", "expect", "next month", "going forward", "better off", or a
comparison with other participants; mentioning the threshold of three or that
a change is the outcome being counted.

1. **Enrol.** Run the five checks of section 1, open the enrolment record,
   assign the next `pNN`, record the window mode and the enrolment date.
2. **The ledger.** Ask the participant for the directory and the machine. The
   ledger stays on their machine; the operator never copies it. Everything
   below runs on the participant's machine, by the participant or by the
   operator at the participant's keyboard, with the release binary installed
   there (`replay version` is recorded).
3. **The alternative cap, chosen by the participant** (element 3: "The
   participant chooses the alternative cap. No cap value is suggested"). Say,
   as written: *"The proxy has four caps: a session token cap, a daily token
   cap, a session dollar cap and a daily dollar cap. Pick any one or more, and
   any value you are curious about."* If asked what to pick, say, as written:
   *"Any value you are curious about. I cannot suggest one; that is one of the
   rules of this experiment."* Write their choice to `policy.json`, for example
   `{"maxSessionUsd": 2}`. Record it as the alternative cap.
4. **Cap before.** Ask, as written: *"When you start `replay serve` on this
   machine, which of those four caps do you set, and to what?"* Record the
   answer as stated. If they are unsure, ask them to show the command line or
   alias they start it with; if it cannot be determined, record UNKNOWN. Do not
   read it from any file.
5. **Run the certified command**, and keep both outputs:
   ```
   replay simulate --policy policy.json <ledger-dir>          > simulate.txt
   replay simulate --policy policy.json <ledger-dir> --json   > simulate.json
   ```
   If the report says `0 requests`, this is not an exposure (element 4):
   record "not yet", and the participant may be shown a later run over a
   non-empty population before the list closes.
6. **Show it.** Put `simulate.txt` in front of the participant and let them read
   it. Say nothing about what it means beyond answering factual questions about
   the words on the page. The moment it is shown is the exposure: record the
   date as experiment start, compute the end, set exposure status to "shown".
   Note the number of simulations shown (`exposures`), which is descriptive and
   decides nothing.
7. **Record the response.** Write down, dated and verbatim, anything the
   participant says about the result. Do not ask whether they will change
   anything. Do not ask what they think of the cap.
8. **Preserve the evidence.** `policy.json`, `simulate.txt`, `simulate.json`,
   and a `sha256sums` of the three, plus the policy hash printed in the first
   line of the report, go into the participant's evidence folder (section 5).
9. **Leave.** Between exposure and the end of the window the operator does not
   contact the participant about caps, Replay, or the experiment, and does not
   look at anything on their machine.
10. **The end-of-window question**, asked on or after the end date and no later
    than three days after it (element 11), in writing, exactly as follows,
    with `<date>` and `<machine>` filled in:
    - Q1: *"Since I showed you the simulation on `<date>`, have you changed any
      of the four spend-cap settings you start `replay serve` with on
      `<machine>`? yes / no / don't know"*
    - Q2, only if Q1 is yes: *"What are they set to now?"*
    - Q3, only if Q1 is yes: *"Did the simulation influence that change? yes /
      no / don't know"*

    Record the answers verbatim with the date they arrived. "Don't know", no
    answer, or an answer that cannot be mapped to yes or no is UNKNOWN. No
    follow-up question, no rephrasing, no reminder of what the simulation
    showed. An answer arriving after the three days is recorded with its real
    date and becomes UNKNOWN in the count. A change on a different machine is
    recorded in the notes and the row says what the participant said about
    `<machine>`.
11. **Transcribe one row** into the dataset (section 4), from the enrolment
    record, by label only. Then stop. The count happens once, after every
    window has closed, with the frozen file.

The operator records what was done and what was said. The operator decides
nothing about whether a change "really" counts; the counter does that from the
row, and the row is the participant's words.

---

## 4. The dataset

Exactly ten rows, one per `pNN` on the enrolment list, in label order. Every
value is what the participant reported, transcribed from the enrolment record.
A value the participant could not give is the string `UNKNOWN`, never a guess,
never left blank, never filled from a ledger, a refusal reason, a file or a
later command (element 15). A participant who was never exposed, never
answered, or answered late is still a row.

The file is frozen before any count: `frozenOn` is set to that day, the SHA-256
of the file is written into the enrolment list and into the Phase 5 artifact,
and the file is never edited again. The counter rejects a file frozen before a
window closed, a file with any number of rows but ten, a duplicate label, an
unknown field, and a row that contradicts itself; the fix for a rejection is a
corrected file, frozen again, before any count is read.

Blank template (the placeholders make it uncountable until filled):

```json
{
  "schema": "replay.simexp.dataset.v1",
  "preregistration": "docs/evidence/simulate-experiment-prereg-2026-10-03.md",
  "frozenOn": "YYYY-MM-DD",
  "window": {"mode": "per-participant"},
  "participants": [
    {
      "id": "p01",
      "exposedOn": "YYYY-MM-DD or UNKNOWN",
      "exposures": 1,
      "capBefore": "UNKNOWN or {\"maxSessionUsd\": 5}",
      "alternativeCap": {"maxSessionUsd": 2},
      "changed": "yes | no | UNKNOWN",
      "capAfter": "present only when changed is yes: {\"maxSessionUsd\": 2} or UNKNOWN",
      "influenced": "yes | no | UNKNOWN",
      "answeredOn": "YYYY-MM-DD or UNKNOWN",
      "ineligibleAfterEnrolment": false
    }
  ]
}
```

`ineligibleAfterEnrolment` is optional; `true` only for a participant found
ineligible under the five checks after enrolment (element 9), whose row then
reads `UNKNOWN: ineligible after enrolment` and stays in the ten. A cap set is
an object with any of the four keys `maxSessionTokens`, `maxDayTokens`,
`maxSessionUsd`, `maxDayUsd`; `{}` means no cap; a key absent or zero is no
cap for that setting. Rows run `p01` to `p10`.

How the fields answer the required observations:

| Required observation | Field | Who supplies it |
|---|---|---|
| simulation shown (exposure, not merely run) | `exposedOn`, the date it was shown; a run with 0 requests is not an exposure | operator, from step 6 |
| cap before | `capBefore` | participant, step 4 |
| alternative cap shown | `alternativeCap` | participant, step 3 |
| cap changed | `changed` | participant, Q1 |
| cap after, if changed | `capAfter` | participant, Q2 |
| whether the simulation influenced the change | `influenced` | participant, Q3 |
| observation status | the counter's `outcome` for the row, derived from the above; also written on the enrolment record | counter |
| provenance | `frozenOn`, the file's SHA-256 in the result, `preregistration` | operator and counter |

Count, once, after the last window closes:

```
go run ./scripts/simexp count <frozen dataset.json> > result.json
```

---

## 5. The evidence folder

Outside the repository, one folder per participant, named by label:
`<experiment root>/pNN/` holding the enrolment record, `policy.json`,
`simulate.txt`, `simulate.json`, `sha256sums`, and the verbatim answers. The
experiment root also holds the enrolment list (ten lines, labels and the window
mode), the frozen `dataset.json`, its SHA-256, and `result.json`. No ledger
file is ever copied into it. The repository receives, in Phase 5, the frozen
dataset's digest, the result and the analysis; it never receives the enrolment
records.

---

## 6. Audit of the counter against the pre-registration

Checked on 2026-10-03 against the counter as committed in `8191486`
(`scripts/simexp/count.go`, blob `0f1dd068`). No change was made to it.

| Element | The pre-registration says | The counter does | Status |
|---|---|---|---|
| 1, 6 | ten enrolled; denominator ten always; never re-based | rejects any row count but ten; `denominator` is the constant 10; every row is reported | CONSISTENT |
| 2, 10, 11 | thirty days from exposure, or a cohort end; question within three days after the end; later is UNKNOWN | per-participant end is exposure plus 30 days; cohort end is the file's; answer after end plus 3 days is `UNKNOWN: answer after the window`; a file frozen before any window closed is rejected | CONSISTENT |
| 4 | exposure is being shown a non-empty result | `exposedOn` is the shown date the operator records; `UNKNOWN` is `not exposed`; `exposures` decides nothing | CONSISTENT, by procedure step 6 |
| 5 | both a cap change and the participant's statement | qualifies only when `changed` is yes and `influenced` is yes | CONSISTENT |
| 7 | any of the four flags; zero or absent is no cap; enabling and disabling count | the four keys only; zero and absent compared alike; `changed` is the participant's word and the cap values only cross-check it | CONSISTENT |
| 8 | UNKNOWN is a third state; UNKNOWN in the chain is non-qualifying and reported | the chain stops at the first UNKNOWN and names it; the row stays; nothing imputed | CONSISTENT |
| 9 | a participant found ineligible after enrolment is reported as UNKNOWN, not removed | the row stays in the ten; `ineligibleAfterEnrolment: true` gives the outcome `UNKNOWN: ineligible after enrolment`, never qualifying (corrected at Gate 3, below) | CONSISTENT |
| 12 | the row's fields; frozen file with digest; one count | all fields present; `datasetSha256` is the digest of the bytes counted; the command is run once by hand | CONSISTENT |
| 13, 14 | fewer than three kills; no adjustment | `threshold = 3` compared with `<`; constants, not inputs | CONSISTENT |
| 15 | attribution participant-reported, never inferred | `influenced` is the only source; dates, caps and exposures never feed it; frozen mutant M120 guards exactly this | CONSISTENT |
| section 8 | one primary rule; enable count secondary and decides nothing | `enabledACap` is printed under `secondary` and no branch reads it | CONSISTENT |
| duplicates, provenance | | duplicate labels rejected; input order preserved; digest printed | CONSISTENT |

**Two findings, both closed at Gate 3 by the gatekeeper's order** (the
correction is recorded in `simulate-experiment-instrument-2026-10-03.md`,
section 6; the text below is the finding as reported).

1. **Ineligible after enrolment (element 9).** The counter has no way to say
   "ineligible"; the case is expressible only as UNKNOWN on the chain fields,
   which counts correctly and labels imprecisely. Options are a one-field
   addition to the row (`status: "ineligible-after-enrolment"`) with a matching
   outcome, which is a counter change, or the procedure above, which records
   the reason in the enrolment record and the notes. **Resolution: the field
   was added.** The enrolment record still carries the reason.
2. **An answer dated before the window end.** The pre-registration says the
   question is asked on or after the end; it does not say what an early answer
   is. The counter accepts one. The procedure (step 10) never asks early, so no
   such row can arise from a correctly run experiment; a row with
   `answeredOn` before the end is an operator error to be corrected before the
   freeze. **Resolution: the counter now rejects it**, naming the participant,
   the answer date and the window end; an answer on the end date is accepted.

---

## 7. Technical validation of the template, NON-EXPERIMENTAL

To check that the template above is the shape the counter accepts, a copy with
synthetic values was run through it on 2026-10-03. **The values are invented
for the shape check only. They are not observations, not participants, and
appear in no evidence folder.** The blank template itself, with its
placeholders, is rejected by the counter at the first placeholder it meets (the
`capBefore` placeholder string: "a cap is an object of the four serve settings
or UNKNOWN"), which is the intended property of a template. The
synthetic copy, ten rows, every chain field UNKNOWN except dates, was accepted
and counted as 0 qualifying, 10 UNKNOWN, decision KILLED, which is what a
dataset with no observations must say and is why the real count happens only
after the real windows close.
