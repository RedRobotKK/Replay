package probe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Running probes against a real provider.
//
// This is the first thing in Replay that originates a billable request. The
// proxy forwards what an agent already sent; this creates traffic on purpose,
// with the operator's credential, and spends their money to learn something.
// Every constraint below follows from that.
//
//	R1  a plan is shown and nothing is sent until execution is asked for
//	R2  the credential is never written to output, however the run ends
//	R3  the credential comes from the environment, never from a flag
//	R4  each probe is unique, so it can never read an existing cache entry
//	R5  the run stops at the authorised number of requests
//	R6  a provider error ends the run rather than being counted as evidence

// newRunner wires a Runner to a test server.
//
// The counting endpoint is answered here rather than in each test. Sizing the
// probe prefix by measurement means every probe consults it, and a test about
// budgets or credentials should not have to reimplement a tokenizer to say
// what it is about. A test that cares about counting supplies its own handler
// for the path, which this defers to.
func newRunner(t *testing.T, h http.HandlerFunc) (*Runner, *httptest.Server, *bytes.Buffer) {
	t.Helper()
	wrapped := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if strings.HasSuffix(req.URL.Path, "/count_tokens") {
			var body struct {
				System []struct {
					Text string `json:"text"`
				} `json:"system"`
			}
			_ = json.NewDecoder(req.Body).Decode(&body)
			n := 0
			if len(body.System) > 0 {
				n = len(body.System[0].Text) / tokensPerProbeChar
			}
			w.Header().Set("content-type", "application/json")
			_, _ = fmt.Fprintf(w, `{"input_tokens":%d}`, n)
			return
		}
		h(w, req)
	})
	srv := httptest.NewServer(wrapped)
	t.Cleanup(srv.Close)
	var out bytes.Buffer
	return &Runner{
		BaseURL: srv.URL,
		APIKey:  "sk-ant-SECRETVALUE-do-not-print",
		Client:  srv.Client(),
		Out:     &out,
	}, srv, &out
}

// R1: PASS: Plan writes the sizes and the cost, and sends nothing.
// FAIL: a request during planning. Money must not move because someone ran a
// command to find out what it would cost.
func TestR1_PlanSendsNothing(t *testing.T) {
	var requests int
	r, _, out := newRunner(t, func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(200)
	})
	r.Plan(Config{Min: 0, Max: 65536, Resolution: 512, MaxProbes: 12, Confirm: 1}, "claude-opus-5")

	if requests != 0 {
		t.Errorf("planning sent %d request(s); it must send none", requests)
	}
	s := out.String()
	for _, want := range []string{"claude-opus-5", "12", "probe"} {
		if !strings.Contains(s, want) {
			t.Errorf("the plan must mention %q; got:\n%s", want, s)
		}
	}
}

// R2: PASS: the key appears nowhere in output, on success or failure.
// FAIL: a leak. This output is pasted into issues and terminals, and a
// provider key in it is a compromise rather than a bug.
func TestR2_TheCredentialIsNeverPrinted(t *testing.T) {
	// Success path.
	r, _, out := newRunner(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":10,"cache_creation_input_tokens":2048,"cache_read_input_tokens":0}}`))
	})
	_, _ = r.Run(Config{Min: 0, Max: 4096, Resolution: 512, MaxProbes: 6, Confirm: 1}, "claude-opus-5")
	if strings.Contains(out.String(), "SECRETVALUE") {
		t.Error("the credential appeared in successful output")
	}

	// Failure path: an error message is where a key most often leaks, because
	// the request gets dumped.
	r2, _, out2 := newRunner(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(401)
		_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
	})
	_, err := r2.Run(Config{Min: 0, Max: 4096, Resolution: 512, MaxProbes: 6, Confirm: 1}, "claude-opus-5")
	if err == nil {
		t.Fatal("a 401 must end the run")
	}
	if strings.Contains(out2.String()+err.Error(), "SECRETVALUE") {
		t.Error("the credential appeared in an error")
	}
}

// R3: PASS: the key is taken from the environment only.
// FAIL: a flag. A credential on a command line lands in shell history and in
// the process table, where every other user on the box can read it.
//
// This test used to grep run.go for three literal strings: `flag.String("api-key`,
// `flag.String("key` and `--api-key`. A denylist of names cannot see a name
// nobody put on it. Adding
//
//	var credentialFlag = flag.String("token", "", "provider API key")
//
// to run.go, with
//
//	if *credentialFlag != "" {
//		r.APIKey = *credentialFlag
//	}
//
// at the top of Run, compiled, answered to `go test -token=sk-ant-...` — so
// the key really was on the command line and really did reach the field — and
// left this test PASSING. The property was never "no flag is called api-key".
// It is "no flag reaches the credential field", which is a question about the
// syntax tree rather than about the bytes of the file.
//
// So the check parses. Every flag definition binds a name, every name that
// takes its value from one is tainted, and any write into Runner.APIKey fails
// whatever the flag is called — an assignment, a struct literal, a pointer
// handed to StringVar, or an assignment inside a flag's callback.
//
// It reads two directories: this package, which owns the field, and
// cmd/replay, which is the only place that fills it. Guarding this package
// alone watches files that have no flags in them and never will; the
// realistic leak is a flag in the command, wired into the Runner literal.
func TestR3_TheCredentialComesFromTheEnvironment(t *testing.T) {
	defs := 0
	for _, dir := range []string{".", filepath.Join("..", "..", "cmd", "replay")} {
		found, n := scanForCredentialFlags(t, dir)
		defs += n
		for _, f := range found {
			t.Errorf("a credential flag puts the key in shell history and the process table: %s", f)
		}
	}
	// The scan has to be able to see a flag at all. cmd/replay defines dozens
	// of them; finding none means the walk missed the files, or the detector
	// stopped matching how flags are written, either of which makes every pass
	// above vacuous.
	if defs == 0 {
		t.Fatal("no flag definitions found in either directory; this check is reading nothing")
	}

	// And it has to be able to fail. The first fixture is the mutation that
	// survived the denylist, plus the two shapes it did not use: a pointer
	// straight to the field, and a callback that assigns to it.
	leaked, leakyDefs := credentialFlagsIn(t, "mutation.go", credentialFlagMutation)
	if len(leaked) != 3 || leakyDefs != 3 {
		t.Errorf("the check saw %d of the 3 credential flags in the mutation fixture, from %d of its 3 flag definitions: %v",
			len(leaked), leakyDefs, leaked)
	}
	// A command that takes flags and reads the key from the environment must
	// come back clean, or the check is not discriminating between them, it is
	// only failing.
	clean, cleanDefs := credentialFlagsIn(t, "clean.go", flagsThatAreNotCredentials)
	if len(clean) != 0 || cleanDefs != 2 {
		t.Errorf("a flag that never touches the credential was reported (%v), or its 2 definitions counted as %d",
			clean, cleanDefs)
	}
	// A file that writes the field and declares no flag at all is the other
	// half of that: the flag is what makes the write a defect, so a count of
	// zero here is what stops this check from condemning `APIKey:
	// os.Getenv(…)` everywhere it appears.
	envOnly, envDefs := credentialFlagsIn(t, "env.go", credentialFromTheEnvironment)
	if len(envOnly) != 0 || envDefs != 0 {
		t.Errorf("a credential read from the environment was reported (%v), or %d flags were counted in a file with none",
			envOnly, envDefs)
	}
}

// R4: PASS: no two probes send the same prefix content.
// FAIL: repeated content, which caches on the first probe and is then READ by
// every later one — and a read tests nothing, so the whole run learns nothing
// while costing full price.
func TestR4_EveryProbeIsUnique(t *testing.T) {
	seen := map[string]int{}
	r, _, _ := newRunner(t, func(w http.ResponseWriter, req *http.Request) {
		var body bytes.Buffer
		_, _ = body.ReadFrom(req.Body)
		seen[body.String()]++
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":10,"cache_creation_input_tokens":1024,"cache_read_input_tokens":0}}`))
	})
	_, _ = r.Run(Config{Min: 0, Max: 65536, Resolution: 256, MaxProbes: 12, Confirm: 1}, "claude-opus-5")

	if len(seen) < 2 {
		t.Fatalf("only %d distinct bodies sent; the run did not probe", len(seen))
	}
	for body, n := range seen {
		if n > 1 {
			t.Errorf("a probe body was sent %d times; the second would read the first's cache entry and test nothing (%.40s)", n, body)
		}
	}
}

// R5: PASS: never more requests than authorised.
// FAIL: one over. This spends the operator's money, and a cap that is
// approximately respected is not a cap.
func TestR5_StopsAtTheAuthorisedRequestCount(t *testing.T) {
	var requests int
	r, _, _ := newRunner(t, func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":10,"cache_creation_input_tokens":1024,"cache_read_input_tokens":0}}`))
	})
	_, _ = r.Run(Config{Min: 0, Max: 1 << 20, Resolution: 1, MaxProbes: 5, Confirm: 1}, "claude-opus-5")
	if requests > 5 {
		t.Errorf("sent %d requests against an authorised 5", requests)
	}
}

// R6: PASS: a non-200 ends the run and is not recorded as evidence.
// FAIL: treating an error body's absent usage as "cached nothing", which would
// push the reported floor above a size that was never actually tested.
func TestR6_AProviderErrorIsNotEvidence(t *testing.T) {
	calls := 0
	r, _, _ := newRunner(t, func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 2 {
			w.WriteHeader(529)
			_, _ = w.Write([]byte(`{"error":"overloaded"}`))
			return
		}
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":10,"cache_creation_input_tokens":1024,"cache_read_input_tokens":0}}`))
	})
	s, err := r.Run(Config{Min: 0, Max: 65536, Resolution: 64, MaxProbes: 20, Confirm: 1}, "claude-opus-5")
	if err == nil {
		t.Fatal("a provider error must end the run")
	}
	if s != nil && s.Probes() > 1 {
		t.Errorf("recorded %d decisions; the errored request must not be one", s.Probes())
	}
}

// R7: inconclusive probes consume budget, and only the runner's cap stops them.
//
// A read does not advance the search, so Search's own MaxProbes never trips: it
// counts decisions, and an inconclusive probe decides nothing. A provider that
// keeps answering with reads would loop forever, paying full price each time.
// This is the case where the runner's request cap is the only thing standing
// between an operator and unbounded spend — a mutation removing it survived
// until this test existed, because every other test reached a decision.
//
// PASS: the run stops at the authorised request count.
// FAIL: any number above it, and in the real world an unbounded bill.
func TestR7_InconclusiveProbesStillConsumeBudget(t *testing.T) {
	var requests int
	r, _, _ := newRunner(t, func(w http.ResponseWriter, _ *http.Request) {
		requests++
		if requests > 50 {
			// Fail fast rather than hang. Without the runner's cap this loop
			// is infinite — the search never advances on a read — and a
			// mutation removing the cap would otherwise run until the test
			// timeout, which is a worse failure than a red test.
			w.WriteHeader(500)
			return
		}
		w.Header().Set("content-type", "application/json")
		// Always a read: found an existing entry, decided nothing.
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":10,"cache_creation_input_tokens":512,"cache_read_input_tokens":4096}}`))
	})
	s, _ := r.Run(Config{Min: 0, Max: 65536, Resolution: 1, MaxProbes: 4, Confirm: 1}, "claude-opus-5")
	if requests > 4 {
		t.Errorf("sent %d requests against an authorised 4; inconclusive probes must still spend budget", requests)
	}
	if s.Probes() != 0 {
		t.Errorf("recorded %d decisions from probes that all read; none of them decided anything", s.Probes())
	}
}

// R8: a transport failure cannot leak the credential either.
//
// The 401 path in R2 covers a provider that answered. This covers one that did
// not: connection refused, TLS failure, timeout. It is the path where a naive
// implementation wraps the *error*, and Go's http errors carry the request URL
// — and where a hand-written message is most likely to helpfully include the
// key that was being tried.
//
// PASS: no credential in the error or the output.
// FAIL: a leak into text that reaches terminals and issue trackers.
func TestR8_ATransportFailureDoesNotLeakTheCredential(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // Nothing is listening now.

	var out bytes.Buffer
	r := &Runner{BaseURL: url, APIKey: "sk-ant-SECRETVALUE-do-not-print", Client: srv.Client(), Out: &out}
	_, err := r.Run(Config{Min: 0, Max: 4096, Resolution: 512, MaxProbes: 4, Confirm: 1}, "claude-opus-5")
	if err == nil {
		t.Fatal("an unreachable provider must end the run")
	}
	if strings.Contains(err.Error()+out.String(), "SECRETVALUE") {
		t.Errorf("the credential leaked into a transport error: %v", err)
	}
}

// R9: the prefix is sized by measurement, not by an approximation.
//
// The first live runs stalled: the search asks for n tokens built from a
// chars-per-token guess, the prefix becomes some other size, and the upper
// bound can never fall below whatever it actually became. Four models stalled
// at brackets ~170 tokens wide and a budget of 40 probes changed nothing,
// because the budget was never the constraint.
//
// The provider will count tokens for us. Sizing the filler against that closes
// the gap, and both bounds become real tokens rather than one measured and one
// guessed.
//
// PASS: the built prefix counts within tolerance of the size requested, and
// the counting endpoint is consulted.
// FAIL: shipping a prefix whose size nobody checked, which is what made the
// brackets un-narrowable.
func TestR9_ThePrefixIsSizedByCounting(t *testing.T) {
	var counted, sent int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("content-type", "application/json")
		if strings.HasSuffix(req.URL.Path, "/count_tokens") {
			counted++
			var body struct {
				System []struct{ Text string } `json:"system"`
			}
			_ = json.NewDecoder(req.Body).Decode(&body)
			// A deliberately unhelpful ratio: 7 characters per token, so an
			// implementation assuming 4 lands nowhere near the target.
			n := 0
			if len(body.System) > 0 {
				n = len(body.System[0].Text) / 7
			}
			_, _ = fmt.Fprintf(w, `{"input_tokens":%d}`, n)
			return
		}
		sent++
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":10,"cache_creation_input_tokens":1000,"cache_read_input_tokens":0}}`))
	}))
	t.Cleanup(srv.Close)

	r := &Runner{BaseURL: srv.URL, APIKey: "k", Client: srv.Client(), Out: io.Discard}
	got, _, err := r.sizedFiller("claude-opus-5", 1000)
	if err != nil {
		t.Fatalf("sizing failed: %v", err)
	}
	if counted == 0 {
		t.Fatal("the provider's token count was never consulted; the size is still a guess")
	}
	// Within 2% of the target, at a ratio the caller did not know in advance.
	if n := len(got) / 7; n < 980 || n > 1020 {
		t.Errorf("built a prefix of about %d tokens, want ~1000; the sizing did not converge", n)
	}
}

// R10: sizes are reported as the cacheable prefix, not the whole request.
//
// The breakpoint sits on the system block, so the prefix that has to clear the
// provider's minimum is the system block alone — but a token count covers the
// whole request, including the user message and the envelope. On this API that
// is 7 tokens, and it is the difference between a result that reads as a
// contradiction and one that confirms the documentation to the token: a
// 519-token request has a 512-token prefix, which is exactly the documented
// minimum for opus-5, and it caches, while a 512-token request has a 505-token
// prefix and does not.
//
// PASS: the run measures the overhead once and reports prefix sizes net of it.
// FAIL: reporting request sizes against a documented prefix minimum, which
// compares two different quantities and makes every model look wrong by a
// constant.
func TestR10_SizesAreTheCacheablePrefix(t *testing.T) {
	const overhead = 7
	var probed []int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("content-type", "application/json")
		var body struct {
			System []struct {
				Text string `json:"text"`
			} `json:"system"`
		}
		_ = json.NewDecoder(req.Body).Decode(&body)
		sysTokens := 0
		if len(body.System) > 0 {
			sysTokens = len(body.System[0].Text) / tokensPerProbeChar
		}
		if strings.HasSuffix(req.URL.Path, "/count_tokens") {
			// The whole request, as the real endpoint reports it.
			_, _ = fmt.Fprintf(w, `{"input_tokens":%d}`, sysTokens+overhead)
			return
		}
		probed = append(probed, sysTokens)
		_, _ = fmt.Fprintf(w, `{"usage":{"input_tokens":10,"cache_creation_input_tokens":%d,"cache_read_input_tokens":0}}`, sysTokens)
	}))
	t.Cleanup(srv.Close)

	r := &Runner{BaseURL: srv.URL, APIKey: "k", Client: srv.Client(), Out: io.Discard}
	s, err := r.Run(Config{Min: 0, Max: 2048, Resolution: 64, MaxProbes: 20, Confirm: 1}, "claude-opus-5")
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if len(probed) == 0 {
		t.Fatal("nothing was probed")
	}
	// Every probe's system block must be sized to the target, not to the
	// target minus the overhead.
	_, hi := s.Bracket()
	if hi > 2048 {
		t.Errorf("upper bound %d exceeds the search range; sizes are not net of overhead", hi)
	}
	if r.Overhead() != overhead {
		t.Errorf("overhead measured as %d, want %d", r.Overhead(), overhead)
	}
}

// R11: the token ratio is learned once, then used as arithmetic.
//
// Sizing was a search: build a prefix, count it, adjust, repeat — up to forty
// counting round trips per probe. That was necessary while the filler was
// English, where a character is a blunt and irregular dial. It is not
// necessary now. Varied CJK measured at exactly 2.00 tokens per rune on this
// API, perfectly linear across 200, 201 and 202 runes, so the rune count for a
// target is a division rather than a search.
//
// The ratio is still measured rather than assumed, because a different model
// or a future tokenizer may not be 2.00 — it is learned from the first probe
// and reused, with a verification count on every probe to catch it drifting.
//
// PASS: after the first sizing, later ones cost a small constant number of
// counting calls.
// FAIL: re-searching every time, which is latency nobody needs and round trips
// nobody is paying for information they already have.
func TestR11_TheRatioIsLearnedOnceThenApplied(t *testing.T) {
	var counts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("content-type", "application/json")
		var body struct {
			System []struct {
				Text string `json:"text"`
			} `json:"system"`
		}
		_ = json.NewDecoder(req.Body).Decode(&body)
		runes := 0
		if len(body.System) > 0 {
			runes = len([]rune(body.System[0].Text))
		}
		if strings.HasSuffix(req.URL.Path, "/count_tokens") {
			counts++
			// Exactly two tokens per rune, plus the envelope.
			_, _ = fmt.Fprintf(w, `{"input_tokens":%d}`, runes*2+7)
			return
		}
		_, _ = fmt.Fprintf(w, `{"usage":{"input_tokens":10,"cache_creation_input_tokens":%d,"cache_read_input_tokens":0}}`, runes*2)
	}))
	t.Cleanup(srv.Close)

	r := &Runner{BaseURL: srv.URL, APIKey: "k", Client: srv.Client(), Out: io.Discard}
	if _, _, err := r.sizedFiller("m", 1000); err != nil {
		t.Fatalf("first sizing failed: %v", err)
	}
	afterFirst := counts

	for _, target := range []int{800, 600, 500, 450} {
		if _, _, err := r.sizedFiller("m", target); err != nil {
			t.Fatalf("sizing %d failed: %v", target, err)
		}
	}
	perProbe := float64(counts-afterFirst) / 4
	if perProbe > 3 {
		t.Errorf("%.1f counting calls per probe after the ratio is known; it should be arithmetic plus a check", perProbe)
	}
}

// R12: a response without usage is inconclusive, never "not cached".
//
// Found by a red-team review. `Wrote` was `CacheCreation > 0` with no check
// that usage was present at all, so a 200 with a reshaped or missing usage
// object read as "this prefix did not cache" — and every such failure pushes
// the lower bound UP, which is precisely the direction that manufactures a
// confirmation of a documented figure. A run against a stub returning 200 with
// no usage reported "floor above 61490" with no error and no caveat.
//
// The nested shape matters too: this API reports cache writes split by TTL in
// `cache_creation.ephemeral_5m_input_tokens` / `_1h_`, and this repository
// parses that everywhere except here.
//
// PASS: a missing usage object ends the run; a nested-only write counts as a
// write.
// FAIL: silence read as evidence, in the direction that flatters the claim.
func TestR12_MissingUsageIsNotEvidence(t *testing.T) {
	r, _, _ := newRunner(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("content-type", "application/json")
		// A well-formed message with no usage at all.
		_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","content":[]}`))
	})
	s, err := r.Run(Config{Min: 0, Max: 65536, Resolution: 64, MaxProbes: 6, Confirm: 1}, "m")
	if err == nil {
		t.Error("a response with no usage must end the run, not be read as 'did not cache'")
	}
	if s != nil {
		if lo, _ := s.Bracket(); lo != 0 {
			t.Errorf("lower bound moved to %d on a response that reported nothing", lo)
		}
	}
}

func TestR12b_ANestedTTLWriteCountsAsAWrite(t *testing.T) {
	// The write is reported only in the per-TTL breakdown, which is a shape
	// this API really uses.
	r, _, _ := newRunner(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":10,"cache_creation_input_tokens":0,
			"cache_creation":{"ephemeral_5m_input_tokens":900,"ephemeral_1h_input_tokens":124},
			"cache_read_input_tokens":0}}`))
	})
	s, err := r.Run(Config{Min: 0, Max: 4096, Resolution: 256, MaxProbes: 8, Confirm: 1}, "m")
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	_, hi := s.Bracket()
	if hi > 1024 {
		t.Errorf("upper bound %d; a nested TTL write of 900+124 was not counted as a write", hi)
	}
}

// R13: a run cut short by the request cap says so.
//
// StoppedEarly is set from the DECISION count, and an inconclusive probe
// consumes a request without deciding anything. So a provider answering every
// request with a cache read burns the whole budget and the run reports a full
// bracket with no caveat at all.
//
// PASS: the caveat is set when requests ran out.
// FAIL: a bracket the run never established, presented without qualification.
func TestR13_ARunCutShortByRequestsSaysSo(t *testing.T) {
	calls := 0
	r, _, _ := newRunner(t, func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("content-type", "application/json")
		if calls > 40 {
			w.WriteHeader(500)
			return
		}
		// Always a read: consumes a request, decides nothing.
		_, _ = w.Write([]byte(`{"usage":{"input_tokens":10,"cache_creation_input_tokens":512,"cache_read_input_tokens":4096}}`))
	})
	s, _ := r.Run(Config{Min: 0, Max: 65536, Resolution: 64, MaxProbes: 5, Confirm: 1}, "m")
	if s == nil {
		t.Fatal("the search must be returned even when the budget runs out")
	}
	if !s.StoppedEarly() {
		t.Error("a run that spent its whole request budget without deciding anything must be flagged; " +
			"otherwise it reports the full range as a measured bracket")
	}
}

// R14: what answered is recorded, not just what was asked.
//
// A run measured `claude-opus-5`, which is an alias. Nothing recorded which
// snapshot it resolved to, which tier served it, or from which geography — so
// the measurement is not reproducible and cannot be checked against a routing
// or tier difference. The API returns all three and the probe discarded them.
//
// This is the difference between a number and a record. A dated series only
// has value if each entry says what it measured.
//
// PASS: the resolved model, service tier and geography are captured from the
// response and reported.
// FAIL: publishing a floor against an alias, with no way to tell later whether
// two readings measured the same thing.
func TestR14_ProvenanceIsCaptured(t *testing.T) {
	r, _, _ := newRunner(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("content-type", "application/json")
		_, _ = w.Write([]byte(`{"model":"claude-opus-5-20261101",
			"usage":{"input_tokens":10,"cache_creation_input_tokens":1024,"cache_read_input_tokens":0,
			         "service_tier":"standard","inference_geo":"us"}}`))
	})
	if _, err := r.Run(Config{Min: 0, Max: 4096, Resolution: 256, MaxProbes: 8, Confirm: 1}, "claude-opus-5"); err != nil {
		t.Fatalf("run failed: %v", err)
	}
	p := r.Provenance()
	if p.ResolvedModel != "claude-opus-5-20261101" {
		t.Errorf("resolved model = %q, want the snapshot the alias answered as", p.ResolvedModel)
	}
	if p.ServiceTier != "standard" {
		t.Errorf("service tier = %q, want standard", p.ServiceTier)
	}
	if p.Geo != "us" {
		t.Errorf("geo = %q, want us", p.Geo)
	}
}

// R15: a run served by more than one snapshot or tier says so.
//
// PASS: mixed provenance is reported rather than silently taking the last.
// FAIL: a single value implying one thing answered throughout, when a routing
// change mid-run means the bracket has two different subjects in it.
func TestR15_MixedProvenanceIsReported(t *testing.T) {
	n := 0
	r, _, _ := newRunner(t, func(w http.ResponseWriter, _ *http.Request) {
		n++
		w.Header().Set("content-type", "application/json")
		// Switch after the first response: the run below stalls quickly
		// because the reported cached size is constant, so a later switch
		// would never be reached.
		model := "claude-opus-5-20261101"
		if n > 1 {
			model = "claude-opus-5-20261201"
		}
		_, _ = fmt.Fprintf(w, `{"model":%q,"usage":{"input_tokens":10,"cache_creation_input_tokens":%d,"cache_read_input_tokens":0,"service_tier":"standard"}}`, model, 4096/n)
	})
	_, _ = r.Run(Config{Min: 0, Max: 65536, Resolution: 64, MaxProbes: 12, Confirm: 1}, "claude-opus-5")
	if !r.Provenance().Mixed {
		t.Error("two snapshots answered one run; a reading that does not say so implies a single subject it did not have")
	}
}

// The machinery behind R3.
//
// It is kept here rather than beside the test so the numbered narrative above
// reads without interruption; nothing else in the package uses it.

// credentialField is the field on Runner that holds the provider key.
const credentialField = "APIKey"

// The mutation that passed the literal denylist, kept verbatim as a fixture so
// the check is tested against the thing it was written for, and extended with
// the two other ways a flag can reach a field.
const credentialFlagMutation = `package probe

import "flag"

var credentialFlag = flag.String("token", "", "provider API key")

func (r *Runner) Run() {
	if *credentialFlag != "" {
		r.APIKey = *credentialFlag
	}
}

func register(fs *flag.FlagSet, r *Runner) {
	fs.StringVar(&r.APIKey, "secret", "", "provider API key")
	fs.Func("pass", "provider API key", func(s string) error {
		r.APIKey = s
		return nil
	})
}
`

// A command that takes flags and still reads the key from the environment.
// Without this, a check that reported every flag would look identical to a
// check that reported the right ones.
const flagsThatAreNotCredentials = `package main

import (
	"flag"
	"os"
)

func run(fs *flag.FlagSet) *Runner {
	model := fs.String("model", "", "model id to measure")
	max := fs.Int("max", 65536, "largest prefix size the floor could be")
	_ = max
	return &Runner{BaseURL: *model, APIKey: os.Getenv("ANTHROPIC_API_KEY")}
}
`

// A file that writes the credential and takes no flag. What R2 and the
// command already do, and what this check must not condemn.
const credentialFromTheEnvironment = `package main

import "os"

func run() *Runner {
	return &Runner{BaseURL: os.Getenv("ANTHROPIC_BASE_URL"), APIKey: os.Getenv("ANTHROPIC_API_KEY")}
}
`

// flagDefiners are the methods that define a command-line flag, on the flag
// package and on any *flag.FlagSet. The receiver is deliberately not checked:
// a FlagSet can be held in a variable, a field or a parameter, and a guard
// that only recognises `flag.String` is the same denylist in another form.
// What is checked is the shape of the call, below.
var flagDefiners = map[string]bool{
	"Bool": true, "BoolFunc": true, "BoolVar": true,
	"Duration": true, "DurationVar": true,
	"Float64": true, "Float64Var": true,
	"Func": true,
	"Int":  true, "IntVar": true,
	"Int64": true, "Int64Var": true,
	"String": true, "StringVar": true,
	"TextVar": true,
	"Uint":    true, "UintVar": true,
	"Uint64": true, "Uint64Var": true,
	"Var": true,
}

// isFlagDefinition reports whether a call declares a flag.
//
// Two shapes, and both need a flag name as a string literal, which is what
// separates `fs.String("model", "", "…")` from `buf.String()`:
//
//	x := fs.String("model", "", "usage")   // returns the value
//	fs.StringVar(&x, "model", "", "usage") // fills it through a pointer
func isFlagDefinition(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || !flagDefiners[sel.Sel.Name] {
		return false
	}
	if len(call.Args) < 2 {
		return false
	}
	if isStringLit(call.Args[0]) {
		return true
	}
	// The Var forms take the destination first and the name second.
	return isAddressOf(call.Args[0]) && isStringLit(call.Args[1])
}

func isStringLit(e ast.Expr) bool {
	lit, ok := e.(*ast.BasicLit)
	return ok && lit.Kind == token.STRING
}

func isAddressOf(e ast.Expr) bool {
	u, ok := e.(*ast.UnaryExpr)
	return ok && u.Op == token.AND
}

// credWrite is one place the credential field is written or handed out by
// address. val is the expression written, when there is one to trace.
type credWrite struct {
	pos token.Pos
	val ast.Expr
}

// credentialWrites finds every write into the credential field under n.
func credentialWrites(n ast.Node) []credWrite {
	var out []credWrite
	ast.Inspect(n, func(node ast.Node) bool {
		switch x := node.(type) {
		case *ast.AssignStmt:
			for i, lhs := range x.Lhs {
				sel, ok := lhs.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != credentialField {
					continue
				}
				w := credWrite{pos: sel.Pos()}
				if len(x.Rhs) == len(x.Lhs) {
					w.val = x.Rhs[i]
				}
				out = append(out, w)
			}
		case *ast.CompositeLit:
			for _, el := range x.Elts {
				kv, ok := el.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				if k, ok := kv.Key.(*ast.Ident); ok && k.Name == credentialField {
					out = append(out, credWrite{pos: kv.Pos(), val: kv.Value})
				}
			}
		case *ast.UnaryExpr:
			// &r.APIKey: whoever holds this pointer writes the field, and
			// StringVar is exactly such a holder.
			if x.Op != token.AND {
				return true
			}
			if sel, ok := x.X.(*ast.SelectorExpr); ok && sel.Sel.Name == credentialField {
				out = append(out, credWrite{pos: x.Pos()})
			}
		}
		return true
	})
	return out
}

// flagFed reports the writes into the credential field that a flag feeds, and
// how many flag definitions the file contains.
func flagFed(fset *token.FileSet, f *ast.File) ([]string, int) {
	defs := 0
	tainted := map[string]bool{}
	type span struct{ from, to token.Pos }
	var flagCalls []span

	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !isFlagDefinition(call) {
			return true
		}
		defs++
		flagCalls = append(flagCalls, span{call.Pos(), call.End()})
		// The Var forms name their destination.
		if isAddressOf(call.Args[0]) {
			if id, ok := call.Args[0].(*ast.UnaryExpr).X.(*ast.Ident); ok {
				tainted[id.Name] = true
			}
		}
		return true
	})

	// Names that take their value from a flag, transitively: `key := *tok`
	// carries the credential exactly as far as `*tok` did.
	for changed := true; changed; {
		changed = false
		ast.Inspect(f, func(n ast.Node) bool {
			var lhs, rhs []ast.Expr
			switch x := n.(type) {
			case *ast.AssignStmt:
				lhs, rhs = x.Lhs, x.Rhs
			case *ast.ValueSpec:
				for _, name := range x.Names {
					lhs = append(lhs, name)
				}
				rhs = x.Values
			default:
				return true
			}
			if !carriesFlagValue(rhs, tainted) {
				return true
			}
			for _, l := range lhs {
				id, ok := l.(*ast.Ident)
				if !ok || id.Name == "_" || tainted[id.Name] {
					continue
				}
				tainted[id.Name] = true
				changed = true
			}
			return true
		})
	}

	var found []string
	for _, w := range credentialWrites(f) {
		inFlagCall := false
		for _, s := range flagCalls {
			if w.pos >= s.from && w.pos < s.to {
				inFlagCall = true
				break
			}
		}
		if !inFlagCall && (w.val == nil || !carriesFlagValue([]ast.Expr{w.val}, tainted)) {
			continue
		}
		found = append(found, fset.Position(w.pos).String()+": "+credentialField+" is written from a flag")
	}
	return found, defs
}

// carriesFlagValue reports whether any of these expressions mentions a
// flag-bound name or declares a flag itself.
func carriesFlagValue(exprs []ast.Expr, tainted map[string]bool) bool {
	hit := false
	for _, e := range exprs {
		ast.Inspect(e, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.Ident:
				if tainted[x.Name] {
					hit = true
				}
			case *ast.CallExpr:
				if isFlagDefinition(x) {
					hit = true
				}
			}
			return !hit
		})
	}
	return hit
}

// scanForCredentialFlags runs the check over every non-test Go file in dir.
func scanForCredentialFlags(t *testing.T, dir string) ([]string, int) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	var found []string
	defs := 0
	fset := token.NewFileSet()
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", path, err)
		}
		hits, n := flagFed(fset, f)
		found = append(found, hits...)
		defs += n
	}
	return found, defs
}

// credentialFlagsIn runs the check over source held in the test itself, and
// returns what it found along with the number of flag definitions it saw.
func credentialFlagsIn(t *testing.T, name, src string) ([]string, int) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		t.Fatalf("parsing the %s fixture: %v", name, err)
	}
	return flagFed(fset, f)
}
