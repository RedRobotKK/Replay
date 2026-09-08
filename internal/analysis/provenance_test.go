package analysis

import (
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// A session whose reads all came from one directory is told so.
//
// From a real failure on 2026-09-07: a question of the form "what was actually
// sent" was answered by reading five files, all of them under funding/, and the
// answer was wrong because the authoritative record lived under
// assets/applications/. Every source agreed, and the agreement meant nothing,
// because the sources shared an origin.
//
// The warning does not say the conclusion is wrong. It says the evidence came
// from one place, which is a fact about the search rather than a judgement
// about the answer.
func TestProvenanceWarnsWhenEveryReadSharesADirectory(t *testing.T) {
	p := ReadProvenance([]string{
		"funding/AI-SAAS-INVESTORS.md",
		"funding/CONNECTION-NOTES-DRAFT.md",
		"funding/FOLLOW-QUEUE.md",
		"funding/FUND-BAKEOFF.md",
		"funding/RAISE-PIPELINE.md",
	})
	if p.Paths != 5 || p.Dirs != 1 {
		t.Fatalf("paths=%d dirs=%d, want 5 and 1", p.Paths, p.Dirs)
	}
	if p.Concentration != 1.0 {
		t.Errorf("concentration = %.2f, want 1.0", p.Concentration)
	}
	w := p.Warning()
	if w == "" {
		t.Fatal("five files from one directory produced no warning")
	}
	if !strings.Contains(w, "funding") || !strings.Contains(w, "5") {
		t.Errorf("the warning must name the directory and the count: %q", w)
	}
}

// A search that actually spread out is not nagged.
func TestProvenanceIsSilentWhenTheSearchSpread(t *testing.T) {
	p := ReadProvenance([]string{
		"funding/AI-SAAS-INVESTORS.md",
		"assets/applications/SENT-LEDGER.md",
		"docs/RAISE.md",
		"outbound/warm_network.txt",
	})
	if w := p.Warning(); w != "" {
		t.Errorf("four directories should not warn, got %q", w)
	}

	// And a lopsided search is still not warned about. Two of three files from
	// one directory is 67% concentration, which a session working inside one
	// package produces legitimately. This cannot tell that case from a narrow
	// search, so it says nothing. Only TOTAL concentration is reported, because
	// that is the one shape where agreement is guaranteed to be uninformative.
	lopsided := ReadProvenance([]string{
		"funding/a.md", "funding/b.md", "assets/applications/SENT-LEDGER.md",
	})
	if lopsided.Concentration <= 0.5 || lopsided.Concentration >= 1.0 {
		t.Fatalf("fixture is not lopsided: %.2f", lopsided.Concentration)
	}
	if w := lopsided.Warning(); w != "" {
		t.Errorf("a lopsided but multi-directory search warned: %q", w)
	}
}

// One or two reads say nothing about search breadth, so they say nothing.
//
// Warning on every small session is how a signal gets ignored. The threshold
// exists so the warning means something when it fires.
func TestProvenanceIsSilentBelowTheThreshold(t *testing.T) {
	for _, paths := range [][]string{
		{},
		{"funding/a.md"},
		{"funding/a.md", "funding/b.md"},
	} {
		if w := ReadProvenance(paths).Warning(); w != "" {
			t.Errorf("%d path(s) warned: %q", len(paths), w)
		}
	}
}

// The same file read many times is one source, not many.
//
// A loop that re-reads one file would otherwise look like a thorough search.
func TestProvenanceCountsDistinctPathsNotReads(t *testing.T) {
	p := ReadProvenance([]string{
		"funding/a.md", "funding/a.md", "funding/a.md",
		"funding/b.md", "funding/b.md",
	})
	if p.Paths != 2 {
		t.Errorf("paths = %d, want 2: a re-read is not a second source", p.Paths)
	}
	// The per-directory tally has to collapse duplicates too. If it counted
	// reads, TopDirPaths would be 5 against 2 paths and Concentration would
	// exceed 1.0, which is not a proportion.
	if p.TopDirPaths != 2 {
		t.Errorf("top-dir paths = %d, want 2: the tally counted reads, not files", p.TopDirPaths)
	}
	if p.Concentration != 1.0 {
		t.Errorf("concentration = %.2f, want exactly 1.0", p.Concentration)
	}
}

// Bash is not parsed for paths, on purpose.
//
// Its label carries a command line, and a command line mentions paths in too
// many shapes to parse without guessing. A guessed path would inflate the
// directory count and SILENCE the warning, which is the failure direction that
// matters: a false all-clear is worse than saying nothing.
func TestProvenanceDoesNotGuessPathsOutOfBashCommands(t *testing.T) {
	blocks := []transcript.Block{
		{ToolName: "Bash", Label: "tool result: Bash grep -r x funding/ assets/ docs/"},
		{ToolName: "Read", Label: "tool result: Read funding/a.md"},
		{ToolName: "Read", Label: "tool result: Read funding/b.md"},
		{ToolName: "Read", Label: "tool result: Read funding/c.md"},
	}
	got := pathsFromBlocks(blocks)
	if len(got) != 3 {
		t.Fatalf("paths = %v, want only the three Read arguments", got)
	}
	// The Bash line names three directories. If it were parsed, Dirs would be
	// 4 and the warning would go quiet on a search that only ever read one.
	if w := ReadProvenance(got).Warning(); w == "" {
		t.Error("a bash command mentioning other directories silenced the warning")
	}
}

// A tool call whose argument the parser did not keep is not a read of nothing.
func TestProvenanceSkipsLabelsWithNoArgument(t *testing.T) {
	got := pathsFromBlocks([]transcript.Block{
		{ToolName: "Read", Label: "tool call: Read"},
		{ToolName: "Read", Label: "tool result: Read "},
	})
	if len(got) != 0 {
		t.Errorf("paths = %v, want none", got)
	}
}
