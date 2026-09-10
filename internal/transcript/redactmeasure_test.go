package transcript

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// A redacted transcript has to MEASURE like the original.
//
// Redact's doc comment states the contract: it "keeps everything the analysis
// needs (structure, block kinds, byte lengths, usage, timestamps, ids, tool
// names) and nothing anyone could read", and it is "what a user can attach to a
// bug report, and what this repository uses as test fixtures".
//
// Both halves of that make the same demand. A bug report whose numbers differ
// from the reporter's is a bug report about the redactor. A fixture whose
// numbers differ from the corpus is a fixture that cannot reproduce anything.
//
// These tests measure the original and the redaction and require them to agree,
// on the two axes the analysis actually reads: block bytes, and the structural
// records the parser counts.

// sessionBytes totals every block byte the analysis would see.
func sessionBytes(s *Session) int {
	n := 0
	for _, l := range s.Lanes {
		for _, r := range l.Requests {
			for _, m := range r.Context {
				n += m.Bytes()
			}
			if r.Output != nil {
				n += r.Output.Bytes()
			}
		}
	}
	return n
}

// parseBoth parses a transcript and its redaction.
func parseBoth(t *testing.T, in string) (orig, red *Session) {
	t.Helper()
	orig, err := ParseClaudeCode(strings.NewReader(in))
	if err != nil {
		t.Fatalf("original will not parse: %v", err)
	}
	var buf bytes.Buffer
	if err := Redact(strings.NewReader(in), &buf); err != nil {
		t.Fatalf("redact: %v", err)
	}
	red, err = ParseClaudeCode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("redacted will not parse: %v\n%s", err, buf.String())
	}
	if strings.Contains(buf.String(), "SECRETWORD") {
		t.Fatalf("content survived redaction, so any agreement below is agreement "+
			"about an unredacted file:\n%s", buf.String())
	}
	return orig, red
}

// RM-1: a compaction survives redaction, with its sizes.
//
// Redact kept a top-level allowlist that did not include compactMetadata or
// isCompactSummary, so every compaction record was stripped. The whole
// compaction_test.go file — five tests over a defect the project fixed one
// version late — describes records that could not appear in any fixture this
// repository ships, because every fixture here is a redacted transcript, and
// the one checked in (testdata/session-redacted.jsonl) contains zero.
//
// A compaction's payload is four token counts and a duration. There is nothing
// in it to read.
//
// PASS: the redaction reports the same compaction, with preTokens 999029 and
// postTokens 23218 intact.
// FAIL: zero compactions, which is what shipped. Restore it by removing
// "isCompactSummary" and "compactMetadata" from keptTop.
func TestRM1_ACompactionSurvivesRedactionWithItsSizes(t *testing.T) {
	in := `{"uuid":"u1","type":"user","sessionId":"s","version":"2.0","cwd":"/home/real","timestamp":"2026-01-01T00:00:00Z","message":{"role":"user","content":"SECRETWORD in the prompt"}}
{"uuid":"u2","parentUuid":"u1","type":"system","subtype":"compact_boundary","timestamp":"2026-01-01T00:00:01Z","compactMetadata":{"trigger":"auto","preTokens":999029,"postTokens":23218,"cumulativeDroppedTokens":975811,"durationMs":42}}
{"uuid":"u3","parentUuid":"u2","type":"user","isCompactSummary":true,"timestamp":"2026-01-01T00:00:02Z","message":{"role":"user","content":"SECRETWORD summary of the above"}}
{"uuid":"u4","parentUuid":"u3","type":"assistant","requestId":"r1","apiBlockIndex":0,"timestamp":"2026-01-01T00:00:03Z","message":{"role":"assistant","model":"m","content":[{"type":"text","text":"SECRETWORD reply"}],"usage":{"input_tokens":5,"output_tokens":2}}}
`
	orig, red := parseBoth(t, in)
	if len(orig.Compactions) != 1 {
		t.Fatalf("the fixture is wrong: original has %d compactions, want 1", len(orig.Compactions))
	}
	if len(red.Compactions) != len(orig.Compactions) {
		t.Fatalf("redaction reports %d compaction(s) where the original has %d. A "+
			"transcript attached to a bug report about compaction would arrive "+
			"with the compaction removed.", len(red.Compactions), len(orig.Compactions))
	}
	o, r := orig.Compactions[0], red.Compactions[0]
	if o != r {
		t.Errorf("compaction changed under redaction:\n  original %+v\n  redacted %+v", o, r)
	}
	if !r.Sized() || r.Dropped() != 975811 {
		t.Errorf("sizes lost: Sized=%v Dropped=%d, want true/975811", r.Sized(), r.Dropped())
	}
}

// RM-2: the pairing still holds after redaction, so nothing is double-counted.
//
// CM-5 records that the parser once counted a compaction twice, once for the
// boundary and once for the summary, and that the note derived from the count
// was doubled with it. Keeping BOTH markers through redaction reintroduces the
// two records that defect needed. This asserts the pair still collapses to one.
//
// PASS: two boundary/summary pairs are two compactions in both files.
// FAIL: four, or two in the original and something else in the redaction.
func TestRM2_TwoCompactionsStayTwoAfterRedaction(t *testing.T) {
	line := func(n, parent, ts string) string {
		return `{"uuid":"b` + n + `","parentUuid":"` + parent + `","type":"system","subtype":"compact_boundary","timestamp":"` + ts + `","compactMetadata":{"trigger":"auto","preTokens":100,"postTokens":10,"cumulativeDroppedTokens":90,"durationMs":1}}` + "\n" +
			`{"uuid":"s` + n + `","parentUuid":"b` + n + `","type":"user","isCompactSummary":true,"timestamp":"` + ts + `","message":{"role":"user","content":"SECRETWORD"}}` + "\n"
	}
	in := `{"uuid":"u1","type":"user","sessionId":"s","version":"2.0","timestamp":"2026-01-01T00:00:00Z","message":{"role":"user","content":"SECRETWORD hi"}}` + "\n" +
		line("1", "u1", "2026-01-01T00:00:01Z") +
		line("2", "s1", "2026-01-01T00:00:02Z") +
		`{"uuid":"u9","parentUuid":"s2","type":"assistant","requestId":"r1","apiBlockIndex":0,"timestamp":"2026-01-01T00:00:09Z","message":{"role":"assistant","model":"m","content":[{"type":"text","text":"SECRETWORD"}],"usage":{"input_tokens":5,"output_tokens":2}}}` + "\n"
	orig, red := parseBoth(t, in)
	if len(orig.Compactions) != 2 {
		t.Fatalf("the fixture is wrong: original has %d compactions, want 2", len(orig.Compactions))
	}
	if len(red.Compactions) != 2 {
		t.Errorf("redaction reports %d compaction(s), want 2. Two boundary/summary "+
			"pairs are two compactions, not four.", len(red.Compactions))
	}
}

// RM-3: only sizes come through, not free text.
//
// compactMetadata is kept because its payload is counts. A client that starts
// putting a string in there must not have it copied out verbatim on the
// strength of the field name.
//
// PASS: an injected string field and an implausible trigger are both gone; the
// counts remain.
// FAIL: either string surviving, which is content leaving the machine under a
// promise that it would not.
func TestRM3_OnlyTheSizesComeThroughACompactionRecord(t *testing.T) {
	in := `{"uuid":"u1","type":"system","subtype":"compact_boundary","timestamp":"2026-01-01T00:00:01Z","compactMetadata":{"trigger":"/Users/real/secret/path.go","preTokens":100,"postTokens":10,"cumulativeDroppedTokens":90,"durationMs":1,"note":"SECRETWORD the user's prompt"}}
{"uuid":"u2","parentUuid":"u1","type":"assistant","requestId":"r1","apiBlockIndex":0,"timestamp":"2026-01-01T00:00:03Z","message":{"role":"assistant","model":"m","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":5,"output_tokens":2}}}
`
	var buf bytes.Buffer
	if err := Redact(strings.NewReader(in), &buf); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	for _, banned := range []string{"SECRETWORD", "secret/path.go", `"note"`} {
		if strings.Contains(got, banned) {
			t.Errorf("%q survived inside compactMetadata: %s", banned, got)
		}
	}
	if !strings.Contains(got, `"preTokens":100`) || !strings.Contains(got, `"postTokens":10`) {
		t.Errorf("the sizes were dropped along with the text: %s", got)
	}
	// A well-formed trigger is structure and must survive; the malformed one
	// above must not, and this is the contrast that keeps the guard honest.
	var ok bytes.Buffer
	if err := Redact(strings.NewReader(
		`{"uuid":"u1","type":"system","compactMetadata":{"trigger":"auto","preTokens":1,"postTokens":0,"cumulativeDroppedTokens":1,"durationMs":1}}`+"\n"), &ok); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ok.String(), `"trigger":"auto"`) {
		t.Errorf("a plain trigger is structure and must survive: %s", ok.String())
	}
}

// RM-4: escaping does not change what a block measures.
//
// Block.Bytes promises to be "unaffected by JSON escaping so redacted and
// original transcripts measure the same". The image and document branch of
// DecodeBlock used len(rb.Content) — the RAW JSON length — so the same five
// characters measured 7 as "abcde" and 9 as "a\"b\nc". The redactor writes
// filler of the same DECODED length, so an escape-heavy document came back
// smaller than it went in.
//
// PASS: the two files measure identically.
// FAIL: a mismatch. Restore it by writing `Bytes: len(rb.Content)` in
// DecodeBlock's KindImage/KindDocument branch.
func TestRM4_RedactionPreservesBytesThroughEscaping(t *testing.T) {
	in := `{"uuid":"u1","type":"user","sessionId":"s","version":"2.0","timestamp":"2026-01-01T00:00:00Z","message":{"role":"user","content":[{"type":"text","text":"SECRETWORD\"quoted\"\nnewline\ttab\\backslash"},{"type":"document","content":"SECRETWORD\"x\"\ny"}]}}
{"uuid":"u2","parentUuid":"u1","type":"assistant","requestId":"r1","apiBlockIndex":0,"timestamp":"2026-01-01T00:00:03Z","message":{"role":"assistant","model":"m","content":[{"type":"text","text":"SECRETWORD ok"}],"usage":{"input_tokens":5,"output_tokens":2}}}
`
	orig, red := parseBoth(t, in)
	ob, rb := sessionBytes(orig), sessionBytes(red)
	if ob == 0 {
		t.Fatalf("the fixture measures nothing, so agreement proves nothing")
	}
	if ob != rb {
		t.Errorf("original measures %d bytes, redaction measures %d. Redaction is "+
			"required to preserve byte lengths, and an escape-heavy block is "+
			"exactly where the raw-length shortcut shows.", ob, rb)
	}
}

// RM-5: escaping does not change what a block measures, at the block level.
//
// RM-4 sums a session, and a sum can agree by two errors cancelling. This
// pins the branch directly against a literal somebody has to change on purpose.
//
// PASS: 5, both ways.
// FAIL: 9 and 7 respectively, the raw JSON lengths.
func TestRM5_ADocumentMeasuresItsDecodedContent(t *testing.T) {
	escaped := DecodeBlock(RawBlock{Type: KindDocument, Content: []byte(`"a\"b\nc"`)}, RoleUser, nil, nil)
	plain := DecodeBlock(RawBlock{Type: KindDocument, Content: []byte(`"abcde"`)}, RoleUser, nil, nil)
	if escaped.Bytes != 5 {
		t.Errorf(`"a\"b\nc" measured %d, want 5: five characters, whatever they cost to escape`, escaped.Bytes)
	}
	if plain.Bytes != 5 {
		t.Errorf(`"abcde" measured %d, want 5`, plain.Bytes)
	}
}

// RM-6: a compactMetadata that is not an object is REMOVED, not kept as null.
//
// Found by the guard-reachability reviewer on 2026-09-10, which reported
// redact.go:99 — `if !ok { return nil }` in (*redactor).compaction — as
// SURVIVED: neutralising it left every test green.
//
// The reason it matters is a Go detail that reads as a no-op and is not one.
// compaction returns `any`. With the guard, a non-object returns an untyped
// nil, the caller's `kept != nil` is false, and the key is deleted. Without it,
// `m` is a nil map[string]any, and returning that boxes a TYPED nil into the
// interface — so `kept != nil` is TRUE, and the caller writes the key back.
// A field whose shape the redactor could not verify survives into the redacted
// file as `compactMetadata: null`.
//
// That inverts the rule the redactor exists to enforce. Its allowlist keeps
// what it can name and drops everything else; a branch that silently converts
// "cannot verify this" into "emit a placeholder for it" means the allowlist did
// not hold, and the one place that would show up is a shape nobody wrote a
// fixture for.
//
// Four shapes stand in for "not an object", because the assertion fails
// identically for all of them and a test that only tried one would not notice a
// future guard that special-cased strings.
//
// PASS: none of the four leaves a compactMetadata key behind.
// FAIL: the key survives as null, which is what removing the guard produces.
func TestRM6_ANonObjectCompactMetadataIsRemoved(t *testing.T) {
	for _, shape := range []struct{ name, json string }{
		{"a string", `"SECRETWORD"`},
		{"a number", `42`},
		{"an array", `["SECRETWORD"]`},
		{"null", `null`},
	} {
		t.Run(shape.name, func(t *testing.T) {
			in := `{"uuid":"u1","type":"system","subtype":"compact_boundary","sessionId":"s","cwd":"/home/real","timestamp":"2026-01-01T00:00:01Z","compactMetadata":` + shape.json + `}` + "\n"

			var buf bytes.Buffer
			if err := Redact(strings.NewReader(in), &buf); err != nil {
				t.Fatalf("redact: %v", err)
			}
			out := buf.String()
			if strings.Contains(out, "SECRETWORD") {
				t.Fatalf("content survived redaction:\n%s", out)
			}

			var obj map[string]any
			if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &obj); err != nil {
				t.Fatalf("the redacted line will not parse: %v\n%s", err, out)
			}
			if _, present := obj["compactMetadata"]; present {
				t.Errorf("compactMetadata (%s) survived redaction as %q. The redactor could "+
					"not verify its shape, so the key must be deleted rather than written "+
					"back — a typed nil map returned through an `any` is not nil at the "+
					"call site, which is exactly how a placeholder gets emitted for a field "+
					"nothing validated.", shape.name, out)
			}
		})
	}
}
