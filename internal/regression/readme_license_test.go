package regression

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The licence is BUSL-1.1 and the repository has to say so in words.
//
// GitHub's licence detector does not recognise the Business Source License,
// so the repository page labels it "Other". A reader who stops at that label
// learns nothing; a reader who opens LICENSE learns everything. The README's
// job is to close that gap by naming the licence, the Change Date and the
// file, so nobody has to guess what "Other" means.
//
// The LICENSE itself must be the canonical BUSL-1.1 text. Covenant 4 of that
// text forbids modifying it in any other way than filling in the parameters,
// and the first version of this file was missing the paragraph in which
// MariaDB grants permission to use the text at all. A licence that omits its
// own permission clause is a licence the project is not entitled to be using.

const githubLabelSentence = "GitHub labels this repository 'Other' because its " +
	"licence detector does not know BUSL-1.1; the licence is the Business " +
	"Source License 1.1, converting to the Apache License 2.0 on 2029-09-06, " +
	"and the text is in LICENSE."

// mariadbPermission is the paragraph the LICENSE was missing, whitespace
// normalised because the file wraps at 79 columns.
const mariadbPermission = "MariaDB hereby grants you permission to use this " +
	"License's text to license your works, and to refer to it using the " +
	"trademark \"Business Source License\", as long as you comply with the " +
	"Covenants of Licensor below."

func licenseText(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "LICENSE"))
	if err != nil {
		t.Fatalf("reading LICENSE: %v", err)
	}
	return string(b)
}

// flatten collapses runs of whitespace so a wrapped paragraph can be matched
// as a single sentence.
func flatten(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// RL1: the README names the licence and its Change Date.
func TestReadmeNamesTheLicence(t *testing.T) {
	text := flatten(readme(t))
	for _, want := range []string{"BUSL-1.1", "2029-09-06"} {
		if !strings.Contains(text, want) {
			t.Errorf("the README does not contain %q; a reader who sees GitHub's "+
				"\"Other\" label has nowhere to learn what the licence is", want)
		}
	}
}

// RL2: the README says in words what GitHub cannot, once near the badges
// where the "Other" label is first met and once in the License section.
func TestReadmeExplainsTheGitHubLabel(t *testing.T) {
	text := flatten(readme(t))
	if n := strings.Count(text, githubLabelSentence); n < 2 {
		t.Errorf("the README carries the GitHub-label sentence %d time(s); it "+
			"belongs both near the badge row and in the License section", n)
	}
	i := strings.Index(text, "## License")
	if i == -1 {
		t.Fatal("the README has no \"## License\" section; this test has nothing to read")
	}
	if !strings.Contains(text[i:], githubLabelSentence) {
		t.Error("the License section does not carry the GitHub-label sentence")
	}
}

// RL3: the LICENSE carries the paragraph in which MariaDB grants permission
// to use its text, without which the project is using the text unlicensed.
func TestLicenseCarriesTheMariaDBPermission(t *testing.T) {
	text := flatten(licenseText(t))
	if !strings.Contains(text, mariadbPermission) {
		t.Error("the LICENSE is missing the paragraph in which MariaDB grants " +
			"permission to use the Business Source License text. Covenant 4 " +
			"forbids the omission; it belongs after the warranty disclaimer and " +
			"before \"Covenants of Licensor\"")
	}
}

// RL4: the four parameters the licence needs are the ones the README claims.
func TestLicenseParametersAreFilledIn(t *testing.T) {
	text := licenseText(t)
	for _, p := range []struct{ name, pattern string }{
		{"Licensor", `(?m)^Licensor:\s+RedRobot KK$`},
		{"Licensed Work", `(?m)^Licensed Work:\s+Replay$`},
		{"Change Date", `(?m)^Change Date:\s+2029-09-06$`},
		{"Change License", `(?m)^Change License:\s+Apache License, Version 2\.0$`},
	} {
		if !regexp.MustCompile(p.pattern).MatchString(text) {
			t.Errorf("the LICENSE does not set the %s parameter as expected (%s)",
				p.name, p.pattern)
		}
	}
}
