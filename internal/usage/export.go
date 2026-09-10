package usage

import (
	"encoding/json"
	"fmt"
	"sort"
)

// The usage-only input: token counts per request, with no conversation
// content anywhere in the file.
//
// Three situations produce one. A regulated or enterprise operator cannot let
// a tool read prompt content at all, but can export a usage ledger — and for
// that segment this path is the difference between an approval and a rejection
// in security review. A finance owner wants a reconciliation figure and has no
// interest in the conversation that produced it. And transcripts get rotated
// away before anyone thought to keep them, which leaves usage records as the
// only surviving evidence that the spend happened.
//
// What makes this shape workable is that Replay's cache arithmetic never
// needed the content in the first place. cachemodel.ExpectedRead,
// ClassifyRead and the usage-decidable half of ClassifyBreak read nothing but
// usage, model and timing; analysis.Calibrate touches no message body. The
// content is needed for cause, blame and alternative layouts — not for cost,
// and not for the deficit.
//
// What it costs is everything a transcript gives for free. A transcript is
// append-only, self-contained, and written by the client that made the
// requests, so it is in order and it is complete by construction. An export is
// a query result: it arrives in whatever order the query returned, it may be
// missing rows nobody noticed, and its column called "input tokens" may mean
// the opposite of what this build assumes. None of those can be detected from
// the file. They can only be declared or refused, and this file refuses.

// ExportSchema is the one shape this build reads.
//
// It is checked rather than assumed because the failure without it is silent:
// encoding/json ignores unknown keys and leaves known ones at their zero
// value, so a foreign vendor's export decodes "successfully" into a set of
// zeros and reports a corpus that cost nothing.
const ExportSchema = "replay.usage.v1"

// Entry is one usage record with the identity a report needs to place it.
//
// The counts are Record's, unchanged, because a second vocabulary for the same
// tokens is a second thing to get wrong. What is added here is only what an
// export has and a bare Record does not: which session the request belonged
// to, and — where the exporter knows them — which agent lane and which request.
type Entry struct {
	Record
	// Session is the unit of the report and is required. A record without one
	// is not a row with a missing label, it is a row that cannot be placed:
	// Go's zero value would gather every such record into a single phantom
	// session whose median and p90 are arithmetic over requests that never ran
	// together.
	Session string `json:"session"`
	// Lane is the agent lane, where the exporter records one. Absent is the
	// normal case, and it is why the usage-only report cannot state fan-out.
	Lane string `json:"lane,omitempty"`
	// ID is the provider's request id, kept so a figure can be joined back to
	// the provider's own record of the same request.
	ID string `json:"id,omitempty"`
}

// Export is a usage-only file.
type Export struct {
	Schema string `json:"schema"`
	// Complete is the exporter asserting that every request of every session
	// named here is present.
	//
	// It cannot be derived. A cache break is a request that read less than the
	// request before it wrote, so a request MISSING from the export is
	// indistinguishable from a cache break: the record after the hole is
	// compared against a request two steps back and the difference is booked
	// as a deficit. Nothing in the file says whether the hole is there. So the
	// exporter declares it, and without the declaration the break figures are
	// NOT MEASURED rather than wrong.
	Complete bool    `json:"complete"`
	Records  []Entry `json:"records"`
}

// Group is one session's records in the order they ran.
type Group struct {
	Session string
	Records []Entry
}

// ParseExport reads a usage export, refusing anything it cannot stand behind.
//
// Every refusal here is a figure that would otherwise have been printed as a
// number. They are checked at the door rather than at the point of use because
// a partially-refused export is worse than a refused one: the reader gets a
// total that silently excludes whatever failed.
func ParseExport(b []byte) (*Export, error) {
	var e Export
	if err := json.Unmarshal(b, &e); err != nil {
		// Deliberately does not mention the schema. Three different mistakes
		// arrive here and each needs its own door: not JSON at all (most often
		// the wrong file entirely, or an error page an export endpoint
		// returned with a 200), JSON of a foreign schema, and a real export
		// whose counts disagree. Sending the first reader off to check their
		// schema is the message costing more than saying nothing.
		return nil, fmt.Errorf("this file is not readable JSON, so nothing in it could be a usage record: %w", err)
	}
	if e.Schema != ExportSchema {
		got := e.Schema
		if got == "" {
			got = "no schema at all"
		}
		return nil, fmt.Errorf("this build reads %s and the file declares %q; refusing to read it hopefully, "+
			"because a foreign shape decodes into zeros and reports a corpus that cost nothing", ExportSchema, got)
	}
	if len(e.Records) == 0 {
		return nil, fmt.Errorf("the export names no records. NOT MEASURED: an export that matched nothing is "+
			"not a corpus that cost nothing, and %s will not print a $0.00 total over it", ExportSchema)
	}
	for i, r := range e.Records {
		if r.Session == "" {
			return nil, fmt.Errorf("record %d names no session; the session is the unit of this report and a "+
				"record that cannot be placed cannot be counted", i)
		}
		if r.Model == "" {
			return nil, fmt.Errorf("record %d names no model; without one there is no price to apply and no "+
				"way to tell a model change from a cache break", i)
		}
		// Negative counts pass Validate: -500 + 500 sums as happily as 0 + 0.
		// A negative token count measures nothing and subtracts from every
		// total it reaches.
		if r.Prompt < 0 || r.Fresh < 0 || r.CachedRead < 0 || r.CachedWrite < 0 || r.Output < 0 {
			return nil, fmt.Errorf("record %d carries a negative token count (%+v); it would subtract from the "+
				"corpus total", i, r.Record)
		}
		if err := r.Validate(); err != nil {
			return nil, fmt.Errorf("record %d: %w", i, err)
		}
	}
	return &e, nil
}

// BySession groups the records into per-session sequences, in the order they
// ran.
//
// The sort is the load-bearing part. A cache break is defined against the
// request immediately before it, so the order IS the measurement — and file
// order is a property of whatever query produced the export, not of the
// traffic. Sessions themselves are returned in first-appearance order so the
// output is deterministic before any caller sorts it.
func (e *Export) BySession() []Group {
	var order []string
	by := map[string][]Entry{}
	for _, r := range e.Records {
		if _, seen := by[r.Session]; !seen {
			order = append(order, r.Session)
		}
		by[r.Session] = append(by[r.Session], r)
	}
	out := make([]Group, 0, len(order))
	for _, id := range order {
		rs := by[id]
		sort.SliceStable(rs, func(i, j int) bool { return rs[i].At.Before(rs[j].At) })
		out = append(out, Group{Session: id, Records: rs})
	}
	return out
}

// SequenceEvidence reports whether this export supports the sequence a cache
// break is defined against, and names the missing fact when it does not.
//
// Two facts are needed and they fail differently, so the reason distinguishes
// them: without timestamps the records cannot be ordered at all, and without
// the completeness declaration they can be ordered but a gap between two of
// them is indistinguishable from a break. A caller that cannot tell which is
// missing cannot tell the reader what to fix.
func (e *Export) SequenceEvidence() (bool, string) {
	undated := 0
	for _, r := range e.Records {
		if r.At.IsZero() {
			undated++
		}
	}
	if undated > 0 {
		return false, fmt.Sprintf("%d of %d records carry no timestamp, so the requests cannot be put in order; "+
			"a cache break is defined against the request before it, and there is no 'before' here",
			undated, len(e.Records))
	}
	if !e.Complete {
		return false, "the export does not declare itself complete, so a request missing from it is " +
			"indistinguishable from a cache break: the record after a hole is compared against a request two " +
			"steps back and the difference books as a deficit"
	}
	return true, ""
}
