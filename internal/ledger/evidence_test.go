package ledger

import (
	"testing"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// Turning ledger records into testimony about a model's caching floor.
//
// Only one direction is sound from what the ledger records today, and the
// asymmetry is worth stating rather than papering over.
//
// An upper bound is exact. `cache_creation_input_tokens` IS the number of
// tokens the provider cached, so a request reporting 512 proves that model
// caches a 512-token prefix, and any documented floor above 512 is refuted by
// that single record.
//
// A lower bound is not derivable. It would need the prompt size AT THE
// BREAKPOINT for a marked request that cached nothing — and the proxy records
// only how many `cache_control` markers a request carried, never where they
// sat. Using the whole prompt instead would claim the floor is above a number
// far larger than the marked prefix, which overstates it in the dangerous
// direction: it would refute correct documented figures.
//
//   L1  a cache write yields an exact upper bound
//   L2  no lower bound is invented from data that cannot support one
//   L3  a request that cached nothing contributes nothing
//   L4  session and machine breadth is carried through
//   L5  records from other providers and failed requests are ignored

func rec(model string, created, read int, markers int, session string) Record {
	r := Record{SessionID: session, Status: 200}
	r.Model = model
	r.Prompt.CacheControlCount = markers
	r.Response.Usage = &transcript.Usage{CacheCreation: created, CacheRead: read, Input: 10}
	return r
}

// L1: PASS: upperBound equals the smallest cache_creation seen.
// FAIL: any other number — this is the one figure the ledger states exactly,
// and rounding or re-deriving it throws away the only precision available.
func TestL1_CacheWriteGivesAnExactUpperBound(t *testing.T) {
	// Cold writes only. A turn that also READ from cache wrote an increment
	// on top of an existing entry, and that increment is not a prefix size —
	// reading it as one invents a floor far below the truth. Both of these
	// wrote with no prior entry, so what was written is what was cached.
	ev := EvidenceFrom([]Record{
		rec("claude-opus-5", 4096, 0, 1, "s1"),
		rec("claude-opus-5", 512, 0, 1, "s1"),
		// A warm write of 118 tokens on a 20,000-token cached prefix. If this
		// counted, the floor would be reported as 118 and every documented
		// figure above it "refuted". It is the exact shape that produced a
		// false contradiction against the real ledger on 2026-09-05.
		rec("claude-opus-5", 118, 20000, 1, "s2"),
	}, "machine-a")

	claims := cachemodel.MeasureClaims(ev)
	o := claims["claude-opus-5"].Observed
	if o == nil || o.UpperBound == nil {
		t.Fatal("a cache write must produce an upper bound")
	}
	if *o.UpperBound != 512 {
		t.Errorf("upperBound = %d, want 512: the smallest prefix the provider actually cached", *o.UpperBound)
	}
}

// L2: PASS: no lower bound, ever, from this source.
// FAIL: inventing one. The prompt total is not the marked prefix, and
// asserting a floor above it would refute documented figures that are correct.
func TestL2_NoLowerBoundIsInvented(t *testing.T) {
	ev := EvidenceFrom([]Record{
		// A large marked request that cached nothing. Tempting, and unusable:
		// where the marker sat is not recorded.
		rec("claude-opus-5", 0, 0, 1, "s1"),
		rec("claude-opus-5", 512, 0, 1, "s2"),
	}, "machine-a")

	for _, e := range ev {
		if !e.Wrote && e.Marked {
			t.Errorf("a non-writing record became lower-bound evidence: %+v", e)
		}
	}
	o := cachemodel.MeasureClaims(ev)["claude-opus-5"].Observed
	if o != nil && o.LowerBound != nil {
		t.Errorf("lowerBound = %d; the ledger cannot support one until marker positions are recorded", *o.LowerBound)
	}
}

// L3: PASS: records with no cache write contribute nothing at all.
// FAIL: a claim built from records that witnessed nothing.
func TestL3_NoWriteContributesNothing(t *testing.T) {
	ev := EvidenceFrom([]Record{
		rec("claude-opus-5", 0, 0, 1, "s1"),
		rec("claude-opus-5", 0, 9999, 1, "s2"),
	}, "machine-a")
	if len(cachemodel.MeasureClaims(ev)) != 0 {
		t.Error("records that cached nothing must not produce a claim")
	}
}

// L4: PASS: distinct sessions and the machine identifier are carried.
// FAIL: dropping them, which makes one chatty session indistinguishable from
// broad evidence — and breadth is the whole weight of an agreement verdict.
func TestL4_BreadthIsCarried(t *testing.T) {
	ev := EvidenceFrom([]Record{
		rec("claude-opus-5", 1024, 0, 1, "s1"),
		rec("claude-opus-5", 1024, 0, 1, "s1"),
		rec("claude-opus-5", 2048, 0, 1, "s2"),
	}, "machine-a")
	o := cachemodel.MeasureClaims(ev)["claude-opus-5"].Observed
	if o.Sessions != 2 {
		t.Errorf("sessions = %d, want 2 distinct", o.Sessions)
	}
	if o.Machines != 1 {
		t.Errorf("machines = %d, want 1", o.Machines)
	}
}

// L5: PASS: records with no model, no usage, or a non-200 status are skipped.
// FAIL: measuring from a refused or errored request, whose usage describes
// something other than a served response.
func TestL5_UnusableRecordsAreSkipped(t *testing.T) {
	noModel := rec("", 512, 0, 1, "s1")
	failed := rec("claude-opus-5", 512, 0, 1, "s1")
	failed.Status = 429
	refused := rec("claude-opus-5", 512, 0, 1, "s1")
	refused.Refusal = "max-session-usd"
	noUsage := rec("claude-opus-5", 0, 0, 1, "s1")
	noUsage.Response.Usage = nil

	ev := EvidenceFrom([]Record{noModel, failed, refused, noUsage}, "machine-a")
	if len(ev) != 0 {
		t.Errorf("kept %d unusable records as evidence: %+v", len(ev), ev)
	}
}

// L6: a warm write is never floor evidence.
//
// The defect this guards against produced a real false result. Reading
// `cache_creation_input_tokens` as a prefix size reported opus-5 caching a
// 118-token prefix from the maintainer's own ledger, contradicting a
// documented floor of 512 — a fabricated refutation that would have been the
// paid feed's headline finding on the day it shipped.
//
// PASS: a record that read from cache contributes nothing, whatever it wrote.
// FAIL: any evidence derived from an increment, because the increment is not
// the prefix and reading it as one always errs downward — the direction that
// manufactures contradictions.
func TestL6_AWarmWriteIsNotFloorEvidence(t *testing.T) {
	warm := []Record{
		rec("claude-opus-5", 118, 20000, 1, "s1"),
		rec("claude-opus-5", 4, 180000, 1, "s2"),
		rec("claude-opus-5", 1, 1, 1, "s3"),
	}
	if ev := EvidenceFrom(warm, "m1"); len(ev) != 0 {
		t.Errorf("kept %d warm writes as floor evidence: %+v", len(ev), ev)
	}
	if len(cachemodel.MeasureClaims(EvidenceFrom(warm, "m1"))) != 0 {
		t.Error("warm writes must not produce a claim, and must never produce a contradiction")
	}

	// A cold write in the same set is still evidence, and is the only one.
	mixed := append(warm, rec("claude-opus-5", 2048, 0, 1, "s4"))
	o := cachemodel.MeasureClaims(EvidenceFrom(mixed, "m1"))["claude-opus-5"].Observed
	if o == nil || o.UpperBound == nil || *o.UpperBound != 2048 {
		t.Fatalf("the cold write must set the bound; got %+v", o)
	}
	if o.Sessions != 1 {
		t.Errorf("sessions = %d, want 1: only the cold write is evidence", o.Sessions)
	}
}
