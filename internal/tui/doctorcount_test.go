package tui

import (
	"strings"
	"testing"
)

// The doctor screen and `replay doctor` count the same corpus. They must not
// report it differently.
//
// Measured on a real machine before this file existed:
//
//	$ replay doctor
//	transcripts   124 sessions across 12 projects
//
//	$ replay tui --screen doctor
//	  1,809 transcripts across 12 projects
//
// Two near-identical sentences, one order of magnitude apart, about one
// directory. The command counts sessions; the screen folded sub-agent lanes in
// and kept the word "transcripts", so a reader who ran both had to conclude one
// of the two was broken — and had nothing to tell them which.
//
// The command already gets this right: it prints sessions, then the file count
// separately, and only when the two differ. The screen now says the same two
// numbers with the same two words.

func aFannedOutMachine() Machine {
	m := aMachine()
	m.Sessions, m.Lanes, m.Transcripts = 124, 1685, 1809
	return m
}

// DC1: the headline counts sessions, because that is what the command counts.
func TestDoctorHeadlineCountsSessions(t *testing.T) {
	body := DoctorScreen(aFannedOutMachine()).String()
	if !strings.Contains(body, "124 sessions across 12 projects") {
		t.Errorf("the headline does not report the session count `replay doctor` "+
			"reports:\n%s", body)
	}
	if strings.Contains(body, "1,809 transcripts across") {
		t.Errorf("the headline still calls the file count transcripts, which is the "+
			"figure the command does not print:\n%s", body)
	}
}

// DC2: and the file count is still shown, as files, where the fan-out is real.
//
// Dropping it would trade one wrong number for one missing one: `replay cost`
// does read every lane file, so a reader comparing this screen to that report
// needs the larger figure to be somewhere.
func TestDoctorStillShowsTheFileCountAsFiles(t *testing.T) {
	body := DoctorScreen(aFannedOutMachine()).String()
	for _, want := range []string{"1,809 files", "1,685"} {
		if !strings.Contains(body, want) {
			t.Errorf("the screen does not carry %q, so the fan-out is invisible:\n%s", want, body)
		}
	}
}

// DC3: with no sub-agent lanes the two figures are the same figure, and the
// screen does not manufacture a distinction where there is none.
func TestDoctorSaysItOnceWhenThereIsNoFanOut(t *testing.T) {
	m := aMachine()
	m.Sessions, m.Lanes, m.Transcripts = 12, 0, 12
	body := DoctorScreen(m).String()
	if !strings.Contains(body, "12 sessions across") {
		t.Errorf("headline lost its count:\n%s", body)
	}
	if strings.Contains(body, "lanes") && !strings.Contains(body, "sub-agent lanes") {
		t.Errorf("unexpected lane wording:\n%s", body)
	}
}

// DC4: a screen told a session count of zero and a file count above it is
// reporting something incoherent, and says so rather than picking one.
//
// This is the shape the defect took: two fields that must agree, populated
// independently, with nothing checking. The screen cannot fix the walk, but it
// can refuse to present the contradiction as a finding.
func TestDoctorDoesNotInventASessionCount(t *testing.T) {
	m := aMachine()
	m.Sessions, m.Lanes, m.Transcripts = 0, 0, 1809
	body := DoctorScreen(m).String()
	if strings.Contains(body, "0 sessions across") {
		t.Errorf("reported zero sessions beside 1,809 files as though both were "+
			"measured:\n%s", body)
	}
}
