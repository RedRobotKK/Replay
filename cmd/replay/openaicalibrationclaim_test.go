package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// RPL-C039: nobody has calibrated AstraRules() against real OpenAI traffic,
// because no OPENAI_API_KEY, billing-linked account, or other mechanism for
// real, billable OpenAI traffic exists in this environment (checked
// explicitly 2026-10-08). This is NOT_MEASURED and must stay sayable as such:
// a later pass that lands AstraRules() more deeply into production (RPL-C038)
// must not quietly start claiming the one thing that change does not do.
//
// This scans the repository's own top-level claim surfaces -- the files a
// reader of this repository, not a published website, would actually open --
// for a sentence asserting the calibration happened. It is deliberately
// narrower than userFacingSources (which scans Go source string literals):
// the claim this guards against is prose in documentation, not a CLI string.
//
// CLAIM-REGISTER.md is excluded for the same reason userFacingSources
// excludes internal/claims: it is a generated, verbatim rendering of this
// claim's own Text field, which states the sentence under test IN ORDER TO
// mark it NOT_MEASURED. Scanning it would make the register self-poison the
// test that is supposed to keep the register honest.
func TestC039_NoOpenAICalibrationAgainstLiveTrafficClaimIsAsserted(t *testing.T) {
	// calibrat*/verif*/measur* ... against/with ... real/live ... openai,
	// in either order, within a short span -- the shape a "we did it"
	// sentence takes.
	bad := regexp.MustCompile(`(?i)(calibrat\w*|verif\w*|measur\w*)\w*\s+(\w+\s+){0,4}(against|with)\s+(\w+\s+){0,3}(real|live)\s+openai|` +
		`(?i)openai\s+(\w+\s+){0,2}calibration\s+(is\s+)?(complete|done|finished)|` +
		`(?i)(real|live)\s+openai\s+(\w+\s+){0,2}(calibrat\w*|verif\w*|measur\w*)`)

	// A match preceded closely by a negation is the disclosure sentence this
	// claim is SUPPOSED to carry ("no calibration against real OpenAI
	// traffic has been performed"), not an assertion that it happened.
	// Without this, this project's own correct disclosure of RPL-C039 would
	// fail the test that exists to let it be disclosed.
	negation := regexp.MustCompile(`(?i)\b(no|not|never|none|nobody|cannot|can't|n't|zero|without)\b[^.!?]{0,40}$`)

	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{
		"README.md",
		"RELEASE-CRITERIA.md",
		"CHANGELOG.md",
		filepath.Join("docs", "ROADMAP.md"),
	}
	for _, rel := range paths {
		b, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			t.Skipf("%s not readable from here: %v", rel, err)
			continue
		}
		// Markdown wraps prose across lines, and this project's own
		// negation ("...and **no\ncalibration corpus against real OpenAI
		// traffic exists.**") regularly lands the negating word on the
		// PREVIOUS line. Matching line by line would see only the half
		// after the wrap and misread a disclosure as a claim. Flattening
		// newlines to spaces is length-preserving, so byte offsets into the
		// flattened text are still valid offsets into the original for
		// computing a line number to report.
		text := string(b)
		flat := strings.ReplaceAll(text, "\n", " ")
		for _, loc := range bad.FindAllStringIndex(flat, -1) {
			before := flat[:loc[0]]
			if negation.MatchString(before) {
				continue
			}
			line := strings.Count(text[:loc[0]], "\n") + 1
			end := loc[1]
			if end > loc[0]+120 {
				end = loc[0] + 120
			}
			t.Errorf("RPL-C039 is NOT_MEASURED but %s:%d asserts it: %s", rel, line, strings.TrimSpace(flat[loc[0]:end]))
		}
	}

	// Positive control: the detector has to actually fire on the sentence it
	// exists to catch, or a passing run above proves nothing.
	if !bad.MatchString(`AstraRules() has been calibrated against real OpenAI traffic.`) {
		t.Fatal("the calibration-claim detector does not fire on a calibration claim")
	}
	if !bad.MatchString(`OpenAI calibration complete.`) {
		t.Fatal("the calibration-claim detector does not fire on an explicit completion claim")
	}

	// Negative control: the negation guard has to actually suppress the
	// disclosure sentence this project is required to carry, or the test
	// above is unrunnable against this project's own honest documentation.
	negatedLine := `Checked explicitly 2026-10-08: no calibration against real OpenAI traffic has been performed.`
	loc := bad.FindStringIndex(negatedLine)
	if loc == nil {
		t.Fatal("the calibration-claim detector does not fire on the negated sentence either; it cannot be exercising the negation guard")
	}
	if !negation.MatchString(negatedLine[:loc[0]]) {
		t.Fatal("the negation guard does not recognise this project's own honest disclosure sentence")
	}
}
