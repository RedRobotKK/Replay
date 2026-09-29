# Prior art matrix

**Compiled 2026-09-29. Language rule, applied throughout: "No matching
capability was found in the products examined." Never "nobody does this."**

Coverage limit, stated first. The surveys behind this file exhausted their
search budget partway through, so some entries rest on direct fetches of known
URLs rather than on discovery search. **Absence of a 2026 entrant is
NOT_VERIFIED, not a negative finding.**

## Per source: what it demonstrates, and what it does not

Structured so that neither the overlap nor its limits can be quietly dropped.

### ECLoop, the one that matters most

[2607.28815](https://arxiv.org/abs/2607.28815), 2026-07-30.

**Demonstrates:** an evidence-conditioned execution layer interposed between a
coding agent and a repository. It compiles conditions the agent must observe
before each class of edit and postpones any action whose conditions are unmet.
Evaluated on all 500 SWE-bench Verified instances, two models, two scaffolds:
Pass@1 +4.8 to +11.8 points, tokens down to 12.1%.

**Exact overlap:** evidence-conditioned agent behaviour, and evidence
sufficiency as a gate on action. **This is documented prior art, and it
materially weakens any broad Replay claim around "evidence-conditioned agent
behaviour."** That should be stated plainly rather than minimised: ECLoop is
implemented, measured against deterministic ground truth, and published.

**What it does NOT demonstrate:** it assumes insufficient evidence is bad and
gates on it. It does not measure whether *supplying* a weak, executable,
correctly targeted but non-dispositive check changes an agent's search
behaviour relative to supplying none. There is no no-anchor control in its
design, because that is not the question it asks.

**What ECLoop does not decide:** patentability. It establishes relevant prior
art and bears on freedom to operate. Those are separate questions for counsel,
and this document draws no legal conclusion.

### The rest

| Source | Demonstrates | Exact overlap | Partial overlap | Does NOT demonstrate |
|---|---|---|---|---|
| **Hindsight** [2512.12818](https://arxiv.org/abs/2512.12818) | Evidence-backed observations with "exact quotes and a proof count"; four memory types separating fact from inference; its stated thesis is that existing systems "blur the line between evidence and inference" | evidence-linked facts; fact/inference separation | | typing a check by what it can settle; any behavioural claim about search policy |
| **Zep / Graphiti** [2501.13956](https://arxiv.org/abs/2501.13956) | Bi-temporal knowledge graph; facts trace to refetchable source episodes; contradicted edges get `t_invalid` on ingest | evidence provenance; contradiction handling | | proactive checking against a live external source; unresolved questions |
| **AutoGen MagenticOne** | `MagenticOneOrchestratorState` persists `task`, `facts`, `plan`, `n_rounds`, `n_stalls`. `facts` and `plan` are LLM distillations, not verbatim messages | a persisted next-action artifact | | evidence attached to the plan; fact/inference tagging; staleness |
| **AutoGen task-centric memory** | `Memo` objects with `insight` and `task`; `retrieve_relevant_memos()` validates relevance via LLM | semantic retrieval | | verified at source: **absent explicitly** for citation, unresolved questions, fact/inference, contradiction detection |
| **Semantic Kernel Whiteboard** | Extracts requirements, proposals, **decisions** and **actions**; framework intercepts every message | decision and next-action extraction | "requirements"/"proposals" incidentally keep open items in play | evidence field; capability typing; it is experimental and C#-only |
| **STALE** [2605.06527](https://arxiv.org/abs/2605.06527) | 400 conflict scenarios, 1,200 queries, contexts to 150K. Probes state resolution, premise resistance, implicit policy adaptation. Best model 55.2% | stale-state detection, benchmarked | | whether an agent's *search* changes; scoring mechanism NOT_VERIFIED, abstract only |
| **STATE-Bench** [repo](https://github.com/microsoft/STATE-Bench) | 450 enterprise workflow tasks, three domains, sandboxed DB per task, protocol-locked LLM judge plus an explicitly LLM-judged UX score | agent task completion with state mutation | | **not reboot continuity.** Whether the judge performs a literal DB diff is NOT_VERIFIED |
| **Premature commitment** [2606.22936](https://arxiv.org/abs/2606.22936) | Agents settle early then defend; hidden-state monitor at AUROC 0.97; commitment does NOT track correctness | premature closure in agents, named and diagnosed | | that a supplied weak check causes it |
| **"Building to the Test"** [2606.28430](https://arxiv.org/abs/2606.28430) | Two production agents, hidden 222-test oracle, 18 runs. Near-perfect scores while the shipped library is "dead or absent". Names "validation self-awareness" | a passing check leaving the claim unsettled | | no no-oracle control, so no statement about a weak check versus none |
| **"Silence Is Endorsement"** [2609.20211](https://arxiv.org/abs/2609.20211), date REQUIRES VERIFICATION | Stripping "unverified" provenance raises risky approval 5% to 60% and 9% to 98%; explicit instructions do not reliably fix it | verification status changing downstream behaviour | | it REMOVES a status rather than supplying a weak check |
| **DEMM-Bench** [2606.20634](https://arxiv.org/abs/2606.20634) | Benchmarks whether runtime records suffice to reconstruct decision properties; trace-present baselines overclaim sufficiency on 75% of cases | evidence sufficiency of a record | | agent search behaviour |

### Citation caveat, recorded rather than resolved

The source report gave "Silence Is Endorsement" as 2026-07-25 while its arXiv
identifier `2609.20211` implies 2026-09. The two do not agree. The date is
marked **REQUIRES VERIFICATION** wherever it appears rather than being silently
corrected to fit the identifier, because no primary-source re-fetch was
performed in this audit pass. The paper's substance is unaffected; only its date
is in question.

## The residual question, and its current status

> Whether an executable-but-nondispositive verification path can itself produce
> a measurable reduction in contradiction detection relative to no verification
> path.

**No matching result was found in the sources examined.** Every near neighbour
above either assumes insufficiency is bad and gates on it, removes a status
rather than supplying a weak one, or lacks a no-anchor control.

**Current status of the residual question: NOT REPLICATED under the completed
task.** Both arms reached 100% detection, so the completed experiment does not
discriminate whether the mechanism is absent or whether the task lacked
difficulty.

## Capability reconciliation

Three absences are distinguished throughout and must not be collapsed:

- **Absent from the searched sample** the search did not find it. Says nothing about existence.
- **Documented absence** a primary source states or a schema shows the capability is not there.
- **Actual novelty** a legal or scientific conclusion this document does not draw.

| Capability | Existing prior art | Exact overlap | Partial overlap | Not found in examined sources | Replay hypothesis |
|---|---|---|---|---|---|
| Durable memory | Letta, Mem0, Zep, Supermemory, Vertex Memory Bank, LangGraph, AGENTS.md convention | **yes** | | | none. Replay should not enter |
| Session continuity | OpenAI Sessions, Claude Code resume, Vertex Agent Engine | **yes** | | | none |
| Work continuity | AutoGen MagenticOne `plan`+`facts`+`n_stalls`, SK Whiteboard decisions/actions | | **yes**, both uncited free-text prose | a work-state form measured to change retrieval | claims table, retrieval effect measured |
| Evidence-linked facts | Zep/Graphiti (source episodes refetchable), Hindsight ("exact quotes and a proof count") | **yes** | | | none |
| Source/proof citation | Hindsight; Vertex Memory Bank **documented absence**, schema enumerated, source session discarded | **yes** | | | none |
| Contradiction handling | Zep/Graphiti bi-temporal `t_invalid`; STALE benchmarks detection | | **yes**, all reactive on ingest | proactive check against a live external source | contradiction without inventing a replacement |
| Unresolved questions | | | SK Whiteboard "requirements"/"proposals", incidental | **first-class open-question state: not found in the 28 products examined** | first-class, with no resolution field |
| Next-action state | AutoGen `plan`, SK "actions" | | **yes**, uncited prose | a next action tied to a re-checkable anchor | required-next-check |
| Executable verification | ECLoop, every coding agent's tool loop | **yes** | | | none |
| Dispositive verification | ECLoop gates on evidence conditions | | **yes**, gates on conditions rather than typing the check | **typing a check by what it can settle: not found in examined sources** | LOCATES/EXECUTABLE/DISPOSITIVE, `Settle()` refuses non-dispositive |
| Evidence sufficiency gating | **ECLoop**, implemented, Pass@1 +4.8 to +11.8 on SWE-bench Verified; DEMM-Bench benchmarks record sufficiency | **yes** | | | none. Position occupied |
| Stale-state detection | **STALE**, 400 scenarios, best model 55.2% | **yes** | | | none |
| Premature-closure detection | 2606.22936 (AUROC 0.97 hidden-state detector); 2607.10275 (clinical, names premature closure) | **yes** | | | none |
| **Weak-anchor harm** | ECLoop assumes it, does not measure it; "Building to the Test" has no no-anchor control; "Silence Is Endorsement" removes a status rather than supplying a weak check | | | **no matching result found in the sources examined** | **UNTESTED. The one experiment built for it hit a ceiling** |
| Independent observation | first-party runtime instrumentation only (SK AIContextProvider, Claude Code rewind, OpenAI Sessions, Devin Session Insights) | | **yes**, captures without agent cooperation but is vendor-internal | **an outside observer reading artifacts across vendors: not found in the 28 products examined** | Replay reads four surfaces with no provider cooperation |
| Reconstruction | LangGraph deterministic replay; DEMM-Bench reconstructs decision properties | | **yes** | reconciliation against an independent record with a stated interval | demonstrated for resources |
| Claim/evidence provenance | Zep episodes, Hindsight proof counts, AutoGen `Memory.metadata` by convention only | **yes** | | | none |
| Objective task-outcome measurement | SWE-bench Verified, STATE-Bench (LLM-judged), ECLoop | **yes**, in benchmarks | | **on any live provider surface examined: NOT AVAILABLE, documented** | none. This is the blocker, not a Replay capability |

## The columns that matter

Of the eighteen fields the program asks for, five separate the field. The rest
are recorded per product in the source surveys.

| Product | Evidence attached, re-checkable | Fact vs inference | Unresolved questions | Contradiction detection | Independent observer |
|---|---|---|---|---|---|
| **Zep / Graphiti** | **YES.** Every fact traces to a fetchable source episode | no | no | **YES.** Bi-temporal: an LLM compares new edges against existing, sets `t_invalid`. Reactive on ingest | no |
| **Hindsight** (vectorize-io) | **YES.** "exact quotes and a proof count" | **YES.** fact / experience / observation / mental-model | no | partial, refine-not-overwrite | no |
| Letta / MemGPT | partial: re-queryable, no citation field | no | no | no | no |
| Mem0 | no. Docs state no provenance or source tracking | no | no | internal doc inconsistency; not external drift | no |
| LangGraph checkpointer | self-evidencing by deterministic replay, not a fact store | no | no | no | **no**, but fully automatic |
| LangMem | no | no | no | no | no |
| Cognee | partial, re-queryable | no | no | no | no |
| Supermemory | partial | no | no | reactive consolidation + TTL | no |
| **Vertex Memory Bank** | **NO, verified at schema level.** Every field enumerated; no source or citation; **source session discarded after extraction** | no | no | write-time only | no |
| AutoGen MagenticOne | no | no | no | no | no |
| Semantic Kernel Whiteboard | no | no | partial: "requirements"/"proposals" | no | **no**, but intercepts every message |
| Devin Knowledge | no. Docs: "flat assertions without evidence checking or re-validation capabilities" | no | no | no | no |
| CLAUDE.md / AGENTS.md | no | no | no | no | no |
| Claude Code auto-memory | no | no | no | `modified` timestamp only | no |

### Decisions and next action, recorded separately

Two products persist a next-action artifact, both uncited free-text prose with
no fact-versus-inference tag: **AutoGen MagenticOne's `plan`** (alongside
`facts`, an LLM distillation, and `n_stalls`, the only progress-or-stuck signal
found anywhere in the survey), and **Semantic Kernel Whiteboard's "actions"**,
which is also the only product naming "decisions" as an explicit extraction
target. LangMem captures "situation, thought process, why it worked" as episodic
memory, the closest thing to decision rationale.

## Evidence-sufficiency gating: oversight products

Six vendors examined: NVIDIA NeMo Guardrails, Invariant Labs (acquired by Snyk),
Lakera, Protect AI (now Palo Alto Prisma AIRS), HiddenLayer, CalypsoAI (now F5).
Aim Security returned HTTP 403 on every attempt and is **NOT_VERIFIED, not ruled
out**.

**No matching capability was found in the products examined.** Every mechanism
is classifier, pattern or rule based. Invariant Labs is closest: its DSL can
require that a verification-type call occurred before a write. That checks the
**presence of a prior call**, never whether that call's **result** justified the
next action, which is the distinction this program is about.

## Cost reconciliation: observability and FinOps

21 products across Langfuse, LangSmith, Braintrust, Phoenix, Weave, Helicone,
Datadog, Vantage, Finout, CloudZero and others. **No matching capability was
found in the products examined** for reconciling a reconstructed cost against a
provider's actual bill, nor for attributing a cache break to its cause.

Every product computes cost forward as tokens times a price table. None reports
an uncertainty interval. The OTel GenAI semantic conventions have **no cost
attribute at all**, so there is no standard wire field for "the provider said
this cost X" for anyone to consume. Finout and CloudZero both market their
usage-API approach as better than waiting for an invoice, which by their own
framing confirms it is not reconciliation.

## Named artifacts, now researched

All URLs fetched directly 2026-09-29 against arXiv and GitHub primary sources.

### The four papers that bear directly on the mechanism under test

**These were found after the research program was written and they change its
outlook. Recorded here before any of them was allowed to influence a reading of
the running experiment.**

| Artifact | What it establishes | Relation to this program |
|---|---|---|
| **ECLoop**, "Preventing Premature Commitment in Coding Agents with an Evidence-Conditioned Execution Layer", 2026-07-30, [2607.28815](https://arxiv.org/abs/2607.28815) | **Implements an evidence-sufficiency gate.** Interposes between agent and repository, compiles conditions the agent must observe before each edit class, postpones actions whose conditions are unmet. All 500 SWE-bench Verified instances, two models, two scaffolds: **Pass@1 +4.8 to +11.8 points**, tokens down to 12.1% | **This is R12, published.** Evidence-conditioned action gating for coding agents exists, is implemented, and is measured against deterministic ground truth |
| **"When Agents Commit Too Soon"**, 2026-06-22, [2606.22936](https://arxiv.org/abs/2606.22936) | Defines **premature commitment**: agents settle on a reading early then defend it. A hidden-state monitor reaches AUROC 0.97. Crucially, **commitment does not track correctness**: committed-wrong and committed-correct are not separable | R2 as a named, diagnosed phenomenon with a runtime detector |
| **"Building to the Test"**, 2026-06-26, [2606.28430](https://arxiv.org/abs/2606.28430) | Two production coding agents against a hidden 222-test oracle, 18 runs. With the oracle in the loop scores are near-perfect while the shipped library is "dead or absent". Names the disposition **"validation self-awareness"** | **A green check leaving the claim unsettled, published and measured.** The closest existing statement of `EXECUTABLE != DISPOSITIVE` |
| **"Silence Is Endorsement: Verification-Status Laundering"**, 2026-07-25, [2609.20211](https://arxiv.org/abs/2609.20211) | Stripping "unverified" provenance while holding the action claim fixed raises risky-action approval **5% to 60%** and **9% to 98%**. Explicit instructions to reject unverified authorization do not reliably fix it | Verification STATUS changing downstream behaviour, with effect sizes far larger than anything measured here |

Also: **DEMM-Bench**, 2026-05-30, [2606.20634](https://arxiv.org/abs/2606.20634), benchmarks whether agent-runtime records are SUFFICIENT to reconstruct decision-level properties. Trace-present and schema-present baselines **overclaim sufficiency on 75% of cases**. That is evidence-sufficiency as a benchmarked property of a record, which is close to Replay's reconstruction positioning.

### STALE, confirmed

"STALE: Can LLM Agents Know When Their Memories Are No Longer Valid?", 2026-05-07, [2605.06527](https://arxiv.org/abs/2605.06527). 400 conflict scenarios, 1,200 queries, contexts to 150K tokens. Probes state resolution, premise resistance and implicit policy adaptation. **Best model 55.2% overall.** This is R13 and R14 with a benchmark already attached. Scoring mechanism NOT_VERIFIED, abstract only.

### STATE-Bench, found but a fit mismatch

[microsoft/STATE-Bench](https://github.com/microsoft/STATE-Bench). 450 enterprise
workflow tasks over three domains, sandboxed DB per task. **Not agent continuity
across reboots**: it is enterprise task completion with DB mutation, scored by a
protocol-locked LLM judge, with an explicitly LLM-judged UX score. Whether the
judge performs a literal DB diff is NOT_VERIFIED. A differently-named
`kohlsy/StateBench` exists and was **not** fetched; flagged, not confirmed.

### Hindsight, confirmed verbatim

Evidence field confirmed: *"Each observation keeps its supporting evidence with
exact quotes and a proof count, and is refined rather than overwritten when new
evidence arrives."* Four memory types confirmed. Its paper
([2512.12818](https://arxiv.org/abs/2512.12818)) states the differentiator
directly: existing memory systems **"blur the line between evidence and
inference"**. That is one of the distinctions this program claimed; it is
Hindsight's stated thesis, published 2025-12.

### Explicit NOT_FOUND

No artifact titled "verification-induced search termination" exists on arXiv.
The four papers above are nearest matches, not confirmations of a named concept.
No agent handoff or reboot-recovery **benchmark** was found; the nearest artifact
is a systems paper with its own test suite.

## Current reading on R18, revised downward

**The earlier reading was too favourable and is withdrawn.** It was written
against a survey of PRODUCTS. Against the 2026 literature the position is much
weaker:

| Claimed distinction | Status after this research |
|---|---|
| Evidence-sufficiency gating on agent action | **PUBLISHED** (ECLoop), implemented, measured on SWE-bench Verified |
| Fact versus inference separation | **PUBLISHED** (Hindsight), and it is their stated thesis |
| Premature closure in agents | **PUBLISHED** twice, with a runtime detector |
| A green check leaving a claim unsettled | **PUBLISHED** ("Building to the Test") |
| Verification status changing behaviour | **PUBLISHED**, with larger effects than measured here |
| Evidence sufficiency of a record, benchmarked | **PUBLISHED** (DEMM-Bench) |
| Stale-memory detection | **PUBLISHED** with a benchmark (STALE) |

What is left is narrow and specific: **none of these papers tests whether an
executable, correctly targeted, non-dispositive anchor makes detection WORSE
than providing no anchor at all.** ECLoop assumes insufficient evidence is bad
and gates on it; it does not measure the harm of offering an insufficient check.
"Silence Is Endorsement" removes a status rather than supplying a weak check.
"Building to the Test" shows a passing oracle leaves the claim unsettled but
makes no comparison against a no-oracle control.

That residual is a direction, not a subsystem. It is one cell in a table the
field is already filling in.

**Consequence for the IP gate:** condition 8, "materially distinct from known
memory/RAG/provenance/verification systems", is now substantially harder to
satisfy and should be assumed unmet until argued against the rows above
specifically. **Consequence for the product thesis:** ECLoop is third-party
evidence that evidence-conditioned gating improves deterministic task outcomes,
which strengthens the AREA while weakening any claim to owning it.
