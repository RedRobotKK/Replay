package analysis

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// Attribution must reconcile against what the provider billed.
//
// This is the check that did not exist, and its absence let the product be
// silently wrong on screen. On the real ledger, `replay context` reported "1k
// tokens of content entered this context" for a session whose last prompt the
// provider measured at 203,466 tokens, and printed `tool 0.0%` against 439,611
// bytes of tool definitions. The cause was that Blame read the shared prefix
// only from the first request, and MCP tools bind after it: three of four real
// sessions carry no tools on request one and the whole block on request two.
//
// The law is conservation, in the units the screen actually shows. Tokens is a
// count-once figure, so it reconciles against ONE prompt — the last one, which
// carries everything that ever entered and was not removed — plus whatever the
// provider reported clearing along the way. It does not reconcile against the
// sum over requests: that is a cost integral, counts a carried block once per
// request that carried it, and is what BlameEntry.PromptTokens measures.
//
// Getting that denominator wrong is not academic. Against the summed bill this
// session's own first version of this test passed at 11.3% on one session and
// would have gone red on correct code as soon as the session grew one more
// turn.
//
// PASS: attributed tokens equal the provider's own last prompt, within a
// tolerance that only absorbs the rounding in splitting a turn's tokens across
// its blocks by bytes.
// FAIL: a gap. And a gap here is not a rounding question — the failures this
// was written for were two orders of magnitude.
func TestReconcile_AttributionSumsToWhatTheProviderBilled(t *testing.T) {
	// The shipped fixture runs everywhere and cannot be skipped. A check that
	// only runs on one machine's private ledger is not a check in CI.
	t.Run("fixture", func(t *testing.T) {
		session, err := transcript.ParseClaudeCodeFile(
			filepath.Join("..", "transcript", "testdata", "session-redacted.jsonl"))
		if err != nil {
			t.Fatalf("the shipped fixture must parse: %v", err)
		}
		reconcile(t, "session-redacted.jsonl", session)
	})

	// A ledger fixture with the shape that broke this: tools bind after the
	// first request. It ships, so the defect is guarded on a machine that has
	// never run the proxy, where the private ledger arm below skips.
	t.Run("late-binding-tools", func(t *testing.T) {
		session, err := ledger.ReadFile(
			filepath.Join("..", "ledger", "testdata", "late-binding-tools.jsonl"))
		if err != nil {
			t.Fatalf("the shipped ledger fixture must parse: %v", err)
		}
		reconcile(t, "late-binding-tools.jsonl", session)
	})

	t.Run("ledger", func(t *testing.T) {
		dir := os.Getenv("REPLAY_LEDGER_DIR")
		if dir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				t.Skip("no home directory")
			}
			dir = filepath.Join(home, ".replay", "ledger")
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Skipf("no ledger to reconcile against: %v", err)
		}
		var checked int
		for _, e := range entries {
			path := filepath.Join(dir, e.Name())
			if e.IsDir() || !ledger.IsLedgerFile(path) {
				continue
			}
			session, serr := ledger.ReadFile(path)
			if serr != nil || session == nil {
				continue
			}
			checked++
			reconcile(t, e.Name(), session)
		}
		if checked == 0 {
			t.Skip("no ledger sessions to reconcile")
		}
		t.Logf("reconciled %d ledger session(s)", checked)
	})
}

// reconcile asserts the conservation law on one session.
func reconcile(t *testing.T, name string, session *transcript.Session) {
	t.Helper()
	lane := MainLane(session)
	if lane == nil || len(lane.Requests) == 0 {
		return
	}
	// The same call the command makes: cmd/replay/context.go builds its rows
	// from rep.Blame, so reconciling anything else would be reconciling a
	// figure nobody is shown.
	rep := AnalyzeLane(session, lane)
	var attributed int
	for _, c := range EnteredContext(rep.Blame) {
		attributed += c.Tokens
	}

	last := lane.Requests[len(lane.Requests)-1]
	want := last.Usage.PromptTotal() + MeasureGap(session, lane, attributed).ClearedTokens
	if want == 0 {
		return
	}

	// Exactly, not approximately. shareByBytes gives the last block the
	// remainder when it splits a turn's tokens by bytes, so the sum is an
	// identity rather than an estimate, and it holds to the token on the
	// shipped fixtures and on every real session measured. A tolerance here
	// would be slack with nothing to absorb: a one-percent band was tried and
	// its only effect was to let a mutation through that dropped the first
	// request's whole prefix.
	//
	// If this ever fails by a small margin rather than an order of magnitude,
	// the likely causes are a turn whose write was smaller than the previous
	// output and got clamped to zero, or a block list whose bytes sum to zero
	// while its token count does not. Both are defects worth seeing.
	if attributed != want {
		t.Errorf("%s: attribution accounts for %d tokens against the %d the provider "+
			"measured in the last prompt (%.1f%%, off by %d). Every prompt token the "+
			"provider charged for came from somewhere, and attribution that does not "+
			"sum to the bill is not attribution.",
			name, attributed, want, 100*float64(attributed)/float64(want), attributed-want)
	}
}

// Content that entered the context must be attributed to the content that
// carried it, not merely counted somewhere.
//
// Conservation is necessary and not sufficient: it pins the total and says
// nothing about the distribution. Both mutations that reintroduce the original
// defect keep the total exact and move 202,271 tokens off the tool definitions
// and onto whatever block happened to be adjacent — which is precisely the bug
// as the user saw it, `tool 0.0%` printed against 439,611 bytes of tool
// definitions on a session that reconciled to the token.
//
// So this is the law with teeth: a block carrying bytes cannot be attributed
// nothing. Zero-byte blocks are exempt, because a request that carried no
// tools genuinely has none to attribute.
//
// PASS: every label that ever carried bytes holds a non-zero share.
// FAIL: a label with bytes and no tokens — a row the report prints as 0.0%.
func TestReconcile_EveryBlockWithBytesIsAttributed(t *testing.T) {
	t.Run("fixture", func(t *testing.T) {
		session, err := transcript.ParseClaudeCodeFile(
			filepath.Join("..", "transcript", "testdata", "session-redacted.jsonl"))
		if err != nil {
			t.Fatalf("the shipped fixture must parse: %v", err)
		}
		assertEveryBlockAttributed(t, "session-redacted.jsonl", session)
	})

	// A ledger fixture with the shape that broke this: tools bind after the
	// first request. It ships, so the defect is guarded on a machine that has
	// never run the proxy, where the private ledger arm below skips.
	t.Run("late-binding-tools", func(t *testing.T) {
		session, err := ledger.ReadFile(
			filepath.Join("..", "ledger", "testdata", "late-binding-tools.jsonl"))
		if err != nil {
			t.Fatalf("the shipped ledger fixture must parse: %v", err)
		}
		assertEveryBlockAttributed(t, "late-binding-tools.jsonl", session)
	})

	t.Run("ledger", func(t *testing.T) {
		dir := os.Getenv("REPLAY_LEDGER_DIR")
		if dir == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				t.Skip("no home directory")
			}
			dir = filepath.Join(home, ".replay", "ledger")
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Skipf("no ledger: %v", err)
		}
		for _, e := range entries {
			path := filepath.Join(dir, e.Name())
			if e.IsDir() || !ledger.IsLedgerFile(path) {
				continue
			}
			session, serr := ledger.ReadFile(path)
			if serr != nil || session == nil {
				continue
			}
			assertEveryBlockAttributed(t, e.Name(), session)
		}
	})
}

func assertEveryBlockAttributed(t *testing.T, name string, session *transcript.Session) {
	t.Helper()
	lane := MainLane(session)
	if lane == nil || len(lane.Requests) == 0 {
		return
	}
	bytesByLabel := map[string]int{}
	for _, r := range lane.Requests {
		for _, m := range r.Context {
			for _, b := range m.Blocks {
				bytesByLabel[b.Label] += b.Bytes
			}
		}
	}
	tokensByLabel := map[string]int{}
	for _, e := range AnalyzeLane(session, lane).Blame {
		tokensByLabel[e.Label] += e.Tokens.Value
	}
	for label, bytes := range bytesByLabel {
		if bytes == 0 {
			continue
		}
		if tokensByLabel[label] == 0 {
			t.Errorf("%s: %q carried %d bytes into the context and was attributed no tokens. "+
				"The report prints that row as 0.0%%, which is how 439,611 bytes of tool "+
				"definitions were shown as costing nothing.", name, label, bytes)
		}
	}
}

// A turn that re-lays the shared prefix must not enter the byte-to-token fit.
//
// The fit relates user-side content bytes to tokens so that prose can be
// sized. Tool definitions are JSON schemas and are markedly denser, so a turn
// whose write is mostly a re-laid prefix would drag the ratio and every
// estimated figure derived from it.
//
// This test exists because the guard had none. Deleting the guard outright
// makes the loop index unused, so the mutant did not compile — and a harness
// that treats a compiler refusal as a caught defect scored it as guarded twice
// over. It is not: with the mutant written so it builds, nothing in the suite
// noticed.
//
// PASS: the late-binding fixture, whose only comparable turn re-lays the
// prefix, contributes no fitted turns, and the fit says it is estimating.
// FAIL: that turn sampled, which prices every later estimate off tool JSON.
func TestReconcile_ThePrefixTurnIsNotFitted(t *testing.T) {
	session, err := ledger.ReadFile(
		filepath.Join("..", "ledger", "testdata", "late-binding-tools.jsonl"))
	if err != nil {
		t.Fatalf("the shipped ledger fixture must parse: %v", err)
	}
	lane := MainLane(session)
	if lane == nil {
		t.Fatal("no main lane")
	}
	cal := Calibrate(lane)

	// The fixture has to contain the shape, or this asserts nothing.
	var prefixTurns int
	seen := make(map[string]bool)
	markSeen(seen, lane.Requests[0])
	for _, turn := range cal.Turns {
		if turn.Outcome == cachemodel.ReadFirst {
			markSeen(seen, turn.Request)
			continue
		}
		if splitTurn(turn, seen).prefixChanged {
			prefixTurns++
		}
	}
	if prefixTurns == 0 {
		t.Fatal("the fixture no longer contains a turn that re-lays the prefix, so this test " +
			"cannot fail and is not evidence")
	}

	fit := Fit(cal, session.Source.PrefixVisible())
	if fit.Turns != 0 {
		t.Errorf("%d turn(s) were fitted, but every comparable turn here re-lays the shared "+
			"prefix. Fitting one prices prose off tool-definition JSON: the ratio came out "+
			"%.3f tokens/byte against a prose default of %.3f.",
			fit.Turns, fit.TokensPerByte, defaultTokensPerByte)
	}
	if fit.TokensPerByte != defaultTokensPerByte {
		t.Errorf("tokens/byte = %.4f, want the stated default %.4f: with nothing fitted the fit "+
			"must say so rather than report a number it did not measure",
			fit.TokensPerByte, defaultTokensPerByte)
	}
}
