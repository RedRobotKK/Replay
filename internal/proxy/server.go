// Package proxy is the local gateway: it forwards requests to the provider
// byte for byte, taps responses for usage and structure without delaying
// them, and records derived data in the ledger.
//
// Nothing here rewrites a request body or removes a client header. The
// invariants are in the repository CLAUDE.md; the client-side facts the
// proxy honors are in docs/architecture/proxy-protocol.md.
package proxy

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/RedRobotKK/Buffy/internal/ledger"
	"github.com/RedRobotKK/Buffy/internal/policy"
)

// Timeouts. Provider turns on frontier models can run for minutes, so the
// response header timeout is long and there is no overall request timeout;
// the client owns that decision.
const (
	ReadHeaderTimeout     = 30 * time.Second
	ResponseHeaderTimeout = 10 * time.Minute
	IdleConnTimeout       = 90 * time.Second
	ShutdownTimeout       = 5 * time.Second
	// DialTimeout and TLSHandshakeTimeout bound connecting to the upstream.
	DialTimeout         = 30 * time.Second
	TLSHandshakeTimeout = 30 * time.Second
)

// Body limits for what the tap keeps in memory. Requests are read fully
// to summarize them (they are already fully in the client's memory);
// non-streaming responses are buffered up to the cap for parsing.
const (
	MaxRequestBytes  = 64 << 20
	MaxResponseBytes = 16 << 20
)

// Client headers the proxy reads for attribution. They are forwarded
// unchanged as well.
const (
	HeaderSessionID = "x-claude-code-session-id"
	HeaderAgentID   = "x-claude-code-agent-id"
	HeaderToken     = "x-buffy-token"
)

// Config is everything serve needs.
type Config struct {
	// Listen is the loopback address to bind.
	Listen string
	// Upstream is the provider base URL.
	Upstream *url.URL
	// Token, when set, must match HeaderToken on every request.
	Token string
	// Store receives one record per proxied request.
	Store *ledger.Store
	// Logger receives one line per request. Never headers, never bodies.
	Logger *log.Logger
	// Guards are optional; a nil guard is off.
	Spend   *SpendGuard
	Loops   LoopLimits
	Breaker *Breaker
	// ContextEdit, when set, is applied to sessions whose first request
	// admits it and pinned for their life (ADR-0003). Nil is off.
	ContextEdit *policy.ContextEdit
}

// HealthPath answers "ok" for anything that wants to know the proxy is up.
// StatusPath and MetricsPath are the read endpoints.
const (
	HealthPath  = "/buffy/healthz"
	StatusPath  = "/buffy/status"
	MetricsPath = "/buffy/metrics"
)

// HeaderOverride is the header a client sets to acknowledge a spend cap or
// a loop block and proceed once. Its value is logged as the reason.
const HeaderOverride = "x-buffy-override"

// HeaderWarning is added to a forwarded response when a guard has
// something to say but did not block. It is the only header Buffy adds to
// a response.
const HeaderWarning = "x-buffy-warning"

// Server is the running proxy.
type Server struct {
	cfg   Config
	http  *http.Server
	rp    *httputil.ReverseProxy
	ready chan struct{}
	addr  string
	stats *stats
}

// New builds a server. It does not listen yet.
func New(cfg Config) (*Server, error) {
	if cfg.Upstream == nil {
		return nil, errors.New("upstream URL is required")
	}
	if cfg.Store == nil {
		return nil, errors.New("ledger store is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = log.New(io.Discard, "", 0)
	}
	if !isLoopback(cfg.Listen) {
		return nil, fmt.Errorf("listen address %q is not loopback; Buffy only binds locally", cfg.Listen)
	}
	s := &Server{cfg: cfg, ready: make(chan struct{}), stats: newStats()}
	s.rp = &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(cfg.Upstream)
			r.Out.Host = cfg.Upstream.Host
			// A Rewrite adds no forwarding headers of its own, so the
			// request carries only what the client sent, minus Buffy's own
			// listener token, which is not the client's header to the
			// provider.
			r.Out.Header.Del(HeaderToken)
		},
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           (&net.Dialer{Timeout: DialTimeout}).DialContext,
			TLSHandshakeTimeout:   TLSHandshakeTimeout,
			ResponseHeaderTimeout: ResponseHeaderTimeout,
			IdleConnTimeout:       IdleConnTimeout,
			// The client decides whether it accepts compressed responses;
			// the transport must not add its own header and decompress
			// behind the client's back.
			DisableCompression: true,
		},
		// Flush every write so streamed events reach the client as they
		// arrive.
		FlushInterval: -1,
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			cfg.Logger.Printf("upstream error: %v", err)
			if tap, ok := w.(*responseTap); ok {
				tap.upstreamFailed = true
			}
			http.Error(w, "buffy: upstream request failed: "+err.Error()+"\nTo bypass Buffy, unset ANTHROPIC_BASE_URL.", http.StatusBadGateway)
		},
	}
	mux := http.NewServeMux()
	mux.HandleFunc(HealthPath, s.health)
	mux.HandleFunc(StatusPath, s.status)
	mux.HandleFunc(MetricsPath, s.metrics)
	mux.HandleFunc("/", s.handle)
	s.http = &http.Server{Handler: mux, ReadHeaderTimeout: ReadHeaderTimeout}
	return s, nil
}

// ListenAndServe binds and serves until the context is cancelled.
func (s *Server) ListenAndServe(ctx context.Context) error {
	ln, err := net.Listen("tcp", s.cfg.Listen)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.cfg.Listen, err)
	}
	s.addr = ln.Addr().String()
	close(s.ready)
	errc := make(chan error, 1)
	go func() { errc <- s.http.Serve(ln) }()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
		defer cancel()
		return s.http.Shutdown(shutdownCtx)
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// Addr is the bound address, valid after ListenAndServe has started.
func (s *Server) Addr() string {
	<-s.ready
	return s.addr
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = io.WriteString(w, "ok\n") // best-effort health response
}

// localOnly guards the read endpoints the same way requests are guarded:
// no browser origins, and the token when one is configured.
func (s *Server) localOnly(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("Origin") != "" || r.Header.Get("Sec-Fetch-Mode") != "" {
		http.Error(w, "buffy: browser-originated requests are not accepted", http.StatusForbidden)
		return false
	}
	if s.cfg.Token != "" && r.Header.Get(HeaderToken) != s.cfg.Token {
		http.Error(w, "buffy: missing or wrong "+HeaderToken+" header", http.StatusUnauthorized)
		return false
	}
	return true
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	if !s.localOnly(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	// A failed write means the reader went away; nothing to do.
	_ = json.NewEncoder(w).Encode(s.stats.status())
}

func (s *Server) metrics(w http.ResponseWriter, r *http.Request) {
	if !s.localOnly(w, r) {
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = io.WriteString(w, s.stats.metrics()) // best-effort scrape response
}

// handle is the passthrough. It rejects browser-originated calls, checks the
// optional token, summarizes the request, forwards it, taps the response,
// and records the ledger entry. Any failure inside the tap is logged and
// the bytes still flow.
func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	if !s.localOnly(w, r) {
		return
	}

	start := time.Now()
	messages := isMessages(r.URL.Path)
	rec := ledger.Record{Timestamp: start, Path: r.URL.Path, SessionID: r.Header.Get(HeaderSessionID), AgentID: r.Header.Get(HeaderAgentID)}

	if ok, wait := s.cfg.Breaker.Allow(); !ok {
		s.refuse(w, refusalCircuitOpen, fmt.Sprintf("the provider has been failing; Buffy is holding requests for %s so the agent stops burning retries", wait.Round(time.Second)), wait)
		return
	}
	// A half-open probe that never reaches an outcome (refused below, or
	// aborted) is given back so the next request can probe instead.
	observed := false
	defer func() {
		if !observed {
			s.cfg.Breaker.Release()
		}
	}()

	body, err := io.ReadAll(io.LimitReader(r.Body, MaxRequestBytes+1))
	if err != nil {
		http.Error(w, "buffy: could not read request body", http.StatusBadRequest)
		return
	}
	if len(body) > MaxRequestBytes {
		http.Error(w, "buffy: request body exceeds the proxy limit", http.StatusRequestEntityTooLarge)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))

	if messages && len(body) > 0 {
		if sum, err := ledger.SummarizeRequest(body, s.cfg.Store.Labeler()); err == nil {
			rec.RequestSummary = sum
		}
		if rec.SessionID == "" {
			rec.SessionID = rec.SessionHash
		}
		if !s.guard(w, r, &rec) {
			return
		}
		body = s.applyPolicy(r, &rec, body)
		r.Body = io.NopCloser(bytes.NewReader(body))
		r.ContentLength = int64(len(body))
	}

	tap := &responseTap{ResponseWriter: w}
	// Bookkeeping runs in a deferred function because the reverse proxy
	// aborts the handler with a panic when the client goes away mid-stream
	// (the user interrupting a turn). The provider still billed that turn,
	// so it must still be observed, counted, and recorded; the panic is
	// then re-raised for the server to handle as it normally does.
	defer func() {
		aborted := recover()
		s.cfg.Breaker.Observe(tap.upstreamFailed || IsRetryableStatus(tap.status))
		observed = true
		rec.Status = tap.status
		rec.LatencyMS = time.Since(start).Milliseconds()
		rec.RequestID = tap.Header().Get("request-id")
		if messages {
			rec.Response = tap.result()
			if u := rec.Response.Usage; u != nil {
				s.cfg.Spend.Record(rec.SessionID, u.Input+u.CacheCreation+u.CacheRead+u.Output)
			}
		}
		rec.Cache = s.stats.observe(&rec)
		if messages && rec.SessionID != "" {
			if err := s.cfg.Store.Append(rec); err != nil {
				s.cfg.Logger.Printf("ledger write failed: %v", err)
			}
		}
		whatIf := ""
		if messages && rec.SessionID != "" && rec.Response.Usage != nil {
			whatIf = s.stats.rescore(&rec)
		}
		note := ""
		if aborted != nil {
			note = " aborted=client-disconnected"
		}
		s.cfg.Logger.Printf("%s %s status=%d ms=%d session=%s model=%s %s%s", r.Method, r.URL.Path, rec.Status, rec.LatencyMS, short(rec.SessionID), rec.Model, usageSummary(rec.Response.Usage), note)
		if rec.Cache != nil && rec.Cache.Deficit > 0 {
			s.cfg.Logger.Printf("cache break session=%s: read %d of %d expected, %d tokens re-billed; likely cause: %s", short(rec.SessionID), rec.Response.Usage.CacheRead, rec.Cache.Expected, rec.Cache.Deficit, rec.Cache.Cause)
		}
		if whatIf != "" {
			s.cfg.Logger.Print(whatIf)
		}
		if aborted != nil {
			panic(aborted)
		}
	}()
	s.rp.ServeHTTP(tap, r)
}

// applyPolicy adds the configured request parameter when the session's
// pinned decision allows it. The decision is made at the session's first
// request and kept: a session either always carries the parameter or
// never does. Every transformation is logged with the body hashes before
// and after (PX-10), never the bodies.
func (s *Server) applyPolicy(r *http.Request, rec *ledger.Record, body []byte) []byte {
	p := s.cfg.ContextEdit
	if p == nil || rec.SessionID == "" {
		return body
	}
	admissible := p.Admissible(r.Header.Get("anthropic-beta"), rec.Prompt.ContextEdits)
	decision := s.stats.pinPolicy(rec.SessionID, admissible)
	if decision != policy.Applied {
		return body
	}
	if admissible != policy.Applied {
		// Pinned on, but this request cannot carry the parameter.
		s.cfg.Logger.Printf("policy %s session=%s %s", policy.Name, short(rec.SessionID), admissible)
		return body
	}
	out, applied := p.Apply(body)
	if applied != policy.Applied {
		s.cfg.Logger.Printf("policy %s session=%s %s", policy.Name, short(rec.SessionID), applied)
		return body
	}
	rec.Policy = policy.Name
	s.cfg.Logger.Printf("policy %s session=%s applied body sha256 before=%s after=%s", p, short(rec.SessionID), bodyHash(body), bodyHash(out))
	return out
}

// bodyHash is a content-free fingerprint of a request body for the log.
func bodyHash(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])[:16]
}

// guard applies the spend cap and loop detector to a summarized request.
// It reports false when the request was answered locally.
func (s *Server) guard(w http.ResponseWriter, r *http.Request, rec *ledger.Record) bool {
	override := r.Header.Get(HeaderOverride)
	if reason := s.cfg.Spend.Check(rec.SessionID); reason != "" {
		if override == "" {
			s.refuse(w, refusalSpendCap, reason+". Raise the cap, start a new session, or send "+HeaderOverride+" with a reason to proceed once.", 0)
			return false
		}
		s.cfg.Logger.Printf("spend cap overridden for session=%s: %s", short(rec.SessionID), override)
	}
	v := DetectLoop(rec.Prompt, s.cfg.Loops)
	switch {
	case v.Block && override == "":
		s.refuse(w, refusalLoop, fmt.Sprintf("the same %s call was just made %d times in a row; Buffy stopped the loop. Send %s with a reason to proceed once.", v.Label, v.Repeats, HeaderOverride), 0)
		return false
	case v.Block:
		s.cfg.Logger.Printf("loop block overridden for session=%s: %s", short(rec.SessionID), override)
	case v.Warn:
		w.Header().Set(HeaderWarning, fmt.Sprintf("loop: the same %s call was just made %d times in a row", v.Label, v.Repeats))
	}
	return true
}

// isMessages reports whether a path is the Messages endpoint proper (not
// count_tokens), which is the only one whose responses carry usage.
func isMessages(path string) bool {
	return strings.HasSuffix(path, "/v1/messages")
}

// refusal is one way Buffy answers a request itself instead of forwarding
// it: the status it sends, the error type a provider-aware client shows,
// and the counter it lands in.
type refusal struct {
	status  int
	errType string
	counter string
}

var (
	refusalCircuitOpen = refusal{http.StatusServiceUnavailable, "buffy_circuit_open", "circuit_open"}
	refusalSpendCap    = refusal{http.StatusBadRequest, "buffy_spend_cap", "spend_cap"}
	refusalLoop        = refusal{http.StatusBadRequest, "buffy_loop", "loop"}
)

// refuse answers a request locally in the provider's error shape so any
// client that understands provider errors shows the message to the user.
func (s *Server) refuse(w http.ResponseWriter, kind refusal, message string, retryAfter time.Duration) {
	s.stats.refused(kind.counter)
	w.Header().Set("Content-Type", "application/json")
	if retryAfter > 0 {
		w.Header().Set("Retry-After", fmt.Sprintf("%d", int(retryAfter.Seconds())+1))
	}
	w.WriteHeader(kind.status)
	body := map[string]any{"type": "error", "error": map[string]string{"type": kind.errType, "message": message}}
	// A failed write here means the client went away; nothing to do.
	_ = json.NewEncoder(w).Encode(body)
}

func isLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func short(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

func usageSummary(u *ledger.Usage) string {
	if u == nil {
		return "usage=none"
	}
	return fmt.Sprintf("in=%d write=%d read=%d out=%d", u.Input, u.CacheCreation, u.CacheRead, u.Output)
}

// responseTap passes every byte to the client immediately and keeps a
// parser fed on the side. For event streams the parser consumes lines as
// they pass; for JSON responses the body is buffered up to the cap.
type responseTap struct {
	http.ResponseWriter
	// upstreamFailed is set by the error handler when no response arrived.
	upstreamFailed bool
	status         int
	stream         *ledger.StreamParser
	buffer         bytes.Buffer
	gz             bool
	dropped        bool
}

func (t *responseTap) WriteHeader(code int) {
	t.status = code
	ct := t.Header().Get("Content-Type")
	t.gz = strings.EqualFold(t.Header().Get("Content-Encoding"), "gzip")
	if ledger.IsEventStream(ct) && !t.gz {
		t.stream = &ledger.StreamParser{}
	}
	t.ResponseWriter.WriteHeader(code)
}

func (t *responseTap) Write(p []byte) (int, error) {
	if t.status == 0 {
		t.WriteHeader(http.StatusOK)
	}
	n, err := t.ResponseWriter.Write(p)
	// The tap must never affect delivery: parse after forwarding, and stop
	// buffering rather than grow without bound.
	switch {
	case t.stream != nil:
		_, _ = t.stream.Write(p[:n]) // StreamParser.Write never fails
	case t.buffer.Len()+n <= MaxResponseBytes:
		t.buffer.Write(p[:n])
	default:
		t.dropped = true
	}
	return n, err
}

// Flush forwards flushes so streamed events are not held by buffering.
func (t *responseTap) Flush() {
	if f, ok := t.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (t *responseTap) result() ledger.Response {
	if t.stream != nil {
		return t.stream.Result()
	}
	if t.dropped {
		return ledger.Response{}
	}
	body := t.buffer.Bytes()
	if t.gz {
		zr, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return ledger.Response{}
		}
		decoded, err := io.ReadAll(io.LimitReader(zr, MaxResponseBytes))
		if err != nil {
			return ledger.Response{}
		}
		body = decoded
	}
	if ledger.IsEventStream(t.Header().Get("Content-Type")) {
		// A gzip-compressed event stream: parse the decoded body whole.
		sp := &ledger.StreamParser{}
		_, _ = sp.Write(body) // StreamParser.Write never fails
		return sp.Result()
	}
	return ledger.ParseResponse(body)
}
