package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// The fixtures are built as maps so a test can produce any shape, including
// the malformed ones a typed struct could not express.

type row = map[string]any

func baseRow(id string) row {
	return row{
		"id":             id,
		"exposedOn":      "2026-11-01",
		"exposures":      1,
		"capBefore":      map[string]any{"maxSessionUsd": 5},
		"alternativeCap": map[string]any{"maxSessionUsd": 2},
		"changed":        "no",
		"influenced":     "no",
		"answeredOn":     "2026-12-02",
	}
}

// baseDataset is a cohort of ten, window closed on 2026-12-01, frozen four
// days later, nobody changed anything. Every test edits from here.
func baseDataset() map[string]any {
	rows := make([]any, 0, 10)
	for i := 1; i <= 10; i++ {
		rows = append(rows, baseRow("p"+strings.Repeat("0", 2-len(itoa(i)))+itoa(i)))
	}
	return map[string]any{
		"schema":          datasetSchema,
		"preregistration": "docs/evidence/simulate-experiment-prereg-2026-10-03.md",
		"frozenOn":        "2026-12-05",
		"window":          map[string]any{"mode": "cohort", "end": "2026-12-01"},
		"participants":    rows,
	}
}

func itoa(i int) string { return string(rune('0'+i/10)) + string(rune('0'+i%10)) }

func rowsOf(d map[string]any) []any { return d["participants"].([]any) }

func setRow(d map[string]any, i int, k string, v any) { rowsOf(d)[i].(row)[k] = v }

// qualifying makes row i a participant who changed because of the simulation.
func qualifying(d map[string]any, i int) {
	setRow(d, i, "changed", "yes")
	setRow(d, i, "capAfter", map[string]any{"maxSessionUsd": 2})
	setRow(d, i, "influenced", "yes")
}

func encode(t *testing.T, d map[string]any) []byte {
	t.Helper()
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func mustCount(t *testing.T, d map[string]any) result {
	t.Helper()
	r, err := count(encode(t, d))
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	return r
}

func rowByID(t *testing.T, r result, id string) participantRow {
	t.Helper()
	for _, p := range r.Participants {
		if p.ID == id {
			return p
		}
	}
	t.Fatalf("no participant %s in the result", id)
	return participantRow{}
}

// Exposure recorded correctly: an UNKNOWN exposure is never qualifying, even
// when the participant reports a change they attribute to the simulation.
func TestCount_AnUnexposedParticipantCannotQualify(t *testing.T) {
	d := baseDataset()
	qualifying(d, 0)
	setRow(d, 0, "exposedOn", "UNKNOWN")
	r := mustCount(t, d)
	p := rowByID(t, r, "p01")
	if p.Qualifying || p.Outcome != "not exposed" {
		t.Errorf("p01 with UNKNOWN exposure: qualifying=%v outcome=%q; want false, \"not exposed\"", p.Qualifying, p.Outcome)
	}
	if r.Primary.Qualifying != 0 || r.Secondary.NotExposed != 1 {
		t.Errorf("qualifying=%d notExposed=%d; want 0, 1", r.Primary.Qualifying, r.Secondary.NotExposed)
	}
}

// Baseline cap and alternative cap are captured: the result carries both
// exactly as reported, and the baseline decides enabled-vs-changed-existing.
func TestCount_BaselineAndAlternativeCapsAreCarriedAsReported(t *testing.T) {
	d := baseDataset()
	qualifying(d, 0)
	setRow(d, 0, "capBefore", map[string]any{})
	setRow(d, 0, "alternativeCap", map[string]any{"maxDayUsd": 20, "maxSessionTokens": 100000})
	setRow(d, 0, "capAfter", map[string]any{"maxDayUsd": 20})
	qualifying(d, 1)
	r := mustCount(t, d)
	p := rowByID(t, r, "p01")
	if p.CapBefore.Unknown || len(p.CapBefore.Values) != 0 {
		t.Errorf("p01 capBefore: %+v; want an empty, known cap set", p.CapBefore)
	}
	if got := p.AlternativeCap.Values; got["maxDayUsd"] != 20 || got["maxSessionTokens"] != 100000 || len(got) != 2 {
		t.Errorf("p01 alternativeCap: %+v; want the two values as reported", got)
	}
	if p.CapAfter == nil || p.CapAfter.Values["maxDayUsd"] != 20 {
		t.Errorf("p01 capAfter: %+v; want maxDayUsd 20", p.CapAfter)
	}
	if r.Secondary.EnabledACap != 1 || r.Secondary.ChangedAnExistingCap != 1 {
		t.Errorf("enabledACap=%d changedAnExistingCap=%d; want 1 and 1", r.Secondary.EnabledACap, r.Secondary.ChangedAnExistingCap)
	}
}

// A cap change with confirmed attribution is recognised.
func TestCount_AChangeTheParticipantAttributesQualifies(t *testing.T) {
	d := baseDataset()
	qualifying(d, 3)
	r := mustCount(t, d)
	p := rowByID(t, r, "p04")
	if !p.Qualifying || p.Outcome != "changed a cap because of the simulation" {
		t.Errorf("p04: qualifying=%v outcome=%q", p.Qualifying, p.Outcome)
	}
	if r.Primary.Qualifying != 1 {
		t.Errorf("qualifying=%d; want 1", r.Primary.Qualifying)
	}
}

// An unchanged cap is recognised, whatever the participant says about
// influence.
func TestCount_AnUnchangedCapNeverQualifies(t *testing.T) {
	d := baseDataset()
	setRow(d, 2, "influenced", "yes")
	r := mustCount(t, d)
	p := rowByID(t, r, "p03")
	if p.Qualifying || p.Outcome != "cap unchanged" {
		t.Errorf("p03: qualifying=%v outcome=%q; want false, \"cap unchanged\"", p.Qualifying, p.Outcome)
	}
	if r.Secondary.CapUnchanged != 10 {
		t.Errorf("capUnchanged=%d; want 10", r.Secondary.CapUnchanged)
	}
}

// Repeated simulations do not create a cap change: the exposure count is
// descriptive and the change is the participant's report alone.
func TestCount_RepeatedSimulationsDoNotCreateAChange(t *testing.T) {
	d := baseDataset()
	setRow(d, 5, "exposures", 7)
	r := mustCount(t, d)
	if p := rowByID(t, r, "p06"); p.Qualifying || p.Outcome != "cap unchanged" {
		t.Errorf("p06 after seven simulations and no reported change: qualifying=%v outcome=%q", p.Qualifying, p.Outcome)
	}
	if r.Primary.Qualifying != 0 {
		t.Errorf("qualifying=%d; want 0", r.Primary.Qualifying)
	}
}

// Attribution is not inferred: a change the participant does not attribute
// to the simulation is counted separately and never toward the primary
// outcome, however close the dates are.
func TestCount_AChangeWithoutAttributionDoesNotQualify(t *testing.T) {
	d := baseDataset()
	qualifying(d, 0)
	setRow(d, 0, "influenced", "no")
	setRow(d, 0, "answeredOn", "2026-12-01")
	r := mustCount(t, d)
	p := rowByID(t, r, "p01")
	if p.Qualifying || p.Outcome != "changed a cap, not attributed to the simulation" {
		t.Errorf("p01: qualifying=%v outcome=%q", p.Qualifying, p.Outcome)
	}
	if r.Primary.Qualifying != 0 || r.Secondary.ChangedNotAttributed != 1 {
		t.Errorf("qualifying=%d changedNotAttributed=%d; want 0, 1", r.Primary.Qualifying, r.Secondary.ChangedNotAttributed)
	}
}

// Missing fields are handled as the pre-registration says: UNKNOWN in the
// chain (exposure, change, attribution, answer) makes the participant
// non-qualifying and reported as UNKNOWN; UNKNOWN cap values do not block
// the primary outcome but do keep the participant out of the secondary
// enabled/changed-existing split.
func TestCount_UnknownIsAThirdStateNotASuccessOrAFailure(t *testing.T) {
	cases := []struct {
		name    string
		edit    func(d map[string]any)
		outcome string
	}{
		{"changed UNKNOWN", func(d map[string]any) { setRow(d, 0, "changed", "UNKNOWN"); setRow(d, 0, "influenced", "yes") }, "UNKNOWN: whether the cap changed"},
		{"influenced UNKNOWN", func(d map[string]any) { qualifying(d, 0); setRow(d, 0, "influenced", "UNKNOWN") }, "UNKNOWN: whether the simulation influenced the change"},
		{"answer UNKNOWN", func(d map[string]any) { qualifying(d, 0); setRow(d, 0, "answeredOn", "UNKNOWN") }, "UNKNOWN: no end-of-window answer"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := baseDataset()
			c.edit(d)
			r := mustCount(t, d)
			p := rowByID(t, r, "p01")
			if p.Qualifying || p.Outcome != c.outcome {
				t.Errorf("qualifying=%v outcome=%q; want false, %q", p.Qualifying, p.Outcome, c.outcome)
			}
			if r.Secondary.Unknown != 1 || r.Primary.Qualifying != 0 {
				t.Errorf("unknown=%d qualifying=%d; want 1, 0", r.Secondary.Unknown, r.Primary.Qualifying)
			}
		})
	}
	t.Run("cap values UNKNOWN", func(t *testing.T) {
		d := baseDataset()
		qualifying(d, 0)
		setRow(d, 0, "capBefore", "UNKNOWN")
		setRow(d, 0, "capAfter", "UNKNOWN")
		r := mustCount(t, d)
		p := rowByID(t, r, "p01")
		if !p.Qualifying || !p.CapBefore.Unknown {
			t.Errorf("p01 with UNKNOWN cap values but a reported, attributed change: qualifying=%v capBefore=%+v; want true, UNKNOWN", p.Qualifying, p.CapBefore)
		}
		if r.Primary.Qualifying != 1 || r.Secondary.CapValuesUnknown != 1 || r.Secondary.EnabledACap != 0 || r.Secondary.ChangedAnExistingCap != 0 {
			t.Errorf("qualifying=%d capValuesUnknown=%d enabled=%d existing=%d; want 1, 1, 0, 0",
				r.Primary.Qualifying, r.Secondary.CapValuesUnknown, r.Secondary.EnabledACap, r.Secondary.ChangedAnExistingCap)
		}
	})
}

// Malformed records are rejected as a whole dataset, with the reason, and
// nothing is counted.
func TestCount_AMalformedDatasetIsRejected(t *testing.T) {
	cases := []struct {
		name string
		edit func(d map[string]any)
		want string
	}{
		{"not json", nil, "dataset rejected"},
		{"wrong schema", func(d map[string]any) { d["schema"] = "replay.simexp.dataset.v0" }, "schema"},
		{"unknown top-level field", func(d map[string]any) { d["threshold"] = 2 }, "unknown field"},
		{"unknown row field", func(d map[string]any) { setRow(d, 0, "notes", "seemed keen") }, "unknown field"},
		{"bad tri-state", func(d map[string]any) { setRow(d, 0, "changed", "maybe") }, "changed"},
		{"bad date", func(d map[string]any) { setRow(d, 0, "exposedOn", "Nov 1") }, "exposedOn"},
		{"negative exposures", func(d map[string]any) { setRow(d, 0, "exposures", -1) }, "exposures"},
		{"unknown cap key", func(d map[string]any) { setRow(d, 0, "capBefore", map[string]any{"maxMonthUsd": 5}) }, "maxMonthUsd"},
		{"negative cap", func(d map[string]any) { setRow(d, 0, "capBefore", map[string]any{"maxSessionUsd": -5}) }, "negative"},
		{"changed yes without capAfter", func(d map[string]any) { setRow(d, 0, "changed", "yes"); setRow(d, 0, "influenced", "yes") }, "capAfter"},
		{"changed yes but caps identical", func(d map[string]any) {
			qualifying(d, 0)
			setRow(d, 0, "capAfter", map[string]any{"maxSessionUsd": 5})
		}, "identical"},
		{"changed no with a different capAfter", func(d map[string]any) {
			setRow(d, 0, "capAfter", map[string]any{"maxSessionUsd": 1})
		}, "capAfter"},
		{"answered before exposed", func(d map[string]any) { setRow(d, 0, "answeredOn", "2026-10-30") }, "before"},
		{"cohort without an end", func(d map[string]any) { d["window"] = map[string]any{"mode": "cohort"} }, "window"},
		{"unknown window mode", func(d map[string]any) { d["window"] = map[string]any{"mode": "rolling", "end": "2026-12-01"} }, "window"},
		{"wrong preregistration", func(d map[string]any) { d["preregistration"] = "docs/evidence/other.md" }, "preregistration"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var raw []byte
			if c.edit == nil {
				raw = []byte("ten rows, trust me")
			} else {
				d := baseDataset()
				c.edit(d)
				raw = encode(t, d)
			}
			_, err := count(raw)
			if err == nil {
				t.Fatal("accepted")
			}
			if !strings.Contains(err.Error(), "dataset rejected") || !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %q; want it to say \"dataset rejected\" and mention %q", err, c.want)
			}
		})
	}
}

// Duplicate events are handled deterministically: a repeated participant id
// is a rejection, not a merge, and the same bytes always give the same bytes.
func TestCount_DuplicateIDsAreRejectedAndOutputIsDeterministic(t *testing.T) {
	d := baseDataset()
	setRow(d, 9, "id", "p01")
	if _, err := count(encode(t, d)); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("duplicate id: err=%v; want a rejection naming the duplicate", err)
	}
	d = baseDataset()
	qualifying(d, 1)
	qualifying(d, 4)
	raw := encode(t, d)
	a, errA := count(raw)
	b, errB := count(raw)
	if errA != nil || errB != nil {
		t.Fatal(errA, errB)
	}
	ja, _ := json.Marshal(a)
	jb, _ := json.Marshal(b)
	if !bytes.Equal(ja, jb) {
		t.Error("two counts of the same bytes differ")
	}
	for i, p := range a.Participants {
		if want := rowsOf(d)[i].(row)["id"]; p.ID != want {
			t.Errorf("participant %d is %s; want input order (%s)", i, p.ID, want)
		}
	}
	sum := sha256.Sum256(raw)
	if a.DatasetSHA256 != hex.EncodeToString(sum[:]) {
		t.Errorf("datasetSha256 %s is not the digest of the bytes counted", a.DatasetSHA256)
	}
}

// The observation window is enforced as element 11 says: an answer within
// three days after the end counts; later is UNKNOWN; a dataset frozen before
// every window has closed is rejected; per-participant windows end thirty
// days after exposure.
func TestCount_TheWindowIsEnforced(t *testing.T) {
	t.Run("answer on the last allowed day", func(t *testing.T) {
		d := baseDataset()
		qualifying(d, 0)
		setRow(d, 0, "answeredOn", "2026-12-04")
		if p := rowByID(t, mustCount(t, d), "p01"); !p.Qualifying {
			t.Errorf("answer three days after the end: %+v; want qualifying", p)
		}
	})
	t.Run("answer one day too late", func(t *testing.T) {
		d := baseDataset()
		qualifying(d, 0)
		setRow(d, 0, "answeredOn", "2026-12-05")
		p := rowByID(t, mustCount(t, d), "p01")
		if p.Qualifying || p.Outcome != "UNKNOWN: answer after the window" {
			t.Errorf("answer four days after the end: qualifying=%v outcome=%q", p.Qualifying, p.Outcome)
		}
	})
	t.Run("frozen before the window closed", func(t *testing.T) {
		d := baseDataset()
		d["frozenOn"] = "2026-11-30"
		_, err := count(encode(t, d))
		if err == nil || !strings.Contains(err.Error(), "not closed") {
			t.Errorf("err=%v; want a rejection saying the window is not closed", err)
		}
	})
	t.Run("per-participant window is thirty days from exposure", func(t *testing.T) {
		d := baseDataset()
		d["window"] = map[string]any{"mode": "per-participant"}
		qualifying(d, 0) // exposed 2026-11-01, window ends 2026-12-01, answered 2026-12-02: in
		qualifying(d, 1)
		setRow(d, 1, "exposedOn", "2026-11-10")  // ends 2026-12-10
		setRow(d, 1, "answeredOn", "2026-12-14") // one day late
		d["frozenOn"] = "2026-12-20"
		r := mustCount(t, d)
		if p := rowByID(t, r, "p01"); !p.Qualifying {
			t.Errorf("p01: %+v; want qualifying", p)
		}
		if p := rowByID(t, r, "p02"); p.Qualifying || p.Outcome != "UNKNOWN: answer after the window" {
			t.Errorf("p02: qualifying=%v outcome=%q", p.Qualifying, p.Outcome)
		}
		d["frozenOn"] = "2026-12-05"
		if _, err := count(encode(t, d)); err == nil || !strings.Contains(err.Error(), "p02") {
			t.Errorf("frozen before p02's window closed: err=%v; want a rejection naming p02", err)
		}
	})
}

// The denominator cannot change: it is ten, a dataset of any other size is
// rejected, and unexposed or UNKNOWN participants stay in it.
func TestCount_TheDenominatorIsTenAndCannotShrink(t *testing.T) {
	d := baseDataset()
	setRow(d, 0, "exposedOn", "UNKNOWN")
	setRow(d, 1, "exposedOn", "UNKNOWN")
	setRow(d, 2, "changed", "UNKNOWN")
	r := mustCount(t, d)
	if r.Denominator != 10 || len(r.Participants) != 10 {
		t.Errorf("denominator=%d rows=%d; want 10 and 10 with two unexposed and one UNKNOWN", r.Denominator, len(r.Participants))
	}
	for _, n := range []int{9, 11} {
		d := baseDataset()
		rows := rowsOf(d)
		if n == 9 {
			d["participants"] = rows[:9]
		} else {
			d["participants"] = append(rows, baseRow("p11"))
		}
		_, err := count(encode(t, d))
		if err == nil || !strings.Contains(err.Error(), "10") {
			t.Errorf("%d participants: err=%v; want a rejection naming ten", n, err)
		}
	}
}

// The rule is applied mechanically: two qualifying kills, three survives, and
// the secondary figures decide nothing.
func TestCount_TheRuleIsMechanicalAndSecondaryFiguresDecideNothing(t *testing.T) {
	const killed = "PRO HYPOTHESIS KILLED / REDESIGN REQUIRED"
	const survives = "PRO HYPOTHESIS SURVIVES THIS EXPERIMENT"
	d := baseDataset()
	qualifying(d, 0)
	qualifying(d, 1)
	for i := 0; i < 2; i++ { // both enabled a cap where there was none
		setRow(d, i, "capBefore", map[string]any{})
	}
	for i := 2; i < 10; i++ { // eight changed a cap but do not attribute it
		qualifying(d, i)
		setRow(d, i, "influenced", "no")
	}
	r := mustCount(t, d)
	if r.Primary.Qualifying != 2 || r.Primary.Decision != killed {
		t.Errorf("two qualifying, two enables, eight unattributed changes: qualifying=%d decision=%q; want 2, %q", r.Primary.Qualifying, r.Primary.Decision, killed)
	}
	if r.Secondary.EnabledACap != 2 || r.Secondary.ChangedNotAttributed != 8 {
		t.Errorf("secondary enabled=%d notAttributed=%d; want 2, 8", r.Secondary.EnabledACap, r.Secondary.ChangedNotAttributed)
	}
	qualifying(d, 2)
	r = mustCount(t, d)
	if r.Primary.Qualifying != 3 || r.Primary.Decision != survives {
		t.Errorf("three qualifying: qualifying=%d decision=%q; want 3, %q", r.Primary.Qualifying, r.Primary.Decision, survives)
	}
	if !strings.Contains(r.Primary.Rule, "fewer than 3 of 10") {
		t.Errorf("rule %q does not state the frozen threshold and denominator", r.Primary.Rule)
	}
	if r.Schema != resultSchema || !strings.HasSuffix(r.Preregistration, "simulate-experiment-prereg-2026-10-03.md") {
		t.Errorf("result schema %q preregistration %q", r.Schema, r.Preregistration)
	}
}

// The command: usage, rejection and success exit codes through run, which is
// everything main does.
func TestRun_ExitCodesAndStreams(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run(nil, &out, &errb); code != 1 || !strings.Contains(errb.String(), "usage") {
		t.Errorf("no args: code=%d stderr=%q; want 1 and usage", code, errb.String())
	}
	out.Reset()
	errb.Reset()
	if code := run([]string{"count", t.TempDir() + "/missing.json"}, &out, &errb); code != 2 || !strings.Contains(errb.String(), "dataset rejected") || out.Len() != 0 {
		t.Errorf("missing file: code=%d stderr=%q stdout=%q; want 2, a rejection, nothing on stdout", code, errb.String(), out.String())
	}
	d := baseDataset()
	qualifying(d, 0)
	path := t.TempDir() + "/dataset.json"
	if err := os.WriteFile(path, encode(t, d), 0o600); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errb.Reset()
	if code := run([]string{"count", path}, &out, &errb); code != 0 {
		t.Fatalf("count: code=%d stderr=%q", code, errb.String())
	}
	var r result
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatalf("stdout is not the result JSON: %v\n%s", err, out.String())
	}
	if r.Primary.Qualifying != 1 || r.Denominator != 10 {
		t.Errorf("qualifying=%d denominator=%d; want 1, 10", r.Primary.Qualifying, r.Denominator)
	}
	for _, banned := range []string{"saving", "forecast", "will save", "predict"} {
		if strings.Contains(strings.ToLower(out.String()), banned) {
			t.Errorf("result contains %q", banned)
		}
	}
}

// Gate 3 correction 1: the end-of-window question is asked on or after the
// end (pre-registration element 11, procedure step 10), so an answer dated
// before the window end is not an observation and the file is rejected, in
// both window modes. An answer on the end date itself is in.
func TestCount_AnAnswerBeforeTheWindowEndIsRejected(t *testing.T) {
	t.Run("cohort", func(t *testing.T) {
		d := baseDataset()
		qualifying(d, 0)
		setRow(d, 0, "answeredOn", "2026-11-30") // end is 2026-12-01
		_, err := count(encode(t, d))
		if err == nil || !strings.Contains(err.Error(), "dataset rejected") || !strings.Contains(err.Error(), "p01") || !strings.Contains(err.Error(), "before the window end") {
			t.Errorf("answer the day before the end: err=%v; want a rejection naming p01 and the window end", err)
		}
		setRow(d, 0, "answeredOn", "2026-12-01")
		if p := rowByID(t, mustCount(t, d), "p01"); !p.Qualifying {
			t.Errorf("answer on the end date: %+v; want qualifying", p)
		}
	})
	t.Run("per-participant", func(t *testing.T) {
		d := baseDataset()
		d["window"] = map[string]any{"mode": "per-participant"}
		d["frozenOn"] = "2026-12-20"
		qualifying(d, 1)
		setRow(d, 1, "exposedOn", "2026-11-10") // end is 2026-12-10
		setRow(d, 1, "answeredOn", "2026-12-09")
		_, err := count(encode(t, d))
		if err == nil || !strings.Contains(err.Error(), "p02") || !strings.Contains(err.Error(), "before the window end") {
			t.Errorf("answer the day before a per-participant end: err=%v; want a rejection naming p02", err)
		}
		setRow(d, 1, "answeredOn", "2026-12-10")
		if p := rowByID(t, mustCount(t, d), "p02"); !p.Qualifying {
			t.Errorf("answer on the per-participant end date: %+v; want qualifying", p)
		}
	})
}

// Gate 3 correction 2: a participant found ineligible after enrolment is
// reported as UNKNOWN and stays in the ten (element 9), under a label that
// says why, whatever else the row reports. The field is optional and typed.
func TestCount_IneligibleAfterEnrolmentIsReportedAsUnknownAndStays(t *testing.T) {
	d := baseDataset()
	qualifying(d, 0)
	setRow(d, 0, "ineligibleAfterEnrolment", true)
	r := mustCount(t, d)
	p := rowByID(t, r, "p01")
	if p.Qualifying || p.Outcome != "UNKNOWN: ineligible after enrolment" {
		t.Errorf("p01: qualifying=%v outcome=%q", p.Qualifying, p.Outcome)
	}
	if r.Denominator != 10 || len(r.Participants) != 10 || r.Secondary.Unknown != 1 || r.Primary.Qualifying != 0 {
		t.Errorf("denominator=%d rows=%d unknown=%d qualifying=%d; want 10, 10, 1, 0", r.Denominator, len(r.Participants), r.Secondary.Unknown, r.Primary.Qualifying)
	}
	d = baseDataset()
	setRow(d, 0, "ineligibleAfterEnrolment", "yes")
	if _, err := count(encode(t, d)); err == nil || !strings.Contains(err.Error(), "dataset rejected") {
		t.Errorf("a string where the flag should be a bool: err=%v; want a rejection", err)
	}
	d = baseDataset()
	setRow(d, 0, "ineligibleAfterEnrolment", false)
	if p := rowByID(t, mustCount(t, d), "p01"); p.Outcome != "cap unchanged" {
		t.Errorf("flag false: outcome=%q; want the ordinary chain", p.Outcome)
	}
}
