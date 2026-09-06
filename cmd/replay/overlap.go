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
