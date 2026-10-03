package main

import (
	"bytes"
	"strings"
	"testing"
)

// SP: CLAIM CONSERVATION ACROSS PUBLIC SURFACES.
//
// The canonical claim is costSummary. Every public surface renders some part of
// it. The convergence rule is one sentence: a downstream representation must
// never be stronger than the canonical result it came from.
//
// "Stronger" has an operational meaning here. The canonical summary carries
// three independent statements of what its dollars do NOT cover:
//
//	unpriced          whole transcripts excluded, counted beside the summary
//	UnpricedRequests  records inside priced transcripts that could not be priced
//	UnpricedRebilledTokens  break tokens outside the re-billed dollar figure
//
// A surface that prints the dollars and drops all three has made the claim
// stronger than the summary did, because the reader cannot tell a complete
// figure from a partial one.
//
// This file tests the CEILING GATE first, because its output is the one a user
// acts on: a PASS is a green build.

// c2Summary is a canonical claim with a known, deliberate hole in it: the
// dollars cover part of the work and the summary says so three ways.
func c2Summary() costSummary {
	return costSummary{
		Unit:                   unitSession,
		Tasks:                  2,
		TotalUSD:               10.00,
		RebilledUSD:            1.00,
		RebilledTokens:         40_000,
		UnpricedRebilledTokens: 20_000, // half the re-billed tokens are outside the dollars
		PricedRequests:         3,
		UnpricedRequests:       1, // a quarter of the records priced nothing
	}
}

// SP-02. The gate's PASS verdict must not be silent about the hole its own
// FAIL verdict discloses.
func TestSP_TheGatePassMustStateWhatItsFigureExcludes(t *testing.T) {
	s := c2Summary()

	var pass bytes.Buffer
	if err := checkRebilledCeiling(100.00, s, 2 /*unpriced*/, 1 /*unreadable*/, &pass); err != nil {
		t.Fatalf("the ceiling is far above the figure, so this must pass: %v", err)
	}
	var fail bytes.Buffer
	if err := checkRebilledCeiling(0.50, s, 2, 1, &fail); err == nil {
		t.Fatal("the ceiling is below the figure, so this must fail")
	}

	// The FAIL branch is the positive control: it already does the right thing,
	// so a detector that cannot see disclosure in it is broken, not the gate.
	if !strings.Contains(fail.String(), "excluded as unpriced") {
		t.Fatalf("POSITIVE CONTROL BROKEN: the FAIL branch no longer discloses excluded "+
			"transcripts, so this test cannot tell disclosure from silence.\n%s", fail.String())
	}

	// The claim under test.
	got := pass.String()
	for _, want := range []struct{ what, needle string }{
		{"excluded transcripts", "excluded"},
		{"unreadable transcripts", "could not be read"},
	} {
		if !strings.Contains(got, want.needle) {
			t.Errorf("the PASS verdict does not name %s. The FAIL verdict does, over "+
				"the same summary. A gate that passes is the verdict excluded spend "+
				"could flip, so silence is strongest exactly where it is least "+
				"affordable.\nPASS output:\n%s", want.what, got)
		}
	}
}

// SP-02b. Neither verdict knows about the record-level coverage that C035 and
// C037 put on the summary. The dollars are compared against a ceiling while a
// quarter of the records and half the re-billed tokens sit outside them.
func TestSP_TheGateIsBlindToRecordLevelCoverage(t *testing.T) {
	s := c2Summary()
	for _, arm := range []struct {
		name    string
		ceiling float64
	}{{"pass", 100.00}, {"fail", 0.50}} {
		var b bytes.Buffer
		_ = checkRebilledCeiling(arm.ceiling, s, 0 /*no whole transcripts excluded*/, 0, &b)
		got := b.String()
		// With unpriced==0 the existing disclosures are correctly silent, so
		// anything found here must come from the record-level counters.
		if s.UnpricedRequests > 0 && !strings.Contains(got, "request") {
			t.Errorf("%s verdict: %d of %d requests priced nothing and the gate says "+
				"nothing about them.\n%s", arm.name, s.UnpricedRequests,
				s.PricedRequests+s.UnpricedRequests, got)
		}
		if s.UnpricedRebilledTokens > 0 && !strings.Contains(got, "token") {
			t.Errorf("%s verdict: %d of %d re-billed tokens are outside the dollar "+
				"figure this ceiling compared, and the gate says nothing about "+
				"them.\n%s", arm.name, s.UnpricedRebilledTokens, s.RebilledTokens, got)
		}
	}
}

// SP-02c. NEGATIVE CONTROL. A summary with no hole must produce no disclosure,
// or the repair is a sentence that fires on everything and means nothing.
func TestSP_AFullyCoveredSummaryDisclosesNothing(t *testing.T) {
	s := c2Summary()
	s.UnpricedRequests = 0
	s.UnpricedRebilledTokens = 0

	var b bytes.Buffer
	if err := checkRebilledCeiling(100.00, s, 0, 0, &b); err != nil {
		t.Fatalf("must pass: %v", err)
	}
	got := b.String()
	for _, forbidden := range []string{"excluded", "could not be read", "priced nothing"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("a fully covered summary disclosed %q. A qualifier that appears "+
				"when there is nothing to qualify trains the reader to ignore it.\n%s",
				forbidden, got)
		}
	}
	if !strings.Contains(got, "within the") {
		t.Errorf("the pass verdict stopped saying it passed:\n%s", got)
	}
}
