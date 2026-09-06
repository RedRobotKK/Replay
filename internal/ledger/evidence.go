package ledger

import "github.com/RedRobotKK/Replay/internal/cachemodel"

// EvidenceFrom turns served requests into testimony about where a model's
// prompt cache actually begins.
//
// This is the content behind the maintained rules feed. The compiled table
// carries what a provider documents; this carries what the wire showed. A
// price check on 2026-09-05 found no disagreement across ten models against an
// independent database, 73 days after the table was dated, so currency is not
// the scarce thing. Where the caching floor sits — and whether the published
// figure survives contact with traffic — is not published by anyone, costs
// real API spend to establish, and is the only part that works on a model that
// did not exist when the binary shipped.
//
// Only one direction is sound from what the ledger records today, and the
// asymmetry matters more than the convenience of having both.
//
// An upper bound is exact. `cache_creation_input_tokens` IS the number of
// tokens the provider cached, so one record reporting 512 proves that model
// caches a 512-token prefix and refutes any documented floor above it. No
// inference, no fit, no error bars.
//
// A lower bound is not available. It would need the prompt size AT THE
// BREAKPOINT for a marked request that cached nothing, and the proxy records
// how many `cache_control` markers a request carried but never where they sat
// — `Prompt.CacheControlCount` is a count, and the positions are discarded
// when the request is summarised. Substituting the whole prompt would assert a
// floor above a number far larger than the marked prefix, which is wrong in
// the dangerous direction: it would refute documented figures that are
// correct.
//
// So this produces upper bounds only, and `Claim.Status` reports an open
// interval as `unverified` rather than as agreement. Recording the block index
// of each marker would close the gap; it is a small change to the summariser
// and the single most valuable capture this feed is missing.
func EvidenceFrom(records []Record, machine string) []cachemodel.PrefixEvidence {
	out := make([]cachemodel.PrefixEvidence, 0, len(records))
	for _, r := range records {
		if r.Model == "" || r.Response.Usage == nil {
			continue
		}
		// A request the provider did not serve describes something other than
		// a served response: a guard refused it locally, or it errored, and
		// its usage is not evidence about caching.
		if r.Status != 200 || r.Refusal != "" {
			continue
		}
		created := r.Response.Usage.CacheCreation
		if created <= 0 {
			// No write, so nothing was learned. It is not lower-bound
			// evidence either, for the reason above.
			continue
		}
		// Only a COLD write measures the floor.
		//
		// `cache_creation_input_tokens` is the number of tokens written by
		// this request, not the size of the cached prefix. A turn that reads
		// 20,000 cached tokens and writes 118 more has a 20,118-token prefix;
		// the 118 is an increment on top of an existing entry and says
		// nothing about the smallest prompt that can be cached at all.
		//
		// Reading it as a prefix size produces exactly the wrong answer in the
		// most alarming direction. On the maintainer's own ledger it reported
		// opus-5 caching a 118-token prefix and therefore contradicting a
		// documented floor of 512 — a fabricated refutation, from four real
		// sessions, that looked like the product's headline finding.
		//
		// When the read is zero there was no prior entry, so what was written
		// IS the prefix, and the figure is exact.
		if r.Response.Usage.CacheRead != 0 {
			continue
		}
		out = append(out, cachemodel.PrefixEvidence{
			Model: r.Model,
			// Exact, not estimated: this is the count the provider billed for
			// writing.
			PrefixTokens: created,
			Wrote:        true,
			Marked:       true,
			Session:      r.SessionID,
			Machine:      machine,
		})
	}
	return out
}
