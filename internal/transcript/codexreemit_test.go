package transcript

import "testing"

// Codex re-emits a token_count carrying a turn it has already reported.
//
// The re-emission is byte-identical in `last_token_usage` and leaves
// `total_token_usage` standing still, so the session's own cumulative counter
// says no new turn happened. Billing the delta again doubles the turn.
//
// Measured across 176 rollout files in one local corpus on 2026-09-17: 1,860
// such re-emissions, concentrated in three long sessions. On the worst of them
// the reader summed 392,199,759 tokens against a final cumulative of
// 198,661,781, a 1.97x over-report. Skipping records where the cumulative did
// not advance reconciled all three sessions to their cumulative exactly, and
// took the corpus from three conservation failures to none.
//
// The log is self-consistent and the reader was not: across every record where
// the cumulative advanced, the advance equalled `last_token_usage.total_tokens`
// 4,846 times out of 4,846. The cumulative IS the running sum of the distinct
// deltas. Some of them were just counted more than once.
func TestCodexBillsAReEmittedTurnOnlyOnce(t *testing.T) {
	s, err := ParseCodexFile("codexdata/reemitted.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := s.Billed.Total(), 3300; got != want {
		t.Errorf("billed total = %d, want %d: a re-emitted turn was billed twice", got, want)
	}
	if got, want := s.Turns, 2; got != want {
		t.Errorf("turns = %d, want %d: a re-emission is not a turn", got, want)
	}
}

// Billed and Reported are different quantities and must agree here.
//
// Reported is the client's own final cumulative. Billed is the sum of the
// deltas. On a session with no compaction they describe the same spend, and a
// disagreement means the reader dropped a record or counted one twice. This is
// the conservation law the corpus test applies, asserted on one small file so
// it can fail for a reason a reader can see.
func TestCodexBilledMatchesTheCumulativeWhenNothingRebased(t *testing.T) {
	s, err := ParseCodexFile("codexdata/reemitted.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if s.Rebased {
		t.Fatal("this fixture does not compact; the law applies")
	}
	if s.Billed.Total() != s.Reported.Total() {
		t.Errorf("billed %d != reported %d with no compaction",
			s.Billed.Total(), s.Reported.Total())
	}
}

// A distinct turn that happens to repeat a previous turn's numbers is still a
// turn. The signal is the cumulative standing still, not the delta repeating.
func TestCodexBillsTwoIdenticalTurnsWhenTheCumulativeAdvanced(t *testing.T) {
	s, err := ParseCodexFile("codexdata/twinturns.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := s.Billed.Total(), 2200; got != want {
		t.Errorf("billed total = %d, want %d: two real turns with equal deltas "+
			"must both be billed", got, want)
	}
	if got, want := s.Turns, 2; got != want {
		t.Errorf("turns = %d, want %d", got, want)
	}
}

// A refused record is counted, never dropped in silence.
//
// `Skipped` exists so a format change cannot pass without a word: its own
// comment says "Non-zero is not an error, but it is reported, because a format
// change must not pass silently." A re-emission is refused, so it must be
// counted. Mutation testing caught this: deleting the Skipped++ from the
// re-emission branch left every other test green, which meant a future Codex
// that stopped advancing its cumulative could erase every turn in a session and
// report nothing unusual.
func TestCodexCountsARefusedReEmission(t *testing.T) {
	s, err := ParseCodexFile("codexdata/reemitted.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := s.Skipped, 2; got != want {
		t.Errorf("skipped = %d, want %d: a refused re-emission must be counted, "+
			"because a reader that drops records in silence cannot tell a quiet "+
			"format change from a quiet session", got, want)
	}
}
