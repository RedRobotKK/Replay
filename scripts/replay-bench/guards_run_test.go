package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/masking"
	"github.com/RedRobotKK/Replay/internal/proxy"
)

// closedUpstream is the URL of a server that has stopped listening, so every
// request to it fails at the transport.
func closedUpstream(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	return srv.URL
}

// failingAfter is the mock upstream for its first n requests and a 500 for
// every request after, so a suite can be failed at a chosen step.
func failingAfter(t *testing.T, n int64) string {
	t.Helper()
	url, _ := countingFailingAfter(t, n) // the count is for tests that ask how far a suite got
	return url
}

// countingFailingAfter is failingAfter, also returning how many requests the
// upstream has seen, so a test can show a suite stopped at its first failure
// rather than carrying on into the next phase.
func countingFailingAfter(t *testing.T, n int64) (string, *atomic.Int64) {
	t.Helper()
	seen := new(atomic.Int64)
	h := upstreamHandler()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if seen.Add(1) > n {
			http.Error(w, "planted failure", http.StatusInternalServerError)
			return
		}
		h.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv.URL, seen
}

// noProxy makes every proxy fail to construct.
func noProxy(t *testing.T) {
	t.Helper()
	swap(t, &newProxyServer, func(proxy.Config) (*proxy.Server, error) { return nil, errInjected })
}

// liveUpstream is the mock upstream, closed with the test.
func liveUpstream(t *testing.T) string {
	t.Helper()
	up := newUpstream()
	t.Cleanup(up.Close)
	return up.URL
}

// tinyConfig is the least Execute will run: every always-present suite, one
// request where one is needed, nothing optional.
func tinyConfig() Config {
	return Config{Repeats: 2, SSEEvents: 1, ColdFirstN: 1, ColdSteady: 1, LedgerIters: 1}
}

func TestExecuteFlagsNoisyRepeatsAndKeepsTheLabelFirst(t *testing.T) {
	busy := LoadAvg{One: 1e6, OK: true}
	quiet := LoadAvg{One: 0, OK: true}

	swap(t, &currentLoad, func() LoadAvg { return busy })
	cfg := tinyConfig()
	cfg.NoisyLabel = "known noisy"
	res, err := Execute(cfg, "sha", nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "known noisy; 2 of 2 repeats started with 1-minute load above"
	if !res.Noisy || !strings.HasPrefix(res.NoisyReason, want) {
		t.Errorf("busy and labelled: Noisy %v, reason %q; want prefix %q", res.Noisy, res.NoisyReason, want)
	}
	for _, m := range res.Repeats {
		if !m.Noisy {
			t.Errorf("repeat %d under load %v not flagged", m.Index, m.Load.One)
		}
	}

	swap(t, &currentLoad, func() LoadAvg { return quiet })
	res, err = Execute(cfg, "sha", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !res.Noisy || res.NoisyReason != "known noisy" {
		t.Errorf("quiet and labelled: Noisy %v, reason %q; want the label alone", res.Noisy, res.NoisyReason)
	}

	res, err = Execute(tinyConfig(), "sha", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Noisy || res.NoisyReason != "" {
		t.Errorf("quiet and unlabelled: Noisy %v, reason %q", res.Noisy, res.NoisyReason)
	}
}

func TestExecuteReportsAComponentSuiteThatCannotBeBuilt(t *testing.T) {
	// Both must stop Execute before the first repeat, with the constructor's
	// own error: carrying on would measure a run missing a suite.
	t.Run("mask", func(t *testing.T) {
		swap(t, &openVault, func(string) (*masking.Vault, error) { return nil, errInjected })
		cfg := tinyConfig()
		cfg.Sizes = SmokeConfig().Sizes
		res, err := Execute(cfg, "sha", nil)
		if !errors.Is(err, errInjected) || len(res.Repeats) != 0 {
			t.Errorf("err = %v after %d repeats; want the vault failure before any", err, len(res.Repeats))
		}
	})
	t.Run("ledger", func(t *testing.T) {
		missingTempDir(t)
		res, err := Execute(tinyConfig(), "sha", nil)
		if !isMkdirTemp(err) || len(res.Repeats) != 0 {
			t.Errorf("err = %v after %d repeats; want the ledger directory failure before any", err, len(res.Repeats))
		}
	})
}

func TestExecuteNamesTheSuiteAndRepeatThatFailed(t *testing.T) {
	noProxy(t)
	_, err := Execute(tinyConfig(), "sha", nil)
	if err == nil || !strings.Contains(err.Error(), "coldstart/45KB, repeat 0") {
		t.Errorf("err = %v, want it to name coldstart/45KB, repeat 0", err)
	}
}

func TestExecuteReportsASuiteThatFailsToClose(t *testing.T) {
	orig := removeAll
	swap(t, &removeAll, func(p string) error {
		if err := orig(p); err != nil {
			return err
		}
		return errInjected
	})
	if _, err := Execute(tinyConfig(), "sha", nil); !errors.Is(err, errInjected) {
		t.Errorf("err = %v, want the close failure", err)
	}
}

func TestPairedRunBatchAndOverheadReportFailures(t *testing.T) {
	ok := func() (timing, error) { return timing{total: time.Millisecond}, nil }
	bad := func() (timing, error) { return timing{}, errInjected }
	if _, err := pairedRun(2, map[string]func() (timing, error){ArmDirect: ok, ArmProxied: bad}); err == nil ||
		!strings.Contains(err.Error(), ArmProxied) {
		t.Errorf("pairedRun err = %v, want the failing arm named", err)
	}
	if _, _, err := batch(2, bad); err == nil {
		t.Error("batch timed a failed request")
	}
	c := newCollector("x", nil, "")
	if err := putOverhead(&c, "", []int64{1, 2}, []int64{1}); err == nil {
		t.Error("putOverhead paired arms of unequal length")
	}
	unequal := map[string][]timing{ArmDirect: {{total: 1}, {total: 2}}, ArmProxied: {{total: 1}}}
	if err := recordStream(&c, 0, unequal); err == nil {
		t.Error("recordStream paired arms of unequal length")
	}
}

func TestFreshProxyReportsAProxyThatWillNotStart(t *testing.T) {
	noProxy(t)
	c := newCollector("x", nil, "")
	if _, err := freshProxy(&c, liveUpstream(t), false); err == nil {
		t.Error("freshProxy returned a proxy that did not start")
	}
	if _, ok := c.s.PerRepeat["proxied_prior_requests"]; ok {
		t.Error("a proxy that did not start was recorded as fresh")
	}
}

func TestLatencySuiteReportsEachFailingPhase(t *testing.T) {
	for _, c := range []struct {
		name string
		size SizeSpec
	}{
		{"warmup", SizeSpec{Name: "t", Warmup: 1, Pairs: 1, Batch: 1}},
		{"pairs", SizeSpec{Name: "t", Pairs: 1, Batch: 1}},
		{"batch", SizeSpec{Name: "t", Batch: 1}},
	} {
		t.Run(c.name, func(t *testing.T) {
			// The first request fails; a suite that stopped there sent one.
			up, seen := countingFailingAfter(t, 0)
			if err := newLatencySuite(c.size, false, up).repeat(0); err == nil {
				t.Errorf("a failed %s phase was recorded", c.name)
			}
			if n := seen.Load(); n != 1 {
				t.Errorf("a failed %s phase went on to send %d requests, want 1", c.name, n)
			}
		})
	}
	t.Run("proxy", func(t *testing.T) {
		noProxy(t)
		if err := newLatencySuite(SizeSpec{Name: "t", Batch: 1}, false, liveUpstream(t)).repeat(0); err == nil {
			t.Error("a repeat ran without its proxy")
		}
	})
}

func TestSSESuiteReportsEachFailingPhase(t *testing.T) {
	cfg := Config{SSEEvents: 1, SSEPairs: 1}
	up, seen := countingFailingAfter(t, 0)
	if err := newSSESuite(cfg, up).repeat(0); err == nil {
		t.Error("a failed warmup was recorded")
	}
	if n := seen.Load(); n != 1 {
		t.Errorf("a failed warmup went on to send %d requests, want 1", n)
	}
	// Warmup is one direct and one proxied request; the third fails.
	if err := newSSESuite(cfg, failingAfter(t, 2)).repeat(0); err == nil {
		t.Error("a failed pair was recorded")
	}
	noProxy(t)
	if err := newSSESuite(cfg, liveUpstream(t)).repeat(0); err == nil {
		t.Error("a repeat ran without its proxy")
	}
}

func TestConcurrencyRunArmStopsAWorkerAtItsFirstError(t *testing.T) {
	var calls atomic.Int64
	s := &concurrencySuite{c: 1, perWorker: 5, send: map[string]func() (timing, error){
		ArmDirect: func() (timing, error) { calls.Add(1); return timing{}, errInjected },
	}}
	if _, _, _, err := s.runArm(ArmDirect); !errors.Is(err, errInjected) {
		t.Errorf("runArm err = %v, want the worker's error", err)
	}
	if n := calls.Load(); n != 1 {
		t.Errorf("a failing worker sent %d requests, want 1", n)
	}
}

func TestConcurrencyRunArmRecordsTheGoroutinePeak(t *testing.T) {
	// The workers are many, so the peak they make cannot be confused with
	// the count runArm starts from, which includes whatever earlier tests
	// left winding down; comparing against a count taken here was flaky for
	// exactly that reason.
	const workers = 100
	s := &concurrencySuite{c: workers, perWorker: 1, send: map[string]func() (timing, error){
		ArmDirect: func() (timing, error) { time.Sleep(50 * time.Millisecond); return timing{}, nil },
	}}
	_, _, d, err := s.runArm(ArmDirect)
	if err != nil {
		t.Fatal(err)
	}
	// They sleep for 50ms together while the sampler ticks every millisecond.
	if d.GoroutinesPeak < workers {
		t.Errorf("goroutine peak %d, want at least the %d workers", d.GoroutinesPeak, workers)
	}
}

func TestConcurrencySuiteReportsEachFailingPhase(t *testing.T) {
	cfg := Config{PerWorker: 1}
	up, seen := countingFailingAfter(t, 0)
	if err := newConcurrencySuite(cfg, 1, up).repeat(0); err == nil {
		t.Error("a failed warmup round was recorded")
	}
	if n := seen.Load(); n != 1 {
		t.Errorf("a failed warmup round went on to send %d requests, want 1", n)
	}
	// The warmup round is one direct and one proxied request; the third fails.
	if err := newConcurrencySuite(cfg, 1, failingAfter(t, 2)).repeat(0); err == nil {
		t.Error("a failed measured round was recorded")
	}
	noProxy(t)
	if err := newConcurrencySuite(cfg, 1, liveUpstream(t)).repeat(0); err == nil {
		t.Error("a repeat ran without its proxy")
	}
}

func TestSessionSuiteRecordsTheBuildAndReportsEachFailingPhase(t *testing.T) {
	cfg := Config{SSEEvents: 1}
	s := newSessionSuite(cfg, 2, liveUpstream(t)).(*sessionSuite)
	if err := s.repeat(0); err != nil {
		t.Fatal(err)
	}
	if got := s.s.PerRepeat["build_mean_request_us"]; len(got) != 1 {
		t.Errorf("build_mean_request_us = %v, want one value for a session built from one turn", got)
	}
	if err := newSessionSuite(cfg, 2, closedUpstream(t)).repeat(0); err == nil {
		t.Error("a failed session build was recorded")
	}
	// One turn builds the session; the first pair's direct request fails.
	cfg.SessionPairs = 1
	if err := newSessionSuite(cfg, 2, failingAfter(t, 1)).repeat(0); err == nil {
		t.Error("a failed pair was recorded")
	}
	noProxy(t)
	if err := newSessionSuite(cfg, 2, liveUpstream(t)).repeat(0); err == nil {
		t.Error("a repeat ran without its proxy")
	}
}

func TestColdSuiteRunsTheProxiedArmThroughAFreshProxyAndClosesIt(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	cfg := Config{ColdFirstN: 1, ColdWarm: 1, ColdSteady: 2}
	s := newColdSuite(cfg, liveUpstream(t)).(*coldSuite)
	if err := s.repeat(0); err != nil {
		t.Fatal(err)
	}
	if got := s.s.PerRepeat["proxied_startup_us"]; len(got) != 1 {
		t.Errorf("proxied_startup_us = %v: the proxied arm did not start a proxy", got)
	}
	for _, r := range s.s.Runs {
		want := map[string]int{"cold-first-n": 1, "steady": 2}[r.Phase]
		if len(r.SamplesNS) != want {
			t.Errorf("%s %s has %d samples, want %d", r.Arm, r.Phase, len(r.SamplesNS), want)
		}
	}
	// The proxy is closed, which removes its ledger directory.
	if left, err := os.ReadDir(tmp); err != nil || len(left) != 0 {
		t.Errorf("the cold proxy left %v behind (%v)", left, err)
	}
}

func TestColdSuiteReportsAFailedRequestAndAProxyThatWillNotStart(t *testing.T) {
	cfg := Config{ColdFirstN: 1, ColdSteady: 1}
	if err := newColdSuite(cfg, closedUpstream(t)).repeat(0); err == nil {
		t.Error("a failed cold request was recorded")
	}
	noProxy(t)
	if err := newColdSuite(cfg, liveUpstream(t)).repeat(0); err == nil {
		t.Error("the proxied arm ran without its proxy")
	}
}

// failingMasker refuses every body.
type failingMasker struct{}

func (failingMasker) Mask([]byte) ([]byte, masking.Report, error) {
	return nil, masking.Report{}, errInjected
}

func TestMaskComponentSuiteReportsEachFailure(t *testing.T) {
	size := SizeSpec{Name: "t", ComponentIters: 1}
	t.Run("tempdir", func(t *testing.T) {
		missingTempDir(t)
		if _, err := newMaskComponentSuite(size); !isMkdirTemp(err) {
			t.Errorf("err = %v, want the failed MkdirTemp itself", err)
		}
	})
	t.Run("vault", func(t *testing.T) {
		swap(t, &openVault, func(string) (*masking.Vault, error) { return nil, errInjected })
		if _, err := newMaskComponentSuite(size); !errors.Is(err, errInjected) {
			t.Error("a mask suite was built without its vault")
		}
	})
	t.Run("mask", func(t *testing.T) {
		s := &maskComponentSuite{collector: newCollector("x", nil, ""), size: size, mask: failingMasker{}}
		if err := s.repeat(0); err == nil {
			t.Error("a failed Mask was timed")
		}
	})
}

func TestLedgerComponentSuiteReportsEachFailure(t *testing.T) {
	cfg := Config{LedgerIters: 1}
	t.Run("tempdir", func(t *testing.T) {
		missingTempDir(t)
		if _, err := newLedgerComponentSuite(cfg); !isMkdirTemp(err) {
			t.Errorf("err = %v, want the failed MkdirTemp itself", err)
		}
	})
	t.Run("open", func(t *testing.T) {
		swap(t, &openLedger, func(string) (*ledger.Store, error) { return nil, errInjected })
		if _, err := newLedgerComponentSuite(cfg); !errors.Is(err, errInjected) {
			t.Error("a ledger suite was built without its store")
		}
	})
	t.Run("append", func(t *testing.T) {
		s, err := newLedgerComponentSuite(cfg)
		if err != nil {
			t.Fatal(err)
		}
		// With its directory gone, the store cannot open a session file.
		if err := s.close(); err != nil {
			t.Fatal(err)
		}
		if err := s.repeat(0); err == nil {
			t.Error("a failed Append was timed")
		}
	})
}
