package proxy

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// A break must name what actually changed, and only what actually changed.
//
// "system prompt or tool definitions changed" was the one string covering
// every prefix change, and it names a cause that mostly did not happen. Across
// the whole 30-lane trial of 2026-09-06, system_bytes never moved once. Every
// real prefix change was the tool SET changing, and all three genuine ones
// were an MCP connector finishing its handshake mid-session: WaitForMcpServers
// dropped, that connector's whole tool block appended, system_bytes identical.
//
// Telling an operator "your system prompt or your tools changed" when the
// proxy knows it was thirty-four Calendly tools arriving is the same silence
// this codebase keeps finding: a message that reports nothing actionable when
// the actionable fact is already in hand.
//
// PASS: each case names its own cause, and the detail names the tools.
// FAIL: everything is still the combined string, or the detail is empty.
func TestBreakCause_NamesWhatChanged(t *testing.T) {
	mcpBlock := []transcript.ToolDef{
		{Name: "Calendly__list_events", Bytes: 900},
		{Name: "Calendly__create_invitee", Bytes: 800},
	}

	tests := []struct {
		name       string
		prevTools  []transcript.ToolDef
		tools      []transcript.ToolDef
		prevSystem int
		system     int
		wantCause  cachemodel.BreakCause
		wantDetail []string
	}{
		{
			name:       "a connector's tool block arrives",
			prevTools:  []transcript.ToolDef{{Name: "Read", Bytes: 500}, {Name: "WaitForMcpServers", Bytes: 100}},
			tools:      append([]transcript.ToolDef{{Name: "Read", Bytes: 500}}, mcpBlock...),
			prevSystem: 29199,
			system:     29199,
			wantCause:  cachemodel.CauseToolsChanged,
			wantDetail: []string{"Calendly__list_events", "WaitForMcpServers"},
		},
		{
			name:       "only the system prompt moved",
			prevTools:  []transcript.ToolDef{{Name: "Read", Bytes: 500}},
			tools:      []transcript.ToolDef{{Name: "Read", Bytes: 500}},
			prevSystem: 29199,
			system:     31000,
			wantCause:  cachemodel.CauseSystemChanged,
			wantDetail: []string{"29199", "31000"},
		},
		{
			name:       "both moved, so the combined cause is still correct",
			prevTools:  []transcript.ToolDef{{Name: "Read", Bytes: 500}},
			tools:      append([]transcript.ToolDef{{Name: "Read", Bytes: 500}}, mcpBlock...),
			prevSystem: 29199,
			system:     31000,
			wantCause:  cachemodel.CausePrefixChange,
			wantDetail: []string{"Calendly__list_events"},
		},
		{
			name:       "a tool grew under the same name",
			prevTools:  []transcript.ToolDef{{Name: "Read", Bytes: 500}},
			tools:      []transcript.ToolDef{{Name: "Read", Bytes: 9000}},
			prevSystem: 29199,
			system:     29199,
			wantCause:  cachemodel.CauseToolsChanged,
			wantDetail: []string{"Read"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newStats()
			s.observe(prefixRecord("lane-a", 0, tt.prevSystem, tt.prevTools, 0))
			out := s.observe(prefixRecord("lane-a", 1, tt.system, tt.tools, 0))

			if out == nil || out.Outcome != "broken" {
				t.Fatalf("the second request must break for its cause to be under test; got %+v", out)
			}
			if out.Cause != tt.wantCause {
				t.Errorf("cause = %q, want %q", out.Cause, tt.wantCause)
			}
			for _, want := range tt.wantDetail {
				if !strings.Contains(out.CauseDetail, want) {
					t.Errorf("detail %q does not name %q. The proxy has the tool list in hand "+
						"and reporting a change without saying which is the silence this "+
						"change removes.", out.CauseDetail, want)
				}
			}
		})
	}
}

// The cause vocabulary must stay bounded, because it is a metrics label.
//
// state.go emits replay_cache_break_total{cause=%q}. Putting tool names in the
// cause would give Prometheus one series per distinct tool set, which is
// unbounded cardinality and the classic way to take down a metrics backend.
// The specifics belong in CauseDetail, which is logged and written to the
// ledger and never becomes a label.
//
// PASS: the cause is one of the declared constants.
// FAIL: a cause was built from request content.
func TestBreakCause_StaysABoundedVocabulary(t *testing.T) {
	known := map[cachemodel.BreakCause]bool{
		cachemodel.CauseTTLExpired:    true,
		cachemodel.CauseModelChanged:  true,
		cachemodel.CausePrefixChange:  true,
		cachemodel.CauseToolsChanged:  true,
		cachemodel.CauseSystemChanged: true,
		cachemodel.CauseEffortChange:  true,
		cachemodel.CauseHistoryEdit:   true,
		cachemodel.CauseRerendered:    true,
		cachemodel.CauseUnknown:       true,
	}

	s := newStats()
	// Every request carries a distinct tool set, so an unbounded cause would
	// produce a new value each time.
	for i := 0; i < 6; i++ {
		tools := []transcript.ToolDef{{Name: "Read", Bytes: 500 + i}}
		out := s.observe(prefixRecord("lane-a", i, 1000+i, tools, 0))
		if out == nil || out.Cause == "" {
			continue
		}
		if !known[out.Cause] {
			t.Errorf("cause %q is not one of the declared constants. It is emitted as a "+
				"Prometheus label, so a cause built from request content is unbounded "+
				"cardinality.", out.Cause)
		}
	}
}

// prefixRecord builds a request with a given system size and tool set, whose
// prefix hash follows from them.
func prefixRecord(agentID string, i, systemBytes int, tools []transcript.ToolDef, cacheRead int) *ledger.Record {
	rec := &ledger.Record{
		Timestamp: time.Date(2026, 9, 6, 13, 0, i, 0, time.UTC),
		SessionID: "sess-cause",
		AgentID:   agentID,
		Status:    200,
	}
	rec.Model = "claude-opus-5"
	rec.Prompt.SystemBytes = systemBytes
	rec.Prompt.Tools = tools
	rec.Prompt.ToolCount = len(tools)
	for _, tool := range tools {
		rec.Prompt.ToolBytes += tool.Bytes
	}
	// A hash that changes with the prefix the way the real one does: it
	// covers the tool definitions AS SENT, so a tool growing under the same
	// name changes it. The first version of this fixture hashed names only,
	// and the "a tool grew under the same name" case then silently exercised
	// nothing, because the two requests hashed identically and no prefix
	// change was ever detected.
	var b strings.Builder
	for _, tool := range tools {
		fmt.Fprintf(&b, "%s:%d;", tool.Name, tool.Bytes)
	}
	fmt.Fprintf(&b, "system=%d", systemBytes)
	rec.PrefixHash = b.String()
	rec.Response.Usage = &transcript.Usage{
		Input:         100,
		CacheCreation: 1000,
		CacheRead:     cacheRead,
		Output:        10,
	}
	return rec
}
