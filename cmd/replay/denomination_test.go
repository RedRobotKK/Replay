package main

import (
	"io"
	"strings"
	"testing"
)

// The report must be readable by someone who does not spend dollars.
//
// docs/WHAT-YOU-GET.md is blunt: "It does not pay for itself on a flat seat.
// Claude Max, Team, Copilot, Cursor: a broken prompt cache costs you nothing."
// Every figure this report leads with is denominated in a currency that
// roughly 80-85% of users do not spend, so for most readers the headline is
// literally $0.00 of relevance, printed with two decimal places of confidence.
//
// The waste is real for them; it is just not money. It is context that the
// work did not get and rate-limit budget spent on nothing. Both are already
// measured - the deficit is in tokens before it is ever multiplied by a price.
//
// So the tokens are stated beside the dollars, and the report says plainly who
// each figure is for. That is a presentation change over data already in hand,
// not a new measurement.

// DN-1: avoidable is reported in tokens as well as dollars.
//
// PASS: both denominations present.
// FAIL: dollars alone, which is the current report and is meaningless to a
// subscriber.
func TestDN1_AvoidableIsAlsoInTokens(t *testing.T) {
	s := costSummary{
		Tasks: 1477, TotalUSD: 3000.56, MedianUSD: 0.65, P90USD: 2.21,
		AvoidableUSD: 149.44, AvoidableShare: 0.0498, AvoidableTokens: 31_264_349,
	}
	out := renderCost(s, 0, io.Discard, "")
	if !strings.Contains(out, "31.3M") {
		t.Errorf("the avoidable figure is not stated in tokens:\n%s", out)
	}
	if !strings.Contains(out, "149.44") {
		t.Errorf("the dollar figure must survive for metered readers:\n%s", out)
	}
}

// DN-2: the report says who the dollar figure is for.
//
// PASS: a line naming the flat-seat case explicitly.
// FAIL: silence, which lets a subscriber read $149.44 as money they lost.
func TestDN2_TheReportNamesWhoPaysDollars(t *testing.T) {
	s := costSummary{
		Tasks: 1477, TotalUSD: 3000.56, MedianUSD: 0.65, P90USD: 2.21,
		AvoidableUSD: 149.44, AvoidableShare: 0.0498, AvoidableTokens: 31_264_349,
	}
	out := strings.ToLower(renderCost(s, 0, io.Discard, ""))
	if !strings.Contains(out, "subscription") && !strings.Contains(out, "flat seat") {
		t.Errorf("nothing tells a subscriber the dollars are not theirs:\n%s", out)
	}
}

// DN-3: no tokens measured, no token claim.
//
// PASS: the line is omitted rather than printed as 0.
// FAIL: "0 tokens re-billed", which reads as a measurement that found nothing
// rather than one that was not taken.
func TestDN3_NoTokensNoClaim(t *testing.T) {
	s := costSummary{Tasks: 3, TotalUSD: 10, MedianUSD: 1, AvoidableUSD: 0.5, AvoidableShare: 0.05}
	out := renderCost(s, 0, io.Discard, "")
	if strings.Contains(out, "0 tokens re-billed") || strings.Contains(out, "0.0M") {
		t.Errorf("printed a token figure it did not measure:\n%s", out)
	}
}
