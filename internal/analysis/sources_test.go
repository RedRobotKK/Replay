package analysis

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tree builds a scratch directory from a path -> contents map.
func tree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for p, body := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func scan(t *testing.T, root string) SourceMap {
	t.Helper()
	m, err := ScanSources(root)
	if err != nil {
		t.Fatalf("ScanSources: %v", err)
	}
	return m
}

func (m SourceMap) dirs() []string {
	out := make([]string, 0, len(m.Dirs))
	for _, d := range m.Dirs {
		out = append(out, d.Dir)
	}
	return out
}

func has(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// TestSC1 is the failure this whole feature exists for, reduced to a fixture.
// A question was answered from funding/ alone while the authoritative record
// sat in assets/applications/. A scan run beforehand had to name that directory.
func TestSC1(t *testing.T) {
	root := tree(t, map[string]string{
		"funding/DEAL-LEDGER.md":             "x",
		"funding/notes.md":                   "x",
		"funding/pitch.md":                   "x",
		"assets/applications/SENT-LEDGER.md": "x",
	})
	got := scan(t, root).dirs()
	if !has(got, "assets/applications") {
		t.Fatalf("the record that was missed is still missed: %v", got)
	}
	if !has(got, "funding") {
		t.Fatalf("funding/DEAL-LEDGER.md should match too: %v", got)
	}
}

// TestSC2 pins the two rules that keep a code repo from matching everything:
// record words match as whole tokens, not substrings, and only documents can
// be records. Without both, every Go file named logger.go is a "source".
func TestSC2(t *testing.T) {
	root := tree(t, map[string]string{
		"internal/log/logger.go":    "x",
		"web/index.html":            "x",
		"web/catalog_index.ts":      "x",
		"internal/store/history.go": "x",
		"cmd/record.go":             "x",
		// A plain document, so Scanned proves the walk ran and this proves an
		// ordinary note is not a record.
		"docs/notes.md": "x",
	})
	m := scan(t, root)
	if len(m.Dirs) != 0 {
		t.Fatalf("code named like records must not match, got %v", m.dirs())
	}
	if m.Scanned == 0 {
		t.Fatal("Scanned==0 means the walk found nothing; the test proves nothing")
	}
}

// TestSC3: an append-only file is a record by its shape, with no record word
// in its name at all.
func TestSC3(t *testing.T) {
	root := tree(t, map[string]string{"social/replay-metrics.jsonl": "{}\n"})
	if got := scan(t, root).dirs(); !has(got, "social") {
		t.Fatalf(".jsonl is append-only by construction and is a record: %v", got)
	}
}

// TestSC4: a dated series is a record even when no single filename says so,
// but two files are a coincidence and three are a series.
func TestSC4(t *testing.T) {
	two := tree(t, map[string]string{
		"docs/ev/a-2026-09-06.md": "x",
		"docs/ev/b-2026-09-07.md": "x",
	})
	if got := scan(t, two).dirs(); has(got, "docs/ev") {
		t.Fatalf("two dated files are not a series: %v", got)
	}
	three := tree(t, map[string]string{
		"docs/ev/a-2026-09-05.md": "x",
		"docs/ev/b-2026-09-06.md": "x",
		"docs/ev/c-2026-09-07.md": "x",
	})
	m := scan(t, three)
	if !has(m.dirs(), "docs/ev") {
		t.Fatalf("three dated files are a series: %v", m.dirs())
	}
	if m.Dirs[0].Why != WhySeries {
		t.Fatalf("Why = %q, want %q", m.Dirs[0].Why, WhySeries)
	}
}

// TestSC5 is the guard against this tool becoming the very thing it warns
// about. A list of sources implies completeness unless it says otherwise, and
// a false all-clear here is worse than no list: it would license exactly the
// narrow search that caused the original failure. The rendering must state the
// rules it used AND name what it skipped.
func TestSC5(t *testing.T) {
	root := tree(t, map[string]string{"funding/DEAL-LEDGER.md": "x"})
	out := scan(t, root).Render()
	for _, want := range []string{
		"not a complete list",
		"node_modules",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("Render() must disclose its own limits, missing %q in:\n%s", want, out)
		}
	}
}

// TestSC6: skipped trees stay skipped. A ledger inside .git or vendor is not
// the project's record.
func TestSC6(t *testing.T) {
	root := tree(t, map[string]string{
		".git/LEDGER.md":              "x",
		"node_modules/p/CHANGELOG.md": "x",
		"vendor/q/HISTORY.md":         "x",
		"real/LEDGER.md":              "x",
	})
	got := scan(t, root).dirs()
	if len(got) != 1 || got[0] != "real" {
		t.Fatalf("only real/ is a project record, got %v", got)
	}
}
