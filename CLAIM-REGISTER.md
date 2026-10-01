# Claim register, complete

**Generated from `internal/claims` at build time. Do not edit by hand:
regenerate it, or it will drift from the code it describes.**

| Result | Count |
|---|---|
| BOUNDED | 7 |
| DELIBERATE_NON_CLAIM | 1 |
| ESTABLISHED | 2 |
| NOT_MEASURED | 2 |
| NO_ENDPOINT | 2 |
| REFUTED | 2 |
| **Total** | **16** |

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

**Why this result:** Falsified as worded, by the machine-readable surface. Separately the three-word vocabulary is not how the human report labels money, and the two documents asserting the claim disagree on what the words are: README names three tiers, ADR-0002 is titled two and never contains the word structural. What is true is registered as RPL-C025 rather than read back into this one.

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
| Scope | surface.Classify only. NOT the printed surface, which RPL-C005 refutes |
| Oracle | paired fixtures whose correct verdict is known from the contract, not from the classifier |
| Asserted at | internal/surface/identify.go:174 |

- **Establishes:**
  - an empty corpus, and zero write observations under an unknown provider contract, both reach ClassUndetermined
  - the refusal carries a reason rather than arriving as a bare verdict
  - an unsourced contract fact is rejected outright, so an assertion with no provenance cannot enter as a provider fact
- **Does NOT establish:**
  - that any refusal reaches the user. RPL-C005 establishes that at least one does not
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

**Why this result:** The internal layer does what the claim describes. The bound is that it is internal: a provenance field that does not reach the screen protects nobody, which is exactly what RPL-C005 found.

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
| Oracle | static scan of every non-test Go file for an account-shaped identity |
| Asserted at | _nowhere. This is a non-claim or an inferred boundary_ |

- **Establishes:** _none_
- **Does NOT establish:**
  - anything. No account, tenant, organisation, project or workspace identity exists anywhere in the correlation path. A ledger record carries SessionID, AgentID, RequestID and SessionHash and nothing that names whose account it was
- **Assumptions Replay does not verify:** _none_
- **Known gaps:**
  - not a gap in testing. There is no endpoint to test.
- **Positive control:** _none; not applicable to a non-claim_
- **Negative control:** _none; not applicable to a non-claim_
- **Insufficient-evidence control:** a grep of every non-test Go file finds no AccountID, TenantID, OrgID, ProjectID or WorkspaceID
- **Tests:**
  - TestXW6_NoAccountIdentityExistsToCorrelateOn

**Why this result:** Not a testing gap: there is no endpoint to test. Two records from different accounts sharing a provider request id are indistinguishable from the same request seen twice, because nothing in the evidence model names the account. Consistent with the distinct-account claim removed at 8e871bf as structurally unavailable, which needed a provider to issue an account-scoped credential and none does.

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
| Oracle | type-level: the summary type exposes no addition |
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

