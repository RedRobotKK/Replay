package observation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Guards the reachability reviewer reported as unobserved.
//
// Every test below exists because neutralising one conditional left the suite
// green. Several of the guards ARE reached by an existing test — the problem
// was that another branch produced an error too, so the test saw a failure and
// could not tell which one. A test that asserts "it errored" over code with two
// error paths is asserting almost nothing, and that is the shape these fix:
// each one pins the SPECIFIC refusal to the guard that should have produced it.

// GD1: the duplicate-digest refusal is distinguishable from the same-hour one.
//
// PL3 passed with the digest check disabled, because two identical submissions
// also collide on tag-and-timestamp and the supersession branch refused them
// anyway — with a message saying the figures DIFFER, which for an identical
// resubmission is false. The guard is what makes the right thing get said.
func TestGD1_TheDuplicateRefusalNamesTheDuplicate(t *testing.T) {
	p := NewPool("x")
	c := corpusFor("aaa", "2026-09-01T00:00Z", 100, 1000, 50, 0.70)
	mustAdd(t, p, c)
	err := p.Add(c, "")
	if err == nil {
		t.Fatal("the identical submission was pooled twice")
	}
	if !strings.Contains(err.Error(), "already in this pool") {
		t.Errorf("a re-sent identical file was refused by the wrong branch: %v\n"+
			"The same-hour branch says the figures differ, which is false here.", err)
	}
}

// GD2: a lower median arriving later widens the range downward.
//
// PL8 fed the medians in ascending order, so the `<` branch never ran: every
// candidate low was already the low. Descending order is what exercises it.
func TestGD2_ALaterLowerMedianWidensTheRange(t *testing.T) {
	p := NewPool("x")
	mustAdd(t, p, corpusFor("aaa", "2026-09-01T00:00Z", 10, 100, 5, 3.40))
	mustAdd(t, p, corpusFor("bbb", "2026-09-01T00:00Z", 10, 100, 5, 0.20))

	got, err := p.Totals()
	if err != nil {
		t.Fatal(err)
	}
	if got.MedianTaskUSDLow != 0.20 || got.MedianTaskUSDHigh != 3.40 {
		t.Errorf("range is %v-%v, want 0.20-3.40; a median lower than everything "+
			"seen so far did not move the floor", got.MedianTaskUSDLow, got.MedianTaskUSDHigh)
	}
}

// GD3: an earlier price table arriving later moves the low bound.
//
// Same defect as GD2, on the other bound: PL10 added dates in ascending order.
func TestGD3_AnEarlierPriceTableMovesTheLowBound(t *testing.T) {
	p := NewPool("x")
	mustAdd(t, p, corpusFor("aaa", "2026-09-01T00:00Z", 10, 100, 5, 1.0))
	early := corpusFor("bbb", "2026-09-01T00:00Z", 10, 100, 5, 1.0)
	early.PricedAt = "2026-08-01"
	mustAdd(t, p, early.Digested())

	got, err := p.Totals()
	if err != nil {
		t.Fatal(err)
	}
	if got.PricedAtLow != "2026-08-01" || got.PricedAtHigh != "2026-09-07" {
		t.Errorf("span is %s..%s, want 2026-08-01..2026-09-07", got.PricedAtLow, got.PricedAtHigh)
	}
}

// GD4: account-derived tags are described differently from local ones.
//
// Every other fixture uses BasisLocal, so the branch of TagNote that speaks
// about real accounts had never run. It is the branch that would matter most if
// it were wrong: it is the one that says a count IS a count of accounts.
func TestGD4_AccountTagsAreDescribedAsAccounts(t *testing.T) {
	p := NewPool("x")
	for _, tag := range []string{"aaa", "bbb"} {
		c := corpusFor(tag, "2026-09-01T00:00Z", 10, 100, 5, 1.0)
		c.TagBasis = BasisAccount
		mustAdd(t, p, c.Digested())
	}
	got, err := p.Totals()
	if err != nil {
		t.Fatal(err)
	}
	if !got.TagsAreIdentities {
		t.Fatal("account-derived tags are reported as non-identities")
	}
	note := TagNote(got)
	if !strings.Contains(note, "distinct provider accounts") {
		t.Errorf("account tags get the machine-local caveat: %q", note)
	}
	if strings.Contains(note, "MACHINES AT MOST") {
		t.Errorf("account tags are described as machines: %q", note)
	}
}

// GD5: with nothing superseded, the note is absent rather than empty-but-present.
//
// The guard returns "" so Render prints no section at all. Neutralised, Render
// printed "0 further submission(s) are named below and NOT counted" over an
// empty list, which is a sentence about nothing.
func TestGD5_NoSupersessionMeansNoNote(t *testing.T) {
	p := NewPool("x")
	mustAdd(t, p, corpusFor("aaa", "2026-09-01T00:00Z", 10, 100, 5, 1.0))
	got, err := p.Totals()
	if err != nil {
		t.Fatal(err)
	}
	if note := SupersededNote(got); note != "" {
		t.Errorf("a pool with nothing superseded produced a note: %q", note)
	}
	out, err := p.Render()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "NOT counted") {
		t.Errorf("the report explains a supersession that did not happen:\n%s", out)
	}
}

// GD6: the unpriced footnote appears only when something was unpriced, and the
// price-table span only when the dates actually differ.
//
// Both are conditional lines in Render that no test read. A footnote that
// always prints says nothing; one that never prints hides what the total does
// not cover.
func TestGD6_ConditionalFootnotesTrackTheirCondition(t *testing.T) {
	clean := NewPool("x")
	mustAdd(t, clean, corpusFor("aaa", "2026-09-01T00:00Z", 10, 100, 5, 1.0))
	quiet, err := clean.Render()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(quiet, "left out") {
		t.Errorf("a pool with nothing unpriced printed the exclusion footnote:\n%s", quiet)
	}
	if strings.Contains(quiet, "Price tables span") {
		t.Errorf("a pool on one price table printed a span:\n%s", quiet)
	}

	mixed := NewPool("x")
	withUnpriced := corpusFor("aaa", "2026-09-01T00:00Z", 10, 100, 5, 1.0)
	withUnpriced.Unpriced = 6
	mustAdd(t, mixed, withUnpriced.Digested())
	later := corpusFor("bbb", "2026-09-01T00:00Z", 10, 100, 5, 1.0)
	later.PricedAt = "2026-09-14"
	mustAdd(t, mixed, later.Digested())

	loud, err := mixed.Render()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(loud, "left out") || !strings.Contains(loud, "6 further") {
		t.Errorf("six unpriced transcripts are not reported:\n%s", loud)
	}
	if !strings.Contains(loud, "Price tables span 2026-09-07 to 2026-09-14") {
		t.Errorf("two price-table dates are not disclosed:\n%s", loud)
	}
}

// GD7: a digest is abbreviated in the table and whole in the payload.
//
// shortDigest's truncation had no assertion: the roster row was only checked
// for containing the filename, which is untruncated either way.
func TestGD7_TheRosterAbbreviatesAndTheFileDoesNot(t *testing.T) {
	p := NewPool("x")
	c := corpusFor("aaa", "2026-09-01T00:00Z", 10, 100, 5, 1.0)
	mustAdd(t, p, c)
	if len(c.Digest) != 64 {
		t.Fatalf("the fixture's digest is %d chars, expected a 64-char sha256", len(c.Digest))
	}
	out, err := p.Render()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, c.Digest) {
		t.Error("the full 64-character digest is printed in the table, which is what the " +
			"abbreviation exists to prevent")
	}
	if !strings.Contains(out, c.Digest[:12]) {
		t.Errorf("the abbreviated digest is not in the table:\n%s", out)
	}
	// Short input passes through untouched: a tag of three characters must not
	// be padded or sliced.
	if got := shortDigest("aaa"); got != "aaa" {
		t.Errorf("shortDigest(%q) = %q", "aaa", got)
	}
}

// GD8: a filename is built from a short or absent digest without panicking.
//
// corpusFileName slices the digest, and every fixture handed it a 64-character
// one. A Corpus that has not been through Digested has an empty digest, and the
// slice bound is what stops that being a panic in the writer.
func TestGD8_AFileNameSurvivesAnUndigestedCorpus(t *testing.T) {
	bare := Corpus{Schema: CorpusSchema, SourceTag: "aaa"}
	name := corpusFileName(bare)
	if !strings.HasPrefix(name, "replay-corpus-aaa-") || !strings.HasSuffix(name, ".json") {
		t.Errorf("an undigested corpus names its file %q", name)
	}
}

// GD9: the symlink refusal is distinguishable from the already-exists refusal.
//
// CW4 passed with the symlink check disabled: Lstat still succeeded on the
// link, so the next branch refused it as an existing file. Two refusals, one
// assertion, and the test could not tell them apart — which matters because
// only one of them is about writing outside the directory.
func TestGD9_TheSymlinkRefusalSaysSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(t.TempDir(), "elsewhere.json")
	if err := os.Symlink(target, filepath.Join(dir, corpusFileName(sampleCorpus()))); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, err := WriteCorpus(dir, sampleCorpus())
	if err == nil {
		t.Fatal("a submission was written through a symlink")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("the symlink was refused as an ordinary existing file, so the "+
			"redirected-path check is not what stopped it: %v", err)
	}
}

// GD10: a destination that cannot be inspected is an error, not an absence.
//
// The `else if !errors.Is(err, os.ErrNotExist)` arm separates "there is nothing
// here" from "I could not look". Treating the second as the first would write
// into a path nobody has established is safe. A file used as a directory
// component gives ENOTDIR, which is the cheapest way to reach it.
func TestGD10_AnUninspectableDestinationIsAnError(t *testing.T) {
	dir := t.TempDir()
	notADir := filepath.Join(dir, "file")
	if err := os.WriteFile(notADir, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := WriteCorpus(filepath.Join(notADir, "under"), sampleCorpus())
	if err == nil {
		t.Fatal("a destination whose parent is a file was treated as an empty directory")
	}
	if !strings.Contains(err.Error(), "cannot inspect") {
		t.Errorf("the refusal came from the write rather than from the inspection, so the "+
			"could-not-look branch is not what stopped it: %v", err)
	}
}

// GD11: EarlierSubmissions distinguishes the four things in a directory.
//
// Its loop has four conditionals and no test read any of them: an unreadable
// directory, a subdirectory, the file just written, and a file belonging to
// somebody else. Each one silently wrong produces a different bad message to a
// contributor deciding what to send.
func TestGD11_EarlierSubmissionsPicksOnlyThisMachinesOlderFiles(t *testing.T) {
	if got := EarlierSubmissions(filepath.Join(t.TempDir(), "nope"), "aaa", ""); got != nil {
		t.Errorf("an unreadable directory reported %v", got)
	}

	dir := t.TempDir()
	mine := "replay-corpus-aaa-111111111111.json"
	older := "replay-corpus-aaa-222222222222.json"
	theirs := "replay-corpus-bbb-333333333333.json"
	for _, n := range []string{mine, older, theirs, "notes.txt", "replay-corpus-aaa-444.md"} {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// A directory whose name matches the prefix must not be offered as a file.
	if err := os.Mkdir(filepath.Join(dir, "replay-corpus-aaa-dir.json"), 0o700); err != nil {
		t.Fatal(err)
	}

	got := EarlierSubmissions(dir, "aaa", filepath.Join(dir, mine))
	if len(got) != 1 || got[0] != older {
		t.Errorf("EarlierSubmissions = %v, want exactly [%s]:\n"+
			"  the file just written must be excluded, another machine's tag must not "+
			"match, a non-.json must not match, and a directory is not a submission", got, older)
	}
}
