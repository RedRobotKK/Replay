// Contract tests for the Jev capture reader.
//
// These were written before the reader existed, and sat behind a `jevcontract`
// build tag for one reason: a test file naming symbols that have not been
// declared makes the whole package's test binary fail to build, and every
// unrelated transcript test then reports as a build failure rather than as a
// pass. The tag said so itself, and said it lasted only "until the reader
// lands".
//
// The reader landed in c0450a9 and the tag outlived its reason. It was also
// costing something: guard-reachability runs `go test` with no tags, so all 82
// conditionals in jev.go were reported as reached by no test at all, when in
// fact these seventeen cover them. A gate cannot observe a test it does not
// compile. They run in the ordinary suite now.
//
// Two of these tests exist because their defects pass silently. JC3 guards a
// score of 0.14 against becoming 0, which under the observed legend reads
// "Not urgent at all" — the inverse of the truth, with nothing to see in a
// diff. JC1 guards a noul answer against acquiring a confidence of 0.0 from
// absence, which is the same failure wearing a different hat: 0.0 is a real
// confidence value, so a defaulted one is indistinguishable from a measured
// one. Everything else here is ordinary contract coverage; those two are the
// reason the file exists.
package transcript

import (
	"reflect"
	"strings"
	"testing"
)

// The canonical record, built from the successful live response observed on
// 2026-09-20. The values are the provider's own: noul 0.95, choice confidence
// 1.0 over probabilities that include two genuine 0.0 entries, score 0.14
// against a three-level legend keyed by stringified integers, usage 366 and 78.
const jevValidLine = `{"contract_version":1,"evaluation_id":"replay_eval_9f2c41ab7d0e4c6188aa3b55e1d72f04","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":200,"provider_request_id":"req_01a0bc36271b768dac7d467bdc3f86bd","response":{"model":"jev-1.13.0","answers":{"is_facilities":{"type":"noul","noul":0.95},"owning_team":{"type":"choice","choice":"facilities","confidence":1.0,"probabilities":{"facilities":1.0,"it":0.0,"hr":0.0}},"urgency":{"type":"score","score":0.14,"confidence":0.79,"legend":{"0":"Not urgent at all","1":"Somewhat urgent","2":"Extremely urgent"},"probabilities":{"0":0.86,"1":0.14,"2":0.0}}},"usage":{"input_tokens":366,"output_tokens":78}}}]}`

func parseJevLines(t *testing.T, lines ...string) ([]JevEvaluation, error) {
	t.Helper()
	return ParseJevCapture(strings.NewReader(strings.Join(lines, "\n") + "\n"))
}

func mustParseJev(t *testing.T, lines ...string) []JevEvaluation {
	t.Helper()
	evs, err := parseJevLines(t, lines...)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return evs
}

// soleResponse returns the one response in an evaluation, failing if the count
// is not exactly one. Tests that care about a response should not also have to
// assert how many there were.
func soleResponse(t *testing.T, ev JevEvaluation) *JevResponse {
	t.Helper()
	var found *JevResponse
	n := 0
	for _, a := range ev.Attempts {
		if a.Response != nil {
			found, n = a.Response, n+1
		}
	}
	if n != 1 {
		t.Fatalf("responses = %d, want 1", n)
	}
	return found
}

// JC1 — a noul answer has no confidence member to default.
//
// This is asserted against the Go type, not against parsed output, and the
// distinction is the whole point. Checking that some confidence field happened
// to come back zero would pass just as well on a flattened representation that
// carries confidence for every variant — which is exactly the representation
// this invariant exists to forbid. The type either has the field or it does
// not, so that is what gets checked.
func TestJC1_NoulHasNoConfidenceMember(t *testing.T) {
	ty := reflect.TypeOf(JevNoul{})
	if _, ok := ty.FieldByName("Confidence"); ok {
		t.Errorf("JevNoul declares a Confidence field; absence would default to 0.0, "+
			"which is indistinguishable from a measured zero. type = %v", ty)
	}
	if ty.NumField() != 1 {
		t.Errorf("JevNoul has %d fields, want 1 (Noul)", ty.NumField())
	}

	ev := mustParseJev(t, jevValidLine)[0]
	a, ok := soleResponse(t, ev).Answers["is_facilities"].(JevNoul)
	if !ok {
		t.Fatalf("is_facilities = %T, want JevNoul", soleResponse(t, ev).Answers["is_facilities"])
	}
	if a.Noul != 0.95 {
		t.Errorf("Noul = %v, want 0.95", a.Noul)
	}
}

// JC2 — a confidence the provider actually reported as zero stays zero.
//
// The mirror of JC1. Absence must not become 0.0, and a measured 0.0 must not
// be mistaken for absence and dropped.
func TestJC2_GenuineZeroConfidenceSurvives(t *testing.T) {
	const line = `{"contract_version":1,"evaluation_id":"replay_eval_0000000000000000000000000000000a","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":200,"response":{"model":"jev-1.13.0","answers":{"owning_team":{"type":"choice","choice":"facilities","confidence":0.0,"probabilities":{"facilities":1.0,"it":0.0}}},"usage":{"input_tokens":1,"output_tokens":1}}}]}`
	ev := mustParseJev(t, line)[0]
	a, ok := soleResponse(t, ev).Answers["owning_team"].(JevChoice)
	if !ok {
		t.Fatalf("owning_team = %T, want JevChoice", soleResponse(t, ev).Answers["owning_team"])
	}
	if a.Confidence != 0.0 {
		t.Errorf("Confidence = %v, want 0.0 preserved", a.Confidence)
	}
	if got, want := a.Probabilities["it"], 0.0; got != want {
		t.Errorf("probabilities[it] = %v, want %v", got, want)
	}
}

// JC3 — a fractional score stays fractional.
//
// score is an expected value over the rubric, not an index into it. The
// observed 0.14 truncates to 0 under integer typing, and 0 in the observed
// legend is "Not urgent at all" — a confident reading of the opposite of what
// the provider said, produced without an error.
func TestJC3_FractionalScoreIsNotTruncated(t *testing.T) {
	ev := mustParseJev(t, jevValidLine)[0]
	a, ok := soleResponse(t, ev).Answers["urgency"].(JevScore)
	if !ok {
		t.Fatalf("urgency = %T, want JevScore", soleResponse(t, ev).Answers["urgency"])
	}
	if a.Score != 0.14 {
		t.Errorf("Score = %v, want 0.14", a.Score)
	}
	if a.Score == 0 {
		t.Errorf("Score truncated to 0; the legend reads that as %q", a.Legend["0"])
	}
	if a.Confidence != 0.79 {
		t.Errorf("Confidence = %v, want 0.79", a.Confidence)
	}
	if got := a.Legend["2"]; got != "Extremely urgent" {
		t.Errorf("legend[2] = %q, want %q", got, "Extremely urgent")
	}
	if got := a.Probabilities["0"]; got != 0.86 {
		t.Errorf("probabilities[0] = %v, want 0.86", got)
	}
}

// JC4 — a float64 field represented as an integer is refused, not coerced.
//
// Carrying provider values as observed means refusing a representation the
// contract does not declare, rather than quietly widening it.
func TestJC4_IntegerRepresentationOfFloatFieldIsRefused(t *testing.T) {
	cases := map[string]string{
		"score":       `{"contract_version":1,"evaluation_id":"replay_eval_0000000000000000000000000000000b","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":200,"response":{"model":"jev-1.13.0","answers":{"urgency":{"type":"score","score":0,"confidence":0.79,"legend":{"0":"a","1":"b"},"probabilities":{"0":1.0,"1":0.0}}},"usage":{"input_tokens":1,"output_tokens":1}}}]}`,
		"noul":        `{"contract_version":1,"evaluation_id":"replay_eval_0000000000000000000000000000000c","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":200,"response":{"model":"jev-1.13.0","answers":{"q":{"type":"noul","noul":1}},"usage":{"input_tokens":1,"output_tokens":1}}}]}`,
		"probability": `{"contract_version":1,"evaluation_id":"replay_eval_0000000000000000000000000000000d","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":200,"response":{"model":"jev-1.13.0","answers":{"q":{"type":"choice","choice":"a","confidence":0.5,"probabilities":{"a":1,"b":0}}},"usage":{"input_tokens":1,"output_tokens":1}}}]}`,
	}
	for name, line := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parseJevLines(t, line); err == nil {
				t.Errorf("%s: integer representation accepted; want refusal", name)
			}
		})
	}
}

// JC5 — an unknown contract version is refused loudly, and nothing comes back.
//
// Skipping it silently, parsing it partially, or treating it as the current
// version all turn evidence Replay does not understand into evidence it
// appears to.
func TestJC5_UnknownContractVersionIsRefused(t *testing.T) {
	const line = `{"contract_version":99,"evaluation_id":"replay_eval_0000000000000000000000000000000e","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":500}]}`
	evs, err := parseJevLines(t, line)
	if err == nil {
		t.Fatal("unknown contract_version accepted; want refusal")
	}
	if !strings.Contains(err.Error(), "99") {
		t.Errorf("error does not name the unsupported version: %v", err)
	}
	if len(evs) != 0 {
		t.Errorf("evaluations = %d, want 0 alongside an error", len(evs))
	}
}

// JC6 — a version the reader accepts parses, and the version is carried.
func TestJC6_AcceptedContractVersionParses(t *testing.T) {
	ev := mustParseJev(t, jevValidLine)[0]
	if ev.ContractVersion != 1 {
		t.Errorf("ContractVersion = %d, want 1", ev.ContractVersion)
	}
	if ev.EvaluationID != "replay_eval_9f2c41ab7d0e4c6188aa3b55e1d72f04" {
		t.Errorf("EvaluationID = %q, not carried verbatim", ev.EvaluationID)
	}
	if ev.RequestedModel != "jev-latest" {
		t.Errorf("RequestedModel = %q, want jev-latest", ev.RequestedModel)
	}
	if got := soleResponse(t, ev).Model; got != "jev-1.13.0" {
		t.Errorf("resolved Model = %q, want jev-1.13.0", got)
	}
}

// JC7 — the response may sit on any attempt, and stays on the one that got it.
//
// The current SDK returns on first success, so the successful attempt happens
// to be last today. Encoding that would bind Replay's reader to a third party's
// retry loop, and would reject valid evidence the day it changed.
func TestJC7_ResponseMayOccurOnANonFinalAttempt(t *testing.T) {
	const line = `{"contract_version":1,"evaluation_id":"replay_eval_0000000000000000000000000000000f","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":200,"response":{"model":"jev-1.13.0","answers":{"q":{"type":"noul","noul":0.5}},"usage":{"input_tokens":1,"output_tokens":1}}},{"ordinal":1,"http_status":500}]}`
	ev := mustParseJev(t, line)[0]
	if len(ev.Attempts) != 2 {
		t.Fatalf("attempts = %d, want 2", len(ev.Attempts))
	}
	if ev.Attempts[0].Response == nil {
		t.Error("attempt 0 lost its response")
	}
	if ev.Attempts[1].Response != nil {
		t.Error("attempt 1 acquired a response it did not have")
	}
	if ev.Attempts[0].Ordinal != 0 || ev.Attempts[1].Ordinal != 1 {
		t.Errorf("ordinals = %d,%d, want 0,1", ev.Attempts[0].Ordinal, ev.Attempts[1].Ordinal)
	}
}

// JC8 — an evaluation that never got an answer is valid, and stays empty.
//
// Every attempt failed. There is no model, no answer and no usage to report,
// and the reader must not manufacture a zero-valued one: a JevResponse full of
// zeroes would read as a successful evaluation that cost nothing.
func TestJC8_EvaluationWithNoResponseIsValidAndFabricatesNothing(t *testing.T) {
	const line = `{"contract_version":1,"evaluation_id":"replay_eval_00000000000000000000000000000010","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":500},{"ordinal":1,"http_status":529}]}`
	ev := mustParseJev(t, line)[0]
	if len(ev.Attempts) != 2 {
		t.Fatalf("attempts = %d, want 2", len(ev.Attempts))
	}
	for i, a := range ev.Attempts {
		if a.Response != nil {
			t.Errorf("attempt %d fabricated a response: %+v", i, *a.Response)
		}
	}
	if ev.Attempts[1].HTTPStatus != 529 {
		t.Errorf("HTTPStatus = %d, want 529 carried as observed", ev.Attempts[1].HTTPStatus)
	}
}

// JC9 — two response-bearing attempts is an invalid artifact, not a precedence
// question. The reader refuses rather than choosing one.
func TestJC9_MultipleResponsesAreRefused(t *testing.T) {
	const line = `{"contract_version":1,"evaluation_id":"replay_eval_00000000000000000000000000000011","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":200,"response":{"model":"jev-1.13.0","answers":{"q":{"type":"noul","noul":0.1}},"usage":{"input_tokens":1,"output_tokens":1}}},{"ordinal":1,"http_status":200,"response":{"model":"jev-1.13.0","answers":{"q":{"type":"noul","noul":0.9}},"usage":{"input_tokens":1,"output_tokens":1}}}]}`
	if _, err := parseJevLines(t, line); err == nil {
		t.Error("two response-bearing attempts accepted; want refusal")
	}
}

// JC10 — an omitted provider request id is absence, and says so.
//
// The header is optional on the wire. An empty string alone cannot carry that,
// which is why the measured flag exists beside it.
func TestJC10_OmittedProviderRequestIDIsAbsentNotEmpty(t *testing.T) {
	const line = `{"contract_version":1,"evaluation_id":"replay_eval_00000000000000000000000000000012","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":500}]}`
	ev := mustParseJev(t, line)[0]
	a := ev.Attempts[0]
	if a.ProviderRequestID != "" {
		t.Errorf("ProviderRequestID = %q, want empty", a.ProviderRequestID)
	}
	if a.RequestIDMeasured {
		t.Error("RequestIDMeasured = true for an omitted header")
	}

	ev2 := mustParseJev(t, jevValidLine)[0]
	if !ev2.Attempts[0].RequestIDMeasured {
		t.Error("RequestIDMeasured = false for a header that was present")
	}
}

// JC11 — null, empty string and empty object are not how absence is written.
//
// Omission is. Each of these asserts something the producer did not observe:
// a known-null, an empty identifier, a response with no fields.
func TestJC11_InvalidOptionalRepresentationsAreRefused(t *testing.T) {
	cases := map[string]string{
		"null provider_request_id":  `{"contract_version":1,"evaluation_id":"replay_eval_00000000000000000000000000000013","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":500,"provider_request_id":null}]}`,
		"empty provider_request_id": `{"contract_version":1,"evaluation_id":"replay_eval_00000000000000000000000000000014","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":500,"provider_request_id":""}]}`,
		"null response":             `{"contract_version":1,"evaluation_id":"replay_eval_00000000000000000000000000000015","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":500,"response":null}]}`,
		"empty response":            `{"contract_version":1,"evaluation_id":"replay_eval_00000000000000000000000000000016","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":500,"response":{}}]}`,
	}
	for name, line := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parseJevLines(t, line); err == nil {
				t.Errorf("%s accepted; want refusal", name)
			}
		})
	}
}

// JC12 — identical content does not merge two evaluations.
//
// Identity is minted per invocation and carried verbatim. Nothing about the
// request, the questions or the answers participates in it, so two records
// that differ only in their identity are two evaluations.
func TestJC12_IdenticalContentDoesNotCollapseTwoEvaluations(t *testing.T) {
	const a = `{"contract_version":1,"evaluation_id":"replay_eval_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":200,"response":{"model":"jev-1.13.0","answers":{"q":{"type":"noul","noul":0.95}},"usage":{"input_tokens":366,"output_tokens":78}}}]}`
	const b = `{"contract_version":1,"evaluation_id":"replay_eval_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":200,"response":{"model":"jev-1.13.0","answers":{"q":{"type":"noul","noul":0.95}},"usage":{"input_tokens":366,"output_tokens":78}}}]}`
	evs := mustParseJev(t, a, b)
	if len(evs) != 2 {
		t.Fatalf("evaluations = %d, want 2", len(evs))
	}
	if evs[0].EvaluationID == evs[1].EvaluationID {
		t.Fatal("identities collapsed")
	}
	if evs[0].EvaluationID != "replay_eval_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" ||
		evs[1].EvaluationID != "replay_eval_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" {
		t.Errorf("identities not carried verbatim: %q, %q", evs[0].EvaluationID, evs[1].EvaluationID)
	}
}

// JC13 — duplicate object keys are refused rather than resolved.
//
// encoding/json takes the last one without complaint. Which of two usage
// objects the provider meant is not something a parser default should decide.
func TestJC13_DuplicateJSONKeysAreRefused(t *testing.T) {
	const line = `{"contract_version":1,"evaluation_id":"replay_eval_00000000000000000000000000000017","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":200,"response":{"model":"jev-1.13.0","answers":{"q":{"type":"noul","noul":0.1}},"usage":{"input_tokens":1,"output_tokens":1},"usage":{"input_tokens":999,"output_tokens":999}}}]}`
	if _, err := parseJevLines(t, line); err == nil {
		t.Error("duplicate JSON key accepted; want refusal rather than last-wins")
	}
}

// JC14 — structural violations of the contract are refused.
//
// One table rather than fourteen functions, because each case is the same
// assertion against a different malformation and the value is in the list
// being complete.
func TestJC14_StructuralViolationsAreRefused(t *testing.T) {
	cases := map[string]string{
		"missing contract_version": `{"evaluation_id":"replay_eval_00000000000000000000000000000018","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":500}]}`,
		"missing evaluation_id":    `{"contract_version":1,"requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":500}]}`,
		"missing requested_model":  `{"contract_version":1,"evaluation_id":"replay_eval_00000000000000000000000000000019","attempts":[{"ordinal":0,"http_status":500}]}`,
		"empty attempts":           `{"contract_version":1,"evaluation_id":"replay_eval_0000000000000000000000000000001a","requested_model":"jev-latest","attempts":[]}`,
		"missing ordinal":          `{"contract_version":1,"evaluation_id":"replay_eval_0000000000000000000000000000001b","requested_model":"jev-latest","attempts":[{"http_status":500}]}`,
		"missing http_status":      `{"contract_version":1,"evaluation_id":"replay_eval_0000000000000000000000000000001c","requested_model":"jev-latest","attempts":[{"ordinal":0}]}`,
		"negative ordinal":         `{"contract_version":1,"evaluation_id":"replay_eval_0000000000000000000000000000001d","requested_model":"jev-latest","attempts":[{"ordinal":-1,"http_status":500}]}`,
		"non-contiguous ordinals":  `{"contract_version":1,"evaluation_id":"replay_eval_0000000000000000000000000000001e","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":500},{"ordinal":2,"http_status":500}]}`,
		"ordinals not 0-based":     `{"contract_version":1,"evaluation_id":"replay_eval_0000000000000000000000000000001f","requested_model":"jev-latest","attempts":[{"ordinal":1,"http_status":500}]}`,
		"missing response model":   `{"contract_version":1,"evaluation_id":"replay_eval_00000000000000000000000000000020","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":200,"response":{"answers":{"q":{"type":"noul","noul":0.1}},"usage":{"input_tokens":1,"output_tokens":1}}}]}`,
		"missing response usage":   `{"contract_version":1,"evaluation_id":"replay_eval_00000000000000000000000000000021","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":200,"response":{"model":"jev-1.13.0","answers":{"q":{"type":"noul","noul":0.1}}}}]}`,
		"empty answers":            `{"contract_version":1,"evaluation_id":"replay_eval_00000000000000000000000000000022","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":200,"response":{"model":"jev-1.13.0","answers":{},"usage":{"input_tokens":1,"output_tokens":1}}}]}`,
		"unknown discriminator":    `{"contract_version":1,"evaluation_id":"replay_eval_00000000000000000000000000000023","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":200,"response":{"model":"jev-1.13.0","answers":{"q":{"type":"verdict","verdict":true}},"usage":{"input_tokens":1,"output_tokens":1}}}]}`,
		"missing discriminator":    `{"contract_version":1,"evaluation_id":"replay_eval_00000000000000000000000000000024","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":200,"response":{"model":"jev-1.13.0","answers":{"q":{"noul":0.1}},"usage":{"input_tokens":1,"output_tokens":1}}}]}`,
		"float token count":        `{"contract_version":1,"evaluation_id":"replay_eval_00000000000000000000000000000025","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":200,"response":{"model":"jev-1.13.0","answers":{"q":{"type":"noul","noul":0.1}},"usage":{"input_tokens":1.5,"output_tokens":1}}}]}`,
		"string probability value": `{"contract_version":1,"evaluation_id":"replay_eval_00000000000000000000000000000026","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":200,"response":{"model":"jev-1.13.0","answers":{"q":{"type":"choice","choice":"a","confidence":0.5,"probabilities":{"a":"1.0"}}},"usage":{"input_tokens":1,"output_tokens":1}}}]}`,
		"non-string legend value":  `{"contract_version":1,"evaluation_id":"replay_eval_00000000000000000000000000000027","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":200,"response":{"model":"jev-1.13.0","answers":{"q":{"type":"score","score":0.5,"confidence":0.5,"legend":{"0":1,"1":2},"probabilities":{"0":0.5,"1":0.5}}},"usage":{"input_tokens":1,"output_tokens":1}}}]}`,
		"wrong id namespace":       `{"contract_version":1,"evaluation_id":"req_01a0bc36271b768dac7d467bdc3f86bd","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":500}]}`,
		"id not hex":               `{"contract_version":1,"evaluation_id":"replay_eval_zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":500}]}`,
		"id too short":             `{"contract_version":1,"evaluation_id":"replay_eval_9f2c41ab","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":500}]}`,
		"id uppercase hex":         `{"contract_version":1,"evaluation_id":"replay_eval_9F2C41AB7D0E4C6188AA3B55E1D72F04","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":500}]}`,
		"malformed json":           `{"contract_version":1,"evaluation_id":`,
	}
	for name, line := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := parseJevLines(t, line); err == nil {
				t.Errorf("%s accepted; want refusal", name)
			}
		})
	}
}

// JC15 — a bad record stops the stream and takes the good ones with it.
//
// Fail-fast, and no partial slice beside the error. A Jev capture is produced
// solely as evidence, so an invalid record means the producer is broken. A
// count of skipped lines would be a quieter way for that to disappear.
func TestJC15_InvalidRecordAbortsTheStream(t *testing.T) {
	const bad = `{"contract_version":99,"evaluation_id":"replay_eval_00000000000000000000000000000028","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":500}]}`
	evs, err := parseJevLines(t, jevValidLine, bad, jevValidLine)
	if err == nil {
		t.Fatal("stream with an invalid record parsed; want refusal")
	}
	if len(evs) != 0 {
		t.Errorf("evaluations = %d alongside an error, want 0", len(evs))
	}
	if !strings.Contains(err.Error(), "2") {
		t.Errorf("error does not locate the offending line: %v", err)
	}
}

// JC16 — the file wrapper reads what the io.Reader form reads.
func TestJC16_FileWrapperMatchesReaderForm(t *testing.T) {
	fromFile, err := ParseJevCaptureFile("jevdata/valid.jsonl")
	if err != nil {
		t.Fatalf("ParseJevCaptureFile: %v", err)
	}
	fromReader := mustParseJev(t, jevValidLine)
	if len(fromFile) != len(fromReader) {
		t.Fatalf("file = %d evaluations, reader = %d", len(fromFile), len(fromReader))
	}
	if fromFile[0].EvaluationID != fromReader[0].EvaluationID {
		t.Errorf("file and reader disagree: %q vs %q",
			fromFile[0].EvaluationID, fromReader[0].EvaluationID)
	}
}

// JC17 — a future version is reported as a future version.
//
// A version bump is how new fields arrive, so a v2 record carrying a field v1
// has never heard of is the expected shape of a future producer. Blaming the
// field tells the reader to go and delete it; naming the version tells them
// what is actually true, which is that this build does not read v2.
func TestJC17_FutureVersionIsNamedRatherThanItsNewField(t *testing.T) {
	const line = `{"contract_version":2,"evaluation_id":"replay_eval_00000000000000000000000000000099","requested_model":"jev-latest","attempts":[{"ordinal":0,"http_status":500}],"captured_by":"future-producer"}`
	evs, err := parseJevLines(t, line)
	if err == nil {
		t.Fatal("v2 record accepted; want refusal")
	}
	if !strings.Contains(err.Error(), "2") {
		t.Errorf("error does not name the unsupported version: %v", err)
	}
	if strings.Contains(err.Error(), "captured_by") {
		t.Errorf("error blames the new field instead of the version: %v", err)
	}
	if len(evs) != 0 {
		t.Errorf("evaluations = %d, want 0", len(evs))
	}
}
