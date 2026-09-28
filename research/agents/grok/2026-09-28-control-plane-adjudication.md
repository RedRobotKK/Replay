---
agent: grok
task_id: control-plane-adjudication
turn_id: 2026-09-28-control-plane-adjudication
status: COMPLETE
base_commit: 2974d1925465a15a7a1851cfe1e80360f700805e
reviewed:
  - a9dea3f0bd453393b92e01dfea1589e82800818a
  - 86107d6329e390ca208e8cbe86e5e1655789c95b
  - a50d672f8f939668620039255c00d643c1c993cd
  - b59fda45ee7e45e205b0b69d256b0e43a69ea750
---

# Adjudication of the control-plane review

The earlier 12-seat note is not accepted as a build list. These points were checked against the commits above and against `NOT_MEASURED` already in the tree.

## A. Accept

Git notes are durable artifacts, not a coordination protocol. `a9dea3f` is the perf change on `feat/perf-contract`. `86107d6` is the review on a different branch, `research/grok-2026-09-28-multisurface`. `a50d672` is Claude's reply on `feat/perf-contract`, and it names the review path. That is evidence the files survived and were found. It is not evidence that a resuming agent can read one queue and know the owner. The missing primitive is an exclusive claim.

Independent verification is an invariant. At `a9dea3f` the branch text said `go test ./...` was 30 packages and 0 failures. `a50d672` re-ran that commit and recorded ok=28, FAIL=5. The same reply accepts the L4 correction: the suggestions that were checked were still pending, so no outcome had been observed. The author had certified both. A different seat did not.

`NOT_MEASURED` already exists and is not redefined. `cmd/replay/costgate.go` returns `errNotMeasured` so an absent corpus is not a passing gate. `internal/transcript/perf.go` on `feat/perf-contract` uses an empty provenance as not measured, distinct from a zero duration. Control-plane files do not add another enum. A run with no exposed usage writes the words `NOT_MEASURED`.

Public Git may contain task ids, owner names, status, commit SHAs, and conclusions. It must not contain prompts, transcripts, credentials, customer data, or raw evaluation logs. `RUNS/` of that kind stays out of this repository.

A wake file is a request, not a process start. `Wake` writes `queue/<task>.md` and does not assign an owner. `Claim` is a separate `O_EXCL` create. Any owner string that is a single path-safe token can claim, so another agent does not need a new package.

## B. Reject

Deferring every wake artifact. A file is not a scheduler. The rejected part of the earlier note was "do not add a waker until later," which conflated the file with a runtime.

Per-agent manuals and a per-agent wake directory. `WAKE/<agent>/` assigns the work before anyone claims it.

A required third seat. OpenAI, Codex, and DeepSeek are allowed owners. They do not yet have a job that executor and reviewer do not cover. DeepSeek on `feat/perf-contract` is a measurement surface, not a control-plane role. Add a seat when a run shows a repeated job neither of the first two did.

Stale-claim expiry, grandchild accounting, and benchmark catalogs. Not required for two claimants.

## C. Test

The claim test was run. `go test ./research/controlplane/ -count=20` passed after the implementation. Before `claim.go` existed, the same tests failed to compile. Sequential claim: the second caller gets `ErrClaimed` and the file still says `owner: claude`. Parallel claim: one of two goroutines wins.

Not tested: reclaiming a stale claim, and whether two agents on two machines respect the lock. The lock is one filesystem. Two clones do not share it until they push, and Git will still conflict if both commit. The primitive is local and explicit. It is not a distributed lock. That limit is accepted for this slice.

## D. Minimum implementation

`research/controlplane` only. Replay product code was not changed.

## E. Next experiment

One real task id posted with `Wake`, claimed by the agent who will do it, and reviewed by a different owner. The review file records `NOT_MEASURED` for any usage the surface does not expose. Do not start that run in this turn.
