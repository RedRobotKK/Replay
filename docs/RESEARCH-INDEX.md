# Research index

A catalogue of external findings, indexed so they can be looked up rather than
re-read. Compiled 2026-09-10 from five parallel literature sweeps.

**Why this file exists.** Five agents returned roughly 55 papers with a top-3
each. Fifteen recommendations, most of them contradicting at least one other,
none of them addressable without a way to ask "what does the literature say
about X" and get an answer in under a minute. A pile of reports is not a
reference. This is the reference.

---

## 0. Provenance, and what "verified" means here

This repository distinguishes Measured from Estimated (ADR-0002) and treats
absence, zero and unknown as three values (ADR-0018). The same discipline
applies to citations, in three tiers:

| Tier | Meaning |
|---|---|
| **ID-VERIFIED** | arXiv id, exact title and date fetched from the arXiv API and matched. The paper exists and is what the entry says it is. |
| **CLAIM-VERIFIED** | The headline number was additionally found in the paper's own abstract. |
| **REPORTED** | A subagent asserted the id and title. Not independently re-fetched. |

**What was actually checked on 2026-09-10.** Ten ids were re-fetched directly:
all ten returned matching titles and dates. Five of those were further checked
for their headline numbers in the abstract; every number was present. One
incidental signal is worth recording because it bears on the rest: the entry
for `2607.15516` carries a missing space in its own title
(`Compression:A Two-Tier`), which the reporting agent flagged as verbatim and
which the API confirms. A model reconstructing a plausible title from memory
normalises that typo. It did not.

**What was NOT checked.** No full text was read. Peer-review status was not
established, and a large share of these are 2026 preprints, several apparently
single-author. Existence is not soundness. **Any number from this index that
reaches a user-facing surface must be read in full text first** — that is the
FD-9 record-lag rule applied to somebody else's numbers instead of our own.

---

## 1. The semantic lens

Papers are indexed by **what the finding lets this project do**, not by topic.
Five facets, each of which can disqualify a finding on its own.

### Facet A — locus of control

The first question, because it decides whether advice is honest at all.

| Value | Meaning |
|---|---|
| `CLIENT` | The user holds the lever. Advice is actionable. |
| `HARNESS` | The agent's author holds it — the user can see the problem and not fix it. |
| `SERVER` | The provider holds it. Naming a client-side fix here is dishonest. |
| `NONE` | An evaluation result. Nobody holds a lever; it constrains what we may claim. |

### Facet B — action class

Inherited from the interceptor spec, where it governs what a proxy may touch.

| Value | Meaning |
|---|---|
| `MEASURE` | Read and report. No mutation. |
| `MUTATE-SAFE` | Identical token stream, different billing. Only `cache_control` placement qualifies. |
| `ADVISE` | Changes what the model sees. Report to a human; never do it silently. |
| `INERT` | The lever exists but does nothing on this surface. |

### Facet C — billing basis dependence

The constraint that has killed more candidate features here than any other.

| Value | Meaning |
|---|---|
| `METERED-ONLY` | The finding is denominated in dollars. False for a subscription seat. |
| `BASIS-FREE` | Denominated in tokens, turns, ratio or latency. True for everyone. |
| `LOCAL` | Applies where there is no bill at all; the currency is latency and headroom. |

`BASIS-FREE` is the single most useful column in this document. Every idea that
survived all five sweeps is denominated in something other than money.

### Facet D — detectability from a transcript

| Value | Meaning |
|---|---|
| `TRANSCRIPT` | Computable from what Replay already reads. |
| `PROXY` | Needs the request/response stream — a local proxy, which is a serious install. |
| `SECOND-STREAM` | Needs a source outside the transcript (a server log, a coverage profile). |
| `LABEL` | Needs an outcome signal the transcript does not carry. |
| `NOT-DETECTABLE` | Cannot be observed from our position at all. |

### Facet E — evidence grade

| Value | Meaning |
|---|---|
| `PRODUCTION` | Real fleet traces at scale. |
| `FIELD` | Real artefacts in the wild (repositories, pull requests, sessions). |
| `BENCHMARK` | Real code, curated task set. |
| `SYNTHETIC` | Constructed environment. |
| `ANALYTIC` | A model, no measurement. |
| `NONE` | Position or methodology paper. |

---

## 2. Primitive glossary

The nouns this project already computes. The catalogue's `consumes` column
points here, so "what could we do with `n_past`" is answerable by lookup.

| Primitive | Where it comes from | Notes |
|---|---|---|
| `cache_creation_input_tokens` | Anthropic usage, all surfaces | Carries a write premium |
| `cache_read_input_tokens` | Anthropic usage | Deeply discounted |
| `ephemeral_5m` / `ephemeral_1h` | Anthropic creation split | The only TTL-tier signal any provider gives |
| `cached_tokens` | OpenAI `prompt_tokens_details` | One field, no write/read split |
| `cache_write_input_tokens` | Codex | A field Anthropic's own wire does not carry |
| `reasoning_output_tokens` | Codex | Thinking billed separately |
| `codexWindow{UsedPercent,ResetsAt}` | Codex | A quota counter that actually moves |
| `n_past` | Ollama server log | KV reuse ground truth. **Only present on requests that HIT**, so any share computed from it is defined over a population selected by having been cached. Ceiling is (n−1)/n, never 1. |
| `prompt_cache_key` | Grok request | A client-declared key; the provider does not infer a prefix |
| inter-request wall-clock gap | Any transcript timestamp | The TTL-expiry signal |
| longest common prefix (LCP) | Two consecutive request bodies | Needs `PROXY` |
| declared breakpoint offset | Anthropic-family request | Compared against LCP |
| compaction boundary index | Transcript marker | The pivot for §4's strongest idea |
| tool-array hash | Request | Separates tool-list churn from conversation growth |
| provider request id | Response header | Without it, joins are by arrival order, which is wrong under fan-out |
| message-array shrinkage | Consecutive requests | Server-side pruning, where the client did not do it |

---

## 3. The catalogue

Sorted by facet A, then by how load-bearing the finding is. `K` is the stable
citation key; cite these, not the arXiv id, so a superseded paper can be
swapped without breaking every reference.

### 3.1 CLIENT — the lever is the user's

| K | arXiv | Finding in one line | B | C | D | E | Tier |
|---|---|---|---|---|---|---|---|
| `CACHE-KEEPALIVE` | 2607.19214 | Replaying the prefix on a ~4-minute timer (not the 30s convention) cuts post-pause cost up to 12.5x; break-even ~46 min idle on Anthropic | ADVISE | METERED-ONLY | TRANSCRIPT | BENCHMARK | CLAIM |
| `CACHE-DEADPOINT` | 2607.15516 | A breakpoint downstream of a per-request mutation can never hit; two-tier cache with a ~3,500-token threshold, hit rate plateauing ~0.83 not 1.0 | MUTATE-SAFE | BASIS-FREE | PROXY | BENCHMARK | ID |
| `COST-NOT-TOKENS` | 2607.12161 | Compression cut tool-output tokens 38.4% and RAISED billed cost 6.8%; token/cost correlation r=0.15; cache traffic dominates input spend | MEASURE | BASIS-FREE | TRANSCRIPT | BENCHMARK | CLAIM |
| `COMPACT-INVOICE` | 2608.16370 | Compression left completion unchanged (p=1.0) while retrieval calls went 21.0 → 63.9 (p=.002) — the agent silently reacquires | MEASURE | BASIS-FREE | TRANSCRIPT | SYNTHETIC | CLAIM |
| `WASTE-TRAJECTORY` | 2509.23586 | Trajectory reduction cut input tokens 39.9–59.7% and cost 21.1–35.9% at equal performance, on a real coding agent | MEASURE | BASIS-FREE | TRANSCRIPT | BENCHMARK | ID |
| `SEARCH-TAX` | 2608.05886 | Agents spend the budget finding the file, not patching it — 23 rounds and 631K tokens per resolved issue | MEASURE | BASIS-FREE | TRANSCRIPT | BENCHMARK | REPORTED |
| `THINK-TAX` | 2608.26235 | Reasoning share often dominates spend; diminishing returns, including cases where more thinking REDUCES accuracy | MEASURE | BASIS-FREE | TRANSCRIPT | BENCHMARK | REPORTED |
| `THINK-CONTRACT` | 2608.16956 | Explicit high effort cost +$0.0103/call with no detected accuracy gain; effort-omission semantics are model-specific and undocumented | ADVISE | METERED-ONLY | TRANSCRIPT | BENCHMARK | REPORTED |
| `THINK-LATE` | 2604.22266 | ~760 reasoning tokens generated AFTER the answer stabilised; early stop saves ~500 for a 2% accuracy drop | ADVISE | BASIS-FREE | NOT-DETECTABLE | SYNTHETIC | REPORTED |
| `HANDOFF-TAX` | 2608.24358 | Escalating to a stronger model mid-session recovers <half the quality gap at a cost premium — kills naive routing advice | ADVISE | METERED-ONLY | TRANSCRIPT | BENCHMARK | REPORTED |
| `TOOLS-VS-CACHE` | 2608.22708 | Progressive tool disclosure and prompt caching are structurally opposed; every tool-list change invalidates the prefix | ADVISE | BASIS-FREE | TRANSCRIPT | SYNTHETIC | REPORTED |
| `SELECTIVE-CACHE` | 2601.06007 | Naive full-context caching can be WORSE than selective; dynamic content belongs at the END of the system prompt | MUTATE-SAFE | BASIS-FREE | PROXY | BENCHMARK | REPORTED |
| `SCAFFOLD-COST` | 2608.08654 | Scaffolding dominates interface; the stable finding is failure spend — 12.9% of MCP spend bought no completed work vs 2.2% CLI | MEASURE | BASIS-FREE | TRANSCRIPT | BENCHMARK | REPORTED |
| `RESEND-DOMINATES` | 2608.24188 | Re-sent file reads and tool outputs dominate the bill; using a frontier model as compressor is net-negative | MEASURE | BASIS-FREE | TRANSCRIPT | BENCHMARK | REPORTED |

### 3.2 HARNESS / SERVER — the user can see it and not fix it

| K | arXiv | Finding | B | C | D | E | Tier |
|---|---|---|---|---|---|---|---|
| `TRACE-COPILOT` | 2608.00101 | Production traces, 3.2M users: cache hit 90% within a turn, **55% across turn boundaries**, "drastically invalidated" by model switches and compaction | MEASURE | BASIS-FREE | TRANSCRIPT | PRODUCTION | CLAIM |
| `TRACE-LAB` | 2606.30560 | ~4,300 real Claude Code and Codex sessions, ~350k steps, publicly released; "high but imperfect prefix cache hit rates" | MEASURE | BASIS-FREE | TRANSCRIPT | PRODUCTION | ID |
| `SERVER-OWNS` | 2605.27744 | Formal argument that cache and context policy belong to a runtime layer between harness and engine — the Grok case, generalised | INERT | BASIS-FREE | NOT-DETECTABLE | BENCHMARK | REPORTED |
| `TOKENIZE-TTFT` | 2607.29678 | Agents resubmit a long transcript after a ~1.4K-char append; as hit rate nears 0.99, tokenization grows to 64% of TTFT | MEASURE | LOCAL | TRANSCRIPT | PRODUCTION | REPORTED |
| `LOOPS-REAL` | 2607.01641 | 6,549 agent repositories scanned; 68 confirmed infinite-loop failures across 47 projects at 91.9% precision | MEASURE | BASIS-FREE | TRANSCRIPT | FIELD | REPORTED |
| `RATIONED` | 2608.23986 | Under congestion providers route down and truncate; a degraded answer returns as a retry. **The flat seat is the class rationed first.** | INERT | BASIS-FREE | NOT-DETECTABLE | ANALYTIC | REPORTED |
| `CACHE-DIVERGE` | 2609.04748 | With everything else fixed, enabling prefix caching changed agent trajectory on 36.2% of episodes at 16-bit, 75.0% at 4-bit; 0/800 with cache off | MEASURE | LOCAL | SECOND-STREAM | BENCHMARK | REPORTED |

### 3.3 NONE — constrains what we may claim

| K | arXiv | Finding | Why it is here |
|---|---|---|---|
| `ROT-BY-STEPS` | 2609.01660 | Degradation is driven by **step count, not context length**; bounding the window makes decay steeper (p=3e-6), 10,664 trajectories | Hostile to the naive pitch. Cite it to avoid shipping a claim that must be retracted. |
| `AGENTS-MATTER` | 2407.01502 | Accuracy-only leaderboards produce needlessly costly agents; cost-accuracy is a joint frontier | The canonical citation for why a cost tool matters. Cite, don't build. |
| `MAST` | 2503.13657 | 14 failure modes, 1600+ annotated real traces, κ=0.88 | A taxonomy label is not a dollar. Nobody pays for one. |

### 3.4 TEST ADEQUACY — the second product line

| K | arXiv | Finding | D | E | Tier |
|---|---|---|---|---|---|
| `ORACLE-SMOKE` | 2606.18168 | **80.2% of agent-authored test patches carry weak or no oracle signal** — 86,156 patches, 33,596 PRs, 2,807 repos, five agents | TRANSCRIPT | FIELD | CLAIM |
| `MUTATE-META` | 2010.13464 | Mutation plus instrumentation measuring which tests visited the mutated code. **>half of 15,000 mutants survived a rigorous suite.** | SECOND-STREAM | PRODUCTION | ID |
| `MUTATE-GOOGLE` | 2102.11378 | Surviving 24,000 developers needed arid-node suppression, per-line caps, and operator selection by historical action | SECOND-STREAM | PRODUCTION | REPORTED |
| `CRITIC-LOOP` | 2607.23002 | Tester writes, mutation names survivors, Critic kills them — every verdict mechanical, no model judges another's output | SECOND-STREAM | BENCHMARK | REPORTED |
| `COVERAGE-LIES` | 2607.22880 | Coverage and mutation score carry signal in a regression setting and stop being reliable when the code under test may already be buggy | — | BENCHMARK | REPORTED |
| `ORACLE-FIELDS` | 2510.03071 | Proportion of an object's fields an oracle can reach, computed statically; 249,027 assertions | SECOND-STREAM | BENCHMARK | REPORTED |
| `CRITERIA-WEAK` | 2609.09315 | Mutation only marginally beats coverage at finding LLM-code bugs, because the oracles fail | — | BENCHMARK | REPORTED |

---

## 4. Cross-reference: primitive → findings

Look up a field, get everything the literature says you can do with it.

| Primitive | Findings |
|---|---|
| inter-request gap | `CACHE-KEEPALIVE`, `TRACE-COPILOT` (turn-boundary idle) |
| `cache_creation` / `cache_read` | `COST-NOT-TOKENS`, `COMPACT-INVOICE`, `TRACE-COPILOT`, `CACHE-DEADPOINT` |
| compaction boundary | `COMPACT-INVOICE`, `TRACE-COPILOT` |
| tool-array hash | `TOOLS-VS-CACHE`, `SELECTIVE-CACHE`, `CACHE-DEADPOINT` |
| `reasoning_output_tokens` | `THINK-TAX`, `THINK-CONTRACT`, `THINK-LATE` |
| repeated tool-call arg hash | `WASTE-TRAJECTORY`, `LOOPS-REAL` |
| first Edit/Write index | `SEARCH-TAX` |
| provider request id | (none — this is a correctness prerequisite, not a finding) |
| `n_past` | (none — no paper in this sweep addresses llama.cpp cache accounting) |

**That last row is an asset.** The `n_past` log-only exposure and the (n−1)/n
ceiling appear in no paper found by any of five sweeps. It is original to this
project — and the local-inference user is also the one least likely to pay.

## 5. Cross-reference: our artefact → findings

| Artefact | Findings that bear on it |
|---|---|
| ADR-0002 truth tiers | `COVERAGE-LIES` (a proxy's validity is setting-dependent) |
| ADR-0011 cache_control opt-in | `CACHE-DEADPOINT`, `SELECTIVE-CACHE` |
| ADR-0014 a check that cannot fail | `MUTATE-META`, `ORACLE-SMOKE`, `CRITERIA-WEAK` |
| ADR-0018 provenance is a field | `THINK-CONTRACT` (you cannot infer the billing contract from a model id) |
| ADR-0019 capability probe | `THINK-CONTRACT`, `SERVER-OWNS` |
| FD-5 Grok's wire | `SERVER-OWNS`, `RATIONED` |
| FD-11 empty state | `TRACE-LAB` (Codex and Claude Code sessions, the surfaces it now names) |
| `internal/guardcheck` | `MUTATE-META` (prior art for UNREACHED/INERT), `MUTATE-GOOGLE`, `ORACLE-SMOKE` |
| `internal/cachemodel` | `COST-NOT-TOKENS`, `CACHE-DEADPOINT` |
| task #11 keep-alive experiment | `CACHE-KEEPALIVE` — **largely answers it**; see §7 |
| task #19 request id | `TELEMETRY-GAP` below |

`TELEMETRY-GAP` = 2608.07899, TelemetrySuffBench: OTel-shaped agent telemetry
retains 99.5–100% F1 at DETECTING a failure while capping origin-step accuracy
at **0.5%**, and concludes explicit decision-to-provenance links are required.
That is the published case for task #19.

---

## 6. What the index says when read as a whole

Six statements the catalogue supports that no single paper does.

1. **Denominate in anything but dollars.** Every idea surviving all five
   sweeps is `BASIS-FREE`. The subscription seat is the largest population and
   structurally unsellable in dollars; `RATIONED` argues it is also the class
   rationed first, so it has a real problem and no dollar figure for it.

2. **A cache miss is three problems averaged into one scalar.** TTL expiry,
   client mutation, and server pruning have different owners.
   `TRACE-COPILOT`'s 90%-within-turn / 55%-across-turn split is that argument
   measured at production scale.

3. **Compaction is sold as free and is not.** `COMPACT-INVOICE` measured
   completion unchanged while retrieval tripled; `TRACE-COPILOT` records
   compaction as a cache-kill event at fleet scale. Both are transcript-visible.

4. **The literature's correct metric is one we cannot compute.** `AGENTS-MATTER`
   and every waste paper converge on cost per successful task. Transcripts carry
   no outcome label, and inferring one from the agent's own closing message
   would be an Estimated figure wearing a Measured badge. `SCAFFOLD-COST`
   offers the nearest honest proxy — failure spend — and warns in the same
   breath that agents' self-reports do not match their behaviour.

5. **Do not ship routing advice.** `HANDOFF-TAX` measured mid-session
   escalation as a bad buy, and `SCAFFOLD-COST`'s MCP:CLI ratios span 0.43x to
   29x. The honest reading of the best evidence is that we do not know.

6. **The UNREACHED/INERT split is not novel.** `MUTATE-META` shipped mutation
   plus visit-instrumentation at Meta in 2020. That is a stronger commercial
   position than novelty, not a weaker one — it is validated and unshipped —
   but it forecloses a patent claim and must not be described as new.

---

## 7. Open items this index creates

| Item | Why |
|---|---|
| Read `CACHE-KEEPALIVE` in full before task #11 | It may already answer the $0.10 experiment, and it says the folk 30-second interval is ~8x wasteful. Cheaper to read than to run. |
| Read `COST-NOT-TOKENS`, `CACHE-DEADPOINT`, `TOKENIZE-TTFT` in full | The three papers under the strongest idea. Nothing from them ships until someone has read past the abstract. |
| Run our parser over `TRACE-LAB` | The 7.94x rests on one private corpus from one machine. A public corpus makes it independently checkable — or refutes it, which is worth knowing before a launch. |
| Re-verify `REPORTED` tier ids | Two thirds of this catalogue has not been re-fetched. |
| Record `n_past` as original | No paper in five sweeps covers it. |
