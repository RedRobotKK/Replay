# Durable work state: the research program

**Opened 2026-09-29. This file is a plan, not a finding. Every row below is a
question with a gate, and no row may be promoted without the evidence its gate
names.**

The program exists because one measurement is interesting and one measurement is
not a law. The origin result, E007/E008, was n=10 per arm, one operator, one
model, one repository the operator authored. The standing rule in this
repository is that six claims died in a single day from exactly that shape.

## What is being tested, in three separable claims

These must never be merged. Most of the confusion in this area comes from
sliding between them.

| | Claim | Status |
|---|---|---|
| **1** | A structured representation changes what a fresh agent RETRIEVES | Supported at n=10, one model, one repository. Replication running |
| **2** | An anchor's CAPABILITY, independent of its correctness, changes retrieval | Supported at n=10. Replication running |
| **3** | Any of this causes BETTER WORK | **Not established and not currently measurable.** No task-outcome signal exists in any corpus examined |

Claim 3 is the one a reader will assume. It is the one with no evidence.

## The questions

Status vocabulary: OPEN, RUNNING, SUPPORTED, REFUTED, NOT_MEASURED, BLOCKED.

### R1 retrieval/search behaviour
- **Hypothesis** a structured claims table changes which locations a fresh agent searches.
- **IV** representation form (table vs prose vs absent). **Control** no cursor. **Treatment** three-row table.
- **Observable** distinct paths and queries; count of claim-directed tool calls.
- **Ground truth** the artifact is either in a tool result or it is not. Mechanical.
- **Confounders** effort; anchor salience; prompt leakage.
- **n** 20/arm. **Stop** two failed replications.
- **Status** SUPPORTED at origin (0/8 no cursor, 0/4 prose, 9-10/10 table; 51/60 vs 0/12, p=1.9e-08). Needs a second model and repository.

### R2 claim closure behaviour
- **Hypothesis** agents declare an inherited claim settled without reaching evidence that could settle it.
- **IV** anchor capability. **Observable** `m9_declared_settled` with `e1` false.
- **Ground truth** mechanical: the artifact did not enter any tool result.
- **Confounder** a claim may be declared settled for a good reason the harness cannot see. Report false closure separately from closure.
- **Status** RUNNING.

### R3 executable-but-insufficient anchor effect
- **Hypothesis** an executable, correctly targeted, non-dispositive anchor reduces detection BELOW no anchor.
- **IV** anchor capability only. **Control** C0. **Treatment** T2.
- **Observable** `e1_artifact_in_tool_result`. **n** 20/arm, Fisher exact.
- **Status** **RUNNING**, 80 trials, preregistered, `PREREG.md` hash `31570895b75b`.
- **Advance requires** T2 < C0 on a second model AND a second repository.

### R4 explicit insufficiency labelling
- **Hypothesis** stating "this check cannot establish the claim" recovers detection.
- **IV** the capability statement, anchor text held constant. **Treatment** T4.
- **Why it matters** T4 separates INTERPRETATION from CAPABILITY. Recovery implies a cheap remedy; no recovery implies the harm is in the check itself.
- **Status** OPEN, designed, gated on R3. Deliberately absent from the running experiment.

### R5 model replication
- **Hypothesis** the effect is not a property of one model.
- **Status** RUNNING supplies model 2 (`claude-haiku-4-5`) against origin's `claude-sonnet-5`. A third, materially stronger model is required before generality.

### R6 repository replication
- **Status** RUNNING supplies repository 2 (`deepseek-cursor-proxy`, not operator-authored). A third is required.

### R7 claim-type replication
- **Hypothesis** the effect is not specific to a constant's VALUE.
- **Treatments** behavioural claim (does X call Y), absence claim (nothing does Z).
- **Status** OPEN. Absence claims are the interesting case: they may be unfalsifiable by any single check.

### R8 tool-surface replication
- **Hypothesis** the effect survives a different anchor implementation (file citation, test name) and evidence split across files.
- **Status** OPEN.

### R9 adversarial evidence
- **Hypothesis** the effect has a shape across ten anchor classes, not a single point.
- **Status** OPEN, matrix drafted in `EXPERIMENT_MATRIX.md`.

### R10 objective task outcome
- **Hypothesis** the representation changes whether work is correctly completed.
- **Ground truth** deterministic repository assertions ONLY. No LLM judge where an assertion is possible.
- **Status** **BLOCKED.** This is the missing primitive named in `../RESEARCH-CLOSEOUT-2026-09-29.md`. Until a benchmark with hidden ground truth exists, claim 3 above stays unevidenced.

### R11 independent reconstruction
- **Hypothesis** state written by an independent observer differs in effect from state written by the agent, holding information constant.
- **Arms** none / agent-generated / Replay-reconstructed / externally authored.
- **Why it matters** this question isolates whether INDEPENDENCE itself matters, as opposed to having the same information available. If it does not matter, independence leaves the thesis.
- **Status** OPEN. Hardest to control: holding semantic content constant across authorship is the whole difficulty.

### R12 evidence required to settle a claim
- **Hypothesis** `Settle()` refusing non-dispositive checks changes agent behaviour, not just internal bookkeeping.
- **Status** OPEN. `internal/stateledger` holds the prototype, unwired.

### R13 contradiction handling
- **Hypothesis** recording a contradiction WITHOUT inventing a replacement keeps a question open that would otherwise close.
- **Status** OPEN. Prototype invariant exists and is tested; behavioural effect unmeasured.

### R14 evidence frontier / temporal availability
- **Hypothesis** when evidence became available changes whether a claim should be believed.
- **Status** OPEN. Closest prior art is Zep/Graphiti's bi-temporal edge invalidation, which is reactive on ingest.

### R15 reboot work-state reconstruction
- **Hypothesis** a fresh agent can reconstruct where work stands from artifacts alone.
- **Status** OPEN. One anecdote: the `RECONCILE FIRST` block caught its own author within an hour. n=1 is not evidence.

### R16 heterogeneous provider operation
- **Status** partially SUPPORTED by construction. Replay reads Claude Code, Codex, Grok and Ollama without provider cooperation. See `PROVIDER_EVIDENCE_MATRIX.md`.

### R17 operation alongside existing agent memory
- **Hypothesis** the mechanism composes with CLAUDE.md, Claude memory, Zep, Mem0 rather than competing.
- **Status** OPEN, and commercially decisive: if it does not compose, it is a competitor in the most crowded category in the market.

### R18 distinction from ordinary memory/RAG/provenance/KG/eval
- **Hypothesis** the mechanism is not reducible to any of those.
- **Status** OPEN. `PRIOR_ART_MATRIX.md` holds the current evidence. **If the mechanism reduces to ordinary memory, the program stops.**

## Stop conditions

Any one of these ends the program rather than adjusting it: the replication gate fails twice; a result has no causal interpretation; the mechanism reduces to ordinary memory or RAG; deterministic ground truth is replaced by model judgement where an assertion was possible; provider evidence is assumed rather than verified.

Failed hypotheses are recorded as failed. They are not rescued.
