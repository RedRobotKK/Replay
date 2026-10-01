package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// RPL-C004, attacked across the output surface rather than one report.
//
// The precondition finding came first and is evidence about the MECHANISM,
// not the verdict: there is no tier type, no enum, no field carrying a tier
// beside a figure, and the vocabulary is free string literals at 15+ sites
// across 11 packages. ADR-0002 defines two tiers and README defines three,
// and no later ADR supersedes 0002, so the specification itself is in
// conflict before compliance is even measured.
//
// The oracle below does NOT call Replay's tier or classification code,
// because none exists to call. It is a reference judgement over text: a
// figure is present, and a status statement is either adjacent to it or it is
// not. That is a weaker oracle than a typed comparison and it is the
// strongest one the product's own shape permits, which is itself the finding.

// A figure, in the operational sense frozen before measuring: a money amount,
// a token count, a percentage, or a bare quantity presented as a result.
var (
	figMoney   = regexp.MustCompile(`\$[0-9][0-9,]*\.?[0-9]*`)
	figPercent = regexp.MustCompile(`[0-9]+(\.[0-9]+)?%`)
	figTokens  = regexp.MustCompile(`\b[0-9][0-9,]*(\.[0-9]+)?[kKmM]? tokens?\b`)
)

// A status statement, in the operational sense. Deliberately GENEROUS: it
// accepts the three tier words AND the prose mechanism the cost report
// actually uses, because the campaign already established that a dated price
// basis is how that report labels money. Being generous here means a surface
// that fails has really failed.
var statusStatement = regexp.MustCompile(`(?i)` +
	`\b(measured|estimated|structural)\b` + // the documented vocabulary
	`|list price` + // the mechanism the cost report uses
	`|\bnot established\b|\bcannot\b|\bunknown\b|\bunpriced\b` + // refusals
	`|\b20\d\d-\d\d-\d\d\b`) // a dated basis

type surfaceProbe struct {
	name string
	run  func(args []string, stdout, stderr io.Writer) error
	args func(dir string) []string
}

// The thirteen surfaces that share one callable signature. Enumerated rather
// than sampled, so coverage is a fact and not a claim.
func tierSurfaces() []surfaceProbe {
	return []surfaceProbe{
		{"cost", runCost, func(d string) []string { return []string{d} }},
		{"cost --json", runCost, func(d string) []string { return []string{"--json", d} }},
		{"burn", runBurn, func(d string) []string { return nil }},
		{"advise", runAdvise, func(d string) []string { return []string{d} }},
		{"since", runSince, func(d string) []string { return []string{d} }},
		{"context", runContext, func(d string) []string { return []string{d} }},
		{"trim", runTrim, func(d string) []string { return []string{d} }},
		{"route", runRoute, func(d string) []string { return []string{d} }},
		{"ceiling", runCeiling, func(d string) []string { return []string{d} }},
		{"budget", runBudget, func(d string) []string { return []string{d} }},
		{"prefix", runPrefix, func(d string) []string { return []string{d} }},
		{"learn", runLearn, func(d string) []string { return []string{d} }},
		{"rules", runRules, func(d string) []string { return nil }},
		{"privacy", runPrivacy, func(d string) []string { return nil }},
	}
}

// TS1. The surface inventory, measured. For each surface: did it emit a
// figure, and did it emit a status statement?
//
// This test does not fail on a surface that carries no tier. It RECORDS the
// inventory, because the verdict on C004b belongs in the claim register and
// not in a test that would then have to be edited every time a report changes.
// What it does fail on is a broken sweep: if no surface emits a figure at all,
// the inventory is worthless.
func TestC004_OutputSurfaceInventory(t *testing.T) {
	dir := tierFixture(t)

	type result struct {
		name      string
		ran       bool
		hasFigure bool
		hasStatus bool
		sample    string
	}
	var results []result
	withFigures, withStatus := 0, 0

	for _, s := range tierSurfaces() {
		var out, errOut bytes.Buffer
		err := s.run(s.args(dir), &out, &errOut)
		text := out.String() + "\n" + errOut.String()

		r := result{name: s.name, ran: err == nil}
		r.hasFigure = figMoney.MatchString(text) || figPercent.MatchString(text) || figTokens.MatchString(text)
		r.hasStatus = statusStatement.MatchString(text)
		if r.hasFigure {
			withFigures++
			if r.hasStatus {
				withStatus++
			} else {
				// Keep the first unlabelled figure, as the example the report asks for.
				if m := figMoney.FindString(text); m != "" {
					r.sample = m
				} else if m := figPercent.FindString(text); m != "" {
					r.sample = m
				} else {
					r.sample = figTokens.FindString(text)
				}
			}
		}
		results = append(results, r)
	}

	// POSITIVE CONTROL for the sweep itself.
	if withFigures == 0 {
		t.Fatal("no surface emitted a figure. The sweep is broken and the inventory " +
			"below would describe nothing.")
	}

	sort.Slice(results, func(i, j int) bool { return results[i].name < results[j].name })
	var b strings.Builder
	b.WriteString("\nOUTPUT SURFACE INVENTORY (C004b evidence)\n")
	b.WriteString("surface              ran   figure  status  unlabelled example\n")
	for _, r := range results {
		b.WriteString(padRight(r.name, 20) + " " +
			yn(r.ran) + "    " + yn(r.hasFigure) + "      " + yn(r.hasStatus) + "     " + r.sample + "\n")
	}
	b.WriteString("\n")
	b.WriteString("surfaces emitting a figure: " + tierItoa(withFigures) + "\n")
	b.WriteString("of those, carrying a status statement: " + tierItoa(withStatus) + "\n")
	t.Log(b.String())
}

func padRight(s string, n int) string {
	for len(s) < n {
		s += " "
	}
	return s
}
func yn(b bool) string {
	if b {
		return "yes"
	}
	return "no "
}
func tierItoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	return string(d)
}

// TS2. The detectors must discriminate, or TS1 is noise.
func TestC004_TheSurfaceDetectorsDiscriminate(t *testing.T) {
	cases := []struct {
		text       string
		wantFigure bool
		wantStatus bool
		why        string
	}{
		{"total $12.34", true, false, "money with no status"},
		{"total $12.34, at list prices dated 2026-09-07", true, true, "money with a dated basis"},
		{"cache hit 94.2%", true, false, "a percentage with no status"},
		{"12k tokens re-billed", true, false, "a token count with no status"},
		{"4 sessions, 12 requests", false, false, "bare counts are not figures under the frozen definition"},
		{"the cost cannot be established", false, true, "a refusal with no figure"},
	}
	for _, c := range cases {
		gotFig := figMoney.MatchString(c.text) || figPercent.MatchString(c.text) || figTokens.MatchString(c.text)
		gotStat := statusStatement.MatchString(c.text)
		if gotFig != c.wantFigure {
			t.Errorf("figure detector on %q = %v, want %v (%s)", c.text, gotFig, c.wantFigure, c.why)
		}
		if gotStat != c.wantStatus {
			t.Errorf("status detector on %q = %v, want %v (%s)", c.text, gotStat, c.wantStatus, c.why)
		}
	}
}

// TS3. The specification conflict, pinned.
//
// ADR-0002 is titled "two tiers of truth" and names estimated and measured.
// README names three, adding structural. No ADR supersedes 0002. This test
// fails if either document changes, so the conflict cannot be resolved
// silently in one place and left standing in the other.
func TestC004_TheTierSpecificationIsInConflict(t *testing.T) {
	adr := readRepoFile(t, "docs/adr/0002-replay-engine-and-truth-tiers.md")
	readme := readRepoFile(t, "README.md")

	adrHasStructural := regexp.MustCompile(`(?i)\bstructural\b`).MatchString(adr)
	readmeHasStructural := regexp.MustCompile(`(?i)\bstructural\b`).MatchString(readme)

	if adrHasStructural && readmeHasStructural {
		t.Fatal("both documents now name a structural tier; the conflict is resolved " +
			"and the claim register must be updated rather than this test")
	}
	if !readmeHasStructural {
		t.Fatal("README no longer names a structural tier; the conflict is resolved " +
			"the other way and the register must be updated")
	}
	t.Log("RECORDED: README names a structural tier and ADR-0002 does not, and no " +
		"ADR supersedes 0002. The authoritative vocabulary is UNRESOLVED. No third " +
		"tier is invented by this campaign.")
}

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		if b, err := os.ReadFile(filepath.Join(dir, rel)); err == nil {
			return string(b)
		}
		dir = filepath.Dir(dir)
	}
	t.Fatalf("could not find %s above the working directory", rel)
	return ""
}
