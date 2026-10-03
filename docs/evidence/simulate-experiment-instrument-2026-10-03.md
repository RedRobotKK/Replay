# The simulate experiment's measurement: a counter over a frozen dataset

**2026-10-03. Phase 2 of the post-P0 roadmap. Nothing in the shipped binary
changes. No participant exists. No observation exists.**

The pre-registration (`simulate-experiment-prereg-2026-10-03.md`) fixes what is
observed, who reports it and how it is counted. This records the smallest
mechanism that applies that rule, the evidence that it applies it and nothing
else, and the one thing it refuses to do.

---

## 1. The smallest observation mechanism

**No production code.** The gatekeeper's Decisions 2 and 3 make every value
participant-reported and forbid Replay inferring a cap or its cause from the
ledger, refusal reasons, filesystem state or later commands. A mechanism inside
`replay` would therefore have nothing to measure. The only logic the experiment
needs is the count of element 13 over the ten rows of element 12, and the places
a hand count goes wrong are exactly the pre-registered rules: the denominator,
UNKNOWN, the window, attribution.

**What was built.** `scripts/simexp`, a command run by hand once, after every
window has closed, over a frozen JSON file of exactly ten rows:

```
go run ./scripts/simexp count dataset.json
```

It is not in the shipped binary (listed as correctly absent in
`TestNoNewlyUnwiredPackages`), builds no store, keeps no state, writes nothing,
and reads one file. It prints the result as JSON on stdout with the SHA-256 of
the bytes it counted, so the result can be laid next to the frozen file. Exit 1
is a usage error, exit 2 is a rejected dataset with the reason on stderr and
nothing on stdout.

**The row.** Each participant, as they reported it: `id`; `exposedOn` (date or
UNKNOWN); `exposures` (descriptive count); `capBefore`, `alternativeCap` and,
when changed, `capAfter`, each a set of the four serve cap settings or UNKNOWN;
`changed` and `influenced`, each yes, no or UNKNOWN; `answeredOn` (date or
UNKNOWN). The file names the pre-registration it is counted under, the date it
was frozen, and the window mode (a cohort end date, or per participant).

**The chain, in order (element 8).** Not exposed; no end-of-window answer; answer
after the window plus three days; whether the cap changed; whether the
simulation influenced it. The first UNKNOWN or negative answer stops the chain
and names the outcome. Only the row that reaches the end qualifies.

**What it refuses.** A dataset of any size but ten; a duplicate id; an unknown
field anywhere; a cap key that is not one of the four; a negative cap or
exposure count; `changed: yes` with no `capAfter`, or with identical known caps;
`changed: no` with a different known `capAfter`; an answer dated before the
exposure; a file frozen before every window closed. A row that contradicts
itself is a rejection of the whole file, never a classification.

**What it decides.** `qualifying < 3` is KILLED / REDESIGN REQUIRED; otherwise
SURVIVES THIS EXPERIMENT. The four secondary figures of section 8 of the
pre-registration (enabled a cap, changed an existing cap, changed without
attribution, UNKNOWN) and three more descriptive counts (not exposed, cap
unchanged, cap values UNKNOWN) are printed and decide nothing.

---

## 2. The gates

| Gate | Measured |
|---|---|
| 1 RED | 37 test cases (12 top-level tests, 29 subtests) written against a stub that returns `not implemented`; all 37 failed, every failure on that error; output kept in the session scratch |
| 2 GREEN | The same 37 pass on the implementation, first run, no test edited |
| 3 MUTATION, hand sweep | 18 hand mutations of `count.go`, each applied, tested and restored (file byte-identical afterwards): threshold 2, threshold `<=`, denominator accepts 11, grace 4 days, attribution ignored, exposure ignored, window never closed, duplicates merged, unknown fields allowed, UNKNOWN change read as no, identical caps allowed, 31-day window, enabled and existing swapped, digest of nothing, UNKNOWN influence qualifies, late answer counts, negative exposures allowed, cap key unchecked. **18/18 killed** |
| 3 MUTATION, frozen | M120 `simexp-infers-attribution` (a change the participant does not attribute to the simulation counts as qualifying) frozen in the catalogue and killed through the registered harness: `119 catalogued, 1 mutants: 1 killed, 0 survived, 0 stillborn` |
| 4 PRODUCTION PATH | `binary_test.go` (mutation tag) builds the command and runs it as a child process: a frozen file with three qualifying and one unexposed counts 3 of 10, SURVIVES, digest equal to the file's, two runs byte-identical, file unchanged; nine rows exits 2 naming ten with nothing on stdout; a missing file exits 2; no arguments exits 1 with usage |
| 5 DETERMINISM | Same bytes, same result bytes, in-process and as a binary; output order is input order |
| 6 IMMUTABILITY | The dataset file is read once and never written; asserted after a count |
| 7 INVALID INPUT | Sixteen malformed shapes rejected with the reason, plus duplicates and an unclosed window |
| 8 RACE | `go test -race` over the package, 0 races (the counter has no concurrency; the run is the regression's) |
| 9 REGRESSION | Full suite and race run recorded in section 4 |
| 10 SCOPE | Files in section 5; nothing under `cmd/` or `internal/` except the allowlist entry and the catalogue; no new dependency |

---

## 3. What this does not do

It does not record exposure. Exposure under element 4 is "actually shown the
result"; the participant establishes it by showing the report, and `replay
simulate` writes nothing. A local exposure record was considered and not built:
it would be a product primitive, it would record a run and not a showing, and
Decision 2 says the participant records it. It does not know who the
participants are. It does not read a ledger. It does not infer a cap. It does
not adjust anything after a result: the threshold, denominator, window and grace
are constants named after the pre-registration elements that fix them.

---

## 4. Regression

Measured on the tree as committed. `go test ./...`: 38 packages ok, 0 failed
(37 before this package). `go test -race ./...`: 38 ok, 0 races. `go vet` with
and without the mutation tag: clean. `gofmt -l`: nothing. `golangci-lint
--new-from-rev=1ea88d9`: 0 issues. `git diff --check`: clean. Docs guards
(orphaned documents, dangling links, printed commands, catalogue counts in the
README and CI) and the regression package: green. The wiring and
production-grade matrices were regenerated and did not move, because M120 is
anchored outside the replay surfaces.

---

## 5. Files

`scripts/simexp/count.go`, `run.go`, `main.go` (new, the command);
`scripts/simexp/count_test.go` (new, the 37 cases); `scripts/simexp/binary_test.go`
(new, mutation tag, the production path); `internal/regression/unwired_packages_test.go`
(one allowlist entry); `internal/mutation/testdata/mutants.json` (M120);
`README.md` and `.github/workflows/ci.yml` (catalogue count); this document and
its index row.
