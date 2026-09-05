# Proxy Added Latency

How much time `replay serve` adds to a request, measured against a local fake provider so the number is the proxy's own overhead and not the network's.

## Method

- Fake provider: a local HTTP server returning a fixed 2KB Messages API response with usage fields, on loopback.
- Request: a 46,252-byte Messages API body (4KB system prompt, 20 tool definitions, 41 messages with 300-line tool results), sent 300 times directly and 300 times through `replay serve` with the ledger enabled, after 20 warm-up requests each way.
- Hardware: 4 vCPU Intel Xeon at 2.80GHz, Linux, Go 1.24.
- Timing: client-side wall clock per request; percentiles taken over the sorted samples. "Added" is the difference between the proxied and direct percentiles.

## Result

| Path | p50 | p99 |
|------|-----|-----|
| Direct to fake provider | 43.98ms | 48.04ms |
| Through `replay serve` | 44.03ms | 48.14ms |
| **Added by Replay** | **48µs** | **98µs** |

The fake provider itself dominates both columns; a real provider round trip is hundreds of milliseconds to minutes, so the overhead is below measurement noise in practice. Streaming responses are flushed per write and add no buffering delay; that path was verified for ordering in the test suite but not timed here.

## What is not covered

- Real provider latency and TLS handshake cost (the upstream connection pool keeps connections warm; the first request pays one handshake).
- Very large responses near the 16MB buffering cap for non-streaming bodies.
- Macs and Windows, which run the same code path but were not measured.

---

[Evidence](README.md) · [Documentation index](../README.md) · [Repository README](../../README.md)

---

## Addendum, 2026-09-05: the method above cannot measure what it claims

A benchmark reproducing this shape now lives in
`internal/proxy/latency_bench_test.go` and can be run with:

```sh
go test ./internal/proxy -run '^$' -bench BenchmarkAddedLatency -benchtime 300x
```

On an Apple M1 Pro, a faster machine than the Xeon above, it reports:

| Upstream | Added p50 | Added p99 |
|---|---|---|
| Instant (no delay) | **1697µs** | 2539µs |
| 44ms, as the fake provider above | 5162µs | 12816µs |

**Neither is close to 48µs, and the second is worse than the first.** That second row
is the tell. Real overhead cannot rise when the upstream gets slower, so what moved
is not overhead — it is noise.

**The original method subtracts two separately-measured percentiles.** Direct p50 was
43.98ms and proxied p50 was 44.03ms, and the difference between them was reported as
48µs of added latency. But both distributions carry jitter far larger than 48µs, so
their difference is dominated by that jitter rather than by the proxy. The note above
that "the fake provider itself dominates both columns" identified the problem exactly
and then drew the wrong conclusion from it: a quantity swamped by noise was recorded
as a small measurement rather than as no measurement at all.

**The honest figure is the instant-upstream one, around 1.7ms p50 at a 45KB request on
an M1 Pro**, and it is the one to watch for regressions because nothing hides in it.
It is also what a first-principles check would have predicted: `encoding/json` runs at
roughly 100–200 MB/s, so parsing a 45KB body cannot complete in 48µs whatever else the
proxy does. That arithmetic was available before either measurement was taken.

**What this does not change:** the practical claim still holds, and holds by a wide
margin. A real provider round trip is hundreds of milliseconds to minutes, so ~1.7ms
of proxy overhead remains between a tenth of a percent and a rounding error of the
request it sits inside. The tool is fast. The number published for it was arrived at
by a method that could not have detected it being otherwise, which is the part that
needed fixing.

The benchmark asserts no threshold. Shared runners are noisy enough that a tight bound
would fail for reasons unrelated to this code, and a benchmark that cries wolf gets
muted — which is worse than no benchmark. It reports; a human compares.
