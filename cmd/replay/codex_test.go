package main

import (
	"bytes"
	"strings"
	"testing"
)

// The viewer leads with what was billed, and never with the rebased counter.
func TestCodexViewLeadsWithTheBilledTotal(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := run([]string{"codex", "../../internal/transcript/codexdata"}, &out, &errOut); err != nil {
		t.Fatalf("%v\n%s", err, errOut.String())
	}
	got := out.String()
	// 1100 + 2200 + 550 across the compacted fixture, and nothing from the
	// impossible one, whose subsets cannot be believed.
	if !strings.Contains(got, "3,850") {
		t.Errorf("the billed total (3,850) is not stated:\n%s", got)
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
	if err := run([]string{"codex", "../../internal/transcript/codexdata"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "refused") {
		t.Errorf("the impossible-subset record was dropped without saying so:\n%s", out.String())
	}
}

// The quota reading is surfaced, because nothing else in this tool has one.
func TestCodexViewSurfacesTheQuotaSignal(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := run([]string{"codex", "../../internal/transcript/codexdata"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if !strings.Contains(got, "4%") && !strings.Contains(got, "4.0%") {
		t.Errorf("the rate-limit reading is not shown; it is the only live quota "+
			"signal this project has found in any client:\n%s", got)
	}
}
