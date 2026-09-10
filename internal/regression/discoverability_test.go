package regression

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The surfaces a stranger and a machine both read first.
//
// A repository is indexed by search engines, summarised by answer engines, and
// increasingly quoted by models that never render a page. All three read the
// same two artefacts: README.md and llms.txt. Neither has a build step, so
// neither has anything watching it — which is how the README lost its licence
// badge, its demo and its machine-readable summary without a single test going
// red.
//
// These checks are deliberately mechanical and few. Prose quality is not
// testable and this does not try; what is testable is whether the extractable
// parts exist and are well formed, which is the part that rots silently.
//
// The standing instruction behind this file: every release re-checks these
// surfaces rather than assuming a previous release left them correct.

func readmeText(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "README.md"))
	if err != nil {
		t.Fatalf("reading README.md: %v", err)
	}
	return string(b)
}

var imageRe = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]+)`)

// DS1: every content image describes itself.
//
// Alt text is what a screen reader announces, what a model quotes when the
// image is the answer, and what search indexes. Badges are exempt: their alt
// is conventionally the bare label, and shields.io renders the same words into
// the image itself.
func TestDS1_ContentImagesHaveDescriptiveAltText(t *testing.T) {
	var checked int
	for _, m := range imageRe.FindAllStringSubmatch(readmeText(t), -1) {
		alt, src := m[1], m[2]
		if strings.Contains(src, "img.shields.io") || strings.Contains(src, "badge.svg") {
			continue
		}
		checked++
		if len(strings.TrimSpace(alt)) < 20 {
			t.Errorf("the image %s has alt text %q, which describes nothing to a reader who "+
				"cannot see it or a model quoting it", src, alt)
		}
	}
	if checked == 0 {
		t.Fatal("no content images found in README.md; this guard is matching nothing and " +
			"would pass however many shipped without alt text")
	}
}

// DS2: the opening answers "what is this" on its own.
//
// The first prose paragraph is what a search result, a link preview and an
// answer engine quote. It has to stand without the heading above it and
// without the paragraph after it.
func TestDS2_TheOpeningIsASelfContainedAnswer(t *testing.T) {
	var first string
	for _, para := range strings.Split(readmeText(t), "\n\n") {
		p := strings.TrimSpace(para)
		if p == "" || strings.HasPrefix(p, "#") || strings.HasPrefix(p, "[!") ||
			strings.HasPrefix(p, "<!--") || strings.HasPrefix(p, "!") {
			continue
		}
		first = strings.Join(strings.Fields(p), " ")
		break
	}
	if first == "" {
		t.Fatal("README.md has no prose paragraph before its first heading")
	}
	// Long enough to say something, short enough to survive being quoted whole.
	if n := len(first); n < 60 || n > 400 {
		t.Errorf("the opening paragraph is %d characters. Under 60 it says nothing a reader "+
			"can act on; over 400 it is truncated wherever it is quoted:\n  %s", n, first)
	}
}

// DS3: llms.txt exists, and says what the project is.
//
// The convention answer engines and crawlers now look for: a short, structured
// summary at a fixed path, so a machine does not have to infer the project from
// a README written for humans.
//
// It is checked for shape rather than wording — a title, a summary blockquote,
// and links onward — because those are the parts a consumer parses.
func TestDS3_LLMsTxtIsPresentAndWellFormed(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "llms.txt"))
	if err != nil {
		t.Fatalf("llms.txt is missing: %v\n"+
			"It is the fixed path answer engines look for. Without it they infer the "+
			"project from prose written for a different reader.", err)
	}
	s := string(b)
	if !strings.HasPrefix(strings.TrimSpace(s), "# ") {
		t.Error("llms.txt does not open with an H1 naming the project")
	}
	if !strings.Contains(s, "\n> ") {
		t.Error("llms.txt carries no `> ` summary line, which is the part a consumer quotes")
	}
	if n := strings.Count(s, "]("); n < 4 {
		t.Errorf("llms.txt links onward %d time(s); it is an index, and a summary with no "+
			"routes leaves every follow-up question unanswered", n)
	}
}

// DS4: the badges point at things that exist.
//
// A badge is a claim about the project's state. One pointing at a workflow or
// a file that is gone renders as an error image, which reads worse than no
// badge at all.
func TestDS4_BadgesReferenceRealTargets(t *testing.T) {
	readme := readmeText(t)
	root := repoRoot(t)
	var checked int
	for _, m := range regexp.MustCompile(`\[!\[[^\]]*\]\([^)]*\)\]\(([^)]+)\)`).FindAllStringSubmatch(readme, -1) {
		target := m[1]
		if strings.HasPrefix(target, "http") {
			continue // remote, and this suite makes no network calls
		}
		checked++
		if _, err := os.Stat(filepath.Join(root, target)); err != nil {
			t.Errorf("a badge links to %s, which does not exist", target)
		}
	}
	if checked == 0 {
		t.Skip("no repository-relative badge targets to check")
	}
}
