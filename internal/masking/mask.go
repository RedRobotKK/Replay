package masking

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Keys whose string values are never read or changed: the provider's
// reasoning and its signature, and redacted reasoning data. A block typed
// thinking is skipped whole in case its keys come in another order.
var untouchedKeys = map[string]bool{"thinking": true, "signature": true, "data": true}

// Masker applies the pattern set to request bodies.
type Masker struct {
	patterns []Pattern
	vault    *Vault
}

// New builds a masker over the built-in patterns plus user patterns.
func New(vault *Vault, user []Pattern) *Masker {
	return &Masker{patterns: append(append([]Pattern(nil), Patterns...), user...), vault: vault}
}

// Report is what one body's masking did, by pattern name. It never holds
// a secret or a placeholder.
type Report map[string]int

// Total is the number of secrets masked.
func (r Report) Total() int {
	n := 0
	for _, c := range r {
		n += c
	}
	return n
}

// String renders the report as "name:count" pairs, sorted.
func (r Report) String() string {
	names := make([]string, 0, len(r))
	for n := range r {
		names = append(names, n)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, n := range names {
		parts = append(parts, fmt.Sprintf("%s:%d", n, r[n]))
	}
	return strings.Join(parts, ", ")
}

// stringValue is one JSON string literal's raw byte range in the body.
type stringValue struct {
	start, end int // including the quotes
}

// Mask replaces every secret inside the body's JSON string values with
// its placeholder and returns the new body and a report. Every byte
// outside a matched secret is returned as it was. A body that is not
// valid JSON is returned unchanged with an error.
func (m *Masker) Mask(body []byte) ([]byte, Report, error) {
	values, err := stringValues(body)
	if err != nil {
		return body, nil, err
	}
	report := Report{}
	var out []byte
	last := 0
	for _, sv := range values {
		// Match on the literal's inner bytes. Secrets are ASCII and the
		// placeholder is too, so the literal stays valid JSON.
		inner := body[sv.start+1 : sv.end-1]
		matches := Find(inner, m.patterns)
		if len(matches) == 0 {
			continue
		}
		if out == nil {
			out = make([]byte, 0, len(body))
		}
		base := sv.start + 1
		for _, mt := range matches {
			secret := string(inner[mt.Start:mt.End])
			if strings.HasPrefix(secret, PlaceholderPrefix) {
				continue
			}
			ph, err := m.vault.Placeholder(secret, mt.Pattern)
			if err != nil {
				return body, nil, err
			}
			out = append(out, body[last:base+mt.Start]...)
			out = append(out, ph...)
			last = base + mt.End
			report[mt.Pattern]++
		}
	}
	if out == nil {
		return body, report, nil
	}
	out = append(out, body[last:]...)
	return out, report, nil
}

// stringValues locates every JSON string value in the body that may be
// masked, skipping untouched keys and thinking blocks, using the decoder's
// offsets so no byte is re-serialized.
func stringValues(body []byte) ([]stringValue, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var out []stringValue
	type frame struct {
		object   bool
		key      string
		haveKey  bool
		skipAll  bool
		start    int
		captured []stringValue // strings seen in this object, dropped if it turns out to be thinking
	}
	var stack []*frame
	before := int64(0)
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("mask: %w", err)
		}
		after := dec.InputOffset()
		var top *frame
		if len(stack) > 0 {
			top = stack[len(stack)-1]
		}
		switch t := tok.(type) {
		case json.Delim:
			switch t {
			case '{', '[':
				stack = append(stack, &frame{object: t == '{', start: int(after)})
			case '}', ']':
				done := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				if !done.skipAll {
					if len(stack) > 0 {
						stack[len(stack)-1].captured = append(stack[len(stack)-1].captured, done.captured...)
					} else {
						out = append(out, done.captured...)
					}
				}
				if len(stack) > 0 {
					stack[len(stack)-1].haveKey = false
				}
			}
			before = after
			continue
		case string:
			if top != nil && top.object && !top.haveKey {
				top.key, top.haveKey = t, true
				before = after
				continue
			}
			// A value string: find its literal bytes.
			start := bytes.IndexByte(body[before:after], '"') + int(before)
			sv := stringValue{start: start, end: int(after)}
			switch {
			case top == nil:
				out = append(out, sv)
			case !top.object:
				top.captured = append(top.captured, sv)
			case top.key == "type" && (t == "thinking" || t == "redacted_thinking"):
				top.skipAll = true
				top.haveKey = false
			case untouchedKeys[top.key]:
				top.haveKey = false
			default:
				top.captured = append(top.captured, sv)
				top.haveKey = false
			}
		default:
			if top != nil && top.object {
				top.haveKey = false
			}
		}
		before = after
	}
	sort.Slice(out, func(i, j int) bool { return out[i].start < out[j].start })
	return out, nil
}
