package cachemodel

import (
	"encoding/json"
	"testing"
)

// E1: the exported document is the compiled table, exactly.
// PASS: one row per compiled row, in order, with the same match, floor and
// prices, and the document loads through the ordinary validator.
// FAIL: a missing row, a reordered row, or a value that differs — any of which
// would make the published free tier a different product from the binary.
func TestExportRulesMatchesCompiledTable(t *testing.T) {
	doc := ExportRules()

	if doc.Schema != RulesSchema {
		t.Errorf("schema = %q, want %q", doc.Schema, RulesSchema)
	}
	if doc.Version != RulesVersion {
		t.Errorf("version = %q, want the compiled %q", doc.Version, RulesVersion)
	}
	if len(doc.Models) != len(modelTable) {
		t.Fatalf("exported %d rows, compiled table has %d", len(doc.Models), len(modelTable))
	}
	for i, want := range modelTable {
		got := doc.Models[i]
		if got.Match != want.match {
			t.Errorf("row %d: match = %q, want %q (order carries meaning: rows match by substring, most specific first)", i, got.Match, want.match)
		}
		if got.MinPrefix != want.minPrefix {
			t.Errorf("row %d (%s): minPrefix = %d, want %d", i, want.match, got.MinPrefix, want.minPrefix)
		}
		if got.Priced != want.priced {
			t.Errorf("row %d (%s): priced = %v, want %v", i, want.match, got.Priced, want.priced)
		}
		if got.InputPerMTok != want.price.InputPerMTok {
			t.Errorf("row %d (%s): inputPerMTok = %v, want %v", i, want.match, got.InputPerMTok, want.price.InputPerMTok)
		}
		if got.OutputPerMTok != want.price.OutputPerMTok {
			t.Errorf("row %d (%s): outputPerMTok = %v, want %v", i, want.match, got.OutputPerMTok, want.price.OutputPerMTok)
		}
		// Against what lookup() answers, not against the raw struct field.
		// Comparing to want.price.ReadMult made this assertion unfalsifiable
		// for exactly the rows that drift: those store 0 and behave as 0.10,
		// so the test compared 0 to 0 and passed while the exported document
		// disagreed with the binary.
		wantRead := lookup(want.match).price.ReadMult
		if got.ReadMult != wantRead {
			t.Errorf("row %d (%s): readMult = %v, want %v (what lookup answers, which is what the binary uses)",
				i, want.match, got.ReadMult, wantRead)
		}
		if got.ReadMult == 0 {
			t.Errorf("row %d (%s): readMult exported as 0; omitempty drops it and the feed prices cache reads at nothing", i, want.match)
		}
	}
}

// E2: the export round-trips through the loader it will be installed with.
// PASS: marshalling and validating succeeds.
// FAIL: the binary can publish a document it would itself refuse to install,
// which is the drift this whole export exists to prevent.
func TestExportRulesIsInstallable(t *testing.T) {
	b, err := json.Marshal(ExportRules())
	if err != nil {
		t.Fatal(err)
	}
	var back Rules
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if err := back.validate(); err != nil {
		t.Fatalf("the binary exports a document it will not load: %v", err)
	}
}
