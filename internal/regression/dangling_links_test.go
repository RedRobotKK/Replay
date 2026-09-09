package regression

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// A markdown link to a file that does not exist.
//
// Found by nearly shipping one. The README gained a reference to
// docs/adr/0018-this-is-an-instrument-not-an-app.md on a branch where that file
// did not yet exist, and the whole suite went green: docs_drift_test.go asks
// whether every document is linked FROM somewhere, and nothing asked whether
// what a document links TO is there.
//
// The two checks look alike and catch opposite failures. An orphan is a
// document nobody can reach. A dangling link is a promise the reader follows
// and finds nothing behind — worse on this project than most, because the
// argument is that every claim is checkable, and the links are how a reader
// checks them. A 404 on the way to the evidence is the evidence failing.
//
// Only repository-relative links are checked. External URLs need the network,
// which this suite does not use; scripts/installer-drift covers the one hosted
// artifact that matters.

// linkRe matches an inline markdown link target.
var linkRe = regexp.MustCompile(`\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)

// DL1: every repository-relative markdown link resolves to a file.
func TestDL1_NoDanglingLinks(t *testing.T) {
	root := repoRoot(t)
	var checked int
	var broken []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			case ".git", ".claude", "vendor", "node_modules", "dist", "bin":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".md" {
			return nil
		}
		b, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		from, _ := filepath.Rel(root, path)
		for _, m := range linkRe.FindAllStringSubmatch(string(b), -1) {
			target := m[1]
			switch {
			case strings.HasPrefix(target, "http://"), strings.HasPrefix(target, "https://"),
				strings.HasPrefix(target, "mailto:"), strings.HasPrefix(target, "#"):
				continue
			}
			// A fragment on a repository path still names a file.
			if i := strings.IndexByte(target, '#'); i >= 0 {
				target = target[:i]
			}
			if target == "" {
				continue
			}
			checked++
			var abs string
			if strings.HasPrefix(target, "/") {
				abs = filepath.Join(root, target)
			} else {
				abs = filepath.Join(filepath.Dir(path), target)
			}
			if _, statErr := os.Stat(abs); statErr != nil {
				broken = append(broken, from+" -> "+m[1])
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// A link checker that found no links would pass on an empty repository and
	// on a regex that stopped matching. This project has hundreds.
	if checked < 50 {
		t.Fatalf("only %d repository-relative links were checked; the matcher is finding "+
			"almost nothing and would pass however many were broken", checked)
	}
	if len(broken) > 0 {
		sort.Strings(broken)
		t.Errorf("%d markdown link(s) point at files that do not exist. Every claim here is "+
			"meant to be checkable, and the links are how a reader checks it:\n  %s",
			len(broken), strings.Join(broken, "\n  "))
	}
	t.Logf("%d repository-relative links resolved", checked)
}
