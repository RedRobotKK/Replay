package main

import (
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
)

// `replay cost` printed "priced at list rates (anthropic-2026-09-01)", which is
// RulesVersionInEffect — the caching rules, not the prices. The two are
// different documents with different dates, and on 2026-09-05 they were 73 days
// apart. Citing the rules date beside a dollar total tells the reader the money
// is as current as that date when it is not.
//
// The rules govern what gets cached. The price table governs what it costs.
// A dollar figure must cite the second.
func TestCostHeaderCitesThePriceTable(t *testing.T) {
	head := costHeaderLine(1384)

	// The date attached to the word "prices" must be the price table's.
	if !strings.Contains(head, "list prices dated "+cachemodel.PriceTableVersion) {
		t.Errorf("the dollar figures must cite the price table (%s), got: %s",
			cachemodel.PriceTableVersion, head)
	}
	// The rules version may appear, but only labelled as what it governs, so it
	// is never read as the date the money came from.
	if i := strings.Index(head, cachemodel.RulesVersionInEffect()); i >= 0 {
		if !strings.Contains(head[:i], "caching rules") {
			t.Errorf("the rules version appears unlabelled beside a dollar total, "+
				"which reads as the price date: %s", head)
		}
	}
}
