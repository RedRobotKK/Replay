package main

import (
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/learn"
	"github.com/RedRobotKK/Replay/internal/transcript"
	"github.com/RedRobotKK/Replay/internal/version"
)

// The counterfactual claim, authorized by ADR-0025.
//
// Two rules shape every line below and neither is negotiable.
//
// THE REQUEST IS VALIDATED BEFORE ANY EVIDENCE IS READ. An absent or
// unrecognised alternative is a defect in the request, not in the corpus, so
// it returns errUsage and exit 1 with nothing examined and nothing printed.
// Only once the request is well formed does coverage decide whether the claim
// can be made, and a well-formed request the corpus cannot support returns
// errNotMeasured and exit 4. Those are opposite situations. Returning one code
// for both is the bug costgate.go records from 2026-09-13, where a CI runner
// with no transcripts failed a merge exactly as an agent that had wasted money
// did.
//
// THE SAVING IS SESSION-LEVEL. learn.Score simulates a candidate over one
// session's main lane and returns a share of that lane's effective tokens.
// Nothing observable says how to divide that share between the individual
// breaks inside the lane, so nothing here tries. Breaks are still listed, as
// they were before this flag existed; none of them carries a figure.

// counterfactualAlternative resolves the requested alternative to a catalog
// candidate, byte for byte.
//
// The catalog is the vocabulary (ADR-0025, locked contract §6), and it is read
// here rather than copied, so a catalog that grows or moves cannot leave a
// stale accepted set behind. No case folding, no trimming, no aliasing and no
// family selection: `ttl` names a family and `as-run` names the observed
// baseline, and neither is an alternative to compare against.
func counterfactualAlternative(name string) (learn.Candidate, error) {
	if name == "" {
		return learn.Candidate{}, fmt.Errorf(
			"--counterfactual <alternative> is required, and Replay does not choose one for you: %w", errUsage)
	}
	catalog := learn.Catalog()
	for _, c := range catalog {
		if c.Name == name {
			return c, nil
		}
	}
	names := make([]string, 0, len(catalog))
	for _, c := range catalog {
		names = append(names, c.Name)
	}
	return learn.Candidate{}, fmt.Errorf(
		"%q is not an alternative this build can score. Exactly one of: %s: %w",
		name, strings.Join(names, ", "), errUsage)
}

// writeCounterfactual appends the session-level counterfactual to a report that
// has already been written.
//
// Every refusal below names the coverage state that produced it, drawn from the
// closed vocabulary the contract fixes: no new limitation enum exists and none
// is created here.
//
// drifted is ST-1's answer for this lane's model, decided by the caller across
// the whole corpus because drift is not a property one session can see. It is
// passed in rather than computed here so that this file holds no second copy of
// the staleness rule.
func writeCounterfactual(rep *analysis.LaneReport, alt learn.Candidate, drifted bool, w io.Writer) error {
	// calibration. Only `passing` admits a claim that depends on the engine
	// reproducing the provider. The absence of evidence is its own state and
	// is neither a pass nor a failure, so it is named separately.
	cal := rep.Calibration
	switch {
	case cal == nil || !cal.HasEvidence():
		return fmt.Errorf(
			"NOT MEASURED: calibration no-evidence. No turn in this lane had a predecessor to compare "+
				"against, so the engine reproduced nothing and an alternative cannot be scored: %w", errNotMeasured)
	case !cal.Passes():
		return fmt.Errorf(
			"NOT MEASURED: calibration failing. The engine reproduced %.1f%% of this lane's cache reads, "+
				"below the %.0f%% required before an alternative is scored: %w",
			cal.MatchRate()*100, analysis.CalibrationThreshold*100, errNotMeasured)
	case drifted:
		// This lane calibrates. Its model no longer does, which is a different
		// fact and a stricter one: ST-1 stops scoring alternatives for a model
		// whose newest lanes stopped reproducing, and a counterfactual is
		// exactly the alternative scoring ST-1 names. Scoring it here because
		// this one lane happened to pass would route around the stop.
		return fmt.Errorf(
			"NOT MEASURED: calibration drifted. This lane calibrates, but the provider's behaviour "+
				"changed for %s across the sessions read, so alternatives are not scored for that "+
				"model: %w", laneModel(rep), errNotMeasured)
	}

	score, ok := learn.Score(rep.Session, []learn.Candidate{alt})
	if !ok {
		// learn.Score refuses for three reasons and only one of them can arrive
		// here. A missing main lane was already turned into a skipped file by
		// forEachSession, and failing calibration was already refused above by
		// name, so what is left is a lane that parsed and calibrated and still
		// carries no effective tokens.
		//
		// That is an absence, and it does not get to borrow a coverage state.
		// `completeness partial-parse` is a claim about transcript.Session.
		// Skipped, the observable §3.2 names for it, so it is asserted here only
		// when that observable actually establishes it. Where it does not, the
		// refusal says what is true instead: there was nothing to measure a
		// share against.
		if rep.Session != nil && rep.Session.Skipped > 0 {
			return fmt.Errorf(
				"NOT MEASURED: completeness partial-parse. The reader could not interpret %d line(s) of "+
					"this session, and what it could read carries no effective tokens to score an "+
					"alternative against: %w", rep.Session.Skipped, errNotMeasured)
		}
		return fmt.Errorf(
			"NOT MEASURED: this session's main lane carries no effective tokens, so there is no scale "+
				"for a saving to be a share of. Nothing was mis-read and nothing is missing from the "+
				"rules; there is simply nothing here to measure: %w", errNotMeasured)
	}
	saving, ok := score.Saving[alt.Name]
	if !ok {
		// Unreachable: learn.Score fills Saving for every candidate it is given
		// and it was given this one. Kept as a guard, and labelled for what it
		// would actually be. A scorer that returned no entry would be a defect
		// in this build, not a fact about the provider or the transcript, and
		// calling it a calibration failure would report a measurement that was
		// never taken.
		return fmt.Errorf(
			"NOT MEASURED: %s was not scored on this session, though the session itself scored. That is "+
				"a defect in this build rather than a statement about the evidence: %w",
			alt.Name, errNotMeasured)
	}

	// Numerical stability is a claim gate, not a coverage dimension. Replaying
	// a policy equivalent to as-run does not reproduce it bit for bit, so a
	// candidate that changes nothing still scores a few billionths. Below the
	// floor there is no evidence, only arithmetic residue.
	scale := score.AsRun.EffectiveTokens
	if scale <= 0 {
		scale = float64(score.AsRun.PromptTokens)
	}
	if math.Abs(saving) < learn.MinMeaningfulShare*scale {
		// The reason names the gate, not a mood. The closed set of limitation
		// reasons is the coverage states plus this one plus the break causes,
		// and "numerical stability" is what this gate is called.
		return fmt.Errorf(
			"NOT MEASURED: numerical stability. The difference between %s and as-run on this session is "+
				"below the replay's own floor, so it is arithmetic residue rather than a saving: %w",
			alt.Name, errNotMeasured)
	}

	tokens := saving * score.AsRun.EffectiveTokens
	verb := "would have avoided"
	if tokens < 0 {
		verb = "would have cost a further"
		tokens = -tokens
	}

	// provenance and pricing basis are independent observables and are printed
	// as such. A transcript-derived figure is still a measurement; a dollar
	// figure additionally needs the model to be in the price table, and its
	// absence suppresses the money without touching the tokens.
	var b strings.Builder
	fmt.Fprintf(&b, "\nCounterfactual (%s), session lane: %s %.0f effective tokens",
		alt.Name, verb, math.Round(tokens))
	// Whether a dollar figure may be printed at all is decided before the
	// arithmetic runs, because the arithmetic cannot tell a complete basis from
	// a partial one: `saving` is a share of EVERY effective token in the lane
	// while CostUSD covers only the requests a rules document priced, so on a
	// mixed lane the two sides count different requests. dollarNote holds that
	// decision and says which state produced it.
	priced, unpriced := lanePricing(rep.Lane)
	if note, dollars := dollarNote(priced, unpriced, promptSideUSD(score.AsRun)); !dollars {
		b.WriteString(note)
	} else {
		// Prompt-side only; counterfactualDollars carries the reasoning and the
		// sign. The magnitude is printed because `verb` above already carries
		// the direction, and the function returns a signed figure so the sign
		// is testable on its own rather than being decided by the sentence.
		usd, _ := counterfactualDollars(saving, score.AsRun)
		fmt.Fprintf(&b, " ($%.2f at list, prompt-side)", math.Abs(usd))
	}
	b.WriteString(".\n")

	// CF-3, reconciled with RE-5 under ADR-0025. header() already states the
	// unchanged-agent-behaviour assumption on this surface; the population, the
	// read date and the build belong to the figure itself.
	fmt.Fprintf(&b, "  Population: 1 session lane, this one. Read %s. Build %s. Rules %s.\n",
		time.Now().UTC().Format("2006-01-02"), version.String(), cachemodel.RulesVersionInEffect())
	if score.Estimated[alt.Name] {
		fmt.Fprintf(&b, "  Provenance: %s; this score depends on the byte-to-token fit.\n", rep.Session.Source.Tier())
	} else {
		fmt.Fprintf(&b, "  Provenance: %s.\n", rep.Session.Source.Tier())
	}
	b.WriteString("  The saving is for the lane. It is not divided between the breaks above, because " +
		"nothing observable says how to divide it.\n")

	_, err := io.WriteString(w, b.String())
	return err
}

// counterfactualDollars prices a saving share on the prompt side only.
//
// CostUSD prices prompt tokens as EffectiveTokens at the input rate and output
// at the output rate, and the four legs on the tally sum to it, so
// CostUSD - OutputUSD is exactly the prompt-side cost and equals
// EffectiveTokens * inputPrice. A layout alternative cannot change what the
// model wrote back, so that is the only leg it can move. Scaling the whole
// bill by a prompt-side share charges the alternative for output tokens it
// never touched: on the bundled corpus output is 43.6% of cost, and the first
// version of this overstated by 77%. RE-5 says the same thing from the other
// end, that alternatives are scored on prompt-side tokens only.
//
// THE SIGN IS RETURNED, NOT DISCARDED. A negative share means the alternative
// would have cost more, which is the most useful result a reader can get. What
// to call that is the caller's sentence to write; this function must not
// decide it by throwing the sign away, and a test can hold it here in a way it
// cannot hold a magnitude embedded in prose.
//
// priced is false when no prompt-side cost is known, which suppresses the
// money without touching the token figure.
func counterfactualDollars(saving float64, t analysis.Tally) (usd float64, priced bool) {
	promptSide := promptSideUSD(t)
	if promptSide <= 0 {
		return 0, false
	}
	return saving * promptSide, true
}

// promptSideUSD is the one place the prompt side of a bill is defined.
//
// The four legs on the tally sum to CostUSD, so subtracting the output leg
// leaves exactly the prompt side, which is EffectiveTokens at the input rate.
// It is a function rather than an expression because two callers need it now -
// the arithmetic below and the note that decides whether the arithmetic runs at
// all - and two copies of a definition are two things that can drift. A drift
// here would let one of them price a lane the other had already declined to.
func promptSideUSD(t analysis.Tally) float64 { return t.CostUSD - t.OutputUSD }

// laneModel names the model a lane ran on, which is the unit ST-1 measures
// drift in. An empty name belongs to no model and matches no stale entry.
func laneModel(rep *analysis.LaneReport) string {
	if rep == nil || rep.Lane == nil || len(rep.Lane.Requests) == 0 {
		return ""
	}
	return rep.Lane.Requests[0].Model
}

// lanePricing counts how many of a lane's requests a rules document prices.
//
// It asks the same oracle the cost basis asks, at the same model and the same
// instant: analysis.AsRun feeds every request through Tally.AddAt, which prices
// it only `if p, ok := cachemodel.PriceForAt(model, at); ok`. A request that
// misses adds its tokens to EffectiveTokens and nothing to CostUSD. That
// asymmetry is the whole reason this function exists, and reading it from the
// same predicate is what keeps the two from drifting apart.
//
// The three answers are the three states the contract already names, so nothing
// here is a new vocabulary: every request priced is `complete`, none priced is
// `pricing_basis absent`, and a mix is `completeness partial-pricing`.
func lanePricing(lane *transcript.Lane) (priced, unpriced int) {
	if lane == nil {
		return 0, 0
	}
	for _, req := range lane.Requests {
		if _, ok := cachemodel.PriceForAt(req.Model, req.Timestamp); ok {
			priced++
			continue
		}
		unpriced++
	}
	return priced, unpriced
}

// dollarNote decides whether a dollar figure may be printed, and when it may
// not, says what actually stopped it.
//
// The four inputs it can see are two counts and one sum, and each answer below
// is a statement about those and nothing else. That matters because the note is
// read as a claim about evidence: a reader told `pricing_basis absent` goes to
// the rules document to look for a missing model. Saying it of a lane whose
// models all resolved sends them after a model that is already there, and
// saying it of a lane with no requests at all says a lookup failed when no
// lookup ever happened.
//
// So absence keeps its own answer, the way Calibration.HasEvidence keeps one
// next door: an absence is not a failure and not a success, and a caller must
// not be able to mistake it for either. No new vocabulary is introduced. Where
// a coverage state IS the reason, the note names that state, and where it is
// not, the note does not borrow one.
func dollarNote(priced, unpriced int, promptSideUSD float64) (note string, dollars bool) {
	switch {
	case priced == 0 && unpriced == 0:
		// Nothing was read, so nothing was priced and nothing was unpriced.
		return " (no contributing request was read, so there is nothing to price)", false
	case priced == 0:
		// Requests exist and no rules document resolved for any of them. This
		// is the coverage state, and here it is true.
		return fmt.Sprintf(" (pricing_basis absent: no rules document carries the model on any of "+
			"the %d contributing requests, so no dollar figure)", unpriced), false
	case unpriced > 0:
		// §4.5: not partial-pricing where dollars are claimed. The token share
		// covers every request; the price covers only some of them.
		return fmt.Sprintf(" (completeness partial-pricing: %d of %d contributing requests are on a "+
			"model no rules document carries, so no dollar figure)", unpriced, priced+unpriced), false
	case promptSideUSD <= 0:
		// Every model resolved, so the pricing basis is documented. There is
		// simply no prompt-side cost for a prompt-side change to move, which is
		// an arithmetic fact about this lane rather than a gap in its evidence.
		return " (pricing_basis documented, but this lane carries no prompt-side cost to scale, " +
			"so no dollar figure)", false
	}
	return "", true
}
