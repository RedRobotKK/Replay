# Panel retry: the three failure classes are one mechanism

**2026-10-02. Second attempt. The previous panel was wrong on its central
structural claim. Verdict: CROSS-DOMAIN ENGINEERING PRIMITIVE. No invention.**

## 1. What the previous panel got wrong

It concluded the six defects were three unrelated mechanisms and that a scope
primitive would prevent "four of six". **That is wrong.** All six are one
failure, and the unification is in §5. **MEASURED** (by re-derivation from the
defect mechanics, below).

## 2-3. The six defects and their apparent classes

| | what happened |
|---|---|
| C035 | total covered the priced subset; the surface did not say so |
| C037 | re-billed dollars and tokens covered different populations |
| SP-01 | the gate's PASS stated no exclusions its own FAIL stated |
| V1 | the invoice-facing median admitted rows that could contribute nothing |
| Q01 | the folded session's gate flag came from whichever lane sorted first |
| C036 | each break's deficit was priced at the first record's rate |

Previous classification: scope propagation (4), membership (1), parameter (1).

## 4. Ten candidate primitives

| candidate | what it constrains | verdict |
|---|---|---|
| population | row selection | dies: a `WHERE` clause renamed |
| coverage | ratio reporting | survives as output, not as mechanism |
| support set | which rows contributed | dies into provenance semirings |
| eligibility set | which rows could have contributed | **survives**, and semirings do not express it |
| provenance | input-to-output lineage | ordinary |
| evidence frontier | time-bounded admissibility | killed previously; postmortem product |
| claim contract | value plus declared scope | weak: depends on the producer declaring honestly (§15) |
| **qualifier binding** | which element a qualifier may come from | **strongest survivor, §5** |
| validity condition | predicate under which a value holds | collapses into qualifier binding |
| scoped computation | typed wrapper preventing bare extraction | the enforcement mechanism for the above, not a separate primitive |

## 5. THE STRONGEST COMMON INVARIANT

> **A derived value and anything that qualifies, gates, scales or bounds it must
> originate from the same evidence element, or from an explicitly stated rule
> for combining elements.**

Call the failure **wrong-element qualification**. All six defects are instances,
and the previous panel's three classes collapse:

- **C036** quantity from record *i*, rate from record *0*. Wrong element,
  explicitly.
- **Q01** dollars summed over all lanes, the flag that gates them taken from
  lane *0*. Wrong element, explicitly.
- **C035, C037, SP-01, V1** the qualifier was **dropped**. A reader presented
  with an unqualified figure supplies the only qualifier available to them: the
  whole population. **Dropping is the degenerate case of wrong-element
  qualification**, where the wrong element is "everything".

That last step is what the previous panel missed. **INFERRED**, by construction
from the six defect mechanics; each individual mechanic is MEASURED.

## 6. Counterexamples

| | caught? |
|---|---|
| 1 correct population, wrong parameter | **yes**, C036 is this |
| 2 correct parameter, wrong population | **yes**, Q01 is this |
| 3 population correct, identity changes | **yes**, Q01 again: identity *is* the membership rule |
| 4 evidence disappears after computation | **no.** Binding is established at derivation; later deletion is not detected |
| 5 evidence arrives later | **no**, see §9 |
| 6 transformation combines two populations | yes, if the combination rule must be stated |
| 7 *legitimate* combination | **not falsely rejected**, provided the rule is stateable. If it cannot be stated, rejection is correct |
| 8 two derivations, different ordering | **yes**, this is exactly C036's FM repair |
| 9 a field is renamed | **yes** for a behavioural oracle, **no** for the shipped one (§7) |
| 10 a developer adds an aggregation unaware of the convention | **the decisive one, §7** |

## 7. C021 BEHAVIOURAL ORACLE: built, run, and it falsifies a registered claim

`RPL-C021` is registered **Established**, `Oracle: "type-level: the summary type
exposes no addition"`, `Why: "Enforced by the shape of the type rather than by a
convention"`.

Its actual implementation bans five substrings — `grandtotal`, `alltotal`,
`combined`, `sumall`, `overall` — in field and method names, plus requires a
`has*` prefix.

**Experiment.** A behavioural detector was written: find any function consuming
more than one `surfaceBurn` and returning a bare numeric type. Name-independent.
Then an adversarial plant, `func rollup(bs []surfaceBurn) int`, summing
`b.requests` across surfaces.

```
shipped name-lint   : ok            (MISSED)
behavioural oracle  : 1 found       (rollup returns int from 2 surfaceBurn)
```

**MEASURED.** A cross-surface total passes the registered guard under any
unbanned name. Counterexample 10 fails for the shipped oracle and passes for the
behavioural one.

**Why was the invariant hard to encode?** Because Go erases it at field access.
`s.TotalUSD` is a `float64`; the moment the value is extracted, everything about
what it is true of is gone, and no later check can recover it. The invariant can
only be enforced by preventing the bare extraction, which is abstract-data-type
discipline, not a new idea.

## 8. Composition

Composing claims over populations A and B must not imply a claim over A ∪ B.
Under qualifier binding this is well defined: the combination rule must be
stated, and where overlap is unknown the result is bounded, not summed. The repo
already enforces the degenerate case — `RPL-C021` exists precisely to refuse
cross-surface addition — **but enforces it by name only**. **MEASURED.**

## 9. Monotonicity

Later evidence should widen coverage without rewriting history. The repo cannot
express this: every version gate discards rather than migrates, and the temporal
frontier is classified NOT APPLICABLE to a postmortem product. **Counterexamples
4 and 5 are therefore outside the mechanism.** This is a real bound, not an
oversight. **MEASURED.**

## 10. Four non-cost domains

| domain | the wrong-element instance | status |
|---|---|---|
| verification | V1: median qualified by a population that included non-contributors | **MEASURED** |
| policy enforcement | the proxy cap enforces on an upper bound while the disclosing flag cannot fire | **MEASURED** |
| agent evaluation | R10: the claim's qualifier (task headroom) came from a population with none | **MEASURED** |
| transcript reconstruction | an unknown-kind record counted as `Skipped`, qualifying "parse failure" with an element that parsed fine | **MEASURED**, repaired this week |

The primitive protects all four by the same rule. **INFERRED.**

## 11. Five capabilities

| | |
|---|---|
| quota titration | **SUPPORTED.** An allocation qualified by progress from a different population is exactly wrong-element |
| work continuity | **PLAUSIBLE.** A handoff claim qualified by evidence from the wrong session. Untested |
| inter-agent communication | **PLAUSIBLE.** "B acted on A's message" qualifies B's action with A's element. Untested |
| realtime | **NOT MEASURED.** Needs the frontier, killed in §9 |
| durable scratch | **UNRELATED.** Scratch is content, not a derived value |

## 12. Security

The mechanism addresses **evidentiary validity, not authorization**, and the two
must not be collapsed. It does bear on claim laundering: "B observed X" asserted
by A is a claim about B's element made from A's. Under the invariant that is
exactly the forbidden shape. **HYPOTHESIS**, no implementation, no test.

## 13. Protocol

The invariant is transport-independent: what must travel with a claim is the
identity of the element its qualifier came from. That is a payload requirement,
not a protocol feature. **INFERRED.**

## 14. SQL comparison, done rigorously

Given all records, lineage metadata and arbitrary SQL, an analyst **can** compute
coverage. `SELECT SUM(cost) FILTER (WHERE priced), COUNT(*)` is trivial.

What SQL does not do is **prevent the number from being presented without it**.
The result of a query is a bare scalar; nothing travels with it. That is the
whole of §7's finding, restated: the failure is at extraction, not at
computation.

**This is a real distinction and it is also an old one.** It is the argument for
abstract data types over exposed representations, made in 1972.

## 15. Untrusted producer

If a producer declares its own scope, it can lie, and the claim-contract
candidate dies here. **MEASURED** by inspection: nothing in Replay verifies a
declared denominator against the records it was computed from.

## 16. Reconstruction

Replay's actual strength: it does not take a declaration, it recomputes from the
ledger. So the support domain can be derived independently of any producer, and
a discrepancy between declared and reconstructed scope could itself be a claim.
**That is the strongest Replay-shaped use of the invariant** — and it is
reconciliation, which accounting has done for centuries. **INFERRED.**

## 17. Prior art

| mechanism | prior art | does it subsume? |
|---|---|---|
| qualifier travels with value | abstract data types, information hiding (Parnas 1972); newtype plus smart constructor | **yes**, as an enforcement technique |
| annotation propagates through aggregation | provenance semirings (Green/Karvounarakis/Tannen, PODS 2007) | **yes**, for which tuples contributed |
| label cannot be dropped silently | information-flow typing, taint tracking (Jif, FlowCaml, LIO) | **yes** |
| incommensurable units not added | units-of-measure types, dimensional analysis | **yes** |
| declared versus recomputed scope | double-entry reconciliation, accounting controls | **yes** |

**Not subsumed, and recorded without a novelty claim:** semirings annotate which
tuples *contributed*; the defects here turn on which tuples were *eligible and
absent*. Taint lattices join labels through an operation; none of them constrain
**which element a qualifier may be drawn from**. Whether that gap is substantive
is **NOT MEASURED**, and the patent and forensics search arms remain uncleared
from the previous session.

## 18-21. The surviving mechanism, minimally

**Invariant:** a derived value's qualifier must come from the same evidence
element as the value, or from a stated combination rule.

**Representation:** the pair, inseparable. No accessor returns the value alone.

**Enforcement rule:** a function may not consume more than one evidence element
and return a bare primitive.

**Reconstruction rule:** recompute the support domain from the ledger and compare
it to what the surface presented; the discrepancy is itself a claim.

**Falsification test:** the §7 experiment, generalised — plant an adversarially
named reducer over any figure-bearing type and require the oracle to fire.

## 22. Product wedge

Every figure carries what it is true of, and the carrying is enforced rather
than reviewed. Demand: **HYPOTHESIS**. No customer evidence exists.

## 23. Demo

**Before:** `re-billed $0.02 · 40k tokens`. **After:** the same, plus
`covers 50%: 20,000 of 40,000 priced`. **Mutate the parameter** — price each
break at the first record's rate — and the figure moves from $0.048 to $0.300
while the coverage line is unchanged, which is C036 made visible.

## 24-25. Strongest alternative, and the verdict

> "This is abstract data types plus provenance plus reconciliation, applied to
> agent telemetry."

**That alternative wins on mechanism.** Every component is established. What is
not established elsewhere is the *combination applied to a reconstructable
substrate*, and combination is not invention.

**FINAL VERDICT: 3. CROSS-DOMAIN ENGINEERING PRIMITIVE.**

Real, reusable, unifies all six defects rather than four, mechanically
enforceable as §7 demonstrates, protects four non-cost domains — and ordinary
engineering given §17. **NO INVENTION IDENTIFIED.**

## 26. Single next experiment

**Run the behavioural oracle against every figure-bearing type in the
repository, and repair `RPL-C021`'s registered claim.**

§7 proved the oracle discriminates on one type with a negative control. The
experiment is to generalise it and count: if it fires only on cost types, this is
an accounting product. If it fires across verification, policy and evaluation
types, §10's four instances become a measured rate rather than four hand-picked
examples.

It is cheap, needs no architecture, spends no credits, and it repairs a
registered **Established** claim that this session measured to be false.
