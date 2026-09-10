package usage

import (
	"strings"
	"testing"
	"time"
)

// The usage-only input shape, and what it refuses.
//
// A usage export is not a transcript. A transcript is append-only, self-
// contained and produced by the client that made the requests; an export is a
// query result produced by somebody else, and every property the transcript
// path gets for free — that the records are in order, that none is missing,
// that "input tokens" means what this build thinks it means — has to be either
// declared or refused. These tests are the refusals.
//
// ADR-0018: absence, zero and unknown are three values. An export that cannot
// support a figure must make the tool say so, never hand it a zero.

const t0 = "2026-09-01T10:00:00Z"

func exportAt(offset time.Duration) time.Time {
	base, _ := time.Parse(time.RFC3339, t0)
	return base.Add(offset)
}

// wellFormed is one session of four requests, exclusive counts, in order.
func wellFormed() string {
	return `{
	  "schema": "replay.usage.v1",
	  "complete": true,
	  "records": [
	    {"session":"s1","at":"2026-09-01T10:00:00Z","model":"claude-sonnet-4-5",
	     "prompt":9000,"fresh":1000,"cached_read":0,"cached_write":8000,"output":100},
	    {"session":"s1","at":"2026-09-01T10:01:00Z","model":"claude-sonnet-4-5",
	     "prompt":9700,"fresh":500,"cached_read":8000,"cached_write":1200,"output":100}
	  ]
	}`
}

// UO1: a well-formed export parses and every count survives verbatim.
//
// PASS: two records, counts identical to the file.
// FAIL: any field silently dropped or recomputed. Reintroduce by having
// ParseExport fill in Prompt rather than requiring it.
func TestUO1_AWellFormedExportKeepsEveryCountVerbatim(t *testing.T) {
	e, err := ParseExport([]byte(wellFormed()))
	if err != nil {
		t.Fatalf("a well-formed export was refused: %v", err)
	}
	if len(e.Records) != 2 {
		t.Fatalf("read %d records, want 2", len(e.Records))
	}
	r := e.Records[1]
	if r.Prompt != 9700 || r.Fresh != 500 || r.CachedRead != 8000 || r.CachedWrite != 1200 || r.Output != 100 {
		t.Errorf("counts did not survive the parse: %+v", r.Record)
	}
	if r.Session != "s1" || r.Model != "claude-sonnet-4-5" {
		t.Errorf("identity did not survive the parse: session %q model %q", r.Session, r.Model)
	}
	if !r.At.Equal(exportAt(time.Minute)) {
		t.Errorf("timestamp %v, want %v", r.At, exportAt(time.Minute))
	}
}

// UO2: a schema this build does not know is refused, not read hopefully.
//
// The failure it prevents is the quiet one. JSON decoding into a struct
// ignores unknown keys and leaves known ones zero, so a different vendor's
// export would parse "successfully" into a set of zeros and report a corpus
// that cost nothing.
//
// PASS: both the absent and the foreign schema are refused, and the message
// names what was found.
// FAIL: either is accepted. Reintroduce by deleting the schema check.
func TestUO2_AnUnknownSchemaIsRefused(t *testing.T) {
	cases := map[string]string{
		"absent":  `{"complete":true,"records":[{"session":"s1","model":"m","prompt":1,"fresh":1}]}`,
		"foreign": `{"schema":"vendor.usage.v3","complete":true,"records":[{"session":"s1","model":"m","prompt":1,"fresh":1}]}`,
	}
	said := map[string]string{}
	for name, doc := range cases {
		e, err := ParseExport([]byte(doc))
		if err == nil {
			t.Errorf("%s schema was accepted, parsing %d records", name, len(e.Records))
			continue
		}
		if !strings.Contains(err.Error(), ExportSchema) {
			t.Errorf("%s: refusal does not name the schema this build reads (%s): %v", name, ExportSchema, err)
		}
		said[name] = err.Error()
	}
	// The two are different mistakes and get different sentences. A file with
	// no schema at all is most likely the wrong file entirely; one with a
	// foreign schema is the right kind of file from the wrong producer, and
	// quoting `""` at that reader tells them nothing.
	if said["absent"] == said["foreign"] {
		t.Errorf("both refusals read identically (%q); an absent schema and a foreign one are different mistakes", said["absent"])
	}
	if !strings.Contains(said["absent"], "no schema") {
		t.Errorf("the absent case quotes an empty string rather than saying there was no schema: %q", said["absent"])
	}
}

// UO3: a record whose parts do not add up to its prompt is refused, by index.
//
// This is the defect internal/usage was written for, arriving through the
// front door instead of through a reader. Anthropic counts exclusively;
// OpenAI counts inclusively. An exporter that copies an inclusive provider's
// prompt figure into `prompt` and its cached figure into `cached_read` has
// double-counted the cache, and the error is largest on exactly the sessions
// that cache best.
//
// PASS: refused, and the message locates the record.
// FAIL: accepted. Reintroduce by dropping the Validate call in ParseExport.
func TestUO3_ARecordThatDoesNotAddUpIsRefused(t *testing.T) {
	// prompt 1000 with fresh 1000 AND read 800: the inclusive shape copied.
	doc := `{"schema":"replay.usage.v1","complete":true,"records":[
	  {"session":"s1","at":"2026-09-01T10:00:00Z","model":"m","prompt":1000,"fresh":1000,"cached_read":800}]}`
	_, err := ParseExport([]byte(doc))
	if err == nil {
		t.Fatal("an inclusive-counted record was accepted; every share and cost below it divides by a prompt that is 800 tokens short")
	}
	if !strings.Contains(err.Error(), "record 0") {
		t.Errorf("refusal does not locate the record, so a 40,000-row export cannot be fixed: %v", err)
	}
	if !strings.Contains(err.Error(), "does not add up") {
		t.Errorf("refusal does not say what is wrong with it: %v", err)
	}
}

// UO4: a record with no session is refused.
//
// Session is the unit of this report. A record without one is not a row with a
// missing label, it is a row that cannot be placed — and Go's zero value would
// gather every such record into one phantom session whose median and p90 are
// arithmetic over things that never ran together.
//
// PASS: refused, by index.
// FAIL: accepted. Reintroduce by dropping the session check.
func TestUO4_ARecordWithNoSessionIsRefused(t *testing.T) {
	doc := `{"schema":"replay.usage.v1","complete":true,"records":[
	  {"session":"s1","at":"2026-09-01T10:00:00Z","model":"m","prompt":10,"fresh":10},
	  {"at":"2026-09-01T10:01:00Z","model":"m","prompt":10,"fresh":10}]}`
	_, err := ParseExport([]byte(doc))
	if err == nil {
		t.Fatal("a record with no session was accepted; it would be folded into a session named \"\"")
	}
	if !strings.Contains(err.Error(), "record 1") {
		t.Errorf("refusal does not locate the record: %v", err)
	}
}

// UO4b: a record with no model is refused.
//
// The model is load-bearing twice over. It is the key into the price table, so
// without it there is no cost; and ClassifyBreak decides "model changed
// between requests" by comparing this field across two records, so an absent
// model would make two requests on different models look like one unbroken
// prefix. Neither failure announces itself.
//
// PASS: refused, by index.
// FAIL: accepted. Reintroduce by dropping the model check in ParseExport.
func TestUO4b_ARecordWithNoModelIsRefused(t *testing.T) {
	doc := `{"schema":"replay.usage.v1","complete":true,"records":[
	  {"session":"s1","at":"2026-09-01T10:00:00Z","model":"claude-sonnet-4-5","prompt":10,"fresh":10},
	  {"session":"s1","at":"2026-09-01T10:01:00Z","prompt":10,"fresh":10}]}`
	_, err := ParseExport([]byte(doc))
	if err == nil {
		t.Fatal("a record with no model was accepted; it has no price and it cannot be told apart from the model before it")
	}
	if !strings.Contains(err.Error(), "record 1") {
		t.Errorf("refusal does not locate the record: %v", err)
	}
}

// UO5: an export with no records is refused rather than reported as $0.00.
//
// A finance owner pointing this at the wrong file, or at an export whose date
// filter matched nothing, must not be handed a clean bill of health. ADR-0018:
// absence is not zero.
//
// PASS: refused.
// FAIL: parses, and the report downstream prints a total of $0.00 over 0
// sessions. Reintroduce by deleting the empty check.
func TestUO5_AnEmptyExportIsRefusedNotPricedAtZero(t *testing.T) {
	for name, doc := range map[string]string{
		"empty list": `{"schema":"replay.usage.v1","complete":true,"records":[]}`,
		"no key":     `{"schema":"replay.usage.v1","complete":true}`,
	} {
		if _, err := ParseExport([]byte(doc)); err == nil {
			t.Errorf("%s: an export naming no records was accepted, so the report below it is a $0.00 clean bill over nothing", name)
		}
	}
}

// UO6: a negative count is refused.
//
// Validate checks that the parts sum to the prompt, and -500 + 500 sums as
// happily as 0 + 0. A negative token count is not a measurement of anything,
// and left in place it subtracts from a corpus total.
//
// PASS: refused.
// FAIL: accepted. Reintroduce by deleting the negative check.
func TestUO6_ANegativeCountIsRefused(t *testing.T) {
	doc := `{"schema":"replay.usage.v1","complete":true,"records":[
	  {"session":"s1","at":"2026-09-01T10:00:00Z","model":"m","prompt":0,"fresh":-500,"cached_read":500}]}`
	if _, err := ParseExport([]byte(doc)); err == nil {
		t.Fatal("a negative token count was accepted; it sums correctly and subtracts from the corpus total")
	}
}

// UO7: records are sequenced by their timestamps, not by their position in the
// file.
//
// A cache break is defined against the request BEFORE it. An export is a query
// result and arrives in whatever order the query returned; trusting file order
// would compare request 7 against request 3 and manufacture a break out of the
// comparison.
//
// The expectation is a literal order somebody has to change deliberately, not
// a re-sort of the input.
//
// PASS: 10:00, 10:01, 10:02 whatever order they were written in.
// FAIL: file order preserved. Reintroduce by deleting the sort in BySession.
func TestUO7_RecordsAreSequencedByTimeNotByFileOrder(t *testing.T) {
	doc := `{"schema":"replay.usage.v1","complete":true,"records":[
	  {"session":"s1","at":"2026-09-01T10:02:00Z","model":"m","prompt":10,"fresh":10,"id":"third"},
	  {"session":"s1","at":"2026-09-01T10:00:00Z","model":"m","prompt":10,"fresh":10,"id":"first"},
	  {"session":"s2","at":"2026-09-01T10:05:00Z","model":"m","prompt":10,"fresh":10,"id":"other"},
	  {"session":"s1","at":"2026-09-01T10:01:00Z","model":"m","prompt":10,"fresh":10,"id":"second"}]}`
	e, err := ParseExport([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	groups := e.BySession()
	if len(groups) != 2 {
		t.Fatalf("grouped into %d sessions, want 2", len(groups))
	}
	var s1 []string
	for _, g := range groups {
		if g.Session != "s1" {
			continue
		}
		for _, r := range g.Records {
			s1 = append(s1, r.ID)
		}
	}
	want := []string{"first", "second", "third"}
	if strings.Join(s1, ",") != strings.Join(want, ",") {
		t.Errorf("s1 sequenced as %v, want %v; a break is defined against the request before it, so the order IS the measurement", s1, want)
	}
}

// UO8: an export that cannot support a sequence says so, and says which fact
// is missing.
//
// Two separate facts are needed and they fail differently, so the reason has
// to distinguish them: a missing timestamp means the records cannot be
// ordered; an undeclared `complete` means they can be ordered but a gap
// between two of them is indistinguishable from a cache break.
//
// PASS: the well-formed export has the evidence; each defective one does not,
// and the two reasons are different sentences.
// FAIL: either defect passes, or both produce the same reason — which would
// make the message unable to tell a reader what to fix.
func TestUO8_AnExportWithoutSequenceEvidenceSaysWhich(t *testing.T) {
	ok, why := mustParse(t, wellFormed()).SequenceEvidence()
	if !ok {
		t.Fatalf("a complete, timestamped export was said to lack sequence evidence: %s", why)
	}
	if why != "" {
		t.Errorf("evidence present but a reason was given anyway: %q", why)
	}

	noTime := `{"schema":"replay.usage.v1","complete":true,"records":[
	  {"session":"s1","at":"2026-09-01T10:00:00Z","model":"m","prompt":10,"fresh":10},
	  {"session":"s1","model":"m","prompt":10,"fresh":10}]}`
	okT, whyT := mustParse(t, noTime).SequenceEvidence()
	if okT {
		t.Error("an export with an undated record claimed sequence evidence; the records cannot be put in order at all")
	}

	notDeclared := strings.Replace(wellFormed(), `"complete": true`, `"complete": false`, 1)
	okC, whyC := mustParse(t, notDeclared).SequenceEvidence()
	if okC {
		t.Error("an export that does not declare itself complete claimed sequence evidence; a missing request reads as a cache break")
	}

	if whyT == whyC {
		t.Errorf("both defects give the same reason (%q), so the message cannot tell a reader which one to fix", whyT)
	}
}

func mustParse(t *testing.T, doc string) *Export {
	t.Helper()
	e, err := ParseExport([]byte(doc))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return e
}
