package cachemodel

import (
	"strings"
	"testing"
	"time"
)

// A table can be old and still be right, and the report must be able to say so.
//
// The published price table is dated 2026-06-24. Seventy-four days later
// `rules --check-prices` compared it against an independent database of 28
// first-party models and found no disagreement on any of the 11 it covers. So
// the numbers are current; only the fetch is old.
//
// With one date, those two facts are indistinguishable and the report says
// "74 days old; check it against current rates" to somebody who checked it
// this morning. That reads as abandonment, and it is the sentence a buyer sees
// on a feed being sold on freshness.
//
// So there are two clocks. fetchedAt is when the provider's own page was read;
// it does not move because a second observer agreed. checkedAt is when the
// table was last verified against that observer, and it is the heartbeat.
// Conflating them would mean bumping fetchedAt on a LiteLLM check, which
// claims a source that was not read.

// C1: the age note distinguishes stale-and-unverified from old-but-checked.
//
// PASS: an old table with a recent check reports the check; an old table with
// no check keeps the original warning.
// FAIL: the two cases print the same thing, which is what one date forces.
func TestC1_AnOldTableThatWasCheckedSaysSo(t *testing.T) {
	now := time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC)

	unchecked := PriceTableAgeNoteAt(now, "")
	if !strings.Contains(unchecked, "days old") {
		t.Errorf("an unverified old table must still warn: %q", unchecked)
	}

	checked := PriceTableAgeNoteAt(now, "2026-09-06")
	if checked == unchecked {
		t.Fatal("a table verified today reads identically to one nobody has looked at since June")
	}
	if !strings.Contains(checked, "checked") {
		t.Errorf("the note does not say it was checked: %q", checked)
	}
	if strings.Contains(checked, "check it against current rates") {
		t.Errorf("still telling the reader to do the thing that was just done: %q", checked)
	}
}

// C2: a check that is itself stale stops counting.
//
// The heartbeat is only worth anything while it beats. A checkedAt from three
// months ago must not suppress the warning, or the field becomes a way to
// silence the staleness notice permanently by writing a date once.
//
// PASS: an old check warns like no check at all.
// FAIL: any past date suppressing the warning.
func TestC2_AStaleCheckDoesNotSuppressTheWarning(t *testing.T) {
	now := time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC)
	old := PriceTableAgeNoteAt(now, "2026-05-01")
	if !strings.Contains(old, "days old") {
		t.Errorf("a check from four months ago suppressed the warning: %q", old)
	}
}

// C3: a malformed checkedAt is not a valid check.
//
// PASS: garbage is treated as absent, so the warning stands.
// FAIL: an unparseable date silencing the notice — the failure direction that
// hides staleness rather than over-reporting it.
func TestC3_AMalformedCheckIsTreatedAsNoCheck(t *testing.T) {
	now := time.Date(2026, 9, 6, 0, 0, 0, 0, time.UTC)
	for _, bad := range []string{"soon", "2026-13-45", "26-09-06", " "} {
		if got := PriceTableAgeNoteAt(now, bad); !strings.Contains(got, "days old") {
			t.Errorf("checkedAt %q suppressed the staleness warning: %q", bad, got)
		}
	}
}

// C4: a fresh table needs no note at all, checked or not.
//
// PASS: silence within the stale window.
// FAIL: a note on a table nobody needs told about.
func TestC4_AFreshTableSaysNothing(t *testing.T) {
	soon := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	if got := PriceTableAgeNoteAt(soon, ""); got != "" {
		t.Errorf("a table within the window should print nothing: %q", got)
	}
	if got := PriceTableAgeNoteAt(soon, "2026-07-01"); got != "" {
		t.Errorf("a fresh checked table should print nothing: %q", got)
	}
}

// C5: the exported document carries the check date.
//
// The feed is what a buyer reads. If checkedAt lives only in the binary, the
// published document still looks abandoned.
//
// PASS: ExportRules emits it and it parses.
// FAIL: absent, or not a date.
func TestC5_TheExportedFeedCarriesTheCheckDate(t *testing.T) {
	doc := ExportRules()
	if doc.CheckedAt == "" {
		t.Fatal("the exported feed carries no checkedAt, so a reader cannot tell " +
			"a maintained table from an abandoned one")
	}
	if _, err := time.Parse("2006-01-02", doc.CheckedAt); err != nil {
		t.Errorf("checkedAt %q does not parse as a date: %v", doc.CheckedAt, err)
	}
	if doc.FetchedAt == "" {
		t.Error("fetchedAt disappeared; the two clocks are both needed")
	}
}
