package analysis

import "github.com/RedRobotKK/Replay/internal/transcript"

// SessionSpend is what a whole session cost as it ran: every lane, each
// request once.
//
// It exists because `replay cost` priced MainLane(session) and nothing else.
// On the corpus that surfaced it, that dropped 21,854 of 60,401 requests —
// 36.2% — concentrated in 8 files out of 1,812. Those eight are the fan-out
// sessions, which is why 36% of the requests was 2.8x of the dollars:
// `replay cost` reported $3,721 where `replay burn`, pricing every lane,
// reported $10,499 on the same corpus in the same minute.
//
// `replay blame` has disclosed the gap per session all along — "Scope: 1 of 17
// lanes; 8336 requests in this session's other lanes are not counted here" —
// and the surface reporting the TOTAL had no equivalent line. A scope note on
// the report that looks at one session, and silence on the report that adds
// them up, is the wrong way round.
type SessionSpend struct {
	PolicyResult
	// Lanes is how many lanes the total covers.
	Lanes int
	// Duplicated counts requests skipped because their id had already been
	// priced from another lane. A sub-agent lane re-renders some of its
	// parent's requests, and summing lanes naively would bill those twice —
	// replacing an undercount with an overcount, which is harder to notice
	// because the number moves the way people expect.
	Duplicated int
	// Unidentified counts requests carrying no id. They cannot be deduplicated
	// and are counted anyway: absence is not a duplicate, and treating an
	// unidentifiable request as already-seen would undercount precisely the
	// traffic nobody can check.
	Unidentified int
	// MixedEpochs is true when the session spans more than one kernel epoch.
	// CostUSD is still the sum; it is not one as-run. The judge of an epoch
	// is provider usage / ExpectedRead, not the epoch label itself.
	//
	// An unlabelled request is absence, not a third epoch. A body with no
	// tools key hashes to "", which is routine — a first request, or a
	// tool-free sub-agent lane — and counting it would report MixedEpochs on
	// a session that ran under exactly one.
	MixedEpochs bool
}

// AsRunSession prices every lane of a session, counting each request once.
//
// The dedup is by request id across lanes, which is the same key `replay cost`
// already uses to disclose cross-FILE re-renders. On the corpus this was
// written against it removes 428 requests of 59,967 — the known ~1.1% — so it
// is a correction rather than the point. The point is the 36% that was missing.
func AsRunSession(s *transcript.Session) SessionSpend {
	out := SessionSpend{PolicyResult: PolicyResult{Name: "as-run", ReachableLive: "n/a"}}
	if s == nil {
		return out
	}
	// No nil checks on the lanes or the requests. The parser appends only what
	// it constructed — session.Lane never yields a nil, and a request that
	// failed to build is counted in Skipped and never appended — so a nil here
	// is a state nothing produces. AsRun, which this generalises, does not
	// check for one either. guard-reachability reported both as unobserved and
	// it was right: they were guarding against nothing.
	seen := make(map[string]bool)
	epochs := map[string]bool{}
	for _, lane := range s.Lanes {
		out.Lanes++
		for _, req := range lane.Requests {
			switch {
			case req.ID == "":
				out.Unidentified++
			case seen[req.ID]:
				out.Duplicated++
				continue
			default:
				seen[req.ID] = true
			}
			if req.Epoch != "" {
				epochs[req.Epoch] = true
			}
			out.AddAt(req.Usage, req.Model, req.Timestamp)
		}
	}
	out.MixedEpochs = len(epochs) > 1
	return out
}

// AnalyzeEveryLane returns one report per lane, in session order.
//
// The single-lane case, which is 1,804 of 1,812 files on the corpus this was
// written against, allocates one report exactly as before.
//
// It exists so a session's causes cover the same traffic as its cost. Reporting
// a whole session's dollars beside the main lane's cache breaks is the defect
// this file fixes, one column over: the total moved and the explanation for it
// did not.
func AnalyzeEveryLane(s *transcript.Session) []*LaneReport {
	if s == nil {
		return nil
	}
	out := make([]*LaneReport, 0, len(s.Lanes))
	for _, lane := range s.Lanes {
		out = append(out, AnalyzeLane(s, lane))
	}
	return out
}
