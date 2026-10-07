package transcript

import (
	"encoding/json"
	"strings"
	"testing"
)

// twoVersionLines builds a minimal transcript whose lines carry two distinct
// client versions, oldest first, exactly as a real transcript does when the
// client was updated mid-session.
func twoVersionLines(v1, v2 string) string {
	line := func(uuid, parent, version, req string) string {
		p := "null"
		if parent != "" {
			p = `"` + parent + `"`
		}
		return `{"type":"assistant","uuid":"` + uuid + `","parentUuid":` + p + `,"requestId":"` + req +
			`","apiBlockIndex":0,"timestamp":"2026-09-02T00:00:0` + "1" + `Z","version":"` + version +
			`","message":{"role":"assistant","model":"m","content":[{"type":"text","text":"ok"}],` +
			`"usage":{"input_tokens":1,"cache_creation_input_tokens":2,"cache_read_input_tokens":3,"output_tokens":4}}}` + "\n"
	}
	return line("a1", "", v1, "r1") + line("a2", "a1", v2, "r2")
}

// Before this change, Session.ClientVersion held only the first version seen
// (internal/transcript/claudecode.go: `if l.Version != "" && session.ClientVersion
// == "" { session.ClientVersion = l.Version }`). A session whose transcript shows
// 2.1.291 and then 2.1.292 cannot be represented as a single first-seen version:
// the transition itself is the fact, and collapsing it to "2.1.291" erases it.
func TestClientVersionStraddlingSessionRecordsBothVersions(t *testing.T) {
	s, err := ParseClaudeCode(strings.NewReader(twoVersionLines("2.1.291", "2.1.292")))
	if err != nil {
		t.Fatal(err)
	}
	if got := s.ClientVersions; len(got) != 2 || got[0] != "2.1.291" || got[1] != "2.1.292" {
		t.Fatalf("ClientVersions = %v, want [2.1.291 2.1.292] in first-seen order", got)
	}
	if prov := s.ClientVersionProvenance(); prov != ClientVersionStraddling {
		t.Fatalf("ClientVersionProvenance() = %q, want %q", prov, ClientVersionStraddling)
	}
}

func TestClientVersionSingleVersionProvenance(t *testing.T) {
	s, err := ParseClaudeCode(strings.NewReader(twoVersionLines("2.1.291", "2.1.291")))
	if err != nil {
		t.Fatal(err)
	}
	if got := s.ClientVersions; len(got) != 1 || got[0] != "2.1.291" {
		t.Fatalf("ClientVersions = %v, want [2.1.291] (deduplicated)", got)
	}
	if prov := s.ClientVersionProvenance(); prov != ClientVersionSingle {
		t.Fatalf("ClientVersionProvenance() = %q, want %q", prov, ClientVersionSingle)
	}
	// The legacy first-seen field stays in step with the single version for
	// backward compatibility with existing consumers (cmd/replay/corpus.go).
	if s.ClientVersion != "2.1.291" {
		t.Fatalf("ClientVersion = %q, want 2.1.291", s.ClientVersion)
	}
}

func TestClientVersionUnmeasuredWhenNoVersionField(t *testing.T) {
	input := `{"type":"assistant","uuid":"a1","parentUuid":null,"requestId":"r1","apiBlockIndex":0,` +
		`"timestamp":"2026-09-02T00:00:01Z","message":{"role":"assistant","model":"m","content":[{"type":"text","text":"ok"}],` +
		`"usage":{"input_tokens":1,"cache_creation_input_tokens":2,"cache_read_input_tokens":3,"output_tokens":4}}}` + "\n"
	s, err := ParseClaudeCode(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if len(s.ClientVersions) != 0 {
		t.Fatalf("ClientVersions = %v, want empty", s.ClientVersions)
	}
	if prov := s.ClientVersionProvenance(); prov != ClientVersionUnmeasured {
		t.Fatalf("ClientVersionProvenance() = %q, want %q", prov, ClientVersionUnmeasured)
	}
	// Unmeasured must not collide with "single" or the straddling marker: it
	// is a third state, not either of the other two spelled emptily.
	if ClientVersionUnmeasured == ClientVersionSingle || ClientVersionUnmeasured == ClientVersionStraddling {
		t.Fatal("ClientVersionUnmeasured must be distinct from the other two provenance states")
	}
}

// Ordering and uniqueness: a version seen, left, and seen again stays a
// single entry at its first position, never appended a second time.
func TestClientVersionOrderingAndUniqueness(t *testing.T) {
	line := func(uuid, parent, version, req string) string {
		p := "null"
		if parent != "" {
			p = `"` + parent + `"`
		}
		return `{"type":"assistant","uuid":"` + uuid + `","parentUuid":` + p + `,"requestId":"` + req +
			`","apiBlockIndex":0,"timestamp":"2026-09-02T00:00:01Z","version":"` + version +
			`","message":{"role":"assistant","model":"m","content":[{"type":"text","text":"ok"}],` +
			`"usage":{"input_tokens":1,"cache_creation_input_tokens":2,"cache_read_input_tokens":3,"output_tokens":4}}}` + "\n"
	}
	input := line("a1", "", "2.1.291", "r1") + line("a2", "a1", "2.1.292", "r2") + line("a3", "a2", "2.1.291", "r3")
	s, err := ParseClaudeCode(strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	got := s.ClientVersions
	if len(got) != 2 || got[0] != "2.1.291" || got[1] != "2.1.292" {
		t.Fatalf("ClientVersions = %v, want [2.1.291 2.1.292] (first-seen order, deduplicated)", got)
	}
}

// Session is not itself serialized anywhere in this codebase today (only
// Block, Usage and ToolDef carry json tags, because those are what the ledger
// persists), but ClientVersions is still given json tags for future-proofing,
// and this proves the tag round-trips correctly.
func TestClientVersionsSurvivesJSONRoundTrip(t *testing.T) {
	s, err := ParseClaudeCode(strings.NewReader(twoVersionLines("2.1.278", "2.1.291")))
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(s.ClientVersions)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "2.1.278" || got[1] != "2.1.291" {
		t.Fatalf("round trip = %v, want [2.1.278 2.1.291]", got)
	}
}
