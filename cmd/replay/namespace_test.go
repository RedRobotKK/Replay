package main

import (
	"strings"
	"testing"
)

// Which route the traffic took, because it bounds who the tool can pay for.
//
// docs/WHAT-YOU-GET.md says Replay does not pay for itself on a flat seat: a
// broken cache costs a Max or Team subscriber nothing. So "is this person
// billed per token" is the precondition for the whole product, and nothing
// measures it.
//
// The model id is a free, content-free discriminator that is already parsed.
// Bedrock and Vertex ids are unambiguous: neither offers a flat seat, so that
// traffic is metered. A bare first-party id is genuinely ambiguous — the same
// string appears whether the caller holds an API key or a subscription — and
// the card must say so rather than guess, because guessing here would invent
// the one number the product's case rests on.

// N1: the three routes are told apart, and unknown ids are not forced.
//
// PASS: each id lands in its own bucket; nonsense lands in "other".
// FAIL: a Bedrock or Vertex id read as first-party, which would understate
// the only population that is certainly metered.
func TestN1_ModelIdsAreClassifiedByRoute(t *testing.T) {
	cases := []struct{ id, want string }{
		{"claude-opus-5", routeFirstParty},
		{"claude-sonnet-5-20260101", routeFirstParty},
		{"us.anthropic.claude-opus-5-v1:0", routeBedrock},
		{"anthropic.claude-3-5-sonnet-20241022-v2:0", routeBedrock},
		{"arn:aws:bedrock:us-east-1::foundation-model/anthropic.claude-opus-5", routeBedrock},
		{"publishers/anthropic/models/claude-opus-5", routeVertex},
		{"claude-opus-5@20260101", routeVertex},
		{"", routeOther},
		{"gpt-4o", routeOther},
	}
	for _, c := range cases {
		if got := classifyRoute(c.id); got != c.want {
			t.Errorf("classifyRoute(%q) = %q, want %q", c.id, got, c.want)
		}
	}
}

// N2: the card names the route and refuses to claim what it cannot see.
//
// PASS: a first-party-only corpus is reported as first-party WITHOUT being
// called metered.
// FAIL: any wording asserting the billing mode from a first-party id. That
// would be the product inventing evidence for its own precondition.
func TestN2_FirstPartyIsNotReportedAsMetered(t *testing.T) {
	got := routeLine([]string{"claude-opus-5", "claude-opus-5", "claude-haiku-4-5"})
	if !strings.Contains(got, routeFirstParty) {
		t.Errorf("route line does not name the route: %q", got)
	}
	for _, forbidden := range []string{"metered", "per token", "pay-as-you-go"} {
		if strings.Contains(strings.ToLower(got), forbidden) {
			t.Errorf("a bare first-party id was reported as %q: %q", forbidden, got)
		}
	}
}

// N3: a metered route is named as such, because that one is unambiguous.
//
// PASS: Bedbrock or Vertex present means the line says metered.
// FAIL: silence, which throws away the only certain signal available.
func TestN3_BedrockAndVertexAreNamedMetered(t *testing.T) {
	for _, id := range []string{"us.anthropic.claude-opus-5-v1:0", "publishers/anthropic/models/claude-opus-5"} {
		got := routeLine([]string{id})
		if !strings.Contains(strings.ToLower(got), "metered") {
			t.Errorf("%s traffic is metered by construction and the line does not say so: %q", id, got)
		}
	}
}

// N4: mixed routes are reported as mixed, not as the majority.
//
// PASS: both named.
// FAIL: the minority dropped — the shape that hides the one Bedrock account
// in a corpus of subscriptions.
func TestN4_MixedRoutesAreBothNamed(t *testing.T) {
	got := routeLine([]string{"claude-opus-5", "claude-opus-5", "us.anthropic.claude-opus-5-v1:0"})
	if !strings.Contains(got, routeFirstParty) || !strings.Contains(got, routeBedrock) {
		t.Errorf("a mixed corpus reported only one route: %q", got)
	}
}

// N5: the route line carries no identifier.
//
// The card is built to be posted publicly and already refuses totals and
// paths. A route is a category of at most four values; a model id is not.
//
// PASS: only the bucket names appear.
// FAIL: any raw id, which would put an account's exact model and region on a
// card people paste into public threads.
func TestN5_TheRouteLineLeaksNoIdentifier(t *testing.T) {
	got := routeLine([]string{"arn:aws:bedrock:us-east-1::foundation-model/anthropic.claude-opus-5"})
	for _, leak := range []string{"arn:", "us-east-1", "foundation-model", "v1:0"} {
		if strings.Contains(got, leak) {
			t.Errorf("the route line leaked %q: %q", leak, got)
		}
	}
}

// N6: no models, no line.
//
// PASS: empty input yields an empty line rather than "other".
// FAIL: a card asserting a route it did not observe.
func TestN6_NoModelsYieldsNoLine(t *testing.T) {
	if got := routeLine(nil); got != "" {
		t.Errorf("routeLine(nil) = %q, want empty", got)
	}
}

// N7: the card prints the route when one was observed, and omits the line
// entirely when none was.
//
// PASS: a summary carrying a route prints it; one carrying none prints no
// label at all.
// FAIL: an empty "routed via" label, which reads as a measurement returning
// nothing rather than as a measurement not taken.
func TestN7_TheCardCarriesTheRouteOnlyWhenObserved(t *testing.T) {
	base := costSummary{Tasks: 12, MedianUSD: 1.10, P90USD: 4.00, AvoidableShare: 0.11, TotalUSD: 90}

	with := base
	with.Route = routeLine([]string{"claude-opus-5"})
	card := shareCard(with, 3)
	if !strings.Contains(card, routeFirstParty) {
		t.Errorf("the card dropped an observed route:\n%s", card)
	}

	without := shareCard(base, 3)
	if strings.Contains(without, "routed via") {
		t.Errorf("the card printed a route label with no route observed:\n%s", without)
	}
}

// N8: summarise derives the route from the units it was given.
//
// PASS: a mixed set reports both routes.
// FAIL: the field left empty, which would make the card silently routeless on
// every real run while N7 stayed green on a hand-built summary.
func TestN8_SummariseDerivesTheRoute(t *testing.T) {
	got := summarise([]costUnit{
		{ID: "a", Model: "claude-opus-5", CostUSD: 1},
		{ID: "b", Model: "us.anthropic.claude-opus-5-v1:0", CostUSD: 2},
	}).Route
	if !strings.Contains(got, routeFirstParty) || !strings.Contains(got, routeBedrock) {
		t.Errorf("summarise reported route %q, want both routes named", got)
	}
}
