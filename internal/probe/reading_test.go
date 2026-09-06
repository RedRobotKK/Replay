package probe

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The series: one appended line per measurement.
//
// A single floor is a fact anyone can copy the day it is published. "The floor
// changed on this date" can only be produced by someone who was measuring
// before the change, and cannot be backfilled at any price. That is the whole
// value here, and it exists only if readings are stored — a series that lives
// in a terminal's scrollback is not a series.
//
//	S1  readings append; an earlier one is never lost
//	S2  a reading records what answered, not only what was asked
//	S3  the method is fingerprinted, so incomparable readings are visible
//	S4  an inconclusive run is recorded as inconclusive, never as a bracket
//	S5  the file is owner-only

func TestS1_ReadingsAppend(t *testing.T) {
	// PASS: three writes leave three lines, oldest first.
	// FAIL: truncation. The point of the series is the past, so losing an
	// earlier entry costs exactly the thing that cannot be rebuilt.
	path := filepath.Join(t.TempDir(), "series.jsonl")
	for i := 0; i < 3; i++ {
		if err := AppendReading(path, Reading{Model: "m", AtMost: 512 + i}); err != nil {
			t.Fatal(err)
		}
	}
	lines := readLines(t, path)
	if len(lines) != 3 {
		t.Fatalf("%d lines after three readings", len(lines))
	}
	var first Reading
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatal(err)
	}
	if first.AtMost != 512 {
		t.Errorf("first line has atMost %d; the oldest reading must survive", first.AtMost)
	}
}

func TestS2_AReadingSaysWhatAnswered(t *testing.T) {
	// PASS: the alias asked for and the identity that answered are both kept.
	// FAIL: recording only the alias, which makes two readings a month apart
	// incomparable and neither reproducible.
	path := filepath.Join(t.TempDir(), "s.jsonl")
	if err := AppendReading(path, Reading{
		Model:       "claude-opus-5",
		AnsweredBy:  "claude-opus-5-20261101",
		ServiceTier: "standard",
		Geo:         "global",
		Above:       508, AtMost: 512, Probes: 13,
	}); err != nil {
		t.Fatal(err)
	}
	var r Reading
	if err := json.Unmarshal([]byte(readLines(t, path)[0]), &r); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ got, want, field string }{
		{r.Model, "claude-opus-5", "model"},
		{r.AnsweredBy, "claude-opus-5-20261101", "answeredBy"},
		{r.ServiceTier, "standard", "serviceTier"},
		{r.Geo, "global", "geo"},
	} {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.field, c.got, c.want)
		}
	}
	if r.TakenAt == "" {
		t.Error("a reading with no timestamp cannot be part of a series")
	}
}

func TestS3_TheMethodIsFingerprinted(t *testing.T) {
	// The method changed four times in one day, and each change moved the
	// numbers: sizing by estimate rather than measurement, counting the whole
	// request rather than the prefix, English filler rather than CJK. Readings
	// taken either side of such a change are not comparable, and nothing in a
	// bare number says so.
	// PASS: every reading carries the method version.
	// FAIL: a series that silently mixes methods, where a change in the
	// numbers cannot be told from a change in how they were taken.
	path := filepath.Join(t.TempDir(), "s.jsonl")
	if err := AppendReading(path, Reading{Model: "m"}); err != nil {
		t.Fatal(err)
	}
	var r Reading
	_ = json.Unmarshal([]byte(readLines(t, path)[0]), &r)
	if r.Method == "" {
		t.Fatal("a reading must record the method that produced it")
	}
	if r.Method != MethodVersion {
		t.Errorf("method = %q, want the current %q", r.Method, MethodVersion)
	}
}

func TestS4_AnInconclusiveRunIsNotABracket(t *testing.T) {
	// PASS: the outcome is recorded and the bounds are omitted.
	// FAIL: storing (0, 65536] as though it were a measurement. A series is
	// only worth keeping if a reader can tell a result from a failure.
	path := filepath.Join(t.TempDir(), "s.jsonl")
	if err := AppendReading(path, Reading{Model: "m", Outcome: "non-deterministic"}); err != nil {
		t.Fatal(err)
	}
	line := readLines(t, path)[0]
	if !strings.Contains(line, "non-deterministic") {
		t.Errorf("the outcome must be recorded: %s", line)
	}
	if strings.Contains(line, `"above"`) || strings.Contains(line, `"atMost"`) {
		t.Errorf("an inconclusive run must not carry bounds: %s", line)
	}
}

func TestS5_TheFileIsOwnerOnly(t *testing.T) {
	// The series says which models an account measured and when. Not secret,
	// but not the world's business either, and the rest of ~/.replay is
	// owner-only.
	path := filepath.Join(t.TempDir(), "s.jsonl")
	if err := AppendReading(path, Reading{Model: "m"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		t.Errorf("mode %04o; the series must not be group- or world-readable", perm)
	}
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, l := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}

// S6: an anomalous reading keeps its bounds and stays in the record.
//
// AppendReading used to zero Above and AtMost whenever an outcome was set, and
// trend.go's comparable() excludes any reading with an outcome — so every run
// that observed non-determinism was stripped of its numbers and then dropped
// from the analysis. The retained series was therefore SELECTED on agreement
// with the sharp-threshold model, and could never exhibit evidence against it.
// A referee called it publication bias implemented in code, and was right.
//
// The bounds an anomalous run reached are evidence about where the anomaly
// lives. Excluding a reading from change detection is a judgement about
// comparability; deleting its numbers destroys the record.
//
// PASS: bounds survive, and the outcome marks the reading rather than emptying
// it.
// FAIL: a stored reading that cannot say where its anomaly was.
func TestS6_AnAnomalousReadingKeepsItsBounds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	if err := AppendReading(path, Reading{
		Model: "m", Outcome: "non-deterministic",
		Above: 508, AtMost: 512,
		Anomalies: []Anomaly{{Kind: "non-deterministic", Size: 510, Wrote: 1, DidNotWrite: 1}},
	}); err != nil {
		t.Fatal(err)
	}
	var r Reading
	if err := json.Unmarshal([]byte(readLines(t, path)[0]), &r); err != nil {
		t.Fatal(err)
	}
	if r.Above != 508 || r.AtMost != 512 {
		t.Errorf("bounds = (%d, %d]; an anomalous run still reached a bracket and the record must keep it", r.Above, r.AtMost)
	}
	if r.Outcome != "non-deterministic" {
		t.Errorf("outcome = %q, want it retained so the reading is not compared as if clean", r.Outcome)
	}
	if len(r.Anomalies) != 1 || r.Anomalies[0].Size != 510 {
		t.Error("the anomaly and the size it happened at must survive into the series")
	}
}
