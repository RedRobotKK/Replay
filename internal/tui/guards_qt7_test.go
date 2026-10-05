package tui

import (
	"strings"
	"testing"
)

// The two QT-7 lines on the guards screen, proven from inside this package.
//
// QT-7a added the count of 2xx responses that carried no usage and QT-7c the
// count of bodies that could not be read. Both were proven on the doctor
// surface and across the two surfaces from cmd/replay, and never from here,
// so the repository's guard-reachability check, which scores a conditional
// only against its own package's tests, reported both branches as never
// entered. This is the in-package evidence; the behaviour is unchanged.
//
// Each count is its own branch and its own sentence, because the two states
// want different operator actions: a provider that omits usage wants a
// usage-reporting option or a price-table update, a body nobody could read
// is a defect between client, proxy and provider. The line is urgent only
// when a dollar cap is set, because that is when traffic the cap cannot see
// is a limit the operator believes in and does not have.
func TestGuardsScreenNamesTheResponsesADollarCapDidNotSee(t *testing.T) {
	capped := GuardState{Reachable: true, Addr: "127.0.0.1:4000",
		ResponsesWithoutUsage: 5, ResponsesUnparsed: 4, Caps: Caps{DayUSD: true}}
	body := GuardsScreen(capped, nil, 0).String()
	for _, want := range []string{
		"  ! 5 response(s) carried no usage; nothing was",
		"  ! 4 response(s) could not be read; nothing was",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the guards screen does not say %q under a day cap:\n%s", want, body)
		}
	}
	if strings.Count(body, "so a dollar cap did not see them") != 2 {
		t.Errorf("each count carries its own consequence line:\n%s", body)
	}

	// Without a dollar cap the same counts are reported and not urgent.
	uncapped := capped
	uncapped.Caps = Caps{SessionTokens: true}
	body = GuardsScreen(uncapped, nil, 0).String()
	for _, want := range []string{
		"  - 5 response(s) carried no usage; nothing was",
		"  - 4 response(s) could not be read; nothing was",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("without a dollar cap the line is informational, not urgent; want %q:\n%s", want, body)
		}
	}

	// Each line answers to its own count. A zero on one must not suppress or
	// invent the other.
	onlyUnread := capped
	onlyUnread.ResponsesWithoutUsage = 0
	body = GuardsScreen(onlyUnread, nil, 0).String()
	if strings.Contains(body, "carried no usage") {
		t.Errorf("zero responses without usage still produced a line:\n%s", body)
	}
	if !strings.Contains(body, "4 response(s) could not be read") {
		t.Errorf("the unread count was dropped with the other count:\n%s", body)
	}
	onlyNoUsage := capped
	onlyNoUsage.ResponsesUnparsed = 0
	body = GuardsScreen(onlyNoUsage, nil, 0).String()
	if strings.Contains(body, "could not be read") {
		t.Errorf("zero unreadable bodies still produced a line:\n%s", body)
	}
	if !strings.Contains(body, "5 response(s) carried no usage") {
		t.Errorf("the no-usage count was dropped with the other count:\n%s", body)
	}
}
