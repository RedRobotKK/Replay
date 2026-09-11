# Built-but-unwired: running log

**Opened 2026-09-09.** A live register of capabilities that exist, pass tests, and
cannot be reached by a user — and of what was done about each.

The pattern was noticed after the fourth instance and confirmed after the ninth.
It is not a run of bad luck. It is a systematic gap between *written and tested*
and *reachable*, and the reason it survived is that **a passing test proves the
function works, not that anything calls it**.

## Status key

| | |
|---|---|
| **OPEN** | Confirmed unwired, nothing done yet |
| **GUARDED** | Still unwired, but a check now fails if it gets worse |
| **WIRED** | Reachable by a user, with a test that goes red if it stops being |
| **BY DESIGN** | Not a defect. Reason recorded |

## Confirmed, before the full audit

Found incidentally over 2026-09-08/09 while doing other work. Every one cited.

| # | Capability | Evidence | Consequence | Status |
|---|---|---|---|---|
| 1 | `LiveScreen` | Task #25 | Built, tested, unreachable | WIRED |
| 2 | Five TUI screens (`context`, `advise`, `guards`, `safe`, `model`) | [tui-example-screens](../evidence/tui-example-screens-2026-09-08.md) | Half the TUI described nobody; the installer opened it | WIRED |
| 3 | `PoolFits` second half | [#96](https://github.com/RedRobotKK/Replay/pull/96) | Wired, but quadrature downstream kept sigma's band at 86% | WIRED |
| 4 | `internal/quota/forecast.go` | Blue-team review | Projection implemented, imported by nothing | OPEN |
| 5 | `internal/quota` (whole package) | `go list -deps ./cmd/replay` → absent | Zero importers. The only consumer of `Record.Quota` | OPEN |
| 6 | `saveQuota` | Blue-team review | No production caller; the quota line always answers "no reading stored" | OPEN |
| 7 | `internal/proxy/preflight.go` | `Config.PreFlight` never assigned; `serve.go:136` omits it | **120 lines, a refusal kind and a counter that can never fire. Eight tests, all calling `s.preFlight` directly** | OPEN |
| 8 | `internal/usage` (`FromInclusive`, `Validate`) | Zero importers | **WIRED 2026-09-10.** `cmd/replay/costusage.go` imports it: `replay cost --usage` reads a usage export into `usage.Entry`, and `Validate` runs on every record at the door. It is a real guard there and was not one in the Codex reader — the export's `prompt` is written by whoever produced the file, not derived from the parts, so the two sides move independently and an inclusive-counted export is refused. The correction below still stands for why wiring alone would not have closed it in the reader | WIRED |
| 9 | `internal/otlp`, `internal/feed` | `go list -deps` → absent | Not determined; may be by design | OPEN |

## Adjacent defects found the same way

Not unwired, but the same family — a thing that looks connected and is not.

| Defect | Evidence | Status |
|---|---|---|
| `withUsageReporting` re-marshals the whole OpenAI body, on by default, contradicting `proxy-protocol.md:35` | Found independently by taxonomy parts 1 and 3 | OPEN |
| `transcript.Request` has no `Quota` field, so `requestFromRecord` cannot carry it | `store.go:249-259`, `types.go:122-139` | OPEN |
| `SessionBuilder.Add` skips records with nil usage, so 429s and 401s never become requests | `store.go:217-220` | OPEN |
| Retries discard failed-attempt headers; only the final attempt's survive | `retry.go:73,82`, `server.go:595` | OPEN |
| Tool reorder yields an empty prefix delta and falls to the unnamed default cause | `summarize.go:91` order-sensitive vs `causedetail.go:98-121` name-keyed | OPEN |

## Fixes, in order

1. **The guard** — stop the bleeding, catch the rest. *This branch.*
2. `withUsageReporting` splice — repairs a documented promise.
3. ~~Wire `usage.FromInclusive` into the Codex reader~~ — **done differently, and the plan was wrong.**
   `internal/usage` imports `internal/transcript`, so the reader cannot call
   `FromInclusive`; the subtraction is now in `codexUsage.usage()` itself.
   More importantly, wiring alone would have closed nothing. `Validate` compares
   `Fresh + read + write` against `Prompt`, and the reader's `Prompt` came from
   `PromptTotal()` — the same sum. Both sides moved together, so `Validate`
   returned nil for the exact defect this row calls it "the guard against", and
   the row would have been marked WIRED with the 1.94x intact. That is
   ADR-0018's "an oracle may not derive from the thing it checks", one level up.
   `TestUC3_WhatValidateCatchesAndWhatItCannot` states both halves. The guard
   only bites where `Prompt` is the provider's own figure, independently read.
   Wiring the package remains open, on its own merits.
4. Retry header capture (`Attempts` field) — unblocks the quota work.

## Audit in flight

Three agents, launched 2026-09-09, each using a different detection method because
the confirmed cases were found four different ways: exported symbols with no
non-test caller; config fields, flags and env vars nothing sets; unreachable
branches and documentation promising what the code cannot do. Results land here.

---

[Design](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
