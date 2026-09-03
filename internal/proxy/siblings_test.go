package proxy

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

// holdingUpstream answers at once unless the request carries X-Test-Hold,
// in which case it waits for the hold channel current at arrival before
// writing headers. Arrivals are reported by session id.
type holdingUpstream struct {
	mu      sync.Mutex
	hold    chan struct{}
	arrived chan string
}

func (u *holdingUpstream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_, _ = io.ReadAll(r.Body)
	u.mu.Lock()
	hold := u.hold
	u.mu.Unlock()
	u.arrived <- r.Header.Get(HeaderSessionID)
	if r.Header.Get("X-Test-Hold") != "" {
		select {
		case <-hold:
		case <-r.Context().Done():
			return
		}
	}
	if r.Header.Get("X-Test-Drop") != "" {
		if hj, ok := w.(http.Hijacker); ok {
			conn, _, err := hj.Hijack()
			if err == nil {
				_ = conn.Close()
			}
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = io.WriteString(w, messageResponse)
}

func (u *holdingUpstream) setHold(ch chan struct{}) {
	u.mu.Lock()
	u.hold = ch
	u.mu.Unlock()
}

// expectArrival waits for the upstream to see a session, or fails.
func (u *holdingUpstream) expectArrival(t *testing.T, session string) {
	t.Helper()
	select {
	case got := <-u.arrived:
		if got != session {
			t.Fatalf("upstream saw %q, want %q", got, session)
		}
	case <-time.After(ledgerWait):
		t.Fatalf("upstream never saw %q", session)
	}
}

// expectNoArrival checks nothing reaches the upstream for a while.
func (u *holdingUpstream) expectNoArrival(t *testing.T, d time.Duration) {
	t.Helper()
	select {
	case got := <-u.arrived:
		t.Fatalf("upstream saw %q while it should have been held", got)
	case <-time.After(d):
	}
}

const siblingProbe = 300 * time.Millisecond

func postAsync(t *testing.T, base, body string, headers map[string]string) <-chan struct{} {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		req, err := http.NewRequest(http.MethodPost, base+"/v1/messages", strings.NewReader(body))
		if err != nil {
			t.Error(err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Error(err)
			return
		}
		_, _ = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
	}()
	return done
}

// A request whose prefix is in flight and not yet cached waits until the
// first response begins; a request with another prefix does not; once the
// prefix is warm nothing waits behind a later in-flight request. The wait
// is on the ledger, the log, status, and metrics.
func TestSiblingsAreHeldUntilTheFirstResponseBegins(t *testing.T) {
	up := &holdingUpstream{hold: make(chan struct{}), arrived: make(chan string, 16)}
	base, dir, logs := startProxyWith(t, up, Config{Siblings: SiblingSettings{MaxWait: DefaultSiblingWait}})
	first := up.hold

	lead := postAsync(t, base, requestBody, map[string]string{HeaderSessionID: "lead", "X-Test-Hold": "1"})
	up.expectArrival(t, "lead")
	sib := postAsync(t, base, requestBody, map[string]string{HeaderSessionID: "sib"})
	up.expectNoArrival(t, siblingProbe)
	other := strings.Replace(requestBody, "be brief", "be verbose", 1)
	postWith(t, base, other, map[string]string{HeaderSessionID: "other"})
	up.expectArrival(t, "other")
	up.expectNoArrival(t, siblingProbe/3)

	close(first)
	up.expectArrival(t, "sib")
	<-lead
	<-sib

	// The prefix is warm now: a new leader in flight holds nobody.
	up.setHold(make(chan struct{}))
	late := postAsync(t, base, requestBody, map[string]string{HeaderSessionID: "late", "X-Test-Hold": "1"})
	up.expectArrival(t, "late")
	postWith(t, base, requestBody, map[string]string{HeaderSessionID: "after"})
	up.expectArrival(t, "after")
	close(up.hold)
	<-late

	recs := waitLedger(t, dir, 5)
	held := map[string]int64{}
	for _, r := range recs {
		held[r.SessionID] = r.HeldMS
	}
	if held["sib"] < siblingProbe.Milliseconds() || held["lead"] != 0 || held["other"] != 0 || held["late"] != 0 || held["after"] != 0 {
		t.Fatalf("held_ms by session: %v", held)
	}
	if !strings.Contains(logs.String(), "session=sib") || !strings.Contains(logs.String(), " held_ms=") {
		t.Fatalf("the wait must be logged:\n%s", logs.String())
	}
	st := getStatus(t, base)
	for _, s := range st.Sessions {
		if (s.Session == "sib") != (s.Held == 1 && s.HeldMS >= siblingProbe.Milliseconds()) {
			t.Fatalf("status: %+v", s)
		}
	}
	resp, err := http.Get(base + "/buffy/metrics")
	if err != nil {
		t.Fatal(err)
	}
	metrics, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(string(metrics), "buffy_held_total 1\n") || !strings.Contains(string(metrics), "buffy_held_milliseconds_total ") {
		t.Fatalf("metrics:\n%s", metrics)
	}
}

// The wait is bounded, a leader that fails without a response lets its
// siblings go, and with the policy off nothing is held.
func TestSiblingsWaitIsBoundedAndFailuresRelease(t *testing.T) {
	up := &holdingUpstream{hold: make(chan struct{}), arrived: make(chan string, 16)}
	base, dir, _ := startProxyWith(t, up, Config{Siblings: SiblingSettings{MaxWait: siblingProbe}})
	lead := postAsync(t, base, requestBody, map[string]string{HeaderSessionID: "lead", "X-Test-Hold": "1"})
	up.expectArrival(t, "lead")
	before := time.Now()
	postWith(t, base, requestBody, map[string]string{HeaderSessionID: "sib"})
	up.expectArrival(t, "sib")
	if waited := time.Since(before); waited < siblingProbe || waited > ledgerWait {
		t.Fatalf("the sibling must go after the bound, went after %s", waited)
	}
	close(up.hold)
	<-lead
	recs := waitLedger(t, dir, 2)
	for _, r := range recs {
		if r.SessionID == "sib" && (r.HeldMS < siblingProbe.Milliseconds()) {
			t.Fatalf("held_ms must show the bounded wait: %+v", r)
		}
	}

	// A failing leader with a fresh prefix releases its sibling at once.
	up.setHold(make(chan struct{}))
	fresh := strings.Replace(requestBody, "be brief", "be quick", 1)
	req, _ := http.NewRequest(http.MethodPost, base+"/v1/messages", strings.NewReader(fresh))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderSessionID, "drop")
	req.Header.Set("X-Test-Drop", "1")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	up.expectArrival(t, "drop")
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("dropped leader status %d", resp.StatusCode)
	}
	before = time.Now()
	postWith(t, base, fresh, map[string]string{HeaderSessionID: "sib2"})
	up.expectArrival(t, "sib2")
	if time.Since(before) >= siblingProbe {
		t.Fatal("a sibling of a failed leader must not wait for the bound")
	}

	// Off: parallel requests with one prefix all go at once.
	off := &holdingUpstream{hold: make(chan struct{}), arrived: make(chan string, 16)}
	offBase, _, _ := startProxyWith(t, off, Config{})
	l := postAsync(t, offBase, requestBody, map[string]string{HeaderSessionID: "lead", "X-Test-Hold": "1"})
	off.expectArrival(t, "lead")
	postWith(t, offBase, requestBody, map[string]string{HeaderSessionID: "sib"})
	off.expectArrival(t, "sib")
	close(off.hold)
	<-l
}

// The warm window expires with the cache lifetime and the table stays
// bounded.
func TestSiblingGateWarmWindowAndBound(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	g := newSiblingGate(SiblingSettings{MaxWait: time.Second})
	g.now = func() time.Time { return now }
	release, waited := g.enter(context.Background(), "p")
	if waited != 0 || len(g.flights) != 1 {
		t.Fatal("first request must lead at once")
	}
	g.began("p")
	release()
	if _, w := g.enter(context.Background(), "p"); w != 0 || len(g.flights) != 0 {
		t.Fatal("a warm prefix must not register a flight")
	}
	now = now.Add(siblingWarmWindow)
	if _, w := g.enter(context.Background(), "p"); w != 0 || len(g.flights) != 1 {
		t.Fatal("an expired prefix must lead again")
	}
	for i := 0; i < maxSiblingPrefixes+10; i++ {
		g.began(string(rune('a'+i%26)) + string(rune(i)))
		now = now.Add(time.Millisecond)
	}
	if len(g.warm) > maxSiblingPrefixes {
		t.Fatalf("warm table unbounded: %d", len(g.warm))
	}
	// A cancelled request stops waiting.
	g2 := newSiblingGate(SiblingSettings{MaxWait: time.Minute})
	_, _ = g2.enter(context.Background(), "q")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, w := g2.enter(ctx, "q"); w > time.Second {
		t.Fatal("a cancelled request must not wait")
	}
	if r, _ := newSiblingGate(SiblingSettings{}).enter(context.Background(), "p"); r == nil {
		t.Fatal("off must return a release function")
	}
}
