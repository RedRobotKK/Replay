# Proxy overhead harness: SCOUT run, 2026-10-30

This file records the first runs of `scripts/replay-bench` ([ADR-0029](../adr/0029-proxy-overhead-is-measured-paired-and-interleaved.md)). It also explains why the three earlier overhead figures disagree.

**Every number here is SCOUT.** They come from a loaded laptop while another agent ran Go race tests on it. This is not a baseline, and nothing here may be quoted as one. They show what the harness can see and where the method was wrong. They do not say how fast the proxy is. A baseline run must meet the acceptance bar in the gap table below, on a quiet machine. None of these figures is to be carried into it.

## Environment

| | |
|---|---|
| Date | 2026-10-30, 22:04:41Z to 22:18:00Z (run 2) |
| Commit measured | `794bed7` (run 2), `f98f27a` (run 1). The proxy code is identical to `main` at `fa10b39`; only the harness differs |
| Go | go1.27.3 darwin/arm64 |
| CPU | Apple M1 Pro, 10 logical CPUs, GOMAXPROCS 10 |
| Load, 1/5/15 min | before 2.33 / 2.28 / 3.30; after 2.71 / 4.53 / 4.17 |
| Per-repeat 1-min load | 2.33, 2.35, 2.19, 3.01, 3.31, 5.49, 10.02, 4.89, 4.41, 4.47. **7 of 10 above the noisy threshold** (0.25 per CPU, so 2.5 here) |
| Other processes | sum of %CPU over all processes 123.8% before and 105.6% after. Top consumer before: `codex` at 77.6% of one core. Full lists are in the run log |
| Peak RSS of the harness | 282,001,408 bytes |
| Label | `noisy — concurrent workloads present (another agent running Go race tests on this machine)` |

## Commands and exit statuses

```sh
OUT=docs/evidence/harness-scout-2026-10-30 ./scripts/replay-bench/run.sh \
  -noisy-label "noisy — concurrent workloads present (another agent running Go race tests on this machine)"
```

| Run | Commit | Exit | Duration | Raw result | Run log |
|---|---|---|---|---|---|
| 1 | `f98f27a` | 0 | 1,265 s | [json](harness-scout-2026-10-30/replay-bench-20261030T220145Z.json), [csv](harness-scout-2026-10-30/replay-bench-20261030T220145Z.csv) | [log](harness-scout-2026-10-30/replay-bench-run-20261030T214038Z.txt) |
| 2 | `794bed7` | 0 | 798 s | [json](harness-scout-2026-10-30/replay-bench-20261030T221800Z.json), [csv](harness-scout-2026-10-30/replay-bench-20261030T221800Z.csv) | [log](harness-scout-2026-10-30/replay-bench-run-20261030T220440Z.txt) |

A first attempt at run 1 exited 141 before measuring anything. `ps | head` under `pipefail` turned ps's SIGPIPE into the script's exit status. Fixed in `run.sh`.

**Edits made to the raw files after the run, and only these.** The results were written to `docs/evidence/harness-baseline-2026-10-30/`. The directory was then renamed, and every occurrence of that name inside the files was replaced with `harness-scout-2026-10-30`. The `-sidecar` argument recorded in each JSON file was a local `mktemp` path under `/var/folders/`, and it was replaced with `<sidecar>`. The harness now records that placeholder itself (`TestRecordedArgsCarryNoLocalSidecarPath`). No number was touched.

## Run 1 is withdrawn, and the reason is a finding

Run 1 kept one proxy per scenario for all ten repeats. In every such scenario, overhead rose with each repeat, and in a straight line:

| Scenario (run 1) | Paired p50 overhead by repeat, µs |
|---|---|
| latency/45KB/mask-off | 1109, 2631, 4294, 5943, 7583, 9933, 11550, 13070, 14472, 15979 |
| concurrency/c32, proxied rps | 260, 104, 66, 47, 37, 31, 26, 23, 20, 18 |

That is state, not noise. When a request carries no `x-claude-code-session-id`, the proxy uses the request body's `SessionHash` as its session ID (`internal/proxy/passthrough.go:218-220`). Identical requests therefore form one session that grows by one with every request. End-of-request `stats.rescore` then walks the whole session every time. A temporary CPU profile, taken over 3,000 session-less requests and deleted afterwards, put 56% of CPU time in `stats.rescore` and 34% in `analysis.AnalyzeLane`. Per-request latency rose from 3.4 ms at request 500 to 16.4 ms at request 2,999.

Run 2 starts a fresh proxy for every repeat and records that it had served no requests (`proxied_prior_requests` = 0 in every repeat; `TestEveryRepeatStartsOnAFreshProxy`). Within a repeat the session still grows from zero, to 310 requests in the 45KB scenario. Every repeat now sees the same growth, so the repeats are comparable. The session suite measures the growth on purpose.

## Run 2 results (SCOUT)

Median across 10 repeats, then [min – max], then the coefficient of variation. "Paired" means the median of proxied-minus-direct over back-to-back pairs whose order alternates. Sample counts are per repeat.

| Slice | Samples per repeat | Paired overhead p50 | Paired overhead p99 | CV of p50 |
|---|---|---|---|---|
| latency 45KB (44,981 B), mask off | 200 pairs | **978 µs** [825 – 1,044] | 1,639 µs [1,313 – 1,890] | 0.095 |
| latency 45KB, mask on | 200 pairs | **11.9 ms** [10.3 – 13.2] | 13.8 ms [11.1 – 23.5] | 0.118 |
| latency 1MB, mask off | 30 pairs | **10.9 ms** [8.7 – 11.4] | 14.0 ms [11.0 – 35.2] | 0.113 |
| latency 1MB, mask on | 30 pairs | **269 ms** [230 – 299] | 301 ms [237 – 606] | 0.128 |
| latency 16MB, mask off | 4 pairs | **143 ms** [122 – 173] | p99 of 4 is the max; not a percentile | 0.141 |
| latency 16MB, mask on | 4 pairs | **3.77 s** [3.64 – 4.76] | as above | 0.130 |
| SSE 45KB, 300 events: first byte | 100 pairs | **442 µs** [397 – 512] | | 0.110 |
| SSE 45KB, 300 events: message_stop to EOF | 100 pairs | **452 µs** [329 – 569] | proxied tail p99 696 µs [590 – 2,220] | 0.186 |

| End-of-stream tail by session length | 5 pairs per repeat | tail overhead p50 | CV |
|---|---|---|---|
| n = 10 | | 69 µs [50 – 262] | 0.677 |
| n = 100 | | 170 µs [140 – 198] | 0.123 |
| n = 250 | | 427 µs [284 – 1,205] | 0.576 |
| n = 500 | | 1.58 ms [1.16 – 1.99] | 0.219 |
| n = 1,000 | | 3.58 ms [3.22 – 5.77] | 0.241 |

First byte on the same streams stayed at 627–711 µs median for every n. Session length moves the tail and does not move the start of a stream.

| Throughput, 45KB, closed loop | direct rps | proxied rps | proxied p50 latency |
|---|---|---|---|
| concurrency 1 (16 requests per arm) | 12,928 [8,467 – 19,076] | 1,280 [1,094 – 1,500] | 756 µs |
| concurrency 8 (128) | 39,778 [17,367 – 46,540] | 1,216 [987 – 1,239] | 6.65 ms |
| concurrency 32 (512) | 19,498 [16,449 – 34,863] | 298 [257 – 333] | 99.2 ms |

| Allocation, from `runtime/metrics`, whole process per request | direct | proxied | proxy share (proxied − direct) |
|---|---|---|---|
| 45KB mask off: allocs / bytes | 99.5 / 74,430 B | 1,215 / 548,394 B | 1,110 / 474,259 B |
| 45KB mask on | 110 / 74,766 B | 2,514 / 710,341 B | 2,408 / 635,325 B |
| 1MB mask off | 302 / 155,632 B | 13,783 / 7.89 MB | 13,445 / 7.74 MB |
| 16MB mask off | 17,197 / 8.10 MB | 202,081 / 128.8 MB | 185,408 / 120.9 MB |
| 16MB mask on | 18,307 / 8.27 MB | 591,787 / 181.8 MB | 578,241 / 174.2 MB |

| Cold start, 45KB | median [range] |
|---|---|
| proxy `New` to listening | 107 µs [62 – 242] |
| first request, fresh connections: direct / proxied | 504 µs [325 – 1,117] / 1,123 µs [947 – 1,453] |
| steady p50 after 20 warm requests: direct / proxied | 115 µs / 847 µs |

| Component, called directly through exported APIs | ns per op, median [range] |
|---|---|
| `masking.Masker.Mask`, 45KB, no secret present | 9.80 ms [9.23 – 12.11] |
| `masking.Masker.Mask`, 1MB | 229 ms [221 – 288] |
| `masking.Masker.Mask`, 16MB | 3.71 s [3.55 – 4.60] |
| `ledger.Store.Append`, one minimal record | 45.2 µs [34.3 – 76.7] |

### What these SCOUT numbers suggest and do not establish

- **Masking is the largest cost the harness found.** The direct `Mask` call accounts for nearly all of the masking-on overhead: 9.8 of 11.9 ms at 45KB, 229 of 269 ms at 1MB, 3.71 of 3.77 s at 16MB. It grows with body size and is near-linear. Nothing here was optimised.
- **Proxied throughput does not rise with concurrency, and falls at 32.** One hypothesis, not tested: the proxy's upstream `http.Transport` (`internal/proxy/server.go`, `New`) does not set `MaxIdleConnsPerHost`, so Go's default of 2 applies, and more than two concurrent requests open and close upstream connections. The other candidates are shared locks in `stats` and the single derived session that every concurrent request lands in, which is serialized per session by `scoreMu` (`rescore_bench_test.go`). The harness cannot yet tell these apart.
- **There is a step mid-run.** Around repeats 4–5, many scenarios got 10–25% faster at the same point, even though the load average went *up*: 45KB mask-off went from about 1.03 ms to 0.85 ms. The cause is not established. The M1 Pro has two core types, and placement by the scheduler under foreign load is one candidate. This is the reason the run is SCOUT and not a figure.
- Run 1 against run 2 shows that the overhead a client sees depends on how many requests the proxy instance has already served. Any figure taken without stating that count is under-specified.

## Reconciliation with 98 µs, ~1.7 ms and ~2.8 ms

None of these is picked as right. They are different quantities taken by different methods, and none recorded what would let it be compared with the others.

| Figure | Source | Method | Upstream | Machine, load | Session state |
|---|---|---|---|---|---|
| 48 µs p50 / **98 µs** p99 | [proxy-latency-2026-09-03](proxy-latency-2026-09-03.md); [ROADMAP](../ROADMAP.md) v0.2 gate | 300 direct, then 300 proxied (blocked); proxied percentile minus direct percentile; one run | fake with ~44 ms latency | 4 vCPU Xeon 2.80GHz, Linux, Go 1.24; load not recorded | Unknown. The file entered git in a docs reorganisation on 2026-09-04 (`5db1382`), so which code produced it cannot be read from history. The session-hash fallback and rescore call landed in `db17901`, 2026-09-03 01:50 UTC |
| **~1.7 ms** p50 (2,539 µs p99) | 2026-09-05 addendum, same file; [evidence README](README.md) | `BenchmarkAddedLatency/instant -benchtime 300x`: blocked arms; percentile difference with index floor(p·(n−1)); one run | instant | Apple M1 Pro; Go version and load not recorded | After `db17901`. A fresh proxy receives 20 warm-up and 300 measured requests, with no session header. On this harness's evidence they form one session growing to 320 |
| **~2.8 ms** | Not found anywhere in the repository. The nearest is 2.50–3.20 ms p50 (p99 4.10–7.59 ms), reported to this workstream by another seat on 2026-10-30 and not recorded in the repository | `go test -bench . -count 6 -cpu 1,8`: the same benchmark with default benchtime, so the framework chooses b.N | instant | "one loaded laptop", per that seat | Session length at measurement follows b.N, which the framework chooses. `-cpu 1` sets GOMAXPROCS to 1 for half the runs |
| 978 µs p50 [825 – 1,044] | this file, run 2, SCOUT | paired, ABBA order, 10 repeats, fresh proxy per repeat | instant, in-process | M1 Pro, noisy, recorded | session grows from 0 to 310 within each repeat |

Methodological differences that can be sourced:

1. **Paired difference against percentile difference.** Here, on run 2's 45KB slice, the two agree to within 1% at p50: 978 µs paired, 971 µs percentile difference. The 98 µs figure was not wrong because of the subtraction alone. The addendum's point still stands, because the subtraction rested on a 44 ms upstream whose jitter is larger than the effect being measured.
2. **Upstream delay.** The 98 µs figure used a 44 ms upstream, and every other figure here used an instant one. The addendum's own 44 ms row (5,162 µs) shows that this setting did not give a stable answer.
3. **Session growth.** This is found here, and it was not considered by any earlier figure. A session-less benchmark request still pays rescore over a session that grows with every request (`passthrough.go:218-220`). A figure depends on how many requests preceded it: compare run 1's 1.1 → 16.0 ms. `-benchtime 300x` and default benchtime put the measurement at different session lengths. This may explain part of the gap between ~1.7 ms and 2.5–3.2 ms. It is not shown to, because neither of those runs recorded b.N together with its load.
4. **Load and GOMAXPROCS.** The 2.5–3.2 ms figure was taken on a loaded machine, half of it at GOMAXPROCS 1. The ~1.7 ms and 98 µs figures recorded no load at all.
5. **Bytes per op.** The other seat's 620–730 KB/op and ~1,550 allocs/op came from `-benchmem` on `BenchmarkAddedLatency`, where one op is one direct plus one proxied request, plus warm-up spread across the run. Here, direct plus proxied at 45KB is 74,430 + 548,394 B = 622,824 B, and 99.5 + 1,215 = 1,314.5 allocs. That is the same order, but not the same quantity.
6. **Rescore.** The other seat's 23.5 ms for `BenchmarkRescoreWholeSession` at n = 200 is the sum of 200 rescores, about 118 µs each on average. That is consistent in order with the tail overhead here at n = 100 (170 µs) and n = 250 (427 µs). The tail also includes the ledger append (about 45 µs) and is not rescore alone.

This change does not edit the 98 µs in ROADMAP or the ~1.7 ms in the evidence README. Another workstream owns those documents. The discrepancy is stated here, for that workstream to resolve with a baseline run.

## Gap against the acceptance bar

The acceptance bar is the Controller's section 5, saved at `grok-section5-benchmark-methodology.md` outside this repository.

| # | Requirement | Status |
|---|---|---|
| 1 | Recorded streaming corpus with a tool-heavy turn and an unreadable POST | **Not met.** Bodies are generated: tools are defined, but no turn carries `tool_use` or `tool_result` blocks, and nothing is replayed from a recording. No unreadable POST |
| 2 | Cold start and steady state separately | **Met** for 45KB (`coldstart/45KB`). Retries are off, so the retry path's connection reuse is not exercised |
| 3 | p50/p95/p99 on a named slice with the sample count | **Partly met.** Paired p50/p95/p99 per named slice, and per-repeat counts in params and raw samples. The renderer does not print the sample count beside each percentile. 16MB has 4 pairs, so its p99 is a maximum |
| 4 | Feature flags on the artifact: masking, policy, retries and spend cap, each on and off | **Not met.** Masking is on and off. Policy, retries and spend cap are always off, and the artifact does not list flags as such |
| 5 | Allocations and peak RSS beside latency | **Partly met.** allocs/op and B/op sit beside every latency slice. Peak RSS is read once per process, not per slice |
| 6 | One command, plus a hash of the binary and of the corpus | **Partly met.** One command, with git SHA and Go version recorded. No binary hash, and no corpus hash because there is no corpus |
| 7 | Upstream arrivals reconciled against ledger rows, with any difference named; missing usage is NOT MEASURED | **Not met.** The mock upstream does not count arrivals, and the harness does not read the ledger back |
| 8 | No comparison to other products without the same corpus, machine and criteria | **Met.** None is made |
| pin | `FlushInterval -1` and `DisableCompression true` are not to be touched | **Met.** No production code changed |

## Proposals, not implemented

1. **Spans inside the request** (production change, so separate). These would be optional timestamps for body read complete, mask done, summarize done, guard done, upstream request written, first upstream byte, last byte to the client, and bookkeeping done. Bookkeeping should be broken down into observe, `Store.Append` and rescore. The span from body read to upstream written is the §12 `replay_added_latency_seconds`, which no client-side harness can measure.
2. **An injectable upstream transport or root pool** in `proxy.Config`, so that a TLS upstream can be measured. The transport now trusts only the system roots, so the `httptest` TLS server cannot be used without a production change. That is why the TLS row is NOT MEASURED.
3. **A run that varies the upstream transport's `MaxIdleConnsPerHost`**, to test the concurrency hypothesis above before anyone changes it.
4. **What the bar needs:** a recorded corpus (point 1), with an unreadable POST. Policy, retries and spend cap on and off (point 4). An arrivals counter in the mock upstream, reconciled against `ledger` rows (point 7). Binary and corpus hashes (point 6). Peak RSS per slice (point 5).
5. **Separate processes** for the client, the upstream and the proxy, to remove the shared-GOMAXPROCS confound. This needs orchestration from outside Go, because os/exec is confined (`x402_test.go` X6c).

## Not measured

TLS on either hop. A real provider, or any upstream delay. Any client other than this harness's Go `http.Client`. Policy, retries, spend cap, freeze-prefix, siblings and rehydration. Masking with a secret present: only the no-match scan was measured. The OpenAI and Responses paths. Linux. Unix-socket listeners. Crash durability, which is a safety test and not a job for this harness.

## Re-run on a quiet machine

```sh
OUT=bench-results ./scripts/replay-bench/run.sh
```

Close other workloads first. Every repeat whose 1-minute load starts above 0.25 per CPU is flagged in the result, and the summary leads with the flag. That run is still not a baseline until the gaps above are closed.

---

[Evidence](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)
