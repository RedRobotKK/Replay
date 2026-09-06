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
	return "  " + cell(label, labelW) + value
}

// The header block is a table too, and it has to be the same table in every
// state. State 1 put its third column at 54 and state 2 at 48, because both
// were assembled from hand-counted spaces inside a value string. A reader
// moving between screens re-finds the column each time, which is the cost the
// traffic grid already avoids and this block was not.
const (
	labelW = 24 // label column: "corpus contribution" is the longest
	valueW = 22 // value column: "api.anthropic.com" plus room
	statW  = 10 // stat label: "requests", "sessions", "billed"
)

// stat renders a header line with the same geometry in every state: a label, a
// value, a second label, and its figure.
func stat(label, value, statLabel, statValue string) string {
	return "  " + cell(label, labelW) + cell(value, valueW) +
		cell(statLabel, statW) + statValue
}

// note2 renders a header line that has no stat column, keeping the value
// column where every other state puts it.
func note2(label, value, trailing string) string {
	if trailing == "" {
		return strings.TrimRight("  "+cell(label, labelW)+value, " ")
	}
	return "  " + cell(label, labelW) + cell(value, valueW) + trailing
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
			note2("listening", "127.0.0.1:4000", ""),
			note2("upstream", "api.anthropic.com", ""),
			note2("ledger", "~/.replay/ledger", "(writable)"),
			"", "  traffic",
		}, append(hdr,
			Empty(), Empty(), Empty(), "",
			"  no requests yet. point a client at the listen address above.", "",
			Footer(0))...)...)

	add(2, "Active traffic, mixed surfaces",
		append([]string{
			"  replay serve                                                    v0.4.0", "",
			stat("listening", "127.0.0.1:4000", "sessions", "3"),
			stat("upstream", "api.anthropic.com", "billed", "1,204,881 tokens"),
			stat("ledger", "~/.replay/ledger", "spend", "$2.41 of $5.00 cap"),
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
			"", Footer(0))...)...)

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
		"", Footer(0),
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

	add(3, "Shutting down",
		[]string{
			"  replay serve                                                    v0.4.0", "",
			"  session totals", "",
			stat("requests", "412", "billed", "8,204,551 tokens"),
			stat("re-billed", "336,060", "share", "4.1% of prompt tokens"),
			stat("forwarded", "38", "read", "nothing"),
			"", "  notes",
			note(false, "ledger written to ~/.replay/ledger"),
			note(false, "38 requests on /responses were forwarded and not recorded."),
		}...)

	add(5, "Parsed against a stub, never verified live",
		append([]string{"  traffic"}, append(hdr,
			Traffic("15:06:41", "openai", "api.openai.com", "chat/completions", "stub"),
			"", "  notes",
			note(false, "this path parses a payload we wrote and has never been checked"),
			"      against a live provider. Figures from it carry that caveat.")...)...)

	add(9, "Spend cap reached",
		[]string{
			"  spend", "",
			stat("day cap", "$5.00", "reached", "15:41:07"),
			stat("counted", "$5.00", "requests", "204"),
			"", "  notes",
			note(true, "refusing before the next request. Nothing was interrupted"),
			"      mid-stream, and nothing already in flight was cut short.",
			note(false, "override with x-replay-override, and the reason is logged."),
		}...)

	add(11, "The same tool call, over and over",
		[]string{
			"  loop", "",
			stat("tool", "Read", "repeats", "7 in a row"),
			stat("argument", "src/main.go", "cost", "412,880 tokens"),
			"", "  notes",
			note(true, "loop: the same Read call was just made 7 times in a row."),
			note(false, "warn at 5, block at 12. Both are flags, both are off by default."),
		}...)

	add(13, "Upstream failing, breaker open",
		[]string{
			"  breaker", "",
			stat("state", "open", "since", "15:52:11"),
			stat("failures", "3 of 3", "cooldown", "47s remaining"),
			"", "  notes",
			note(true, "holding requests rather than passing them to a failing provider."),
			note(false, "one request will be let through when the cooldown ends."),
		}...)

	add(17, "Never asked",
		[]string{
			"  consent", "",
			stat("corpus contribution", "undecided", "asked", "never"),
			"", "  notes",
			note(false, "absence is not refusal. Replay has not put the question to you,"),
			"      and will not send anything until it does and you say yes.",
		}...)

	add(19, "Refused, and remembered",
		[]string{
			"  consent", "",
			stat("corpus contribution", "refused", "recorded", "2026-09-04"),
			"", "  notes",
			note(false, "a refusal is kept so nothing asks again. Delete the file to"),
			"      return to undecided.",
		}...)

	add(20, "The answer cannot be trusted",
		[]string{
			"  consent", "",
			stat("corpus contribution", "unreadable", "mode", "0666"),
			"", "  notes",
			note(true, "this file is writable by anyone on the machine, so it is not"),
			"      evidence of your decision. Nothing will be sent.",
			note(false, "chmod 600 the file, or answer again."),
		}...)

	add(23, "What masking does not cover",
		[]string{
			"  masking                 on", "",
			stat("patterns", "14", "entropy", "4.20"),
			stat("covered", "/v1/messages", "records", "412"),
			"", "  notes",
			note(true, "NOT covered: /responses, /v1/chat/completions."),
			"      Secrets on those paths are forwarded unmasked and unrecorded.",
		}...)

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
