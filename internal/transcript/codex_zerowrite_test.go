package transcript

import "testing"

// A cache write of zero in a rollout file stays zero, and is never rebuilt
// from the uncached remainder.
//
// Every Codex rollout on this machine carries cache_write_input_tokens and
// carries it at zero: 0 of 11,776 records non-zero, measured on the raw files.
// That zero is a transcript observability loss rather than a free write.
// OpenAI bills GPT-5.6 and later 1.25x for cache writes, and 117 of 117 local
// gpt-5.6-terra turns carry uncached input, 79 of them above the 1,024-token
// cache minimum, while recording zero. The tokens were written and billed; the
// client did not persist the count.
//
// Knowing that, the tempting repair is to derive the write from what was not
// cached, because input_tokens minus cached_input_tokens is sitting right
// there and is the right order of magnitude. This test exists to refuse that.
// The uncached remainder is the remainder; on a turn that wrote nothing it is
// still non-zero, so deriving a write from it would report cache traffic that
// was never observed, in a field that is multiplied by a price and shown to
// somebody as money. An unobserved quantity is reported as zero and named as
// unobserved elsewhere, never estimated here.
//
// The fixture is one real record shape from the corpus: 12,963 input, 9,600
// cached, 3,363 uncached, write stated as zero.
//
// See docs/evidence/cross-surface-observability-2026-09-26.md.
func TestCodexZeroWriteIsCarriedNotReconstructed(t *testing.T) {
	s, err := ParseCodexFile("codexdata/zerowrite.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Billed.CacheCreation; got != 0 {
		t.Errorf("billed cache creation = %d, want 0: the provider stated a "+
			"write of zero and a write may not be derived from the uncached "+
			"remainder (3,363 here), which would bill an unobserved quantity", got)
	}
	// The remainder has to land somewhere, and Input is where it belongs.
	// Usage is exclusive, so Input is the prompt minus the cached and written
	// shares: 12,963 - 9,600 - 0.
	if got, want := s.Billed.Input, 3363; got != want {
		t.Errorf("billed input = %d, want %d: the uncached remainder belongs in "+
			"Input, not in CacheCreation", got, want)
	}
	if got, want := s.Billed.CacheRead, 9600; got != want {
		t.Errorf("billed cache read = %d, want %d: the read counter beside the "+
			"zero write is populated, which is what makes the zero legible as a "+
			"gap rather than an absent field", got, want)
	}
	// A zero write must not make the record unbelievable. Refusing it would
	// drop the whole corpus, since every record on this surface reads zero.
	if s.Billed.Total() == 0 {
		t.Error("record refused: a stated zero write is a valid record, and " +
			"refusing it would discard every Codex rollout on this machine")
	}
}
