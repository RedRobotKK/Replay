package analysis

import (
	"bytes"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// RLR: the report header names the cache rules that were applied, not
// Anthropic's for every session (2026-10-16). A gpt-5 session's breaks are
// classified with no rules since #346, so a "Rules: <Anthropic version>" line
// above them names rules nothing used.
func rulesLine(t *testing.T, models ...string) string {
	t.Helper()
	var reqs []*transcript.Request
	for _, m := range models {
		reqs = append(reqs, &transcript.Request{Model: m})
	}
	lane := &transcript.Lane{Requests: reqs}
	sess := &transcript.Session{ID: "sess-1", ClientVersion: "1.0.0", Source: transcript.SourceLedger, Lanes: []*transcript.Lane{lane}}
	r := &LaneReport{Session: sess, Lane: lane, Calibration: &Calibration{}}
	var b bytes.Buffer
	r.header(NewPrinter(&b))
	for _, line := range strings.Split(b.String(), "\n") {
		if strings.HasPrefix(line, "Rules: ") {
			return line
		}
	}
	t.Fatalf("no Rules line in the header:\n%s", b.String())
	return ""
}

func TestRLR1_AModelWithoutRulesIsNotLabelledWithAnthropicRules(t *testing.T) {
	line := rulesLine(t, "gpt-5", "gpt-5")
	if strings.Contains(line, cachemodel.RulesVersionInEffect()) {
		t.Errorf("a gpt-5 session is labelled with Anthropic's rules: %s", line)
	}
	if !strings.Contains(line, "no published cache rules") || !strings.Contains(line, "gpt-5") {
		t.Errorf("the line does not say gpt-5 has no published cache rules: %s", line)
	}
}

func TestRLR2_MixedAndAnthropicSessionsNameWhatApplied(t *testing.T) {
	if line := rulesLine(t, "claude-opus-5"); !strings.HasPrefix(line, "Rules: "+cachemodel.RulesVersionInEffect()+";") {
		t.Errorf("an Anthropic session's rules line changed: %s", line)
	}
	line := rulesLine(t, "claude-opus-5", "o4-mini")
	if !strings.Contains(line, cachemodel.RulesVersionInEffect()) || !strings.Contains(line, "o4-mini") {
		t.Errorf("a mixed session does not name both: %s", line)
	}
	if line := rulesLine(t, "gpt-6-astra"); strings.Contains(line, cachemodel.RulesVersionInEffect()) || !strings.Contains(line, "Astra") {
		t.Errorf("an Astra session is not labelled with Astra's rules: %s", line)
	}
}
