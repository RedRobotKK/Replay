package proxy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/RedRobotKK/Replay/internal/learn"
	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/policy"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// The only code in this package that adds anything to a request body.
//
// ADR-0003 admits exactly one kind of change: a request parameter the client
// left unset. Everything here is under that rule and no wider — the
// context-edit parameter for the Messages family, and stream_options.
// include_usage for the OpenAI-compatible one, which real clients do not set
// and without which the spend cap, the error budget and every cost figure
// see nothing on the only shape those clients send.
//
// Two invariants make it safe to do that at all, and they are why this is a
// file and not a branch inside handle:
//
//   - A session decides once, at its first request, and is pinned for its
//     life. A session either always carries the parameter or never does, so
//     an earlier turn is never re-rendered differently than it was sent, and
//     provider history-binding checks have nothing to reject. A rewritten
//     policy file, or a restart, cannot change a session already running.
//   - Nothing here touches a thinking block, a signature, or a cache_control
//     marker, and a body the summarizer could not read is returned exactly
//     as it arrived, because a body that may already carry the parameter
//     getting a second copy is worse than getting none.
//
// Every applied transformation is logged with the body hashes before and
// after (PX-10), never the bodies.
//
// Being its own file does not make the trial arithmetic or the guardrail
// correct; it makes the set of things that may edit a body enumerable, which
// it was not while they were interleaved with forwarding and retries.

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
		s.cfg.Logger.Printf("policy %s reverted at %s (%s); session=%s runs without it until replay learn writes a newer file", edit, r.At.Format(time.RFC3339), r.Reason, short(sessionID))
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

// bodyHash is a content-free fingerprint of a request body.
//
// For the log, and now for the ledger: Record.BodyHashBefore and
// BodyHashAfter bracket every rewrite in handle, and record.go says those two
// fields are "what makes 'the proxy forwards bytes unchanged' checkable rather
// than promised". Equal hashes are the proof. So this function is no longer a
// debugging convenience, and a change to its truncation or its input changes
// what that proof is worth.
//
// Truncated the way every other label here is: long enough that two different
// bodies will not collide by accident, and not a cryptographic commitment —
// the reader and the writer are the same machine, and the question is "did
// these bytes change", not "can you prove they did not".
func bodyHash(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])[:16]
}

// withUsageReporting asks the provider to report usage on a streamed
// OpenAI-compatible response, when the client did not.
//
// This family sends no usage object on a stream unless stream_options.
// include_usage is set, and real clients do not set it: Cursor does not. Without
// it the spend cap, the error budget and every cost figure see nothing on the
// only shape those clients actually send, which is the silent-failure mode this
// proxy exists not to have.
//
// It is admissible under ADR-0003's first kind, a request parameter the client
// left unset. A client that set stream_options keeps its own value, whatever it
// is. Nothing else in the body is touched, and a body this build cannot parse
// is returned exactly as it arrived.
func withUsageReporting(body []byte) ([]byte, bool) {
	var raw map[string]json.RawMessage
	if json.Unmarshal(body, &raw) != nil {
		return body, false
	}
	var stream bool
	if v, ok := raw["stream"]; !ok || json.Unmarshal(v, &stream) != nil || !stream {
		return body, false
	}
	if _, ok := raw["stream_options"]; ok {
		return body, false
	}
	raw["stream_options"] = json.RawMessage(`{"include_usage":true}`)
	out, err := json.Marshal(raw)
	// guard-reachability reports this UNREACHED and no test will fix it.
	// raw is a map[string]json.RawMessage that json.Unmarshal filled on the
	// first line of this function, so every value in it is already valid
	// JSON, and the one key added here is a literal. Marshalling valid
	// RawMessages under string keys has no failure mode.
	//
	// Probed rather than assumed: invalid UTF-8 in a key and in a value,
	// 500-deep nesting, 1e308, a lone surrogate escape, duplicate keys and
	// HTML-significant characters. Every one marshalled with a nil error;
	// invalid UTF-8 is replaced with U+FFFD rather than refused.
	//
	// It stays because dropping it would mean ignoring err and returning a
	// nil out as though it were a rewritten body — the one outcome this
	// function must never produce.
	if err != nil {
		return body, false
	}
	return out, true
}
