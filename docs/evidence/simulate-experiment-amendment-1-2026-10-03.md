# Amendment 1 to the simulate experiment: sequential enrolment

**2026-10-03. Filed before any participant exists, before any exposure, before
any observation. An amendment to the MAIN experiment's enrolment timing, not a
pilot. It changes element 1's timing requirement and nothing else.**

Amends: `simulate-experiment-prereg-2026-10-03.md` (frozen at `4e2689b`,
resolution at `15c4614`) and the closing paragraph of section 1 of
`simulate-experiment-operations-2026-10-03.md` (frozen at `87ca95a`). Neither
file is edited. Where this amendment and those files differ on the timing of
list closure, this amendment governs; on everything else, they govern.

---

## 1. What is replaced

Element 1 of the pre-registration says the ten "are named on an enrolment list
held outside this repository. The list is complete before the first enrolment
and cannot grow or shrink afterwards." The operating procedure restates it:
"The enrolment list is complete, with exactly ten eligible people, before the
first of them is exposed." Both sentences exist to keep the denominator from
being chosen after an outcome is seen.

Only that timing is replaced. The population (ten existing users), the five
eligibility checks, the denominator and its rule that it never shrinks are
untouched.

## 2. The amended rule

The participant list is permitted to grow sequentially after the first
exposure. Each candidate is classified ELIGIBLE, INELIGIBLE or UNKNOWN under
the five checks, independently of every other candidate, and only an ELIGIBLE
candidate is enrolled and may be exposed.

**The list closes at the earlier of:**

1. **the tenth ELIGIBLE enrolment; or**
2. **the end of the fixed recruitment window, defined as 30 days after p01's
   first exposure.**

The recruitment window is anchored to p01's first exposure date, written on
the enrolment list on that day, and is immutable from that moment. It is not
extended for any reason, including observed recruitment pace.

**If the list closes with fewer than ten ELIGIBLE participants, the experiment
is ABORTED as a recruitment failure.** No outcome is counted. N is not reduced.
The threshold is not changed. No partial result is reported as an experimental
finding. Whatever was collected is preserved in the evidence folder, labelled
ABORTED, and never counted; the counter refuses a dataset of any size but ten,
so an aborted experiment cannot be counted by accident. An aborted experiment
is reported as exactly that, with the number enrolled and the dates, and the
hypothesis is left untested.

## 3. What this amendment does not permit

- It does not permit exposing an INELIGIBLE or UNKNOWN candidate. Enrolment
  still requires ELIGIBLE on all five checks, with the evidence the operating
  procedure names, before that person is shown anything.
- It does not change N (ten), the threshold (three), the thirty-day
  observation window per participant, the three-day collection period, the
  treatment, the definition of exposure, the attribution rule, the UNKNOWN
  rule, the decision procedure, the kill criterion, the no-adjustment rule, or
  the counter. Elements 2 to 15 stand as frozen.
- It does not permit asking any end-of-window question before the list has
  closed. By construction the earliest such question falls on the day p01's
  window ends, which is the day the recruitment window ends, so the list is
  closed before the first outcome can exist.
- It does not permit a recruitment decision to depend on another participant's
  outcome or on anything said in another participant's session.

## 4. Recruitment bias, stated

Sequential recruitment creates an operational possibility that the operator
recruits a later candidate knowing how an earlier participant reacted when
shown their report. That knowledge is not an outcome: no end-of-window answer,
the only outcome the experiment uses, can exist before the list closes. The
safeguards of the operating procedure stay in force unchanged and are the
control for this: the fixed scripts; the forbidden-phrase list; no contact
about caps or the experiment between exposure and the end of a window;
eligibility determined independently for each candidate from that candidate's
own evidence; and no recruitment decision made on the basis of another
participant's outcome, which cannot be known at the time. The operator does not
discuss any participant's session with any other candidate.

## 5. Gates added by this amendment

| Gate | PASS | FAIL |
|---|---|---|
| A1 | This amendment is committed before the first exposure | Any exposure predates its commit: the experiment is void |
| A2 | Every exposure follows an ELIGIBLE classification for that person | Any exception: the experiment is void |
| A3 | The list closes at ten ELIGIBLE within 30 days of p01's first exposure | Otherwise ABORT, as section 2 says |
| A4 | No end-of-window question is asked before the list closes | Otherwise the experiment is void |
| A5 | Every gate of the pre-registration and the operating procedure, unchanged | As they say |

## 6. Consistency check, performed before this file was committed

- The pre-registration file is byte-identical to `15c4614` and the operating
  procedure to `87ca95a`; neither was edited for this amendment.
- The counter (`scripts/simexp/count.go` at `587009d`) carries `denominator =
  10` and `threshold = 3` as constants and rejects any dataset that does not
  hold exactly ten rows. A labelled NON-EXPERIMENTAL nine-row file was run
  through it on 2026-10-03 and was refused with the reason naming ten. No
  change to the counter was needed or made.
- The release is v0.7.0 at `c883513`; no file under `cmd/`, `internal/` or
  `scripts/` changes with this amendment.
- No participant existed, was enrolled, or was exposed when this was written.
  Next label: p01. Exposure: zero.
