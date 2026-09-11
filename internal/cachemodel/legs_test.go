package cachemodel

import (
	"testing"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

func TestCostLegsUSD_SumsToCostUSD(t *testing.T) {
	p, ok := PriceFor("claude-opus-5")
	if !ok {
		t.Fatal("opus-5 must be priced")
	}
	u := transcript.Usage{Input: 1000, CacheCreation: 0, Create5m: 2000, Create1h: 8000, CacheRead: 100_000, Output: 50}
	legs := CostLegsUSD(u, p)
	got := legs.Total()
	want := CostUSD(u, p)
	if got != want {
		t.Fatalf("legs total %v, CostUSD %v", got, want)
	}
	if legs.Write <= legs.Uncached {
		t.Fatalf("1h writes should dominate uncached on this fixture: %+v", legs)
	}
	if legs.Read <= 0 || legs.Output <= 0 {
		t.Fatalf("read and output must be positive: %+v", legs)
	}
}

func TestCostLegsUSD_AReadCutIsNotAnInputCut(t *testing.T) {
	p, ok := PriceFor("claude-opus-5")
	if !ok {
		t.Fatal("opus-5 must be priced")
	}
	u := transcript.Usage{CacheRead: 1_000_000, Output: 0}
	legs := CostLegsUSD(u, p)
	inputPriced := 1_000_000 / 1e6 * p.InputPerMTok
	if legs.Read >= inputPriced {
		t.Fatalf("cache-read of 1M tokens priced as input ($%.2f); read multiple is %.3f so want $%.2f",
			inputPriced, p.ReadMult, legs.Read)
	}
}
