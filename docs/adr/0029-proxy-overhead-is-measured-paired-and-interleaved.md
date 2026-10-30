# 29. Proxy overhead is measured paired and interleaved, with its spread and its machine

**Status:** Proposed
**Date:** 2026-10-30

## Context

This record sets how the repository measures what the proxy adds to a request.
Three figures for that overhead are in circulation and they disagree by more
than an order of magnitude:

- 98µs p99 (48µs p50), [proxy-latency-2026-09-03](../evidence/proxy-latency-2026-09-03.md),
  still cited by [ROADMAP v0.2](../ROADMAP.md) as the published gate figure.
- ~1.7ms p50, the 2026-09-05 addendum to the same file, which says the first
  figure was noise.
- 2.50–3.20ms p50, taken on 2026-10-30 by another seat on a loaded laptop with
  `BenchmarkAddedLatency`, and recorded nowhere in the repository.

The 2026-09-03 method sent 300 requests direct and then 300 through the proxy,
and subtracted one arm's percentile from the other's. That has three defects.
The arms are blocked in time, so anything that drifts during the run lands on
one arm. Subtracting two percentiles cancels nothing that moves both. And one
run has no spread, so a reader cannot tell a figure from its noise. The
addendum diagnosed the second defect, and `BenchmarkAddedLatency` still has the
first and third. No measurement recorded the load the machine was under, so
none of the three can be checked against the others.

Requirements §12 asks for `replay_added_latency_seconds` as the gap between the
last request byte in and the request going out. Nothing in the proxy exposes
that span. A client-side harness can only see the round trip, so it measures a
superset of the requirement.

## Decision

Proxy overhead is measured by `scripts/replay-bench`. It runs the real
`proxy.Server` against an in-process mock upstream. It sends direct and proxied
requests in back-to-back pairs whose order alternates (ABBA), and reports the
median of the per-pair differences beside the old percentile difference. It
runs at least ten repeats in repeat-major order and reports each metric's
median, range and coefficient of variation across them. With every result it
writes the toolchain, CPU, core count, GOMAXPROCS and load. Any repeat that
starts above 0.25 one-minute load per CPU is flagged noisy. Raw samples go to
timestamped files that are never overwritten.

## Consequences

- An overhead figure has a spread and a machine attached, so two figures can be
  compared or rejected as not comparable rather than argued about.
- Masking, body size, streaming tail, concurrency, session length and cold
  start are separate scenarios. One headline number no longer stands for all of
  them.
- The harness measures from the client. It cannot report the §12 span
  (last byte in to forwarded out) or split the proxy's time by phase. Doing
  either needs timestamps inside `internal/proxy`, which this record
  deliberately does not add. The evidence file lists them as a proposal.
- The mock upstream is plain HTTP. The proxy's upstream transport trusts only
  the system roots and has no injection point, so a TLS upstream cannot be
  measured without a production change. That is also a proposal, not done.
- Client, mock upstream and proxy share one process and one GOMAXPROCS. That
  isolates the proxy's code from the network. It also means the three compete
  for the same cores, which a deployed proxy does not.
- The harness imports `runtime/metrics`, added to the module's import allowlist
  with its reason. It is listed as a loopback-only sender in the outbound
  inventory and as correctly absent from the binary.
- `BenchmarkAddedLatency` stays as it is. It is a quick regression signal, and
  its figures are not interchangeable with the harness's: it picks percentiles
  as floor(p·(n−1)), the harness uses nearest rank, and its arms are blocked.

## Alternatives considered

**Fix `BenchmarkAddedLatency` in place.** `go test -bench` controls the
iteration count and reports one figure per benchmark. Repeats, interleaving,
environment capture and raw-sample files would all have to be built around it,
and a benchmark function is the wrong place to write files. Keeping it
unchanged also keeps the September figures reproducible by the method that
produced them.

**Measure inside the proxy with timestamps per phase.** This is the only way to
get the §12 figure, and it is the better instrument. It changes a production
code path, though, and this decision exists to fix the measurement before
anything is changed on the strength of one. Proposed separately.

**An external load generator (wrk, hey, k6).** These measure the round trip
well. They are not in this module, which has no third-party dependencies, and
they cannot see the runtime's allocation and GC counters, which is where the
proxy's per-request cost shows up.
