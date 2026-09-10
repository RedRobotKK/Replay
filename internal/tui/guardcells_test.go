package tui

import (
	"strings"
	"testing"
)

// Each cell of the guards table, in each state it has.
//
// The screen-level tests in guards_test.go assert on the whole rendered body,
// which is how a branch stays inert while a test looks like it covers it: a
// string that appears in two places passes a Contains check whichever place
// produced it. guard-reachability reported thirty such branches on this change.
// These are the cells themselves, one state at a time.

func TestProxyRowSaysWhichOfThreeThingsIsTrue(t *testing.T) {
	if got := proxyRow(GuardState{}); !strings.Contains(got, "no answer") {
		t.Errorf("an unreachable proxy renders %q", got)
	}
	// Reachable but nameless: the addr is unknown, not empty-string-as-address.
	if got := proxyRow(GuardState{Reachable: true}); got != "answering" {
		t.Errorf("a reachable proxy with no address renders %q, want a bare "+
			"\"answering\" rather than \"answering at \"", got)
	}
	if got := proxyRow(GuardState{Reachable: true, Addr: "127.0.0.1:4000"}); !strings.Contains(got, "127.0.0.1:4000") {
		t.Errorf("a reachable proxy does not name where it answered: %q", got)
	}
}

func TestCapsRowDistinguishesNoneFromUnknown(t *testing.T) {
	unknown := capsRow(GuardState{})
	none := capsRow(GuardState{Reachable: true})
	if unknown == none {
		t.Fatalf("a proxy that did not answer and a proxy with no caps set both "+
			"render %q. One means nothing is enforced and the other means nobody "+
			"asked", unknown)
	}
	if !strings.Contains(none, "nothing will be refused") {
		t.Errorf("a reachable proxy with no caps does not say what that means: %q", none)
	}
	set := capsRow(GuardState{Reachable: true, Caps: Caps{SessionUSD: true, DayTokens: true}})
	for _, want := range []string{"session $", "day tokens"} {
		if !strings.Contains(set, want) {
			t.Errorf("the caps row omits %q: %q", want, set)
		}
	}
	if strings.Contains(set, "day $") {
		t.Errorf("the caps row names a cap that is not set: %q", set)
	}
}

func TestRefusedRowCountsAndNames(t *testing.T) {
	if got := refusedRow(GuardState{}); !strings.Contains(got, "unknown") {
		t.Errorf("an unreachable proxy reports refusals as %q", got)
	}
	if got := refusedRow(GuardState{Reachable: true}); !strings.Contains(got, "none so far") {
		t.Errorf("no refusals renders %q; zero is a measurement here and must not "+
			"read as unknown", got)
	}
	// A count with no breakdown: the endpoint reported a total and no kinds.
	if got := refusedRow(GuardState{Reachable: true, Refused: 3}); got != "3" {
		t.Errorf("three refusals with no named guard renders %q, want the bare count", got)
	}
	named := refusedRow(GuardState{Reachable: true, Refused: 4,
		Refusals: map[string]int{"spend_cap": 3, "loop": 1}})
	if !strings.Contains(named, "spend_cap 3") || !strings.Contains(named, "loop 1") {
		t.Errorf("the refusal row does not name which guard fired: %q", named)
	}
	// Sorted, so two looks at one state read the same.
	if strings.Index(named, "loop") > strings.Index(named, "spend_cap") {
		t.Errorf("refusal kinds are not sorted: %q", named)
	}
}

func TestSpentRowSaysWhenTheCapIsNotEnforced(t *testing.T) {
	if got := spentRow(GuardState{}); !strings.Contains(got, "unknown") {
		t.Errorf("an unreachable proxy reports spend as %q", got)
	}
	ok := spentRow(GuardState{Reachable: true, CostUSD: 12.34, DayCostUSD: 2.10})
	if !strings.Contains(ok, "12.34") || !strings.Contains(ok, "2.10") {
		t.Errorf("the spend row drops a figure: %q", ok)
	}
	loud := spentRow(GuardState{Reachable: true, CostUSD: 12.34, DayCostUSD: 2.10,
		SpendCapNotEnforced: true})
	if !strings.Contains(loud, "NOT enforced") {
		t.Errorf("an unenforced cap is not marked in the spend cell: %q", loud)
	}
	if loud == ok {
		t.Error("an unenforced cap renders identically to an enforced one")
	}
}

func TestNextForPrefersTheCapTheOperatorAlreadyHas(t *testing.T) {
	covered := nextFor(GuardState{Caps: Caps{DayTokens: true}})
	if len(covered) != 1 || !strings.Contains(covered[0], "already covers") {
		t.Errorf("an operator running a token cap is told to add one: %v", covered)
	}
	bare := nextFor(GuardState{})
	if len(bare) < 2 {
		t.Fatalf("an operator with no token cap gets %d line(s) of advice", len(bare))
	}
	if !strings.Contains(strings.Join(bare, " "), "--max-day-tokens") {
		t.Errorf("the advice does not name the flag that fixes it: %v", bare)
	}
}

// The advice block is indented to match the rest of the screen. The command
// side writes these for a printed report, where the left margin is the
// terminal.
func TestAdviceLinesAreIndentedToTheScreen(t *testing.T) {
	body := GuardsScreen(liveGuards(), []string{"caps from your own spread:"}, 20).String()
	if strings.Contains(body, "\ncaps from your own spread") {
		t.Errorf("an advice line starts at column zero, so it reads as a different "+
			"screen element:\n%s", body)
	}
}
