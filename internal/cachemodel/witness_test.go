package cachemodel

import "testing"

// A read with no write is not improbable. It is impossible.
//
// A cache read serves a prefix some earlier request wrote. So a surface
// reporting cumulative reads above zero and cumulative writes of exactly zero
// is not describing a cheap workload; it is reporting a field it does not
// have, as a number. The zero is absence wearing a value, which is the
// ADR-0018 failure with the two states swapped: not a zero mistaken for
// absence, an absence mistaken for zero.
//
// This is worth its own check because it needs no reader. It does not care
// which client wrote the file or which provider served the request, only that
// two counters were reported. Measured on this machine 2026-09-12 to 09-15 it
// already holds against Grok (cacheCreationTokens zero on every record beside
// a live cachedReadTokens) and OpenClaw (cacheWrite zero on every assistant
// row beside cacheRead counters that are not).
//
// The mechanism is public and not specific to those two: the OpenAI usage
// shape carries cached_tokens and no write counterpart, so every layer that
// normalises to it drops the write even where the provider reported one.

func TestCounters_ReadWithoutWriteIsImpossibleRatherThanCheap(t *testing.T) {
	got := ClassifyCounters(4_000_000, 0)
	if got != CountersWriteMissing {
		t.Fatalf("read 4,000,000 write 0 classified %q; something wrote the prefix being read, "+
			"so this is a missing field rather than a measurement", got)
	}
}

// The honest negative. Both counters zero says nothing at all: a surface that
// genuinely never cached and a surface that reports neither counter look
// identical from here, and guessing between them is the thing this package
// refuses everywhere else.
func TestCounters_BothZeroIsUnknownNotUncached(t *testing.T) {
	got := ClassifyCounters(0, 0)
	if got == CountersWriteMissing {
		t.Error("both zero called a missing write; with no read there is nothing that needs explaining")
	}
	if got != CountersSilent {
		t.Fatalf("read 0 write 0 classified %q, want %q: no caching and no instrumentation are "+
			"different states and neither is observable here", got, CountersSilent)
	}
}

// A write with no read is ordinary and must not be flagged. It is every cold
// session that never got a second turn, and a check that cried about those
// would be ignored within a day.
func TestCounters_WriteWithoutReadIsAnOrdinaryColdSession(t *testing.T) {
	if got := ClassifyCounters(0, 250_000); got != CountersColdOnly {
		t.Fatalf("write 250,000 read 0 classified %q, want %q; a session that wrote and never "+
			"read again is the common case, not a defect", got, CountersColdOnly)
	}
}

func TestCounters_BothPresentIsInstrumented(t *testing.T) {
	if got := ClassifyCounters(4_000_000, 250_000); got != CountersInstrumented {
		t.Fatalf("read and write both reported classified %q, want %q", got, CountersInstrumented)
	}
}

// One read and no write is the same impossibility as four million and no
// write. The check is about the logic, not about a volume that looks
// suspicious, so it must not carry a threshold anyone has to justify.
func TestCounters_TheImpossibilityHasNoThreshold(t *testing.T) {
	if got := ClassifyCounters(1, 0); got != CountersWriteMissing {
		t.Fatalf("read 1 write 0 classified %q; one read still had to be written, and a threshold "+
			"here would be a number nobody could defend", got)
	}
}

// Negative counters are a broken reader, not a cache story. Reporting them as
// an instrumentation verdict would put this check's name on somebody else's
// parsing bug.
func TestCounters_NegativeInputIsRefusedRatherThanClassified(t *testing.T) {
	for _, c := range []struct{ read, write int64 }{{-1, 0}, {0, -1}, {-5, -5}} {
		if got := ClassifyCounters(c.read, c.write); got != CountersUnreadable {
			t.Errorf("read %d write %d classified %q, want %q", c.read, c.write, got, CountersUnreadable)
		}
	}
}
