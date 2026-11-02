package evidenceverify

import (
	"crypto/sha256"
	"encoding/json"
	"strings"
	"testing"
)

// mkLoc builds a Locator that pins transcript[off:off+fragLen].
func mkLoc(transcript string, off, fragLen int) Locator {
	return mkLocBytes([]byte(transcript), off, fragLen)
}

// mkLocBytes builds a Locator that pins b[off:off+fragLen].
func mkLocBytes(b []byte, off, fragLen int) Locator {
	return Locator{
		TranscriptSHA256: sha256.Sum256(b),
		ValueOffset:      int64(off),
		FragmentLength:   int64(fragLen),
		FragmentSHA256:   sha256.Sum256(b[off : off+fragLen]),
	}
}

// pinToken verifies a claim on the first occurrence of token in transcript.
func pinToken(transcript, token, key string, expect Literal) Result {
	off := strings.Index(transcript, token)
	if off < 0 {
		panic("token not found in fixture: " + token)
	}
	return Verify([]byte(transcript), mkLoc(transcript, off, len(token)), Predicate{Key: key, Expect: expect})
}

func wantVerdict(t *testing.T, r Result, v Verdict) {
	t.Helper()
	if r.Verdict != v {
		t.Fatalf("verdict = %s (reason %s), want %s; detail=%q", r.Verdict, r.Reason, v, r.Detail)
	}
}

func wantReason(t *testing.T, r Result, reason Reason) {
	t.Helper()
	if r.Reason != reason {
		t.Fatalf("reason = %s, want %s; verdict=%s detail=%q", r.Reason, reason, r.Verdict, r.Detail)
	}
}

// ---------------------------------------------------------------------------
// A. Four-state truth table
// ---------------------------------------------------------------------------

func TestFourStates(t *testing.T) {
	trueP := BoolLiteral(true)

	t.Run("confirmed", func(t *testing.T) {
		r := pinToken(`{"is_error":true}`, "true", "is_error", trueP)
		wantVerdict(t, r, Confirmed)
		wantReason(t, r, ReasonLiteralMatch)
	})
	t.Run("refuted", func(t *testing.T) {
		r := pinToken(`{"is_error":false}`, "false", "is_error", trueP)
		wantVerdict(t, r, Refuted)
		wantReason(t, r, ReasonLiteralMismatch)
	})
	t.Run("ambiguous_conflicting", func(t *testing.T) {
		r := pinToken(`{"is_error":true,"is_error":false}`, "true", "is_error", trueP)
		wantVerdict(t, r, Ambiguous)
		wantReason(t, r, ReasonDuplicateConflict)
	})
	t.Run("ambiguous_conflicting_reversed", func(t *testing.T) {
		// Order must not matter.
		r := pinToken(`{"is_error":false,"is_error":true}`, "true", "is_error", trueP)
		wantVerdict(t, r, Ambiguous)
		wantReason(t, r, ReasonDuplicateConflict)
	})
	t.Run("ambiguous_second_occurrence", func(t *testing.T) {
		// Pinning the second conflicting value is ambiguous too.
		doc := `{"is_error":true,"is_error":false}`
		off := strings.LastIndex(doc, "false")
		r := Verify([]byte(doc), mkLoc(doc, off, len("false")), Predicate{Key: "is_error", Expect: trueP})
		wantVerdict(t, r, Ambiguous)
	})
	t.Run("unverified_hash_mismatch", func(t *testing.T) {
		doc := `{"is_error":true}`
		loc := mkLoc(doc, strings.Index(doc, "true"), len("true"))
		loc.TranscriptSHA256[0] ^= 0xFF
		r := Verify([]byte(doc), loc, Predicate{Key: "is_error", Expect: trueP})
		wantVerdict(t, r, Unverified)
		wantReason(t, r, ReasonTranscriptHashMismatch)
	})
	t.Run("unverified_fragment_mismatch", func(t *testing.T) {
		doc := `{"is_error":true}`
		loc := mkLoc(doc, strings.Index(doc, "true"), len("true"))
		loc.FragmentSHA256[0] ^= 0xFF
		r := Verify([]byte(doc), loc, Predicate{Key: "is_error", Expect: trueP})
		wantVerdict(t, r, Unverified)
		wantReason(t, r, ReasonFragmentHashMismatch)
	})
}

// Identical duplicates are not a conflict.
func TestIdenticalDuplicatesAreDetermined(t *testing.T) {
	trueP := BoolLiteral(true)

	r := pinToken(`{"is_error":true,"is_error":true}`, "true", "is_error", trueP)
	wantVerdict(t, r, Confirmed)

	r = pinToken(`{"is_error":false,"is_error":false}`, "false", "is_error", trueP)
	wantVerdict(t, r, Refuted)
}

// ---------------------------------------------------------------------------
// B. Scope isolation
// ---------------------------------------------------------------------------

func TestScopeIsolation(t *testing.T) {
	trueP := BoolLiteral(true)

	t.Run("nested_objects_do_not_conflict", func(t *testing.T) {
		doc := `{"a":{"is_error":true},"b":{"is_error":false}}`
		r := pinToken(doc, "true", "is_error", trueP)
		wantVerdict(t, r, Confirmed)
	})
	t.Run("nested_false_is_refuted", func(t *testing.T) {
		doc := `{"a":{"is_error":false},"b":{"is_error":true}}`
		r := pinToken(doc, "false", "is_error", trueP)
		wantVerdict(t, r, Refuted)
	})
	t.Run("array_elements_do_not_conflict", func(t *testing.T) {
		doc := `[{"is_error":true},{"is_error":false}]`
		r := pinToken(doc, "true", "is_error", trueP)
		wantVerdict(t, r, Confirmed)
	})
	t.Run("outer_key_does_not_contaminate_inner", func(t *testing.T) {
		doc := `{"is_error":false,"inner":{"is_error":true}}`
		r := pinToken(doc, "true", "is_error", trueP)
		wantVerdict(t, r, Confirmed)
	})
}

// ---------------------------------------------------------------------------
// C. Escaped duplicate keys
// ---------------------------------------------------------------------------

func TestEscapedDuplicateKeys(t *testing.T) {
	trueP := BoolLiteral(true)

	// "\u0069s_error" decodes to "is_error".
	doc := `{"is_error":true,"\u0069s_error":false}`
	r := pinToken(doc, "true", "is_error", trueP)
	wantVerdict(t, r, Ambiguous)
	wantReason(t, r, ReasonDuplicateConflict)

	// Escaped-equivalent string literals: "x" and "\u0078" are the same.
	doc = `{"k":"x"}`
	r = pinToken(doc, `"x"`, "k", StringLiteral("x"))
	wantVerdict(t, r, Confirmed)

	doc = `{"k":"\u0078"}`
	r = pinToken(doc, `"\u0078"`, "k", StringLiteral("x"))
	wantVerdict(t, r, Confirmed)
}

// ---------------------------------------------------------------------------
// D. Locator attacks
// ---------------------------------------------------------------------------

func TestLocatorAttacks(t *testing.T) {
	trueP := BoolLiteral(true)
	doc := `{"is_error":true}`
	// Byte layout (no whitespace):
	//   0 '{'  1..10 "is_error"  11 ':'  12..15 "true"  16 '}'
	const valueOffset = 12

	for off := 0; off <= len(doc)+1; off++ {
		if off == valueOffset {
			continue // tested below as the confirmed case
		}
		// A 1-byte fragment is enough to exercise offset resolution; length
		// bounds and fragment hashing are tested separately.
		if off+1 > len(doc) {
			// Offsets at and past EOF: out of range.
			loc := Locator{
				TranscriptSHA256: sha256.Sum256([]byte(doc)),
				ValueOffset:      int64(off),
				FragmentLength:   1,
			}
			r := Verify([]byte(doc), loc, Predicate{Key: "is_error", Expect: trueP})
			wantVerdict(t, r, Unverified)
			wantReason(t, r, ReasonLocatorOutOfRange)
			continue
		}
		r := Verify([]byte(doc), mkLoc(doc, off, 1), Predicate{Key: "is_error", Expect: trueP})
		if r.Verdict != Unverified {
			t.Fatalf("offset %d: verdict = %s, want UNVERIFIED (no value starts there)", off, r.Verdict)
		}
		if r.Reason != ReasonLocatorNotValue {
			t.Fatalf("offset %d: reason = %s, want LOCATOR_NOT_VALUE", off, r.Reason)
		}
	}

	// The exact value boundary confirms.
	r := Verify([]byte(doc), mkLoc(doc, valueOffset, 4), Predicate{Key: "is_error", Expect: trueP})
	wantVerdict(t, r, Confirmed)

	// A value token at the right offset but the wrong key is UNVERIFIED, not
	// REFUTED: the claimed occurrence was never established.
	doc2 := `{"type":"tool_result","is_error":true}`
	r = pinToken(doc2, `"tool_result"`, "is_error", trueP)
	wantVerdict(t, r, Unverified)
	wantReason(t, r, ReasonLocatorKeyMismatch)
}

func TestLocatorBoundaries(t *testing.T) {
	trueP := BoolLiteral(true)
	doc := `{"is_error":true}`

	t.Run("negative_offset_malformed", func(t *testing.T) {
		loc := mkLoc(doc, 0, 1)
		loc.ValueOffset = -1
		r := Verify([]byte(doc), loc, Predicate{Key: "is_error", Expect: trueP})
		wantReason(t, r, ReasonLocatorMalformed)
	})
	t.Run("zero_fragment_malformed", func(t *testing.T) {
		loc := mkLoc(doc, 0, 1)
		loc.FragmentLength = 0
		r := Verify([]byte(doc), loc, Predicate{Key: "is_error", Expect: trueP})
		wantReason(t, r, ReasonLocatorMalformed)
	})
	t.Run("fragment_extends_past_eof", func(t *testing.T) {
		loc := Locator{
			TranscriptSHA256: sha256.Sum256([]byte(doc)),
			ValueOffset:      int64(len(doc) - 1),
			FragmentLength:   5,
		}
		r := Verify([]byte(doc), loc, Predicate{Key: "is_error", Expect: trueP})
		wantReason(t, r, ReasonLocatorOutOfRange)
	})
}

// ---------------------------------------------------------------------------
// E. JSON string attacks
// ---------------------------------------------------------------------------

func TestStringGrammar(t *testing.T) {
	trueP := BoolLiteral(true)

	validKeys := []string{
		`"is_error"`,
		`"is\u0065rror"`,
		`"quote\"key"`,
		`"back\\slash"`,
		`"slash\/key"`,
		`"emoji\ud83d\ude00"`,
	}
	for _, key := range validKeys {
		doc := `{` + key + `:true}`
		r := pinToken(doc, "true", strings.Trim(key, `"`), trueP)
		// These keys don't decode to "is_error" except the first; the point
		// is that they must PARSE, not that they confirm.
		if r.Verdict == Unverified && (r.Reason == ReasonMalformedJSON || r.Reason == ReasonInvalidUTF8) {
			t.Fatalf("valid key %s was rejected: %s", key, r.Reason)
		}
	}

	// The escape handling must decode to the right key for confirmation.
	// \u0069 is 'i', so this key decodes to "is_error".
	r := pinToken(`{"\u0069s_error":true}`, "true", "is_error", trueP)
	wantVerdict(t, r, Confirmed)
}

func TestStringAttacksRejected(t *testing.T) {
	trueP := BoolLiteral(true)

	cases := []struct {
		name string
		key  []byte
		want Reason
	}{
		{"lone_high_surrogate", []byte(`{"\uD800":true}`), ReasonMalformedJSON},
		{"lone_low_surrogate", []byte(`{"\uDC00":true}`), ReasonMalformedJSON},
		{"high_high_pair", []byte(`{"\uD800\uD800":true}`), ReasonMalformedJSON},
		{"truncated_unicode", []byte(`{"\u00":true}`), ReasonMalformedJSON},
		{"invalid_hex", []byte(`{"\uZZZZ":true}`), ReasonMalformedJSON},
		{"invalid_escape", []byte(`{"\q":true}`), ReasonMalformedJSON},
		{"truncated_escape", []byte(`{"\`), ReasonMalformedJSON},
		{"unterminated_string", []byte(`{"key`), ReasonMalformedJSON},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			loc := mkLocBytes(tc.key, 0, 1)
			r := Verify(tc.key, loc, Predicate{Key: "is_error", Expect: trueP})
			wantVerdict(t, r, Unverified)
			wantReason(t, r, tc.want)
		})
	}

	t.Run("raw_control_byte", func(t *testing.T) {
		doc := []byte{'{', '"', 'k', 0x0A, '"', ':', 't', 'r', 'u', 'e', '}'}
		r := Verify(doc, mkLocBytes(doc, 0, 1), Predicate{Key: "is_error", Expect: trueP})
		wantVerdict(t, r, Unverified)
		wantReason(t, r, ReasonMalformedJSON)
	})

	t.Run("invalid_utf8", func(t *testing.T) {
		doc := []byte{'{', '"', 'k', 0xFF, '"', ':', 't', 'r', 'u', 'e', '}'}
		r := Verify(doc, mkLocBytes(doc, 0, 1), Predicate{Key: "is_error", Expect: trueP})
		wantVerdict(t, r, Unverified)
		wantReason(t, r, ReasonInvalidUTF8)
	})

	t.Run("string_literal_equality", func(t *testing.T) {
		doc := `{"k":"\u0078"}`
		r := pinToken(doc, `"\u0078"`, "k", StringLiteral("x"))
		wantVerdict(t, r, Confirmed)
	})
}

// ---------------------------------------------------------------------------
// F. Number grammar and lexical equality
// ---------------------------------------------------------------------------

func TestNumberGrammar(t *testing.T) {
	valid := map[string]string{
		"0":     "0",
		"-0":    "-0",
		"1":     "1",
		"-1":    "-1",
		"1.5":   "1.5",
		"1e0":   "1e0",
		"1E0":   "1E0",
		"1e+2":  "1e+2",
		"1e-2":  "1e-2",
		"0.5":   "0.5",
		"12345": "12345",
	}
	for lex := range valid {
		doc := `{"n":` + lex + `}`
		r := pinToken(doc, lex, "n", NumberLiteral(lex))
		if r.Verdict != Confirmed {
			t.Fatalf("valid number %s: %s", lex, r.Verdict)
		}
	}

	invalid := []string{"01", "00", "-", "1.", ".1", "1e", "1e+", "1e-", "NaN", "Infinity", "+1"}
	for _, lex := range invalid {
		doc := `{"n":` + lex + `}`
		r := Verify([]byte(doc), mkLoc(doc, 0, 1), Predicate{Key: "n", Expect: NumberLiteral(lex)})
		if r.Verdict != Unverified || r.Reason != ReasonMalformedJSON {
			t.Fatalf("invalid number %q: verdict=%s reason=%s, want UNVERIFIED/MALFORMED_JSON", lex, r.Verdict, r.Reason)
		}
	}
}

func TestNumberLexicalEquality(t *testing.T) {
	// "1", "1.0" and "1e0" are three different recorded literals.
	doc := `{"n":1}`
	r := pinToken(doc, "1", "n", NumberLiteral("1.0"))
	wantVerdict(t, r, Refuted)

	r = pinToken(doc, "1", "n", NumberLiteral("1"))
	wantVerdict(t, r, Confirmed)

	doc = `{"n":1e0}`
	r = pinToken(doc, "1e0", "n", NumberLiteral("1"))
	wantVerdict(t, r, Refuted)
}

// ---------------------------------------------------------------------------
// G. Structural malformed JSON
// ---------------------------------------------------------------------------

func TestMalformedJSON(t *testing.T) {
	trueP := BoolLiteral(true)

	cases := []string{
		`{"is_error" true}`,       // missing colon
		`{"is_error":true "x":1}`, // missing comma
		`{"is_error":true,}`,      // trailing comma
		`{"is_error":true`,        // unterminated object
		`[1,2`,                    // unterminated array
		`{"is_error":true}}`,      // trailing bytes
		`{} {}`,                   // multiple roots
		`{"is_error":tru`,         // bad literal
		`{"is_error":truex}`,      // bad trailing token
	}
	for _, doc := range cases {
		r := Verify([]byte(doc), mkLoc(doc, 0, 1), Predicate{Key: "is_error", Expect: trueP})
		if r.Verdict != Unverified {
			t.Fatalf("malformed %q: verdict=%s reason=%s", doc, r.Verdict, r.Reason)
		}
		if r.Reason == ReasonTranscriptHashMismatch || r.Reason == ReasonFragmentHashMismatch || r.Reason == ReasonLocatorOutOfRange || r.Reason == ReasonLocatorMalformed {
			t.Fatalf("malformed %q: reason %s masked the structural failure", doc, r.Reason)
		}
	}
}

// ---------------------------------------------------------------------------
// H. Resource boundaries
// ---------------------------------------------------------------------------

func TestResourceLimits(t *testing.T) {
	trueP := BoolLiteral(true)

	t.Run("transcript_bytes", func(t *testing.T) {
		lim := DefaultLimits()
		lim.MaxTranscriptBytes = 16
		doc := strings.Repeat("x", 17) // invalid JSON but exceeds the size gate first
		loc := Locator{TranscriptSHA256: sha256.Sum256([]byte(doc)), ValueOffset: 0, FragmentLength: 1, FragmentSHA256: sha256.Sum256([]byte(doc[:1]))}
		r := VerifyWithLimits([]byte(doc), loc, Predicate{Key: "is_error", Expect: trueP}, lim)
		wantReason(t, r, ReasonResourceLimit)

		// Exactly at the limit is not a limit violation (it fails on JSON).
		doc = strings.Repeat("x", 16)
		loc = Locator{TranscriptSHA256: sha256.Sum256([]byte(doc)), ValueOffset: 0, FragmentLength: 1, FragmentSHA256: sha256.Sum256([]byte(doc[:1]))}
		r = VerifyWithLimits([]byte(doc), loc, Predicate{Key: "is_error", Expect: trueP}, lim)
		if r.Reason == ReasonResourceLimit {
			t.Fatalf("at-limit input hit the byte limit")
		}
	})

	t.Run("nesting_depth", func(t *testing.T) {
		lim := DefaultLimits()
		lim.MaxDepth = 8
		deep := strings.Repeat("[", 10) + strings.Repeat("]", 10)
		loc := Locator{TranscriptSHA256: sha256.Sum256([]byte(deep)), ValueOffset: 0, FragmentLength: 1, FragmentSHA256: sha256.Sum256([]byte(deep[:1]))}
		r := VerifyWithLimits([]byte(deep), loc, Predicate{Key: "is_error", Expect: trueP}, lim)
		wantReason(t, r, ReasonResourceLimit)

		shallow := strings.Repeat("[", 8) + strings.Repeat("]", 8)
		loc = Locator{TranscriptSHA256: sha256.Sum256([]byte(shallow)), ValueOffset: 0, FragmentLength: 1, FragmentSHA256: sha256.Sum256([]byte(shallow[:1]))}
		r = VerifyWithLimits([]byte(shallow), loc, Predicate{Key: "is_error", Expect: trueP}, lim)
		if r.Reason == ReasonResourceLimit {
			t.Fatalf("at-depth-limit input hit the depth limit")
		}
	})

	t.Run("matching_duplicates", func(t *testing.T) {
		lim := DefaultLimits()
		lim.MaxMatches = 2
		doc := `{"is_error":true,"is_error":true,"is_error":true}`
		r := VerifyWithLimits([]byte(doc), mkLoc(doc, strings.Index(doc, "true"), len("true")), Predicate{Key: "is_error", Expect: trueP}, lim)
		wantReason(t, r, ReasonResourceLimit)
	})
}

// ---------------------------------------------------------------------------
// I. Naive parser regression (the reason this subsystem exists)
// ---------------------------------------------------------------------------

func TestNaiveDecoderDestroysOccurrence(t *testing.T) {
	trueP := BoolLiteral(true)

	doc := `{"is_error":true,"is_error":false}`
	var m map[string]any
	if err := json.Unmarshal([]byte(doc), &m); err != nil {
		t.Fatalf("fixture must be valid JSON: %v", err)
	}
	// A standard decoder resolves the duplicate last-wins, destroying the true.
	if m["is_error"] != false {
		t.Fatalf("sanity: naive decoder did not collapse as expected: %v", m["is_error"])
	}

	// The verifier must NOT collapse it.
	r := pinToken(doc, "true", "is_error", trueP)
	wantVerdict(t, r, Ambiguous)

	r = pinToken(doc, "false", "is_error", trueP)
	wantVerdict(t, r, Ambiguous)
}

// ---------------------------------------------------------------------------
// Recorded evidence is not execution truth
// ---------------------------------------------------------------------------

func TestRecordedIsNotExecution(t *testing.T) {
	// CONFIRMED establishes only that the transcript records is_error: true.
	// It establishes nothing about whether the tool actually failed. The API
	// carries no execution-outcome field to accidentally equate them.
	trueP := BoolLiteral(true)
	r := pinToken(`{"is_error":true}`, "true", "is_error", trueP)
	wantVerdict(t, r, Confirmed)
	if r.Verdict == Confirmed {
		// This is the only assertion the verdict licenses. Nothing in Result
		// names an execution outcome.
		if r.Reason != ReasonLiteralMatch {
			t.Fatalf("confirmed verdict must carry LITERAL_MATCH")
		}
	}
}

// ---------------------------------------------------------------------------
// Result metadata is established only where safe
// ---------------------------------------------------------------------------

func TestResultSpansEstablishedOnlyWhenSafe(t *testing.T) {
	trueP := BoolLiteral(true)

	// Unverified: no spans established, ObjectOffset is -1.
	doc := `{"is_error":true}`
	loc := mkLoc(doc, 0, 1)
	loc.TranscriptSHA256[0] ^= 0xFF
	r := Verify([]byte(doc), loc, Predicate{Key: "is_error", Expect: trueP})
	if r.ObjectOffset != -1 || r.KeySpan != (Span{}) || r.ValueSpan != (Span{}) || r.MatchingOccurrences != 0 {
		t.Fatalf("unverified result leaked spans: %+v", r)
	}

	// Confirmed: spans point into the transcript.
	r = pinToken(doc, "true", "is_error", trueP)
	wantVerdict(t, r, Confirmed)
	if r.ObjectOffset != 0 {
		t.Fatalf("object offset = %d, want 0", r.ObjectOffset)
	}
	if r.ValueSpan.Start != 12 || r.ValueSpan.End != 16 {
		t.Fatalf("value span = %+v, want [12,16)", r.ValueSpan)
	}
	if r.KeySpan.Start != 1 || r.KeySpan.End != 11 {
		t.Fatalf("key span = %+v, want [1,11)", r.KeySpan)
	}
	if r.MatchingOccurrences != 1 {
		t.Fatalf("matching occurrences = %d, want 1", r.MatchingOccurrences)
	}
}
