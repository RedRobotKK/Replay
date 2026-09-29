package observation

import (
	"encoding/json"
	"strings"
	"testing"
)

// What the pool KNOWS, versus what the contributor SAYS.
//
// This began as a RED evidence test and is now the regression for the fix it
// prompted. It holds the boundary in both forms a claim can take: the English
// sentence a person reads, and the JSON field a program parses.
//
// THE CLAIM UNDER TEST. TagNote publishes one of two sentences, selected by
// TagsAreIdentities, which pool.go:344 computes from the contributor-supplied
// tagBasis field alone:
//
//	account -> "N submissions from N distinct provider accounts"
//	local   -> "...a count of MACHINES AT MOST ... anyone can mint unlimited ones"
//
// THE EVIDENCE PROBLEM. 631698b closed the enumeration, so a basis outside
// {account, local} is refused. It cannot close this, because "account" IS
// legitimate. A tag is HMAC(campaign, identity) truncated to 16 hex characters
// (observation.go derive). Given only the tag, the pool cannot recompute that
// HMAC without the identity, and the identity is precisely the thing the
// privacy design refuses to transmit. So the basis is a self-declaration and
// there is no artifact anywhere in the payload that could contradict it.
//
// AND IT IS WORSE THAN THAT, which is the finding this test exists to record:
// AccountTag has NO production caller. Every shipped contribution path calls
// LocalTag (contribute.go:82, :241, :387). There is no --account flag and no
// account-identifier input anywhere in cmd/replay. So the shipped binary CANNOT
// produce tagBasis="account" at all, and the stronger published sentence is
// reachable only by hand-editing a file.

// TP1: a self-declared basis produces no provider-account claim, in either form.
//
// The two submissions below are byte-identical except for the basis word. One
// of them causes the public document to assert something about provider
// accounts that nothing in it establishes.
func TestTP1_ASelfDeclaredBasisCannotProduceAnAccountClaim(t *testing.T) {
	// The strongest artifact a contributor can actually produce today. Nothing
	// here is forged: this is what `replay cost --contribute` writes, with one
	// word changed by hand afterwards.
	honest := tbCorpus(BasisLocal, "aaaaaaaaaaaaaaaa")
	edited := tbCorpus(BasisAccount, "aaaaaaaaaaaaaaaa")

	// The ONLY difference between them.
	if honest.SourceTag != edited.SourceTag {
		t.Fatal("the fixtures differ in more than the basis; the test would not be " +
			"isolating the declaration")
	}

	note := func(c Corpus) string {
		p := NewPool("2026-09-29")
		if err := p.Add(c, "replay-corpus-x.json"); err != nil {
			t.Fatalf("a submission the shipped binary would write was refused: %v", err)
		}
		tot, err := p.Totals()
		if err != nil {
			t.Fatal(err)
		}
		return TagNote(tot)
	}

	// The MACHINE-READABLE form of the same claim. Fixing only the sentence
	// would leave `"tagsAreIdentities": true` in the published JSON, which is
	// the identical assertion in the form a consumer parses rather than reads.
	p := NewPool("2026-09-29")
	if err := p.Add(edited, "replay-corpus-x.json"); err != nil {
		t.Fatalf("a syntactically valid submission was refused: %v", err)
	}
	doc, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(doc), `"tagsAreIdentities":true`) {
		t.Errorf("the published document asserts tagsAreIdentities on a self-declaration:\n%s", doc)
	}

	if strings.Contains(note(edited), "distinct provider accounts") {
		t.Errorf("the pool published a claim about PROVIDER ACCOUNTS on the strength of "+
			"one word supplied by the contributor.\n"+
			"  same tag, basis %q -> %q\n"+
			"  same tag, basis %q -> %q\n"+
			"No artifact in the payload binds a tag to an account: the tag is "+
			"HMAC(campaign, identity) and the pool does not hold the identity, by design. "+
			"AccountTag also has no production caller, so the shipped binary cannot "+
			"produce this basis at all and only a hand edit reaches it.",
			BasisLocal, note(honest), BasisAccount, note(edited))
	}
}
