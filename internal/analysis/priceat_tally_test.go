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

// WithTTL still called Tally.Add, which is PriceForAt at the zero time —
// today's row. Replaying the TTL a session actually used has to price at
// the request timestamp or as-run and the replay disagree on dollars
// across a dated window, for no reason the tokens can see.
func TestWithTTL_UsesTheRequestTimestamp(t *testing.T) {
	r := &cachemodel.Rules{
		Schema:  cachemodel.RulesSchema,
		Version: "test",
		Models: []cachemodel.ModelRule{
			{Match: "fable-5-1", MinPrefix: 512, InputPerMTok: 10, OutputPerMTok: 50, ReadMult: 0.1, Priced: true},
			{Match: "fable-5-1", MinPrefix: 512, InputPerMTok: 5, OutputPerMTok: 25, ReadMult: 0.1, Priced: true,
				EffectiveFrom: "2026-09-01", EffectiveUntil: "2026-09-30"},
		},
	}
	defer cachemodel.Override(r)()

	lane := syntheticLane(4, -1, 0, true)
	asRun := AsRun(lane)
	replayed := WithTTL(Calibrate(lane), cachemodel.TTLOf(lane.Requests[0].Usage))
	if asRun.CostUSD <= 0 {
		t.Fatal("as-run priced nothing")
	}
	if asRun.CostUSD == replayed.CostUSD && asRun.PromptTokens == replayed.PromptTokens {
		// Same tokens, same dollars: either both used the request clock or
		// neither did. Distinguish by comparing against today's row.
		today, _ := cachemodel.PriceFor(lane.Requests[0].Model)
		then, _ := cachemodel.PriceForAt(lane.Requests[0].Model, lane.Requests[0].Timestamp)
		if today.InputPerMTok == then.InputPerMTok {
			t.Fatal("PriceFor and PriceForAt agree; the fixture cannot tell today from request time")
		}
	}
	if asRun.CostUSD != replayed.CostUSD {
		t.Fatalf("WithTTL $%.6f != as-run $%.6f; same TTL must price at the request timestamp, not today",
			replayed.CostUSD, asRun.CostUSD)
	}
}
