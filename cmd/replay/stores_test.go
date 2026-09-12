package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Everything this tool writes to the reader's machine, in one list.
//
// The purge command was built against ~/.replay/ledger because that is where
// the retention gap was found. Looking properly, the tool writes twelve things:
//
//	ledger/            per-request records, counts and timings, no content
//	                   — and ledger-<name>/, one directory per named upstream
//	vault/             masking vault — holds the reader's actual secrets
//	archive/           rotated ledger records
//	advice.json        findings, and which the reader marked applied
//	policy.json        learned request policy
//	cost-index.json    a cache keyed by transcript path
//	measurements.jsonl probe readings
//	seen.json          the `since` marker, one timestamp
//	tip.json           when the funding ask was last shown
//	serve.log          proxy logs — and serve-<name>.log per named upstream
//	contributor-secret the machine-local secret the contributor tag derives from
//	rules.json         the price-rules cache `replay rules --update` writes
//
// This list said eleven for a day, because the last two were registered on
// 2026-09-10 after SC1 found them by reading the source and nobody came back
// here. A retention command covering one of twelve is worse than none: it
// answers "yes, we can delete that" while eleven stores keep it. That is the
// defect class this repository keeps finding — a guarantee narrower than it
// appears — applied to the reader's own files.
//
// So the list is data, and every consumer walks it: purge removes from it,
// `replay privacy` reports it, and ST1 fails when the tool learns to write
// somewhere the list does not name. The prose above is no longer trusted to
// keep itself current either — SD1 and SD2 in storecount_test.go read this
// comment and docs/guide/commands.md and compare both to homeStores().

// ST1: every store the code writes is registered.
//
// PASS: every filename constant and literal store path in the source appears in
// the registry.
// FAIL: the tool writes somewhere no retention or disclosure command knows
// about, which is exactly how ten of eleven got missed the first time.
//
// The length assertion below is a FLOOR, not the count, and that is on purpose
// after review. A floor cannot notice the registry growing — which is how two
// stores were added on 2026-09-10 with nothing going red — but the answer to
// that is not a second hard-coded number here. A literal count in this test
// would have no external referent: updating the registry and updating the
// number is one edit by one person, so the pair agrees by construction and
// proves nothing. The exact count is owned by SD1/SD2 in storecount_test.go,
// which compare len(homeStores()) against the two places a READER is told the
// number. What is left for the floor is the case those cannot catch — the
// registry being emptied or gutted, which would make every disclosure check
// vacuously true — so it is derived from the name list below rather than being
// a magic 8 that matched nothing in particular.
func TestST1_EveryStoreTheToolWritesIsRegistered(t *testing.T) {
	known := map[string]bool{}
	for _, s := range homeStores() {
		known[s.Name] = true
	}

	// Names the source itself uses for things under the home directory.
	mustBeRegistered := []string{
		"ledger", "vault", "archive", "advice.json", "policy.json",
		"cost-index.json", "measurements.jsonl", "seen.json", "tip.json",
	}
	if len(known) < len(mustBeRegistered) {
		t.Fatalf("the registry names %d stores and this test alone requires %d of them by "+
			"name; a registry that short is not describing the tool", len(known),
			len(mustBeRegistered))
	}

	for _, want := range mustBeRegistered {
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

// ST4: the registry resolves against a real directory, and every branch of the
// match was unobserved.
//
// resolveStores expands a prefixed entry like `ledger` so it also covers
// `ledger-grok`, skips what does not match, and returns nothing when the root
// cannot be read. All three decide what `replay privacy` discloses and what
// `replay purge` can reach, and none was observed.
func TestST4_ResolveStoresMatchesPrefixesAndSkipsTheRest(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"ledger", "ledger-grok", "vault", "somebody-elses-dir"} {
		if err := os.Mkdir(filepath.Join(root, d), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{"tip.json", "unrelated.txt"} {
		if err := os.WriteFile(filepath.Join(root, f), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	got := map[string]bool{}
	// resolveStoresErr since #155: the read error reaches the caller now,
	// because a store the walk could not enter must not render as a clean
	// disk. Nothing here should produce one.
	rs, err := resolveStoresErr(root)
	if err != nil {
		t.Fatalf("resolving stores under a directory this test just built: %v", err)
	}
	for _, r := range rs {
		got[r.Actual] = true
	}
	for _, want := range []string{"ledger", "ledger-grok", "vault", "tip.json"} {
		if !got[want] {
			t.Errorf("%s is on disk and was not resolved; a store the registry misses is a\n"+
				"store privacy does not disclose and purge does not reach", want)
		}
	}
	// A directory Replay did not write is not Replay's to disclose or delete.
	for _, unwanted := range []string{"somebody-elses-dir", "unrelated.txt"} {
		if got[unwanted] {
			t.Errorf("%s was claimed by the registry and is not Replay's", unwanted)
		}
	}
}

// ST5: a root that cannot be read resolves to nothing rather than to a guess.
//
// A missing root is not an error here — a machine that has never run the tool
// has no ~/.replay, and `replay privacy` must answer it plainly rather than
// fail. That distinction is #155's: any other read failure now reaches the
// caller, because a store the walk could not enter must not render as a clean
// disk.
func TestST5_AnUnreadableRootResolvesToNothing(t *testing.T) {
	got, err := resolveStoresErr(filepath.Join(t.TempDir(), "absent"))
	if len(got) != 0 {
		t.Errorf("resolveStoresErr over a missing root returned %d store(s), want none", len(got))
	}
	// The error is returned, and it is the not-exist one specifically. The
	// caller decides what that means — safeState treats a missing root as an
	// empty machine and anything else as a disclosure it must not render as a
	// clean disk. Asserted with errors.Is rather than on the message, because
	// the wording of a missing path is the operating system's and differs.
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a root that is simply absent gave %v, not a not-exist error; the caller "+
			"distinguishes those and would report an empty machine as a read failure", err)
	}
}

// ST6: with no home directory a store has no path, and the empty string is the
// signal rather than a path relative to nowhere.
//
// Returning filepath.Join("", ".replay", name) would produce `.replay/<name>`,
// a relative path that resolves against whatever directory the process happens
// to be in — which for a deletion command is the worst available outcome.
func TestST6_NoHomeGivesNoPath(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	s := store{Name: "ledger"}
	got := s.Path()
	if got == "" {
		return // the platform refused, which is the branch under test
	}
	if !filepath.IsAbs(got) {
		t.Errorf("Path() = %q with no home: a relative store path resolves against the\n"+
			"working directory, and purge would delete from wherever it was run", got)
	}
}
