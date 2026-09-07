package transcript

import "testing"

// The billed total is the sum of the per-turn deltas, never the cumulative.
//
// Codex writes both: last_token_usage is the delta and total_token_usage is a
// running total. The running total is REBASED when the context is compacted,
// so on a compacted session it is smaller than what was actually paid for.
// Measured across 148 local rollouts, the deltas and the final cumulative
// agree on 146 and disagree on exactly the 2 that compacted — on the larger,
// 392,301,269 billed against 198,661,781 reported, a 49% under-count.
//
// The fixture reproduces that in miniature: deltas of 1100 + 2200 + 550 = 3850
// against a final cumulative of 550.
func TestCodexBillsTheDeltasNotTheRebasedCumulative(t *testing.T) {
	s, err := ParseCodexFile("codexdata/compacted.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := s.Billed.Total(), 3850; got != want {
		t.Errorf("billed total = %d, want %d: the cumulative counter is rebased on "+
			"compaction and must never be read as a bill", got, want)
	}
	if len(s.Compactions) != 1 {
		t.Errorf("compactions = %d, want 1", len(s.Compactions))
	}
	if !s.Rebased {
		t.Error("a session that compacted must be marked Rebased, so a reader knows " +
			"the cumulative counter and the billed total are different quantities")
	}
}

// The session-open token_count carries no usage, and that is by design.
//
// Every session begins with a token_count whose info is null, because nothing
// has been spent yet; it carries rate_limits instead. Measured locally: all 26
// such events were the first of their session. Treating them as corrupt turned
// 0.46% of events into 96% of sessions being unreportable in another tool, and
// discarded the only live quota signal the format carries.
func TestCodexOpeningEventIsQuotaNotCorruption(t *testing.T) {
	s, err := ParseCodexFile("codexdata/compacted.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if s.Skipped != 0 {
		t.Errorf("Skipped = %d, want 0: an info:null token_count is a rate-limit "+
			"record, not an unparseable line", s.Skipped)
	}
	if s.Quota == nil {
		t.Fatal("the rate_limits payload was dropped; it is the only live quota " +
			"signal in this format")
	}
	if s.Quota.PrimaryUsedPercent != 4.0 || s.Quota.PrimaryWindowMinutes != 300 {
		t.Errorf("primary window = %.1f%% over %dm, want 4.0%% over 300m",
			s.Quota.PrimaryUsedPercent, s.Quota.PrimaryWindowMinutes)
	}
	if s.Quota.PlanType != "plus" {
		t.Errorf("plan = %q, want %q", s.Quota.PlanType, "plus")
	}
}

// A subset larger than its superset means the record is corrupt.
//
// cached_input_tokens is a share of input_tokens, not an addition to it, and
// reasoning_output_tokens is a share of output_tokens. Either exceeding its
// parent is impossible, so the snapshot is rejected and counted rather than
// quietly added to a total somebody will later be billed against.
func TestCodexRejectsImpossibleSubsets(t *testing.T) {
	s, err := ParseCodexFile("codexdata/impossible.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if s.Billed.Total() != 0 {
		t.Errorf("billed = %d, want 0: a record with cached > input must not be "+
			"counted", s.Billed.Total())
	}
	if s.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1: the rejection must be recorded, not silent",
			s.Skipped)
	}
}
