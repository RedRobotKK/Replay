package analysis

import (
	"bytes"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// Counting a provider failure without reporting it would leave the defect in
// place.
//
// Session.Refusals is the cautionary precedent: 874f026 moved local refusals
// out of Skipped and nothing in production ever read the new field, so the
// misleading line stopped counting them and no line replaced it. A reader
// learned less than before. The count is only half the fix.
//
// The two cases are reported apart because they are different observations.
// One is a status the provider sent. The other is the absence of any status,
// and printing that as 0 would be inventing a response nobody received.

// pfReport builds the smallest report whose header can be written.
func pfReport(pf transcript.ProviderFailures, skipped int) string {
	req := &transcript.Request{Model: "claude-opus-5"}
	lane := &transcript.Lane{ID: "", Requests: []*transcript.Request{req}}
	sess := &transcript.Session{
		ID:               "sess-1",
		ClientVersion:    "1.0.0",
		Source:           transcript.SourceLedger,
		Lanes:            []*transcript.Lane{lane},
		Skipped:          skipped,
		ProviderFailures: pf,
	}
	r := &LaneReport{Session: sess, Lane: lane, Calibration: &Calibration{}}
	var b bytes.Buffer
	p := NewPrinter(&b)
	r.header(p)
	return b.String()
}

// pfOnly is only what providerFailures emitted.
//
// The assertions below are about what THIS function claims. Scanning the whole
// header would read the pre-existing Assumption and Calibration lines too, and
// fail on words those lines have always carried.
func pfOnly(pf transcript.ProviderFailures) string {
	var keep []string
	for _, line := range strings.Split(pfReport(pf, 0), "\n") {
		if strings.Contains(line, "HTTP status") {
			keep = append(keep, line)
		}
	}
	return strings.Join(keep, "\n")
}

// PFR1: statuses at or above 400 are reported, and the observed status is what
// is printed.
func TestPFR1_StatusFailuresAreReportedWithTheirObservedStatus(t *testing.T) {
	out := pfReport(transcript.ProviderFailures{ByStatus: map[int]int{429: 2, 500: 1}}, 0)
	if !strings.Contains(out, "429") || !strings.Contains(out, "500") {
		t.Errorf("the observed statuses are not in the report:\n%s", out)
	}
	if !strings.Contains(out, "3") {
		t.Errorf("the total is not reported:\n%s", out)
	}
}

// PFR2: a failure with no status is reported separately and never as a status.
//
// This is the one that must not regress into "status 0". Zero is not a status
// the provider sent; it is the absence of one.
func TestPFR2_ATransportFailureIsNeverRenderedAsAnHTTPStatus(t *testing.T) {
	out := pfOnly(transcript.ProviderFailures{NoStatus: 2})
	if !strings.Contains(out, "no HTTP status") {
		t.Errorf("a status-less failure is not reported as such:\n%s", out)
	}
	for _, banned := range []string{"status 0", "status of 0", " 0 ", "HTTP 0"} {
		if strings.Contains(out, banned) {
			t.Errorf("zero was rendered as an HTTP status (%q):\n%s", banned, out)
		}
	}
}

// PFR3: the two kinds are reported apart, not summed into one figure.
func TestPFR3_TheTwoKindsAreNotCollapsed(t *testing.T) {
	out := pfReport(transcript.ProviderFailures{ByStatus: map[int]int{401: 1}, NoStatus: 1}, 0)
	if !strings.Contains(out, "401") {
		t.Errorf("the observed status is missing:\n%s", out)
	}
	if !strings.Contains(out, "no HTTP status") {
		t.Errorf("the status-less failure is missing:\n%s", out)
	}
	// A single "2 provider failures" line would satisfy neither reader: one
	// wants to know which status, the other that there was none.
	if strings.Count(out, "recorded request") < 2 {
		t.Errorf("the two kinds were collapsed into one statement:\n%s", out)
	}
}

// PFR4: nothing is said when there is nothing to say.
func TestPFR4_NoLineWhenThereAreNoProviderFailures(t *testing.T) {
	out := pfReport(transcript.ProviderFailures{}, 0)
	if strings.Contains(out, "HTTP status") {
		t.Errorf("a provider-failure line appeared with no failures:\n%s", out)
	}
}

// PFR5: the report claims only what was observed.
//
// A status is not a cause. The record carries no usage, which is absence
// rather than zero, and nothing about billing was observed at all.
func TestPFR5_TheReportClaimsNoCauseAndNoAccounting(t *testing.T) {
	out := strings.ToLower(pfOnly(transcript.ProviderFailures{
		ByStatus: map[int]int{401: 1, 429: 1, 500: 1}, NoStatus: 1,
	}))
	for _, banned := range []string{
		"authentication", "unauthorized", "quota", "rate limit", "rate-limited",
		"billed", "billing", "cost", "$", "savings", "token", "usage",
		"skipped", "not conversation content", "lost", "could not be read",
		"retry", "retried",
	} {
		if strings.Contains(out, banned) {
			t.Errorf("the report claims %q, which the record does not establish:\n%s", banned, out)
		}
	}
}

// PFR6: the existing Skipped line is unchanged and still means what it meant.
func TestPFR6_TheSkippedLineIsUnchanged(t *testing.T) {
	out := pfReport(transcript.ProviderFailures{ByStatus: map[int]int{429: 1}}, 3)
	if !strings.Contains(out, "3 transcript lines were not conversation content and were skipped") {
		t.Errorf("the existing Skipped line changed:\n%s", out)
	}
}

// PFR7: a usage-bearing failure never appears in the provider-failure lines.
//
// Such a record is classified as an ordinary request upstream of the report, so
// the counters it would have to pass through are empty. This pins the report
// side of that decision: if the classification ever started counting them, this
// says so here rather than leaving it to the ledger tests alone.
func TestPFR7_UsageBearingFailuresAreNotReportedAsProviderFailures(t *testing.T) {
	// Nothing classified, because the record stayed usage-bearing.
	out := pfReport(transcript.ProviderFailures{}, 0)
	if strings.Contains(out, "HTTP status") {
		t.Errorf("a provider-failure line appeared for a session whose only failing "+
			"request carried usage:\n%s", out)
	}
}

// PFR8: several statuses stay several statuses in the report.
func TestPFR8_EachObservedStatusIsReported(t *testing.T) {
	out := pfOnly(transcript.ProviderFailures{ByStatus: map[int]int{401: 1, 429: 3, 500: 2}})
	for _, want := range []string{"401", "429", "500"} {
		if !strings.Contains(out, want) {
			t.Errorf("status %s is missing from the report:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "6 recorded requests") {
		t.Errorf("the total does not match the statuses:\n%s", out)
	}
}
