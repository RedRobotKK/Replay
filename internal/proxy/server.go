// Package proxy is the local gateway: it forwards requests to the provider,
// taps responses for usage and structure without delaying them, and records
// derived data in the ledger.
//
// IT REWRITES THE REQUEST BODY IN FOUR PLACES, and this comment said it did
// not. The line read "Nothing here rewrites a request body or removes a client
// header" while the same file masked secrets, applied the context-edit policy,
// and added include-usage — and stripped two client headers, x-replay-token
// and Accept-Encoding. Recorded as F2 in docs/design/surface-taxonomy-1-request.md
// on 2026-09-11 and left standing as a disagreement between code and doc; this
// is the doc conceding.
//
// It mattered beyond tidiness. A reader deciding whether to put this binary on
// their wire reads this paragraph first, and it told them the proxy is a
// pass-through. It is not: it is a gateway that mutates on purpose, in named
// ways, for stated reasons. Every one of those reasons is defensible and none
// of them survives being discovered by a reader who was told they did not
// happen.
//
// WHAT IT ACTUALLY DOES TO A REQUEST:
//
//	masking        replaces secret byte ranges in place, fail-closed
//	context-edit   inserts one member when a policy is pinned to the session
//	include-usage  adds stream_options so usage is reported at all
//	freeze-prefix  pins a cc_version hash to a same-length constant, off by
//	               default, so the prefix key does not fork on a billing header
//	headers        strips x-replay-token, and Accept-Encoding when it taps
//
// Everything else is forwarded unchanged, and the tap never rewrites a
// response — the one outbound rewrite, rehydration, lives in masking.go so
// that tap.go has no exceptions.
//
// The invariants are in the repository CLAUDE.md; the client-side facts the
// proxy honors are in docs/architecture/proxy-protocol.md.
package proxy

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/masking"
	"github.com/RedRobotKK/Replay/internal/policy"
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
	HeaderToken     = "x-replay-token"
)

// Config is everything serve needs.
type Config struct {
	// Listen is the loopback address to bind.
	Listen string
	// MetricsListen, when set, binds a SECOND listener serving only the read
	// endpoints. It exists because Prometheus cannot scrape a Unix socket, so
	// without it the socket transport and being scraped are mutually
	// exclusive. It never proxies; see metrics_listener.go.
	MetricsListen string
	// Upstream is the provider base URL.
	Upstream *url.URL
	// Token, when set, must match HeaderToken on every request.
	Token string
	// Store receives one record per proxied request.
	Store *ledger.Store
	// Logger receives one line per request. Never headers, never bodies.
	Logger *log.Logger
	// Guards are optional; a nil guard is off.
	Spend       *SpendGuard
	Loops       LoopLimits
	Breaker     *Breaker
	ErrorBudget ErrorBudget
	// ContextEdit, when set, is applied to sessions whose first request
	// admits it and pinned for their life (ADR-0003). Nil is off.
	ContextEdit *policy.ContextEdit
	// PolicyFile, when set, is the replay learn result to read at each
	// session's first request when ContextEdit is not set. A pinned
	// session never changes when the file does (PX-8).
	PolicyFile string
	// Trial bounds how a learned policy is tried live (LN-5).
	Trial TrialSettings
	// PreFlight is the operator's ceiling on the tokens a changed prefix may
	// re-lay. The zero value warns and never refuses, which is the default:
	// a ceiling nobody set must not refuse anybody's request.
	PreFlight analysis.PolicyState
	// NoPolicy turns every live policy off, including one a persisted pin
	// would otherwise restore (PX-6). It also turns masking off.
	NoPolicy bool
	// Masker, when set, replaces secrets in request bodies with vault
	// placeholders before anything else reads the body (ADR-0004). Nil is
	// off.
	Masker *masking.Masker
	// Rehydrator, when set, restores placeholders in response bodies
	// within its scope (ADR-0004). Responses are then requested
	// uncompressed, because a compressed body cannot be rewritten as it
	// passes. Nil is off.
	Rehydrator *masking.Rehydrator
	// FreezePrefix, when set, pins same-length volatile system bytes
	// (cc_version hashes) and labels a tool-set epoch from the exact tools
	// JSON on the wire. Off by default. PX8: a new epoch is a set change,
	// not a claim the next request will miss.
	FreezePrefix bool
	// Siblings, when MaxWait is set, holds a request whose prefix is in
	// flight and not yet cached until the first response begins (the
	// hold-parallel-siblings policy).
	Siblings SiblingSettings
	// Retries, when Attempts is set, resend a request the provider refused
	// with a retryable status or that never connected, before any byte of
	// a response has reached the client.
	Retries RetrySettings
}

// HealthPath answers "ok" for anything that wants to know the proxy is up.
// StatusPath and MetricsPath are the read endpoints.
const (
	HealthPath  = "/replay/healthz"
	StatusPath  = "/replay/status"
	MetricsPath = "/replay/metrics"
)

// forwardingHeaders are the ones httputil.ReverseProxy removes from a
// rewritten request; Replay puts the client's back.
var forwardingHeaders = []string{"Forwarded", "X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto"}

// HeaderOverride is the header a client sets to acknowledge a spend cap or
// a loop block and proceed once. Its value is logged as the reason.
const HeaderOverride = "x-replay-override"

// HeaderWarning is added to a forwarded response when a guard has
// something to say but did not block. It is the only header Replay adds to
// a response.
const HeaderWarning = "x-replay-warning"

// Server is the running proxy.
type Server struct {
	cfg   Config
	http  *http.Server
	rp    *httputil.ReverseProxy
	ready chan struct{}
	addr  string
	stats *stats
	// siblings holds parallel requests with the same prefix (Config.Siblings).
	siblings *siblingGate
	// shutdownGrace overrides ShutdownTimeout in tests. Zero is the constant.
	shutdownGrace time.Duration
	// idleConns holds accepted connections that have not sent a request.
	// A shutdown closes them rather than waiting: they carry no turn.
	idleMu    sync.Mutex
	idleConns map[net.Conn]struct{}
	// readyOnce guards ready so both the success path and the failure defer
	// can signal it.
	readyOnce   sync.Once
	metricsAddr string
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
	// A socket path is local by construction; the loopback question only
	// applies to an address that names a host.
	if !isUnixAddr(cfg.Listen) && !isLoopback(cfg.Listen) {
		return nil, fmt.Errorf("listen address %q is not loopback; Replay only binds locally", cfg.Listen)
	}
	var transport http.RoundTripper = &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialContext(&net.Dialer{Timeout: DialTimeout}),
		TLSHandshakeTimeout:   TLSHandshakeTimeout,
		ResponseHeaderTimeout: ResponseHeaderTimeout,
		IdleConnTimeout:       IdleConnTimeout,
		// The client decides whether it accepts compressed responses;
		// the transport must not add its own header and decompress
		// behind the client's back.
		DisableCompression: true,
	}
	switch err := cfg.Retries.validate(); {
	case err == nil:
		transport = newRetryTransport(transport, cfg.Retries, cfg.Logger)
	case !errors.Is(err, errRetriesOff):
		return nil, err
	}
	s := &Server{cfg: cfg, ready: make(chan struct{}), stats: newStats(), siblings: newSiblingGate(cfg.Siblings)}
	s.rp = &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(cfg.Upstream)
			r.Out.Host = cfg.Upstream.Host
			// A Rewrite adds no forwarding headers of its own, so the
			// request carries only what the client sent, minus Replay's own
			// listener token, which is not the client's header to the
			// provider.
			r.Out.Header.Del(HeaderToken)
			// Rewrite drops the client's own forwarding headers; they are
			// the client's bytes and go through like any other header.
			for _, h := range forwardingHeaders {
				if v, ok := r.In.Header[h]; ok {
					r.Out.Header[h] = v
				}
			}
		},
		Transport: transport,
		// Flush every write so streamed events reach the client as they
		// arrive.
		FlushInterval: -1,
		ModifyResponse: func(resp *http.Response) error {
			if tap, ok := resp.Request.Context().Value(tapKey{}).(*responseTap); ok && tap.rehydrate != nil {
				tap.rehydrate.modify(resp)
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			cfg.Logger.Printf("upstream error: %v", err)
			if tap, ok := w.(*responseTap); ok {
				tap.upstreamFailed = true
			}
			http.Error(w, "replay: upstream request failed: "+err.Error()+"\nTo bypass Replay, unset ANTHROPIC_BASE_URL.", http.StatusBadGateway)
		},
	}
	mux := http.NewServeMux()
	mux.HandleFunc(HealthPath, s.health)
	mux.HandleFunc(StatusPath, s.status)
	mux.HandleFunc(MetricsPath, s.metrics)
	mux.HandleFunc("/", s.handle)
	s.idleConns = map[net.Conn]struct{}{}
	s.http = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: ReadHeaderTimeout,
		// A connection is tracked from the moment it is accepted until it
		// carries a request, so a shutdown can tell "pooled but unused"
		// from "serving a turn".
		ConnState: s.noteConnState,
	}
	return s, nil
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
