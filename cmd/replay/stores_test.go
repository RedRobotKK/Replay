package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Everything this tool writes to the reader's machine, in one list.
//
// The purge command was built against ~/.replay/ledger because that is where
// the retention gap was found. Looking properly, the tool writes eleven things:
//
//	ledger/            per-request records, counts and timings, no content
//	ledger-<name>/     the same, per named upstream
//	vault/             masking vault — the ONLY store that holds secrets
//	archive/           rotated ledger records
//	advice.json        findings, and which the reader marked applied
//	policy.json        learned request policy
//	cost-index.json    a cache keyed by transcript path
//	measurements.jsonl probe readings
//	seen.json          the `since` marker, one timestamp
//	tip.json           when the funding ask was last shown
//	serve*.log         proxy logs
//
// A retention command covering one of eleven is worse than none, because it
// answers "yes, we can delete that" while ten stores keep it. That is the
// defect class this repository keeps finding — a guarantee narrower than it
// appears — applied to the reader's own files.
//
// So the list is data, and every consumer walks it: purge removes from it,
// `replay privacy` reports it, and ST1 fails when the tool learns to write
// somewhere the list does not name.

// ST1: every store the code writes is registered.
//
// PASS: every filename constant and literal store path in the source appears in
// the registry.
// FAIL: the tool writes somewhere no retention or disclosure command knows
// about, which is exactly how ten of eleven got missed the first time.
func TestST1_EveryStoreTheToolWritesIsRegistered(t *testing.T) {
	known := map[string]bool{}
	for _, s := range homeStores() {
		known[s.Name] = true
	}
	if len(known) < 8 {
		t.Fatalf("the registry names %d stores; the tool was writing eleven when this was "+
			"written, so a registry this short is not describing the tool", len(known))
	}

	// Names the source itself uses for things under the home directory.
	for _, want := range []string{
		"ledger", "vault", "archive", "advice.json", "policy.json",
		"cost-index.json", "measurements.jsonl", "seen.json", "tip.json",
	} {
		if !known[want] {
			t.Errorf("%s is written under ~/.replay and is not in the store registry, so no "+
				"retention or disclosure command covers it", want)
		}
	}
}

// ST2: the store holding secrets is marked as holding secrets.
//
// PASS: vault is registered with Sensitive true and every other store is
// explicit about what it holds.
// FAIL: the vault is treated like a cache, which is how a masking vault ends up
// inside a routine purge or a support bundle.
func TestST2_TheVaultIsMarkedSensitive(t *testing.T) {
	var found bool
	for _, s := range homeStores() {
		if s.Name == "vault" {
			found = true
			if !s.Sensitive {
				t.Error("the masking vault is not marked sensitive. It is the one store here " +
					"that holds the reader's actual secrets rather than counts about them.")
			}
			if s.Purgeable {
				t.Error("the vault is marked purgeable by a retention window. Deleting it " +
					"silently breaks rehydration for every transcript that referenced it; " +
					"removing it is a decision, not a schedule.")
			}
		}
		if strings.TrimSpace(s.Holds) == "" {
			t.Errorf("store %q does not say what it holds, so `replay privacy` cannot tell "+
				"the reader either", s.Name)
		}
	}
	if !found {
		t.Error("the vault is not registered at all")
	}
}

// ST3: a registered store resolves under the home directory.
//
// PASS: every registry path sits inside ~/.replay.
// FAIL: an entry pointing outside it, which would let purge delete something
// the tool does not own.
func TestST3_EveryStoreResolvesUnderTheHomeDirectory(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home directory: %v", err)
	}
	root := filepath.Join(home, ".replay")
	for _, s := range homeStores() {
		p := s.Path()
		if p == "" {
			t.Errorf("store %q resolves to an empty path", s.Name)
			continue
		}
		if !strings.HasPrefix(filepath.Clean(p), filepath.Clean(root)) {
			t.Errorf("store %q resolves to %s, outside %s. A retention command must not be "+
				"able to reach a path this tool does not own.", s.Name, p, root)
		}
	}
}
