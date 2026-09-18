package main

import (
	"bytes"
	"strings"
	"testing"
)

// `replay grok` is the Grok surface on its own terms, as `replay codex` is for
// Codex.
//
// The reader shipped inside `replay burn` first, which is the right home for
// comparing surfaces against each other and the wrong one for reading a single
// surface closely: burn prints one row and a note, and the Grok record carries
// a cached share, a reasoning count and a rollup divergence that a row cannot
// hold.
//
// Fixtures come from grok_test.go and are the real shape measured 2026-09-17.
// Nothing here reads ~/.grok.
func runGrokIn(t *testing.T, home string, args ...string) string {
	t.Helper()
	t.Setenv("HOME", home)
	var out, errb bytes.Buffer
	if err := runGrok(args, &out, &errb); err != nil {
		t.Fatalf("replay grok: %v (stderr: %s)", err, errb.String())
	}
	return out.String()
}

func TestGrokCommandReportsTheSurface(t *testing.T) {
	// 34,801 prompt tokens over two turns, 17,792 of them cached.
	home := writeGrokHome(t, map[string]map[string]string{
		"s1": {"updates.jsonl": grokTurn(17415, 35, 6272, 0, 30) + grokTurn(17386, 37, 11520, 0, 32)},
	})
	got := runGrokIn(t, home)

	for _, want := range []string{"34,801", "51%"} {
		if !strings.Contains(got, want) {
			t.Errorf("output does not carry %q:\n%s", want, got)
		}
	}
	if !strings.Contains(strings.ToLower(got), "reasoning") {
		t.Errorf("62 reasoning tokens were read and never surfaced:\n%s", got)
	}
}

// Grok states costUsdTicks on every record and its own client documentation
// states the scale, but nothing here has checked that against a statement of
// account. A dollar figure would be a number nobody has verified wearing the
// authority of one that was.
func TestGrokCommandNeverPrintsDollars(t *testing.T) {
	home := writeGrokHome(t, map[string]map[string]string{
		"s1": {"updates.jsonl": grokTurn(17415, 35, 6272, 0, 30)},
	})
	got := runGrokIn(t, home)
	if strings.Contains(got, "$") {
		t.Errorf("a dollar figure was printed from a scale nothing here has checked:\n%s", got)
	}
	if !strings.Contains(got, "costUsdTicks") {
		t.Errorf("the output does not say why it reports no money:\n%s", got)
	}
}

// The cache is INSIDE the prompt on this surface. Dividing by the fresh
// remainder instead of the whole prompt puts a well-cached turn above 100%,
// which is the arithmetic this pins.
func TestGrokCommandCachedShareStaysWithinTheWhole(t *testing.T) {
	home := writeGrokHome(t, map[string]map[string]string{
		"s1": {"updates.jsonl": grokTurn(6300, 10, 6272, 0, 0)},
	})
	got := runGrokIn(t, home)
	if !strings.Contains(got, "99%") && !strings.Contains(got, "100%") {
		t.Errorf("cached share is not read as a share of the whole prompt:\n%s", got)
	}
}

func TestGrokCommandSaysSoWhenThereIsNothingToRead(t *testing.T) {
	got := runGrokIn(t, t.TempDir())
	if !strings.Contains(got, "No Grok sessions found") {
		t.Errorf("an empty machine was not reported as empty:\n%s", got)
	}
	// Naming where it looked is what turns "nothing here" into something the
	// reader can act on.
	if !strings.Contains(got, ".grok") {
		t.Errorf("the output does not say where it looked:\n%s", got)
	}
}
