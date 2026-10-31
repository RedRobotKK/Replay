# Harness Gate 0: every guard guard-reachability found unobserved, 2026-10-31

This file records, for each of the 109 conditionals in `scripts/replay-bench` that the guard-reachability check on pull request #369 found unobserved, whether it is a protection and how it is now resolved.

## Source

CI run 41157157313, job 124406361857, step "Neutralise the guards this change touches", against branch head `ab25fa0`:
139 guards, 109 survived (0 pre-existing, 109 introduced: 29 INERT, 80 UNREACHED), 0 evidenced, 4 not built here.
The file, line and condition columns below are copied from that log. The line numbers are the ones in `ab25fa0`; the code has moved since.

## Rules followed

- No guard was deleted to reach zero. INERT does not mean unnecessary, and UNREACHED does not mean removable.
- Every resolution is exactly one of:
  - **T**: a test that makes the condition true.
  - **A**: an assertion that depends on the branch, so neutralising it fails a test.
  - **R**: a removal, with a reason a reviewer can reject.
- I/O error branches get tests with injected failures. Where no real failure can be produced on demand, the test uses a package-level seam: `execute`, `createExclusive`, `openLedger`, `openVault`, `newProxyServer`, `readBuildInfo`, `currentLoad` and `removeAll`, plus the `masker` interface.
- The five guards in `env_linux.go` (and the four in `env_darwin.go`, which Linux CI reports as "not built here") moved into helpers in `env.go`. The helpers are `cpuModelFromProc`, `loadFromProc`, `cpuModelFromSysctl`, `loadFromSysctl` and `rssBytes`. Every platform builds and tests them, so the platform files no longer hold a conditional that only one host can score.
- "Protection" means error handling, a refusal, or a bound check. "Incidental" means the branch shapes output or a measurement but guards nothing.

## Totals

| Resolution | Count |
|---|---|
| T: a test makes the condition true | 72 |
| A: an assertion depends on the branch | 28 |
| R: removed, with a rejectable reason | 9 |
| **Total** | **109** |

The R rows fall into three kinds:

- Five checks on an error that was nil by construction: run.go:167, 172, 177, 181 and 186.
- Two checks on `putOverhead` and `recordStream` errors that cannot occur, because `pairedRun` returns arms of equal length: run.go:375 and 617. Both errors are now returned rather than branched on. Their failure paths are tested where they can occur, at run.go:269 and 450.
- Two branches that repeat what the code below them already does: stats.go:112 (`0/0` is already NaN) and env_linux.go:31 (every parse error is already the zero `LoadAvg`). Both outcomes are pinned by tests.

## Not in the 109: env_darwin.go

Linux CI did not score `env_darwin.go:6, 14, 18, 27`. It listed them as "not built here", and those entries fail the run too. Their logic now lives in `cpuModelFromSysctl`, `loadFromSysctl` and `rssBytes`, tested by `TestCPUModelAndLoadFromSysctl` and `TestRSSBytesIsMinusOneWhenGetrusageFails`. The `vm.loadavg` parse-error branch was removed on the same grounds as env_linux.go:31. `env_darwin.go` and `env_linux.go` now hold no conditional.

## Verification

All commands were run in this clone on darwin/arm64 with go1.27.3:

| Command | Exit | Result |
|---|---|---|
| `go vet ./...` | 0 | |
| `go test -race -count=1 ./scripts/replay-bench/` | 0 | ok |
| `make test` | 0 | 90.1% of statements covered |
| `make lint` with golangci-lint v2.14.0, the version CI pins | 0 | 0 issues |
| `go run scripts/guard-reachability/main.go origin/main` | 0 | 132 guards, 0 survived, 0 unexplained, 0 unchecked, 0 not built here |

The local run is close to what CI does, but not identical:

- It scored darwin, while CI scores linux/amd64. Neither platform file now holds a conditional, so both hosts score the same set of guards.
- CI is the authority. A branch that fires only on Linux would show up there and not here.

A neutralised `os.MkdirTemp` guard in `startProxy` makes that mutant write a `ledger/` directory into the package directory. That is an artefact of the run, and it is not committed.

## Per-guard table

| # | CI file:line | Condition (CI log) | Class | Protection? | Res | Test and the assertion that fails when the guard is neutralised, or the removal reason |
|---|---|---|---|---|---|---|
| 1 | `scripts/replay-bench/env_linux.go:14` | `if err == nil {` | INERT | incidental | A | Moved to `cpuModelFromProc` (env.go). `TestCPUModelFromProcReadsTheModelName`: `got != "Test CPU @ 3.00GHz"` fails (reads unknown). |
| 2 | `scripts/replay-bench/env_linux.go:17` | `if ok && strings.TrimSpace(k) == "model name" {` | INERT | incidental | A | Moved to `cpuModelFromProc`. Same test, same assertion; also `!HasPrefix(got, "unknown")` for a line only containing the words. |
| 3 | `scripts/replay-bench/env_linux.go:27` | `if err != nil {` | UNREACHED | protection | T | Moved to `loadFromProc`. `TestLoadFromProcDistrustsAFailedRead`: bytes plus an error, `got.OK` must be false. |
| 4 | `scripts/replay-bench/env_linux.go:31` | `if err != nil {` | UNREACHED | protection | R | Parse-error branch in `loadFromProc` removed: every error return of `parseProcLoadavg` is already the zero `LoadAvg` (OK false), the value the branch returned. Dropped with a commented `_`; `TestParseProcLoadavgRefusesShortAndNonNumericInput` now pins `got.OK == false` on every error, so the removal is watched. Reject if an explicit branch is preferred over that contract. |
| 5 | `scripts/replay-bench/env_linux.go:39` | `if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {` | UNREACHED | protection | T | Moved to `rssBytes`. `TestRSSBytesIsMinusOneWhenGetrusageFails`: `got != -1` fails. |
| 6 | `scripts/replay-bench/main.go:48` | `if *repeats > 0 && !*quick {` | UNREACHED | incidental | T | `TestRepeatsFlagSetsTheRepeatCountUnlessQuick` (fake `execute` seam): `cfg.Repeats != 3` fails. |
| 7 | `scripts/replay-bench/main.go:53` | `if err != nil {` | UNREACHED | protection | T | `TestCommandExitsWithAnErrorWhenTheRunFails`: `code != exitError` and `result:` on stdout fail. |
| 8 | `scripts/replay-bench/main.go:59` | `if err != nil {` | UNREACHED | protection | T | `TestCommandExitsWithAnErrorWhenTheSidecarIsMissing`: `code != exitError` fails. |
| 9 | `scripts/replay-bench/main.go:66` | `if err != nil {` | UNREACHED | protection | T | `TestCommandExitsWithAnErrorWhenTheResultCannotBeWritten` (`-out` is a file): `code != exitError` fails (nil-path deref or exit 0). |
| 10 | `scripts/replay-bench/main.go:71` | `if err := Render(stdout, res); err != nil {` | UNREACHED | protection | T | `TestCommandExitsWithAnErrorWhenTheSummaryCannotBeWritten` (failing writer): `code != exitError` fails. |
| 11 | `scripts/replay-bench/main.go:80` | `if err != nil {` | UNREACHED | protection | T | `TestRenderRefusesAMissingOrMalformedFileAndAFailedWrite`: stderr must contain "open" (the os.PathError op) and not `JSON` (neutralised, the empty read is reported as bad JSON). |
| 12 | `scripts/replay-bench/main.go:85` | `if err := json.Unmarshal(b, &r); err != nil {` | UNREACHED | protection | T | Same test, malformed file: `code != exitError` and stderr naming the file fail. |
| 13 | `scripts/replay-bench/main.go:89` | `if err := Render(stdout, r); err != nil {` | UNREACHED | protection | T | Same test, failing writer on render: `code != exitError` fails. |
| 14 | `scripts/replay-bench/main.go:110` | `case sidecar != "" && a == "-sidecar="+sidecar:` | UNREACHED | protection | T | `TestRecordedArgsReplaceTheSidecarPathInBothFlagForms`: `-sidecar=<path>` must become `-sidecar=<sidecar>`; neutralised the local path is kept. |
| 15 | `scripts/replay-bench/rtmetrics.go:48` | `if m.Value.Kind() == metrics.KindFloat64Histogram {` | INERT | incidental | A | `TestReadSnapshotCarriesTheGCPauseHistogram`: `readSnapshot().pauses == nil` fails. |
| 16 | `scripts/replay-bench/rtmetrics.go:59` | `if v.Kind() != metrics.KindUint64 {` | UNREACHED | protection | T | `TestUint64OfAnUnsupportedMetricIsZero`: neutralised, `Uint64()` on KindBad panics. |
| 17 | `scripts/replay-bench/rtmetrics.go:100` | `if before == nil \|\| after == nil \|\| len(before.Counts) != len(after.Counts) {` | UNREACHED | protection | T | `TestPauseDeltaRefusesHistogramsItCannotSubtract`: nil and mismatched histograms must give 0, 0; neutralised, nil deref or index panic. |
| 18 | `scripts/replay-bench/rtmetrics.go:107` | `if c == 0 {` | INERT | protection | A | `TestPauseDeltaSkipsEmptyBucketsRatherThanMultiplyingByInfinity`: an unchanged (-Inf,+Inf) bucket must give total 0; neutralised 0*Inf = NaN. |
| 19 | `scripts/replay-bench/rtmetrics.go:113` | `case math.IsInf(lo, -1):` | UNREACHED | protection | T | `TestPauseDeltaEstimatesOpenBucketsFromTheirFiniteEdge`: `total != 6e9` fails (neutralised, -Inf). |
| 20 | `scripts/replay-bench/rtmetrics.go:115` | `case math.IsInf(hi, 1):` | UNREACHED | protection | T | Same test: neutralised, the (3,+Inf) midpoint is +Inf and `total != 6e9` fails. |
| 21 | `scripts/replay-bench/stats.go:51` | `if len(samples) == 0 {` | UNREACHED | protection | T | `TestSummarizeOfNothingIsTheZeroSummary`: neutralised, `s[0]` panics. |
| 22 | `scripts/replay-bench/stats.go:112` | `if n < 2 {` | INERT | incidental | R | `n < 2` removed: with one value the variance is 0/0 = NaN, the same undefined CV the branch returned (comment in `SpreadOf`). `TestSpreadMedianAndCoefficientOfVariation` already asserts `SpreadOf([3]).CV` is NaN, so the behaviour stays pinned. Reject if relying on IEEE 0/0 is unwanted. |
| 23 | `scripts/replay-bench/stats.go:163` | `if err := json.Unmarshal(b, &w); err != nil {` | UNREACHED | protection | T | `TestSpreadUnmarshalRefusesMalformedJSON`: `err == nil` fails. |
| 24 | `scripts/replay-bench/workload.go:63` | `if stream {` | INERT | incidental | A | `TestStreamingRequestBodyAsksForAStream`: body must contain `"stream":true,`. |
| 25 | `scripts/replay-bench/workload.go:107` | `if _, err := io.Copy(io.Discard, r.Body); err != nil {` | UNREACHED | protection | T | `TestUpstreamAnswersJSONOrAFlushedStreamAndRefusesABrokenBody`: broken body must get 400. |
| 26 | `scripts/replay-bench/workload.go:113` | `if n <= 0 {` | INERT | incidental | A | Same test: no stream header must give `application/json` with a content-length. |
| 27 | `scripts/replay-bench/workload.go:123` | `if fl != nil {` | INERT | incidental | A | Same test: `rec.Flushed` must be true; without the flush TTFB would measure nothing. |
| 28 | `scripts/replay-bench/workload.go:178` | `if err != nil {` | UNREACHED | protection | T | `TestStartProxyReportsEachConstructorFailure`: an unparseable upstream must fail with a `*url.Error`; neutralised, the error is proxy.New's "upstream URL is required". |
| 29 | `scripts/replay-bench/workload.go:182` | `if err != nil {` | UNREACHED | protection | T | Same test, `tempdir`: error must be MkdirTemp's own (`isMkdirTemp`); neutralised, the ledger is opened in the working directory. |
| 30 | `scripts/replay-bench/workload.go:186` | `if err != nil {` | UNREACHED | protection | T | Same test, `ledger` (seam `openLedger`): `errors.Is(err, errInjected)` and no directory left behind. |
| 31 | `scripts/replay-bench/workload.go:195` | `if mask {` | INERT | incidental | A | `TestMaskOnProxyOpensAVaultAndMaskOffDoesNot`: `opened != 1` fails (neutralised, 0). |
| 32 | `scripts/replay-bench/workload.go:197` | `if verr != nil {` | UNREACHED | protection | T | `TestStartProxyReportsEachConstructorFailure/vault` (seam `openVault`): `errors.Is(err, errInjected)` fails. |
| 33 | `scripts/replay-bench/workload.go:204` | `if err != nil {` | UNREACHED | protection | T | `.../server` (seam `newProxyServer`): `errors.Is(err, errInjected)` fails. |
| 34 | `scripts/replay-bench/workload.go:254` | `if err != nil {` | UNREACHED | protection | T | `TestDoReportsATransportFailureAndABadURL`: base `::not a url` must be an error. |
| 35 | `scripts/replay-bench/workload.go:263` | `if session != "" {` | INERT | incidental | A | `TestDoSendsTheSessionAndStreamHeadersOnlyWhenAsked`: session header must be `sess-1`. |
| 36 | `scripts/replay-bench/workload.go:268` | `if err != nil {` | UNREACHED | protection | T | `TestDoReportsATransportFailureAndABadURL`: a failing RoundTripper must be an error. |
| 37 | `scripts/replay-bench/workload.go:272` | `if resp.StatusCode != http.StatusOK {` | UNREACHED | protection | T | `TestDoReportsANonOKStatusWithItsBody`: error must read `status 502: upstream gone`. |
| 38 | `scripts/replay-bench/workload.go:285` | `if t.ttfb == 0 {` | INERT | protection | A | `TestDoTimesFirstByteAtTheFirstReadNotTheLast` (scripted body): `ttfb >= 5ms` and `total-ttfb >= 30ms` fail. |
| 39 | `scripts/replay-bench/workload.go:302` | `if rerr != nil {` | UNREACHED | protection | T | `TestDoReportsAReadThatFailsMidBody`: a mid-body read error must be an error. |
| 40 | `scripts/replay-bench/workload.go:308` | `if sseEvents > 0 {` | INERT | incidental | A | `TestDoTimesTheStreamTailFromMessageStop`: `tail >= 30ms` fails (neutralised, 0); the missing-marker test also fails. |
| 41 | `scripts/replay-bench/workload.go:309` | `if lastEvent.IsZero() {` | UNREACHED | protection | T | `TestDoRefusesAStreamWithoutMessageStop`: error naming `message_stop` required. |
| 42 | `scripts/replay-bench/env.go:59` | `if gitSHA == "" {` | INERT | incidental | A | `TestCaptureEnvironmentFallsBackToBuildInfoAndRecordsEmptyArgs`: `GitSHA != "cafef00d"` fails. |
| 43 | `scripts/replay-bench/env.go:62` | `if args == nil {` | INERT | incidental | A | Same test: JSON must contain `"args":[]` (neutralised, `null`). |
| 44 | `scripts/replay-bench/env.go:81` | `if bi, ok := debug.ReadBuildInfo(); ok {` | INERT | incidental | A | `TestBuildRevisionComesFromBuildInfo` (seam `readBuildInfo`): stamped revision `deadbeef` required. |
| 45 | `scripts/replay-bench/env.go:83` | `if s.Key == "vcs.revision" {` | UNREACHED | incidental | T | Same test: neutralised, the stamped revision is never matched. |
| 46 | `scripts/replay-bench/env.go:103` | `if len(f) < 3 {` | INERT | protection | A | `TestParseProcLoadavgRefusesShortAndNonNumericInput`: two fields must be an error; neutralised, index out of range. |
| 47 | `scripts/replay-bench/env.go:109` | `if err != nil {` | UNREACHED | protection | T | Same test: `x 0.20 0.30` must be an error (neutralised, a zero load reads OK). |
| 48 | `scripts/replay-bench/env.go:120` | `if len(b) < darwinLoadavgSize-1 \|\| len(b) > darwinLoadavgSize {` | INERT | protection | A | `TestParseDarwinLoadavgRefusesAnOversizedValue`: 25 bytes must be an error. |
| 49 | `scripts/replay-bench/env.go:143` | `if err != nil {` | UNREACHED | protection | T | `TestReadSidecarReportsAMissingFile`: `err == nil` fails. |
| 50 | `scripts/replay-bench/results.go:85` | `if err := os.MkdirAll(dir, 0o755); err != nil {` | UNREACHED | protection | T | `TestWriteResultFailsWhenTheDirectoryCannotBeMade`: error must be a `*fs.PathError` with Op `mkdir`. |
| 51 | `scripts/replay-bench/results.go:90` | `if err != nil {` | UNREACHED | protection | T | `TestWriteResultRefusesAResultJSONCannotHold` (NaN in PerRepeat): error required and no file left. |
| 52 | `scripts/replay-bench/results.go:103` | `if err != nil {` | UNREACHED | protection | T | `TestWriteResultReportsOpenWriteAndCloseFailures/open` (seam `createExclusive`): neutralised, nil-file panic. |
| 53 | `scripts/replay-bench/results.go:108` | `if werr != nil {` | UNREACHED | protection | T | Same test `/write`: error required and no CSV written. |
| 54 | `scripts/replay-bench/results.go:111` | `if cerr != nil {` | UNREACHED | protection | T | Same test `/close`: error required and no CSV written. |
| 55 | `scripts/replay-bench/results.go:115` | `if err := os.WriteFile(csvPath, []byte(metricsCSV(r)), resultFilePerm); err != nil {` | UNREACHED | protection | T | `TestWriteResultFailsWhenTheCSVCannotBeWritten` (directory at the CSV path): error required. |
| 56 | `scripts/replay-bench/results.go:145` | `if math.IsNaN(x) {` | INERT | protection | A | `TestUndefinedStatisticsReadAsUndefinedInTheCSVAndTheSummary`: CSV row must end `,undefined`; no `NaN` anywhere. |
| 57 | `scripts/replay-bench/results.go:158` | `if reason == "" {` | UNREACHED | protection | T | Same test: `**NOISY: load above threshold.**` required. |
| 58 | `scripts/replay-bench/results.go:170` | `if s.Notes != "" {` | INERT | incidental | A | Same test: the scenario's notes must be rendered. |
| 59 | `scripts/replay-bench/results.go:185` | `if math.IsNaN(x) {` | UNREACHED | protection | T | Same test: row `\| undefined \| 0 \| undefined ...` required. |
| 60 | `scripts/replay-bench/results.go:188` | `if math.Abs(x) >= 100 {` | INERT | incidental | A | Same test: row `\| big \| 2 \| 2500 \| 100 \| 99.50 ...` (neutralised, `2500.00`). |
| 61 | `scripts/replay-bench/results.go:195` | `if math.IsNaN(x) {` | INERT | protection | A | Same test: CV cell `undefined`, not `NaN`. |
| 62 | `scripts/replay-bench/run.go:159` | `if err != nil {` | UNREACHED | protection | T | `TestExecuteReportsAComponentSuiteThatCannotBeBuilt`: error required before any repeat (`len(res.Repeats) == 0`). |
| 63 | `scripts/replay-bench/run.go:167` | `if err := add(newColdSuite(cfg, up.URL), nil); err != nil {` | UNREACHED | incidental | R | Removed: `add(newColdSuite(...), nil)` cannot return an error; the suite is appended directly. Reject if the uniform `add` call shape is preferred. |
| 64 | `scripts/replay-bench/run.go:172` | `if err := add(newLatencySuite(size, mask, up.URL)); err != nil {` | UNREACHED | incidental | R | Removed: `newLatencySuite` only builds a body and never returned a non-nil error; its error result is dropped and the suite appended. |
| 65 | `scripts/replay-bench/run.go:177` | `if err := add(newSSESuite(cfg, up.URL)); err != nil {` | UNREACHED | incidental | R | Removed: `newSSESuite`, same reason. |
| 66 | `scripts/replay-bench/run.go:181` | `if err := add(newConcurrencySuite(cfg, c, up.URL)); err != nil {` | UNREACHED | incidental | R | Removed: `newConcurrencySuite`, same reason. |
| 67 | `scripts/replay-bench/run.go:186` | `if err := add(newSessionSuite(cfg, n, up.URL), nil); err != nil {` | UNREACHED | incidental | R | Removed: `add(newSessionSuite(...), nil)` cannot return an error; appended directly. |
| 68 | `scripts/replay-bench/run.go:191` | `if err := add(newMaskComponentSuite(size)); err != nil {` | UNREACHED | protection | T | `TestExecuteReportsAComponentSuiteThatCannotBeBuilt/mask` (seam `openVault`): `errors.Is(err, errInjected)` with 0 repeats. |
| 69 | `scripts/replay-bench/run.go:195` | `if err := add(newLedgerComponentSuite(cfg)); err != nil {` | UNREACHED | protection | T | `.../ledger` (missing TMPDIR, no sizes): MkdirTemp error with 0 repeats. |
| 70 | `scripts/replay-bench/run.go:203` | `if meta.Noisy {` | INERT | protection | A | `TestExecuteFlagsNoisyRepeatsAndKeepsTheLabelFirst` (seam `currentLoad`): reason must start `known noisy; 2 of 2 repeats`. |
| 71 | `scripts/replay-bench/run.go:208` | `if err := s.repeat(r); err != nil {` | UNREACHED | protection | T | `TestExecuteNamesTheSuiteAndRepeatThatFailed`: error must name `coldstart/45KB, repeat 0`. |
| 72 | `scripts/replay-bench/run.go:216` | `if err := closeAll(); err != nil {` | UNREACHED | protection | T | `TestExecuteReportsASuiteThatFailsToClose` (seam `removeAll`): `errors.Is(err, errInjected)`. |
| 73 | `scripts/replay-bench/run.go:221` | `if noisyRepeats > 0 {` | INERT | protection | A | `TestExecuteFlagsNoisyRepeatsAndKeepsTheLabelFirst`: busy load must mark the result noisy with the count. |
| 74 | `scripts/replay-bench/run.go:227` | `if res.NoisyReason != "" {` | INERT | incidental | A | Same test: busy+label must give `label; count`; quiet+label exactly the label. |
| 75 | `scripts/replay-bench/run.go:244` | `if err != nil {` | UNREACHED | protection | T | `TestPairedRunBatchAndOverheadReportFailures`: error naming the failing arm. |
| 76 | `scripts/replay-bench/run.go:269` | `if err != nil {` | UNREACHED | protection | T | Same test: `putOverhead` with unequal arms must error. |
| 77 | `scripts/replay-bench/run.go:292` | `if err != nil {` | UNREACHED | protection | T | Same test: `batch` with a failing send must error. |
| 78 | `scripts/replay-bench/run.go:346` | `if err != nil {` | UNREACHED | protection | T | `TestFreshProxyReportsAProxyThatWillNotStart`: error required and no `proxied_prior_requests` recorded. |
| 79 | `scripts/replay-bench/run.go:355` | `if err != nil {` | UNREACHED | protection | T | `TestLatencySuiteReportsEachFailingPhase/proxy`: error required. |
| 80 | `scripts/replay-bench/run.go:362` | `if _, err := send[arm](); err != nil {` | UNREACHED | protection | T | `.../warmup` (upstream fails from request 1): error and exactly 1 upstream request. |
| 81 | `scripts/replay-bench/run.go:368` | `if err != nil {` | UNREACHED | protection | T | `.../pairs`: error and exactly 1 upstream request (neutralised, the batch runs too). |
| 82 | `scripts/replay-bench/run.go:375` | `if err := putOverhead(&s.collector, "", d, p); err != nil {` | UNREACHED | protection | R | Branch removed, error kept: `putOverhead` can only fail on unequal arms and `pairedRun` returns equal arms by construction. It now runs last and is returned (`return putOverhead(...)`); it reads samples already taken and sends nothing, so no measurement changes. Its own error path is tested at run.go:269. |
| 83 | `scripts/replay-bench/run.go:381` | `if err != nil {` | UNREACHED | protection | T | `.../batch`: error and exactly 1 upstream request. |
| 84 | `scripts/replay-bench/run.go:423` | `if err != nil {` | UNREACHED | protection | T | `TestSSESuiteReportsEachFailingPhase` (no proxy): error required. |
| 85 | `scripts/replay-bench/run.go:429` | `if _, err := send[arm](); err != nil {` | UNREACHED | protection | T | Same test, warmup: error and exactly 1 upstream request. |
| 86 | `scripts/replay-bench/run.go:434` | `if err != nil {` | UNREACHED | protection | T | Same test, upstream failing after 2: error required. |
| 87 | `scripts/replay-bench/run.go:450` | `if err := putOverhead(c, f.prefix, pick(pairs[ArmDirect], f.get), pick(pairs[ArmProxied],…` | UNREACHED | protection | T | `TestPairedRunBatchAndOverheadReportFailures`: `recordStream` with unequal arms must error. |
| 88 | `scripts/replay-bench/run.go:496` | `if g := runtime.NumGoroutine(); g > peak {` | INERT | incidental | A | `TestConcurrencyRunArmRecordsTheGoroutinePeak`: 100 sleeping workers, `GoroutinesPeak >= 100`. |
| 89 | `scripts/replay-bench/run.go:512` | `if err != nil && firstErr == nil {` | UNREACHED | protection | T | `TestConcurrencyRunArmStopsAWorkerAtItsFirstError`: `errors.Is(err, errInjected)`. |
| 90 | `scripts/replay-bench/run.go:517` | `if err != nil {` | UNREACHED | protection | T | Same test: a failing worker must send exactly 1 of its 5 requests. |
| 91 | `scripts/replay-bench/run.go:534` | `if err != nil {` | UNREACHED | protection | T | `TestConcurrencySuiteReportsEachFailingPhase` (no proxy): error required. |
| 92 | `scripts/replay-bench/run.go:541` | `if _, _, _, err := s.runArm(arm); err != nil {` | UNREACHED | protection | T | Same test, warmup: error and exactly 1 upstream request. |
| 93 | `scripts/replay-bench/run.go:550` | `if err != nil {` | UNREACHED | protection | T | Same test, upstream failing after 2: error required. |
| 94 | `scripts/replay-bench/run.go:596` | `if err != nil {` | UNREACHED | protection | T | `TestSessionSuiteRecordsTheBuildAndReportsEachFailingPhase` (no proxy): error required. |
| 95 | `scripts/replay-bench/run.go:605` | `if _, err := px.send(s.client, jsonBody, 0, session); err != nil {` | UNREACHED | protection | T | Same test, closed upstream: build failure must error. |
| 96 | `scripts/replay-bench/run.go:609` | `if s.n > 1 {` | INERT | protection | A | Same test: `build_mean_request_us` must hold one value for n=2 (the branch also guards a divide by zero at n=1). |
| 97 | `scripts/replay-bench/run.go:614` | `if err != nil {` | UNREACHED | protection | T | Same test, upstream failing after 1: pair failure must error. |
| 98 | `scripts/replay-bench/run.go:617` | `if err := recordStream(&s.collector, r, pairs); err != nil {` | UNREACHED | protection | R | Branch removed, error kept: same reason as run.go:375; now `return errors.Join(recordStream(...), px.Close())`. `recordStream`'s error is tested at run.go:450. |
| 99 | `scripts/replay-bench/run.go:648` | `if arm == ArmProxied {` | INERT | incidental | A | `TestColdSuiteRunsTheProxiedArmThroughAFreshProxyAndClosesIt`: `proxied_startup_us` must have one value. |
| 100 | `scripts/replay-bench/run.go:650` | `if px, err = startProxy(s.upstream, false); err != nil {` | UNREACHED | protection | T | `TestColdSuiteReportsAFailedRequestAndAProxyThatWillNotStart` (no proxy): error required. |
| 101 | `scripts/replay-bench/run.go:665` | `case i >= s.cfg.ColdFirstN+s.cfg.ColdWarm:` | INERT | incidental | A | `TestColdSuiteRunsTheProxiedArmThroughAFreshProxyAndClosesIt`: steady runs must hold 2 samples. |
| 102 | `scripts/replay-bench/run.go:670` | `if px != nil {` | INERT | protection | A | Same test: TMPDIR must be empty afterwards (neutralised, the proxy's directory is left). |
| 103 | `scripts/replay-bench/run.go:673` | `if err != nil {` | UNREACHED | protection | T | `TestColdSuiteReportsAFailedRequestAndAProxyThatWillNotStart`: closed upstream must error. |
| 104 | `scripts/replay-bench/run.go:706` | `if err != nil {` | UNREACHED | protection | T | `TestMaskComponentSuiteReportsEachFailure/tempdir`: MkdirTemp's own error required. |
| 105 | `scripts/replay-bench/run.go:710` | `if err != nil {` | UNREACHED | protection | T | `.../vault` (seam `openVault`): `errors.Is(err, errInjected)`. |
| 106 | `scripts/replay-bench/run.go:726` | `if _, _, err := s.mask.Mask(s.body); err != nil {` | UNREACHED | protection | T | `.../mask` (`failingMasker` via the new `masker` interface): error required. |
| 107 | `scripts/replay-bench/run.go:750` | `if err != nil {` | UNREACHED | protection | T | `TestLedgerComponentSuiteReportsEachFailure/tempdir`: MkdirTemp's own error required. |
| 108 | `scripts/replay-bench/run.go:754` | `if err != nil {` | UNREACHED | protection | T | `.../open` (seam `openLedger`): `errors.Is(err, errInjected)`. |
| 109 | `scripts/replay-bench/run.go:777` | `if err := s.store.Append(rec); err != nil {` | UNREACHED | protection | T | `.../append` (directory removed first): error required. |
