# Mutation score: 275 of 375 viable mutants killed, 2026-09-13

**What this measures:** what fraction of the mutants this tree admits the test
suite actually catches. Not how many seeded defects still die, which is what
[`internal/mutation/`](../../internal/mutation/) already measures and says so
about itself, but killed over generated: a population of 8,150 mutants produced
mechanically from the syntax of 45,556 lines of production Go, a uniform random
sample of 400 of them drawn with a stated seed, and the suite run once per
mutant against a scratch copy of the tree.

Written because the project had a numerator and no denominator. The catalogue
reports "76 mutants, 76 killed" and its own note is explicit that this is error
seeding rather than mutation analysis, because the 76 are hand-chosen historical
defects and not a generated operator population. A reader could not tell from it
what share of the mutations the code admits are caught, and therefore could not
tell whether that share was rising or falling. The rule this repository applies
to everyone else's numbers is that a figure carries its population and its date.
This one now does.

## Headline

| | |
|---|---:|
| Population (mutants the tree admits under the operator set below) | **8,150** |
| Sampled (uniform, seed 20260913) | **400** (4.91%) |
| Stillborn (the compiler refused them, so no test was ever asked) | 16 |
| Viable | 384 |
| Identified equivalent, removed from the denominator | 9 |
| **Killed / viable-and-not-equivalent** | **275 / 375 = 73.3%** |
| 95% confidence interval (Wilson) | **[68.6%, 77.6%]** |

Before the equivalence adjustment the reading is 275/384 = 71.6%, with a 95%
interval of [66.9%, 75.9%]. Both are published, because the adjustment is a
judgement and the raw ratio is not.

Measured at commit `c0ed888`, on a tree pinned by `git archive HEAD` shortly
before the run started. The run finished at 2026-09-13 04:40 UTC, which is
13:40 JST and 2026-09-12 21:40 PDT on the machine that ran it.
The working tree was not used, and that was not a stylistic choice. Other work
was in flight in this repository throughout: at the moment the sample started,
`internal/proxy/unmasked_test.go` called a method that did not exist yet, so the
working copy was correctly and deliberately red. A score taken there would have
been a score over a broken baseline. HEAD moved one commit
(`ffb4c86`) while the run was in flight. That commit is not in this reading.

## Scope and operator set

**Scope.** Every `.go` file under `cmd/` and `internal/`, excluding `_test.go`,
`testdata/`, and files carrying a `Code generated ... DO NOT EDIT` header. That
is 206 files, 27 packages, 45,556 lines. Test code (76,107 lines) is the
instrument, not the subject, and is never mutated.

**Operators.** Six, applied at every syntactic site that admits them. The
population column is the number of mutants each operator generates over the
whole tree, which is the denominator this file exists to supply.

| Operator | What it does | Population |
|---|---|---:|
| negate-conditional | `==` to `!=`, `<` to `>=`, and the rest of the negation table | 3,297 |
| remove-statement | deletes a discarded call statement or a `defer` | 1,227 |
| conditional-boundary | `<` to `<=`, `>` to `>=`, and back | 1,211 |
| arithmetic | `+` to `-`, `*` to `/`, `%` to `*` | 1,174 |
| bool-literal | `true` to `false` and back | 795 |
| return-value | a returned `err` to `nil`, a string or numeric literal to its zero, a `&T{}` to `nil` | 446 |

String concatenation is excluded from the arithmetic operator, because `"a" - b`
is a compile error rather than a question anyone can put to a test suite.

## Method

Each mutant is a byte splice: file, offset, length, replacement. Not a textual
anchor. An anchor that matches twice mutates a site other than the one the
mutant is about, and an anchor chosen by hand cannot enumerate a population at
all.

1. The tree is pinned once with `git archive HEAD` into a scratch directory and
   the full suite is run there unmutated. It passed in 9.7 seconds. A mutant
   killed by an already-red suite has been killed by nothing.
2. Four workers each clone that tree. Work is pulled from one shuffled queue, so
   an interrupted run would still be a uniform sample rather than whichever
   mutants happened to be cheap.
3. Per mutant: apply the splice, run `go build ./...` and classify a refusal as
   **stillborn**, then run the entire suite as `go test ./... -count=1 -timeout
   90s`. Exit zero is **survived**. A failure is **killed**. A hang caught by
   the Go test timeout is **killed by timeout** and counted separately.
4. Restore the file. The working tree is never touched at any point.

**No tests were skipped.** The brief allowed skipping long integration tests and
none needed skipping: the whole suite runs in about 10 seconds, so every mutant
was put to all 29 test packages rather than to its own package alone. That
matters, and it is measurable: **29 of the 269 test-failure kills, 10.8%, were
killed only by a package other than the mutant's own**. A run scoped to the
mutated package would have scored all 29 as survivors and reported a score
around 66% instead of 73%.

**The instrument was checked against the failure it could hide.** A mutation
score computed under heavy machine load can manufacture kills out of timing
flake, and this machine was loaded by other agents throughout. Four unmutated
copies of the tree were run concurrently at the end of the sample, under the
same contention: all four green, 29 of 29 packages each. The 6 timeout kills
were also read one by one rather than trusted, and all 6 are hangs the mutation
explains. Three are a removed `defer s.mu.Unlock()` in
`internal/proxy/state.go`, which deadlocks. Two are in
`internal/proxy/lifecycle.go`, where inverting `err != nil` at line 42 returns
before `Serve` starts and inverting `mln != nil` at line 87 runs the metrics
server on a nil listener, which the comment three lines above it says blocks
forever. One is `internal/proxy/retry.go:110`, where inverting the ceiling test
makes the client honour a `Retry-After` longer than the ceiling and sit there.
Those are detections, not a slow machine.

## Result by operator

Killed over viable, with identified equivalents removed.

| Operator | Killed / viable | Score | Stillborn |
|---|---:|---:|---:|
| negate-conditional | 159 / 188 | 85% | 0 |
| return-value | 12 / 14 | 86% | 1 |
| bool-literal | 27 / 37 | 73% | 0 |
| arithmetic | 27 / 44 | 61% | 8 |
| remove-statement | 29 / 49 | 59% | 7 |
| **conditional-boundary** | **21 / 43** | **49%** | 0 |

The shape of the result is one sentence: **this suite tests what a branch
decides and not where the branch sits.** Flip a condition and 85% of the time
something goes red. Move the same condition by one, from `>` to `>=`, and it is
a coin toss. Off-by-one is the defect class this suite is weakest against, and
it is the class that reaches users as a crash rather than as a wrong number.

## Result by package

Worst ratio first. The population column is what the whole package admits, so a
package with a wide interval and a large population is where more sampling would
pay. Packages with one or two sampled mutants are listed for completeness and
decide nothing.

| Package | Killed / viable | Score | 95% CI | Population |
|---|---:|---:|---:|---:|
| internal/reference | 0 / 2 | 0% | [0%, 66%] | 29 |
| internal/money | 1 / 2 | 50% | [9%, 91%] | 69 |
| internal/learn | 3 / 6 | 50% | [19%, 81%] | 162 |
| **cmd/replay** | **83 / 128** | **65%** | **[56%, 73%]** | **2,378** |
| internal/card | 6 / 9 | 67% | [35%, 88%] | 229 |
| **internal/tui** | **16 / 24** | **67%** | **[47%, 82%]** | **855** |
| **internal/analysis** | **26 / 38** | **68%** | **[53%, 81%]** | **844** |
| internal/observation | 3 / 4 | 75% | [30%, 95%] | 153 |
| internal/cachemodel | 15 / 19 | 79% | [57%, 91%] | 432 |
| internal/proxy | 45 / 57 | 79% | [67%, 88%] | 839 |
| internal/probe | 9 / 11 | 82% | [52%, 95%] | 361 |
| internal/masking | 19 / 23 | 83% | [63%, 93%] | 514 |
| internal/advisor | 6 / 7 | 86% | [49%, 97%] | 123 |
| internal/transcript | 16 / 18 | 89% | [67%, 97%] | 505 |
| internal/ledger | 9 / 9 | 100% | [70%, 100%] | 222 |
| internal/guardcheck | 4 / 4 | 100% | [51%, 100%] | 128 |
| internal/quota | 4 / 4 | 100% | [51%, 100%] | 83 |
| internal/feed, selfupdate, usage | 2 / 2 each | 100% | [34%, 100%] | 24, 85, 36 |
| internal/consent, otlp, ownerdir, policy | 1 / 1 each | 100% | [21%, 100%] | 20, 25, 8, 14 |

`internal/facts` and `internal/version` are in the population (3 and 2 mutants)
and neither was drawn. They are not scored here.

**The three that are actionable are the three in bold**, because they combine a
low ratio with a large population and an interval that does not reach the
tree-wide figure from below. `cmd/replay` alone is 29% of every mutant this tree
admits and it is the worst-scoring large package: 45 survivors out of 128
viable, in the code a user's command line reaches first.

## Survivors worth fixing

Not a list of all 109. These are the ones where the surviving mutant names a
defect a user would meet.

**A crash, not a wrong number.** `cmd/replay/main.go:267`, `i+1 < len(args)` to
`i+1 <= len(args)`, survives. That mutant makes `hoistFlagsFor` read `args[i]`
one past the end when the last argument is a value-taking flag with no value.
Typing `replay purge --older-than` and pressing return would panic. The function
has a dedicated test file with a table of eight cases, and not one of them ends
on a bare value-taking flag.

**The flagship extension is unasserted.** `internal/analysis/sources.go:60`,
`".md": true` to `false`, survives. Nothing in the suite asks whether a Markdown
file counts as a document source, in the map whose entire purpose is to separate
documents from program source.

**A privacy guard with no test behind it.** `internal/masking/mask.go:219`,
`top.skipAll = true` to `false`, survives. That flag is what stops the masker
walking into `thinking` and `redacted_thinking` blocks. The masking package
scores 83% overall and this particular guard is unguarded.

**A failed write reported as a success.** `internal/observation/observation.go:259`,
`return "", err` to `return "", nil`, survives. `WriteObservation` would report
a path it did not finish writing.

**Two timeout defaults that can be zeroed silently.**
`internal/proxy/retry.go:31` and `cmd/replay/serve.go:31` both hold
`30 * time.Second`. Mutating `*` to `/` gives zero in both, which is a retry
ceiling of nothing and a breaker cooldown of nothing, and both survive.

**Six metrics rows out of eight that nothing reads.** Eight `line(...)` calls in
the Prometheus exposition in `internal/proxy/state.go` were sampled. Two die:
deleting the `# TYPE` for `replay_cache_write_tokens_total` or for
`replay_masked_total` turns a test red. The other six survive, among them
`replay_request_latency_seconds_sum` and the `# TYPE` rows for
`replay_upstream_errors_total`, `replay_held_total`,
`replay_held_milliseconds_total` and `replay_rehydration_denied_total`. The
exposition is checked in two places and unchecked in six.

**A JSON fraction beginning with nine.** `internal/transcript/wire.go:632`,
`b[i] > '9'` to `b[i] >= '9'`, survives. `measureJSONNumber` would refuse to
measure `0.9`. No fixture in the suite carries one in that position.

## Equivalent mutants, named

A mutant that cannot change observable behaviour is noise, not a survivor, and
counting it makes the score wrong in the pessimistic direction. Nine were
identified in the 109 survivors and removed from the denominator. Each is named
with the ground, so a reader who disagrees can put it back.

| Mutant | Why it changes nothing |
|---|---|
| `internal/tui/rows.go:95`, `w > len(rows)` to `>=` | At the boundary the body assigns `w = len(rows)`, which is what `w` already is. A no-op self-assignment. |
| `internal/tui/rows.go:99`, `start < 0` to `<=` | Same shape: at `start == 0` the body assigns `start = 0`. |
| `internal/observation/observation.go:223`, `len(out) > 64` to `>=` | At exactly 64 the body evaluates `out[:64]`, which is `out`. |
| `internal/probe/run.go:346`, `got > target` to `>=` | Line 323 returns when `got == target`, so the added case is unreachable. |
| `internal/mergeguard/mergeguard.go:60`, `i >= 0` to `i > 0` | The line was passed through `strings.TrimSpace` two lines earlier, so `IndexByte(l, '\t')` cannot return 0. |
| `internal/guardcheck/preexisting.go:138`, `return "?"` to `return ""` | Unreachable. The switch above it covers `*ast.Ident`, `*ast.StarExpr`, `*ast.IndexExpr` and `*ast.IndexListExpr`, which is every form Go's grammar admits for a receiver. |
| `internal/guardcheck/coverage.go:63`, removing `sc.Buffer(make([]byte, 1<<20), 1<<20)` | It raises the scanner's token cap from 64 KB. A line in a Go coverage profile is a path plus six integers, so no profile the toolchain can emit approaches the default cap. |
| `internal/money/currency.go:45`, `i+1 >= len(s)` to `i-1 >= len(s)` | `i` indexes an underscore inside `s`, so `i-1 >= len(s)` is never true. The only input the two forms separate is a locale ending in `_`, where the original returns `""` and the mutant falls through to `s[i+1:]`, which is `""`, fails `len(t) != 2` and returns `""`. |
| `internal/card/atlas.go:121`, `y < Max.Y` to `<=` | The extra row calls `img.At` out of bounds, which returns the zero color, and `g.SetGray` out of bounds, which returns without writing. Independently, the branch runs only for an atlas that is not mode `L`, and the embedded one is. |

Two things this list is not. It is not exhaustive: 100 survivors were not
examined one by one, and the tree-wide equivalent rate is therefore unknown
rather than 2.3%. And it is not a defence of the survivors that remain: every
mutant named in the section above changes behaviour a person could see.

## What this does not measure, and where it is weak

**It is a sample, and it says so.** 400 of 8,150, which is 4.91%. Every figure
here carries a Wilson interval and the per-package intervals are wide because
the per-package counts are small. Nothing in this file supports a claim finer
than its interval, and the correct reading of the headline is "somewhere between
69% and 78%", not "73.3%".

**It scores the operator set, not the concept of mutation.** A different set
gives a different number. `negate-conditional` is 40% of the population and the
easiest class to kill, so a score weighted toward it flatters the suite. Split
by operator, the honest summary is the conditional-boundary row at 49%.

**Stillborn mutants are excluded from the denominator, which is a choice.** 16
of 400 did not compile, concentrated in the arithmetic and remove-statement
operators, where deleting a call leaves a variable declared and not used. The
compiler refused them, so the suite was never asked about them, and counting
them either way would be inflation in one direction or the other. The count is
published so a reader can put them back.

**A kill says a test went red, not that the test was the right one.** The frozen
catalogue records which named test kills each of its mutants and fails when a
different one does the killing. This run records only which packages failed.
That is weaker, and it is the part the catalogue does better.

**It cannot see whether the number is moving yet.** This is the first reading.
The comparison it invites is against the next one, on a population regenerated
from that tree, with the seed and the sample size stated the same way.

**No relationship to the 76 frozen mutants should be inferred.** The catalogue
is 76 hand-chosen historical defects, each admitted only because a named test
died on it, which makes its ratio 100% by construction on the first run and
informative only afterwards. This is a generated population with no admission
criterion beyond compiling. The two denominators are not commensurable and the
two numbers must never be quoted beside each other as if they were.

## Reproducing it

The generator, the runner and the scoring script are throwaway tools that live
outside the repository, and this file is the artifact. What the repository
carries is the recipe: pin the tree with `git archive HEAD`, enumerate the
splices with `go/parser` over `cmd/` and `internal/`, shuffle with the stated
seed, and run `go test ./... -count=1 -timeout 90s` once per mutant against a
private copy, classifying a `go build` refusal as stillborn before anything else.
The wall clock was 44 minutes for 400 mutants on four workers, on a ten-core
laptop shared with other work.

---

[Evidence](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
