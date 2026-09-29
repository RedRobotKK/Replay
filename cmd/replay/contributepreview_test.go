package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// The contribution is shown, not described.
//
// The note already tells a contributor what the file does and does not carry.
// A promise about a file is not the file. The bargain this project asks a
// person to accept is "publish this artifact", and the only version of that
// ask which respects them is one where the artifact is on the screen when they
// decide, rather than a path they have to go and cat before they can judge it.
//
// It is also the cheapest possible privacy control. A sentence saying "no
// prompts" is a claim the reader has to trust; twenty lines of JSON with no
// prompts in them is a claim they can check in the time it takes to read it.

// PV1: the contribution note contains the artifact itself.
//
// PASS: every field of the written corpus appears in the note.
// FAIL: the note names a path and the contributor has to go and find it.
func TestPV1_TheNoteShowsTheArtifactItself(t *testing.T) {
	body := []byte(`{"schema":"replay.corpus.v2","takenAt":"2026-09-29T18:00:00Z",` +
		`"tasks":3,"totalUsd":1.5,"rebilledUsd":0.1,"rebilledShare":6.67,` +
		`"medianTaskUsd":0.5,"pricedAt":"2026-09-07","rulesVersion":"anthropic-2026-09-01",` +
		`"sourceTag":"abcdef0123456789","tagBasis":"local","digest":"deadbeef"}`)

	got := corpusContributionNote("replay-corpus-x.json", nil, body)

	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	if len(m) == 0 {
		t.Fatal("the fixture has no fields; this guard proves nothing")
	}
	for k := range m {
		if !strings.Contains(got, k) {
			t.Errorf("the note does not show %q, so the contributor cannot see what "+
				"they would be publishing without opening the file:\n%s", k, got)
		}
	}
	for _, want := range []string{"1.5", "6.67", "abcdef0123456789"} {
		if !strings.Contains(got, want) {
			t.Errorf("the note does not show the value %q:\n%s", want, got)
		}
	}
}

// PV2: the shown artifact is the file, not a re-rendering of it.
//
// A preview built by formatting the struct a second time is a second
// implementation of the payload, free to disagree with what was written. The
// contributor would then be approving one thing and publishing another, which
// is worse than showing nothing.
func TestPV2_WhatIsShownIsTheBytesThatWereWritten(t *testing.T) {
	body := []byte(`{"schema":"replay.corpus.v2","tasks":7,"digest":"abc"}`)
	got := corpusContributionNote("replay-corpus-x.json", nil, body)
	if !strings.Contains(got, string(body)) {
		t.Errorf("the note does not contain the written bytes verbatim, so what the "+
			"contributor approves is not what gets published:\n%s", got)
	}
}

// PV3: the note still says nothing was sent, and does not claim anonymity.
//
// "Anonymous" is a property this project has not established: sourceTag is a
// stable per-machine identifier by construction, and a spend trajectory across
// repeated submissions is a correlate. Saying "nothing was sent" is checkable.
// Saying "anonymous" is not, and the difference is the whole posture.
func TestPV3_TheNoteClaimsNothingItHasNotEstablished(t *testing.T) {
	got := corpusContributionNote("replay-corpus-x.json", nil, []byte(`{"tasks":1}`))
	if !strings.Contains(got, "Nothing was sent") {
		t.Errorf("the note no longer says nothing was sent:\n%s", got)
	}
	for _, banned := range []string{"anonymous", "anonymised", "anonymized", "untraceable", "tamper-proof"} {
		if strings.Contains(strings.ToLower(got), banned) {
			t.Errorf("the note claims %q, which this project has not established. "+
				"sourceTag is a stable per-machine identifier and repeated submissions "+
				"are a time series:\n%s", banned, got)
		}
	}
}
