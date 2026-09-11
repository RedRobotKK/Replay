package transcript

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
)

// deepChain writes a transcript that is one unbroken parent chain: a user
// line, an assistant line that answers it, a user line whose parent is that
// assistant line, and so on for turns turns. Every user message carries its
// own index in its text, so a request's context can be checked line by line
// against the ancestry it is supposed to have.
//
// The shape matters: buildRequest walks the parent chain from scratch for
// every request, so one long chain is the worst case for that walk, and a
// transcript of many short lanes would not exercise it.
func deepChain(turns int) string {
	var b strings.Builder
	b.WriteString(`{"type":"summary","summary":"deep"}` + "\n")
	prev := ""
	for i := 0; i < turns; i++ {
		u := fmt.Sprintf("u%d", i)
		a := fmt.Sprintf("a%d", i)
		ts := fmt.Sprintf("2026-01-01T00:%02d:%02d.000Z", i/60%60, i%60)
		fmt.Fprintf(&b, `{"type":"user","uuid":%q,"parentUuid":%q,"timestamp":%q,"message":{"role":"user","content":"ask %d"}}`+"\n", u, prev, ts, i)
		fmt.Fprintf(&b, `{"type":"assistant","uuid":%q,"parentUuid":%q,"timestamp":%q,"requestId":"req%d","message":{"role":"assistant","model":"claude-opus-5","content":[{"type":"text","text":"reply %d"}],"usage":{"input_tokens":1,"output_tokens":1,"cache_read_input_tokens":1,"cache_creation_input_tokens":1}}}`+"\n", a, u, ts, i, i)
		prev = a
	}
	return b.String()
}

// TestCB1EachRequestCarriesItsOwnAncestry pins what the chain walk is for.
// The buffer it builds is reused across requests, so a failure to reset it
// would leave the previous request's ancestors in front of this one's, and a
// failure to reverse it would put the newest ancestor first. Both are
// invisible to a length check and to any assertion that only reads the last
// message, so this walks every context message of every request and names the
// turn it should have come from.
func TestCB1EachRequestCarriesItsOwnAncestry(t *testing.T) {
	const turns = 6
	s, err := ParseClaudeCode(strings.NewReader(deepChain(turns)))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Lanes) != 1 {
		t.Fatalf("lanes = %d, want 1: the fixture is one unbroken chain", len(s.Lanes))
	}
	reqs := s.Lanes[0].Requests
	if len(reqs) != turns {
		t.Fatalf("requests = %d, want %d", len(reqs), turns)
	}
	for n, r := range reqs {
		// Request n answers user turn n, so its context is every message
		// from turn 0 up to and including user turn n: 2n+1 messages,
		// alternating user, assistant, user, ... starting and ending user.
		want := 2*n + 1
		if len(r.Context) != want {
			t.Fatalf("request %d: context = %d messages, want %d", n, len(r.Context), want)
		}
		for i, m := range r.Context {
			turn := i / 2
			if i%2 == 0 {
				if m.Role != RoleUser {
					t.Fatalf("request %d context[%d]: role %s, want user", n, i, m.Role)
				}
				if got, exp := contextText(m), fmt.Sprintf("ask %d", turn); got != exp {
					t.Fatalf("request %d context[%d]: %q, want %q", n, i, got, exp)
				}
				continue
			}
			if m.Role != RoleAssistant {
				t.Fatalf("request %d context[%d]: role %s, want assistant", n, i, m.Role)
			}
			if got, exp := contextText(m), fmt.Sprintf("reply %d", turn); got != exp {
				t.Fatalf("request %d context[%d]: %q, want %q", n, i, got, exp)
			}
		}
	}
}

func contextText(m *Message) string {
	var parts []string
	for _, b := range m.Blocks {
		parts = append(parts, b.Text)
	}
	return strings.Join(parts, "")
}

// allocBytesForChain reports bytes allocated parsing one chain of that depth.
func allocBytesForChain(turns int) uint64 {
	src := deepChain(turns)
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	s, err := ParseClaudeCode(strings.NewReader(src))
	runtime.ReadMemStats(&after)
	if err != nil || s == nil || len(s.Lanes) != 1 {
		panic("deepChain fixture did not parse as one lane")
	}
	return after.TotalAlloc - before.TotalAlloc
}

// TestCB2ChainWalkDoesNotReallocatePerRequest freezes the cost of the walk
// itself. Every request visits every one of its ancestors, so a chain of d
// turns performs about d*d ancestor visits no matter what; what this pins is
// how many bytes each visit costs.
//
// A request's Context is one pointer per ancestor and is kept, so the floor is
// 8 bytes a visit and the total is quadratic however it is written. The old
// code paid roughly seven times that: a fresh chain slice per request, grown
// by doubling, thrown away at the end of the call, and a Context slice grown
// the same way. Measured on the fixture below, Go 1.24, darwin/arm64:
//
//	depth    before    after
//	  250   3.92 MB   1.59 MB
//	  500  12.75 MB   4.19 MB
//	 1000  52.80 MB  12.53 MB
//	 2000 225.41 MB  41.90 MB
//
// which is 56.4 bytes a visit before and 10.5 after. The ceiling sits at 25:
// more than twice the cost of the presized form, less than half the cost of
// the growing one, so reverting either the reused buffer or the presized
// Context fails it and ordinary allocator variation does not.
func TestCB2ChainWalkDoesNotReallocatePerRequest(t *testing.T) {
	const depth = 2000
	const maxBytesPerVisit = 25.0

	got := float64(allocBytesForChain(depth)) / float64(depth*depth)
	if got > maxBytesPerVisit {
		t.Fatalf("chain walk allocates %.1f bytes per ancestor visit at depth %d, ceiling %.0f\n"+
			"the parent walk is allocating per request again: check that buildRequest\n"+
			"reuses decoder.chain and presizes req.Context", got, depth, maxBytesPerVisit)
	}
	t.Logf("%.1f bytes per ancestor visit at depth %d (ceiling %.0f)", got, depth, maxBytesPerVisit)
}
