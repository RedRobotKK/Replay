package cachemodel

// Whether a surface's cache counters can be believed, from the counters alone.
//
// Everything else in this package needs to know which provider served a
// request and which client wrote the file. This does not. It rests on one
// fact that holds wherever prompt caching exists: a cache READ serves a prefix
// that some earlier request WROTE. Nothing reads an entry into existence.
//
// So cumulative reads above zero beside cumulative writes of exactly zero is
// not a cheap workload and not an unlikely one. It cannot have happened. The
// zero is a field the surface does not carry, rendered as a number, and every
// figure computed from it understates the bill by exactly the expensive half.
//
// The mechanism is public. The OpenAI usage shape reports cached_tokens and
// has no write counterpart, so layers that normalise to it drop the write even
// where the provider underneath did report one. Measured on this machine
// between 2026-09-12 and 2026-09-15, the same fingerprint appears on Grok
// (cacheCreationTokens zero on every record, beside a live cachedReadTokens)
// and on OpenClaw (cacheWrite zero on every assistant row, beside cacheRead
// counters that are not). Two unrelated clients, one shape.
//
// Why this is worth a named check rather than a note: it is the cheapest
// honest thing this tool can say about a surface it has no reader for. It
// needs no parsing beyond two integers, so it can be pointed at a surface
// before anyone writes a reader for it, and it distinguishes "this surface
// says nothing" from "this surface says something that cannot be true".

// Counters is a verdict about instrumentation, not about spend.
type Counters string

const (
	// CountersInstrumented reports both halves. Its figures can be argued
	// with, which is more than the others allow.
	CountersInstrumented Counters = "read and write both reported"

	// CountersWriteMissing is the impossible one: reads happened and the
	// write counter is zero. Something wrote the prefix being read.
	CountersWriteMissing Counters = "reads reported with no write: the write field is absent rather than zero"

	// CountersColdOnly wrote and never read again. Ordinary, and the common
	// shape of a session with a single turn.
	CountersColdOnly Counters = "writes reported with no read"

	// CountersSilent is both at zero, which is the honest unknown. A surface
	// that never cached and a surface that reports neither counter are
	// different states and this is not the instrument that tells them apart.
	CountersSilent Counters = "neither counter reported"

	// CountersUnreadable is a negative counter, which is a reader defect
	// upstream. Naming it as a cache verdict would put this check's name on
	// somebody else's parsing bug.
	CountersUnreadable Counters = "counters are not readable as token counts"
)

// Impossible reports whether the verdict is one that cannot describe any real
// session, so a caller can refuse a figure rather than print it with a note.
func (c Counters) Impossible() bool { return c == CountersWriteMissing }

// ClassifyCounters judges a surface's cumulative cache counters.
//
// There is deliberately no threshold. One read with no write is the same
// impossibility as four million with no write, and a floor here would be a
// number nobody could defend and that every mis-instrumented surface would
// eventually sit under.
func ClassifyCounters(read, write int64) Counters {
	switch {
	case read < 0 || write < 0:
		return CountersUnreadable
	case read > 0 && write == 0:
		return CountersWriteMissing
	case read > 0 && write > 0:
		return CountersInstrumented
	case write > 0:
		return CountersColdOnly
	default:
		return CountersSilent
	}
}
