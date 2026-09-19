package transcript

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The billing gate agrees with a measurement taken outside it.
//
// Skipped where there is no local Codex corpus, which is most machines and
// every CI runner. Where there is one, this is the check that the gate
// describes the world rather than the fixtures.
//
// It used to assert a conservation law: that a session's summed deltas equal
// its final cumulative wherever it never compacted. That law is not a property
// of the format, and saying so cost this reader a 34% over-count. Codex emits
// TokenCountEvent as a BROADCAST of session state, on a rate-limit update and
// a context estimate as well as on a completed response, and `last_token_usage`
// is the most recent append held as state rather than a delta belonging to the
// event carrying it. A broadcast that follows no append repeats the previous
// snapshot, and summing the field counts that response again. On this machine
// three sessions did that, one of them 1,690 times, and the law read their
// arithmetic as a rebase that never happened: none of the three carries a
// single context_compacted event.
//
// So what is checked now is the gate, not the law. Every session must either
// establish a billing basis on evidence, or be refused for a reason this
// reader can name.
func TestCodexBillingBasisHoldsOnTheLocalCorpus(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}
	root := filepath.Join(home, ".codex")
	if _, err := os.Stat(root); err != nil {
		t.Skip("no local Codex corpus on this machine")
	}
	var files []string
	_ = filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil //nolint:nilerr // an unreadable subtree is not this test's business
		}
		if strings.HasPrefix(filepath.Base(p), "rollout-") && strings.HasSuffix(p, ".jsonl") {
			files = append(files, p)
		}
		return nil
	})
	if len(files) == 0 {
		t.Skip("no rollout files")
	}

	var billable, reEmitting, unreadable, compacted, withQuota int
	for _, f := range files {
		s, err := ParseCodexFile(f)
		if err != nil {
			t.Errorf("%s: %v", filepath.Base(f), err)
			continue
		}
		name := filepath.Base(f)
		if s.Quota != nil {
			withQuota++
		}
		if s.Rebased {
			compacted++
		}
		if s.reEmitted > 0 {
			reEmitting++
		}
		if s.unreadableUsage > 0 {
			unreadable++
		}

		// A billed figure must rest on every condition that admits it. This is
		// the whole gate, restated here from the contract rather than read back
		// out of basisEstablished, so an implementation that quietly widens
		// itself fails instead of agreeing with itself.
		want := s.reEmitted == 0 && s.unreadableUsage == 0 &&
			(s.Rebased || s.Billed.Total() == s.Reported.Total())
		if s.BasisEstablished != want {
			t.Errorf("%s: basis established = %v, want %v (re-emitted %d, unreadable %d, "+
				"rebased %v, billed %d, reported %d)", name, s.BasisEstablished, want,
				s.reEmitted, s.unreadableUsage, s.Rebased, s.Billed.Total(), s.Reported.Total())
		}
		if s.BasisEstablished && !s.Rebased && s.Billed.Total() != s.Reported.Total() {
			t.Errorf("%s: billed %d against a provider cumulative of %d and still claimed a "+
				"basis; an ordinary session that does not reconcile has no corroboration",
				name, s.Billed.Total(), s.Reported.Total())
		}
		if s.BasisEstablished {
			billable++
		}
	}
	t.Logf("corpus: %d sessions · %d billable · %d re-emitting · %d with unreadable usage · "+
		"%d compacted · %d carry quota",
		len(files), billable, reEmitting, unreadable, compacted, withQuota)
}
