package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A line the reader could not parse is not a record it refused for a reason.
//
// The refusal note names two causes: a share that exceeded the total it is
// part of, and a total that arrived with no breakdown. Both describe a record
// the reader understood well enough to judge. Neither describes a line that is
// not JSON, and the reader counts those in the same number.
//
// Measured before this split: a rollout whose only fault was one line of
// non-JSON printed "1 record(s) refused, either because a share exceeded the
// total it is part of, or because a total arrived with no breakdown at all",
// sending a reader to look for a malformed subset in a record that never
// parsed.
//
// The repository has already made this call once, one field over. ReEmitted
// was split out of Skipped because folding distinct facts into one number made
// every surface reporting it say something untrue of a re-emission, and the
// comment naming that defect quotes this very sentence. The same test applied
// to unparsable input gives the same answer.

// codexRollout writes a rollout of the given lines and returns its directory.
func codexRollout(t *testing.T, lines ...string) string {
	t.Helper()
	dir := t.TempDir()
	body := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "rollout-probe.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func codexRun(t *testing.T, dir string) string {
	t.Helper()
	var out, errOut bytes.Buffer
	if err := run([]string{"codex", dir}, &out, &errOut); err != nil {
		t.Fatalf("replay codex %s: %v\n%s", dir, err, errOut.String())
	}
	return out.String()
}

// The lines a rollout is built from. Kept as constants so every test below
// names the same shapes and a reader can see what differs between them.
const (
	codexMeta = `{"timestamp":"2026-03-21T02:00:00.000Z","type":"session_meta","payload":{"id":"019d0f52-8ca6-7f00-0000-0000000000aa","cli_version":"0.117.0-alpha.10"}}`
	// A turn of 110 tokens, billed.
	codexTurn = `{"timestamp":"2026-03-21T02:00:10.000Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":100,"cached_input_tokens":0,"output_tokens":10,"reasoning_output_tokens":0,"total_tokens":110},"last_token_usage":{"input_tokens":100,"cached_input_tokens":0,"output_tokens":10,"reasoning_output_tokens":0,"total_tokens":110}}}}`
	// Not JSON at all.
	codexBadLine = `this line is not JSON at all`
	// Valid JSON whose payload is a string, so the payload cannot decode.
	codexBadPayload = `{"timestamp":"2026-03-21T02:00:20.000Z","type":"event_msg","payload":"not-an-object"}`
	// A total in the thousands with every component zero: the breakdown is
	// absent, not measured as none, so the record is understood and refused.
	codexNoBreakdown = `{"timestamp":"2026-03-21T02:00:30.000Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":0,"cached_input_tokens":0,"output_tokens":0,"reasoning_output_tokens":0,"total_tokens":11014},"last_token_usage":{"input_tokens":0,"cached_input_tokens":0,"output_tokens":0,"reasoning_output_tokens":0,"total_tokens":11014},"model_context_window":258400}}}`
)

// refusalSentence is the fragment that names the two breakdown causes. It must
// never appear for a record that was never parsed.
const refusalSentence = "a share exceeded the"

// CXU1: an unparsable line is reported as unparsable, not as a refusal.
func TestCXU1_AnUnparsableLineIsReportedSeparately(t *testing.T) {
	out := codexRun(t, codexRollout(t, codexMeta, codexTurn, codexBadLine))
	if !strings.Contains(out, "could not be parsed") {
		t.Errorf("an unparsable line is not reported as unparsable:\n%s", out)
	}
	if strings.Contains(out, refusalSentence) {
		t.Errorf("an unparsable line was described as refused for a breakdown reason, "+
			"which is a cause the reader never established:\n%s", out)
	}
}

// CXU2: an unparsable payload lands in the same population as an unparsable
// line, because the reader learned the same nothing from both.
func TestCXU2_AnUnparsablePayloadIsReportedSeparately(t *testing.T) {
	out := codexRun(t, codexRollout(t, codexMeta, codexTurn, codexBadPayload))
	if !strings.Contains(out, "could not be parsed") {
		t.Errorf("an unparsable payload is not reported as unparsable:\n%s", out)
	}
	if strings.Contains(out, refusalSentence) {
		t.Errorf("an unparsable payload was described as refused for a breakdown reason:\n%s", out)
	}
}

// CXU3: the existing refusal population still reports as it did.
//
// This is the half that must not move. A total with no breakdown was
// understood, judged and refused, and the sentence that says so is true of it.
func TestCXU3_AnUnusableBreakdownStillReportsAsRefused(t *testing.T) {
	out := codexRun(t, codexRollout(t, codexMeta, codexTurn, codexNoBreakdown))
	if !strings.Contains(out, "refused") {
		t.Errorf("the refused record was dropped without saying so:\n%s", out)
	}
	if !strings.Contains(out, refusalSentence) {
		t.Errorf("the refusal no longer names why:\n%s", out)
	}
	if strings.Contains(out, "could not be parsed") {
		t.Errorf("a record the reader understood was called unparsable:\n%s", out)
	}
}

// CXU4: both populations, both reported, neither borrowing the other's reason.
//
// This is the case the defect was made of. One number covered both, so the
// reader was given one reason for two different facts.
func TestCXU4_BothPopulationsAreReportedIndependently(t *testing.T) {
	out := codexRun(t, codexRollout(t, codexMeta, codexTurn, codexBadLine, codexNoBreakdown))

	if !strings.Contains(out, "110") {
		t.Errorf("the valid turn's tokens are missing, so the split changed accounting:\n%s", out)
	}
	if !strings.Contains(out, "could not be parsed") {
		t.Errorf("the unparsable line is not reported:\n%s", out)
	}
	if !strings.Contains(out, refusalSentence) {
		t.Errorf("the refused record is not reported:\n%s", out)
	}
	// One each, not two of either. A count of 2 anywhere would mean the
	// populations are still sharing a number.
	if strings.Contains(out, "2 record(s) refused") {
		t.Errorf("the unparsable line was counted as a refusal as well:\n%s", out)
	}
	if strings.Contains(out, "2 record(s) could not be parsed") {
		t.Errorf("the refused record was counted as unparsable as well:\n%s", out)
	}
}

// CXU5: a clean rollout says neither thing.
func TestCXU5_ACleanRolloutReportsNeither(t *testing.T) {
	out := codexRun(t, codexRollout(t, codexMeta, codexTurn))
	for _, banned := range []string{"refused", "could not be parsed"} {
		if strings.Contains(out, banned) {
			t.Errorf("a clean rollout reported %q:\n%s", banned, out)
		}
	}
	if !strings.Contains(out, "110") {
		t.Errorf("the clean rollout's tokens are missing:\n%s", out)
	}
}

// CXU6: the unparsable note claims only what the reader knows.
//
// It could not read the record. That is all. It does not know whether the
// provider refused anything, whether a breakdown was missing, what the record
// would have cost, or whether any tokens were involved at all.
func TestCXU6_TheUnparsableNoteClaimsNoCause(t *testing.T) {
	out := strings.ToLower(codexRun(t, codexRollout(t, codexMeta, codexTurn, codexBadLine)))
	var line string
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "could not be parsed") {
			line = l
		}
	}
	if line == "" {
		t.Fatalf("no unparsable line to check:\n%s", out)
	}
	for _, banned := range []string{
		"refus", "breakdown", "share", "provider", "billing", "billed",
		"zero", "cost", "$", "quota", "failure",
	} {
		if strings.Contains(line, banned) {
			t.Errorf("the unparsable note claims %q, which the reader never established: %q", banned, line)
		}
	}
}

// CXU7: accounting is untouched by the split.
//
// The billed total comes from the valid turn and from nothing else, whichever
// faults accompany it.
func TestCXU7_AccountingIsUnchangedByTheSplit(t *testing.T) {
	clean := codexRun(t, codexRollout(t, codexMeta, codexTurn))
	faulty := codexRun(t, codexRollout(t, codexMeta, codexTurn, codexBadLine, codexBadPayload, codexNoBreakdown))
	for _, out := range []string{clean, faulty} {
		if !strings.Contains(out, "110 tokens billed") {
			t.Errorf("the billed figure moved: want 110 tokens billed\n%s", out)
		}
	}
}
