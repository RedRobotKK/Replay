package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/learn"
	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/policy"
)

// Tests for policyinject.go: a session decides once and is pinned for its
// life, a rewritten policy file or a restart cannot change a session already
// running, --no-policy overrides a persisted pin, an unparsed body never
// receives the parameter, and the trial splits sessions into arms the
// guardrail can revert.
//
// Every one of these is a test of the invariant that makes editing a request
// body admissible at all, so they belong in the same file as the edit.
// Nothing in them changed in the move, and moving them does not close the
// coverage gaps policyrefusal_test.go names in its own header.

// The live policy adds exactly one member to the client's body, is
// recorded on the ledger with the provider's applied edits, is logged with
// hashes only, and is pinned per session from the first request.
func TestContextEditPolicyIsAppliedRecordedAndPinned(t *testing.T) {
	up := &bodyEcho{}
	base, dir, logs := startProxyWith(t, up, Config{ContextEdit: &policy.ContextEdit{TriggerTokens: 150000, KeepLast: 6}})
	withBeta := map[string]string{HeaderSessionID: "sess-on", "anthropic-beta": "fast-mode-2026-02-01," + policy.BetaFeature}
	noBeta := map[string]string{HeaderSessionID: "sess-off"}

	postWith(t, base, requestBody, withBeta)
	postWith(t, base, requestBody, noBeta)
	// The session pinned on stays on; a request in it without the beta
	// header goes through unchanged and the skip is logged.
	postWith(t, base, requestBody, map[string]string{HeaderSessionID: "sess-on"})
	// The session pinned off stays off even when a later request admits it.
	postWith(t, base, requestBody, map[string]string{HeaderSessionID: "sess-off", "anthropic-beta": policy.BetaFeature})
	// A client that set the parameter itself is never overridden.
	clientSet := strings.TrimSuffix(requestBody, "}") + `,"context_management":{"edits":[]}}`
	postWith(t, base, clientSet, map[string]string{HeaderSessionID: "sess-client", "anthropic-beta": policy.BetaFeature})

	bodies := up.seen()
	if len(bodies) != 5 {
		t.Fatalf("upstream saw %d requests", len(bodies))
	}
	if !bytes.HasPrefix(bodies[0], []byte(strings.TrimSuffix(requestBody, "}"))) || !bytes.Contains(bodies[0], []byte(`"context_management":{"edits":[{"type":"clear_tool_uses_20250919","trigger":{"type":"input_tokens","value":150000}`)) {
		t.Fatalf("first request must carry the parameter after the client's bytes: %s", bodies[0])
	}
	for i, want := range []string{requestBody, requestBody, requestBody, clientSet} {
		if string(bodies[i+1]) != want {
			t.Fatalf("request %d must be byte-identical to the client's: %s", i+1, bodies[i+1])
		}
	}

	recs := waitLedger(t, dir, 5)
	byID := map[string][]ledger.Record{}
	for _, r := range recs {
		byID[r.SessionID] = append(byID[r.SessionID], r)
	}
	on := byID["sess-on"]
	if len(on) != 2 || on[0].Policy != policy.Name || on[1].Policy != "" || on[0].Response.AppliedEdits != 1 || on[0].Response.ClearedInputTokens != 1234 {
		t.Fatalf("ledger for the pinned-on session wrong: %+v", on)
	}
	for _, id := range []string{"sess-off", "sess-client"} {
		for _, r := range byID[id] {
			if r.Policy != "" {
				t.Fatalf("session %s must never carry the policy: %+v", id, r)
			}
		}
	}

	log := logs.String()
	if !strings.Contains(log, "policy context-edit(keep=6,trigger=150000) session=sess-on applied body sha256 before=") || strings.Contains(log, "be brief") {
		t.Fatalf("applied transformation must be logged with hashes and never content:\n%s", log)
	}
	if !strings.Contains(log, string(policy.SkipNoBeta)) {
		t.Fatalf("skip in a pinned-on session must be logged:\n%s", log)
	}

	st := getStatus(t, base)
	for _, sess := range st.Sessions {
		switch sess.Session {
		case "sess-on":
			if sess.Policy != string(policy.Applied) || sess.PolicyApplied != 1 || sess.ClearedInputTokens != 1234*2 {
				t.Fatalf("status for sess-on: %+v", sess)
			}
		case "sess-off":
			if sess.Policy != string(policy.SkipNoBeta) || sess.PolicyApplied != 0 {
				t.Fatalf("status for sess-off: %+v", sess)
			}
		case "sess-client":
			if sess.Policy != string(policy.SkipClientSet) {
				t.Fatalf("status for sess-client: %+v", sess)
			}
		}
	}
	resp, err := http.Get(base + "/replay/metrics")
	if err != nil {
		t.Fatal(err)
	}
	metrics, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(string(metrics), `replay_policy_applied_total{policy="context-edit"} 1`) {
		t.Fatalf("metrics missing policy counter:\n%s", metrics)
	}
}

// writePolicyFile writes a learn result selecting a context-edit trigger,
// or selecting nothing when trigger is zero.
func writePolicyFile(t *testing.T, path string, trigger int) {
	t.Helper()
	res := learn.Result{Schema: learn.PolicyFileSchema, Rules: cachemodel.RulesVersion, Reason: "test"}
	if trigger > 0 {
		p := analysis.ContextEditPolicy{KeepLast: 6, TriggerTokens: trigger}
		res.Selected = &learn.Candidate{Name: fmt.Sprintf("context-edit(keep=6,trigger=%d)", trigger), Family: learn.FamilyContextEdit, ContextEdit: &p}
	}
	data, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func triggerSeen(body []byte) string {
	i := bytes.Index(body, []byte(`"trigger":{"type":"input_tokens","value":`))
	if i < 0 {
		return "none"
	}
	rest := body[i+len(`"trigger":{"type":"input_tokens","value":`):]
	j := bytes.IndexByte(rest, '}')
	return string(rest[:j])
}

// PX-8: the policy is chosen at a session's first request from the policy
// file and pinned for the session's life, on disk, so neither a rewritten
// file nor a restarted proxy changes a running session.
func TestPolicyFileIsReadAtSessionStartAndPinsSurviveRewritesAndRestarts(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "policy.json")
	writePolicyFile(t, file, 200000)
	up := &bodyEcho{}
	base, _, logs := startProxyIn(t, up, Config{PolicyFile: file}, dir)
	beta := func(id string) map[string]string {
		return map[string]string{HeaderSessionID: id, "anthropic-beta": policy.BetaFeature}
	}
	postWith(t, base, requestBody, beta("sess-a"))
	writePolicyFile(t, file, 400000)
	postWith(t, base, requestBody, beta("sess-a"))
	postWith(t, base, requestBody, beta("sess-b"))
	writePolicyFile(t, file, 0)
	postWith(t, base, requestBody, beta("sess-c"))
	got := up.seen()
	if len(got) != 4 {
		t.Fatalf("upstream saw %d requests", len(got))
	}
	for i, want := range []string{"200000", "200000", "400000", "none"} {
		if triggerSeen(got[i]) != want {
			t.Fatalf("request %d carried trigger %s, want %s", i, triggerSeen(got[i]), want)
		}
	}
	if !strings.Contains(logs.String(), "policy file selects nothing") {
		t.Fatalf("an empty selection must be logged:\n%s", logs.String())
	}
	st := getStatus(t, base)
	for _, sess := range st.Sessions {
		switch sess.Session {
		case "sess-a":
			if sess.PinnedPolicy != "context-edit(keep=6,trigger=200000)" || sess.Policy != string(policy.Applied) {
				t.Fatalf("sess-a status: %+v", sess)
			}
		case "sess-c":
			if sess.PinnedPolicy != "" || sess.Policy != string(policy.NotConfigured) {
				t.Fatalf("sess-c status: %+v", sess)
			}
		}
	}

	// A new proxy over the same ledger directory, with the file now
	// selecting 400k: sess-a keeps 200k from its persisted pin, sess-c
	// keeps none, and a fresh session gets 400k.
	writePolicyFile(t, file, 400000)
	up2 := &bodyEcho{}
	base2, _, logs2 := startProxyIn(t, up2, Config{PolicyFile: file}, dir)
	postWith(t, base2, requestBody, beta("sess-a"))
	postWith(t, base2, requestBody, beta("sess-c"))
	postWith(t, base2, requestBody, beta("sess-d"))
	got = up2.seen()
	for i, want := range []string{"200000", "none", "400000"} {
		if triggerSeen(got[i]) != want {
			t.Fatalf("after restart request %d carried trigger %s, want %s", i, triggerSeen(got[i]), want)
		}
	}
	if !strings.Contains(logs2.String(), "pinned earlier") {
		t.Fatalf("a restored pin must be logged:\n%s", logs2.String())
	}
}

// The explicit flag wins over the file, and a stale file applies nothing.
func TestPolicyFlagWinsOverFileAndStaleFileAppliesNothing(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "policy.json")
	writePolicyFile(t, file, 400000)
	up := &bodyEcho{}
	base, _, _ := startProxyIn(t, up, Config{PolicyFile: file, ContextEdit: &policy.ContextEdit{TriggerTokens: 150000, KeepLast: 6}}, dir)
	postWith(t, base, requestBody, map[string]string{HeaderSessionID: "sess-flag", "anthropic-beta": policy.BetaFeature})
	if got := triggerSeen(up.seen()[0]); got != "150000" {
		t.Fatalf("flag must win over the file: %s", got)
	}

	stale := filepath.Join(t.TempDir(), "stale.json")
	if err := os.WriteFile(stale, []byte(`{"schema":99,"rules":"x","selected":{"name":"context-edit","family":"context-edit","context_edit":{"KeepLast":6,"TriggerTokens":100}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	up2 := &bodyEcho{}
	base2, _, logs := startProxyWith(t, up2, Config{PolicyFile: stale})
	postWith(t, base2, requestBody, map[string]string{HeaderSessionID: "sess-stale", "anthropic-beta": policy.BetaFeature})
	if got := triggerSeen(up2.seen()[0]); got != "none" || !strings.Contains(logs.String(), "schema 99") {
		t.Fatalf("stale file must apply nothing and say why: %s\n%s", got, logs.String())
	}
}

// PX-6: turning policies off must stop a session an earlier process
// pinned on, and a restart with no policy source must do the same.
func TestPoliciesOffOverrideAPersistedPin(t *testing.T) {
	dir := t.TempDir()
	up := &bodyEcho{}
	base, _, _ := startProxyIn(t, up, Config{ContextEdit: &policy.ContextEdit{TriggerTokens: 150000, KeepLast: 6}}, dir)
	beta := map[string]string{HeaderSessionID: "sess-pinned", "anthropic-beta": policy.BetaFeature}
	postWith(t, base, requestBody, beta)
	if triggerSeen(up.seen()[0]) != "150000" {
		t.Fatal("first process must pin the session on")
	}
	for name, cfg := range map[string]Config{
		"no policy env":    {ContextEdit: &policy.ContextEdit{TriggerTokens: 150000, KeepLast: 6}, NoPolicy: true},
		"no policy source": {},
	} {
		t.Run(name, func(t *testing.T) {
			up2 := &bodyEcho{}
			base2, _, _ := startProxyIn(t, up2, cfg, dir)
			postWith(t, base2, requestBody, beta)
			got := up2.seen()
			if len(got) != 1 || strings.Contains(string(got[0]), "context_management") {
				t.Fatalf("pinned session must run untouched: %s", got)
			}
		})
	}
}

// A body the summarizer cannot read never gets the parameter, since it
// may already carry one the summarizer failed to see.
func TestUnparsedRequestsNeverCarryThePolicy(t *testing.T) {
	up := &bodyEcho{}
	base, _, logs := startProxyWith(t, up, Config{ContextEdit: &policy.ContextEdit{TriggerTokens: 150000, KeepLast: 6}})
	odd := `{"model":"claude-opus-5","max_tokens":50,"messages":[{"role":"user","content":{"unexpected":"shape"}}]}`
	postWith(t, base, odd, map[string]string{HeaderSessionID: "sess-odd", "anthropic-beta": policy.BetaFeature})
	if got := up.seen(); len(got) != 1 || string(got[0]) != odd {
		t.Fatalf("unparsed body must pass through byte for byte: %s", got)
	}
	if !strings.Contains(logs.String(), string(policy.SkipUnparsed)) {
		t.Fatalf("skip must be logged:\n%s", logs.String())
	}
}

// A session gets the selection learned for its type, judged at its first
// request from the model and the prompt's size, and the type is pinned.
func TestPolicyIsChosenBySessionType(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "policy.json")
	large := analysis.ContextEditPolicy{KeepLast: 6, TriggerTokens: 300000}
	res := learn.Result{Schema: learn.PolicyFileSchema, Rules: cachemodel.RulesVersion, Generated: time.Now(), Reason: "types disagree",
		Types: []learn.TypeResult{{Type: "opus/large-prefix", Sessions: 9, Selected: &learn.Candidate{Name: "context-edit(keep=6,trigger=300000)", Family: learn.FamilyContextEdit, ContextEdit: &large}}, {Type: "opus/small-prefix", Sessions: 9, Reason: "hurts"}}}
	data, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, data, 0o600); err != nil {
		t.Fatal(err)
	}
	up := &bodyEcho{}
	base, _, logs := startProxyIn(t, up, Config{PolicyFile: file}, dir)
	big := `{"model":"claude-opus-5","max_tokens":50,"system":"` + strings.Repeat("s", 90_000) + `","messages":[{"role":"user","content":"hi"}]}`
	postSession(t, base, "sess-large", big)
	postSession(t, base, "sess-small", requestBody)
	got := up.seen()
	if triggerSeen(got[0]) != "300000" || triggerSeen(got[1]) != "none" {
		t.Fatalf("large-prefix session must get its type's policy and small none: %s / %s", triggerSeen(got[0]), triggerSeen(got[1]))
	}
	store, err := ledger.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if p, ok := store.Pin("sess-large"); !ok || p.Type != "opus/large-prefix" || p.Trigger != 300000 {
		t.Fatalf("large pin: %+v %v", p, ok)
	}
	if p, ok := store.Pin("sess-small"); !ok || p.Type != "opus/small-prefix" || p.Policy != "" {
		t.Fatalf("small pin: %+v %v", p, ok)
	}
	if !strings.Contains(logs.String(), "type=opus/small-prefix runs without a policy") {
		t.Fatalf("the typed no-selection must be logged:\n%s", logs.String())
	}
}

// writePolicyFileAt is writePolicyFile with a generation time, which a
// revert is tied to.
func writePolicyFileAt(t *testing.T, path string, trigger int, generated time.Time) {
	t.Helper()
	p := analysis.ContextEditPolicy{KeepLast: 6, TriggerTokens: trigger}
	res := learn.Result{Schema: learn.PolicyFileSchema, Rules: cachemodel.RulesVersion, Generated: generated, Selected: &learn.Candidate{Name: fmt.Sprintf("context-edit(keep=6,trigger=%d)", trigger), Family: learn.FamilyContextEdit, ContextEdit: &p}}
	data, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// sessionInArm finds a session id the trial assigns to the wanted arm.
func sessionInArm(t *testing.T, settings TrialSettings, treated bool) string {
	t.Helper()
	for i := 0; i < 1000; i++ {
		id := fmt.Sprintf("trial-%d", i)
		if settings.treated(id) == treated {
			return id
		}
	}
	t.Fatal("no session id lands in the wanted arm")
	return ""
}

func postSession(t *testing.T, base, id, body string) {
	t.Helper()
	postWith(t, base, body, map[string]string{HeaderSessionID: id, "anthropic-beta": policy.BetaFeature})
}

// readingBody renders turn n of a session that reads the same file on
// every turn, which is what a context edit makes expensive.
func readingBody(turn int) string {
	filler := strings.Repeat("y", 700)
	msgs := []string{`{"role":"user","content":"` + strings.Repeat("x", 700) + `"}`}
	for i := 1; i < turn; i++ {
		id := fmt.Sprintf("r%d", i)
		msgs = append(msgs,
			`{"role":"assistant","content":[{"type":"tool_use","id":"`+id+`","name":"Read","input":{"file_path":"/src/a.go"}}]}`,
			`{"role":"user","content":[{"type":"tool_result","tool_use_id":"`+id+`","content":"`+filler+`"}]}`)
	}
	return `{"model":"claude-opus-5","max_tokens":50,"system":"be brief","messages":[` + strings.Join(msgs, ",") + `]}`
}

// lastBody returns the body of the request the upstream saw after the
// first n.
func lastBody(t *testing.T, up *invariantUpstream, n int) []byte {
	t.Helper()
	up.mu.Lock()
	defer up.mu.Unlock()
	if len(up.bodies) <= n {
		t.Fatalf("upstream saw %d requests, want more than %d", len(up.bodies), n)
	}
	return up.bodies[n]
}

// LN-5: a learned policy is tried on a stable share of new sessions with
// the rest held out as controls, and the arms are pinned.
func TestTrialShareSplitsSessionsIntoArms(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "policy.json")
	writePolicyFileAt(t, file, 200000, time.Now())
	settings := TrialSettings{Share: 0.5, RevertAfter: DefaultRevertAfter}
	up := &bodyEcho{}
	base, _, logs := startProxyIn(t, up, Config{PolicyFile: file, Trial: settings}, dir)
	treated, control := sessionInArm(t, settings, true), sessionInArm(t, settings, false)
	postSession(t, base, treated, requestBody)
	postSession(t, base, control, requestBody)
	got := up.seen()
	if triggerSeen(got[0]) != "200000" || triggerSeen(got[1]) != "none" {
		t.Fatalf("treated must carry the parameter and control must not: %s / %s", triggerSeen(got[0]), triggerSeen(got[1]))
	}
	st := getStatus(t, base)
	if st.Trial.Treated != 1 || st.Trial.Control != 1 || st.Trial.Reverted != "" {
		t.Fatalf("trial status: %+v", st.Trial)
	}
	store, err := ledger.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if p, ok := store.Pin(control); !ok || p.Trial != trialControl || p.Decision != string(policy.Control) {
		t.Fatalf("control pin: %+v %v", p, ok)
	}
	if p, ok := store.Pin(treated); !ok || p.Trial != trialTreated || p.Policy != policy.Name {
		t.Fatalf("treated pin: %+v %v", p, ok)
	}
	if !strings.Contains(logs.String(), "is a control") {
		t.Fatalf("control assignment must be logged:\n%s", logs.String())
	}
}

// LN-5: treated sessions whose re-read rate after the provider's clears
// reaches the guardrail breach it; enough breaches revert the policy for
// new sessions, persistently, until a newer learning result lifts it.
func TestGuardrailBreachesRevertThePolicyUntilANewerFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "policy.json")
	learned := time.Now().Add(-time.Hour)
	writePolicyFileAt(t, file, 200000, learned)
	settings := TrialSettings{Share: 1, ReReadRate: 0.5, RevertAfter: 2}
	up := &invariantUpstream{perTurn: 800, edits: true}
	base, _, logs := startProxyIn(t, up, Config{PolicyFile: file, Trial: settings}, dir)
	const turns = 7
	// Session ids are logged truncated to twelve characters.
	for _, id := range []string{"breach-1", "breach-2"} {
		for i := 1; i <= turns; i++ {
			postSession(t, base, id, readingBody(i))
		}
		waitFor(t, "guardrail to be judged for "+id, func() bool {
			return strings.Contains(logs.String(), "guardrail session="+id+":")
		})
	}
	waitFor(t, "revert to be recorded", func() bool { return getStatus(t, base).Trial.Reverted != "" })
	st := getStatus(t, base)
	if st.Trial.Breached != 2 || !strings.Contains(st.Trial.Reverted, "re-read rate after clears") {
		t.Fatalf("trial status after breaches: %+v", st.Trial)
	}
	if !strings.Contains(logs.String(), "reverted for new sessions") {
		t.Fatalf("revert must be logged:\n%s", logs.String())
	}
	before := up.requests()
	postSession(t, base, "sess-after-revert", requestBody)
	if got := triggerSeen(lastBody(t, up, before)); got != "none" {
		t.Fatalf("a session after the revert must run without the policy, got trigger %s", got)
	}
	store, err := ledger.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if r, ok := store.Revert(); !ok || r.Breached != 2 || !r.PolicyGenerated.Equal(learned) {
		t.Fatalf("revert not persisted: %+v %v", r, ok)
	}
	if p, ok := store.Pin("sess-after-revert"); !ok || p.Decision != string(policy.Reverted) {
		t.Fatalf("post-revert pin: %+v %v", p, ok)
	}

	// A restarted proxy honors the revert; a newer learning result lifts it.
	up2 := &invariantUpstream{perTurn: 800}
	base2, _, _ := startProxyIn(t, up2, Config{PolicyFile: file, Trial: settings}, dir)
	postSession(t, base2, "sess-restart", requestBody)
	if got := triggerSeen(lastBody(t, up2, 0)); got != "none" {
		t.Fatalf("revert must survive a restart, got trigger %s", got)
	}
	writePolicyFileAt(t, file, 300000, time.Now())
	postSession(t, base2, "sess-relearned", requestBody)
	if got := triggerSeen(lastBody(t, up2, 1)); got != "300000" {
		t.Fatalf("a newer policy file must lift the revert, got trigger %s", got)
	}
}
