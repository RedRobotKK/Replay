package transcript

import "testing"

// Permanent RED-1 regression: the specialized scanner must never let a present
// non-boolean is_error value become Absent, True, or False. This is the
// remediation of the differential divergence found by independent review.
func TestNonBooleanIsErrorIsPreserved(t *testing.T) {
	cases := []struct {
		name    string
		content string // bare tool_result object; the scanner sees it directly
		want    ErrorFlag
	}{
		{"null_alone", `{"type":"tool_result","is_error":null}`, ErrorFlagNonBoolean},
		{"null_then_true", `{"type":"tool_result","is_error":null,"is_error":true}`, ErrorFlagConflict},
		{"true_then_null", `{"type":"tool_result","is_error":true,"is_error":null}`, ErrorFlagConflict},
		{"null_then_false", `{"type":"tool_result","is_error":null,"is_error":false}`, ErrorFlagConflict},
		{"false_then_null", `{"type":"tool_result","is_error":false,"is_error":null}`, ErrorFlagConflict},
		{"null_null", `{"type":"tool_result","is_error":null,"is_error":null}`, ErrorFlagNonBoolean},
		{"string_value", `{"type":"tool_result","is_error":"true"}`, ErrorFlagNonBoolean},
		{"number_value", `{"type":"tool_result","is_error":1}`, ErrorFlagNonBoolean},
		{"array_value", `{"type":"tool_result","is_error":[]}`, ErrorFlagNonBoolean},
		{"object_value", `{"type":"tool_result","is_error":{}}`, ErrorFlagNonBoolean},
		{"string_then_true", `{"type":"tool_result","is_error":"true","is_error":true}`, ErrorFlagConflict},
		{"object_then_true", `{"type":"tool_result","is_error":{},"is_error":true}`, ErrorFlagConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Drive the scanner directly on the raw object bytes, since
			// encoding/json would reject most non-boolean values before the
			// scanner ran. The scanner itself must be correct on raw bytes.
			raw := []byte("[" + tc.content + "]")
			s := &flagScanner{b: raw}
			s.skipWS()
			if !s.expect('[') {
				t.Fatalf("fixture is not an array")
			}
			s.skipWS()
			got := s.scanToolResult()
			if got == nil {
				t.Fatalf("scanToolResult returned nil")
			}
			if *got != tc.want {
				t.Fatalf("got %v, want %v", *got, tc.want)
			}
		})
	}
}

// The same non-boolean values must never be counted as verified errors by the
// analysis path. Confirmed through ErrorCosts in the analysis package.
