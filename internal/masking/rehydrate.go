package masking

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// placeholderRE matches a placeholder in a JSON string literal's bytes.
// Placeholder characters never need escaping, so a placeholder the
// model spelled with escapes (BUFFY_...) does not match and stays a
// placeholder: forgery through escaping fails closed.
var placeholderRE = regexp.MustCompile(PlaceholderPrefix + `[0-9a-f]{` + strconv.Itoa(placeholderHex) + `}`)

// Rehydrator restores placeholders in response bodies within scope.
type Rehydrator struct {
	vault  *Vault
	scopes Scopes
}

// NewRehydrator builds a rehydrator over a vault and a scope policy.
func NewRehydrator(vault *Vault, scopes Scopes) *Rehydrator {
	return &Rehydrator{vault: vault, scopes: scopes}
}

// Scopes returns the policy in force.
func (r *Rehydrator) Scopes() Scopes { return r.scopes }

// RehydrationReport is what one response's rehydration did. It never holds
// a secret, a placeholder, or a path.
type RehydrationReport struct {
	// Restored counts placeholders restored, by destination.
	Restored map[string]int
	// Denied counts placeholders left in place, by "destination/reason".
	Denied map[string]int
}

func (r *RehydrationReport) restored(d Destination) {
	if r.Restored == nil {
		r.Restored = map[string]int{}
	}
	r.Restored[d.String()]++
}

func (r *RehydrationReport) denied(d Destination, reason string) {
	if r.Denied == nil {
		r.Denied = map[string]int{}
	}
	r.Denied[d.String()+"/"+reason]++
}

// Empty reports whether nothing was restored or denied.
func (r RehydrationReport) Empty() bool {
	return len(r.Restored) == 0 && len(r.Denied) == 0
}

// Total is the number of placeholders restored.
func (r RehydrationReport) Total() int {
	n := 0
	for _, c := range r.Restored {
		n += c
	}
	return n
}

// Summary renders counts as "key:count" pairs, sorted.
func summary(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s:%d", k, m[k]))
	}
	return strings.Join(parts, ", ")
}

// RestoredSummary renders the restored counts.
func (r RehydrationReport) RestoredSummary() string { return summary(r.Restored) }

// DeniedSummary renders the denied counts.
func (r RehydrationReport) DeniedSummary() string { return summary(r.Denied) }

// ErrInvalidResult is returned when rewriting a body produced invalid
// JSON; the caller forwards the original bytes.
var ErrInvalidResult = errors.New("rehydration produced invalid JSON")

// responseMessage is the part of a non-streaming response that decides
// where each string literal lives.
type responseMessage struct {
	Type    string `json:"type"`
	Content []struct {
		Type  string          `json:"type"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	} `json:"content"`
}

// Body restores placeholders in a non-streaming Messages API response.
// Only the bytes of a restored placeholder change. A body that is not a
// message, or that the walker cannot read, is returned as it was.
func (r *Rehydrator) Body(body []byte) ([]byte, RehydrationReport, error) {
	var rep RehydrationReport
	var msg responseMessage
	if err := json.Unmarshal(body, &msg); err != nil || msg.Type != "message" {
		return body, rep, nil
	}
	lits, err := literals(body)
	if err != nil {
		return body, rep, err
	}
	dests := map[int]Destination{}
	var out []byte
	last := 0
	for _, lit := range lits {
		if len(lit.path) < 3 || lit.path[0] != "content" {
			continue
		}
		idx, err := strconv.Atoi(lit.path[1])
		if err != nil || idx < 0 || idx >= len(msg.Content) {
			continue
		}
		block := msg.Content[idx]
		var dest Destination
		switch {
		case block.Type == "text" && lit.path[2] == "text":
			dest = Destination{Kind: DestinationText}
		case block.Type == "tool_use" && lit.path[2] == "input":
			d, ok := dests[idx]
			if !ok {
				d = r.scopes.toolDestination(block.Name, block.Input)
				dests[idx] = d
			}
			dest = d
		default:
			// Thinking, signatures, ids, names, and every other field
			// are never read for placeholders.
			continue
		}
		inner := body[lit.start+1 : lit.end-1]
		restored, changed := r.restore(inner, dest, &rep)
		if !changed {
			continue
		}
		if out == nil {
			out = make([]byte, 0, len(body))
		}
		out = append(out, body[last:lit.start+1]...)
		out = append(out, restored...)
		last = lit.end - 1
	}
	if out == nil {
		return body, rep, nil
	}
	out = append(out, body[last:]...)
	if !json.Valid(out) {
		return body, RehydrationReport{}, ErrInvalidResult
	}
	return out, rep, nil
}

// restore replaces the placeholders in a string literal's inner bytes
// that the scope allows into the destination. The secret goes in as the
// vault holds it, in literal form, so the literal stays valid.
func (r *Rehydrator) restore(inner []byte, dest Destination, rep *RehydrationReport) ([]byte, bool) {
	matches := placeholderRE.FindAllIndex(inner, -1)
	if len(matches) == 0 {
		return inner, false
	}
	var out []byte
	last := 0
	for _, m := range matches {
		secret, pattern, ok := r.vault.Secret(string(inner[m[0]:m[1]]))
		if !ok {
			rep.denied(dest, ReasonUnknown)
			continue
		}
		if reason := r.scopes.For(pattern).deny(dest); reason != "" {
			rep.denied(dest, reason)
			continue
		}
		if out == nil {
			out = make([]byte, 0, len(inner))
		}
		out = append(out, inner[last:m[0]]...)
		out = append(out, secret...)
		last = m[1]
		rep.restored(dest)
	}
	if out == nil {
		return inner, false
	}
	return append(out, inner[last:]...), true
}

// restoreJSONText restores placeholders inside the string literals of a
// JSON document (a tool input) for one destination. Placeholders outside
// a string literal, in a document that does not parse, are counted as
// denied and nothing changes.
func (r *Rehydrator) restoreJSONText(doc []byte, dest Destination, rep *RehydrationReport) ([]byte, bool) {
	lits, err := literals(doc)
	if err != nil {
		for range placeholderRE.FindAllIndex(doc, -1) {
			rep.denied(dest, ReasonUnparsedInput)
		}
		return doc, false
	}
	var out []byte
	last := 0
	for _, lit := range lits {
		restored, changed := r.restore(doc[lit.start+1:lit.end-1], dest, rep)
		if !changed {
			continue
		}
		if out == nil {
			out = make([]byte, 0, len(doc))
		}
		out = append(out, doc[last:lit.start+1]...)
		out = append(out, restored...)
		last = lit.end - 1
	}
	if out == nil {
		return doc, false
	}
	return append(out, doc[last:]...), true
}

// inputPath finds a file-edit tool's target path in its input object.
func inputPath(input []byte) (string, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(input, &fields); err != nil {
		return "", err
	}
	for _, k := range pathKeys {
		raw, ok := fields[k]
		if !ok {
			continue
		}
		var p string
		if err := json.Unmarshal(raw, &p); err != nil {
			return "", err
		}
		return p, nil
	}
	return "", nil
}
