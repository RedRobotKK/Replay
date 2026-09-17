// Package surfaces classifies what an agent store will and will not tell you
// about its prompt cache.
//
// cachemodel.ClassifyCounters answers a narrower question: given a read and a
// write count, is the pair coherent. It cannot separate a surface whose usage
// shape has no write field from one that has the field and reports zero on
// every record, because both arrive as read>0 write=0.
//
// That separation is the point. An absent field claims nothing, and a reader
// correctly concludes nothing. A zero looks like a measurement, and every
// layer downstream will sum it, average it and chart it.
package surfaces

// State is what a surface's own records establish about its cache writes.
type State string

const (
	// StateInstrumented: reads and writes are both reported and non-zero.
	StateInstrumented State = "instrumented"
	// StateWriteFieldAbsent: reads are reported and no write field exists in
	// the shape at all. Honest absence.
	StateWriteFieldAbsent State = "write field absent"
	// StateWriteReportedZero: a write field exists and is zero on every record
	// beside live reads. A read serves a prefix some earlier request wrote, so
	// this is absence wearing a value.
	StateWriteReportedZero State = "write reported zero"
	// StateSilent: nothing is reported, and no caching and no instrumentation
	// are indistinguishable from here.
	StateSilent State = "silent"
	// StateUnreadable: the counts are not usable. A broken reader is not a
	// cache story and must not get this package's verdict attached to it.
	StateUnreadable State = "unreadable"
)

// Reading is what one scan of a surface observed.
type Reading struct {
	Name    string
	Records int
	Reads   int64
	Writes  int64
	// WriteFieldPresent records whether any scanned record carried the write
	// key at all, independently of its value. This is the field that makes the
	// absent/zero distinction possible and it must be set by the scanner, not
	// inferred from Writes.
	WriteFieldPresent bool
}

// Classify reports what the reading establishes. It never guesses between two
// states that the record cannot separate.
func Classify(r Reading) State {
	switch {
	case r.Records < 0 || r.Reads < 0 || r.Writes < 0:
		return StateUnreadable
	case r.Records == 0 && (r.Reads > 0 || r.Writes > 0):
		// No records found, yet tokens counted. The counts came from
		// somewhere, so the scan is what is broken, not the surface.
		return StateUnreadable
	case r.Records == 0:
		return StateSilent
	case r.Reads == 0 && r.Writes == 0:
		return StateSilent
	case r.Writes > 0:
		return StateInstrumented
	case !r.WriteFieldPresent:
		return StateWriteFieldAbsent
	default:
		return StateWriteReportedZero
	}
}
