package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// RPL-C005 fixture matrix, attacked at the user-visible boundary.
//
// The refusal path was read from source before any fixture was built. In
// cmd/replay/cost.go, inside forEachSession:
//
//	if err != nil || rep == nil || session == nil { unreadable++; return nil }
//	if asRun.CostUSD <= 0                         { unpriced++;   return nil }
//
// Two counters, BOTH PER SESSION FILE, and no third state. Neither is per
// record. That shape is what the matrix below is built to probe, and it
// predicts two opposite failures from one line: a mixed session undercounts
// because its cost is above zero, and a genuinely free session overcounts
// because a true zero cannot be told from an absent price.

// epistemic is the oracle's own vocabulary, deliberately distinct from any of
// Replay's. Keeping them apart is the point: if the product cannot express a
// distinction the oracle makes, that is the finding.
type epistemic string

const (
	epObserved   epistemic = "OBSERVED"
	epUnknown    epistemic = "UNKNOWN"
	epZero       epistemic = "ZERO"
	epPartial    epistemic = "PARTIAL"
	epNoEndpoint epistemic = "NO_ENDPOINT"
)

type recSpec struct {
	model  string
	input  int
	output int
	// free zeroes every usage field, so the record genuinely costs nothing
	// rather than merely having no input tokens. The first version of MX8 set
	// input and output to zero and left 9,000 cache reads priced, so it was
	// not a zero at all and did not test what it claimed.
	free bool
	// want is the oracle's independent judgement of this record's cost.
	want epistemic
}

// writeCorpus builds one session file per group. The oracle never reads it
// back; it already knows what it wrote.
func c005WriteCorpus(t *testing.T, groups map[string][]recSpec) string {
	t.Helper()
	home := t.TempDir()
	isolateHome(t, home)
	dir := filepath.Join(home, "ledger")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 30, 9, 0, 0, 0, time.UTC)
	n := 0
	for session, specs := range groups {
		var lines []byte
		for i, sp := range specs {
			n++
			u := transcript.Usage{Input: sp.input, CacheRead: 9000, CacheCreation: 800, Output: sp.output}
			if sp.free {
				u = transcript.Usage{}
			}
			rec := ledger.Record{
				Schema: ledger.SchemaVersion, Timestamp: at.Add(time.Duration(n) * time.Minute),
				SessionID: session, RequestID: session + "-" + string(rune('A'+i)),
				Path: "/v1/messages", Status: 200, LatencyMS: 800,
				RequestSummary: ledger.RequestSummary{Model: sp.model,
					Prompt: ledger.Prompt{SystemBytes: 50, Messages: []ledger.Message{{Role: "user", Blocks: []ledger.Block{{Kind: "text", Label: "user text", Bytes: 150}}}}}},
				Response: ledger.Response{Usage: &u, Blocks: []ledger.Block{{Kind: "text", Label: "assistant text", Bytes: 100}}},
			}
			b, err := json.Marshal(rec)
			if err != nil {
				t.Fatal(err)
			}
			lines = append(lines, append(b, '\n')...)
		}
		if err := os.WriteFile(filepath.Join(dir, session+".jsonl"), lines, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

type observedOutput struct {
	human      string
	total      int
	unpriced   int
	unreadable int
	discloses  bool
}

func runBoth(t *testing.T, dir string) observedOutput {
	t.Helper()
	var h, e bytes.Buffer
	_ = runCost([]string{dir}, &h, &e)
	var j bytes.Buffer
	_ = runCost([]string{"--json", dir}, &j, &e)
	var doc map[string]any
	_ = json.Unmarshal(j.Bytes(), &doc)
	num := func(k string) int {
		if v, ok := doc[k].(float64); ok {
			return int(v)
		}
		return -1
	}
	txt := h.String()
	return observedOutput{
		human: txt, total: num("totalRequests"),
		unpriced: num("unpriced"), unreadable: num("unreadable"),
		// Widened after MX2 showed the first version missed the wholly-unpriced
		// report, which says "No transcript could be priced" and contains none
		// of the three phrases originally looked for. The detector gap was
		// mine, and it is recorded rather than quietly patched.
		discloses: strings.Contains(txt, "not priced") || strings.Contains(txt, "could not be read") ||
			strings.Contains(txt, "excluded") || strings.Contains(txt, "could be priced") ||
			strings.Contains(txt, "were read"),
	}
}

const (
	knownModel   = "claude-opus-5"
	unknownModel = "totally-unknown-model-x9"
)

// MX0. The fixture's own assumptions, verified before anything is concluded.
func TestC005M_FixtureAssumptionsHold(t *testing.T) {
	if _, ok := cachemodel.PriceFor(knownModel); !ok {
		t.Fatalf("%s is not priced; the measurable half of every fixture is broken", knownModel)
	}
	if p, ok := cachemodel.PriceFor(unknownModel); ok {
		t.Fatalf("%s IS priced (%v); the unmeasurable half of every fixture is broken", unknownModel, p)
	}
}

// MX1-MX10. The matrix. Each row states the oracle's independent expectation
// and the row's own verdict is recorded rather than asserted, EXCEPT where a
// pass is genuinely required.
func TestC005M_TheFixtureMatrix(t *testing.T) {
	type row struct {
		id       string
		name     string
		groups   map[string][]recSpec
		oracle   epistemic
		expect   string // the oracle's independent expectation, in words
		required bool   // true when a mismatch is a hard failure
	}

	rows := []row{
		{
			id: "MX1", name: "fully measurable", oracle: epObserved, required: true,
			groups: map[string][]recSpec{"all-priced": {{model: knownModel, input: 2000, output: 400, want: epObserved}, {model: knownModel, input: 2000, output: 400, want: epObserved}}},
			expect: "a total, no disclosure, unpriced=0",
		},
		{
			id: "MX2", name: "completely unmeasurable", oracle: epUnknown, required: true,
			groups: map[string][]recSpec{"none-priced": {{model: unknownModel, input: 2000, output: 400, want: epUnknown}, {model: unknownModel, input: 2000, output: 400, want: epUnknown}}},
			expect: "no fabricated figure; an explicit reason; unpriced=1 session",
		},
		{
			// REPAIRED 2026-10-01. Before the repair this row reported
			// unpriced=0 with no disclosure, and the whole session was dropped
			// before its tokens could be counted. It is kept in the matrix as
			// the regression case rather than deleted.
			id: "MX3", name: "partially measurable, one session", oracle: epPartial,
			groups: map[string][]recSpec{"mixed": {{model: knownModel, input: 2000, output: 400, want: epObserved}, {model: knownModel, input: 2000, output: 400, want: epObserved}, {model: unknownModel, input: 2000, output: 400, want: epUnknown}, {model: unknownModel, input: 2000, output: 400, want: epUnknown}}},
			expect: "a total over the measurable half ONLY; the oracle says 2 of 4 records are unmeasurable",
		},
		{
			id: "MX4", name: "mixed epistemic classes across sessions", oracle: epPartial,
			groups: map[string][]recSpec{
				"priced":   {{model: knownModel, input: 2000, output: 400, want: epObserved}},
				"unpriced": {{model: unknownModel, input: 2000, output: 400, want: epUnknown}},
				"mixed":    {{model: knownModel, input: 2000, output: 400, want: epObserved}, {model: unknownModel, input: 2000, output: 400, want: epUnknown}},
			},
			expect: "distinctions preserved; the oracle says 1 whole session unmeasurable and 1 session partly so",
		},
		{
			id: "MX8", name: "zero versus absent", oracle: epZero,
			groups: map[string][]recSpec{
				"genuinely-zero": {{model: knownModel, free: true, want: epZero}},
			},
			expect: "a session that genuinely cost zero must NOT be reported as unpriced",
		},
		{
			id: "MX9", name: "cross-session, foreign evidence present", oracle: epPartial, required: true,
			groups: map[string][]recSpec{
				"target":  {{model: unknownModel, input: 2000, output: 400, want: epUnknown}},
				"foreign": {{model: knownModel, input: 2000, output: 400, want: epObserved}},
			},
			expect: "the foreign session must NOT rescue the unmeasurable one; unpriced=1",
		},
		{
			id: "MX10", name: "aggregation attack, 2 measurable + 2 not", oracle: epPartial,
			groups: map[string][]recSpec{"agg": {{model: knownModel, input: 2000, output: 400, want: epObserved}, {model: knownModel, input: 2000, output: 400, want: epObserved}, {model: unknownModel, input: 2000, output: 400, want: epUnknown}, {model: unknownModel, input: 2000, output: 400, want: epUnknown}}},
			expect: "compare every printed figure against the oracle",
		},
	}

	var b strings.Builder
	b.WriteString("\nC005 FIXTURE MATRIX\n")
	b.WriteString("id    fixture                              total unpriced unread discloses  oracle\n")

	for _, r := range rows {
		dir := c005WriteCorpus(t, r.groups)
		got := runBoth(t, dir)

		// Oracle: how many records are unmeasurable, counted from the spec.
		unmeasurable := 0
		totalRecs := 0
		for _, specs := range r.groups {
			for _, sp := range specs {
				totalRecs++
				if sp.want == epUnknown {
					unmeasurable++
				}
			}
		}

		b.WriteString(padRight(r.id, 6) + padRight(r.name, 36) +
			padRight(tierItoa(got.total), 6) + padRight(tierItoa(got.unpriced), 9) +
			padRight(tierItoa(got.unreadable), 7) + padRight(yn(got.discloses), 11) +
			tierItoa(unmeasurable) + "/" + tierItoa(totalRecs) + " records unmeasurable\n")

		if r.required {
			switch r.id {
			case "MX1":
				if got.unpriced != 0 || got.discloses {
					t.Errorf("MX1: a fully measurable corpus disclosed something "+
						"(unpriced=%d discloses=%v). The refusal fires when it must not.",
						got.unpriced, got.discloses)
				}
			case "MX2":
				if got.unpriced == 0 {
					t.Errorf("MX2: a corpus with NO priceable record reported unpriced=0. " +
						"A wholly unmeasurable corpus is not disclosed at all.")
				}
			case "MX9":
				if got.unpriced == 0 {
					t.Errorf("MX9: an unmeasurable session beside a measurable one reported " +
						"unpriced=0. Foreign evidence appears to have rescued it.")
				}
			}
		}
	}
	t.Log(b.String())
}
