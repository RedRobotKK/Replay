package main

import (
	"encoding/json"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
)

// SC. Nobody knows whether a finding is ever read.
//
// `replay advise` has printed findings since it shipped and there is no record
// anywhere of whether a person looked at one. The product direction written on
// 2026-09-24 proposes building more findings; building more of something nobody
// has established is read is the same error this repository keeps cataloguing
// in other forms.
//
// The funnel this needs is shown -> opened -> acted upon -> outcome. Three of
// those four already exist or are unobservable, and the census matters more
// than the code:
//
//	shown       NOT RECORDED. This is the gap, and it is what these tests cover.
//	opened      NOT OBSERVABLE on the command line. `advise` prints every
//	            finding at once, so there is no open. The TUI has a cursor
//	            (AdviseScreenAt) and could observe it; the CLI cannot, ever.
//	acted upon  ALREADY RECORDED, as the decisions object on the advice file.
//	outcome     ALREADY RECORDED, as the computed status, and bounded by the
//	            verifier's coverage.
//
// So the only honest thing to add is "shown", and the only honest thing to say
// about "opened" is that this build cannot see it.

func surfaceFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	if err := os.MkdirAll(filepath.Join(dir, ".replay"), 0o700); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, ".replay", surfaceCountFileName)
}

// SC1: a run is recorded against the surface that ran.
func TestSC1_ARunIsRecordedAgainstItsSurface(t *testing.T) {
	path := surfaceFixture(t)

	if err := recordSurfaceRun("advise", 140); err != nil {
		t.Fatalf("recording a run failed: %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("nothing was written: %v", err)
	}
	var rec surfaceCounts
	if err := json.Unmarshal(b, &rec); err != nil {
		t.Fatal(err)
	}
	s, ok := rec.Surfaces["advise"]
	if !ok {
		t.Fatal("advise ran and was not recorded")
	}
	if s.Runs != 1 {
		t.Errorf("runs = %d, want 1", s.Runs)
	}
	if s.Shown != 140 {
		t.Errorf("shown = %d, want 140. The count of findings put in front of a "+
			"person is the one number this file exists to carry", s.Shown)
	}
}

// SC2: a surface that never ran is ABSENT, not present with zero.
//
// The distinction ADR-0018 draws for prices applies here and matters more.
// "Nobody ran `replay trim`" and "`replay trim` ran and showed nothing" are
// different facts, and a reader deciding what to build next would act
// differently on each. A map with a zero-valued entry for every known
// subcommand would destroy that distinction on the first write.
func TestSC2_AnUnrunSurfaceIsAbsentRatherThanZero(t *testing.T) {
	surfaceFixture(t)

	if err := recordSurfaceRun("advise", 3); err != nil {
		t.Fatal(err)
	}
	rec, err := readSurfaceCounts()
	if err != nil {
		t.Fatal(err)
	}
	if _, present := rec.Surfaces["trim"]; present {
		t.Error("a surface that never ran has an entry. A zero there reads as " +
			"'ran and did nothing', which is a different fact from 'never ran'")
	}
	if len(rec.Surfaces) != 1 {
		t.Errorf("%d surfaces recorded after one run", len(rec.Surfaces))
	}
}

// SC3: this build states which funnel stages it can observe, and opened is not
// one of them.
//
// The point is not the list. It is that the list exists and is asserted, so
// that a later reader cannot take the absence of open counts as evidence that
// nobody opened anything. A stage this build cannot see must say so rather than
// be silently missing.
func TestSC3_OpenedIsDeclaredUnobservableRatherThanMissing(t *testing.T) {
	obs := observableStages()

	for _, want := range []string{"shown", "acted"} {
		if !obs[want] {
			t.Errorf("stage %q is observable and is not declared so", want)
		}
	}
	if obs["opened"] {
		t.Error("this build declares it can observe whether a finding was opened. " +
			"`replay advise` prints every finding at once, so there is no open to " +
			"observe on the command line")
	}
	if _, declared := obs["opened"]; !declared {
		t.Error("the opened stage is missing from the declaration entirely. " +
			"Missing and false are different: a reader must be able to tell " +
			"'we looked and cannot see it' from 'nobody thought about it'")
	}
}

// SC4: the record never leaves the machine.
//
// Replay's binary sends nothing, and a usage counter is exactly the feature
// that erodes that. This is enforced by reading the source rather than by
// intention: the file that owns the counter may not import a transport.
func TestSC4_TheSurfaceCounterCannotTransmit(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "surfacecount.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("cannot read the counter's own source: %v", err)
	}
	banned := map[string]bool{
		`"net"`: true, `"net/http"`: true, `"net/url"`: true,
		`"os/exec"`: true, `"net/rpc"`: true,
	}
	for _, imp := range f.Imports {
		if banned[imp.Path.Value] {
			t.Errorf("surfacecount.go imports %s. A local usage counter that can "+
				"reach the network is a telemetry client wearing a different name",
				imp.Path.Value)
		}
	}
}

// SC5: a missing or unreadable file is not an error, and never loses a run.
//
// The first run on any machine has no file. A truncated one is a normal
// consequence of a kill. Neither is a reason to fail a command the user
// actually asked for, and a counter that can break `replay advise` is worse
// than no counter.
func TestSC5_AMissingOrCorruptFileNeverBreaksTheCommand(t *testing.T) {
	path := surfaceFixture(t)

	rec, err := readSurfaceCounts()
	if err != nil {
		t.Errorf("reading a missing file errored: %v", err)
	}
	if len(rec.Surfaces) != 0 {
		t.Error("a missing file produced surfaces")
	}

	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readSurfaceCounts(); err != nil {
		t.Errorf("reading a corrupt file errored: %v", err)
	}
	if err := recordSurfaceRun("advise", 1); err != nil {
		t.Errorf("recording over a corrupt file errored: %v", err)
	}
}
