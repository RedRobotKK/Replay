package proxy

import (
	"fmt"

	"github.com/RedRobotKK/Replay/internal/ledger"
)

// What may appear in a log line.
//
// The logger gets one line per request and never a header, never a body, and
// never a whole session id. These two helpers are the whole of that rule in
// code: short cuts an identifier to a prefix long enough to correlate two
// lines in one run, and usageSummary renders token counts rather than the
// response they came from. Every log site in this package routes a session id
// through short; putting the function here rather than beside its first caller
// is what makes that checkable by grep instead of by reading nine files.
//
// short is a readability measure, not a privacy one. A twelve-character prefix
// still identifies a session to anyone holding the ledger, which is the point:
// the line has to be joinable to the record. It is not a redaction, and
// nothing here stops someone formatting a body into a Printf elsewhere. This
// only gives the rule somewhere to live.

func short(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

func usageSummary(u *ledger.Usage) string {
	if u == nil {
		return "usage=none"
	}
	return fmt.Sprintf("in=%d write=%d read=%d out=%d", u.Input, u.CacheCreation, u.CacheRead, u.Output)
}
