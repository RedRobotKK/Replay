# 20. Compile the merge, not the branch

**Status:** Accepted
**Date:** 2026-09-11

## Context

Every check in this repository runs against a branch head. Nothing ran against
the result of merging that branch into its base. So two changes that were each
green could produce a `main` that did not build, and the first evidence of it
was a red run on `main` itself.

That happened twice.

**#157 with #160.** Each respected the TUI row budget alone. Together the doctor
screen needed 21 rows against a budget of 20, and `pad*()` truncates silently
from the bottom, so the row that vanished did so without complaint. It was
repaired by #169.

**#184 with #185.** The first carried `internal/transcript/zzprof_test.go` to
`main`. The second added `parse_bench_test.go`, which is that same file renamed — and the
rename's delete side was dropped when #185 was rebased onto a `main` that had
meanwhile taken the first. Both PRs were 14/14 green. The merge would not
compile:

```text
internal/transcript/zzprof_test.go:8:6: BenchmarkParseRealTranscript redeclared
    internal/transcript/parse_bench_test.go:26:6: other declaration of it
FAIL    github.com/RedRobotKK/Replay/internal/transcript [build failed]
```

Repaired by #191, but `main` was red in between, and every branch opened during
that window inherited a broken base.

Three things about the second incident decided the design, and each of them
rules out an approach that looks obvious first.

**The merge is clean.** `git merge-tree` reports no conflict, writes a tree and
exits zero. The two files do not overlap textually, because one is the other
renamed. A conflict is the easy case — git says so, a person expects it, and
nobody merges through it. The case worth tooling is the one where git says
nothing, so conflict detection alone would have caught neither incident.

**Git did warn, in a sentence nobody reads.** The rebase that dropped the delete
side printed `warning: skipped previously applied commit` and carried on
successfully. That is the only signal, it appears mid-rebase, and it is
indistinguishable from the many harmless cases of the same message.

**`go build ./...` does not catch it.** That is the check the obvious design
reaches for, and it is the wrong one: it does not type-check test files, and
this collided in a `_test.go`. Run against the merged tree it printed nothing at
all. Only a type-check that includes tests named it.

None of the existing guards cover this. `guard-reachability`, the frozen mutants
job and the TUI layout audit each run against one tree. This is a composition
question, and nothing was asking it.

## Decision

`scripts/merge-guard` computes the merge of the current branch into its base and
compiles the result, and CI runs it on every pull request.

The merge is computed with `git merge-tree --write-tree`, which writes a tree
object and no commit and does not touch the working tree. On a clean merge the
tree is extracted with `git archive | tar` into a directory from
`os.MkdirTemp`, removed on every exit path, and the checks in
`mergeguard.Checks()` run inside it.

`Checks()` contains `go vet ./...` and deliberately does not contain
`go build ./...`. **MG4 fails if `go build ./...` is ever added**, because it is
faster only by doing less, and the less it does is the part that catches this.

It reports. It is not wired to block. A new guard that stops merges before it
has been watched working is its own hazard.

## Consequences

`main` is checked against the thing that will exist after the button is pressed,
which no other job here looks at.

**Uncommitted work is invisible to it.** The guard merges committed `HEAD`,
because that is what a merge is. This surprised its own author on its own
branch: the first run reported a trivially clean merge because the changes were
not committed yet. That is the correct behaviour and a real sharp edge.

**It is only as good as `go vet`.** Two changes can compose into something that
builds and is wrong — #157 with #160 was exactly that, a budget breach rather
than a compile error, and `go vet` would not have caught it either. This closes
the compile case and leaves the semantic case open. Saying so is the point; a
guard believed to cover more than it does is worse than none.

**It costs a CI job and a scratch extraction per pull request.** `go vet` does
not run anything, so the cost is a type-check, and the extraction is a tar of a
tree already in the object store.

`internal/mergeguard` joins `internal/guardcheck` as a package outside the
binary's dependency closure, registered as a known-absent case in
`internal/regression/unwired_packages_test.go`, and both new files are exempted
by path from the `os/exec` confinement in `cmd/replay/x402_test.go`. The
exemptions are by path with a written reason rather than by build tag: a tag is
one line anybody can add, and an entry a reviewer reads is not. The test-file
exemption argues differently again — a `_test.go` cannot reach the shipped
binary by a structural fact, not by a permission.

## Alternatives considered

**Detect conflicts and stop there.** Cheapest, and it would have caught neither
incident. Both merges were clean. Kept as the first step, because a conflict
should not be reported as a compile failure, but it is not the check.

**`go build ./...` on the merged tree.** Faster than `go vet`, and measured
against the real incident it printed nothing. Rejected on evidence, and MG4
holds the rejection so that a later reader who notices the speed difference
finds the reason instead of the temptation.

**Run the full suite on the merged tree.** It would catch the semantic case the
type-check misses. Rejected for now: the extracted tree has no `.git`, so tests
that read repository state — including this tool's own MG1 — skip or fail there,
and the run is minutes rather than seconds. If the compile case proves
insufficient, this is where to go next, with the sandbox given a repository
rather than a snapshot.

**Teach the rebase to be louder.** The real signal was
`warning: skipped previously applied commit`, and a hook could refuse a rebase
that skips. Rejected as too narrow: it addresses one of the two incidents and
one of several ways a merge can compose badly, and it puts the check on the
developer's machine at the moment they are least receptive to it.

**A `git worktree` on the computed tree instead of `archive | tar`.** Equivalent
in effect and it keeps a `.git`, which would unblock running the full suite.
Not taken now because it writes into the repository's administrative directory,
and "never touches the working tree" is easier to verify when nothing is
written into the repository at all. Worth revisiting with the alternative above.
