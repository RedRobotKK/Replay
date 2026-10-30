package main

import (
	"strings"
	"testing"
)

// A tiny end-to-end run through the real proxy handler. It asserts shape,
// never speed: a test that asserted a latency would fail on a loaded runner
// for reasons unrelated to the code, which is the reason the benchmark in
// internal/proxy asserts no threshold either.
func TestRunProducesEveryScenarioThroughTheRealProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("starts real servers")
	}
	cfg := SmokeConfig()
	res, err := Execute(cfg, "test-sha", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Environment.GitSHA != "test-sha" {
		t.Errorf("GitSHA = %q", res.Environment.GitSHA)
	}
	if len(res.Repeats) != cfg.Repeats {
		t.Errorf("repeats recorded = %d, want %d", len(res.Repeats), cfg.Repeats)
	}

	want := []string{
		"latency/45KB/mask-off", "latency/45KB/mask-on",
		"sse/45KB/events-", "concurrency/c1", "session/n10", "coldstart/45KB",
		"component/mask/45KB", "component/ledger-append",
	}
	byPrefix := func(p string) *Scenario {
		for i := range res.Scenarios {
			if strings.HasPrefix(res.Scenarios[i].Name, p) {
				return &res.Scenarios[i]
			}
		}
		return nil
	}
	for _, w := range want {
		if byPrefix(w) == nil {
			var names []string
			for _, s := range res.Scenarios {
				names = append(names, s.Name)
			}
			t.Errorf("scenario %q missing; have %v", w, names)
		}
	}

	lat := byPrefix("latency/45KB/mask-off")
	if lat == nil {
		t.FailNow()
	}
	arms := map[string]int{}
	for _, r := range lat.Runs {
		arms[r.Arm]++
		if len(r.SamplesNS) == 0 {
			t.Errorf("run %+v has no raw samples", r.Arm)
		}
	}
	if arms[ArmDirect] == 0 || arms[ArmProxied] == 0 {
		t.Errorf("latency scenario arms = %v, want both", arms)
	}
	for _, m := range []string{"overhead_paired_p50_us", "overhead_p99_diff_us", "proxied_allocs_per_op", "direct_allocs_per_op"} {
		if _, ok := lat.Metrics[m]; !ok {
			t.Errorf("latency scenario missing metric %q; have %v", m, keys(lat.Metrics))
		}
	}
	// The proxy allocates on every request (it parses the body), so the
	// proxied arm must allocate more per request than the direct arm. If it
	// does not, the proxied arm did not go through the proxy.
	if lat.Metrics["proxied_allocs_per_op"].Median <= lat.Metrics["direct_allocs_per_op"].Median {
		t.Errorf("proxied allocs/op %v not above direct %v: did the proxied arm reach the proxy?",
			lat.Metrics["proxied_allocs_per_op"].Median, lat.Metrics["direct_allocs_per_op"].Median)
	}

	sse := byPrefix("sse/45KB/events-")
	if sse == nil {
		t.FailNow()
	}
	for _, m := range []string{"overhead_ttfb_paired_p50_us", "overhead_tail_paired_p50_us"} {
		if _, ok := sse.Metrics[m]; !ok {
			t.Errorf("sse scenario missing %q; have %v", m, keys(sse.Metrics))
		}
	}
	conc := byPrefix("concurrency/c1")
	if conc == nil {
		t.FailNow()
	}
	if s, ok := conc.Metrics["proxied_rps"]; !ok || !(s.Median > 0) {
		t.Errorf("concurrency throughput missing or zero: %v", conc.Metrics)
	}
	cold := byPrefix("coldstart/45KB")
	if cold == nil {
		t.FailNow()
	}
	for _, m := range []string{"proxied_first_us", "proxied_steady_p50_us", "direct_first_us"} {
		if _, ok := cold.Metrics[m]; !ok {
			t.Errorf("coldstart missing %q; have %v", m, keys(cold.Metrics))
		}
	}
	sess := byPrefix("session/n10")
	if sess == nil {
		t.FailNow()
	}
	if _, ok := sess.Metrics["overhead_tail_paired_p50_us"]; !ok {
		t.Errorf("session scenario missing tail overhead; have %v", keys(sess.Metrics))
	}
}

func keys(m map[string]Spread) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestCommandWritesAResultAndRendersIt(t *testing.T) {
	if testing.Short() {
		t.Skip("starts real servers")
	}
	dir := t.TempDir()
	var out, errb strings.Builder
	if code := run([]string{"-quick", "-out", dir, "-noisy-label", "test"}, &out, &errb); code != exitOK {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	line, _, _ := strings.Cut(out.String(), "\n")
	path := strings.TrimPrefix(line, "result: ")
	if !strings.HasPrefix(path, dir) {
		t.Fatalf("no result path in output: %q", line)
	}
	var rendered strings.Builder
	if code := run([]string{"-render", path}, &rendered, &errb); code != exitOK {
		t.Fatalf("render exit %d: %s", code, errb.String())
	}
	if !strings.Contains(rendered.String(), "NOISY: test") {
		t.Errorf("rendered result lost its noise label:\n%s", rendered.String())
	}
	if code := run([]string{"-no-such-flag"}, &out, &errb); code != exitUsage {
		t.Errorf("bad flag exit = %d, want %d", code, exitUsage)
	}
}
