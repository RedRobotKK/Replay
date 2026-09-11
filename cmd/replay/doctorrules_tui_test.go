package main

import (
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/tui"
)

// The command and the screen answer the same question, so this file's job is to
// keep them from answering it differently.
//
// `replay doctor` reports the rules document through rulesNotice. The TUI
// doctor screen reports it through tui.Machine. Two code paths, one fact — and
// the way that fact goes wrong is not that one of them breaks, it is that one
// of them stops being called and nobody notices, because a missing row looks
// like a clean bill of health.

// W1: machineState always says which rules document is in effect.
//
// The screen draws no row when it was not told (absence is not "compiled in"),
// which is the right rendering rule and exactly the rule that would let this
// wiring rot silently. This is the test that notices.
func TestMachineStateReportsTheRulesDocument(t *testing.T) {
	m := machineState()
	if m.RulesVersion == "" {
		t.Error("machineState names no rules document, so the doctor screen draws no " +
			"rules row and an operator on a months-old table is told nothing")
	}
	if m.RulesState == tui.RulesUntold {
		t.Error("machineState left the rules state unset")
	}
}

// W2: and it classifies a date the same way the printed notice does.
//
// Both go through parseFetchedAt. This asserts they still do, by checking the
// two outputs agree on every state — including the one that matters, where a
// date this build cannot read must not become a fresh one.
func TestRulesAgeAgreesWithTheDoctorNotice(t *testing.T) {
	now := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		name      string
		fetchedAt string
		want      tui.RulesAge
		notice    string
	}{
		{"no document installed", "", tui.RulesBuiltIn, "compiled in"},
		{"rfc3339", "2026-08-01T00:00:00Z", tui.RulesFetched, "days ago"},
		{"bare date", "2026-08-01", tui.RulesFetched, "days ago"},
		{"unreadable", "last tuesday", tui.RulesUndated, "unreadable"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, days := rulesAge(c.fetchedAt, now)
			if got != c.want {
				t.Errorf("rulesAge(%q) = %v, want %v", c.fetchedAt, got, c.want)
			}
			if n := rulesNotice("anthropic-2026-09-01", c.fetchedAt, now); !strings.Contains(n, c.notice) {
				t.Errorf("rulesNotice(%q) does not say %q, so the two surfaces disagree:\n%s",
					c.fetchedAt, c.notice, n)
			}
			if c.want == tui.RulesFetched && days != 40 {
				t.Errorf("aged %q at %d days, want 40", c.fetchedAt, days)
			}
			if c.want != tui.RulesFetched && days != 0 {
				t.Errorf("a %v document was given an age of %d days", c.want, days)
			}
		})
	}
}

// W3: the screen crosses the stale line on the same day the notice does.
//
// Thirty days lives in cmd as a duration and in the tui as a day count. Two
// spellings of one threshold is how an operator gets told their table is fine
// by one surface and stale by the other.
func TestBothSurfacesGoStaleOnTheSameDay(t *testing.T) {
	if got := int(rulesStaleAfter.Hours() / 24); got != tui.RulesStaleDays() {
		t.Errorf("replay doctor goes stale at %d days, the doctor screen at %d",
			got, tui.RulesStaleDays())
	}
}
