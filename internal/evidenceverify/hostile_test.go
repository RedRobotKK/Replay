package evidenceverify

import (
	"math"
	"testing"
)

// Hostile-review RED tests. These encode suspected defects in Candidate A and
// are expected to FAIL (or panic) before the repair, then PASS after.

// A fragment shorter than the value token must not bind the value. Candidate A
// pins the occurrence by value-start offset only and never checks that the
// fragment length equals the value token's span, so a 1-byte fragment over
// "true" confirms as if it had bound all four bytes.
func TestHostileFragmentSpanBinding(t *testing.T) {
	doc := `{"is_error":true}`
	loc := mkLoc(doc, 12, 1) // fragment = "t", value token = "true" (4 bytes)
	r := Verify([]byte(doc), loc, Predicate{Key: "is_error", Expect: BoolLiteral(true)})
	if r.Verdict == Confirmed {
		t.Fatalf("fragment shorter than the value token must not CONFIRM (verdict=%s reason=%s)", r.Verdict, r.Reason)
	}
	wantVerdict(t, r, Unverified)
	wantReason(t, r, ReasonFragmentSpanMismatch)
}

// Offset+length must not overflow int64 and slip past the bounds check into a
// slice panic. Candidate A checks `ValueOffset+FragmentLength > len` which
// wraps for hostile values and then slices with the wrapped bound.
func TestHostileLocatorOverflow(t *testing.T) {
	doc := `{"is_error":true}`
	loc := mkLoc(doc, 12, 4)
	loc.ValueOffset = math.MaxInt64 - 3
	loc.FragmentLength = 10

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Verify panicked on overflowing locator arithmetic: %v", r)
		}
	}()
	res := Verify([]byte(doc), loc, Predicate{Key: "is_error", Expect: BoolLiteral(true)})
	wantVerdict(t, res, Unverified)
}
