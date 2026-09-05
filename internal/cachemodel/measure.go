package cachemodel

import (
	"sort"
	"strings"
)

// Measuring where a model's prompt cache actually starts.
//
// The compiled table records what the provider documents. This records what
// replaying real traffic witnessed, and the two answer different questions.
// A price check on 2026-09-05 found zero disagreement across ten models
// against an independent database, 73 days after the table was dated — so
// currency is not the scarce thing. What the provider does not publish, and
// what costs real API spend to establish, is where the caching floor sits and
// whether the published figure survives contact with traffic.
//
// It is also the only thing that works on a model nobody has documented yet. A
// compiled table can only contain what existed when the binary shipped; a new
// model appears and the table has nothing, while traffic against it starts the
// same day. The measurement needs no table entry, so an unknown model gets a
// real bound immediately.

// PrefixEvidence is one request's worth of testimony about the caching floor.
//
// Marked is the load-bearing field. A request with no cache breakpoint was
// never going to cache, so its silence says nothing about where the floor is —
// and counting it would push the reported floor arbitrarily high.
type PrefixEvidence struct {
	Model        string
	PrefixTokens int
	// Wrote is true when the provider reported creating a cache entry.
	Wrote bool
	// Marked is true when the request carried a cache breakpoint at all.
	Marked bool
	// Session and Machine measure breadth. Agreement is only as good as its
	// sampling, so what matters is how varied the sources were, not how many
	// requests one chatty session made.
	Session string
	Machine string
}

// MeasureClaims turns evidence into a claim per model.
//
// Bounds are the tightest the evidence supports: the smallest prompt seen to
// cache is the upper bound, and the largest marked prompt seen NOT to cache is
// the lower bound. Both are kept even when they disagree — see Incoherent.
func MeasureClaims(evidence []PrefixEvidence) map[string]Claim {
	type acc struct {
		upper, lower *int
		sessions     map[string]struct{}
		machines     map[string]struct{}
	}
	byModel := map[string]*acc{}

	for _, e := range evidence {
		if e.Model == "" || e.PrefixTokens < 0 {
			continue
		}
		// A request with no breakpoint is not testimony. It could not have
		// cached whatever its size, so neither its silence nor its size tells
		// us anything about the floor.
		if !e.Marked {
			continue
		}
		a := byModel[e.Model]
		if a == nil {
			a = &acc{sessions: map[string]struct{}{}, machines: map[string]struct{}{}}
			byModel[e.Model] = a
		}
		if e.Session != "" {
			a.sessions[e.Session] = struct{}{}
		}
		if e.Machine != "" {
			a.machines[e.Machine] = struct{}{}
		}
		n := e.PrefixTokens
		if e.Wrote {
			// It cached at n, so the floor is at or below n.
			if a.upper == nil || n < *a.upper {
				v := n
				a.upper = &v
			}
			continue
		}
		// It was marked and did not cache, so the floor is above n.
		if a.lower == nil || n > *a.lower {
			v := n
			a.lower = &v
		}
	}

	out := make(map[string]Claim, len(byModel))
	for model, a := range byModel {
		if a.upper == nil && a.lower == nil {
			continue
		}
		out[model] = Claim{
			Documented: DocumentedMinPrefix(model),
			Observed: &Observation{
				UpperBound: a.upper,
				LowerBound: a.lower,
				Sessions:   len(a.sessions),
				Machines:   len(a.machines),
			},
		}
	}
	return out
}

// Incoherent reports evidence that cannot all be true of a single floor: a
// prompt cached at a size another marked prompt failed to cache at.
//
// This is surfaced rather than smoothed away. It is a real result — it says
// the floor is not the whole story for that model, and something else (a
// block granularity, a per-account difference, a change over time) is moving
// the boundary. Averaging the two into a clean number would manufacture an
// answer the evidence does not support and hide the interesting part.
func Incoherent(o Observation) bool {
	return o.UpperBound != nil && o.LowerBound != nil && *o.LowerBound >= *o.UpperBound
}

// DocumentedMinPrefix is the published floor for a model, or 0 when the
// compiled table does not name it.
//
// Zero means "no published figure", not "caches from the first token". Callers
// must not print it as a number; Claim.Status treats a zero documented figure
// as nothing to agree or disagree with.
func DocumentedMinPrefix(model string) int {
	if r, ok := activeRow(model); ok {
		return r.MinPrefix
	}
	for _, row := range modelTable {
		if containsFold(model, row.match) {
			return row.minPrefix
		}
	}
	return 0
}

// MeasuredModels lists the models a claim set covers, in a stable order so
// that a generated document does not churn between runs.
func MeasuredModels(claims map[string]Claim) []string {
	out := make([]string, 0, len(claims))
	for m := range claims {
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}

// containsFold is a case-insensitive substring test, matching how the compiled
// table is matched elsewhere. lookup() lowercases before comparing, and a
// documented figure that applied only to lowercase model ids would be a
// different bug of the same family.
func containsFold(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}
