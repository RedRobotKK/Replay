package observation

import (
	"encoding/json"
	"strings"
	"testing"
)

// tagBasis is a CLOSED ENUMERATION, and the reason is claim integrity.
//
// THE DEFECT THESE TESTS WERE WRITTEN AGAINST. tagBasis has exactly two
// legitimate values, observation.go:50 and :54. Build() already refuses
// anything else on the Observation path (observation.go:157), naming both. The
// corpus and calibration paths carried the same field and checked only that it
// was NON-EMPTY (corpus.go:362, calibration.go:128), so three artifacts carry
// this field and one of them validated it.
//
// WHY IT IS NOT MERELY INPUT HYGIENE. pool.go:344 reads
//
//	TagsAreIdentities: true                      // the default
//	if e.TagBasis != BasisAccount { ... = false } // only this falsifies it
//
// and TagNote renders two different published sentences from that boolean. So
// an unvalidated string does not just sit in a roster row looking untidy: it
// selects which claim the public document makes about what its own numbers
// mean. Measured before the fix, a single submission whose basis read
// "not-a-basis-at-all" published verbatim into the roster.
//
// WHAT THIS FIXES AND WHAT IT DOES NOT. Closing the enumeration stops an
// ARBITRARY string reaching a published artifact. It does NOT stop a
// contributor hand-editing "local" to "account", because "account" is
// legitimate and nothing in the payload can verify how a tag was derived.
// TestTB4 pins that residual defect rather than leaving it implied.

// tbCorpus is a submission valid in every respect but its basis.
func tbCorpus(basis, tag string) Corpus {
	return Corpus{
		Schema: CorpusSchema, TakenAt: "2026-09-29T18:00:00Z",
		Tasks: 3, TotalUSD: 1.5, RebilledUSD: 0.1, RebilledShare: 6.67,
		MedianTaskUSD: 0.5, PricedAt: "2026-09-07", RulesVersion: "anthropic-2026-09-01",
		SourceTag: tag, TagBasis: basis,
	}.Digested()
}

// TB1: a basis outside the enumeration never reaches a published document.
//
// The assertion is the whole chain, not just the refusal: rejected at Validate,
// refused by Pool.Add, and therefore absent from the marshalled roster. A test
// that stopped at "Validate returned an error" would not show that the value
// cannot be published, which is the property that matters.
func TestTB1_AnArbitraryTagBasisCannotReachAPublishedDocument(t *testing.T) {
	c := tbCorpus("not-a-basis-at-all", "abcdef0123456789")

	err := c.Validate()
	if err == nil {
		t.Fatal("a tagBasis outside the two-value enumeration validated. It would then " +
			"reach the published roster verbatim, and because pool.go:344 reads this " +
			"field to choose between two published sentences, it also selects what the " +
			"document claims its own numbers mean")
	}
	for _, want := range []string{BasisAccount, BasisLocal} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name the legitimate value %q, so a contributor "+
				"cannot tell what to put there: %v", want, err)
		}
	}

	p := NewPool("2026-09-29")
	if err := p.Add(c, "replay-corpus-x.json"); err == nil {
		t.Fatal("Pool.Add admitted a submission whose basis is outside the enumeration")
	}
	b, err := json.Marshal(p)
	if err == nil && strings.Contains(string(b), "not-a-basis-at-all") {
		t.Errorf("the value reached the published document:\n%s", b)
	}
}

// TB2: the enumeration is closed, not a blacklist.
//
// Both legitimate values pass independently, and the mutations around them all
// fail. The near-misses are the point: a check that accepted "accoun",
// "account " or "LOCAL" would be matching loosely rather than enumerating.
func TestTB2_TheEnumerationIsClosed(t *testing.T) {
	for _, tc := range []struct {
		basis string
		want  bool
	}{
		{BasisLocal, true},
		{BasisAccount, true},
		{"", false},
		{"accoun", false},
		{"account ", false},
		{" account", false},
		{"LOCAL", false},
		{"Local", false},
		{"zq7", false},
		{"local,account", false},
	} {
		name := tc.basis
		if name == "" {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			err := tbCorpus(tc.basis, "abcdef0123456789").Validate()
			if tc.want && err != nil {
				t.Errorf("the legitimate basis %q was refused: %v", tc.basis, err)
			}
			if !tc.want && err == nil {
				t.Errorf("the basis %q was accepted; the enumeration is %q and %q and "+
					"nothing else", tc.basis, BasisAccount, BasisLocal)
			}
		})
	}
}

// TB3: the calibration artifact has the same rule.
//
// It carries the same field for the same reason and validated it the same way,
// which is to say not at all.
func TestTB3_TheCalibrationArtifactEnforcesTheSameEnumeration(t *testing.T) {
	build := func(basis string) Calibration {
		return Calibration{
			Schema: CalibrationSchema, TakenAt: "2026-09-29T18:00:00Z",
			RulesVersion: "anthropic-2026-09-01",
			Models:       []ModelCalibrationRow{{Model: "claude-opus-5", Sessions: 1, Compared: 1, Matched: 1}},
			SourceTag:    "abcdef0123456789", TagBasis: basis,
		}.Digested()
	}
	if err := build(BasisLocal).Validate(); err != nil {
		t.Errorf("a legitimate calibration basis was refused: %v", err)
	}
	if err := build("zq7").Validate(); err == nil {
		t.Error("a calibration carrying a basis outside the enumeration validated")
	}
}

// TB4: the declared basis cannot change the strength of the published claim.
//
// THIS TEST REPLACES ONE THAT PINNED THE DEFECT. Until 2026-09-29 it recorded
// that a self-declared "account" basis upgraded the published sentence, and
// said in its own comment to delete it with the fix. That is what happened.
//
// The invariant now is the one that matters: changing tagBasis from "local" to
// "account", which is the entire attack, cannot increase what the document
// asserts. The basis is still a validated enumeration and still travels in the
// roster; it simply no longer selects a claim.
func TestTB4_TheDeclaredBasisCannotStrengthenTheClaim(t *testing.T) {
	note := func(basis, tag string) string {
		p := NewPool("2026-09-29")
		if err := p.Add(tbCorpus(basis, tag), "replay-corpus-"+tag+".json"); err != nil {
			t.Fatalf("a legitimate submission was refused: %v", err)
		}
		tot, err := p.Totals()
		if err != nil {
			t.Fatal(err)
		}
		return TagNote(tot)
	}

	local := note(BasisLocal, "aaaaaaaaaaaaaaaa")
	account := note(BasisAccount, "aaaaaaaaaaaaaaaa")

	if local != account {
		t.Errorf("the declared basis still changes the published claim:\n  local:   %q\n  account: %q",
			local, account)
	}
	if !strings.Contains(local, "MACHINES AT MOST") {
		t.Errorf("the conservative wording was lost: %q", local)
	}
	if strings.Contains(account, "distinct provider accounts") {
		t.Errorf("the provider-account claim survives: %q", account)
	}
}
