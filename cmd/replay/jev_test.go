package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Contract tests for `replay jev`, the Jev capture inspection surface.
//
// The command exists to make an approved capture artifact readable without
// promoting it into the Session/Tier/analysis architecture. Two properties
// carry most of the weight here and neither is visible from the happy path:
// the command must fail on any malformed input rather than printing a report
// that looks complete, and it must never emit a word from the accounting or
// provenance vocabulary, because Jev reports no cache fields and a zero there
// would be a measurement nobody made.

const jevValidFixture = "../../internal/transcript/jevdata/valid.jsonl"

// jevRun invokes the command through the real dispatch table.
func jevRun(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	var out, errb bytes.Buffer
	err := run(append([]string{"jev"}, args...), &out, &errb)
	return out.String(), errb.String(), err
}

// jevWrite puts a capture in a temporary directory. Fixtures are written here
// rather than committed so that the frozen jevdata/ directory keeps exactly
// the one artifact MOC-3 approved.
func jevWrite(t *testing.T, name, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// A valid evaluation whose single attempt failed, so no response exists.
const jevAllFailed = `{"contract_version":1,"evaluation_id":"replay_eval_00000000000000000000000000000501","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":500},{"ordinal":1,"http_status":503}]}`

// A response on the first of two attempts, which the contract permits.
const jevResponseFirst = `{"contract_version":1,"evaluation_id":"replay_eval_00000000000000000000000000000777","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":200,"response":{"model":"jev-1.13.0","answers":{"a":{"type":"noul","noul":0.25}},"usage":{"input_tokens":10,"output_tokens":2}}},{"ordinal":1,"http_status":500}]}`

// TestJevReadsTheApprovedFixture is the happy path over the committed artifact.
func TestJevReadsTheApprovedFixture(t *testing.T) {
	out, errb, err := jevRun(t, jevValidFixture)
	if err != nil {
		t.Fatalf("replay jev <valid>: %v (stderr: %s)", err, errb)
	}
	if n := strings.Count(out, "replay_eval_"); n != 1 {
		t.Fatalf("evaluations printed = %d, want 1:\n%s", n, out)
	}
	if !strings.Contains(out, "replay_eval_9f2c41ab7d0e4c6188aa3b55e1d72f04") {
		t.Errorf("evaluation identity not preserved:\n%s", out)
	}
}

// TestJevKeepsRequestedAndResolvedModelDistinct pins the two apart.
//
// The fixture asked for jev-latest and was answered by jev-1.13.0. Printing
// one of them twice would lose the resolution, which is the only place the
// alias is observable.
func TestJevKeepsRequestedAndResolvedModelDistinct(t *testing.T) {
	out, _, err := jevRun(t, jevValidFixture)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "jev-latest") {
		t.Errorf("requested model absent:\n%s", out)
	}
	if !strings.Contains(out, "jev-1.13.0") {
		t.Errorf("resolved model absent:\n%s", out)
	}
}

// TestJevShowsAttemptOrdinalAndStatus keeps the attempt observable.
func TestJevShowsAttemptOrdinalAndStatus(t *testing.T) {
	out, _, err := jevRun(t, jevValidFixture)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"attempt 0", "200"} {
		if !strings.Contains(out, want) {
			t.Errorf("output does not carry %q:\n%s", want, out)
		}
	}
}

// TestJevShowsProviderRequestIDOnlyWhenMeasured.
//
// The header is optional, so an absent id and an empty id are different facts.
// The command prints the id when RequestIDMeasured is true and prints nothing
// in its place when it is false, rather than an empty column that reads as a
// measured blank.
func TestJevShowsProviderRequestIDOnlyWhenMeasured(t *testing.T) {
	out, _, err := jevRun(t, jevValidFixture)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "req_01a0bc36271b768dac7d467bdc3f86bd") {
		t.Errorf("measured provider request id absent:\n%s", out)
	}

	out2, _, err := jevRun(t, jevWrite(t, "noid.jsonl", jevAllFailed+"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out2, "request id") {
		t.Errorf("unmeasured provider request id was given a column:\n%s", out2)
	}
}

// TestJevProviderRequestIDIsNotEvaluationIdentity.
//
// Grouping by the provider's id would make one retried evaluation look like
// two, and would make identity depend on a field the provider may omit.
func TestJevProviderRequestIDIsNotEvaluationIdentity(t *testing.T) {
	out, _, err := jevRun(t, jevWrite(t, "retry.jsonl", jevResponseFirst+"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(out, "replay_eval_"); n != 1 {
		t.Fatalf("one evaluation with two attempts printed %d identities:\n%s", n, out)
	}
	for _, want := range []string{"attempt 0", "attempt 1"} {
		if !strings.Contains(out, want) {
			t.Errorf("retry attempt %q not nested under the evaluation:\n%s", want, out)
		}
	}
}

// TestJevAcceptsAResponseOnANonFinalAttempt.
//
// The contract permits the response anywhere in the sequence, so the command
// must not assume it belongs to the last attempt.
func TestJevAcceptsAResponseOnANonFinalAttempt(t *testing.T) {
	out, _, err := jevRun(t, jevWrite(t, "first.jsonl", jevResponseFirst+"\n"))
	if err != nil {
		t.Fatalf("response on a non-final attempt rejected: %v", err)
	}
	i0, i1 := strings.Index(out, "attempt 0"), strings.Index(out, "attempt 1")
	usage := strings.Index(out, "10")
	if i0 < 0 || i1 < 0 || usage < 0 {
		t.Fatalf("attempts or usage missing:\n%s", out)
	}
	if i0 >= usage || usage >= i1 {
		t.Errorf("the response was not attached to attempt 0:\n%s", out)
	}
}

// TestJevAllFailedEvaluationIsValidAndFabricatesNothing.
//
// An evaluation whose every attempt failed is a valid record. The refusal to
// invent a response is the point: a zero-valued usage line would read as a
// successful evaluation that cost nothing.
func TestJevAllFailedEvaluationIsValidAndFabricatesNothing(t *testing.T) {
	out, errb, err := jevRun(t, jevWrite(t, "failed.jsonl", jevAllFailed+"\n"))
	if err != nil {
		t.Fatalf("all-failed evaluation rejected: %v (stderr: %s)", err, errb)
	}
	if !strings.Contains(out, "no response") {
		t.Errorf("no explicit no-response marker:\n%s", out)
	}
	for _, banned := range []string{"0 input", "0 output", "input 0", "output 0"} {
		if strings.Contains(out, banned) {
			t.Errorf("fabricated zero usage %q for an evaluation with no response:\n%s", banned, out)
		}
	}
}

// TestJevShowsUsageOnlyWhereAResponseExists.
func TestJevShowsUsageOnlyWhereAResponseExists(t *testing.T) {
	out, _, err := jevRun(t, jevValidFixture)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"366", "78"} {
		if !strings.Contains(out, want) {
			t.Errorf("observed usage %q absent:\n%s", want, out)
		}
	}
}

// TestJevPreservesProviderAnswerValues.
//
// The three answer forms are carried as the provider reported them. Nothing is
// re-derived: a probability is printed, never recomputed from the others, and
// a score is not read as an index into its legend.
func TestJevPreservesProviderAnswerValues(t *testing.T) {
	out, _, err := jevRun(t, jevValidFixture)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"0.95", // noul, carried as a probability rather than a boolean
		"facilities",
		"0.14", // score, between legend levels
		"0.79", // its confidence
		"0.86", // a provider probability, not recomputed
	} {
		if !strings.Contains(out, want) {
			t.Errorf("provider value %q was not preserved:\n%s", want, out)
		}
	}
}

// TestJevDoesNotEmitProvenanceVocabulary.
//
// C-1 keeps Source.Tier() a two-valued label and authorises no new source into
// the Session path. This command never constructs a Session, and this test is
// what notices if it ever starts.
func TestJevDoesNotEmitProvenanceVocabulary(t *testing.T) {
	out, _, err := jevRun(t, jevValidFixture)
	if err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{
		"Tier:", "measured", "estimated", "proxy-recorded", "transcripts only",
	} {
		if strings.Contains(out, banned) {
			t.Errorf("output carries the provenance token %q, so Jev has entered a "+
				"reporting path C-1 does not authorise:\n%s", banned, out)
		}
	}
}

// TestJevDoesNotEmitAccountingVocabulary.
//
// Jev reports input and output tokens and no cache fields at all. ADR-0019 is
// explicit that a missing capability must not read as a zero, so the command
// says nothing about cache, cost or re-billing rather than saying zero.
func TestJevDoesNotEmitAccountingVocabulary(t *testing.T) {
	out, _, err := jevRun(t, jevValidFixture)
	if err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{
		"$", "cost", "savings", "avoidab", "rebilled", "pricing", "cache",
	} {
		if strings.Contains(strings.ToLower(out), banned) {
			t.Errorf("output carries the accounting token %q, which this surface has no "+
				"evidence for:\n%s", banned, out)
		}
	}
}

// TestJevRefusesMalformedInput covers every refusal the reader owns.
//
// Each case is a whole-command failure. The reader is fail-fast and the command
// must not soften that into a warning beside a report.
func TestJevRefusesMalformedInput(t *testing.T) {
	cases := []struct{ name, body string }{
		{"malformed json", `{"contract_version":1,`},
		{"unknown contract version", `{"contract_version":2,"evaluation_id":"replay_eval_00000000000000000000000000000099","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":500}]}`},
		{"unknown field", `{"contract_version":1,"evaluation_id":"replay_eval_00000000000000000000000000000099","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":500}],"captured_by":"x"}`},
		{"duplicate key", `{"contract_version":1,"contract_version":1,"evaluation_id":"replay_eval_00000000000000000000000000000099","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":500}]}`},
		{"integer for a float", `{"contract_version":1,"evaluation_id":"replay_eval_00000000000000000000000000000099","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":200,"response":{"model":"m","answers":{"a":{"type":"noul","noul":1}},"usage":{"input_tokens":1,"output_tokens":1}}}]}`},
		{"non-contiguous ordinal", `{"contract_version":1,"evaluation_id":"replay_eval_00000000000000000000000000000099","requested_model":"jev-latest","attempts":[{"ordinal":7,"http_status":500}]}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out, _, err := jevRun(t, jevWrite(t, "bad.jsonl", c.body+"\n"))
			if err == nil {
				t.Fatalf("accepted %s; want a command failure:\n%s", c.name, out)
			}
			if strings.Contains(out, "replay_eval_") {
				t.Errorf("printed an evaluation after a parse failure:\n%s", out)
			}
		})
	}
}

// TestJevMalformedInputIsAtomicAcrossPaths.
//
// One good file and one bad one is a failed invocation with no report. A report
// covering only the files that happened to parse is the shape a reader cannot
// tell from a complete one.
func TestJevMalformedInputIsAtomicAcrossPaths(t *testing.T) {
	bad := jevWrite(t, "bad.jsonl", `{"contract_version":9}`+"\n")
	out, _, err := jevRun(t, jevValidFixture, bad)
	if err == nil {
		t.Fatalf("one malformed input did not fail the invocation:\n%s", out)
	}
	if strings.Contains(out, "replay_eval_") {
		t.Errorf("partial report emitted alongside a failed input:\n%s", out)
	}
}

// TestJevWithNoArgumentsIsAUsageError.
//
// There is no vendor home to search. The capture is Replay-owned and arrives on
// a stream, so a path is the only way to name one and guessing would be wrong.
func TestJevWithNoArgumentsIsAUsageError(t *testing.T) {
	if _, _, err := jevRun(t); err == nil {
		t.Fatal("replay jev with no path succeeded; want a usage error")
	}
}
