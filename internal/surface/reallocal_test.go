package surface

import (
	"os"
	"path/filepath"
	"testing"
)

// Probe the real corpora on this machine, when asked to.
//
// This test is OPT-IN and skipped by default, because its subject is the
// machine rather than the code. A suite that asserted "Codex writes are zero
// here" would pass or fail on whose laptop ran it, which is the defect
// docs/evidence records as a test whose subject is the runner.
//
// Run with: REPLAY_PROBE_REAL=1 go test ./internal/surface/ -run RealLocal -v
//
// What it is for: the cross-surface audit's numbers were produced by throwaway
// scripts that no longer exist. This makes the same readings reproducible from
// the repository, so a reviewer can re-derive them rather than trust them.
func TestRealLocalCorporaMatchTheRecordedClass(t *testing.T) {
	if os.Getenv("REPLAY_PROBE_REAL") == "" {
		t.Skip("opt-in: set REPLAY_PROBE_REAL=1 to probe this machine's corpora")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		dir  string
		spec FieldSpec
		c    ContractFact
		want Class
	}{
		{"claude code", filepath.Join(home, ".claude", "projects"), AnthropicFields, AnthropicContract, Class0Identified},
		{"codex transcript", filepath.Join(home, ".codex", "sessions"), CodexFields, OpenAIModernContract, ClassIIArtifactLoss},
		{"grok", filepath.Join(home, ".grok"), GrokFields, XAIContract, ClassUndetermined},
		// Cursor keeps agent transcripts as JSON Lines under
		// ~/.cursor/projects/<project>/agent-transcripts/, a boundary the
		// original audit did not scan: that scan read the sqlite state store
		// only. Both boundaries lack a cache-write field, so the audit's
		// conclusion holds and now rests on two independent corpora.
		{"cursor transcripts", filepath.Join(home, ".cursor", "projects"), AnthropicFields, AnthropicContract, ClassIIINoObservable},
	}
	// Ollama and JEV are covered by the audit but NOT by this prober, and the
	// difference is the boundary's format rather than its content. Probe reads
	// JSON Lines; Ollama's records live in logs, and no JEV capture exists on
	// this machine.
	//
	// They are listed anyway, and the expected verdict is undetermined. That
	// is the point: a prober that found no JSONL must not report "no write
	// observable", because a corpus it cannot read and a corpus with no such
	// field are different facts. The audit's rows for these three rest on
	// their own scans and are not reproduced here.
	outOfScope := []struct {
		name string
		dir  string
	}{
		{"ollama (log boundary)", filepath.Join(home, ".ollama")},
		{"jev (no captures on this machine)", filepath.Join(home, ".jev")},
	}
	for _, tc := range outOfScope {
		t.Run(tc.name, func(t *testing.T) {
			o, err := Probe(tc.dir, AnthropicFields)
			if err != nil {
				t.Fatal(err)
			}
			v := Classify(o, AnthropicContract)
			t.Logf("records=%d writeFieldPresent=%v -> class=%s", o.Records, o.WriteFieldPresent, v.Class)
			if v.Class != ClassUndetermined {
				t.Errorf("class = %q, want %q: this boundary is not JSON Lines, "+
					"so finding nothing here is a fact about the prober's reach "+
					"and not about the surface's fields", v.Class, ClassUndetermined)
			}
		})
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := os.Stat(tc.dir); err != nil {
				t.Skipf("no corpus at %s on this machine", tc.dir)
			}
			o, err := Probe(tc.dir, tc.spec)
			if err != nil {
				t.Fatal(err)
			}
			if o.Records == 0 {
				t.Skipf("corpus at %s has no parseable records", tc.dir)
			}
			v := Classify(o, tc.c)
			t.Logf("records=%d write(present=%v records=%d obs=%d nonzero=%d) read(records=%d obs=%d nonzero=%d) seq=%v oracle=%d nulls-interpretable=%v",
				o.Records, o.WriteFieldPresent, o.RecordsWithWrite, o.WriteObservations, o.WriteNonZero,
				o.RecordsWithRead, o.ReadObservations, o.ReadNonZero, o.PerRequestSequencing, o.OracleClasses,
				NullsInterpretable(o))
			t.Logf("class=%s why=%s", v.Class, v.Why)
			if v.Class != tc.want {
				t.Errorf("class = %q, want %q (the class recorded in "+
					"docs/evidence/cross-surface-observability-2026-09-26.md)", v.Class, tc.want)
			}
		})
	}
}
