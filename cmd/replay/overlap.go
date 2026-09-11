package main

import "fmt"

// overlapNote discloses requests counted in more than one transcript, or
// returns empty when there were none.
//
// A sub-agent lane re-renders its parent's requests, so the same requestId can
// appear in several files. MainLane skips sidechain lanes and absorbs most of
// it - measured at 430 of 30,716 requests, 1.4%, against 54.9% of raw usage
// records. The residue is small and it is not zero, and a total printed without
// it is a total printed as exact.
func overlapNote(duplicated, total int) string {
	if duplicated <= 0 || total <= 0 {
		return ""
	}
	return fmt.Sprintf("\n%d of %d requests (%.1f%%) appear in more than one transcript, because a\n"+
		"sub-agent lane re-renders its parent's requests. They are counted once per\n"+
		"transcript here, so the total is high by about that much.\n",
		duplicated, total, float64(duplicated)/float64(total)*100)
}

// unjoinableNote discloses requests whose id was not the provider's, or
// returns empty when every one of them was.
//
// The overlap figure above is a count of requests that matched across files by
// id. That is only a measurement where the id came from the provider. A ledger
// record whose provider sent none is named for its position in its file, so
// every ledger file has a `ledger-0`, and matching on it would report requests
// from unrelated sessions as the same request. They are excluded from the
// match rather than joined on a name this program invented — and excluding
// them silently would turn the overlap figure back into a claim of exactness.
func unjoinableNote(unjoinable, total int) string {
	if unjoinable <= 0 || total <= 0 {
		return ""
	}
	return fmt.Sprintf("\n%d of %d requests (%.1f%%) carry no provider request id, so they cannot be\n"+
		"matched across files at all and are not part of the overlap figure above.\n"+
		"Run the proxy to record one, or expect this count to stay where it is.\n",
		unjoinable, total, float64(unjoinable)/float64(total)*100)
}

// requestJoin counts a corpus's requests by whether they can be joined across
// files, and by whether they already were.
//
// It exists as a type because the rule it enforces is one line and easy to
// drop: an id that this program synthesised is not a join key. Written inline
// at the call site, that rule was `seenReq[r.ID]` with nothing to say the two
// kinds of id were different.
type requestJoin struct {
	seen       map[string]bool
	total      int
	duplicated int
	unjoinable int
}

func newRequestJoin() *requestJoin { return &requestJoin{seen: map[string]bool{}} }

// add offers one request. measured says the id came off the provider's wire.
func (j *requestJoin) add(id string, measured bool) {
	j.total++
	if !measured {
		j.unjoinable++
		return
	}
	if j.seen[id] {
		j.duplicated++
		return
	}
	j.seen[id] = true
}

// addCached replays a file already summarised in the index: the provider ids
// it contributed, and how many of its requests had none.
func (j *requestJoin) addCached(ids []string, unjoinable int) {
	for _, id := range ids {
		j.add(id, true)
	}
	for i := 0; i < unjoinable; i++ {
		j.add("", false)
	}
}
