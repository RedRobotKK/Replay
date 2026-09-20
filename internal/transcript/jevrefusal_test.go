package transcript

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The refusal paths, one contract violation at a time.
//
// guard-reachability reported 48 surviving conditionals in jev.go on
// 2026-09-20, after the obsolete build tag was removed and the contract tests
// began running in the ordinary suite. They are almost all error returns, and
// the reason they survived is not that the reader is wrong. It is that the
// existing tests reach each refusal through one representative input and the
// reader has many call sites that refuse for the same reason in different
// places.
//
// The largest single cause is worth naming, because it looks like coverage and
// is not. JC17 feeds a record carrying an unknown top-level field, which is the
// obvious test for jevOnly, and it deliberately sets contract_version to 2 so
// it can prove the version is blamed rather than the field. That record is
// refused at the version check, three lines above jevOnly, so the unknown-field
// refusal at version 1 had never run. The same hole exists at every nesting
// level: attempt, response, usage, and each of the three answer forms.
//
// These cases are not a fuzz corpus. Each one is a single named violation of a
// contract obligation MOC-3 already established, fed as the smallest record
// that reaches it, and each asserts the same two things the contract promises:
// the record is refused, and no partial result comes back with the error.

// jevBase is the smallest fully valid record. Cases below alter exactly one
// thing in it, so what each case tests is what differs from this line.
const jevBase = `{"contract_version":1,"evaluation_id":"replay_eval_00000000000000000000000000000001",` +
	`"requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":200,` +
	`"response":{"model":"jev-1.13.0","answers":{"q":{"type":"noul","noul":0.1}},` +
	`"usage":{"input_tokens":1,"output_tokens":1}}}]}`

// refuses asserts the contract's two standing promises about a bad record.
func refuses(t *testing.T, name, line string) {
	t.Helper()
	evs, err := parseJevLines(t, line)
	if err == nil {
		t.Errorf("%s: accepted; want refusal", name)
		return
	}
	if len(evs) != 0 {
		t.Errorf("%s: %d evaluations returned alongside an error; the reader returns no "+
			"partial result", name, len(evs))
	}
}

// An unknown field is refused at every level the contract declares a field set.
//
// This is the jevOnly family. A producer that adds a field bumps the version,
// so an unrecognised field at a version this build accepts means the two sides
// disagree about what the version means. Every one of these is at version 1,
// which is what JC17 cannot reach.
func TestJR1_UnknownFieldsAreRefusedAtEveryLevel(t *testing.T) {
	cases := map[string]string{
		"record": `{"contract_version":1,"evaluation_id":"replay_eval_00000000000000000000000000000002",` +
			`"requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":500}],"captured_by":"x"}`,
		"attempt": `{"contract_version":1,"evaluation_id":"replay_eval_00000000000000000000000000000003",` +
			`"requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":500,"latency_ms":12}]}`,
		"response": `{"contract_version":1,"evaluation_id":"replay_eval_00000000000000000000000000000004",` +
			`"requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":200,"response":{` +
			`"model":"jev-1.13.0","answers":{"q":{"type":"noul","noul":0.1}},` +
			`"usage":{"input_tokens":1,"output_tokens":1},"finish_reason":"stop"}}]}`,
		"usage": `{"contract_version":1,"evaluation_id":"replay_eval_00000000000000000000000000000005",` +
			`"requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":200,"response":{` +
			`"model":"jev-1.13.0","answers":{"q":{"type":"noul","noul":0.1}},` +
			`"usage":{"input_tokens":1,"output_tokens":1,"cached_tokens":0}}}]}`,
		"noul answer": `{"contract_version":1,"evaluation_id":"replay_eval_00000000000000000000000000000006",` +
			`"requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":200,"response":{` +
			`"model":"jev-1.13.0","answers":{"q":{"type":"noul","noul":0.1,"confidence":0.5}},` +
			`"usage":{"input_tokens":1,"output_tokens":1}}}]}`,
		"choice answer": `{"contract_version":1,"evaluation_id":"replay_eval_00000000000000000000000000000007",` +
			`"requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":200,"response":{` +
			`"model":"jev-1.13.0","answers":{"q":{"type":"choice","choice":"a","confidence":0.5,` +
			`"probabilities":{"a":1.0},"legend":{"0":"x"}}},"usage":{"input_tokens":1,"output_tokens":1}}}]}`,
		"score answer": `{"contract_version":1,"evaluation_id":"replay_eval_00000000000000000000000000000008",` +
			`"requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":200,"response":{` +
			`"model":"jev-1.13.0","answers":{"q":{"type":"score","score":0.5,"confidence":0.5,` +
			`"legend":{"0":"x"},"probabilities":{"0":1.0},"rubric":"v2"}},` +
			`"usage":{"input_tokens":1,"output_tokens":1}}}]}`,
	}
	for name, line := range cases {
		t.Run(name, func(t *testing.T) {
			refuses(t, name, line)
			// The refusal names the level it happened at, which is the only way
			// a producer can tell which of seven field sets it violated.
			if _, err := parseJevLines(t, line); err != nil {
				if !strings.Contains(err.Error(), "unknown field") {
					t.Errorf("%s: refused for some other reason than an unknown field: %v", name, err)
				}
			}
		})
	}
}

// A field whose JSON type is not the declared one is refused.
//
// These reach the decode failures inside jevString, jevInt, jevFloat,
// jevFloatMap and jevUnmarshal. The existing tests reach one call site of each
// helper; the reader calls them from many places and each call site is its own
// conditional.
func TestJR2_WrongJSONTypesAreRefused(t *testing.T) {
	cases := map[string]string{
		"requested_model not a string": strings.Replace(jevBase, `"requested_model":"jev-latest"`, `"requested_model":123`, 1),
		"http_status not a number":     strings.Replace(jevBase, `"http_status":200`, `"http_status":"200"`, 1),
		"attempts not an array":        strings.Replace(jevBase, `"attempts":[`, `"attempts":"[`, 1) + `"`,
		"answers not an object":        strings.Replace(jevBase, `"answers":{"q":{"type":"noul","noul":0.1}}`, `"answers":[]`, 1),
		"attempt not an object":        strings.Replace(jevBase, `"attempts":[{"ordinal":0`, `"attempts":["x",{"ordinal":0`, 1),
		"answer not an object":         strings.Replace(jevBase, `"q":{"type":"noul","noul":0.1}`, `"q":"noul"`, 1),
		"record not an object":         `"a bare string is not a record"`,
		"probabilities not an object": strings.Replace(jevBase,
			`"q":{"type":"noul","noul":0.1}`,
			`"q":{"type":"choice","choice":"a","confidence":0.5,"probabilities":"none"}`, 1),
		"output_tokens fractional": strings.Replace(jevBase, `"output_tokens":1`, `"output_tokens":1.5`, 1),
	}
	for name, line := range cases {
		t.Run(name, func(t *testing.T) { refuses(t, name, line) })
	}
}

// A declared field present as JSON null is refused rather than defaulted.
//
// null and absent are different statements and the reader accepts neither in
// place of a value. JC11 covers the two optional members; these are the
// required ones, which take a different path through jevField.
func TestJR3_NullRequiredFieldsAreRefused(t *testing.T) {
	cases := map[string]string{
		"requested_model": strings.Replace(jevBase, `"requested_model":"jev-latest"`, `"requested_model":null`, 1),
		"attempts":        strings.Replace(jevBase, `"attempts":[{"ordinal":0`, `"attempts":null,"unused":[{"ordinal":0`, 1),
		"answers":         strings.Replace(jevBase, `"answers":{"q":{"type":"noul","noul":0.1}}`, `"answers":null`, 1),
		"probabilities": strings.Replace(jevBase,
			`"q":{"type":"noul","noul":0.1}`,
			`"q":{"type":"choice","choice":"a","confidence":0.5,"probabilities":null}`, 1),
	}
	for name, line := range cases {
		t.Run(name, func(t *testing.T) { refuses(t, name, line) })
	}
}

// A required field that is simply absent is refused.
//
// jevField's missing branch is reached from every helper. JC14 covers the
// top-level members; these are the nested ones it does not reach.
func TestJR4_MissingNestedFieldsAreRefused(t *testing.T) {
	cases := map[string]string{
		"attempts": `{"contract_version":1,"evaluation_id":"replay_eval_00000000000000000000000000000009",` +
			`"requested_model":"jev-latest"}`,
		"choice probabilities": strings.Replace(jevBase,
			`"q":{"type":"noul","noul":0.1}`,
			`"q":{"type":"choice","choice":"a","confidence":0.5}`, 1),
		"score probabilities": strings.Replace(jevBase,
			`"q":{"type":"noul","noul":0.1}`,
			`"q":{"type":"score","score":0.5,"confidence":0.5,"legend":{"0":"x"}}`, 1),
		"choice confidence": strings.Replace(jevBase,
			`"q":{"type":"noul","noul":0.1}`,
			`"q":{"type":"choice","choice":"a","probabilities":{"a":1.0}}`, 1),
		"output_tokens": strings.Replace(jevBase, `,"output_tokens":1`, ``, 1),
	}
	for name, line := range cases {
		t.Run(name, func(t *testing.T) { refuses(t, name, line) })
	}
}

// An integer where the contract declares a float is refused, at every call
// site that declares one.
//
// JC4 covers score, noul and a choice probability. The reader reads a float in
// four more places and each is its own conditional.
func TestJR5_IntegerFloatsAreRefusedAtEveryCallSite(t *testing.T) {
	cases := map[string]string{
		"choice confidence": strings.Replace(jevBase,
			`"q":{"type":"noul","noul":0.1}`,
			`"q":{"type":"choice","choice":"a","confidence":1,"probabilities":{"a":1.0}}`, 1),
		"score confidence": strings.Replace(jevBase,
			`"q":{"type":"noul","noul":0.1}`,
			`"q":{"type":"score","score":0.5,"confidence":1,"legend":{"0":"x"},"probabilities":{"0":1.0}}`, 1),
		"score probability": strings.Replace(jevBase,
			`"q":{"type":"noul","noul":0.1}`,
			`"q":{"type":"score","score":0.5,"confidence":0.5,"legend":{"0":"x"},"probabilities":{"0":1}}`, 1),
	}
	for name, line := range cases {
		t.Run(name, func(t *testing.T) { refuses(t, name, line) })
	}
}

// A choice member of the wrong type is refused.
func TestJR6_ChoiceMembersAreTypeChecked(t *testing.T) {
	line := strings.Replace(jevBase,
		`"q":{"type":"noul","noul":0.1}`,
		`"q":{"type":"choice","choice":123,"confidence":0.5,"probabilities":{"a":1.0}}`, 1)
	refuses(t, "choice not a string", line)
}

// Malformed JSON is refused by the duplicate-key walk before any field is read.
//
// jevWalk runs first, so a stream that is not well-formed JSON never reaches
// the decoder. Its own failure paths are separate conditionals from the
// decoder's and no test reached them: JC14's "malformed json" case truncates
// after a key, which fails at a different point in the walk than a bad value,
// a bad key token, or trailing content does.
func TestJR7_MalformedJSONIsRefusedByTheWalk(t *testing.T) {
	cases := map[string]string{
		"truncated object":     `{"contract_version":1,`,
		"unterminated array":   `{"contract_version":1,"attempts":[`,
		"trailing content":     jevBase + ` trailing`,
		"not json at all":      `this line is not JSON`,
		"unterminated string":  `{"contract_version":"`,
		"value is not a token": `{"contract_version":@}`,
	}
	for name, line := range cases {
		t.Run(name, func(t *testing.T) { refuses(t, name, line) })
	}
}

// A blank line in the stream is skipped, not refused and not counted.
//
// The reader trims and continues. A capture written with a trailing newline or
// a blank separator is still one evaluation, and nothing in the existing tests
// put a blank line anywhere but at the end, where the scanner never yields it.
func TestJR8_BlankLinesAreSkipped(t *testing.T) {
	evs, err := parseJevLines(t, "", jevBase, "   ", jevBase2(), "")
	if err != nil {
		t.Fatalf("blank lines in the stream were refused: %v", err)
	}
	if len(evs) != 2 {
		t.Errorf("evaluations = %d, want 2: a blank line is not a record and is not an error", len(evs))
	}
}

// jevBase2 is jevBase under a second identity, so a two-record stream carries
// two distinct evaluations rather than one repeated.
func jevBase2() string {
	return strings.Replace(jevBase,
		"replay_eval_00000000000000000000000000000001",
		"replay_eval_00000000000000000000000000000002", 1)
}

// The file wrapper reports the path and refuses what the reader refuses.
//
// ParseJevCaptureFile has two failure paths of its own: the file could not be
// opened, and the stream inside it was invalid. Both wrap with context a caller
// needs and neither had a test.
func TestJR9_TheFileWrapperReportsItsOwnFailures(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		evs, err := ParseJevCaptureFile(filepath.Join(t.TempDir(), "absent.jsonl"))
		if err == nil {
			t.Fatal("a capture path that does not exist was read successfully")
		}
		if !strings.Contains(err.Error(), "open jev capture") {
			t.Errorf("the error does not say the file could not be opened: %v", err)
		}
		if len(evs) != 0 {
			t.Errorf("evaluations = %d, want 0", len(evs))
		}
	})
	t.Run("invalid content names the file", func(t *testing.T) {
		p := filepath.Join(t.TempDir(), "broken.jsonl")
		if err := os.WriteFile(p, []byte(`{"contract_version":99}`+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		evs, err := ParseJevCaptureFile(p)
		if err == nil {
			t.Fatal("an invalid capture file was read successfully")
		}
		// The base name, because a reader with several captures needs to know
		// which one is broken.
		if !strings.Contains(err.Error(), "broken.jsonl") {
			t.Errorf("the error does not name the file it came from: %v", err)
		}
		if len(evs) != 0 {
			t.Errorf("evaluations = %d, want 0", len(evs))
		}
	})
}

// A line the scanner could not hold is a broken stream, not an empty one.
//
// ParseJevCapture caps the scanner at 8 MiB. A longer line makes Scan return
// false with bufio.ErrTooLong parked in sc.Err(), and the loop ends exactly as
// it does at a clean end of input. Without the sc.Err() check the two are
// indistinguishable: the reader returns whatever it had collected so far and a
// nil error.
//
// That is the one failure this file's header rules out in as many words: "It is
// fail-fast and returns no partial result." Measured on 2026-09-20, neutralising
// that check turned an oversized capture from a refusal into an acceptance, and
// no test noticed. The valid record in front of the oversized line is what makes
// the defect visible as a partial result rather than merely as a missing error.
func TestJR10_AnOversizedLineIsARefusalNotAPartialResult(t *testing.T) {
	// One byte over the reader's own ceiling, built rather than guessed so the
	// test says what it depends on.
	const ceiling = 8 * 1024 * 1024
	oversized := `{"pad":"` + strings.Repeat("x", ceiling) + `"}`

	evs, err := parseJevLines(t, jevBase, oversized)
	if err == nil {
		t.Fatal("a capture whose line exceeds the reader's buffer was accepted; the reader " +
			"is fail-fast and a line it could not read is a broken stream")
	}
	if len(evs) != 0 {
		t.Errorf("evaluations = %d alongside the error, want 0: the record before the "+
			"oversized line was returned as a partial result, which this reader promises "+
			"never to do", len(evs))
	}
}

// A token the decoder rejects stops the walk.
//
// jevWalk reads a token and, when that fails, returns. Dropping the error does
// not merely lose a diagnostic: the function falls through to a type assertion
// on a nil token, which does not hold, and returns nil without having consumed
// anything. Its caller at the recursion site then loops on a decoder that never
// advances. Measured on 2026-09-20, neutralising this check did not change any
// refusal; it hung the package's test binary until the 600-second deadline.
//
// The liveness consequence is why the check matters and is not what this test
// asserts, because a test for termination has to bound the wait and a bounded
// wait is a worse check than the one below. What is asserted is the invariant
// underneath it, at the boundary where it is observable: a decode failure
// leaves the walk with an error rather than a nil that claims the document is
// well formed. ParseJevCapture cannot see this, because jevObject rejects the
// same input a moment later, so the assertion is made where the behaviour is.
func TestJR11_ADecodeFailureStopsTheDuplicateKeyWalk(t *testing.T) {
	for _, raw := range []string{
		`@`,       // not a token at all
		`{"a":@}`, // a value the decoder cannot read
		`{"a":1,`, // input ends mid-object
	} {
		if err := jevRejectDuplicateKeys([]byte(raw)); err == nil {
			t.Errorf("jevRejectDuplicateKeys(%q) returned nil; a decode failure must leave "+
				"the walk with an error rather than report the document well formed", raw)
		}
	}
}

// The namespace check is what keeps the length check from reading off the end.
//
// jevIdentity checks the prefix, then slices past it to count hex digits. The
// two reads look like one check with a better message in front of it, and they
// are not: the slice is only in range because the prefix was confirmed. Measured
// on 2026-09-20, neutralising the prefix check turned an id of "x" from a refusal
// into `slice bounds out of range [12:1]`, which is a panic out of a reader whose
// whole contract is that a bad record comes back as an error.
//
// Every existing case carries an id long enough to slice, which is why the
// guard survived neutralisation with the suite green. The ids below are shorter
// than the prefix, which is the only shape that tells the two reads apart.
func TestJR12_AnIDShorterThanTheNamespaceIsRefusedNotSliced(t *testing.T) {
	for _, id := range []string{"", "x", "replay_", "replay_eval"} {
		if err := jevIdentity(id); err == nil {
			t.Errorf("jevIdentity(%q) returned nil; an id shorter than the namespace "+
				"carries no namespace and must be refused", id)
		}
	}
}
