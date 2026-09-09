package main

import (
	"bytes"
	"strings"
	"testing"
)

// What the flat-seat paragraph is allowed to assert.
//
// `replay cost` tells a subscriber that re-billed tokens are "rate-limit budget
// spent on nothing". This repository measured that exact question and published
// the answer as a null: matched cold-write and warm-read arms, 3.09M tokens,
// and the utilisation counter moved zero steps (README.md:228-235).
//
// So the product asserted on the reader's screen the thing the README refuses
// to assert two clicks away, and the reader has no way to know which one the
// evidence supports. It is not a lie — nobody knew it was contradicted — but it
// is a claim with a measurement behind it that points the other way, which is
// the one kind of sentence this project has said repeatedly it will not print.
//
// The rule this encodes is narrow and worth stating exactly: cost may say what
// re-billed tokens ARE, because that is arithmetic from the transcript. It may
// not say what they COST a subscriber, because that is a quota measurement and
// the only one anyone has run came back null.
//
// This is also the operator's standing instruction — no dollars, and no quota
// consequences, until the quota measurement exists.

// FS1: the flat-seat paragraph renders, so FS2 is not guarding dead text.
//
// Without this, deleting the whole paragraph would make FS2 pass. That is the
// failure mode of every "must not contain" test, and it is why this one comes
// first.
func TestFS1_TheFlatSeatParagraphIsReached(t *testing.T) {
	out := renderCostReport(t)
	if !strings.Contains(out, "subscription seat") {
		t.Fatalf("the flat-seat paragraph did not render, so FS2 asserts nothing:\n%s", out)
	}
	if !strings.Contains(out, "re-billed") {
		t.Errorf("no re-billed tokens in this corpus, so the paragraph is not the one "+
			"under test:\n%s", out)
	}
}

// FS2: it does not assert a rate-limit consequence that was measured as null.
func TestFS2_NoUnmeasuredRateLimitClaim(t *testing.T) {
	out := renderCostReport(t)
	for _, forbidden := range []string{
		"rate-limit budget spent on nothing",
		"budget spent on nothing",
	} {
		if strings.Contains(out, forbidden) {
			t.Errorf("cost asserts %q as fact. The titration in README.md:228-235 measured "+
				"that question over 3.09M tokens and the utilisation counter moved zero "+
				"steps, published as a null. The product must not claim what the evidence "+
				"declined to.", forbidden)
		}
	}
}

// FS3: if it mentions the rate-limit question at all, it names the null.
//
// Silence would also satisfy FS2, and silence is worse than it looks: a
// subscriber's whole reason to care about re-billed tokens is what they cost
// them, so leaving the question out invites them to assume the answer. Saying
// it was measured and came back null is both honest and more useful.
func TestFS3_TheNullIsNamedIfTheQuestionIsRaised(t *testing.T) {
	out := renderCostReport(t)
	lower := strings.ToLower(out)
	if !strings.Contains(lower, "rate limit") && !strings.Contains(lower, "rate-limit") {
		t.Skip("cost does not raise the rate-limit question; FS2 is the operative guard")
	}
	for _, want := range []string{"measured", "not"} {
		if !strings.Contains(lower, want) {
			t.Errorf("the rate-limit question is raised without saying it was measured and "+
				"came back null; %q is missing:\n%s", want, out)
		}
	}
}

func renderCostReport(t *testing.T) string {
	t.Helper()
	var out, errb bytes.Buffer
	if err := run([]string{"cost", "../../internal/transcript/testdata"}, &out, &errb); err != nil {
		t.Fatalf("cost failed: %v (stderr %q)", err, errb.String())
	}
	return out.String()
}
