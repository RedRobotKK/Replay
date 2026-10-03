# E7-01/E7-07: does provider syntax disappear at the adapter boundary?

**2026-10-02. Campaign Phase 7. Result: the campaign's stated expectation is
wrong for this system, the repository's opposite choice is defensible, and it has
one measurable cost that is already a live defect.**

## Question

The brief states: "The provider-specific syntax should disappear at the adapter
boundary." E7-07 calls cross-source composition "a major architecture test".
This measures whether that is true, and whether it should be.

## The Replacement Test, measured

Non-comment references to any provider name (anthropic, codex, grok, ollama) in
the reconstruction core:

| package | non-comment provider references |
|---|---:|
| **`internal/analysis`** | **0** |

Three hits exist in that package and all three are inspected:

| site | what it is |
|---|---|
| `internal/analysis/calibrate.go:38` | a comment |
| `internal/analysis/replay.go:203` | a provider-specific REACHABILITY STRING inside `PolicyResult.ReachableLive`, naming a Claude Code setting |
| `internal/analysis/trim.go:148` | **`model := "claude-opus-5"`**, a hardcoded pricing fallback when the lane names no model |

So the core's LOGIC is provider-free and two provider-specific VALUES are
embedded in data. `internal/cachemodel` is provider-specific by design: it is a
price table, which is an adapter.

The Anthropic wire names on `transcript.Usage`
(`cache_creation_input_tokens`, `ephemeral_5m_input_tokens`) do not leak as a
dependency: every hit outside `internal/transcript` is a comment, the proxy's own
Anthropic-shaped record in `internal/ledger`, or the OTel export mapping.

**Replacement Test: PASSES for the core, with two named leaks.**

## E7-01: the adapters do NOT converge, and that is deliberate

| parser | returns |
|---|---|
| `ParseClaudeCode` | `*Session` |
| `ParseCodex` | `*CodexSession` |
| `ParseOllamaLog` | `[]OllamaRequest` |
| `ParseOpenAIRequest` | `*OpenAIRequest` |

There is **no normalizer** from the provider types to `Session`. They are
parallel ontologies, and the concepts they carry do not translate:

- `CodexSession` separates `Billed` from `Reported` and flags `Rebased`, because
  Codex re-emits `last_token_usage` unchanged and rebases its running total on
  compaction. Anthropic has no analogue; forcing this into `Usage` destroys the
  distinction between "what was paid for" and "what the client last said".
- `OllamaRequest.CachedPrefix` is `n_past`, a KV-cache prefix count that is free
  and invisible to the total. It is deliberately NOT mapped to `Usage.CacheRead`,
  which is a billed quantity.
- `Usage.Create5m`/`Create1h` is an Anthropic TTL split neither other provider has.

**The campaign's expectation is therefore wrong for this system.** Collapsing
these into one shape would make Anthropic's wire format the ontology for every
provider, which is precisely the failure mode the brief forbids one paragraph
earlier. The repository chose parallel ontologies and the choice is right.

## The measurable cost, and it is a live defect

Because there is no shared type, every downstream surface is written per
provider, and the disclosure discipline has drifted:

| surface | an unreadable input |
|---|---|
| `cmd/replay/burn.go:545` (**Grok**) | counted in `r.Unreadable` and **disclosed**: "N line(s) or entries did not parse and are counted nowhere". Also discloses `Inconsistent` turns |
| `cmd/replay/burn.go:570` (**Claude Code**) | **silent.** The comment reads "a file that will not parse is counted nowhere" and there is no counter |

The same sentence is a disclosure on one surface and a rationalisation for
silence on the other. `cmd/replay/cost.go` counts `unreadable` and reports it, a
third standard in the same binary.

This is the parallel-ontology tax: the semantics are preserved per provider and
the DISCLOSURE is re-implemented per provider, so it diverges.

## Conclusion

- **E7-01 provider substitution: the premise is FALSIFIED and the design is
  sound.** Syntax should not disappear; coverage differences are real evidence.
- **E7-02 information loss: already ESTABLISHED** elsewhere. `surface.Class`
  encodes exactly "same semantic event is not necessarily same evidence
  coverage", with RPL-C001 BOUNDED over four surfaces and three refused.
- **E7-07 cross-source composition: NOT ATTEMPTED**, and the evidence says it
  should not be. Composition would require the convergence this experiment
  argues against.
- **The real Phase 7 work is not an adapter.** It is one shared disclosure
  helper for "inputs I could not read", applied to three surfaces that already
  disagree.

## Limitations

Static analysis plus source inspection. No behavioural substitution run was
performed, because the parallel types mean there is no common output to compare:
the absence of a convergence point IS the result. The two leaks in
`internal/analysis` were not repaired here.
