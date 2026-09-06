package proxy

import (
	"fmt"
	"net/http"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/ledger"
)

// Pre-flight deficit, checked before the request reaches the provider.
//
// The ledger already names a cache break after the fact: server.go logs "read N
// of M expected, D tokens re-billed; likely cause: ...". That line arrives once
// the money is spent. This is the same fact one request earlier.
//
// It is narrow on purpose. The only break cause knowable before the wire is a
// changed prefix, because the prefix hash is computed from what the client just
// sent and compared with what the session sent last. That is an equality test,
// not a prediction. It is also the cause worth catching: in the 30-lane trial
// on 2026-09-06, "system prompt or tool definitions changed" was 34 of 71
// breaks and 3,734,134 of the 3,778,706 re-billed tokens. The other cause,
// divergence inside the message history, needs the provider's answer and is not
// available here.
//
// The token figure IS a prediction: prefix bytes through a fitted ratio. That
// is why a ceiling landing inside the estimate's error band suppresses the
// refusal rather than deciding it. See analysis.PreFlightDeficit.Straddles.

// tokensPerByte prices prefix bytes for the pre-flight estimate.
//
// System prompts and tool definitions are JSON schemas, which are denser than
// prose, so the prose default understates them. It is deliberately the same
// constant the analysis uses rather than a second number that could drift:
// a pre-flight figure that disagreed with the ledger's own figure for the same
// request would be worse than no figure at all.
const tokensPerByte = 0.25

// preFlight refuses a request whose changed prefix would re-lay more tokens
// than the operator agreed to spend, and warns when it would not refuse.
//
// It returns false only when it has answered the request itself.
//
// Three things it deliberately does not do. It does not read consent from a
// request header, because that would let any client switch the guard on or off;
// consent is s.cfg.PreFlight, set by the operator. It does not re-read the
// body, because the prefix hash and byte counts are already on rec. And it does
// not write a bare status, because every other guard here answers in the
// provider's error shape, which is the only thing an agent renders to a user.
func (s *Server) preFlight(w http.ResponseWriter, rec *ledger.Record, override string) bool {
	policy := s.cfg.PreFlight
	st := s.stats.session(rec.SessionID)
	if st == nil || rec.PrefixHash == "" {
		return true
	}

	// First request of a session establishes the prefix; there is nothing to
	// have diverged from, and treating it as a break would refuse every
	// session's opening request.
	prior := st.prefixHash
	if prior == "" {
		return true
	}
	diverged := prior != rec.PrefixHash
	if !diverged {
		return true
	}

	bytes := rec.Prompt.SystemBytes + rec.Prompt.ToolBytes
	d := analysis.NewPreFlightDeficit(int64(float64(bytes)*tokensPerByte), true, false)

	// Nothing to say when the deficit is inside what the operator accepts,
	// or when they have set no ceiling at all. A warning on every prefix
	// change would fire on the first tool binding of every session.
	if !d.WouldRefuse(policy) {
		return true
	}

	msg := fmt.Sprintf("the system prompt or tool definitions changed, so the cached prefix is void; "+
		"re-laying it costs about %d tokens (%d-%d), over your %d-token pre-flight ceiling",
		d.Tokens, d.Low, d.High, policy.CeilingTokens)

	// The ceiling sits inside the estimate's own error band, so the tool
	// cannot tell which side of it this request falls on. Refusing here
	// would report precision the measurement does not have. Warn and let it
	// through, which is the same shape as the loop guard's warn case.
	if d.Straddles(policy) {
		w.Header().Set(HeaderWarning, "preflight: "+msg+", but the ceiling is inside the estimate's "+
			"error band, so this was not refused")
		if s.cfg.Logger != nil {
			s.cfg.Logger.Printf("preflight straddle session=%s: %s; passed through", short(rec.SessionID), msg)
		}
		return true
	}

	if override != "" {
		s.cfg.Logger.Printf("preflight ceiling overridden for session=%s: %s", short(rec.SessionID), override)
		return true
	}

	s.refuseSession(w, rec.SessionID, rec.Model, refusalPreFlight,
		msg+". Raise the ceiling, or send "+HeaderOverride+" with a reason to proceed once.", 0)
	return false
}
