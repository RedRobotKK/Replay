# Design

Open design questions, written up before a decision is made rather than after.

These are not the architecture (which describes what exists) and not the ADRs
(which record what was decided and are never edited). A document lives here
while the question is still open. When it is settled, the decision goes to an
ADR and the built thing goes to the architecture.

| Document | The question it opens | State |
|---|---|---|
| [Live context visibility: the options](P3-VISIBILITY-OPTIONS.md) | Where live context cost should surface, and who consumes it: the `serve` dashboard, the status line, a tool the agent calls, or a published shape with no consumer | Open. Four options, no recommendation, three checkable questions that decide between them |
| [`replay quota`: what an honest estimator can tell a flat-seat subscriber](quota-estimator-blue.md) | What the captured rate-limit headers can honestly answer for a subscription seat, whether time-to-lockout can be forecast at all, and what the command says to the majority who have no headers | Open. Forecast yes, avoidable-share refused, three checkable questions |
| [Red team: attacking the quota claim](quota-estimator-red.md) | A hostile review of the claim that Replay can show a subscriber how much of their window went to re-sent content. 15 findings, each cited to file:line or a command run | **Five rated thread-defining.** Its lead finding retracted an evidence document of this repository's own, published hours earlier |
| [Quota data census](quota-data-census.md) | What rate-limit data actually exists on this machine, established so a design and a critique argue about something real | **Zero of 17 ledger records carry any quota header**, `retry-after` never observed, and the trial the design rested on was never committed |

---

[Documentation index](../README.md) · [Repository README](../../README.md)
