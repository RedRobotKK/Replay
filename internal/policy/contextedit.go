// Package policy holds the request-parameter policies the proxy can apply
// live. Each one is a pure function from the client's bytes to the
// provider's bytes: it adds only what the client left unset, keeps every
// byte the client sent, and says why when it does nothing. The
// admissibility rules are ADR-0003.
package policy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/RedRobotKK/Buffy/internal/analysis"
)

// ContextEdit asks the provider to clear old tool results server-side
// once the prompt passes a trigger. The provider excludes its own context
// editing from the history-binding check, and the cache is invalidated
// only from the earliest cleared block, which is what the replay
// simulator scores.
type ContextEdit struct {
	// TriggerTokens is the prompt size at which the provider clears.
	TriggerTokens int
	// KeepLast is how many most recent tool results survive a clear.
	KeepLast int
}

// Name is the policy's name in ledger records, logs, and status.
const Name = "context-edit"

// BetaFeature is the header value the client must already send for the
// provider to accept the parameter. The proxy never adds header values,
// so a client without it is left alone.
const BetaFeature = "context-management-2025-06-27"

// Provider parameter names, from the context editing documentation.
const (
	parameterKey = "context_management"
	editType     = "clear_tool_uses_20250919"
	betaHeader   = "anthropic-beta"
)

// Decision says what happened to a request and, when nothing did, why.
type Decision string

// Decisions. Every skip names its reason so a log reader never guesses.
const (
	Applied         Decision = "applied"
	SkipClientSet   Decision = "skipped: client set " + parameterKey
	SkipNoBeta      Decision = "skipped: client did not enable " + BetaFeature
	SkipNotAnObject Decision = "skipped: body is not a JSON object"
	SkipInvalid     Decision = "skipped: edited body would not parse"
	// NotConfigured is the decision pinned for a session that started
	// while no policy was configured or selected.
	NotConfigured Decision = "no policy configured"
	// Control is pinned for a session a live trial holds out so the
	// treated sessions have something to be compared with.
	Control Decision = "control: held out of the trial"
	// Reverted is pinned for a session that started after the trial's
	// guardrail tripped.
	Reverted Decision = "reverted: the trial's guardrail tripped"
)

// Validate rejects parameters the provider or the simulator would not
// accept.
func (p ContextEdit) Validate() error {
	if p.TriggerTokens <= 0 {
		return fmt.Errorf("%s: trigger must be positive, got %d", Name, p.TriggerTokens)
	}
	if p.KeepLast < 0 {
		return fmt.Errorf("%s: keep must not be negative, got %d", Name, p.KeepLast)
	}
	return nil
}

// Simulated is the replay policy with the same parameters, so the live
// what-if rows and the applied policy describe one thing.
func (p ContextEdit) Simulated() analysis.ContextEditPolicy {
	return analysis.ContextEditPolicy{KeepLast: p.KeepLast, TriggerTokens: p.TriggerTokens}
}

// Admissible reports whether the request may carry the parameter at all:
// the client enabled the provider feature and did not set the parameter
// itself. It reads only what the proxy already parsed.
func (p ContextEdit) Admissible(beta string, clientSet bool) Decision {
	if clientSet {
		return SkipClientSet
	}
	if !hasBeta(beta, BetaFeature) {
		return SkipNoBeta
	}
	return Applied
}

// Apply returns the body with the parameter added as the last member of
// the top-level object, and a decision. Every byte the client sent is
// kept in place; only a comma and the new member are spliced in before
// the closing brace. On any doubt the original body is returned.
func (p ContextEdit) Apply(body []byte) ([]byte, Decision) {
	end := bytes.LastIndexByte(body, '}')
	start := bytes.IndexByte(body, '{')
	if start < 0 || end <= start {
		return body, SkipNotAnObject
	}
	member := fmt.Sprintf(`%q:%s`, parameterKey, p.parameter())
	inner := bytes.TrimSpace(body[start+1 : end])
	var out []byte
	out = append(out, body[:end]...)
	if len(inner) > 0 {
		out = append(out, ',')
	}
	out = append(out, member...)
	out = append(out, body[end:]...)
	if !json.Valid(out) {
		return body, SkipInvalid
	}
	return out, Applied
}

// parameter renders the provider's value. clear_at_least mirrors the
// simulator's hysteresis so a clear removes enough to stay below the
// trigger for a while instead of invalidating the cache every turn.
func (p ContextEdit) parameter() string {
	clearAtLeast := int(float64(p.TriggerTokens) * analysis.ClearHysteresis)
	return fmt.Sprintf(`{"edits":[{"type":%q,"trigger":{"type":"input_tokens","value":%d},"keep":{"type":"tool_uses","value":%d},"clear_at_least":{"type":"input_tokens","value":%d}}]}`, editType, p.TriggerTokens, p.KeepLast, clearAtLeast)
}

// String names the policy with its parameters, for status and logs.
func (p ContextEdit) String() string {
	return fmt.Sprintf("%s(keep=%d,trigger=%d)", Name, p.KeepLast, p.TriggerTokens)
}

// hasBeta reports whether a comma-separated beta header lists a feature.
func hasBeta(header, feature string) bool {
	for _, f := range strings.Split(header, ",") {
		if strings.EqualFold(strings.TrimSpace(f), feature) {
			return true
		}
	}
	return false
}
