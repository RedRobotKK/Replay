package regression

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
)

// FC-MP. One figure had four populations.
//
// On 2026-09-13 a review counted the mutation catalogue four different ways in
// four places that a launch-day reader would open:
//
//	internal/mutation/testdata/mutants.json   76 entries, M1 to M77, M71 retired
//	README.md                                 "75 ... numbered to M76"
//	.github/workflows/ci.yml                  "75 mutants"
//	.github/workflows/ci.yml                  "mutant 66 of 72"
//
// The file was right and every prose statement about it was wrong, in a
// repository whose stated rule is that every figure carries its population.
// Nothing checked, because a number in a comment is invisible to a compiler.
//
// This is the check. It reads the catalogue and requires the prose to agree.

func mutantCount(t *testing.T, root string) (count, highest int) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, "internal", "mutation", "testdata", "mutants.json"))
	if err != nil {
		t.Fatalf("reading the catalogue: %v", err)
	}
	var doc struct {
		Mutants []struct {
			ID string `json:"id"`
		} `json:"mutants"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("parsing the catalogue: %v", err)
	}
	num := regexp.MustCompile(`\d+`)
	for _, m := range doc.Mutants {
		count++
		if n, err := strconv.Atoi(num.FindString(m.ID)); err == nil && n > highest {
			highest = n
		}
	}
	return count, highest
}

// FC-MP1: the README states the catalogue's real size.
//
// PASS: the count and the highest id in the README match the file.
// FAIL: the page a launch-day auditor reads describes a population that does
// not exist, on the one project whose differentiator is that it does not do
// that.
func TestFCMP1_TheReadmeStatesTheRealCatalogueSize(t *testing.T) {
	root := repoRoot(t)
	count, highest := mutantCount(t, root)

	b, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	readme := string(b)

	for what, want := range map[string]int{"catalogue size": count, "highest mutant id": highest} {
		if !regexp.MustCompile(`\b` + strconv.Itoa(want) + `\b`).MatchString(readme) {
			t.Errorf("README.md does not state the %s, which is %d.\n"+
				"The catalogue holds %d mutants numbered to M%d. Every figure here carries "+
				"its population, and this one is the population.", what, want, count, highest)
		}
	}
}

// FC-MP2: the workflow comments agree with the file too.
//
// They are comments, so nothing else could ever catch them, and they are read
// by whoever is debugging the job at the moment they are least likely to check.
func TestFCMP2_TheWorkflowCommentsAgreeWithTheCatalogue(t *testing.T) {
	root := repoRoot(t)
	count, _ := mutantCount(t, root)

	b, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	ci := string(b)

	// Any two-digit count near the word "mutant" should be the real one.
	for _, m := range regexp.MustCompile(`(?i)(\d{2,3}) mutants`).FindAllStringSubmatch(ci, -1) {
		if n, _ := strconv.Atoi(m[1]); n != count {
			t.Errorf("ci.yml says %q and the catalogue holds %d", m[0], count)
		}
	}
	for _, m := range regexp.MustCompile(`(?i)mutant \d+ of (\d{2,3})`).FindAllStringSubmatch(ci, -1) {
		if n, _ := strconv.Atoi(m[1]); n != count {
			t.Errorf("ci.yml says %q and the catalogue holds %d", m[0], count)
		}
	}
}
