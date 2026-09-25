package analysis

import (
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/transcript"
)

// CC. The sentence a reader acts on says a compaction happened. Sometimes
// nothing recorded one.
//
// ContextGap.Compactions is incremented by two different things. A recorded
// compaction is the client writing down that it rewrote the history. An
// inferred one is a prompt that shrank between turns, and that also fires on a
// rewind or a resume, which the code comment at the increment site says in so
// many words.
//
// Note() then writes "The history was compacted N times" from the total. So a
// session where nothing recorded a compaction, and a prompt merely got smaller
// because the user resumed, tells the reader the history was compacted. That is
// an inference stated as an observation, in the one sentence a reader is most
// likely to act on, in the subsystem this product's whole claim rests on.
//
// The total keeps its meaning. What changes is that the sentence stops claiming
// more than the evidence carries.

// CC1: an inferred compaction is not described as a recorded one.
func TestCC1_AnInferredCompactionIsNotStatedAsRecorded(t *testing.T) {
	// No recorded compaction anywhere. Only a prompt that got smaller.
	lane := &transcript.Lane{Requests: []*transcript.Request{
		{Usage: transcript.Usage{Input: 900}},
		{Usage: transcript.Usage{Input: 100}},
	}}

	note := MeasureGap(nil, lane, 1000).Note()

	if strings.Contains(note, "was compacted") {
		t.Errorf("the note says the history was compacted, from a shrinking prompt "+
			"and nothing else. A prompt that shrank also fires on a rewind or a "+
			"resume, which this package's own comment says.\n  note: %s", note)
	}
	if !strings.Contains(note, "may have") && !strings.Contains(note, "shrank") {
		t.Errorf("the note does not mark the claim as inferred at all.\n  note: %s", note)
	}
}

// CC2: a recorded compaction still reads as recorded.
//
// The regression case. Hedging a claim the client actually wrote down would be
// the opposite error and is just as bad.
func TestCC2_ARecordedCompactionStillReadsAsRecorded(t *testing.T) {
	s := &transcript.Session{Compactions: []transcript.Compaction{
		{Trigger: "auto", PreTokens: 1000, PostTokens: 400},
	}}

	note := MeasureGap(s, nil, 5000).Note()

	if !strings.Contains(note, "was compacted") {
		t.Errorf("a compaction the client recorded is not stated plainly.\n  note: %s", note)
	}
	if strings.Contains(note, "may have") {
		t.Errorf("a recorded compaction is hedged. The client wrote it down; hedging "+
			"it is the same defect pointing the other way.\n  note: %s", note)
	}
	if !strings.Contains(note, "600") && !strings.Contains(note, "the client recorded") {
		t.Errorf("the recorded size is missing.\n  note: %s", note)
	}
}

// CC3: the two never mix in one sentence.
//
// The existing rule is that a recorded compaction suppresses the heuristic, so
// this case cannot arise today. It is asserted anyway: if that rule is ever
// relaxed, the sentence must not silently start summing the two.
func TestCC3_RecordedEvidenceSuppressesTheInferredSentence(t *testing.T) {
	s := &transcript.Session{Compactions: []transcript.Compaction{{Trigger: "auto"}}}
	lane := &transcript.Lane{Requests: []*transcript.Request{
		{Usage: transcript.Usage{Input: 900}},
		{Usage: transcript.Usage{Input: 100}},
	}}

	note := MeasureGap(s, lane, 1000).Note()

	if strings.Contains(note, "may have") {
		t.Errorf("the inferred wording appears although the client recorded a "+
			"compaction.\n  note: %s", note)
	}
}
