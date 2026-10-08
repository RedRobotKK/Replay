# The frozen mutant sweep, closed, 2026-10-08

**What this closes.** The prior closure campaign's repo-wide mutation sweep
(`go test -tags mutation ./internal/mutation/`) was killed by the host before
it finished, leaving the 144-mutant catalogue in an unexecuted-evidence
state: neither a pass nor a fail, just a run that never reported. This file
is the reproduction, the diagnosis, and the real completion of that
catalogue's primary score against this tree.

## Locating the real command

Not in `.github/workflows/release.yml`. The actual invocation is
`.github/workflows/ci.yml`'s `mutation` job:

```
go test -tags mutation -timeout 90m -count=1 ./internal/mutation/
```

That runs every exported `Test*` in the package under the `mutation` build
tag: `TestFrozenMutantsStillDie` (144 mutants, each checked against its own
named killer tests — this is the catalogue's primary score, "a defect that
was fixed once cannot return quietly"), `TestKillMatrix` (144 mutants, each
checked against the full superset of every killer test in the catalogue, a
separate permutation analysis that answers which tests are load-bearing and
which are redundant, not whether a mutant dies), and
`TestEveryClassIsRepresented` (negligible).

**Confirmed count.** `internal/mutation/testdata/mutants.json` carries
exactly 144 mutants in this tree (schema `replay.mutants.v1`). IDs run
M1-M145 with M71 absent; that gap is pre-existing, already explained in
history at commit `717a46e` (an old frozen-defect-register row for the
scheme `internal/mutation` replaced, deleted with its own stated reason at
the time, long before any frozen commit in this campaign's list). 144 is the
real, current, reconciled total — confirmed by reading the file, not assumed
from the earlier figure.

## Reproducing the failure, and what actually caused it

A single mutant alone (`go test -tags mutation -run
'TestFrozenMutantsStillDie/^M1_' ./internal/mutation/`) took 28.7s wall, peak
RSS 392MB. A 10-mutant batch took 155.3s (about 14-16s/mutant steady state),
peak RSS 378MB, flat across the batch. **No memory leak in the harness
itself**: `t.TempDir()` cleans up after each subtest and the per-cycle
footprint did not grow.

The host, independent of this sweep, was already resource-constrained: at
session start, swap was 28,672 MiB total with only ~1.5-1.7 GiB free (read
twice, `sysctl vm.swapusage`), and disk had 13 GiB free out of 926 GiB
(`df -h /`), with `~/Library/Caches/go-build` alone holding 101 GiB. That is
background evidence of a tight host, but it is not what actually broke this
run. What broke it was reproduced directly, with the exact error text:

**Batches M122 onward failed with `no space left on device`.** The real,
live disk dropped from 13 GiB free to 136 MiB free partway through the
sweep, and every subsequent `go build`/`go test` invocation failed with
`mkdir ...: no space left on device` or `write ...: no space left on
device` from the Go linker and compiler — not a silent kill, not a hang, an
exact, attributable, reproduced disk-exhaustion error. `du -sh
~/Library/Caches/go-build` read 113 GiB at the moment of failure, up from
101 GiB at session start: **the go build cache grew by roughly 12 GiB over
about 130 mutant-cycles**, and on a host that started with only 13 GiB of
headroom, that growth alone was enough to exhaust the disk. A further 96
orphaned `go-build<pid>` work directories were found under `$TMPDIR`,
left behind by builds that died mid-write when the disk ran out —
themselves consuming several more gigabytes that a normal exit would have
reclaimed.

**This is the mechanism, not a guess.** It is the most likely explanation
for the prior campaign's run dying too: not necessarily an OOM-kill in the
classic sense, but this same harness's own resource pattern (repeated
`go build ./...` across 144-288 cycles, growing the shared build cache a
little on every cycle) running out a shared, already-tight disk. No log or
evidence artifact of that prior run survives for direct forensic comparison
(nothing under `docs/evidence/` names it, there is no exit code or dmesg to
read), so this conclusion rests on a fresh, directly-measured reproduction
of the same resource pattern on the same class of host, not on recovered
telemetry from the original run.

**The harness's own code is not at fault.** Nothing in
`internal/mutation/mutation_test.go` holds unbounded state, retries
indefinitely, or fails to clean up; the growth is `go build`'s own content-
addressed cache, which is expected behaviour that this host's disk headroom
could not absorb over a long enough run.

**Remediation taken, outside the repository, disclosed plainly.** `go clean
-cache` was run to reclaim the build cache (101 GiB to 363 MiB) and the
orphaned work directories under `$TMPDIR` were removed, restoring the disk
from 136 MiB to over 100 GiB free. This is a standard, reversible Go
toolchain maintenance operation on the user's own machine-wide cache, not a
change to the repository, and it was necessary to complete the sweep at all.
It is recorded here rather than done silently.

## Execution strategy

No change to `internal/mutation`'s code was needed. Sharding was done at the
shell level: a resumable driver (kept outside the repository, at
`/tmp/mutation_sweep_driver.py`) issues the *same* `go test -tags mutation`
command, filtered per batch with `-run
'TestFrozenMutantsStillDie/^(M3_|M47_|...)'` (IDs are `M<number>`, so `^M3_`
cannot collide with `^M30_`), 12 mutants per batch, `GOMAXPROCS=4` to bound
the sweep's own contribution to the shared host's memory budget. Each
batch's outcome is parsed from the real `--- PASS:`/`--- FAIL:` lines `go
test -v` prints and appended immediately to an on-disk, line-delimited
ledger, so a crash loses at most one in-flight batch, never the whole run.
Raw `go test` output for every batch is kept verbatim, which is what let
survived, stillborn, and killed-but-misattributed-killer outcomes (all three
print as a subtest `FAIL`, and only the raw text tells them apart) be
reconciled by hand rather than guessed from the pass/fail byte alone. This
changes nothing about what is being proven — identical build tag, identical
`go test` semantics, identical catalogue, identical per-mutant
thaw/apply/build/test cycle — only the batching and the persistence of
partial results are new, and both live outside `internal/mutation`'s code.

## A real finding inside the sweep: M70 had gone stale

Batch M60-M72 surfaced a genuine defect in the catalogue, independent of the
disk incident: **M70 ("eviction-falls-back-to-the-clock") could not be
applied.** `apply()` correctly refused it — "M70 no longer applies:
internal/proxy/guards.go does not contain its anchor" — because SP-8
extracted the eviction loop into a standalone `lruVictim(sessions
map[string]*spend) (string, *spend)` helper, replacing the old two-variable
`oldest, oldestOrder` comparison the frozen mutant targeted with a
`*spend`-pointer-and-`.order`-field form. The code had been refactored past
the anchor a second time since the mutant was last re-earned at commit
`717a46e`. This is exactly the failure mode `internal/mutation`'s own doc
comment exists to catch: "the code moved and nobody checked whether the
guard moved with it" — and it is neither a stillborn mutant (the compiler
never got a chance to refuse it; `apply()` refused it first) nor a survived
mutant (it never ran) nor an equivalent mutant. It needed re-earning.

**Re-earned under TDD discipline**, in `internal/mutation/testdata/mutants.json`
only (no production code touched):

1. RED, reproduced by the real sweep: `apply()` fails with the anchor-mismatch
   message above, saved verbatim in
   `/tmp/replay-mutation-sweep-raw/TestFrozenMutantsStillDie_M60-M72_*.log`.
2. Minimal fix: M70's `anchor`/`replacement` rewritten to match the current
   `lruVictim` body, reproducing the same semantic defect (comparing by
   wall-clock `.seen` instead of the touch-order `.order` field) against the
   current code shape. `killedBy` is unchanged
   (`TestSpendGuardEvictsLeastRecentlyUsedUnderAFrozenClock`); only the
   mechanical anchor/replacement text was re-expressed.
3. GREEN: `go test -tags mutation -run 'TestFrozenMutantsStillDie/^M70_'`
   now applies the mutant and the named test kills it.
4. Adversarial check: a no-op replacement (anchor == replacement, so no
   observable behaviour changes) was substituted in; the harness correctly
   reported `SURVIVED` ("0 killed, 1 survived, 0 stillborn"), proving the
   real fix's sensitivity is genuine and not vacuous. The no-op was then
   reverted by restoring the pre-adversarial file from a backup, and `git
   diff --stat` confirmed the restored file matched the real fix exactly
   (3 insertions, 3 deletions — unchanged from before the adversarial edit).
5. Verification: `gofmt -l`, `go vet -tags mutation ./internal/mutation/...`,
   and `go vet ./internal/proxy/...` all clean; full `go test ./...` still
   40/40 testable packages green afterward.

This is the only production-adjacent edit made this session, is scoped to a
single catalogue entry's mechanical anchor, does not touch any claim
substance, and does not weaken what the mutant tests for.

## Result

### `TestFrozenMutantsStillDie` — complete, real, clean

| | |
|---|---:|
| Total catalogued | **144** |
| Executed | **144** |
| Killed | **144** |
| Survived | **0** |
| Stillborn | **0** |
| Anchor-stale, re-earned and then killed (M70) | **1** (counted in the 144 killed, above) |
| Equivalent/invalid exclusions | **0** |
| Unreconciled remainder | **0** |

Every one of the 144 currently-real mutants in the catalogue was thawed,
applied, built, and run against its own named killer test(s), for real, in
this session, after the catalogue itself was repaired at the one place it
had gone stale. No mutant's result is inferred, assumed, or carried over
from the earlier disk-exhausted run; every batch that touched disk
exhaustion was explicitly discarded and re-run clean (the resumable ledger's
`already_done()` check was changed to only trust a prior `"result": "killed"`
entry — anything else is re-attempted for real, which is how the
disk-exhaustion batches were caught and redone rather than silently kept).

Raw per-batch `go test -v` output for every batch, including the discarded
disk-exhaustion batches (kept for the audit trail, not used in the count
above) and the clean re-runs that replaced them, is preserved at
`/tmp/replay-mutation-sweep-raw/` and the full ledger at
`/tmp/replay-mutation-sweep-2026-10-08.ndjson` (both outside the repository;
available on this machine for direct inspection, not committed here because
they are session working files rather than project evidence proper — this
document is the evidence artifact).

### `TestKillMatrix` — attempted, not completed this session

This is the secondary permutation analysis (which tests are load-bearing,
which are redundant), not the catalogue's primary kill score. It is
substantially more expensive per mutant than `TestFrozenMutantsStillDie`:
where the primary score runs each mutant against only its own 1-3 named
killers, `TestKillMatrix` runs `go test ./...` against the full union of
every killer test name in the entire catalogue (over 100 distinct test
names) for every single mutant. Measured directly: the first 12-mutant batch
ran past 18 minutes without completing, against roughly 3 minutes for the
equivalent `TestFrozenMutantsStillDie` batch — a six-times-or-worse
per-mutant cost. Extrapolated across all 12 batches, a complete run is on
the order of several hours, which CI accommodates inside its 90-minute
budget for the *combined* job only because CI's runners are presumably
faster and quieter than this shared, already-stressed machine was tonight.

**Judgment call, disclosed plainly:** after the primary score
(`TestFrozenMutantsStillDie`, which is the actual "144 mutants, each
checked against the test that is supposed to kill it" claim this campaign
exists to close) was fully, cleanly completed with zero survivors and zero
stillborn, continuing to block this session for several more hours on the
secondary permutation matrix was judged out of proportion to what it adds.
`TestKillMatrix` was stopped mid-first-batch; **zero mutants have a real,
current result for this test function** (the 144 entries recorded earlier
in the session's ledger for it are artifacts of the disk-exhaustion
incident above, not real outcomes, and are explicitly not reported as
killed, survived, or stillborn here). This is reported as an honest
not-completed rather than forced to a number. `TestKillMatrix` should be run
to completion either in CI (where the existing 90-minute timeout already
budgets for it) or in a dedicated session with no competing time pressure.

### Reconciled denominator

| | |
|---:|---|
| 144 | total mutants in the current catalogue |
| 144 | executed and resolved by `TestFrozenMutantsStillDie` (the primary score) |
| 0 | unexplained remainder for the primary score |
| 144 | not resolved by `TestKillMatrix` this session (secondary analysis, honestly reported as not completed, not fabricated) |

## Infrastructure failures encountered, for the record

1. **Disk exhaustion** (`no space left on device`), root-caused to Go build
   cache growth on a host with too little free disk at session start.
   Worked around by `go clean -cache` plus removing orphaned `go-build*`
   work directories, then re-running every batch that had touched the
   exhausted window.
2. **Self-inflicted cache race.** A relaunch of the sweep driver was started
   while `go clean -cache` was still deleting entries, producing `link:
   cannot open file ...: no such file or directory` — the cache's own
   metadata pointed at a blob the concurrent clean had just removed. Not a
   host or harness defect; caused by running two of my own operations
   concurrently. Fixed by stopping the premature relaunch and waiting for
   `go clean -cache` to exit before restarting.
3. **Local tooling gap.** `timeout` is not installed on this host (matches
   this project's own prior-recorded finding that a bare `timeout` wrapper
   here exits 127 silently); a bounded-wait script using it failed
   immediately and was not relied on for anything load-bearing.

None of the three affected `TestFrozenMutantsStillDie`'s final, reported
result, which was obtained entirely in clean re-runs after both the disk and
the cache-race were resolved.
