package analysis

import (
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

func TestTallyAddAt_UsesTheRequestTimestamp(t *testing.T) {
	r := &cachemodel.Rules{
		Schema:  cachemodel.RulesSchema,
		Version: "test",
		Models: []cachemodel.ModelRule{
			{Match: "opus-5", MinPrefix: 512, InputPerMTok: 10, OutputPerMTok: 50, ReadMult: 0.1, Priced: true},
			{Match: "opus-5", MinPrefix: 512, InputPerMTok: 5, OutputPerMTok: 25, ReadMult: 0.1, Priced: true,
				EffectiveFrom: "2026-09-01", EffectiveUntil: "2026-09-30"},
		},
	}
	defer cachemodel.Override(r)()
	u := transcript.Usage{Input: 1_000_000}
	var inside, outside Tally
	inside.AddAt(u, "claude-opus-5", time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC))
	outside.AddAt(u, "claude-opus-5", time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC))
	if inside.CostUSD == outside.CostUSD {
		t.Fatalf("same usage priced the same inside and outside a dated window: $%.2f", inside.CostUSD)
	}
	if inside.UncachedUSD <= 0 || outside.UncachedUSD <= 0 {
		t.Fatalf("legs missing: inside %+v outside %+v", inside, outside)
	}
}
