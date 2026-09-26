# Identification conditions for E005

**Theory layer, 2026-09-26. No experiment run. E005 evidence not reopened, and
no number here is new.**

What has to be true of a surface for E005 to be validly measured there. The
answer is a set of conditions, a classification that falls out of them, and one
correction to how the question was being asked.

Built on `docs/evidence/detector-observables-2026-09-26.md` (frozen) and
`docs/evidence/cross-surface-observability-2026-09-26.md` (measured).

## 1. The estimand, stated first

Everything follows from what E005 actually estimated, so it is worth being exact.

E005 did not estimate "the cost of a cache break." It estimated a **family of
conditional medians**:

> m(c) = median change in cache-creation tokens at a boundary, **given** that
> the provider assigned that boundary to divergence class c.

The finding is the **contrast among the m(c)**: +5,005 for `tools_changed`,
0 for `messages_changed`, `system_changed` and `model_changed`.

This has an immediate consequence that reorganises the proposed condition list.
**The write observable and the cause oracle are not two independent
requirements. They are the two arguments of one estimand.** Without the write
counter there is no dependent variable. Without the cause partition there is no
conditioning variable, and the family m(c) has one member, which is not a
contrast and not E005.

## 2. The conditions

### N1. A distinctly priced, per-request, provider-reported write counter

Three separate requirements hide in this one, and surfaces fail at different
ones.

- **Reported, not merely present in a schema.** Codex and Grok both declare a
  write counter and populate it zero times (Codex 0 of 11,776 occurrences
  across 5,830 records, Grok 0 of 2,288 across 1,144).
  Declaration is not observation.
- **Per-request.** A cumulative counter cannot be differenced at a boundary
  without the per-turn breakdown. Codex carries both `total_token_usage` and
  `last_token_usage`, so it satisfies this.
- **Distinctly priced.** The quantity has to be billed at a rate that differs
  from ordinary input, or "write" is not an economic category at all, just a
  description of where tokens went.

### N2. Request sequencing along one prefix lineage

Δ is a difference between consecutive requests, so boundaries must be
identifiable and orderable. This is a property of the client's record, not of
the provider.

### N3. A cause partition, published by the provider, independent of N1

Each boundary assigned to a class, by something that is not the write counter.
Independence is load-bearing and section 5 is about why.

### N4. Commensurable price contract

Needed only to **compare** surfaces, not to identify E005 within one. Section 6.

### N5. No inferential substitution for the quantities in N1 or N3

Not a stylistic preference. Section 5 shows E005 supplies its own counterexample
to the obvious substitution.

## 3. What the proposed condition 2 is actually for

A populated cache-**read** observable was proposed as a necessary condition. On
the identification argument it is not one: Δwrite is computed from the write
counter alone, and the read counter never enters the estimand.

It earns its place somewhere else, and the place matters.

**A zero is ambiguous between "no write occurred" and "no write was recorded."**
A populated read counter beside the zero narrows that ambiguity: it shows the
usage block is being written at all, so the surface is not simply idle. Had both
counters been zero, an idle surface and a broken one would be indistinguishable.

**It narrows the ambiguity without closing it.** A populated read is consistent
with a write field that is genuinely zero and with one that is defaulted to
zero, and it cannot tell them apart on its own. What settled Codex was a
separate argument that did not use the read counter: OpenAI prices GPT-5.6+
writes at 1.25x, and 117 of 117 local `gpt-5.6-terra` turns carry uncached input
with 79 above the 1,024-token minimum while recording zero. **The read counter
was part of the signature; the pricing argument was the evidence.**

So the read observable is **not an identification condition for the effect. It is
a falsifiability condition for the null.**

That distinction would be academic except for one fact about E005: **its headline
finding is a set of nulls.** Three of the four classes report median Δwrite of 0.
Those nulls are only interpretable because Claude Code populates reads on 99.0%
of records, so a zero write on a record with a live read is an observed zero.

**Condition 2 is therefore necessary for E005 as actually conducted, by a
different route than the one proposed.** It is dispensable for a study reporting
only positive effects, and indispensable for this one.

## 4. The measurement boundary is a variable, not a constant

This is the correction to how the question was being asked, and it explains an
error in this project's own documents.

Codex has two different identification statuses depending on where you measure:

| boundary | write observable | status |
|---|---|---|
| Codex transcript on disk | absent in practice, 0 of 11,776 occurrences | fails N1 |
| OpenAI API response | `input_tokens_details.cache_write_tokens`, priced 1.25x on GPT-5.6+ | satisfies N1 |

Same provider, same model, same traffic, opposite answers. **Identification is a
property of the (surface, boundary) pair, not of the surface.**

`docs/RESEARCH-ARCHITECTURE-2026-09-26.md` originally classified surfaces and
concluded no second write contract existed. It was classifying the wrong object.
For **provider-reported usage counters** the transcript is a lossy function of
the API response, so the seam weakly dominates the artifact on those fields. The
claim is scoped to them: a transcript also carries things the response does not,
such as lane and sub-agent structure, which is what Replay's own detector reads. Codex's September builds make
the loss vivid: usage records fell from 26 of 26 sessions to 20 of 235.

This yields the practical separation:

> **N1 decomposes, and the two halves belong to different owners.** N1a, that
> the counter exists and is distinctly priced, is a property of the **provider's
> published contract**. N1b, that it survives to the measurement boundary, is a
> property of the **client's record**. N3 is wholly a contract property; N2 is
> wholly a record property.
>
> Client-side work can recover **N1b** where the provider reports the counter
> and the client discards it. **No client-side work can create N1a or N3.**

Codex fails N1b while satisfying N1a, which is the recoverable shape. OpenAI
lacks N3, which is not. **That spend would in fact recover Codex's writes is a
prediction of this framework and not a finding**, and section 9 records it as
one.

## 5. Why N5 cannot be assumed away, from frozen evidence

The tempting substitution is to derive an unobserved write from observed
quantities, most naturally from tokens that were not cached. E005 already
measured what that substitution would have produced, on the one corpus where the
true write was also observable.

| class | median `cache_missed_input_tokens` | median Δwrite |
|---|---:|---:|
| tools_changed | 13,482 | **+5,005** |
| messages_changed | 54,125 | **0** |
| system_changed | 82,484 | **0** |
| model_changed | **243,095** | **0** |

The proxy is not merely noisy. **It orders the classes against the truth**: the
class with the largest missed-token median shows no write movement, and the only
class with write movement has the smallest missed-token median of the four. A
study substituting missed tokens for Δwrite would have ranked `model_changed`
most material and `tools_changed` least, reversing E005's finding.

No correlation is computed or claimed. Four classes, three of them tied at zero,
do not support a correlation coefficient, and the ordering is what the argument
needs.

**So N5 is not a methodological preference.** E005's own numbers supply a
counterexample, on the one corpus where both quantities are observable, and a
counterexample is enough to forbid assuming the proxy is adequate. It is **not**
a proof that the proxy must fail on every surface, and none is offered; a rule
against substitution does not need one. This needs no new data.

A second, structural reason applies where the derivation would be arithmetic. If
write is defined as input minus cached, then Δwrite is a deterministic function
of Δinput and Δcached, and any finding about it restates its own inputs. The
estimand degenerates rather than becoming inaccurate.

`internal/transcript/codex.go` carries this rule at the one place where the
substitution is reachable in code, pinned by
`TestCodexZeroWriteIsCarriedNotReconstructed` and mutation-proven.

## 6. A circularity guard, because the obvious workaround is circular

N3 requires a partition **independent of** the write counter. The obvious way to
run something E005-shaped on a surface with no oracle is to use Replay's own
detector to assign classes. That is not available, for a reason worth recording
before somebody tries it.

There are two objections and the second is the harder one.

**First, it is reflexive.** E005's entire result on the structural arm is a
measurement of Replay's detector **against** the oracle: 58% to 93% recovery on
`tools_changed`, 24% to 29% on the rest. Using that detector as the partition in
a study meant to generalise E005 would measure the instrument with the
instrument, and the output would be a claim about Replay's classifier rather
than about the provider's cache behaviour.

**Second, and structurally worse, the detector is not independent of the DV's
own source.** `cachemodel.ClassifyBreakWith` branches on `cur.CacheRead == 0`
and on the TTL taken from the cache-creation breakdown (`TTLOf` reads `Create5m`
and `Create1h`), and break detection itself keys on the usage-derived
`ReadBroken` outcome. The candidate partition is therefore a deterministic
function of the same usage block that supplies Δwrite. **That is not an
instrument being imperfect. It is a conditioning variable computed from the
dependent variable**, which is the circularity N3's independence clause exists
to forbid.

This does not touch E005, where the partition was the provider's oracle and the
detector was the thing being measured. It forecloses the workaround, not the
original.

**A detector-partitioned study on a surface without an oracle is a legitimate
study of Replay. It is not an E005 replication and must not be labelled one.**

## 7. The classification, and every class is instantiated

The conditions partition the surfaces into five classes. Classes 0, II and III
were measured on raw files; Classes I and IV are placed from published provider
contracts and are marked **(doc)**. None is hypothetical, but the two are not
the same grade of evidence.

| class | conditions | surfaces | what can be estimated |
|---|---|---|---|
| **0. Identified** | N1, N2, N3 | **Claude Code** | the full family m(c). E005 |
| **I. Marginal only** *(doc)* | N1, N2; **N3 absent** | **OpenAI GPT-5.6+** at the API seam | the marginal distribution of Δwrite. Not the contrast, so not E005 |
| **II. Artifact loss** | N2; N1 at the provider, not at the artifact | **Codex** transcripts, **Grok** | nothing, at this boundary. Codex is recoverable upstream, Grok's provider contract is unknown |
| **III. No observable** | N1 absent entirely | **Cursor, JEV, Ollama** | nothing, at any boundary |
| **IV. Incommensurable** *(doc)* | write not distinctly priced | **DeepSeek, Gemini** | a real economic quantity under a different contract. A different estimand |

Class IV deserves its own line because it is the one that looks like success.
Both providers report cached reads and bill a miss at ordinary input price. The
cost of a break is genuinely visible there. But the quantity is "tokens at 1x
input," while E005's is "tokens at a 1.25x write premium." These are different
random variables under different contracts, and differencing or pooling them is
the scale-identity error already prohibited by
`docs/evidence/gtm-preregistration-2026-09-24.md`:

> Mixed-scale observations may not be differenced until scale identity is
> established.

Note also that Class I passes N4 where Class IV fails it. OpenAI GPT-5.6+ and
Anthropic both price writes at 1.25x of uncached input, so the **units** are
commensurate. What differs is the generating process: Anthropic uses explicit
breakpoints, OpenAI automatic prefix matching. Commensurate units license
comparing magnitudes. They do not license transporting a claim about
class-dependence, because the classes are produced by different machinery.

## 8. The necessary-condition statement

> **E005's estimand is identified at a measurement boundary B of surface S ONLY
> IF B exposes (a) a per-request, distinctly priced, provider-reported
> cache-write counter, (b) sequencing sufficient to difference it along one
> prefix lineage, and (c) a provider-published cause partition independent of
> that counter. Interpreting a null m(c) additionally requires (d) a populated
> read counter on the same records.**
>
> **Corollary.** (a) and (c) are properties of the provider's published
> contract and cannot be manufactured client-side. (b) and the preservation of
> (a) are properties of the client's record and can be. Therefore a blocked
> replication is spend-solvable exactly when the provider publishes the
> observable and the client discards it, and permanently closed when the
> provider does not publish it.
>
> **Status of (c) as of this date:** published by Anthropic, by no other
> provider examined. It is the binding constraint, and it binds for a reason
> nothing in this project can change.

**This is a one-directional statement and is written that way deliberately.**
Each of (a), (b) and (c) is deductively necessary, since each names an argument
the estimand cannot be computed without. **The converse is not claimed.** A
surface satisfying all four might still fail to identify m(c) for a reason not
listed here, and nothing rules out a further condition that Claude Code happens
to satisfy silently. Do not read this box as a biconditional, and do not quote
it as one.

## 9. Claim taxonomy

### ESTABLISHED, deductively from the estimand

- N1, N2 and N3 are individually necessary. Each names something m(c) cannot be
  computed without.
- The write counter and the cause oracle are arguments of one estimand, not two
  independent requirements.
- A detector-supplied partition is circular for generalising E005.

### ESTABLISHED, from frozen E005 numbers

- Substituting missed tokens for Δwrite reverses the finding on the corpus where
  both are observable. N5 is empirically forced.

### ESTABLISHED, from the measured audit

- Classes 0, II and III are instantiated by surfaces **measured on raw files**
  (Claude Code; Codex and Grok; Cursor, JEV and Ollama).
- Identification differs by boundary within one surface, demonstrated on Codex.

### INSTANTIATED FROM PUBLISHED CONTRACTS, NOT MEASURED HERE

- **Class I** (OpenAI GPT-5.6+ at the API seam) and **Class IV** (DeepSeek,
  Gemini). No traffic was captured at an OpenAI seam and no DeepSeek or Gemini
  record was read on this machine. Their placement rests on provider
  documentation, and an earlier draft of this document wrongly described every
  class as measured.

### SUPPORTED, NOT ESTABLISHED

- That the conditions are jointly **sufficient**. Only necessity is argued.
- That Grok belongs in class II rather than class IV. xAI's cache pricing
  contract was not found, so its zero is uninterpreted.

### NOT ESTABLISHED

- Anything about what a Class I study on OpenAI would find. It has not been run,
  and this document does not recommend running it.
- That Codex's write field would be non-zero at the API seam. This is the
  framework's own **falsifiable prediction**: on GPT-5.6+ traffic, writes
  observed at the seam should be non-zero where the transcript reads zero. If a
  seam capture also read zero, N1 would fail at the provider for OpenAI too and
  section 4's table would be wrong.
- That fixing Codex's client logging is feasible or will happen.

## 10. What this changes

Nothing about E005, which stays frozen.

It replaces the question "which surfaces qualify" with "which (surface,
boundary) pairs qualify, and which of the failures are purchasable." That is a
better question because it has different answers, and because the previous
framing produced a wrong conclusion in this project's own architecture document.

**It does not open a new experiment.** The ranking in the cross-surface audit is
unchanged: a second operator's Claude Code corpus is still the only Class 0 test
available, and it is still the cheapest.
