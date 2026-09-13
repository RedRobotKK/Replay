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

// The EXPERIMENTAL, UNMASKED disclosure.
//
// /v1/chat/completions is read, guarded and ledgered by this build, and the
// masker does not understand its body shape. Before that path became readable
// the NOT PARSED warning covered it; once it became readable that warning
// stopped firing, and this disclosure is the only thing left that tells an
// operator what they are and are not getting on that traffic.
//
// WHY THE LABEL CHANGED, 2026-09-12.
//
// The line said NOT MASKED and fired only when a masker was configured and no
// policy switch had turned it off. That gate encoded an assumption: that the
// only person who needs the warning is the one who asked for masking. It is
// the wrong assumption twice over.
//
// First, the gap is not only masking. The coverage on this family is real but
// partial, and the partial half is invisible from a running proxy. An operator
// with no masker configured still needs to know which providers this path has
// actually been driven against, because nothing else on their screen tells
// them the Messages path and this one were proven to different depths.
//
// Second, a warning an ordinary flag switches off is a warning that is absent
// exactly when the operator has stopped paying attention. REPLAY_NO_POLICY=1
// clears the mask flag in serve, which nilled the masker, which silenced the
// disclosure. The same shape as the day cap that reset on restart: the
// protection disappears for the case it exists to cover.
//
// WHAT THE LINE MAY NOT SAY, AND WHY THIS IS A COMMENT RATHER THAN A DIFF.
//
// RELEASE-CRITERIA.md stated as fact that this path "has only ever run against
// a test stub". Checked before it was repeated, that is false, and the
// repository holds its own refutation in two places: multi-provider.md records
// a live DeepSeek run on 2026-09-05 across four surfaces that caught a defect
// no stub could produce (prompt_cache_hit_tokens being dropped from RawUsage),
// and ollama-cache-observable-2026-09-09.md records this endpoint driven
// against a local Ollama. internal/ledger/testdata/deepseek holds the captured
// bytes. The surface registry already splits the difference correctly: the
// DeepSeek row is LIVE, the Cursor row is STUB.
//
// So the disclosure says what is true: never against OpenAI itself, and never
// against any OpenAI-compatible provider other than those two. Writing "only
// ever a stub" into the binary would have shipped a false statement under a
// label whose whole purpose is to be believed.

// UM1: the warning fires once per path, names the path, and says what is not
// happening on both counts.
//
// PASS: exactly one "EXPERIMENTAL, UNMASKED" line for three calls on one path,
// naming the path, the consequence that secrets go out in clear, and the
// bound on which providers it has been driven against.
// FAIL: no line at all (the operator believes --mask covers this traffic and
// that the path is as proven as the Messages one), or one per request (noise
// the operator learns to scroll past, which is the same as no warning).
func TestUM1_TheUnmaskedWarningFiresOncePerPathAndNamesIt(t *testing.T) {
	var buf bytes.Buffer
	s := &Server{cfg: Config{Logger: log.New(&buf, "", 0)}, stats: newStats()}

	for i := 0; i < 3; i++ {
		s.noteExperimentalUnmasked("/v1/chat/completions")
	}

	got := buf.String()
	if n := strings.Count(got, "EXPERIMENTAL, UNMASKED"); n != 1 {
		t.Fatalf("the disclosure must fire exactly once per path, fired %d times:\n%s", n, got)
	}
	// The label alone does not tell a reader their API keys are in flight, so
	// each of these is a separate assertion rather than one on the whole line.
	for _, want := range []string{"/v1/chat/completions", "in clear", "never against", messagesPath} {
		if !strings.Contains(got, want) {
			t.Fatalf("the warning must name the path, the consequence, the bound on what it has "+
				"been driven against, and the shape that IS masked: missing %q in\n%s", want, got)
		}
	}
	if !strings.Contains(s.stats.metrics(), "replay_unmasked_requests_total 3") {
		t.Fatalf("every unmasked request must be counted, not just the announced one:\n%s", s.stats.metrics())
	}
}

// UM2: a second unfamiliar path is its own warning.
//
// PASS: two disclosure lines for two distinct paths.
// FAIL: one line, which would mean the first path's warning suppresses every
// later one and a newly-added unmasked route lands silently.
func TestUM2_ASecondUnmaskedPathWarnsAgain(t *testing.T) {
	var buf bytes.Buffer
	s := &Server{cfg: Config{Logger: log.New(&buf, "", 0)}, stats: newStats()}
	s.noteExperimentalUnmasked("/v1/chat/completions")
	s.noteExperimentalUnmasked("/openai/v1/chat/completions")
	if n := strings.Count(buf.String(), "EXPERIMENTAL, UNMASKED"); n != 2 {
		t.Fatalf("each new unmasked path warns once, got %d:\n%s", n, buf.String())
	}
}

// UM3: the warning crosses the join.
//
// ADR-0018's consequence for two-part fixes: a field nothing reads is the
// built-but-unwired shape, so the guard has to span the boundary. UM1 and UM2
// call the method directly and would both stay green if the call site in
// handle were deleted. This one drives a real request through a real proxy
// with a real masker configured.
//
// PASS: a POST to /v1/chat/completions on a proxy with --mask on produces the
// disclosure and increments replay_unmasked_requests_total.
// FAIL: neither happens, meaning the operator ran --mask over this traffic and
// nothing anywhere said the masker was not running on it.
func TestUM3_ARealOpenAIRequestUnderMaskAnnouncesItIsNotMasked(t *testing.T) {
	base, logs := startOpenAIProxy(t, Config{Masker: maskerFor(t)})
	postChatCompletion(t, base)

	waitFor(t, "the EXPERIMENTAL, UNMASKED warning", func() bool {
		return strings.Contains(logs.String(), "EXPERIMENTAL, UNMASKED")
	})
	if !strings.Contains(logs.String(), chatCompletionsPath) {
		t.Fatalf("the warning must name the path that went out unmasked:\n%s", logs.String())
	}
	if out := fetchMetrics(t, base); !strings.Contains(out, "replay_unmasked_requests_total 1") {
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
	base, _, logs := startProxyWith(t, up, Config{Masker: maskerFor(t)})

	resp := postBody(t, base, requestBody)
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	if out := fetchMetrics(t, base); !strings.Contains(out, "replay_unmasked_requests_total 0") {
		t.Fatalf("a path the masker does understand was counted as unmasked:\n%s", out)
	}
	if strings.Contains(logs.String(), "EXPERIMENTAL, UNMASKED") {
		t.Fatalf("%s is masked and verified live, and must not be announced otherwise:\n%s", messagesPath, logs.String())
	}
}

// UM5: the disclosure does not depend on the operator having asked for masking.
//
// This is the half that was missing. An operator who never passes --mask is
// the likeliest person to assume the OpenAI-compatible path is as finished as
// the Messages one, because nothing on their screen distinguishes them. The
// coverage half of the label is theirs whether or not a masker exists, and the
// unmasked half tells them what turning --mask on would and would not buy.
//
// PASS: a POST with no Masker configured still produces the disclosure.
// FAIL: silence, which is the state this whole file exists to end.
func TestUM5_TheDisclosureFiresWithNoMaskerConfigured(t *testing.T) {
	base, logs := startOpenAIProxy(t, Config{})
	postChatCompletion(t, base)

	waitFor(t, "the EXPERIMENTAL, UNMASKED warning without a masker", func() bool {
		return strings.Contains(logs.String(), "EXPERIMENTAL, UNMASKED")
	})
	if !strings.Contains(logs.String(), "never against") {
		t.Fatalf("an operator with no masker still needs the coverage half of the label:\n%s", logs.String())
	}
}

// UM6: no ordinary flag switches the disclosure off.
//
// REPLAY_NO_POLICY=1 is the operator's way to rule Replay's edits out while
// keeping the ledger and the guards, and serve implements it by clearing the
// mask flag. That nilled the masker, and a nil masker silenced the warning, so
// the single switch a cautious operator reaches for removed the disclosure
// that their caution most depends on.
//
// PASS: NoPolicy set, disclosure still printed.
// FAIL: silence under NoPolicy, which makes the label a flag rather than a
// fact.
func TestUM6_NoPolicyDoesNotSuppressTheDisclosure(t *testing.T) {
	base, logs := startOpenAIProxy(t, Config{NoPolicy: true, Masker: maskerFor(t)})
	postChatCompletion(t, base)

	waitFor(t, "the EXPERIMENTAL, UNMASKED warning under NoPolicy", func() bool {
		return strings.Contains(logs.String(), "EXPERIMENTAL, UNMASKED")
	})
}

// maskerFor builds a masker over a throwaway vault.
func maskerFor(t *testing.T) *masking.Masker {
	t.Helper()
	vault, err := masking.OpenVault(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return masking.New(vault, nil)
}

// startOpenAIProxy starts a proxy in front of a stub speaking the OpenAI
// chat-completions shape. The stub is the point: it is the only upstream this
// path has ever been run against, which is half of what the disclosure says.
func startOpenAIProxy(t *testing.T, extra Config) (string, *syncBuffer) {
	t.Helper()
	up := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"c1","choices":[],"usage":{"prompt_tokens":5,"completion_tokens":1}}`))
	})
	base, _, logs := startProxyWith(t, up, extra)
	return base, logs
}

func postChatCompletion(t *testing.T, base string) {
	t.Helper()
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
}

func fetchMetrics(t *testing.T, base string) string {
	t.Helper()
	resp, err := http.Get(base + MetricsPath)
	if err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}
