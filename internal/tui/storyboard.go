package tui

import (
	"fmt"
	"sort"
	"strings"
)

// The storyboard: every state the live surface has to survive, rendered by the
// same formatter that will render the real thing.
//
// Hand-drawn frames are how a design ships a defect. The first storyboard of
// this surface stated as an invariant that its borders could never shear, and
// its own data rows measured 78 cells against a 79-cell header. Generating the
// frames from Row means a state cannot be drawn that the formatter would not
// produce.

// Width is the frame budget. Eighty columns, because that is the narrowest
// terminal anyone still defends and every wider one is a superset.
const Width = 80

// trafficCols is the request log. Sized so the longest real value fits.
//
// The endpoint column is 23 because cli-chat-proxy.grok.com is that wide; the
// wire column is 16 because chat/completions is. Those two belong to different
// surfaces: Grok posts to /responses at that origin, and is not on the
// chat-completions family at all.
var trafficCols = []Column{
	{"time", 8}, {"surface", 9}, {"endpoint", 23}, {"wire", 16}, {"status", 9},
}

// Scene is one state, named as the design names it.
type Scene struct {
	N     int
	Name  string
	Lines []string
}

// Header draws the traffic table's heading and rule.
func Header() []string {
	names := make([]string, 0, len(trafficCols))
	rule := make([]string, 0, len(trafficCols))
	for _, c := range trafficCols {
		names = append(names, c.Name)
		rule = append(rule, strings.Repeat("-", c.Width))
	}
	return []string{Row(trafficCols, names...), Row(trafficCols, rule...)}
}

// Traffic renders one request row.
func Traffic(t, surface, endpoint, wire, status string) string {
	return Row(trafficCols, t, surface, endpoint, wire, status)
}

// Empty renders a held-open slot, so the grid does not resize under load.
func Empty() string { return Row(trafficCols, "", "", "", "", "") }

// note renders a line under the table. "!" is something the operator must act
// on; "-" is something they should know.
func note(urgent bool, s string) string {
	if urgent {
		return "  ! " + s
	}
	return "  - " + s
}

// kv renders a label and value at a fixed column, for the header block and the
// settings screen, where the label column is the thing a reader scans.
func kv(label, value string) string {
	return "  " + cell(label, 24) + value
}

// Storyboard returns every scene, in lifecycle order.
func Storyboard() []Scene {
	hdr := Header()
	var s []Scene

	add := func(n int, name string, lines ...string) {
		s = append(s, Scene{N: n, Name: name, Lines: lines})
	}

	title := func(sub string) []string {
		return []string{"  replay serve" + strings.Repeat(" ", 48) + "v0.4.0", "", sub}
	}
	_ = title

	add(1, "Idle, listening",
		append([]string{
			"  replay serve                                                    v0.4.0", "",
			kv("listening", "127.0.0.1:4000"),
			kv("upstream", "api.anthropic.com"),
			kv("ledger", "~/.replay/ledger            (writable)"),
			"", "  traffic",
		}, append(hdr,
			Empty(), Empty(), Empty(), "",
			"  no requests yet. point a client at the listen address above.", "",
			"  [s] settings   [q] quit")...)...)

	add(2, "Active traffic, mixed surfaces",
		append([]string{
			"  replay serve                                                    v0.4.0", "",
			kv("listening", "127.0.0.1:4000        sessions  3"),
			kv("upstream", "api.anthropic.com     billed    1,204,881 tokens"),
			kv("ledger", "~/.replay/ledger      spend     $2.41 of $5.00 cap"),
			"", "  traffic",
		}, append(hdr,
			Traffic("15:06:44", "anthropic", "api.anthropic.com", "messages", "parsed"),
			Traffic("15:06:41", "openai", "api.openai.com", "chat/completions", "stub"),
			Traffic("15:05:58", "grok", "cli-chat-proxy.grok.com", "/responses", "forwarded"),
			Traffic("15:05:44", "anthropic", "api.anthropic.com", "messages", "parsed"),
			"", "  notes",
			note(true, "the day dollar cap is NOT being enforced: 12 requests could not"),
			"      be priced, so they add nothing to the total and it cannot be reached.",
			note(false, "grok /responses is forwarded unread: no ledger record, no cap,"),
			"      no masking and no loop detection apply to it.",
			note(false, "openai is parsed against a stub, never verified live."),
			"", "  [s] settings   [q] quit")...)...)

	add(6, "A path forwarded blind",
		append([]string{"  traffic"}, append(hdr,
			Traffic("15:05:58", "grok", "cli-chat-proxy.grok.com", "/responses", "forwarded"),
			"", "  notes",
			note(false, "NOT PARSED. Replay forwards this path unchanged and reads none"),
			"      of it. You are getting bytes moved, and nothing measured.")...)...)

	add(15, "A cap configured but not enforceable",
		"  spend", "",
		kv("day cap", "$5.00"),
		kv("counted so far", "$2.41"),
		kv("could not be priced", "12 requests"),
		"", "  notes",
		note(true, "this cap cannot be reached. Unpriced requests add nothing to"),
		"      the total, so the limit you set is not being applied to them.",
		note(false, "priced traffic is still capped normally."),
	)

	add(16, "Settings, with provenance",
		"  replay settings                                                 v0.4.0", "",
		kv("caps", "value            from"),
		kv("max-session-tokens", "unset            default"),
		kv("max-day-tokens", "unset            default"),
		kv("max-day-usd", "5.00             flag"),
		kv("error-budget", "0.15             default"),
		"", kv("consent", "state            checked"),
		kv("corpus contribution", "refused          yes"),
		kv("update checks", "undecided        never asked"),
		"", kv("masking", "on"),
		kv("  patterns", "14"),
		kv("  NOT covered", "/responses, /v1/chat/completions"),
		"", "  [esc] back",
	)

	add(22, "Ownership not verified (Windows)",
		"  consent", "",
		kv("corpus contribution", "granted"),
		kv("ownership checked", "no"),
		"", "  notes",
		note(true, "this platform has no Unix permission bits, so Replay cannot"),
		"      confirm the file is yours. The decision was read, not verified.",
	)

	add(24, "Not a TTY",
		"  (piped or redirected: one line per event, no repaint, no escapes)", "",
		"  15:06:44 anthropic api.anthropic.com messages parsed 1,204 tokens",
		"  15:05:58 grok cli-chat-proxy.grok.com /responses forwarded not-parsed",
		"  15:05:44 anthropic api.anthropic.com messages parsed 880 tokens",
	)

	add(25, "Terminal narrower than 80",
		"  (columns dropped in a fixed order: wire, endpoint, surface)", "",
		Row([]Column{{"time", 8}, {"surface", 9}, {"status", 9}},
			"time", "surface", "status"),
		Row([]Column{{"time", 8}, {"surface", 9}, {"status", 9}},
			"--------", "---------", "---------"),
		Row([]Column{{"time", 8}, {"surface", 9}, {"status", 9}},
			"15:06:44", "anthropic", "parsed"),
		Row([]Column{{"time", 8}, {"surface", 9}, {"status", 9}},
			"15:05:58", "grok", "forwarded"),
	)

	sort.Slice(s, func(i, j int) bool { return s[i].N < s[j].N })
	return s
}

// Render writes the whole storyboard as text.
func Render() string {
	var b strings.Builder
	for _, sc := range Storyboard() {
		fmt.Fprintf(&b, "State %d: %s\n", sc.N, sc.Name)
		fmt.Fprintf(&b, "%s\n", strings.Repeat("=", Width))
		for _, l := range sc.Lines {
			b.WriteString(l + "\n")
		}
		b.WriteString("\n")
	}
	return b.String()
}
