// Package claims is the register of what Replay asserts it can do.
//
// It exists because a capability claim is not a feature. A feature either
// compiles or does not; a claim is true only under stated conditions, against
// stated evidence, with a stated oracle, and it carries a boundary beyond
// which it is false. The boundary is the part that goes missing first, so
// DoesNotEstablish is a required field here and an empty one fails a test.
//
// This register does NOT introduce a new epistemic vocabulary. The repository
// already has several, each local to the package that needed it:
// surface.Class, stateledger.Standing, surface.WriteContract,
// evidencepin.Status, probe Reading.Outcome. Inventing a seventh that spanned
// them would be a rewrite wearing the clothes of a test campaign. Each claim
// below names the vocabulary that actually decides it, in Vocabulary, and the
// Result values here are about the CLAIM's verification status, not about any
// individual measurement's epistemic class.
package claims

// Result is the verification status of a claim, which is a different question
// from the epistemic class of any single figure the claim is about.
type Result string

const (
	// Established means a positive control passes, a negative control fails, and
	// at least one mutation that should break it does break it.
	Established Result = "ESTABLISHED"
	// Bounded means true only inside conditions named in Scope. The commonest
	// honest outcome, and the one that over-claiming erases.
	Bounded Result = "BOUNDED"
	// Refuted means a controlled fixture contradicts the claim as worded.
	Refuted Result = "REFUTED"
	// NotMeasured means no endpoint exists. Not evidence of absence.
	NotMeasured Result = "NOT_MEASURED"
	// NoEndpoint is stronger than NotMeasured. No endpoint can exist from the
	// evidence Replay has access to, for a structural reason that is stated.
	NoEndpoint Result = "NO_ENDPOINT"
	// Unresolved means evidence is present but does not settle it.
	Unresolved Result = "UNRESOLVED"
	// DeliberateNonClaim means the product explicitly declines to assert this, and
	// the decline is itself load-bearing and tested.
	DeliberateNonClaim Result = "DELIBERATE_NON_CLAIM"
)

// Claim is one externally meaningful assertion about what Replay can do.
type Claim struct {
	ID string
	// Text is the claim as a buyer would state it, not as marketing states it.
	Text string
	// Asserted is where the product says this, file:line. Empty means the
	// claim is inferred from behaviour rather than asserted in prose, which
	// is itself worth knowing.
	Asserted []string
	// Scope names the conditions under which the claim is evaluated at all.
	Scope string
	// Establishes is what a passing verification would license a reader to believe.
	Establishes []string
	// DoesNotEstablish is the boundary. REQUIRED. An empty one fails
	// TestEveryClaimStatesItsBoundary, because the boundary is the first
	// thing to go missing and the most expensive thing to lose.
	DoesNotEstablish []string
	// Vocabulary names the existing in-repo type that decides this claim,
	// e.g. "surface.Class". Empty means no in-code vocabulary governs it,
	// which is a finding rather than an omission.
	Vocabulary string
	// Oracle describes how the claim is checked INDEPENDENTLY of the code
	// under test. "self" is a legal value and marks a gap, loudly.
	Oracle string
	// Tests are the Go test functions that exercise this claim. Every name
	// listed must exist; TestEveryClaimNamesTestsThatExist enforces it.
	Tests []string
	// Result is the verified status.
	Result Result
	// Why explains the Result in one or two sentences. REQUIRED.
	Why string
}
