package proxy

import (
	"fmt"
	"hash/fnv"
	"time"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/policy"
)

// TrialSettings bound a live trial of a learned policy (LN-5). They
// apply only to a policy read from the policy file; an explicit flag is
// the user's own decision and is never split or reverted.
type TrialSettings struct {
	// Share is the fraction of new sessions that get the policy; the rest
	// run as controls. 1 applies it to every session.
	Share float64
	// ReReadRate is the guardrail: a treated session whose re-read rate
	// after the provider's first clear reaches it has breached. Zero
	// turns the guardrail off.
	ReReadRate float64
	// RevertAfter is how many breached sessions revert the policy for new
	// sessions.
	RevertAfter int
}

// Trial defaults: every session treated, revert after two breaches, and
// no guardrail until the user sets a rate, since the rate that means
// trouble depends on how the agent works.
const (
	DefaultTrialShare  = 1.0
	DefaultRevertAfter = 2
	// minGuardrailReads is how many file reads after the first clear a
	// session needs before its re-read rate is judged.
	minGuardrailReads = 5
	trialTreated      = "treated"
	trialControl      = "control"
)

// normalized fills the zero value's defaults: every session treated,
// revert after DefaultRevertAfter breaches.
func (t TrialSettings) normalized() TrialSettings {
	if t.Share <= 0 {
		t.Share = DefaultTrialShare
	}
	if t.RevertAfter <= 0 {
		t.RevertAfter = DefaultRevertAfter
	}
	return t
}

// treated assigns a session to an arm by a stable hash of its id, so the
// split does not depend on order or restarts.
func (t TrialSettings) treated(sessionID string) bool {
	t = t.normalized()
	if t.Share >= 1 {
		return true
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(sessionID)) // hash.Hash never fails
	return float64(h.Sum32()%1000)/1000 < t.Share
}

// breached reports whether a treated session's re-reads after the first
// clear tripped the guardrail.
func (t TrialSettings) breached(rr analysis.ReReads) bool {
	return t.ReReadRate > 0 && rr.ReadsAfterClear >= minGuardrailReads && rr.RateAfterClear() >= t.ReReadRate
}

// noteBreach records a treated session's first guardrail breach and, once
// enough sessions have breached, reverts the policy for new sessions and
// persists the revert. It returns the log line to print, if any.
func (s *stats) noteBreach(store *ledger.Store, settings TrialSettings, sessionID string, edit *policy.ContextEdit, rr analysis.ReReads, generated time.Time) string {
	settings = settings.normalized()
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.sessions[sessionID]
	if !ok || st.breached {
		return ""
	}
	st.breached = true
	key := policyKey(edit, generated)
	s.breaches[key]++
	n := s.breaches[key]
	line := fmt.Sprintf("guardrail session=%s: re-read rate after clears %.0f%% (%d of %d reads) reached %.0f%%; %d of %d sessions breached", short(sessionID), rr.RateAfterClear()*100, rr.RepeatedAfterClear, rr.ReadsAfterClear, settings.ReReadRate*100, n, settings.RevertAfter)
	if n < settings.RevertAfter || s.reverted[key] {
		return line
	}
	r := ledger.Revert{Policy: policy.Name, Trigger: edit.TriggerTokens, Keep: edit.KeepLast, Reason: fmt.Sprintf("re-read rate after clears reached %.0f%% on %d sessions", settings.ReReadRate*100, n), Breached: n, At: time.Now(), PolicyGenerated: generated}
	if err := store.SetRevert(r); err != nil {
		return line + "; revert not persisted: " + err.Error()
	}
	s.reverted[key] = true
	s.revertReason = r.Reason
	return line + "; policy " + edit.String() + " reverted for new sessions until replay learn writes a newer file"
}

// policyKey identifies the exact policy a session was pinned to.
//
// The parameters alone are not enough: `replay learn` can write a file with the
// same trigger and keep-last after a revert, and that is a new decision made on
// newer evidence, not a continuation of the one that was reverted. The
// generation timestamp is what separates them.
func policyKey(edit *policy.ContextEdit, generated time.Time) string {
	if edit == nil {
		return "none"
	}
	return edit.String() + "@" + generated.UTC().Format(time.RFC3339Nano)
}
