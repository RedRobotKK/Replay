package proxy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/ledger"
)

// The request path: one request in, one response out, one ledger record.
//
// handle is the whole order of operations, and the order is the invariant.
// Mask before anything else reads the body, summarize, guard, apply policy,
// enter the lane, tap, forward, and do the bookkeeping in a deferred
// function so that a client who interrupts a turn — which the reverse proxy
// delivers as a panic — is still counted and recorded, because the provider
// still billed it. Reordering any two of those changes what the operator is
// charged for or what the ledger says happened.
//
// The transformations themselves are NOT here. Masking is in masking.go,
// policy injection in policyinject.go, the response tap in tap.go, the local
// answers in refusal.go. This file calls them in sequence and nothing more,
// so that a patch which widens one of them is a diff against that file and
// has to say so, instead of arriving as one more `if` in a thousand-line
// handler.
//
// It also holds what decides whether a request is readable at all —
// isMessages and isChatCompletions — and noteUnparsed, which says out loud
// that a path Replay cannot read has no cap, no budget, no loop detection
// and no masking on it. Protection that quietly is not there is worse than
// protection nobody claimed.
//
// Splitting the file does not narrow the handler. It is still long, and the
// guards are still gated on a readable, non-empty body.

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
	openai := isChatCompletions(r.URL.Path)
	// readable is a body this build can summarise, guard and ledger.
	readable := messages || openai
	if !readable && r.Method == http.MethodPost {
		// A POST somewhere else is a client sending real work down a path
		// this build cannot read. It is forwarded unchanged, and everything
		// Replay offers is inert for it, so say so rather than let the
		// operator infer protection from a running proxy.
		s.noteUnparsed(r.URL.Path)
	}
	rec := ledger.Record{Timestamp: start, Path: r.URL.Path, SessionID: r.Header.Get(HeaderSessionID), AgentID: r.Header.Get(HeaderAgentID)}

	ok, probe, wait := s.cfg.Breaker.Allow()
	if !ok {
		s.refuseSession(w, r.Header.Get(HeaderSessionID), "", refusalCircuitOpen, fmt.Sprintf("the provider has been failing; Replay is holding requests for %s so the agent stops burning retries", wait.Round(time.Second)), wait)
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
		http.Error(w, "replay: could not read request body", http.StatusBadRequest)
		return
	}
	if len(body) > MaxRequestBytes {
		http.Error(w, "replay: request body exceeds the proxy limit", http.StatusRequestEntityTooLarge)
		return
	}
	setBody(r, body)

	if messages && len(body) > 0 && s.cfg.Masker != nil && !s.cfg.NoPolicy {
		body = s.mask(&rec, body)
		setBody(r, body)
	}
	if openai && s.cfg.Masker != nil && !s.cfg.NoPolicy {
		// The masker walks the Messages body shape. This family's body is
		// different and it is not masked. Saying so matters more here than
		// anywhere: the path is now read, guarded and ledgered, so the
		// NOT PARSED warning no longer fires and nothing else would tell the
		// operator that --mask is not running on this traffic.
		s.noteUnmasked(r.URL.Path)
	}

	summarized := false
	if readable && len(body) > 0 {
		summarize := ledger.SummarizeRequest
		if openai {
			summarize = ledger.SummarizeOpenAIRequest
		}
		if sum, err := summarize(body, s.cfg.Store.Labeler()); err == nil {
			rec.RequestSummary = sum
			summarized = true
		}
		if rec.SessionID == "" {
			rec.SessionID = rec.SessionHash
		}
		if !s.guard(w, r, &rec) {
			return
		}
		if messages {
			body = s.applyPolicy(r, &rec, body, summarized)
			setBody(r, body)
		}
		if openai && !s.cfg.NoPolicy {
			// Ask for usage on a stream when the client did not. Without it
			// this family reports none, and every guard below sees a free
			// request. ADR-0003 kind one: a parameter the client left unset.
			if out, changed := withUsageReporting(body); changed {
				body = out
				setBody(r, body)
				rec.Policy = "openai-include-usage"
			}
		}
	}
	// This request is in flight in its lane from here until its response is
	// finished. Everything downstream that names a cause for it is comparing
	// it against the request before it, and only this counter can say whether
	// there WAS one request before it or two racing it. Registered after the
	// guards, so a refusal that never reaches the provider is not counted as
	// traffic, and released after the bookkeeping below has read it: deferred
	// functions run last-registered-first, and the bookkeeping defer comes
	// after this one.
	overlapped, leaveLane := s.stats.enterLane(rec.SessionID, rec.AgentID)
	defer leaveLane()

	r, retries := withRetryCounter(r)

	tap := &responseTap{ResponseWriter: w, openai: openai}
	if messages && s.cfg.Rehydrator != nil && !s.cfg.NoPolicy {
		tap.rehydrate = &rehydration{rh: s.cfg.Rehydrator}
		r = r.WithContext(context.WithValue(r.Context(), tapKey{}, tap))
		r.Header.Del(headerAcceptEncoding)
	}
	if readable && summarized {
		// Wait behind a sibling with the same prefix, then lead for the
		// ones behind this request until its response begins.
		release, waited := s.siblings.enter(r.Context(), rec.PrefixHash)
		defer release()
		rec.HeldMS = waited.Milliseconds()
		prefix := rec.PrefixHash
		tap.onHeaders = func(status int) {
			if status < http.StatusMultipleChoices {
				s.siblings.began(prefix)
			} else {
				release()
			}
		}
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
		rec.RequestID = providerRequestID(tap.Header())
		rec.Correlation = s.stats.correlation(overlapped)
		rec.Quota = quotaFrom(tap.Header())
		if readable {
			rec.Response = tap.result()
			if u := rec.Response.Usage; u != nil {
				s.cfg.Spend.Record(rec.SessionID, u.Input+u.CacheCreation+u.CacheRead+u.Output, listCost(*u, rec.Model))
			}
		}
		if tap.rehydrate != nil {
			s.noteRehydration(&rec, tap.rehydrate)
		}
		rec.Cache = s.stats.observe(&rec)
		// Lane overlap means overlap at the provider, not during local
		// bookkeeping. correlation() has already read this request's flag.
		// Release before Append: the ledger write is local, and a client
		// that posts the next turn when the body closes (waitLedger in
		// tests) unblocks on that write. Holding the lane open through it
		// marked sequential turns as overlapping and named the cause
		// NOT MEASURED — two turns that never raced at the provider.
		leaveLane()
		if readable && rec.SessionID != "" {
			if err := s.cfg.Store.Append(rec); err != nil {
				s.cfg.Logger.Printf("ledger write failed: %v", err)
			}
		}
		whatIf, guardrail := "", ""
		if readable && rec.SessionID != "" && rec.Response.Usage != nil {
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
		if rec.HeldMS > 0 {
			note += fmt.Sprintf(" held_ms=%d", rec.HeldMS)
		}
		s.cfg.Logger.Printf("%s %s status=%d ms=%d session=%s model=%s %s%s", r.Method, r.URL.Path, rec.Status, rec.LatencyMS, short(rec.SessionID), rec.Model, usageSummary(rec.Response.Usage), note)
		if rec.Cache != nil && rec.Cache.Deficit > 0 {
			// The detail names what actually changed. It is omitted rather
			// than printed empty: a bare "()" reads as a measurement that
			// came back nothing, which is a different claim from one that was
			// not taken.
			detail := ""
			if rec.Cache.CauseDetail != "" {
				detail = " (" + rec.Cache.CauseDetail + ")"
			}
			// The correlation is on the line, not in the documentation.
			// Naming a cause is naming a PREDECESSOR, and a reader who never
			// opens the ledger has only this line to tell them whether that
			// predecessor was the one request that could have been or the one
			// that happened to finish first.
			s.cfg.Logger.Printf("cache break session=%s lane=%s: read %d of %d expected, %d tokens re-billed; correlated by %s; likely cause: %s%s", short(rec.SessionID), laneName(rec.AgentID), rec.Response.Usage.CacheRead, rec.Cache.Expected, rec.Cache.Deficit, rec.Correlation, rec.Cache.Cause, detail)
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

// setBody installs an in-memory body that the retry transport can reopen.
func setBody(r *http.Request, body []byte) {
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	r.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
}

// guard applies the spend cap and loop detector to a summarized request.
// It reports false when the request was answered locally.
func (s *Server) guard(w http.ResponseWriter, r *http.Request, rec *ledger.Record) bool {
	override := r.Header.Get(HeaderOverride)
	if reason := s.cfg.Spend.Check(rec.SessionID); reason != "" {
		if override == "" {
			s.refuseSession(w, rec.SessionID, rec.Model, refusalSpendCap, reason+". Raise the cap, start a new session, or send "+HeaderOverride+" with a reason to proceed once.", 0)
			return false
		}
		s.cfg.Logger.Printf("spend cap overridden for session=%s: %s", short(rec.SessionID), override)
	}
	if reason := s.cfg.ErrorBudget.Check(s.stats.errorTokens(rec.SessionID)); reason != "" {
		if override == "" {
			s.refuseSession(w, rec.SessionID, rec.Model, refusalErrorBudget, reason+". Look at what is failing (replay replay on the ledger names it), start a new session, or send "+HeaderOverride+" with a reason to proceed once.", 0)
			return false
		}
		s.cfg.Logger.Printf("error budget overridden for session=%s: %s", short(rec.SessionID), override)
	}
	v := DetectLoop(rec.Prompt, s.cfg.Loops)
	switch {
	case v.Block && override == "":
		s.refuseSession(w, rec.SessionID, rec.Model, refusalLoop, fmt.Sprintf("the same %s call was just made %d times in a row; Replay stopped the loop. Send %s with a reason to proceed once.", v.Label, v.Repeats, HeaderOverride), 0)
		return false
	case v.Block:
		s.cfg.Logger.Printf("loop block overridden for session=%s: %s", short(rec.SessionID), override)
	case v.Warn:
		w.Header().Set(HeaderWarning, fmt.Sprintf("loop: the same %s call was just made %d times in a row", v.Label, v.Repeats))
	}
	return s.preFlight(w, rec, override)
}

// isMessages reports whether a path is the Messages endpoint proper (not
// count_tokens), which is the only one whose responses carry usage.
const (
	messagesPath        = "/v1/messages"
	chatCompletionsPath = "/v1/chat/completions"
)

func isMessages(path string) bool {
	return strings.HasSuffix(path, messagesPath)
}

// isChatCompletions reports the OpenAI-compatible endpoint, which Cursor,
// DeepSeek and OpenAI itself all speak.
//
// Grok was named on this line until 2026-09-06 and it does not belong here. It
// was grouped by an assumption about what an OpenAI-compatible CLI must send;
// captured off a live authenticated session it posts to /responses at
// cli-chat-proxy.grok.com, which nothing in this build parses. It is forwarded
// and warned about like any other unknown path, and saying otherwise would
// promise a user a report that comes back empty.
//
// It is read but not rewritten. Policy application stays off for this family:
// ADR-0003 admits a parameter the client left unset, and this provider caches
// automatically with no breakpoint to place and no TTL to choose, so there is
// no admissible policy to apply. Observation and guards are the whole of what
// Replay offers here, and that is stated rather than left to be discovered.
func isChatCompletions(path string) bool {
	return strings.HasSuffix(path, chatCompletionsPath)
}

// noteUnparsed records a request Replay forwarded without understanding, and
// says so out loud the first time it sees each path.
//
// Everything this proxy does hangs off parsing the Messages body: the ledger
// record, the spend cap, the error budget, the loop detector and the secret
// masker all sit behind isMessages. A request on any other path is forwarded
// byte for byte, which is correct, and leaves every one of those inert, which
// the operator has no way to discover. They configured a cap and believe it is
// on.
//
// That is the failure mode the persisted day counter and CapNotEnforced both
// exist to prevent: protection that quietly is not there is worse than
// protection nobody claimed, because the user has stopped watching.
//
// It warns once per path rather than per request, because a line on every
// request is noise an operator learns to scroll past.
func (s *Server) noteUnparsed(path string) {
	if !s.stats.noteUnparsed(path) || s.cfg.Logger == nil {
		return
	}
	s.cfg.Logger.Printf("NOT PARSED %s: Replay forwards this path unchanged and cannot read it. "+
		"No ledger record, no spend cap, no error budget, no loop detection and no secret masking apply to it. "+
		"Only %s is understood by this build.", path, messagesPath)
}

// listCost prices one request's usage at list price, zero for a model
// the price table does not know.
func listCost(u ledger.Usage, model string) float64 {
	price, ok := cachemodel.PriceFor(model)
	if !ok {
		return 0
	}
	return cachemodel.CostUSD(u, price)
}
