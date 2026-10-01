# Claim register, complete

**Generated from `internal/claims` at build time. Do not edit by hand:
regenerate it, or it will drift from the code it describes.**

| Result | Count |
|---|---|
| BOUNDED | 6 |
| DELIBERATE_NON_CLAIM | 1 |
| ESTABLISHED | 2 |
| NOT_MEASURED | 2 |
| NO_ENDPOINT | 2 |
| UNRESOLVED | 1 |
| **Total** | **14** |

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
| **Result** | **UNRESOLVED** |
| Evidence basis | NOT_APPLICABLE |
| Deciding layer | none. No in-code type governs tier presence |
| Scope | printed output of the CLI |
| Oracle | partial. The price basis is shown to declare its own provenance and age; exhaustive per-figure tier coverage is NOT established |
| Asserted at | README.md:293, docs/adr/0002-replay-engine-and-truth-tiers.md:9 |

- **Establishes:**
  - a reader can tell how a number was obtained
- **Does NOT establish:**
  - that a 'measured' figure is a billed figure
  - that the tier is correct, only that one is present
- **Assumptions Replay does not verify:** _none_
- **Known gaps:**
  - no enumeration of every user-visible figure against its tier. The claim is asserted at a level this campaign has not reached
  - no insufficient-evidence control exists, because the claim is about presence rather than about a measurement
- **Positive control:** the price basis declares version, check date and staleness
- **Negative control:** a table two years past its check date emits a staleness note; a fresh one does not
- **Insufficient-evidence control:** _none. Recorded as a gap_
- **Tests:**
  - TestC011_ThePriceBasisDeclaresItsOwnProvenanceAndAge

**Why this result:** This campaign established that the price basis declares its version, its check date and its staleness. It did NOT enumerate every printed figure and confirm each carries a tier. The claim is plausible and untested at the level it is stated.

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
| **Result** | **BOUNDED** |
| Evidence basis | NOT_APPLICABLE |
| Deciding layer | surface.Class, surface.WriteContract |
| Scope | all printed figures |
| Oracle | scripts/refusal-reachability neutralises each refusal and requires the suite to notice |
| Asserted at | README.md:21 |

- **Establishes:**
  - absence of a figure is distinguishable from a figure of zero
- **Does NOT establish:**
  - that every refusal is correct, only that refusals are reachable and distinguishable
- **Assumptions Replay does not verify:** _none_
- **Known gaps:**
  - refusal correctness is established per-site by scripts/refusal-reachability and not globally
  - the user-visible output after a refusal is not inspected; only the internal verdict is
- **Positive control:** empty corpus, and zero writes under an unknown contract, both reach ClassUndetermined with a reason
- **Negative control:** a clean corpus with a sourced contract does NOT refuse, so the refusal is not firing on everything
- **Insufficient-evidence control:** the refusal IS the insufficient-evidence result here
- **Tests:**
  - TestC005_RefusalIsDistinguishableFromZero

**Why this result:** Reachability of refusals is machine-checked. Correctness of each refusal is per-site and is not established globally.

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

