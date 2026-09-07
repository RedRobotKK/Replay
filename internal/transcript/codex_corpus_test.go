package transcript

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The reader agrees with a measurement taken outside it.
//
// Skipped where there is no local Codex corpus, which is most machines and
// every CI runner. Where there is one, this is the check that the reader
// describes the world rather than the fixture: the conservation law is that a
// session's summed deltas equal its final cumulative, and it holds on every
// session that never compacted. A disagreement on an uncompacted session means
// the reader, the log, or the law is wrong, and any of the three is worth
// stopping for.
func TestCodexConservationHoldsOnTheLocalCorpus(t *testing.T) {
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

	var agreed, compacted, withQuota int
	for _, f := range files {
		s, err := ParseCodexFile(f)
		if err != nil {
			t.Errorf("%s: %v", filepath.Base(f), err)
			continue
		}
		if s.Quota != nil {
			withQuota++
		}
		if s.Rebased {
			compacted++
			continue // the cumulative was rebased; the law does not apply
		}
		if s.Billed.Total() == s.Reported.Total() {
			agreed++
			continue
		}
		t.Errorf("%s: summed deltas %d != final cumulative %d with no compaction; "+
			"either the reader is wrong or this format has a second way to rebase",
			filepath.Base(f), s.Billed.Total(), s.Reported.Total())
	}
	t.Logf("corpus: %d sessions · %d reconcile · %d compacted (law suspended) · %d carry quota",
		len(files), agreed, compacted, withQuota)
}
