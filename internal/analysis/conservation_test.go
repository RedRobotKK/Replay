package analysis

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// The conservation laws.
//
// reconcile_test.go already pins the total of the count-once column against
// the provider's last prompt, and says of itself: "Conservation is necessary
// and not sufficient: it pins the total and says nothing about the split."
// These two laws close the two halves of that gap that a total cannot see.
//
// The first says the split is a PARTITION rather than a sum: both printed
// columns account for the bill with nothing dropped between the attribution
// and the screen, and nothing counted twice, and the one row that sits outside
// the partition is named on the screen and sized exactly.
//
// The second says the partition holds INSIDE each agent lane, not merely
// across the session. That is the half a session-keyed accumulator breaks: it
// can keep the session total exact to the token while moving one lane's
// content onto another lane's rows. Five fields in the proxy were keyed by
// session where they should have been keyed by AgentID, and the shape is worth
// guarding on this side of the boundary too, because nothing in the arithmetic
// here would have noticed: every conservation check in this package ran on the
// one lane MainLane picks.

// Every token the provider billed lands in exactly one bucket.
//
// Two columns are printed for every row of `replay blame` and `replay
// context`: "once", the count-once size of everything carrying the label, and
// "in prompts", the same size multiplied by the requests that carried it. Both
// are attributions of provider-reported figures and both must therefore close:
// the count-once column against the last prompt (everything that entered and
// was not removed), the in-prompts column against the sum of every prompt.
//
// Exactly one row is allowed outside the partition, and it is allowed only
// because the screen names it: RebillLabel, the history a cache break made the
// provider process again. Those tokens are already attributed to the content
// that carried them - the content rows count every block once per carrying
// request, break or no break - so the row re-states them under a name that
// tells a reader why a turn was expensive. This law pins the excess to exactly
// that row's size, in both columns. Anything else that ever failed to land in
// a bucket, or landed in two, moves the excess off it and fails here.
//
// PASS: buckets minus the named rebill row equal the bill, in both columns.
// FAIL: a residual. On the first run of this law's in-prompts half the residual
// was +189,229 tokens on session-redacted.jsonl (23,541,269 attributed against
// 23,352,040 billed, +0.81%), which is the rebill row exactly and is the reason
// it is named here rather than left implicit.
func TestConserve_EveryBilledTokenLandsInExactlyOneBucket(t *testing.T) {
	var lanesChecked, lanesWithRebill int
	for _, s := range conservationCorpus(t) {
		for _, lane := range s.session.Lanes {
			if len(lane.Requests) == 0 {
				continue
			}
			c := measureConservation(s.session, lane)
			if c.billedOnce == 0 {
				continue
			}
			lanesChecked++
			if c.rebilledOnce > 0 {
				lanesWithRebill++
			}
			assertConserves(t, s.name, c)
		}
	}
	if lanesChecked == 0 {
		t.Fatal("no lane was checked, so this law asserts nothing and is not evidence")
	}
	// The rebill exemption is the one clause that lets a figure sit outside
	// the partition. A corpus with no cache break never exercises it, and a
	// clause no fixture reaches is a clause that cannot fail.
	if lanesWithRebill == 0 {
		t.Fatal("no lane in the corpus re-billed history after a cache break, so the one " +
			"exemption in this law is never exercised and could hide any residual")
	}
	t.Logf("conserved %d lane(s), %d of them carrying a cache-break rebill row", lanesChecked, lanesWithRebill)
}

// The same law holds independently inside every agent lane.
//
// Every offline command reports on one lane, and until now every conservation
// check ran on one lane too - the one MainLane picks. A session that fans out
// to sub-agents was therefore unguarded on the lanes a fan-out is about: the
// attribution for `agent-b` could have been `agent-a`'s and nothing would have
// gone red, because the session total would still have been exact.
//
// The shipped fixture is built to make that failure reachable. Two sub-agent
// lanes are fanned out with the same system prefix, the same opening prompt
// and the same first tool call - the ordinary shape when two agents are told
// the same thing and both begin by reading the same file - so the ledger
// reader hands both lanes ONE memoized message per position. Blame decides
// what is new against a map of messages already seen; if that map were keyed
// by session rather than by lane - the exact defect shape that was just fixed
// elsewhere - the second lane's whole turn would be found already seen, its
// tokens would share out across an empty block list and vanish, and the first
// lane's would stay exact.
//
// PASS: each lane's buckets close against that lane's own bill, and the lanes
// together close against the session's.
// FAIL: a lane that does not close on its own, or lanes that close only in
// aggregate.
func TestConserve_EachLaneConservesIndependently(t *testing.T) {
	var fanOutSessions int
	for _, s := range conservationCorpus(t) {
		var lanes []*transcript.Lane
		for _, l := range s.session.Lanes {
			if len(l.Requests) > 0 {
				lanes = append(lanes, l)
			}
		}
		if len(lanes) < 2 {
			continue
		}
		fanOutSessions++

		var sumAttributed, sumBilled int
		for _, lane := range lanes {
			c := measureConservation(s.session, lane)
			assertConserves(t, s.name, c)
			sumAttributed += c.attributed
			sumBilled += c.billedOnce
			// A lane that closes against another lane's bill is not this lane
			// conserving, and with two lanes of equal size the per-lane check
			// above would not tell the difference. Nothing may be attributed
			// to a lane that carries no request of its own.
			if c.requests == 0 && c.attributed != 0 {
				t.Errorf("%s lane %q: %d tokens attributed to a lane with no requests",
					s.name, lane.ID, c.attributed)
			}
		}
		if sumAttributed != sumBilled {
			t.Errorf("%s: the lanes together account for %d tokens against the %d billed "+
				"across them (off by %+d). Read with the per-lane failures above, or without "+
				"any: content that belongs to no lane, or to two.",
				s.name, sumAttributed, sumBilled, sumAttributed-sumBilled)
		}
	}
	if fanOutSessions == 0 {
		t.Fatal("no session in the corpus carries more than one lane, so a per-lane " +
			"conservation law asserts nothing and is not evidence")
	}
	assertFanOutFixtureStillFansOut(t)
}

// assertFanOutFixtureStillFansOut pins the shape the law above needs.
//
// The fixture is the check here as much as the assertions are: an edit that
// gave the two sub-agent lanes different opening prompts would leave every
// assertion green while removing the only shape that distinguishes a
// lane-keyed accumulator from a session-keyed one.
func assertFanOutFixtureStillFansOut(t *testing.T) {
	t.Helper()
	session, err := ledger.ReadFile(filepath.Join("..", "ledger", "testdata", "agent-lanes.jsonl"))
	if err != nil {
		t.Fatalf("the shipped fan-out fixture must parse: %v", err)
	}
	var sidechains int
	// Messages a lane meets after its own first request are the ones splitTurn
	// consults the seen-map for, so those are the ones whose sharing across
	// lanes makes the keying observable.
	laneOfMessage := map[string]string{}
	var sharedAfterFirst int
	for _, l := range session.Lanes {
		if len(l.Requests) == 0 {
			continue
		}
		if l.Sidechain {
			sidechains++
		}
		for i, r := range l.Requests {
			for _, m := range r.Context {
				if owner, ok := laneOfMessage[m.UUID]; ok {
					if owner != l.ID && i > 0 {
						sharedAfterFirst++
					}
					continue
				}
				laneOfMessage[m.UUID] = l.ID
			}
		}
	}
	if sidechains < 2 {
		t.Fatalf("the fan-out fixture carries %d sub-agent lanes, want at least 2: with fewer "+
			"there is no lane for another lane's tokens to land on", sidechains)
	}
	if sharedAfterFirst == 0 {
		t.Fatal("no lane in the fan-out fixture meets a message another lane already carried, " +
			"so a seen-map keyed by session would pass this law unchanged and the fixture " +
			"cannot distinguish the defect it exists for")
	}
}

// conservation is one lane's attribution measured against the provider's own
// figures for that lane. Both printed columns, and the one row allowed out.
type conservation struct {
	lane     string
	requests int
	// attributed is the count-once total the report shows, summed over the
	// buckets `replay context` prints.
	attributed int
	// billedOnce is what that total must equal: the last prompt, which carries
	// everything that entered and was not removed, plus whatever the provider
	// reported clearing along the way.
	//
	// The clearing term is UNEXERCISED. No session in the shipped corpus, and
	// none on the machine this was written on, carries a provider context
	// edit, so every lane checked here reports zero cleared tokens and that
	// half of the denominator cannot fail. It is written the way reconcile
	// writes it rather than dropped, because dropping it would silently change
	// the law the day a recorded context edit arrives - but it is not yet
	// evidence, and a fixture with a real recorded edit is what would make it
	// so. Deriving one by hand was tried and abandoned: the rebill clamp in
	// splitTurn, min(expected-actual, written), can consume a broken turn's
	// whole write and leave that turn's new content unattributed, and whether
	// a real provider emits that usage shape is not something a hand-built
	// fixture can settle.
	billedOnce int
	// blamed and carried are the two columns of the attribution table, summed
	// over every row including the rebill row.
	blamed, carried int
	// billedAcross is the provider's bill over the whole lane: the cost
	// integral the in-prompts column is an attribution of.
	billedAcross int
	// rebilledOnce and rebilledCarried are the named cache-break row in each
	// column: the one row allowed to sit outside the partition. They are read
	// separately rather than once, because a row counted twice in one column
	// only would otherwise move both sides of that column's law together and
	// close it while being wrong.
	rebilledOnce, rebilledCarried int
	// buckets are the rows the screen actually shows.
	buckets []ContextEntry
}

func measureConservation(s *transcript.Session, lane *transcript.Lane) conservation {
	rep := AnalyzeLane(s, lane)
	c := conservation{lane: lane.ID, requests: len(lane.Requests), buckets: EnteredContext(rep.Blame)}
	for _, e := range rep.Blame {
		c.blamed += e.Tokens.Value
		c.carried += e.PromptTokens.Value
		if e.Label == RebillLabel {
			c.rebilledOnce += e.Tokens.Value
			c.rebilledCarried += e.PromptTokens.Value
		}
	}
	for _, b := range c.buckets {
		c.attributed += b.Tokens
	}
	for _, r := range lane.Requests {
		c.billedAcross += r.Usage.PromptTotal()
	}
	if len(lane.Requests) > 0 {
		last := lane.Requests[len(lane.Requests)-1]
		c.billedOnce = last.Usage.PromptTotal() + MeasureGap(s, lane, c.attributed).ClearedTokens
	}
	return c
}

// assertConserves is the law itself, stated once for both callers.
func assertConserves(t *testing.T, name string, c conservation) {
	t.Helper()
	if c.billedOnce == 0 {
		return
	}
	// 1. The buckets on the screen sum to the bill, exactly.
	if c.attributed != c.billedOnce {
		t.Errorf("%s lane %q: the buckets account for %d tokens against the %d the provider "+
			"billed (off by %+d, %.1f%%). Every prompt token came from somewhere.",
			name, c.lane, c.attributed, c.billedOnce, c.attributed-c.billedOnce,
			100*float64(c.attributed)/float64(c.billedOnce))
	}
	// 2. Grouping the attribution into the rows a reader sees drops nothing
	//    but the one row the report names.
	if c.blamed-c.rebilledOnce != c.attributed {
		t.Errorf("%s lane %q: %d tokens of attribution became %d tokens of buckets with %d "+
			"re-billed, leaving %+d that reached neither the screen nor a named row",
			name, c.lane, c.blamed, c.attributed, c.rebilledOnce,
			c.blamed-c.rebilledOnce-c.attributed)
	}
	// 3. The in-prompts column is an attribution of the whole bill and closes
	//    against it, with the same single named exception.
	if c.carried-c.rebilledCarried != c.billedAcross {
		t.Errorf("%s lane %q: the in-prompts column sums to %d, and %d of that is the named "+
			"rebill row, against the %d the provider billed across %d requests (off by %+d). "+
			"A residual here is content counted twice or not at all.",
			name, c.lane, c.carried, c.rebilledCarried, c.billedAcross, c.requests,
			c.carried-c.rebilledCarried-c.billedAcross)
	}
	// 3b. The exemption is bounded by its own arithmetic. A break re-bills
	//     history once, on the request that broke, so the row's two columns
	//     must agree. Without this an inflated rebill row moves both sides of
	//     rule 3 together and closes it while overstating the table.
	if c.rebilledOnce != c.rebilledCarried {
		t.Errorf("%s lane %q: the rebill row reports %d tokens once and %d across prompts. "+
			"A break re-bills history on one request, so the two must agree; a row that can "+
			"differ can absorb a residual on either side of the law above.",
			name, c.lane, c.rebilledOnce, c.rebilledCarried)
	}
	// 4. No bucket is an unnamed residual. A row with no label is where a
	//    remainder goes to be invisible, and the shares must add to the whole.
	var share float64
	for _, b := range c.buckets {
		if b.Label == "" {
			t.Errorf("%s lane %q: %d tokens sit in a bucket with no name", name, c.lane, b.Tokens)
		}
		share += b.Share
	}
	if len(c.buckets) > 0 && (share < 0.9999 || share > 1.0001) {
		t.Errorf("%s lane %q: the printed shares add to %.4f, not 1: a share is a bucket over "+
			"the attributed total and they cannot fail to close unless a bucket is missing "+
			"from one side", name, c.lane, share)
	}
}

// namedSession pairs a session with the name a failure should print.
type namedSession struct {
	name    string
	session *transcript.Session
}

// conservationCorpus is every session these laws run against: the shipped
// fixtures, which run everywhere and cannot be skipped, and the machine's own
// ledger where there is one.
func conservationCorpus(t *testing.T) []namedSession {
	t.Helper()
	var out []namedSession

	tr, err := transcript.ParseClaudeCodeFile(
		filepath.Join("..", "transcript", "testdata", "session-redacted.jsonl"))
	if err != nil {
		t.Fatalf("the shipped transcript fixture must parse: %v", err)
	}
	out = append(out, namedSession{"session-redacted.jsonl", tr})

	for _, name := range []string{"late-binding-tools.jsonl", "agent-lanes.jsonl"} {
		s, rerr := ledger.ReadFile(filepath.Join("..", "ledger", "testdata", name))
		if rerr != nil {
			t.Fatalf("the shipped ledger fixture %s must parse: %v", name, rerr)
		}
		out = append(out, namedSession{name, s})
	}

	dir := os.Getenv("REPLAY_LEDGER_DIR")
	if dir == "" {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return out
		}
		dir = filepath.Join(home, ".replay", "ledger")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return out
	}
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if e.IsDir() || !ledger.IsLedgerFile(path) {
			continue
		}
		s, rerr := ledger.ReadFile(path)
		if rerr != nil || s == nil {
			continue
		}
		out = append(out, namedSession{e.Name(), s})
	}
	return out
}
