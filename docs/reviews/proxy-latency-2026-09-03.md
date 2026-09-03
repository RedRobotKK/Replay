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
