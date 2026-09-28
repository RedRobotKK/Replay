---
agent: claude
task_id: surface-architecture-perf-contract
turn_id: 2026-09-28-perf-contract
started_at: 2026-09-28T00:00:00Z
completed_at: 2026-09-28T00:00:00Z
status: PARTIAL
base_commit: 2974d1925465a15a7a1851cfe1e80360f700805e
head_commit: a9dea3f0bd453393b92e01dfea1589e82800818a

files_changed:
  - internal/transcript/perf.go
  - internal/transcript/perf_test.go
  - cmd/replay/burn.go
  - cmd/replay/burnperf_test.go
  - docs/SURFACE-REGISTRY.md
  - research/agents/claude/2026-09-28-perf-contract.md

tests_run:
  - "go test ./... : 30 packages ok, 0 failures"
  - "go vet ./... : clean"
  - "golangci-lint run ./internal/transcript/... ./cmd/replay/... : 0 issues"
  - "gofmt -l <changed files> : no output"
  - "mutation check: removing the `r.EvalMS > 0` guard in Perf() turns TestThroughputIsDerivedAndOnlyFromObservedInputs red; guard restored and suite re-run green"

findings:
  - "OBSERVED: OllamaRequest.PromptMS/EvalMS/TotalMS were parsed and read by no production code. At base_commit, `git grep PromptMS -- '*.go'` returns internal/transcript/ollama.go and no other non-test file."
  - "OBSERVED: ledger.Record.LatencyMS is consumed (proxy logging, state aggregation, output timestamp), so proxy latency was already wired while Ollama timings were not."
  - "OBSERVED: the canonical transcript.Request carried evidence provenance for identity (IDMeasured) and correlation (CorrelationUnmeasured) but no performance fields of any kind."
  - "DERIVED: Ollama throughput for the repository fixture is 19.79 tokens/second, independently computed from generated tokens over generation time. The server prints the same figure for the same block, which serves as a positive control on the derivation."
  - "OBSERVED: five surfaces have readers (claude-code-transcript, codex-rollout, ollama-server-log, jev-capture, replay-ledger) plus ParseOpenAIRequest. All other surfaces named in docs/SURFACE-REGISTRY.md are L0 with no reader."
  - "NOT_MEASURED: time to first token, for every surface. No source currently read establishes the boundary between request dispatch and first output byte."
  - "NOT_MEASURED: dollar cost for local inference. Electricity, amortised hardware and opportunity cost are not modelled."

decisions:
  - "Measure{Value, Provenance} with NotMeasured as the zero value, so an unpopulated timing reads as unmeasured rather than as zero. Follows the existing convention of CorrelationUnmeasured and advisor.meanSeen."
  - "TTFT is not derived from Ollama's prompt-eval and eval times. Those durations do not bound the client-visible boundary, and reconstructing it would assert a measurement the source does not make."
  - "Throughput is reported through a dedicated surfaceBurn field rather than the problems channel. The first implementation used problems and was rejected by the pre-existing TestBO1_TheCachedShareIsWithheldOnAFullyLabelledLog, which pins that this surface emits exactly one note."
  - "No provider adapters were implemented. Promoting an L0 surface requires a new reader per surface and yields no architectural leverage; the measurement contract was the only change with leverage across future surfaces."

unresolved:
  - "PromptProcessMS and TotalMS are carried in Perf but displayed nowhere. They are recorded as a known gap in docs/SURFACE-REGISTRY.md rather than left implicit."
  - "Whether the proxy's LatencyMS should populate Perf.TTFTMS. The proxy could in principle establish that boundary, but whether the recorded value does so is not established here."
  - "Codex verification (L3 to L4) is untested. Its cache-write field is present and uniformly zero across the examined corpus, which is a third state rather than evidence of no write."
  - "Ollama verification (L3 to L4) is untested."

next_action:
  - "Open a pull request for feat/perf-contract, or request review of the branch."
  - "Decide whether Perf.TTFTMS can be populated from the proxy ledger, on evidence rather than on plausibility."
---

# Performance measurements carried with their provenance

## What changed

`internal/transcript` gains a canonical `Perf` record. Every timing is a
`Measure` carrying a `Provenance` of `observed`, `derived`, `estimated` or
`NotMeasured`, and `NotMeasured` is the zero value so that an unpopulated field
reads as absent rather than as a measurement of zero.

`replay burn` now reports Ollama output throughput, derived from generated
tokens over the generation time the server printed, counted only over blocks
that carry one.

## Why

The Ollama reader has parsed prompt-eval, eval and total time since it was
written, and no production code read them. At the base commit,
`git grep PromptMS -- '*.go'` returns the parser and nothing else. This is the
same pattern `docs/design/UNWIRED-LOG.md` records nine times: a capability that
exists, passes tests, and cannot be reached by a user.

Cost already had this discipline. A price is versioned, a derived figure says
it is derived, and an absent one is `NOT_MEASURED`. Timings had none, which is
how evidence Replay already held about a surface it already supports reached no
reader.

## What is not claimed

Time to first token remains `NOT_MEASURED` on every surface. An Ollama server
log bounds prompt evaluation and generation; it does not establish when the
first token reached the client, because queueing, transport and streaming sit
outside those figures.

Local inference carries no dollar cost.

Throughput is labelled `derived`, not observed. It is a quotient of two
measurements, and it is computed only when both inputs were reported.

## Evidence

The derivation has a positive control. The repository fixture
`cmd/replay/burndata/ollama/server-1.log` contains a block whose eval line reads
`4699.52 ms / 93 tokens ... 19.79 tokens per second`. Replay computes the rate
independently from the count and the duration, and
`TestOllamaThroughputMatchesTheServersOwnFigure` fails if the two disagree.

`TestThroughputIsDerivedAndOnlyFromObservedInputs` was confirmed able to fail:
removing the `r.EvalMS > 0` condition in `Perf()` turns it red. The guard was
restored and the full suite re-run.

## Correction recorded

The first implementation wrote the throughput figure into `surfaceBurn.problems`.
`TestBO1_TheCachedShareIsWithheldOnAFullyLabelledLog`, which predates this
change, asserts that the Ollama surface emits exactly one note, and it failed.
That test is correct: a measurement printed in the problems channel reads as a
fault. The figure was moved to a dedicated field and the existing test was left
unmodified.

## Scope

No provider adapters were added and no surface was promoted from L0.
`docs/SURFACE-REGISTRY.md` records the conformance level of each surface as
established by the code, not by intent.
