package main

import "testing"

// The corpus report is the payload ADR-0007 designed for submission — never
// sent, because no submit path was built — and its
// own header promises "a session id prefix, never a path, project name, or
// content". The "Sessions not analyzed" list broke that promise: it scrubbed
// the directory but kept the filename, which for Claude Code is the full
// session UUID. Analysed rows were already shortened; skipped ones were not.
func TestScrubPathShortensSessionIdentifiers(t *testing.T) {
	cases := []struct{ in, want string }{
		{
			"/Users/someone/.claude/projects/-Users-someone-secret-client/2415858d-1344-4ecd-b752-aa9363029fe8.jsonl: no provider requests found",
			"2415858d: no provider requests found",
		},
		{
			"2415858d-1344-4ecd-b752-aa9363029fe8.jsonl: no provider requests found",
			"2415858d: no provider requests found",
		},
		{"something went wrong", "something went wrong"},
	}
	for _, c := range cases {
		if got := scrubPath(c.in); got != c.want {
			t.Errorf("scrubPath(%q)\n got  %q\n want %q", c.in, got, c.want)
		}
	}
}
