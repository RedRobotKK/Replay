package surface

import (
	"os"
	"path/filepath"
	"testing"
)

// The probe must return the same Observables for the same bytes, every time.
//
// This is the floor under every other claim in the package. A reading that
// moves between runs cannot be reproduced by a reviewer, and a classification
// built on it is not evidence however carefully it is argued.
//
// Observables is comparable on purpose, so this is a struct equality rather
// than a field-by-field check that a new field could silently escape.
func TestProbeIsDeterministic(t *testing.T) {
	for _, tc := range []struct {
		dir  string
		spec FieldSpec
	}{
		{"testdata/anthropic", AnthropicFields},
		{"testdata/codex", CodexFields},
		{"testdata/grok", GrokFields},
		{"testdata/cursor", AnthropicFields},
	} {
		t.Run(tc.dir, func(t *testing.T) {
			first, err := Probe(tc.dir, tc.spec)
			if err != nil {
				t.Fatal(err)
			}
			for i := 1; i < 5; i++ {
				got, err := Probe(tc.dir, tc.spec)
				if err != nil {
					t.Fatal(err)
				}
				if got != first {
					t.Fatalf("run %d differs from run 0:\n  %+v\n  %+v", i, first, got)
				}
			}
		})
	}
}

// Classify must be a pure function of its two inputs.
//
// If it ever consulted the clock, the filesystem or a package variable, a
// verdict would stop being reproducible from the inputs a reviewer can see.
func TestClassifyIsPure(t *testing.T) {
	o := Observables{
		Boundary: "b", Records: 10,
		WriteFieldPresent: true, RecordsWithWrite: 10, WriteObservations: 10, WriteNonZero: 9,
		RecordsWithRead: 10, ReadObservations: 10, ReadNonZero: 10,
		PerRequestSequencing: true, OracleClasses: 4,
	}
	first := Classify(o, AnthropicContract)
	for i := 0; i < 5; i++ {
		got := Classify(o, AnthropicContract)
		if got.Class != first.Class || got.Why != first.Why {
			t.Fatalf("run %d differs: %q/%q vs %q/%q", i, got.Class, got.Why, first.Class, first.Why)
		}
		if len(got.Conditions) != len(first.Conditions) {
			t.Fatalf("run %d produced %d conditions, want %d", i, len(got.Conditions), len(first.Conditions))
		}
	}
}

// Records and observations are counted separately and must not be conflated.
//
// The Grok fixture states its counters twice per record, at `usage` and again
// under `modelUsage`. A single count would report a corpus of twice its true
// size, which is exactly the misreading that made a documented 1,144 look like
// 2,288 on a re-run.
func TestRepeatedCountersDoNotInflateTheRecordCount(t *testing.T) {
	o, err := Probe("testdata/grok", GrokFields)
	if err != nil {
		t.Fatal(err)
	}
	if o.RecordsWithWrite != o.Records {
		t.Errorf("records with a write field = %d, want %d (every fixture record has one)",
			o.RecordsWithWrite, o.Records)
	}
	if o.WriteObservations != 2*o.RecordsWithWrite {
		t.Errorf("write observations = %d, want %d: the grok fixture states the "+
			"counter twice per record and both counts must survive",
			o.WriteObservations, 2*o.RecordsWithWrite)
	}
}

// A verdict must not flip as a corpus grows.
//
// The live Claude Code tree gains records while this machine works, so any
// re-run reads different totals than the one before it. That is fine for the
// counts and fatal for the conclusion, so the property the audit rests on is
// the CLASS and the sign of the counters, never an exact total. This pins that
// growth cannot change the answer.
func TestVerdictIsStableUnderCorpusGrowth(t *testing.T) {
	base := Observables{
		Boundary: "growing", Records: 1000,
		WriteFieldPresent: true, RecordsWithWrite: 900, WriteObservations: 900, WriteNonZero: 899,
		RecordsWithRead: 900, ReadObservations: 900, ReadNonZero: 890,
		PerRequestSequencing: true, OracleClasses: 6,
	}
	want := Classify(base, AnthropicContract).Class

	grown := base
	for i := 0; i < 20; i++ {
		grown.Records += 137
		grown.RecordsWithWrite += 91
		grown.WriteObservations += 91
		grown.WriteNonZero += 90
		grown.RecordsWithRead += 91
		grown.ReadObservations += 91
		grown.ReadNonZero += 89
		if got := Classify(grown, AnthropicContract).Class; got != want {
			t.Fatalf("after %d rounds of growth the class changed from %q to %q; "+
				"a conclusion that depends on how long the corpus has been "+
				"accumulating is not reproducible", i+1, want, got)
		}
	}

	// The zero-write surfaces must be stable in the same way: more records
	// carrying more zeros is still zero, and still undetermined without a
	// contract.
	zero := Observables{
		Boundary: "growing zeros", Records: 100,
		WriteFieldPresent: true, RecordsWithWrite: 100, WriteObservations: 100, WriteNonZero: 0,
		RecordsWithRead: 100, ReadObservations: 100, ReadNonZero: 99,
		PerRequestSequencing: true,
	}
	for i := 0; i < 20; i++ {
		zero.Records += 50
		zero.RecordsWithWrite += 50
		zero.WriteObservations += 50
		zero.RecordsWithRead += 50
		zero.ReadObservations += 50
		zero.ReadNonZero += 49
		if got := Classify(zero, XAIContract).Class; got != ClassUndetermined {
			t.Fatalf("after %d rounds the zero-write class became %q; accumulating "+
				"zeros is not evidence, however many of them there are", i+1, got)
		}
	}
}

// Re-running the real probe twice in one process must agree with itself.
//
// Opt-in, because its subject is the machine. It catches the case the fixtures
// cannot: a corpus large enough that a buffer limit, a symlink loop or a
// partially written file makes the walk non-deterministic in practice even
// though the code is deterministic in principle.
func TestRealLocalProbeAgreesWithItself(t *testing.T) {
	if os.Getenv("REPLAY_PROBE_REAL") == "" {
		t.Skip("opt-in: set REPLAY_PROBE_REAL=1")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	// Codex and Grok only: the Claude Code tree is written to while this runs,
	// so disagreement there would be the corpus moving rather than the probe.
	for _, tc := range []struct {
		name string
		dir  string
		spec FieldSpec
	}{
		{"codex", filepath.Join(home, ".codex", "sessions"), CodexFields},
		{"grok", filepath.Join(home, ".grok"), GrokFields},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := os.Stat(tc.dir); err != nil {
				t.Skipf("no corpus at %s", tc.dir)
			}
			a, err := Probe(tc.dir, tc.spec)
			if err != nil {
				t.Fatal(err)
			}
			b, err := Probe(tc.dir, tc.spec)
			if err != nil {
				t.Fatal(err)
			}
			if a != b {
				t.Fatalf("two probes of the same corpus disagree:\n  %+v\n  %+v", a, b)
			}
			t.Logf("stable: records=%d recordsWithWrite=%d writeObs=%d writeNonZero=%d readNonZero=%d",
				a.Records, a.RecordsWithWrite, a.WriteObservations, a.WriteNonZero, a.ReadNonZero)
		})
	}
}
