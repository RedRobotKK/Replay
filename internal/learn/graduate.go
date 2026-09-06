package learn

import (
	"fmt"
	"math"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// Graduation (DR-2): a policy the proxy tried live has to show, on
// sessions it was applied to, a saving that holds against the sessions
// held out as controls. The arms are compared on the cost of new content,
// effective tokens per token of new input and output, because a policy
// that clears old tool results shrinks the prompt but never the work the
// user brought, so the denominator is the same under either arm.
const (
	// DefaultTrialMinSessions is how many sessions each arm needs before
	// the trial is judged.
	DefaultTrialMinSessions = 5
	// graduationTolerance is the share of the predicted saving the
	// realized saving must reach.
	graduationTolerance = 0.5
)

// ArmCost is one session's cost per token of new content and its arm.
type ArmCost struct {
	SessionID string
	Arm       string
	// CostPerNewToken is effective tokens divided by new input and output.
	CostPerNewToken float64
	// ErrorShare and ReadsAfterClear are the OUTCOME. Both were already
	// computed - errors.go classifies failed edits, anchor-not-found and
	// repeated identical calls; rereads.go counts content the agent fetched
	// again after it had been cleared - and neither was joined to an arm.
	//
	// Without them a policy that cut spend a fifth while doubling failed
	// edits graduated, and the report said so approvingly. Cheaper is not
	// better if the work got worse.
	ErrorShare      float64
	ReadsAfterClear int
}

// armCostsOf carries a score's cost AND its outcome into the trial.
//
// Split out so the wiring is testable. A guard fed nothing is decoration, and
// this project has shipped exactly that shape before: a payment gate no test
// imported, where deleting it left 25 of 25 tests green.
func armCostsOf(scores []SessionScore) []ArmCost {
	out := make([]ArmCost, 0, len(scores))
	for _, s := range scores {
		out = append(out, ArmCost{
			SessionID: s.SessionID, Arm: s.Arm, CostPerNewToken: s.CostPerNewToken,
			ErrorShare: s.ErrorShare, ReadsAfterClear: s.ReadsAfterClear,
		})
	}
	return out
}

// ArmOf returns a session's trial arm as the ledger and pins recorded it.
func ArmOf(s *transcript.Session) string { return s.Trial }

// TrialReport is the outcome of the live trial across the corpus.
type TrialReport struct {
	Policy  string `json:"policy"`
	Treated int    `json:"treated_sessions"`
	Control int    `json:"control_sessions"`
	// TreatedCost and ControlCost are the arms' mean cost per new token
	// with their bands.
	TreatedCost     float64    `json:"treated_cost_per_new_token"`
	TreatedInterval [2]float64 `json:"treated_interval"`
	ControlCost     float64    `json:"control_cost_per_new_token"`
	ControlInterval [2]float64 `json:"control_interval"`
	// Realized is the relative saving of treated over control; Predicted
	// the saving the selection promised.
	Realized  float64 `json:"realized_saving"`
	Predicted float64 `json:"predicted_saving"`
	Graduated bool    `json:"graduated"`
	Reason    string  `json:"reason"`
	// OutcomeObserved is false when no session carried an outcome signal.
	// Distinct from an outcome that was measured and unchanged: one is
	// evidence, the other its absence, and reporting them the same way is how
	// a cost tool says a policy is safe when nobody looked.
	OutcomeObserved   bool    `json:"outcome_observed"`
	TreatedErrorShare float64 `json:"treated_error_share"`
	ControlErrorShare float64 `json:"control_error_share"`
	TreatedRereads    float64 `json:"treated_reads_after_clear"`
	ControlRereads    float64 `json:"control_reads_after_clear"`
}

// outcomeTolerance is how much worse an arm's outcome may be before a saving
// stops counting as a win.
//
// Declared, not measured. Noise in an error share across a handful of sessions
// is real, so a policy is not blocked for a rounding difference - but a fifth
// worse is not rounding.
const outcomeTolerance = 1.20

// Graduate judges a trial. It needs enough sessions in each arm, a
// difference between the arms above noise, and a realized saving of at
// least graduationTolerance of the prediction.
func Graduate(policy string, costs []ArmCost, predicted float64, minSessions int) *TrialReport {
	if minSessions <= 0 {
		minSessions = DefaultTrialMinSessions
	}
	var treated, control []float64
	var tErr, cErr, tRe, cRe []float64
	seen := false
	for _, c := range costs {
		if c.ErrorShare > 0 || c.ReadsAfterClear > 0 {
			seen = true
		}
		switch c.Arm {
		case "treated":
			treated = append(treated, c.CostPerNewToken)
			tErr = append(tErr, c.ErrorShare)
			tRe = append(tRe, float64(c.ReadsAfterClear))
		case "control":
			control = append(control, c.CostPerNewToken)
			cErr = append(cErr, c.ErrorShare)
			cRe = append(cRe, float64(c.ReadsAfterClear))
		}
	}
	if len(treated) == 0 && len(control) == 0 {
		return nil
	}
	r := &TrialReport{Policy: policy, Treated: len(treated), Control: len(control), Predicted: predicted}
	r.TreatedCost, r.TreatedInterval = meanInterval(treated)
	r.ControlCost, r.ControlInterval = meanInterval(control)
	if r.ControlCost > 0 {
		r.Realized = 1 - r.TreatedCost/r.ControlCost
	}
	r.OutcomeObserved = seen
	r.TreatedErrorShare, _ = meanInterval(tErr)
	r.ControlErrorShare, _ = meanInterval(cErr)
	r.TreatedRereads, _ = meanInterval(tRe)
	r.ControlRereads, _ = meanInterval(cRe)
	worseErrors := r.ControlErrorShare > 0 && r.TreatedErrorShare > r.ControlErrorShare*outcomeTolerance
	worseRereads := r.ControlRereads > 0 && r.TreatedRereads > r.ControlRereads*outcomeTolerance
	switch {
	case len(treated) < minSessions || len(control) < minSessions:
		r.Reason = fmt.Sprintf("not judged: fewer than %d sessions in an arm (%d treated, %d control)", minSessions, len(treated), len(control))
	case !separated(treated, control):
		r.Reason = "not graduated: the arms are not separated above noise"
	case seen && worseErrors:
		r.Reason = fmt.Sprintf("not graduated: the treated arm saved %.0f%% and its error share "+
			"rose from %.3f to %.3f. Cheaper is not better if the work got worse",
			r.Realized*100, r.ControlErrorShare, r.TreatedErrorShare)
	case seen && worseRereads:
		r.Reason = fmt.Sprintf("not graduated: the treated arm saved %.0f%% and re-read cleared "+
			"content %.1f times against %.1f, which is the agent failing to use what it was given",
			r.Realized*100, r.TreatedRereads, r.ControlRereads)
	case r.Realized < graduationTolerance*predicted:
		r.Reason = fmt.Sprintf("not graduated: realized saving %.0f%% is under half of the predicted %.0f%%", r.Realized*100, predicted*100)
	default:
		r.Graduated = true
		r.Reason = fmt.Sprintf("graduated: realized saving %.0f%% against a predicted %.0f%%",
			r.Realized*100, predicted*100)
		if !seen {
			r.Reason += "; no outcome signal was observed in either arm, so this is a cost " +
				"result only"
		}
	}
	return r
}

// separated reports whether treated costs are below control costs by more
// than the noise of both arms: the difference of means exceeds two of its
// standard errors.
func separated(treated, control []float64) bool {
	mt, it := meanInterval(treated)
	mc, ic := meanInterval(control)
	if math.IsInf(it[0], 0) || math.IsInf(ic[0], 0) {
		return false
	}
	seT := (it[1] - it[0]) / (2 * intervalWidth)
	seC := (ic[1] - ic[0]) / (2 * intervalWidth)
	diff := mc - mt
	return diff > intervalWidth*math.Sqrt(seT*seT+seC*seC)
}
