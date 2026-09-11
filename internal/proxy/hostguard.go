package proxy

import (
	"net"
	"net/http"
	"strconv"
	"strings"
)

// The Host guard, finding 6 of the 2026-09-04 security review.
//
// Replay binds loopback and refuses anything else, and that is not enough on
// its own. A hostile page can point a name it controls at 127.0.0.1 — DNS
// rebinding — and the browser will then treat the loopback listener as
// same-origin for that name and send requests with no Origin header at all.
// The existing browser guard reads Origin and Sec-Fetch-Mode, which a plain
// <form> post and an <img> do not set, so it was a guard resting on the
// attacker's cooperation.
//
// The Host header does not rest on that. A rebound request necessarily
// carries the attacker's name in it, because that is the name the page asked
// for. Checking it costs nothing and does not depend on the browser
// volunteering anything.
//
// Confined to TCP on purpose. A Unix socket has no DNS to rebind and no
// browser reach, and its clients address it through a placeholder authority
// (`http://replay/…`), so the same check there would refuse the transport
// that is already the more isolated one.

// hostIsLocal reports whether a Host header names this machine.
//
// Accepted: an empty Host (HTTP/1.0 and some local clients send none), any
// loopback IP literal, `localhost`, and any name under `.localhost`, which
// RFC 6761 reserves for loopback. Everything else is refused.
//
// The `.localhost` clause is a label-boundary check, not a suffix match:
// `notlocalhost` and `localhost.evil.example` are foreign names and must
// stay foreign.
func hostIsLocal(host string) bool {
	if host == "" {
		return true
	}
	name := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		name = h
	}
	name = strings.TrimSuffix(strings.TrimPrefix(name, "["), "]")
	if ip := net.ParseIP(name); ip != nil {
		return ip.IsLoopback()
	}
	name = strings.ToLower(strings.TrimSuffix(name, "."))
	if name == "localhost" {
		return true
	}
	// A label under .localhost, with a non-empty label in front of it so
	// "..localhost" is not accepted as one.
	rest, ok := strings.CutSuffix(name, ".localhost")
	return ok && rest != "" && !strings.HasSuffix(rest, ".")
}

// overTCP reports whether the request arrived on a TCP listener. A Unix
// socket connection carries a *net.UnixAddr as its local address.
func overTCP(r *http.Request) bool {
	addr, _ := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
	if addr == nil {
		return true
	}
	return addr.Network() != "unix"
}

// notLocal answers 403 and reports true when the request must be refused
// because it did not come from this machine: a browser origin on any route,
// or a Host header naming somewhere else on a TCP listener.
//
// This is the part of the guard that applies to /replay/healthz too. The
// token is not: `replay doctor` probes healthz without one, and that probe is
// how an operator finds out why their agent is failing.
func (s *Server) notLocal(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("Origin") != "" || r.Header.Get("Sec-Fetch-Mode") != "" {
		http.Error(w, "replay: browser-originated requests are not accepted", http.StatusForbidden)
		return true
	}
	if overTCP(r) && !hostIsLocal(r.Host) {
		// Name the value. This refusal has one false positive worth being
		// explicit about — a custom name in /etc/hosts pointed at 127.0.0.1,
		// which is byte-for-byte what DNS rebinding looks like from here — and
		// an operator who hits it needs to know which header did it and what
		// to change, not just that something was forbidden.
		http.Error(w, "replay: Host "+strconv.Quote(r.Host)+" does not name this machine. "+
			"Replay accepts localhost, a .localhost name, or a loopback address; "+
			"a name pointed at 127.0.0.1 in /etc/hosts is indistinguishable from DNS rebinding and is refused. "+
			"Set ANTHROPIC_BASE_URL to http://127.0.0.1:PORT or http://localhost:PORT.",
			http.StatusForbidden)
		return true
	}
	return false
}
