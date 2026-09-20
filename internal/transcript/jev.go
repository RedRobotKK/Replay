package transcript

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// SourceJev marks evidence read from a Jev capture stream.
const SourceJev Source = "jev-capture"

// jevContractVersion is the only artifact contract version this build reads.
//
// The producer bumps it on any change to the field set it emits, so two
// records carrying the same version have the same shape. That is what lets the
// reader refuse an unrecognised version outright instead of parsing what it
// recognises and leaving the rest to be noticed later, or not.
const jevContractVersion = 1

// JevEvaluation is one observed Jev SDK invocation and the attempts it made.
//
// The identity is Replay's, minted at the observation seam, and is carried
// here verbatim. It is opaque: nothing reads through it, and nothing derives
// it from the request, the questions or the answers, so two invocations
// carrying the same bytes remain two evaluations.
type JevEvaluation struct {
	ContractVersion int
	EvaluationID    string
	RequestedModel  string
	Attempts        []JevAttempt
}

// JevAttempt is one HTTP attempt within an evaluation.
type JevAttempt struct {
	Ordinal    int
	HTTPStatus int
	// ProviderRequestID is the provider's own id when the response carried
	// one. RequestIDMeasured says whether it did: the header is optional, so
	// an empty string on its own cannot tell absence from an empty value.
	ProviderRequestID string
	RequestIDMeasured bool
	// Response is nil on every attempt that did not produce one, which is
	// every attempt of an evaluation that never succeeded. A zero-valued
	// response would read as a successful evaluation that cost nothing.
	Response *JevResponse
}

// JevResponse is the provider material from the attempt that produced it.
type JevResponse struct {
	Model   string
	Answers map[string]JevAnswer
	Usage   JevUsage
}

// JevUsage is the provider's own accounting. Jev reports no cache figures, so
// there are none here to report as zero.
type JevUsage struct {
	InputTokens  int
	OutputTokens int
}

// JevAnswer is one typed answer. The marker keeps the union closed to the
// three forms the provider returns.
type JevAnswer interface{ jevAnswer() }

// JevNoul carries a probability, not a boolean, and has no confidence member.
//
// The absent member is the point. A shared answer struct would give a noul
// answer a confidence field that JSON absence sets to 0.0, and 0.0 is a
// confidence the provider does report, so the defaulted one would be
// indistinguishable from a measured one.
type JevNoul struct {
	Noul float64
}

// JevChoice carries the selected label with probabilities over every label
// offered, including the ones that came back at zero.
type JevChoice struct {
	Choice        string
	Confidence    float64
	Probabilities map[string]float64
}

// JevScore carries an expected score over the rubric.
//
// Score is not an index into Legend. The observed 0.14 lies between levels,
// and reading it as an index would report level 0 with no sign of trouble.
// Legend and Probabilities keep their string keys for the same reason: they
// are the provider's, and converting them would be reinterpreting them.
type JevScore struct {
	Score         float64
	Confidence    float64
	Legend        map[string]string
	Probabilities map[string]float64
}

func (JevNoul) jevAnswer()   {}
func (JevChoice) jevAnswer() {}
func (JevScore) jevAnswer()  {}

// ParseJevCaptureFile reads a capture file. The stream form is the reader.
func ParseJevCaptureFile(path string) ([]JevEvaluation, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open jev capture: %w", err)
	}
	defer f.Close()
	evs, err := ParseJevCapture(f)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return evs, nil
}

// ParseJevCapture reads a JSON Lines capture stream, one evaluation per line.
//
// It is fail-fast and returns no partial result. The artifact is produced
// solely as evidence, so an invalid record means the producer is broken or the
// stream is corrupt; counting it and carrying on would be a quieter way for
// that to disappear than an error is.
func ParseJevCapture(r io.Reader) ([]JevEvaluation, error) {
	var out []JevEvaluation
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	for line := 1; sc.Scan(); line++ {
		raw := strings.TrimSpace(sc.Text())
		if raw == "" {
			continue
		}
		ev, err := jevEvaluation([]byte(raw))
		if err != nil {
			return nil, fmt.Errorf("jev capture line %d: %w", line, err)
		}
		out = append(out, ev)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read jev capture: %w", err)
	}
	return out, nil
}

func jevEvaluation(raw []byte) (JevEvaluation, error) {
	var ev JevEvaluation
	if err := jevRejectDuplicateKeys(raw); err != nil {
		return ev, err
	}
	obj, err := jevObject(raw, "record")
	if err != nil {
		return ev, err
	}
	// The version is read before this version's field set is enforced. A bump
	// is how new fields arrive, so a v2 record carrying a field v1 never heard
	// of is the expected shape of a future producer: blaming the field would
	// send a reader off to delete it, when what is true is that this build does
	// not read v2.
	if ev.ContractVersion, err = jevInt(obj, "contract_version"); err != nil {
		return ev, err
	}
	if ev.ContractVersion != jevContractVersion {
		return ev, fmt.Errorf("unknown contract_version %d, this build reads %d",
			ev.ContractVersion, jevContractVersion)
	}
	if err := jevOnly(obj, "record", "contract_version", "evaluation_id", "requested_model", "attempts"); err != nil {
		return ev, err
	}
	if ev.EvaluationID, err = jevString(obj, "evaluation_id"); err != nil {
		return ev, err
	}
	if err := jevIdentity(ev.EvaluationID); err != nil {
		return ev, err
	}
	if ev.RequestedModel, err = jevString(obj, "requested_model"); err != nil {
		return ev, err
	}

	var items []json.RawMessage
	if err := jevUnmarshal(obj, "attempts", &items); err != nil {
		return ev, err
	}
	if len(items) == 0 {
		return ev, fmt.Errorf("attempts is empty")
	}

	responses := 0
	for i, item := range items {
		a, err := jevAttempt(item, i)
		if err != nil {
			return ev, fmt.Errorf("attempt %d: %w", i, err)
		}
		if a.Response != nil {
			responses++
		}
		ev.Attempts = append(ev.Attempts, a)
	}
	// One invocation has one outcome. Two responses is an invalid artifact,
	// not a question about which of them to prefer.
	if responses > 1 {
		return ev, fmt.Errorf("%d attempts carry a response, at most one may", responses)
	}
	return ev, nil
}

func jevAttempt(raw json.RawMessage, want int) (JevAttempt, error) {
	var a JevAttempt
	obj, err := jevObject(raw, "attempt")
	if err != nil {
		return a, err
	}
	if err := jevOnly(obj, "attempt", "ordinal", "http_status", "provider_request_id", "response"); err != nil {
		return a, err
	}
	if a.Ordinal, err = jevInt(obj, "ordinal"); err != nil {
		return a, err
	}
	// Ordinals are the observation order and are carried explicitly, so a
	// gap or a restart is a producer defect rather than something to infer
	// around.
	if a.Ordinal != want {
		return a, fmt.Errorf("ordinal = %d, want %d: ordinals are 0-based and contiguous", a.Ordinal, want)
	}
	if a.HTTPStatus, err = jevInt(obj, "http_status"); err != nil {
		return a, err
	}

	if v, ok := obj["provider_request_id"]; ok {
		id, err := jevRawString(v, "provider_request_id")
		if err != nil {
			return a, err
		}
		if id == "" {
			return a, fmt.Errorf("provider_request_id is empty; omit it when the header was absent")
		}
		a.ProviderRequestID, a.RequestIDMeasured = id, true
	}

	if v, ok := obj["response"]; ok {
		ro, err := jevObject(v, "response")
		if err != nil {
			return a, err
		}
		resp, err := jevResponse(ro)
		if err != nil {
			return a, fmt.Errorf("response: %w", err)
		}
		a.Response = resp
	}
	return a, nil
}

func jevResponse(obj map[string]json.RawMessage) (*JevResponse, error) {
	if err := jevOnly(obj, "response", "model", "answers", "usage"); err != nil {
		return nil, err
	}
	var r JevResponse
	var err error
	if r.Model, err = jevString(obj, "model"); err != nil {
		return nil, err
	}

	var answers map[string]json.RawMessage
	if err := jevUnmarshal(obj, "answers", &answers); err != nil {
		return nil, err
	}
	if len(answers) == 0 {
		return nil, fmt.Errorf("answers is empty")
	}
	r.Answers = make(map[string]JevAnswer, len(answers))
	for key, item := range answers {
		a, err := jevAnswerOf(item)
		if err != nil {
			return nil, fmt.Errorf("answer %q: %w", key, err)
		}
		r.Answers[key] = a
	}

	usage, err := jevObject(obj["usage"], "usage")
	if err != nil {
		return nil, err
	}
	if err := jevOnly(usage, "usage", "input_tokens", "output_tokens"); err != nil {
		return nil, err
	}
	if r.Usage.InputTokens, err = jevInt(usage, "input_tokens"); err != nil {
		return nil, err
	}
	if r.Usage.OutputTokens, err = jevInt(usage, "output_tokens"); err != nil {
		return nil, err
	}
	return &r, nil
}

func jevAnswerOf(raw json.RawMessage) (JevAnswer, error) {
	obj, err := jevObject(raw, "answer")
	if err != nil {
		return nil, err
	}
	kind, err := jevString(obj, "type")
	if err != nil {
		return nil, err
	}
	switch kind {
	case "noul":
		if err := jevOnly(obj, "noul answer", "type", "noul"); err != nil {
			return nil, err
		}
		n, err := jevFloat(obj, "noul")
		if err != nil {
			return nil, err
		}
		return JevNoul{Noul: n}, nil

	case "choice":
		if err := jevOnly(obj, "choice answer", "type", "choice", "confidence", "probabilities"); err != nil {
			return nil, err
		}
		var c JevChoice
		if c.Choice, err = jevString(obj, "choice"); err != nil {
			return nil, err
		}
		if c.Confidence, err = jevFloat(obj, "confidence"); err != nil {
			return nil, err
		}
		if c.Probabilities, err = jevFloatMap(obj, "probabilities"); err != nil {
			return nil, err
		}
		return c, nil

	case "score":
		if err := jevOnly(obj, "score answer", "type", "score", "confidence", "legend", "probabilities"); err != nil {
			return nil, err
		}
		var s JevScore
		if s.Score, err = jevFloat(obj, "score"); err != nil {
			return nil, err
		}
		if s.Confidence, err = jevFloat(obj, "confidence"); err != nil {
			return nil, err
		}
		if err := jevUnmarshal(obj, "legend", &s.Legend); err != nil {
			return nil, err
		}
		if s.Probabilities, err = jevFloatMap(obj, "probabilities"); err != nil {
			return nil, err
		}
		return s, nil
	}
	return nil, fmt.Errorf("unknown answer type %q", kind)
}

// jevIdentity checks the shape of a Replay-owned identity without reading
// anything out of it. The namespace is what keeps it from being mistaken for
// the provider's own id, which has a different prefix and is not this.
func jevIdentity(id string) error {
	const prefix = "replay_eval_"
	if !strings.HasPrefix(id, prefix) {
		return fmt.Errorf("evaluation_id %q does not carry the %s namespace", id, prefix)
	}
	hex := id[len(prefix):]
	if len(hex) != 32 {
		return fmt.Errorf("evaluation_id has %d hex digits, want 32 for 128 bits", len(hex))
	}
	for _, c := range hex {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return fmt.Errorf("evaluation_id %q is not lowercase hexadecimal", id)
		}
	}
	return nil
}

// jevRejectDuplicateKeys walks the record's token stream.
//
// encoding/json takes the last of two identical keys and says nothing, so
// which of two usage objects the provider meant would be settled by a parser
// default. It is not a thing a default should settle.
func jevRejectDuplicateKeys(raw []byte) error {
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	return jevWalk(dec)
}

func jevWalk(dec *json.Decoder) error {
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("malformed JSON: %w", err)
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := make(map[string]bool)
		for dec.More() {
			kt, err := dec.Token()
			if err != nil {
				return fmt.Errorf("malformed JSON: %w", err)
			}
			key, ok := kt.(string)
			if !ok {
				return fmt.Errorf("malformed JSON: object key is not a string")
			}
			if seen[key] {
				return fmt.Errorf("duplicate JSON key %q", key)
			}
			seen[key] = true
			if err := jevWalk(dec); err != nil {
				return err
			}
		}
	case '[':
		for dec.More() {
			if err := jevWalk(dec); err != nil {
				return err
			}
		}
	}
	if _, err := dec.Token(); err != nil {
		return fmt.Errorf("malformed JSON: %w", err)
	}
	return nil
}

func jevObject(raw json.RawMessage, what string) (map[string]json.RawMessage, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("missing %s", what)
	}
	if string(raw) == "null" {
		return nil, fmt.Errorf("%s is null; omit it when absent", what)
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("%s: %w", what, err)
	}
	return obj, nil
}

// jevOnly refuses a field the contract does not declare. A producer that adds
// one bumps the version, so an unrecognised field at a version this build
// accepts means the two sides disagree about what the version means.
func jevOnly(obj map[string]json.RawMessage, what string, allowed ...string) error {
	ok := make(map[string]bool, len(allowed))
	for _, a := range allowed {
		ok[a] = true
	}
	for k := range obj {
		if !ok[k] {
			return fmt.Errorf("%s carries unknown field %q", what, k)
		}
	}
	return nil
}

func jevField(obj map[string]json.RawMessage, key string) (json.RawMessage, error) {
	v, ok := obj[key]
	if !ok {
		return nil, fmt.Errorf("missing %s", key)
	}
	if string(v) == "null" {
		return nil, fmt.Errorf("%s is null; omit it when absent", key)
	}
	return v, nil
}

func jevString(obj map[string]json.RawMessage, key string) (string, error) {
	v, err := jevField(obj, key)
	if err != nil {
		return "", err
	}
	return jevRawString(v, key)
}

func jevRawString(raw json.RawMessage, key string) (string, error) {
	if string(raw) == "null" {
		return "", fmt.Errorf("%s is null; omit it when absent", key)
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", fmt.Errorf("%s: %w", key, err)
	}
	return s, nil
}

// jevInt refuses a fractional literal where the contract declares an integer.
func jevInt(obj map[string]json.RawMessage, key string) (int, error) {
	v, err := jevField(obj, key)
	if err != nil {
		return 0, err
	}
	if strings.ContainsAny(string(v), ".eE") {
		return 0, fmt.Errorf("%s = %s, want an integer", key, v)
	}
	var n int
	if err := json.Unmarshal(v, &n); err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return n, nil
}

// jevFloat refuses an integer literal where the contract declares a float.
//
// JSON has one number type, so 0 and 0.0 unmarshal into float64 alike. The
// contract declares these fields float64 and the provider writes them that
// way, so a bare integer is a representation this reader has not seen and does
// not carry.
func jevFloat(obj map[string]json.RawMessage, key string) (float64, error) {
	v, err := jevField(obj, key)
	if err != nil {
		return 0, err
	}
	return jevRawFloat(v, key)
}

func jevRawFloat(raw json.RawMessage, key string) (float64, error) {
	if !strings.ContainsAny(string(raw), ".eE") {
		return 0, fmt.Errorf("%s = %s, want a float", key, raw)
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return f, nil
}

func jevFloatMap(obj map[string]json.RawMessage, key string) (map[string]float64, error) {
	v, err := jevField(obj, key)
	if err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(v, &raw); err != nil {
		return nil, fmt.Errorf("%s: %w", key, err)
	}
	out := make(map[string]float64, len(raw))
	for k, item := range raw {
		f, err := jevRawFloat(item, fmt.Sprintf("%s[%s]", key, k))
		if err != nil {
			return nil, err
		}
		out[k] = f
	}
	return out, nil
}

func jevUnmarshal(obj map[string]json.RawMessage, key string, into any) error {
	v, err := jevField(obj, key)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(v, into); err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	return nil
}
