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

// A re-emission is counted, never dropped in silence, and it has its own
// counter.
//
// The counting rule is unchanged and its reason is unchanged: a format change
// cannot pass without a word. Mutation testing caught the original gap, where
// deleting the increment from the re-emission branch left every other test
// green, which meant a future Codex that stopped advancing its cumulative could
// erase every turn in a session and report nothing unusual.
//
// What changed is WHERE it is counted. This used to increment Skipped, whose
// own comment scopes it to "records this reader refused" and whose consumers
// describe it as unreadable or unparsable. A re-emission is neither: the record
// parsed, the usage was readable, and it was left out because it had already
// been billed. Three distinct facts in one counter made all three
// indistinguishable, and the surfaces reporting it said things that were not
// true of a re-emission.
func TestCodexCountsAReEmissionSeparatelyFromSkipped(t *testing.T) {
	s, err := ParseCodexFile("codexdata/reemitted.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := s.ReEmitted, 2; got != want {
		t.Errorf("re-emitted = %d, want %d: a re-emission must be counted, because a "+
			"reader that drops records in silence cannot tell a quiet format change "+
			"from a quiet session", got, want)
	}
	if got, want := s.Skipped, 0; got != want {
		t.Errorf("skipped = %d, want %d: a re-emission is not a refused record. The "+
			"record parsed and its usage was readable; it was not billed again because "+
			"the cumulative had not moved", got, want)
	}
}

// Skipped keeps its own meaning, and an unreadable usage record still lands
// there rather than in the re-emission count.
//
// The two are checked from opposite sides on purpose. One test showing a
// re-emission leaves Skipped alone would still pass if every unreadable record
// had quietly moved to ReEmitted.
func TestAnUnreadableUsageRecordIsSkippedAndNotAReEmission(t *testing.T) {
	s, err := ParseCodexFile("codexdata/absent-breakdown.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := s.Skipped, 1; got != want {
		t.Errorf("skipped = %d, want %d: a total with no breakdown is unreadable, "+
			"which is what Skipped counts", got, want)
	}
	if got, want := s.ReEmitted, 0; got != want {
		t.Errorf("re-emitted = %d, want %d: an unreadable record is not a repeat of "+
			"anything", got, want)
	}
}

// Two genuine turns carrying equal numbers touch neither counter.
//
// The cumulative advanced on both, so both are turns and neither is a repeat.
// This is the case the discriminator exists to protect: keying on the delta
// would call the second one a duplicate and drop a turn that was really billed.
func TestTwoIdenticalTurnsAreNeitherSkippedNorReEmitted(t *testing.T) {
	s, err := ParseCodexFile("codexdata/twinturns.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if s.ReEmitted != 0 || s.Skipped != 0 {
		t.Errorf("re-emitted = %d and skipped = %d, want 0 and 0: both turns advanced "+
			"the cumulative, so both are turns", s.ReEmitted, s.Skipped)
	}
	if got, want := s.Turns, 2; got != want {
		t.Errorf("turns = %d, want %d", got, want)
	}
}
