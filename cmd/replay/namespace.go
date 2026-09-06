package main

import "strings"

// Which route the traffic took to reach the model.
//
// docs/WHAT-YOU-GET.md is blunt that Replay does not pay for itself on a flat
// seat: a broken cache re-bills tokens, and a Max or Team subscriber is not
// billed per token, so the finding is real and costs them nothing. Every
// question downstream of that — who the tool is for, what a share card is
// evidence of — turns on whether the reader is metered. Nothing measured it.
//
// The model id is already parsed on every request and is a free discriminator,
// but it answers a narrower question than it first appears to. Bedrock and
// Vertex ids are decisive: neither offers a flat seat, so that traffic is
// metered by construction. A bare first-party id is not, because the same
// string is emitted whether the caller holds an API key or a subscription.
// So the route is reported and the billing mode is claimed only where the id
// actually settles it.
//
// Measured across the 1,471-file corpus behind docs/evidence, every id was
// first-party — not one Bedrock ARN or Vertex publisher path — which is the
// result this exists to keep honest rather than to assume.
const (
	routeFirstParty = "first-party API"
	routeBedrock    = "Bedrock"
	routeVertex     = "Vertex"
	routeOther      = "other"
)

// classifyRoute buckets one model id. An id it does not recognise is "other",
// never forced into a route, because a wrong route here would be reported with
// the same confidence as a measured one.
func classifyRoute(id string) string {
	switch {
	case id == "":
		return routeOther
	// Vertex first: a Vertex publisher path also contains the Bedrock-ish
	// substring "anthropic.", and the version suffix form carries no vendor
	// token at all.
	case strings.Contains(id, "publishers/anthropic/models/"):
		return routeVertex
	case strings.Contains(id, "@"):
		return routeVertex
	case strings.HasPrefix(id, "arn:aws:bedrock"):
		return routeBedrock
	case strings.Contains(id, "anthropic.claude"):
		return routeBedrock
	case strings.HasPrefix(id, "claude-"):
		return routeFirstParty
	default:
		return routeOther
	}
}

// routeLine describes the routes a set of requests took, in one short phrase
// fit to print on a card that gets posted publicly.
//
// It names a category, never an id: the input can carry a region and an account
// path, and none of that reaches the output.
func routeLine(models []string) string {
	if len(models) == 0 {
		return ""
	}
	present := map[string]bool{}
	for _, m := range models {
		present[classifyRoute(m)] = true
	}
	// Stable, most-common-first, so a card reads the same way twice.
	var named []string
	for _, r := range []string{routeFirstParty, routeBedrock, routeVertex, routeOther} {
		if present[r] {
			named = append(named, r)
		}
	}
	if len(named) == 0 {
		return ""
	}
	line := strings.Join(named, " + ")

	metered := present[routeBedrock] || present[routeVertex]
	ambiguous := present[routeFirstParty] || present[routeOther]
	switch {
	case metered && ambiguous:
		// Some of this traffic is certainly per-token and some cannot be told
		// apart. Saying "metered" flat would claim the part that is unknown.
		return line + ", partly metered"
	case metered:
		return line + ", metered"
	default:
		// A bare first-party id is emitted by an API key and by a subscription
		// alike. The route is all this can honestly report.
		return line
	}
}
