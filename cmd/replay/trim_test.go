package main

import (
	"strings"
	"testing"
)

// Everything the PRD required before this half could ship has to reach the
// page, because each item is a way the report would otherwise mislead.
func TestTrimReportCarriesEveryRequiredCaveat(t *testing.T) {
	var sb strings.Builder
	writeTrimNotes(&sb)
	got := sb.String()
	for _, want := range []string{
		"context-edit-trigger", // 5: recommend the provider-sanctioned lever first
		"compaction",           // 4: this does not give you a longer session
		"count_tokens",         //    and here is why
		"LOWER BOUND",          // 2: the probe is a lower bound
		"Write has no",         //    and these are its blind spots
		"Line numbers",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("the report omits %q:\n%s", want, got)
		}
	}
}

// The correction that made this shippable: a resent byte is a cache read, so
// the dollar figure is roughly a tenth of what a token-share report implies.
// Both numbers print, with the ratio, so the reader can see the difference
// rather than be told about it.
func TestTrimReportShowsBothPricesAndTheRatio(t *testing.T) {
	got := trimSummaryLine(0.42, 4.20, 10.0)
	if !strings.Contains(got, "0.42") || !strings.Contains(got, "4.20") {
		t.Fatalf("both figures must appear: %s", got)
	}
	if !strings.Contains(got, "10.0") && !strings.Contains(got, "10x") {
		t.Fatalf("the overstatement ratio must appear: %s", got)
	}
	if !strings.Contains(strings.ToLower(got), "cache-read") {
		t.Fatalf("the smaller figure must say what it is: %s", got)
	}
}

// The command must exist and be reachable, not just compile.
func TestTrimIsWiredIntoDispatchAndHelp(t *testing.T) {
	src := string(mustRead(t, "main.go"))
	if !strings.Contains(src, `case "trim":`) {
		t.Fatal("trim is not in the dispatch switch")
	}
	if !strings.Contains(src, "replay trim") {
		t.Fatal("trim is not in the usage text")
	}
}

// A path is required; trim with no argument is a usage error, not a stat error
// on the subcommand's own name.
func TestTrimNeedsAPath(t *testing.T) {
	err := runTrim([]string{"--cap", "8192"}, io0{}, io0{})
	if err == nil {
		t.Fatal("trim with no path must fail")
	}
	if strings.Contains(err.Error(), "stat trim") {
		t.Fatalf("the subcommand name reached the path list: %v", err)
	}
}

type io0 struct{}

func (io0) Write(p []byte) (int, error) { return len(p), nil }
