# Claim register, complete

**Generated from `internal/claims` at build time. Do not edit by hand:
regenerate it, or it will drift from the code it describes.**

| Result | Count |
|---|---|
| BOUNDED | 13 |
| DELIBERATE_NON_CLAIM | 1 |
| ESTABLISHED | 2 |
| NOT_MEASURED | 3 |
| NO_ENDPOINT | 3 |
| REFUTED | 4 |
| UNRESOLVED | 1 |
| **Total** | **27** |

---

## RPL-C001

> Replay reads agent transcripts already on disk and attributes prompt-cache behaviour per turn, without provider cooperation.

| | |
|---|---|
| **Result** | **BOUNDED** |
| Evidence basis | RECONSTRUCTED |
| Deciding layer | surface.Class |
| Scope | Claude Code, Codex, Grok, Ollama transcripts on local disk. Not Cursor, AnythingLLM or OpenClaw, which have no readable usage field. |
| Oracle | fixture corpora under internal/surface/testdata classified by an audit performed independently of the classifier |
| Asserted at | README.md:13, cmd/replay/main.go:639 |

- **Establishes:**
  - a per-turn record can be reconstructed from local artifacts alone
- **Does NOT establish:**
  - that the reconstruction matches the provider's own accounting
  - that any surface other than the four named ones can be read
  - anything about surfaces whose usage field is absent or always zero
- **Assumptions Replay does not verify:** _none_
- **Known gaps:**
  - no shared conformance suite spans the internal/transcript parsers; each has its own fixtures
- **Positive control:** four surfaces classify as the independent audit found
- **Negative control:** three surfaces (Cursor, AnythingLLM, OpenClaw) are refused outright
- **Insufficient-evidence control:** a corpus with zero records returns ClassUndetermined
- **Tests:**
  - TestFixtureCorporaClassifyAsTheAuditFound
  - TestE2E_Diff
  - TestE2E_Cost

**Why this result:** Holds for four surfaces and is explicitly refused for three others. The bound is the claim.

---

## RPL-C004

> Every figure Replay prints carries a truth tier: measured, estimated or structural.

| | |
|---|---|
| **Result** | **REFUTED** |
| Evidence basis | NOT_APPLICABLE |
| Deciding layer | none. No tier type, field or enum exists: the vocabulary is free string literals at 15+ sites across 11 packages |
| Scope | printed output of the shipped binary, human and machine-readable |
| Oracle | the real cost report run over a fixture corpus and read as text, plus the parsed JSON document |
| Asserted at | README.md:293, docs/adr/0002-replay-engine-and-truth-tiers.md:12 |

- **Establishes:** _none_
- **Does NOT establish:**
  - the universal form. `replay cost --json` emits dollar figures under seven top-level keys and not one of them names a tier, a provenance or a basis. A machine consumer receives money with no way to tell how it was obtained
  - that the tier vocabulary is the mechanism anywhere. The human cost report labels its dollars by naming a dated price basis and uses none of the three tier words to do it
  - ASSUMPTION, not established: that a tier is conveyed by those three words. A report conveying provenance by other wording is understated by this measurement, and the cost report turned out to be exactly that case
- **Assumptions Replay does not verify:**
  - ASSUMPTION: a tier is conveyed by the words measured, estimated or structural. A report conveying provenance by other wording is understated by this measurement, and the cost report turned out to be exactly that case
- **Known gaps:**
  - the printed surface was not enumerated. One report and one JSON document were measured; burn, advise, route, diff, errors, warnings and the TUI were not
  - README names three tiers and ADR-0002 names two and never contains the word structural. The documents disagree and this campaign did not correct either
- **Positive control:** the tier detector fires on a labelled line
- **Negative control:** it does not fire on an unlabelled line, and the figure detector does not fire on a bare integer
- **Insufficient-evidence control:** not applicable. The claim is about presence, not about a measurement
- **Tests:**
  - TestC004_TheCostReportStatesHowItsDollarsWereObtained
  - TestC004_TheTierVocabularyIsNotHowTheCostReportLabelsMoney
  - TestC004_TheTierDetectorDiscriminates
  - TestC004_TheJSONSurfaceCarriesNoTierField

**Why this result:** Split into RPL-C027, RPL-C028 and RPL-C029, which carry three different proof obligations. Falsified as worded, by the machine-readable surface. Separately the three-word vocabulary is not how the human report labels money, and the two documents asserting the claim disagree on what the words are: README names three tiers, ADR-0002 is titled two and never contains the word structural. What is true is registered as RPL-C025 rather than read back into this one.

---

## RPL-C025

> The human cost report states the basis of its dollar figures, with a date, above the figures themselves.

| | |
|---|---|
| **Result** | **BOUNDED** |
| Evidence basis | RECONSTRUCTED |
| Deciding layer | none |
| Scope | `replay cost` human output. NOT the JSON surface, and not established for any other report |
| Oracle | the rendered report inspected as text, with the header isolated from the figures |
| Asserted at | cmd/replay/cost.go:1057 |

- **Establishes:**
  - a reader meeting the money also meets the basis it was computed on and the date that basis was read
  - the report says plainly that on a subscription seat the dollars are list price for someone billed per token
- **Does NOT establish:**
  - anything about reports other than `replay cost`. The printed surface was not enumerated
  - that the basis is correct, only that it is stated
- **Assumptions Replay does not verify:** _none_
- **Known gaps:**
  - one report only. Generality across the printed surface is unmeasured
- **Positive control:** the header above the first dollar figure names a price basis and a date
- **Negative control:** removing the basis from cmd/replay/cost.go kills the test; removing the date kills it separately
- **Insufficient-evidence control:** a corpus with no priced request reports unpriced rather than a figure
- **Tests:**
  - TestC004_TheCostReportStatesHowItsDollarsWereObtained

**Why this result:** Mutation-proven at two points in cmd/replay/cost.go: removing the price basis from the header kills it, and removing the date kills it separately.

---

## RPL-C011

> The 'measured' truth tier means the provider reported the USAGE, not that the provider reported the CHARGE.

| | |
|---|---|
| **Result** | **BOUNDED** |
| Evidence basis | RECONSTRUCTED |
| Deciding layer | none. No type carries a tier on the figure |
| Scope | all dollar figures, including those obtained through replay serve |
| Oracle | independent reimplementation of the cost arithmetic in the test, not an import of CostLegsUSD |
| Asserted at | README.md:299 |

- **Establishes:**
  - token counts in the measured tier came from the provider's own counters
- **Does NOT establish:**
  - that the dollar figure beside them is what the provider billed
  - that the price table matched the account's negotiated rate
  - that the figure is reconcilable with an invoice
  - ASSUMPTION, not established: that the compiled price table matches the rate the account was actually billed on. Replay cannot check this, and the figure is wrong by exactly the discount where it does not hold
- **Assumptions Replay does not verify:**
  - ASSUMPTION: the compiled price table matches the rate the account was actually billed on. Replay cannot check this and the figure is wrong by exactly the discount if it does not.
- **Known gaps:** _none_
- **Positive control:** production arithmetic agrees with a longhand reimplementation
- **Negative control:** identical provider usage prices at $0.020400 on list and $0.016320 negotiated
- **Insufficient-evidence control:** a price table with no matching model returns not-found rather than a figure
- **Tests:**
  - TestC011_IndependentArithmeticAgreesWithProduction
  - TestC011_MeasuredUsageTimesLocalPriceIsNotABill
  - TestC011_ThePriceBasisDeclaresItsOwnProvenanceAndAge
  - TestC012_NoProviderBillingEndpointExists

**Why this result:** Usage is provider-reported. Price is a local table. Their product is reconstructed, and the word 'measured' covers only the first factor.

---

## RPL-C012

> Replay can establish what a provider actually billed.

| | |
|---|---|
| **Result** | **NO_ENDPOINT** |
| Evidence basis | NOT_APPLICABLE |
| Deciding layer | none |
| Scope | all providers |
| Oracle | static scan of every outbound HTTP destination in non-test code |
| Asserted at | _nowhere. This is a non-claim or an inferred boundary_ |

- **Establishes:** _none_
- **Does NOT establish:**
  - anything. No code path reaches a provider invoice, balance or usage-report endpoint
- **Assumptions Replay does not verify:** _none_
- **Known gaps:** _none_
- **Positive control:** the destination scan finds the URLs that are present
- **Negative control:** a planted /v1/invoices is detected; an ordinary /v1/messages is not
- **Insufficient-evidence control:** not applicable: the result IS the absence
- **Tests:**
  - TestC012_NoProviderBillingEndpointExists

**Why this result:** Structural, not incidental. Every dollar figure in the repository is usage times a locally held price table. The only second source, LiteLLM, is documented in-code as an observer and not an authority.

---

## RPL-C024

> Attributed prompt tokens sum back to the provider's reported prompt total, with any excess carried in a named re-billed row rather than absorbed.

| | |
|---|---|
| **Result** | **BOUNDED** |
| Evidence basis | RECONCILED |
| Deciding layer | none |
| Scope | blame and reconcile output |
| Oracle | hand-computed totals in the conservation fixtures |
| Asserted at | internal/analysis/conservation_test.go:12 |

- **Establishes:**
  - no token is silently dropped or double-counted in attribution
- **Does NOT establish:**
  - that the split between buckets is correct. Conservation pins the total and says nothing about the division
- **Assumptions Replay does not verify:** _none_
- **Known gaps:**
  - conservation pins the total and says nothing about the split, as the repository's own test states
- **Positive control:** every attributed token lands in exactly one bucket
- **Negative control:** the RebillLabel row is sized exactly to the excess rather than absorbing it
- **Insufficient-evidence control:** _none. Recorded as a gap_
- **Tests:**
  - TestConserve_EveryBilledTokenLandsInExactlyOneBucket
  - TestConserve_EachLaneConservesIndependently

**Why this result:** A necessary condition that the repository's own tests already state is not sufficient.

---

## RPL-C027

> A truth vocabulary exists: a defined, agreed set of tier words.

| | |
|---|---|
| **Result** | **UNRESOLVED** |
| Evidence basis | NOT_APPLICABLE |
| Deciding layer | none. The vocabulary is not a type |
| Scope | the specification, not any output |
| Oracle | the two specification documents read directly, with a test that fails if either changes |
| Asserted at | README.md:293, docs/adr/0002-replay-engine-and-truth-tiers.md:12 |

- **Establishes:**
  - the words measured, estimated and structural appear in the product and in its documentation
- **Does NOT establish:**
  - that the set is agreed. ADR-0002 is titled two tiers of truth and names estimated and measured; README names three, adding structural; and of 28 ADRs none supersedes 0002. The authoritative vocabulary is in conflict and nothing resolves it
  - that the vocabulary is a type. There is no enum, no field and no enforcement: 15+ free string literals across 11 packages
- **Assumptions Replay does not verify:** _none_
- **Known gaps:**
  - no authoritative specification exists to measure compliance against
- **Positive control:** README names a structural tier
- **Negative control:** ADR-0002 does not, and the test fails if either document changes
- **Insufficient-evidence control:** no ADR of 28 supersedes 0002, so nothing resolves the conflict
- **Tests:**
  - TestC004_TheTierSpecificationIsInConflict

**Why this result:** The words exist and the specification disagrees with itself. A vocabulary two documents define differently is not yet a vocabulary, and this campaign declines to invent a third tier to reconcile them.

---

## RPL-C028

> Every relevant printed output carries a truth status.

| | |
|---|---|
| **Result** | **REFUTED** |
| Evidence basis | NOT_APPLICABLE |
| Deciding layer | none |
| Scope | 14 CLI surfaces sharing one callable signature, driven over a ledger fixture |
| Oracle | a text-level reference judgement over the fixture's own ground truth, which never calls Replay's classification code because none exists to call |
| Asserted at | README.md:293 |

- **Establishes:** _none_
- **Does NOT establish:**
  - the claim. `replay advise` is a CONFIRMED COUNTEREXAMPLE: it emits 10% without a status statement, isolated by removing it from the sweep, where figures fall 4 to 3 while status holds at 3. The campaign does NOT establish that it is the only such surface
  - anything about 5 of the 14 surfaces. budget, ceiling, prefix and route did not run against this fixture and were not measured
  - an exhaustive inventory of JSON figures. Two separate facts: DIRECT STRUCTURAL INSPECTION established that `replay cost --json` attaches no tier, provenance or basis field to its numerical values, and SEPARATELY the figure detector under-detected JSON because it looks for a dollar sign while JSON emits bare numbers such as totalUsd. The detector limitation is campaign evidence, not a product pass, and the inventory row for that surface must not be read as one
  - ASSUMPTION, not established: that a status is conveyed by a tier word, a dated basis, a list-price statement or a refusal. The detector is deliberately generous, so a surface it marks as failing has really failed
- **Assumptions Replay does not verify:**
  - ASSUMPTION: a status is conveyed by a tier word, a dated basis, a list-price statement or a refusal. Deliberately generous, so a surface marked failing has really failed
- **Known gaps:**
  - budget, ceiling, prefix and route did not run against this fixture
  - the JSON surface is UNDER-MEASURED: the figure detector looks for a dollar sign and JSON emits bare numbers
- **Positive control:** removing advise from the sweep drops figures 4 to 3 while status holds at 3, isolating it as the sole offender
- **Negative control:** neutralising the figure detector is killed; widening the status detector to match everything is killed
- **Insufficient-evidence control:** 5 of 14 surfaces did not run and are reported as unmeasured rather than as passes
- **Tests:**
  - TestC004_OutputSurfaceInventory
  - TestC004_TheSurfaceDetectorsDiscriminate
  - TestC004_TheJSONSurfaceCarriesNoTierField

**Why this result:** Falsified by at least one surface, with 5 of 14 unmeasured and the JSON row under-measured by the detector's own admission. The partial coverage makes it weaker as a census and no weaker as a falsification: one counterexample is enough.

---

## RPL-C029

> The truth status a figure carries is epistemically correct.

| | |
|---|---|
| **Result** | **NO_ENDPOINT** |
| Evidence basis | NOT_APPLICABLE |
| Deciding layer | none |
| Scope | any figure carrying a status |
| Oracle | none is constructible. The mutations this would need, altering a tier/value association or flipping observed to estimated, have no association to alter |
| Asserted at | _nowhere. This is a non-claim or an inferred boundary_ |

- **Establishes:** _none_
- **Does NOT establish:**
  - anything. There is no machine-readable status attached to any figure, so there is nothing to compare against a ground truth. An absent status cannot be wrong, only missing
- **Assumptions Replay does not verify:** _none_
- **Known gaps:**
  - the mutations this claim would need, altering a tier/value association, have no association to alter
- **Positive control:** _none; not applicable to a non-claim_
- **Negative control:** _none; not applicable to a non-claim_
- **Insufficient-evidence control:** no machine-readable status is attached to any figure, so no comparison to a ground truth is constructible
- **Tests:**
  - TestC004_OutputSurfaceInventory

**Why this result:** NO_ENDPOINT rather than NOT_MEASURED, and the distinction is the finding. Nothing was left unmeasured: there is no status attached to any figure, so there is nothing that could be correct or incorrect. Correctness of a label is a separate obligation from presence of one, and presence fails first. This is NOT a recommendation to build a tier system.

---

## RPL-C005

> Anything Replay cannot measure it declines to print, and says why in the place the number would have gone.

| | |
|---|---|
| **Result** | **REFUTED** |
| Evidence basis | NOT_APPLICABLE |
| Deciding layer | none. No type governs what reaches the screen |
| Scope | printed output of `replay cost`, human and JSON |
| Oracle | the real cost report over a corpus whose unpriceability is verified independently with cachemodel.PriceFor before any output is read |
| Asserted at | README.md:21 |

- **Establishes:** _none_
- **Does NOT establish:**
  - the claim as worded. A transcript holding two priceable and two unpriceable requests produces a printed total over the priceable half only, with NO disclosure in the human report and `unpriced` reporting 0 in the JSON
  - that the disclosure sentence is reachable per record. cost.go:488 exists and fires only when a WHOLE transcript is unpriced, because `unpriced` has transcript granularity while pricing has record granularity
  - ASSUMPTION, not established: that a refusal is conveyed by the words unpriced, not priced or excluded. A refusal worded otherwise would be understated by this measurement
- **Assumptions Replay does not verify:**
  - ASSUMPTION: a refusal is conveyed by the words unpriced, not priced or excluded. A refusal worded otherwise is understated by this measurement
- **Known gaps:**
  - only `replay cost` was measured. burn, advise, route, diff, errors, warnings and the TUI were not
- **Positive control:** a fully priced corpus correctly discloses nothing, so the detection is not firing on everything
- **Negative control:** a mixed corpus discloses nothing either, which is the defect
- **Insufficient-evidence control:** the fixture itself: price is unavailable while usage is present and provider-reported
- **Tests:**
  - TestC005_TheFixtureIsolatesExactlyOneUnmeasurableQuantity
  - TestC005_AMixedTranscriptHidesItsUnpricedRecords
  - TestC005_TheHumanReportAssertsCompletenessOverAPartlyPricedCorpus
  - TestC005_AFullyPricedCorpusCorrectlyDisclosesNothing

**Why this result:** Falsified at the printed surface, which is where the claim is made. The report prints a figure it could only partly compute and then states that it is what the work cost. The internal refusal machinery is correct and is registered separately as RPL-C026 rather than used to rescue this wording.

---

## RPL-C026

> Replay's surface classifier refuses to reach a verdict when the evidence cannot support one, and gives a reason.

| | |
|---|---|
| **Result** | **BOUNDED** |
| Evidence basis | NOT_APPLICABLE |
| Deciding layer | surface.Class, surface.WriteContract |
| Scope | surface.Classify, and its verdict with reason on `replay doctor`'s cache signal line since 2026-10-02. NOT the cost report's printed figures, which RPL-C005 refutes |
| Oracle | paired fixtures whose correct verdict is known from the contract, not from the classifier |
| Asserted at | internal/surface/identify.go:174, cmd/replay/doctor.go |

- **Establishes:**
  - an empty corpus, and zero write observations under an unknown provider contract, both reach ClassUndetermined
  - the refusal carries a reason rather than arriving as a bare verdict
  - an unsourced contract fact is rejected outright, so an assertion with no provenance cannot enter as a provider fact
  - the verdict and its reason reach the user: replay doctor prints the class and, on an empty boundary, undetermined with the reason, through the shipped dispatch path
- **Does NOT establish:**
  - that every refusal reaches the user. RPL-C005 establishes that the cost report's does not; this claim's reaches doctor
  - that every refusal site is correct; reachability is machine-checked per site by scripts/refusal-reachability, correctness is not
- **Assumptions Replay does not verify:** _none_
- **Known gaps:**
  - internal only. RPL-C005 shows the refusal does not always reach the user
- **Positive control:** empty corpus and zero-writes-under-unknown-contract both reach ClassUndetermined with a reason
- **Negative control:** a clean corpus with a sourced contract does NOT refuse
- **Insufficient-evidence control:** ClassUndetermined is itself the insufficient-evidence result
- **Tests:**
  - TestC005_RefusalIsDistinguishableFromZero
  - TestC005_AnUnsourcedContractIsRefused
  - TestE2E_Doctor
  - TestE2E_DoctorRefusesToClassifyAnEmptyBoundary

**Why this result:** The classifier does what the claim describes and, since 2026-10-02, its verdict and reason are printed by replay doctor. The bound is the surface: doctor only, for the Anthropic transcript boundary, under a contract fact doctor states with its source. The cost report's own refusal (RPL-C005) is a separate finding.

---

## RPL-C034

> When a model has no price, Replay never reports its cost as zero.

| | |
|---|---|
| **Result** | **BOUNDED** |
| Evidence basis | OBSERVED |
| Deciding layer | costUnit.Unpriced, a presence field serialized with the unit |
| Scope | MEASURED SURFACES ONLY, narrowed 2026-10-01 to what the evidence covers: the re-billed token count in `replay cost` and the worst-window ranking in `replay since`. The original scope said every consumer of cachemodel.PriceFor and PriceForAt, and 7 of 8 consumers were never measured at the user-visible boundary |
| Oracle | a source census classifying each false branch, with the contract quoted from the repository's own document rather than assumed |
| Asserted at | docs/TOKEN-PRICES.md:53 |

- **Establishes:**
  - 9 of 19 call sites count the exclusion, refuse with a reason, or substitute a labelled upper bound
  - at the two measured surfaces, a known re-billed token count survives an unavailable price. Two corpora differing only in a model id reported 40,000 tokens and 0 before 6794522 and report 40,000 on both arms after it
  - `replay since` ranks by that count even when the price is unknown: with the unpriced session carrying four times the other's deficit, the command names it, cold and warm
- **Does NOT establish:**
  - anything about the other consumers of the price boolean. One was repaired; the remaining sites were censused from source and NEVER measured at the user-visible boundary, so TOKEN-PRICES.md:53 is NOT established as satisfied for them
  - that 8 consumer sites are defective. That census was too coarse and is RETRACTED: most sites gate only dollars, which require a price, and order.go and trim.go carry presence flags the detector missed. The source census was necessary and not sufficient
  - that the remaining sites are equivalent to each other. The 9 that handle it use at least four different conventions, so there is no shared contract in code, only a shared document
  - the magnitude on any surface. The call-site census is established; what each silent zero does to a printed figure is measured only for cost.go and route.go
- **Assumptions Replay does not verify:** _none_
- **Known gaps:**
  - 7 of 8 consumers were censused from source and NEVER measured at the user-visible boundary. TOKEN-PRICES.md:53 is established as satisfied ONLY at `replay cost` and `replay since` for the re-billed token count
  - the 9 compliant sites use at least four different conventions, so there is still no shared contract in code
- **Positive control:** after 6794522, both arms report 40,000 re-billed tokens and `since` names the larger session, cold and warm
- **Negative control:** five mutations all kill: Unpriced propagation, the token assignment, the warm reconstruction, the genuine-zero collapse, and unpriced rows re-entering the dollar statistics
- **Insufficient-evidence control:** 2 of the 10 originally flagged sites are inside PriceForAt itself and are delegation rather than consumption, so the consumer count is 8
- **Tests:**
  - TestEC00_ThePriceBooleanIsTheSharedCompressionPoint
  - TestCW1_TheFourStates
  - TestCW2_ColdAndWarmAgree
  - TestCW3_RankingHoldsColdAndWarm
  - TestRB2_AKnownDeficitIsReportedWhetherOrNotAPriceExists
  - TestRB5_AnUnpricedSessionCannotBeNamedTheWorstWindow

**Why this result:** CLOSED at the two surfaces that were measured and BOUNDED because the others were not. Repaired by 6794522 at three sites, verified cold and warm, and five mutations kill it. The original REFUTED verdict stands as the before state. CENSUS CORRECTED: a per-site audit found that most flagged sites compute their TOKEN quantities outside the price branch and gate only dollars, which is correct, and that order.go and trim.go carry presence flags the first detector missed. ONE price-independent quantity is suppressed, the re-billed token count, and a mutation campaign then corrected its attribution too: it is gated TWICE, at cost.go:680 and again at cost.go:717, and removing either alone changes nothing. Measured at the user-visible boundary as 40,000 tokens against 0 on corpora differing only in a model name.

---

## RPL-C033

> A dollar figure states the TTL basis its cache writes were priced on.

| | |
|---|---|
| **Result** | **BOUNDED** |
| Evidence basis | OBSERVED |
| Deciding layer | none. transcript.Usage has no presence field for the TTL split |
| Scope | every surface printing a cache-write cost: burn, cost --usage, route |
| Oracle | longhand multiplier arithmetic, independent of writeEquivalent |
| Asserted at | README.md:293 |

- **Establishes:**
  - where the provider supplies a TTL split, the distinction survives and prices correctly
- **Does NOT establish:**
  - the claim where the provider supplies NO split. (*WireUsage).Usage() drops the presence bit the wire type carries, so an absent breakdown and a reported 0/0 arrive identically, and writeEquivalent then prices the whole write leg at the SHORT multiplier
  - ASSUMPTION, not established: that an absent breakdown means a 5-minute TTL. It is the documented provider default and Replay never verifies it. The same 10,000 cache-creation tokens price at $0.037500 on that assumption and $0.060000 if the writes were in fact 1h, a 60% difference
  - that the assumption is disclosed. It is declared in two code comments and in no output
- **Assumptions Replay does not verify:**
  - ASSUMPTION: an absent TTL breakdown means the provider's 5-minute default. RESPONSIBILITY: the provider. REPLAY VERIFIES: nothing. ON VIOLATION: the cache-write leg is understated by 60%, undisclosed
- **Known gaps:**
  - whether providers omit the breakdown for 1h writes is unmeasured and needs live traffic
- **Positive control:** an explicit 5m split and an explicit 1h split price differently (EC4)
- **Negative control:** absent and present-0/0 are byte-identical downstream (EC1)
- **Insufficient-evidence control:** the absent case IS the insufficient-evidence case, and it is given a default rather than refused
- **Tests:**
  - TestEC1_AbsentAndZeroTTLBreakdownAreIndistinguishable
  - TestEC2_AnAbsentBreakdownIsPricedOnAnAssumedTTL
  - TestEC3_TheAssumedDefaultIsDocumentedAtTheSite
  - TestEC4_TheTTLDistinctionSurvivesWhereTheProviderSuppliesIt

**Why this result:** Bounded rather than refuted, and the distinction matters. The collapse is declared at both sites, so this is not a silent defect; the TTL distinction is honoured wherever the provider reports it. What is undisclosed is that a figure computed on an absent breakdown rests on an assumed default, and the reader is given the dollars without the assumption.

---

## RPL-C030

> When a whole session cannot be priced, Replay refuses and says why.

| | |
|---|---|
| **Result** | **BOUNDED** |
| Evidence basis | NOT_APPLICABLE |
| Deciding layer | none. Two per-session counters, unpriced and unreadable |
| Scope | session granularity only |
| Oracle | a fixture matrix whose ground truth the test constructs and therefore knows without reading it back |
| Asserted at | README.md:21, cmd/replay/cost.go:488 |

- **Establishes:**
  - a wholly unpriceable session is counted and disclosed
  - a fully priceable corpus discloses nothing, so the refusal does not fire on everything
  - a measurable session beside an unmeasurable one does NOT rescue it; foreign evidence is not borrowed
- **Does NOT establish:**
  - anything below session granularity. RPL-C031 refutes that
  - that the explanation is complete, only that it fires and is accurate when it does
- **Assumptions Replay does not verify:** _none_
- **Known gaps:**
  - contradictory and stale evidence were INAPPLICABLE: the ledger reader has no two-source reconciliation for one quantity, so there is nothing to contradict
- **Positive control:** MX2 and MX9 disclose; MX4 discloses the wholly unpriced session
- **Negative control:** MX1, a fully measurable corpus, discloses nothing
- **Insufficient-evidence control:** MX2 is the insufficient-evidence case and is handled correctly
- **Tests:**
  - TestC005M_FixtureAssumptionsHold
  - TestC005M_TheFixtureMatrix

**Why this result:** MX1, MX2, MX4 and MX9 all behave correctly at session granularity. Three mutations on the refusal path all kill.

---

## RPL-C031

> Replay discloses partial coverage: when some records in a session cannot be priced, it says so.

| | |
|---|---|
| **Result** | **REFUTED** |
| Evidence basis | NOT_APPLICABLE |
| Deciding layer | none |
| Scope | record granularity within one session |
| Oracle | the record-level ground truth the fixture declares, compared against the printed figures |
| Asserted at | README.md:21 |

- **Establishes:** _none_
- **Does NOT establish:**
  - the claim. The counter operates at SESSION granularity and this claim is about RECORD granularity. A session holding two priceable and two unpriceable records has a priceable model on its first request, so it is not flagged, its unpriceable records still contribute nothing to the total, and nothing is disclosed on either surface
  - UNCHANGED BY THE REPAIR OF 6794522, verified rather than assumed. That repair replaced an inferred priceability test with a real one at session granularity, which closed RPL-C032 and RPL-C034's measured surface. Re-measured afterwards, MX3 and MX10 still report unpriced=0 with no disclosure. A record-level counter is a separate change and was not made
- **Assumptions Replay does not verify:** _none_
- **Known gaps:**
  - only `replay cost` was measured
  - UNCHANGED BY 6794522. That repair works at session granularity; this claim is about record granularity. A record-level counter is a separate change and was not made
- **Positive control:** MX1 proves the disclosure can be silent correctly
- **Negative control:** MX3 and MX10 both show 2 of 4 records unmeasurable with unpriced=0 and no disclosure, RE-MEASURED after 6794522
- **Insufficient-evidence control:** MX4 isolates the boundary: a wholly unpriced session IS disclosed, a half unpriced one is not
- **Tests:**
  - TestC005M_TheFixtureMatrix
  - TestC005_AMixedTranscriptHidesItsUnpricedRecords

**Why this result:** Reproduced from scratch with an independent oracle. The report prints a total over the measurable half and states that it is what the work cost. Re-evaluated after 6794522 and STILL REFUTED: that repair works at session granularity and this claim is about record granularity, so the two do not meet. Status unchanged on measurement, not on inference.

---

## RPL-C032

> Replay distinguishes a measured zero from an unmeasurable quantity.

| | |
|---|---|
| **Result** | **BOUNDED** |
| Evidence basis | OBSERVED |
| Deciding layer | none. A price-table lookup, not an inference from cost |
| Scope | session cost in `replay cost` |
| Oracle | a session constructed with every usage field zero on a model verified priced before the run |
| Asserted at | README.md:603 |

- **Establishes:**
  - a session that genuinely cost nothing is no longer reported as unpriced. The repair of 6794522 replaced `asRun.CostUSD <= 0` with a real price-table lookup, so priceability is asked rather than inferred from a cost of zero
- **Does NOT establish:**
  - the distinction anywhere other than session cost in `replay cost`. The repair asked the price table directly at ONE site; no other site was revalidated
  - anything about the three-state distinction elsewhere. RPL-C008 establishes it for cache counters, where WriteFieldPresent exists; this is a different site with no equivalent
- **Assumptions Replay does not verify:** _none_
- **Known gaps:**
  - only session cost in `replay cost` was revalidated. Other sites may or may not collapse the same way and were not re-measured
- **Positive control:** CW1's genuine-zero state: a session with every usage field zero on a PRICED model reports unpriced=0 and keeps its dollar figures
- **Negative control:** CW1's unpriced state on the same corpus shape reports unpriced=1, so the two are distinguishable
- **Insufficient-evidence control:** before 6794522 both states reached the same counter; MX8 reported unpriced=1 for a genuine zero
- **Tests:**
  - TestC005M_TheFixtureMatrix
  - TestCW1_TheFourStates
  - TestRB6_AGenuineZeroIsNotTheSameAsUnavailable

**Why this result:** CLOSED by 6794522 at the site it was raised against, and bounded because only that site was revalidated. Before: MX8 built a session with every usage field zero on a PRICED model and it reported unpriced=1. After: it reports 0, CW1's genuine-zero state passes, and the mutation that restores the inferred test is killed. The original REFUTED verdict stands as history; the first version of MX8 did not test this at all because it left cache usage priced, and that defect in the test is preserved in the evidence file.

---

## RPL-C037

> The re-billed dollar figure and the re-billed token count `replay cost` prints together describe the same re-billing.

| | |
|---|---|
| **Result** | **BOUNDED** |
| Evidence basis | RECONSTRUCTED |
| Deciding layer | none. No type carries break-level priceability; `breaks` counts priceable and unpriceable alike |
| Scope | break granularity within `replay cost`, human and JSON surfaces |
| Oracle | a break-token ledger computed from the fixture specification, never from RebilledUSD or RebilledTokens, compared against the rendered and serialized re-billing surface |
| Asserted at | cmd/replay/cost.go:547, cmd/replay/cost.go:549 |

- **Establishes:**
  - the token count. RebilledTokens covers every break and is correct, which RPL-C036 settled
  - the pairing is now disclosed rather than true. `replay cost` states what share of the re-billed tokens the dollar figure represents, floored and never clamped, and states the excluded count. Suppressed where nothing is excluded and where there was no break
  - the disclosure describes the figure rather than the price lookup. A unit-flagged session's break tokens are excluded although every one of its breaks priced, which a per-break-only counter reported as 100% against a figure covering 33%
- **Does NOT establish:**
  - the pairing itself, which is unchanged. RebilledUSD still covers only breaks whose own record could be priced, at cost.go:885 and again at cost.go:453, and RebilledTokens still counts every break at cost.go:433. The repair is a disclosure: no dollar and no token figure moved, which TestC037_P1 asserts in both directions
  - anything about break COUNT coverage, which is deliberately not disclosed. On a corpus of one 90,000-token priceable break against three 1,000-token unpriceable ones, token coverage is 96% and count coverage 25%; the dollar figure is token-weighted, so the count would describe a different population
  - any surface other than `replay cost`. `replay advise`, the card and the share surface read re-billed figures and were not examined for the same collapse
  - that C035's disclosure covers this. That sentence is about requests and about TotalUSD; held identical across two corpora it leaves break coverage at 100% and 50% indistinguishable
  - ASSUMPTION, not established: that a future disclosure would be recognisable to the frozen detector. It is proven against a planted sentence, not against a shipped one, and a disclosure worded without the words priced, price table or unpriceable would not be seen
  - anything about `replay since`, which ranks on RebilledTokens alone and is outside this claim
  - `replay cost --usage`, which is a SECOND path inside the same command with the same shape: costusage.go:194 accumulates sDeficit unconditionally, costusage.go:196 adds to sRebilled only `if priced`, and costusage.go:267 renders the two as one pair. Found by the post-repair surface audit, NOT measured and NOT repaired
- **Assumptions Replay does not verify:**
  - that a future disclosure would be recognisable to the frozen detector. It is proven against a planted sentence and against C035's sentence, which it must not match, but not against a shipped one
- **Known gaps:**
  - the five rejected candidates in TestC037_TheOracleRejectsEveryWrongCandidate are SIMULATED disclosures, retained as an oracle self-check. They are NOT the mutation evidence; ten mutations of the landed implementation are, and are recorded in the evidence file
  - only the transcript path of `replay cost` was measured. `replay cost --usage` has the same two-population shape at costusage.go:194/196 and renders the pair at costusage.go:267; it was found by the post-repair audit and is neither measured nor repaired. `replay advise`, the card and the share surface also read re-billed figures and were not examined
- **Positive control:** P1/Q and P2/Q2, corpora with 20,000 of 40,000 break tokens unpriceable, where the oracle computes 50% coverage and the report now discloses 50%; plus the second-gate corpus, where every break priced and the figure covers 33%; plus the unequal-deficit corpus at 96% token against 25% count coverage
- **Negative control:** P1/P and P2/R, fully priceable corpora at 100% coverage, which must never carry a partial-coverage disclosure; and a zero-break corpus, which must carry neither a disclosure nor a re-billed token line
- **Insufficient-evidence control:** a corpus where request-level and break-level coverage happen to coincide does not discriminate, which is why P1/Q is built so they diverge (2 of 3 requests priced against 1 of 2 break token halves)
- **Tests:**
  - TestC037_FixtureAssumptions
  - TestC037_P1_TheRebillingSurfaceCollapses
  - TestC037_P1b_RequestCoverageCarriesNoBreakCoverage
  - TestC037_P2_NoBreakCoverageIsDisclosed
  - TestC037_P3_FullCoverageEmitsNoPartialDisclosure
  - TestC037_P4_ZeroBreakCorpusEmitsNoDisclosure
  - TestC037_TheDetectorDiscriminates
  - TestC037_TheOracleRejectsEveryWrongCandidate
  - TestC037R1_TheFourCoverageCases
  - TestC037R2_UnequalDeficitsRuleOutCountCoverage
  - TestC037R3_TheUnitGateDecidesRepresentation
  - TestC037R4_AZeroRatePriceableBreakIsFullyRepresented
  - TestC037R5_ColdAndWarmDiscloseTheSame
  - TestC037R6_TheIndexKeyCarriesTheCounter
  - TestC037R7_TheCounterSurvivesTheLaneFold

**Why this result:** REPAIRED 2026-10-02 by one counter, UnpricedRebilledTokens, and one disclosure. Frozen first: two corpora on one model name, 40,000 break tokens each, one fully priceable and one with 20,000 unpriceable, rendered an identical re-billed result and identical JSON. After the repair the 100% arm discloses nothing and the 50% arm discloses 50%, while both still report $0.020000 over 40,000 tokens. Ten production mutations killed, including count-based, request-based, per-break-only, forced-100%, inverted, and deriving priceability from a nonzero contribution; the lane-fold mutation SURVIVED first because every fixture was single-lane, and a two-lane case was added rather than the survivor explained away. Bounded because only `replay cost` was measured. cost.go:70-77's principle, that unpriced is not zero cost, now holds for RebilledUSD as it already did for CostUSD.

---

## RPL-C008

> Absence, zero and unknown are three different values, and Replay never collapses them.

| | |
|---|---|
| **Result** | **ESTABLISHED** |
| Evidence basis | OBSERVED |
| Deciding layer | surface.WriteContract, Observables.WriteFieldPresent |
| Scope | cache counters on every surface |
| Oracle | fixtures that omit a field versus fixtures that set it to zero, compared on output |
| Asserted at | README.md:603, docs/adr/0018-provenance-is-a-field.md |

- **Establishes:**
  - a missing counter is reported as missing, not as zero
- **Does NOT establish:**
  - that a reported zero is a true zero. That depends on the provider's contract, which may be unknown
- **Assumptions Replay does not verify:** _none_
- **Known gaps:** _none_
- **Positive control:** absent write counter classifies III-no-observable
- **Negative control:** present-and-zero counter classifies undetermined, a different verdict
- **Insufficient-evidence control:** present-and-zero under an unknown contract is itself the refusal
- **Tests:**
  - TestC008_AbsentFieldAndZeroFieldDiffer
  - TestB5_AnOmittedQuantitativeKeyIsRefusedNotReadAsZero
  - TestE2E_Cost

**Why this result:** Positive, negative and mutation controls all discriminate. WriteFieldPresent and PrefixMeasured exist for exactly this.

---

## RPL-C019

> Replay distinguishes evidence belonging to different provider accounts.

| | |
|---|---|
| **Result** | **NO_ENDPOINT** |
| Evidence basis | NOT_APPLICABLE |
| Deciding layer | none. No account identity exists to govern |
| Scope | every correlation path: ledger, transcript readers, cost report |
| Oracle | static scan of the ledger, transcript-reader and cost-report source for an account-shaped identity |
| Asserted at | _nowhere. This is a non-claim or an inferred boundary_ |

- **Establishes:** _none_
- **Does NOT establish:**
  - anything. No account, tenant, organisation, project or workspace identity exists anywhere in the correlation path. A ledger record carries SessionID, AgentID, RequestID and SessionHash and nothing that names whose account it was
- **Assumptions Replay does not verify:** _none_
- **Known gaps:**
  - not a gap in testing. There is no endpoint to test.
- **Positive control:** _none; not applicable to a non-claim_
- **Negative control:** _none; not applicable to a non-claim_
- **Insufficient-evidence control:** the scan no longer treats every TenantID as an account identity by spelling alone: internal/tenancy.TenantID (commit 646736d) is Replay's own internal ownership/namespace partition for hosted multi-tenancy, not a provider-account correlation handle, so the detector exempts it specifically while still catching AccountID, OrgID, OrganizationID, OrganisationID, ProjectID and WorkspaceID, and any TenantID that is locally redeclared rather than the registered primitive
- **Tests:**
  - TestXW6_NoAccountIdentityExistsToCorrelateOn

**Why this result:** Not a testing gap: there is no endpoint to test. Two records from different accounts sharing a provider request id are indistinguishable from the same request seen twice, because nothing in the evidence model names the account. Consistent with the distinct-account claim removed at 8e871bf as structurally unavailable, which needed a provider to issue an account-scoped credential and none does. The scan was repo-wide until a hosted-service identity existed anywhere to find; one now does, in internal/tenancy, an unwired primitive for ADR-0028's hosted service (docs/design/UNWIRED-LOG.md) rather than for correlating provider accounts, so the scan is scoped to this claim's own Scope field rather than failing on an unrelated, already-recorded identity forever.

---

## RPL-C020

> Records from two different sessions are never combined into one history.

| | |
|---|---|
| **Result** | **BOUNDED** |
| Evidence basis | OBSERVED |
| Deciding layer | transcript.Request.IDMeasured |
| Scope | cross-file and cross-session joins, ledger reader and cost report |
| Oracle | oracleJoin in cmd/replay/crosswire_test.go, a reference implementation of the stated rule that never calls requestJoin |
| Asserted at | cmd/replay/overlap.go:23, cmd/replay/requestjoin_test.go:16 |

- **Establishes:**
  - a locally synthesised id is never used as a join key, whatever it is spelled
  - two sessions with no shared provider id are never reported as sharing a request, end to end through the cost report
  - adding a foreign session's evidence does not change what is reported about the first
- **Does NOT establish:**
  - that an id collision across sessions would be caught. Two requests on different models with different token counts merge on a shared provider id alone, because the join compares no other field. The implementation conforms to the current join contract under the provider-id uniqueness assumption; this campaign does NOT establish that Replay independently verifies that assumption. Observed under fixture, not theoretical: TestXW2 constructs it and the merge happens
  - that evidence from two different provider accounts is distinguishable at all. There is no account identity in the model; see RPL-C019, which classifies that as NO_ENDPOINT rather than as an untested case
- **Assumptions Replay does not verify:**
  - ASSUMPTION: provider request ids used by this join are globally unique across the scope in which Replay correlates them. RESPONSIBILITY: the provider. PROVIDER DOCUMENTATION: no authoritative statement of this guarantee was located during this campaign. REPLAY OBSERVES: session id, agent id, model, usage and timestamps alongside the id, and uses none of them to corroborate a merge. ON VIOLATION: two distinct requests are silently counted as one, with no disclosure. Classified as an EXTERNAL ASSUMPTION, not a Replay-established fact.
- **Known gaps:**
  - whether corroborating on session id before merging is worth its cost is a production decision and is not taken here
- **Positive control:** A to A and B to B survive; four distinct ids across two sessions report zero duplicates end to end
- **Negative control:** three mutations on the join all kill the suite
- **Insufficient-evidence control:** a synthesised id is counted unjoinable rather than merged or dropped silently
- **Tests:**
  - TestXW1_LegitimateSessionsJoinWithinThemselvesAndNotAcrossThem
  - TestXW2_AnIDCollisionAcrossSessionsMergesWithoutCorroboration
  - TestXW3_ASuperficiallyCompatibleIdentifierDoesNotJoin
  - TestXW4_TwoSessionsOnDiskDoNotShareUsage
  - TestXW5_AddingAForeignSessionDoesNotChangeTheFirstSessionsFigures
  - TestXW7_TheJoinContractBoundary
  - TestRJ1_SynthesisedIDsAreNotJoinedAcrossFiles
  - TestRJ2_ProviderIDsStillJoin

**Why this result:** No tested cross-session fixture produced an unintended merge except the deliberate provider-id collision case. That case is an OBSERVED behaviour under a constructed fixture, not a theoretical limitation, and it demonstrates that current join safety depends on provider request-id uniqueness, which Replay does not independently establish.

---

## RPL-C021

> Replay offers no cross-surface grand total.

| | |
|---|---|
| **Result** | **ESTABLISHED** |
| Evidence basis | NOT_APPLICABLE |
| Deciding layer | none. Enforced by the shape of the type |
| Scope | replay burn |
| Oracle | type-level: the summary type exposes no addition; and behavioural: the rendered burn report, with two surfaces populated, carries no total line and not the sum of the per-surface figures |
| Asserted at | cmd/replay/burn.go:20 |

- **Establishes:**
  - figures with incommensurable units are not added
- **Does NOT establish:**
  - that per-surface figures are themselves comparable
- **Assumptions Replay does not verify:** _none_
- **Known gaps:** _none_
- **Positive control:** the type is found and carries fields
- **Negative control:** no field or method name offers a total; has* applicability flags are present
- **Insufficient-evidence control:** has* flags keep 'not applicable to this surface' distinct from zero
- **Tests:**
  - TestC021_NoCrossSurfaceTotalIsReachable
  - TestC021_BurnPrintsNoCrossSurfaceTotal

**Why this result:** Enforced by the shape of the type rather than by a convention, and a mutation that adds a total is caught.

---

## RPL-C013

> Replay establishes realized savings.

| | |
|---|---|
| **Result** | **DELIBERATE_NON_CLAIM** |
| Evidence basis | NOT_APPLICABLE |
| Deciding layer | none |
| Scope | all output |
| Oracle | scan of user-facing strings for savings and forecast vocabulary |
| Asserted at | _nowhere. This is a non-claim or an inferred boundary_ |

- **Establishes:** _none_
- **Does NOT establish:**
  - anything. The product explicitly refuses to forecast or claim savings
- **Assumptions Replay does not verify:** _none_
- **Known gaps:**
  - only Go string literals are scanned; README and docs prose are not
- **Positive control:** the forecast detector fires on a planted savings forecast
- **Negative control:** no user-facing string in the tree matches
- **Insufficient-evidence control:** _none. Recorded as a gap_
- **Tests:**
  - TestC013_NoSavingsClaimReachesTheUser

**Why this result:** README:523 and the agent skill both forbid it. Payback computes a projection from two reconstructed costs under an unverifiable counterfactual, and labels it an assumption.

---

## RPL-C015

> Replay can establish that acting on its advice changed anything.

| | |
|---|---|
| **Result** | **NOT_MEASURED** |
| Evidence basis | NOT_APPLICABLE |
| Deciding layer | advisor.Status |
| Scope | advisor findings |
| Oracle | the refusal is in the product: track returns AdviceOnly before any scoring runs |
| Asserted at | _nowhere. This is a non-claim or an inferred boundary_ |

- **Establishes:** _none_
- **Does NOT establish:**
  - outcome for KindCacheBreaks or KindHotFile, the two kinds carrying the largest predicted value
- **Assumptions Replay does not verify:** _none_
- **Known gaps:** _none_
- **Positive control:** KindCacheBreaks and KindHotFile return AdviceOnly with a zero realized figure
- **Negative control:** 4 of 4 remaining kinds ARE scoreable on the same falling share, so the refusal is a decision not an incapacity
- **Insufficient-evidence control:** AdviceOnly is itself the insufficient-evidence result
- **Tests:**
  - TestC015_TheTwoLargestKindsAreRefusedScoring
  - TestC015_VerifierInferenceIsNotAReaderDecision

**Why this result:** The verifier refuses by design, because a share that moves with the work cannot be attributed to a user action. The seeded-intervention pilot then failed its gating question.

---

## RPL-C016

> Replay improves agent task outcomes.

| | |
|---|---|
| **Result** | **NOT_MEASURED** |
| Evidence basis | NOT_APPLICABLE |
| Deciding layer | none |
| Scope | durable work state |
| Oracle | R10 hidden-oracle benchmark |
| Asserted at | _nowhere. This is a non-claim or an inferred boundary_ |

- **Establishes:** _none_
- **Does NOT establish:**
  - anything in either direction
- **Assumptions Replay does not verify:** _none_
- **Known gaps:**
  - no substrate with demonstrated control-arm headroom exists, so the question cannot currently be asked
- **Positive control:** the improvement detector fires on a planted claim
- **Negative control:** no user-facing string asserts it
- **Insufficient-evidence control:** R10 returned INVALID EXPERIMENT at a preregistered ceiling; the treatment arm never ran
- **Tests:**
  - TestC016_NoTaskImprovementClaimIsAsserted

**Why this result:** R1 established retrieval, not outcome. The R10 trial on 2026-09-30 stopped at a preregistered ceiling and the treatment arm never ran.

---

## RPL-C022

> The binary makes no network request except for commands the user explicitly types.

| | |
|---|---|
| **Result** | **BOUNDED** |
| Evidence basis | NOT_APPLICABLE |
| Deciding layer | none |
| Scope | the shipped binary |
| Oracle | static enumeration of outbound destinations in non-test code |
| Asserted at | README.md:407, plugins/replay/skills/replay-doctor/SKILL.md:9 |

- **Establishes:**
  - no telemetry, no beacon, no first-run call
- **Does NOT establish:**
  - anything about the hosted installer, which is checked separately
  - ASSUMPTION, not established: that a destination is reachable only through a URL literal in Go source. A host assembled at runtime from parts would not be seen by this scan
- **Assumptions Replay does not verify:**
  - ASSUMPTION: a destination reachable only through a URL literal in Go source. A host assembled at runtime from parts would not be seen.
- **Known gaps:**
  - static only. No sandboxed dynamic run was performed.
- **Positive control:** the scan counts remote destinations; zero would fail it
- **Negative control:** the loopback skip is proven not to swallow api.anthropic.com
- **Insufficient-evidence control:** _none. Recorded as a gap_
- **Tests:**
  - TestC022_OutboundDestinationsAreAnEnumeratedSet

**Why this result:** Checkable statically. A dynamic proof would need a sandboxed run and is not attempted here.

---

## RPL-C038

> Replay's production cache-break classification selects OpenAI's published GPT-6 Astra-tier CacheRules for an Astra-tier request, not Anthropic's.

| | |
|---|---|
| **Result** | **BOUNDED** |
| Evidence basis | RECONSTRUCTED |
| Deciding layer | cachemodel.CacheRules |
| Scope | model ids containing the substring "gpt-6-astra", at the three production call sites that classify a cache break: internal/proxy/state.go (the live proxy), internal/analysis/diff.go (the offline diff), cmd/replay/costusage.go (the usage-export cost path). TTL and effort-change dispatch only |
| Oracle | reverting the production wiring at each of the three call sites in turn and watching the corresponding test (internal/proxy/breakcauseprovider_test.go for the live proxy, internal/analysis/diffprovider_test.go for the offline diff) fail for the predicted reason (a 20m Astra gap reported as TTL expiry under Anthropic's 5m rule), then re-applying the fix and watching it pass, with every pre-existing test in internal/cachemodel, internal/proxy and internal/analysis staying green throughout. A repository-wide guard-reachability sweep independently confirmed the gap this claim closes: before this change, nothing in internal/analysis's own test suite drove the usage/timing branch of diff.go's classifier at all |
| Asserted at | RELEASE-CRITERIA.md, docs/ROADMAP.md, internal/cachemodel/openai.go |

- **Establishes:**
  - AstraRules(), typed and mutation-tested since 2026-09-15, is reachable from production rather than only from this package's own tests
  - an Astra-tier request's TTL expiry is read against OpenAI's documented 30 minutes, not Anthropic's 5-minute default, at all three call sites
- **Does NOT establish:**
  - that AstraRules()'s numbers match real OpenAI behaviour. They are read from OpenAI's prompt-caching guide on 2026-09-15 and have never been replayed against a live response; see RPL-C039
  - that any OpenAI-compatible request has ever been observed by this build against a live OpenAI endpoint
  - the MinPrefix floor (1024 tokens). None of the three call sites knows the current request's visible prefix size at the point of classification, so ClassifyBreakForModel passes 0 exactly as the Anthropic-only ClassifyBreak already did, and the floor test is skipped rather than guessed. Unreachable from these three call sites before this change and unreachable after it
  - any OpenAI tier other than gpt-6-astra. gpt-5.6-terra, gpt-5.4 and gpt-5.4-mini fall back to AnthropicRules(), unchanged, because this package has no published CacheRules for them
  - ASSUMPTION, not established: that OpenAI's real model id for the Astra tier contains the literal substring "gpt-6-astra", and that the writer's model (not a mid-lane provider change) is always the right ruleset to classify a read against
- **Assumptions Replay does not verify:**
  - ASSUMPTION: the Astra tier is identified by the substring "gpt-6-astra" in the model id. A real OpenAI model id spelled differently would not be recognised and would silently fall back to AnthropicRules()
  - ASSUMPTION: the ruleset that governs a cache entry's survival is the predecessor's model (prevModel), not the current request's. A provider migration mid-lane is read under the writer's rules, which is correct for TTL/floor but untested against a real cross-provider lane
- **Known gaps:**
  - the MinPrefix floor is not reachable from any of the three production call sites, because none carries the current request's visible prefix size at the point of classification
  - no real OpenAI traffic has ever exercised this path; see RPL-C039
- **Positive control:** a 40m gap on a gpt-6-astra entry reports TTL expiry (Astra's own published 30m), through RulesForModel, ClassifyBreakForModel, and the live proxy's own breakCause method
- **Negative control:** a 20m gap on the same entry does NOT report TTL expiry (Anthropic's 5m default would wrongly fire it); reverting the production wiring back to the Anthropic-pinned ClassifyBreak reproduces this exact false positive and is how the fix was proven, not merely asserted
- **Insufficient-evidence control:** a model id for an OpenAI tier this package has no published CacheRules for (gpt-5.6-terra, gpt-5.4, gpt-5.4-mini) falls back to AnthropicRules() rather than failing loudly, which is a silent default rather than a refusal
- **Tests:**
  - TestRulesForModel_SelectsAstraForTheGPT6AstraTier
  - TestRulesForModel_FallsBackToAnthropicForEverythingElse
  - TestClassifyBreakForModel_AstraTierUsesAstraTTLNotAnthropics
  - TestClassifyBreakForModel_AnthropicTierIsUnchanged
  - TestBreakCause_AstraTierUsesAstraTTLInProduction
  - TestBreakCause_AnthropicTierUnchangedInProduction
  - TestBreakCause_AstraTierStillExpiresPastItsOwnTTL
  - TestFindBreaks_AstraTierUsesAstraTTLNotAnthropics
  - TestFindBreaks_AnthropicTierUnchanged

**Why this result:** RED/GREEN-proven at both the unit level and against the live proxy's own classification method, with the reverted-wiring state reproducing the exact pre-existing defect on demand. Bounded to the Astra tier and to TTL/effort dispatch; the floor is a separate, unclosed gap and real-traffic calibration is a separate, unclosed claim (RPL-C039).

---

## RPL-C039

> AstraRules()'s published numbers have been calibrated against real OpenAI traffic.

| | |
|---|---|
| **Result** | **NOT_MEASURED** |
| Evidence basis | NOT_APPLICABLE |
| Deciding layer | none |
| Scope | whether this claim is made anywhere a reader of this repository's own surfaces would see it |
| Oracle | no OPENAI_API_KEY, no billing-linked OpenAI account, and no other mechanism for real, billable OpenAI traffic exists anywhere in this environment, checked explicitly on 2026-10-08 (env, common .env locations, repository-wide grep) |
| Asserted at | _nowhere. This is a non-claim or an inferred boundary_ |

- **Establishes:** _none_
- **Does NOT establish:**
  - anything in either direction about whether OpenAI's documented Astra-tier numbers are accurate. Nobody has checked
- **Assumptions Replay does not verify:** _none_
- **Known gaps:**
  - no OPENAI_API_KEY, billing-linked account, or other mechanism for real OpenAI traffic exists in this environment; the question cannot currently be asked from here
- **Positive control:** the calibration-claim detector fires on a planted completion sentence (two forms), proving it can fail
- **Negative control:** no repository surface this scan reads currently asserts the calibration happened
- **Insufficient-evidence control:** _none. Recorded as a gap_
- **Tests:**
  - TestC039_NoOpenAICalibrationAgainstLiveTrafficClaimIsAsserted

**Why this result:** Not a missing feature; a missing external dependency this environment cannot supply. The harness RPL-C038 wires is ready to consume a real corpus the day one exists; none exists today.

---
