package observation

// The pooled document, vendored.
//
// WHY IT IS COMPILED IN RATHER THAN FETCHED.
//
// Replay originates no network request the reader did not ask for. That is the
// standing claim, it is why the installer path carries no analytics, and a
// binary that quietly fetched a comparison corpus at runtime would break it for
// a feature nobody asked for. So the pool travels the way the price table
// travels: a dated value in the binary, refreshed when a release is cut.
//
// The cost is that this snapshot ages. PooledAt is printed on the population
// line of every comparison derived from it, and AgeNote warns past
// PooledStaleDays. Both of those were promised by an earlier version of this
// comment and implemented by neither, which a review caught.
//
// REGENERATED, NOT HAND-EDITED.
//
// The values below are `totals` from the published pooled document at
// https://replay.doctor/pool/replay-pool.json, which a daily job produces by
// running `replay pool` over every contributed submission. Copying a figure in
// by hand is how the two drift; pooled_test.go checks the shape, and the
// release checklist is what checks the date.
//
// WHAT IT SAYS TODAY, HONESTLY.
//
// One submission carrying one machine tag, pooled 2026-09-12. That is the
// author's own machine and nothing else, which is why Benchmark refuses to
// compare against it: telling a reader they are "above the pool" would mean
// telling them they are above Daniel. The refusal names the reason and points
// at `replay cost --contribute`, which is the only thing that changes it.

// PooledAt is the date the vendored pool was produced.
const PooledAt = "2026-09-12"

// PooledStaleDays is when a vendored pool stops being worth leaning on.
//
// Ninety days, and the reasoning differs from the price table's sixty. Prices
// move on the provider's schedule; this document is produced by a daily job, so
// a copy that has not moved in a quarter says the vendoring stopped rather than
// that nobody contributed. AgeNote prints it and pooledstale_test.go fails on
// it, because the previous version of this file claimed "the release checklist
// is what checks the date" and the release criteria do not mention the pool.
const PooledStaleDays = 90

// PooledSchema is the document version these values were read from.
const PooledSchema = "replay.pool.v1"

// Pooled returns the vendored pooled totals.
func Pooled() PoolTotals {
	return PoolTotals{
		Submissions:           1,
		DistinctTags:          1,
		TagsAreIdentities:     false,
		SupersededSubmissions: 0,
		Tasks:                 121,
		TotalUSD:              11968.212283200028,
		AvoidableUSD:          334.11072200000007,
		AvoidableShare:        0.027916510343737523,
		Unpriced:              0,
		MedianTaskUSDLow:      0.7892915,
		MedianTaskUSDHigh:     0.7892915,
		// Every submission's own median, sorted. One submission, so one value.
		MedianTaskUSDs:   []float64{0.7892915},
		RulesVersion:     "anthropic-2026-09-01",
		PricedAtLow:      "2026-09-07",
		PricedAtHigh:     "2026-09-07",
		PricedAtDistinct: 1,
	}
}
