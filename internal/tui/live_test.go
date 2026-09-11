package tui

import (
	"strings"
	"testing"
	"time"
)

func liveText(s Screen) string { return strings.Join(s.Lines, "\n") }

// Unreachable is not zero. A proxy that is not running must never render as a
// proxy with nothing flowing through it.
func TestLive1(t *testing.T) {
	got := liveText(LiveScreen(Live{Reachable: false, Addr: "127.0.0.1:4000"}, time.Now()))
	if !strings.Contains(got, "127.0.0.1:4000") {
		t.Errorf("must name the address it tried:\n%s", got)
	}
	// Distinguish it from up-and-empty, which also names the address and
	// prints no percentages. Without this the two states are the same screen.
	if !strings.Contains(got, "no proxy answered") {
		t.Errorf("must say no proxy answered, not merely omit numbers:\n%s", got)
	}
	for _, bad := range []string{"proxy up", "recorded nothing", "ANTHROPIC_BASE_URL"} {
		if strings.Contains(got, bad) {
			t.Errorf("unreachable screen claimed %q, which belongs to the up-and-empty state:\n%s", bad, got)
		}
	}
}

// Up and recording nothing is its own state, and the most important one.
//
// The proxy on this machine ran 24.8 hours with sessions: null because nothing
// was pointed at it. Nobody noticed, because nothing renders it. An empty table
// looks like a quiet day; this has to look like a misconfiguration.
func TestLive2(t *testing.T) {
	got := liveText(LiveScreen(Live{Reachable: true, Addr: "127.0.0.1:4000", UptimeSeconds: 89236}, time.Now()))
	if !strings.Contains(got, "24h") {
		t.Errorf("must state how long it has been up, got:\n%s", got)
	}
	// The HEADLINE must carry it. Prose two paragraphs down is not the same
	// thing: a reader glancing at the top of the screen has to see the problem.
	//
	// The screen opens with two rows it did not have before — the question it
	// answers, and the blank under it — so the headline is the third row rather
	// than the first. It is pinned by index here rather than searched for, which is
	// stricter than what it replaced: the old check read everything before the
	// first newline and would have gone on passing if the headline slid a row
	// further down behind more chrome. This fails the moment it moves.
	rows := strings.Split(got, "\n")
	if len(rows) < 3 {
		t.Fatalf("the live screen has no body under its title:\n%s", got)
	}
	head := rows[2]
	if !strings.Contains(head, "recorded nothing") {
		t.Errorf("the headline must say it has recorded nothing, got %q", head)
	}
	if !strings.Contains(strings.ToLower(got), "anthropic_base_url") {
		t.Errorf("must name the fix, since the cause is always the same:\n%s", got)
	}
}

// With traffic, it renders what is flowing through.
func TestLive3(t *testing.T) {
	now := time.Now()
	got := liveText(LiveScreen(Live{
		Reachable: true, Addr: "127.0.0.1:4000", UptimeSeconds: 300,
		Sessions: []LiveSession{
			{ID: "facfd32e", Model: "claude-opus-5", Requests: 14, PromptTokens: 1_200_000,
				CachedShare: 0.981, Breaks: 2, CostUSD: 3.10, LastSeen: now.Add(-20 * time.Second)},
		},
	}, now))
	for _, want := range []string{"facfd32e", "opus-5", "98%", "2"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}
}

// A live reading is Measured. It comes off the wire, not from a fixture.
func TestLive4(t *testing.T) {
	if from := LiveScreen(Live{Reachable: true, UptimeSeconds: 5}, time.Now()).From; from != Measured {
		t.Errorf("live proxy state is Measured, got %v", from)
	}
}
