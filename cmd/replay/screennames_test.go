package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/tui"
)

// The list of screens is written down once, or it drifts.
//
// It was written down three times: tui.Shortcuts(), the -screen flag's usage
// string, and the error a wrong -screen produces. The tenth screen, live, was
// added to the first and to neither of the others, so `replay tui --screen
// live` worked while the flag help and the error message both said it did not
// exist and both called the set "the nine". A working feature that every
// surface denies is indistinguishable from a broken one.

func screenLabels() []string {
	var out []string
	for _, s := range tui.Shortcuts() {
		out = append(out, s.Label)
	}
	return out
}

// SN1: the flag help names every screen.
func TestScreenFlagHelpNamesEveryScreen(t *testing.T) {
	var out bytes.Buffer
	// -h makes the FlagSet print its usage, which is where the list lives.
	_ = runTUI([]string{"-h"}, &out, &out)
	help := out.String()
	for _, l := range screenLabels() {
		if !strings.Contains(help, l) {
			t.Errorf("-screen help does not offer %q, which is a screen that exists:\n%s", l, help)
		}
	}
}

// SN2: and so does the error for a screen that does not.
//
// This is the message a reader gets at the moment they are already wrong about
// the screen names, so it is the worst place to be missing one.
func TestUnknownScreenErrorNamesEveryScreen(t *testing.T) {
	var out bytes.Buffer
	err := runTUI([]string{"-screen", "nosuchscreen"}, &out, &out)
	if err == nil {
		t.Fatal("an unknown screen was accepted")
	}
	msg := err.Error()
	for _, l := range screenLabels() {
		if !strings.Contains(msg, l) {
			t.Errorf("the error for an unknown screen does not list %q:\n%s", l, msg)
		}
	}
}

// SN3: and nothing calls the set by a count that has to be maintained by hand.
//
// "The nine" was accurate once. A number spelled into a sentence beside a list
// that grows is a claim with a maintenance cost and no owner.
func TestNothingCountsTheScreensInProse(t *testing.T) {
	var out bytes.Buffer
	err := runTUI([]string{"-screen", "nosuchscreen"}, &out, &out)
	if err == nil {
		t.Fatal("an unknown screen was accepted")
	}
	for _, n := range []string{"The nine", "the nine", "The ten", "the ten"} {
		if strings.Contains(err.Error(), n) {
			t.Errorf("the error counts the screens in prose (%q); the list is right "+
				"there and cannot go stale, the count can:\n%s", n, err.Error())
		}
	}
}

// SN3B: and nothing anywhere else counts them either.
//
// SN3 above states the rule and could only ever apply it to one string, the
// error a wrong -screen produces. The same claim was standing in nine other
// places on the day SN3 went green: the doc comment on Shortcuts() itself,
// which argued at length that a tenth screen could not exist while sitting
// three lines above a ten-element literal; the FX field in
// internal/tui/measured.go, whose count named a number of screens that never
// shared that struct; two paragraphs of docs/guide/commands.md; two of
// docs/AGENT-SURFACE.md, on a page whose own section 4 says "a count written
// here was wrong within days of being written"; and three more in this
// package and internal/tui that the first run of this scan found.
//
// WHAT IT LOOKS FOR, and the reasoning is the narrowness.
//
// A count next to screens/questions/shortcuts/keys that is exactly one away
// from len(tui.Shortcuts()). Off by one is the shape of this defect: the set
// grows by a screen and the prose does not follow. A subset count — "four
// screens cannot disagree about what this machine means", "the other seven
// questions are still one keystroke away" — is honest prose about a part of
// the set, and there is no syntax that separates it from a stale total, so
// small numbers are left alone. This test would not have caught the surface
// going from eight to ten in one step, and says so here rather than implying
// otherwise.
//
// WHY IT JOINS LINES BEFORE MATCHING.
//
// docs/AGENT-SURFACE.md wrapped "the nine\nshortcuts" across a line break. A
// line scanner cannot see that claim at all, which is the scan-scope defect
// internal/regression/proxy_doc_claims_test.go records having shipped twice.
// Markdown paragraphs and runs of // comments are joined before matching, and
// the match is reported at the source line it started on.
//
// WHAT IT DOES NOT COVER: docs/CLI.md is generated from the binary; docs/design
// and docs/evidence hold dated records of a run, and stripping their figures
// would delete the evidence they exist to carry. Same reasoning as
// internal/regression TestFCDC_NoHandWrittenCommandOrFlagTotals, which owns the
// flag and command totals and deliberately does not own this one.
func TestNothingElseCountsTheScreensInProse(t *testing.T) {
	want := len(tui.Shortcuts())

	near := []int{want - 1, want + 1}
	alts := numberAlternation(near)
	patterns := []*regexp.Regexp{
		// A count standing directly in front of the noun.
		regexp.MustCompile(`(?i)\b(?:` + alts + `)\s+(?:more\s+|other\s+|remaining\s+)?(?:screens?|questions?|shortcuts?|keys?)\b`),
		// And the form that names the whole set without naming the noun:
		// "four of the nine read this machine".
		regexp.MustCompile(`(?i)\bof\s+the\s+(?:` + alts + `)\b`),
	}

	// The positive control. A scan that stops matching finds nothing and reads
	// as a clean tree, which is a check that cannot fail.
	//
	// Each control is a sentence this repository actually carried, with the
	// stale number templated off Shortcuts() rather than frozen at "nine", so
	// the control survives the eleventh screen instead of turning red on the
	// day the set legitimately grows. The pattern in this repository is
	// internal/regression/proxy_doc_claims_test.go and
	// corpus_sample_claims_test.go.
	stale := spellNumber(want - 1)
	for _, c := range []struct{ name, positive string }{
		{"the doc comment on Shortcuts()",
			"Shortcuts is the whole surface: " + stale + " questions, " + stale + " keys."},
		{"the FX field in measured.go",
			"than in the report: " + stale + " screens share this struct and most of them will"},
		{"docs/guide/commands.md",
			"The " + stale + " questions as screens you move between, instead of " + stale + " commands"},
		{"docs/AGENT-SURFACE.md, the subset form",
			"`replay tui` runs them as of 0.5.0, and four of the " + stale + " read this machine:"},
	} {
		if !matchesAny(patterns, c.positive) {
			t.Errorf("the scan no longer matches the claim from %s:\n  %s\n"+
				"A pattern edited into uselessness reports an all-clear on every "+
				"file it opens. Fix the pattern, not this control.", c.name, c.positive)
		}
	}

	// And the wrapped control, which is the reason lines are joined at all.
	// It must be invisible line by line and visible once joined, or the join
	// has stopped doing the only thing it is there for.
	wrapped := []string{
		"front of the traffic as a proxy, or answer a question through the " + stale,
		"shortcuts. Which one is right depends on whether the surface writes transcripts,",
	}
	if matchesAny(patterns, wrapped[0]) || matchesAny(patterns, wrapped[1]) {
		t.Errorf("the wrapped control is visible line by line, so it no longer "+
			"proves the join does anything:\n  %s\n  %s", wrapped[0], wrapped[1])
	}
	if !matchesAny(patterns, strings.Join(wrapped, " ")) {
		t.Errorf("the scan cannot see a claim that wraps across a line break:\n  %s\n  %s\n"+
			"docs/AGENT-SURFACE.md carried exactly this and a line scanner read "+
			"the page as clean.", wrapped[0], wrapped[1])
	}

	// Honest historical prose, quoted verbatim with the file that holds it.
	//
	// These three record a state the tree used to be in, or a dated audit run
	// taken over the screens as they stood that day. Rewriting them to today's
	// count would make each one false.
	//
	// Each is asserted to still be present below. An exemption that matches
	// nothing is an exemption nobody can see has stopped applying, so a
	// reworded comment turns this red and a human decides again rather than
	// the allowlist quietly widening. That is the opposite trade from FCDC's
	// whole-directory exemption and it is affordable here because there are
	// three lines, named, in one package.
	// Compared case-insensitively: the number is templated in lower case and
	// live.go's sentence opens with it.
	exempt := []struct{ file, text string }{
		{"internal/tui/fit.go",
			"After the width became a measurement, 72 lines across the " + stale + " screens still exceeded"},
		{"internal/tui/cols.go",
			"A tui-audit run found 88 lines across the " + stale + " screens that exceed"},
		{"internal/tui/live.go",
			stale + " screens opened with a banner; this one opened straight into"},
	}
	seen := make([]bool, len(exempt))

	root := filepath.Join("..", "..")
	var found []string
	var files int
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			case ".git", "node_modules", ".claude":
				return filepath.SkipDir
			}
			return nil
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		rel = filepath.ToSlash(rel)

		comments := strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, "_test.go")
		markdown := strings.HasPrefix(rel, "docs/") && strings.HasSuffix(rel, ".md") &&
			rel != "docs/CLI.md" &&
			!strings.HasPrefix(rel, "docs/design/") &&
			!strings.HasPrefix(rel, "docs/evidence/")
		if !comments && !markdown {
			return nil
		}
		b, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		files++
		for _, u := range proseUnits(string(b), comments) {
			for i, e := range exempt {
				if rel == e.file && strings.Contains(strings.ToLower(u.text), strings.ToLower(e.text)) {
					seen[i] = true
				}
			}
			for _, p := range patterns {
				for _, loc := range p.FindAllStringIndex(u.text, -1) {
					m := u.text[loc[0]:loc[1]]
					if exemptedAt(exempt, rel, u.text) {
						continue
					}
					found = append(found, fmt.Sprintf("%s:%d  %q  in: %s",
						rel, u.lineAt(loc[0]), m, excerpt(u.text, loc[0])))
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// A walk that opened nothing passes everything.
	if files < 100 {
		t.Fatalf("only %d files were scanned, which is too few for this repository: "+
			"the walk is not reaching the tree and this test proved nothing", files)
	}
	for i, e := range exempt {
		if !seen[i] {
			t.Errorf("the exemption for %s matches nothing in the tree:\n  %s\n"+
				"Either the comment was reworded, in which case delete this entry, "+
				"or the walk stopped reading that file.", e.file, e.text)
		}
	}

	for _, f := range found {
		t.Errorf("the screens are counted in prose, and the count is not %d:\n  %s\n\n"+
			"tui.Shortcuts() is the list and cannot go stale; a number spelled into "+
			"a sentence beside it can. Name the set, or cite the list.", want, f)
	}
}

// exemptedAt reports whether this unit is one of the historical sentences.
func exemptedAt(exempt []struct{ file, text string }, rel, unit string) bool {
	for _, e := range exempt {
		if rel == e.file && strings.Contains(strings.ToLower(unit), strings.ToLower(e.text)) {
			return true
		}
	}
	return false
}

// excerpt is enough of the sentence to recognise it without printing the file.
func excerpt(s string, at int) string {
	from := at - 50
	if from < 0 {
		from = 0
	}
	to := at + 90
	if to > len(s) {
		to = len(s)
	}
	return strings.TrimSpace(s[from:to])
}

func matchesAny(ps []*regexp.Regexp, s string) bool {
	for _, p := range ps {
		if p.MatchString(s) {
			return true
		}
	}
	return false
}

var spelledNumbers = map[int]string{
	0: "zero", 1: "one", 2: "two", 3: "three", 4: "four", 5: "five",
	6: "six", 7: "seven", 8: "eight", 9: "nine", 10: "ten", 11: "eleven",
	12: "twelve", 13: "thirteen", 14: "fourteen", 15: "fifteen",
	16: "sixteen", 17: "seventeen", 18: "eighteen", 19: "nineteen", 20: "twenty",
}

// spellNumber is the word a writer would use. Past twenty they write digits,
// and so does this.
func spellNumber(n int) string {
	if w, ok := spelledNumbers[n]; ok {
		return w
	}
	return strconv.Itoa(n)
}

// numberAlternation matches each count as a word and as digits, because both
// spellings have appeared in this repository's prose.
func numberAlternation(ns []int) string {
	var out []string
	for _, n := range ns {
		if w, ok := spelledNumbers[n]; ok {
			out = append(out, w)
		}
		out = append(out, strconv.Itoa(n))
	}
	return strings.Join(out, "|")
}

// proseUnit is a run of lines that reads as one stream of sentences: a Markdown
// paragraph, or a block of // comments.
type proseUnit struct {
	text  string
	marks []proseMark
}

type proseMark struct {
	off  int
	line int
}

// lineAt is the source line the byte at off came from.
func (u proseUnit) lineAt(off int) int {
	line := 0
	for _, m := range u.marks {
		if m.off > off {
			break
		}
		line = m.line
	}
	return line
}

// proseUnits splits a file into those runs. With comments set it reads only //
// lines, stripped of the slashes; otherwise it reads every non-blank line.
func proseUnits(body string, comments bool) []proseUnit {
	var out []proseUnit
	var b strings.Builder
	var marks []proseMark
	flush := func() {
		if len(marks) > 0 {
			out = append(out, proseUnit{text: b.String(), marks: marks})
		}
		b.Reset()
		marks = nil
	}
	for i, raw := range strings.Split(body, "\n") {
		t := strings.TrimSpace(raw)
		var keep string
		if comments {
			if !strings.HasPrefix(t, "//") {
				flush()
				continue
			}
			keep = strings.TrimSpace(strings.TrimPrefix(t, "//"))
		} else {
			if t == "" {
				flush()
				continue
			}
			keep = t
		}
		if b.Len() > 0 {
			b.WriteString(" ")
		}
		marks = append(marks, proseMark{off: b.Len(), line: i + 1})
		b.WriteString(keep)
	}
	flush()
	return out
}

// SN4: `replay mcp --install` still installs, now that it goes through a
// FlagSet instead of a hand-written argument match in the dispatch switch.
//
// The reason it moved is that docs/CLI.md is generated by asking each
// subcommand what flags it has, and a flag matched by hand has none to give:
// the reference said "mcp: Takes no flags" while --install shipped and worked.
// This test is the one that would notice if the move broke it.
func TestMCPInstallStillPrintsTheConfiguration(t *testing.T) {
	for _, spelling := range []string{"--install", "-install"} {
		var out, errs bytes.Buffer
		if err := runMCPCommand([]string{spelling}, &out, &errs); err != nil {
			t.Fatalf("%s returned %v", spelling, err)
		}
		got := out.String()
		for _, want := range []string{"MCP configuration", "replay", "mcp"} {
			if !strings.Contains(got, want) {
				t.Errorf("%s does not print %q:\n%s", spelling, want, got)
			}
		}
		// And it must not have started serving: a server writes JSON-RPC, and
		// a snippet is not JSON-RPC.
		if strings.Contains(got, `"jsonrpc"`) {
			t.Errorf("%s served instead of printing the snippet:\n%s", spelling, got)
		}
	}
}

// SN5: the snippet names something runnable even when the OS will not say what
// this binary is.
//
// os.Executable does not fail on demand, so the fallback lived in a branch no
// test could enter — reported by the guard-reachability check, which
// neutralises every conditional a change touches and names the ones nothing
// observes. The name is the entire output of `mcp --install`: an empty one
// produces a configuration that launches nothing, and it would be found by
// whoever pasted it, not by us.
func TestBinaryNameFallsBackToACommandThatExists(t *testing.T) {
	for _, c := range []struct {
		name string
		exe  string
		err  error
		want string
	}{
		{"the usual case", "/usr/local/bin/replay", nil, "/usr/local/bin/replay"},
		{"the OS refused", "", errNoExe, "replay"},
		{"an error with a path anyway", "/some/path", errNoExe, "replay"},
		{"no error but no path", "", nil, "replay"},
	} {
		if got := binaryName(c.exe, c.err); got != c.want {
			t.Errorf("%s: binaryName(%q, %v) = %q, want %q", c.name, c.exe, c.err, got, c.want)
		}
	}
}

var errNoExe = errors.New("cannot determine executable path")

// SN6: `replay mcp` with a flag it does not have fails instead of serving.
//
// Reported by guard-reachability as INERT. It matters more here than the
// verdict suggests: runMCP blocks reading stdin, so a mis-typed flag that fell
// through to it would hang rather than tell anybody why. The command that
// serves an agent over a pipe is the worst one to have hang on a typo.
func TestMCPRefusesAFlagItDoesNotHave(t *testing.T) {
	var out, errs bytes.Buffer
	err := runMCPCommand([]string{"--no-such-flag"}, &out, &errs)
	if err == nil {
		t.Fatal("an unknown flag was accepted; the command would have served instead")
	}
	if strings.Contains(out.String(), `"jsonrpc"`) {
		t.Errorf("the server started despite the bad flag:\n%s", out.String())
	}
}
