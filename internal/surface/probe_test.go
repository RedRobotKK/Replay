package surface

import "testing"

// The prober must report a present-and-zero counter differently from an absent
// one, because the whole classification turns on that difference.
func TestProbeSeparatesZeroFromAbsent(t *testing.T) {
	codex, err := Probe("testdata/codex", CodexFields)
	if err != nil {
		t.Fatal(err)
	}
	if !codex.WriteFieldPresent {
		t.Error("codex fixture: write field reported absent, but every record carries it")
	}
	if codex.WriteObservations == 0 || codex.WriteNonZero != 0 {
		t.Errorf("codex fixture: write records=%d non-zero=%d, want records>0 and non-zero=0",
			codex.WriteObservations, codex.WriteNonZero)
	}
	if codex.ReadNonZero == 0 {
		t.Error("codex fixture: reads reported zero, but the fixture populates them; " +
			"a prober that cannot see the live read counter beside the dead write " +
			"counter cannot report the signature that matters")
	}

	cursor, err := Probe("testdata/cursor", AnthropicFields)
	if err != nil {
		t.Fatal(err)
	}
	if cursor.WriteFieldPresent {
		t.Error("cursor fixture: write field reported present, but no record carries one")
	}
	if cursor.Records == 0 {
		t.Error("cursor fixture: no records parsed, so the absence proves nothing")
	}
}

// The oracle is counted by CLASS, not by event. Keying on the whole diagnostic
// object would turn a four-class partition into one class per event and make
// the count meaningless.
func TestProbeCountsOracleClassesNotEvents(t *testing.T) {
	o, err := Probe("testdata/anthropic", AnthropicFields)
	if err != nil {
		t.Fatal(err)
	}
	if o.OracleClasses != 2 {
		t.Errorf("oracle classes = %d, want 2: the fixture carries two distinct "+
			"cause types across three records", o.OracleClasses)
	}
	if o.WriteNonZero == 0 || o.ReadNonZero == 0 {
		t.Errorf("anthropic fixture: write non-zero=%d read non-zero=%d, want both > 0",
			o.WriteNonZero, o.ReadNonZero)
	}
}

// A surface with no oracle must report none, rather than borrowing one.
func TestProbeReportsNoOracleWhereThereIsNone(t *testing.T) {
	for _, tc := range []struct {
		dir  string
		spec FieldSpec
	}{
		{"testdata/codex", CodexFields},
		{"testdata/grok", GrokFields},
	} {
		o, err := Probe(tc.dir, tc.spec)
		if err != nil {
			t.Fatal(err)
		}
		if o.OracleClasses != 0 {
			t.Errorf("%s: oracle classes = %d, want 0", tc.dir, o.OracleClasses)
		}
	}
}

// End to end, on fixture corpora, with each surface's own contract.
//
// This is the reproducible form of the cross-surface audit: the classification
// follows from a probe anyone can re-run plus a contract anyone can dispute,
// rather than from a number in a paragraph.
func TestFixtureCorporaClassifyAsTheAuditFound(t *testing.T) {
	cases := []struct {
		name string
		dir  string
		spec FieldSpec
		c    ContractFact
		want Class
	}{
		{"claude code", "testdata/anthropic", AnthropicFields, AnthropicContract, Class0Identified},
		{"codex transcript", "testdata/codex", CodexFields, OpenAIModernContract, ClassIIArtifactLoss},
		{"grok, contract unknown", "testdata/grok", GrokFields, XAIContract, ClassUndetermined},
		{"cursor", "testdata/cursor", AnthropicFields, AnthropicContract, ClassIIINoObservable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o, err := Probe(tc.dir, tc.spec)
			if err != nil {
				t.Fatal(err)
			}
			v := Classify(o, tc.c)
			if v.Class != tc.want {
				t.Errorf("class = %q, want %q\n  why: %s", v.Class, tc.want, v.Why)
			}
			if v.Why == "" {
				t.Error("verdict carries no reason")
			}
		})
	}
}

// A probe of a directory that does not exist reports nothing observed rather
// than inventing an absence. Zero records is undetermined, never class III.
func TestMissingCorpusIsUndeterminedNotAbsent(t *testing.T) {
	o, _ := Probe("testdata/does-not-exist", AnthropicFields)
	if o.Records != 0 {
		t.Fatalf("records = %d, want 0", o.Records)
	}
	if got := Classify(o, AnthropicContract).Class; got != ClassUndetermined {
		t.Errorf("class = %q, want %q: a corpus that was never there is not "+
			"evidence that a field is missing", got, ClassUndetermined)
	}
}
