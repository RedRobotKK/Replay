# QT-7a: a response with no usage now counts somewhere an operator can read

**2026-10-03. Post-1.0 qualification item QT-7a. A narrow observability
repair on the branch; v0.7.0 at `c883513` is untouched and the simulate
experiment's participants run that release.**

## The question

When a 200 response carries no `usage` object while a dollar cap is set, can
the operator see that the traffic was never counted?

## The path, traced before any change

- **Created.** The tap hands the body to the parser. A well-formed message
  with no `usage` key yields a response whose usage pointer is nil. A
  malformed body, a dropped body or a truncated gzip stream yield an entirely
  empty response, also nil usage.
- **Transformed.** The ledger record keeps the distinction, nil against a zero
  struct, which is why `replay cost` and `replay simulate` can report "without
  usage" after the fact.
- **Deciding site.** One line in the passthrough calls the spend guard only
  when usage is non-nil. The stats observer returns before counting anything
  when it is nil. The `unpriced` counter lives inside cost accounting, which
  the nil case never reaches.
- **Surfaced.** Nothing: no status field, no metric, no doctor line, no TUI
  line. The qualification pass measured thirty such responses under a dollar
  cap: none refused, cost $0, `spend_cap_not_enforced` false.

**Distinguishable today?** Missing usage and a malformed body are not
distinguishable at the deciding site; QT-7c owns that split and this change
does not attempt it. Missing usage and a measured zero are distinguishable in
the record, and after this change in the count. An unpriced model is a
different case with its own signal. A genuine zero-cost success with tokens is
counted as tokens.

## The RED tests, written before the change

`internal/proxy/qt7a_nousage_test.go`, on the real server path with a fake
upstream:

- Five 200s with no usage object under a $0.001 day cap: all forwarded, cost
  $0, the ledger confirms nil usage. FAIL: no `responses_without_usage` on the
  status endpoint, no `replay_responses_without_usage_total` on metrics.
- Three responses with a measured zero usage. FAIL: the field that would tell
  zero from missing does not exist.
- **Positive control, PASS before and after:** an unpriced model under the same
  harness and cap arms `spend_cap_not_enforced` and counts on metrics. The
  harness sees the signal that exists.
- **Negative control, PASS before and after:** priced usage counts cost and
  flags nothing.

`cmd/replay/qt7a_doctor_test.go`: a status carrying five no-usage responses
under a day cap, built from JSON so it compiled before the field existed.
FAIL: the doctor printed nothing.

## The change

| Where | What |
|---|---|
| `internal/proxy/passthrough.go` | At the deciding site: a 2xx on a readable path whose parsed usage is nil is counted, in the branch where the decision not to price it is made |
| `internal/proxy/state.go` | A `noUsage` counter; `responses_without_usage` on the status JSON, always present so zero reads as zero; `replay_responses_without_usage_total` on `/replay/metrics` |
| `cmd/replay/doctor_guards.go` | One line when the count is positive; a WARNING when a dollar cap is set: nothing was counted, so the cap did not see that traffic |
| `cmd/replay/tui.go`, `internal/tui/guards.go` | The guards screen carries the same field and says the same thing, urgent when a dollar cap is set |
| `docs/guide/commands.md` | The metrics table row |

Not changed: the parser, the guard, admission, pricing, refusals, the unpriced
signal, every other status key. The counter is not split between a missing
object and a malformed body; QT-7c decides that.

## Falsification

Seven hand mutants, each applied, tested and restored byte-identical:

| Mutant | Killed by |
|---|---|
| the increment removed (M123, frozen) | the status and metrics test |
| a measured zero folded into "no usage" | the measured-zero test |
| no-usage traffic recorded as priced against the guard (compiling mutant; a first attempt was a compile error and does not count) | the status and metrics test, through the cap refusing the next request |
| count dropped before the status field | the status and metrics test |
| count dropped before the metric line | the status and metrics test |
| doctor stops reporting | the doctor test and the cross-surface test |
| TUI stops reporting | the cross-surface test |

M123 `proxy-never-counts-a-response-without-usage` is frozen in the catalogue
and killed through the registered harness: `122 catalogued, 1 mutants: 1
killed, 0 survived, 0 stillborn`.

## Same count on every surface

The cross-surface test builds a status with five no-usage responses, renders
the doctor and the TUI guards screen from the same fields, and requires both to
say it. Status JSON and the metric are read from the same counter in the same
locked section.

## Classification

**CLOSED / PASS.** A real production signal exists, is created at the deciding
site, reaches the status endpoint, the metric, the doctor and the TUI from one
counter, distinguishes a missing usage object from a measured zero, and the
registered oracle kills every mutation in the falsification list.

## Regression

Measured on the working tree before commit: `go test ./...` 38 packages ok,
0 failed; `-race` 38 ok, 0 races; vet with and without the mutation tag clean;
`gofmt -l` nothing; `golangci-lint --new-from-rev` 0 issues on the new code;
`git diff --check` clean; docs guards and the regression package green; both
evidence matrices regenerated and unchanged (no dispatch surface or E2E-held
mutant changed).

