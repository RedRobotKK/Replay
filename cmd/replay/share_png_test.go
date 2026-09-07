package main

import (
	"bytes"
	"fmt"
	"image/png"

	"github.com/RedRobotKK/Replay/internal/card"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// corpusAt builds a transcript directory under a project name of the caller's
// choosing, and returns the path. The project name is a parameter because the
// card must be provably independent of it.
func corpusAt(t *testing.T, project string, lanes int) string {
	t.Helper()
	home := t.TempDir()
	dir := filepath.Join(home, ".claude", "projects", project)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	src, err := filepath.Abs(filepath.Join("..", "..", "internal", "transcript", "testdata", "session-redacted.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < lanes; i++ {
		if err := os.Symlink(src, filepath.Join(dir, fmt.Sprintf("lane-%d.jsonl", i))); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// The card is posted to services that crop anything else, and what they crop
// first is the bottom edge, where the install line is.
func TestSharePNGIsAnOpenGraphCard(t *testing.T) {
	dir := corpusAt(t, "acme-secret-project", 2)
	out := filepath.Join(t.TempDir(), "card.png")

	var stdout, stderr bytes.Buffer
	if err := runCost([]string{"--share", "--png", out, "--card", "b", dir}, &stdout, &stderr); err != nil {
		t.Fatalf("cost --share --png: %v (stderr: %s)", err, stderr.String())
	}
	f, err := os.Open(out)
	if err != nil {
		t.Fatalf("no file was written: %v", err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("what was written is not a PNG: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 1200 || b.Dy() != 630 {
		t.Errorf("card is %dx%d, want 1200x630", b.Dx(), b.Dy())
	}
	// The text card still goes to stdout unchanged. --png adds a file; it does
	// not replace the thing that is safe to paste.
	if !strings.Contains(stdout.String(), "curl -fsSL") {
		t.Errorf("--png suppressed the pasteable card:\n%s", stdout.String())
	}
	if !strings.Contains(stderr.String(), out) {
		t.Errorf("the run never said where it wrote the file:\n%s", stderr.String())
	}
}

// The whole privacy argument of the share card is that it carries no path and
// no project name. A picture is harder to audit than a block of text, so this
// asserts the property that makes the audit unnecessary: two corpora with the
// same content under different project names and different parent directories
// must produce byte-identical files.
//
// PASS: identical bytes, so nothing about where the transcripts live reached
// the image.
// FAIL: any difference, which is a channel a path could travel down.
func TestSharePNGIsInvariantToWhereTheCorpusLives(t *testing.T) {
	first := renderTo(t, corpusAt(t, "acme-payments-prod", 2), "b")
	second := renderTo(t, corpusAt(t, "z", 2), "b")
	if !bytes.Equal(first, second) {
		t.Errorf("the same measurements under two different project names produced "+
			"two different cards (%d vs %d bytes). Something about the path reached "+
			"the image, and the card is posted in public", len(first), len(second))
	}
	// And the obvious check as well, cheap and worth having: the names must not
	// be sitting in the file as literal bytes.
	for _, leak := range []string{"acme-payments-prod", ".claude", string(filepath.Separator) + "projects"} {
		if bytes.Contains(first, []byte(leak)) {
			t.Errorf("the PNG contains %q", leak)
		}
	}
}

// Refusing beats posting zeros that look like a finding. `--share` already
// declines when nothing was measured; the picture has to decline on exactly the
// same condition, and must leave no file behind when it does — a zero-byte or
// half-written PNG in the place the user asked for one is worse than an error,
// because it looks like it worked.
func TestSharePNGRefusesWhenNothingWasMeasured(t *testing.T) {
	// A transcript that exists and prices to nothing, which is the condition
	// the guard is actually about. A directory with no transcripts at all is
	// caught earlier, by a different check, and would not exercise this one.
	empty := t.TempDir()
	if err := os.WriteFile(filepath.Join(empty, "nothing.jsonl"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "card.png")

	var stdout, stderr bytes.Buffer
	err := runCost([]string{"--share", "--png", out, empty}, &stdout, &stderr)
	if err == nil {
		t.Fatalf("an empty corpus produced a card rather than a refusal (stderr: %s)", stderr.String())
	}
	if !strings.Contains(err.Error(), "nothing measured") {
		t.Errorf("the refusal must say why: %v", err)
	}
	if _, statErr := os.Stat(out); statErr == nil {
		t.Errorf("%s was written anyway; a file that exists reads as a card that worked", out)
	}
}

// --png without --share would be a second, silent way to produce the card, and
// the guard that decides whether there is anything worth posting lives on the
// --share path.
func TestSharePNGRequiresShare(t *testing.T) {
	dir := corpusAt(t, "proj", 2)
	out := filepath.Join(t.TempDir(), "card.png")

	var stdout, stderr bytes.Buffer
	err := runCost([]string{"--png", out, dir}, &stdout, &stderr)
	if err == nil {
		t.Fatal("--png without --share produced something rather than saying what it needs")
	}
	if !strings.Contains(err.Error(), "--share") {
		t.Errorf("the error must name the flag it needs: %v", err)
	}
	if _, statErr := os.Stat(out); statErr == nil {
		t.Errorf("%s was written by a run that errored", out)
	}
}

// --card picks an arm, and the two arms are different pictures.
func TestCardFlagSelectsTheArm(t *testing.T) {
	dir := corpusAt(t, "proj", 2)
	b, c := renderTo(t, dir, "b"), renderTo(t, dir, "c")
	if bytes.Equal(b, c) {
		t.Error("--card b and --card c produced the same file; there is one design, not two")
	}
	if again := renderTo(t, dir, "b"); !bytes.Equal(b, again) {
		t.Error("two runs of --card b over the same corpus differ; the card is not reproducible")
	}
}

func TestCardFlagRejectsAnUnknownDesign(t *testing.T) {
	dir := corpusAt(t, "proj", 2)
	out := filepath.Join(t.TempDir(), "card.png")
	var stdout, stderr bytes.Buffer
	err := runCost([]string{"--share", "--png", out, "--card", "a", dir}, &stdout, &stderr)
	if err == nil {
		t.Fatal("--card a was accepted; there is no design a")
	}
	if !strings.Contains(err.Error(), "b") || !strings.Contains(err.Error(), "c") {
		t.Errorf("the error must name the designs that do exist: %v", err)
	}
}

// With no --card, the card is design c, and nothing is said about testing.
//
// Randomising this was considered and rejected: the design would be assigned
// per posting machine while installs are counted per reader, so the effective
// sample size is the number of distinct posters rather than the number of
// installs, and at two posters no volume of traffic makes the comparison
// readable. What it would have cost is a line of output telling the user their
// card was picked at random for an experiment, which is a bad trade for
// learning nothing.
//
// PASS: the default is c every time, and the run never mentions randomness or
// testing.
// FAIL: a varying default, or output that describes an experiment that is not
// running.
func TestTheDefaultDesignIsCAndIsNotAnExperiment(t *testing.T) {
	dir := corpusAt(t, "proj", 2)
	explicitC := renderTo(t, dir, "c")

	for i := 0; i < 8; i++ {
		out := filepath.Join(t.TempDir(), "card.png")
		var stdout, stderr bytes.Buffer
		if err := runCost([]string{"--share", "--png", out, dir}, &stdout, &stderr); err != nil {
			t.Fatalf("cost --share --png: %v (stderr: %s)", err, stderr.String())
		}
		got, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, explicitC) {
			t.Fatalf("run %d produced something other than design c; the default must "+
				"not vary between runs", i)
		}
		if !strings.Contains(stderr.String(), "design c") {
			t.Errorf("the run never said which design it wrote:\n%s", stderr.String())
		}
		for _, word := range []string{"random", "experiment", "A/B", "compared"} {
			if strings.Contains(stderr.String(), word) {
				t.Errorf("the output mentions %q. Nothing is being tested, and saying "+
					"otherwise makes the tool look like it is experimenting on the "+
					"user:\n%s", word, stderr.String())
			}
		}
	}
}

// The design letter travels in the URL path whichever design was chosen, so the
// dimension exists in the traffic data from the first card onward rather than
// needing a redesign to introduce later.
func TestEachDesignCarriesItsOwnPath(t *testing.T) {
	for arm, want := range map[string]string{
		"b": "curl -fsSL https://redrobot.jp/c/b | sh",
		"c": "curl -fsSL https://redrobot.jp/c/c | sh",
	} {
		v, err := pickVariant(arm)
		if err != nil {
			t.Fatal(err)
		}
		if got := card.InstallLine(v); got != want {
			t.Errorf("--card %s install line:\n got %q\nwant %q", arm, got, want)
		}
	}
}

// The card must carry no spend total, for the reason set out at the top of
// share.go, and the token figure on design b is where that could be undone by
// arithmetic rather than by printing.
//
// The corpus sum of re-billed tokens, divided by the avoidable rate that is on
// the same card, reconstructs an order-of-magnitude spend total. The peak of a
// single row divides into nothing, because a reader cannot know how many rows
// produced it. So the function that computes it is exercised directly, over
// rows whose sum and whose maximum are far apart: an earlier version of this
// test called cardData with a literal and passed happily while
// peakAvoidableTokens was replaced by a sum, which is a check that could not
// fail.
//
// PASS: the peak row's figure, not the total.
// FAIL: the sum, which is the leak.
func TestSharePNGCarriesNoSpendTotal(t *testing.T) {
	units := []costUnit{
		{AvoidableTokens: 40_000},
		{AvoidableTokens: 189_000},
		{AvoidableTokens: 12_000},
		{AvoidableTokens: 90_000},
	}
	got := peakAvoidableTokens(units)
	if got == 331_000 {
		t.Fatalf("peakAvoidableTokens returned the corpus sum (%d). Divided by the "+
			"avoidable rate printed beside it, that reconstructs a spend total, which "+
			"is the one figure the share card refuses to carry", got)
	}
	if got != 189_000 {
		t.Fatalf("peakAvoidableTokens = %d, want the largest single row, 189000", got)
	}
	if peakAvoidableTokens(nil) != 0 {
		t.Error("no rows must mean no figure, not a figure derived from nothing")
	}

	// And the summary's own total must not survive the trip into the picture.
	s := costSummary{
		Tasks: 1384, Unit: unitSession, TotalUSD: 2906.39, MedianUSD: 0.77,
		P90USD: 2.30, AvoidableUSD: 151.54, AvoidableShare: 0.052,
	}
	d := cardData(s, 67, got)
	if d.AvoidableShare != s.AvoidableShare || d.MedianUSD != s.MedianUSD || d.P90USD != s.P90USD {
		t.Errorf("the comparable figures did not survive the trip: %+v", d)
	}
	if d.PeakAvoidableTokens != 189_000 {
		t.Errorf("PeakAvoidableTokens = %d, want the peak row's figure", d.PeakAvoidableTokens)
	}
}

// renderTo runs the command and returns the bytes it wrote.
func renderTo(t *testing.T, dir, arm string) []byte {
	t.Helper()
	out := filepath.Join(t.TempDir(), "card.png")
	var stdout, stderr bytes.Buffer
	if err := runCost([]string{"--share", "--png", out, "--card", arm, dir}, &stdout, &stderr); err != nil {
		t.Fatalf("cost --share --png --card %s: %v (stderr: %s)", arm, err, stderr.String())
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// --tone picks the register, and the register is copy.
//
// PASS: the two tones write different files, the default is measured, the run
// says which register it wrote, and an unknown one is refused before anything
// is read.
// FAIL: a tone that does not reach the file, a default that varies, a silent
// write, or a typo accepted.
func TestToneFlagSelectsTheRegister(t *testing.T) {
	dir := corpusAt(t, "proj", 2)
	measured, rekt := renderTone(t, dir, "c", "measured"), renderTone(t, dir, "c", "rekt")
	if bytes.Equal(measured, rekt) {
		t.Error("--tone measured and --tone rekt produced the same file; there is one " +
			"register, not two")
	}
	if def := renderTone(t, dir, "c", ""); !bytes.Equal(def, measured) {
		t.Error("the default tone is not measured. A card that changes register " +
			"between runs is a card nobody can review")
	}

	out := filepath.Join(t.TempDir(), "card.png")
	var stdout, stderr bytes.Buffer
	if err := runCost([]string{"--share", "--png", out, "--tone", "rekt", dir}, &stdout, &stderr); err != nil {
		t.Fatalf("cost --share --png --tone rekt: %v (stderr: %s)", err, stderr.String())
	}
	if !strings.Contains(stderr.String(), "rekt") {
		t.Errorf("the run never said which register it wrote:\n%s", stderr.String())
	}

	err := runCost([]string{"--share", "--png", out, "--tone", "wrecked", dir}, &stdout, &stderr)
	if err == nil {
		t.Fatal("--tone wrecked was accepted; there is no such register")
	}
	for _, want := range []string{"measured", "rekt"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error must name the registers that do exist: %v", err)
		}
	}
}

// The tone changes the words and cannot change a figure, all the way through
// the command rather than only inside the card package.
//
// PASS: both registers over one corpus carry the same measurements, and the
// text card printed beside them is identical.
// FAIL: a figure that moved with the register.
func TestToneDoesNotMoveAFigureThroughTheCommand(t *testing.T) {
	dir := corpusAt(t, "proj", 3)
	var first, second bytes.Buffer
	var errb bytes.Buffer
	for _, tone := range []string{"measured", "rekt"} {
		out := filepath.Join(t.TempDir(), "card.png")
		var stdout bytes.Buffer
		if err := runCost([]string{"--share", "--png", out, "--tone", tone, dir}, &stdout, &errb); err != nil {
			t.Fatalf("tone %s: %v", tone, err)
		}
		if first.Len() == 0 {
			first.Write(stdout.Bytes())
			continue
		}
		second.Write(stdout.Bytes())
	}
	if first.String() != second.String() {
		t.Errorf("the register changed the pasteable card:\n%s\n---\n%s", first.String(), second.String())
	}
}

// renderTone runs the command in one register and returns the bytes it wrote.
func renderTone(t *testing.T, dir, arm, tone string) []byte {
	t.Helper()
	out := filepath.Join(t.TempDir(), "card.png")
	args := []string{"--share", "--png", out, "--card", arm}
	if tone != "" {
		args = append(args, "--tone", tone)
	}
	var stdout, stderr bytes.Buffer
	if err := runCost(append(args, dir), &stdout, &stderr); err != nil {
		t.Fatalf("cost --share --png --card %s --tone %q: %v (stderr: %s)", arm, tone, err, stderr.String())
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
