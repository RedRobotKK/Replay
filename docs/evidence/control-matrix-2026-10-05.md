# Control-surface matrix, 2026-10-05

**Question.** What can Replay observe and then actually improve? Built from
three read-only inventories of the code at `60fea09` (in-path proxy controls,
recommendation surfaces and their consumers, waste and invariant detectors)
and from this machine's records: 994 Claude Code transcripts, 465 Codex
rollouts, a proxy ledger of 18 requests. One operator, one machine; nothing
below is a law.

Every cell is labelled by how it is known: OBSERVED (the client or provider
wrote it), CALCULATED (derived from observed figures), MEASURED (counted on
this corpus today), ESTIMATED, INFERRED, UNAVAILABLE, or HYPOTHESIS.

## What exists, by stage of the loop

| Stage | What the code does today | Wired | Evidence |
|---|---|---|---|
| Observe | transcripts (Claude Code, Codex, Grok, Ollama, Cursor, others), proxy ledger with usage, status, retries, refusals, body hashes, rate-limit headers verbatim | yes | strong; conservation laws in CI |
| Diagnose | cost, blame, context, diff (cache-break cause per turn), re-read rate, error classes, compaction accounting, idle risk, staleness of calibration | yes | strong |
| Optimize (recommend) | advise (6 kinds, action sentence), learn (policy from own sessions, 30% holdout), route (model switch arithmetic), trim, prefix gate, ceiling, guards fence | yes | strong for the figures; adoption of advice is a human keystroke, never detected |
| Control (in path) | circuit breaker, session and day spend cap, error budget, loop detector, pre-flight deficit, sibling hold, retry with Retry-After, masking, context-edit injection, include_usage injection | yes, all off by default except hostguards, size limit, include_usage | strong; each measured with a local fake upstream on 2026-10-03 |
| Route | none: one fixed upstream, model never rewritten, path never redirected | absent, by decision (routing execution killed) | n/a |
| Preserve / recover | none, by evidence (durable scratch rejected three times; third time 12 to 0 on 2026-10-05) | absent | strong, negative |
| Verify improvement | one loop in code: learn selects a context-edit policy, the proxy trials it treated against control, graduate compares realized to predicted with error-share and re-read guards, guardrail revert; `advise` tracks realized share only after a human marks a suggestion applied | behind `serve -policy-file`, EXPERIMENTAL; graduation has no reader | never run on live traffic here; the ledger holds 18 requests |

## Matrix

| Primitive | Existing code | Wiring | Observable inputs | Evidence | Real corpus | Intervene | Automate | Outcome measurable | Quality guard | User pain | Frequency | Revenue | Leverage | Risk | Provider dep. | Falsification risk | State |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| Spend cap (session, day) | internal/proxy/guards.go | wired, flag | OBSERVED usage, list price | strong | ledger only (18 req) | yes, refuses | yes | yes: refusals counted | n/a (a refusal is the outcome) | medium | low here | medium | high | low | low | low | shipped |
| Loop detector | guards.go DetectLoop | wired, flag | OBSERVED identical tool-call runs | strong | 106 repeated identical failing calls in 56,681 results (MEASURED, transcripts) | yes, warn then refuse | yes | partial | no | low here | low | low | medium | low | none | low | shipped |
| Error budget | guards.go | wired, flag | OBSERVED error content share | strong | 1,759 error results of 56,681 (MEASURED) | yes | yes | partial | no | low | low | low | medium | low | none | low | shipped |
| Circuit breaker, retry | guards.go, retry.go | wired, flag | OBSERVED 429/5xx, Retry-After | strong | no 429 in the ledger here | yes | yes | yes | n/a | medium elsewhere | unknown | medium | high | low | high | low | shipped |
| Quota admission (headers) | quota.go captures; internal/quota unwired | absent | rate-limit headers: captured, read by nothing | weak | 0 records with headers here | possible | possible | possible | n/a | unknown | unknown | medium | medium | medium | high | high | INVESTIGATE, blocked on live traffic with spend |
| Codex depletion instant | none | absent | OBSERVED used_percent, resets_at | medium: fit on first half of a window predicts its last reading within 1.3 points median, 4.4 p90, 19 windows (MEASURED) | yes | recommendation only | no | partial | no | low: 1 of 32 windows reached 90% | low | low | low | low | high | medium | DEFER |
| Anthropic quota | statusline re-display of Claude Code's own rate_limits | wired | OBSERVED, verbatim | strong for what is shown | yes | no | no | no | n/a | low | high | low | low | low | high | low | shipped, display only |
| Routing on capacity, cost, capability | route.go arithmetic only | recommendation only, execution killed | OBSERVED usage; capability UNAVAILABLE; capacity UNAVAILABLE | medium for cost, none for capacity or quality | 10 of 908 sessions switch model (MEASURED) | no | no | no outcome signal | no | low here | low | high in theory | low | high | high | high | DEFER |
| Stale or superseded context | none | absent | Read then Edit of the same path | MEASURED: 0.2% of tool-result bytes | yes | no mechanism in the client | no | no | no | low | low | low | low | low | none | n/a | REJECT on incidence |
| Duplicate unchanged reads | CountReReads (aggregate rate) | wired | OBSERVED tool-result labels | MEASURED: 0.0% identical re-reads; re-read rate reported per session | yes | recommendation | no | partial | no | low | low | low | low | low | none | n/a | shipped as diagnosis; REJECT as intervention |
| Work redone after compaction | none | absent | Reads after a boundary | MEASURED: median 0 re-reads over 22 boundaries | yes | no | no | no | no | low | low | low | low | low | none | n/a | REJECT on incidence |
| Compaction accounting (retrospective) | analysis/context.go CompactionDetail | wired | OBSERVED compactMetadata, usage | strong | 99 boundaries | no | no | n/a | n/a | medium | medium | low | medium | low | medium | low | shipped today |
| Compaction approaching signal | none | absent | window UNAVAILABLE in the record | panel 11 MODIFY, 1 REJECT | yes | no | no | no | no | medium | medium | low | low | high | high | high | REJECT as a signal; inferred-ceiling line gated on n>=10 per tier, not built |
| Advisor share bound | advisor.go note, noteReads | wired | CALCULATED share | strong: 155% reproduced and bounded; positive control 4 sessions set aside | yes | yes, sets aside | yes | yes | it is the guard | medium (trust) | every run | low | high | low | none | low | shipped today, M140 |
| Cache-TTL setting (apply, then verify) | apply.go writes promptCacheTtl; learn selects ttl-5m here at 28.0% saving, held-out 27.8%, 755 sessions (MEASURED) | apply wired; record and verify absent | OBSERVED per-session as-run effective tokens | strong for the prediction; none for the realized effect | yes | yes, Replay writes it | yes with `--yes` | possible: sessions after the write against before | partial: error share, re-reads after clear exist in graduate | medium | once per machine | medium | high | low | medium | medium | panel question B |
| Learn, trial, graduate loop | internal/learn, proxy/policyinject.go, trial.go | behind `-policy-file`, EXPERIMENTAL; Graduated has no reader | OBSERVED ledger arms and usage | design strong; never run live | no live data here | yes, in path | yes | yes by design | yes by design | unknown | unknown | high in theory | high | medium | medium | high until run live | panel question C |
| Policy engine with provenance | learn.Result carries candidates, interval, holdout, reason | partial | as above | medium | yes for selection | partial | partial | partial | partial | low | low | medium | medium | medium | low | medium | INVESTIGATE after B |
| Experiment runner (before vs after) | learn holdout, graduate, simulate --policy (SIMULATED) | partial | OBSERVED | medium | yes offline | n/a | n/a | yes offline | partial | low | low | medium | high | low | low | medium | INVESTIGATE |
| Regression detection | diff per turn; staleness of calibration per model; since (digest) | wired | OBSERVED | strong per turn; none across periods | yes | no | no | n/a | n/a | medium | medium | low | medium | low | low | low | shipped as diagnosis |
| Quality guard (task outcome) | none; graduate has error-share and re-read guards only | absent | UNAVAILABLE: no task-outcome signal in any record | the 2026-09-29 closeout found every behavioural hypothesis died on this | yes, negative | no | no | no | no | high | high | high | n/a | n/a | n/a | n/a | BLOCKED; the restart condition for every "improved the work" claim |
| Durable state and recovery | none | absent | commit prose and tree resume work (measured 2026-10-02) | strong, negative; 12 to 0 today | yes | no | no | no | no | low | low | low | low | high | none | n/a | REJECT, third time |

## Ranking

Ordered by observability, pain, intervention, outcome measurability,
evidence, leverage, revenue, risk, provider dependence, compounding.

1. Advisor share bound. Observed, intervenes on Replay's own output, outcome is the count printed, proven on real data. Shipped today.
2. Cache-TTL apply with recorded adoption and before-versus-after measurement. The one intervention Replay already performs; prediction measured at 28% on 755 sessions; the realized half is absent. Panel question B.
3. Compaction accounting. Shipped today; diagnosis, not control.
4. Learn, trial, graduate loop promotion. Complete by design, never run live, cannot be run here without spend. Panel question C.
5. Quota admission from provider headers. Captured, unread, no data here.
6. Codex depletion instant. Linear on this operator, rarely matters here.
7. Regression across periods (cost, context, quota pressure per week). Diagnosis only until an outcome signal exists.
8. Routing on capacity and capability. Capability and capacity are UNAVAILABLE; execution killed.
9. Waste interventions (stale, duplicate, redone). Incidence measured near zero on this machine.
10. Compaction approaching, durable state. Rejected by panel with evidence.

## Classification

- BUILD NOW: advisor share bound (done); cache-TTL adoption record and measurement, subject to the panel's conditions.
- INVESTIGATE: quota admission from headers (needs live proxied traffic with spend); policy provenance and experiment runner as the TTL measurement grows.
- DEFER: Codex depletion instant; routing; period regression.
- REJECT: stale-context, duplicate-read and redo-after-compaction interventions (incidence); compaction approaching signal; durable state.
- BLOCKED: any "the work got better" claim, on the missing task-outcome signal.
