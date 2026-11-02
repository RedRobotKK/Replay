package evidenceverify

// Guard-reachability contract tests. Every test here makes one introduced
// conditional true and asserts the contract-level outcome (Verdict + Reason)
// that the branch exists to protect. The point is not line coverage: if the
// guarded branch were neutralised, the asserted Verdict or Reason would change
// and the test would fail. Each test names the guard it pins so the intent is
// auditable without decoding the source.

import (
	"crypto/sha256"
	"strings"
	"testing"
)

// --- limits normalisation (verify.go:200,203,206,209) ---

func TestNormalizedLimitsDefaults(t *testing.T) {
	want := DefaultLimits()

	// All-zero limits mean "use the production defaults" for every field.
	got := (Limits{}).normalized()
	if got != want {
		t.Fatalf("zero limits normalized to %+v, want defaults %+v", got, want)
	}

	// A single set field survives; the rest fall back to defaults.
	part := Limits{MaxDepth: 5}.normalized()
	if part.MaxDepth != 5 {
		t.Fatalf("MaxDepth = %d, want 5 (explicit value must survive)", part.MaxDepth)
	}
	if part.MaxTranscriptBytes != want.MaxTranscriptBytes ||
		part.MaxMembers != want.MaxMembers ||
		part.MaxMatches != want.MaxMatches {
		t.Fatalf("partial limits did not default the unset fields: %+v", part)
	}
}

// --- predicate validation (verify.go:227) ---

func TestPredicateValidation(t *testing.T) {
	doc := `{"is_error":true}`
	off := strings.Index(doc, "true")

	// Empty key must be refused before any evidence is read.
	r := Verify([]byte(doc), mkLoc(doc, off, len("true")), Predicate{Key: "", Expect: BoolLiteral(true)})
	wantVerdict(t, r, Unverified)
	wantReason(t, r, ReasonUnsupportedPredicate)

	// An invalid literal kind must be refused too.
	r = Verify([]byte(doc), mkLoc(doc, off, len("true")), Predicate{Key: "is_error", Expect: Literal{Kind: KindInvalid}})
	wantVerdict(t, r, Unverified)
	wantReason(t, r, ReasonUnsupportedPredicate)
}

// --- mixed-kind duplicate keys are ambiguous (verify.go:367) ---

func TestMixedKindDuplicateIsAmbiguous(t *testing.T) {
	// true and "true" are different recorded literals of different kinds. They
	// cannot establish a unique proposition, so the scope must be Ambiguous,
	// never Confirmed or Refuted.
	doc := `{"is_error":true,"is_error":"true"}`
	r := pinToken(doc, "true", "is_error", BoolLiteral(true))
	wantVerdict(t, r, Ambiguous)
	wantReason(t, r, ReasonDuplicateConflict)
}

// --- scanWhole on empty input (verify.go:479) ---

func TestScanWholeRejectsEmptyInput(t *testing.T) {
	_, err := scanWhole([]byte{}, -1, DefaultLimits())
	if err == nil {
		t.Fatalf("scanWhole accepted empty input")
	}
	r := scanFailure(err)
	wantVerdict(t, r, Unverified)
	wantReason(t, r, ReasonMalformedJSON)
}

// --- scanScope structural guards (verify.go:500,505,510,514,519,525,529) ---
// scanScope is called by Verify only after the whole document already parsed,
// so its malformed branches are unreachable through Verify. They are exercised
// directly here, and each asserts the exact fail-closed outcome.

func TestScanScopeStructural(t *testing.T) {
	lim := DefaultLimits()

	t.Run("not_an_object", func(t *testing.T) {
		// verify.go:500 — the scope slice must begin with '{'.
		_, _, err := scanScope([]byte(`[]`), 0, 2, "k", lim)
		assertReason(t, err, ReasonMalformedJSON)
	})

	t.Run("empty_object", func(t *testing.T) {
		// verify.go:505 — an empty object is a valid scope with no members.
		vals, count, err := scanScope([]byte(`{}`), 0, 2, "k", lim)
		if err != nil {
			t.Fatalf("empty object scope errored: %v", err)
		}
		if len(vals) != 0 || count != 0 {
			t.Fatalf("empty scope returned %d vals / %d count", len(vals), count)
		}
	})

	t.Run("member_key_not_string", func(t *testing.T) {
		// verify.go:510 — a member key must be a quoted string.
		_, _, err := scanScope([]byte(`{1:2}`), 0, 5, "k", lim)
		assertReason(t, err, ReasonMalformedJSON)
	})

	t.Run("unterminated_key", func(t *testing.T) {
		// verify.go:514 — scanString failure inside the scope.
		_, _, err := scanScope([]byte(`{"k`), 0, 3, "k", lim)
		assertReason(t, err, ReasonMalformedJSON)
	})

	t.Run("missing_colon", func(t *testing.T) {
		// verify.go:519 — ':' must follow the key.
		_, _, err := scanScope([]byte(`{"k" 1}`), 0, 7, "k", lim)
		assertReason(t, err, ReasonMalformedJSON)
	})

	t.Run("missing_value", func(t *testing.T) {
		// verify.go:525 — scanValue failure inside the scope.
		_, _, err := scanScope([]byte(`{"k":}`), 0, 6, "k", lim)
		assertReason(t, err, ReasonMalformedJSON)
	})

	t.Run("member_limit", func(t *testing.T) {
		// verify.go:529 — members exceeding the configured limit.
		small := lim
		small.MaxMembers = 2
		doc := []byte(`{"a":1,"b":2,"c":3}`)
		_, _, err := scanScope(doc, 0, len(doc), "k", small)
		assertReason(t, err, ReasonResourceLimit)
	})
}

func assertReason(t *testing.T, err error, want Reason) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with reason %s, got nil", want)
	}
	r := scanFailure(err)
	if r.Reason != want {
		t.Fatalf("reason = %s, want %s (detail=%q)", r.Reason, want, r.Detail)
	}
	if r.Verdict != Unverified {
		t.Fatalf("scan failure must be Unverified, got %s", r.Verdict)
	}
}

// --- scanValue at end of input (verify.go:553) ---

func TestScanValueUnexpectedEOF(t *testing.T) {
	s := &scanner{b: []byte{}, lim: DefaultLimits()}
	_, _, err := s.scanValue()
	assertReason(t, err, ReasonMalformedJSON)
}

// --- null literal parses and can confirm (verify.go:577) ---

func TestNullLiteral(t *testing.T) {
	doc := `{"k":null}`
	r := pinToken(doc, "null", "k", NullLiteral())
	wantVerdict(t, r, Confirmed)
}

// --- malformed boolean/null literals (verify.go:573,578) ---

func TestMalformedKeywordLiterals(t *testing.T) {
	for _, tc := range []struct{ name, doc string }{
		{"malformed_false", `{"k":fals}`},
		{"malformed_null", `{"k":nul}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Pin the malformed token itself so the parse must fail on it,
			// not merely fail somewhere downstream.
			tok := strings.TrimSuffix(strings.TrimPrefix(tc.doc, `{"k":`), "}")
			off := strings.Index(tc.doc, tok)
			r := Verify([]byte(tc.doc), mkLoc(tc.doc, off, len(tok)), Predicate{Key: "k", Expect: NullLiteral()})
			wantVerdict(t, r, Unverified)
			wantReason(t, r, ReasonMalformedJSON)
		})
	}
}

// --- object nesting depth (verify.go:608) ---

func TestObjectNestingDepth(t *testing.T) {
	lim := DefaultLimits()
	lim.MaxDepth = 3
	deep := `{"a":{"b":{"c":{"d":1}}}}`
	r := VerifyWithLimits([]byte(deep), mkLoc(deep, 0, 1), Predicate{Key: "d", Expect: NumberLiteral("1")}, lim)
	wantVerdict(t, r, Unverified)
	wantReason(t, r, ReasonResourceLimit)
}

// --- object member limit (verify.go:665) ---

func TestObjectMemberLimit(t *testing.T) {
	lim := DefaultLimits()
	lim.MaxMembers = 2
	doc := `{"a":1,"b":2,"c":3}`
	r := VerifyWithLimits([]byte(doc), mkLoc(doc, 0, 1), Predicate{Key: "a", Expect: NumberLiteral("1")}, lim)
	wantVerdict(t, r, Unverified)
	wantReason(t, r, ReasonResourceLimit)
}

// --- scanObject malformed key and colon, with a correct locator so the
// --- branch is behaviourally load-bearing, not merely executed (645,654) ---

func TestScanObjectMalformedMember(t *testing.T) {
	t.Run("non_string_key", func(t *testing.T) {
		doc := `{"a":1,1:2}`
		r := Verify([]byte(doc), mkLoc(doc, strings.Index(doc, "1"), len("1")), Predicate{Key: "a", Expect: NumberLiteral("1")})
		wantVerdict(t, r, Unverified)
		wantReason(t, r, ReasonMalformedJSON)
	})
	t.Run("missing_colon", func(t *testing.T) {
		// Pins the value "true" with a correct locator. With the colon guard
		// intact the parse fails; without it the value would resolve, so this
		// assertion is what makes the guard load-bearing.
		doc := `{"is_error" true}`
		off := strings.Index(doc, "true")
		r := Verify([]byte(doc), mkLoc(doc, off, len("true")), Predicate{Key: "is_error", Expect: BoolLiteral(true)})
		wantVerdict(t, r, Unverified)
		wantReason(t, r, ReasonMalformedJSON)
	})
}

// --- unterminated array (verify.go:703) and array member limit (710) ---

func TestScanArrayStructural(t *testing.T) {
	t.Run("unterminated", func(t *testing.T) {
		doc := `[1,2`
		r := Verify([]byte(doc), mkLoc(doc, 0, 1), Predicate{Key: "k", Expect: NumberLiteral("1")})
		wantVerdict(t, r, Unverified)
		wantReason(t, r, ReasonMalformedJSON)
	})
	t.Run("element_limit", func(t *testing.T) {
		lim := DefaultLimits()
		lim.MaxMembers = 2
		doc := `[1,2,3]`
		r := VerifyWithLimits([]byte(doc), mkLoc(doc, 0, 1), Predicate{Key: "k", Expect: NumberLiteral("1")}, lim)
		wantVerdict(t, r, Unverified)
		wantReason(t, r, ReasonResourceLimit)
	})
}

// --- unicode escape structural guards (verify.go:779,817,829) ---

func TestUnicodeEscapeStructural(t *testing.T) {
	t.Run("truncated_escape", func(t *testing.T) {
		// readHex4 i+4 > len — the string ends inside the \u escape.
		doc := []byte(`{"\u00`)
		r := Verify(doc, mkLocBytes(doc, 0, 1), Predicate{Key: "k", Expect: BoolLiteral(true)})
		wantVerdict(t, r, Unverified)
		wantReason(t, r, ReasonMalformedJSON)
	})
	t.Run("truncated_surrogate_pair", func(t *testing.T) {
		// A high surrogate whose low surrogate is a truncated \u escape.
		doc := []byte(`{"\uD800\u00`)
		r := Verify(doc, mkLocBytes(doc, 0, 1), Predicate{Key: "k", Expect: BoolLiteral(true)})
		wantVerdict(t, r, Unverified)
		wantReason(t, r, ReasonMalformedJSON)
	})
	t.Run("uppercase_hex", func(t *testing.T) {
		// 'A'-'F' are valid hex digits (verify.go:829); the escape decodes.
		doc := `{"\u0041":true}`
		r := Verify([]byte(doc), mkLoc(doc, strings.Index(doc, "true"), len("true")), Predicate{Key: "A", Expect: BoolLiteral(true)})
		wantVerdict(t, r, Confirmed)
	})
}

// --- number grammar: bare '-' (849), truncated number (853), leading zero (859) ---

func TestNumberGrammarStructural(t *testing.T) {
	for _, tc := range []struct{ name, doc string }{
		{"bare_minus", `{"n":-}`},
		{"truncated_number", `{"n":}`},
		{"leading_zero", `{"n":01}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Verify([]byte(tc.doc), mkLoc(tc.doc, 0, 1), Predicate{Key: "n", Expect: NumberLiteral("1")})
			wantVerdict(t, r, Unverified)
			wantReason(t, r, ReasonMalformedJSON)
		})
	}
}

// --- trailing bytes after the root value (verify.go:486) ---

func TestTrailingBytesRefused(t *testing.T) {
	doc := `{"is_error":true}x`
	off := strings.Index(doc, "true")
	r := Verify([]byte(doc), mkLoc(doc, off, len("true")), Predicate{Key: "is_error", Expect: BoolLiteral(true)})
	wantVerdict(t, r, Unverified)
	wantReason(t, r, ReasonMalformedJSON)
}

// --- scanValue string scan error: invalid UTF-8 in a value (verify.go:563) ---
// This must surface ReasonInvalidUTF8, not a generic malformed reason, so that
// evidence is rejected rather than repaired.

func TestScanValueInvalidUTF8(t *testing.T) {
	doc := []byte{'{', '"', 'k', '"', ':', '"', 0xFF, '"', '}'}
	r := Verify(doc, mkLocBytes(doc, 6, 1), Predicate{Key: "k", Expect: StringLiteral("x")})
	wantVerdict(t, r, Unverified)
	wantReason(t, r, ReasonInvalidUTF8)
}

// --- hasConflict with fewer than two values (verify.go:393) ---

func TestHasConflictFewerThanTwo(t *testing.T) {
	if hasConflict(nil) {
		t.Fatalf("zero values must not be a conflict")
	}
	if hasConflict([]value{{kind: vBool, boolVal: true}}) {
		t.Fatalf("one value must not be a conflict")
	}
	if !hasConflict([]value{{kind: vBool, boolVal: true}, {kind: vBool, boolVal: false}}) {
		t.Fatalf("two differing values must be a conflict")
	}
}

// --- empty object (637) and empty array (697) parse as values ---

func TestEmptyCompositeValues(t *testing.T) {
	t.Run("empty_object_value", func(t *testing.T) {
		doc := `{"k":{}}`
		r := pinToken(doc, `{}`, "k", StringLiteral("x"))
		// A non-scalar pinned value can only be REFUTED (it contradicts any
		// scalar claim) once the basis holds; never Confirmed.
		wantVerdict(t, r, Refuted)
	})
	t.Run("empty_array_value", func(t *testing.T) {
		doc := `{"k":[]}`
		r := pinToken(doc, `[]`, "k", StringLiteral("x"))
		wantVerdict(t, r, Refuted)
	})
}

// --- LocateValue bounds (locate.go:15,19) ---

func TestLocateValueBounds(t *testing.T) {
	doc := `{"is_error":true}`

	if _, ok := LocateValue([]byte(doc), -1); ok {
		t.Fatalf("negative offset must not locate")
	}
	if _, ok := LocateValue([]byte(doc), int64(len(doc))); ok {
		t.Fatalf("offset past the end must not locate")
	}
	// An offset pointing at a non-value token (the '{') must not locate.
	if _, ok := LocateValue([]byte(doc), 0); ok {
		t.Fatalf("offset at '{' must not locate a value")
	}
	// A real value offset locates.
	if _, ok := LocateValue([]byte(doc), int64(strings.Index(doc, "true"))); !ok {
		t.Fatalf("offset at 'true' should locate")
	}
}

// Ensure the sha256 import is retained for the locator helpers used above.
var _ = sha256.Sum256
