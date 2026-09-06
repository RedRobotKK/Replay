package ledger

import (
	"os"
	"path/filepath"
	"testing"
)

// The registry of CLI and TUI GenAI surfaces, and what is actually known about
// each one.
//
// This file exists because "Replay supports X" is the easiest false claim in
// the project to make and the hardest to notice. A parser that compiles, a stub
// that passes, and a README bullet are all indistinguishable from real support
// until somebody's numbers are wrong.
//
// So support is a STATUS with an evidence requirement attached, and the test
// below fails if a status outruns its evidence. It is deliberately impossible
// to promote a surface to Live by editing a string: Live requires a fixture on
// disk that was captured off the wire.
//
// What this file does NOT do is enumerate every GenAI CLI in existence. That
// list cannot be known, changes weekly, and a registry claiming completeness
// would be the same defect one level up. It enumerates what this build has
// touched, what it has assumed, and what it has refused, which is a claim that
// can be checked.

// SurfaceStatus is how much is actually known about a surface.
type SurfaceStatus string

const (
	// StatusLive means a payload from the real provider was captured and is in
	// testdata. The strongest claim available, and the only one that survives
	// the "a stub only sends fields the parser already knows" problem.
	StatusLive SurfaceStatus = "LIVE"

	// StatusStub means it parses a payload we wrote. Useful, and NOT evidence
	// the provider sends that shape. The OpenAI-compatible path shipped on a
	// stub and the release notes say so.
	StatusStub SurfaceStatus = "STUB"

	// StatusForwarded means the proxy passes it through, reads nothing, and
	// warns. Honest, and the correct state for anything unknown.
	StatusForwarded SurfaceStatus = "FORWARDED"

	// StatusRefused means measured and found unmeasurable. A kill, with the
	// evidence for it. Cursor is the worked example.
	StatusRefused SurfaceStatus = "REFUSED"
)

// Surface is one CLI or TUI agent, and the wire shape it speaks.
type Surface struct {
	// Client is the tool a person runs in a terminal.
	Client string
	// Wire is the request shape it sends, which is what Replay actually reads.
	// Several clients share one wire, and that is the only reason this is
	// tractable at all.
	Wire string
	// Status is what is known, not what is hoped.
	Status SurfaceStatus
	// Fixture is a path under testdata/ required when Status is LIVE, and it
	// must exist. Empty for every other status.
	Fixture string
	// Evidence names where the claim comes from, so a reader can go and check
	// rather than trust this table.
	Evidence string
	// Promote states what would raise the status. A surface with no route to
	// promotion is a surface nobody can improve, and saying so is the point.
	Promote string
}

// surfaces is the registry. Adding a row is cheap; promoting one is not.
var surfaces = []Surface{
	{
		Client: "Claude Code", Wire: "anthropic:/v1/messages",
		Status: StatusLive, Fixture: "anthropic",
		Evidence: "docs/evidence/spike-4-real-provider-2026-09-05.md: ten turns, all 200, " +
			"1,816,417 prompt tokens measured against the real provider",
		Promote: "already the strongest status available",
	},
	{
		Client: "DeepSeek CLI", Wire: "openai:/v1/chat/completions",
		Status: StatusLive, Fixture: "deepseek",
		Evidence: "captured from api.deepseek.com 2026-09-05. Two defects found that a " +
			"stub could not have shown, including prompt_cache_hit_tokens",
		Promote: "already the strongest status available",
	},
	{
		Client: "Cursor and other OpenAI-compatible CLIs", Wire: "openai:/v1/chat/completions",
		Status: StatusStub, Fixture: "",
		Evidence: "CHANGELOG 0.3.0 and docs/SURFACES.md both say verified against a stub " +
			"and never against a live OpenAI-compatible provider. Masking does not cover " +
			"this path and the proxy warns at runtime",
		Promote: "point one at replay serve on the fleet and capture a real response. " +
			"RELEASE-CRITERIA.md makes this a v1.0 gate: verified live, or labelled " +
			"EXPERIMENTAL and UNMASKED wherever it is offered",
	},
	{
		Client: "Cursor, transcript path", Wire: "cursor:sqlite",
		Status: StatusRefused, Fixture: "",
		Evidence: "docs/evidence/spike-cursor-2026-09-05.md: 29,665 message rows and ZERO " +
			"cache fields, so the transcript path could never produce cache forensics",
		Promote: "nothing to promote. This is a kill with a measurement behind it, and it " +
			"is recorded so nobody spends a week rediscovering it",
	},
	{
		Client: "Grok CLI", Wire: "openai:/responses",
		Status: StatusForwarded, Fixture: "",
		Evidence: "docs/evidence/wire-families-2026-09-06.md: captured off the wire from a " +
			"live authenticated session. It posts to /responses at " +
			"cli-chat-proxy.grok.com, not /v1/chat/completions at api.x.ai, so it was " +
			"listed on the wrong row until it was measured. It names its own " +
			"prompt_cache_key in the request body, and its x-ratelimit-remaining-* " +
			"headers did not move across 8 model calls and ~940KB of responses",
		Promote: "parse /responses and land a captured fixture. Note that the quota " +
			"headers are not a route to a live guard on this surface: no live quota " +
			"state was found on any endpoint, /settings included, so a guard here has " +
			"to be built from what Replay counts itself",
	},
	{
		Client: "Anything else with a terminal", Wire: "unknown",
		Status: StatusForwarded, Fixture: "",
		Evidence: "docs/SURFACES.md: any other POST path is forwarded unchanged and NOT " +
			"read. No ledger record, no guard, no masking, and the proxy warns once per " +
			"path with replay_unparsed_requests_total",
		Promote: "capture a payload, add a fixture, write the surface's own condition in " +
			"provider_conformance_test.go. Forwarded is the honest default and costs a " +
			"user nothing but the measurement",
	},
}

// A LIVE status must have a fixture that exists.
//
// This is the check that makes the registry more than a comment. Promoting a
// surface by editing a string fails here unless a captured payload landed with
// it, which is the exact discipline provider_conformance_test.go was built on:
// a stub only ever sends the fields the parser already knows about.
//
// PASS: every LIVE surface has a real fixture directory.
// FAIL: a support claim outran its evidence.
func TestSurfaceRegistry_LiveRequiresACapturedFixture(t *testing.T) {
	for _, s := range surfaces {
		if s.Status != StatusLive {
			if s.Fixture != "" {
				t.Errorf("%s is %s but names fixture %q. A fixture on a non-live status "+
					"reads as evidence that is not being claimed", s.Client, s.Status, s.Fixture)
			}
			continue
		}
		if s.Fixture == "" {
			t.Errorf("%s claims LIVE with no fixture. LIVE means a payload from the real "+
				"provider is on disk; without one the claim rests on somebody's memory",
				s.Client)
			continue
		}
		dir := filepath.Join("testdata", s.Fixture)
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Errorf("%s claims LIVE with fixture %q, which does not exist: %v",
				s.Client, s.Fixture, err)
			continue
		}
		if len(entries) == 0 {
			t.Errorf("%s claims LIVE with fixture %q, which is empty", s.Client, s.Fixture)
		}
	}
}

// Every surface must say what its claim rests on and how it could improve.
//
// A registry row with no evidence is an assertion, and one with no promotion
// path is a dead end nobody can act on. Both are how a table like this decays
// into decoration.
//
// PASS: every row carries both.
// FAIL: a row was added without them.
func TestSurfaceRegistry_EveryRowIsCheckableAndActionable(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range surfaces {
		if s.Client == "" || s.Wire == "" {
			t.Errorf("a surface row is missing its client or wire: %+v", s)
		}
		if len(s.Evidence) < 40 {
			t.Errorf("%s has evidence %q, which is too short to point anywhere. Name the "+
				"file or the measurement", s.Client, s.Evidence)
		}
		if len(s.Promote) < 20 {
			t.Errorf("%s has no promotion path. A surface nobody can improve should say so "+
				"in words", s.Client)
		}
		key := s.Client + "|" + s.Wire
		if seen[key] {
			t.Errorf("duplicate registry row for %s", key)
		}
		seen[key] = true
	}
}

// The status vocabulary stays closed.
//
// Four statuses, each with a different evidential weight. A fifth invented at
// the call site ("PARTIAL", "MOSTLY") is how a bounded scale becomes a mood.
//
// PASS: every row uses a declared constant.
// FAIL: a status was invented.
func TestSurfaceRegistry_StatusVocabularyIsClosed(t *testing.T) {
	known := map[SurfaceStatus]bool{
		StatusLive: true, StatusStub: true, StatusForwarded: true, StatusRefused: true,
	}
	for _, s := range surfaces {
		if !known[s.Status] {
			t.Errorf("%s has status %q, which is not one of the four declared. Each status "+
				"carries a different evidential weight; a new one dilutes all of them",
				s.Client, s.Status)
		}
	}
}

// At least one surface must be REFUSED, and at least one FORWARDED.
//
// Not a formality. A registry where everything is supported is a registry that
// has stopped being read as a measurement and started being read as marketing.
// Cursor's transcript path is a kill with 29,665 rows behind it, and unknown
// paths are forwarded rather than guessed at. Both belong in the table
// precisely because they are not wins.
//
// PASS: the table records what does not work.
// FAIL: the honest entries were quietly dropped.
func TestSurfaceRegistry_RecordsWhatDoesNotWork(t *testing.T) {
	var refused, forwarded int
	for _, s := range surfaces {
		switch s.Status {
		case StatusRefused:
			refused++
		case StatusForwarded:
			forwarded++
		}
	}
	if refused == 0 {
		t.Error("no REFUSED surface. Cursor's transcript path was measured and killed; a " +
			"table that lost it is a table that only remembers its wins")
	}
	if forwarded == 0 {
		t.Error("no FORWARDED surface. Unknown paths are forwarded and warned about rather " +
			"than parsed on a guess, and that default is the honest one")
	}
}
