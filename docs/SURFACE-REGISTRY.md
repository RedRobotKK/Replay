# Surface registry

**Version 2026-09-28.** A dated inventory of the surfaces Replay reads, at the
conformance level the code actually reaches. It is not a claim about which AI
surfaces exist in the world, and a surface listed here as a candidate is not on
any roadmap by virtue of appearing.

Levels are evidence-bounded. **L4 means verified where the source exposes
sufficient evidence**, never that a missing field was filled in to reach it. A
field the source does not report stays `NOT_MEASURED`.

| level | meaning |
|---|---|
| **L0** | Discovered. Identified and documented, nothing reads it |
| **L1** | Ingested. Replay can read the source's records |
| **L2** | Measured. Usage, economics and available performance normalised |
| **L3** | Optimized. Findings and recommendations apply |
| **L4** | Verified. A follow-up comparison and an observed outcome |

## Conformance matrix

Derived from the code at `2974d192`. A cell is filled only where a code path
exists, not where one is planned.

| Surface | Category | Evidence source | Ingest | Usage | Cache | Cost | Perf | Optimize | Verify | Level |
|---|---|---|---|---|---|---|---|---|---|---|
| Claude Code CLI | Agent | `SourceTranscript` (`claudecode.go`) | ✅ | ✅ | ✅ | derived | `NOT_MEASURED` | ✅ `advisor` | ✅ `advice.json` | **L4** |
| Replay proxy | Replay | `SourceLedger` (`ledger/record.go`) | ✅ | ✅ | ✅ | derived | ✅ `LatencyMS` observed | ✅ | ✅ | **L4** |
| Codex CLI | Agent | `SourceCodex` (`codex.go`) | ✅ | ✅ | ⚠️ write field present, uniformly zero | derived | `NOT_MEASURED` | ✅ | untested | **L3** |
| Ollama | Local runtime | `SourceOllama` (`ollama.go`) | ✅ | ✅ | counts only, no share | **`NOT_MEASURED` by design** | ✅ throughput derived | ✅ `advise` | untested | **L3** |
| Jev capture | Eval | `SourceJev` (`jev.go`) | ✅ | ✅ | n/a | n/a | `NOT_MEASURED` | ✗ | ✗ | **L2** |
| OpenAI-compatible | Direct API | `openai.go` (`ParseOpenAIRequest`) | ✅ request only | partial | inclusive-count aware | derived | `NOT_MEASURED` | ✗ | ✗ | **L1** |
| Grok / xAI | Agent | `discover` knows `~/.grok`; **no reader** | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | **L0** |
| Cursor | Agent | `~/.cursor`, `state.vscdb`; **no reader** | ✗ | ✗ | none in source | ✗ | ✗ | ✗ | ✗ | **L0** |
| DeepSeek, Gemini, Mistral | Direct API | none | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | **L0** |
| OpenRouter, LiteLLM, Open WebUI | Gateway | none | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | **L0** |
| vLLM, llama.cpp | Local runtime | none | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | ✗ | **L0** |

Replay's own CLI, statusline, web, MCP and export are presentation surfaces over
the rows above, not separate evidence sources.

## Candidates

Listed so that discovering one again does not read as new. **Not in scope.**

LM Studio · LocalAI · Open Responses-compatible sources · further coding agents ·
further gateways, providers and runtimes.

## Rules this registry enforces

1. **A missing provider field stays `NOT_MEASURED`.** Codex's cache-write field
   is present and uniformly zero across the corpus, which is a third state and
   not a licence to infer a write happened.
2. **Local inference gets no dollar figure.** Electricity, amortised hardware
   and opportunity cost are not modelled, so Ollama's cost cell is
   `NOT_MEASURED` rather than zero.
3. **No timing is reconstructed across a boundary the source does not draw.** An
   Ollama server log establishes prompt-eval, eval and total time. It says
   nothing about when the first token reached the client, so TTFT stays
   unmeasured rather than being inferred from figures that do not bound it.
4. **Parsing is not support.** A surface reaches a level when a user-reachable
   code path uses the evidence, not when a struct field is populated.

## Known gap, recorded

`OllamaRequest.PromptMS/EvalMS/TotalMS` were parsed and read by nothing: at
`2974d192`, `git grep PromptMS` returned the parser and no consumer. Throughput
is now derived and reported by `replay burn`. `PromptProcessMS` and `TotalMS`
are carried in the canonical `Perf` record and are not yet displayed anywhere.
