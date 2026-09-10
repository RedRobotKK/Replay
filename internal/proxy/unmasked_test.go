package proxy

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/masking"
)

// The NOT MASKED disclosure.
//
// /v1/chat/completions is read, guarded and ledgered by this build, and the
// masker does not understand its body shape. Before that path became readable
// the NOT PARSED warning covered it; once it became readable that warning
// stopped firing, and noteUnmasked is the only thing left that tells an
// operator running --mask that secrets on this traffic go out in clear.
//
// As of this audit nothing in the suite executed it: Server.noteUnmasked and
// stats.noteUnmasked were both at 0% statement coverage, the call site in
// handle was uncovered, and the replay_unmasked_requests_total accumulator in
// metrics() was never fed a non-zero value. A disclosure nobody has observed
// is exactly the shape ADR-0018 is about — the screen is silent and no test
// can tell silence from a working warning.

// UM1: the warning fires once per path, names the path, and says what is not
// happening.
//
// PASS: exactly one "NOT MASKED" line for three calls on one path, naming the
// path and /v1/messages as the only shape the masker understands.
// FAIL: no line at all (the operator believes --mask covers this traffic), or
// one per request (noise the operator learns to scroll past, which is the same
// as no warning).
func TestUM1_TheUnmaskedWarningFiresOncePerPathAndNamesIt(t *testing.T) {
	var buf bytes.Buffer
	s := &Server{cfg: Config{Logger: log.New(&buf, "", 0)}, stats: newStats()}

	for i := 0; i < 3; i++ {
		s.noteUnmasked("/v1/chat/completions")
	}

	got := buf.String()
	if n := strings.Count(got, "NOT MASKED"); n != 1 {
		t.Fatalf("the unmasked warning must fire exactly once per path, fired %d times:\n%s", n, got)
	}
	for _, want := range []string{"/v1/chat/completions", messagesPath, "clear"} {
		if !strings.Contains(got, want) {
			t.Fatalf("the warning must name the path, the shape that is understood, and the consequence: missing %q in\n%s", want, got)
		}
	}
	if !strings.Contains(s.stats.metrics(), "replay_unmasked_requests_total 3") {
		t.Fatalf("every unmasked request must be counted, not just the announced one:\n%s", s.stats.metrics())
	}
}

// UM2: a second unfamiliar path is its own warning.
//
// PASS: two "NOT MASKED" lines for two distinct paths.
// FAIL: one line, which would mean the first path's warning suppresses every
// later one and a newly-added unmasked route lands silently.
func TestUM2_ASecondUnmaskedPathWarnsAgain(t *testing.T) {
	var buf bytes.Buffer
	s := &Server{cfg: Config{Logger: log.New(&buf, "", 0)}, stats: newStats()}
	s.noteUnmasked("/v1/chat/completions")
	s.noteUnmasked("/openai/v1/chat/completions")
	if n := strings.Count(buf.String(), "NOT MASKED"); n != 2 {
		t.Fatalf("each new unmasked path warns once, got %d:\n%s", n, buf.String())
	}
}

// UM3: the warning crosses the join.
//
// ADR-0018's consequence for two-part fixes: a field nothing reads is the
// built-but-unwired shape, so the guard has to span the boundary. UM1 and UM2
// call noteUnmasked directly and would both stay green if the call site in
// handle were deleted. This one drives a real request through a real proxy
// with a real masker configured.
//
// PASS: a POST to /v1/chat/completions on a proxy with --mask on produces the
// NOT MASKED line and increments replay_unmasked_requests_total.
// FAIL: neither happens, meaning the operator ran --mask over this traffic and
// nothing anywhere said the masker was not running on it.
func TestUM3_ARealOpenAIRequestUnderMaskAnnouncesItIsNotMasked(t *testing.T) {
	up := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"c1","choices":[],"usage":{"prompt_tokens":5,"completion_tokens":1}}`))
	})
	vault, err := masking.OpenVault(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	base, _, logs := startProxyWith(t, up, Config{Masker: masking.New(vault, nil)})

	body := `{"model":"gpt-x","messages":[{"role":"user","content":"hi"}]}`
	req, err := http.NewRequest(http.MethodPost, base+chatCompletionsPath, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	waitFor(t, "the NOT MASKED warning", func() bool {
		return strings.Contains(logs.String(), "NOT MASKED")
	})
	if !strings.Contains(logs.String(), chatCompletionsPath) {
		t.Fatalf("the warning must name the path that went out unmasked:\n%s", logs.String())
	}

	mresp, err := http.Get(base + MetricsPath)
	if err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(mresp.Body)
	_ = mresp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "replay_unmasked_requests_total 1") {
		t.Fatalf("unmasked traffic is invisible on %s:\n%s", MetricsPath, out)
	}
}

// UM4: masked traffic must not trip it, or the counter means nothing.
//
// PASS: a /v1/messages request under the same masker leaves
// replay_unmasked_requests_total at zero.
// FAIL: any non-zero count, which would make the warning fire on traffic that
// IS masked and train the operator to ignore it.
func TestUM4_MaskedTrafficIsNotCountedAsUnmasked(t *testing.T) {
	up := &upstream{t: t}
	vault, err := masking.OpenVault(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	base, _, logs := startProxyWith(t, up, Config{Masker: masking.New(vault, nil)})

	resp := postBody(t, base, requestBody)
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	mresp, err := http.Get(base + MetricsPath)
	if err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(mresp.Body)
	_ = mresp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "replay_unmasked_requests_total 0") {
		t.Fatalf("a path the masker does understand was counted as unmasked:\n%s", out)
	}
	if strings.Contains(logs.String(), "NOT MASKED") {
		t.Fatalf("%s is masked and must not be announced as unmasked:\n%s", messagesPath, logs.String())
	}
}
