package cachemodel

import (
	"strings"
	"testing"
	"time"
)

// PriceTableVersion's own documentation says every dollar figure "is only as
// current as the table". Replay cites the date and stops there, leaving the
// reader to work out that a date is months old — which is a weaker standard
// than the tool applies to everything else it prints: it declines routing
// advice below ten sessions and refuses a dollar figure it would have to guess
// at, rather than printing one with a caveat.
func TestPriceTableAgeNote(t *testing.T) {
	table, err := time.Parse("2006-01-02", PriceTableVersion)
	if err != nil {
		t.Fatalf("PriceTableVersion is not a date: %v", err)
	}

	if note := PriceTableAgeNote(table.AddDate(0, 0, 3)); note != "" {
		t.Errorf("a three-day-old table should not be flagged, got %q", note)
	}

	old := table.AddDate(0, 0, PriceTableStaleDays+1)
	note := PriceTableAgeNote(old)
	if note == "" {
		t.Fatalf("a table %d days old should be flagged", PriceTableStaleDays+1)
	}
	if !strings.Contains(note, "days old") {
		t.Errorf("the note should say how old, got %q", note)
	}
	if !strings.Contains(note, "replay rules") {
		t.Errorf("the note should name the command that updates it, got %q", note)
	}
}

// A clock behind the table's own date must not produce a negative age.
func TestPriceTableAgeNoteClockBehind(t *testing.T) {
	table, _ := time.Parse("2006-01-02", PriceTableVersion)
	if note := PriceTableAgeNote(table.AddDate(0, 0, -30)); note != "" {
		t.Errorf("a clock behind the table should be silent, got %q", note)
	}
}
