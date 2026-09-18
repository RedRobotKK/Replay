package version

import "testing"

// "unknown" is not a commit, and omitempty cannot tell the difference.
//
// Commit defaults to the string "unknown" for any build that was not stamped
// by goreleaser or the Makefile, which is every `go install ...@latest` and
// every plain `go build`. `omitempty` drops an EMPTY string and "unknown" is
// not empty, so the sentinel shipped in the corpus payload and the receiver
// refused the submission: `field commit is not 7 to 40 lowercase hex
// characters`.
//
// Every unit test passed while this was broken. It took building a real
// submission and handing it to the real validator to see it, which is the
// argument for that check existing at all.
func TestKnownCommitDropsTheSentinel(t *testing.T) {
	for _, tc := range []struct {
		in, want, why string
	}{
		{"unknown", "", "the sentinel is not a commit and must leave as absent"},
		{"", "", "an empty commit is already absent"},
		{"87a9b2a", "87a9b2a", "a real abbreviated sha is carried through"},
		{"0bcb2cb1f4e5a6d7c8b9a0e1f2d3c4b5a6978877", "0bcb2cb1f4e5a6d7c8b9a0e1f2d3c4b5a6978877", "a full sha is carried through"},
	} {
		if got := KnownCommit(tc.in); got != tc.want {
			t.Errorf("KnownCommit(%q) = %q, want %q: %s", tc.in, got, tc.want, tc.why)
		}
	}
}
