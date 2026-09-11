package proxy

import (
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// The seeded-blame trial: does Replay name the event that actually happened?
//
// Every evidence file in this repository measures a property of the tool. Not
// one measures whether the tool's central claim is true. The claim is that
// Replay names the event that broke your prefix, and until this ran, nothing
// had ever checked it against a break whose cause was known in advance.
//
// The method is to seed the event, not the symptom. Setting cache_read to 0
// and then asserting the classifier says "prefix changed" would be testing
// that a equals a — the classifier's own rule, restated. So each case below
// describes a thing that happened to a session (a model swap, an MCP
// connector finishing its handshake, a lunch break) and lets the usage and the
// prefix follow from it. The expectation is sealed in the case, written before
// the run.
//
// REFUTED IF blame names a different event than the one seeded.

// seed is one event, with what the wire would carry after it.
type seed struct {
	name string
	// what happened
	gap                time.Duration
	prevModel, model   string
	prevTools, tools   []transcript.ToolDef
	prevSystem, system int
	// what should be said about it, decided before the run
	want cachemodel.BreakCause
	// why this case exists
	note string
}

func tool(name string, bytes int) transcript.ToolDef {
	return transcript.ToolDef{Name: name, Bytes: bytes}
}

// The tool set a Claude Code session carries before anything changes.
func baseTools() []transcript.ToolDef {
	return []transcript.ToolDef{
		tool("Bash", 1200), tool("Read", 800), tool("Edit", 950),
		tool("Glob", 400), tool("Grep", 600),
	}
}

func TestSeededBlame_TheNamedCauseIsTheSeededEvent(t *testing.T) {
	const sys = 4200

	seeds := []seed{
		{
			name: "a lunch break", gap: 20 * time.Minute,
			prevModel: "claude-opus-4", model: "claude-opus-4",
			prevTools: baseTools(), tools: baseTools(),
			prevSystem: sys, system: sys,
			want: cachemodel.CauseTTLExpired,
			note: "nothing was edited; the cache simply aged out",
		},
		{
			name: "the operator switched model mid-session", gap: 30 * time.Second,
			prevModel: "claude-opus-4", model: "claude-sonnet-4",
			prevTools: baseTools(), tools: baseTools(),
			prevSystem: sys, system: sys,
			want: cachemodel.CauseModelChanged,
			note: "a different model cannot read the other's cache",
		},
		{
			name: "an MCP connector finished its handshake", gap: 30 * time.Second,
			prevModel: "claude-opus-4", model: "claude-opus-4",
			prevTools: baseTools(),
			tools: append(baseTools(),
				tool("mcp__notion__search", 1400), tool("mcp__notion__page", 1100)),
			prevSystem: sys, system: sys,
			want: cachemodel.CauseToolsChanged,
			note: "the shape the 30-lane trial found in every real prefix change",
		},
		{
			name: "a connector dropped out", gap: 30 * time.Second,
			prevModel: "claude-opus-4", model: "claude-opus-4",
			prevTools:  append(baseTools(), tool("mcp__notion__search", 1400)),
			tools:      baseTools(),
			prevSystem: sys, system: sys,
			want: cachemodel.CauseToolsChanged,
			note: "removal is the same class of event as addition",
		},
		{
			name: "a tool's schema grew", gap: 30 * time.Second,
			prevModel: "claude-opus-4", model: "claude-opus-4",
			prevTools: baseTools(),
			tools: []transcript.ToolDef{
				tool("Bash", 1200), tool("Read", 800), tool("Edit", 1350),
				tool("Glob", 400), tool("Grep", 600),
			},
			prevSystem: sys, system: sys,
			want: cachemodel.CauseToolsChanged,
			note: "same names, one different size — a version bump, not a new tool",
		},
		{
			name: "the project instructions were edited", gap: 30 * time.Second,
			prevModel: "claude-opus-4", model: "claude-opus-4",
			prevTools: baseTools(), tools: baseTools(),
			prevSystem: sys, system: sys + 180,
			want: cachemodel.CauseSystemChanged,
			note: "CLAUDE.md gained a paragraph",
		},
		{
			name: "an edit that preserved its length", gap: 30 * time.Second,
			prevModel: "claude-opus-4", model: "claude-opus-4",
			prevTools: baseTools(), tools: baseTools(),
			prevSystem: sys, system: sys,
			want: cachemodel.CausePrefixChange,
			note: "the known limit, and the tool's response to it is right. " +
				"systemChanged compares byte counts, so swapping 2026-09-10 for " +
				"2026-09-11 in CLAUDE.md moves the prefix and moves no count. " +
				"Neither half accounts for the break, and the tool widens to the " +
				"cause that covers both rather than naming one at random",
		},
		{
			name: "the model changed after a long gap", gap: 20 * time.Minute,
			prevModel: "claude-opus-4", model: "claude-sonnet-4",
			prevTools: baseTools(), tools: baseTools(),
			prevSystem: sys, system: sys,
			want: cachemodel.CauseTTLExpired,
			note: "both are true and only one is the cause. The cache was gone " +
				"before the model changed, so the swap cost nothing; blaming it " +
				"would send a reader to pin a model that was never the problem",
		},
		{
			name: "the model changed and so did the tools", gap: 30 * time.Second,
			prevModel: "claude-opus-4", model: "claude-sonnet-4",
			prevTools: baseTools(), tools: append(baseTools(), tool("mcp__x__y", 900)),
			prevSystem: sys, system: sys,
			want: cachemodel.CauseModelChanged,
			note: "a model swap invalidates on its own, so the tool change is " +
				"downstream of a break that had already happened",
		},
		{
			name: "both moved at once", gap: 30 * time.Second,
			prevModel: "claude-opus-4", model: "claude-opus-4",
			prevTools: baseTools(), tools: append(baseTools(), tool("mcp__x__y", 900)),
			prevSystem: sys, system: sys + 180,
			want: cachemodel.CausePrefixChange,
			note: "neither half alone explains it, and naming one would be a guess",
		},
	}

	for _, s := range seeds {
		t.Run(s.name, func(t *testing.T) {
			prev := transcript.Usage{CacheCreation: 12000, Create5m: 12000}
			// The break itself: nothing was read back. That is the consequence
			// of every event above, not an input chosen to steer the answer.
			cur := transcript.Usage{CacheCreation: 12000, Create5m: 12000, CacheRead: 0}

			got, decided := cachemodel.ClassifyBreak(prev, cur, s.prevModel, s.model, s.gap)
			if !decided {
				t.Fatalf("usage and timing settled nothing for %q, which they must for a "+
					"break with no read at all", s.name)
			}
			// Where usage says only "the prefix moved", the request content
			// decides which half.
			if got == cachemodel.CausePrefixChange {
				got = diffPrefix(s.prevTools, s.tools, s.prevSystem, s.system).cause()
			}

			if got != s.want {
				t.Errorf("seeded %q (%s)\n  blamed: %s\n  actual: %s",
					s.name, s.note, got, s.want)
			}
		})
	}
}
