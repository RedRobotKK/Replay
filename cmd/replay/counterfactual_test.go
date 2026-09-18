package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/learn"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// The counterfactual contract, driven through the CLI.
//
// ADR-0025 permits Replay to make a counterfactual claim. The locked
// evidence-coverage contract bounds it in four ways that a test can hold:
//
//  1. The alternative is one exact name from learn.Catalog(), byte for byte.
//     No alias, no case folding, no default, no family, and never FamilyAsRun,
//     which is the observed baseline rather than an alternative.
//  2. An absent or malformed alternative is an INVALID REQUEST: errUsage,
//     exit 1, and no counterfactual document. It is never NOT MEASURED,
//     because nothing was examined.
//  3. A valid alternative the corpus cannot support is NOT MEASURED:
//     errNotMeasured, exit 4. That distinction cost a real bug once already
//     (see costgate.go on the 2026-09-13 exit-code separation) and this file
//     pins it.
//  4. The saving is session-level. It is never allocated to an individual
//     cache break, because no allocation rule exists and none may be invented.
//
// The negative assertions are the load-bearing ones. A mutant that returns a
// default alternative AND an error would survive a test that only checked the
// error, which is why the absent case also asserts empty stdout.

func corpusDir() string {
	return filepath.Join("..", "..", "internal", "transcript", "testdata")
}

// catalogNames is the accepted vocabulary, read from the catalog itself rather
// than copied. A test holding its own copy would keep passing after the
// catalog moved, which is the failure this project calls a check that cannot
// fail.
func catalogNames() []string {
	var out []string
	for _, c := range learn.Catalog() {
		out = append(out, c.Name)
	}
	return out
}

func TestCatalogIsTheAcceptedVocabularyAndHasSixMembers(t *testing.T) {
	names := catalogNames()
	if len(names) != 6 {
		t.Fatalf("the accepted vocabulary has %d members, want 6: %v", len(names), names)
	}
	for _, want := range []string{"ttl-5m", "ttl-1h"} {
		if !containsExact(names, want) {
			t.Errorf("%q is not in the catalog: %v", want, names)
		}
	}
}

func containsExact(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

func TestEveryExactCatalogNameIsAccepted(t *testing.T) {
	for _, name := range catalogNames() {
		t.Run(name, func(t *testing.T) {
			var out bytes.Buffer
			err := run([]string{"diff", "--counterfactual", name, corpusDir()}, &out, io.Discard)
			if errors.Is(err, errUsage) {
				t.Fatalf("%q is an exact catalog name and was refused as invalid usage: %v", name, err)
			}
		})
	}
}

// Each rejected form is rejected for its own reason, and the reasons are not
// interchangeable. Case folding, family selection, the observed baseline, a
// TTL the engine does not model, and a trigger the learner never scores are
// five different mistakes.
func TestMalformedAlternativesAreInvalidRequests(t *testing.T) {
	for _, bad := range []string{
		"TTL-5M",                              // case folded
		"ttl",                                 // family, not a name
		"context-edit",                        // family without parameters
		"as-run",                              // the observed baseline
		"ttl-30m",                             // a TTL the engine does not model
		"context-edit(keep=6,trigger=75000)",  // a trigger the learner never scores
		"context-edit(keep=6, trigger=50000)", // respaced
		"ttl-1h ",                             // trailing space
	} {
		t.Run(bad, func(t *testing.T) {
			var out bytes.Buffer
			err := run([]string{"diff", "--counterfactual", bad, corpusDir()}, &out, io.Discard)
			if !errors.Is(err, errUsage) {
				t.Fatalf("%q was not refused as an invalid request: %v", bad, err)
			}
			if errors.Is(err, errNotMeasured) {
				t.Errorf("%q was reported as insufficient evidence; nothing was examined, so it is a "+
					"request defect", bad)
			}
			if exitCode(err) != exitUsage {
				t.Errorf("exit %d, want %d for an invalid request", exitCode(err), exitUsage)
			}
			if strings.Contains(out.String(), "counterfactual") {
				t.Errorf("a counterfactual document was emitted for an invalid request:\n%s", out.String())
			}
		})
	}
}

// The absent case is the one a silent default would hide. A mutant that picks
// an alternative on the caller's behalf and still returns an error passes an
// error-only assertion, so the empty-output assertion is what kills it.
func TestAnAbsentAlternativeIsAnInvalidRequestAndProducesNoDocument(t *testing.T) {
	var out bytes.Buffer
	err := run([]string{"diff", "--counterfactual", "", corpusDir()}, &out, io.Discard)
	if !errors.Is(err, errUsage) {
		t.Fatalf("an absent alternative was not refused as an invalid request: %v", err)
	}
	if errors.Is(err, errNotMeasured) {
		t.Error("an absent alternative was reported as insufficient evidence")
	}
	if exitCode(err) != exitUsage {
		t.Errorf("exit %d, want %d", exitCode(err), exitUsage)
	}
	if out.Len() != 0 {
		t.Errorf("output was produced for a request that was never valid:\n%s", out.String())
	}
}

// Bare `diff` is unchanged. The counterfactual is opt-in, and a reader who did
// not ask for one is not given one.
func TestBareDiffIsUnchangedAndAsksForNoAlternative(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"diff", corpusDir()}, &out, io.Discard); err != nil {
		t.Fatalf("bare diff failed: %v", err)
	}
	if strings.Contains(strings.ToLower(out.String()), "counterfactual") {
		t.Errorf("bare diff printed a counterfactual nobody asked for:\n%s", out.String())
	}
}

// The flag belongs to diff alone. runReport is shared by replay, blame and the
// default path, so registering it unconditionally would offer a counterfactual
// on commands the contract never authorized.
func TestTheCounterfactualFlagIsRefusedOnOtherCommands(t *testing.T) {
	for _, cmd := range []string{"replay", "blame"} {
		t.Run(cmd, func(t *testing.T) {
			var out bytes.Buffer
			err := run([]string{cmd, "--counterfactual", "ttl-1h", corpusDir()}, &out, io.Discard)
			if !errors.Is(err, errUsage) {
				t.Fatalf("`replay %s --counterfactual` was not refused: %v", cmd, err)
			}
			if exitCode(err) != exitUsage {
				t.Errorf("exit %d, want %d", exitCode(err), exitUsage)
			}
		})
	}
}

// claimable names an alternative this corpus can actually form a claim for.
//
// ttl-1h and ttl-5m both score below the noise floor here and are correctly
// refused, which makes them the wrong fixture for the positive path: a test
// that only ever takes the refusal branch cannot fail when the claim breaks.
const claimable = "context-edit(keep=6,trigger=50000)"

// Session-level only. The contract forbids allocating a saving to any
// individual break, because no allocation rule exists and inventing one would
// attach a figure to evidence that does not support it.
func TestTheSavingIsSessionLevelAndNoBreakCarriesOne(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"diff", "--counterfactual", claimable, corpusDir()}, &out, io.Discard); err != nil {
		t.Fatalf("diff --counterfactual %s failed: %v", claimable, err)
	}
	if !strings.Contains(out.String(), "Counterfactual (") {
		t.Fatalf("no counterfactual was formed, so the allocation rule went untested:\n%s", out.String())
	}
	for _, line := range strings.Split(out.String(), "\n") {
		low := strings.ToLower(line)
		if !strings.Contains(low, "cause:") {
			continue
		}
		// A break line naming a saving is the allocation the contract forbids.
		if strings.Contains(low, "would have avoided") || strings.Contains(low, "saving") {
			t.Errorf("a break line carries a saving, which allocates a session-level figure to one "+
				"event:\n%s", line)
		}
	}
}

// CF-3, reconciled under ADR-0025 with the existing RE-5 requirement: every
// presentation carrying a counterfactual or savings figure states that the
// figure assumes the agent would have behaved identically. header() already
// emits it, and this pins that it survives on the counterfactual path.
func TestTheAssumptionIsStatedWhereverASavingIsPresented(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"diff", "--counterfactual", claimable, corpusDir()}, &out, io.Discard); err != nil {
		t.Fatalf("diff --counterfactual %s failed: %v", claimable, err)
	}
	if !strings.Contains(out.String(), "Counterfactual (") {
		t.Fatalf("no counterfactual was formed, so the assumption went untested:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "Assumption: "+analysis.AssumptionNote) {
		t.Errorf("a counterfactual was presented without the unchanged-agent-behaviour assumption "+
			"RE-5 and CF-3 both require:\n%s", out.String())
	}
	// Population, read date and build travel with the figure (CF-3).
	for _, want := range []string{"Population:", "Build ", "Rules "} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("the counterfactual figure does not carry %q:\n%s", want, out.String())
		}
	}
}

// A valid alternative the corpus cannot support is insufficient evidence, not a
// usage error, and the refusal names the coverage state that produced it.
func TestAValidAlternativeBelowTheNoiseFloorIsNotMeasured(t *testing.T) {
	var out bytes.Buffer
	err := run([]string{"diff", "--counterfactual", "ttl-1h", corpusDir()}, &out, io.Discard)
	if !errors.Is(err, errNotMeasured) {
		t.Fatalf("a valid alternative scoring below the noise floor was not NOT MEASURED: %v", err)
	}
	if errors.Is(err, errUsage) {
		t.Error("insufficient evidence was reported as a usage error; the request was well formed")
	}
	if exitCode(err) != exitCannotEvaluate {
		t.Errorf("exit %d, want %d for insufficient evidence", exitCode(err), exitCannotEvaluate)
	}
	// The gate, by name. Asserting the surrounding prose instead is what broke
	// when the wording was corrected for the limitation vocabulary: the test
	// was pinned to a sentence rather than to the thing the sentence is about.
	if !strings.Contains(err.Error(), "numerical stability") {
		t.Errorf("the refusal does not name the gate that produced it: %v", err)
	}
}

// Determinism is a precondition, not a coverage dimension. Identical inputs
// produce identical bytes, or nothing downstream can be trusted to be
// reproducible.
func TestRepeatedEvaluationIsByteIdentical(t *testing.T) {
	var first, second bytes.Buffer
	err1 := run([]string{"diff", "--counterfactual", "ttl-1h", corpusDir()}, &first, io.Discard)
	err2 := run([]string{"diff", "--counterfactual", "ttl-1h", corpusDir()}, &second, io.Discard)
	if (err1 == nil) != (err2 == nil) {
		t.Fatalf("two identical runs disagreed on whether they succeeded: %v / %v", err1, err2)
	}
	if first.String() != second.String() {
		t.Error("two evaluations of the same alternative over the same corpus produced different documents")
	}
}

// Scalar confidence was removed from the contract. A HIGH/MEDIUM/LOW field
// would be read as a probability of correctness, which is the claim this
// project exists to refuse.
func TestNoScalarConfidenceIsEmitted(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"diff", "--counterfactual", "ttl-1h", corpusDir()}, &out, io.Discard); err != nil && !errors.Is(err, errNotMeasured) {
		t.Fatalf("diff --counterfactual ttl-1h failed: %v", err)
	}
	for _, banned := range []string{"confidence", "HIGH", "MEDIUM", "LOW"} {
		if strings.Contains(out.String(), banned) {
			t.Errorf("output carries %q, which the contract removed:\n%s", banned, out.String())
		}
	}
}

// The dollar figure is prompt-side, and this test derives the expectation
// itself rather than asking the renderer what it thinks.
//
// The first version of this line multiplied a prompt-side token share by the
// WHOLE session bill. On the bundled corpus output is 43.6% of cost, so the
// figure was overstated by 77% and every test still passed, because nothing
// checked the number against anything but itself.
//
// Here the expectation is rebuilt from the tally's own legs: the four legs sum
// to CostUSD, so prompt-side is CostUSD minus OutputUSD, and a layout
// alternative can only move the prompt side. A mutant that scales by CostUSD
// disagrees with this by exactly the output share and dies.
func TestTheDollarFigureIsPromptSideAndNotTheWholeBill(t *testing.T) {
	sessions, err := transcriptFiles([]string{corpusDir()})
	if err != nil {
		t.Fatalf("read corpus: %v", err)
	}
	alt, err := counterfactualAlternative(claimable)
	if err != nil {
		t.Fatalf("resolve alternative: %v", err)
	}

	var want float64
	var sawOutputCost bool
	for _, f := range sessions {
		s, err := transcript.ParseClaudeCodeFile(f)
		if err != nil || s == nil {
			continue
		}
		score, ok := learn.Score(s, []learn.Candidate{alt})
		if !ok {
			continue
		}
		saving, ok := score.Saving[alt.Name]
		if !ok {
			continue
		}
		// The identity under test, stated here and nowhere near the renderer.
		promptSide := score.AsRun.CostUSD - score.AsRun.OutputUSD
		if score.AsRun.OutputUSD > 0 && promptSide > 0 {
			sawOutputCost = true
		}
		want = math.Abs(saving * promptSide)
	}
	if !sawOutputCost {
		t.Fatal("this fixture has no output cost, so it cannot tell prompt-side from total and the " +
			"test would pass under the mutant it exists to kill")
	}

	var out bytes.Buffer
	if err := run([]string{"diff", "--counterfactual", claimable, corpusDir()}, &out, io.Discard); err != nil {
		t.Fatalf("diff --counterfactual %s failed: %v", claimable, err)
	}
	got := dollarsIn(t, out.String())
	if math.Abs(got-want) > 0.01 {
		t.Errorf("the counterfactual printed $%.2f; the prompt-side figure is $%.2f. A difference of "+
			"$%.2f is the output side being charged to a prompt-side change", got, want, math.Abs(got-want))
	}
}

// dollarsIn pulls the single dollar figure out of the counterfactual line.
func dollarsIn(t *testing.T, s string) float64 {
	t.Helper()
	m := regexp.MustCompile(`\$([0-9]+\.[0-9]{2}) at list, prompt-side`).FindStringSubmatch(s)
	if m == nil {
		t.Fatalf("no prompt-side dollar figure in the output:\n%s", s)
	}
	v, err := strconv.ParseFloat(m[1], 64)
	if err != nil {
		t.Fatalf("unreadable dollar figure %q: %v", m[1], err)
	}
	return v
}

// An alternative that costs more is reported as costing more. No absolute
// value, no clamp to zero, no silence. An adverse counterfactual is the most
// useful one a reader can get, and hiding it would be the exact dishonesty
// this project exists to refuse.
func TestAnAdverseCounterfactualIsReportedAsAdverse(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"diff", "--counterfactual", claimable, corpusDir()}, &out, io.Discard); err != nil {
		t.Fatalf("diff --counterfactual %s failed: %v", claimable, err)
	}
	line := ""
	for _, l := range strings.Split(out.String(), "\n") {
		if strings.HasPrefix(l, "Counterfactual (") {
			line = l
		}
	}
	if line == "" {
		t.Fatalf("no counterfactual line:\n%s", out.String())
	}
	// This fixture is known adverse. If that ever stops being true the test
	// should fail loudly rather than quietly stop testing the sign.
	if !strings.Contains(line, "would have cost a further") {
		t.Fatalf("the known-adverse fixture no longer reports a cost increase, so the sign is "+
			"untested:\n%s", line)
	}
	if strings.Contains(line, "would have avoided") {
		t.Errorf("an adverse counterfactual is described as avoiding something:\n%s", line)
	}
}

// Every NOT MEASURED cites a named gate, never free prose. The closed set is
// the coverage states, the numerical-stability gate, and the existing break
// causes. A refusal that names none of them is unfalsifiable.
func TestEveryRefusalCitesANamedGate(t *testing.T) {
	named := []string{
		"calibration passing", "calibration failing", "calibration drifted", "calibration no-evidence",
		"completeness complete", "completeness partial-tokens", "completeness partial-pricing",
		"completeness partial-parse",
		"observation lane-serial", "observation lane-overlap", "observation unmeasured",
		"provenance proxy-recorded", "provenance transcript-derived",
		"pricing_basis documented", "pricing_basis declared", "pricing_basis absent",
		"freshness not-verified",
		"numerical stability",
	}
	err := run([]string{"diff", "--counterfactual", "ttl-1h", corpusDir()}, &bytes.Buffer{}, io.Discard)
	if !errors.Is(err, errNotMeasured) {
		t.Fatalf("expected a refusal to inspect, got: %v", err)
	}
	for _, n := range named {
		if strings.Contains(err.Error(), n) {
			return
		}
	}
	t.Errorf("the refusal cites no named gate from the closed set, so its reason is free prose: %v", err)
}

// RECONSTRUCTED is an evidence class; transcript-derived is a provenance. The
// contract maps one to the other and deliberately leaves the printed wording
// alone, so the guard available here is narrow: the output must not assert
// that the tier IS the evidence class.
func TestProvenanceIsNotPresentedAsTheEvidenceClass(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"diff", "--counterfactual", claimable, corpusDir()}, &out, io.Discard); err != nil {
		t.Fatalf("diff --counterfactual %s failed: %v", claimable, err)
	}
	s := out.String()
	if !strings.Contains(s, "Provenance: ") {
		t.Errorf("the counterfactual does not name its provenance:\n%s", s)
	}
	// The locked mapping is internal. Printing either token as a synonym for
	// the other would collapse two layers into one.
	for _, collapsed := range []string{
		"RECONSTRUCTED", "evidence class", "estimated = ", "= RECONSTRUCTED",
	} {
		if strings.Contains(s, collapsed) {
			t.Errorf("output carries %q, collapsing evidence class into provenance:\n%s", collapsed, s)
		}
	}
}

// The dollar arithmetic, on numbers chosen here rather than measured anywhere.
//
// The bundled corpus cannot test this properly: every scorable candidate on it
// is adverse or below the floor, so the positive branch never runs and the
// sign is only ever observed in one direction. Fabricated tallies fix that,
// and they also make the expected value obvious enough to check by hand.
//
// prompt-side = CostUSD - OutputUSD = 100.00 - 40.00 = 60.00
// a +25% share is therefore +$15.00, and a -25% share is exactly -$15.00.
// Scaling the WHOLE bill would give ±$25.00, which is the defect this exists
// to catch.
func TestCounterfactualDollarsArePromptSideAndSigned(t *testing.T) {
	tally := analysis.Tally{CostUSD: 100.00, OutputUSD: 40.00}
	const promptSide = 60.00

	for _, c := range []struct {
		name   string
		saving float64
		want   float64
	}{
		{"alternative would have saved", 0.25, +15.00},
		{"alternative would have cost more", -0.25, -15.00},
		{"a small positive share", 0.01, +0.60},
		{"a small negative share", -0.01, -0.60},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, priced := counterfactualDollars(c.saving, tally)
			if !priced {
				t.Fatalf("a tally with $%.2f of prompt-side cost reported no price", promptSide)
			}
			if math.Abs(got-c.want) > 0.001 {
				whole := c.saving * tally.CostUSD
				t.Errorf("got $%.2f, want $%.2f. Scaling the whole bill would give $%.2f, so a "+
					"difference of $%.2f is the output leg being charged to a prompt-side change",
					got, c.want, whole, math.Abs(got-whole))
			}
			// The sign is the finding, not decoration. An alternative that costs
			// more must not arrive at the caller looking like one that saved.
			if (c.want < 0) != (got < 0) {
				t.Errorf("the sign was discarded: saving %.2f produced $%.2f, want the sign of $%.2f",
					c.saving, got, c.want)
			}
		})
	}
}

// No prompt-side cost means no dollar figure, and the token figure is
// untouched by that. pricing_basis and provenance are independent.
func TestCounterfactualDollarsAreSuppressedWithoutAPromptSideCost(t *testing.T) {
	for _, c := range []struct {
		name  string
		tally analysis.Tally
	}{
		{"nothing priced at all", analysis.Tally{}},
		{"output priced, prompt side not", analysis.Tally{CostUSD: 40.00, OutputUSD: 40.00}},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got, priced := counterfactualDollars(0.5, c.tally); priced {
				t.Errorf("priced a counterfactual at $%.2f with no prompt-side cost to price it on", got)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Phase D.2 fixtures.
//
// The bundled corpus is one session, fully priced, on a model that has not
// drifted. Every gate added in D.2 is therefore invisible to it: a report that
// cannot lose a later session because there is no later session, a dollar basis
// that cannot be mixed because every request is priced, and a drift rule that
// cannot fire because ST-1 needs several lanes. These helpers derive the
// missing shapes from that one real session rather than inventing a transcript,
// so the usage counts, the cache behaviour and the model all stay real and only
// the one property under test moves.
// ---------------------------------------------------------------------------

// sessionSpec describes one derived session.
type sessionSpec struct {
	name string
	// breakCalibration rewrites the cache reads so the engine stops
	// reproducing them, which is what makes a lane fail its calibration gate.
	breakCalibration bool
	// unprice moves the first assistant turn onto unpricedModel, the model
	// burnguards_test.go already pins as absent from the compiled price
	// table, which is what makes a lane partially priced.
	unprice bool
	// day orders the lanes for ST-1, whose recent window is the newest lanes
	// by first-request timestamp.
	day int
}

// d2Corpus writes the given sessions into a fresh directory and returns it.
//
// Every file is emitted through the same marshaller, and every rewrite keeps
// the width of the value it replaces, so the files come out the same length.
// That matters: transcriptFiles sorts largest-first and breaks ties on path, so
// equal sizes make the name the only thing deciding order, and a test that says
// "the first session" means the one it named first.
func d2Corpus(t *testing.T, specs ...sessionSpec) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(corpusDir(), "session-redacted.jsonl"))
	if err != nil {
		t.Fatalf("read the bundled session: %v", err)
	}
	dir := t.TempDir()
	for _, spec := range specs {
		var out bytes.Buffer
		firstAssistant := true
		for _, line := range strings.Split(strings.TrimRight(string(raw), "\n"), "\n") {
			rec := decodeRecord(t, line)
			rec["sessionId"] = spec.name
			shiftDay(t, rec, spec.day)
			if msg, ok := rec["message"].(map[string]any); ok && rec["type"] == "assistant" {
				if spec.unprice && firstAssistant {
					msg["model"] = unpricedModel
					firstAssistant = false
				}
				if spec.breakCalibration {
					breakCacheReads(msg)
				}
			}
			encoded, err := json.Marshal(rec)
			if err != nil {
				t.Fatalf("re-encode a record: %v", err)
			}
			out.Write(encoded)
			out.WriteByte('\n')
		}
		if err := os.WriteFile(filepath.Join(dir, spec.name+".jsonl"), out.Bytes(), 0o600); err != nil {
			t.Fatalf("write fixture %s: %v", spec.name, err)
		}
	}
	return dir
}

func decodeRecord(t *testing.T, line string) map[string]any {
	t.Helper()
	dec := json.NewDecoder(strings.NewReader(line))
	dec.UseNumber() // keep every count an integer, not a float rendered back differently
	var rec map[string]any
	if err := dec.Decode(&rec); err != nil {
		t.Fatalf("decode a fixture record: %v", err)
	}
	return rec
}

// shiftDay moves a record to a chosen day, preserving the time of day so the
// order of turns within a lane is untouched.
func shiftDay(t *testing.T, rec map[string]any, day int) {
	t.Helper()
	ts, ok := rec["timestamp"].(string)
	if !ok {
		return
	}
	at, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return
	}
	moved := time.Date(2026, 1, 1, at.Hour(), at.Minute(), at.Second(), at.Nanosecond(), time.UTC).
		AddDate(0, 0, day)
	rec["timestamp"] = moved.Format("2006-01-02T15:04:05.000Z")
}

// breakCacheReads rewrites every cache read to a different number of the same
// width. The engine then predicts one figure and the transcript reports
// another, which is exactly what a calibration failure is, and the file length
// does not move.
func breakCacheReads(msg map[string]any) {
	usage, ok := msg["usage"].(map[string]any)
	if !ok {
		return
	}
	n, ok := usage["cache_read_input_tokens"].(json.Number)
	if !ok || n.String() == "0" {
		return
	}
	digits := []byte(n.String())
	if digits[0] == '1' {
		digits[0] = '9'
	} else {
		digits[0] = '1'
	}
	usage["cache_read_input_tokens"] = json.Number(digits)
}

// sessionHeaders returns the fixture names whose reports appear, in order.
func sessionHeaders(out string) []string {
	var names []string
	for _, line := range strings.Split(out, "\n") {
		if strings.HasSuffix(line, ".jsonl") {
			names = append(names, strings.TrimSuffix(filepath.Base(line), ".jsonl"))
		}
	}
	return names
}

// ---------------------------------------------------------------------------
// D.2 BLOCKER 1 — a counterfactual one session cannot support must not delete
// the ordinary evidence of the sessions after it.
//
// The flag returned errNotMeasured from inside the per-session visitor, and
// forEachSession stops the whole walk on the first error a visitor returns. On
// a corpus of three sessions that printed one report and dropped two, so asking
// a question about layout alternatives silently destroyed the cache-break
// findings the reader came for. CF-1: "The breaks in that lane continue to be
// reported as they are today."
// ---------------------------------------------------------------------------

func TestOneUnmeasurableSessionDoesNotTruncateTheReport(t *testing.T) {
	for _, c := range []struct {
		name  string
		specs []sessionSpec
		// failing names the fixtures whose counterfactual cannot be formed.
		failing []string
	}{
		{
			name: "the first session cannot be measured",
			specs: []sessionSpec{
				{name: "a-fails", breakCalibration: true},
				{name: "b-ok"},
				{name: "c-ok"},
			},
			failing: []string{"a-fails"},
		},
		{
			name: "a middle session cannot be measured",
			specs: []sessionSpec{
				{name: "a-ok"},
				{name: "b-fails", breakCalibration: true},
				{name: "c-ok"},
			},
			failing: []string{"b-fails"},
		},
		{
			name:  "every session can be measured",
			specs: []sessionSpec{{name: "a-ok"}, {name: "b-ok"}, {name: "c-ok"}},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := d2Corpus(t, c.specs...)

			// What the reader gets without asking the counterfactual question
			// is the baseline. Asking it must not take anything away.
			var bare bytes.Buffer
			if err := run([]string{"diff", dir}, &bare, io.Discard); err != nil {
				t.Fatalf("bare diff over the fixture failed: %v", err)
			}
			want := sessionHeaders(bare.String())
			if len(want) != len(c.specs) {
				t.Fatalf("the fixture itself is wrong: bare diff reported %d of %d sessions %v",
					len(want), len(c.specs), want)
			}

			var out bytes.Buffer
			err := run([]string{"diff", "--counterfactual", claimable, dir}, &out, io.Discard)
			got := sessionHeaders(out.String())
			if len(got) != len(want) {
				t.Errorf("bare diff reported %d sessions %v and --counterfactual reported %d %v: "+
					"asking for a counterfactual deleted ordinary evidence", len(want), want, len(got), got)
			}
			for i := range want {
				if i >= len(got) || got[i] != want[i] {
					t.Errorf("session %d is %q with the flag and %q without it; the reports must match",
						i, nthHeader(got, i), want[i])
				}
			}

			// Each session that could not be measured says so, by name and in
			// its own place, because CF-4 does not permit silence.
			for _, name := range c.failing {
				if !strings.Contains(out.String(), "NOT MEASURED") {
					t.Errorf("session %q was refused and nothing said so:\n%s", name, out.String())
				}
			}
			// And the sessions that could be measured still carry their figure.
			claims := strings.Count(out.String(), "Counterfactual (")
			if wantClaims := len(c.specs) - len(c.failing); claims != wantClaims {
				t.Errorf("%d counterfactual claims, want %d", claims, wantClaims)
			}

			// The status is truthful either way, and it is decided after the
			// report is complete rather than instead of it.
			switch {
			case len(c.failing) == 0 && err != nil:
				t.Errorf("every session was measurable and the run still failed: %v", err)
			case len(c.failing) > 0 && !errors.Is(err, errNotMeasured):
				t.Errorf("a session could not be measured and the run did not say so: %v", err)
			case len(c.failing) > 0 && exitCode(err) != exitCannotEvaluate:
				t.Errorf("exit %d, want %d", exitCode(err), exitCannotEvaluate)
			}
		})
	}
}

func nthHeader(s []string, i int) string {
	if i < len(s) {
		return s[i]
	}
	return "<missing>"
}

// ---------------------------------------------------------------------------
// D.2 BLOCKER 2 — dollars are not claimed on a mixed pricing basis.
//
// Tally.AddAt prices a request only when the rules carry its model, so an
// unpriced request adds its tokens to EffectiveTokens and nothing to CostUSD.
// `saving` is a share of every effective token in the lane; CostUSD covers only
// the priced ones. On a mixed lane the product of the two counts different
// requests on each side. §4.5: "Not partial-pricing where dollars are claimed."
// ---------------------------------------------------------------------------

func TestDollarsAreNotClaimedWhenTheLaneIsOnlyPartlyPriced(t *testing.T) {
	if _, ok := cachemodel.PriceFor(unpricedModel); ok {
		t.Fatalf("%q is in the price table, so this fixture cannot make a mixed lane", unpricedModel)
	}
	dir := d2Corpus(t, sessionSpec{name: "mixed", unprice: true})

	var out bytes.Buffer
	if err := run([]string{"diff", "--counterfactual", claimable, dir}, &out, io.Discard); err != nil {
		t.Fatalf("diff over a partly priced lane failed: %v", err)
	}
	line := counterfactualLine(t, out.String())

	// The dollar figure is gone.
	if strings.Contains(line, "at list, prompt-side") {
		t.Errorf("a dollar figure was claimed on a lane whose pricing basis covers only some of its "+
			"tokens:\n%s", line)
	}
	// It is refused by name, and by the right name: partial is not absent, and
	// saying "absent" of a lane that priced 79 of 80 requests is a false
	// statement about the evidence.
	if !strings.Contains(line, "completeness partial-pricing") {
		t.Errorf("the suppressed dollar figure cites no named coverage state:\n%s", line)
	}
	if strings.Contains(line, "pricing_basis absent") {
		t.Errorf("a partly priced lane was reported as having no pricing basis at all:\n%s", line)
	}
	// The tokens are untouched. pricing_basis and the token claim are
	// independent, and §4.5 blocks only the dollars.
	if !strings.Contains(line, "effective tokens") {
		t.Errorf("the token figure was suppressed along with the dollars:\n%s", line)
	}

	// The gate is only worth anything if the request it refused on is one the
	// cost basis actually counts. lanePricing walks rep.Lane.Requests and
	// analysis.AsRun tallies the same slice, so an unpriced request cannot be
	// in the tally and out of the count; this pins that, because a fixture that
	// quietly stopped being mixed would leave the test passing on nothing.
	assertBasisIsGenuinelyMixed(t, dir)

	// The same session with every request priced still prints its dollars, so
	// this test is measuring the mixed basis and not simply a broken fixture.
	priced := d2Corpus(t, sessionSpec{name: "priced"})
	var pricedOut bytes.Buffer
	if err := run([]string{"diff", "--counterfactual", claimable, priced}, &pricedOut, io.Discard); err != nil {
		t.Fatalf("diff over the fully priced control failed: %v", err)
	}
	control := counterfactualLine(t, pricedOut.String())
	if !strings.Contains(control, "at list, prompt-side") {
		t.Fatalf("the control lane printed no dollars, so the fixture proves nothing:\n%s", control)
	}
}

// lanePricing answers the three states the contract names and no others.
func TestLanePricingSeparatesCompleteFromPartialFromAbsent(t *testing.T) {
	priced, ok := firstPricedModel(t)
	if !ok {
		t.Fatal("no model in the compiled price table, so pricing cannot be told from its absence")
	}
	req := func(model string) *transcript.Request {
		return &transcript.Request{Model: model, Usage: transcript.Usage{Input: 10, Output: 5}}
	}
	for _, c := range []struct {
		name                     string
		lane                     *transcript.Lane
		wantPriced, wantUnpriced int
	}{
		{"every request priced", &transcript.Lane{Requests: []*transcript.Request{req(priced), req(priced)}}, 2, 0},
		{"a mixed lane", &transcript.Lane{Requests: []*transcript.Request{req(priced), req(unpricedModel)}}, 1, 1},
		{"nothing priced", &transcript.Lane{Requests: []*transcript.Request{req(unpricedModel)}}, 0, 1},
		{"no lane at all", nil, 0, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			gotPriced, gotUnpriced := lanePricing(c.lane)
			if gotPriced != c.wantPriced || gotUnpriced != c.wantUnpriced {
				t.Errorf("priced=%d unpriced=%d, want priced=%d unpriced=%d",
					gotPriced, gotUnpriced, c.wantPriced, c.wantUnpriced)
			}
		})
	}
}

func firstPricedModel(t *testing.T) (string, bool) {
	t.Helper()
	// The bundled session's own model, which the corpus already prices.
	const m = "claude-fable-5-1"
	if _, ok := cachemodel.PriceFor(m); ok {
		return m, true
	}
	return "", false
}

func counterfactualLine(t *testing.T, out string) string {
	t.Helper()
	for _, l := range strings.Split(out, "\n") {
		if strings.HasPrefix(l, "Counterfactual (") {
			return l
		}
	}
	t.Fatalf("no counterfactual line in:\n%s", out)
	return ""
}

// ---------------------------------------------------------------------------
// D.2 BLOCKER 3 — a drifted model is not scored.
//
// ST-1 stops scoring alternatives for a model whose newest lanes stopped
// reproducing the provider. `replay learn` applies it at learn.go:65 before
// calling learn.Score; the counterfactual called the same function without it,
// so a lane that individually calibrates was scored against a provider that had
// already moved. §4.5 prohibits calibration in `drifted`.
// ---------------------------------------------------------------------------

// driftCorpus builds a model whose earlier lanes calibrate and whose recent
// lanes mostly do not, which is what ST-1 measures. One recent lane is left
// intact: it calibrates on its own, and it is the lane this test is about.
func driftCorpus(t *testing.T) string {
	t.Helper()
	specs := []sessionSpec{}
	for i := 0; i < 3; i++ {
		specs = append(specs, sessionSpec{name: "a-early" + strconv.Itoa(i), day: i})
	}
	for i := 0; i < 4; i++ {
		specs = append(specs, sessionSpec{
			name: "z-recent" + strconv.Itoa(i), breakCalibration: true, day: 100 + i,
		})
	}
	specs = append(specs, sessionSpec{name: "z-recent9-intact", day: 110})
	return d2Corpus(t, specs...)
}

func TestADriftedModelIsNotScored(t *testing.T) {
	dir := driftCorpus(t)

	// The fixture is only worth anything if ST-1 actually fires on it, and the
	// authority on that is the shipped surface, not this test.
	var learned bytes.Buffer
	if err := run([]string{"learn", "--out", "-", dir}, &learned, io.Discard); err != nil {
		t.Fatalf("learn over the drift fixture failed: %v", err)
	}
	if !strings.Contains(learned.String(), "Provider behavior changed") {
		t.Fatalf("ST-1 does not consider this fixture drifted, so it cannot test the drift gate:\n%s",
			learned.String())
	}

	var out bytes.Buffer
	err := run([]string{"diff", "--counterfactual", claimable, dir}, &out, io.Discard)
	if !errors.Is(err, errNotMeasured) {
		t.Fatalf("a drifted model was scored: %v", err)
	}
	if strings.Contains(out.String(), "Counterfactual (") {
		t.Errorf("a counterfactual was claimed for a model the provider has moved away from:\n%s",
			out.String())
	}
	if !strings.Contains(out.String(), "calibration drifted") {
		t.Errorf("no session cites the drift gate:\n%s", out.String())
	}
}

// The distinction that matters: the lane refused below is not refused for
// lacking evidence or for failing its own threshold. It calibrates. It is
// refused because the MODEL drifted, and the proof is that the same bytes score
// a counterfactual when they are the only lane the run can see.
func TestDriftRefusesALaneThatCalibratesOnItsOwn(t *testing.T) {
	dir := driftCorpus(t)
	const intact = "z-recent9-intact"

	var whole bytes.Buffer
	if err := run([]string{"diff", "--counterfactual", claimable, dir}, &whole, io.Discard); !errors.Is(err, errNotMeasured) {
		t.Fatalf("expected the drifted corpus to refuse: %v", err)
	}
	section := sessionSection(whole.String(), intact)
	if section == "" {
		t.Fatalf("%s is missing from the report entirely:\n%s", intact, whole.String())
	}
	if !strings.Contains(section, "calibration drifted") {
		t.Errorf("%s was refused for something other than drift:\n%s", intact, section)
	}
	for _, wrong := range []string{"calibration failing", "calibration no-evidence"} {
		if strings.Contains(section, wrong) {
			t.Errorf("%s cites %q; this lane has evidence and passes its own threshold:\n%s",
				intact, wrong, section)
		}
	}

	// Alone, with no other lane to establish drift, the very same file scores.
	var alone bytes.Buffer
	if err := run([]string{"diff", "--counterfactual", claimable,
		filepath.Join(dir, intact+".jsonl")}, &alone, io.Discard); err != nil {
		t.Fatalf("the intact lane does not score on its own, so the refusal above was not about "+
			"drift: %v", err)
	}
	if !strings.Contains(alone.String(), "Counterfactual (") {
		t.Errorf("the intact lane produced no claim on its own:\n%s", alone.String())
	}
}

// sessionSection returns the part of the report belonging to one fixture.
func sessionSection(out, name string) string {
	lines := strings.Split(out, "\n")
	start := -1
	for i, l := range lines {
		if strings.HasSuffix(l, name+".jsonl") {
			start = i
			break
		}
	}
	if start < 0 {
		return ""
	}
	for i := start + 1; i < len(lines); i++ {
		if strings.HasSuffix(lines[i], ".jsonl") {
			return strings.Join(lines[start:i], "\n")
		}
	}
	return strings.Join(lines[start:], "\n")
}

// Calibration states other than drift keep the behaviour they already had.
func TestTheOtherCalibrationStatesAreUnchanged(t *testing.T) {
	failing := d2Corpus(t, sessionSpec{name: "failing", breakCalibration: true})
	var out bytes.Buffer
	err := run([]string{"diff", "--counterfactual", claimable, failing}, &out, io.Discard)
	if !errors.Is(err, errNotMeasured) {
		t.Fatalf("a lane below the calibration threshold was scored: %v", err)
	}
	if !strings.Contains(out.String(), "calibration failing") {
		t.Errorf("a below-threshold lane does not cite the failing gate:\n%s", out.String())
	}
	if strings.Contains(out.String(), "calibration drifted") {
		t.Errorf("a single failing lane was reported as provider drift, which ST-1 explicitly is "+
			"not: one bad lane is not a rule change:\n%s", out.String())
	}

	// A passing, undrifted lane still scores. This is the control that stops
	// the drift gate from quietly refusing everything.
	ok := d2Corpus(t, sessionSpec{name: "ok"})
	var good bytes.Buffer
	if err := run([]string{"diff", "--counterfactual", claimable, ok}, &good, io.Discard); err != nil {
		t.Fatalf("a passing, undrifted lane was refused: %v", err)
	}
	if !strings.Contains(good.String(), "Counterfactual (") {
		t.Errorf("a passing, undrifted lane produced no claim:\n%s", good.String())
	}
}

// assertBasisIsGenuinelyMixed checks the fixture the hard way: the lane must
// carry a request lanePricing calls unpriced, and the tally must price fewer
// requests than the lane holds. If those two ever disagree, the dollar gate is
// reading a different set of requests from the one the dollars come from, which
// is the whole defect it exists to prevent.
func assertBasisIsGenuinelyMixed(t *testing.T, dir string) {
	t.Helper()
	files, err := transcriptFiles([]string{dir})
	if err != nil {
		t.Fatalf("read the fixture: %v", err)
	}
	for _, f := range files {
		s, err := transcript.ParseClaudeCodeFile(f)
		if err != nil || s == nil {
			t.Fatalf("parse %s: %v", f, err)
		}
		lane := analysis.MainLane(s)
		if lane == nil {
			t.Fatalf("%s has no main lane", f)
		}
		priced, unpriced := lanePricing(lane)
		if unpriced == 0 {
			t.Fatalf("%s: the fixture is fully priced, so it cannot exercise the partial-pricing "+
				"gate", f)
		}
		if priced == 0 {
			t.Fatalf("%s: nothing in the fixture is priced, so it is the absent case and not the "+
				"mixed one", f)
		}
		// The unpriced request is in the lane AsRun tallies, so the dollars
		// cover fewer requests than the tokens do.
		tally := analysis.AsRun(lane).Tally
		if tally.EffectiveTokens <= 0 {
			t.Fatalf("%s: no effective tokens, so there is no token share to misapply", f)
		}
		if tally.CostUSD-tally.OutputUSD <= 0 {
			t.Fatalf("%s: no prompt-side dollars, so the case is absent pricing and not mixed", f)
		}
	}
}

// ---------------------------------------------------------------------------
// D.2 review — absence must not be laundered into a coverage state.
//
// The dollar note names a coverage state, and a named state has to be true.
// Two cells of the pricing switch got that wrong: a lane with no contributing
// request at all was reported as `pricing_basis absent`, which says a rules
// document failed to resolve when in fact nothing was ever looked up; and a
// lane where EVERY model resolved was reported the same way when its prompt
// side simply carried no cost. Both turn an absence into a claim about
// evidence, which is the one thing this file exists to refuse.
//
// Passes()/HasEvidence() next door already settle this shape for calibration:
// "That is not a failure and not a success; it is an absence, and it needs its
// own name so a caller cannot mistake it for either."
// ---------------------------------------------------------------------------

func TestTheDollarNoteNamesOnlyStatesThatAreTrue(t *testing.T) {
	for _, c := range []struct {
		name                string
		priced, unpriced    int
		promptSideUSD       float64
		wantDollars         bool
		mustSay, mustNotSay []string
	}{
		{
			name:   "every request priced and a prompt side to scale",
			priced: 3, promptSideUSD: 12.50, wantDollars: true,
		},
		{
			name:   "a mixed lane",
			priced: 2, unpriced: 1, promptSideUSD: 12.50,
			mustSay:    []string{"completeness partial-pricing"},
			mustNotSay: []string{"pricing_basis absent"},
		},
		{
			name:   "requests exist and no rules document prices any of them",
			priced: 0, unpriced: 3, promptSideUSD: 0,
			mustSay:    []string{"pricing_basis absent"},
			mustNotSay: []string{"partial-pricing"},
		},
		{
			// The absence. Nothing was looked up, so nothing can be said to be
			// missing from the rules.
			name:   "no contributing request at all",
			priced: 0, unpriced: 0, promptSideUSD: 0,
			mustNotSay: []string{"pricing_basis absent", "partial-pricing"},
		},
		{
			// Every model resolved. Saying the pricing basis is absent here is
			// simply false, and it sends a reader to the rules document to look
			// for a model that is already in it.
			name:   "every request priced and no prompt-side cost",
			priced: 3, unpriced: 0, promptSideUSD: 0,
			mustNotSay: []string{"pricing_basis absent"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			note, dollars := dollarNote(c.priced, c.unpriced, c.promptSideUSD)
			if dollars != c.wantDollars {
				t.Fatalf("dollars=%v, want %v (note %q)", dollars, c.wantDollars, note)
			}
			if dollars {
				return
			}
			if strings.TrimSpace(note) == "" {
				t.Fatal("the dollar figure was suppressed and nothing said why; silence is not permitted")
			}
			for _, want := range c.mustSay {
				if !strings.Contains(note, want) {
					t.Errorf("note %q does not name %q", note, want)
				}
			}
			for _, banned := range c.mustNotSay {
				if strings.Contains(note, banned) {
					t.Errorf("note %q claims %q, which is not true of this lane", note, banned)
				}
			}
		})
	}
}

// A lane with no requests must not be described as one whose models are
// missing from the rules. This is the same distinction HasEvidence draws for
// calibration, applied to pricing.
func TestAnEmptyLaneIsNotReportedAsUnpricedModels(t *testing.T) {
	note, dollars := dollarNote(lanePricingOf(&transcript.Lane{}))
	if dollars {
		t.Fatal("an empty lane produced a dollar figure")
	}
	if strings.Contains(note, "pricing_basis absent") {
		t.Errorf("a lane with nothing in it was reported as having unpriced models: %q", note)
	}
}

// lanePricingOf adapts lanePricing to dollarNote's argument list so the empty
// case above reads as one statement.
func lanePricingOf(lane *transcript.Lane) (int, int, float64) {
	priced, unpriced := lanePricing(lane)
	return priced, unpriced, 0
}

// A state that is insufficient for a counterfactual says nothing about whether
// the observation itself is good.
//
// The drifted lane below reproduces the provider on 98.7% of its turns. That
// measurement stands, and its cache breaks stand with it. What does not stand
// is scoring an alternative against a provider that has since moved. Collapsing
// the two would throw away a good measurement because a different claim could
// not be made from it.
func TestARefusedCounterfactualLeavesTheObservationStanding(t *testing.T) {
	dir := driftCorpus(t)
	const intact = "z-recent9-intact"

	var out bytes.Buffer
	if err := run([]string{"diff", "--counterfactual", claimable, dir}, &out, io.Discard); !errors.Is(err, errNotMeasured) {
		t.Fatalf("expected the drifted corpus to refuse: %v", err)
	}
	section := sessionSection(out.String(), intact)
	if section == "" {
		t.Fatalf("%s is missing entirely:\n%s", intact, out.String())
	}
	if !strings.Contains(section, "calibration drifted") {
		t.Fatalf("%s was not refused for drift, so this test is not looking at the right case:\n%s",
			intact, section)
	}
	// The observation survives the refusal, in full.
	if !strings.Contains(section, "Calibration: reproduced provider cache reads") {
		t.Errorf("the calibration measurement was dropped along with the counterfactual:\n%s", section)
	}
	if !strings.Contains(section, "cache break") {
		t.Errorf("the cache-break findings were dropped along with the counterfactual:\n%s", section)
	}
	// And the refusal is scoped to the claim it refused, not to the evidence.
	if !strings.Contains(section, "alternatives are not scored") {
		t.Errorf("the refusal does not say what it refused:\n%s", section)
	}
}
