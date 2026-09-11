package main

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"time"
)

// `replay --help` is a published surface, and a flag default that changes with
// the clock cannot be published.
//
// `-pooled-at` defaulted to timeNow().UTC().Format("2006-01-02"), so
// flag.PrintDefaults rendered today's date into the help text. docs/CLI.md is
// generated from that help and committed; scripts/cli-blueprint/check.sh
// regenerates and diffs. The check therefore compared a frozen file against a
// value that changes at 00:00 UTC, and failed on pristine main every day from
// midnight until somebody regenerated the file — which would go green for one
// day and fail again the next.
//
// Observed: main's last green run started at 23:58:37Z and the rollover to
// 2026-09-11 happened 83 seconds later. The first run after it failed with one
// line of diff, on a tree nobody had touched.
//
// The dated default itself is not the defect and is not removed. internal's
// NewPool wants a date rather than a clock read, so a regenerated document can
// be diffed against a published one. What changed is WHERE the date is
// resolved: at use, not in the flag's printed default.

var aDate = regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)

// PM1: the help text carries no date.
func TestPM1_PoolHelpIsStableAcrossMidnight(t *testing.T) {
	var out bytes.Buffer
	_ = runPool([]string{"-h"}, &out, &out)
	got := out.String()
	if !strings.Contains(got, "pooled-at") {
		t.Fatalf("the help does not mention the flag, so this test checks nothing:\n%s", got)
	}
	if m := aDate.FindString(got); m != "" {
		t.Errorf("the help bakes in the date %q, so docs/CLI.md goes stale at the "+
			"next UTC midnight:\n%s", m, got)
	}
}

// PM2: and it still says what the default is, in words.
//
// Removing the date must not remove the answer. A reader asking what happens
// when they omit the flag has to be told.
func TestPM2_TheHelpStillSaysWhatTheDefaultIs(t *testing.T) {
	var out bytes.Buffer
	_ = runPool([]string{"-h"}, &out, &out)
	got := strings.ToLower(out.String())
	if !strings.Contains(got, "today") {
		t.Errorf("the help no longer says an unset flag means today:\n%s", out.String())
	}
}

// PM3: the behaviour is unchanged — an unset flag still records today.
//
// This is the property the dated default existed for, and it has to survive the
// move. Pinned against the same clock seam the flag used to read.
func TestPM3_AnUnsetFlagStillRecordsToday(t *testing.T) {
	frozen := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	restore := timeNow
	timeNow = func() time.Time { return frozen }
	defer func() { timeNow = restore }()

	if got := pooledAtOr(""); got != "2026-03-04" {
		t.Errorf("an unset --pooled-at recorded %q, want the frozen today", got)
	}
	// And an explicit value is passed through untouched, because a caller
	// regenerating a published document supplies the date it was published on.
	if got := pooledAtOr("2026-01-02"); got != "2026-01-02" {
		t.Errorf("an explicit --pooled-at was overwritten with %q", got)
	}
}

// PM4: no flag default anywhere in the binary contains a date.
//
// The class, not the instance. docs/CLI.md is generated from every subcommand's
// help and committed, so ANY flag whose printed default moves with the clock
// makes a published file go stale on a schedule — and the failure appears on a
// tree nobody touched, which is the hardest kind to attribute.
//
// Scanned from the rendered help of every dispatched command rather than from
// the source, because what matters is what PrintDefaults writes.
func TestPM4_NoFlagDefaultCarriesADate(t *testing.T) {
	cmds := []string{
		"cost", "pool", "corpus", "probe", "rules", "advise", "learn", "serve",
		"context", "trim", "route", "burn", "since", "budget", "purge", "privacy",
		"prefix", "upgrade", "tui", "mcp", "agents", "statusline", "ceiling",
	}
	seen := 0
	for _, c := range cmds {
		var out bytes.Buffer
		_ = run([]string{c, "-h"}, &out, &out)
		help := out.String()
		if !strings.Contains(help, "-") {
			continue // no flags rendered for this one
		}
		seen++
		for _, line := range strings.Split(help, "\n") {
			if !strings.Contains(line, "(default") {
				continue
			}
			if m := aDate.FindString(line); m != "" {
				t.Errorf("%s renders the date %q in a flag default, so docs/CLI.md "+
					"goes stale at the next UTC midnight: %s", c, m, strings.TrimSpace(line))
			}
		}
	}
	if seen < 10 {
		t.Fatalf("only %d command(s) rendered help, so this scan is not covering the "+
			"flag surface", seen)
	}
}
