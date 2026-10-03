# Panel: are population-bounded claims a primitive?

**2026-10-02. Second invention session. Verdict: NARROW PRODUCT and partial
CROSS-DOMAIN PRIMITIVE. Not a platform primitive. Not an invention.**

## 1. EXECUTIVE VERDICT

The six defects are **not one failure**. They are three, and only four of the six
share the structure the brief proposed. The shared structure is real and it
recurs outside cost, including inside the mechanism this repository built to
prevent it, which is the strongest evidence for it. But the mechanism that would
enforce it is subsumed by established prior art, and the temporal extension is
classified NOT APPLICABLE to the wired product. The honest classification is
**C, a product wedge around evidence-bounded accounting**, with a partial
cross-domain claim. **D is rejected.**

## 2. THE REPEATING FAILURE, and where the analogy breaks

Round 2 required classifying each defect rather than asserting unity. Doing so
splits them:

| defect | what was actually wrong | type |
|---|---|---|
| C035 | total covered the priced subset, surface said otherwise | **scope dropped at a boundary** |
| C037 | re-billed dollars and tokens covered different populations | **scope dropped at a boundary** |
| SP-01 | gate PASS stated no exclusions its own FAIL stated | **scope dropped at a boundary** |
| V1 | median admitted rows that could contribute nothing | **scope dropped at a boundary** |
| **Q01** | membership decided by an unstable key: lane file name and cache state | **membership determination** |
| **C036** | each deficit priced at the wrong record's rate | **parameter selection** |

**Four of six** are one structure: a value and its scope are separable in the
code, so the value propagates across a boundary and the scope does not.

**Q01 is different.** Nothing was dropped. The population's *membership rule*
was non-deterministic. A scope-carrying value would not have prevented it.

**C036 is different again.** The population was correct. The wrong parameter was
selected per element. Carrying scope would not have prevented it either.

Stating the six as one failure is itself the failure under discussion: a claim
true of a subset presented as true of the whole. **MEASURED.**

## 3. COMPETING PRIMITIVES

| candidate | verdict |
|---|---|
| **population / denominator** | survives as vocabulary, dies as a primitive. Round 4: a careful analyst with SQL computes it. Renaming a `WHERE` clause is not a mechanism |
| **evidence coverage** | survives, and is the one the repository already implements |
| **evidence frontier / observation frontier** | **killed by Round 8.** The wired product is postmortem by construction and classifies the temporal frontier NOT APPLICABLE. No wired surface justifies an earlier decision with later evidence |
| **claim contract (value inseparable from scope)** | strongest survivor as a mechanism, but see §10 |
| **support set / admissible evidence set** | dies as a rename of coverage |
| **negative claim ("unpriced ≠ $0")** | **already implemented**, so not the missing primitive. `CauseUnknown` vs `CauseNotMeasured`, `BranchTaken (taken, known)`, three-valued absence throughout |

## 4. THE STRONGEST SURVIVING PRIMITIVE

> **An aggregate whose value cannot be obtained without its coverage.**

Not "report the denominator". The value and the coverage are one object and the
language offers no way to take the first without the second.

## 5. WHAT IT MECHANICALLY DOES

It converts a class of defects from *caught by review* to *unrepresentable*.
SP-01, C037-at-the-card and the corpus-payload gap each occurred because
`TotalUSD float64` and `RebilledUSD float64` can be read, printed and serialized
by any caller with no obligation to carry what they cover.

It would prevent **four of the six** defects. It would not have prevented Q01 or
C036. That bound is the honest scope of the mechanism.

## 6. EXISTING REPLAY SUBSTRATE

- Coverage counters exist and are computed: `PricedRequests`, `UnpricedRequests`,
  `UnpricedRebilledTokens`. **MEASURED.**
- Three-valued absence is implemented throughout. **MEASURED.**
- A derived cache key reflected from the data's own shape, so a shape change
  self-invalidates. **MEASURED**, and it is the repo's strongest idea.

## 7. WHAT IS MISSING, and a correction to the claim register

The register asserts `RPL-C021` is **Established** with
`Why: "Enforced by the shape of the type rather than by a convention"` and
`Oracle: "type-level: the summary type exposes no addition"`.

Inspected: `TestC021_NoCrossSurfaceTotalIsReachable` reflects over `surfaceBurn`
and fails on **banned substrings in field and method names** —
`grandtotal`, `alltotal`, `combined`, `sumall`, `overall` — plus the presence of
a `has*` prefix.

That is a lint over identifiers. A method named `Rollup()`, `Everything()` or a
field named `Aggregate` passes it untouched. **It is a convention, policed by a
name check, described in the register as type-level enforcement.**

So the repeating failure appears **inside the mechanism built to prevent it**.
That is the single most persuasive piece of evidence that the pattern is
structural rather than six cost bugs. **MEASURED.**

It also means the primitive in §4 does **not** already exist. Nothing in Replay
makes a figure unobtainable without its coverage.

## 8. CROSS-DOMAIN TEST

| domain | instance | status |
|---|---|---|
| cost | C035, C037, SP-01 | **MEASURED** |
| verification | V1: the invoice-facing median admitted unpriceable rows | **MEASURED** |
| policy / enforcement | the proxy cap enforces on an upper bound while the flag that discloses it could not fire | **MEASURED** |
| agent evaluation | R10 stopped because the control arm was at ceiling: the claim "durable state improves outcomes" had no admissible population | **MEASURED** |
| inter-agent communication | "B received A's message" over which evidence | **NOT MEASURED** |

Four domains with measured instances. The cross-domain test **passes**, which is
why this is not classified KILL.

## 9. FIVE-CAPABILITY TEST

| | |
|---|---|
| durable scratch | **NO CONNECTION.** Scratch is content; coverage is about aggregates over evidence |
| work continuity | **PLAUSIBLE.** A handoff claim could name its evidence population. Untested, and gated on an outcome signal that does not exist |
| inter-agent communication | **PLAUSIBLE.** received/observed/used as progressively stronger claims over different evidence. **NOT MEASURED** |
| quota titration | **SUPPORTED.** An allocation is a claim about a population and inherits the structure directly |
| realtime | **NOT MEASURED.** Would require the evidence frontier, which Round 8 killed for the wired product |

Two of five connect. Forcing the other three would be the failure this document
is about.

## 10. PRIOR-ART RESULT

**Green, Karvounarakis and Tannen, "Provenance semirings", PODS 2007**
formalises exactly this: annotate tuples with semiring elements and relational
operations propagate the annotations, with different semirings yielding lineage,
why-provenance, trust and probability. Coverage propagating through aggregation
is a semiring annotation on relational algebra.

**Information-flow typing and taint tracking** (Jif, FlowCaml, LIO) is the same
shape in programming languages: a value carries a label, the label propagates
through operations, and declassification must be explicit.

**Units-of-measure types** (F#, dimensional-analysis libraries) are the direct
analogue of the repo's own incommensurability guard.

The mechanism in §4 is an application of established work to a new domain. That
is engineering, not invention.

## 11. IP CANDIDATES

**ORDINARY / PRIOR ART.** No candidate survived §10.

The one asymmetry worth recording without claiming novelty: provenance semirings
and taint lattices both propagate a label through a *join*. The operation that
matters here is the **denominator**, which has no analogue in either formalism,
because a semiring annotation says which tuples contributed and not which tuples
were eligible but absent. Whether that gap is substantive is **NOT MEASURED**,
and the patent and digital-forensics search arms from the previous session were
never cleared.

## 12. PRODUCT FORM

Every figure Replay prints is accompanied by what it covers, and the gate, the
card, the JSON and the pooled document cannot drop it.

## 13. DEMO

**Before:** `re-billed $0.02 · 40k tokens re-billed`. A reviewer asks what it
covers. There is no answer in the output.

**After:** `re-billed $0.02 · 40k tokens re-billed · covers 50% of those tokens:
20,000 of 40,000 were priced, 20,000 ran on a model no price table carries.`

The reviewer can now challenge the figure and get a defensible answer instead of
a smaller one. This is shipped for `replay cost` and the ceiling gate; it is not
shipped for the card or the pooled corpus.

## 14. KILL LIST

| killed | why |
|---|---|
| "the six defects are one failure" | Q01 is membership determination, C036 is parameter selection. Four of six, not six |
| evidence frontier as the deeper primitive | Round 8: NOT APPLICABLE to a postmortem product |
| population as a primitive | Round 4: it is a `WHERE` clause with a better name |
| negative claims as the missing piece | already implemented |
| platform primitive (classification D) | prior art subsumes the mechanism; two of five capabilities connect |
| any IP candidate | provenance semirings, information-flow typing, units-of-measure |

## 15. EVIDENCE STATUS

**MEASURED:** the six defects and their three-way classification; four
cross-domain instances; that `RPL-C021`'s oracle is a name lint and the register
overstates it; that coverage counters and three-valued absence exist.

**INFERRED:** that making coverage inseparable would have prevented four of six.
Reasoned from the defect mechanics, not demonstrated by building it.

**HYPOTHESIS:** that a buyer values coverage-carrying figures.

**NOT MEASURED:** demand; communication and continuity connections; whether the
denominator gap in §11 is substantive.

## 16. SINGLE NEXT EXPERIMENT

**Repair the `RPL-C021` oracle so it discriminates, and see what else fails.**

Replace the banned-substring lint with a check that actually enforces the
invariant, then run it against every figure-bearing type in the repository. The
experiment is cheap, needs no new architecture and no credits, and it
discriminates directly between the two surviving readings: if the honest oracle
fires only on cost types, this is a narrow accounting product (B/C). If it fires
across evaluation, policy and verification types too, the cross-domain claim in
§8 stops being four hand-picked instances and becomes a measured rate.

It also repairs a verified overclaim in the claim register, which must happen
regardless of the verdict.
