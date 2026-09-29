---
turn: 2026-09-28-fanout-production
agent: claude-opus-5
spend_usd: 0.00
network_calls_to_provider: 0
campaign_status: PAUSED
---

# Fan-out rebuilt to production shape

## What changed

| File | Change |
| --- | --- |
| `experiment/harness/fanout.py` | rewritten: warm-then-fan, locked budget in integer nano-USD, `submit`/`as_completed` |
| `experiment/harness/transport.py` | new: pooled `requests.Session`, credential read from the existing 0600 curl config |
| `experiment/harness/test_fanout.py` | rewritten, 12 tests |
| `experiment/harness/test_transport.py` | new, 7 tests |
| `experiment/deepseek/fanout-audit-2026-09-28.md` | new: the four defects, with measurements |

No Replay production source was touched. No provider call was made.

## Defects fixed

F-A parallel fan-out defeats prefix caching (19-28x on the workload it was
built for). F-B the budget ceiling was advisory, not locked. F-C one unhandled
exception discarded every paid result. F-D one curl subprocess per call.

Full statement with measurements: `experiment/deepseek/fanout-audit-2026-09-28.md`.

## Defect found by the new tests

Money was held as `float`. Ten additions of `0.002` sum to
`0.020000000000000004`, over a ceiling of `0.02`, so the tenth call of a
ten-call budget was refused. Wrong in the safe direction, still wrong. Money is
now integer nano-USD.

## Verification

Ten mutations, each compiled and run, all killing at least one test:

| # | Mutation | Result |
| --- | --- | --- |
| M1 | remove the lock from `reserve()` | killed |
| M2 | never warm | killed |
| M3 | `ex.map` instead of `as_completed` | killed |
| M4 | count undecided outcomes as failures | killed |
| M5 | float money instead of integer nano-USD | killed |
| M6 | a truncated call releases instead of settling | killed |
| T1 | split a header on every colon | killed |
| T2 | accept a credential config with no headers | killed |
| T3 | default connection pool regardless of worker count | killed |
| T4 | silent HTTP retries | killed |

The first sweep was VOID: `cp` restored files with an mtime inside the same
whole second as the mutant's `__pycache__` entry, so CPython reused mutant
bytecode. Redone with `PYTHONDONTWRITEBYTECODE=1`. Three of the first sweep's
SURVIVED verdicts were real and the tests were replaced before the redo; see
the audit file.

Python: `test_fanout.py` 12/12, `test_transport.py` 7/7.
Go, counted from ONE captured run (`go test ./...`, exit 0): ok=30, FAIL=0,
one package with no test files. `gofmt -l .` empty.

## Not done, and why

No fan-out has been run against the provider. The campaign is PAUSED pending
credential rotation, and the 19-28x saving is DERIVED from published rates, not
OBSERVED. Cache-hit pricing is still unvalidated: both reconciliation batches
reported `cache_read = 0`. The first real fan-out is also the experiment that
would settle it.
