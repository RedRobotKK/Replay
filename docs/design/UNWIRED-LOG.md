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
| 10 | `selfupdate.StaleNotice` | `upgrade.go:131-141`; no non-test caller in the tree | Exported, tested, and reachable by nobody. The only way to learn a newer release existed was `replay upgrade --check`, a command you type when you already suspect the answer | WIRED |

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
5. `StaleNotice` into `replay doctor` — **done**, and the delay was the point.
   `upgrade.go:131-141` had removed the hint rather than carried it forward,
   because *where* an unprompted staleness line belongs is a product decision
   and that commit had arrived by accident. Row 10 is the deliberate answer:
   `doctor`, which is asked rather than volunteered, and which was already the
   surface ageing the rules document. Not the bare report, because README's
   Footprint promises one ask at most once every thirty days and a line on every
   run is one a reader learns to skip. Not the TUI doctor screen either — its
   worst case fills the body exactly (`TestDoctorWorstCaseKeepsEveryNote`), and
   an eleventh row there deletes the rules warning silently, which is the defect
   #169 had just finished fixing.

## 10. internal/surface, the cross-surface identification harness

Added 2026-09-26. Not reachable from `cmd/replay`, and that is the current
intent rather than an oversight.

It exists because the cross-surface observability audit
(`docs/evidence/cross-surface-observability-2026-09-26.md`) was produced by
throwaway scripts that no longer exist. Nobody could re-run them, and nothing
failed if a number in them was wrong. The package turns that audit into a probe
plus a contract registry, so a reviewer can re-derive the readings instead of
trusting them, and a wrong claim about a surface fails a test.

Its central invariant is that a corpus probe must never infer a provider's
economic class from transcript bytes alone. A corpus of populated cache reads
beside a cache-write counter that is zero on every record is observationally
identical under a provider that never writes and a client that drops the
counter. Codex is the instance. So `Classify` takes corpus evidence and an
independently sourced contract fact, and returns undetermined when the contract
is absent. Seven mutations that would upgrade a class on weaker evidence are
killed by the suite.

**What would wire it:** a command that reports what this machine's agent
surfaces can and cannot measure, which `replay doctor` is the natural home for.
That has not been proposed or authorised, so the package stays out of the binary
rather than growing a user-facing surface nobody asked for. The opt-in
real-machine probe (`REPLAY_PROBE_REAL=1`) is deliberately outside the hermetic
CI path, because its subject is the machine rather than the code.

## 11. internal/e005, the recovered scorer

Added 2026-09-26. Not reachable from `cmd/replay`, and correctly so: it
re-computes one dated experiment, and a user's install has no use for it.

It exists because E005's blinder and scorer were run as inline shell heredocs
and were never written to disk, while its corpus sat in a job directory that is
deleted with the job. The result was published and could not be re-derived by
anyone, including its author. The nine executed steps were recovered verbatim
from the session transcript, and this package is their analysis arm made
durable.

Its tests re-derive every figure in
`docs/evidence/detector-observables-2026-09-26.md` from two content-free
artifacts in `testdata/`, and fail if any of them moves. Seven mutations that
would silently alter a score are killed, including one that only fires on a
synthetic input because the real corpus never reaches that branch.

**What would wire it:** nothing should. If a second operator's corpus is ever
scored, it runs through this package in a test or a developer command, not
through the shipped binary.

## 12. internal/outputdiscipline, the re-derivation harness

Added 2026-09-26. Not reachable from `cmd/replay`, and correctly so: it
re-computes one dated benchmark and a user's install has no use for it.

It exists because `docs/evidence/output-discipline-2026-09-25.md` published two
tables without naming the run sets behind them, and the corpus holding those
runs holds nine experiment directories. A reader who swept the corpus got 54,418
cache-write for haiku verbose where the table says 53,700. The evidence was
correct and undefendable at the same time.

The harness names `bench/` (18 runs, first table) and `n10/` (40 runs, n=10
table), pins both fixtures by hash, and re-derives every column. Six mutations
that would silently move a figure are killed. It also surfaces what the tables
did not: six of 58 results are not bare numbers, and none is excluded, because
the published figures included them.

**What would wire it:** nothing should.

## Audit in flight

Three agents, launched 2026-09-09, each using a different detection method because
the confirmed cases were found four different ways: exported symbols with no
non-test caller; config fields, flags and env vars nothing sets; unreachable
branches and documentation promising what the code cannot do. Results land here.

---

[Design](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
