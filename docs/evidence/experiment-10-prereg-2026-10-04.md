# Pre-registration, Experiment 10: do per-token developers adopt Replay when introduced to it?

**2026-10-04. FROZEN before the first invitation. A separate experiment from
Experiment 9 (`simulate-experiment-prereg-2026-10-03.md`): different
population, different outcome, its own labels, register rows, records and
tables. No person, row or sentence is shared between the two. Nothing in
Experiment 9's frozen documents changes.**

## 1. Research question

If a developer who pays per token for an AI coding tool is introduced to
Replay because it can show what their recorded requests cost at list price,
will they install it and route their own real work through `replay serve`?

## 2. Hypothesis and falsifier

**Hypothesis.** Such a developer, so introduced, adopts the tool.
**Falsifier.** Fewer than five of twenty people who accept the invitation
adopt it under element 10 within fourteen days of accepting.

**The treatment, stated precisely.** The introduction is the invitation in
section 6, verbatim, which says the tool records what their recorded requests
cost at list price and nothing else, plus the one install note of section 9.
The treatment does not mention, show or suggest a spend cap. A cap simulation
is not part of the adoption treatment. It appears only in the secondary
showing of element 11, after adoption, under Experiment 9's exposure
procedure, where the participant chooses the cap and the operator never
suggests a value. This resolves the wording conflict found at drafting: the
hypothesis names what the invitation actually says, not the later showing.

## 3. Population

A person who, before any install, states that they pay per token for Claude
Code, Codex CLI, Aider or another OpenAI-compatible client that honours a
base-URL override; uses macOS or Linux; is not the operator; has never
committed code to Replay; is not a current client of the operator; has never
run Replay.

## 4. Exclusions, decided before onboarding

The operator; anyone on the repository's author list; anyone with an active
commercial engagement with the operator; anyone who has run Replay before;
anyone in Experiment 9's register under any event. There are no post-onboarding
exclusions. A person found ineligible afterwards is reported as UNKNOWN and
stays in the denominator of those who accepted.

## 5. Recruitment channels, frozen, in this order

1. One post in the General category of openai/codex Discussions, the invitation
   verbatim. Checked 2026-10-04: the area carries GitHub's community guidelines
   and no rule against studies; the post is removed on any maintainer request.
2. One post on the operator's X account.
3. One post on the operator's LinkedIn account.
4. Tokyo developer meetups attended by the operator, the invitation spoken,
   handles collected.
5. The Aider Discord, only after the operator has read its rules and found
   recruitment permitted; recorded before posting.
6. r/ClaudeAI and r/ChatGPTCoding, only after the operator has read each
   subreddit's rules and found recruitment permitted; recorded before posting.
7. One Ask HN.
8. Direct notes by handle to people who have publicly written about per-token
   agent costs, in a list frozen before the first note.
9. A research-participant platform, only if fewer than three acceptances exist
   after the first twenty-five direct invitations, and within section 12.

An invitation is a message addressed to one identifiable person. Posts are
channels; the replies they produce are logged with the post as channel.

## 6. The invitation, verbatim

> I am running a small pre-registered study of developers who pay per token for an AI coding tool (Claude Code, Codex CLI, Aider or anything OpenAI-compatible) on macOS or Linux. It asks you to install one open-source tool, point your coding tool at it for a week of your normal work, and answer a short report form on day 14. It records what your recorded requests cost at list price and nothing else; nothing leaves your machine. A ¥3,000 gift card on submitting the day-14 report, whether or not you kept the tool. Reply with a handle, which tool you pay per token for, and macOS or Linux. Pre-registration is public before the first install.

No sales copy, no savings, no cap, no expected result.

## 7. Sample target and denominator

Twenty acceptances. An acceptance is a reply to the invitation that states a
handle, a qualifying tool and a platform, and is screened into the population
of section 3. The denominator for the primary outcome is everyone who accepted
and was screened in, whether or not they installed.

## 8. Primary outcome: adoption

Within fourteen days of acceptance, the participant reports a ledger directory
on a machine they control holding at least five session files with usage.
Participant-reported on the day-14 form. The operator reads nothing on the
participant's machine. UNKNOWN is a third state and never becomes adopted.

## 9. Onboarding

On acceptance: eligibility confirmed, register row, then one install note
naming v0.7.0 and the base-URL variable for the participant's tool, and
nothing else. The operator answers questions the participant asks and does
not troubleshoot unasked, does not coach, does not check in. The install
note's date starts seasoning.

## 10. Adoption definition and seasoning

Adopted: five or more session files with usage, reported on day 14.
Seasoned: seven days of the participant's own real work after the install
note, with at least five session files. Seasoning is a precondition of the
secondary showing, not of adoption.

## 11. Secondary outcome: Experiment 9's question in population O

A seasoned participant is offered the sitting of Experiment 9's operating
procedure, steps 3 to 10, verbatim: they choose a cap, the released
`replay simulate --policy` is run on their machine over their own ledger, they
are shown the report, and thirty days later they are asked the three fixed
questions. Labels `o01` to `o20`. The count of participants who report
changing a cap because of the simulation is reported in its own table,
labelled population O, counted by hand under Experiment 9's element 8 rules.
No decision rides on it, it is never pooled with Experiment 9, and no cost
report is shown by the operator before that participant's thirty days end.

## 12. Compensation

¥3,000 as a gift card, or the local equivalent, on submission of the day-14
report, whether or not the participant installed, adopted or kept the tool,
and whatever they answered. Stated in the invitation and the notice. Total
budget for this experiment, all lines: ¥120,000; spending past it needs an
amendment filed before the spend.

## 13. Controls

Channel list and invitation order frozen here. Arrival order for every
screening and label. Experiment 9's forbidden phrases apply to every message:
no saving, forecast, expect, next month, going forward, better off, no cap
value, no threshold. The operator never suggests a cap and never states the
expected result. Handles only in any conversation; identity only in the
operator's folder. The register is append-only.

## 14. Stop rules

Acceptances close 2026-10-28 23:59 UTC. If fewer than twenty accepted by then:
UNDER-RECRUITED, the adoption rate is reported on those who accepted, and the
five-of-twenty rule is not applied. Each accepted participant's day-14 window
runs its course. The last day-14 report is due 2026-11-11 23:59 UTC. Decision:
five or more adopters of twenty is SURVIVES; fewer is FAILED. Nothing here is
adjusted after an observation exists. Fewer than three acceptances after the
first twenty-five direct invitations permits channel 9 within the budget.

## 15. Deadlines

| Event | Date |
|---|---|
| This pre-registration committed | before the first invitation |
| First invitations | after the commit |
| Acceptance checkpoint | after the 25th direct invitation |
| Acceptances close | 2026-10-28 23:59 UTC |
| Last day-14 report | 2026-11-11 23:59 UTC |
| Secondary showings | each participant at seasoning, never before day 7 |
| Secondary answers | thirty days after each showing |

## 16. What this experiment can and cannot support

It can say whether this introduction produces adoption among per-token
developers who accepted. It cannot say anything about Experiment 9's
population, about people who declined, about any dollar figure, or about
Replay's existing users. A survival is adoption among acceptors, not demand.

## 17. Separation from Experiment 9

Experiment 9 is the existing-user experiment, frozen at `15c4614` with
Amendment 1 at `8c7cab6` and its Day 0 at `ea5e6f9`, deadline 2026-10-18
23:59 UTC. A person encountered through this experiment is population O and
stays so. A person discovered to be an existing user is Experiment 9's and is
never invited here. The two registers share a file and never a row.
