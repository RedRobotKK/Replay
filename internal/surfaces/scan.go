package surfaces

import (
	"bufio"
	"encoding/json"
	"io"
)

// FieldSpec names the counters one surface uses. Every agent store spells them
// differently and nests them differently, and at least one spells two
// different quantities the same way.
type FieldSpec struct {
	Name     string
	ReadKey  string
	WriteKey string
	// ExcludeUnder is a parent key whose subtree is skipped. OpenClaw carries
	// `cacheRead` twice per record, in tokens at /message/usage and in dollars
	// at /message/usage/cost. Summing both mixes units and produces a token
	// count with a decimal point.
	ExcludeUnder string
}

// Scan reads JSONL and reports what one surface's own records establish.
//
// A record counts only when its read counter is present and numeric. A read
// counter that is a string is a broken reader, not a zero, and contributes
// nothing rather than pulling the total down.
func Scan(r io.Reader, spec FieldSpec) Reading {
	out := Reading{Name: spec.Name}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var v any
		if json.Unmarshal(line, &v) != nil {
			continue
		}
		walk(v, spec, &out)
	}
	return out
}

// walk descends looking for objects carrying the read key, skipping any
// subtree under ExcludeUnder.
func walk(v any, spec FieldSpec, out *Reading) {
	switch t := v.(type) {
	case map[string]any:
		if raw, ok := t[spec.ReadKey]; ok {
			if n, ok := numeric(raw); ok {
				out.Records++
				out.Reads += n
				if w, present := t[spec.WriteKey]; present {
					out.WriteFieldPresent = true
					if wn, ok := numeric(w); ok {
						out.Writes += wn
					}
				}
			}
		}
		for k, child := range t {
			if spec.ExcludeUnder != "" && k == spec.ExcludeUnder {
				continue
			}
			walk(child, spec, out)
		}
	case []any:
		for _, child := range t {
			walk(child, spec, out)
		}
	}
}

// numeric accepts only a JSON number. A string in a counter position is a
// reader problem and reports false rather than zero.
func numeric(v any) (int64, bool) {
	f, ok := v.(float64)
	if !ok {
		return 0, false
	}
	return int64(f), true
}
