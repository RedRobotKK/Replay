package observation

import "testing"

// withCounts returns a corpus reporting the optional break counters, or not.
// Digested() is re-run because the counters are part of what is digested.
func withCounts(tag string, tasks int, breaks, rereads *int) Corpus {
	c := corpusFor(tag, "2026-09-12T00:00:00Z", tasks, 100, 5, 1)
	c.CacheBreaks, c.ReReads = breaks, rereads
	return c.Digested()
}

func n(v int) *int { return &v }

// Breaks and re-reads are additive, which is the whole reason to pool them:
// they are the only unclaimed primitives in what contributors already send.
func TestPoolBreaks_TheyAddUp(t *testing.T) {
	var p Pool
	mustAdd(t, &p, withCounts("a", 100, n(30), n(12)))
	mustAdd(t, &p, withCounts("b", 50, n(9), n(4)))

	got, err := p.Totals()
	if err != nil {
		t.Fatal(err)
	}
	if got.CacheBreaks == nil || *got.CacheBreaks != 39 {
		t.Errorf("cache breaks = %v, want 39", got.CacheBreaks)
	}
	if got.ReReads == nil || *got.ReReads != 16 {
		t.Errorf("re-reads = %v, want 16", got.ReReads)
	}
	if got.BreaksTasks != 150 || got.BreaksReportedBy != 2 {
		t.Errorf("denominator = %d tasks over %d submissions, want 150 over 2",
			got.BreaksTasks, got.BreaksReportedBy)
	}
}

// The counters are optional, so the sum's population is not the pool's. A pool
// where some contributors stayed quiet must not offer their tasks as the
// denominator: breaks per session would be understated by exactly the share of
// the pool that said nothing.
func TestPoolBreaks_TheDenominatorIsOnlyWhoReported(t *testing.T) {
	var p Pool
	mustAdd(t, &p, withCounts("a", 100, n(30), n(12)))
	mustAdd(t, &p, withCounts("quiet", 900, nil, nil))

	got, err := p.Totals()
	if err != nil {
		t.Fatal(err)
	}
	if got.Tasks != 1000 {
		t.Fatalf("pooled tasks = %d, want 1000", got.Tasks)
	}
	if got.BreaksTasks != 100 {
		t.Errorf("breaks denominator = %d, want 100. The quiet submission's 900 tasks are "+
			"in the pool but not behind the breaks figure, and dividing by them would "+
			"understate breaks per session tenfold.", got.BreaksTasks)
	}
	if got.BreaksReportedBy != 1 {
		t.Errorf("reported by %d, want 1", got.BreaksReportedBy)
	}
}

// ADR-0018: absence, zero and unknown are three values. A pool nobody reported
// breaks for has not observed zero breaks.
func TestPoolBreaks_NobodyReportingIsNotZero(t *testing.T) {
	var p Pool
	mustAdd(t, &p, withCounts("a", 100, nil, nil))

	got, err := p.Totals()
	if err != nil {
		t.Fatal(err)
	}
	if got.CacheBreaks != nil {
		t.Errorf("cache breaks = %d for a pool where nobody reported any. A zero here "+
			"claims an observation nobody made.", *got.CacheBreaks)
	}
	if got.ReReads != nil {
		t.Errorf("re-reads = %d for a pool where nobody reported any", *got.ReReads)
	}
	if got.BreaksReportedBy != 0 || got.BreaksTasks != 0 {
		t.Errorf("denominator is %d over %d, want 0 over 0", got.BreaksTasks, got.BreaksReportedBy)
	}
}

// And a reported zero is a measurement, which must survive as one.
func TestPoolBreaks_AReportedZeroIsNotAbsence(t *testing.T) {
	var p Pool
	mustAdd(t, &p, withCounts("a", 100, n(0), n(0)))

	got, err := p.Totals()
	if err != nil {
		t.Fatal(err)
	}
	if got.CacheBreaks == nil {
		t.Fatal("a contributor who measured zero breaks was recorded as never having measured")
	}
	if *got.CacheBreaks != 0 || got.BreaksReportedBy != 1 || got.BreaksTasks != 100 {
		t.Errorf("breaks=%d reportedBy=%d tasks=%d; want 0, 1, 100",
			*got.CacheBreaks, got.BreaksReportedBy, got.BreaksTasks)
	}
}
