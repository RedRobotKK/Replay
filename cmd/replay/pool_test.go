package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/observation"
)

// corpusFixture builds a valid submission, digested the way the writer digests
// one, so the digest a test hands the pool is the digest the pool recomputes.
func corpusFixture(tag string, tasks int, total, rebilled float64) string {
	c := observation.Corpus{
		Schema:        observation.CorpusSchema,
		TakenAt:       "2026-09-10T04:00:00Z",
		Tasks:         tasks,
		TotalUSD:      total,
		RebilledUSD:   rebilled,
		RebilledShare: rebilled / total,
		MedianTaskUSD: total / float64(tasks),
		PricedAt:      "2026-09-07",
		RulesVersion:  "anthropic-2026-09-01",
		SourceTag:     tag,
		TagBasis:      "local",
	}.Digested()
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(b) + "\n"
}

func writeSubmission(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// A submission that nothing reads is a donation to a claim nobody makes.
//
// internal/observation.Pool has been complete since it was written — totals as
// methods over the roster so no drifted figure can be stored, digests
// re-derived on Add, a RulesVersion refusal — and it had no caller anywhere in
// the binary. `cost --contribute` wrote files; nothing read one back. The
// unwired-packages guard could not see it, because it asks whether a PACKAGE is
// reachable and internal/observation is: contribute.go imports Corpus, and that
// one live type vouched for the dead one beside it.
func TestPoolAggregatesSubmissions(t *testing.T) {
	dir := t.TempDir()
	a := writeSubmission(t, dir, "a.json", corpusFixture("aaaaaaaaaaaaaaaa", 10, 100, 5))
	b := writeSubmission(t, dir, "b.json", corpusFixture("bbbbbbbbbbbbbbbb", 20, 200, 10))

	var out, errOut strings.Builder
	if err := runPool([]string{a, b}, &out, &errOut); err != nil {
		t.Fatalf("pool: %v\nstderr: %s", err, errOut.String())
	}
	got := out.String()
	if !strings.Contains(got, "$300.00") {
		t.Errorf("the pooled total must be the sum of the roster, $300.00:\n%s", got)
	}
	if !strings.Contains(got, "2 contributed corpora") {
		t.Errorf("the n must travel with the total:\n%s", got)
	}
	// The roster is the point: a figure whose parts a reader can fetch.
	for _, want := range []string{"a.json", "b.json"} {
		if !strings.Contains(got, want) {
			t.Errorf("the roster must name %q so the total can be walked back to its parts:\n%s", want, got)
		}
	}
}

// An empty pool must refuse, not render a zero.
//
// Totals() already refuses; this pins that the command surfaces the refusal
// rather than printing a $0.00 across 0 submissions that looks exactly like a
// real result.
func TestPoolRefusesToRenderNothing(t *testing.T) {
	var out, errOut strings.Builder
	err := runPool([]string{}, &out, &errOut)
	if err == nil {
		t.Fatal("pooling no files returned no error; a zero total that renders like a " +
			"measured one is this repository's oldest defect")
	}
	if strings.Contains(out.String(), "$0.00") {
		t.Errorf("a zero total was rendered:\n%s", out.String())
	}
}

// A submission that cannot be pooled is named, not skipped.
//
// Add refuses a mismatched digest and a disagreeing rulesVersion. If the
// command swallowed those, the pooled figure would silently cover fewer
// submissions than the operator handed it, which is the failure mode the whole
// roster design exists to prevent.
func TestPoolNamesWhatItRefused(t *testing.T) {
	dir := t.TempDir()
	good := writeSubmission(t, dir, "good.json", corpusFixture("cccccccccccccccc", 10, 100, 5))
	bad := writeSubmission(t, dir, "bad.json", strings.Replace(
		corpusFixture("dddddddddddddddd", 10, 100, 5), `"digest": "`, `"digest": "0`, 1))

	var out, errOut strings.Builder
	err := runPool([]string{good, bad}, &out, &errOut)
	if err != nil {
		t.Fatalf("one bad submission must not fail the whole run: %v", err)
	}
	if !strings.Contains(errOut.String(), "bad.json") {
		t.Errorf("the refused submission must be named on stderr:\n%s", errOut.String())
	}
	if !strings.Contains(out.String(), "1 contributed corpora") {
		t.Errorf("the total must cover only what was admitted:\n%s", out.String())
	}
}

// A file that cannot be read is named, and the run continues.
//
// The unreadable-file branch and the refused-submission branch print the same
// sentence, so a mutation that turned one into `return err` survived: no test
// handed the command a path that does not exist. A pool run over a directory of
// contributions should not be ended by one missing file, and should never be
// ended silently.
func TestPoolContinuesPastAnUnreadableFile(t *testing.T) {
	dir := t.TempDir()
	good := writeSubmission(t, dir, "good.json", corpusFixture("eeeeeeeeeeeeeeee", 10, 100, 5))
	missing := filepath.Join(dir, "not-here.json")

	var out, errOut strings.Builder
	if err := runPool([]string{missing, good}, &out, &errOut); err != nil {
		t.Fatalf("one unreadable file must not end the run: %v", err)
	}
	if !strings.Contains(errOut.String(), "not-here.json") {
		t.Errorf("the unreadable file must be named on stderr:\n%s", errOut.String())
	}
	if !strings.Contains(out.String(), "1 contributed corpora") {
		t.Errorf("the total must still cover what was admitted:\n%s", out.String())
	}
}

// The roster names the submission, not the reader's filesystem.
//
// A roster row exists so a reader can fetch the file it names. An absolute path
// from the machine that assembled the pool is not fetchable by anyone else, and
// it discloses the assembler's directory layout into a document meant to be
// published.
func TestPoolRosterNamesTheFileNotThePath(t *testing.T) {
	dir := t.TempDir()
	f := writeSubmission(t, dir, "sub.json", corpusFixture("ffffffffffffffff", 10, 100, 5))

	var out, errOut strings.Builder
	if err := runPool([]string{f}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "sub.json") {
		t.Errorf("the roster must name the submission:\n%s", out.String())
	}
	if strings.Contains(out.String(), dir) {
		t.Errorf("the roster leaked the assembling machine's directory %q into a document\n"+
			"meant to be published:\n%s", dir, out.String())
	}
}

// An error share over no requests is absent, not zero.
//
// The guard was three lines inlined at the call site, where nothing could reach
// it: a mutation that computed the share unconditionally survived because no
// test could construct a corpus with zero requests through the cost path. ADR-
// 0014's rule is that the untestable shape is the vulnerability, so the
// decision is a function now.
func TestErrorShareOverNoRequestsIsAbsent(t *testing.T) {
	if got := errorShare(0, 0); got != nil {
		t.Errorf("errorShare(0, 0) = %v, want nil: a share over no requests is a "+
			"division, and 0.0 would tell a pool this corpus had no errors when "+
			"nothing was counted", *got)
	}
	if got := errorShare(5, 0); got != nil {
		t.Errorf("errorShare(5, 0) = %v, want nil", *got)
	}
	got := errorShare(0, 100)
	if got == nil || *got != 0 {
		t.Errorf("a measured zero must be reported as zero, not dropped: %v", got)
	}
	if got := errorShare(8, 100); got == nil || *got != 0.08 {
		t.Errorf("errorShare(8, 100) = %v, want 0.08", got)
	}
}

// The unreadable-file refusal and the not-a-submission refusal are different
// guards, and each must be reachable on its own.
//
// They print the same shape of line, so a test that only checks "the file was
// named" is satisfied whichever one fired. Neutralising the read check let the
// nil body fall through to the parse check, which refused it anyway — a guard
// shadowed by the one after it, which is the failure this repository has hit
// before.
func TestPoolDistinguishesUnreadableFromUnparseable(t *testing.T) {
	dir := t.TempDir()
	good := writeSubmission(t, dir, "good.json", corpusFixture("1111111111111111", 10, 100, 5))
	junk := writeSubmission(t, dir, "junk.json", "this is not json at all\n")
	missing := filepath.Join(dir, "gone.json")

	var out, errOut strings.Builder
	if err := runPool([]string{missing, junk, good}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	e := errOut.String()
	if !strings.Contains(e, "not a submission") {
		t.Errorf("a file that is not JSON must say so, not merely be named:\n%s", e)
	}
	// The read failure must be reported and kept distinct from a parse failure.
	// It is NOT asserted against the OS's own phrasing of "file not found" —
	// that string is "no such file" on Unix and "cannot find the file" on
	// Windows, and pinning either makes a portable test fail on the other
	// platform. The portable invariant is that the missing file is named and
	// takes the read branch, so "not a submission" (the parse branch) appears
	// exactly once, for the junk file alone.
	if !strings.Contains(e, "gone.json") {
		t.Errorf("an unreadable file must be named on stderr:\n%s", e)
	}
	if n := strings.Count(e, "not a submission"); n != 1 {
		t.Errorf("the parse-failure message should appear once (the junk file), not %d — "+
			"the missing file must take the read branch, not the parse branch:\n%s", n, e)
	}
}

// --json emits the pooled document, and refuses an empty pool before it.
func TestPoolJSONOutput(t *testing.T) {
	dir := t.TempDir()
	f := writeSubmission(t, dir, "one.json", corpusFixture("2222222222222222", 10, 100, 5))

	var out, errOut strings.Builder
	if err := runPool([]string{"--json", f}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(out.String()), &doc); err != nil {
		t.Fatalf("--json did not emit a document: %v\n%s", err, out.String())
	}
	if doc["schema"] == nil || doc["roster"] == nil {
		t.Errorf("the pooled document must carry its schema and roster:\n%s", out.String())
	}
	// The table must NOT be what --json prints.
	if strings.Contains(out.String(), "contributed corpora") {
		t.Errorf("--json printed the table as well:\n%s", out.String())
	}

	var out2, err2 strings.Builder
	if err := runPool([]string{"--json"}, &out2, &err2); err == nil {
		t.Error("--json over an empty pool emitted a document; a roster of nothing is not a figure")
	}
}
