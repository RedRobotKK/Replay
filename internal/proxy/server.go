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
	"strconv"
	"strings"
	"time"

	"github.com/RedRobotKK/Buffy/internal/analysis"
	"github.com/RedRobotKK/Buffy/internal/cachemodel"
	"github.com/RedRobotKK/Buffy/internal/learn"
	"github.com/RedRobotKK/Buffy/internal/ledger"
	"github.com/RedRobotKK/Buffy/internal/masking"
	"github.com/RedRobotKK/Buffy/internal/policy"
	"github.com/RedRobotKK/Buffy/internal/transcript"
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
	Spend       *SpendGuard
	Loops       LoopLimits
	Breaker     *Breaker
	ErrorBudget ErrorBudget
	// ContextEdit, when set, is applied to sessions whose first request
	// admits it and pinned for their life (ADR-0003). Nil is off.
	ContextEdit *policy.ContextEdit
	// PolicyFile, when set, is the buffy learn result to read at each
	// session's first request when ContextEdit is not set. A pinned
	// session never changes when the file does (PX-8).
	PolicyFile string
	// Trial bounds how a learned policy is tried live (LN-5).
	Trial TrialSettings
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
	// Retries, when Attempts is set, resend a request the provider refused
	// with a retryable status or that never connected, before any byte of
	// a response has reached the client.
	Retries RetrySettings
}

// HealthPath answers "ok" for anything that wants to know the proxy is up.
// StatusPath and MetricsPath are the read endpoints.
const (
	HealthPath  = "/buffy/healthz"
	StatusPath  = "/buffy/status"
	MetricsPath = "/buffy/metrics"
)

// forwardingHeaders are the ones httputil.ReverseProxy removes from a
// rewritten request; Buffy puts the client's back.
var forwardingHeaders = []string{"Forwarded", "X-Forwarded-For", "X-Forwarded-Host", "X-Forwarded-Proto"}

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

	ok, probe, wait := s.cfg.Breaker.Allow()
	if !ok {
		s.refuse(w, refusalCircuitOpen, fmt.Sprintf("the provider has been failing; Buffy is holding requests for %s so the agent stops burning retries", wait.Round(time.Second)), wait)
		return
	}
	// A half-open probe that never reaches an outcome (refused below, or
	// aborted) is given back so the next request can probe instead. Only
	// the probe itself may give the slot back.
	observed := false
	defer func() {
		if probe && !observed {
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
	setBody(r, body)

	if messages && len(body) > 0 && s.cfg.Masker != nil && !s.cfg.NoPolicy {
		body = s.mask(&rec, body)
		setBody(r, body)
	}

	summarized := false
	if messages && len(body) > 0 {
		if sum, err := ledger.SummarizeRequest(body, s.cfg.Store.Labeler()); err == nil {
			rec.RequestSummary = sum
			summarized = true
		}
		if rec.SessionID == "" {
			rec.SessionID = rec.SessionHash
		}
		if !s.guard(w, r, &rec) {
			return
		}
		body = s.applyPolicy(r, &rec, body, summarized)
		setBody(r, body)
	}
	r, retries := withRetryCounter(r)

	tap := &responseTap{ResponseWriter: w}
	if messages && s.cfg.Rehydrator != nil && !s.cfg.NoPolicy {
		tap.rehydrate = &rehydration{rh: s.cfg.Rehydrator}
		r = r.WithContext(context.WithValue(r.Context(), tapKey{}, tap))
		r.Header.Del(headerAcceptEncoding)
	}
	// Bookkeeping runs in a deferred function because the reverse proxy
	// aborts the handler with a panic when the client goes away mid-stream
	// (the user interrupting a turn). The provider still billed that turn,
	// so it must still be observed, counted, and recorded; the panic is
	// then re-raised for the server to handle as it normally does.
	defer func() {
		aborted := recover()
		if tap.status != 0 || tap.upstreamFailed {
			// A client that left before any response is no observation of
			// the provider.
			s.cfg.Breaker.Observe(tap.upstreamFailed || IsRetryableStatus(tap.status))
			observed = true
		}
		rec.Status = tap.status
		rec.Retries = retries.n
		rec.LatencyMS = time.Since(start).Milliseconds()
		rec.RequestID = tap.Header().Get("request-id")
		if messages {
			rec.Response = tap.result()
			if u := rec.Response.Usage; u != nil {
				s.cfg.Spend.Record(rec.SessionID, u.Input+u.CacheCreation+u.CacheRead+u.Output, listCost(*u, rec.Model))
			}
		}
		if tap.rehydrate != nil {
			s.noteRehydration(&rec, tap.rehydrate)
		}
		rec.Cache = s.stats.observe(&rec)
		if messages && rec.SessionID != "" {
			if err := s.cfg.Store.Append(rec); err != nil {
				s.cfg.Logger.Printf("ledger write failed: %v", err)
			}
		}
		whatIf, guardrail := "", ""
		if messages && rec.SessionID != "" && rec.Response.Usage != nil {
			var rr analysis.ReReads
			whatIf, rr = s.stats.rescore(&rec)
			if edit, generated, ok := s.stats.trialSession(rec.SessionID); ok && s.cfg.Trial.breached(rr) {
				guardrail = s.stats.noteBreach(s.cfg.Store, s.cfg.Trial, rec.SessionID, edit, rr, generated)
			}
		}
		note := ""
		if aborted != nil {
			note = " aborted=client-disconnected"
		}
		if rec.Retries > 0 {
			note += fmt.Sprintf(" retries=%d", rec.Retries)
		}
		s.cfg.Logger.Printf("%s %s status=%d ms=%d session=%s model=%s %s%s", r.Method, r.URL.Path, rec.Status, rec.LatencyMS, short(rec.SessionID), rec.Model, usageSummary(rec.Response.Usage), note)
		if rec.Cache != nil && rec.Cache.Deficit > 0 {
			s.cfg.Logger.Printf("cache break session=%s: read %d of %d expected, %d tokens re-billed; likely cause: %s", short(rec.SessionID), rec.Response.Usage.CacheRead, rec.Cache.Expected, rec.Cache.Deficit, rec.Cache.Cause)
		}
		if whatIf != "" {
			s.cfg.Logger.Print(whatIf)
		}
		if guardrail != "" {
			s.cfg.Logger.Print(guardrail)
		}
		if aborted != nil {
			panic(aborted)
		}
	}()
	s.rp.ServeHTTP(tap, r)
}

// mask replaces secrets in the body with placeholders and records what it
// did. A body the masker cannot read goes through unchanged: masking
// fails open like every other feature, and the log says so without the
// content.
func (s *Server) mask(rec *ledger.Record, body []byte) []byte {
	out, report, err := s.cfg.Masker.Mask(body)
	if err != nil {
		s.cfg.Logger.Printf("mask session=%s: body not masked: %v", short(rec.SessionID), err)
		return body
	}
	if report.Total() > 0 {
		rec.Masked = report
		s.cfg.Logger.Printf("masked %d secret(s) session=%s: %s", report.Total(), short(rec.SessionID), report)
	}
	return out
}

// headerAcceptEncoding is dropped from requests whose response will be
// rehydrated.
const headerAcceptEncoding = "Accept-Encoding"

// tapKey carries the response tap to the reverse proxy's response hook.
type tapKey struct{}

// rehydration is one response's rehydration: set up on the response
// hook, read by the handler's bookkeeping once the body has passed.
type rehydration struct {
	rh     *masking.Rehydrator
	stream *masking.StreamRehydrator
	report masking.RehydrationReport
	// skipped says why the body was not inspected, when it was not.
	skipped string
	err     error
}

// modify installs the rehydrating body. A compressed response, or one
// past the size limit, is forwarded as it is and the skip is reported.
func (h *rehydration) modify(resp *http.Response) {
	if resp.Header.Get("Content-Encoding") != "" {
		h.skipped = "compressed response"
		return
	}
	ct := resp.Header.Get("Content-Type")
	switch {
	case ledger.IsEventStream(ct):
		h.stream = h.rh.NewStream()
		resp.Body = masking.NewTransformReader(resp.Body, h.stream)
	case strings.HasPrefix(strings.ToLower(ct), "application/json"):
		body, err := io.ReadAll(io.LimitReader(resp.Body, MaxResponseBytes+1))
		if err != nil {
			h.skipped = "body not read: " + err.Error()
			resp.Body = readCloser{Reader: io.MultiReader(bytes.NewReader(body), resp.Body), Closer: resp.Body}
			return
		}
		if len(body) > MaxResponseBytes {
			h.skipped = "response over the size limit"
			resp.Body = readCloser{Reader: io.MultiReader(bytes.NewReader(body), resp.Body), Closer: resp.Body}
			return
		}
		out, report, err := h.rh.Body(body)
		if err != nil {
			h.err = err
			out = body
		}
		h.report = report
		// The original body is fully read; its close only releases the
		// connection, which the replacement's close does in turn.
		resp.Body = readCloser{Reader: bytes.NewReader(out), Closer: resp.Body}
		resp.ContentLength = int64(len(out))
		resp.Header.Set("Content-Length", strconv.Itoa(len(out)))
	}
}

// result is the report once the body has passed.
func (h *rehydration) result() masking.RehydrationReport {
	if h.stream != nil {
		return h.stream.Report()
	}
	return h.report
}

type readCloser struct {
	io.Reader
	io.Closer
}

// noteRehydration records and logs what a response's rehydration did
// (MK-6): counts by destination, never a value or a path.
func (s *Server) noteRehydration(rec *ledger.Record, h *rehydration) {
	rep := h.result()
	rec.Rehydrated, rec.RehydrationDenied = rep.Restored, rep.Denied
	switch {
	case h.err != nil:
		s.cfg.Logger.Printf("rehydration session=%s: response forwarded with placeholders: %v", short(rec.SessionID), h.err)
	case h.skipped != "":
		s.cfg.Logger.Printf("rehydration skipped session=%s: %s", short(rec.SessionID), h.skipped)
	}
	if len(rep.Restored) > 0 {
		s.cfg.Logger.Printf("rehydrated %d placeholder(s) session=%s: %s", rep.Total(), short(rec.SessionID), rep.RestoredSummary())
	}
	if len(rep.Denied) > 0 {
		s.cfg.Logger.Printf("rehydration denied session=%s: %s", short(rec.SessionID), rep.DeniedSummary())
	}
}

// setBody installs an in-memory body that the retry transport can reopen.
func setBody(r *http.Request, body []byte) {
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	r.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
}

// applyPolicy adds the session's pinned request parameter when its
// decision allows it. The decision and the parameters are made at the
// session's first request, from the flag, else the policy file, else the
// persisted pin of an earlier process, and kept for the session's life:
// a session either always carries the parameter or never does. Every
// transformation is logged with the body hashes before and after
// (PX-10), never the bodies.
func (s *Server) applyPolicy(r *http.Request, rec *ledger.Record, body []byte, summarized bool) []byte {
	if rec.SessionID == "" || !s.policyConfigured() {
		return body
	}
	beta, clientSet := r.Header.Get("anthropic-beta"), rec.Prompt.ContextEdits
	edit, decision, ok := s.stats.pinned(rec.SessionID)
	if !ok {
		var generated time.Time
		edit, decision, generated = s.decidePolicy(rec.SessionID, beta, clientSet, rec.Model, promptSize(rec.Prompt))
		s.stats.pin(rec.SessionID, edit, decision, generated)
	}
	if edit == nil || decision != policy.Applied {
		return body
	}
	if !summarized {
		// A body the summarizer could not read may already carry the
		// parameter; a second copy is worse than none.
		s.cfg.Logger.Printf("policy %s session=%s %s", policy.Name, short(rec.SessionID), policy.SkipUnparsed)
		return body
	}
	if admissible := edit.Admissible(beta, clientSet); admissible != policy.Applied {
		// Pinned on, but this request cannot carry the parameter.
		s.cfg.Logger.Printf("policy %s session=%s %s", policy.Name, short(rec.SessionID), admissible)
		return body
	}
	out, applied := edit.Apply(body)
	if applied != policy.Applied {
		s.cfg.Logger.Printf("policy %s session=%s %s", policy.Name, short(rec.SessionID), applied)
		return body
	}
	rec.Policy = policy.Name
	s.cfg.Logger.Printf("policy %s session=%s applied body sha256 before=%s after=%s", edit, short(rec.SessionID), bodyHash(body), bodyHash(out))
	return out
}

// policyConfigured reports whether any live policy source is on. With
// none, a persisted pin is left alone: turning policies off must stop
// every session, including one pinned on by an earlier process.
func (s *Server) policyConfigured() bool {
	return !s.cfg.NoPolicy && (s.cfg.ContextEdit != nil || s.cfg.PolicyFile != "")
}

// decidePolicy makes a session's decision at its first request in this
// process. A pin persisted by an earlier process wins over everything,
// then the flag, then the policy file. The decision is persisted so a
// restart or a rewritten file cannot change a running session.
func (s *Server) decidePolicy(sessionID, beta string, clientSet bool, model string, promptBytes int) (*policy.ContextEdit, policy.Decision, time.Time) {
	if pin, ok := s.cfg.Store.Pin(sessionID); ok {
		var edit *policy.ContextEdit
		if pin.Policy == policy.Name {
			edit = &policy.ContextEdit{TriggerTokens: pin.Trigger, KeepLast: pin.Keep}
			if err := edit.Validate(); err != nil {
				// A pin another process wrote is data, not an order.
				s.cfg.Logger.Printf("policy session=%s pinned earlier with invalid parameters (%v); running without it", short(sessionID), err)
				return nil, policy.NotConfigured, time.Time{}
			}
		}
		s.cfg.Logger.Printf("policy session=%s pinned earlier: %s", short(sessionID), transcript.SanitizeLabel(pin.Decision))
		// A restored pin carries no file generation; the guardrail judges
		// sessions this process started.
		return edit, policy.Decision(pin.Decision), time.Time{}
	}
	edit := s.cfg.ContextEdit
	decision := policy.NotConfigured
	var generated time.Time
	pin := ledger.Pin{SessionID: sessionID, At: time.Now(), Decision: string(decision)}
	if edit == nil && s.cfg.PolicyFile != "" {
		var arm string
		sessionType := learn.TypeFromBytes(model, promptBytes)
		edit, decision, arm, generated = s.trialPolicy(sessionID, sessionType)
		pin.Trial, pin.Type, pin.Decision = arm, sessionType, string(decision)
	}
	if edit != nil {
		decision = edit.Admissible(beta, clientSet)
		pin.Policy, pin.Trigger, pin.Keep, pin.Decision = policy.Name, edit.TriggerTokens, edit.KeepLast, string(decision)
	}
	if err := s.cfg.Store.SetPin(pin); err != nil {
		// Fail open: the session runs under the in-memory pin.
		s.cfg.Logger.Printf("policy pin not persisted for session=%s: %v", short(sessionID), err)
	}
	return edit, decision, generated
}

// trialPolicy reads the learned selection for a session that is starting
// and assigns the session to an arm of the trial: treated sessions get
// the policy, control sessions are held out so the two can be compared,
// and once the guardrail has reverted the policy nobody gets it until a
// newer learning result replaces it.
func (s *Server) trialPolicy(sessionID, sessionType string) (*policy.ContextEdit, policy.Decision, string, time.Time) {
	edit, generated := s.policyFromFile(sessionID, sessionType)
	if edit == nil {
		return nil, policy.NotConfigured, "", time.Time{}
	}
	if r, ok := s.cfg.Store.Revert(); ok && !generated.After(r.At) {
		s.cfg.Logger.Printf("policy %s reverted at %s (%s); session=%s runs without it until buffy learn writes a newer file", edit, r.At.Format(time.RFC3339), r.Reason, short(sessionID))
		return nil, policy.Reverted, "", time.Time{}
	}
	if !s.cfg.Trial.treated(sessionID) {
		s.cfg.Logger.Printf("policy %s session=%s is a control: held out of the trial", edit, short(sessionID))
		return nil, policy.Control, trialControl, time.Time{}
	}
	return edit, policy.Applied, trialTreated, generated
}

// promptSize is the size of a summarized request as the client sent it:
// the prefix and every message block, which is what the session type is
// estimated from at a first request.
func promptSize(p ledger.Prompt) int {
	n := p.SystemBytes + p.ToolBytes
	for _, m := range p.Messages {
		for _, b := range m.Blocks {
			n += b.Bytes
		}
	}
	return n
}

// policyFromFile reads the learned selection for the session's type,
// falling back to the overall one. Only the context-edit family is
// something the proxy can apply; a TTL selection is advice for a client
// setting and is logged. The file's generation time comes back so a
// revert can be tied to the file it happened under.
func (s *Server) policyFromFile(sessionID, sessionType string) (*policy.ContextEdit, time.Time) {
	res, err := learn.LoadFile(s.cfg.PolicyFile)
	if err != nil {
		s.cfg.Logger.Printf("policy file %s not read for session=%s: %v", s.cfg.PolicyFile, short(sessionID), err)
		return nil, time.Time{}
	}
	c, note := res.SelectionFor(sessionType)
	switch {
	case note != "":
		s.cfg.Logger.Printf("policy file: %s (session=%s type=%s runs without a policy)", transcript.SanitizeLabel(note), short(sessionID), sessionType)
		return nil, time.Time{}
	case c.ContextEdit == nil:
		s.cfg.Logger.Printf("policy file selects %s, which is a client setting (%s); session=%s runs without a proxy policy", transcript.SanitizeLabel(c.Name), transcript.SanitizeLabel(c.Live), short(sessionID))
		return nil, time.Time{}
	}
	edit := &policy.ContextEdit{TriggerTokens: c.ContextEdit.TriggerTokens, KeepLast: c.ContextEdit.KeepLast}
	if err := edit.Validate(); err != nil {
		s.cfg.Logger.Printf("policy file selection rejected: %v", err)
		return nil, time.Time{}
	}
	return edit, res.Generated
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
	if reason := s.cfg.ErrorBudget.Check(s.stats.errorTokens(rec.SessionID)); reason != "" {
		if override == "" {
			s.refuse(w, refusalErrorBudget, reason+". Look at what is failing (buffy replay on the ledger names it), start a new session, or send "+HeaderOverride+" with a reason to proceed once.", 0)
			return false
		}
		s.cfg.Logger.Printf("error budget overridden for session=%s: %s", short(rec.SessionID), override)
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
	refusalErrorBudget = refusal{http.StatusBadRequest, "buffy_error_budget", "error_budget"}
)

// listCost prices one request's usage at list price, zero for a model
// the price table does not know.
func listCost(u ledger.Usage, model string) float64 {
	price, ok := cachemodel.PriceFor(model)
	if !ok {
		return 0
	}
	return cachemodel.CostUSD(u, price)
}

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
	// rehydrate, when set, rewrites the response body on its way through.
	rehydrate *rehydration
	status    int
	stream    *ledger.StreamParser
	buffer    bytes.Buffer
	gz        bool
	dropped   bool
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
