package transcript

import (
	"strings"
	"testing"
)

// The second door into the defect ollamaunmeasured_test.go closed.
//
// That file records the original filing and why it was wrong: the defect was
// reported as "atoiOr returning 0 on an unparseable number", and the note
// observes that atoiOr cannot fire, because every count the regexes capture is
// (\d+) and strconv.Atoi then fails only on overflow.
//
// Correct about the instance, and it stopped there. atoiOr's default was left
// in place on the grounds that it was unreachable, and it is not: (\d+) matches
// a twenty-digit run, Atoi refuses anything above 2^63, and the default turns
// that refusal into the number zero. Zero on the prefix is exactly the claim
// OU-1 exists to forbid — a full cache miss asserted about a request nobody
// measured — reached through a different line.
//
// A number the parser could not read is not a small number.

const unreadableCount = "99999999999999999999"

// OU-4: an unreadable n_past leaves the prefix unmeasured.
//
// PASS: PrefixMeasured() false, CacheHitRate() not ok, ContextTokens() not ok.
// FAIL: CachedPrefix 0 with PrefixMeasured() true and a 0% hit rate reported as
// measured, which is what shipped. Restore it by writing
// `cur.CachedPrefix = atoiOr(m[1], 0)` in place of the atoiOK branch.
func TestOU4_AnUnreadableNPastIsNotAMeasuredZero(t *testing.T) {
	log := "slot launch_slot_: id  0 | task 0 | processing task\n" +
		"slot update_slots: id  0 | task 0 | n_past was set to " + unreadableCount + "\n" +
		"slot print_timing: id  0 | task 0 | prompt eval time =     10.00 ms /   100 tokens\n" +
		"slot print_timing: id  0 | task 0 |        eval time =     20.00 ms /    50 tokens\n" +
		"slot print_timing: id  0 | task 0 |       total time =     30.00 ms /   150 tokens\n"
	rs, err := ParseOllamaLog(strings.NewReader(log))
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 1 {
		t.Fatalf("got %d request(s), want 1: the request is real, only its prefix "+
			"is unreadable, and dropping it would lose 150 tokens of measured work", len(rs))
	}
	r := rs[0]
	if r.PrefixMeasured() {
		t.Errorf("PrefixMeasured = true with CachedPrefix %d. The server wrote a "+
			"number this parser could not convert; reporting it as a measured "+
			"prefix asserts something about the request rather than about the log.",
			r.CachedPrefix)
	}
	if rate, ok := r.CacheHitRate(); ok {
		t.Errorf("CacheHitRate reported %v as measured. %v is the worst reading "+
			"available and nobody took it.", rate, rate)
	}
	if ctx, ok := r.ContextTokens(); ok {
		t.Errorf("ContextTokens reported %d as known; without the prefix the "+
			"context size is not derivable", ctx)
	}
	// The measured half must survive: this is a request whose prefix is unknown,
	// not a request that did not happen.
	if r.Total != 150 || r.PromptEval != 100 || r.Generated != 50 {
		t.Errorf("the readable numbers were lost too: %+v", r)
	}
}

// OU-5: an unreadable token count drops the block rather than zeroing it.
//
// A prompt_eval_count that will not convert is not a prompt of zero tokens.
// Kept, it produced a record whose halves (0 + 50) do not reach its own total
// (150), which every average downstream then divides by.
//
// PASS: no request, because there is no readable one here.
// FAIL: one request with PromptEval 0 and Total 150, which is what shipped.
func TestOU5_AnUnreadableTokenCountDropsTheBlock(t *testing.T) {
	for _, c := range []struct {
		name string
		log  string
	}{
		{"prompt eval", "slot print_timing: prompt eval time =     10.00 ms /   " + unreadableCount + " tokens\n" +
			"slot print_timing:        eval time =     20.00 ms /    50 tokens\n" +
			"slot print_timing:       total time =     30.00 ms /   150 tokens\n"},
		{"eval", "slot print_timing: prompt eval time =     10.00 ms /   100 tokens\n" +
			"slot print_timing:        eval time =     20.00 ms /    " + unreadableCount + " tokens\n" +
			"slot print_timing:       total time =     30.00 ms /   150 tokens\n"},
		{"total", "slot print_timing: prompt eval time =     10.00 ms /   100 tokens\n" +
			"slot print_timing:        eval time =     20.00 ms /    50 tokens\n" +
			"slot print_timing:       total time =     30.00 ms /   " + unreadableCount + " tokens\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			rs, err := ParseOllamaLog(strings.NewReader(c.log))
			if err != nil {
				t.Fatal(err)
			}
			if len(rs) != 0 {
				t.Errorf("kept %d request(s) from a block whose %s count could not be "+
					"read: %+v. A count that did not convert is not a count of zero, "+
					"and the surviving record does not even satisfy its own total.",
					len(rs), c.name, rs)
			}
		})
	}
}

// OU-6: a readable log is unaffected.
//
// Without this, OU-4 and OU-5 are satisfied by a parser that refuses
// everything, which is the shape ADR-0014 catalogues: a check whose subject
// never appears.
func TestOU6_AReadableBlockStillParses(t *testing.T) {
	log := "slot launch_slot_: id  3 | task 7 | processing task\n" +
		"slot update_slots: id  3 | task 7 | n_past was set to 600\n" +
		"slot print_timing: id  3 | task 7 | prompt eval time =     10.00 ms /   100 tokens\n" +
		"slot print_timing: id  3 | task 7 |        eval time =     20.00 ms /    50 tokens\n" +
		"slot print_timing: id  3 | task 7 |       total time =     30.00 ms /   150 tokens\n"
	rs, err := ParseOllamaLog(strings.NewReader(log))
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 1 {
		t.Fatalf("got %d, want 1", len(rs))
	}
	r := rs[0]
	if !r.PrefixMeasured() || r.CachedPrefix != 600 {
		t.Errorf("CachedPrefix = %d measured=%v, want 600 measured", r.CachedPrefix, r.PrefixMeasured())
	}
	ctx, ok := r.ContextTokens()
	if !ok || ctx != 700 {
		t.Errorf("ContextTokens = %d,%v, want 700,true", ctx, ok)
	}
	if r.Slot != 3 || r.Task != 7 {
		t.Errorf("slot/task = %d/%d, want 3/7", r.Slot, r.Task)
	}
}

// OU-7: a prefix measured for one request is not inherited by the next.
//
// Only a total used to clear the block, so a request killed before its total —
// a rotated log, a killed process, the case
// TestOllamaDropsATruncatedBlockRatherThanCountItAsZero already covers for the
// eval count — left its n_past standing. The NEXT request then reported that
// prefix as its own.
//
// This is a harder failure than the one OU-1 fixed. OU-1's zero was at least
// visibly the worst case; this is a specific, plausible, defensible-looking
// number that was genuinely measured, on a different prompt. Nothing
// downstream can tell it apart from a real reading.
//
// PASS: the surviving request's prefix is unmeasured, so its context size is
// declined rather than answered with 500 + 200.
// FAIL: CachedPrefix 500 reported as measured, which is what shipped. Restore
// it by deleting the `if reSlotStart.MatchString(line) { reset() }` branch.
func TestOU7_APrefixDoesNotLeakIntoTheNextRequest(t *testing.T) {
	log := "slot launch_slot_: id  0 | task 0 | processing task, is_child = 0\n" +
		"slot update_slots: id  0 | task 0 | n_past was set to 500\n" +
		"slot print_timing: id  0 | task 0 | prompt eval time =     10.00 ms /   100 tokens\n" +
		"slot print_timing: id  0 | task 0 |        eval time =     20.00 ms /    50 tokens\n" +
		// task 0 is killed here: no total line ever arrives.
		"slot launch_slot_: id  0 | task 1 | processing task, is_child = 0\n" +
		"slot print_timing: id  0 | task 1 | prompt eval time =     11.00 ms /   200 tokens\n" +
		"slot print_timing: id  0 | task 1 |        eval time =     21.00 ms /    60 tokens\n" +
		"slot print_timing: id  0 | task 1 |       total time =     32.00 ms /   260 tokens\n"
	rs, err := ParseOllamaLog(strings.NewReader(log))
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 1 {
		t.Fatalf("got %d request(s), want 1: task 0 never closed and task 1 did", len(rs))
	}
	r := rs[0]
	if r.Task != 1 {
		t.Fatalf("kept task %d, want the one that closed (1)", r.Task)
	}
	if r.PrefixMeasured() {
		t.Errorf("task 1 reports a measured prefix of %d. It logged no n_past; "+
			"500 was measured for task 0, which was killed. A number carried over "+
			"from a different prompt is not a measurement of this one.", r.CachedPrefix)
	}
	if ctx, ok := r.ContextTokens(); ok {
		t.Errorf("ContextTokens answered %d for a request with no prefix reading", ctx)
	}
	if r.PromptEval != 200 || r.Generated != 60 || r.Total != 260 {
		t.Errorf("task 1's own numbers were disturbed: %+v", r)
	}
}

// OU-8: the abandoned block's own numbers do not leak either.
//
// The complement of OU-7, and the reason the reset is at the block boundary
// rather than on the n_past line: a request whose eval line is missing must not
// borrow the previous one's.
func TestOU8_AnAbandonedBlocksCountsDoNotLeakEither(t *testing.T) {
	log := "slot launch_slot_: id  0 | task 0 | processing task\n" +
		"slot print_timing: id  0 | task 0 | prompt eval time =     10.00 ms /   100 tokens\n" +
		"slot print_timing: id  0 | task 0 |        eval time =     20.00 ms /    50 tokens\n" +
		"slot launch_slot_: id  0 | task 1 | processing task\n" +
		// task 1 has no eval line of its own; it must be dropped, not given task 0's.
		"slot print_timing: id  0 | task 1 | prompt eval time =     11.00 ms /   200 tokens\n" +
		"slot print_timing: id  0 | task 1 |       total time =     32.00 ms /   260 tokens\n"
	rs, err := ParseOllamaLog(strings.NewReader(log))
	if err != nil {
		t.Fatal(err)
	}
	if len(rs) != 0 {
		t.Errorf("kept %d request(s): %+v. Task 1 logged no completion; the 50 "+
			"tokens belong to task 0, which was abandoned.", len(rs), rs)
	}
}
