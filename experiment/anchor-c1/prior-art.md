# Prior art: machine-derived evidence sufficiency as a termination condition for agent verification

Investigation date: 2026-09-29. Every finding below was fetched or surfaced on 2026-09-29 unless
otherwise noted. Documentation and literature research only; no paid API calls were made.

## Scope

Mechanism under investigation:

> An agent-facing evidence representation that records not merely WHERE evidence lives, but whether
> that specific location is SUFFICIENT to terminate verification of a SPECIFIC claim, with
> sufficiency derived from a machine-checkable predicate rather than asserted.

This is a prior-art sweep, not a novelty or patentability or freedom-to-operate assessment. The
working bias throughout is "assume this is already covered", and the sweep was run until that bias
was confirmed rather than until a gap appeared.

## Headline answer, stated plainly

**Yes. Multiple independent systems already represent evidence sufficiency as a first-class,
machine-derived property explicitly distinguished from relevance.** This is not an inference from
adjacent work. At least three sources state the distinction in their own words:

- **SURE-RAG** opens on the sentence "relevance does not guarantee sufficiency" and treats
  sufficiency as a set-level computed property with an auditable score, emitting a three-way
  support / refute / insufficient decision. Fetched 2026-09-29.
- **ClaimReceipt** names sufficiency and coverage as "two distinct evidentiary questions" and turns
  both into machine-checkable contracts returning PASS / INVALID / INCONCLUSIVE per claim. Fetched
  2026-09-29.
- **groundcheck**, a shipping open-source MCP server, carries a per-verdict `sufficiency` tag with
  the states `sufficient`, `insufficient`, `no_sources`, `no_stance`, `conflict`, explicitly so that
  "found nothing", "sources exist but do not establish it", and "sources disagree" do not collapse
  into one negative. Fetched 2026-09-29.

The crux question is therefore closed. The distinction between relevance and sufficiency, machine
derived, attached per claim, is established prior art as of mid-2026.

## Category 2: DIRECTLY ANALOGOUS SYSTEM

These do substantially the thing described.

### 2.1 Evidence-Carrying Termination (ECT) -- closest single match

*When May an Agent Stop? Evidence-Carrying Termination for Tool-Using LLMs*, Jason Liu,
arXiv:2608.23623, submitted 2026-08-22. Fetched 2026-09-29.

An agent may return COMPLETE only when a typed certificate binds **every required answer claim** to
valid, in-scope trace evidence **and** a deterministic replay reconstructs the claimed value. The
certificate is the machine-checkable predicate; termination is gated on it. The paper explicitly
scopes the guarantee to "support in a recorded trace under declared assumptions, not external truth",
which is the same move as separating sufficiency from correctness-in-the-world. Reported result: zero
unsafe premature completions across 66 trajectories versus 40 for the baseline.

This is the tightest match to the full sentence of the candidate mechanism: per-claim binding,
machine-checkable predicate, sufficiency as the termination gate. If one artifact closes the
question, it is this one.

### 2.2 ClaimReceipt

*ClaimReceipt: Verifying Evidence Sufficiency and Coverage in Agent Evaluations*, Peiying Zhu and
Sidi Chang, arXiv:2609.01992, submitted 2026-09-02. Fetched 2026-09-29.

Binds typed transaction evidence to a signed experiment manifest and returns PASS / INVALID /
INCONCLUSIVE per claim. Sufficiency is defined operationally as "whether a reported claim is
recomputable from retained evidence"; coverage as "whether the retained records cover the committed
experiment set". Sufficiency is therefore derived, not asserted, and is named as distinct from other
evidentiary questions. The INCONCLUSIVE verdict is exactly the "stopped at an insufficient
destination" state that the experiment under investigation observed as the dominant failure mode.

### 2.3 SURE-RAG

*SURE-RAG: Sufficiency and Uncertainty-Aware Evidence Verification for Selective
Retrieval-Augmented Generation*, Jingxi Qiu, Zeyu Han, Cheng Huang, arXiv:2605.03534, submitted
2026-05-05, revised 2026-07-24. Fetched 2026-09-29.

Directly on the crux. A claim-evidence verifier scores each (claim, passage) pair into relation
distributions, aggregated into coverage, relation strength, uncertainty and retrieval-characteristic
feature blocks, producing support / refute / insufficient with an auditable score. States that a
passage can be topically relevant yet fail to justify the answer, and that sufficiency is a
set-level property individual passage scoring cannot capture. That last point matters for the
experiment under investigation: it means the destination-level sufficiency framing is already known
to be the harder and correct framing.

### 2.4 groundcheck (shipping open-source, MCP)

<https://github.com/beepboop2025/groundcheck>. Fetched 2026-09-29. Last-update timestamp
NOT_VERIFIED (repository footer showed 2026 and 68 commits; no explicit date visible).

An MCP server exposing live-source claim and citation verification to agents. Claims are decomposed
into atoms, each verified independently and recombined weakest-link. Each verdict carries a
`sufficiency` tag distinct from the verdict itself. Signed delivery receipts bind payment, delivery
and grounded content. This is the candidate mechanism as a deployed agent-facing tool surface, not a
paper. Its existence is the strongest single reason to treat the idea as covered rather than novel.

### 2.5 HALT

*HALT: Verification-Aware Stopping for Retrieval-Augmented Search Agents*, Daeyoung Roh and Donghee
Han, arXiv:2608.02009v1, 2026-08-03. Fetched 2026-09-29.

Stopping policy where a supervised verifier (Qwen2.5-3B with LoRA) classifies each claim-evidence
pair as match / partial / null, and the agent halts only when cumulative evidence supports every
expected hop claim. Per-claim, machine-derived, termination-gating. Reduces search loops 17 to 45
percent at equal exact match. Detects both over-search and under-search, which is the same
bidirectional control the experiment's "stopped at the wrong artifact" finding implies is needed.

### 2.6 EviGuard

*EviGuard: Machine-Verifiable Evidence Grounding for LLM-Based Industrial Incident Reasoning*,
Applied Sciences 16(18):8925, MDPI. Direct fetch returned HTTP 403 on 2026-09-29; details below are
from search-result excerpts and are NOT_VERIFIED against the full text.

Stores auditable cross-layer evidence in a provenance graph, restricts the LLM to proposing
hypotheses, compiles each hypothesis into atomic machine-checkable claims in an "Incident Claim
Language", and dispatches an ensemble of deterministic verifiers. This is the candidate mechanism in
an industrial setting, with the additional move of restricting the model to hypothesis generation so
the sufficiency predicate is always executed by a deterministic checker.

## Category 1: GENERIC PRIOR ART

These cover the idea broadly and predate or subsume the specific framing.

### 1.1 Toulmin argumentation (1958)

Claim, grounds, warrant, backing, qualifier, rebuttal. The **warrant** is precisely "the reasoning
that authorises the inferential leap from the grounds to the claim", and **backing** is the support
for the warrant. That is the sufficiency question, named and structured, sixty-eight years ago. AI &
Law has carried it into computational argument-scheme work with critical questions that function as
sufficiency tests. Any claim that "recording whether evidence suffices for a claim, as distinct from
where the evidence is" is a new representational idea has to answer Toulmin first.

- <https://link.springer.com/rwe/10.1007/978-94-007-6883-3_4-1>
- <https://aclanthology.org/W15-0507.pdf>

### 1.2 Fact verification with a NOT ENOUGH INFO label (FEVER lineage)

The three-way SUPPORTED / REFUTED / NOT ENOUGH INFO labelling scheme has been the standard output of
fact verification systems for years, and NEI is a sufficiency judgement by construction.
*Fact Checking with Insufficient Evidence* (TACL 2022) states that models must predict veracity
"only when there is sufficient evidence and otherwise indicate when it is not enough".
*Evidence Absence Is Not Evidence Insufficiency* (arXiv:2605.26663, 2026) goes further and separates
"no evidence was retrieved" from "evidence was retrieved and does not settle it", which is the exact
distinction the experiment's 4/25 "stopped there" versus 9/25 "answered from an unrelated aggregate"
split exposes.

- <https://aclanthology.org/2022.tacl-1.43/>
- <https://arxiv.org/abs/2204.02007>
- <https://arxiv.org/html/2605.26663>

### 1.3 Audit evidence standards: sufficiency and appropriateness

The accounting profession has formalised evidence sufficiency as a term of art. PCAOB AS 1105
governs "the sufficiency and appropriateness of audit evidence", strengthened for fiscal years
ending on or after 2024-12-15, and COSO's February 2026 generative-AI internal control guidance
requires an audit trail "sufficient to reconstruct what the AI acted on", with each AI touchpoint
tagged to the assertion it influences. Sufficiency-per-assertion is the professional baseline, not a
new representation. (Secondary sources; the standards texts themselves are NOT_VERIFIED here.)

- <https://www.kognitos.com/blog/ai-audit-trail-requirements-2026-checklist/>
- <https://www.cpajournal.com/2026/08/19/leveraging-artificial-intelligence-in-audit-planning/>

### 1.4 Attribution evaluation: AIS, ALCE, SourceCheckup

The attribution-evaluation literature has measured "does this specific cited source support this
specific statement" since 2023. ALCE's citation precision is computed by leave-one-out entailment:
drop a cited document and test whether the remaining set still entails the sentence. That is a
machine-computed sufficiency test on a specific location for a specific claim, already standard.
SourceCheckup (Nature Communications 2025) separates relevance from supportiveness explicitly and
found 50 to 90 percent of LLM medical responses not fully supported by their own citations, with 89
percent agreement against a three-expert consensus.

- <https://arxiv.org/html/2305.14627v2.pdf>
- <https://www.nature.com/articles/s41467-025-58551-6>
- <https://arxiv.org/abs/2402.02008>
- <https://github.com/kevinwu23/SourceCheckup>

## Category 3: MATERIALLY SIMILAR MECHANISM (close, with a named difference)

### 3.1 PaperTrail

*PaperTrail: A Claim-Evidence Interface for Grounding Provenance in LLM-based Scholarly Q&A*,
Martin-Boyle, Brown, Leckey, Kaur, CHI 2026, arXiv:2602.21045. Fetched 2026-09-29.

Decomposes both answers and source documents into discrete claims and evidence and maps them,
surfacing supported claims, unsupported claims and omissions. **Named difference:** the sufficiency
judgement is rendered to a human in an interface rather than consumed by an agent as a termination
predicate. Notable finding for the closeout: across 26 researchers, PaperTrail *lowered* trust
relative to baseline. Exposing sufficiency changes behaviour, and not always in the direction the
designer expects.

- <https://doi.org/10.1145/3772318.3791101>

### 3.2 CoVeR

*CoVeR: Coverage-Based Routing of Verifier Calls in Agentic Retrieval*, Daeyoung Roh and Donghee
Han, arXiv:2609.26086v1, 2026-09-03. Fetched 2026-09-29.

Explicitly separates coverage (cheap, geometric: one minus best cosine similarity to any evidence so
far) from sufficiency ("deciding evidence actually supports claims, requires semantic judgement only
an LLM can reliably provide"), and routes verifier calls on a coverage-margin threshold. Its
headline sentence is "matching is not supporting". **Named difference:** CoVeR concludes that a cheap
machine predicate can detect *insufficiency* but cannot certify *sufficiency*, which is a direct
constraint on any claim that sufficiency can be fully machine-derived from a cheap signal. Reported
AUROC 0.69 to 0.76 for embedding-based coverage prediction against 0.89 to 0.90 with oracle targets.

### 3.3 ProClaim

*ProClaim: Sufficiency-Aware Evidence Programming for In-the-Wild Scientific Claim Verification*,
saezlab. <https://github.com/saezlab/ProClaim>. Fetched 2026-09-29. Publication date NOT_VERIFIED.

Sufficiency-aware agentic verifier that builds evidence iteratively until a claim's consensual
stance is established. **Named difference:** from the repository documentation alone, sufficiency
reads as an emergent property of the verification loop rather than an explicitly specified
machine-checkable predicate attached to a location. The paper text was not retrieved; this
characterisation is NOT_VERIFIED.

### 3.4 SLSA Verification Summary Attestation and in-toto

<https://slsa.dev/spec/v1.0/attestation-model>, <https://slsa.dev/spec/v0.1/verification_summary>,
<https://slsa.dev/blog/2023/05/in-toto-and-slsa>. Fetched 2026-09-29.

in-toto statements carry a typed predicate about a subject artifact; policy engines consume the
predicate and return true/false. A Verification Summary Attestation is literally a machine-readable
record that some verifier checked a specific artifact against a specific policy and reached a
verdict, which downstream consumers treat as sufficient without re-deriving it. **Named difference:**
sufficiency here is policy-relative and is fixed by the layout author in advance; the framework does
not model "is this evidence enough to settle this claim" as a derived property, it models "did the
declared policy pass". The SLSA documentation is explicit that a provenance predicate mitigates only
a limited threat class and that other claims need other predicate types, which is an admission that
per-claim sufficiency is outside the model.

### 3.5 Trust tiers and evidence-quality metadata in retrieval

Multiple systems attach evidence-quality metadata distinct from relevance score. Example: a
`trust_tier` graded on demand from graph node confidence plus corroboration count, recorded in
retrieval metadata, with a `min_trust_tier` retrieval filter. The motivating observation is that
"retrieval results typically carry a relevance score but no evidence-quality signal". **Named
difference:** quality and reliability are properties of the source, not of the source-claim pair, so
these systems rank evidence rather than decide whether a specific location settles a specific claim.

- <https://github.com/semantica-agi/semantica/issues/1557>
- <https://github.com/semantica-agi/semantica/pull/1703>

### 3.6 Artifact-centered claim-aware observability

*Artifact-centered Claim-aware Observability for Autonomous Scientific Agents*, Yin, Du, Prince,
Cherukara, arXiv:2608.18312, 2026-08-18. Fetched 2026-09-29.

Treats scientific claims as first-class individuals with explicit evidence bindings and verification
records, as a semantic layer complementing telemetry and provenance standards. **Named difference:**
from the abstract, it binds artifacts to claims and records verification outcomes but does not
clearly derive a sufficiency verdict; whether sufficiency is distinguished from relevance there is
NOT_VERIFIED (full text not retrieved).

### 3.7 ReAgent

*Beyond the Text: Verifying That Agent-Written Papers Are Backed by Their Artifacts*, Shen, Wu, Liu,
Qi, Chen, arXiv:2609.22111, 2026-08-18. Fetched 2026-09-29.

Extracts structured claims from a paper and uses them to direct investigation of the backing
repository, combining static analysis with dynamic execution to catch hard-coded metrics and
unimplemented methods. **Named difference:** the sufficiency test is execution-based reproduction of
the claim rather than a declared predicate carried alongside the pointer. Relevant to the experiment
because it is built around exactly the failure the experiment measured: a plausible-looking artifact
that does not actually establish the claim.

### 3.8 ProvenanceTrace

*ProvenanceTrace: Atomic Claim Verification Across Financial and Open-Domain AI Systems*, Springer.
<https://link.springer.com/chapter/10.1007/978-3-032-27997-2_5>. Surfaced 2026-09-29, full text
NOT_VERIFIED.

Decomposes outputs into atomic claims and links each to evidence or flags it unsupported. **Named
difference:** binary supported/unsupported rather than a graded sufficiency predicate governing
termination.

## Category 4: GENUINELY DIFFERENT (related field, does not cover it)

### 4.1 W3C PROV (PROV-DM, PROV-O)

<https://www.w3.org/TR/prov-dm/>, <https://www.w3.org/TR/prov-o/>. Fetched 2026-09-29.

PROV models entities, activities and agents so that consumers can "form assessments about quality,
reliability or trustworthiness". It deliberately supplies the substrate for such an assessment and
does **not** express the assessment. PROV has no vocabulary for "this entity is sufficient to settle
that claim"; it has no first-class claim at all. It is also known not to express the plan an
execution was supposed to follow, which is the same class of gap. Extension points exist and would be
the natural place to add a sufficiency predicate, but the core standard does not carry one.

- <https://blogs.ncl.ac.uk/paolomissier/2021/02/07/w3c-prov-some-interesting-extensions-to-the-core-standard/>
- <https://pmc.ncbi.nlm.nih.gov/articles/PMC12376154/>

### 4.2 RAGAS metric suite

<https://docs.ragas.io/en/stable/concepts/metrics/available_metrics/>. Fetched 2026-09-29.

The published metric list is Context Precision, Context Recall, Context Entities Recall, Noise
Sensitivity, Response Relevancy, Faithfulness, plus Nvidia metrics (Answer Accuracy, Context
Relevance, Response Groundedness) and agent metrics. **There is no metric named context sufficiency.**
Context Recall is the nearest neighbour ("does the retrieved context contain all necessary
information") but is computed against a reference answer, not as a standalone per-claim sufficiency
predicate. TruLens Groundedness likewise measures overlap of response statements with provided
context. Both frameworks measure grounding of an answer in a context; neither asks whether a given
location suffices to terminate verification. This is the clearest case where the standard tooling
does *not* already cover the mechanism.

### 4.3 e-discovery and technology-assisted review

Predictive coding / TAR ranks documents by **relevance**, and "dispositive documents" appear in early
case assessment as a retrieval target, not as a computed property carried on the document. Tagging is
human-applied or model-predicted responsiveness, and defensibility is described in the field as a
documentation problem rather than a computational one. No machine-derived per-claim sufficiency
predicate surfaced in this area.

- <https://www.americanbar.org/groups/science_technology/resources/scitech-lawyer/2026-winter/ai-e-discovery/>
- <https://www.revealdata.com/blog/unlock-ai-ediscovery-closing-the-gap-between-review-and-legal-writing>
- <https://csdisco.com/blog/how-artificial-intelligence-transforms-ediscovery>

### 4.4 llms.txt, agent-readable docs and stable anchors

The addressability half of the experiment's finding is well covered and thoroughly commoditised:
llms.txt, llms-full.txt, per-page markdown endpoints, stable H2/H3 anchor ids held constant across
releases. None of it carries any statement about whether a destination settles a claim. This is prior
art for "give the agent a resolvable address" and no prior art at all for "say whether the address
suffices". The experimental result that addressability alone lifts retrieval from 6/25 to 25/25 while
leaving correct resolution at 12/25 on a wrong-but-plausible path is precisely the gap this body of
practice leaves open.

- <https://docs.across.to/ai-agents/llms-txt>
- <https://telawiki.com/tela/tela-blog/1055/agent-readable-documentation-llms-txt-mcp-and-why-docs-are>

### 4.5 Survey framing: the field has not unified it

*From Agent Traces to Trust: A Survey of Evidence Tracing and Execution Provenance in LLM Agents*,
Wang et al., arXiv:2606.04990v4, June 2026. Fetched 2026-09-29.

The survey distinguishes evidence **presence** from evidence **support** ("a source may be Cited but
not Support a claim; a relevant passage may be available but Omitted") but, on the version fetched,
stops short of formalising sufficiency as a separate evaluation dimension. Its named open problems
include unified trace schemas and claim-level semantic provenance across transformations. So the
components exist in many systems and are not yet standardised into one representation. That is a
standardisation gap, not a conceptual one.

## Assessment

On the crux: **evidence sufficiency as a first-class, machine-derived property distinct from
relevance already exists** in at least ECT, ClaimReceipt, SURE-RAG, HALT, CoVeR, EviGuard
(NOT_VERIFIED) and the shipping groundcheck MCP server, and its conceptual ancestry runs back through
FEVER's NEI label, audit evidence standards, and Toulmin's warrant. Nothing in the candidate
mechanism's sentence is unclaimed territory.

Two narrower observations that survive the sweep, offered as observations and not as novelty claims:

1. Most of the work above computes sufficiency over an **accumulated evidence set** (SURE-RAG says so
   explicitly, and says set-level is the correct level). Sufficiency carried on a **single named
   destination, before the agent goes there**, so that the pointer itself declares what it can and
   cannot settle, is less commonly the framing. ECT and ClaimReceipt attach the predicate to the
   claim and check it after the fact; llms.txt-style addressability attaches nothing. This is a
   difference of placement, not of concept, and CoVeR's result argues placement will not rescue it:
   cheap signals detect insufficiency but do not certify sufficiency.
2. The standard evaluation tooling a practitioner would actually reach for (RAGAS, TruLens) does not
   ship a sufficiency metric. The research literature has the concept; the default toolchain does
   not expose it.

Honest ranking of how much of the mechanism is already standard practice:

- Addressable evidence pointers for agents: fully standard, commodity.
- Claim decomposition and per-claim evidence binding: standard in research, common in products.
- Machine-computed support/refute/insufficient verdicts per claim: standard in research, present in
  shipping tools, absent from default eval toolchains.
- Sufficiency explicitly separated from relevance: established, stated in those words by multiple
  independent 2026 sources.
- Sufficiency as the gate on agent termination: established as of mid-2026 (ECT, HALT), recent but
  not novel.
- Sufficiency declared on a specific destination in advance of visiting it: the least-covered
  framing, and the one the surveyed literature gives reason to doubt is achievable cheaply.

Overall: treat the mechanism as covered. The defensible contribution from the experiment is the
**measurement** (addressability initiates investigation at 25/25 while destination sufficiency
governs correct termination at 12/25 versus 25/25, with 12 of 13 failures asserting confidently
rather than declining), not the representation.

## Sources

- [ClaimReceipt: Verifying Evidence Sufficiency and Coverage in Agent Evaluations (arXiv:2609.01992)](https://arxiv.org/abs/2609.01992)
- [When May an Agent Stop? Evidence-Carrying Termination for Tool-Using LLMs (arXiv:2608.23623)](https://arxiv.org/abs/2608.23623)
- [HALT: Verification-Aware Stopping for Retrieval-Augmented Search Agents (arXiv:2608.02009v1)](https://arxiv.org/html/2608.02009v1)
- [CoVeR: Coverage-Based Routing of Verifier Calls in Agentic Retrieval (arXiv:2609.26086)](https://arxiv.org/html/2609.26086)
- [SURE-RAG: Sufficiency and Uncertainty-Aware Evidence Verification (arXiv:2605.03534)](https://arxiv.org/abs/2605.03534)
- [ProClaim: Sufficiency-Aware Evidence Programming (GitHub, saezlab)](https://github.com/saezlab/ProClaim)
- [groundcheck: live-source claim and citation verification over MCP (GitHub)](https://github.com/beepboop2025/groundcheck)
- [EviGuard: Machine-Verifiable Evidence Grounding (Applied Sciences 16(18):8925) -- fetch returned HTTP 403](https://www.mdpi.com/2076-3417/16/18/8925)
- [From Agent Traces to Trust: A Survey of Evidence Tracing and Execution Provenance in LLM Agents (arXiv:2606.04990v4)](https://arxiv.org/html/2606.04990v4)
- [PaperTrail: A Claim-Evidence Interface (arXiv:2602.21045)](https://arxiv.org/html/2602.21045v1)
- [PaperTrail (CHI 2026 proceedings)](https://doi.org/10.1145/3772318.3791101)
- [Artifact-centered Claim-aware Observability for Autonomous Scientific Agents (arXiv:2608.18312)](https://arxiv.org/abs/2608.18312)
- [Beyond the Text: Verifying That Agent-Written Papers Are Backed by Their Artifacts (arXiv:2609.22111)](https://arxiv.org/abs/2609.22111)
- [ProvenanceTrace: Atomic Claim Verification (Springer)](https://link.springer.com/chapter/10.1007/978-3-032-27997-2_5)
- [Fact Checking with Insufficient Evidence (TACL 2022, ACL Anthology)](https://aclanthology.org/2022.tacl-1.43/)
- [Fact Checking with Insufficient Evidence (arXiv:2204.02007)](https://arxiv.org/abs/2204.02007)
- [Evidence Absence Is Not Evidence Insufficiency (arXiv:2605.26663)](https://arxiv.org/html/2605.26663)
- [Enabling Large Language Models to Generate Text with Citations, ALCE (arXiv:2305.14627)](https://arxiv.org/html/2305.14627v2.pdf)
- [An automated framework for assessing how well LLMs cite relevant medical references (Nature Communications)](https://www.nature.com/articles/s41467-025-58551-6)
- [How well do LLMs cite relevant medical references? (arXiv:2402.02008)](https://arxiv.org/abs/2402.02008)
- [SourceCheckup (GitHub)](https://github.com/kevinwu23/SourceCheckup)
- [Ragas: list of available metrics](https://docs.ragas.io/en/stable/concepts/metrics/available_metrics/)
- [PROV-DM: The PROV Data Model (W3C)](https://www.w3.org/TR/prov-dm/)
- [PROV-O: The PROV Ontology (W3C)](https://www.w3.org/TR/prov-o/)
- [W3C PROV: some interesting extensions to the core standard (Missier)](https://blogs.ncl.ac.uk/paolomissier/2021/02/07/w3c-prov-some-interesting-extensions-to-the-core-standard/)
- [Bridging the Scientific Knowledge Gap: A Survey of Provenance, Assertion and Evidence Ontologies (PMC)](https://pmc.ncbi.nlm.nih.gov/articles/PMC12376154/)
- [SLSA: Software attestations (v1.0 attestation model)](https://slsa.dev/spec/v1.0/attestation-model)
- [SLSA: Verification Summary Attestation (v0.1)](https://slsa.dev/spec/v0.1/verification_summary)
- [SLSA: in-toto and SLSA](https://slsa.dev/blog/2023/05/in-toto-and-slsa)
- [Toulmin's Model of Argumentation (Springer)](https://link.springer.com/rwe/10.1007/978-94-007-6883-3_4-1)
- [A Computational Approach for Generating Toulmin Model Argumentation (ACL Anthology W15-0507)](https://aclanthology.org/W15-0507.pdf)
- [Trust tiers for graph facts and tier-based retrieval filtering (semantica issue 1557)](https://github.com/semantica-agi/semantica/issues/1557)
- [Grade retrieved facts with trust tier and min_trust_tier filter (semantica PR 1703)](https://github.com/semantica-agi/semantica/pull/1703)
- [AI Audit Trail Requirements: A 2026 Checklist (Kognitos)](https://www.kognitos.com/blog/ai-audit-trail-requirements-2026-checklist/)
- [Leveraging Artificial Intelligence in Audit Planning (CPA Journal)](https://www.cpajournal.com/2026/08/19/leveraging-artificial-intelligence-in-audit-planning/)
- [Making AI Compliance Evidence Machine-Readable (arXiv:2604.13767)](https://arxiv.org/html/2604.13767)
- [AI in E-Discovery (American Bar Association)](https://www.americanbar.org/groups/science_technology/resources/scitech-lawyer/2026-winter/ai-e-discovery/)
- [Unlock AI eDiscovery: Closing the Gap Between Review and Legal Writing (Reveal)](https://www.revealdata.com/blog/unlock-ai-ediscovery-closing-the-gap-between-review-and-legal-writing)
- [AI in Ediscovery (DISCO)](https://csdisco.com/blog/how-artificial-intelligence-transforms-ediscovery)
- [Machine-Readable Docs / llms.txt (Across Docs)](https://docs.across.to/ai-agents/llms-txt)
- [Agent-readable documentation: llms.txt, MCP, and why docs are becoming infrastructure (tela)](https://telawiki.com/tela/tela-blog/1055/agent-readable-documentation-llms-txt-mcp-and-why-docs-are)
