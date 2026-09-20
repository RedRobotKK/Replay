package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The viewer leads with what was billed, and never with the rebased counter.
func TestCodexViewLeadsWithTheBilledTotal(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := run([]string{"codex", "codexdata"}, &out, &errOut); err != nil {
		t.Fatalf("%v\n%s", err, errOut.String())
	}
	got := out.String()
	// 3,850 from the compacted fixture plus 22,200 from the break fixture,
	// and nothing from the impossible one, whose subsets cannot be believed.
	if !strings.Contains(got, "26,050") {
		t.Errorf("the billed total (26,050) is not stated:\n%s", got)
	}
	// The rebased cumulative is 550. Printing it as the total would be the
	// defect; printing it beside the billed figure, named, is the point.
	if !strings.Contains(strings.ToLower(got), "compact") {
		t.Errorf("a compacted session must say so, because its cumulative counter "+
			"was rebased and is not a bill:\n%s", got)
	}
}

// A refused record is counted on screen, not dropped in silence.
func TestCodexViewReportsRefusedRecords(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := run([]string{"codex", "codexdata"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "refused") {
		t.Errorf("the impossible-subset record was dropped without saying so:\n%s", out.String())
	}
}

// The quota reading is surfaced, because nothing else in this tool has one.
func TestCodexViewSurfacesTheQuotaSignal(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := run([]string{"codex", "codexdata"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "4%") && !strings.Contains(got, "4.0%") {
		t.Errorf("the rate-limit reading is not shown; it is the only live quota "+
			"signal this project has found in any client:\n%s", got)
	}
}

// writeReEmittingCorpus puts one rollout carrying a re-emission in a temp dir.
//
// Built here rather than added to codexdata because the checked-in fixtures
// describe the reader's inputs, and this one exists to exercise a line of the
// VIEW. internal/transcript/codexdata/reemitted.jsonl already covers the
// reader.
func writeReEmittingCorpus(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	const body = `{"timestamp":"2026-03-21T00:36:01.000Z","type":"session_meta","payload":{"id":"019d0f52-8ca6-7f00-0000-0000000view","cli_version":"0.117.0-alpha.10"}}
{"timestamp":"2026-03-21T00:36:10.000Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":1000,"cached_input_tokens":400,"output_tokens":100,"reasoning_output_tokens":40,"total_tokens":1100},"last_token_usage":{"input_tokens":1000,"cached_input_tokens":400,"output_tokens":100,"reasoning_output_tokens":40,"total_tokens":1100},"model_context_window":258400}}}
{"timestamp":"2026-03-21T00:36:12.000Z","type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":1000,"cached_input_tokens":400,"output_tokens":100,"reasoning_output_tokens":40,"total_tokens":1100},"last_token_usage":{"input_tokens":1000,"cached_input_tokens":400,"output_tokens":100,"reasoning_output_tokens":40,"total_tokens":1100},"model_context_window":258400}}}
`
	if err := os.WriteFile(filepath.Join(dir, "rollout-view.jsonl"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

// The view says a turn was reported twice, and says it in its own words.
func TestCodexViewSurfacesReEmittedRecords(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := run([]string{"codex", writeReEmittingCorpus(t)}, &out, &errOut); err != nil {
		t.Fatalf("%v\n%s", err, errOut.String())
	}
	got := out.String()
	if !strings.Contains(got, "repeated usage this session had already") {
		t.Errorf("the re-emitted record is not reported:\n%s", got)
	}
	if !strings.Contains(got, "cumulative standing still") {
		t.Errorf("the note does not say what identified the repeat:\n%s", got)
	}
	// Billed once, not twice: 1,100 rather than 2,200.
	if !strings.Contains(got, "1,100 tokens billed") {
		t.Errorf("the repeat was billed again:\n%s", got)
	}
}

// The wording must not borrow the refusal vocabulary.
//
// A re-emission parsed cleanly and its usage was readable. Calling it refused,
// unreadable, unparsable, failed or NOT MEASURED would send a reader looking
// for a malformed record that does not exist, and would assert a claim result
// the contract does not carry here.
func TestTheReEmissionNoteClaimsNoFailure(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := run([]string{"codex", writeReEmittingCorpus(t)}, &out, &errOut); err != nil {
		t.Fatalf("%v\n%s", err, errOut.String())
	}
	got := out.String()
	note := ""
	for _, block := range strings.Split(got, "[NOTE]") {
		if strings.Contains(block, "repeated usage this session had already") {
			note = block
		}
	}
	if note == "" {
		t.Fatalf("no re-emission note to check:\n%s", got)
	}
	for _, banned := range []string{
		"refused", "unreadable", "unparseable", "unparsable",
		"failed", "error", "NOT MEASURED", "NOT_MEASURED",
	} {
		if strings.Contains(strings.ToLower(note), strings.ToLower(banned)) {
			t.Errorf("the re-emission note says %q, which asserts a failure that did not "+
				"happen:\n%s", banned, note)
		}
	}
}

// A corpus with no re-emission does not print the note.
//
// A note explaining a number that is not on the page explains nothing, and the
// number it would explain is zero.
func TestTheReEmissionNoteIsAbsentWhenNothingRepeated(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := run([]string{"codex", "codexdata"}, &out, &errOut); err != nil {
		t.Fatalf("%v\n%s", err, errOut.String())
	}
	if strings.Contains(out.String(), "repeated usage this session had already") {
		t.Errorf("the re-emission note appeared on a corpus with none:\n%s", out.String())
	}
}
