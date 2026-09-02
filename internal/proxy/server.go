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
)

// Timeouts. Provider turns on frontier models can run for minutes, so the
// response header timeout is long and there is no overall request timeout;
// the client owns that decision.
const (
	ReadHeaderTimeout     = 30 * time.Second
	ResponseHeaderTimeout = 10 * time.Minute
	IdleConnTimeout       = 90 * time.Second
	ShutdownTimeout       = 5 * time.Second
	// TLSHandshakeTimeout bounds the upstream handshake.
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
}

// Server is the running proxy.
type Server struct {
	cfg   Config
	http  *http.Server
	rp    *httputil.ReverseProxy
	ready chan struct{}
	addr  string
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
	s := &Server{cfg: cfg, ready: make(chan struct{})}
	s.rp = &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(cfg.Upstream)
			r.Out.Host = cfg.Upstream.Host
			// The default rewrite appends the client address; the provider
			// has no use for it and the request should carry nothing the
			// client did not send.
			r.Out.Header.Del("X-Forwarded-For")
		},
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           (&net.Dialer{Timeout: 30 * time.Second}).DialContext,
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
			http.Error(w, "buffy: upstream request failed: "+err.Error()+"\nTo bypass Buffy, unset ANTHROPIC_BASE_URL.", http.StatusBadGateway)
		},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/buffy/healthz", s.health)
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

// handle is the passthrough. It rejects browser-originated calls, checks the
// optional token, summarizes the request, forwards it, taps the response,
// and records the ledger entry. Any failure inside the tap is logged and
// the bytes still flow.
func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Origin") != "" || r.Header.Get("Sec-Fetch-Mode") != "" {
		http.Error(w, "buffy: browser-originated requests are not accepted", http.StatusForbidden)
		return
	}
	if s.cfg.Token != "" && r.Header.Get(HeaderToken) != s.cfg.Token {
		http.Error(w, "buffy: missing or wrong "+HeaderToken+" header", http.StatusUnauthorized)
		return
	}

	start := time.Now()
	rec := ledger.Record{Timestamp: start, Path: r.URL.Path, SessionID: r.Header.Get(HeaderSessionID), AgentID: r.Header.Get(HeaderAgentID)}

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

	if isMessages(r.URL.Path) && len(body) > 0 {
		prompt, model, stream, effort, err := ledger.SummarizeRequest(body)
		if err == nil {
			rec.Prompt, rec.Model, rec.Stream, rec.Effort = prompt, model, stream, effort
		}
		if rec.SessionID == "" {
			rec.SessionID = prefixHash(body)
		}
	}

	tap := &responseTap{ResponseWriter: w}
	s.rp.ServeHTTP(tap, r)

	rec.Status = tap.status
	rec.LatencyMS = time.Since(start).Milliseconds()
	rec.RequestID = tap.Header().Get("request-id")
	if isMessages(r.URL.Path) {
		rec.Response = tap.result()
	}
	if rec.SessionID != "" && isMessages(r.URL.Path) {
		if err := s.cfg.Store.Append(rec); err != nil {
			s.cfg.Logger.Printf("ledger write failed: %v", err)
		}
	}
	s.cfg.Logger.Printf("%s %s status=%d ms=%d session=%s model=%s %s", r.Method, r.URL.Path, rec.Status, rec.LatencyMS, short(rec.SessionID), rec.Model, usageSummary(rec.Response.Usage))
}

// isMessages reports whether a path is the Messages endpoint proper (not
// count_tokens), which is the only one whose responses carry usage.
func isMessages(path string) bool {
	return strings.HasSuffix(path, "/v1/messages")
}

// prefixHash derives a session id from the request body's stable prefix
// when the client sent no session header: the system prompt and the first
// message. It contains no content.
func prefixHash(body []byte) string {
	var probe struct {
		System   any              `json:"system"`
		Messages []map[string]any `json:"messages"`
	}
	if err := jsonUnmarshal(body, &probe); err != nil || len(probe.Messages) == 0 {
		return ""
	}
	h := sha256.New()
	_, _ = fmt.Fprint(h, probe.System)      // hashing cannot fail
	_, _ = fmt.Fprint(h, probe.Messages[0]) // hashing cannot fail
	return "prefix-" + hex.EncodeToString(h.Sum(nil))[:16]
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
	status  int
	stream  *ledger.StreamParser
	buffer  bytes.Buffer
	gz      bool
	dropped bool
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
