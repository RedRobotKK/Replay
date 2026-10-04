# QT-7b: nine surfaces said an unpriced model is free; the code charges it at the ceiling

**2026-10-03. Post-1.0 qualification item QT-7b, closed. No admission decision
changes. No participant of the simulate experiment is affected: they run v0.7.0,
and this lands on the branch behind it.**

## The defect

The guard prices a model the price table does not carry at the dearest known
rate and labels the figure an upper bound (#261, `TOKEN-PRICES.md`), so a dollar
cap fires early on such traffic rather than never. The qualification pass
(`replay-1.0-qualification-2026-10-03.md`, QT-7b) found the guide saying the
opposite. Pulling the thread found nine surfaces describing the behaviour the
code had been changed away from, every one of them user-facing or read by the
next maintainer:

| Surface | Said | Says now |
|---|---|---|
| `replay serve --max-session-usd` help | "models not in the price table count as free" | "a model not in the price table is priced at the dearest known rate, as an upper bound" |
| `replay serve --max-day-usd` help | nothing | the same clause |
| `docs/CLI.md` | generated from the above | regenerated |
| `docs/guide/commands.md`, the cap row | "A model not in the table counts as free" | charged at the dearest known rate, as an upper bound, so the cap fires early rather than never |
| `docs/guide/alerting.md`, the dollar-cap alert | heading "is not being applied"; "runs up a real bill without ever reaching the limit" | heading "is counting some traffic at an upper bound"; the cap can fire early and the figures overstate that traffic until the model has a price |
| `replay doctor` WARNING | "the cap is not being applied to it ... without ever reaching your limit" | charged at the dearest known rate, as an upper bound, so the cap can fire early and the figures above overstate it |
| TUI guards screen and spend cell | "NOT enforced on some traffic ... bills with no limit reached"; "cap NOT enforced" | unpriced traffic charged at the dearest known rate, as an upper bound, so the cap can fire early; "cap on an upper bound" |
| TUI storyboard | "this cap cannot be reached. Unpriced requests add nothing to the total" | unpriced requests charged at the dearest known rate, as an upper bound, so this cap can fire early |
| `docs/DASHBOARD-DESIGN.md`, `docs/TUI-FLAG-SURFACE.md` | "the cap cannot be reached", "the limit will never be met", "the user set a cap and does not have one" | the same rule as above |

Two code comments said it too (`SpendGuard.CapNotEnforced`, the status
endpoint) and now describe the rule. The status key `spend_cap_not_enforced`
and the method name are kept: they are what readers and tests already look for,
and the comments now say the name predates the rule.

An operator reading any of the nine would have expected a cap that never fires
on a new model and got one that fires early, with no document to explain why.
That is the inverse of the defect the dearest-row rule was introduced to fix,
produced by fixing the code and not the sentences about it.

## The guard

`internal/regression/qt7b_unpriced_rule_test.go` ties every statement to the
code. Its positive control calls the production pricing function on a model no
table carries and fails unless the result is an upper bound above zero. It then
reads each surface's passage and fails if it says the traffic is free,
uncounted, unreachable or not applied, or does not say "upper bound". If the
code's rule ever changes, the positive control fails first and the surfaces are
rewritten with it.

## Gates

| Gate | Measured |
|---|---|
| RED | The guard failed on the committed tree: five code and guide surfaces with twelve findings (the two TUI files and the two design documents were added to it after the first run surfaced them, and failed in turn) |
| GREEN | After the rewrites, the guard passes; `cmd/replay`, `internal/proxy`, `internal/tui` and `internal/regression` pass; four tests pinned to the old label (`doctor_guards_test.go`, `doctorcount_test.go`, `tui/guards_test.go`, `tui/guardcells_test.go`) now assert the rule; the second of them is the cross-surface check that the doctor and the TUI say the same thing, which they again do |
| MUTATION | M122 `serve-help-says-unpriced-models-are-free` frozen (the session cap's help reverts to "count as free") and killed through the registered harness: `121 catalogued, 1 mutants: 1 killed, 0 survived, 0 stillborn` |
| SEMANTICS | No change to admission: `listCost`, the guard, the refusal reasons and the status JSON are untouched; `go test ./internal/proxy` green unchanged |
| REGRESSION | Recorded in the commit message: full suite, race, vet both tags, gofmt, lint on the new code, diff-check, docs guards, CLI blueprint regenerated, both evidence matrices regenerated |

## Not done

QT-7a (a response with no `usage` advances no cap and raises no signal) and
QT-7c (a malformed body passes as 200 with no usage and no signal) are still
open. They need a signal, not a sentence, and are the next item.
