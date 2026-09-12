package proxy

import (
	"encoding/json"
	"io"
	"net/http"
)

// The read endpoints, and the gate in front of everything.
//
// localOnly is the invariant this file exists to hold in one place: no
// browser origin, a Host header that names this machine, and the token when
// one is configured. Every handler in the package passes through it, the
// passthrough in passthrough.go included, so a new endpoint that forgets it
// is a diff against this file rather than a line added somewhere else.
//
// health deliberately carries less. It runs the browser and Host guards but
// not the token, because `replay doctor` probes it to explain a broken agent
// setup and has no way to learn a token it did not set. That asymmetry is
// intentional and is the thing to read twice before changing.
//
// Moving these handlers does not change what they answer or who may call
// them. hostguard.go still owns notLocal; this file only orders it.

// health answers "ok". It carries the browser and Host guards — a page that
// could reach it learns Replay is running here, which is a fingerprint — but
// deliberately NOT the token: `replay doctor` probes this endpoint to explain
// a broken agent setup and has no way to learn a token it did not set.
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if s.notLocal(w, r) {
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = io.WriteString(w, "ok\n") // best-effort health response
}

// localOnly guards the read endpoints and the passthrough: no browser
// origins, a Host header that names this machine, and the token when one is
// configured.
func (s *Server) localOnly(w http.ResponseWriter, r *http.Request) bool {
	if s.notLocal(w, r) {
		return false
	}
	if s.cfg.Token != "" && r.Header.Get(HeaderToken) != s.cfg.Token {
		http.Error(w, "replay: missing or wrong "+HeaderToken+" header", http.StatusUnauthorized)
		return false
	}
	return true
}

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	if !s.localOnly(w, r) {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	// A failed write means the reader went away; nothing to do.
	st := s.stats.status()
	// A dollar cap that cannot be applied is worth reporting: an unpriced model
	// contributes nothing to the running total, so the cap never fires and the
	// operator silently has no cap on that traffic.
	st.SpendCapNotEnforced = s.cfg.Spend.CapNotEnforced()
	st.Caps = s.cfg.Spend.Configured()
	_ = json.NewEncoder(w).Encode(st)
}

func (s *Server) metrics(w http.ResponseWriter, r *http.Request) {
	if !s.localOnly(w, r) {
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = io.WriteString(w, s.stats.metrics()) // best-effort scrape response
}
