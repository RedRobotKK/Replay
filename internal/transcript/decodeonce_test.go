package transcript

import (
	"encoding/json"
	"testing"
)

// The decode happens once, and a cache that works is indistinguishable from no
// cache by its values alone.
//
// That is why these assert IDENTITY rather than equality. Three call sites in
// claudecode.go read the same line's content; if each decoded it again, every
// value here would still match and only the allocation count would move.
// guard-reachability reported both branches of the memo as inert for exactly
// that reason: nothing depended on whether the cached path ran.

func lineWithBlocks(t *testing.T) *rawLine {
	t.Helper()
	return &rawLine{Message: &RawMessage{
		Role:    "assistant",
		Content: json.RawMessage(`[{"type":"text","text":"hello"},{"type":"text","text":"world"}]`),
	}}
}

// DO1: the second read returns the very same slice, not an equal one.
func TestDO1_ContentIsDecodedOnce(t *testing.T) {
	l := lineWithBlocks(t)
	_, first, _, err := l.content()
	if err != nil {
		t.Fatalf("first decode: %v", err)
	}
	if len(first) != 2 {
		t.Fatalf("decoded %d blocks, want 2", len(first))
	}
	_, second, _, err := l.content()
	if err != nil {
		t.Fatalf("second decode: %v", err)
	}
	if &first[0] != &second[0] {
		t.Error("the second read decoded again: it returned a different backing " +
			"array, so the three call sites in claudecode.go each pay the full " +
			"decode and every nested RawMessage copy with it")
	}
}

// DO2: the error is memoised with the result.
//
// A line whose content will not decode fails the same way for every reader.
// Re-attempting it per call site was three failures where the transcript has
// one defect.
func TestDO2_TheErrorIsMemoisedToo(t *testing.T) {
	l := &rawLine{Message: &RawMessage{Content: json.RawMessage(`[{"type":`)}}
	_, _, _, first := l.content()
	if first == nil {
		t.Fatal("malformed content decoded without error, so this test pins nothing")
	}
	_, _, _, second := l.content()
	if second == nil {
		t.Error("the second read reported success where the first reported failure")
	}
	if first.Error() != second.Error() {
		t.Errorf("two reads of one malformed line report different failures:\n %v\n %v",
			first, second)
	}
}

// DO3: a line carrying no message is not a decode failure.
//
// Housekeeping lines have no message at all. Absence is not malformed content
// (ADR-0018), and the caller must be able to tell them apart: one is a line
// with nothing to read, the other is a transcript with a defect in it.
func TestDO3_ALineWithNoMessageIsNotAnError(t *testing.T) {
	l := &rawLine{Type: "system"}
	text, blocks, isText, err := l.content()
	if err != nil {
		t.Errorf("a line with no message reported %v", err)
	}
	if text != "" || blocks != nil || isText {
		t.Errorf("a line with no message decoded to %q / %v / %v", text, blocks, isText)
	}
	// And it is still memoised, so the nil check runs once.
	if _, _, _, err := l.content(); err != nil {
		t.Errorf("the second read of a message-less line reported %v", err)
	}
}

// DO4: the cached value is what the uncached decode would have produced.
//
// Identity without equivalence would pass if content() returned a cached empty
// slice forever.
func TestDO4_TheCachedValueIsTheDecodedValue(t *testing.T) {
	l := lineWithBlocks(t)
	_, cached, _, _ := l.content()
	_, direct, _, err := DecodeContent(l.Message.Content)
	if err != nil {
		t.Fatalf("direct decode: %v", err)
	}
	if len(cached) != len(direct) {
		t.Fatalf("cached %d blocks, direct decode gives %d", len(cached), len(direct))
	}
	for i := range cached {
		if cached[i].Type != direct[i].Type || cached[i].Text != direct[i].Text {
			t.Errorf("block %d differs: cached %+v, direct %+v", i, cached[i], direct[i])
		}
	}
}
