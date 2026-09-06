package analysis

import "testing"

// Switching models is not free, and every router prices it as if it were.
//
// `replay route` already compares what the same work would cost on another
// model: topology, tokenizer dilation, the crossover share. All of it is steady
// state. None of it charges for the move.
//
// The move is not free because a cache entry is keyed per model. The
// destination starts cold, so the whole shared prefix is written again at the
// write multiple before a single read discount applies. Measured on live
// traffic 2026-09-06, one fresh request wrote 156,813 tokens of prefix. A
// cheaper model has to earn that back before it saves anything, and on a short
// task it never will.
//
// So the useful answer is not "cheaper per turn" but "cheaper after N turns,
// and this session was shorter than N".

// SW-1: a cheaper destination pays back after the switch cost is earned.
//
// PASS: 100 of one-time cost against 25 saved per turn is 4 turns.
// FAIL: rounding down to 4 when the cost is not an exact multiple - a payback
// reported one turn early is advice to switch on a task that loses money.
func TestSW1_PaybackIsTheSwitchCostOverThePerTurnSaving(t *testing.T) {
	got := Payback(100, 200, 100, 4) // 100 saved over 4 turns = 25/turn
	if !got.PaysBack {
		t.Fatalf("a cheaper destination must pay back eventually: %+v", got)
	}
	if got.PaybackTurns != 4 {
		t.Errorf("payback = %d turns, want 4: %+v", got.PaybackTurns, got)
	}

	// And the rounding direction, which is the part that costs money.
	up := Payback(101, 200, 100, 4) // 101/25 = 4.04 turns
	if up.PaybackTurns != 5 {
		t.Errorf("payback = %d, want 5: a partial turn does not repay the switch, so this "+
			"must round up", up.PaybackTurns)
	}
}

// SW-2: a destination that is not cheaper never pays back.
//
// PASS: PaysBack false, no turn count, no division.
// FAIL: a negative or enormous payback from dividing by a zero or negative
// saving - which is how "switch to this model" appears under a model that
// costs more.
func TestSW2_NoSavingMeansNoPayback(t *testing.T) {
	for _, c := range []struct {
		name                string
		observed, projected float64
	}{
		{"destination costs more", 100, 150},
		{"destination costs the same", 100, 100},
	} {
		got := Payback(50, c.observed, c.projected, 10)
		if got.PaysBack {
			t.Errorf("%s: reported a payback: %+v", c.name, got)
		}
		if got.PaybackTurns != 0 {
			t.Errorf("%s: carried a turn count with no payback: %+v", c.name, got)
		}
	}
}

// SW-3: a free switch still needs a saving to pay back.
//
// The degenerate case where the prefix is empty. Payback is immediate, but only
// if the destination is actually cheaper.
//
// PASS: zero cost and a real saving pays back on turn 1, not turn 0.
// FAIL: turn 0, which reads as "already paid back" before any turn has run.
func TestSW3_AFreeSwitchPaysBackOnTheFirstTurn(t *testing.T) {
	got := Payback(0, 200, 100, 4)
	if !got.PaysBack || got.PaybackTurns != 1 {
		t.Errorf("a free switch to a cheaper model pays back on turn 1, got %+v", got)
	}
}

// SW-4: a payback longer than the session actually observed is the headline.
//
// This is the whole point. A model that is 20% cheaper per turn and needs
// forty turns to repay its own switch is the wrong choice for a corpus whose
// sessions run twelve. Reporting only "20% cheaper" is how a router loses
// money while displaying a lower price.
//
// PASS: WithinObserved false when payback exceeds the turns measured.
// FAIL: silence, leaving a true per-turn saving to be read as advice.
func TestSW4_PaybackBeyondTheObservedSessionIsFlagged(t *testing.T) {
	// 1000 one-time, 10 saved per turn over 12 turns => 100 turns to repay.
	long := Payback(1000, 132, 12, 12)
	if !long.PaysBack {
		t.Fatalf("it does pay back, eventually: %+v", long)
	}
	if long.WithinObserved {
		t.Errorf("payback of %d turns against %d observed must not be reported as within "+
			"the session: %+v", long.PaybackTurns, 12, long)
	}

	short := Payback(10, 200, 100, 12) // 10/25 per turn => 1 turn
	if !short.WithinObserved {
		t.Errorf("a switch repaid on turn %d of 12 is within the session: %+v",
			short.PaybackTurns, short)
	}
}

// SW-5: no turns, no arithmetic.
//
// PASS: nothing claimed.
// FAIL: a per-turn saving computed by dividing by zero.
func TestSW5_NoTurnsMeansNoClaim(t *testing.T) {
	got := Payback(100, 200, 100, 0)
	if got.PaysBack || got.PaybackTurns != 0 || got.SavingPerTurnUSD != 0 {
		t.Errorf("with no turns observed there is nothing to divide: %+v", got)
	}
}
