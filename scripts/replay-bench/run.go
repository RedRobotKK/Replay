package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/masking"
)

// Sizes, named as the brief names them. The 45KB body is the existing
// benchmark's body, byte for byte.
const (
	size45KB = 0 // requestBody's floor: exactly the 41-message benchRequest shape
	size1MB  = 1 << 20
	size16MB = 16 << 20
)

// goroutineSampleEvery is how often the concurrency suite samples the
// goroutine count to find its peak.
const goroutineSampleEvery = time.Millisecond

// usPerNS converts nanoseconds to the microseconds metrics are reported in.
const usPerNS = 1e3

// SizeSpec is one request-body size and how many samples it gets.
type SizeSpec struct {
	Name     string `json:"name"`
	MinBytes int    `json:"min_bytes"`
	// Pairs is interleaved direct/proxied timing pairs per repeat.
	Pairs int `json:"pairs"`
	// Batch is requests per arm in the allocation phase per repeat.
	Batch int `json:"batch"`
	// Warmup is unrecorded requests per arm at the start of each repeat.
	Warmup int `json:"warmup"`
	// ComponentIters is Masker.Mask calls per repeat in the component suite.
	ComponentIters int `json:"component_iters"`
}

// Config is every knob, recorded in the result.
type Config struct {
	Repeats        int        `json:"repeats"`
	Sizes          []SizeSpec `json:"sizes"`
	SSEEvents      int        `json:"sse_events"`
	SSEPairs       int        `json:"sse_pairs"`
	Concurrency    []int      `json:"concurrency"`
	PerWorker      int        `json:"per_worker"`
	SessionLengths []int      `json:"session_lengths"`
	SessionPairs   int        `json:"session_pairs"`
	ColdFirstN     int        `json:"cold_first_n"`
	ColdWarm       int        `json:"cold_warm"`
	ColdSteady     int        `json:"cold_steady"`
	LedgerIters    int        `json:"ledger_iters"`
	NoisyLabel     string     `json:"noisy_label,omitempty"`
}

// DefaultConfig is the baseline configuration: ten interleaved repeats.
func DefaultConfig() Config {
	return Config{
		Repeats: 10,
		Sizes: []SizeSpec{
			{Name: "45KB", MinBytes: size45KB, Pairs: 200, Batch: 100, Warmup: 10, ComponentIters: 200},
			{Name: "1MB", MinBytes: size1MB, Pairs: 30, Batch: 15, Warmup: 3, ComponentIters: 20},
			{Name: "16MB", MinBytes: size16MB, Pairs: 4, Batch: 3, Warmup: 1, ComponentIters: 3},
		},
		SSEEvents:      300,
		SSEPairs:       100,
		Concurrency:    []int{1, 8, 32},
		PerWorker:      16,
		SessionLengths: []int{10, 100, 250, 500, 1000},
		SessionPairs:   5,
		ColdFirstN:     5,
		ColdWarm:       20,
		ColdSteady:     20,
		LedgerIters:    200,
	}
}

// SmokeConfig is the smallest run that exercises every scenario, for tests.
func SmokeConfig() Config {
	return Config{
		Repeats:        2,
		Sizes:          []SizeSpec{{Name: "45KB", MinBytes: size45KB, Pairs: 4, Batch: 4, Warmup: 1, ComponentIters: 2}},
		SSEEvents:      20,
		SSEPairs:       3,
		Concurrency:    []int{1},
		PerWorker:      3,
		SessionLengths: []int{10},
		SessionPairs:   2,
		ColdFirstN:     2,
		ColdWarm:       2,
		ColdSteady:     3,
		LedgerIters:    3,
	}
}

// suite is one scenario: it runs once per repeat and keeps its own metrics.
type suite interface {
	repeat(r int) error
	scenario() *Scenario
	close() error
}

// collector accumulates per-repeat metric values for one scenario.
type collector struct{ s Scenario }

func newCollector(name string, params map[string]int64, notes string) collector {
	return collector{s: Scenario{Name: name, Params: params, Notes: notes, PerRepeat: map[string][]float64{}}}
}

func (c *collector) put(metric string, v float64) {
	c.s.PerRepeat[metric] = append(c.s.PerRepeat[metric], v)
}

func (c *collector) scenario() *Scenario {
	c.s.Metrics = map[string]Spread{}
	for k, v := range c.s.PerRepeat {
		c.s.Metrics[k] = SpreadOf(v)
	}
	return &c.s
}

func durs(ns []int64) []time.Duration {
	out := make([]time.Duration, len(ns))
	for i, v := range ns {
		out[i] = time.Duration(v)
	}
	return out
}

func us(ns int64) float64 { return float64(ns) / usPerNS }

// Execute runs every suite, repeat-major: repeat r runs every scenario once
// before repeat r+1 starts, so slow drift is spread across all scenarios
// instead of landing on whichever ran last.
func Execute(cfg Config, gitSHA string, args []string) (Result, error) {
	began := time.Now()
	res := Result{Schema: ResultSchema, Config: cfg}
	res.Environment = CaptureEnvironment(time.Now, gitSHA, args)

	up := newUpstream()
	defer up.Close()

	var suites []suite
	closeAll := func() error {
		var errs []error
		for _, s := range suites {
			errs = append(errs, s.close())
		}
		return errors.Join(errs...)
	}
	add := func(s suite, err error) error {
		if err != nil {
			return err
		}
		suites = append(suites, s)
		return nil
	}
	// Cold start runs first in every repeat, so repeat 0 also captures the
	// process's own first request through the proxy.
	if err := add(newColdSuite(cfg, up.URL), nil); err != nil {
		return res, err
	}
	for _, size := range cfg.Sizes {
		for _, mask := range []bool{false, true} {
			if err := add(newLatencySuite(size, mask, up.URL)); err != nil {
				return res, errors.Join(err, closeAll())
			}
		}
	}
	if err := add(newSSESuite(cfg, up.URL)); err != nil {
		return res, errors.Join(err, closeAll())
	}
	for _, c := range cfg.Concurrency {
		if err := add(newConcurrencySuite(cfg, c, up.URL)); err != nil {
			return res, errors.Join(err, closeAll())
		}
	}
	for _, n := range cfg.SessionLengths {
		if err := add(newSessionSuite(cfg, n, up.URL), nil); err != nil {
			return res, errors.Join(err, closeAll())
		}
	}
	for _, size := range cfg.Sizes {
		if err := add(newMaskComponentSuite(size)); err != nil {
			return res, errors.Join(err, closeAll())
		}
	}
	if err := add(newLedgerComponentSuite(cfg)); err != nil {
		return res, errors.Join(err, closeAll())
	}

	noisyRepeats := 0
	for r := range cfg.Repeats {
		load := loadAverage()
		meta := RepeatMeta{Index: r, StartedAt: time.Now().UTC().Format(time.RFC3339), Load: load, Noisy: Noisy(load, runtime.NumCPU())}
		if meta.Noisy {
			noisyRepeats++
		}
		res.Repeats = append(res.Repeats, meta)
		for _, s := range suites {
			if err := s.repeat(r); err != nil {
				return res, errors.Join(fmt.Errorf("%s, repeat %d: %w", s.scenario().Name, r, err), closeAll())
			}
		}
	}
	for _, s := range suites {
		res.Scenarios = append(res.Scenarios, *s.scenario())
	}
	if err := closeAll(); err != nil {
		return res, err
	}
	res.Environment.LoadAfter = loadAverage()
	res.Environment.MaxRSSBytes = maxRSS()
	if noisyRepeats > 0 {
		res.Noisy = true
		res.NoisyReason = fmt.Sprintf("%d of %d repeats started with 1-minute load above %.2f per CPU", noisyRepeats, cfg.Repeats, NoisyLoadPerCPU)
	}
	if cfg.NoisyLabel != "" {
		res.Noisy = true
		if res.NoisyReason != "" {
			res.NoisyReason = cfg.NoisyLabel + "; " + res.NoisyReason
		} else {
			res.NoisyReason = cfg.NoisyLabel
		}
	}
	res.DurationNS = time.Since(began).Nanoseconds()
	return res, nil
}

// pairedRun sends n interleaved pairs, rotating which arm goes first, and
// returns each arm's timings in pair order.
func pairedRun(n int, send map[string]func() (timing, error)) (map[string][]timing, error) {
	out := map[string][]timing{ArmDirect: make([]timing, 0, n), ArmProxied: make([]timing, 0, n)}
	for _, slot := range Order(n, []string{ArmDirect, ArmProxied}) {
		for _, arm := range slot {
			t, err := send[arm]()
			if err != nil {
				return nil, fmt.Errorf("%s: %w", arm, err)
			}
			out[arm] = append(out[arm], t)
		}
	}
	return out, nil
}

func pick(ts []timing, f func(timing) time.Duration) []int64 {
	out := make([]int64, len(ts))
	for i, t := range ts {
		out[i] = f(t).Nanoseconds()
	}
	return out
}

func total(t timing) time.Duration { return t.total }
func ttfb(t timing) time.Duration  { return t.ttfb }
func tail(t timing) time.Duration  { return t.tail }

// putOverhead records paired and percentile-difference overheads of one
// timing field under prefix.
func putOverhead(c *collector, prefix string, d, p []int64) error {
	po, err := PairedOverhead(durs(d), durs(p))
	if err != nil {
		return err
	}
	ds, ps := Summarize(durs(d)), Summarize(durs(p))
	pd := PercentileDifference(ds, ps)
	c.put("overhead"+prefix+"_paired_p50_us", us(po.P50NS))
	c.put("overhead"+prefix+"_paired_p95_us", us(po.P95NS))
	c.put("overhead"+prefix+"_paired_p99_us", us(po.P99NS))
	c.put("overhead"+prefix+"_p50_diff_us", us(pd.P50NS))
	c.put("overhead"+prefix+"_p99_diff_us", us(pd.P99NS))
	c.put("direct"+prefix+"_p50_us", us(ds.P50NS))
	c.put("proxied"+prefix+"_p50_us", us(ps.P50NS))
	c.put("proxied"+prefix+"_p99_us", us(ps.P99NS))
	return nil
}

// batch sends n sequential requests and measures the runtime counters
// across them.
func batch(n int, send func() (timing, error)) ([]timing, RuntimeDelta, error) {
	ts := make([]timing, 0, n)
	before := readSnapshot()
	for range n {
		t, err := send()
		if err != nil {
			return nil, RuntimeDelta{}, err
		}
		ts = append(ts, t)
	}
	return ts, delta(before, readSnapshot(), n), nil
}

// ---- latency: body size x masking --------------------------------------

type latencySuite struct {
	collector
	size     SizeSpec
	mask     bool
	upstream string
	client   *http.Client
	body     []byte
}

// sessionNote is the caveat every proxied suite carries. Without a session
// header the proxy keys a request by its SessionHash
// (internal/proxy/passthrough.go:218-220), so the identical requests a suite
// sends form one session that grows by one per request, and end-of-request
// rescore walks all of it. Each repeat therefore starts a fresh proxy, so
// every repeat sees the same growth from zero; the session suite measures
// the growth itself.
const sessionNote = " No session header, but the proxy derives one from the body (passthrough.go:218-220), so within a repeat the requests form one session growing from 0; each repeat starts a fresh proxy."

func newLatencySuite(size SizeSpec, mask bool, upstream string) (suite, error) {
	body := requestBody(size.MinBytes, false)
	m := "mask-off"
	if mask {
		m = "mask-on"
	}
	return &latencySuite{
		collector: newCollector(fmt.Sprintf("latency/%s/%s", size.Name, m),
			map[string]int64{"body_bytes": int64(len(body)), "pairs": int64(size.Pairs), "batch": int64(size.Batch), "warmup": int64(size.Warmup)},
			"Non-streaming JSON against an instant mock upstream."+sessionNote),
		size: size, mask: mask, upstream: upstream, client: newClient(), body: body,
	}, nil
}

// arms builds the send functions for one repeat's proxy.
func arms(client *http.Client, upstream string, px *proxyInstance, body []byte, sseEvents int, session string) map[string]func() (timing, error) {
	return map[string]func() (timing, error){
		ArmDirect:  func() (timing, error) { return do(client, upstream, body, sseEvents, session) },
		ArmProxied: func() (timing, error) { return px.send(client, body, sseEvents, session) },
	}
}

// freshProxy starts this repeat's proxy and records that it had served
// nothing, which is what makes repeats comparable.
func freshProxy(c *collector, upstream string, mask bool) (*proxyInstance, error) {
	px, err := startProxy(upstream, mask)
	if err != nil {
		return nil, err
	}
	c.put("proxied_prior_requests", float64(px.served.Load()))
	return px, nil
}

func (s *latencySuite) repeat(r int) (err error) {
	px, err := freshProxy(&s.collector, s.upstream, s.mask)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, px.Close()) }()
	send := arms(s.client, s.upstream, px, s.body, 0, "")
	for range s.size.Warmup {
		for _, arm := range []string{ArmDirect, ArmProxied} {
			if _, err := send[arm](); err != nil {
				return err
			}
		}
	}
	pairs, err := pairedRun(s.size.Pairs, send)
	if err != nil {
		return err
	}
	d, p := pick(pairs[ArmDirect], total), pick(pairs[ArmProxied], total)
	s.s.Runs = append(s.s.Runs,
		Run{Repeat: r, Arm: ArmDirect, Phase: "paired", SamplesNS: d, Latency: Summarize(durs(d))},
		Run{Repeat: r, Arm: ArmProxied, Phase: "paired", SamplesNS: p, Latency: Summarize(durs(p))})
	if err := putOverhead(&s.collector, "", d, p); err != nil {
		return err
	}
	rt := map[string]RuntimeDelta{}
	for _, arm := range Order(r+1, []string{ArmDirect, ArmProxied})[r] {
		ts, delta, err := batch(s.size.Batch, send[arm])
		if err != nil {
			return err
		}
		smp := pick(ts, total)
		s.s.Runs = append(s.s.Runs, Run{Repeat: r, Arm: arm, Phase: "batch", SamplesNS: smp, Latency: Summarize(durs(smp)), Runtime: &delta})
		rt[arm] = delta
	}
	for _, arm := range []string{ArmDirect, ArmProxied} {
		s.put(arm+"_allocs_per_op", rt[arm].AllocsPerOp)
		s.put(arm+"_bytes_per_op", rt[arm].BytesPerOp)
		s.put(arm+"_gc_cycles", float64(rt[arm].GCCycles))
		s.put(arm+"_gc_pause_est_us", rt[arm].GCPauseTotalEstNS/usPerNS)
	}
	s.put("overhead_allocs_per_op", rt[ArmProxied].AllocsPerOp-rt[ArmDirect].AllocsPerOp)
	s.put("overhead_bytes_per_op", rt[ArmProxied].BytesPerOp-rt[ArmDirect].BytesPerOp)
	return nil
}

func (s *latencySuite) close() error { s.client.CloseIdleConnections(); return nil }

// ---- SSE streaming -----------------------------------------------------

type sseSuite struct {
	collector
	cfg      Config
	upstream string
	client   *http.Client
	body     []byte
}

func newSSESuite(cfg Config, upstream string) (suite, error) {
	body := requestBody(size45KB, true)
	return &sseSuite{
		collector: newCollector(fmt.Sprintf("sse/45KB/events-%d", cfg.SSEEvents),
			map[string]int64{"body_bytes": int64(len(body)), "events": int64(cfg.SSEEvents), "pairs": int64(cfg.SSEPairs)},
			"Streaming. ttfb = first body byte; tail = the read completing message_stop to EOF, which includes the proxy's end-of-request bookkeeping."+sessionNote),
		cfg: cfg, upstream: upstream, client: newClient(), body: body,
	}, nil
}

func (s *sseSuite) repeat(r int) (err error) {
	px, err := freshProxy(&s.collector, s.upstream, false)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, px.Close()) }()
	send := arms(s.client, s.upstream, px, s.body, s.cfg.SSEEvents, "")
	for _, arm := range []string{ArmDirect, ArmProxied} {
		if _, err := send[arm](); err != nil {
			return err
		}
	}
	pairs, err := pairedRun(s.cfg.SSEPairs, send)
	if err != nil {
		return err
	}
	return recordStream(&s.collector, r, pairs)
}

func recordStream(c *collector, r int, pairs map[string][]timing) error {
	for _, arm := range []string{ArmDirect, ArmProxied} {
		smp := pick(pairs[arm], total)
		c.s.Runs = append(c.s.Runs, Run{Repeat: r, Arm: arm, Phase: "paired", SamplesNS: smp,
			TTFBNS: pick(pairs[arm], ttfb), TailNS: pick(pairs[arm], tail), Latency: Summarize(durs(smp))})
	}
	for _, f := range []struct {
		prefix string
		get    func(timing) time.Duration
	}{{"_total", total}, {"_ttfb", ttfb}, {"_tail", tail}} {
		if err := putOverhead(c, f.prefix, pick(pairs[ArmDirect], f.get), pick(pairs[ArmProxied], f.get)); err != nil {
			return err
		}
	}
	return nil
}

func (s *sseSuite) close() error { s.client.CloseIdleConnections(); return nil }

// ---- throughput at concurrency c ---------------------------------------

type concurrencySuite struct {
	collector
	c, perWorker int
	upstream     string
	client       *http.Client
	body         []byte
	send         map[string]func() (timing, error)
}

func newConcurrencySuite(cfg Config, c int, upstream string) (suite, error) {
	body := requestBody(size45KB, false)
	return &concurrencySuite{
		collector: newCollector(fmt.Sprintf("concurrency/c%d", c),
			map[string]int64{"workers": int64(c), "requests_per_arm": int64(c * cfg.PerWorker), "body_bytes": int64(len(body))},
			"Closed loop: c workers each send per_worker requests back to back. Arms alternate order by repeat (ABBA). rps = requests / wall time."+sessionNote),
		c: c, perWorker: cfg.PerWorker, upstream: upstream, client: newClient(), body: body,
	}, nil
}

func (s *concurrencySuite) runArm(arm string) ([]int64, time.Duration, RuntimeDelta, error) {
	var mu sync.Mutex
	samples := make([]int64, 0, s.c*s.perWorker)
	var firstErr error
	peak := runtime.NumGoroutine()
	stop := make(chan struct{})
	sampled := make(chan struct{})
	go func() {
		defer close(sampled)
		tk := time.NewTicker(goroutineSampleEvery)
		defer tk.Stop()
		for {
			select {
			case <-stop:
				return
			case <-tk.C:
				if g := runtime.NumGoroutine(); g > peak {
					peak = g
				}
			}
		}
	}()
	before := readSnapshot()
	start := time.Now()
	var wg sync.WaitGroup
	for range s.c {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range s.perWorker {
				t, err := s.send[arm]()
				mu.Lock()
				if err != nil && firstErr == nil {
					firstErr = err
				}
				samples = append(samples, t.total.Nanoseconds())
				mu.Unlock()
				if err != nil {
					return
				}
			}
		}()
	}
	wg.Wait()
	wall := time.Since(start)
	d := delta(before, readSnapshot(), s.c*s.perWorker)
	close(stop)
	<-sampled
	d.GoroutinesPeak = peak
	return samples, wall, d, firstErr
}

func (s *concurrencySuite) repeat(r int) (err error) {
	px, err := freshProxy(&s.collector, s.upstream, false)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, px.Close()) }()
	s.send = arms(s.client, s.upstream, px, s.body, 0, "")
	// One unrecorded round per arm warms both connection pools to c.
	for _, arm := range []string{ArmDirect, ArmProxied} {
		if _, _, _, err := s.runArm(arm); err != nil {
			return err
		}
	}
	rps := map[string]float64{}
	sum := map[string]Summary{}
	rt := map[string]RuntimeDelta{}
	for _, arm := range Order(r+1, []string{ArmDirect, ArmProxied})[r] {
		smp, wall, d, err := s.runArm(arm)
		if err != nil {
			return err
		}
		sum[arm] = Summarize(durs(smp))
		rps[arm] = float64(len(smp)) / wall.Seconds()
		rt[arm] = d
		s.s.Runs = append(s.s.Runs, Run{Repeat: r, Arm: arm, Phase: "batch", SamplesNS: smp, Latency: sum[arm], WallNS: wall.Nanoseconds(), Runtime: &d})
	}
	for _, arm := range []string{ArmDirect, ArmProxied} {
		s.put(arm+"_rps", rps[arm])
		s.put(arm+"_p50_us", us(sum[arm].P50NS))
		s.put(arm+"_p99_us", us(sum[arm].P99NS))
		s.put(arm+"_allocs_per_op", rt[arm].AllocsPerOp)
		s.put(arm+"_bytes_per_op", rt[arm].BytesPerOp)
		s.put(arm+"_gc_cycles", float64(rt[arm].GCCycles))
		s.put(arm+"_goroutines_peak", float64(rt[arm].GoroutinesPeak))
	}
	s.put("rps_ratio_proxied_over_direct", rps[ArmProxied]/rps[ArmDirect])
	s.put("overhead_p50_diff_us", us(sum[ArmProxied].P50NS-sum[ArmDirect].P50NS))
	s.put("overhead_p99_diff_us", us(sum[ArmProxied].P99NS-sum[ArmDirect].P99NS))
	return nil
}

func (s *concurrencySuite) close() error { s.client.CloseIdleConnections(); return nil }

// ---- session length: end-of-stream bookkeeping --------------------------

type sessionSuite struct {
	collector
	cfg      Config
	n        int
	upstream string
	client   *http.Client
}

func newSessionSuite(cfg Config, n int, upstream string) suite {
	return &sessionSuite{
		collector: newCollector(fmt.Sprintf("session/n%d", n),
			map[string]int64{"session_length": int64(n), "pairs": int64(cfg.SessionPairs), "events": int64(cfg.SSEEvents)},
			"A fresh proxy per repeat; n-1 JSON turns build one session (x-claude-code-session-id), then SSE pairs are measured at length n..n+pairs-1. tail = message_stop read to EOF; the proxy's ledger append and rescore run before the stream's final chunk."),
		cfg: cfg, n: n, upstream: upstream, client: newClient(),
	}
}

func (s *sessionSuite) repeat(r int) error {
	px, err := startProxy(s.upstream, false)
	if err != nil {
		return err
	}
	defer func() { _ = px.Close() }() // reported below on the success path
	session := fmt.Sprintf("bench-session-%d-%d", s.n, r)
	jsonBody := requestBody(size45KB, false)
	sseBody := requestBody(size45KB, true)
	buildStart := time.Now()
	for range s.n - 1 {
		if _, err := px.send(s.client, jsonBody, 0, session); err != nil {
			return err
		}
	}
	if s.n > 1 {
		s.put("build_mean_request_us", float64(time.Since(buildStart).Nanoseconds())/float64(s.n-1)/usPerNS)
	}
	send := arms(s.client, s.upstream, px, sseBody, s.cfg.SSEEvents, session)
	pairs, err := pairedRun(s.cfg.SessionPairs, send)
	if err != nil {
		return err
	}
	if err := recordStream(&s.collector, r, pairs); err != nil {
		return err
	}
	return px.Close()
}

func (s *sessionSuite) close() error { s.client.CloseIdleConnections(); return nil }

// ---- cold start --------------------------------------------------------

type coldSuite struct {
	collector
	cfg      Config
	upstream string
}

func newColdSuite(cfg Config, upstream string) suite {
	return &coldSuite{
		collector: newCollector("coldstart/45KB",
			map[string]int64{"first_n": int64(cfg.ColdFirstN), "warm": int64(cfg.ColdWarm), "steady": int64(cfg.ColdSteady)},
			"Each repeat: a fresh client transport and, for the proxied arm, a freshly started proxy. first = the first request (new TCP connections on every hop); first_n = the first first_n; steady = after warm unrecorded requests. Repeat 0 also includes this process's first request through the proxy code."),
		cfg: cfg, upstream: upstream,
	}
}

func (s *coldSuite) repeat(r int) error {
	body := requestBody(size45KB, false)
	for _, arm := range Order(r+1, []string{ArmDirect, ArmProxied})[r] {
		cl := newClient()
		base := s.upstream
		var px *proxyInstance
		if arm == ArmProxied {
			var err error
			if px, err = startProxy(s.upstream, false); err != nil {
				return err
			}
			base = px.base
			s.put("proxied_startup_us", float64(px.startup.Nanoseconds())/usPerNS)
		}
		var first []int64
		var steady []int64
		var err error
		for i := 0; err == nil && i < s.cfg.ColdFirstN+s.cfg.ColdWarm+s.cfg.ColdSteady; i++ {
			var t timing
			t, err = do(cl, base, body, 0, "")
			switch {
			case i < s.cfg.ColdFirstN:
				first = append(first, t.total.Nanoseconds())
			case i >= s.cfg.ColdFirstN+s.cfg.ColdWarm:
				steady = append(steady, t.total.Nanoseconds())
			}
		}
		cl.CloseIdleConnections()
		if px != nil {
			err = errors.Join(err, px.Close())
		}
		if err != nil {
			return err
		}
		fs, ss := Summarize(durs(first)), Summarize(durs(steady))
		s.s.Runs = append(s.s.Runs,
			Run{Repeat: r, Arm: arm, Phase: "cold-first-n", SamplesNS: first, Latency: fs},
			Run{Repeat: r, Arm: arm, Phase: "steady", SamplesNS: steady, Latency: ss})
		s.put(arm+"_first_us", us(first[0]))
		s.put(arm+"_first_n_p50_us", us(fs.P50NS))
		s.put(arm+"_steady_p50_us", us(ss.P50NS))
		s.put(arm+"_cold_minus_steady_us", us(first[0]-ss.P50NS))
	}
	return nil
}

func (s *coldSuite) close() error { return nil }

// ---- component timings, through exported APIs only ----------------------
//
// These call the same exported functions the handler calls, outside the
// handler. They change no production code path; the per-phase spans that
// would need one are listed as a proposal in the evidence file instead.

type maskComponentSuite struct {
	collector
	size SizeSpec
	body []byte
	mask *masking.Masker
	dir  string
}

func newMaskComponentSuite(size SizeSpec) (suite, error) {
	dir, err := os.MkdirTemp("", "replay-bench-vault-")
	if err != nil {
		return nil, err
	}
	vault, err := masking.OpenVault(dir)
	if err != nil {
		return nil, errors.Join(err, os.RemoveAll(dir))
	}
	body := requestBody(size.MinBytes, false)
	return &maskComponentSuite{
		collector: newCollector("component/mask/"+size.Name,
			map[string]int64{"body_bytes": int64(len(body)), "iters": int64(size.ComponentIters)},
			"masking.Masker.Mask on the request body, called directly: the scan the proxy runs when masking is on. The body holds no secret, so this is the no-match cost."),
		size: size, body: body, mask: masking.New(vault, nil), dir: dir,
	}, nil
}

func (s *maskComponentSuite) repeat(int) error {
	before := readSnapshot()
	start := time.Now()
	for range s.size.ComponentIters {
		if _, _, err := s.mask.Mask(s.body); err != nil {
			return err
		}
	}
	el := time.Since(start)
	d := delta(before, readSnapshot(), s.size.ComponentIters)
	s.put("ns_per_op", float64(el.Nanoseconds())/float64(s.size.ComponentIters))
	s.put("allocs_per_op", d.AllocsPerOp)
	s.put("bytes_per_op", d.BytesPerOp)
	return nil
}

func (s *maskComponentSuite) close() error { return os.RemoveAll(s.dir) }

type ledgerComponentSuite struct {
	collector
	iters int
	store *ledger.Store
	dir   string
	seq   int
}

func newLedgerComponentSuite(cfg Config) (suite, error) {
	dir, err := os.MkdirTemp("", "replay-bench-ledger-")
	if err != nil {
		return nil, err
	}
	store, err := ledger.Open(filepath.Join(dir, "ledger"))
	if err != nil {
		return nil, errors.Join(err, os.RemoveAll(dir))
	}
	return &ledgerComponentSuite{
		collector: newCollector("component/ledger-append",
			map[string]int64{"iters": int64(cfg.LedgerIters)},
			"ledger.Store.Append of one minimal record, called directly: the local write the handler makes per request that carries a session id. Includes the filesystem."),
		iters: cfg.LedgerIters, store: store, dir: dir,
	}, nil
}

func (s *ledgerComponentSuite) repeat(int) error {
	base := time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)
	before := readSnapshot()
	start := time.Now()
	for range s.iters {
		s.seq++
		rec := ledger.Record{
			Schema: ledger.SchemaVersion, Timestamp: base.Add(time.Duration(s.seq) * time.Second),
			SessionID: "bench-ledger", RequestID: fmt.Sprintf("req-%d", s.seq), Path: messagesPath,
			Status: http.StatusOK, LatencyMS: 1,
		}
		rec.Model = "claude-fable-5-1"
		if err := s.store.Append(rec); err != nil {
			return err
		}
	}
	el := time.Since(start)
	d := delta(before, readSnapshot(), s.iters)
	s.put("ns_per_op", float64(el.Nanoseconds())/float64(s.iters))
	s.put("allocs_per_op", d.AllocsPerOp)
	s.put("bytes_per_op", d.BytesPerOp)
	return nil
}

func (s *ledgerComponentSuite) close() error { return os.RemoveAll(s.dir) }
