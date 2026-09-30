package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// docs/PRODUCTION-WIRING.md cannot claim what the binary does not do.
//
// The matrix is a support claim, and a support claim in prose is the easiest
// false statement in this project to make. Two things already went stale
// without anything failing: docs/evidence/wire-families-2026-09-06.md recorded
// that Grok's local store carries no token counts, which was true when written
// and is now false on 130 files on this machine, and the first version of the
// matrix omitted Codex and Ollama entirely while `replay burn` reported both.
//
// One reads the document, the other reads the code, and they have to agree.

var wiringRow = regexp.MustCompile(`(?m)^\|\s*([^|]+?)\s*\|\s*([^|]+?)\s*\|.*?\|\s*\*\*(PRODUCTION-WIRED|NOT PRODUCTION-WIRED)\*\*\s*\|`)

func matrixBody(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../../docs/PRODUCTION-WIRING.md")
	if err != nil {
		t.Fatalf("the production matrix must exist: %v", err)
	}
	return string(b)
}

// Every surface reaching the cross-surface report is claimed in the matrix.
//
// This is the omission that actually happened: burn reported codex and ollama
// and the matrix did not mention them, so the document understated support
// while reading as though it were complete.
func TestWM1_EveryReportedSurfaceIsInTheMatrix(t *testing.T) {
	body := matrixBody(t)
	// Only the verdict-bearing rows count. Scanning the whole document would
	// pass on a surface named in prose after its row was deleted, which is the
	// weaker check this guard was first written with and which did not fail
	// when the Codex row was removed to test it.
	var wiredRows []string
	for _, m := range wiringRow.FindAllStringSubmatch(body, -1) {
		if m[3] == "PRODUCTION-WIRED" {
			wiredRows = append(wiredRows, strings.ToLower(m[0]))
		}
	}
	for _, s := range []surfaceBurn{
		burnCodex("", t.TempDir()),
		burnOllama("", t.TempDir()),
		burnClaudeCode("", t.TempDir()),
		burnGrok("", t.TempDir()),
	} {
		found := false
		for _, row := range wiredRows {
			// The burn name is written into the row so the two can be joined
			// by something narrower than the vendor's display name.
			if strings.Contains(row, "burn name `"+strings.ToLower(s.name)+"`") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("`replay burn` reports surface %q and no PRODUCTION-WIRED row in "+
				"docs/PRODUCTION-WIRING.md carries \"burn name `%s`\". A matrix that omits a "+
				"wired surface understates support while reading as complete.", s.name, s.name)
		}
	}
}

// A surface the binary refuses is never claimed PRODUCTION-WIRED.
//
// knownSurfaces carries a measured `why:` for each surface this build cannot
// read. If one of those names appears on a PRODUCTION-WIRED row, either the
// refusal is stale or the matrix is wrong, and which one it is has to be
// decided by looking rather than by whichever was edited last.
func TestWM2_ARefusedSurfaceIsNeverClaimedWired(t *testing.T) {
	body := matrixBody(t)
	var refused []string
	for _, s := range knownSurfaces(t.TempDir()) {
		if s.cmd == "" && s.why != "" {
			refused = append(refused, s.name)
		}
	}
	if len(refused) == 0 {
		t.Fatal("no refused surface found in knownSurfaces; this guard would pass vacuously")
	}
	for _, m := range wiringRow.FindAllStringSubmatch(body, -1) {
		if m[3] != "PRODUCTION-WIRED" {
			continue
		}
		row := m[1] + " " + m[2]
		for _, name := range refused {
			if strings.Contains(strings.ToLower(row), strings.ToLower(name)) {
				t.Errorf("%q is refused by knownSurfaces with a measured reason, and the matrix "+
					"marks a row naming it PRODUCTION-WIRED:\n  %s\n"+
					"Either the refusal is stale and must be re-measured, or the matrix overstates support.",
					name, strings.TrimSpace(row))
			}
		}
	}
}

// The matrix states a verdict for every row it lists.
//
// A row with no PRODUCTION-WIRED or NOT PRODUCTION-WIRED cell is a surface
// whose standing the document declines to give, which is the state this whole
// contract exists to remove.
func TestWM3_TheMatrixReachesAVerdictOnEveryVendorRow(t *testing.T) {
	body := matrixBody(t)
	rows := wiringRow.FindAllStringSubmatch(body, -1)
	if len(rows) < 4 {
		t.Fatalf("the matrix carries %d verdict rows; the wired surfaces alone outnumber that", len(rows))
	}
	wired := 0
	for _, m := range rows {
		if m[3] == "PRODUCTION-WIRED" {
			wired++
		}
	}
	if wired == 0 {
		t.Error("the matrix claims no production-wired surface at all")
	}
}
