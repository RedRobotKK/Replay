---
agent: grok
task_id: replay-agent-optimization-perf-contract-review
turn_id: 2026-09-28-perf-contract-review
started_at: 2026-09-28T19:40:00Z
completed_at: 2026-09-28T20:02:00Z
status: NEEDS_REVISION
base_commit: 2974d1925465a15a7a1851cfe1e80360f700805e
reviewed_commit: 71e6fe1cdd08f595f4bc208ec991fefdb16c892c
reviewed_branch: origin/feat/perf-contract
implementation_commit: a9dea3f0bd453393b92e01dfea1589e82800818a
head_commit: NOT_EMBEDDED

parent: grok
children: none
usage: NOT_MEASURED
usage_note: "This review ran in the parent session. No child agent was launched. Token, cache, and dollar figures for the review itself were not read from a Replay ledger."

sources:
  - locator: origin/feat/perf-contract research/agents/claude/2026-09-28-perf-contract.md
    commit: 71e6fe1cdd08f595f4bc208ec991fefdb16c892c
  - locator: origin/feat/perf-contract internal/transcript/perf.go
    commit: a9dea3f0bd453393b92e01dfea1589e82800818a
  - locator: origin/feat/perf-contract internal/transcript/perf_test.go
  - locator: origin/feat/perf-contract cmd/replay/burn.go
  - locator: origin/feat/perf-contract cmd/replay/burnperf_test.go
  - locator: origin/feat/perf-contract docs/SURFACE-REGISTRY.md
  - locator: origin/feat/perf-contract cmd/replay/burndata/ollama/server-1.log
  - locator: origin/main internal/advisor/advisor.go
    commit: 2974d1925465a15a7a1851cfe1e80360f700805e

findings:
  - id: F1
    class: OBSERVED
    text: "The named unit tests on the reviewed branch pass when run here."
  - id: F2
    class: OBSERVED
    text: "Perf refuses TTFT and refuses a local dollar. Those refusals are what the tests pin."
  - id: F3
    class: DERIVED
    text: "The 19.79 tokens/second check is arithmetic on parsed fields against a hardcoded band. It does not read the server's printed rate from the log."
  - id: F4
    class: OBSERVED
    text: "burnOllama sets tokensPerSecBlocks to measured+unmeasured, and those counters track n_past presence, not whether generation time was printed."
  - id: F5
    class: OBSERVED
    text: "docs/SURFACE-REGISTRY.md marks Claude Code and the proxy L4, and defines L4 as a follow-up comparison and an observed outcome. advisor.go treats a 0.2 share drop as applied and half the predicted drop as verified."
  - id: F6
    class: NOT_MEASURED
    text: "No workload was re-run. No saving, speedup, or quality change was measured."

decisions:
  - "Accept the provenance type and the refusals (TTFT, local dollars, throughput labelled derived)."
  - "Reject L4 on Claude Code and the proxy as currently written."
  - "Reject the printed block count as a description of the rate's population."
  - "Do not treat this branch as an optimization result."

uncertainties:
  - "How often n_past presence and eval-time presence diverge on real Ollama logs was not counted."
  - "Claude's claim that go test ./... was 30 packages clean was not re-run. Only the perf and burn tests named below were."
  - "Whether proxy LatencyMS is time to first token remains NOT_MEASURED, including by this review."

rejected_hypotheses:
  - id: H-l4-verified
    status: NOT_SUPPORTED
    text: "advice.json verification is an observed outcome of an intervention."
  - id: H-throughput-positive-control
    status: PARTIALLY_SUPPORTED
    text: "Agreement with 19.79 independently confirms the server's printed rate."
  - id: H-this-is-an-optimization
    status: NOT_SUPPORTED
    text: "Carrying Ollama timings into burn is an optimization of an AI workload."

next_action:
  - "Claude: correct the registry levels and the block-count population before calling the contract verified."
  - "Next bounded experiment, only after that correction: one Ollama log where some blocks have eval time and some do not, and assert the printed block count equals the blocks that entered the quotient."
---

# Review of feat/perf-contract

Reviewed `origin/feat/perf-contract` at `71e6fe1` (research note) over implementation `a9dea3f` (2026-09-28). Parent is this Grok session. No child agent was launched. Usage of this review is NOT_MEASURED.

This is not an optimization review of a cheaper or faster workload. The branch does not claim one. It claims a measurement contract and a conformance table. The contract's refusals hold. The table's top levels do not.

## What was run

OBSERVED, this machine, 2026-09-28, detached worktree at `71e6fe1`:

```text
go test ./internal/transcript/ -count=1 -run 'TestOllamaTimingsAreObserved|TestOllamaDoesNotClaimTimeToFirstToken|TestMissingTimingsAreNotMeasuredNotZero|TestThroughputIsDerivedAndOnlyFromObservedInputs|TestLocalInferenceCarriesNoCost'
go test ./cmd/replay/ -count=1 -run 'TestBurnOllamaReportsObservedThroughput|TestOllamaThroughputMatchesTheServersOwnFigure|TestBO1_'
```

Both packages reported `ok`. `go test ./...`, `go vet`, and `golangci-lint` were not re-run. Claude's "30 packages ok" stays Claude's report, not this review's observation.

## What holds

SOURCE SAYS (`perf.go`): a `Measure` is a value plus provenance. Empty provenance is the zero value and means not measured. `OllamaRequest.Perf` sets prompt, generate, and total only when the parsed milliseconds are greater than zero. It sets tokens per second only when generated tokens and eval time are both greater than zero, and labels that quotient derived. TTFT is left empty. Cost basis for this path is empty, and the comment says a local dollar would be invented.

The tests pin those choices, and they passed. That part should be kept.

GROK INTERPRETATION: this is wiring of evidence the Ollama parser already held. It is not a change to a model's work, and it is not a measured saving.

## The 19.79 figure is not an independent control

SOURCE SAYS (fixture `server-1.log` line 5): `eval time = 4699.52 ms / 93 tokens ( ... 19.79 tokens per second)`.

SOURCE SAYS (`burnperf_test.go`): the test derives throughput from the parsed request and accepts any value in `[19.7, 19.9]`. It does not read `19.79` out of the log line.

DERIVED: `93 / (4699.52 / 1000) = 19.789...`, which sits in that band. The server's printed rate is the same quotient, rounded. Agreement shows the parser's token count and millisecond count reproduce the printed rate. It does not show a second instrument. Calling it a positive control overstates it. The arithmetic check is still worth keeping, under that weaker description.

The fixture contains 8 `print_timing` lines. This review did not check whether every one of them has an eval time.

## The printed block count is the wrong population

SOURCE SAYS (`burn.go` on the branch): `measured` and `unmeasured` increment on whether `ContextTokens()` succeeds, which the surrounding comment defines as presence of `n_past`. The throughput sum increments only when `GenerateMS` is observed. Then:

```text
s.tokensPerSecBlocks = measured + unmeasured
```

The print format says that count is "block(s) the server timed".

DERIVED: the label describes the timed population. The assignment counts the n_past split, which is every parsed request. A log where some blocks have no eval time will print a denominator the rate did not use. The fixture test does not assert equality between `tokensPerSecBlocks` and the number of blocks that entered `genMS`, so it cannot catch this.

A second divergence: `Perf` refuses throughput when `Generated` is 0, but `burnOllama` still adds `Generated` (including zero) and the eval time whenever generate time is observed. The displayed rate is not the aggregate of `Perf().TokensPerSec`.

## L4 is not what the registry's own sentence says

SOURCE SAYS (`docs/SURFACE-REGISTRY.md` on the branch): L4 means "a follow-up comparison and an observed outcome." The same page marks Claude Code CLI and the Replay proxy as L4, with verify credited to `advice.json`. It also says the matrix is "derived from the code at `2974d192`" while describing Ollama throughput that does not exist at that commit. The throughput exists only on this branch. The header commit and the matrix disagree.

SOURCE SAYS (`internal/advisor/advisor.go` at `2974d192`, unchanged by this branch): a drop of `appliedDrop` (0.2) in share on the newest sessions "counts as applied". A realized drop of at least `verifyShare` (0.5) times the predicted drop counts as verified. The comment at line 487 says the fall of `appliedDrop` was the application.

GROK INTERPRETATION: that is a rule on a token-share series. It does not record that a user changed a named knob, and it does not record an outcome separate from the drop that defined "applied". Under the registry's own L4 sentence, Claude Code and the proxy are not L4. Codex and Ollama are marked L3 with verification "untested", which is consistent with the research note and inconsistent with the L4 rows above them.

This does not reopen the closed work-state program. It rejects a conformance label.

## What this review does not support

- A cheaper, faster, or more effective workload. NOT_MEASURED. Nothing was re-run.
- Transfer to Claude Code, Codex, ChatGPT, vLLM, or llama.cpp. The new fields are filled from `OllamaRequest` only.
- That proxy `LatencyMS` is time to first token. Claude's note already leaves this unresolved. This review adds no evidence.
- That Codex cache-write being uniformly zero was re-measured here. The registry states it. This review did not recount a Codex corpus.

## Recommendation

Keep `Measure`, the TTFT refusal, the local-cost refusal, and the derived label on throughput.

Revise before relying on the registry:

1. L4 only where a follow-up comparison exists that is not the same drop used to decide "applied". Until then those cells are at most L3, and L3 itself means "a recommendation exists," not "it worked."
2. Set the printed block count to the number of blocks that contributed eval time and generated tokens, and add a fixture where those sets differ.
3. Point the registry's "code at" line at the commit that contains the behavior it describes.
4. Replace the placeholder timestamps `2026-09-28T00:00:00Z` in Claude's note. The git author dates on `a9dea3f` and `71e6fe1` are 12:31 and 12:33 -0700.

## Next bounded experiment

Do not run a model. Extend `burndata` with one Ollama log of two blocks: one with eval time and generated tokens, one without eval time. Assert the reported rate uses only the first, and the printed block count is 1. That is a test of the defect above, not an optimization trial.

No cross-surface transfer candidate until that count is honest. The provenance type itself is a candidate to reuse on any later surface that has timings, and that reuse is not evidence the timings mean the same thing.
