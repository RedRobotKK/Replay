package evidencepin

import "testing"

// A pinned document's bytes must still be its pinned bytes.
//
// This is the only mechanical guarantee these two artifacts support. Their
// historical status cannot be established, so nothing here re-derives an
// experiment; it fixes identity from the pin date forward.
func TestPinnedDocumentsAreUnchanged(t *testing.T) {
	for _, p := range Pins() {
		got, err := HashOf(p.Path)
		if err != nil {
			t.Errorf("%s: %v", p.Path, err)
			continue
		}
		if got != p.SHA256 {
			t.Errorf("%s\n  sha256 %s\n  pinned %s\n"+
				"      The document changed after its provenance pin. Either the "+
				"change is deliberate, in which case move the pin and date it, or "+
				"a pinned artifact is drifting silently.", p.Path, got, p.SHA256)
		}
	}
}

// The pin is forward-only and must say so in its own metadata.
//
// The failure this guards is a future reader, or a future edit, quietly
// upgrading these documents to preregistrations because they carry a hash. A
// hash proves the text has not moved since the pin. It proves nothing about
// what existed before it.
func TestPinsDoNotClaimHistoricalStatus(t *testing.T) {
	for _, p := range Pins() {
		if p.HistoricalStatus != NotVerifiable {
			t.Errorf("%s claims historical status %q; the surviving evidence "+
				"establishes none", p.Path, p.HistoricalStatus)
		}
		if p.PinnedFrom == "" {
			t.Errorf("%s has no pin date, so the boundary it asserts is undated", p.Path)
		}
		if p.Why == "" {
			t.Errorf("%s records no reason; a bare hash invites the reader to "+
				"assume the provenance it does not have", p.Path)
		}
	}
}

// Every pinned document carries the boundary in its own text, not only here.
//
// A reader opening the markdown must meet the distinction without knowing this
// package exists.
func TestEachDocumentStatesItsOwnBoundary(t *testing.T) {
	for _, p := range Pins() {
		body, err := Read(p.Path)
		if err != nil {
			t.Fatal(err)
		}
		for _, phrase := range []string{
			"Provenance boundary",
			"NOT VERIFIABLE from surviving evidence",
			"pinned from 2026-09-27 onward",
			"forward-only",
		} {
			if !contains(body, phrase) {
				t.Errorf("%s does not contain %q in its own text", p.Path, phrase)
			}
		}
	}
}

// The pin set must not be empty, and must cover the documents it exists for.
//
// Every other test in this file iterates Pins(). Emptying that set made all of
// them pass, which is the classic vacuous guard: a loop over nothing asserts
// nothing. A mutation proved it, so the census is asserted here rather than
// assumed.
func TestPinSetIsNotVacuous(t *testing.T) {
	pins := Pins()
	if len(pins) != 2 {
		t.Fatalf("pinned documents = %d, want 2. An empty or shrunken pin set "+
			"makes every other test in this file pass while checking nothing.",
			len(pins))
	}
	want := map[string]bool{
		"../../docs/evidence/gtm-preregistration-2026-09-24.md":        false,
		"../../docs/evidence/seeded-intervention-prereg-2026-09-25.md": false,
	}
	for _, p := range pins {
		if _, ok := want[p.Path]; !ok {
			t.Errorf("unexpected pinned path %q", p.Path)
			continue
		}
		want[p.Path] = true
		if len(p.SHA256) != 64 {
			t.Errorf("%s: sha256 is %d characters, want 64", p.Path, len(p.SHA256))
		}
	}
	for path, seen := range want {
		if !seen {
			t.Errorf("%s is not pinned", path)
		}
	}
}
