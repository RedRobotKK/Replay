package proxy

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/tenancy"
)

// What Replay sends when it answers a request itself instead of forwarding
// it: a spend cap, a loop, an error budget, a pre-flight deficit, an open
// circuit.
//
// The shape is the invariant. A refusal is written in the provider's error
// shape so that any client which understands provider errors shows the
// message to the user, rather than a bare status the agent renders as a
// network fault.
//
// The second invariant is what a refusal must NOT do, and it is the reason
// these three functions are together and away from the bookkeeping defer in
// passthrough.go. refusalCircuitOpen is a 503, and IsRetryableStatus counts
// 500-599 as a provider failure, so routing a refusal through Breaker.Observe
// would re-arm the cooldown on every request the open circuit refuses and the
// circuit would never close. Nothing here calls Observe, and it must stay
// that way.
//
// Each refusal also writes its own ledger record, because a counter on
// /replay/metrics cannot be analysed and a guard that saves somebody money
// overnight was otherwise observable only as a request log that stopped.
//
// Collecting them does not change any status, message or counter, and does
// not make the thresholds right; it makes them readable in one place.

// refusal is one way Replay answers a request itself instead of forwarding
// it: the status it sends, the error type a provider-aware client shows,
// and the counter it lands in.
type refusal struct {
	status  int
	errType string
	counter string
}

var (
	refusalCircuitOpen = refusal{http.StatusServiceUnavailable, "replay_circuit_open", "circuit_open"}
	refusalSpendCap    = refusal{http.StatusBadRequest, "replay_spend_cap", "spend_cap"}
	refusalLoop        = refusal{http.StatusBadRequest, "replay_loop", "loop"}
	refusalErrorBudget = refusal{http.StatusBadRequest, "replay_error_budget", "error_budget"}
	refusalPreFlight   = refusal{http.StatusBadRequest, "replay_preflight_deficit", "preflight_deficit"}
	// refusalTenantUnresolved is SP-5's own refusal: a request whose
	// HeaderTenantID failed to resolve (tenancy.ResolveTenant) never reaches
	// the spend guard, the breaker, the loop detector or the ledger's
	// accumulators pooled under any identity at all.
	refusalTenantUnresolved = refusal{http.StatusBadRequest, "replay_tenant_unresolved", "tenant_unresolved"}
)

// tenantUnresolvedMessage is everything a tenant refusal may say about the
// header that failed to resolve: which class of failure, and how many bytes
// were sent. It takes the length and not the value on purpose. The message
// it builds reaches three places (the REFUSED log line, the HTTP error body
// and ledger.Record.RefusalReason), and the ledger's contract for that field
// is counts and thresholds, never content. The first version pasted
// tenancy's own error, which quotes the raw header with %q, so a client
// could write any string into the operator's ledger by sending it as a
// tenant id that does not validate (R1, PR #336 review). A function that is
// never handed the bytes cannot leak them.
//
// The class comes from errors.Is against tenancy's sentinels, not from the
// error text. An error neither sentinel matches is still refused and still
// bounded; it is only described less precisely.
func tenantUnresolvedMessage(headerLen int, err error) string {
	class := "not a recognised identity"
	switch {
	case errors.Is(err, tenancy.ErrIdentityReserved):
		class = "a reserved word"
	case errors.Is(err, tenancy.ErrIdentityIllegal):
		class = "not a legal identity"
	}
	return fmt.Sprintf("tenant identity could not be resolved: %s is %s (%d bytes); "+
		"omit it to run as the local default, or send a valid identity "+
		"(ASCII letters, digits, -._: only, 1-128 chars, first character alphanumeric)",
		HeaderTenantID, class, headerLen)
}

// refuse answers a request locally in the provider's error shape so any
// client that understands provider errors shows the message to the user.
// refuseSession is refuse with attribution: the same local answer, plus one log
// line naming the guard, the session and the numbers, plus one ledger record.
//
// A guard firing was previously invisible. The counter reached /replay/metrics
// and nothing else, so the observable behaviour of a guard saving somebody
// money overnight was that the request log stopped and the agent showed a
// provider-shaped error. This is the durable half of that fix; the ledger
// record is the other.
//
// It deliberately does not touch the circuit breaker. refusalCircuitOpen is a
// 503 and IsRetryableStatus covers 500-599, so feeding a refusal to
// Breaker.Observe would re-arm the cooldown on every request an open circuit
// refuses, and the circuit would never close.
//
// tenant is named in the log line and the ledger record (SP-7, re-read once
// SP-5 landed: naming a session is not naming a tenant), except when tenant
// is tenancy.LocalTenant — the fixed identity every pre-SP-5 and every solo
// install already runs as, which must stay silent about a dimension it has
// never had and does not need.
func (s *Server) refuseSession(w http.ResponseWriter, tenant tenancy.TenantID, sessionID, model string, kind refusal, message string, retryAfter time.Duration) {
	if s.cfg.Logger != nil {
		id := short(sessionID)
		if id == "" {
			id = "unattributed"
		}
		s.cfg.Logger.Printf("REFUSED %s session=%s%s %s", kind.counter, id, tenantLogSuffix(tenant), message)
	}
	s.recordRefusal(tenant, sessionID, model, kind, message)
	s.refuse(w, kind, message, retryAfter)
}

// tenantLogSuffix is " tenant=<id>" for any tenant other than the local
// default, and empty for it — the same omit-the-default rule
// ledger.Record.TenantID follows, applied to the log line rather than the
// JSON field.
func tenantLogSuffix(tenant tenancy.TenantID) string {
	if tenant == "" || tenant == tenancy.LocalTenant {
		return ""
	}
	return " tenant=" + string(tenant)
}

// recordRefusal writes a refusal to the ledger from its own path.
//
// Deliberately not the bookkeeping defer: that path constructs the response tap
// and calls Breaker.Observe, and refusalCircuitOpen is a 503 that
// IsRetryableStatus counts as a provider failure, so routing refusals through
// it would re-arm the cooldown on every refused request and the circuit would
// never close.
//
// A log line cannot be analysed. This record is what makes it possible to ask,
// tomorrow, how many sessions hit a guard and what they looked like when they
// did, which is the only local answer to a threshold nobody has evidence for.
func (s *Server) recordRefusal(tenant tenancy.TenantID, sessionID, model string, kind refusal, reason string) {
	if s.cfg.Store == nil {
		return
	}
	_ = s.cfg.Store.Append(ledger.Record{
		Schema:         ledger.SchemaVersion,
		Timestamp:      time.Now().UTC(),
		SessionID:      sessionID,
		Status:         kind.status,
		Refusal:        kind.counter,
		RefusalReason:  reason,
		TenantID:       ledger.TenantIDOf(tenant),
		RequestSummary: ledger.RequestSummary{Model: model},
	})
}

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
