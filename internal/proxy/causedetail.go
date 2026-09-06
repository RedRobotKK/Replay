package proxy

import (
	"fmt"
	"sort"
	"strings"

	"github.com/RedRobotKK/Replay/internal/cachemodel"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// Naming what changed in a prefix.
//
// The proxy has always known this and never said it. A prefix change was
// reported as "system prompt or tool definitions changed" whatever had
// actually happened, and the request carries the tool list, so the specific
// answer was in hand and thrown away.
//
// It is also the wrong half of the time. Across the whole 30-lane trial of
// 2026-09-06, system_bytes never moved once: every real prefix change was the
// tool SET changing, and the three genuine ones were an MCP connector
// finishing its handshake, which drops WaitForMcpServers and appends that
// connector's whole block.
//
// The cause stays a bounded vocabulary because state.go emits it as
// replay_cache_break_total{cause=...} and tool names in a Prometheus label are
// unbounded cardinality. The specifics go in CauseDetail, which is logged and
// written to the ledger and is never a label.

// maxNamedTools bounds how many tool names a detail line prints before it
// summarises. A connector can bring thirty-four at once, and a log line that
// long is not read.
const maxNamedTools = 6

// prefixDelta describes how one request's prefix differs from the lane's
// previous one.
type prefixDelta struct {
	added, removed, resized []string
	systemBefore, systemNow int
}

func (d prefixDelta) toolsChanged() bool {
	return len(d.added) > 0 || len(d.removed) > 0 || len(d.resized) > 0
}

func (d prefixDelta) systemChanged() bool { return d.systemBefore != d.systemNow }

// cause picks the narrowest cause the delta supports.
func (d prefixDelta) cause() cachemodel.BreakCause {
	switch {
	case d.toolsChanged() && d.systemChanged():
		return cachemodel.CausePrefixChange
	case d.toolsChanged():
		return cachemodel.CauseToolsChanged
	case d.systemChanged():
		return cachemodel.CauseSystemChanged
	default:
		// The hash differs and neither half accounts for it, which means the
		// prefix carries something this build does not summarise. Saying so
		// beats naming a half at random.
		return cachemodel.CausePrefixChange
	}
}

// detail renders the delta for a person. Empty when nothing is known, so a
// caller never prints an empty parenthetical.
func (d prefixDelta) detail() string {
	var parts []string
	if len(d.added) > 0 {
		parts = append(parts, "added "+nameList(d.added))
	}
	if len(d.removed) > 0 {
		parts = append(parts, "removed "+nameList(d.removed))
	}
	if len(d.resized) > 0 {
		parts = append(parts, "resized "+nameList(d.resized))
	}
	if d.systemChanged() {
		parts = append(parts, fmt.Sprintf("system prompt %d to %d bytes", d.systemBefore, d.systemNow))
	}
	return strings.Join(parts, "; ")
}

// nameList prints up to maxNamedTools names and counts the rest.
func nameList(names []string) string {
	if len(names) <= maxNamedTools {
		return fmt.Sprintf("%d tool(s): %s", len(names), strings.Join(names, ", "))
	}
	return fmt.Sprintf("%d tool(s): %s and %d more",
		len(names), strings.Join(names[:maxNamedTools], ", "), len(names)-maxNamedTools)
}

// diffPrefix compares two tool sets and system sizes.
//
// Sorted output, so the same change reads the same way twice. A tool present
// in both under the same name but a different size is "resized" rather than
// added and removed, because that is a different thing to go and look at.
func diffPrefix(prevTools, tools []transcript.ToolDef, prevSystem, system int) prefixDelta {
	before := make(map[string]int, len(prevTools))
	for _, tool := range prevTools {
		before[tool.Name] = tool.Bytes
	}
	now := make(map[string]int, len(tools))
	for _, tool := range tools {
		now[tool.Name] = tool.Bytes
	}

	d := prefixDelta{systemBefore: prevSystem, systemNow: system}
	for name, size := range now {
		switch prevSize, ok := before[name]; {
		case !ok:
			d.added = append(d.added, name)
		case prevSize != size:
			d.resized = append(d.resized, name)
		}
	}
	for name := range before {
		if _, ok := now[name]; !ok {
			d.removed = append(d.removed, name)
		}
	}
	sort.Strings(d.added)
	sort.Strings(d.removed)
	sort.Strings(d.resized)
	return d
}

// laneName renders an AgentID for a person. The main loop's empty ID would
// otherwise print as nothing at all in a log line, which reads as a missing
// field rather than as the main loop.
func laneName(agentID string) string {
	if agentID == "" {
		return "main"
	}
	return short(agentID)
}
