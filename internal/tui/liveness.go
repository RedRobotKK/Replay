package tui

import "strings"

// A screen that never changes is indistinguishable from a hung process.
//
// That is the same defect this project keeps finding in its own checks: a
// signal that cannot vary carries no information. A user staring at a proxy
// dashboard has exactly one question when nothing is happening, and it is not
// "what are the numbers", it is "is this thing still running". A still frame
// answers it wrongly and confidently.
//
// So every live screen carries at least one element that advances on the
// repaint tick, whether or not any data arrived. TestALiveScreenNeverLooksFrozen
// holds that mechanically rather than as a convention.

// pinwheel is the rotating cue for a field that is waiting on data.
//
// Four ASCII characters, no braille or box-drawing spinner, for the same reason
// the rest of the frame is ASCII: those are East Asian Ambiguous and would
// change width mid-rotation, which is a spinner that moves the column it sits in.
var pinwheel = [...]string{"|", "/", "-", "\\"}

// Pinwheel returns the cue for a given repaint tick.
func Pinwheel(tick int) string {
	return pinwheel[((tick%len(pinwheel))+len(pinwheel))%len(pinwheel)]
}

// heartbeat is the always-moving element for a screen with nothing to report.
// A dot that walks a fixed track, so the line never changes width.
const heartbeatTrack = 6

// Heartbeat renders liveness for an idle screen: a marker moving along a track
// of fixed width, so an operator can tell "waiting" from "dead" without reading
// a number.
func Heartbeat(tick int) string {
	pos := ((tick % heartbeatTrack) + heartbeatTrack) % heartbeatTrack
	var b strings.Builder
	b.WriteByte('[')
	for i := 0; i < heartbeatTrack; i++ {
		if i == pos {
			b.WriteByte('=')
		} else {
			b.WriteByte(' ')
		}
	}
	b.WriteByte(']')
	return b.String()
}

// Awaiting renders a field that has no value yet, with the cue inside the frame
// rather than beside it, so the waiting is attached to the thing being waited on.
//
// The value column keeps its width whatever the cue is showing, because a field
// that reflows as it spins is a field that draws the eye for the wrong reason.
func Awaiting(label string, tick int) string {
	return "  " + cell(label, labelW) + cell(Pinwheel(tick)+" waiting", valueW)
}

// LiveRow is a traffic row for a request that has been accepted and has not
// finished, so the cost columns are not knowable yet.
func LiveRow(t, surface, endpoint, wire string, tick int) string {
	return Row(trafficCols, t, surface, endpoint, wire, Pinwheel(tick)+" open")
}

// LiveScene is a screen that redraws on a tick, as opposed to a settings page
// that only changes when the user does something.
//
// The distinction is deliberate. A settings screen that animated would be
// noise; a monitor that does not is a screen you cannot trust.
type LiveScene struct {
	Name string
	// Render draws the scene at a given repaint tick. The same tick must give
	// the same frame, and consecutive ticks must not.
	Render func(tick int) []string
}

// LiveScenes are the screens the liveness rule applies to.
func LiveScenes() []LiveScene {
	return []LiveScene{
		{
			Name: "idle, listening",
			Render: func(tick int) []string {
				return append([]string{
					"  replay serve                                                    v0.4.0", "",
					stat("listening", "127.0.0.1:4000", "waiting", Heartbeat(tick)),
					note2("upstream", "api.anthropic.com", ""),
					"", "  traffic",
				}, append(Header(),
					Empty(), Empty(), Empty(), "",
					"  "+Pinwheel(tick)+" no requests yet. point a client at the address above.")...)
			},
		},
		{
			Name: "a request in flight",
			Render: func(tick int) []string {
				return append([]string{
					"  replay serve                                                    v0.4.0", "",
					stat("listening", "127.0.0.1:4000", "active", Heartbeat(tick)),
					Awaiting("this turn", tick),
					"", "  traffic",
				}, append(Header(),
					LiveRow("15:06:44", "anthropic", "api.anthropic.com", "messages", tick),
					Traffic("15:06:41", "anthropic", "api.anthropic.com", "messages", "parsed"),
					Empty(), Empty())...)
			},
		},
	}
}
