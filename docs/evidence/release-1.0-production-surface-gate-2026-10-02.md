# 1.0 production surface gate

**2026-10-02. Every subcommand the shipped binary dispatches, discovered from
the code, held to an end-to-end test, a production-path oracle, a frozen
mutant and a machine-checkable PASS/FAIL. Production changed in two files.
Nothing committed.**

The runtime wiring gate earlier the same day answered "are all 1.0 claims
wired?" with No and stopped. This pass is the repair and the certification:
the gate itself is now part of the suite, so a surface added without
coverage, a production caller removed, an oracle moved into a package the
binary does not link, or a mutant whose anchor has drifted turns CI red
without anyone maintaining a list.

Labels: **OBSERVED** is what a command printed or a test did. **READING** is
what that permits.

---

## A. Auto-discovered surface inventory

**Mechanism.** `discoverSurfaces` in `cmd/replay/wiring_gate_test.go` parses
`cmd/replay/main.go` with `go/parser`, finds `dispatch()`, and reads every
`case` of its switch: the string literals are the surface's spellings and the
first `run*`/`print*` call in the case body is its entry function. There is no
hand-written list. The `default:` case routes a path argument to `replay`,
which is already a case.

**Correlation.** `shippedClosure` walks non-test imports from `cmd/replay`
(parsing, not `go list`: this package may not import `os/exec`), and
`TestWiringGate_MainReachesDispatch` pins `main() → run() → dispatch()`, so the
tests that call `dispatch` stand on the path `main()` takes.

**OBSERVED: 33 surfaces**, in dispatch order:

`version` `help` `replay` `blame` `diff` `corpus` `pool` `codex` `grok` `jev`
`mcp` `agents` `burn` `doctor` `probe` `rules` `tui` `statusline` `cost`
`ceiling` `trim` `route` `context` `advise` `learn` `redact` `prefix` `since`
`budget` `purge` `privacy` `serve` `upgrade`

Classification: all 33 are **SUPPORTED+SHIPPED** by construction (a dispatch
case is reachable from `main`). The discovery found no **SUPPORTED+UNWIRED**
subcommand. Below the subcommand level, one supported behaviour was
**IMPLEMENTED+DEAD** at the start of the pass and is wired now: RPL-C026's
classifier in `internal/surface` (§B). **TEST-ONLY** packages the binary does
not link remain listed, with reasons, in
`internal/regression/unwired_packages_test.go`; `internal/surface` left that
list today. Packages absent from the closure and cited as evidence for a
shipped claim are now a gate failure (§C, `TestWiringGate_ProvenRowsCiteShippedPackages`).

The documented product support matrix, `docs/PRODUCTION-WIRING.md`, is held
to `replay burn`'s own surface list by the pre-existing `TestWM1`; this gate
did not duplicate that.

---

## B. Repairs

Production files changed by this pass (everything else under `cmd/` and
`internal/` was dirty before it, from the campaign, and is byte-identical to
the pre-gate figure of 24 files, +597/−122):

| File | Change | Why |
|---|---|---|
| `cmd/replay/share.go` | headline reads `<figure> of my priced agent spend was paid twice.`; `>99%` ceiling for a measured 99–100% | PROOF-1.0 blocker 2 was fixed on the PNG only; the text card said "of my agent spend" over `RebilledShare`, a share of priced spend, and rounded 99.6% to 100% |
| `cmd/replay/doctor.go` | `cache signal` line: `surface.Probe` over the transcript root, `surface.Classify` against `anthropicWriteContract()`, class and reason printed; `undetermined` with reason on an empty boundary | RPL-C026 had no production caller; `internal/surface` was outside the closure |
| `internal/claims/claims.go` | C001 `Tests` += `TestE2E_Diff`, `TestE2E_Cost`; C008 += `TestB5_…`, `TestE2E_Cost`; C021 += `TestC021_BurnPrintsNoCrossSurfaceTotal` and a behavioural Oracle; C026 Scope, Establishes, Asserted, Tests, Why rewritten for the wired surface | the register pointed at oracles in packages the binary does not link |
| `internal/mutation/testdata/mutants.json` | 39 new frozen mutants, M78–M116 | one deciding mutant per surface plus five for the repairs |

Tests and documents: `cmd/replay/wiring_gate_test.go` (the gate),
`cmd/replay/e2e_surfaces_test.go` (34 `TestE2E_*`),
`cmd/replay/sharecard_population_test.go`,
`cmd/replay/claim_c021_burn_total_test.go`, `cmd/replay/claim_tier_test.go`
(C025 asserts `cachemodel.PriceTableVersion` literally),
`internal/claims/surfacescan_test.go` moved to `cmd/replay/surfacescan_test.go`
(the repository scans for C012, C013, C016, C022, XW6 now run in a package the
binary links), `internal/regression/unwired_packages_test.go` (`internal/surface`
removed from the allowlist), `docs/design/UNWIRED-LOG.md` §10 marked WIRED,
`docs/PROOF-1.0.md` rows for "refusals are cheap" and blocker 2,
`CLAIM-REGISTER.md` regenerated, `.github/workflows/ci.yml` catalogue count
and `-timeout 90m`, `docs/evidence/wiring-matrix-1.0.md` generated,
`docs/evidence/README.md` indexed.

---

## C. TDD evidence

Every repair went RED first on the real rendered surface, through `dispatch`.

| Surface | RED (observed failure before the repair) | GREEN | Mutation killed | E2E |
|---|---|---|---|---|
| share card wording | `TestShareCardNamesThePricedPopulation`: `"30% of my agent spend was paid twice."` | pass | M114 `share-card-names-all-spend`, M116 `share-halves-the-rebilled-share` (denominator) → both killed by that test | `cost --share` on the mixed ledger prints `30% of my priced agent spend was paid twice.` |
| share card ceiling | `TestShareCardDoesNotRoundToAHundredPercent`: `100% of my agent spend…` at 99.6% | pass | M115 `share-card-rounds-to-a-hundred-percent` → killed | same card at 0.996 prints `>99%` |
| C026 wiring | `TestE2E_Doctor`: output lacks `cache signal  `; `TestE2E_DoctorRefusesToClassifyAnEmptyBoundary`: lacks `undetermined`, `no records at this boundary` | pass | M91 `doctor-drops-the-cache-signal` → killed by `TestE2E_Doctor`; M92 `classifier-reads-an-empty-corpus-as-evidence` (`identify.go`) → killed by the refusal test | `doctor` prints `cache signal  I-marginal-only` with its reason on the fixture, `cache signal  undetermined / no records at this boundary, so nothing was observed to classify` on an empty HOME |
| C021 oracle | the 2026-10-02 `rollup` plant survived the name lint | `TestC021_BurnPrintsNoCrossSurfaceTotal` passes on the tree | M112 `burn-prints-a-cross-surface-total` (a live total row in `runBurn`) → killed | `burn` with Codex and Claude Code populated: no total line, the sum of the two token figures absent |
| C025 oracle | the `"undated"` header mutation survived the date regex | `TestC004_TheCostReportStatesHowItsDollarsWereObtained` now requires `dated <PriceTableVersion>` | M113 `cost-header-drops-the-price-table-date` → killed | header prints `at list prices dated 2026-09-07` |
| C001 / C008 oracles | registered tests lived in `internal/surface`, outside the closure (`TestWiringGate_RegisteredOraclesRunInTheShippedClosure` RED on six claims) | register points at `TestE2E_Diff`, `TestE2E_Cost`, `TestB5`; the gate is green | M82 `diff-drops-the-cause` → killed by `TestE2E_Diff`; M97 `cost-cold-path-forgets-the-unpriced-count` → killed by `TestE2E_Cost` | `diff` names `turn 1 at 12:00:01 (+1s)` and `cause:`; `cost --json` carries `unpriced: 1` |
| "refusals are cheap" PROOF row | `TestWiringGate_ProvenRowsCiteShippedPackages`: `PROOF-1.0.md:51: an ESTABLISHED row cites internal/quota` | row cites `internal/money` and the four live refusal tests | covered by M97, SP-01, V1, C028 mutants | the four refusal lines in §F |

The gate itself was RED before any `TestE2E_` existed (33 × "no TestE2E_…",
33 × "no frozen mutant is killed by…") and went GREEN only when every row did.

---

## D. Wiring matrix

Generated, not written: `docs/evidence/wiring-matrix-1.0.md`, 33 rows, every
column derived from the tree, compared byte for byte by
`TestWiringGate_MatrixIsCurrent`. Columns: Surface, Shipped (entry declared in
`cmd/replay`), Production caller (`dispatch main.go:<line> → run*`), Deciding
code (the frozen mutant's file and anchor), Functional test and E2E (the
`TestE2E_` that reaches `dispatch` with the surface's own name), Production
oracle (the mutant's killers), Mutation (ids), Status.

**OBSERVED: 33 PASS, 0 FAIL. `PRODUCTION SURFACE GATE: PASS`** at the foot of
the file.

A row is PASS only when: the entry function exists; `TestE2E_<Name>` exists in
`cmd/replay`, calls `dispatch`, and passes the surface's literal name; at least
one frozen mutant names that test as its killer, mutates a file in the shipped
closure, and still anchors exactly once. Any missing column is FAIL, by code.

---

## E. Mutation result

**Hand verification, this pass** (`$CLAUDE_JOB_DIR/tmp/verify_new_mutants.py`:
apply in place, run the named killer in its package, restore, `cmp`): M78–M116,
**39 applied, 39 killed, 0 survived, 0 stillborn**, every file restored
byte-identical. Two first drafts survived and were fixed by making the E2E
observe the deciding data rather than by weakening the mutant: `budget`
(`tool_bytes` now asserted) and `privacy` (the registry's `advice.json` row now
asserted after `advise` writes it). One first draft was stillborn (an unused
variable) and was replaced.

**Catalogue run** (`go test -tags mutation -run TestFrozenMutantsStillDie`):
`go test -tags mutation -timeout 60m -count=1 -run 'TestFrozenMutantsStillDie/<group>' -v ./internal/mutation/`,
the registered test, run four times over ID groups with `GOFLAGS=-p=2`
after a single invocation over all 115 was killed by the operating system for
memory at M103 (the tree was untouched; the harness works on copies, and the
eight thaw copies the kill orphaned in the system temp dir were removed before
the groups ran).

| Group | Catalogue mutants in range | Executed | Killed | Survived | Stillborn | Skipped | Exit |
|---|---|---|---|---|---|---|---|
| M1–M29 | 29 | 29 | 29 | 0 | 0 | 0 | 0 (`ok`, 403.3 s) |
| M30–M59 | 30 | 30 | 30 | 0 | 0 | 0 | 0 (`ok`, 404.8 s) |
| M60–M89 | 29 | 29 | 29 | 0 | 0 | 0 | 0 (`ok`, 401.1 s) |
| M90–M116 | 27 | 27 | 27 | 0 | 0 | 0 | 0 (`ok`, 367.2 s) |

Each invocation printed `115 catalogued — N mutants: N killed, 0 survived, 0
stillborn` for its own N, and each ran the harness's red-baseline check first.

**Reconciliation.** The catalogue holds **115 entries with 115 unique IDs from
M1 to M116; ID 71 does not exist in the catalogue and never did**, so M116 is
the last ID and not a 116th mutant. The four ranges partition the 115 exactly
(29 + 30 + 29 + 27); the union of `--- PASS` subtest names across the four
outputs is 115 distinct IDs. Executed unique frozen mutants: **115. Omitted:
0. Duplicated: 0. Non-mutant IDs in the ranges: 1 (M71, absent from the
catalogue, nothing ran for it).**

**OBSERVED: 115/115 killed, 0 survived, 0 stillborn, 0 skipped, 0 failures,
by the registered test.** The 39 new mutants are M78–M116 inside that count.
No file under `cmd/` or `internal/` changed across the runs (`git diff --stat`
identical before and after), and the system temp dir holds no harness copies.

`TestKillMatrix` was not run in this pass. The previous attempt was killed by
the operating system for memory during that test; the brief's resource rule
applies and it is reported, not retried.

---

## F. E2E result

**OBSERVED: 34 `TestE2E_*` run, 34 passed, 0 failed**, on isolated HOMEs with
hand-built ledgers and the one real transcript fixture. Each asserts the
fixture's own figures, not the presence of text: `3,850 tokens billed across 1
Codex session(s)`, `7,142,396` reconstructed Grok prompt tokens, `Overall match
rate: 98.73% (exact reproduction rate: 89.87%)`, `Cache-blind arithmetic runs
3.66x high over these 80 requests`, `Write penalty is 1.25x at 5m and 2.00x at
1h`, `484k tokens of content entered this context`, `largest: session unp`
under `since`, `tool_bytes: 2000` in `budget --json`, exit status for `prefix`
and `serve`.

Two surfaces are held at their first refusal rather than their steady state,
and the exception is executable: `serve` (`--preflight -5 --upstream ""`
refuses before any listener, and the second refusal guarantees the mutant
cannot reach one) and `upgrade` (the shipped release client pointed at a test
server that carries no release). `mcp` and `statusline` are held at
`--install`, the path that renders what a user pastes; their stdin loops take
the process's own stdin. `tui` is held at `--once`. `probe` is held at its
plan. All eight reach `dispatch`.

---

## G. Full suite

**OBSERVED.** `go test ./... -count=1`: **37 ok, 0 FAIL** (one package with no
test files, as before). `go test -race -count=1 ./...`, the invocation CI
uses: **37 ok, 0 FAIL, 0 data races.** Both run with `GOFLAGS=-p=2` on this
machine for memory, which changes scheduling and nothing else. The gate
(`TestWiringGate_*`, 5 tests), the 34 `TestE2E_*`, the share-card and C021
tests, and the register and PROOF-row checks are inside those counts.
`TestFrozenMutantsStillDie` is behind the `mutation` tag and is §E.

Baseline before this pass: 37 packages, 0 failures. One package count change:
`internal/claims` lost its five repository-scan tests to `cmd/replay`
(the scans certify the binary, so they run in the package the binary is), and
`internal/claims` keeps its register tests.

`gofmt -l`: 0. `go vet ./...`: clean. `git diff --check`: clean.

---

## H. Final gate

**PRODUCTION SURFACE GATE: PASS** (`docs/evidence/wiring-matrix-1.0.md`, foot
of the file, regenerated from the tree and compared byte for byte by
`TestWiringGate_MatrixIsCurrent`).

Final read-only audit, 2026-10-02, branch `fix/compaction-observed-vs-inferred`,
HEAD `dc4686b`, working tree dirty and uncommitted by instruction:

- **Surfaces:** 33 cases in `dispatch()`, 33 discovered, 33 wired, 33 PASS.
- **E2E through dispatch:** 34 `TestE2E_*` (one per surface plus the refusal
  half of `doctor`), every one reaching `dispatch` through the `e2e` helper
  that calls it and nothing else; `main() → run() → dispatch()` pinned. 34/34
  pass.
- **New mutants:** 39/39 killed by hand and by the registered test.
- **Frozen mutants:** 115/115 killed by the registered test, reconciled.
- **Oracle closure:** every test an ESTABLISHED or BOUNDED claim cites, and
  every package an ESTABLISHED PROOF-1.0 row cites, lives in the shipped
  dependency closure (`TestWiringGate_RegisteredOraclesRunInTheShippedClosure`,
  `TestWiringGate_ProvenRowsCiteShippedPackages`, both green); each surface's
  mutant is in a shipped file and its killer is the surface's own E2E test.
- **Complete suite:** 37/0 plain, 37/0 race.
- **Repository integrity:** `gofmt -l` 0, `go vet ./...` clean, `git diff
  --check` clean, no scratch files in the tree (the output-dump probe used to
  tighten assertions was deleted the same minute), no harness copies in the
  temp dir, nothing staged.

Certification: **CERTIFIED** under the frozen design. What it certifies is the
property, not the report: the shipped binary reaches every discovered surface,
and for each one a registered test fails when the deciding implementation is
broken.
