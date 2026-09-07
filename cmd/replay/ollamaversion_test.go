package main

import "testing"

// TestOV1: no line exceeds the width, and continuations are indented.
func TestOV1(t *testing.T) {
	in := "This ollama is 0.33.2 and the behaviour below was read from ollama 0.33.3 on 2026-09-07. Defaults move between releases."
	got := wrapAt(in, 40, "  ")
	for i, l := range splitLines(got) {
		if len(l) > 42 { // width plus the indent
			t.Errorf("line %d is %d wide: %q", i+1, len(l), l)
		}
		if i > 0 && l[:2] != "  " {
			t.Errorf("continuation line %d is not indented: %q", i+1, l)
		}
	}
	if len(splitLines(got)) < 2 {
		t.Fatal("nothing wrapped, so this test proves nothing")
	}
}

// TestOV2: an empty note stays empty rather than becoming a blank line.
func TestOV2(t *testing.T) {
	if got := wrapAt("", 40, "  "); got != "" {
		t.Errorf("empty in, %q out", got)
	}
}

func splitLines(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == '\n' {
			out = append(out, cur)
			cur = ""
			continue
		}
		cur += string(r)
	}
	return append(out, cur)
}
