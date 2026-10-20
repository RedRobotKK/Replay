package main

import (
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/analysis"
)

// CNR: the corpus table does not call a model without published cache rules
// "calibrated" (2026-10-16). Its reads can match the engine's prediction, but
// no provider rules were tested, and release criteria require a second
// provider calibrated against real traffic under a protocol; a table row
// saying "calibrated" for gpt-5 reads as that.
func TestCNR1_AModelWithoutRulesIsNotCalledCalibrated(t *testing.T) {
	models := []analysis.ModelCalibration{
		{Model: "claude-opus-5", Sessions: 30, Lanes: 30, Compared: 900, Matched: 899, Exact: 890},
		{Model: "gpt-5", Sessions: 30, Lanes: 30, Compared: 900, Matched: 899, Exact: 890},
	}
	var sb strings.Builder
	if err := writeCorpus(&sb, nil, models, nil); err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(sb.String(), "\n") {
		if !strings.HasPrefix(line, "| ") {
			continue
		}
		verdict := strings.TrimSpace(line[strings.LastIndex(strings.TrimSuffix(line, " |"), "|")+1:])
		verdict = strings.TrimSuffix(verdict, " |")
		switch {
		case strings.HasPrefix(line, "| gpt-5 |"):
			if verdict == "calibrated" || !strings.Contains(verdict, "no published cache rules") {
				t.Errorf("gpt-5 verdict %q, want one that says it has no published cache rules", verdict)
			}
		case strings.HasPrefix(line, "| claude-opus-5 |"):
			if verdict != "calibrated" {
				t.Errorf("claude-opus-5 verdict %q, want calibrated (unchanged)", verdict)
			}
		}
	}
}
