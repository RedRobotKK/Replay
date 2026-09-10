package observation

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func sampleCalibration() Calibration {
	return Calibration{
		Schema:         CalibrationSchema,
		TakenAt:        "2026-09-10T05:00:00Z",
		RulesVersion:   "anthropic-2026-09-01",
		ClientVersions: []string{"2.1.257", "2.1.260"},
		Models: []ModelCalibrationRow{{
			Model: "claude-opus-5", Sessions: 74, Compared: 28400, Matched: 27800,
			RuleMinPrefix: 512, LargestUncached: 0, SmallestCached: 13745,
		}, {
			Model: "claude-opus-4-8", Sessions: 3, Compared: 1200, Matched: 1150,
			RuleMinPrefix: 1024, LargestUncached: 0, SmallestCached: 20457, Stale: true,
		}},
		BreakCauses: map[string]int{"cache expired (gap longer than the TTL)": 49},
		SourceTag:   "a3f19c02b7e4d581",
		TagBasis:    "local",
	}.Digested()
}

// The calibration report is what ADR-0007 specified as the unit of
// contribution, and it is not what shipped.
//
// "replay corpus emits session id prefixes, client version, request counts,
// match rates, fit parameters, prefix bounds and break causes... A contribution
// is that report and nothing else." What shipped instead was five money figures
// — ADR-0008's credibility claim. The moat data is computed on every run and
// discarded.
//
// One deliberate departure from ADR-0007: no session id prefixes. A session id
// is a linkable identifier that appears in other artifacts, and the aggregate
// answers every question the prefixes were there for. Rows are per model.
func TestCalibrationCarriesTheBoundsAndNothingIdentifying(t *testing.T) {
	c := sampleCalibration()
	body, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	for _, want := range []string{"claude-opus-5", "smallestCached", "ruleMinPrefix", "breakCauses"} {
		if !strings.Contains(s, want) {
			t.Errorf("the report must carry %q — it is the interval that one machine cannot "+
				"close and a hundred can:\n%s", want, s)
		}
	}
	// The whole basis for doing this at all.
	for _, forbidden := range []string{"sessionId", "session_id", "path", "project", "prompt", "content"} {
		if strings.Contains(strings.ToLower(s), strings.ToLower(forbidden)) {
			t.Errorf("the report carries %q, which is about the contributor rather than "+
				"about the provider:\n%s", forbidden, s)
		}
	}
}

// A report with no compared turns is not a calibration.
//
// Every refusal here is a case where pooling the submission would move a
// denominator without moving its numerator — the same failure Corpus.Validate
// refuses for tasks.
func TestCalibrationRefusesWhatCannotBePooled(t *testing.T) {
	base := sampleCalibration()

	empty := base
	empty.Models = nil
	err := empty.Digested().Validate()
	if err == nil {
		t.Fatal("a report with no model rows was accepted; there is nothing in it to pool")
	}
	// The message is asserted, not just the refusal. Without it the branch is
	// unfalsifiable: the compared==0 check below refuses an empty report too, so
	// deleting this case changes nothing observable — which ADR-0014 rules out.
	// Two states that a reader must tell apart deserve two sentences.
	if !strings.Contains(err.Error(), "no model rows") {
		t.Errorf("an empty report and a report that compared nothing must say different\n"+
			"things, or one of the two refusals is decoration. got: %v", err)
	}

	nothingCompared := base
	nothingCompared.Models = []ModelCalibrationRow{{Model: "m", Sessions: 3, Compared: 0}}
	if err := nothingCompared.Digested().Validate(); err == nil {
		t.Error("a report whose rows compared no turns was accepted: NOT MEASURED is not zero")
	}

	noRules := base
	noRules.RulesVersion = ""
	if err := noRules.Digested().Validate(); err == nil {
		t.Error("a report with no rules version was accepted; bounds measured against an " +
			"unnamed table cannot be pooled with bounds measured against another")
	}

	matchedOverCompared := base
	matchedOverCompared.Models = []ModelCalibrationRow{{Model: "m", Sessions: 1, Compared: 10, Matched: 11}}
	if err := matchedOverCompared.Digested().Validate(); err == nil {
		t.Error("matched exceeding compared describes a state that cannot occur and was accepted")
	}

	if err := base.Validate(); err != nil {
		t.Errorf("a well-formed report was refused: %v", err)
	}
}

// Each refusal in Validate must be reachable on its own.
//
// guard-reachability neutralises a whole conditional rather than one clause,
// and it found these removable with no test noticing. Asserting the message,
// not merely that something failed, is what makes each one falsifiable: several
// of these refusals catch overlapping inputs, so a test that only checks "an
// error happened" is satisfied by whichever guard fires first.
func TestCalibrationRefusalsAreIndividuallyReachable(t *testing.T) {
	for _, tc := range []struct {
		name string
		mut  func(*Calibration)
		want string
	}{
		{"a row with no model id", func(c *Calibration) {
			c.Models = []ModelCalibrationRow{{Model: "", Sessions: 1, Compared: 10, Matched: 9}}
		}, "no model id"},
		{"negative sessions", func(c *Calibration) {
			c.Models = []ModelCalibrationRow{{Model: "m", Sessions: -1, Compared: 10, Matched: 9}}
		}, "negative counters"},
		{"negative compared", func(c *Calibration) {
			c.Models = []ModelCalibrationRow{{Model: "m", Sessions: 1, Compared: -1}}
		}, "negative counters"},
		{"negative matched", func(c *Calibration) {
			c.Models = []ModelCalibrationRow{{Model: "m", Sessions: 1, Compared: 10, Matched: -1}}
		}, "negative counters"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := sampleCalibration()
			tc.mut(&c)
			err := c.Digested().Validate()
			if err == nil {
				t.Fatalf("%s was accepted", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("refused for the wrong reason: want a message containing %q, got %v",
					tc.want, err)
			}
		})
	}
}

// shortHex must actually shorten.
//
// The file name carries a digest prefix so a reader can match a file to a
// roster row. Returning the whole digest would make every name 64 characters
// and the branch that prevents it was unobserved.
func TestShortHexTruncates(t *testing.T) {
	long := strings.Repeat("a", 64)
	if got := shortHex(long); len(got) != 12 {
		t.Errorf("shortHex(64 chars) = %q (%d), want 12 characters", got, len(got))
	}
	if got := shortHex("abc"); got != "abc" {
		t.Errorf("shortHex(%q) = %q, want it returned whole", "abc", got)
	}
}

// A fit that is not a number must not be written.
//
// FitTokensPerByte is a float from a regression, and a fit over zero bytes is
// NaN. encoding/json refuses NaN, so the marshal error in WriteCalibration is
// reachable — it is not the unfalsifiable branch it looks like, and a report
// carrying one must fail loudly rather than write a truncated file.
func TestCalibrationRefusesANaNFit(t *testing.T) {
	nan := math.NaN()
	c := sampleCalibration()
	c.Models[0].FitTokensPerByte = &nan
	if _, err := WriteCalibration(t.TempDir(), c.Digested()); err == nil {
		t.Error("a report carrying a NaN fit was written; encoding/json cannot represent " +
			"it, so this must fail rather than produce a file nobody can read")
	}
}

// The writer refuses a redirected path and refuses to guess.
func TestCalibrationWriterRefusesASymlink(t *testing.T) {
	dir := t.TempDir()
	c := sampleCalibration()
	name := "replay-calibration-" + c.SourceTag + "-" + shortHex(c.Digest) + ".json"
	target := filepath.Join(dir, "elsewhere.json")
	if err := os.WriteFile(target, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, err := WriteCalibration(dir, c)
	if err == nil {
		t.Fatal("a report was written through a symlink")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("refused for the wrong reason: %v", err)
	}
}

// "I could not look" is not "there is nothing here".
//
// When Lstat fails for a reason other than absence, the writer must refuse
// rather than treat the path as free.
func TestCalibrationWriterRefusesWhenItCannotLook(t *testing.T) {
	if runtime.GOOS == "windows" {
		// chmod 0o000 only toggles the read-only bit on Windows and does not
		// make a directory un-inspectable, so the "cannot look" condition this
		// test needs cannot be produced there. The guard is still exercised on
		// every Unix runner; skipping here is the same category as the root
		// skip below — an environment that can look anyway.
		t.Skip("chmod cannot remove directory inspectability on Windows")
	}
	dir := t.TempDir()
	blocked := filepath.Join(dir, "blocked")
	if err := os.Mkdir(blocked, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(blocked, 0o000); err != nil {
		t.Skipf("cannot remove directory permissions: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o700) })
	if os.Geteuid() == 0 {
		t.Skip("running as root, which can look anyway")
	}
	_, err := WriteCalibration(blocked, sampleCalibration())
	if err == nil {
		t.Fatal("a report was written into a directory the writer could not inspect")
	}
	// The message is asserted because the write would fail anyway: without it
	// this guard is shadowed by os.WriteFile, and neutralising it changes
	// nothing any assertion notices.
	if !strings.Contains(err.Error(), "could not establish whether") {
		t.Errorf("refusing to look and failing to write are different states and must\n"+
			"say different things. got: %v", err)
	}
}
