package probe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
)

// VaryTerm is one prefix field a two-request experiment can change.
//
// The kernel debate's first meter job: vary one term, watch cache_read.
// If the term is in the provider's cache key, the second request writes
// instead of reading. If it is not, cache_read holds. H(policy, epoch,
// tools, model, effort) is OUR epoch id; this experiment asks which of
// those terms actually move THEIR key.
const (
	VaryTools  = "tools"
	VarySystem = "system"
	VaryEffort = "effort"
)

// A `billing-header` term is deliberately absent.
//
// It was in the first draft and it could not report anything but "moved". It
// put cc_version in the SYSTEM TEXT — the exact bytes the cache breakpoint
// covers — while the only headers this file sends are content-type,
// anthropic-version and x-api-key. So the second request had to write instead
// of read, on every provider, for all time, and the operator spent two
// billable requests to read "the billing header is in the provider's cache
// key" from an experiment that never touched a header.
//
// The control arm made it worse rather than better: the control reads fine, so
// Inconclusive stays false and the fabricated positive arrives wearing a passed
// control. A check that cannot fail is not evidence (ADR-0014), and one that
// cannot fail while displaying a passing control is evidence pointing the wrong
// way. It comes back when it varies a real header, or not at all.

// VaryTerms is the allowed set, in the order the plan prints them.
var VaryTerms = []string{VaryTools, VarySystem, VaryEffort}

// VaryResult is one two-request experiment. Absence of usage is an error,
// not a zero cache_read (ADR-0018).
type VaryResult struct {
	Term          string
	Model         string
	BaselineRead  int
	VariantRead   int
	ControlRead   int
	BaselineWrite int
	VariantWrite  int
	Moved         bool
	Inconclusive  bool
}

// PlanVary prints the two-request experiment and sends nothing.
func (r *Runner) PlanVary(model, term string) error {
	if err := knownVaryTerm(term); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(r.Out, "vary plan for %s\n\n", model)
	_, _ = fmt.Fprintf(r.Out, "  term         %s\n", term)
	_, _ = fmt.Fprintf(r.Out, "  request 1    baseline prefix, cache breakpoint on the system block\n")
	_, _ = fmt.Fprintf(r.Out, "  request 2    the same prefix except %s\n", term)
	_, _ = fmt.Fprintf(r.Out, "  request 3    baseline again, unchanged (control)\n")
	_, _ = fmt.Fprintf(r.Out, "  expect       if that term is in the provider cache key, request 2's cache_read is 0 instead of reading request 1's write; request 3 must read, or the run is inconclusive\n")
	_, _ = fmt.Fprintf(r.Out, "  budget       3 billable requests\n")
	_, _ = fmt.Fprintf(r.Out, "\nNothing has been sent yet. Pass --execute to send.\n")
	return nil
}

// Vary sends the two-request experiment. The filler is shared so the only
// intended difference is the named term.
func (r *Runner) Vary(model, term string) (VaryResult, error) {
	if err := knownVaryTerm(term); err != nil {
		return VaryResult{}, err
	}
	// floor, not min: min is a builtin in this Go version and shadowing it
	// here reads as arithmetic rather than as a provider fact.
	floor := cachemodel.DocumentedMinPrefix(model)
	target := 2048
	if floor > 0 {
		// Half again over the documented floor. There was a second clamp here
		// holding target at least floor+512, and it was deleted rather than
		// tested: it can only fire for a floor under 1024, and the check that
		// actually protects the run is `actual < floor` below, which refuses
		// the experiment outright instead of nudging the target. Two guards
		// where the second cannot change an outcome the first does not already
		// catch is one guard and a decoration.
		target = floor + floor/2
	}
	filler, actual, err := r.sizedFiller(model, target)
	if err != nil {
		return VaryResult{}, err
	}
	// Reachable only when the provider's own counter disagrees with what we
	// built, which no fake in this package can produce honestly: a counter that
	// plateaus below the target makes sizedFiller build unboundedly large
	// fillers chasing it, and the test hangs rather than reaches this line. It
	// is left UNTESTED and said so, rather than covered by a fixture that has
	// to misbehave in a way no provider does.
	//
	// Deleting it does not compile — `actual` goes unused — so the reviewer can
	// only neutralise it, and a compiler-rejected mutant is not a caught one
	// (ADR-0014). What proves the branch live is the run above: with the target
	// guard removed, a model published at 8192 counted 2047 and this refused.
	if floor > 0 && actual < floor {
		return VaryResult{}, fmt.Errorf("the probe prefix counted %d tokens, below this model's minimum cacheable prefix %d; the run would not be able to tell a miss from a floor", actual, floor)
	}
	base, err := r.sendVary(model, filler, term, false)
	if err != nil {
		return VaryResult{}, err
	}
	vari, err := r.sendVary(model, filler, term, true)
	if err != nil {
		return VaryResult{}, err
	}
	ctrl, err := r.sendVary(model, filler, term, false)
	if err != nil {
		return VaryResult{}, err
	}
	out := VaryResult{
		Term:          term,
		Model:         model,
		BaselineRead:  base.CacheRead,
		VariantRead:   vari.CacheRead,
		ControlRead:   ctrl.CacheRead,
		BaselineWrite: base.CacheCreation,
		VariantWrite:  vari.CacheCreation,
	}
	if ctrl.CacheRead == 0 {
		out.Inconclusive = true
		return out, nil
	}
	out.Moved = base.CacheCreation > 0 && vari.CacheRead == 0
	return out, nil
}

func knownVaryTerm(term string) error {
	for _, t := range VaryTerms {
		if t == term {
			return nil
		}
	}
	return fmt.Errorf("unknown vary term %q (want %s)", term, strings.Join(VaryTerms, ", "))
}

func (r *Runner) sendVary(model, filler, term string, variant bool) (usageSplit, error) {
	system := filler
	if term == VarySystem && variant {
		system = filler + "X"
	}
	effort := "high"
	if variant && term == VaryEffort {
		effort = "low"
	}
	payload := map[string]any{
		"model":      model,
		"max_tokens": 1,
		"effort":     effort,
		"system": []map[string]any{{
			"type":          "text",
			"text":          system,
			"cache_control": map[string]string{"type": "ephemeral"},
		}},
		"messages": []map[string]any{{"role": "user", "content": "."}},
	}
	if variant && term == VaryTools {
		payload["tools"] = []map[string]any{{
			"name":         "probe_extra",
			"description":  "x",
			"input_schema": map[string]any{"type": "object", "properties": map[string]any{}},
		}}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return usageSplit{}, err
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(r.BaseURL, "/")+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return usageSplit{}, err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("x-api-key", r.APIKey)
	client := r.Client
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return usageSplit{}, fmt.Errorf("probe request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return usageSplit{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return usageSplit{}, fmt.Errorf("the provider answered %d and the run stopped; nothing was recorded from it", resp.StatusCode)
	}
	return parseVaryUsage(raw)
}

type usageSplit struct {
	CacheCreation int
	CacheRead     int
}

func parseVaryUsage(raw []byte) (usageSplit, error) {
	var parsed struct {
		Usage *struct {
			Input         int `json:"input_tokens"`
			CacheCreation int `json:"cache_creation_input_tokens"`
			CacheRead     int `json:"cache_read_input_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return usageSplit{}, fmt.Errorf("the provider's answer could not be read as usage")
	}
	if parsed.Usage == nil || parsed.Usage.Input <= 0 {
		return usageSplit{}, fmt.Errorf("the provider's answer carried no usage, so it says nothing about caching")
	}
	return usageSplit{CacheCreation: parsed.Usage.CacheCreation, CacheRead: parsed.Usage.CacheRead}, nil
}
