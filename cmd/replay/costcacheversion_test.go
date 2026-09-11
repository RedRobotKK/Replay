package main

import (
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/version"
)

// The cost index survives a change to what its numbers MEAN.
//
// costIndexKey already invalidates on a change to costUnit's shape, derived
// from the struct's own field tags, and its comment explains why a hand-bumped
// constant is "the same defect with a comment on it". That catches a field
// being added, removed or renamed.
//
// It does not catch a change to what a field CONTAINS. #168 changed cost from
// pricing one lane per transcript to pricing every lane — the corpus total moved
// from $3,787 to $10,667 — and no field changed name, so the key did not move
// and every machine with a warm index kept reporting the old figure. The
// correction was invisible to exactly the people who already had the tool.
//
// Nothing can detect a semantic change in general. What can be detected is that
// the binary is not the one that wrote the index, which covers every released
// upgrade. That is the case that matters: a developer with a dev build can
// delete the file, a user who upgraded cannot know they need to.

func TestCCV1_TheKeyCarriesTheBuildIdentity(t *testing.T) {
	key := costIndexKey()
	for _, want := range []string{version.Version, version.Commit} {
		if want == "" {
			continue
		}
		if !strings.Contains(key, want) {
			t.Errorf("the index key does not carry %q, so a binary that prices "+
				"differently reads back the previous one's figures:\n%s", want, key)
		}
	}
}

// CCV2: and it still carries everything it carried before.
//
// The build identity is added to the key, not substituted for it. A price table
// or rules change must still invalidate on a binary that did not change.
func TestCCV2_TheKeyStillCarriesTheSchemaAndTheTables(t *testing.T) {
	key := costIndexKey()
	for _, want := range []string{"replay.cost.v", "session", "costUsd"} {
		if !strings.Contains(key, want) {
			t.Errorf("the index key lost %q:\n%s", want, key)
		}
	}
}

// CCV3: two different builds do not share an index.
//
// Asserted through the key rather than through the file, because the property
// is that the key differs — what the cache does with a differing key is
// newCostCache's business and already has its own tests.
func TestCCV3_TwoBuildsDoNotShareAnIndex(t *testing.T) {
	first := costIndexKey()

	restore := version.Commit
	version.Commit = "0000000"
	defer func() { version.Commit = restore }()
	second := costIndexKey()

	if first == second {
		t.Error("two builds from different commits produce the same index key, so " +
			"one reads back the other's figures")
	}
}
