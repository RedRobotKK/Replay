package cachemodel

import (
	"math"
	"testing"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// TTL provenance regression (RED-first).
//
// The defect: an absent TTL breakdown is priced at the assumed short (5m)
// multiplier, and the resulting dollar figure is rendered without any
// indication that the TTL basis was assumed rather than measured. These tests
// pin the presence bit through normalization and the classification of the
// write-TTL basis, plus the money arithmetic that must stay unchanged.

const ttlProvTol = 1e-9

// TestWirePresenceSurvivesNormalization pins the first half of the fix: the
// wire type's CacheBreak != nil presence bit must survive into Usage.
func TestWirePresenceSurvivesNormalization(t *testing.T) {
	absent := (&transcript.WireUsage{Input: 0, CacheCreation: 10_000}).Usage()
	present := (&transcript.WireUsage{Input: 0, CacheCreation: 10_000}).Usage()
	_ = absent
	// Explicit 0/0 breakdown: CacheBreak non-nil but both buckets zero.
	zeroed := &transcript.WireUsage{Input: 0, CacheCreation: 10_000}
	zeroed.CacheBreak = &struct {
		Short int `json:"ephemeral_5m_input_tokens"`
		Long  int `json:"ephemeral_1h_input_tokens"`
	}{Short: 0, Long: 0}
	explicit := zeroed.Usage()

	if absent.CreateSplitPresent {
		t.Fatalf("absent breakdown must report CreateSplitPresent=false")
	}
	if !explicit.CreateSplitPresent {
		t.Fatalf("explicit 0/0 breakdown must report CreateSplitPresent=true (distinct from absent)")
	}
	_ = present
}

func TestWriteTTLBasisClassification(t *testing.T) {
	cases := []struct {
		name string
		u    transcript.Usage
		want WriteTTLBasis
	}{
		{"absent_aggregate", transcript.Usage{CacheCreation: 10_000}, WriteBasisAssumedShort},
		{"explicit_5m", transcript.Usage{CacheCreation: 10_000, Create5m: 10_000, CreateSplitPresent: true}, WriteBasisMeasured},
		{"explicit_1h", transcript.Usage{CacheCreation: 10_000, Create1h: 10_000, CreateSplitPresent: true}, WriteBasisMeasured},
		{"mixed", transcript.Usage{CacheCreation: 10_000, Create5m: 6_000, Create1h: 4_000, CreateSplitPresent: true}, WriteBasisMeasured},
		{"explicit_zero_zero_with_writes", transcript.Usage{CacheCreation: 10_000, CreateSplitPresent: true}, WriteBasisInconsistent},
		{"no_writes_no_split", transcript.Usage{}, WriteBasisAssumedShort},
		{"no_writes_present_split", transcript.Usage{CreateSplitPresent: true}, WriteBasisMeasured},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := WriteTTLBasisOf(tc.u); got != tc.want {
				t.Fatalf("WriteTTLBasisOf = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestWriteAssumedFlag covers the boolean helper used by the analysis tally.
func TestWriteAssumedFlag(t *testing.T) {
	if !WriteBasisAssumed(transcript.Usage{CacheCreation: 10_000}) {
		t.Fatalf("absent breakdown must be assumed")
	}
	if !WriteBasisAssumed(transcript.Usage{CacheCreation: 10_000, CreateSplitPresent: true}) {
		t.Fatalf("explicit 0/0 with writes must be treated as assumed (not measured)")
	}
	if WriteBasisAssumed(transcript.Usage{CacheCreation: 10_000, Create5m: 10_000, CreateSplitPresent: true}) {
		t.Fatalf("measured 5m must not be assumed")
	}
	if WriteBasisAssumed(transcript.Usage{CacheCreation: 10_000, Create1h: 10_000, CreateSplitPresent: true}) {
		t.Fatalf("measured 1h must not be assumed")
	}
}

// TestMoneyArithmeticUnchanged pins that the fallback arithmetic itself did not
// change: absent breakdown still prices the aggregate at short; long still
// prices at long; the 60% gap remains.
func TestMoneyArithmeticUnchanged(t *testing.T) {
	p := Price{InputPerMTok: 3.0, OutputPerMTok: 15.0, ReadMult: 0.1}
	const writes = 10_000

	short := transcript.Usage{CacheCreation: writes, Create5m: writes, CreateSplitPresent: true}
	long := transcript.Usage{CacheCreation: writes, Create1h: writes, CreateSplitPresent: true}
	absent := transcript.Usage{CacheCreation: writes}

	in := 3.0 / 1_000_000.0
	wantShort := float64(writes) * 1.25 * in
	wantLong := float64(writes) * 2.0 * in

	if got := CostUSD(short, p); math.Abs(got-wantShort) > ttlProvTol {
		t.Fatalf("5m priced %.9f, want %.9f", got, wantShort)
	}
	if got := CostUSD(long, p); math.Abs(got-wantLong) > ttlProvTol {
		t.Fatalf("1h priced %.9f, want %.9f", got, wantLong)
	}
	if got := CostUSD(absent, p); math.Abs(got-wantShort) > ttlProvTol {
		t.Fatalf("absent breakdown priced %.9f, want short default %.9f", got, wantShort)
	}
	if wantLong/wantShort < 1.59 || wantLong/wantShort > 1.61 {
		t.Fatalf("expected ~60%% long/short gap, got %.2f", wantLong/wantShort)
	}
}
