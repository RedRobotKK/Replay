package surface

import (
	"bufio"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Observables is what a probe found at ONE measurement boundary.
//
// The boundary is part of the identity, not the surface. Codex at its
// transcript and Codex at the OpenAI API seam are two boundaries of one
// surface and they do not classify the same way. That is why the field is
// Boundary rather than Surface.
type Observables struct {
	Boundary string
	Records  int

	// WriteFieldPresent separates "the schema has no such field" from "the
	// schema has it and it is always zero". Collapsing those two loses the
	// distinction this package exists for: Cursor has no field, Codex has one
	// and never fills it, and they are different failures with different
	// repairs.
	WriteFieldPresent bool

	// Two counts, because they answer different questions and a single
	// number conflating them has already caused one misreading. A surface
	// may carry the same counter more than once in a record: Grok states it
	// at `usage` and again at `modelUsage/<model>`, so 1,144 records yield
	// 2,288 observations. Reporting only the second makes a corpus look
	// twice its size; reporting only the first hides that the field is
	// repeated. Both are kept.
	RecordsWithWrite  int // records carrying the write field at least once
	WriteObservations int // total occurrences of it
	WriteNonZero      int // occurrences that are non-zero

	RecordsWithRead  int
	ReadObservations int
	ReadNonZero      int

	// PerRequestSequencing is whether consecutive requests can be differenced.
	// A cumulative counter alone cannot answer a question about a boundary.
	PerRequestSequencing bool

	// OracleClasses counts the distinct PROVIDER-declared cause classes found.
	// Zero means no oracle. Replay's own detector is never counted here, and
	// Probe has no path that could: it reads named provider fields only.
	OracleClasses int
}

// FieldSpec names the counters to look for at one boundary.
//
// Every surface spells them differently, and the spelling is part of the
// evidence: Codex says cache_write_input_tokens, Grok says cacheCreationTokens,
// Anthropic says cache_creation_input_tokens. A prober that normalised them
// would hide which surface was being read.
type FieldSpec struct {
	Write    []string
	Read     []string
	Oracle   []string
	Sequence []string // a per-request usage block, as opposed to a cumulative one
}

// AnthropicFields, CodexFields and GrokFields are the observed spellings.
var (
	AnthropicFields = FieldSpec{
		Write:    []string{"cache_creation_input_tokens"},
		Read:     []string{"cache_read_input_tokens"},
		Oracle:   []string{"cache_miss_reason"},
		Sequence: []string{"usage"},
	}
	CodexFields = FieldSpec{
		Write:    []string{"cache_write_input_tokens"},
		Read:     []string{"cached_input_tokens"},
		Oracle:   nil,
		Sequence: []string{"last_token_usage"},
	}
	GrokFields = FieldSpec{
		Write:    []string{"cacheCreationTokens"},
		Read:     []string{"cachedReadTokens"},
		Oracle:   nil,
		Sequence: []string{"usage"},
	}
)

// Probe walks a directory of JSON Lines records and reports what is there.
//
// It counts three things separately and never merges them: how many records
// carry a counter, how many of those are non-zero, and whether the field was
// seen at all. The audit this package encodes turned on exactly that split,
// so collapsing it here would make the package unable to state its own result.
//
// Malformed lines are skipped rather than failing the probe, and unreadable
// files are counted. A probe that refused a corpus because one line was bad
// would report nothing about the 99.9% that were fine.
func Probe(root string, spec FieldSpec) (Observables, error) {
	o := Observables{Boundary: root}
	oracle := map[string]bool{}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil //nolint:nilerr // an unreadable subtree is not a reason to abandon the rest
		}
		if d.IsDir() || !strings.HasSuffix(path, ".jsonl") {
			return nil
		}
		f, err := os.Open(path) //nolint:gosec // the caller names the root it wants probed
		if err != nil {
			return nil //nolint:nilerr // same reason
		}
		defer f.Close() //nolint:errcheck // read-only

		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 1024*1024), 64*1024*1024)
		for sc.Scan() {
			line := sc.Bytes()
			if len(line) == 0 {
				continue
			}
			var v any
			if json.Unmarshal(line, &v) != nil {
				continue
			}
			o.Records++
			sawWrite, sawRead := false, false
			walk(v, func(m map[string]any) {
				for _, k := range spec.Write {
					if n, ok := num(m[k]); ok {
						o.WriteObservations++
						o.WriteFieldPresent = true
						sawWrite = true
						if n != 0 {
							o.WriteNonZero++
						}
					}
				}
				for _, k := range spec.Read {
					if n, ok := num(m[k]); ok {
						o.ReadObservations++
						sawRead = true
						if n != 0 {
							o.ReadNonZero++
						}
					}
				}
				for _, k := range spec.Oracle {
					if raw, ok := m[k]; ok {
						oracle[oracleKey(raw)] = true
					}
				}
				for _, k := range spec.Sequence {
					if _, ok := m[k]; ok {
						o.PerRequestSequencing = true
					}
				}
			})
			if sawWrite {
				o.RecordsWithWrite++
			}
			if sawRead {
				o.RecordsWithRead++
			}
		}
		return nil
	})
	o.OracleClasses = len(oracle)
	return o, err
}

// oracleKey reduces a diagnostic to its class name.
//
// Anthropic's field is an object whose `type` is the class and whose siblings
// carry per-event counters. Keying on the whole object would count every event
// as its own class, which would turn a four-class partition into a 2,064-class
// one and make OracleClasses meaningless.
func oracleKey(raw any) string {
	if m, ok := raw.(map[string]any); ok {
		if t, ok := m["type"].(string); ok {
			return t
		}
	}
	if s, ok := raw.(string); ok {
		return s
	}
	return ""
}

func num(v any) (float64, bool) {
	f, ok := v.(float64)
	return f, ok
}

func walk(v any, visit func(map[string]any)) {
	switch t := v.(type) {
	case map[string]any:
		visit(t)
		for _, child := range t {
			walk(child, visit)
		}
	case []any:
		for _, child := range t {
			walk(child, visit)
		}
	}
}
