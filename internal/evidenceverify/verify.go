package evidenceverify

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Verdict is one of the four first-class outcomes. The ordering of the gates
// in Verify guarantees that REFUTED can only be produced after the evidence
// basis has been established.
type Verdict uint8

const (
	// Unverified means the evidence basis could not be established. It is a
	// statement about the inspection, not about the claim: a digest mismatch,
	// an unresolvable locator, malformed JSON, or a resource limit.
	Unverified Verdict = iota
	// Refuted means the evidence basis was established and the pinned bytes
	// unambiguously contradict the predicate (for example is_error: false
	// against a claim of true).
	Refuted
	// Ambiguous means the evidence basis was established but the same decoded
	// key occurs multiple times in one object scope with conflicting values,
	// so no single recorded literal can be read out (the duplicate-key trap).
	Ambiguous
	// Confirmed means the evidence basis was established and the pinned bytes
	// record exactly the claimed literal.
	Confirmed
)

func (v Verdict) String() string {
	switch v {
	case Unverified:
		return "UNVERIFIED"
	case Refuted:
		return "REFUTED"
	case Ambiguous:
		return "AMBIGUOUS"
	case Confirmed:
		return "CONFIRMED"
	default:
		return "UNKNOWN"
	}
}

// Reason is the stable machine-readable reason code. It is the semantic
// contract; human-readable prose lives in Result.Detail and must never be
// parsed as semantics.
type Reason string

// Reason values enumerate the stable machine-readable reason codes emitted by
// the occurrence verifier.
const (
	// Unverified reasons. None of these may ever coexist with Refuted.
	ReasonTranscriptHashMismatch Reason = "TRANSCRIPT_HASH_MISMATCH"
	ReasonLocatorMalformed       Reason = "LOCATOR_MALFORMED"
	ReasonLocatorOutOfRange      Reason = "LOCATOR_OUT_OF_RANGE"
	ReasonFragmentHashMismatch   Reason = "FRAGMENT_HASH_MISMATCH"
	ReasonFragmentSpanMismatch   Reason = "FRAGMENT_SPAN_MISMATCH"
	ReasonLocatorNotValue        Reason = "LOCATOR_NOT_VALUE"
	ReasonLocatorKeyMismatch     Reason = "LOCATOR_KEY_MISMATCH"
	ReasonMalformedJSON          Reason = "MALFORMED_JSON"
	ReasonInvalidUTF8            Reason = "INVALID_UTF8"
	ReasonResourceLimit          Reason = "RESOURCE_LIMIT"
	ReasonUnsupportedPredicate   Reason = "UNSUPPORTED_PREDICATE"

	// Ambiguous reasons.
	ReasonDuplicateConflict Reason = "DUPLICATE_CONFLICT"

	// Determined reasons (reachable only after the basis is established).
	ReasonLiteralMismatch Reason = "LITERAL_MISMATCH"
	ReasonLiteralMatch    Reason = "LITERAL_MATCH"
)

// Span is a byte range [Start, End) into the transcript. The zero value means
// "not established".
type Span struct {
	Start int64
	End   int64
}

// Result is the verdict plus everything established safely along the way.
//
// ObjectOffset, KeySpan, ValueSpan and MatchingOccurrences are only
// meaningful once the occurrence has been established (LOCATOR_KEY_MISMATCH
// carries the object offset, and the determined/ambiguous results carry all
// of them). Callers must gate their use of those fields on Verdict.
type Result struct {
	Verdict Verdict
	Reason  Reason
	// Detail is human-readable diagnostic prose, not semantics. Tests and
	// consumers key off Verdict and Reason.
	Detail string
	// ObjectOffset is the byte offset of the enclosing object's '{'. It is -1
	// until the enclosing object is established.
	ObjectOffset int64
	// KeySpan is the raw byte span of the pinned key, including quotes.
	KeySpan Span
	// ValueSpan is the raw byte span of the pinned value.
	ValueSpan Span
	// MatchingOccurrences counts how many members of the pinned object scope
	// carried the pinned key. It is >= 1 once the occurrence is established.
	MatchingOccurrences int
}

// Locator binds a physical occurrence within an immutable transcript.
//
// TranscriptSHA256 binds the whole document globally; FragmentSHA256 binds
// the pinned value's exact bytes locally. Offsets are raw byte offsets from
// byte zero of the transcript — never rune offsets or line/column positions.
type Locator struct {
	TranscriptSHA256 [32]byte
	// ValueOffset is the byte offset of the first byte of the pinned value
	// token (for example the 't' of true).
	ValueOffset int64
	// FragmentLength is the byte length of the pinned value token.
	FragmentLength int64
	// FragmentSHA256 is the digest of transcript[ValueOffset:ValueOffset+FragmentLength].
	FragmentSHA256 [32]byte
}

// Kind is the literal class of a predicate value.
type Kind uint8

// Kind values enumerate the literal classes of a predicate value.
const (
	KindInvalid Kind = iota
	KindBool
	KindString
	KindNumber
	KindNull
)

// Literal is a scalar literal predicate value. Numbers compare lexically, so
// "1", "1.0" and "1e0" are three different recorded literals: this verifier
// proves what was recorded, not a mathematical equality.
type Literal struct {
	Kind Kind
	Bool bool
	Str  string // decoded string value
	Num  string // lexical number text
}

// BoolLiteral constructs a boolean literal predicate.
func BoolLiteral(b bool) Literal { return Literal{Kind: KindBool, Bool: b} }

// StringLiteral constructs a string literal predicate. The value is compared
// decoded, so "x" and "\u0078" are equivalent.
func StringLiteral(s string) Literal { return Literal{Kind: KindString, Str: s} }

// NumberLiteral constructs a number literal predicate from its lexical text.
func NumberLiteral(lex string) Literal { return Literal{Kind: KindNumber, Num: lex} }

// NullLiteral constructs a null literal predicate.
func NullLiteral() Literal { return Literal{Kind: KindNull} }

func (l Literal) valid() bool {
	switch l.Kind {
	case KindBool, KindString, KindNumber, KindNull:
		return true
	default:
		return false
	}
}

// Predicate names the key and the expected literal of the claim under test.
type Predicate struct {
	Key    string
	Expect Literal
}

// Limits bound the resources a verification may consume. A limit being
// exceeded yields Unverified with ReasonResourceLimit, never Refuted.
//
// Zero or negative fields mean "use DefaultLimits".
type Limits struct {
	MaxTranscriptBytes int
	MaxDepth           int
	MaxMembers         int
	MaxMatches         int
}

// DefaultLimits returns the production limits.
func DefaultLimits() Limits {
	return Limits{
		MaxTranscriptBytes: 64 << 20, // 64 MiB of transcript bytes
		MaxDepth:           256,      // JSON nesting depth
		MaxMembers:         16 << 20, // total object members / array elements scanned
		MaxMatches:         1 << 16,  // matching duplicate keys in one object scope
	}
}

func (l Limits) normalized() Limits {
	d := DefaultLimits()
	if l.MaxTranscriptBytes <= 0 {
		l.MaxTranscriptBytes = d.MaxTranscriptBytes
	}
	if l.MaxDepth <= 0 {
		l.MaxDepth = d.MaxDepth
	}
	if l.MaxMembers <= 0 {
		l.MaxMembers = d.MaxMembers
	}
	if l.MaxMatches <= 0 {
		l.MaxMatches = d.MaxMatches
	}
	return l
}

// Verify answers the claim for the given transcript and locator using the
// production limits.
func Verify(transcript []byte, loc Locator, pred Predicate) Result {
	return VerifyWithLimits(transcript, loc, pred, DefaultLimits())
}

// VerifyWithLimits is Verify with explicit limits. It is the only entry point
// that does the work; Verify delegates to it.
func VerifyWithLimits(transcript []byte, loc Locator, pred Predicate, lim Limits) Result {
	lim = lim.normalized()

	// The predicate itself must be well-formed before any evidence is read.
	if pred.Key == "" || !pred.Expect.valid() {
		return Result{Verdict: Unverified, Reason: ReasonUnsupportedPredicate,
			Detail:       "predicate must name a non-empty key and a scalar literal",
			ObjectOffset: -1}
	}

	// Gate 1: transcript size. Cheapest DoS boundary, checked before hashing.
	if len(transcript) > lim.MaxTranscriptBytes {
		return Result{Verdict: Unverified, Reason: ReasonResourceLimit,
			Detail: "transcript exceeds the configured byte limit", ObjectOffset: -1}
	}

	// Gate 2: transcript identity.
	if sha256.Sum256(transcript) != loc.TranscriptSHA256 {
		return Result{Verdict: Unverified, Reason: ReasonTranscriptHashMismatch,
			Detail: "transcript digest does not match the locator", ObjectOffset: -1}
	}

	// Gate 3: locator shape.
	if loc.ValueOffset < 0 || loc.FragmentLength <= 0 {
		return Result{Verdict: Unverified, Reason: ReasonLocatorMalformed,
			Detail:       "locator offset must be non-negative and fragment length positive",
			ObjectOffset: -1}
	}
	// REPLAY-SECURITY: bound without adding. A hostile ValueOffset +
	// FragmentLength would otherwise overflow int64, wrap negative, and slip
	// past this check into a slice panic. Once ValueOffset is known to be at
	// most len(transcript), the subtraction cannot underflow.
	if loc.ValueOffset > int64(len(transcript)) ||
		loc.FragmentLength > int64(len(transcript))-loc.ValueOffset {
		return Result{Verdict: Unverified, Reason: ReasonLocatorOutOfRange,
			Detail: "locator offset and fragment exceed the transcript", ObjectOffset: -1}
	}

	// Gate 4: fragment identity (the pinned value's local bytes).
	fragment := transcript[loc.ValueOffset : loc.ValueOffset+loc.FragmentLength]
	if sha256.Sum256(fragment) != loc.FragmentSHA256 {
		return Result{Verdict: Unverified, Reason: ReasonFragmentHashMismatch,
			Detail: "fragment digest does not match the locator", ObjectOffset: -1}
	}

	// Gate 5: structural validity and occurrence location. This pass validates
	// the entire document and pins the occurrence at the exact byte boundary.
	occ, err := scanWhole(transcript, loc.ValueOffset, lim)
	if err != nil {
		return scanFailure(err)
	}

	// Gate 6: occurrence resolution.
	if occ == nil {
		return Result{Verdict: Unverified, Reason: ReasonLocatorNotValue,
			Detail: "no value token starts at the pinned offset", ObjectOffset: -1}
	}
	if occ.key != pred.Key {
		// A wrong key at the requested offset means the claimed occurrence was
		// never established. It is NOT a contradiction of the claim.
		return Result{Verdict: Unverified, Reason: ReasonLocatorKeyMismatch,
			Detail:       "the pinned value belongs to a different key",
			ObjectOffset: int64(occ.frame.start)}
	}

	// The fragment must span exactly the pinned value token. A locator whose
	// fragment is a prefix or suffix of the value does not bind the value it
	// names, so it fails closed rather than confirming on weaker evidence.
	if int64(occ.valueEnd-occ.valueStart) != loc.FragmentLength {
		return Result{Verdict: Unverified, Reason: ReasonFragmentSpanMismatch,
			Detail:       "fragment length does not equal the pinned value token's span",
			ObjectOffset: int64(occ.frame.start)}
	}

	// Gate 7: same-object uniqueness. Rescan only the pinned object's
	// immediate members; nested objects must not contaminate this scope.
	vals, count, err := scanScope(transcript, occ.frame.start, occ.frame.end, pred.Key, lim)
	if err != nil {
		r := scanFailure(err)
		r.ObjectOffset = int64(occ.frame.start)
		return r
	}

	established := Result{
		ObjectOffset:        int64(occ.frame.start),
		KeySpan:             Span{Start: int64(occ.keyStart), End: int64(occ.keyEnd)},
		ValueSpan:           Span{Start: int64(occ.valueStart), End: int64(occ.valueEnd)},
		MatchingOccurrences: count,
	}

	if hasConflict(vals) {
		established.Verdict = Ambiguous
		established.Reason = ReasonDuplicateConflict
		established.Detail = "the key occurs multiple times in one object scope with conflicting values"
		return established
	}

	// Gate 8: the literal predicate. Only now may the outcome be determined.
	if literalMatches(occ.value, pred.Expect) {
		established.Verdict = Confirmed
		established.Reason = ReasonLiteralMatch
		established.Detail = "the pinned bytes record the claimed literal"
		return established
	}
	established.Verdict = Refuted
	established.Reason = ReasonLiteralMismatch
	established.Detail = "the pinned bytes record a literal that contradicts the claim"
	return established
}

// scanFailure converts a scanner error into an Unverified Result. By
// construction it can never carry Refuted or Ambiguous.
func scanFailure(err error) Result {
	var se *scanError
	if errors.As(err, &se) {
		return Result{Verdict: Unverified, Reason: se.reason, Detail: se.detail, ObjectOffset: -1}
	}
	return Result{Verdict: Unverified, Reason: ReasonMalformedJSON, Detail: err.Error(), ObjectOffset: -1}
}

// valueKind is the structural class of a scanned value.
type valueKind uint8

const (
	vBool valueKind = iota
	vString
	vNumber
	vNull
	vObject
	vArray
)

// value is a scanned value with enough retained to decide equality. For
// object/array values the raw byte span is retained (not copied) so equality
// is byte equality. For numbers the lexical text is retained.
type value struct {
	kind    valueKind
	boolVal bool
	str     string
	numLex  string
	raw     []byte // object/array raw byte span
}

func (v value) equal(o value) bool {
	if v.kind != o.kind {
		return false
	}
	switch v.kind {
	case vBool:
		return v.boolVal == o.boolVal
	case vString:
		return v.str == o.str
	case vNumber:
		// REPLAY-VERIFY: lexical equality. "1", "1.0" and "1e0" are distinct
		// recorded literals; this verifier proves what was recorded, not a
		// mathematical equivalence.
		return v.numLex == o.numLex
	case vNull:
		return true
	case vObject, vArray:
		return bytes.Equal(v.raw, o.raw)
	default:
		return false
	}
}

// hasConflict reports whether the matching members carry more than one
// distinct value. Identical duplicates are not a conflict: {"x":true,"x":true}
// is determined, {"x":true,"x":false} is ambiguous.
func hasConflict(vals []value) bool {
	if len(vals) < 2 {
		return false
	}
	for _, v := range vals[1:] {
		if !v.equal(vals[0]) {
			return true
		}
	}
	return false
}

// literalMatches compares a scanned value against a scalar predicate literal.
func literalMatches(v value, want Literal) bool {
	switch want.Kind {
	case KindBool:
		return v.kind == vBool && v.boolVal == want.Bool
	case KindString:
		return v.kind == vString && v.str == want.Str
	case KindNumber:
		return v.kind == vNumber && v.numLex == want.Num
	case KindNull:
		return v.kind == vNull
	default:
		return false
	}
}

// scanError carries a stable reason and a human detail out of the scanner.
type scanError struct {
	reason Reason
	detail string
}

func (e *scanError) Error() string { return string(e.reason) + ": " + e.detail }

// occurrence is the pinned member, captured before any decoding could destroy
// its physical identity.
type occurrence struct {
	frame      *objectFrame
	key        string
	keyStart   int
	keyEnd     int
	valueStart int
	valueEnd   int
	value      value
}

// objectFrame records the byte span of one object, so the uniqueness pass can
// rescan exactly its immediate members.
type objectFrame struct {
	start int
	end   int
}

// scanner is a strict, forward-only JSON scanner. It uses recursion for
// structure and bounds that recursion with the configured depth limit.
type scanner struct {
	b       []byte
	pos     int
	lim     Limits
	target  int64 // byte offset of the occurrence to pin, -1 when not pinning
	found   *occurrence
	members int
	depth   int
}

func (s *scanner) errf(reason Reason, format string, args ...any) error {
	return &scanError{reason: reason, detail: fmt.Sprintf(format, args...)}
}

func (s *scanner) skipSpace() {
	for s.pos < len(s.b) {
		switch s.b[s.pos] {
		case ' ', '\t', '\n', '\r':
			s.pos++
		default:
			return
		}
	}
}

// scanWhole validates one complete root value and pins the occurrence (if any)
// whose value starts exactly at target.
func scanWhole(transcript []byte, target int64, lim Limits) (*occurrence, error) {
	s := &scanner{b: transcript, lim: lim, target: target}
	s.skipSpace()
	if s.pos >= len(s.b) {
		return nil, s.errf(ReasonMalformedJSON, "empty input")
	}
	if _, _, err := s.scanValue(); err != nil {
		return nil, err
	}
	s.skipSpace()
	if s.pos != len(s.b) {
		return nil, s.errf(ReasonMalformedJSON, "trailing bytes after the root value")
	}
	return s.found, nil
}

// scanScope rescans exactly one object's immediate members and collects the
// values of the members whose decoded key equals wantKey. The sub-slice runs
// from the object's '{' to just past its '}'.
func scanScope(transcript []byte, objStart, objEnd int, wantKey string, lim Limits) ([]value, int, error) {
	s := &scanner{b: transcript[objStart:objEnd], lim: lim, target: -1}
	var vals []value
	count := 0

	if s.pos >= len(s.b) || s.b[s.pos] != '{' {
		return nil, 0, s.errf(ReasonMalformedJSON, "object scope does not begin with '{'")
	}
	s.pos++
	s.skipSpace()
	if s.pos < len(s.b) && s.b[s.pos] == '}' {
		return vals, count, nil
	}
	for {
		s.skipSpace()
		if s.pos >= len(s.b) || s.b[s.pos] != '"' {
			return nil, 0, s.errf(ReasonMalformedJSON, "object member key expected")
		}
		key, keyEnd, err := s.scanString()
		if err != nil {
			return nil, 0, err
		}
		s.pos = keyEnd
		s.skipSpace()
		if s.pos >= len(s.b) || s.b[s.pos] != ':' {
			return nil, 0, s.errf(ReasonMalformedJSON, "expected ':' after key")
		}
		s.pos++
		s.skipSpace()
		val, _, err := s.scanValue()
		if err != nil {
			return nil, 0, err
		}
		s.members++
		if s.members > lim.MaxMembers {
			return nil, 0, s.errf(ReasonResourceLimit, "object members exceed limit")
		}
		if key == wantKey {
			count++
			if count > lim.MaxMatches {
				return nil, 0, s.errf(ReasonResourceLimit, "matching duplicate keys exceed limit")
			}
			vals = append(vals, val)
		}
		s.skipSpace()
		if s.pos < len(s.b) && s.b[s.pos] == ',' {
			s.pos++
			continue
		}
		if s.pos < len(s.b) && s.b[s.pos] == '}' {
			return vals, count, nil
		}
		return nil, 0, s.errf(ReasonMalformedJSON, "expected ',' or '}'")
	}
}

func (s *scanner) scanValue() (value, int, error) {
	s.skipSpace()
	if s.pos >= len(s.b) {
		return value{}, 0, s.errf(ReasonMalformedJSON, "unexpected end of input")
	}
	switch c := s.b[s.pos]; {
	case c == '{':
		return s.scanObjectValue()
	case c == '[':
		return s.scanArrayValue()
	case c == '"':
		str, end, err := s.scanString()
		if err != nil {
			return value{}, 0, err
		}
		return value{kind: vString, str: str}, end, nil
	case c == 't':
		if err := s.expectKeyword("true"); err != nil {
			return value{}, 0, err
		}
		return value{kind: vBool, boolVal: true}, s.pos, nil
	case c == 'f':
		if err := s.expectKeyword("false"); err != nil {
			return value{}, 0, err
		}
		return value{kind: vBool, boolVal: false}, s.pos, nil
	case c == 'n':
		if err := s.expectKeyword("null"); err != nil {
			return value{}, 0, err
		}
		return value{kind: vNull}, s.pos, nil
	case c == '-' || (c >= '0' && c <= '9'):
		start := s.pos
		if err := s.scanNumber(); err != nil {
			return value{}, 0, err
		}
		return value{kind: vNumber, numLex: string(s.b[start:s.pos])}, s.pos, nil
	default:
		return value{}, 0, s.errf(ReasonMalformedJSON, "unexpected byte 0x%02x", c)
	}
}

func (s *scanner) enter() error {
	// REPLAY-SECURITY: recursion depth equals JSON nesting depth. The depth
	// limit below bounds the recursion, so a hostile transcript cannot grow
	// the goroutine stack past MaxDepth frames. Without this check the depth
	// of a transcript's tool input would be a property of the stack.
	s.depth++
	if s.depth > s.lim.MaxDepth {
		s.depth--
		return s.errf(ReasonResourceLimit, "nesting depth exceeds limit")
	}
	return nil
}

func (s *scanner) scanObjectValue() (value, int, error) {
	start := s.pos
	if err := s.enter(); err != nil {
		return value{}, 0, err
	}
	frame := &objectFrame{start: start}
	end, err := s.scanObject(frame)
	s.depth--
	if err != nil {
		return value{}, 0, err
	}
	return value{kind: vObject, raw: s.b[start:end]}, end, nil
}

func (s *scanner) scanArrayValue() (value, int, error) {
	start := s.pos
	if err := s.enter(); err != nil {
		return value{}, 0, err
	}
	end, err := s.scanArray()
	s.depth--
	if err != nil {
		return value{}, 0, err
	}
	return value{kind: vArray, raw: s.b[start:end]}, end, nil
}

func (s *scanner) scanObject(frame *objectFrame) (int, error) {
	// s.pos is at '{'.
	s.pos++
	s.skipSpace()
	if s.pos < len(s.b) && s.b[s.pos] == '}' {
		s.pos++
		frame.end = s.pos
		return s.pos, nil
	}
	for {
		s.skipSpace()
		keyStart := s.pos
		if s.pos >= len(s.b) || s.b[s.pos] != '"' {
			return 0, s.errf(ReasonMalformedJSON, "object key must be a string")
		}
		key, keyEnd, err := s.scanString()
		if err != nil {
			return 0, err
		}
		s.pos = keyEnd
		s.skipSpace()
		if s.pos >= len(s.b) || s.b[s.pos] != ':' {
			return 0, s.errf(ReasonMalformedJSON, "expected ':' after key")
		}
		s.pos++
		s.skipSpace()
		valueStart := s.pos
		val, valueEnd, err := s.scanValue()
		if err != nil {
			return 0, err
		}
		s.members++
		if s.members > s.lim.MaxMembers {
			return 0, s.errf(ReasonResourceLimit, "object members exceed limit")
		}
		if int64(valueStart) == s.target && s.found == nil {
			s.found = &occurrence{
				frame:      frame,
				key:        key,
				keyStart:   keyStart,
				keyEnd:     keyEnd,
				valueStart: valueStart,
				valueEnd:   valueEnd,
				value:      val,
			}
		}
		s.skipSpace()
		if s.pos < len(s.b) && s.b[s.pos] == ',' {
			s.pos++
			continue
		}
		if s.pos < len(s.b) && s.b[s.pos] == '}' {
			s.pos++
			frame.end = s.pos
			return s.pos, nil
		}
		return 0, s.errf(ReasonMalformedJSON, "expected ',' or '}'")
	}
}

func (s *scanner) scanArray() (int, error) {
	// s.pos is at '['.
	s.pos++
	s.skipSpace()
	if s.pos < len(s.b) && s.b[s.pos] == ']' {
		s.pos++
		return s.pos, nil
	}
	for {
		s.skipSpace()
		if s.pos >= len(s.b) {
			return 0, s.errf(ReasonMalformedJSON, "unterminated array")
		}
		if _, _, err := s.scanValue(); err != nil {
			return 0, err
		}
		s.members++
		if s.members > s.lim.MaxMembers {
			return 0, s.errf(ReasonResourceLimit, "array elements exceed limit")
		}
		s.skipSpace()
		if s.pos < len(s.b) && s.b[s.pos] == ',' {
			s.pos++
			continue
		}
		if s.pos < len(s.b) && s.b[s.pos] == ']' {
			s.pos++
			return s.pos, nil
		}
		return 0, s.errf(ReasonMalformedJSON, "expected ',' or ']'")
	}
}

func (s *scanner) scanString() (string, int, error) {
	// s.pos is at the opening '"'. Returns the decoded string and the byte
	// offset just past the closing '"'.
	start := s.pos
	s.pos++
	var sb strings.Builder
	for s.pos < len(s.b) {
		c := s.b[s.pos]
		switch {
		case c == '"':
			s.pos++
			return sb.String(), s.pos, nil
		case c == '\\':
			s.pos++
			if s.pos >= len(s.b) {
				return "", start, s.errf(ReasonMalformedJSON, "truncated escape")
			}
			switch s.b[s.pos] {
			case '"':
				sb.WriteByte('"')
				s.pos++
			case '\\':
				sb.WriteByte('\\')
				s.pos++
			case '/':
				sb.WriteByte('/')
				s.pos++
			case 'b':
				sb.WriteByte('\b')
				s.pos++
			case 'f':
				sb.WriteByte('\f')
				s.pos++
			case 'n':
				sb.WriteByte('\n')
				s.pos++
			case 'r':
				sb.WriteByte('\r')
				s.pos++
			case 't':
				sb.WriteByte('\t')
				s.pos++
			case 'u':
				r1, next, err := s.readHex4(s.pos + 1)
				if err != nil {
					return "", start, err
				}
				s.pos = next
				if utf16.IsSurrogate(r1) {
					// A surrogate must be a valid high-low pair; anything else
					// (lone high, lone low, or a non-pair) is malformed.
					if s.pos+1 < len(s.b) && s.b[s.pos] == '\\' && s.b[s.pos+1] == 'u' {
						r2, next2, err := s.readHex4(s.pos + 2)
						if err != nil {
							return "", start, err
						}
						dec := utf16.DecodeRune(r1, r2)
						if dec == utf8.RuneError {
							return "", start, s.errf(ReasonMalformedJSON, "invalid surrogate pair")
						}
						sb.WriteRune(dec)
						s.pos = next2
					} else {
						return "", start, s.errf(ReasonMalformedJSON, "lone surrogate in string")
					}
				} else {
					sb.WriteRune(r1)
				}
			default:
				return "", start, s.errf(ReasonMalformedJSON, "invalid escape")
			}
		case c < 0x20:
			// REPLAY-SECURITY: a raw control byte in a string is a syntax
			// error per RFC 8259, not a character. Rejecting it here keeps a
			// byte that a terminal could interpret out of a decoded label.
			return "", start, s.errf(ReasonMalformedJSON, "unescaped control byte in string")
		default:
			r, size := utf8.DecodeRune(s.b[s.pos:])
			if r == utf8.RuneError && size == 1 {
				// REPLAY-SECURITY: invalid UTF-8 is rejected, not repaired.
				// Substituting U+FFFD here would silently rewrite evidence.
				return "", start, s.errf(ReasonInvalidUTF8, "invalid UTF-8 in string")
			}
			sb.WriteRune(r)
			s.pos += size
		}
	}
	return "", start, s.errf(ReasonMalformedJSON, "unterminated string")
}

func (s *scanner) readHex4(i int) (rune, int, error) {
	if i+4 > len(s.b) {
		return 0, i, s.errf(ReasonMalformedJSON, "truncated \\u escape")
	}
	var r rune
	for k := 0; k < 4; k++ {
		c := s.b[i+k]
		r <<= 4
		switch {
		case '0' <= c && c <= '9':
			r += rune(c - '0')
		case 'a' <= c && c <= 'f':
			r += rune(c - 'a' + 10)
		case 'A' <= c && c <= 'F':
			r += rune(c - 'A' + 10)
		default:
			return 0, i, s.errf(ReasonMalformedJSON, "invalid \\u hex digit")
		}
	}
	return r, i + 4, nil
}

func (s *scanner) expectKeyword(word string) error {
	if s.pos+len(word) > len(s.b) || string(s.b[s.pos:s.pos+len(word)]) != word {
		return s.errf(ReasonMalformedJSON, "invalid literal")
	}
	s.pos += len(word)
	return nil
}

func (s *scanner) scanNumber() error {
	if s.pos < len(s.b) && s.b[s.pos] == '-' {
		s.pos++
		if s.pos >= len(s.b) {
			return s.errf(ReasonMalformedJSON, "number: bare '-'")
		}
	}
	if s.pos >= len(s.b) {
		return s.errf(ReasonMalformedJSON, "number: truncated")
	}
	switch c := s.b[s.pos]; {
	case c == '0':
		s.pos++
		if s.pos < len(s.b) && s.b[s.pos] >= '0' && s.b[s.pos] <= '9' {
			return s.errf(ReasonMalformedJSON, "number: leading zero")
		}
	case c >= '1' && c <= '9':
		for s.pos < len(s.b) && s.b[s.pos] >= '0' && s.b[s.pos] <= '9' {
			s.pos++
		}
	default:
		return s.errf(ReasonMalformedJSON, "number: no digits")
	}
	if s.pos < len(s.b) && s.b[s.pos] == '.' {
		s.pos++
		if s.pos >= len(s.b) || s.b[s.pos] < '0' || s.b[s.pos] > '9' {
			return s.errf(ReasonMalformedJSON, "number: fraction needs a digit")
		}
		for s.pos < len(s.b) && s.b[s.pos] >= '0' && s.b[s.pos] <= '9' {
			s.pos++
		}
	}
	if s.pos < len(s.b) && (s.b[s.pos] == 'e' || s.b[s.pos] == 'E') {
		s.pos++
		if s.pos < len(s.b) && (s.b[s.pos] == '+' || s.b[s.pos] == '-') {
			s.pos++
		}
		if s.pos >= len(s.b) || s.b[s.pos] < '0' || s.b[s.pos] > '9' {
			return s.errf(ReasonMalformedJSON, "number: exponent needs a digit")
		}
		for s.pos < len(s.b) && s.b[s.pos] >= '0' && s.b[s.pos] <= '9' {
			s.pos++
		}
	}
	return nil
}
