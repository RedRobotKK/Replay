package transcript

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The billed total is the reconstruction, never the rebased cumulative.
//
// Codex writes both. total_token_usage is a running total the client keeps,
// and it is REBASED when the context is compacted, so on a compacted session
// it is smaller than what was actually paid for.
//
// CORRECTION, 2026-09-18. The paragraph here used to read: "Measured across
// 148 local rollouts, the deltas and the final cumulative agree on 146 and
// disagree on exactly the 2 that compacted." The measurement was real and the
// attribution was wrong. Re-measured across 204 rollouts, the two that
// disagreed carry ZERO context_compacted events, and so does the third that
// has since appeared. They disagree because TokenCountEvent broadcasts session
// state and repeats the previous last_token_usage when no response preceded
// it, which this reader summed again. Nothing compacted. The sentence that
// explained the gap away is the reason the over-count survived, and it is left
// here corrected rather than deleted, because the wrong explanation is the
// part worth remembering.
//
// The fixture reproduces a real compaction in miniature: deltas of
// 1100 + 2200 + 550 = 3850 against a final cumulative of 550.
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

// A total with no breakdown is absent, not zero.
//
// 15 records in a 148-session corpus carry total_tokens in the thousands with
// every component at zero. The components are missing, not measured as none,
// and adding zero for them silently loses the tokens. Counting the refusal is
// the difference between a total that is short and a total that says so.
func TestCodexRefusesATotalWithNoBreakdown(t *testing.T) {
	s, err := ParseCodexFile("codexdata/absent-breakdown.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if s.Billed.Total() != 0 {
		t.Errorf("billed = %d, want 0: a record with no breakdown cannot be priced",
			s.Billed.Total())
	}
	if s.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1: 11,014 tokens went missing without a word",
			s.Skipped)
	}
}

// A cache break on Codex is reported, not inferred.
//
// For Anthropic this engine hashes the prefix and works out that it changed.
// Codex states cached_input_tokens on every turn, so a break is visible
// directly: a turn whose cached share collapses against the turn before it.
// Measured across 148 local sessions, 80 such collapses re-read 10,635,679
// tokens cold, and none of the three largest followed a compaction.
func TestCodexSeesACacheBreakWithoutInferringIt(t *testing.T) {
	s, err := ParseCodexFile("codexdata/break.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Breaks) != 1 {
		t.Fatalf("breaks = %d, want 1: cached went 98%% to 3%% between turns", len(s.Breaks))
	}
	b := s.Breaks[0]
	// 12,000 read, 300 of it cached, so 11,700 arrived cold that the previous
	// turn would have had for a tenth of the price.
	if b.ColdTokens != 11700 {
		t.Errorf("cold tokens = %d, want 11700", b.ColdTokens)
	}
	if b.BeforeShare < 0.97 || b.AfterShare > 0.05 {
		t.Errorf("shares = %.2f -> %.2f, want ~0.98 -> ~0.03", b.BeforeShare, b.AfterShare)
	}
}

// A session that never cached well has no break to report.
func TestCodexDoesNotInventBreaksWhereTheCacheNeverHeld(t *testing.T) {
	s, err := ParseCodexFile("codexdata/compacted.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range s.Breaks {
		if b.BeforeShare < 0.5 {
			t.Errorf("reported a break from a %.0f%% cached turn; that is not a break, "+
				"it is a session that was never warm", 100*b.BeforeShare)
		}
	}
}

// The billing-basis predicate.
//
// `last_token_usage` is not a per-turn delta. It is session state: Codex holds
// a TokenUsageInfo and `append_last_usage` does `total += last; last = last`,
// so `last` is a SNAPSHOT of the most recent append. TokenCountEvent carries
// that state and takes no usage argument, and three of its four emission sites
// involve no append at all — a rate-limit update, a context estimate, and a
// context-exhaustion rebase. A broadcast with no append therefore repeats the
// previous snapshot verbatim, and summing the field counts that response twice.
//
// Replay does not deduplicate, because in the older format a re-emission and a
// genuinely identical consecutive response are indistinguishable from the
// fields present. It refuses the session instead: where the billing basis is
// not established, there is no billed figure rather than a guessed one.

// writeRollout puts a rollout in a temp dir and parses it. The fixtures on
// disk cover the shapes that existed before this predicate; these cover the
// ones it turns on, and they are built here rather than added to codexdata so
// that the corpus of checked-in fixtures still describes the reader's inputs
// rather than its gates.
func writeRollout(t *testing.T, lines ...string) *CodexSession {
	t.Helper()
	path := filepath.Join(t.TempDir(), "rollout-2026-09-18T00-00-00-test.jsonl")
	body := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	s, err := ParseCodexFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// tokenCount renders one token_count event with the given delta and cumulative.
func tokenCount(lastIn, lastOut, totIn, totOut int) string {
	return fmt.Sprintf(`{"timestamp":"2026-09-18T00:00:00.000Z","type":"event_msg","payload":`+
		`{"type":"token_count","info":{`+
		`"total_token_usage":{"input_tokens":%d,"cached_input_tokens":0,"output_tokens":%d,`+
		`"reasoning_output_tokens":0,"total_tokens":%d},`+
		`"last_token_usage":{"input_tokens":%d,"cached_input_tokens":0,"output_tokens":%d,`+
		`"reasoning_output_tokens":0,"total_tokens":%d},"model_context_window":258400}}}`,
		totIn, totOut, totIn+totOut, lastIn, lastOut, lastIn+lastOut)
}

const rolloutMeta = `{"timestamp":"2026-09-18T00:00:00.000Z","type":"session_meta",` +
	`"payload":{"id":"019d0f52-8ca6-7f00-0000-00000000000a","cli_version":"0.155.0-alpha.2.6"}}`

// An ordinary session whose reconstruction reconciles has a billing basis.
func TestAReconcilingSessionEstablishesTheBillingBasis(t *testing.T) {
	s := writeRollout(t, rolloutMeta,
		tokenCount(1000, 100, 1000, 100),
		tokenCount(2000, 200, 3000, 300),
	)
	if !s.BasisEstablished {
		t.Errorf("a session with no re-emission, no unreadable usage and a reconciling "+
			"total has no billing basis; billed %d, reported %d",
			s.Billed.Total(), s.Reported.Total())
	}
	if got, want := s.Billed.Total(), 3300; got != want {
		t.Errorf("billed = %d, want %d", got, want)
	}
}

// A re-emitted snapshot refuses the basis, and the figure is NOT repaired by
// dropping the repeat. Deduplication is not authorized: the older format
// cannot tell a re-emission from an identical consecutive response.
func TestAReEmittedSnapshotRefusesTheBillingBasis(t *testing.T) {
	repeat := tokenCount(2000, 200, 3000, 300)
	s := writeRollout(t, rolloutMeta,
		tokenCount(1000, 100, 1000, 100),
		repeat,
		repeat,
	)
	if s.BasisEstablished {
		t.Error("a session carrying a re-emitted usage snapshot reported a billing basis")
	}
	if got, want := s.Billed.Total(), 5500; got != want {
		t.Errorf("billed = %d, want %d: the repeat must not be silently dropped, "+
			"because dropping it is the deduplication this contract refuses", got, want)
	}
	if s.Rebased {
		t.Error("a re-emission was recorded as a compaction; nothing compacted here")
	}
}

// A compacted session keeps its basis without reconciling.
//
// Codex rebases the cumulative on compaction, so the reconstruction is SUPPOSED
// to exceed it. Requiring reconciliation there would refuse every compacted
// session for doing exactly what the format says it does.
func TestACompactedSessionKeepsItsBasisWithoutReconciling(t *testing.T) {
	s, err := ParseCodexFile("codexdata/compacted.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if !s.Rebased {
		t.Fatal("the fixture no longer compacts, so this test proves nothing")
	}
	if s.Billed.Total() == s.Reported.Total() {
		t.Fatal("the fixture reconciles, so the exception it exists to test is not exercised")
	}
	if !s.BasisEstablished {
		t.Errorf("a compacted session lost its billing basis for failing a reconciliation "+
			"the rebase makes impossible; billed %d, reported %d",
			s.Billed.Total(), s.Reported.Total())
	}
}

// A failed reconciliation is never read as a compaction.
func TestReconciliationFailureIsNotReadAsCompaction(t *testing.T) {
	s := writeRollout(t, rolloutMeta,
		tokenCount(1000, 100, 1000, 100),
		tokenCount(2000, 200, 2500, 250),
	)
	if s.Rebased {
		t.Error("a mismatch was inferred to be a rebase; only an observed " +
			"context_compacted event may set that")
	}
	if s.BasisEstablished {
		t.Error("an ordinary session whose reconstruction does not reconcile reported a basis")
	}
}

// Unreadable usage evidence refuses the basis.
func TestUnreadableUsageEvidenceRefusesTheBillingBasis(t *testing.T) {
	s, err := ParseCodexFile("codexdata/absent-breakdown.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if s.BasisEstablished {
		t.Error("a session whose only usage record could not be read reported a billing basis")
	}
}

// A malformed line alone does not refuse the basis.
//
// Skipped counts three different refusals: an unparsable line, an unparsable
// payload, and an unreadable usage record. Only the third is evidence the
// billing reconstruction needed. Reusing the aggregate would refuse a
// defensible figure because something unrelated in the file did not parse.
func TestAMalformedLineAloneDoesNotRefuseTheBillingBasis(t *testing.T) {
	s := writeRollout(t, rolloutMeta,
		tokenCount(1000, 100, 1000, 100),
		`{"timestamp":"2026-09-18T00:00:00.000Z","type":"event_msg","payload":`,
		tokenCount(2000, 200, 3000, 300),
	)
	if s.Skipped != 1 {
		t.Fatalf("Skipped = %d, want 1: the malformed line must still be counted", s.Skipped)
	}
	if !s.BasisEstablished {
		t.Error("an unparsable line unrelated to usage took the billing basis down with it")
	}
	if got, want := s.Billed.Total(), 3300; got != want {
		t.Errorf("billed = %d, want %d", got, want)
	}
}

// A compacted session carrying a re-emission still refuses.
//
// This is the case where clause (a) is the only thing standing. Compaction
// exempts a session from reconciliation, because Codex rebases the cumulative
// and the reconstruction is supposed to exceed it — so on a compacted session
// the mismatch that catches a re-emission everywhere else says nothing. Drop
// the re-emission check and this session bills a doubled figure with no test
// objecting, which is exactly what a mutant removing it proved.
func TestACompactedSessionCarryingAReEmissionStillRefuses(t *testing.T) {
	repeat := tokenCount(2000, 200, 3000, 300)
	s := writeRollout(t, rolloutMeta,
		tokenCount(1000, 100, 1000, 100),
		`{"timestamp":"2026-09-18T00:00:00.000Z","type":"event_msg",`+
			`"payload":{"type":"context_compacted"}}`,
		repeat,
		repeat,
	)
	if !s.Rebased {
		t.Fatal("the fixture did not compact, so the exemption is not exercised")
	}
	if s.BasisEstablished {
		t.Error("a compacted session repeated a usage snapshot and still claimed a billing " +
			"basis; the compaction exemption covers reconciliation only, never re-emission")
	}
}
