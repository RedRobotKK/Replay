package probe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// VaryTerm is one prefix field a two-request experiment can change.
//
// The kernel debate's first meter job: vary one term, watch cache_read.
// If the term is in the provider's cache key, the second request writes
// instead of reading. If it is not, cache_read holds. H(policy, epoch,
// tools, model, effort) is OUR epoch id; this experiment asks which of
// those terms actually move THEIR key.
const (
	VaryTools         = "tools"
	VarySystem        = "system"
	VaryBillingHeader = "billing-header"
	VaryEffort        = "effort"
)

// VaryTerms is the allowed set, in the order the plan prints them.
var VaryTerms = []string{VaryTools, VarySystem, VaryBillingHeader, VaryEffort}

// VaryResult is one two-request experiment. Absence of usage is an error,
// not a zero cache_read (ADR-0018).
type VaryResult struct {
	Term          string
	Model         string
	BaselineRead  int
	VariantRead   int
	BaselineWrite int
	VariantWrite  int
	Moved         bool
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
	_, _ = fmt.Fprintf(r.Out, "  expect       if that term is in the provider cache key, cache_read drops on request 2\n")
	_, _ = fmt.Fprintf(r.Out, "  budget       2 billable requests\n")
	_, _ = fmt.Fprintf(r.Out, "\nNothing has been sent yet. Pass --execute to send.\n")
	return nil
}

// Vary sends the two-request experiment. The filler is shared so the only
// intended difference is the named term.
func (r *Runner) Vary(model, term string) (VaryResult, error) {
	if err := knownVaryTerm(term); err != nil {
		return VaryResult{}, err
	}
	filler, _, err := r.sizedFiller(model, 2048)
	if err != nil {
		return VaryResult{}, err
	}
	base, err := r.sendVary(model, filler, term, false)
	if err != nil {
		return VaryResult{}, err
	}
	vari, err := r.sendVary(model, filler, term, true)
	if err != nil {
		return VaryResult{}, err
	}
	// A first request writes. A cache hit on the second reads that write.
	// If the varied term is in the provider key, the second writes too and
	// cache_read stays 0.
	return VaryResult{
		Term:          term,
		Model:         model,
		BaselineRead:  base.CacheRead,
		VariantRead:   vari.CacheRead,
		BaselineWrite: base.CacheCreation,
		VariantWrite:  vari.CacheCreation,
		Moved:         base.CacheCreation > 0 && vari.CacheRead == 0,
	}, nil
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
	switch {
	case term == VaryBillingHeader && !variant:
		system = filler + " cc_version=bbbbbbbb;"
	case term == VaryBillingHeader && variant:
		system = filler + " cc_version=aaaaaaaa;"
	case term == VarySystem && variant:
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
