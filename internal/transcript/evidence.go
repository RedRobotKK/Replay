package transcript

import "encoding/json"

// ErrorFlag is the evidence-backed is_error state of one tool_result block,
// captured from raw content bytes before encoding/json collapses duplicate
// keys. It is a narrower concept than the frozen occurrence verifier's
// Verdict: it answers only "what did the raw bytes record for is_error on
// this block", and it carries an Absent state the verifier does not, because
// absence of a key is a fact the verifier is not asked for.
//
// It is NOT a second verifier. It performs no cryptographic binding, no
// strict RFC 8259 rejection, and no epistemic classification. The raw bytes it
// reads have already been accepted by encoding/json, so this is occurrence
// capture (find the is_error value(s) per block), not verification. The frozen
// internal/evidenceverify package remains the forensic oracle for a pinned
// occurrence.
type ErrorFlag uint8

const (
	// ErrorFlagNone means no raw evidence was available. Consumers must fall
	// back to the decoded Block.IsError and label the observation legacy.
	ErrorFlagNone ErrorFlag = iota
	// ErrorFlagAbsent means raw evidence was available and the block records
	// no is_error key: verified not-flagged, distinct from explicit false.
	ErrorFlagAbsent
	// ErrorFlagFalse means the block records exactly is_error:false.
	ErrorFlagFalse
	// ErrorFlagTrue means the block records exactly is_error:true.
	ErrorFlagTrue
	// ErrorFlagConflict means the block records is_error multiple times with
	// conflicting values (the duplicate-key trap), or a mix of boolean and
	// non-boolean values, so no single true/false is established.
	ErrorFlagConflict
	// ErrorFlagNonBoolean means the block records is_error present but with a
	// non-boolean value (null, number, string, array, object). It is neither
	// true nor false and must never read as clean (false) or as an error
	// (true).
	ErrorFlagNonBoolean
)

// attachEvidence walks raw content once and attaches the evidence-backed
// error flag to each decoded block in blks, by index. blocks is the decoded
// RawBlock slice that produced blks; they are parallel. The walk is O(content
// size), not O(content size × block count).
func attachEvidence(blks []Block, raw json.RawMessage, blocks []RawBlock) {
	if len(raw) == 0 {
		return
	}
	flags := errorFlags(raw, blocks)
	for i := range blks {
		if i < len(flags) && flags[i] != nil {
			blks[i].Evidence = flags[i]
		}
	}
}

// errorFlags walks a JSON content array once and returns, parallel to blocks,
// the evidence-backed error flag for each tool_result block. Non-tool_result
// blocks get nil.
func errorFlags(raw json.RawMessage, blocks []RawBlock) []*ErrorFlag {
	out := make([]*ErrorFlag, len(blocks))
	s := &flagScanner{b: raw}
	s.skipWS()
	if !s.expect('[') {
		// Not a block array (e.g. string content): no tool_result blocks.
		return out
	}
	s.skipWS()
	for i := 0; i < len(blocks); i++ {
		if s.peek() == ']' {
			break
		}
		if blocks[i].Type == KindToolResult {
			out[i] = s.scanToolResult()
		} else {
			s.skipValue()
		}
		s.skipWS()
		if s.expect(',') {
			s.skipWS()
		}
	}
	return out
}

// flagScanner is a minimal, forward-only byte scanner over already-accepted
// JSON. It exists only to recover is_error values that encoding/json collapsed.
type flagScanner struct {
	b   []byte
	pos int
}

func (s *flagScanner) peek() byte {
	if s.pos >= len(s.b) {
		return 0
	}
	return s.b[s.pos]
}

func (s *flagScanner) expect(c byte) bool {
	if s.peek() == c {
		s.pos++
		return true
	}
	return false
}

func (s *flagScanner) skipWS() {
	for s.pos < len(s.b) {
		switch s.b[s.pos] {
		case ' ', '\t', '\n', '\r':
			s.pos++
		default:
			return
		}
	}
}

// scanToolResult scans one tool_result object and returns its error flag.
// scanToolResult scans one tool_result object and returns its error flag.
//
// It tracks every is_error occurrence and its logical outcome (boolean true,
// boolean false, or present-non-boolean) so that a non-boolean value is never
// silently dropped to Absent, and a mix of boolean and non-boolean values is a
// Conflict, never a verified true/false. This is the RED-1 repair: the
// specialized scanner must agree with the rigorous occurrence verifier on
// which same-scope duplicate sets establish a unique proposition.
func (s *flagScanner) scanToolResult() *ErrorFlag {
	if !s.expect('{') {
		return nil
	}
	var sawBool bool // at least one boolean is_error value seen
	var boolVal bool // the boolean value seen
	var conflict bool
	var nonBool bool

	s.skipWS()
	if s.expect('}') {
		return flagValue(sawBool, boolVal, conflict, nonBool)
	}
	for {
		s.skipWS()
		key, ok := s.readString()
		if !ok {
			return nil
		}
		s.skipWS()
		if !s.expect(':') {
			return nil
		}
		s.skipWS()
		if key == "is_error" {
			switch s.peek() {
			case 't', 'f':
				b, ok := s.readBool()
				if !ok {
					return nil
				}
				if !sawBool {
					boolVal = b
					sawBool = true
				} else if b != boolVal {
					conflict = true
				}
			default:
				// Present but non-boolean (null, number, string, array,
				// object). It establishes neither true nor false.
				nonBool = true
				s.skipValue()
			}
		} else {
			s.skipValue()
		}
		s.skipWS()
		switch s.peek() {
		case ',':
			s.pos++
		case '}':
			s.pos++
			return flagValue(sawBool, boolVal, conflict, nonBool)
		default:
			return nil
		}
	}
}

func flagValue(sawBool, boolVal, conflict, nonBool bool) *ErrorFlag {
	if !sawBool && !nonBool {
		f := ErrorFlagAbsent
		return &f
	}
	if nonBool {
		// A non-boolean value alone, or mixed with any boolean, cannot
		// establish a unique true/false outcome.
		if sawBool || conflict {
			f := ErrorFlagConflict
			return &f
		}
		f := ErrorFlagNonBoolean
		return &f
	}
	if conflict {
		f := ErrorFlagConflict
		return &f
	}
	if boolVal {
		f := ErrorFlagTrue
		return &f
	}
	f := ErrorFlagFalse
	return &f
}

func (s *flagScanner) readBool() (bool, bool) {
	if s.pos+4 <= len(s.b) && string(s.b[s.pos:s.pos+4]) == "true" {
		s.pos += 4
		return true, true
	}
	if s.pos+5 <= len(s.b) && string(s.b[s.pos:s.pos+5]) == "false" {
		s.pos += 5
		return false, true
	}
	return false, false
}

// readString reads a JSON string at pos and returns its decoded form, handling
// escapes so "\u0069s_error" compares equal to "is_error".
func (s *flagScanner) readString() (string, bool) {
	if s.peek() != '"' {
		return "", false
	}
	start := s.pos
	s.pos++
	for s.pos < len(s.b) {
		c := s.b[s.pos]
		switch c {
		case '"':
			s.pos++
			var v string
			if err := json.Unmarshal(s.b[start:s.pos], &v); err != nil {
				return "", false
			}
			return v, true
		case '\\':
			s.pos += 2
		default:
			s.pos++
		}
	}
	return "", false
}

// skipValue structurally skips one JSON value at pos. It is only correct on
// already-valid JSON, which is guaranteed because encoding/json accepted it.
func (s *flagScanner) skipValue() {
	s.skipWS()
	if s.pos >= len(s.b) {
		return
	}
	switch s.b[s.pos] {
	case '"':
		_, _ = s.readString()
	case '{':
		s.skipObject()
	case '[':
		s.skipArray()
	default:
		// number, true, false, null: advance to the next structural delimiter.
		for s.pos < len(s.b) {
			switch s.b[s.pos] {
			case ',', '}', ']', ' ', '\t', '\n', '\r':
				return
			default:
				s.pos++
			}
		}
	}
}

func (s *flagScanner) skipObject() {
	s.pos++ // '{'
	s.skipWS()
	if s.expect('}') {
		return
	}
	for {
		s.skipWS()
		_, _ = s.readString()
		s.skipWS()
		s.pos++ // ':'
		s.skipValue()
		s.skipWS()
		if s.expect(',') {
			continue
		}
		s.expect('}')
		return
	}
}

func (s *flagScanner) skipArray() {
	s.pos++ // '['
	s.skipWS()
	if s.expect(']') {
		return
	}
	for {
		s.skipValue()
		s.skipWS()
		if s.expect(',') {
			continue
		}
		s.expect(']')
		return
	}
}
