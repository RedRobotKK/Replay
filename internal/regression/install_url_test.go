package regression

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// One install command, everywhere it is printed.
//
// Three addresses shipped in the same product, all serving the same 25,481
// bytes:
//
//	README.md          https://redrobot.jp/replay.sh
//	install.sh:4       https://raw.githubusercontent.com/RedRobotKK/Replay/main/install.sh
//	install.sh:66      https://redrobot.jp/Replay/install.sh   (the installer's own --help)
//
// Nothing is broken. All three return 200 and the same script. That is exactly
// what makes it worth fixing rather than shrugging at: the pitch is that this
// is "written to be read before it is run", and the reader who takes that
// seriously is the one who runs --help, gets a third URL, and now has to decide
// which of the three the project actually meant.
//
// A tool whose whole argument is that its numbers are checkable cannot be
// careless about the one command a stranger types first.
//
// canonicalInstallURL is what the README leads with and what `replay cost
// --share` puts on the card, which is also the address most people will have
// seen before they reach the repository.

const canonicalInstallURL = "https://redrobot.jp/replay.sh"

// installURL matches any address that serves the installer, in any of the
// three shapes that shipped.
var installURL = regexp.MustCompile(
	`https://(?:redrobot\.jp/(?:replay\.sh|Replay/install\.sh)|raw\.githubusercontent\.com/RedRobotKK/Replay/[^/\s"'` + "`" + `]+/install\.sh)`)

// IU1: every surface that prints an install command prints the same one.
func TestIU1_TheInstallCommandIsTheSameEverywhere(t *testing.T) {
	root := repoRoot(t)
	surfaces := []string{
		"README.md",
		"install.sh",
		filepath.Join("cmd", "replay", "share.go"),
	}
	found := map[string][]string{}
	for _, rel := range surfaces {
		b, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("reading %s: %v", rel, err)
		}
		for _, m := range installURL.FindAllString(string(b), -1) {
			// The share card appends ?src=card to the same address; that is a
			// query, not a different location.
			found[m] = appendOnce(found[m], rel)
		}
	}
	if len(found) == 0 {
		t.Fatal("no install URL found on any surface, so this guard is matching nothing " +
			"and would pass however many addresses shipped")
	}
	var wrong []string
	for url, where := range found {
		if url != canonicalInstallURL {
			sort.Strings(where)
			wrong = append(wrong, url+" in "+strings.Join(where, ", "))
		}
	}
	if len(wrong) > 0 {
		sort.Strings(wrong)
		t.Errorf("the installer is advertised at %d different addresses. All of them work, "+
			"which is why nobody noticed; the reader who runs --help still gets a "+
			"different URL than the one that brought them here.\n  canonical: %s\n  also: %s",
			len(found), canonicalInstallURL, strings.Join(wrong, "\n        "))
	}
}

func appendOnce(xs []string, s string) []string {
	for _, x := range xs {
		if x == s {
			return xs
		}
	}
	return append(xs, s)
}
