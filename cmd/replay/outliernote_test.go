package main

import (
	"strings"
	"testing"
)

// The line a first-time reader can act on.
//
// `replay` prints a total, a median and a p90, and none of them tells somebody
// who just installed whether anything is wrong. The comparison that does is the
// reader against themselves — their most expensive session against their own
// median — because it needs no population, no key and no network, and it means
// the same thing on a metered account, a subscription seat and a local model.

func units(costs ...float64) []costUnit {
	out := make([]costUnit, 0, len(costs))
	for i, c := range costs {
		out = append(out, costUnit{ID: string(rune('a'+i)) + "0000000", CostUSD: c})
	}
	return out
}

// ON1: a real outlier is named, with the ratio and the n behind it.
func TestON1_TheOutlierIsNamedWithItsBasis(t *testing.T) {
	u := units(0.50, 0.80, 0.85, 0.90, 3.40)
	note := outlierNote(u, costSummary{TotalUSD: 6.45, Tasks: len(u)})
	if note == "" {
		t.Fatal("a 4x session against a 5-session median produced no note")
	}
	for _, want := range []string{"53%", "6.45", "5 sessions"} {
		if !strings.Contains(note, want) {
			t.Errorf("the note does not carry %q so the reader cannot check it:\n%s", want, note)
		}
	}
}

// ON2: an unremarkable corpus says nothing.
//
// A tool that prints a comparison on every run trains the reader to skip it.
// Below the threshold there is nothing to act on and the right output is
// silence.
func TestON2_AnUnremarkableCorpusIsSilent(t *testing.T) {
	u := units(0.80, 0.85, 0.90, 1.00)
	if note := outlierNote(u, costSummary{TotalUSD: 3.55, Tasks: len(u)}); note != "" {
		t.Errorf("an evenly spread corpus produced a note:\n%s", note)
	}
}

// ON3: too few sessions, or a zero median, says nothing.
//
// The refusals live in analysis.CompareToMedian; this is the check that the
// caller honours them rather than rendering an infinity or a ratio of one.
func TestON3_RefusalsAreHonoured(t *testing.T) {
	if note := outlierNote(units(1.0, 5.0), costSummary{TotalUSD: 6.0, Tasks: 2}); note != "" {
		t.Errorf("two sessions produced a comparison:\n%s", note)
	}
	if note := outlierNote(units(0, 0, 0, 0), costSummary{TotalUSD: 0, Tasks: 4}); note != "" {
		t.Errorf("a zero median produced a comparison:\n%s", note)
	}
	if note := outlierNote(nil, costSummary{TotalUSD: 1, Tasks: 9}); note != "" {
		t.Errorf("no units produced a comparison:\n%s", note)
	}
}

// ON4: the note names the session so the reader can go and look.
//
// A ratio with no way to reach the thing it describes is a fact the reader can
// do nothing with.
func TestON4_TheSessionIsIdentified(t *testing.T) {
	u := units(0.50, 0.85, 0.90, 3.40)
	note := outlierNote(u, costSummary{TotalUSD: 5.65, Tasks: len(u)})
	if !strings.Contains(note, "d0000000") && !strings.Contains(note, "d000000") {
		t.Errorf("the note does not name the session it is about:\n%s", note)
	}
}

// ON6: the instruction names the transcript, and it is the peak's transcript.
//
// The note used to end `replay why <id>`. `why` was never a command — it is a
// TUI screen label that reached CLI output — so the one line written for a
// first-time reader to act on told them to run something that does not exist,
// at the moment they were most likely to try. The tests at the time asserted
// only on the finding line, so the instruction was never checked at all and the
// defect shipped under a green suite.
func TestON6_TheInstructionNamesThePeaksTranscript(t *testing.T) {
	u := units(0.50, 0.80, 0.85, 0.90, 3.40)
	for i := range u {
		u[i].path = "/corpus/" + u[i].ID + ".jsonl"
	}
	note := outlierNote(u, costSummary{TotalUSD: 6.45, Tasks: len(u)})
	if !strings.Contains(note, "replay blame /corpus/e0000000.jsonl") {
		t.Errorf("the note does not tell the reader how to open the session it just "+
			"named:\n%s", note)
	}
	if strings.Contains(note, "replay why") {
		t.Errorf("the note prints `replay why`, which is not a command:\n%s", note)
	}
	for _, other := range []string{"a0000000.jsonl", "d0000000.jsonl"} {
		if strings.Contains(note, "blame /corpus/"+other) {
			t.Errorf("the instruction opens %s, which is not the session the finding "+
				"is about:\n%s", other, note)
		}
	}
}

// ON7: with no transcript to name, the finding stands and the instruction goes.
//
// Absence, zero and unknown are three values (ADR-0018). A row priced from a
// source this build cannot point at still supports the finding; it does not
// support an instruction, and inventing a plausible-looking path would be the
// same defect as `replay why` in a different costume.
func TestON7_NoPathMeansNoInstruction(t *testing.T) {
	u := units(0.50, 0.80, 0.85, 0.90, 3.40) // no paths set
	note := outlierNote(u, costSummary{TotalUSD: 6.45, Tasks: len(u)})
	if note == "" {
		t.Fatal("the finding was dropped along with the instruction")
	}
	if !strings.Contains(note, "53%") {
		t.Errorf("the finding is gone:\n%s", note)
	}
	if strings.Contains(note, "replay ") {
		t.Errorf("an instruction was printed with no transcript behind it:\n%s", note)
	}
}

// ON8: a session folded from several lanes is opened at its own transcript,
// not at whichever sub-agent lane the walk happened to reach first.
//
// blame reads either file, so the wrong choice fails silently: it answers a
// narrower question than the row the reader clicked, and looks like an answer.
func TestON8_TheFoldKeepsTheSessionsOwnTranscript(t *testing.T) {
	id := "facfd32e"
	lanes := []costUnit{
		{ID: id, CostUSD: 3.0, path: "/corpus/agent-9c11beef.jsonl"},
		{ID: id, CostUSD: 1.0, path: "/corpus/" + id + ".jsonl"},
		{ID: id, CostUSD: 0.5, path: "/corpus/agent-77aa0011.jsonl"},
	}
	got := foldSessions(lanes)
	if len(got) != 1 {
		t.Fatalf("three lanes of one session folded to %d rows", len(got))
	}
	if got[0].path != "/corpus/"+id+".jsonl" {
		t.Errorf("the folded session points at %q, not its own transcript", got[0].path)
	}
}

// ON9: the instruction does not promise more than the command delivers.
//
// `replay blame` reports one lane and says so. On a fanned-out session that
// lane can be a minority of the cost the finding above it just quoted — measured
// here, $286.59 of a $1,056.14 session, the remaining $769.55 sitting in 1,013
// sub-agent transcripts. "ranks what filled it" over that is an instruction the
// reader takes at face value for an answer to a narrower question.
func TestON9_TheInstructionDoesNotOverclaimItsScope(t *testing.T) {
	u := units(0.50, 0.80, 0.85, 0.90, 3.40)
	for i := range u {
		u[i].path = "/corpus/" + u[i].ID + ".jsonl"
	}
	note := outlierNote(u, costSummary{TotalUSD: 6.45, Tasks: len(u)})
	if !strings.Contains(note, "main lane") {
		t.Errorf("the instruction does not say blame is scoped to one lane:\n%s", note)
	}
	if strings.Contains(note, "ranks what filled it.") {
		t.Errorf("the instruction still claims blame ranks the whole session:\n%s", note)
	}
}
