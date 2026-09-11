package regression

import (
	"regexp"
	"strings"
	"testing"
)

// A generated artifact under version control may not embed a date.
//
// docs/CLI.md is generated from the binary and compared against it in CI. The
// job's own comment says why it carries no date: "a generated artifact that
// embeds today fails every morning, and a check that fails daily gets switched
// off."
//
// Then `replay pool` grew a --pooled-at whose default is time.Now(), the
// generator wrote that date into the reference, and at the next midnight UTC
// the file said one date and the binary printed another. Every open pull
// request failed the blueprint check and not one of them for a reason of its
// own — the failure looked like twenty separate problems and was one.
//
// The generator now normalises a date-valued default. This is the check that
// the normalisation is still happening, because the next dated default will
// arrive in a different flag and nobody will remember this.
//
// PASS: no generated reference carries a literal date.
// FAIL: one does, and the build breaks at midnight for everybody at once.
func TestGeneratedReferencesCarryNoDate(t *testing.T) {
	// Generated from the binary, compared against it, under version control.
	generated := []string{"docs/CLI.md"}

	// A date-valued DEFAULT, not any date. The first version of this check
	// flagged `context-management-2025-06-27`, which is the name of an
	// Anthropic beta header — a stable literal that will read the same in ten
	// years. What drifts is a default computed from the clock, and that has
	// one shape.
	date := regexp.MustCompile(`\(default "(19|20)\d{2}-\d{2}-\d{2}"\)`)

	files := textFiles(t, ".md")
	for _, name := range generated {
		body, ok := files[name]
		if !ok {
			t.Errorf("%s is not in the tree; this check is asserting over nothing", name)
			continue
		}
		for i, line := range strings.Split(body, "\n") {
			if m := date.FindString(line); m != "" {
				t.Errorf("%s:%d carries a date-valued default %q, so this file disagrees with the "+
					"binary from the next midnight onward and every open branch fails a check "+
					"none of them caused:\n  %s\n\nNormalise it in scripts/cli-blueprint/gen.py "+
					"as the --pooled-at default is normalised.", name, i+1, m, strings.TrimSpace(line))
			}
		}
	}
}
