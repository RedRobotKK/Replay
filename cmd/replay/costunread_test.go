package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A file the parser could not read is absent from the figures, not zero in
// them.
//
// `replay cost` counted two things and reported one. `unpriced` counts
// transcripts that PARSED and priced to nothing because their model is not in
// the price table. A transcript that could not be parsed at all never reached
// that counter — forEachSession handed the error to a visitor that returned
// nil — so it was counted nowhere and disclosed nowhere. Run over a single
// unreadable file, the command printed:
//
//	No transcript could be priced. 0 were read but their model is not in the price table.
//
// Both halves were false about the same file: it was not read, and nothing was
// known about its model. ADR-0018 names this exact error — absence rendered as
// zero — as the one this codebase makes most often.

// unreadableCorpus writes one transcript that the parser cannot turn into a
// request: a conversation line and nothing the provider answered with.
func unreadableCorpus(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	body := `{"type":"user","uuid":"u1","sessionId":"s","timestamp":"2026-09-06T10:00:00Z","message":{"role":"user","content":"hello"}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "unreadable.jsonl"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

// CU1: with nothing priced, the report does not claim the unreadable files
// were read.
func TestCU1_NothingPricedDoesNotCallUnreadableFilesRead(t *testing.T) {
	got := renderCost(summarise(nil), 0, 3, io.Discard, "")
	if strings.Contains(got, "0 were read") {
		t.Errorf("three files could not be read and the report says zero were read. "+
			"Absence, zero and unknown are three values (ADR-0018):\n%s", got)
	}
	if !strings.Contains(got, "3") {
		t.Errorf("the report does not say how many transcripts it could not read:\n%s", got)
	}
	if !strings.Contains(got, "could not be read") {
		t.Errorf("the report does not say what happened to them:\n%s", got)
	}
}

// CU2: the two counts are kept apart. A transcript read and not priced and a
// transcript never read have different causes and different fixes, and one
// sentence covering both would say something untrue about one of them.
func TestCU2_UnpricedAndUnreadableAreDifferentSentences(t *testing.T) {
	got := renderCost(summarise(nil), 5, 3, io.Discard, "")
	// "their modelS". This assertion used to pin the singular, which is the
	// grammar defect it was holding in place: "5 were read but their model"
	// reads as five transcripts sharing one model. The sentence is built by
	// unpricedClause now, and CW1 covers every count including the one this
	// test cannot reach.
	if !strings.Contains(got, "5 were read but their models are not in the price table") {
		t.Errorf("the unpriced count lost its own sentence:\n%s", got)
	}
	if !strings.Contains(got, "3 transcript(s) could not be read") {
		t.Errorf("the unreadable count lost its own sentence:\n%s", got)
	}
}

// CU3: with figures on the screen, the files behind none of them are still
// disclosed. A total that quietly omits what it could not read is the number
// this tool exists to distrust.
func TestCU3_PricedReportStillDisclosesWhatItCouldNotRead(t *testing.T) {
	got := renderCost(summarise([]costUnit{{CostUSD: 1}}), 0, 4, io.Discard, "")
	if !strings.Contains(got, "4 further transcript(s) could not be read") {
		t.Errorf("four transcripts are missing from every figure above and the report does "+
			"not say so:\n%s", got)
	}
	if !strings.Contains(got, "not zero in") {
		t.Errorf("the note does not tell the reader what the omission means for the total:\n%s", got)
	}
}

// CU4: the count reaches the renderer from the real command. CU1 to CU3 call
// renderCost directly, so all three pass with runCost never counting anything
// — which is how an absence fix ships with half of it reverted.
func TestCU4_CostCountsUnreadableFilesEndToEnd(t *testing.T) {
	dir := unreadableCorpus(t)
	var out, errb bytes.Buffer
	if err := run([]string{"cost", dir}, &out, &errb); err != nil {
		t.Fatalf("cost: %v (stderr %s)", err, errb.String())
	}
	got := out.String()
	if strings.Contains(got, "0 were read") {
		t.Errorf("one file could not be read and the command reports zero were read:\n%s", got)
	}
	if !strings.Contains(got, "1 transcript(s) could not be read") {
		t.Errorf("the command does not disclose the file it could not read:\n%s", got)
	}
}

// CU5: the gate discloses the same hole the report does.
//
// The gate is where a build fails, and a reader deciding whether to trust a
// failed build needs to know the total it was compared against had holes in
// it. It already said so for transcripts read and not priced. A transcript
// that could not be read at all is excluded from that total just as
// completely, and was named nowhere.
func TestCU5_TheGateNamesTranscriptsItCouldNotRead(t *testing.T) {
	var out bytes.Buffer
	err := checkAvoidableCeiling(1, summarise([]costUnit{{CostUSD: 100, AvoidableUSD: 50}}), 0, 2, &out)
	if err == nil {
		t.Fatal("avoidable spend of $50 must not pass a $1 ceiling; the disclosure is not under test")
	}
	if !strings.Contains(out.String(), "2 transcript(s) could not be read") {
		t.Errorf("the gate failed over a total with two transcripts missing from it and did "+
			"not say so:\n%s", out.String())
	}
}

// failAfterHeader writes everything until the cost report itself, then fails.
// A writer that fails on the first byte would trip an earlier write and leave
// the report's own error return untested.
type failAfterHeader struct {
	buf bytes.Buffer
}

var errClosedPipe = errors.New("write: broken pipe")

func (w *failAfterHeader) Write(p []byte) (int, error) {
	if bytes.Contains(p, []byte("Cost per task")) || bytes.Contains(p, []byte("No transcript could be priced")) {
		return 0, errClosedPipe
	}
	return w.buf.Write(p)
}

// CU6: a report that could not be written is not a report that was written.
//
// `replay cost | head -1` closes the pipe under the command. The write returns
// an error, and the only thing standing between that and an exit code of 0 is
// this return. Nothing exercised it, so the argument list it sits in could be
// changed - as it was, to carry the unreadable count - with no test entering
// the branch at all.
func TestCU6_AnUnwritableReportIsAnError(t *testing.T) {
	w := &failAfterHeader{}
	var errb bytes.Buffer
	err := runCost([]string{"../../internal/transcript/testdata"}, w, &errb)
	if !errors.Is(err, errClosedPipe) {
		t.Fatalf("cost reported success over a failed write: err = %v, wrote %q", err, w.buf.String())
	}
}
