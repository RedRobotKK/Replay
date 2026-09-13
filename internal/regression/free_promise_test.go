package regression

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// FC-FP. The free promise is a sentence, and this is what keeps it one.
//
// SPONSORS.md promises that nothing free today ever becomes paid, and that any
// paid capability will be something that does not exist today. Those are the
// two sentences the whole commercial path is built on, and until 2026-09-13
// nothing checked that they were still there.
//
// The specific risk is not somebody deleting them in bad faith. It is that a
// promise covering "everything free today" protects a capability nobody wrote
// down only for as long as somebody remembers it exists. `replay cost
// --max-avoidable-usd` fails a build on measured avoidable spend, it is free,
// and it was named in no promise document at all while a paid `replay gate`
// was being designed in the same repository. Those two are easy to confuse,
// and if the paid one had shipped as a better version of the free one the
// promise would have been broken by a sequence of individually reasonable
// commits.
//
// So the capability is named, and this test holds the naming in place.

// freeForever is every capability the promise documents name as free and
// staying free. Adding a row is a decision, and removing one is a promise
// broken.
var freeForever = []string{
	"replay cost --max-avoidable-usd",
}

// FC-FP1: both promise sentences are present, in the file that makes them.
//
// PASS: SPONSORS.md carries the pair.
// FAIL: the commercial path rests on a sentence that is no longer written down,
// and every document citing it is citing nothing.
func TestFCFP1_ThePromiseSentencesAreStillThere(t *testing.T) {
	s := readAt(t, "SPONSORS.md")

	for _, want := range []string{
		"nothing that is free today ever becomes paid",
		"something that does not exist today",
	} {
		if !strings.Contains(strings.ToLower(s), strings.ToLower(want)) {
			t.Errorf("SPONSORS.md no longer contains %q.\n"+
				"This is the sentence docs/MONEY-PATH.md, FUNDING.md and ADR-0023 all "+
				"reason from. Removing it is a commercial decision, not an edit.", want)
		}
	}
}

// FC-FP2: every capability named free-forever is still named.
//
// The list above is short on purpose. Lengthening it should require an
// argument, and shortening it is the thing this test exists to refuse.
func TestFCFP2_EveryFreeForeverCapabilityIsStillNamed(t *testing.T) {
	sponsors := readAt(t, "SPONSORS.md")
	funding := readAt(t, "FUNDING.md")

	for _, cap := range freeForever {
		inSponsors := strings.Contains(sponsors, cap)
		inFunding := strings.Contains(funding, cap)
		if !inSponsors && !inFunding {
			t.Errorf("%q is promised free forever and is named in neither SPONSORS.md nor "+
				"FUNDING.md.\nAn unnamed capability is protected only while somebody "+
				"remembers it exists, and a paid gate is being designed next to this one.", cap)
		}
	}
}

// FC-FP3: the distinction between the free gate and a paid one is stated.
//
// This is the load-bearing one. `replay gate` is unbuilt, and the argument for
// it being sellable at all is that it does something the free gate does not.
// SPONSORS.md states that in one sentence. If the sentence goes, the argument
// went with it, and the next person to write the gate has no written reason to
// keep it distinct.
//
// It checks for the two words the distinction turns on rather than the whole
// paragraph, so that the prose can be improved without failing, and cannot be
// hollowed out without failing.
func TestFCFP3_TheFreeAndPaidGatesAreDistinguished(t *testing.T) {
	s := strings.ToLower(readAt(t, "SPONSORS.md"))

	if !strings.Contains(s, "retrospective") || !strings.Contains(s, "prospective") {
		t.Error("SPONSORS.md no longer distinguishes the free gate from a paid one.\n" +
			"The free gate refuses on waste that already happened; a paid gate would " +
			"price a configuration before the requests it describes have been made. " +
			"Without that written down, a paid gate that is merely a better free gate " +
			"breaks the promise above and nothing would report it.")
	}
}

func readAt(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), name))
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return string(b)
}
