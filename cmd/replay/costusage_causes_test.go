package main

import (
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/usage"
)

// UOC: in cost --usage, the cause block accounts for the breaks counted and no
// others (2026-10-16). A session with no priced record is not a row, and its
// breaks are left out of the break count; its causes were still added to the
// cause block, so the causes summed past the breaks they explain.
func TestUOC1_CausesOfASessionThatIsNotARowAreNotCounted(t *testing.T) {
	recs := append(usageFixture(),
		fixtureRecord{0, 9000, 1000, 0, 8000, 100, "gpt-5", "s2"},
		fixtureRecord{1, 9000, 1000, 0, 8000, 100, "gpt-5", "s2"},
		fixtureRecord{2, 9000, 1000, 0, 8000, 100, "o4-mini", "s2"},
	)
	e, err := usage.ParseExport([]byte(exportDoc(true, true, recs)))
	if err != nil {
		t.Fatalf("fixture is not a valid export: %v", err)
	}
	rep, _ := priceUsage(e)
	if rep.Breaks == nil || *rep.Breaks != usageFixtureBreaks {
		t.Fatalf("breaks = %v, want %d (s2 is not a row)", rep.Breaks, usageFixtureBreaks)
	}
	named := 0
	for _, n := range rep.Causes {
		named += n
	}
	if named+*rep.CauseNotMeasured != *rep.Breaks {
		t.Errorf("causes account for %d of %d breaks: %v, not measured %d", named+*rep.CauseNotMeasured, *rep.Breaks, rep.Causes, *rep.CauseNotMeasured)
	}
}

// UORL: the usage report's header names the rules applied. A session that is a
// row and ran gpt-5 had its breaks classified with no rules.
func TestUORL1_TheUsageHeaderNamesAModelWithoutRules(t *testing.T) {
	recs := []fixtureRecord{
		{0, 9000, 1000, 0, 8000, 100, usageFixtureModel, "s1"},
		{1, 9000, 1000, 0, 8000, 100, "gpt-5", "s1"},
	}
	e, err := usage.ParseExport([]byte(exportDoc(true, true, recs)))
	if err != nil {
		t.Fatalf("fixture is not a valid export: %v", err)
	}
	rep, rows := priceUsage(e)
	out := renderUsageCost(rep, rows, false)
	head := strings.SplitN(out, "\n\n", 2)[0]
	if !strings.Contains(head, cachemodel.RulesVersionInEffect()) {
		t.Errorf("the Anthropic rules that were applied are not named:\n%s", head)
	}
	if !strings.Contains(head, "no published cache rules") || !strings.Contains(head, "gpt-5") {
		t.Errorf("the header does not say gpt-5 has no published cache rules:\n%s", head)
	}

	// The control: an export of Anthropic records only keeps its header.
	rep, rows = priceUsage(parsedFixture(t, true, true))
	out = renderUsageCost(rep, rows, false)
	if strings.Contains(out, "no published cache rules") {
		t.Errorf("an Anthropic-only export names missing rules:\n%s", out)
	}
}
