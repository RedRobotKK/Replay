package analysis

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/RedRobotKK/Replay/internal/ledger"
)

// Attribution must reconcile against what the provider billed.
//
// This is the check that did not exist, and its absence let the product be
// silently wrong on screen. On the real ledger, `replay context` reported "1k
// tokens of content entered this context" for a session the provider billed
// 204,643 prompt tokens for, and printed `tool 0.0%` against 439,611 bytes of
// tool definitions. Three of four sessions understated by 75x to 204x. One
// reconciled exactly, which is why nobody noticed.
//
// The invariant is the only one that matters for a tool whose entire claim is
// that its figures are measured: every prompt token the provider charged for
// came from somewhere, and attribution that does not sum to the bill is not
// attribution. A ranking of sources covering half a percent of the spend is
// not a ranking, it is a sample of unknown provenance.
//
// PASS: attributed tokens are within tolerance of the provider's own count.
// FAIL: a gap. And a gap is not a rounding question — the tolerance here is
// deliberately loose, because the failures are orders of magnitude.
func TestReconcile_AttributionSumsToWhatTheProviderBilled(t *testing.T) {
	dir := os.Getenv("REPLAY_LEDGER_DIR")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Skip("no home directory")
		}
		dir = filepath.Join(home, ".replay", "ledger")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("no ledger to reconcile against: %v", err)
	}

	var checked int
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if e.IsDir() || !ledger.IsLedgerFile(path) {
			continue
		}
		records, _, rerr := ledger.ReadRecords(path)
		if rerr != nil || len(records) == 0 {
			continue
		}

		// What the provider charged for, from its own usage reporting.
		var billed int
		for _, r := range records {
			if r.Response.Usage == nil {
				continue
			}
			u := r.Response.Usage
			billed += u.Input + u.CacheRead + u.CacheCreation
		}
		if billed == 0 {
			continue
		}

		session, serr := ledger.ReadFile(path)
		if serr != nil || session == nil {
			continue
		}
		lane := MainLane(session)
		if lane == nil {
			continue
		}
		// The same call the command makes: cmd/replay/context.go builds rows
		// from rep.Blame, so reconciling anything else would be reconciling a
		// figure nobody is shown.
		rep := AnalyzeLane(session, lane)
		var attributed int
		for _, c := range EnteredContext(rep.Blame) {
			attributed += c.Tokens
		}
		checked++

		// An order of magnitude. Not a precision bar — a sanity bar. Anything
		// that fails this is not slightly off, it is describing a different
		// session.
		if attributed*10 < billed {
			t.Errorf("%s: attribution accounts for %d of %d prompt tokens the provider billed (%.1f%%). "+
				"A ranking that covers this little of the spend is not a ranking of the spend.",
				e.Name(), attributed, billed, 100*float64(attributed)/float64(billed))
		}
	}
	if checked == 0 {
		t.Skip("no ledger sessions with usage to reconcile")
	}
	t.Logf("reconciled %d session(s)", checked)
}
