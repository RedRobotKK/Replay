package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/proxy"
	"github.com/RedRobotKK/Replay/internal/tui"
)

// The doctor screen and `replay doctor` count the same corpus. This checks they
// count it the same way.
//
// `replay doctor` prints countTranscripts().sessions. The doctor screen printed
// sessions+lanes under the word "transcripts", so the two surfaces reported one
// directory an order of magnitude apart — 124 sessions against 1,809
// transcripts, same machine, same minute. Both numbers were correct; only one
// was the number the sentence around it claimed.
//
// The corpus here is built rather than borrowed. The first version of this test
// read whatever HOME held, and the suite's TestMain points HOME at an empty
// temporary directory — so every figure it compared was zero, it passed against
// a deliberately broken build, and it would have gone on passing forever. A
// fixture with a fan-out is what makes the two fields distinguishable at all.

// fannedOutCorpus writes a corpus whose session count and file count differ,
// and returns what a correct walk must find.
func fannedOutCorpus(t *testing.T) (sessions, lanes, projects int) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home) // Windows reads this one
	t.Setenv("REPLAY_TRANSCRIPTS", "")

	root := filepath.Join(home, ".claude", "projects")
	for _, proj := range []string{"proj-a", "proj-b"} {
		dir := filepath.Join(root, proj)
		lanesDir := filepath.Join(dir, "subagents")
		if err := os.MkdirAll(lanesDir, 0o755); err != nil {
			t.Fatal(err)
		}
		// Sessions live at the project's own level.
		for _, s := range []string{"aaaa1111", "bbbb2222", "cccc3333"} {
			write(t, filepath.Join(dir, s+".jsonl"))
			sessions++
		}
		// Sub-agent lanes live below it. Five per project, so the two figures
		// cannot be confused for one another by coincidence.
		for _, l := range []string{"d1", "d2", "d3", "d4", "d5"} {
			write(t, filepath.Join(lanesDir, "agent-"+l+".jsonl"))
			lanes++
		}
		projects++
	}
	return sessions, lanes, projects
}

func write(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTheScreenAndTheCommandCountTheSameSessions(t *testing.T) {
	wantSessions, wantLanes, wantProjects := fannedOutCorpus(t)

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	projects := filepath.Join(claudeConfigDir(home), "projects")
	if roots := defaultTranscriptRoots(home); len(roots) > 0 {
		projects = roots[0]
	}
	c := countTranscripts(projects)

	// The fixture is only useful if the walk finds it. A test whose corpus the
	// code under test cannot see is the zero-against-zero comparison again.
	if c.sessions != wantSessions || c.lanes != wantLanes || c.projects != wantProjects {
		t.Fatalf("the walk found %d sessions / %d lanes / %d projects under %s, "+
			"want %d / %d / %d. The fixture is not being read, so nothing below "+
			"this line means anything",
			c.sessions, c.lanes, c.projects, projects, wantSessions, wantLanes, wantProjects)
	}
	if c.lanes == 0 || c.sessions == c.sessions+c.lanes {
		t.Fatal("the fixture has no fan-out, so sessions and files are the same " +
			"number and this test cannot tell them apart")
	}

	m := machineState()
	if m.Sessions != c.sessions {
		t.Errorf("the screen reports %d sessions, the command %d", m.Sessions, c.sessions)
	}
	if m.Transcripts != c.sessions+c.lanes {
		t.Errorf("the screen reports %d transcript files, the walk found %d",
			m.Transcripts, c.sessions+c.lanes)
	}
	if m.Lanes != c.lanes {
		t.Errorf("the screen reports %d lanes, the walk found %d", m.Lanes, c.lanes)
	}
	if m.Projects != c.projects {
		t.Errorf("the screen reports %d projects, the walk found %d", m.Projects, c.projects)
	}
}

// The guards screen and `replay doctor` read the same guard state.
//
// The screen used to render `advise --guards` — caps suggested from history,
// written nowhere — under a key promising "every guard, whether it is armed,
// and whether it can fire". So the one screen named for the budget question
// could not report proxy.Status.SpendCapNotEnforced: a dollar cap set against
// traffic that cannot be priced, where the operator believes they have a limit
// they do not have. doctor reported it, loudly. This asserts both surfaces
// still take it from the same field.
func TestBothSurfacesReadTheSameGuardFields(t *testing.T) {
	st := proxy.Status{
		Requests:            map[string]int{"refused": 4},
		Refusals:            map[string]int{"spend_cap": 3, "loop": 1},
		CostUSD:             12.34,
		DayCostUSD:          2.10,
		SpendCapNotEnforced: true,
		Caps:                proxy.CapStatus{DayUSD: true, SessionTokens: true},
	}
	// What doctor says.
	doctor := strings.Join(guardLines(st), "\n")
	if !strings.Contains(doctor, "WARNING") {
		t.Fatal("doctor no longer warns about an unenforced dollar cap; this test is " +
			"checking nothing")
	}

	// What the screen says, from a GuardState carrying the same fields.
	g := tui.GuardState{
		Reachable: true, Addr: "127.0.0.1:4000",
		Refused: st.Requests["refused"], Refusals: st.Refusals,
		CostUSD: st.CostUSD, DayCostUSD: st.DayCostUSD,
		SpendCapNotEnforced: st.SpendCapNotEnforced,
		Caps: tui.Caps{
			SessionUSD: st.Caps.SessionUSD, DayUSD: st.Caps.DayUSD,
			SessionTokens: st.Caps.SessionTokens, DayTokens: st.Caps.DayTokens,
		},
	}
	screen := tui.GuardsScreen(g, nil, 0).String()
	if !strings.Contains(screen, "NOT enforced") {
		t.Errorf("doctor warns about the unenforced cap and the screen does not:\n%s", screen)
	}
	// Both name the refusing guard, not just a count.
	for _, want := range []string{"spend_cap", "loop"} {
		if !strings.Contains(doctor, want) {
			t.Errorf("doctor does not name %q", want)
		}
		if !strings.Contains(screen, want) {
			t.Errorf("the screen does not name %q", want)
		}
	}
}
