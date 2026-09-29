# Product fit, by evidence rather than by preference

**No scores, no ranking, no ordering by attractiveness. Each position is
evaluated against what is currently demonstrated. Written 2026-09-29, after the
first replication attempt returned NOT REPLICATED.**

## A. Memory product

**Problem** an agent forgets across sessions. **Substitutes** the most crowded
category examined: file conventions converging on `AGENTS.md` across Claude
Code, Codex, Cursor, Windsurf and Devin; vector recall plus LLM extraction in
Letta, Mem0, Supermemory, Vertex Memory Bank; structured checkpoints in
LangGraph, AutoGen, ADK. **Requirement** a store and a retrieval policy.
**Replay advantage** none demonstrated. **Evidence for** none. **Evidence
missing** all of it. **Friction** competing with vendor defaults that ship in
the product Replay reads. **Defensibility** none identified. **Monetization**
unclear against free defaults. **Kill condition: already met.** Nothing in the
evidence supports Replay entering this position.

## B. Work-state reconstruction

**Problem** after a reboot, compaction or handoff, what is settled and what is
open. **Substitutes** `/compact` summaries, transcript resume, AutoGen
MagenticOne's `plan` and `facts`, Semantic Kernel Whiteboard's decisions and
actions. Both of the latter are uncited free-text prose. **Requirement** a
representation a fresh agent can act on. **Replay advantage** the claims-table
retrieval result, which is real but is about retrieval only. **Evidence for**
origin campaign, 0/8 with no cursor against 9-10/10 with a table; prose 0/4.
**Evidence missing** that reconstructed state improves any outcome; a
reboot-recovery benchmark (none found in examined sources). **Friction** the
category is being absorbed into file conventions. **Defensibility** weak: the
representation is copyable in an afternoon. **Kill condition** if a vendor
default reaches parity on the retrieval effect, this position closes.

## C. Verification layer

**Problem** a claim is asserted; is it actually supported. **Substitutes**
**ECLoop** (2026-07-30) implements evidence-conditioned gating for coding agents
and reports Pass@1 +4.8 to +11.8 on all 500 SWE-bench Verified instances.
**DEMM-Bench** benchmarks whether runtime records are sufficient to reconstruct
decision properties. **Requirement** a way to type what a check can settle.
**Replay advantage** the `LOCATES` / `EXECUTABLE` / `DISPOSITIVE` typing exists
as a committed prototype in `internal/stateledger`. **Evidence for** none
behavioural. The one experiment designed to test it hit a ceiling and returned
NOT REPLICATED. **Evidence missing** everything. **Friction** ECLoop occupies
this position with published results. **Kill condition** if A' also fails at
calibrated difficulty, this position has no mechanism under it.

## D. Independent agent auditor

**Problem** the agent's own account of what happened is the only account.
**Substitutes** none found with true independence. Across 28 products examined,
Semantic Kernel's AIContextProvider, Claude Code rewind, OpenAI Sessions and
Devin Session Insights all capture without the agent's cooperation but remain
first-party instrumentation of the same runtime. **Requirement** read artifacts
from outside the runtime. **Replay advantage** demonstrated by construction: it
reads Claude Code, Codex, Grok and Ollama with no provider cooperation and no
SDK. **Evidence for** the reconstruction and reconciliation primitive, 575
responses, derived cost inside an observed balance interval, four conservation
identities, zero violations. **Evidence missing** that independence *matters* to
anyone: Gate H is unrun, and if independence turns out not to change behaviour
it leaves the thesis. **Friction** nobody is asking for it. **Kill condition**
Gate H showing no difference between agent-generated and independently generated
state.

## E. Evidence-conditioned response support

**Problem** an agent acts on insufficient evidence. **Substitutes** ECLoop,
directly. Oversight vendors gate on policy and pattern, not sufficiency.
**Requirement** supply sufficiency information without becoming the
authorization engine. **Replay advantage** none demonstrated. **Evidence for**
none. **Evidence missing** the behavioural mechanism, which Gate A failed to
test. **Friction** the boundary between informing and authorizing is where this
becomes a security product, which is a consolidating market: three of seven
oversight vendors examined were acquired within about a year. **Kill condition**
if the mechanism does not survive A' and B.

## F. Billing and cost reconciliation

**Problem** what an AI workload cost, checked rather than estimated.
**Substitutes** 21 products examined compute cost forward as tokens times a
price table. **No matching capability was found in the products examined** for
reconciling against a provider's actual bill, nor for attributing a cache break
to its cause. OTel GenAI has no cost attribute at all. **Requirement** a dated
rate table, conservation identities, an independent balance observation.
**Replay advantage** demonstrated and the only position with a completed
reconciliation. **Evidence for** a completed reconciliation, the only position in this document carrying one.
**Evidence missing** demand. **Friction** none technical. **Defensibility**
the refusals as much as the capability: Grok dollars unprinted, the
provider-account claim deleted rather than hedged. **Monetization** unproven;
the Replay Doctor launch produced approximately nothing. **Kill condition**
already partly met on the demand side, not on the technical side.

## Where the evidence stands, by position

Position **F** is the only one with a completed demonstration. Position **D** is
the only one where the market survey found no substitute, and its central
question is unrun. Positions **A**, **C** and **E** are occupied by others with
published results. Position **B** has a real retrieval finding and a copyable
mechanism.

That is the state of the evidence. It is a comparison, not a selection, and no position here is ranked above another.
