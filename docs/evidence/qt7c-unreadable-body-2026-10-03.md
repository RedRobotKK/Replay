# QT-7c: a body nobody could read is now told from a message that carried no usage

**2026-10-03. Post-1.0 qualification item QT-7c. Observability only, on the
branch behind v0.7.0 at `c883513`; the simulate experiment's participants run
that release.**

## The question

Can the running proxy tell a 2xx body it could not read at all from a
well-formed message that simply carried no usage object?

## The path, traced before any change

The Messages parser returns an entirely empty response for a body that is not
JSON or whose type is not `message`, which includes a provider error object
behind a 200. The OpenAI parser returns the same for a body that is not JSON.
The tap returns the same for a body it dropped past the size cap, a body
announced as gzip that did not open, and a gzip stream that ended early. A
well-formed message with no `usage` key returns content blocks and a nil usage
pointer. Nothing after the parser carried a "parsed" bit, and content blocks
are not a usable proxy for one, because a message with no content and no
usage is legal. After QT-7a (`40bed0a`) every unreadable body was therefore
counted as "without usage".

The two states need different operator actions. A provider that omits usage
wants a usage-reporting option or a price-table update. A body nobody could
read is a defect between client, proxy and provider.

## RED, before the change

`internal/proxy/qt7c_malformed_test.go`, on the real server path with a fake
upstream, under a $0.001 day cap:

- Four non-JSON bodies: all forwarded, the ledger parsed nothing, and the
  status reported `responses_without_usage` 4 with no unparsed count and no
  metric line. FAIL.
- Two JSON error objects behind 200: counted as "without usage". FAIL.
- Three truncated bodies: counted as "without usage". FAIL.
- The ledger record for an unreadable body serialised as `{}`, no mark of why.
  FAIL.
- **Positive control, PASS before and after:** a parsed message without usage
  is counted under the QT-7a key and not under the unparsed one.
- **Negative control, PASS before and after:** priced usage raises neither.

`cmd/replay/qt7c_doctor_test.go`: a status carrying four unreadable bodies
under a day cap, built from JSON so it compiled before the field existed. The
doctor said nothing. FAIL.

## The change

| Where | What |
|---|---|
| `internal/ledger/record.go` | `Unparsed bool`, omitted when false, on the response record: the body could not be read as a response at all |
| `internal/ledger/response.go`, `openai.go` | Set in the parsers' not-JSON branch and, on the Messages path, the not-a-message branch. Never inferred from an empty result |
| `internal/proxy/tap.go` | Set where the tap already knows the body is incomplete: dropped past the cap, gzip that did not open, gzip cut short |
| `internal/proxy/passthrough.go` | At the QT-7a deciding site, the nil-usage branch splits on the mark: unreadable to a new counter, the rest to the existing one |
| `internal/proxy/state.go` | `unparsedBody` counter; `responses_unparsed` on the status JSON, always present; `replay_responses_unparsed_total` on metrics, with its help line naming the difference from `replay_unparsed_requests_total`, which counts paths this build cannot read |
| `cmd/replay/doctor_guards.go`, `cmd/replay/tui.go`, `internal/tui/guards.go` | A line when the count is positive, WARNING when a dollar cap is set, the same field on both surfaces |
| `docs/guide/commands.md` | The metrics row, and the QT-7a row narrowed to "parsed as a response" |

Not changed: cost, the guard, admission, pricing, refusals, the unpriced
signal, `replay cost` and `replay simulate`, which keep reporting "without
usage" as before; the record field is their provenance for a later split.
Event streams that decode no usage frame stay under "without usage"; marking a
stream that decoded no event at all would need the stream parsers to count
events, and is not in this change.

## Falsification

Twelve hand mutants, each checked to compile before it was run, then tested
and the file restored byte-identically. One first attempt with an anchor that
matched twice was discarded and rerun with a unique anchor.

| Class | Mutant | Killed by |
|---|---|---|
| mark never set | parser returns an unmarked empty response | the non-JSON and record tests |
| mark never set | OpenAI parser likewise | the parsers test |
| mark never set | tap: dropped body unmarked | the tap test |
| mark never set | tap: unopenable gzip unmarked | the tap test |
| mark never set | tap: truncated gzip unmarked | the tap test |
| mark set everywhere | parser marks a message that parsed | the parsers test |
| split dropped (M124, frozen) | every nil usage counted as "without usage" | the non-JSON test |
| branches swapped | unreadable counted as "without usage" and the reverse | the non-JSON test and the positive control |
| dropped before the surface | status field forced to zero | the non-JSON test |
| dropped before the surface | metric line forced to zero | the non-JSON test |
| surface stops reporting | doctor block disabled | the doctor test and the cross-surface test |
| surface stops reporting | TUI block disabled | the cross-surface test |

M124 `proxy-counts-an-unreadable-body-as-missing-usage` is frozen in the
catalogue and killed through the registered harness together with M123:
`123 catalogued, 2 mutants: 2 killed, 0 survived, 0 stillborn`.

## Same count on every surface

The cross-surface test builds a status with five no-usage responses and four
unreadable bodies, renders the doctor and the TUI guards screen from the same
fields, and requires both to report both. Status JSON and the metric read the
same counter under the same lock.

## Verified before declaring PASS

Non-JSON, error object behind 200, truncated JSON and truncated gzip count as
unparsed and not as without usage. A valid message with no usage counts as
without usage and not as unparsed. Priced usage counts under neither. The
unpriced-model signal is unchanged. Status and metrics agree; doctor and TUI
agree; the ledger record for an unreadable body carries `unparsed: true` and
the record for a parsed message does not. The QT-7a tests pass unchanged.

## Regression

Measured on the working tree before commit: `go test ./...` 38 packages ok,
0 failed; `-race` 38 ok, 0 races; vet with and without the mutation tag clean;
`gofmt -l` nothing; `golangci-lint --new-from-rev` 0 issues on the new code;
`git diff --check` clean; docs guards and the regression package green; both
evidence matrices regenerated and unchanged.

## Classification

**CLOSED / PASS.** The distinction is created where the parser or the tap
knows it, reaches the deciding site, the status endpoint, the metric, the
doctor, the TUI and the ledger record, and every mutation in the
falsification list is killed by a registered test.
