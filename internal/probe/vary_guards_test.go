package probe

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
)

// The conditionals in vary.go, made observable.
//
// guard-reachability reported twenty-one of them introduced and unobserved by
// any test. Each one below is reached by an input, not by a mock of the thing
// being tested: the failures are produced by a server that actually behaves
// that way, because a guard proved with a stub of its own subject proves the
// stub.

// varyServer is a provider that answers token counts and messages, and can be
// told to fail a chosen message request. Counting requests separately from
// message requests matters: sizedFiller spends several counts before the
// experiment sends anything, so "the second request" means the second MESSAGE.
type varyServer struct {
	failOn   int // 1-based message index to fail, 0 for none
	status   int // status to fail with
	messages int
	bodies   []map[string]any
}

func (v *varyServer) start(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		if strings.Contains(r.URL.Path, "count_tokens") {
			w.Header().Set("content-type", "application/json")
			_, _ = fmt.Fprintf(w, `{"input_tokens":%d}`, 7+len(raw)/2)
			return
		}
		v.messages++
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		v.bodies = append(v.bodies, body)
		if v.failOn == v.messages {
			st := v.status
			if st == 0 {
				st = http.StatusBadGateway
			}
			w.WriteHeader(st)
			_, _ = w.Write([]byte(`{"error":"planned"}`))
			return
		}
		// First message writes, later ones read.
		read, create := 4096, 0
		if v.messages == 1 {
			read, create = 0, 4096
		}
		w.Header().Set("content-type", "application/json")
		_, _ = fmt.Fprintf(w,
			`{"id":"m","usage":{"input_tokens":5,"cache_creation_input_tokens":%d,"cache_read_input_tokens":%d,"output_tokens":1}}`,
			create, read)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func runnerFor(srv *httptest.Server) *Runner {
	return &Runner{BaseURL: srv.URL, APIKey: "k", Client: srv.Client(), Out: io.Discard}
}

// An unknown term is refused before anything is sent, not after.
//
// The order is the assertion. A term check that runs after the first request
// has already spent a billable request to reject an argument.
func TestVaryGuard_UnknownTermIsRefusedBeforeAnyRequest(t *testing.T) {
	v := &varyServer{}
	srv := v.start(t)
	if _, err := runnerFor(srv).Vary("claude-opus-5", "not-a-term"); err == nil {
		t.Fatal("an unknown term must be refused")
	} else if !strings.Contains(err.Error(), "unknown vary term") {
		t.Fatalf("wrong error: %v", err)
	}
	if v.messages != 0 {
		t.Fatalf("refusing an unknown term sent %d billable requests; it must send none", v.messages)
	}
}

// A documented floor above the default is what the guard is for.
//
// Two earlier versions of this test were wrong, and both ways are worth
// recording because the reviewer caught what the test did not.
//
// The first compared two model names and asserted the floored one built MORE
// filler. It failed: opus-5's published minimum is 512, so following it targets
// 768 — less than the fixed 2048 a model with no entry gets.
//
// The second asserted only that the two differed, and PASSED WITH THE GUARD
// DELETED. The fake provider counts tokens as len(body)/2, and the two model
// names are different lengths, so the bodies differed whatever the target was.
// It was measuring the model name. guard-reachability still called the guard
// INERT and it was right.
//
// The guard only changes an outcome when the published floor is ABOVE the 2048
// default: then, without it, the prefix is built too small and `actual < floor`
// below refuses the whole experiment. So that is the fixture — a rules document
// declaring a floor of 8192 — and it makes both conditionals load-bearing at
// once, which is the honest reason they were unobserved: nothing in the
// compiled table documents a floor that high.
func TestVaryGuard_AFloorAboveTheDefaultIsFollowedNotIgnored(t *testing.T) {
	restore := cachemodel.Override(&cachemodel.Rules{
		Schema:   "replay.rules/v1",
		Version:  "test-high-floor",
		Provider: "test",
		Models: []cachemodel.ModelRule{{
			Match:        "high-floor-model",
			MinPrefix:    8192,
			InputPerMTok: 1,
			ReadMult:     0.1,
			Priced:       true,
		}},
	})
	defer restore()

	// The premise: this really is a floor above the default target.
	if got := cachemodel.DocumentedMinPrefix("high-floor-model"); got <= 2048 {
		t.Fatalf("the fixture floor is %d, not above the 2048 default; the guard cannot "+
			"change an outcome and this test could not fail", got)
	}

	v := &varyServer{}
	srv := v.start(t)
	if _, err := runnerFor(srv).Vary("high-floor-model", VaryTools); err != nil {
		t.Fatalf("a model whose published floor is 8192 must have its prefix built to clear "+
			"that floor; instead the run refused itself: %v", err)
	}
}

// The token counter failing stops the run rather than guessing a size.
func TestVaryGuard_ACountingFailureStopsTheRun(t *testing.T) {
	// Two things have to be true at once or this guard stays INERT, and both
	// were wrong in my first version.
	//
	// Only the COUNTER fails: a server that refuses everything makes the run
	// fail whether or not this check is here, because the first message errors
	// anyway.
	//
	// And the model must have NO documented floor. With one, deleting this
	// check leaves an empty filler that the `actual < floor` refusal below
	// catches instead — a different guard covering for this one, which is what
	// guard-reachability was reporting. With no floor, deleting it sends three
	// billable requests carrying a prefix nobody sized, and that is the fact
	// worth failing on.
	if got := cachemodel.DocumentedMinPrefix(noFloorModel); got != 0 {
		t.Fatalf("%s now has a documented floor of %d; the floor refusal would catch the "+
			"deletion of this guard and this test could not fail", noFloorModel, got)
	}
	msgs := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "count_tokens") {
			http.Error(w, "counter down", http.StatusInternalServerError)
			return
		}
		msgs++
		w.Header().Set("content-type", "application/json")
		_, _ = io.WriteString(w,
			`{"id":"m","usage":{"input_tokens":5,"cache_creation_input_tokens":4096,"cache_read_input_tokens":0,"output_tokens":1}}`)
	}))
	defer srv.Close()
	if _, err := runnerFor(srv).Vary(noFloorModel, VaryTools); err == nil {
		t.Fatal("a failing token counter must stop the run, not size the prefix by guess")
	}
	if msgs != 0 {
		t.Fatalf("the prefix could not be sized and the run still spent %d billable requests "+
			"on an experiment whose prefix nobody measured", msgs)
	}
}

// noFloorModel is a model id the compiled table documents no minimum for.
const noFloorModel = "a-model-the-table-has-never-seen"

// A prefix that cannot be built up to the floor refuses the experiment.
//
// This hung before sizedFiller grew an upper bound on `chars`. A provider whose
// count plateaus makes the search scale its request without limit, so the test
// exhausted memory rather than reaching this line — which is how the missing
// ceiling was found. The bound is the fix; this is the test that needed it.
//
// Deleting this branch does not compile (`actual` goes unused), so the reviewer
// can only neutralise it, and a compiler-rejected mutant is not a catch
// (ADR-0014). Neutralising it lets a run proceed on a prefix under the
// published minimum, where a miss cannot be told apart from a miss caused by
// the varied term — which is the entire experiment.
func TestVaryGuard_APrefixThatCannotReachTheFloorIsRefused(t *testing.T) {
	restore := cachemodel.Override(&cachemodel.Rules{
		Schema: "replay.rules/v1", Version: "test-unreachable-floor", Provider: "test",
		Models: []cachemodel.ModelRule{{
			Match: "stuck-counter-model", MinPrefix: 8192,
			InputPerMTok: 1, ReadMult: 0.1, Priced: true,
		}},
	})
	defer restore()

	msgs := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "count_tokens") {
			raw, _ := io.ReadAll(r.Body)
			// Grows with the prefix, then plateaus far under the floor.
			n := 7 + len(raw)/20
			if n > 1000 {
				n = 1000
			}
			w.Header().Set("content-type", "application/json")
			_, _ = fmt.Fprintf(w, `{"input_tokens":%d}`, n)
			return
		}
		msgs++
		w.Header().Set("content-type", "application/json")
		_, _ = io.WriteString(w,
			`{"id":"m","usage":{"input_tokens":5,"cache_creation_input_tokens":4096,"cache_read_input_tokens":0,"output_tokens":1}}`)
	}))
	defer srv.Close()

	_, err := runnerFor(srv).Vary("stuck-counter-model", VaryTools)
	if err == nil {
		t.Fatal("a prefix below the published floor must refuse; a miss on an undersized " +
			"prefix cannot be told apart from a miss caused by the varied term")
	}
	if !strings.Contains(err.Error(), "minimum cacheable prefix") {
		t.Fatalf("the refusal does not name the floor it failed to reach: %v", err)
	}
	if msgs != 0 {
		t.Fatalf("the prefix was under the floor and the run still spent %d billable requests", msgs)
	}
}

// Any one of the three requests failing stops the run and says so.
//
// Three cases, not one. The baseline, the variant and the control are sent by
// the same function through three separate error checks, and a test that only
// fails the first leaves the other two unobserved — which is exactly what
// guard-reachability reported.
func TestVaryGuard_EachOfTheThreeRequestsCanStopTheRun(t *testing.T) {
	for _, which := range []int{1, 2, 3} {
		t.Run(map[int]string{1: "baseline", 2: "variant", 3: "control"}[which], func(t *testing.T) {
			v := &varyServer{failOn: which}
			srv := v.start(t)
			_, err := runnerFor(srv).Vary("claude-opus-5", VaryTools)
			if err == nil {
				t.Fatalf("request %d failed and the run reported success", which)
			}
			if !strings.Contains(err.Error(), "502") {
				t.Fatalf("request %d: the error does not name what the provider answered: %v", which, err)
			}
			if v.messages != which {
				t.Fatalf("request %d failed but %d messages were sent; the run continued past a "+
					"failure and spent money on an experiment it could not complete", which, v.messages)
			}
		})
	}
}

// The system term changes the variant's system block and nothing else.
func TestVaryGuard_TheSystemTermChangesOnlyTheVariant(t *testing.T) {
	v := &varyServer{}
	srv := v.start(t)
	if _, err := runnerFor(srv).Vary("claude-opus-5", VarySystem); err != nil {
		t.Fatal(err)
	}
	sysText := func(i int) string {
		sys, _ := v.bodies[i]["system"].([]any)
		first, _ := sys[0].(map[string]any)
		s, _ := first["text"].(string)
		return s
	}
	base, variant, control := sysText(0), sysText(1), sysText(2)
	if base == variant {
		t.Fatal("the system term did not change the variant's system block; the experiment varies nothing")
	}
	if base != control {
		t.Fatal("the control's system block differs from the baseline's; it is not a control")
	}
}

// The effort term sends a different effort on the variant.
//
// The other INERT guard. Deleting `if variant && term == VaryEffort` sends
// "high" three times, and every returned figure is identical to a correct run —
// the experiment reports that effort does not move the key, which is the
// answer it would give if effort genuinely did not, and nothing distinguishes
// them. Only the wire does.
func TestVaryGuard_TheEffortTermChangesOnlyTheVariant(t *testing.T) {
	v := &varyServer{}
	srv := v.start(t)
	if _, err := runnerFor(srv).Vary("claude-opus-5", VaryEffort); err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, b := range v.bodies {
		s, _ := b["effort"].(string)
		got = append(got, s)
	}
	if len(got) != 3 || got[0] != "high" || got[1] != "low" || got[2] != "high" {
		t.Fatalf("efforts on the wire were %v; want high, low, high — a run that sends one "+
			"effort three times reports 'effort does not move the key' whether or not it does", got)
	}
}

// A Runner with no Client still sends.
func TestVaryGuard_ANilClientGetsADefaultOne(t *testing.T) {
	v := &varyServer{}
	srv := v.start(t)
	r := &Runner{BaseURL: srv.URL, APIKey: "k", Out: io.Discard} // Client deliberately nil
	if _, err := r.Vary("claude-opus-5", VaryTools); err != nil {
		t.Fatalf("a nil Client must get a default, not fail: %v", err)
	}
	if v.messages != 3 {
		t.Fatalf("a nil Client sent %d messages, want 3", v.messages)
	}
}

// A URL the request builder cannot parse fails before any network call.
func TestVaryGuard_AnUnparseableBaseURLFailsBeforeSending(t *testing.T) {
	r := &Runner{BaseURL: ":", APIKey: "k", Out: io.Discard}
	if _, err := r.sendVary("claude-opus-5", "filler", VaryTools, false); err == nil {
		t.Fatal("an unparseable base URL must fail rather than be sent")
	}
}

// A transport that cannot reach the provider says the request failed, and
// does not report a zero cache read.
func TestVaryGuard_AnUnreachableProviderIsAnErrorNotAZeroRead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close() // nothing is listening now
	r := &Runner{BaseURL: url, APIKey: "k", Client: srv.Client(), Out: io.Discard}
	_, err := r.sendVary("claude-opus-5", "filler", VaryTools, false)
	if err == nil {
		t.Fatal("an unreachable provider must be an error; a zero cache_read would read as a miss")
	}
	if !strings.Contains(err.Error(), "probe request failed") {
		t.Fatalf("wrong error: %v", err)
	}
}

// A body that ends early is an error, not a short read.
func TestVaryGuard_ATruncatedBodyIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("content-length", "4096")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"m"`))
		// Return without writing the promised bytes: the client sees the
		// connection end mid-body.
	}))
	defer srv.Close()
	r := &Runner{BaseURL: srv.URL, APIKey: "k", Client: srv.Client(), Out: io.Discard}
	_, err := r.sendVary("claude-opus-5", "filler", VaryTools, false)
	if err == nil {
		t.Fatal("a body that ends early must be an error, not a partial reading")
	}
	// The transport failure, not the parse failure it degrades into. Dropping
	// the read check hands the truncated bytes to parseVaryUsage, which reports
	// that the ANSWER could not be read — blaming the provider's content for
	// what was actually a connection that ended. Both are errors; only one is
	// true.
	if strings.Contains(err.Error(), "could not be read as usage") {
		t.Fatalf("a truncated connection was reported as an unreadable answer (%v); the "+
			"provider's content is not what failed", err)
	}
}

// A non-200 names the status and records nothing from it.
func TestVaryGuard_ANon200NamesTheStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer srv.Close()
	r := &Runner{BaseURL: srv.URL, APIKey: "k", Client: srv.Client(), Out: io.Discard}
	_, err := r.sendVary("claude-opus-5", "filler", VaryTools, false)
	if err == nil || !strings.Contains(err.Error(), "429") {
		t.Fatalf("a non-200 must name what the provider answered: %v", err)
	}
}

// An answer that is not JSON is an error, not an empty usage.
func TestVaryGuard_AnUnreadableAnswerIsNotAnEmptyUsage(t *testing.T) {
	// The MESSAGE, not merely that an error happened. Deleting the unmarshal
	// check leaves `parsed` at its zero value, so the nil-usage check below it
	// errors anyway — and a test asserting only `err != nil` passes with the
	// guard gone. The two failures are different facts: one says the answer was
	// unreadable, the other says it was read and carried no usage. Reporting
	// the second for the first tells an operator the provider answered cleanly
	// with nothing in it.
	_, err := parseVaryUsage([]byte(`{"usage": not json`))
	if err == nil {
		t.Fatal("unparseable JSON must be an error; an empty usage would read as cache_read=0")
	}
	if !strings.Contains(err.Error(), "could not be read") {
		t.Fatalf("an unreadable answer reported %q; it must say the answer could not be READ, "+
			"not that it carried no usage — those are different facts about the provider", err)
	}
	if _, err := parseVaryUsage([]byte(`{"usage":{"input_tokens":0}}`)); err == nil {
		t.Fatal("zero input tokens says nothing about caching and must be refused")
	}
}
