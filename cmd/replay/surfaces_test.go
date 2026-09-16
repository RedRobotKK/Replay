package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/surfaces"
)

// The whole point of the verb is that two surfaces which look identical in
// every other tool are printed differently here. If the rendering collapses
// them, the classifier being right upstream does not help a reader.
func TestSurfacesRender_AbsentAndZeroDoNotPrintTheSame(t *testing.T) {
	var buf bytes.Buffer
	renderSurfaces(&buf, []surfaces.Reading{
		{Name: "Codex", Records: 400, Reads: 33_439_104, WriteFieldPresent: false},
		{Name: "Grok", Records: 264, Reads: 343_358_720, WriteFieldPresent: true},
	})
	out := buf.String()
	codex, grok := lineFor(out, "Codex"), lineFor(out, "Grok")
	if codex == "" || grok == "" {
		t.Fatalf("both surfaces must appear; got:\n%s", out)
	}
	if codex == grok {
		t.Fatal("a surface with no write field printed identically to one whose write field is " +
			"present and zero; that is the distinction this verb exists to draw")
	}
	if !strings.Contains(codex, string(surfaces.StateWriteFieldAbsent)) {
		t.Errorf("Codex line does not name its state: %q", codex)
	}
	if !strings.Contains(grok, string(surfaces.StateWriteReportedZero)) {
		t.Errorf("Grok line does not name its state: %q", grok)
	}
}

// A surface that reports nothing must be printed as reporting nothing, never
// as a row of zeros that a reader would take for a measurement.
func TestSurfacesRender_SilentIsNotPrintedAsZero(t *testing.T) {
	var buf bytes.Buffer
	renderSurfaces(&buf, []surfaces.Reading{{Name: "Cursor", Records: 0}})
	line := lineFor(buf.String(), "Cursor")
	if strings.Contains(line, " 0 ") && !strings.Contains(line, string(surfaces.StateSilent)) {
		t.Fatalf("a silent surface printed as zeros: %q", line)
	}
	if !strings.Contains(line, string(surfaces.StateSilent)) {
		t.Fatalf("Cursor line does not name its state: %q", line)
	}
}

// Nothing found at all is a result, and the verb must say so rather than
// printing an empty table that reads as a clean bill of health.
func TestSurfacesRender_NoSurfacesFoundSaysSo(t *testing.T) {
	var buf bytes.Buffer
	renderSurfaces(&buf, nil)
	if !strings.Contains(strings.ToLower(buf.String()), "no agent") {
		t.Fatalf("an empty scan printed nothing a reader could act on: %q", buf.String())
	}
}

func lineFor(out, name string) string {
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, name) {
			return l
		}
	}
	return ""
}
