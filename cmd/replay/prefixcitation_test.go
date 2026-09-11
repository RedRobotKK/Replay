package main

import (
	"bytes"
	"strings"
	"testing"
)

// PX7: the invalidation says the mechanism is general, and says whose
// measurement it is.
//
// Pinning a citation in a test is unusual, so here is why this one earns it.
//
// `replay prefix` tells a reader that changing their tool set voids the cached
// prefix. That claim rests on this repository's own measurement — the 30-lane
// trial of 2026-09-06 recorded in internal/proxy/causedetail.go, where
// system_bytes never moved once and every real prefix change was the tool SET
// changing. From the reader's side there is no way to tell that from their own
// setup misbehaving, and the difference decides what they do next: a quirk gets
// worked around, a general mechanism gets designed for.
//
// So the line has two jobs and both are asserted: name a source a reader can
// check, and keep it in its place. The citation corroborates; the evidence is
// the trial. A cited paper standing in for a local measurement is the exact
// substitution this repository keeps catching elsewhere.
//
// The arXiv number is pinned because getting it wrong is the failure mode this
// test exists against. A search summary first attributed that sentence to a
// different paper; only fetching both settled which one carries it. The wrong
// candidates are named below so a later edit that swaps one in fails here
// rather than shipping a citation that does not say what it is cited for.
func TestPX7_TheInvalidationNamesItsCorroboration(t *testing.T) {
	const (
		paper  = "arXiv:2608.22708"
		wrong1 = "arXiv:2607.15516" // cache-aware prompt compression: tools are cached, not changing
		wrong2 = "arXiv:2601.06007" // don't break the cache: caching's value, not tool churn
	)

	dir := t.TempDir()
	before := writeFile(t, dir, "before.json", mcpTwo)
	after := writeFile(t, dir, "after.json", mcpThree)

	var stdout, stderr bytes.Buffer
	if err := run([]string{"prefix", "--before", before, "--after", after}, &stdout, &stderr); err == nil {
		t.Fatal("the fixture no longer invalidates the prefix, so this asserts nothing")
	}
	out := stdout.String() + stderr.String()

	if !strings.Contains(out, paper) {
		t.Errorf("the invalidation does not cite %s, so a reader cannot tell a general\n"+
			"mechanism from their own setup misbehaving:\n%s", paper, out)
	}
	for _, w := range []string{wrong1, wrong2} {
		if strings.Contains(out, w) {
			t.Errorf("%s is cited for the tool-definition claim; it does not carry that\n"+
				"sentence. %s does. Both were offered by a search summary for the same\n"+
				"claim, and only fetching them told them apart.", w, paper)
		}
	}
	if !strings.Contains(out, "Measured here") {
		t.Errorf("the citation does not say the measurement is this repository's own.\n"+
			"The paper corroborates; the 30-lane trial is the evidence, and a citation\n"+
			"that reads as the source demotes it:\n%s", out)
	}
}
