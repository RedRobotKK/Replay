package main

import (
	"bytes"
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

// A session with no established billing basis is named, not dropped quietly.
//
// codexdata carries one: the impossible-subset fixture, whose only usage record
// the acceptance rules refuse. It contributed nothing to the billed total
// before this gate existed and contributes nothing now, but the reason it
// contributes nothing is a fact the reader is entitled to.
func TestCodexViewNamesSessionsWithNoBillingBasis(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := run([]string{"codex", "codexdata"}, &out, &errOut); err != nil {
		t.Fatalf("%v\n%s", err, errOut.String())
	}
	got := out.String()
	if !strings.Contains(got, "NOT MEASURED") {
		t.Errorf("a session was left out of every figure without the view saying so:\n%s", got)
	}
	// The billed total is unchanged: the refused session was never in it.
	if !strings.Contains(got, "26,050") {
		t.Errorf("the billed total moved; refusing a session that contributed nothing "+
			"must not change the figure:\n%s", got)
	}
}
