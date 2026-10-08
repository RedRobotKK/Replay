package claims

// Controls, assumptions and gaps, keyed by claim id.
//
// Kept beside Register rather than inside it because these fields answer a
// different question. Register says what is claimed and what was concluded.
// This says what the conclusion rests on: which controls discriminate, which
// invariant is assumed rather than verified, and what is still untested.
//
// EVERY registered claim must appear here. TestEveryClaimHasControls enforces
// it, so a claim cannot be added without someone stating how it could fail.

// EvidenceBasis is the epistemic class of the MEASUREMENT a claim is about.
// It is not the claim's verification status, which is claims.Result.
//
// These two were deliberately not merged. A claim can be ESTABLISHED about a
// figure that is RECONSTRUCTED: the first says the verification holds, the
// second says what the figure is made of. Collapsing them is how a verified
// reconstruction starts reading as an observation.
type EvidenceBasis string

// The evidence bases, from the one a provider stated to the one nothing can
// settle.
const (
	Observed      EvidenceBasis = "OBSERVED"      // read from a provider's own record
	Reconstructed EvidenceBasis = "RECONSTRUCTED" // derived from local artifacts
	Reconciled    EvidenceBasis = "RECONCILED"    // two independent sources compared, not merged
	Inferred      EvidenceBasis = "INFERRED"      // supported by evidence that cannot settle it
	NoBasis       EvidenceBasis = "NOT_APPLICABLE"
)

// Controls is what a claim is verified against: its evidence basis, the
// fixtures where it must hold, must not hold, and must refuse, and the
// assumptions and gaps that are stated rather than hidden.
type Controls struct {
	// Basis is the epistemic class of the underlying measurement.
	Basis EvidenceBasis
	// Layer names which vocabulary actually decides the result, so a reader
	// does not assume one subsystem's word means the same elsewhere.
	Layer string
	// Positive is the fixture where the claim must hold.
	Positive string
	// Negative is the fixture where it must NOT hold. Without this a check
	// that always passes is indistinguishable from one that works.
	Negative string
	// Insufficient is the fixture whose correct answer is a refusal rather
	// than a false. Empty means none exists, which is a gap.
	Insufficient string
	// Assumption is an invariant the claim rests on that Replay does NOT
	// itself verify. Naming it prevents it being read as an established fact.
	Assumption []string
	// Gap is what remains untested.
	Gap []string
}

// ControlsFor maps a claim id to its controls.
var ControlsFor = map[string]Controls{
	"RPL-C001": {
		Basis: Reconstructed, Layer: "surface.Class",
		Positive:     "four surfaces classify as the independent audit found",
		Negative:     "three surfaces (Cursor, AnythingLLM, OpenClaw) are refused outright",
		Insufficient: "a corpus with zero records returns ClassUndetermined",
		Gap:          []string{"no shared conformance suite spans the internal/transcript parsers; each has its own fixtures"},
	},
	"RPL-C004": {
		Basis: NoBasis, Layer: "none. No tier type, field or enum exists: the vocabulary is free string literals at 15+ sites across 11 packages",
		Positive:     "the tier detector fires on a labelled line",
		Negative:     "it does not fire on an unlabelled line, and the figure detector does not fire on a bare integer",
		Insufficient: "not applicable. The claim is about presence, not about a measurement",
		Assumption: []string{
			"ASSUMPTION: a tier is conveyed by the words measured, estimated or structural. A report conveying provenance by other wording is understated by this measurement, and the cost report turned out to be exactly that case",
		},
		Gap: []string{
			"the printed surface was not enumerated. One report and one JSON document were measured; burn, advise, route, diff, errors, warnings and the TUI were not",
			"README names three tiers and ADR-0002 names two and never contains the word structural. The documents disagree and this campaign did not correct either",
		},
	},
	"RPL-C025": {
		Basis: Reconstructed, Layer: "none",
		Positive:     "the header above the first dollar figure names a price basis and a date",
		Negative:     "removing the basis from cmd/replay/cost.go kills the test; removing the date kills it separately",
		Insufficient: "a corpus with no priced request reports unpriced rather than a figure",
		Gap:          []string{"one report only. Generality across the printed surface is unmeasured"},
	},
	"RPL-C027": {
		Basis: NoBasis, Layer: "none. The vocabulary is not a type",
		Positive:     "README names a structural tier",
		Negative:     "ADR-0002 does not, and the test fails if either document changes",
		Insufficient: "no ADR of 28 supersedes 0002, so nothing resolves the conflict",
		Gap:          []string{"no authoritative specification exists to measure compliance against"},
	},
	"RPL-C028": {
		Basis: NoBasis, Layer: "none",
		Positive:     "removing advise from the sweep drops figures 4 to 3 while status holds at 3, isolating it as the sole offender",
		Negative:     "neutralising the figure detector is killed; widening the status detector to match everything is killed",
		Insufficient: "5 of 14 surfaces did not run and are reported as unmeasured rather than as passes",
		Assumption: []string{
			"ASSUMPTION: a status is conveyed by a tier word, a dated basis, a list-price statement or a refusal. Deliberately generous, so a surface marked failing has really failed",
		},
		Gap: []string{
			"budget, ceiling, prefix and route did not run against this fixture",
			"the JSON surface is UNDER-MEASURED: the figure detector looks for a dollar sign and JSON emits bare numbers",
		},
	},
	"RPL-C029": {
		Basis: NoBasis, Layer: "none",
		Positive:     "",
		Negative:     "",
		Insufficient: "no machine-readable status is attached to any figure, so no comparison to a ground truth is constructible",
		Gap:          []string{"the mutations this claim would need, altering a tier/value association, have no association to alter"},
	},
	"RPL-C005": {
		Basis: NoBasis, Layer: "none. No type governs what reaches the screen",
		Positive:     "a fully priced corpus correctly discloses nothing, so the detection is not firing on everything",
		Negative:     "a mixed corpus discloses nothing either, which is the defect",
		Insufficient: "the fixture itself: price is unavailable while usage is present and provider-reported",
		Assumption: []string{
			"ASSUMPTION: a refusal is conveyed by the words unpriced, not priced or excluded. A refusal worded otherwise is understated by this measurement",
		},
		Gap: []string{"only `replay cost` was measured. burn, advise, route, diff, errors, warnings and the TUI were not"},
	},
	"RPL-C026": {
		Basis: NoBasis, Layer: "surface.Class, surface.WriteContract",
		Positive:     "empty corpus and zero-writes-under-unknown-contract both reach ClassUndetermined with a reason",
		Negative:     "a clean corpus with a sourced contract does NOT refuse",
		Insufficient: "ClassUndetermined is itself the insufficient-evidence result",
		Gap:          []string{"internal only. RPL-C005 shows the refusal does not always reach the user"},
	},
	"RPL-C034": {
		Basis: Observed, Layer: "costUnit.Unpriced, a presence field serialized with the unit",
		Positive:     "after 6794522, both arms report 40,000 re-billed tokens and `since` names the larger session, cold and warm",
		Negative:     "five mutations all kill: Unpriced propagation, the token assignment, the warm reconstruction, the genuine-zero collapse, and unpriced rows re-entering the dollar statistics",
		Insufficient: "2 of the 10 originally flagged sites are inside PriceForAt itself and are delegation rather than consumption, so the consumer count is 8",
		Gap: []string{
			"7 of 8 consumers were censused from source and NEVER measured at the user-visible boundary. TOKEN-PRICES.md:53 is established as satisfied ONLY at `replay cost` and `replay since` for the re-billed token count",
			"the 9 compliant sites use at least four different conventions, so there is still no shared contract in code",
		},
	},
	"RPL-C033": {
		Basis: Observed, Layer: "none. transcript.Usage has no presence field for the TTL split",
		Positive:     "an explicit 5m split and an explicit 1h split price differently (EC4)",
		Negative:     "absent and present-0/0 are byte-identical downstream (EC1)",
		Insufficient: "the absent case IS the insufficient-evidence case, and it is given a default rather than refused",
		Assumption: []string{
			"ASSUMPTION: an absent TTL breakdown means the provider's 5-minute default. RESPONSIBILITY: the provider. REPLAY VERIFIES: nothing. ON VIOLATION: the cache-write leg is understated by 60%, undisclosed",
		},
		Gap: []string{"whether providers omit the breakdown for 1h writes is unmeasured and needs live traffic"},
	},
	"RPL-C030": {
		Basis: NoBasis, Layer: "none. Two per-session counters, unpriced and unreadable",
		Positive:     "MX2 and MX9 disclose; MX4 discloses the wholly unpriced session",
		Negative:     "MX1, a fully measurable corpus, discloses nothing",
		Insufficient: "MX2 is the insufficient-evidence case and is handled correctly",
		Gap:          []string{"contradictory and stale evidence were INAPPLICABLE: the ledger reader has no two-source reconciliation for one quantity, so there is nothing to contradict"},
	},
	"RPL-C031": {
		Basis: NoBasis, Layer: "none",
		Positive:     "MX1 proves the disclosure can be silent correctly",
		Negative:     "MX3 and MX10 both show 2 of 4 records unmeasurable with unpriced=0 and no disclosure, RE-MEASURED after 6794522",
		Insufficient: "MX4 isolates the boundary: a wholly unpriced session IS disclosed, a half unpriced one is not",
		Gap: []string{
			"only `replay cost` was measured",
			"UNCHANGED BY 6794522. That repair works at session granularity; this claim is about record granularity. A record-level counter is a separate change and was not made",
		},
	},
	"RPL-C032": {
		Basis: Observed, Layer: "none. A price-table lookup, not an inference from cost",
		Positive:     "CW1's genuine-zero state: a session with every usage field zero on a PRICED model reports unpriced=0 and keeps its dollar figures",
		Negative:     "CW1's unpriced state on the same corpus shape reports unpriced=1, so the two are distinguishable",
		Insufficient: "before 6794522 both states reached the same counter; MX8 reported unpriced=1 for a genuine zero",
		Gap:          []string{"only session cost in `replay cost` was revalidated. Other sites may or may not collapse the same way and were not re-measured"},
	},
	"RPL-C037": {
		Basis: Reconstructed, Layer: "none. No type carries break-level priceability; `breaks` counts priceable and unpriceable alike",
		Positive:     "P1/Q and P2/Q2, corpora with 20,000 of 40,000 break tokens unpriceable, where the oracle computes 50% coverage and the report now discloses 50%; plus the second-gate corpus, where every break priced and the figure covers 33%; plus the unequal-deficit corpus at 96% token against 25% count coverage",
		Negative:     "P1/P and P2/R, fully priceable corpora at 100% coverage, which must never carry a partial-coverage disclosure; and a zero-break corpus, which must carry neither a disclosure nor a re-billed token line",
		Insufficient: "a corpus where request-level and break-level coverage happen to coincide does not discriminate, which is why P1/Q is built so they diverge (2 of 3 requests priced against 1 of 2 break token halves)",
		Assumption: []string{
			"that a future disclosure would be recognisable to the frozen detector. It is proven against a planted sentence and against C035's sentence, which it must not match, but not against a shipped one",
		},
		Gap: []string{
			"the five rejected candidates in TestC037_TheOracleRejectsEveryWrongCandidate are SIMULATED disclosures, retained as an oracle self-check. They are NOT the mutation evidence; ten mutations of the landed implementation are, and are recorded in the evidence file",
			"only the transcript path of `replay cost` was measured. `replay cost --usage` has the same two-population shape at costusage.go:194/196 and renders the pair at costusage.go:267; it was found by the post-repair audit and is neither measured nor repaired. `replay advise`, the card and the share surface also read re-billed figures and were not examined",
		},
	},
	"RPL-C008": {
		Basis: Observed, Layer: "surface.WriteContract, Observables.WriteFieldPresent",
		Positive:     "absent write counter classifies III-no-observable",
		Negative:     "present-and-zero counter classifies undetermined, a different verdict",
		Insufficient: "present-and-zero under an unknown contract is itself the refusal",
	},
	"RPL-C011": {
		Basis: Reconstructed, Layer: "none. No type carries a tier on the figure",
		Positive:     "production arithmetic agrees with a longhand reimplementation",
		Negative:     "identical provider usage prices at $0.020400 on list and $0.016320 negotiated",
		Insufficient: "a price table with no matching model returns not-found rather than a figure",
		Assumption:   []string{"ASSUMPTION: the compiled price table matches the rate the account was actually billed on. Replay cannot check this and the figure is wrong by exactly the discount if it does not."},
	},
	"RPL-C012": {
		Basis: NoBasis, Layer: "none",
		Positive:     "the destination scan finds the URLs that are present",
		Negative:     "a planted /v1/invoices is detected; an ordinary /v1/messages is not",
		Insufficient: "not applicable: the result IS the absence",
	},
	"RPL-C013": {
		Basis: NoBasis, Layer: "none",
		Positive:     "the forecast detector fires on a planted savings forecast",
		Negative:     "no user-facing string in the tree matches",
		Insufficient: "",
		Gap:          []string{"only Go string literals are scanned; README and docs prose are not"},
	},
	"RPL-C015": {
		Basis: NoBasis, Layer: "advisor.Status",
		Positive:     "KindCacheBreaks and KindHotFile return AdviceOnly with a zero realized figure",
		Negative:     "4 of 4 remaining kinds ARE scoreable on the same falling share, so the refusal is a decision not an incapacity",
		Insufficient: "AdviceOnly is itself the insufficient-evidence result",
	},
	"RPL-C016": {
		Basis: NoBasis, Layer: "none",
		Positive:     "the improvement detector fires on a planted claim",
		Negative:     "no user-facing string asserts it",
		Insufficient: "R10 returned INVALID EXPERIMENT at a preregistered ceiling; the treatment arm never ran",
		Gap:          []string{"no substrate with demonstrated control-arm headroom exists, so the question cannot currently be asked"},
	},
	"RPL-C019": {
		Basis: NoBasis, Layer: "none. No account identity exists to govern",
		Positive:     "",
		Negative:     "",
		Insufficient: "the scan no longer treats every TenantID as an account identity by spelling alone: internal/tenancy.TenantID (commit 646736d) is Replay's own internal ownership/namespace partition for hosted multi-tenancy, not a provider-account correlation handle, so the detector exempts it specifically while still catching AccountID, OrgID, OrganizationID, OrganisationID, ProjectID and WorkspaceID, and any TenantID that is locally redeclared rather than the registered primitive",
		Gap:          []string{"not a gap in testing. There is no endpoint to test."},
	},
	"RPL-C020": {
		Basis: Observed, Layer: "transcript.Request.IDMeasured",
		Positive:     "A to A and B to B survive; four distinct ids across two sessions report zero duplicates end to end",
		Negative:     "three mutations on the join all kill the suite",
		Insufficient: "a synthesised id is counted unjoinable rather than merged or dropped silently",
		Assumption: []string{
			"ASSUMPTION: provider request ids used by this join are globally unique across the scope in which Replay correlates them. RESPONSIBILITY: the provider. PROVIDER DOCUMENTATION: no authoritative statement of this guarantee was located during this campaign. REPLAY OBSERVES: session id, agent id, model, usage and timestamps alongside the id, and uses none of them to corroborate a merge. ON VIOLATION: two distinct requests are silently counted as one, with no disclosure. Classified as an EXTERNAL ASSUMPTION, not a Replay-established fact.",
		},
		Gap: []string{"whether corroborating on session id before merging is worth its cost is a production decision and is not taken here"},
	},
	"RPL-C021": {
		Basis: NoBasis, Layer: "none. Enforced by the shape of the type",
		Positive:     "the type is found and carries fields",
		Negative:     "no field or method name offers a total; has* applicability flags are present",
		Insufficient: "has* flags keep 'not applicable to this surface' distinct from zero",
	},
	"RPL-C022": {
		Basis: NoBasis, Layer: "none",
		Positive:     "the scan counts remote destinations; zero would fail it",
		Negative:     "the loopback skip is proven not to swallow api.anthropic.com",
		Insufficient: "",
		Assumption:   []string{"ASSUMPTION: a destination reachable only through a URL literal in Go source. A host assembled at runtime from parts would not be seen."},
		Gap:          []string{"static only. No sandboxed dynamic run was performed."},
	},
	"RPL-C024": {
		Basis: Reconciled, Layer: "none",
		Positive:     "every attributed token lands in exactly one bucket",
		Negative:     "the RebillLabel row is sized exactly to the excess rather than absorbing it",
		Insufficient: "",
		Gap:          []string{"conservation pins the total and says nothing about the split, as the repository's own test states"},
	},
}
