package analysis

import "math"

// Switch is what moving to another model costs before it saves.
type Switch struct {
	// CostUSD is the one-time price of writing the shared prefix again on the
	// destination, which starts with a cold cache because cache entries are
	// keyed per model.
	CostUSD float64 `json:"switch_cost_usd"`
	// SavingPerTurnUSD is the steady-state difference, once the prefix is warm.
	SavingPerTurnUSD float64 `json:"saving_per_turn_usd"`
	// PaybackTurns is how many turns of that saving repay the switch. Zero
	// when there is nothing to repay it.
	PaybackTurns int  `json:"payback_turns,omitempty"`
	PaysBack     bool `json:"pays_back"`
	// WithinObserved is whether the payback lands inside the run actually
	// measured. A model that is cheaper per turn and needs more turns than the
	// corpus contains is the wrong advice, and it is the shape a price-only
	// comparison reports as a win.
	WithinObserved bool `json:"within_observed"`
}

// Payback measures whether a model switch repays its own cost.
//
// switchCostUSD is the one-time prefix write on the destination; observedUSD
// and projectedUSD are what the run cost on the source and would cost on the
// destination once warm, over turns.
func Payback(switchCostUSD, observedUSD, projectedUSD float64, turns int) Switch {
	s := Switch{CostUSD: switchCostUSD}
	if turns <= 0 {
		// No run, no per-turn figure. Dividing here would manufacture one.
		return s
	}
	s.SavingPerTurnUSD = (observedUSD - projectedUSD) / float64(turns)
	if s.SavingPerTurnUSD <= 0 {
		// The destination is not cheaper, so the switch cost is never repaid
		// and there is no quotient worth taking. Reported as no payback rather
		// than as a negative or enormous turn count.
		return s
	}
	s.PaysBack = true
	// Rounded up, and at least one: a partial turn does not repay a switch, and
	// a payback reported a turn early is advice to switch on a task that loses
	// money. A free switch still costs the first turn to realise.
	s.PaybackTurns = int(math.Max(1, math.Ceil(switchCostUSD/s.SavingPerTurnUSD)))
	s.WithinObserved = s.PaybackTurns <= turns
	return s
}
