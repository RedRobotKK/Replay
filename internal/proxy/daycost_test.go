package proxy

import (
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

func costRec(session, model string, at time.Time, u transcript.Usage) *ledger.Record {
	r := &ledger.Record{Timestamp: at, SessionID: session}
	r.Model = model
	r.Response.Usage = &u
	return r
}

// /replay/metrics carried no cost figure at all: ListCostUSD was per-session on
// the status page and nothing aggregated it. A proxy whose pitch is knowing
// what a session costs could not answer what today cost.
func TestMetricsCarriesAggregateAndDayCost(t *testing.T) {
	s := newStats()
	day := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return day }

	u := transcript.Usage{Input: 100_000, CacheRead: 900_000, Output: 20_000}
	s.observe(costRec("a", "claude-opus-5", day, u))
	s.observe(costRec("b", "claude-opus-5", day, u))

	m := s.metrics()
	for _, want := range []string{"replay_cost_usd_total", "replay_cost_usd_day"} {
		if !strings.Contains(m, want) {
			t.Fatalf("metrics is missing %s:\n%s", want, m)
		}
	}
	total, dayCost := s.costs()
	if total <= 0 {
		t.Fatalf("two priced requests cost nothing: %.6f", total)
	}
	if dayCost != total {
		t.Fatalf("both requests are on the same UTC day, so the day figure is the total: %.6f vs %.6f", dayCost, total)
	}
}

// The day figure rolls at UTC midnight and the total does not. A cost-per-day
// number that silently carries yesterday's spend is the same failure the day
// cap had before it was persisted.
func TestDayCostRollsAtUTCMidnightAndTheTotalDoesNot(t *testing.T) {
	s := newStats()
	first := time.Date(2026, 9, 4, 23, 59, 0, 0, time.UTC)
	s.now = func() time.Time { return first }
	u := transcript.Usage{Input: 100_000, CacheRead: 900_000, Output: 20_000}
	s.observe(costRec("a", "claude-opus-5", first, u))
	total1, day1 := s.costs()

	next := first.Add(2 * time.Minute) // 2026-09-05 00:01 UTC
	s.now = func() time.Time { return next }
	s.observe(costRec("a", "claude-opus-5", next, u))
	total2, day2 := s.costs()

	if total2 <= total1 {
		t.Fatalf("the running total must keep climbing across midnight: %.6f then %.6f", total1, total2)
	}
	if day2 >= total2 {
		t.Fatalf("the day figure must have dropped yesterday's spend: day %.6f of total %.6f", day2, total2)
	}
	if day2 <= 0 || day2 >= day1*1.5 {
		t.Fatalf("the new day should hold one request's cost: %.6f against yesterday's %.6f", day2, day1)
	}
}

// The rollup is sourced from the stats path, which sees every request, not
// from SpendGuard, which only records when a cap is configured. Cost must be
// reported on a proxy with no caps set at all.
func TestCostIsReportedWithNoCapConfigured(t *testing.T) {
	s := newStats()
	at := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return at }
	s.observe(costRec("a", "claude-opus-5", at, transcript.Usage{Input: 50_000, CacheRead: 500_000}))
	if total, _ := s.costs(); total <= 0 {
		t.Fatal("cost must not depend on a SpendGuard being enabled")
	}
}

// An unpriced model must not contribute a zero that reads as "this was free".
func TestUnpricedTrafficDoesNotFabricateAZeroCost(t *testing.T) {
	s := newStats()
	at := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return at }
	s.observe(costRec("a", "a-model-nobody-priced", at, transcript.Usage{Input: 50_000, CacheRead: 500_000}))
	total, _ := s.costs()
	if total != 0 {
		t.Fatalf("an unpriced model contributed %.6f", total)
	}
	if !strings.Contains(s.metrics(), "replay_cost_unpriced_requests_total 1") {
		t.Fatalf("traffic that could not be priced must be counted, not silently dropped:\n%s", s.metrics())
	}
}
