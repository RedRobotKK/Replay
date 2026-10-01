package cachemodel

import (
	"math"
	"testing"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// Epistemic compression at the TTL breakdown.
//
// The wire type carries presence correctly: WireUsage.CacheBreak is a pointer
// and nil means the provider sent no TTL breakdown. Usage() then drops that
// bit, because transcript.Usage has no companion presence field, so an absent
// breakdown and a present-but-zero one arrive identically as Create5m == 0 &&
// Create1h == 0.
//
// writeEquivalent reads that state as "no split reported" and prices the whole
// cache-write leg at the SHORT multiplier.
//
// The collapse is documented at the site. TTLOf says "Without a breakdown the
// provider default applies" and writeEquivalent says "or at the short
// multiplier when no split was reported". So this is an INTENTIONAL collapse
// at the implementation level, and the question these tests ask is a different
// one: does the resulting ASSUMPTION reach the reader of the dollar figure?

const ttlTol = 1e-9

// EC1. The collapse itself, demonstrated rather than asserted.
func TestEC1_AbsentAndZeroTTLBreakdownAreIndistinguishable(t *testing.T) {
	absent := (&transcript.WireUsage{Input: 100, CacheCreation: 10_000, CacheRead: 0, Output: 50}).Usage()

	zeroed := &transcript.WireUsage{Input: 100, CacheCreation: 10_000, CacheRead: 0, Output: 50}
	zeroed.CacheBreak = &struct {
		Short int `json:"ephemeral_5m_input_tokens"`
		Long  int `json:"ephemeral_1h_input_tokens"`
	}{Short: 0, Long: 0}
	present := zeroed.Usage()

	if absent.Create5m != present.Create5m || absent.Create1h != present.Create1h {
		t.Fatalf("the two states already differ (absent %d/%d, present %d/%d); this test "+
			"no longer observes the collapse it was written for",
			absent.Create5m, absent.Create1h, present.Create5m, present.Create1h)
	}
	t.Logf("COLLAPSE CONFIRMED: provider sent no breakdown, and provider sent a "+
		"breakdown of 0/0, both arrive as Create5m=%d Create1h=%d. The presence bit "+
		"the wire type carried is gone and nothing downstream can recover it.",
		absent.Create5m, absent.Create1h)
}

// EC2. The consequence in money. An absent breakdown is priced at the short
// multiplier, which is an ASSUMPTION about the provider's default and not a
// figure anybody reported.
//
// The oracle is longhand, not writeEquivalent.
func TestEC2_AnAbsentBreakdownIsPricedOnAnAssumedTTL(t *testing.T) {
	p := Price{InputPerMTok: 3.0, OutputPerMTok: 15.0, ReadMult: 0.1}
	const writes = 10_000

	absent := transcript.Usage{Input: 0, CacheCreation: writes, Output: 0}
	asLong := transcript.Usage{Input: 0, CacheCreation: writes, Create1h: writes, Output: 0}

	gotAbsent := CostUSD(absent, p)
	gotLong := CostUSD(asLong, p)

	// Independent arithmetic, from the published multipliers.
	in := 3.0 / 1_000_000.0
	wantShort := float64(writes) * 1.25 * in
	wantLong := float64(writes) * 2.0 * in

	if math.Abs(gotAbsent-wantShort) > ttlTol {
		t.Fatalf("absent breakdown priced %.9f, independent short-multiplier arithmetic "+
			"says %.9f", gotAbsent, wantShort)
	}
	if math.Abs(gotLong-wantLong) > ttlTol {
		t.Fatalf("explicit 1h priced %.9f, independent long-multiplier arithmetic says %.9f",
			gotLong, wantLong)
	}

	t.Logf("SAME cache_creation of %d tokens prices at $%.6f when no TTL breakdown was "+
		"reported and $%.6f when the provider said the writes were 1h. A %.0f%% "+
		"difference decided by a field the provider may simply not have sent, and the "+
		"short multiplier is applied as a default rather than as a measurement.",
		writes, gotAbsent, gotLong, (gotLong/gotAbsent-1)*100)
}

// EC3. What the contract actually says, pinned.
//
// This is the part that keeps the finding honest. The collapse is NOT a
// silent bug: both sites document it. So the classification is
// INTENTIONALLY_COLLAPSED at the implementation, and the open question is
// disclosure, which is a different claim.
func TestEC3_TheAssumedDefaultIsDocumentedAtTheSite(t *testing.T) {
	// TTLOf must return the short TTL for an absent breakdown, as documented.
	if got := TTLOf(transcript.Usage{CacheCreation: 10_000}); got != TTLShort {
		t.Errorf("TTLOf on an absent breakdown = %v, want %v. The documented default "+
			"changed and the claim register must be updated.", got, TTLShort)
	}
	// And it must still honour an explicit long breakdown, or the default is
	// not a default, it is the only behaviour.
	if got := TTLOf(transcript.Usage{CacheCreation: 10_000, Create1h: 10_000}); got != TTLLong {
		t.Errorf("TTLOf on an explicit 1h breakdown = %v, want %v. The breakdown is "+
			"being ignored entirely.", got, TTLLong)
	}
}

// EC4. The metamorphic attack, stated as the contract requires.
//
// Replacing evidence E with E' differing ONLY in the TTL breakdown must change
// the priced figure, because the contract prices the two TTLs differently.
// This passes, which means the distinction IS honoured wherever the provider
// supplies it. The collapse bites only where the provider supplies nothing.
func TestEC4_TheTTLDistinctionSurvivesWhereTheProviderSuppliesIt(t *testing.T) {
	p := Price{InputPerMTok: 3.0, OutputPerMTok: 15.0, ReadMult: 0.1}
	short := transcript.Usage{CacheCreation: 8_000, Create5m: 8_000}
	long := transcript.Usage{CacheCreation: 8_000, Create1h: 8_000}

	if math.Abs(CostUSD(short, p)-CostUSD(long, p)) < ttlTol {
		t.Error("a 5m breakdown and a 1h breakdown of the same size price identically. " +
			"The distinction is lost even where the provider reported it, which would " +
			"be a much larger finding than the absent-breakdown case.")
	}
}
