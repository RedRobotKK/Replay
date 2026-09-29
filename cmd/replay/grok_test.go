package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The Grok surface, which `replay doctor` had been naming as unread.
//
// WHAT THESE TESTS PIN, AND WHY THE EARLIER SET DID NOT.
//
// The first version of this file asserted that Grok's records are CUMULATIVE
// running totals, on a three-line fixture built in a t.TempDir() whose records
// were cumulative by construction. It proved the reader did not sum what it was
// handed. It could not establish anything about real Grok records, because it
// never read any, and the claim it appeared to settle is false.
//
// The vendor's own output settles it, and is committed under
// testdata/grok/vendor-usage: three captures of `grok usage <session>` stdout
// from Grok 1.0.41, taken 2026-09-27. On all three, session.inputTokens equals
// the SUM of turns[].inputTokens. On the six-turn session the sum is 7,142,396
// while the largest turn is 3,716,413 and the last is 2,363,388. On the
// 131-turn session the sum is 222,856,819, the largest turn is 21,452,883 and
// the last turn is 0. A max reader and a last-turn reader each produce a
// different wrong number on both files, which is what makes these fixtures able
// to fail.
//
// TWO ARTIFACTS, NEVER ADDED. A session directory holds updates.jsonl, the
// per-turn stream Replay reconstructs from, and usage.json, the vendor's ledger.
// Measured over 48 local sessions on 2026-09-27, 35 carry both and 13 carry
// updates.jsonl with no usage.json, and `grok usage` reports no usage at all for
// those 13. So a missing ledger is a third value, not a zero, and the two
// columns are compared rather than summed.
//
// costUsdTicks is documented by the Grok 1.0.41 user guide as 10^10 ticks per
// USD. Nobody has reconciled that scale against a statement of account, so no
// dollar figure is printed and GK6 holds that.

const grokVendorGolden = "testdata/grok/vendor-usage"

// grokTurnLine renders one updates.jsonl record carrying a turn's usage.
func grokTurnLine(in, out, cachedRead int, write *int) string {
	w := "null"
	if write != nil {
		w = fmt.Sprintf("%d", *write)
	}
	return fmt.Sprintf(`{"timestamp":"2026-09-27T18:41:22Z","method":"session/update","params":{"update":{"usage":{`+
		`"inputTokens":%d,"outputTokens":%d,"totalTokens":%d,"cachedReadTokens":%d,`+
		`"cacheCreationTokens":%s,"reasoningTokens":0,"modelCalls":1,"costUsdTicks":0,`+
		`"modelUsage":{"grok-4.7-build":{"inputTokens":%d}}}}}}`, in, out, in+out, cachedRead, w, in)
}

// writeGrokSession lays down one session directory. ledger is written as usage.json
// when non-empty; passing "" is the 13-session case with no vendor record.
func writeGrokSession(t *testing.T, root, id string, lines []string, ledger string) string {
	t.Helper()
	dir := filepath.Join(root, "proj", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if len(lines) > 0 {
		if err := os.WriteFile(filepath.Join(dir, "updates.jsonl"),
			[]byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if ledger != "" {
		if err := os.WriteFile(filepath.Join(dir, "usage.json"), []byte(ledger), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// grokVendorLedgerJSON renders a usage.json whose session object states the given
// prompt over the given turn count.
func grokVendorLedgerJSON(id string, prompt, output, cachedRead, turns int) string {
	return fmt.Sprintf(`{"sessionId":%q,"updatedAt":"2026-09-27T19:09:46Z","session":{`+
		`"inputTokens":%d,"outputTokens":%d,"totalTokens":%d,"cachedReadTokens":%d,`+
		`"cacheCreationTokens":0,"reasoningTokens":0,"modelCalls":%d,"costUsdTicks":0,`+
		`"turnCount":%d,"primaryModelId":"grok-4.7-build","modelUsage":{"grok-4.7-build":{"inputTokens":%d}}},`+
		`"turns":[]}`, id, prompt, output, prompt+output, cachedRead, turns, turns, prompt)
}

// The six values below are the real per-turn inputTokens of session
// 01a0e417-88e1-7792-93e1-b5b5d72b7390 as the Grok CLI itself reports them.
var grokRealTurns = []int{3716413, 348136, 175281, 358340, 180838, 2363388}

const (
	grokRealTurnSum  = 7142396 // session.inputTokens in the vendor's own file
	grokRealTurnMax  = 3716413
	grokRealTurnLast = 2363388
)

func grokRealTurnLines() []string {
	lines := make([]string, 0, len(grokRealTurns))
	zero := 0
	for _, in := range grokRealTurns {
		lines = append(lines, grokTurnLine(in, 0, 0, &zero))
	}
	return lines
}

// GK1: per-turn records are SUMMED. Not maximised, not taken from the last one,
// and not treated as a running total.
//
// PASS: the six real turns of session 01a0e417 reconstruct to 7,142,396.
// FAIL: 3,716,413 (the max), 2,363,388 (the last), or anything else.
//
// This replaces a test that asserted the opposite on a fixture built to agree
// with it. The fixture here is the vendor's own per-turn numbers, and the three
// candidate aggregations give three different answers on it.
func TestGK1_PerTurnRecordsAreSummedNotMaximised(t *testing.T) {
	root := t.TempDir()
	writeGrokSession(t, root, "01a0e417", grokRealTurnLines(), "")

	got, err := readGrok(root)
	if err != nil {
		t.Fatal(err)
	}
	if got.Reconstructed.Prompt == grokRealTurnMax {
		t.Errorf("prompt = %d, which is the MAXIMUM turn. The records are per turn, "+
			"not a running total: sum(turns) == session.inputTokens on 35 of 35 "+
			"sessions carrying a vendor ledger. Want %d.", got.Reconstructed.Prompt, grokRealTurnSum)
	}
	if got.Reconstructed.Prompt == grokRealTurnLast {
		t.Errorf("prompt = %d, which is the LAST turn. Want %d.", got.Reconstructed.Prompt, grokRealTurnSum)
	}
	if got.Reconstructed.Prompt != grokRealTurnSum {
		t.Errorf("prompt = %d, want %d (the sum of the six per-turn records)",
			got.Reconstructed.Prompt, grokRealTurnSum)
	}
	if got.Reconstructed.Turns != len(grokRealTurns) {
		t.Errorf("turns = %d, want %d", got.Reconstructed.Turns, len(grokRealTurns))
	}
}

// GK2: the vendor's own captures state the sum, and reject max and last.
//
// This is the golden. The three files are `grok usage` stdout, unmodified, and
// the assertion is the vendor's own arithmetic: the session object equals the
// sum of its turns on every one of them. A reader that regressed to max or last
// would have to disagree with the provider about the provider's own totals.
func TestGK2_TheVendorLedgerStatesTheSumOfItsTurns(t *testing.T) {
	for _, tc := range []struct {
		file                   string
		turns                  int
		sum, max, last, output int
	}{
		{"session-one-turn.json", 1, 1764978, 1764978, 1764978, 25194},
		{"session-01a0e417.json", 6, 7142396, 3716413, 2363388, 85572},
		{"session-01a09207.json", 131, 222856819, 21452883, 0, 906481},
	} {
		t.Run(tc.file, func(t *testing.T) {
			led, ok := readGrokLedger(filepath.Join(grokVendorGolden, tc.file))
			if !ok {
				t.Fatalf("the committed vendor capture did not parse")
			}
			if len(led.Turns) != tc.turns {
				t.Fatalf("turns = %d, want %d", len(led.Turns), tc.turns)
			}

			var summed grokCounts
			maxTurn := 0
			for _, turn := range led.Turns {
				if !summed.add(turn) {
					t.Errorf("a turn's parts did not add back to its prompt: %+v", turn)
				}
				if turn.InputTokens > maxTurn {
					maxTurn = turn.InputTokens
				}
			}
			last := led.Turns[len(led.Turns)-1].InputTokens

			if summed.Prompt != led.Session.InputTokens {
				t.Errorf("sum of turns = %d, session states %d. The vendor's own file "+
					"says these are equal on every capture", summed.Prompt, led.Session.InputTokens)
			}
			if summed.Prompt != tc.sum {
				t.Errorf("sum = %d, want %d", summed.Prompt, tc.sum)
			}
			if summed.Output != tc.output {
				t.Errorf("output sum = %d, want %d", summed.Output, tc.output)
			}
			if maxTurn != tc.max || last != tc.last {
				t.Fatalf("fixture drifted: max = %d want %d, last = %d want %d",
					maxTurn, tc.max, last, tc.last)
			}
			// The discrimination this golden exists for. Where the three differ,
			// a max or last reader cannot reproduce the session object.
			if tc.turns > 1 {
				if led.Session.InputTokens == maxTurn {
					t.Errorf("the session total equals the maximum turn, so this " +
						"capture cannot distinguish a max reader from a correct one")
				}
				if led.Session.InputTokens == last {
					t.Errorf("the session total equals the last turn, so this capture " +
						"cannot distinguish a last-turn reader from a correct one")
				}
			}
		})
	}
}

// GK3: reconstruction and ledger agreeing is reported as a MATCH.
func TestGK3_AgreementWithTheVendorLedgerIsReportedAsAMatch(t *testing.T) {
	root := t.TempDir()
	writeGrokSession(t, root, "01a0e417", grokRealTurnLines(),
		grokVendorLedgerJSON("01a0e417", grokRealTurnSum, 0, 0, len(grokRealTurns)))

	got, err := readGrok(root)
	if err != nil {
		t.Fatal(err)
	}
	if got.Matched != 1 || got.Diverged != 0 || got.NoLedger != 0 {
		t.Errorf("matched/diverged/no-ledger = %d/%d/%d, want 1/0/0",
			got.Matched, got.Diverged, got.NoLedger)
	}
	if got.Reconstructed.Prompt != grokRealTurnSum {
		t.Errorf("reconstructed prompt = %d, want %d", got.Reconstructed.Prompt, grokRealTurnSum)
	}
	if got.Vendor.Prompt != grokRealTurnSum {
		t.Errorf("vendor prompt = %d, want %d", got.Vendor.Prompt, grokRealTurnSum)
	}
	// The two agreeing must not become one doubled number.
	if got.Reconstructed.Prompt+got.Vendor.Prompt == got.Vendor.Prompt+got.Reconstructed.Prompt &&
		got.Vendor.Prompt == 2*grokRealTurnSum {
		t.Error("the vendor total is twice the session's, so the reconstruction was added into it")
	}
}

// GK4: a disagreement is reported as a discrepancy, with its size, and the two
// quantities are never merged.
//
// The real case: session 01a09207 carries 131 records on both surfaces and the
// ledger states a prompt 29,519,212 above the sum of them. Record counts match,
// so nothing is missing; the records drift. A reader that added the two, or
// silently preferred one, would report a third number that is in neither file.
func TestGK4_ADisagreementIsReportedAndTheTwoAreNeverAdded(t *testing.T) {
	const excess = 29519212
	root := t.TempDir()
	writeGrokSession(t, root, "01a09207", grokRealTurnLines(),
		grokVendorLedgerJSON("01a09207", grokRealTurnSum+excess, 0, 0, len(grokRealTurns)))

	got, err := readGrok(root)
	if err != nil {
		t.Fatal(err)
	}
	if got.Diverged != 1 || got.Matched != 0 {
		t.Errorf("diverged/matched = %d/%d, want 1/0", got.Diverged, got.Matched)
	}
	if got.DivergedTokens != excess {
		t.Errorf("divergence = %d, want %d", got.DivergedTokens, excess)
	}
	if got.Reconstructed.Prompt != grokRealTurnSum {
		t.Errorf("reconstructed prompt = %d, want %d: the ledger must not overwrite "+
			"what Replay reconstructed", got.Reconstructed.Prompt, grokRealTurnSum)
	}
	if got.Vendor.Prompt != grokRealTurnSum+excess {
		t.Errorf("vendor prompt = %d, want %d: the reconstruction must not overwrite "+
			"the ledger either", got.Vendor.Prompt, grokRealTurnSum+excess)
	}
	if got.Reconstructed.Prompt == got.Vendor.Prompt {
		t.Error("the two columns are equal, so one was copied over the other")
	}
	if got.Vendor.Prompt == 2*grokRealTurnSum+excess {
		t.Error("the reconstruction was ADDED to the ledger. They are two readings of " +
			"the same work and summing them counts it twice")
	}

	var b bytes.Buffer
	got.render(&b)
	out := b.String()
	if !strings.Contains(out, string(grokLedgerDiffers)) {
		t.Errorf("the rendering does not report the disagreement:\n%s", out)
	}
	if !strings.Contains(out, comma(excess)) {
		t.Errorf("the rendering does not state the size of the disagreement:\n%s", out)
	}
}

// GK5: a missing vendor ledger is UNAVAILABLE, and is not a vendor total of
// zero.
//
// Thirteen of 48 local sessions have updates.jsonl and no usage.json, and
// `grok usage` reports no usage for every one of them. Their reconstruction is
// still shown, because the records are on disk. It is not counted as confirmed,
// it is not added to the vendor column, and its absence is not rendered as 0.
func TestGK5_AMissingVendorLedgerIsUnavailableNotZero(t *testing.T) {
	root := t.TempDir()
	writeGrokSession(t, root, "has-ledger", grokRealTurnLines(),
		grokVendorLedgerJSON("has-ledger", grokRealTurnSum, 0, 0, len(grokRealTurns)))
	writeGrokSession(t, root, "no-ledger", grokRealTurnLines(), "")

	got, err := readGrok(root)
	if err != nil {
		t.Fatal(err)
	}
	if got.Sessions != 2 {
		t.Fatalf("sessions = %d, want 2", got.Sessions)
	}
	if got.NoLedger != 1 || got.Matched != 1 {
		t.Errorf("no-ledger/matched = %d/%d, want 1/1", got.NoLedger, got.Matched)
	}
	// Reconstruction covers both sessions.
	if got.Reconstructed.Prompt != 2*grokRealTurnSum {
		t.Errorf("reconstructed prompt = %d, want %d: the session with no ledger "+
			"still has records on disk", got.Reconstructed.Prompt, 2*grokRealTurnSum)
	}
	// The vendor column covers only the session that has a ledger. Not two
	// sessions, and not a zero contributed by the one without.
	if got.Vendor.Prompt != grokRealTurnSum {
		t.Errorf("vendor prompt = %d, want %d: a session Grok reports no usage for "+
			"must be ABSENT from the vendor figure, not a zero inside it",
			got.Vendor.Prompt, grokRealTurnSum)
	}
	if got.Vendor.Turns != len(grokRealTurns) {
		t.Errorf("vendor turns = %d, want %d", got.Vendor.Turns, len(grokRealTurns))
	}
	if got.NoLedgerTokens != grokRealTurnSum {
		t.Errorf("unconfirmed reconstruction = %d, want %d", got.NoLedgerTokens, grokRealTurnSum)
	}

	var b bytes.Buffer
	got.render(&b)
	out := b.String()
	if !strings.Contains(out, string(grokLedgerUnavailable)) {
		t.Errorf("the rendering does not say the ledger is unavailable:\n%s", out)
	}
	if !strings.Contains(strings.ToLower(out), "not confirmed") {
		t.Errorf("the rendering does not say the unledgered tokens are unconfirmed:\n%s", out)
	}
}

// GK5b: with no ledger anywhere, the vendor column says UNAVAILABLE rather than
// printing zeroes that would read as a measured vendor total of nothing.
func TestGK5b_NoLedgerAnywhereRendersUnavailableNotZeroes(t *testing.T) {
	root := t.TempDir()
	writeGrokSession(t, root, "no-ledger", grokRealTurnLines(), "")

	got, err := readGrok(root)
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	got.render(&b)
	out := b.String()
	if !strings.Contains(out, "UNAVAILABLE") {
		t.Errorf("the vendor block does not say UNAVAILABLE:\n%s", out)
	}
	// The sentence is wrapped, so the assertion is on its distinctive words
	// rather than on one line of it.
	if !strings.Contains(strings.ToLower(flatten(out)), "not a vendor total of zero") {
		t.Errorf("the rendering does not distinguish an absent ledger from a zero one:\n%s", out)
	}
}

// flatten collapses the rendering's wrapping so a sentence can be asserted as a
// sentence. Without it a test pins where the text happens to break.
func flatten(s string) string { return strings.Join(strings.Fields(s), " ") }

// GK6: no dollar figure is produced, and the refusal names the scale correctly.
//
// The earlier version of this test required the reading to say the scale was
// UNDOCUMENTED. That is false: the Grok 1.0.41 user guide states 10^10 ticks
// per USD. What is true, and what this now requires, is that the scale has not
// been checked against an invoice. Undocumented and unchecked are different
// claims, and pinning the wrong one put a false sentence in the binary and in
// three documents.
func TestGK6_DollarsAreRefusedAndTheScaleIsDescribedCorrectly(t *testing.T) {
	root := t.TempDir()
	writeGrokSession(t, root, "01a0e417", grokRealTurnLines(),
		grokVendorLedgerJSON("01a0e417", grokRealTurnSum, 0, 0, len(grokRealTurns)))

	got, err := readGrok(root)
	if err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	got.render(&b)
	out := b.String()

	if strings.Contains(out, "$") {
		t.Errorf("a dollar sign appears in the Grok reading:\n%s\n"+
			"The tick scale is stated by the vendor guide and unchecked against an "+
			"invoice. Printing a figure would present that as a measurement.", out)
	}
	if !strings.Contains(out, "10^10 ticks per USD") {
		t.Errorf("the refusal does not state the scale the guide gives:\n%s", out)
	}
	if !strings.Contains(out, "unchecked against an invoice") {
		t.Errorf("the refusal does not say what is actually missing, which is the "+
			"reconciliation and not the documentation:\n%s", out)
	}
	// The retracted claim must not come back.
	for _, banned := range []string{"no documented scale", "undocumented", "cumulative", "running total"} {
		if strings.Contains(strings.ToLower(out), banned) {
			t.Errorf("the rendering carries the retracted claim %q:\n%s", banned, out)
		}
	}
	if !strings.Contains(out, comma(grokRealTurnSum)) {
		t.Errorf("the token total is missing, so the refusal left nothing behind:\n%s", out)
	}
}

// GK7: a cache write of zero is reported as zero, not as absent.
//
// ADR-0018: absence is not zero and zero is not unknown. Grok reports
// cacheCreationTokens as a present field holding 0; Codex omits the field
// entirely. Those are different facts and the reader must not flatten them.
func TestGK7_AZeroCacheWriteIsNotTheSameAsNoField(t *testing.T) {
	root := t.TempDir()
	zero := 0
	writeGrokSession(t, root, "states-zero", []string{grokTurnLine(1000, 0, 800, &zero)}, "")
	writeGrokSession(t, root, "omits-field", []string{grokTurnLine(1000, 0, 800, nil)}, "")

	got, err := readGrok(root)
	if err != nil {
		t.Fatal(err)
	}
	if got.Sessions != 2 {
		t.Fatalf("sessions = %d, want 2", got.Sessions)
	}
	if got.WritesReported != 1 {
		t.Errorf("WritesReported = %d, want 1: one session stated a write of zero "+
			"and the other said nothing, and those are different facts", got.WritesReported)
	}
}

// GK8: a malformed line does not abandon the session around it.
//
// A truncated final line is ordinary in an append-only log that was still being
// written. Refusing the whole file for it would throw away every good record
// before it.
func TestGK8_AMalformedLineDoesNotDiscardTheGoodOnes(t *testing.T) {
	root := t.TempDir()
	zero := 0
	writeGrokSession(t, root, "truncated", []string{
		grokTurnLine(1000, 0, 0, &zero),
		`{"usage":{"inputTokens":2000,"cached`, // truncated mid-write
	}, "")

	got, err := readGrok(root)
	if err != nil {
		t.Fatalf("a truncated trailing line made the whole read fail: %v", err)
	}
	if got.Reconstructed.Prompt != 1000 {
		t.Errorf("prompt = %d, want 1000: the good record before the truncation "+
			"was discarded with it", got.Reconstructed.Prompt)
	}
	if got.Unreadable != 1 {
		t.Errorf("Unreadable = %d, want 1: what could not be parsed is counted rather "+
			"than silently dropped", got.Unreadable)
	}
}

// GK9: a root with no Grok data reads as empty rather than as an error.
func TestGK9_AnAbsentRootReadsAsEmpty(t *testing.T) {
	got, err := readGrok(filepath.Join(t.TempDir(), "absent"))
	if err != nil {
		t.Fatalf("a missing root returned an error: %v", err)
	}
	if got.Sessions != 0 || got.Reconstructed.Prompt != 0 {
		t.Errorf("a missing root produced %d session(s) and %d token(s), want none",
			got.Sessions, got.Reconstructed.Prompt)
	}
}

// GK10: a valid line that is simply not a usage record is not "unreadable".
//
// The first run of this reader against the real corpus reported 33,399
// unreadable lines out of a log that parses fine. They were message updates,
// tool calls and everything else Grok writes: valid JSON, carrying no usage.
// Counting them as unparseable tells a reader their log is broken when it is
// not, and it buries a real truncation in five figures of noise.
func TestGK10_AValidNonUsageLineIsNotUnreadable(t *testing.T) {
	root := t.TempDir()
	zero := 0
	writeGrokSession(t, root, "ordinary", []string{
		`{"type":"message","role":"assistant","content":"ordinary log line"}`,
		`{"type":"tool_call","name":"read_file","args":{"path":"x.go"}}`,
		grokTurnLine(1000, 0, 800, &zero),
	}, "")

	got, err := readGrok(root)
	if err != nil {
		t.Fatal(err)
	}
	if got.Reconstructed.Prompt != 1000 {
		t.Errorf("prompt = %d, want 1000", got.Reconstructed.Prompt)
	}
	if got.Unreadable != 0 {
		t.Errorf("Unreadable = %d, want 0. Two valid JSON lines carrying no usage "+
			"were counted as unparseable, which is how a healthy log gets "+
			"reported as broken", got.Unreadable)
	}
}

// GK11: the cache is inside the prompt, and fresh is the remainder.
//
// Grok counts inclusively: cachedReadTokens is a SHARE of inputTokens, not a
// figure beside it. A reader that copied inputTokens into a fresh-token field
// would double-count every cached token, worst on the sessions that cache best.
func TestGK11_CachedReadsSitInsideThePromptAndFreshIsTheRemainder(t *testing.T) {
	root := t.TempDir()
	zero := 0
	writeGrokSession(t, root, "cached", []string{grokTurnLine(1000, 50, 800, &zero)}, "")

	got, err := readGrok(root)
	if err != nil {
		t.Fatal(err)
	}
	c := got.Reconstructed
	if c.Prompt != 1000 {
		t.Errorf("prompt = %d, want 1000 (the provider's own inputTokens)", c.Prompt)
	}
	if c.CachedRead != 800 {
		t.Errorf("cached read = %d, want 800", c.CachedRead)
	}
	if c.Fresh != 200 {
		t.Errorf("fresh = %d, want 200. Fresh is the prompt less the cache, and a "+
			"reader that reports 1000 has counted the 800 cached tokens twice", c.Fresh)
	}
	if c.Fresh+c.CachedRead+c.CachedWrite != c.Prompt {
		t.Errorf("the parts do not add back to the prompt: %d + %d + %d != %d",
			c.Fresh, c.CachedRead, c.CachedWrite, c.Prompt)
	}
}

// GK12: a turn whose parts cannot add back to its prompt is excluded and
// counted, not quietly folded in.
func TestGK12_AnInconsistentTurnIsExcludedAndCounted(t *testing.T) {
	root := t.TempDir()
	zero := 0
	writeGrokSession(t, root, "impossible", []string{
		grokTurnLine(1000, 0, 800, &zero),
		grokTurnLine(100, 0, 900, &zero), // cache larger than the prompt it sits in
	}, "")

	got, err := readGrok(root)
	if err != nil {
		t.Fatal(err)
	}
	if got.Inconsistent != 1 {
		t.Errorf("Inconsistent = %d, want 1", got.Inconsistent)
	}
	if got.Reconstructed.Prompt != 1000 {
		t.Errorf("prompt = %d, want 1000: the impossible record must not contribute",
			got.Reconstructed.Prompt)
	}
}

// GK13: a model is counted once per session, not once per artifact naming it.
//
// Found by running the built binary: two sessions, each naming one model in both
// its stream and its ledger, reported that model against three sessions. The
// per-session collection is a set for that reason.
func TestGK13_AModelIsCountedOncePerSessionNotPerArtifact(t *testing.T) {
	root := t.TempDir()
	zero := 0
	lines := []string{grokTurnLine(1000, 0, 0, &zero)}
	writeGrokSession(t, root, "both", lines, grokVendorLedgerJSON("both", 1000, 0, 0, 1))
	writeGrokSession(t, root, "stream-only", lines, "")

	got, err := readGrok(root)
	if err != nil {
		t.Fatal(err)
	}
	if got.Sessions != 2 {
		t.Fatalf("sessions = %d, want 2", got.Sessions)
	}
	if n := got.Models["grok-4.7-build"]; n != 2 {
		t.Errorf("the model is reported against %d session(s), want 2. There are two "+
			"sessions; one of them names the model in two artifacts and that is "+
			"still one session", n)
	}
}

// GK14: the tick field is not decoded, so no code path can multiply it.
//
// The refusal to print dollars is a sentence in the rendering, and a sentence
// can be edited by somebody who does not know why it is there. This is the
// structural half of the same refusal: costUsdTicks never becomes a typed
// number in this package, so converting it would take a deliberate change to
// the type rather than a change to a format string.
//
// The scale is not unknown. The Grok 1.0.41 user guide states 10^10 ticks per
// USD. What is missing is the reconciliation against a statement of account,
// and until that exists the multiplication is the unjustified step.
func TestGK14_TheTickFieldIsNotDecodedIntoATypedNumber(t *testing.T) {
	for _, f := range reflect.VisibleFields(reflect.TypeOf(grokUsage{})) {
		name := strings.ToLower(f.Name)
		tag := strings.ToLower(f.Tag.Get("json"))
		if strings.Contains(name, "tick") || strings.Contains(name, "cost") ||
			strings.Contains(tag, "tick") || strings.Contains(tag, "cost") {
			t.Errorf("grokUsage decodes %q (json %q). The tick scale is stated by the "+
				"vendor guide and unchecked against an invoice, so this package does "+
				"not carry the number that a dollar figure would be computed from.",
				f.Name, f.Tag.Get("json"))
		}
	}
}
