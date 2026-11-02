// Package evidenceclaim is the narrow claim-evaluation layer that feeds
// occurrence-pinned evidence into Replay's existing claim vocabulary.
//
// It exists to make one truth precise:
//
//	Replay observes evidence, understands the limits of that evidence, and
//	derives a claim that is never stronger than what the evidence can prove.
//
// It is NOT a second claim system. It defines no claim registry, no claim
// lifecycle, no ledger and no four-state verifier. It maps the frozen
// occurrence verifier (internal/evidenceverify, Candidate B) into the existing
// State Ledger vocabulary (internal/stateledger.Standing / Outcome), and it
// retains the verifier's fine-grained result so the four epistemic states are
// never flattened away.
//
// # The one claim this bundle proves
//
//	"This transcript records this exact tool-result occurrence as flagged as
//	 an error (is_error: true)."
//
// It proves exactly that — what the immutable bytes record at the pinned
// occurrence. It does NOT prove the tool actually failed, that an external
// side effect failed, or that Replay's own classification is correct. A
// recorded flag is not an execution outcome, and this package refuses to
// equate them.
//
// # Coverage model
//
// Coverage is what evidence dimensions the caller actually holds, never a
// confidence score. FULL means raw bytes plus whole-artifact binding are
// available. NONE means only decoded state exists. A claim evaluated at NONE
// or UNSUPPORTED coverage returns UNRESOLVED / NOT-RUN and must never fall
// back to a decoded bool such as transcript.Block.IsError — that fallback is
// the exact defect this bundle exists to prevent.
package evidenceclaim

import (
	"fmt"
	"strings"

	"github.com/RedRobotKK/Replay/internal/evidenceverify"
	"github.com/RedRobotKK/Replay/internal/stateledger"
)

// Coverage is which evidence dimensions a caller actually holds. It is a
// statement about availability, never about confidence.
type Coverage string

const (
	// CoverageFull: raw immutable bytes plus a trustworthy whole-artifact
	// binding (the locator's transcript digest) are available.
	CoverageFull Coverage = "FULL"
	// CoveragePartial: raw bytes exist but some identity dimension (source
	// identity, or whole-artifact digest trust) is incomplete. Occurrence
	// verification still runs; the result records the partial basis.
	CoveragePartial Coverage = "PARTIAL"
	// CoverageNone: only decoded/normalized state remains. Occurrence-level
	// evidence cannot be established and is never guessed.
	CoverageNone Coverage = "NONE"
	// CoverageUnsupported: the surface cannot provide the evidence class at
	// all (e.g. a plain-text log with no JSON occurrence).
	CoverageUnsupported Coverage = "UNSUPPORTED"
)

func (c Coverage) valid() bool {
	switch c {
	case CoverageFull, CoveragePartial, CoverageNone, CoverageUnsupported:
		return true
	default:
		return false
	}
}

// Result is one evaluated claim. It carries both the coarse claim-level map
// (Standing / Outcome, in Replay's existing vocabulary) and the retained
// fine-grained verifier result (Verdict / Reason), because the existing
// vocabulary cannot by itself distinguish an ambiguous conflict from an
// unavailable basis — both read UNRESOLVED. That distinction is load-bearing
// and is therefore carried, not flattened.
type Result struct {
	// ClaimText is the narrowest proposition evaluated.
	ClaimText string
	// SourceID names the artifact/session the evidence belongs to. It is an
	// identifier, never transcript content.
	SourceID string
	// Coverage is what evidence dimensions were actually available.
	Coverage Coverage

	// Standing and Outcome are the claim result mapped into the State Ledger
	// vocabulary.
	Standing stateledger.Standing
	Outcome  stateledger.Outcome

	// Verdict and Reason are the retained four-state occurrence result. They
	// are zero (UNVERIFIED) and empty when no verification ran.
	Verdict evidenceverify.Verdict
	Reason  evidenceverify.Reason

	// ArtifactSHA256 is the whole-artifact digest the locator bound, hex,
	// empty when no binding existed.
	ArtifactSHA256 string
	// ValueSpan is the pinned value's byte span, zero when unestablished.
	ValueSpan evidenceverify.Span

	// Explanation is stable, content-free prose: ids, hashes, spans, reason
	// codes. It never contains transcript content.
	Explanation string
}

// The documented mapping. A bijection at the Outcome level preserves all four
// states; Standing is a coarse three-way (Supported / Contradicted /
// Unresolved) and deliberately does not distinguish ambiguous from
// unavailable-basis, which is why Verdict and Reason are retained beside it.
var verdictToOutcome = map[evidenceverify.Verdict]stateledger.Outcome{
	evidenceverify.Confirmed:  stateledger.Supports,
	evidenceverify.Refuted:    stateledger.Contradicts,
	evidenceverify.Ambiguous:  stateledger.Inconclusive,
	evidenceverify.Unverified: stateledger.NotRun,
}

var verdictToStanding = map[evidenceverify.Verdict]stateledger.Standing{
	evidenceverify.Confirmed:  stateledger.Supported,
	evidenceverify.Refuted:    stateledger.Contradicted,
	evidenceverify.Ambiguous:  stateledger.Unresolved,
	evidenceverify.Unverified: stateledger.Unresolved,
}

// Evaluate runs one occurrence-pinned claim end to end:
//
//	raw evidence -> artifact identity -> physical occurrence ->
//	occurrence verification -> coverage -> claim result -> explanation
//
// It is the only entry point and the only place a claim result is produced.
// When coverage is NONE or UNSUPPORTED it returns UNRESOLVED / NOT-RUN without
// consulting the verifier and without any fallback to decoded state.
//
// The claim text and source id are supplied by the caller and must not
// contain transcript content (see Explanation's contract); sourceID is an
// identifier, and claimText is the fixed proposition this bundle is scoped to.
func Evaluate(claimText, sourceID string, raw []byte, loc evidenceverify.Locator, pred evidenceverify.Predicate, coverage Coverage) Result {
	r := Result{
		ClaimText: claimText,
		SourceID:  sourceID,
		Coverage:  coverage,
		Standing:  stateledger.Unresolved,
		Outcome:   stateledger.NotRun,
	}

	if !coverage.valid() {
		// REPLAY-SECURITY: an unknown coverage code must fail closed, never
		// default to a stronger reading.
		r.Explanation = "coverage: unknown code, treated as no basis"
		return r
	}

	// REPLAY-SECURITY: missing or unsupported raw evidence is lack of
	// coverage, never a guessed claim. This is the I4 / H4 / H8 / H10 guard:
	// there is no path from "no raw bytes" to a substantive claim result, and
	// specifically no fallback to transcript.Block.IsError.
	if coverage == CoverageNone || coverage == CoverageUnsupported {
		r.Explanation = fmt.Sprintf("no occurrence-level evidence: coverage %s; decoded state is not evidence", coverage)
		return r
	}

	vr := evidenceverify.Verify(raw, loc, pred)
	r.Verdict = vr.Verdict
	r.Reason = vr.Reason
	r.Standing = verdictToStanding[vr.Verdict]
	r.Outcome = verdictToOutcome[vr.Verdict]

	if vr.Verdict != evidenceverify.Unverified {
		r.ArtifactSHA256 = fmt.Sprintf("%x", loc.TranscriptSHA256)
		r.ValueSpan = vr.ValueSpan
	}

	switch vr.Verdict {
	case evidenceverify.Confirmed:
		r.Explanation = "the pinned bytes record the claimed literal; the claim is supported by this evidence"
	case evidenceverify.Refuted:
		r.Explanation = "the pinned bytes record the opposite literal; the claim is contradicted by this evidence"
	case evidenceverify.Ambiguous:
		r.Explanation = "the key occurs multiple times in one object scope with conflicting values; the claim is unresolved"
	case evidenceverify.Unverified:
		r.Explanation = fmt.Sprintf("evidence basis not established (%s); the claim is not evaluated", vr.Reason)
	}
	if coverage == CoveragePartial {
		r.Explanation += "; coverage is PARTIAL, so the basis is not fully bound"
	}
	return r
}

// Render returns a stable, content-free explanation. It names the claim, the
// source id, the artifact digest, the coverage, the verdict, the stable
// reason, and the coarse standing — never any transcript content.
func (r Result) Render() string {
	var b strings.Builder
	fmt.Fprintf(&b, "CLAIM      %s\n", r.ClaimText)
	fmt.Fprintf(&b, "  source   %s\n", r.SourceID)
	fmt.Fprintf(&b, "  coverage %s\n", r.Coverage)
	if r.ArtifactSHA256 != "" {
		fmt.Fprintf(&b, "  artifact sha256 %s\n", r.ArtifactSHA256)
	}
	if r.ValueSpan != (evidenceverify.Span{}) {
		fmt.Fprintf(&b, "  value span [%d,%d)\n", r.ValueSpan.Start, r.ValueSpan.End)
	}
	fmt.Fprintf(&b, "  verdict  %s\n", r.Verdict)
	if r.Reason != "" {
		fmt.Fprintf(&b, "  reason   %s\n", r.Reason)
	}
	fmt.Fprintf(&b, "  standing %s (%s)\n", r.Standing, r.Outcome)
	fmt.Fprintf(&b, "  why      %s\n", r.Explanation)
	return b.String()
}

// ClaimProposition is the fixed, narrow claim text this bundle evaluates. It
// is exported so callers and tests share one wording rather than drift.
const ClaimProposition = "this transcript records this exact tool-result occurrence as flagged as an error (is_error: true)"
