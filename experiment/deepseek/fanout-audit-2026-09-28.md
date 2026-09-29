# Fan-out architecture audit

2026-09-28. Subject: `experiment/harness/fanout.py` as committed at `5de2ca2`.

Four defects, each verified before this file was written. The module was safe --
it could not overspend a ceiling it never reached, and it never touched
production Replay code -- and it was not good. It would have worked correctly
and cost roughly 20x what it should on exactly the workload it was built for.

## F-A. Parallel fan-out defeats prefix caching

The architectural defect. `grep -i 'warm\|prefix\|sequential' fanout.py` returned
zero matches at `5de2ca2`.

DeepSeek's cache is populated by a *completed* request. Requests issued
concurrently all start before any has completed, so every one of them misses on
the shared prefix and pays full input price for it. The cache is then populated
with a prefix that nothing left in the run needs.

Computed on the published per-token rates, prefix shared by every question:

| questions | shared prefix | all-parallel | warm-then-fan | ratio |
| ---: | ---: | ---: | ---: | ---: |
| 30 | 10,000 tok | $0.04500 | $0.00237 | 19.0x |
| 64 | 30,000 tok | $0.28800 | $0.01017 | 28.3x |

These are DERIVED from published rates and a token count, not OBSERVED against
the billing endpoint. The campaign is PAUSED, so no call was made to confirm
them. What is OBSERVED is the absence of any warming in the source.

Fix: run one real question alone, let it complete, then fan out the rest. The
warm call is a deliverable, so warming costs latency and never an extra call.

## F-B. The budget ceiling was advisory, not enforced

`Budget.reserve()` was a read-modify-write with no lock:

    if self.committed + self.est > self.ceiling: ...
    self.committed += self.est

Two threads can both read a committed figure under the ceiling and both commit.
Demonstrated: a ten-slot budget granted **9** reservations where 10 was the
correct cap, under contention, and would drift further with more workers.

This is the money-correctness defect. Scarce personal R&D capital was the stated
constraint for this whole campaign, and the guard protecting it did not hold.

Fix: every mutation holds a `threading.Lock`. Reservations settle against
provider-reported cost rather than staying at the estimate.

## F-C. One unhandled exception discarded every paid result

`rows = list(ex.map(ask, questions))`. `Executor.map` re-raises the first
exception when the result iterator reaches it and discards every later result.
Only `adapter.Truncated` was caught inside `ask`; a connection reset, a JSON
decode failure or a checker importing something missing would throw away calls
already paid for.

Fix: `submit` / `as_completed`, resolving each future on its own, with a row
emitted for every question including the ones that failed. A missing row is
indistinguishable from a question nobody asked.

## F-D. One curl subprocess per call

The transport spawned a process and performed a fresh TLS handshake per request.
At 64 concurrent questions that is 64 spawns and 64 handshakes. Adequate for the
sequential probes it was built for; wrong for measuring a provider whose claimed
advantage is concurrency, because the measurement would describe process startup
as much as the provider.

Fix: a pooled `requests.Session` (2.32.5, present), pool sized to the worker
count so the pool cannot silently serialise the fan-out.

## Not fixed, and why

Cache-hit pricing remains unvalidated. Both reconciliation batches reported
`cache_read = 0`, so the warm-then-fan saving above rests on published rates and
has never been observed end to end. The first real fan-out run is also the
experiment that would settle it. Until then the ratio is DERIVED.

## Instrument failure found while verifying the fixes

The first mutation sweep reported M1 (remove the lock), M3 (`ex.map`) and M5
(float money) as SURVIVED, and then reported the *restored, unmutated* file as
failing. A file that fails after being restored to a passing state is the
instrument talking, not the code.

Cause: `cp` gives the restored file a new mtime that can fall in the same whole
second as the mutant's cached `__pycache__` entry, so CPython reuses the
mutant's bytecode. Some mutation results were therefore measured against the
previous version's compiled code.

The sweep was redone with `PYTHONDONTWRITEBYTECODE=1` and `__pycache__` removed.
All ten mutations then killed their tests. Three of the original SURVIVED
verdicts were nevertheless real and were fixed before the redo:

- M1 survived because CPython's GIL makes the unlocked read-modify-write window
  too small to hit by contention alone. Replaced with a probe that subclasses
  `Budget` and puts a preemption point inside the critical section: timing only,
  `reserve()` inherited unchanged.
- M3 survived because `ask()` catches `Exception`, so no ordinary failure ever
  reaches the executor and `map` and `as_completed` behave identically. Replaced
  with a checker that calls `sys.exit()`: `SystemExit` is a `BaseException` and
  passes straight through `ask()`, which is the case the control exists for.
- M5 survived because floats at nano scale happen not to drift at that
  magnitude. Replaced with a direct assertion on the representation plus four
  ceiling ladders chosen to drift under float summation.

This is the same class of defect as the earlier `go test` caching error, where
separate cached invocations were counted as one clean run. Both times the
harness reported a state the code was not in. Count from one captured run, and
disable every cache the runner controls before trusting a differential result.
