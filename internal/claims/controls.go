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

const (
	Observed      EvidenceBasis = "OBSERVED"      // read from a provider's own record
	Reconstructed EvidenceBasis = "RECONSTRUCTED" // derived from local artifacts
	Reconciled    EvidenceBasis = "RECONCILED"    // two independent sources compared, not merged
	Inferred      EvidenceBasis = "INFERRED"      // supported by evidence that cannot settle it
	NoBasis       EvidenceBasis = "NOT_APPLICABLE"
)

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
	"RPL-C005": {
		Basis: NoBasis, Layer: "surface.Class, surface.WriteContract",
		Positive:     "empty corpus, and zero writes under an unknown contract, both reach ClassUndetermined with a reason",
		Negative:     "a clean corpus with a sourced contract does NOT refuse, so the refusal is not firing on everything",
		Insufficient: "the refusal IS the insufficient-evidence result here",
		Gap:          []string{"refusal correctness is established per-site by scripts/refusal-reachability and not globally", "the user-visible output after a refusal is not inspected; only the internal verdict is"},
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
		Insufficient: "a grep of every non-test Go file finds no AccountID, TenantID, OrgID, ProjectID or WorkspaceID",
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
