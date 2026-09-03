package learn

import (
	"fmt"
	"math"

	"github.com/RedRobotKK/Buffy/internal/transcript"
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
}

// Graduate judges a trial. It needs enough sessions in each arm, a
// difference between the arms above noise, and a realized saving of at
// least graduationTolerance of the prediction.
func Graduate(policy string, costs []ArmCost, predicted float64, minSessions int) *TrialReport {
	if minSessions <= 0 {
		minSessions = DefaultTrialMinSessions
	}
	var treated, control []float64
	for _, c := range costs {
		switch c.Arm {
		case "treated":
			treated = append(treated, c.CostPerNewToken)
		case "control":
			control = append(control, c.CostPerNewToken)
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
	switch {
	case len(treated) < minSessions || len(control) < minSessions:
		r.Reason = fmt.Sprintf("not judged: fewer than %d sessions in an arm (%d treated, %d control)", minSessions, len(treated), len(control))
	case !separated(treated, control):
		r.Reason = "not graduated: the arms are not separated above noise"
	case r.Realized < graduationTolerance*predicted:
		r.Reason = fmt.Sprintf("not graduated: realized saving %.0f%% is under half of the predicted %.0f%%", r.Realized*100, predicted*100)
	default:
		r.Graduated = true
		r.Reason = fmt.Sprintf("graduated: realized saving %.0f%% against a predicted %.0f%%", r.Realized*100, predicted*100)
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
