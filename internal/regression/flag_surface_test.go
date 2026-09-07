package regression

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Every flag the CLI defines must appear in the flag-surface design.
//
// The document classifies all 72 into six kinds of screen element, and its
// value rests entirely on being complete. A design that covers most of the
// flags is a design with a hole exactly where nobody looked, and the hole is
// invisible: the document reads as finished either way.
//
// So adding a flag without deciding what it looks like fails here. That is a
// small cost at the moment the flag is written and a large one later, when the
// surface is built against a map that quietly stopped matching the territory.
func TestFlagSurface_EveryFlagIsClassified(t *testing.T) {
	root := repoRoot(t)

	doc, err := os.ReadFile(filepath.Join(root, "docs", "TUI-FLAG-SURFACE.md"))
	if err != nil {
		t.Fatalf("reading the flag surface design: %v", err)
	}
	documented := map[string]bool{}
	for _, m := range regexp.MustCompile("`--([a-z0-9-]+)`").FindAllStringSubmatch(string(doc), -1) {
		documented[m[1]] = true
	}

	defined := map[string]bool{}
	dir := filepath.Join(root, "cmd", "replay")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	decl := regexp.MustCompile(`\.(?:String|Int|Int64|Bool|Duration|Float64)\("([a-z0-9-]+)"`)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		for _, m := range decl.FindAllStringSubmatch(string(b), -1) {
			defined[m[1]] = true
		}
	}
	if len(defined) == 0 {
		t.Fatal("no flags found in cmd/replay. This check would pass over an empty set, " +
			"which is the shape of a test that cannot fail")
	}

	var missing []string
	for f := range defined {
		if !documented[f] {
			missing = append(missing, f)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("%d flag(s) exist in cmd/replay and are not classified in "+
			"docs/TUI-FLAG-SURFACE.md: %s\n\n"+
			"Decide what each one looks like on screen, or the design has a hole "+
			"exactly where nobody looked. Six archetypes are already defined; most "+
			"new flags are a threshold, a posture, or a scope.",
			len(missing), strings.Join(missing, ", "))
	}
}
