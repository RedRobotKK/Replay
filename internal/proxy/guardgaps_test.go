package proxy

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Guard surfaces that were built and never observed.
//
// Each of these is a place the proxy declines to answer, or declines to claim
// something it does not know. They were all at zero statement coverage as of
// this audit, which under ADR-0014 makes each indistinguishable from an empty
// function.

// GG1: a day cap tripped by spend restored from disk says so, rather than
// blaming a session.
//
// SpendGuard.attributeDay names the session that spent most of the day's
// budget. After a restart the day total comes back from spend-day.json and the
// per-session table does not — it is deliberately not persisted. The refusal is
// then real and entirely unattributable, and attributeDay has a branch that
// says exactly that.
//
// This matters because the alternative is worse than saying nothing. If the
// first session after a restart were named as "most of it", the operator would
// read a lane that has spent nothing today as the cause of the overrun.
//
// PASS: the refusal fires and carries "no live session accounts for it", and
// names no session.
// FAIL: no refusal (the day cap did not survive), or a refusal that names a
// session it cannot possibly have measured.
func TestGG1_ADayCapRestoredFromDiskNamesNobody(t *testing.T) {
	dir := t.TempDir()
	day := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)

	first := NewSpendGuard(SpendLimits{DayTokens: 1_000_000})
	first.now = fixedClock(day)
	first.Record("sess-overnight", 1_200_000, 0)
	first.SaveState(dir)

	// The process dies. The day total returns; the sessions do not.
	second := NewSpendGuard(SpendLimits{DayTokens: 1_000_000})
	second.now = fixedClock(day)
	second.LoadState(dir)

	msg := second.Check("sess-fresh")
	if msg == "" {
		t.Fatal("the day cap did not survive the restart at all")
	}
	if !strings.Contains(msg, "no live session accounts for it") {
		t.Fatalf("spend with no live session behind it must be disclosed, not attributed:\n%s", msg)
	}
	if strings.Contains(msg, "sess-fresh") || strings.Contains(msg, "most of it from session") {
		t.Fatalf("a session that has spent nothing today must not be blamed:\n%s", msg)
	}
}

// GG1b: with live sessions present the same cap still names the spender.
//
// Without this, GG1 is satisfied by an attributeDay that never names anybody,
// which would make the whole SP-7 attribution feature dead code that no test
// could tell from working code.
//
// PASS: the refusal names the largest live spender.
// FAIL: "no live session accounts for it" when one plainly does.
func TestGG1b_ALiveSpenderIsStillNamed(t *testing.T) {
	day := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	g := NewSpendGuard(SpendLimits{DayTokens: 1_000_000})
	g.now = fixedClock(day)
	g.Record("sess-small", 100_000, 0)
	g.Record("sess-big", 950_000, 0)

	msg := g.Check("sess-small")
	if msg == "" {
		t.Fatal("1.05M of a 1M day cap must refuse")
	}
	if !strings.Contains(msg, "sess-big") {
		t.Fatalf("the largest live spender must be named:\n%s", msg)
	}
}

// GG2: an idle proxy still stamps today when it saves.
//
// SaveState is called on shutdown. A proxy that served nothing has no day
// recorded, and skipping the write would leave no marker at all — which is how
// a day cap silently starts over, the exact failure spend-day.json exists to
// prevent. The comment on the branch says so; nothing executed it.
//
// PASS: the file exists after an idle save and carries today's date, so a
// restart on the same day loads a marker rather than starting blank.
// FAIL: no file, or a file with an empty day, which a later LoadState discards
// as "from another day".
func TestGG2_AnIdleProxyStillWritesTodaysMarker(t *testing.T) {
	dir := t.TempDir()
	day := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)

	g := NewSpendGuard(SpendLimits{DayTokens: 1_000_000})
	g.now = fixedClock(day)
	g.SaveState(dir) // nothing was ever recorded

	body, err := os.ReadFile(filepath.Join(dir, spendStateFile))
	if err != nil {
		t.Fatalf("an idle proxy must still leave a marker: %v", err)
	}
	if !strings.Contains(string(body), `"day":"2026-09-09"`) {
		t.Fatalf("the marker must carry today's date, got: %s", body)
	}
}

// GG3: persistence is off when there is nowhere to persist to.
//
// The nil and empty-dir guards on LoadState and SaveState are what make the
// state file optional. A proxy started without a ledger directory must run,
// not panic, and must not write anything into the process's working directory.
//
// PASS: all four calls return without panicking and no file appears.
// FAIL: a panic, or a spend-day.json written into the working directory of
// whoever ran the proxy.
func TestGG3_SpendStateIsOptional(t *testing.T) {
	var nilGuard *SpendGuard
	nilGuard.LoadState(t.TempDir())
	nilGuard.SaveState(t.TempDir())

	g := NewSpendGuard(SpendLimits{DayTokens: 10})
	g.LoadState("")
	g.SaveState("")

	if _, err := os.Stat(spendStateFile); err == nil {
		t.Fatalf("an empty directory must mean no persistence, not the working directory")
	}
}

// GG4: which caps are configured reaches the status endpoint.
//
// Status.Caps is what `replay doctor` reads over HTTP to tell whether a blind
// dollar cap is already covered by a token one. cmd/replay tests the doctor
// against a hand-built CapStatus, and SpendGuard.Configured is tested nowhere:
// the two halves were each covered and the join between them was not, which
// ADR-0018 names as the built-but-unwired shape.
//
// PASS: a proxy started with a session-token cap and a day-dollar cap reports
// exactly those two as configured over /replay/status.
// FAIL: all false, which is a doctor that reports "no cap configured" on a
// proxy that has one and would advise the operator to add a second.
func TestGG4_ConfiguredCapsReachTheStatusEndpoint(t *testing.T) {
	up := &upstream{t: t}
	base, _, _ := startProxyWith(t, up, Config{
		Spend: NewSpendGuard(SpendLimits{SessionTokens: 500_000, DayUSD: 25}),
	})

	st := getStatus(t, base)
	want := CapStatus{SessionTokens: true, DayUSD: true}
	if st.Caps != want {
		t.Fatalf("the status endpoint reports caps %+v, the proxy was started with %+v", st.Caps, want)
	}
}

// GG4b: a cap that was not set is reported as not set.
//
// The first fixture here was a proxy with no spend guard at all, and it could
// not do this job: Configured returns an empty CapStatus for a nil guard before
// reading any limit, so a Configured that hard-coded SessionTokens: true
// survived it. The fixture that distinguishes has a guard present with a
// different cap set — the case a doctor actually has to get right, since its
// whole question is whether the caps that exist cover the traffic.
//
// PASS: with only a day-token cap configured, exactly DayTokens is reported.
// FAIL: any other field true, which is a doctor told a cap exists that does
// not, and an operator who leaves traffic uncapped on that advice.
func TestGG4b_ACapThatWasNotSetIsReportedAsNotSet(t *testing.T) {
	up := &upstream{t: t}
	base, _, _ := startProxyWith(t, up, Config{
		Spend: NewSpendGuard(SpendLimits{DayTokens: 2_000_000}),
	})

	st := getStatus(t, base)
	want := CapStatus{DayTokens: true}
	if st.Caps != want {
		t.Fatalf("only a day-token cap was configured; status says %+v", st.Caps)
	}
}

// GG4c: no spend guard at all reports no caps.
//
// PASS: an all-false CapStatus for a proxy started without any --spend flag.
// FAIL: any true, which would tell a diagnostic that an unguarded proxy is
// protected.
func TestGG4c_NoSpendGuardReportsNoCaps(t *testing.T) {
	up := &upstream{t: t}
	base, _, _ := startProxyWith(t, up, Config{})

	if st := getStatus(t, base); st.Caps != (CapStatus{}) {
		t.Fatalf("no spend guard was configured and status says %+v", st.Caps)
	}
}

// GG5: a refusal is visible on /replay/metrics, by guard.
//
// The per-guard line in metrics() had never been rendered with a value in it.
// A total alone cannot be acted on: "Replay refused 40 requests" is a different
// operational problem depending on whether it was the spend cap or the circuit
// breaker, and the label is the part that says which.
//
// PASS: replay_refused_total carries a line labelled with the guard that fired
// and the count.
// FAIL: no labelled line, leaving a monitor that can see refusals happening and
// cannot see what is causing them.
func TestGG5_RefusalsAreCountedByGuardOnMetrics(t *testing.T) {
	s := &Server{cfg: Config{}, stats: newStats()}
	w := httptest.NewRecorder()

	s.refuse(w, refusalSpendCap, "over", 0)
	s.refuse(w, refusalSpendCap, "over", 0)
	s.refuse(w, refusalCircuitOpen, "open", time.Second)

	out := s.stats.metrics()
	for _, want := range []string{
		`replay_refused_total{guard="spend_cap"} 2`,
		`replay_refused_total{guard="circuit_open"} 1`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s on the metrics endpoint:\n%s", want, out)
		}
	}
}

// GG6: a refusal with no session id is logged as unattributed, not as blank.
//
// The circuit breaker refuses before the body has been read, so there is often
// no session to name. "session=" with nothing after it reads as a truncated log
// line; "session=unattributed" reads as the fact it is.
//
// PASS: the log line says unattributed, and a request that does carry a session
// names it instead.
// FAIL: an empty session field, or "unattributed" printed over a session id
// that was there all along.
func TestGG6_ARefusalWithNoSessionIsNamedUnattributed(t *testing.T) {
	var buf bytes.Buffer
	s := &Server{cfg: Config{Logger: log.New(&buf, "", 0)}, stats: newStats()}

	s.refuseSession(httptest.NewRecorder(), "", "claude-opus-5", refusalCircuitOpen, "provider failing", time.Second)
	if !strings.Contains(buf.String(), "session=unattributed") {
		t.Fatalf("a refusal with no session must say so:\n%s", buf.String())
	}

	buf.Reset()
	s.refuseSession(httptest.NewRecorder(), "sess-known-1234", "claude-opus-5", refusalSpendCap, "over", 0)
	got := buf.String()
	if strings.Contains(got, "unattributed") || !strings.Contains(got, "sess-known-1") {
		t.Fatalf("a refusal that has a session must name it:\n%s", got)
	}
}

// GG7: loopback is decided by what the address is, not by how it is spelled.
//
// New refuses a listen address that is not loopback, and listenMetrics refuses
// the same for the counters. "localhost:4000" is the spelling most people
// reach for and it is loopback; ::1 is loopback; a routable address is not, and
// neither is something that is not a host:port at all.
//
// PASS: exactly the loopback spellings are accepted.
// FAIL: localhost rejected (the proxy refuses the most common way to write its
// own address), or a routable address accepted (the counters and the API key
// go on the network).
func TestGG7_LoopbackIsDecidedByAddressNotSpelling(t *testing.T) {
	for _, addr := range []string{"localhost:4000", "127.0.0.1:4000", "[::1]:4000", "127.9.9.9:0"} {
		if !isLoopback(addr) {
			t.Errorf("%q is loopback and was refused", addr)
		}
	}
	for _, addr := range []string{"0.0.0.0:4000", "192.168.1.10:4000", ":4000", "not-an-address", "", "localhost"} {
		if isLoopback(addr) {
			t.Errorf("%q is not a loopback host:port and was accepted", addr)
		}
	}
}

// GG8: localhost is accepted end to end, on both listeners.
//
// GG7 is a unit test of the predicate. This is the join: a proxy actually
// started on localhost binds and serves, and so does its metrics listener.
//
// PASS: both bind and the counters are served.
// FAIL: a refusal, which is the proxy rejecting an address it considers
// loopback everywhere else.
func TestGG8_AProxyBindsOnLocalhost(t *testing.T) {
	srv, _, done, cancel := metricsServer(t, "localhost:0", "localhost:0")
	t.Cleanup(func() {
		cancel()
		<-done
	})
	srv.Addr()
	addr := srv.MetricsAddr()
	if addr == "" {
		t.Fatal("a localhost metrics address was refused")
	}
	resp, err := http.Get("http://" + addr + MetricsPath)
	if err != nil {
		t.Fatalf("the metrics listener on localhost is not serving: %v", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("metrics over localhost = %d", resp.StatusCode)
	}
}

// GG9: Release on a breaker that is off does nothing and costs nothing.
//
// Release gives back a half-open probe that never reached the provider. Every
// caller runs it from a deferred function on the request path, whether or not a
// breaker is configured, so the disabled case is the common one.
//
// PASS: Release on a nil and on a zero-Failures breaker leaves Allow saying
// yes, with no probe and no wait.
// FAIL: a panic on the nil breaker — a request path that crashes for everybody
// who did not configure a circuit breaker.
func TestGG9_ReleaseOnADisabledBreakerIsANoop(t *testing.T) {
	var nilBreaker *Breaker
	nilBreaker.Release()
	if ok, probe, wait := nilBreaker.Allow(); !ok || probe || wait != 0 {
		t.Fatalf("a breaker that does not exist must allow everything: ok=%v probe=%v wait=%v", ok, probe, wait)
	}

	off := NewBreaker(BreakerSettings{})
	off.Release()
	if ok, probe, wait := off.Allow(); !ok || probe || wait != 0 {
		t.Fatalf("a disabled breaker must allow everything: ok=%v probe=%v wait=%v", ok, probe, wait)
	}
}
