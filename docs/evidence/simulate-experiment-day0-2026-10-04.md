# Day 0 of the Plan A recruitment attempt: what is frozen before the call goes out

**2026-10-04. Plan A only: existing users under the frozen definition. No
onboarding, no second experiment, no amendment. The experiment itself
(`simulate-experiment-prereg-2026-10-03.md`, `simulate-experiment-operations-2026-10-03.md`,
`simulate-experiment-amendment-1-2026-10-03.md`) is untouched. This file records
the operational materials, the channels, the deadline and the stop rule so that
none of them can move after publication.**

## Decision closed before this file

A person onboarded for a study is not an existing user. Experiment 9 keeps its
population as frozen; a study of newly onboarded users would be a separately
pre-registered experiment and is not started. The adjudication is in the
session record of 2026-10-04 and rests on the board review line 666 ("ten
existing users"), the pre-registration lines 20 to 22 and 38 to 40, and the
operating procedure's E1.

## The public notice, verbatim

The text below is published as a pinned post in GitHub Discussions of
RedRobotKK/Replay. Any difference between the published post and this text is
a protocol breach.

# Looking for ten people who already run `replay serve`

I am running a small pre-registered study and need ten participants who already use Replay's proxy. It takes one sitting of about twenty minutes, plus three short questions from me thirty days later.

**You qualify if all of these are true:**

- You have run `replay serve` on a machine you control, and that machine's ledger directory (the default is `~/.replay/ledger`) holds at least one `.jsonl` session file that recorded usage.
- You are on macOS or Linux.
- You have never contributed code to this repository.

**What happens in the sitting.** On your machine, on Replay v0.7.0, you run one released command, `replay simulate --policy`, over your own ledger, with a spend-cap value you pick yourself, and you read what it prints. I keep a copy of that printed report and the cap you chose. I do not read your ledger, and nothing from your machine is sent anywhere.

**What happens after.** I do not contact you about Replay for thirty days. Then I ask you three fixed questions about the proxy's spend-cap settings on that machine and record your answers exactly as you give them. "I don't know" is an answer.

**What is recorded about you.** A label such as `p04`, the dates, the report, your cap, and your answers. Your name or handle is kept in one private file on my machine and appears in no document or dataset. You can withdraw at any time by telling me; what was already recorded stays, under its label, and nothing more is added.

**No payment is offered.** Participation is voluntary.

**To take part, reply in this thread with:**

1. a handle I can use for you;
2. the path of your ledger directory and the number of `.jsonl` files in it (the count, not the files);
3. macOS or Linux;
4. whether you have ever committed code to this repository.

I will screen replies against the five written criteria in arrival order and answer each one here or by direct message. The study's pre-registration and procedure are public in this repository under `docs/evidence/`.

This call is open for fourteen days from the date of this post.

## Declared channels, complete

1. The pinned Discussion post carrying the notice verbatim; replies in that thread are the primary response channel.
2. A notice on replay.doctor linking to the post.
3. A note appended to the v0.7.0 release page linking to the post.
4. One mention in the post of the three accounts that are the repository's entire known external footprint on 2026-10-04: two non-operator stargazers and one non-operator issue author. The complete known set, not a selection.

Nothing else. No personal invitations, no install suggestions, no community
where Replay is not already run.

## Deadline, success, failure, stop

D0 is the UTC date the post goes live, written into the register that day.
The strict-population deadline is D0 + 14 days, 23:59 UTC, not extended for
any reason.

- **Recruitment success:** at least one respondent classified ELIGIBLE on all
  five checks, who acknowledges the participant notice and is shown their
  report at the same sitting, on or before the deadline. That showing is p01's
  first exposure and fixes Amendment 1's 30-day window; Amendment 1 governs
  from then on.
- **NO-POPULATION condition:** on D0 + 14, the post has been live on every
  declared channel for at least 12 days, the three accounts were mentioned,
  every response has a screening record, and zero respondents are ELIGIBLE.
  Then experiment 9 is recorded as NOT RUNNABLE NOW for want of a population,
  with the register's counts as the evidence. The population is not broadened.
- **Stop:** the deadline, ten ELIGIBLE enrolments, or Amendment 1's closure
  after p01. Recruiting past any of them is a breach.
- **Amendment boundary:** extending the deadline, changing channels after
  publication, enrolling anyone who fails E1 as frozen, or opening the clock
  on anything but p01's first exposure at the enrolment sitting.

## Enrolment and exposure at one sitting

The label is assigned at the sitting in which the report is shown, so that
p01 is by construction the first person exposed and Amendment 1's anchor is
unambiguous. Exposure is element 4 verbatim; the checklist quotes it. The
participant runs v0.7.0 and the checklist stops if `replay version` says
otherwise.

## Materials held outside the repository

In the experiment root on the operator's machine, under `day0/`, hashed
before publication:

| File | SHA-256 |
|---|---|
| `01-public-notice.md` | `3e4839714f2900151a89c0bb1aa81e7c6d8c3560e73beae4069e5bf9042e4bca` |
| `02-channels.md` | `64730a851ed5ab1ea680e2325175175465ecfe3920976f956e6e3ed8157c90bb` |
| `03-register.md` | `9912708fa9d4bdfffe49b71c96090b63a72ea20581f73fb243aa0da14df7edc5` |
| `04-screening-record-template.md` | `e127074aaab07207a55438339ac39d2cfe85911245b84506109572512cdcaaba` |
| `05-participant-notice.md` | `59bba39fd3088af39b615912353c314434264023492e3f8599a1c15ca9406240` |
| `06-pre-exposure-checklist.md` | `78322ffd7493e21fa3e32fe5b5bcbb4227232b1ff4e4981f86e61eb35b073b45` |
| `07-deadline-and-no-population.md` | `73a1645ce3ca3f98f22fefcb921abd3aeb317a180a48797950d41bc37bf4917b` |

The register already carries one line: the operator, screened 2026-10-04,
INELIGIBLE on E2 and E3, no label.

## Audit

Every text was scanned for the procedure's forbidden phrases (saving,
forecast, expect, next month, going forward, better off, the threshold, a
recommendation); none occurs outside the quoted rule itself. Two judgement
calls, recorded so they are not mistaken for drift: the participant notice
says the day-thirty questions are about the proxy's spend-cap settings,
because consent requires saying what is recorded, and it does not say that a
change is what is counted or name any threshold; and no payment is offered,
which keeps Plan A free of the compensation question the onboarded design
would have raised.

## State at publication

Enrolment 0 of 10. p01 unassigned. Clock not started. Exposure zero. v0.7.0
at `c883513` is the participant artifact. QT-7 closed.
