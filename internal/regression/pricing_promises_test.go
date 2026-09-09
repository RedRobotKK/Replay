package regression

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The three promises that constrain anything commercial, held as a check.
//
// docs/MONEY-PATH.md section 5 lists amending these as step 1 of 8, "strictly
// first, because every later step is a promise violation until it is done".
// They were amended on 2026-09-09. This is what stops them drifting back, and
// what stops a future paid capability shipping past them.
//
// The reason a test rather than a paragraph: a promise nobody checks is a
// paragraph, and this repository's whole differentiator is that its claims are
// enforced rather than asserted. cmd/replay/outbound_drift_test.go does the same
// job for what the binary may reach on the network.
//
// What each one is for:
//
//   - Sponsorship gates nothing. Funding buys a name in a file, never a
//     capability, and a sponsor tier that unlocked a feature would make every
//     other claim on the page unreadable.
//   - Nothing free becomes paid. This is the one a reader cannot verify for
//     themselves, because it is a promise about future releases rather than a
//     property of this one. It is therefore the one most worth pinning.
//   - The rules feed gates nothing. Subscribing buys maintenance of the price
//     table, and an expired subscription costs a fresher table and nothing else.
//     FUNDING.md already promised this; it said so in words that read wider.
func TestPricingPromisesAreStillMade(t *testing.T) {
	root := repoRoot(t)

	for _, c := range []struct {
		file    string
		require []string
		why     string
	}{
		{
			file: "SPONSORS.md",
			require: []string{
				"Sponsorship gates nothing",
				"nothing that is free today ever becomes paid",
			},
			why: "sponsorship must buy a name and never a capability",
		},
		{
			file: "FUNDING.md",
			require: []string{
				"Nothing in Replay is behind the\nfeed",
				"nothing that is free today ever becomes\npaid",
			},
			why: "the feed sells maintenance of the price table, never access to it",
		},
	} {
		b, err := os.ReadFile(filepath.Join(root, c.file))
		if err != nil {
			t.Fatalf("%s: %v", c.file, err)
		}
		body := string(b)
		for _, want := range c.require {
			if !strings.Contains(body, want) {
				t.Errorf("%s no longer says %q.\n%s.\nIf a paid capability is being "+
					"added, MONEY-PATH.md section 5 step 1 is the argument to answer "+
					"first: amend the documents before selling, not in the release "+
					"notes afterwards.", c.file, want, c.why)
			}
		}
	}
}

// The wider sentences must not come back.
//
// Both files once carried a promise scoped to sponsorship or to the feed in its
// author's mind, and to the whole product on the page. Neither was dishonest and
// both were unusable: a reader who found them while a paid capability existed
// would conclude the project broke its word, and would be right about the
// wording.
//
// Restoring either is easy to do by accident when editing nearby prose, which is
// exactly why this half exists. It fails on the old text, not on the idea.
func TestTheOverbroadPromisesStayRetired(t *testing.T) {
	root := repoRoot(t)

	for file, retired := range map[string]string{
		"SPONSORS.md": "Nothing is behind a tier",
		"FUNDING.md":  "Nothing in Replay is ever behind it",
	} {
		b, err := os.ReadFile(filepath.Join(root, file))
		if err != nil {
			t.Fatalf("%s: %v", file, err)
		}
		if strings.Contains(string(b), retired) {
			t.Errorf("%s has the retired sentence %q back. It reads as a promise "+
				"about the whole product and was meant about one part of it. The "+
				"narrower wording says the same true thing without the claim that "+
				"cannot be kept.", file, retired)
		}
	}
}
