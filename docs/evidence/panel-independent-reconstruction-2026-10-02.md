# LOCKED ROOM V: can Replay independently reconstruct what a claim is bound to

**2026-10-02. Twelve seats, twenty-six phases. No production change. Nothing committed.**

Every substantive statement below is labelled **MEASURED**, **RECONSTRUCTED**,
**INFERRED**, **HYPOTHESIS** or **NOT MEASURED**. A producer claim is not
evidence because it is present. Missing evidence is not negative evidence.

---

## 1. Baseline

**MEASURED.** `go test ./...` before any work: **37 packages ok, 0 FAIL**, one
package with no test files. Re-run at the end of the room with the throwaway
probes present: **37 ok, 0 FAIL**. Re-run after removing them: recorded in §28.

**Ownership boundary**, established from file mtimes rather than from memory.
Six working-tree entries predate this campaign and belong to other sessions.
They were not read for content, not modified, not staged:

| other session | last touched |
|---|---|
| `.claude/hooks/`, `.claude/settings.json` | 2026-09-25 18:35 |
| `docs/evidence/modes-catalogue-2026-09-25.md` | 2026-09-25 22:05 |
| `docs/WORK-STATE.md` | 2026-09-27 10:57 |
| `docs/design/UNWIRED-LOG.md` | 2026-10-01 12:25 |
| `internal/regression/unwired_packages_test.go` | 2026-10-01 12:25 |

Everything from `2026-10-01 16:36` onward is this campaign's. The only
production file this room touched is `cmd/replay/cost.go`, mutated twice and
reverted twice; both reverts verified **byte-identical** against a backup held
outside the repository.

**Throwaway scaffolding**, all removed (§28): `cmd/replay/zz_recon_test.go`,
`cmd/replay/zz_drop_test.go`, `internal/cachemodel/zz_control.go` (written and
deleted inside one test), `internal/cachemodel/zz_rates_test.go`.

---

## 2. Operational claim

> Given raw observed evidence **E** and a producer-declared result **R** with
> declared binding **B**, Replay can independently reconstruct a binding **B′**
> from **E** and determine whether **R** is admissible under **B′**.

Instantiated against `replay cost`, which is the smallest substrate in the
repository that already has all six parts:

| | |
|---|---|
| **E** | the ledger JSONL bytes on disk: one line per proxied request, carrying `ts`, `model`, and `response.usage.{cache_creation_input_tokens, cache_read_input_tokens}` |
| **R** | the published `replay cost --json --per-task` document: `rebilledUsd`, `rebilledTokens`, `unpricedRebilledTokens`, and the row's `model` and `requests` |
| **B** | what the document asserts about what those figures range over: the row's `model` is the declared rate parameter, `requests` the declared population, `unpricedRebilledTokens` the declared exclusion |
| **B′** | reconstructed from E alone: the break set, each break's own deficit, each break's own rate, the convex hull of those rates, the exact per-element total, the frame size, and the eligible-but-absent count |

**admissible** is a conjunction of four independently decidable checks, not one
comparison. Writing it as one comparison is what hides which half failed:

- **T1 quantity conservation** — declared `rebilledTokens` equals reconstructed deficit tokens.
- **T2 exclusion disclosure** — declared `unpricedRebilledTokens` equals the reconstructed unpriceable deficit.
- **T3 parameter hull** — the implied parameter `rebilledUsd × 10⁶ / (rebilledTokens − unpricedRebilledTokens)` lies inside the convex hull of the per-element rates.
- **T4 exact reconciliation** — declared dollars equal the reconstructed per-element sum.

A fifth is computed as a **diagnosis** rather than a gate:

- **T5** — the implied parameter equals the *first* record's rate, which is the parameter a wrong-element producer reaches for.

This is operationalizable. It is not **HYPOTHESIS NOT OPERATIONALIZABLE**.

---

## 3. Toy model

The toy world collapsed into the real one and the room let it. Every structure
Phase 2 asked for already exists in `replay cost` with a hostile property no
toy would have had: the producer is 1,100 lines of shipped Go that nobody
wrote to be reconstructible. Building a clean toy and then declaring success
on it would have been the vacuous version of this experiment.

The structures, as instantiated:

```
Evidence element      one ledger line
Qualifier             whether that line's model is in the price table at that line's timestamp
Parameter             that line's input rate, $/MTok
DerivedValue          rebilledUsd
DeclaredBinding       the row's `model` string + `requests` + `unpricedRebilledTokens`
ReconstructedBinding  break set, per-element rates, hull, exact total, frame, absent count
```

Deliberate corruption is available: the historical C036 defect is a one-line
change to `cmd/replay/cost.go`, and it was applied and reverted twice.

---

## 4. Untrusted producer experiment

**MEASURED.** Phase 3's six cases, as far as the substrate admits them.

| case | construction | reconstruct | compare | detect | classify |
|---|---|---|---|---|---|
| **A** declared == actual | shipped binary, mixed-rate corpus | yes | yes | n/a, admissible | correct |
| **B** declared ≠ actual | C036 mutation reapplied | yes | yes | **yes** (T4, T5) | correct: names the lead-record substitution |
| **C** declared broader than evidence | unpriceable break absorbed into the represented half | yes | yes | **yes** (T2, T3) | correct |
| **D** declared narrower than evidence | **NOT MEASURED** | — | — | — | — |
| **E** producer omits binding | §12: the published document carries no frame size | yes for E | **no** | **no** | undetectable from the document |
| **F** producer invents evidence identity | a break on a model the price table does not know | yes | yes | yes | correct: excluded, not absorbed |

Case D was not constructed. A producer declaring a *narrower* population than
the evidence supports is conservative, and no historical defect took that
direction, so nothing in the repository motivated the fixture. It is a real
gap in this room's coverage, not a case judged uninteresting.

Case E is the important negative and is treated in §8 and §12.

---

## 5. Central reconstruction experiment

**MEASURED.** The reconstructor's independence was enforced rather than
asserted, because the obvious way to write this experiment is to accidentally
call the producer's own code and then report that the producer agrees with
itself:

- it parses the JSONL with its own minimal struct — never `ledger.Record`, never `ledger.ReadRecords`;
- it re-derives breaks from first principles (a request reads what the one before it wrote, so `expected = prev.create + prev.read`) — never `analysis.AnalyzeEveryLane`;
- it uses **literal** rates, not `cachemodel.PriceForAt`. The literals are pinned against the production table in exactly one test, so a rate change breaks the probe loudly instead of letting the reconstructor silently inherit the producer's answer.

**The producer supplies nothing the reconstructor needs.** That is the
condition Phase 4 says the experiment fails without, and it holds.

### Case A — the shipped producer

Corpus: lead `claude-3-5-haiku` ($0.80), break of 20,000 tok on
`claude-opus-5` ($5.00), break of 20,000 tok on `claude-3-5-haiku` ($0.80).

```
frame=3 readable=3 ELIGIBLE-BUT-ABSENT=0
reconstructed: 2 breaks, 40000 tok (0 unpriceable), hull $0.80..$5.00/MTok, exact $0.116000
declared     : model=claude-3-5-haiku $0.116000 over 40000 tok (0 excluded)
implied parameter $2.9000/MTok   lead-record rate $0.80/MTok
T1 tokens=true  T2 exclusion=true  T3 hull=true  T4 exact=true  | T5 implied==lead: false
ADMISSIBLE: true
```

The corpus discriminates: the implied parameter $2.90 is neither of the two
rates and is nowhere near the lead's $0.80, so a passing verdict here is not a
verdict a wrong-element producer could also earn.

### Case B — the C036 defect reintroduced into production code

`cmd/replay/cost.go`, the per-break pricing site, changed from the record's own
model and timestamp to the session lead's:

```go
-if p, ok := cachemodel.PriceForAt(br.Turn.Request.Model, br.Turn.Request.Timestamp); ok {
+if p, ok := cachemodel.PriceForAt(model, at); ok {
```

Same corpus, same reconstructor:

```
reconstructed: 2 breaks, 40000 tok (0 unpriceable), hull $0.80..$5.00/MTok, exact $0.116000
declared     : model=claude-3-5-haiku $0.032000 over 40000 tok (0 excluded)
implied parameter $0.8000/MTok   lead-record rate $0.80/MTok
T1 tokens=true  T2 exclusion=true  T3 hull=TRUE  T4 exact=false  | T5 implied==lead: TRUE
ADMISSIBLE: false
```

Two results the room did not expect and must record:

**T3, the hull check, PASSED under the defect.** $0.80 is a boundary point of
$0.80–$5.00 because the lead record's model is also one of the breaking
records' models. The hull is a necessary condition and a weak one. **T4, exact
reconciliation, is the load-bearing check**, and T5 is what turns detection
into a diagnosis: it names the substituted element rather than only reporting
disagreement.

A comparator that had shipped only the hull check would have been a check that
cannot fail on the very defect it was written for.

**MEASURED.** The mutation was reverted and `cost.go` diffed byte-identical
against a backup held in the job scratch directory, outside the repository.

### What reconstruction did NOT add

**MEASURED, and it is the room's least comfortable result.** The same mutation
turns **seven** shipped `TestC036_*` tests and **four** shipped `TestC037_*`
tests red. The in-repository white-box oracle catches this defect at least as
well as the independent reconstructor does.

Independent reconstruction therefore adds **no detection power** over the
oracle the producer already ships. What it adds is *who can run it*: the
white-box oracle needs the fixture specification and lives inside the
producer's repository, and the reconstructor needs only the bytes and the
document. That is a **trust-transfer** property, not a correctness property,
and §21 shows trust transfer is exactly what authenticated data structures and
proof-carrying data already do, with a commitment Replay does not have.

---

## 6. Six-defect matrix

**MEASURED** where a mutation was run; **RECONSTRUCTED** from the evidence
files where the defect is already repaired and documented.

| Defect | Arithmetic unchanged? | Binding changed? | Independent reconstruction catches? |
|---|---|---|---|
| **C035** total covered the priced subset, the surface said otherwise | **yes** — the dollar figure was always correct over its own subset | yes: population qualifier dropped | **yes.** `pricedRequests`/`unpricedRequests` are in the JSON; a reconstructor recounts and compares |
| **C036** each break priced at the first record's rate | **no** — the number moved, $0.116 → $0.032 | yes: parameter from the wrong element | **yes, MEASURED** (§5), **except inside the hull** (§11) |
| **C037** re-billed dollars and tokens covered different populations | **yes** — both figures were individually correct | yes: two populations, one result | **yes.** T2 fires; MEASURED under the same mutation |
| **SP-01** the gate's PASS stated no exclusions its own FAIL stated | **yes** — identical number | yes: qualifier dropped on one branch | **no.** The qualifier is `[]string` prose, rendered only, absent from any JSON |
| **V1** unpriceable rows counted as $0 in a median | **no** — the median moved | yes: ineligible elements admitted to the population | **yes** in principle; the counts exist as `BeforeCounted`/`AfterCounted` on an unexported struct, render-only |
| **Q01** the folded session's gate flag came from whichever lane sorted first | **yes** on the corpus measured — incidence was zero | yes: qualifier from the wrong element | **yes.** `unpriced` is a JSON field; a reconstructor recomputes it per lane |

**The unity does not hold at the level of a common machine-checkable
mutation.** All six are expressible as binding mutations — that much survives
from the previous room. But three of the six change no arithmetic at all, and
two of those three carry their qualifier only as rendered prose, where no
reconstructor can reach it. A single mechanism that catches all six does not
exist in this substrate. §12 shows that the static alternative catches **none**
of them.

---

## 7. Provenance attack

The strongest provenance seat argues: *everything here is provenance.*

Ordinary how-provenance records `R ← {E₁,E₂,E₃}`. The proposed binding adds
eligibility, qualifier, parameter and transformation edges. Can the first
represent the second?

**Largely yes, and the room concedes it.** Amsterdamer, Deutch and Tannen's
provenance for aggregate queries (PODS 2011) annotates *individual values
within tuples*, not only tuples, which is precisely the granularity C036
needed: a tensor term pairing each quantity with the annotation of the tuple
it came from makes `quantity_i × rate_0` a visibly different provenance term
from `quantity_i × rate_i`. Replay's C036 is that defect and that formalism
represents it.

So the room asks Phase 7's follow-up: *can provenance systems independently
reconstruct and enforce admissibility when the producer is untrusted?*

**Representation, yes. Enforcement, no, and reconstruction only partially.**
A provenance polynomial faithfully records a mis-binding as a mis-binding; it
does not reject it. And the annotation is produced *by the producer's own
engine*. A consumer can recompute the polynomial if they have the inputs and
the query — which is the same white-box position §5 showed adds nothing.

**The precise missing capability is not in the provenance direction at all.**
It is §8: a provenance alphabet in which *eligible-but-unread* is a distinct
symbol. Semiring 0 means "did not contribute" and conflates never-observed
with eligible-and-skipped. That conflation is the one thing provenance
genuinely cannot express, and it is what §12's measurement found missing from
Replay's own output.

---

## 8. Eligibility vs contribution

**MEASURED.** One unparseable line was appended to an otherwise valid ledger
file and both sides were asked for a count.

```
raw frame=4  readable=3  eligible-but-absent=1
published row says requests=3, summary tasks=1
```

Against Phase 8's six claims:

| claim | independently reconstructable? |
|---|---|
| **A** 60 records contributed | **yes** — recount the readable lines that produced a break |
| **B** 60 of 80 eligible contributed | **yes from raw evidence** — the frame is enumerable because unreadable lines are still lines |
| **C** 60 of 100 observed contributed | **no.** "Observed" above the file level is not establishable: a record never written leaves nothing behind |
| **D** 80 eligible records exist | **yes from raw evidence, no from the document** |
| **E** 20 records were excluded | **yes from raw evidence, no from the document** |
| **F** 20 records were not eligible | **no.** Unreadable is not the same as ineligible, and nothing distinguishes them |

**The production measurement is sharper than the toy one.** Replay's
`transcript.Session` already carries **five** distinct absence states, each
documented at length as a deliberate separation: `Skipped`, `UnknownKinds`,
`Refusals`, `ProviderFailures`, `SchemaMismatch`. Asking which of them reach
the `replay cost` document:

| state | reaches `cost --json`? |
|---|---|
| `Skipped` (bytes unparseable) | **no** |
| `UnknownKinds` (parsed, kind not implemented) | **no** |
| `ProviderFailures` | **no** |
| `SchemaMismatch` (intact bytes, wrong schema version) | **no** |
| `Refusals` | referenced in `cost.go`, not a published key |
| file-level `unreadable` | **yes** — a file count, not a record count |
| `unpriced` | **yes** — a transcript count |

**MEASURED: Replay distinguishes five absence states in its type system and
publishes none of them at record granularity.** `SchemaMismatch` is the
sharpest case in the repository: those records are known to exist, known to be
eligible, and deliberately not interpreted, because `SchemaVersion` is an
exact-equality gate whose own documentation says bumping it "DISCARDS EVERY
EXISTING LEDGER". A lineage graph for the output cannot contain them, because
lineage edges exist only for inputs that were read.

This is the one component of the room's hypothesis that the prior-art search
could not kill (§21). It is also, measured here, a **reporting gap in Replay,
not a missing primitive**: the distinction is already made in the type and is
dropped at the document boundary.

---

## 9. Absence experiment

**MEASURED, by construction from §8.** Replay can establish:

- **OBSERVED AND NOT USED** — a `SchemaMismatch` or `UnknownKinds` record: the bytes are present, countable, and deliberately not interpreted.
- **PROVABLY NOT USED** — the same, strengthened: the gate is exact equality on a constant, so non-use is a property of the code, not an accident.

Replay **cannot** establish **NOT OBSERVED**, and no mechanism makes it able
to. A request the proxy never saw leaves nothing behind; the frame is bounded
by what reached disk. Asked "did you use E₃" where E₃ was never written,
the only honest answer is *E₃ is not established as observed evidence* — never
*E₃ was not used*.

**The distinction is preserved and the limit is principled.** It is also not
new: Codd proposed two nulls — missing-but-applicable versus
missing-and-inapplicable — and the ISO standard declined it; AAPOR's survey
dispositions separate eligible non-response, ineligible, and unknown
eligibility. The concept is forty years old (§21).

---

## 10. Qualifier substitution

Same evidence, same transformation, same producer, same numeric output, only
the qualifier changes. **SP-01 is this defect, already in the register.** The
gate's PASS branch and FAIL branch printed the same re-billed figure and only
the FAIL branch stated the exclusions.

**MEASURED: detection depends entirely on where the qualifier lives.**

| defect | qualifier carrier | reconstructable |
|---|---|---|
| C035 | `pricedRequests` / `unpricedRequests`, JSON | **yes** |
| C037 | `unpricedRebilledTokens`, JSON | **yes** |
| Q01 | `unpriced`, JSON | **yes** |
| C036 | **none** — the parameter is the row's `model` string, which is one string beside figures derived from records that each carry their own | only via the implied-rate inversion of §5 |
| SP-01 | `rebilledFigureCaveats() []string` — prose, rendered, in no JSON | **no** |
| V1 | `BeforeCounted`/`AfterCounted` on an unexported struct, rendered | **no** from the document |

**Three of six machine-readable, two prose-only, one with no qualifier field at
all.** Qualifier binding is a correctness mechanism exactly where the qualifier
is a field, and is metadata-at-best where it is a sentence. Phase 10's own
either/or resolves as: **both, per site.**

---

## 11. Parameter substitution — the decisive negative result

Phase 11 asked for `V = f(E,P₁)` and `V′ = f(E,P₂)` with `P₁ ≠ P₂` and
`V == V′` numerically, and said that if reconstruction still detects it, that
is potentially significant.

**It does not, and the room characterised exactly when.**

A blind spot exists iff the substituted parameter lies strictly inside the
convex hull of the per-element parameters. Solving
`lead × (d₁+d₂) = r₁d₁ + r₂d₂` gives `d₁/(d₁+d₂) = (lead−r₂)/(r₁−r₂)`, which
has a solution in range exactly when `min(r₁,r₂) < lead < max(r₁,r₂)`.

The production price table makes this realizable: opus-5 $5.00, **sonnet-5
$2.00**, haiku-4-5 $1.00, 3-5-haiku $0.80. Choosing a lead of sonnet-5 and
deficits of 20,000 at $5.00 and 50,000 at $0.80:

```
record-local  : 5.00×20000/10⁶ + 0.80×50000/10⁶ = $0.140000
lead-rate     : 2.00×70000/10⁶                  = $0.140000
delta                                             $0.000000000000
```

**MEASURED on the real binary, under both the defect and the repair:**

```
reconstructed: 2 breaks, 70000 tok (0 unpriceable), hull $0.80..$5.00/MTok, exact $0.140000
declared     : model=claude-sonnet-5 $0.140000 over 70000 tok (0 excluded)
implied parameter $2.0000/MTok   lead-record rate $2.00/MTok
T1=true  T2=true  T3=true  T4=true
ADMISSIBLE: true
```

Identical document from the defect and the repair. All four checks pass in
both. T5 fires — implied equals lead — but T5 fires on every corpus where the
lead's rate happens to be the weighted mean, so it is a hint, not a finding.

**Independent reconstruction is SOUND and INCOMPLETE, and the incompleteness
region is exactly characterised: the substituted parameter lies inside the
convex hull of the per-element parameters.**

This is not a gap to be engineered away. The two computations are
*extensionally equal* on that corpus: there is no information in the output or
in the evidence that distinguishes them, because there is no difference in the
output. Only the code distinguishes them.

**This forecloses the strong form of the room's question.** Replay cannot
"mechanically prevent a result from being represented as differently
parameterised than the evidence establishes", because on an identifiable class
of corpora the two representations are the same result. The honest claim is
detection on a characterised subset, not prevention.

---

## 12. Binding-drop experiment

The previous room proved a behavioural oracle discriminates on **one** type
(`surfaceBurn`) against **one** planted control, and named generalising it as
the single next experiment. Done here.

**The structural definition, deliberately name-free**, because the shipped
`RPL-C021` guard is a banned-substring lint on identifiers that an
adversarially named reducer walks straight past:

> A type is **figure-bearing-and-scoped** iff it declares at least one
> `float64` field and at least one `bool` field. A function is a **binding
> drop** iff it consumes two or more values of such a type and every one of
> its results is a bare numeric.

**MEASURED.** Over the whole module, excluding `_test.go`:

```
FIGURE-BEARING-AND-SCOPED TYPES: 31   (15 packages)
BINDING-DROP SITES: 2
  peakRebilledTokens   consumes costUnit  (slice)  cmd/replay/sharepng.go
  meanSeen             consumes sample    (slice)  internal/advisor/advisor.go
```

**Negative control passed:** a planted `consolidate(xs []zzScoped) float64`,
whose name contains none of the banned substrings, was written to a real `.go`
file, caught, and removed.

**An oracle defect, found and fixed mid-room.** The first version keyed scoped
types by bare name across the whole module. Two packages each declare a
`sample`; only `internal/advisor`'s has a bool. The oracle attributed one
package's bool to the other's struct and reported a third site,
`weightedSpread` in `internal/analysis/fit.go`, that does not exist. Keyed by
package directory, it disappears. Recorded because an oracle that invents a
finding is the failure mode this campaign exists to catch.

**Both surviving hits were inspected and both are FALSE POSITIVES:**

- `meanSeen(xs []sample) (float64, int)` — the `int` **is** the population; the caller takes `before, nBefore`. The oracle cannot tell a returned population from a returned unrelated number. **The repair and the defect have the same type.**
- `peakRebilledTokens(units []costUnit) int` — returns a max over `RebilledTokens`, a quantity that is complete by construction ("tokens first and unconditionally") and needs no pricing qualifier. The `bool` on `costUnit` qualifies the dollars, not these tokens.

**Both reported sites are false positives: 0 true positives, over 31 scoped types in 15 packages.** The
previous room's single success was a *constructed* positive, not a found one.

**And the decisive measurement: the oracle is blind to the defect.** Run under
the C036 mutation, the census is unchanged — same 31 types, same 2 sites.
All six defects are intra-function: C036 changed an expression inside a loop,
Q01 two statements inside a loop, V1 a predicate inside a closure. None
crosses a function boundary, so no signature-level oracle sees any of them.

Phase 12 asks for the minimum necessary mechanism. **MEASURED: it is not
static analysis at function boundaries.** Of the listed candidates, only
type-level preservation — making bare extraction *unrepresentable*, as
information-flow labels and refinement types do — would reach an intra-function
site, and that is a whole-program rewrite of every figure-bearing type, costed
nowhere and justified by nothing measured here.

---

## 13. Composition

**INFERRED** from the structure, with one **MEASURED** anchor.

| case | admissible result |
|---|---|
| compatible evidence, disjoint | the union, with the union's qualifier |
| incompatible evidence | nothing; the operation is not defined |
| overlapping evidence | nothing, unless the overlap is identified and deduplicated |
| **unknown overlap** | **nothing.** Replay has this case by name |
| compatible qualifiers | the combined result under the shared qualifier |
| incompatible qualifiers | nothing |
| missing qualifier | the degenerate widening: the reader supplies "everything" |
| substituted qualifier | §10: detectable iff the qualifier is a field |
| stale evidence | §18 |
| evidence arriving after the result | §18 |

**MEASURED anchor.** `replay cost` publishes `unjoinableRequests` beside
`duplicatedRequests` and `totalRequests`, and `transcript.Request.IDMeasured`
exists precisely so that a locally synthesised id cannot be used as a join key:
*"two files both have a first record, so an unmeasured id is not a join key."*
Unknown-overlap composition is already refused in this repository, by a field,
for this reason.

**Can composition silently broaden a claim? Yes, and the mechanism is the
missing qualifier.** Q01 is composition: `foldSessions` combines per-lane rows
into a session row, and before the repair it took the gate flag from one lane
and the sum from all of them. The repair is the composition rule written down:
`Unpriced` is an AND over lanes, `MixedEpochs` an OR. **INFERRED: what prevents
silent broadening is not a mechanism but a stated combination rule per
qualifier**, and there is no general rule — AND and OR are both correct, for
different qualifiers, and nothing derives which from the qualifier's type.

---

## 14. Transport

What must survive for the binding to be preserved, by representation:

| representation | carries the binding? |
|---|---|
| Go struct | **yes**, while the struct is intact; lost the moment a field is read out (§12) |
| JSON | **partially.** `omitempty` on `unpricedRebilledTokens` means "nothing excluded" and absence of the key is load-bearing — a receiver that drops unknown keys cannot tell |
| ledger JSONL | **yes by design.** `Record.Correlation` is "omitted rather than written false when unknown: a record from a build that never took this reading must not read as one that measured no overlap" |
| OTel-like event | **no.** A metric point carries labels of bounded cardinality; a per-element parameter set is not a label |
| agent message | **NOT MEASURED** |
| database row | **no**, unless the schema has a column per qualifier |

**Deliberate field deletion.** Remove `unpricedRebilledTokens` from the
document and the receiver sees a figure with no stated exclusion — which, under
`omitempty`, is byte-identical to a figure with nothing excluded. **The
receiving side cannot distinguish binding-preserved from binding-silently-
weakened.** That is not a Replay bug; it is what `omitempty` means, and it is
why `Unpriced` is serialized at all ("a warm run must reach the same disclosure
as a cold one").

Phase 14 says: if the answer is "store provenance metadata", search prior art.
The answer is that, and §21 is that search. It comes back with XBRL contexts,
which are exactly this and are in production worldwide.

---

## 15. SQL superpower test

Given an ideal relational database with complete lineage and event history:

| capability | SQL? |
|---|---|
| 1. independent binding reconstruction | **yes** — a recursive query over lineage plus the source tables |
| 2. producer-vs-reconstruction comparison | **yes** — a constraint or trigger comparing published to recomputed |
| 3. qualifier substitution detection | **yes, iff the qualifier is a column.** §10: in two of six it is prose |
| 4. parameter substitution detection | **no, within the hull.** §11 is information-theoretic, not expressive-power |
| 5. scope widening detection | **yes** — compare the declared population column to `COUNT(*)` over the frame |
| 6. absence distinction | **conditionally.** Yes if unparseable rows are materialised with a parse-status column. **No** if the loader drops them, which is what every loader does by default and what Replay's schema gate does by design |
| 7. composition safety | **yes** for identified overlap; **no** for unknown overlap, which is not a database property |

**Verdict: DATABASE-COMPLETE with two exceptions, and only one of them is
about SQL.**

Item 4 is not a SQL limitation. No system can distinguish two extensionally
equal computations from their outputs.

Item 6 is the real one, and it is sharper than "SQL cannot". SQL *can*, if
someone materialises the rows that failed to load. The failure is that the
denominator comes from the table and the table contains only what loaded, so
`COUNT(*)` silently means "readable" while reading as "all". **A constraint
cannot catch this, because it is a claim about what is not in the database.**
Replay's own `SchemaMismatch` is this exact case: intact bytes, deliberately
not interpreted, no row.

**Nothing uniquely Replay-specific survives item 1, 2, 3, 5 or 7.**

---

## 16. Language superpower test

**Could the producer have been prevented at compile time?**

**Yes, for four of six, HYPOTHESIS.** Refinement types (LiquidHaskell, F*) can
carry the population a value is valid over and refuse a combination of
differently-scoped values unless the rule is stated. Units-of-measure types
(F#, Kennedy) reject dimensionally unsound arithmetic. Information-flow labels
(Jif, FlowCaml, LIO) are the closest structural analogue to the whole
hypothesis: a label travels with every value and can be removed only through an
explicit, checked, authorised `declassify` — "a qualifier that cannot be
silently dropped, and whose removal is an auditable act" is the one-line
summary of both.

**Not for SP-01 or V1**, whose qualifier is prose: no type system constrains a
sentence.

**Could the compiler verify the claim against external observed reality? No,
and this is the sharp distinction Phase 16 demands be made explicit.**

- **Preventing bad computation** is static, white-box, inside the producer, and about the *form* of the derivation. Four of six defects are reachable this way.
- **Reconstructing what actually happened** is dynamic, black-box, outside the producer, and about *which evidence existed*. §8's eligible-but-absent is reachable only this way: no type system knows that a file on disk failed to parse.

**Neither subsumes the other, and this room measured both halves.** §12 showed
static analysis at function boundaries catches none of the six. §5 showed
reconstruction catches the mutation but adds nothing over the producer's own
tests. §11 showed reconstruction has a characterised blind spot that nothing
closes.

---

## 17. Security

| attack | caught? |
|---|---|
| **claim laundering** — B republishes A's qualified claim without the qualifier | **no.** §14: under `omitempty` a dropped exclusion key is byte-identical to no exclusions |
| **evidence laundering** — B claims to have observed what A observed | **partially, MEASURED.** `Record.RequestID` is the provider's, *"never synthesised here, so a reader can tell a provider id from a locally invented one"*, and `IDMeasured` carries that distinction into the request. Observation is attributable to the proxy that recorded it; it is not attributable to a *claimant*, because nothing signs anything |
| **parameter laundering** — B uses A's result with B's parameter | **yes outside the hull, no inside it.** §11 |
| **identity laundering** — A's evidence attributed to B | **NOT MEASURED**, and the register already says so: RPL-C020's own "why this result" records that join safety *"depends on provider request-id uniqueness, which Replay does not independently establish"* |
| **scope laundering** — A claims a subset, B republishes as the population | **yes from raw evidence, no from the document.** §8: the frame is enumerable from the bytes and absent from the output |

**The room notes that four of five attacks assume a second party, and Replay
has no second party.** There is no signature, no commitment to the evidence
corpus, and no claimant identity. Every "caught" above is caught by *recounting
the bytes*, which requires holding the bytes. Against an adversary who controls
the bytes, none of it holds. Authenticated data structures solve exactly this
and Replay does not implement them (§21).

---

## 18. Temporal reconstruction

At T1, E₁ is available and E₂ is not. At T2, E₂ arrives.

**A claim made at T1 is not retrospectively supported by E₂, and must not
become so silently.** The correct statements are two, not one: *not established
at T1* and *established by T2*, both true, neither replacing the other.

**MEASURED: Replay's version gate forecloses the interesting case.**
`SchemaVersion` is exact equality and bumping it "DISCARDS EVERY EXISTING
LEDGER". There is no migration path, so evidence that arrives under a new
schema does not join a corpus read under the old one — it is counted as
`SchemaMismatch` and excluded. The T1→T2 transition cannot occur across a
schema boundary by construction.

Within a schema, late-arriving evidence is an append to a JSONL file, and
re-running `replay cost` reads the longer file. That is **ordinary event
sourcing with a re-derived read model, and the room says so plainly.** Nothing
stronger emerged. The only non-ordinary element is that the two claims are
*both* retained rather than the later overwriting the earlier, and that is a
reporting convention, not a mechanism.

---

## 19. Cross-domain test

The mechanism without `session`, `token`, `agent`, `AI`, `cost` or `lane`:

> A derived value and anything that qualifies, gates, scales or bounds it must
> originate from the same evidence element, or from an explicitly stated rule
> for combining elements; and the population the value is published over must
> be the population the evidence enumerates, including the elements that were
> eligible and could not be read.

It expresses without the vocabulary, so it is not domain-specific on that test.

| domain | instance |
|---|---|
| **evaluation** | a score over a benchmark whose unrunnable cases were dropped from the denominator rather than counted |
| **policy** | a limit compared against consumption measured over a shorter window than the limit names |
| **verification** | a pass rate over the checks that executed, published as a rate over the checks that exist |
| **accounting** | a total over priced line items, published as a total — **MEASURED**, this is C035 |
| **scientific computation** | a measurement published without the calibration condition under which it is valid |

**INFERRED, with one measured instance.** Four of the five are constructed, not
observed. The previous room's "four non-cost domains with a measured instance
each" was stated more strongly than the evidence supports, and this room
withdraws it: **one measured instance, in accounting, which is where Replay
lives.**

---

## 20. Five capabilities

| capability | verdict | mechanism |
|---|---|---|
| **durable scratch** | **UNRELATED** | durability is about surviving a process boundary; nothing here is about persistence |
| **work continuity** | **NOT MEASURED** | a resumed task's evidence spans two runs, so its population spans two frames. Plausible as a widening site; no fixture was built |
| **quota titration** | **SUPPORTED** | the only one with a concrete mechanism. A quota decision is a *limit* compared against a *consumption figure*. §8 measured that `replay cost` omits five absence states, so a consumption figure computed from readable records understates consumption, and a titration gate reading it under-throttles. The qualifier that must bind is the frame the consumption was measured over — and `replay cost` does not publish it |
| **inter-agent communication** | **PLAUSIBLE** | §17's claim laundering is exactly the inter-agent case and §14 shows a dropped `omitempty` key is undetectable. But no second party exists in Replay today, so the case cannot be measured, only constructed |
| **realtime** | **NOT MEASURED** | §18 found ordinary event sourcing; nothing about latency entered any experiment |

**No forced unification.** One SUPPORTED, one PLAUSIBLE, two NOT MEASURED, one
UNRELATED.

---

## 21. Prior art

Searched by a dedicated agent against primary sources (OpenAlex, Crossref,
arXiv, specification documents). Its findings are reported as its findings;
where this room could not independently confirm a verbatim quotation, it says
so.

| system | reconstructs | trusts | enforces | cannot establish |
|---|---|---|---|---|
| **XBRL 2.1** (2003, errata 2013) | arithmetic consistency of a total against its components | the producer's choice of context | that a component binds to a total only if context-equal and unit-equal | that the context matches reality; anything about unreported facts |
| **Provenance semirings** (Green/Karvounarakis/Tannen, PODS 2007) | the derivation polynomial | the producer's engine | propagation through positive relational algebra | any constraint on binding; 0 conflates never-observed with eligible-and-skipped |
| **Provenance for aggregates** (Amsterdamer/Deutch/Tannen, PODS 2011) | value-level annotations inside tuples | the same | that an aggregate's annotation combines the contributing *values'* annotations | validity flags; absence |
| **Completeness over incomplete DBs** (Razniewski/Nutt, VLDB 2011; SIGMOD 2015) | whether a query answer is provably complete | the asserted table-completeness statements | TC–QC entailment, decidably | the truth of the statements; unreadable vs nonexistent |
| **Why-not provenance** (Lee/Köhler/Ludäscher/Glavic, ICDE 2017) | failed derivations | the loaded instance | computed explanations for missing answers | anything about data that never loaded |
| **Proof-carrying data** (Chiesa/Tromer 2010; Necula 1997; Valiant 2008) | whatever the compliance predicate states | nothing — that is the point | the predicate, with zero trust in the producer | anything the predicate omits; the prover supplies the witness |
| **Authenticated data structures** (Devanbu et al.; Pang/Tan 2005; ADSNARK 2015) | correctness *and completeness* of a result set against a commitment | the digest | that a result set came from the committed corpus | qualifier semantics; anything outside the commitment |
| **Jif / FlowCaml / LIO** | nothing | the compiler | that a label cannot be dropped except by an explicit, authorised declassify | anything about population; anything external |
| **Units of measure; refinement types** | nothing | the compiler | dimensional soundness; stated predicates | that the predicate matches reality |
| **W3C PROV-O** (2013) | nothing | — | nothing; it is an ontology | **no construct for "eligible but not used"** |
| **Prometheus staleness / `absent()`** | nothing | the scrape | a two-state stale marker | "never scraped" vs "absent" — collapsed |
| **Codd's two nulls; certain answers** | — | — | nothing implemented; ISO declined the proposal | — |
| **AAPOR Standard Definitions** (10th ed., 2023) | — | the fieldworker's disposition code | an arithmetic convention and a spreadsheet | it is a discipline, not a mechanism |

**Caveat on the load-bearing item.** The agent reports XBRL 2.1 §5.2.5.2
requires a contributing item be *C-Equal* (same entity, period and dimensional
scenario) and *U-Equal* (same unit) with the summation item, and that the spec
states calculation checks "work exclusively on the information that is
explicitly provided in the instance". This room fetched the specification URL
and the retrieved excerpt **did not include** the normative text of §5.2.5.2;
two secondary sources returned 404. **The XBRL claim is NOT INDEPENDENTLY
VERIFIED in this session.** It is reported because it is the single strongest
prior-art item and because the structural fact it rests on — that every XBRL
fact carries a context and a unit, and that calculation consistency is
validated — is not in dispute.

**Verdict on each part of the hypothesis:**

- **Qualifier binding is not novel, as a concept or as a mechanism.** XBRL enforces it in production worldwide; information-flow typing enforces the non-droppability half; units-of-measure enforces the incommensurability half. Dead.
- **Independent reconstruction is not novel.** Verifiable query results and proof-carrying data do it cryptographically and better, with a commitment; recomputing a provenance polynomial does it in the database. What this room could not find anywhere is the specific comparison *scope containment* — detecting that a published claim is **broader** than the binding the evidence supports; everyone implements value equality or set membership. **NOT MEASURED whether that is substantive or merely unstated.**
- **Eligible-but-absent survives.** PROV-O has no construct for it; semirings conflate it with never-observed; XBRL disclaims it; Bazel cannot detect an available-but-undeclared input; Prometheus collapses it. The two places it is genuinely addressed — Razniewski and Nutt's completeness entailment, and survey methodology's disposition codes — are partial and neither is droppable into a pipeline.

---

## 22. Minimum mechanism

Stated because something survived §21, and scoped to exactly what survived.

**INVARIANT.** A published figure must name the frame it was computed over,
where the frame includes every element that was eligible and could not be read.

**REPRESENTATION.** One additional integer beside every published count:
`eligibleUnread`. Not a graph, not a label, not a wrapper. Replay already
computes five such states in `transcript.Session`; this is a field in the
document, not a new structure.

**RECONSTRUCTOR.** Enumerate the frame from the bytes: count lines, count the
ones that fail to interpret, report both. §8 measured this working.

**COMPARATOR.** `declaredFrame == readable + eligibleUnread`, and
`declaredPopulation ≤ declaredFrame`.

**ENFORCEMENT.** None available beyond the comparator. §11 proves prevention
is unreachable for parameter substitution, and §12 measured that static
analysis catches none of the six defects.

**COMPOSITION RULE.** `eligibleUnread` is additive across frames; a composed
figure whose operands have unknown overlap has no frame and must not be
published (§13).

**FAILURE STATE.** `declaredFrame > readable + eligibleUnread` means elements
were lost without being counted. `declaredPopulation > declaredFrame` means the
claim is broader than the evidence enumerates.

That is the whole mechanism. It is a reporting discipline with a comparator,
and it is honestly smaller than what the room was convened to find.

---

## 23. Prototype

Built, run, and removed. Four requirements, all met:

- **RED first.** The reconstructor was written and run against the shipped binary before any mutation; the mixed-rate corpus was chosen *because* its implied parameter ($2.90) is far from the lead's ($0.80), so a pass is not something a wrong-element producer could also earn.
- **Negative control.** The C036 defect reintroduced into `cmd/replay/cost.go`: `ADMISSIBLE: false`, T4 and T5 both firing. **The test fails when the declaration is blindly trusted** — a comparator that read `rebilledUsd` and stopped would pass.
- **Positive control.** The shipped binary: `ADMISSIBLE: true`.
- **Reconstruction removed.** Delete the raw-evidence walk and `admissible()` has nothing to compare against; the four checks degenerate to comparing the document with itself.

**Adversarial mutation of the oracle itself** (§12): the first binding-drop
oracle reported a site that does not exist, from a cross-package name
collision. Found by inspecting every hit rather than counting them.

**Scaffolding removed** (§28). The room does **not** conclude it belongs in
production: §5 measured that it adds no detection power over the tests already
shipped.

---

## 24. Six-defect proof

| | DEFECT | EVIDENCE | DECLARED BINDING | RECONSTRUCTED BINDING | MISMATCH | DETECTION |
|---|---|---|---|---|---|---|
| **C035** | total covered the priced subset | per-record priceability | a total, unqualified | priced + unpriced counts | population dropped | **yes** — `pricedRequests`/`unpricedRequests` recomputable |
| **C036** | each break at the lead's rate | each break's own model and timestamp | one `model` string per row | per-element rate hull + exact sum | parameter from the wrong element | **yes, MEASURED**; **no inside the hull, MEASURED** |
| **C037** | dollars and tokens over different populations | which breaks priced | one token figure, one dollar figure | represented and unrepresented halves | two populations, one result | **yes, MEASURED** via T2 |
| **SP-01** | PASS stated no exclusions its FAIL stated | the same `unpriced`/`unreadable` counts both branches read | PASS: nothing | the exclusions, both branches | qualifier dropped on one branch | **NOT RECONSTRUCTABLE from the document** — `[]string` prose, in no JSON |
| **V1** | unpriceable rows counted as $0 | `PricedRequests`/`UnpricedRequests` per row | a median over all rows | a median over eligible rows | ineligible elements admitted | **partially** — the counts exist on an unexported struct, render-only |
| **Q01** | gate flag from whichever lane sorted first | per-lane priceability | one `unpriced` per session | AND over lanes | qualifier from the wrong element | **yes** — `unpriced` is a JSON field, recomputable per lane |

**One marked NOT RECONSTRUCTABLE and one partial.** Not inferred, not
force-fit: SP-01's qualifier is a `[]string` returned by
`rebilledFigureCaveats` and consumed only by two `Fprintf` sites. There is no
JSON key. A consumer holding the document cannot recover it.

---

## 25. Strongest alternative explanation

| claim | verdict |
|---|---|
| **just provenance** | **ACCEPT for representation, REFUTE for the absence half.** Amsterdamer et al. represent C036 exactly. But semiring 0 conflates never-observed with eligible-and-skipped, and PROV-O has no construct for eligible-but-not-used. Provenance records what was read; §8's defect is about what was not |
| **just database constraints** | **ACCEPT, with one exception.** §15: five of seven capabilities are database-complete. The exception is item 6, and it is not expressive power — a constraint cannot range over rows the loader never created |
| **just type safety** | **REFUTE, MEASURED.** §12: the structural oracle produced 0 true positives over 31 types, and is blind to C036 under mutation. All six defects are intra-function. §16 concedes refinement types *could* prevent four of six, but that is a whole-program rewrite with no measured justification |
| **just static analysis** | **REFUTE, MEASURED.** Same measurement. The shipped `RPL-C021` guard is itself a name lint that the previous room proved an adversarially named reducer passes |
| **just reconciliation** | **ACCEPT.** T4 — compare the published figure to the recomputed one — is double-entry reconciliation, and it is the only one of the four checks that caught the mutation. The room should say this plainly: **the load-bearing mechanism is recomputation** |
| **just observability** | **REFUTE.** §21: Prometheus collapses "never scraped" into "absent" and `absent()` cannot distinguish them. Observability has the weakest form of the absence distinction found anywhere |
| **just event sourcing** | **ACCEPT for §18.** Re-deriving a read model from an append-only log is exactly what re-running `replay cost` over a longer JSONL file is. Nothing stronger emerged |
| **just data quality** | **ACCEPT in substance.** "The denominator must come from the same enumeration as the numerator" is a data-quality rule. That it is unfashionable does not make it novel |
| **just audit logging** | **REFUTE.** An audit log records actions; none of the six defects is an action. C036 is a correct action with a wrong parameter |

**Four accepted outright, one accepted with an exception, four refuted.** The
accepted ones include reconciliation, which is where the only check that
actually fired lives.

---

## 26. Final falsification

Three things the room set out to establish, and what happened to each.

1. **That reconstruction detects wrong-element binding.** **Established, MEASURED, and bounded.** It detects it outside the convex hull of the per-element parameters and provably cannot inside it (§11). "Mechanically prevent" is unreachable.

2. **That reconstruction adds something the producer's own tests do not.** **Falsified, MEASURED.** The same mutation turns 7 shipped C036 tests and 4 C037 tests red (§5). The only addition is trust transfer, which §21 shows authenticated data structures already do with a commitment Replay lacks.

3. **That the six defects share a machine-checkable invariant.** **Falsified, MEASURED.** §12: the generalised oracle finds 0 true positives in 31 types and is blind to C036. §24: one of six is not reconstructable at all and one is partial. The defects share a *description*; they do not share a *check*.

What survives all three: **§8's eligible-but-absent**, which is a real gap in
Replay's published output, is measured, is not covered by any prior-art system
found, and is also forty years old as a concept.

---

## 27. FINAL VERDICT

### **C — GENERAL ENGINEERING PRIMITIVE**

A reusable technical invariant exists across domains, and existing engineering
adequately explains it.

**Why not D or E.** The distinct part of the room's claim was *independent
reconstruction + comparison + enforcement*. Enforcement is unreachable (§11,
information-theoretic, not an implementation gap). Independent reconstruction
is measured to add no detection power over the producer's own tests (§5), and
its trust-transfer value is better served by authenticated data structures,
which Replay does not implement. The comparator that actually fires is
recomputation, which is double-entry reconciliation (§25).

**Why not A.** Reconstruction demonstrably works. §5 is a real, repeated,
controlled measurement with a mutation and a revert.

**Why not B.** The surviving invariant is not domain-specific; §19 states it
without a single Replay word and it instantiates in evaluation, policy,
verification, accounting and scientific computation.

**The one component with an unoccupied prior-art position**, recorded without
a novelty claim: a provenance alphabet in which *eligible-but-unread* is a
distinct symbol, so a denominator computed from lineage is forced to account
for it. Every lineage standard checked lacks it. Whether that is substantive
or merely unstated is **NOT MEASURED**, and the patent arm remains uncleared.
The room did not select E, and notes that the honest reason is not that the
gap is absent but that the gap is **small, old, and already half-built inside
Replay's own type system**.

---

## 28. Single next experiment

**Publish `transcript.Session`'s five absence states at record granularity in
`replay cost --json`, and measure how many appear in a real corpus.**

This is the only surviving item and it is one afternoon. §8 measured that
`Skipped`, `UnknownKinds`, `ProviderFailures` and `SchemaMismatch` are computed
and never published, while `unreadable` is published as a *file* count. The
experiment is to publish them and run against `~/.claude/projects`.

- If all five are **zero** on a real corpus, the gap is theoretical and the room's surviving component dies with it. That result would close this line permanently.
- If any is **non-zero**, then a published figure in circulation has a denominator smaller than its frame, and §20's quota-titration mechanism becomes measurable rather than SUPPORTED-by-argument.

It needs no architecture, no graph, no credits, and it is the same shape as
the measurement that settled Q01: incidence first, mechanism second.

---

## Closing state

**MEASURED.** `go test ./...` with the probes present: 37 ok, 0 FAIL. Probes
removed and re-run: 37 ok, 0 FAIL, no `zz_` file remaining under the module.
`cmd/replay/cost.go` byte-identical to its pre-room backup. No production code
changed. No other-session file touched. Nothing staged, nothing committed.
