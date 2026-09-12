package proxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"time"
)

// Binding, serving and stopping. Nothing in this file reads a request body
// or a response body, and that is the point of it being a file: a change to
// how the proxy starts or stops cannot reach what the proxy sends.
//
// The invariant it keeps is that readiness is signalled on every path,
// including the ones that fail. Addr blocks on the ready channel, so a bind
// that returns an error without closing it deadlocks every caller waiting on
// a server that is never going to serve — an error path, which is where
// nobody looks, and an ordinary one since the socket transport refuses for
// several good reasons. The second invariant is the shutdown one: a
// connection that was accepted and never sent a request carries no turn and
// is closed rather than waited for, because waiting for it turned Ctrl-C
// into a five-second hang.
//
// Separating it does not add a test to any of this. It only means the next
// patch to the listen path is not written three screens from the masker.

// ListenAndServe binds and serves until the context is cancelled.
func (s *Server) ListenAndServe(ctx context.Context) error {
	// Whatever happens, unblock anything waiting on Addr. A bind that fails
	// used to leave ready unclosed, so every caller of Addr blocked forever
	// on a server that was never going to serve — a deadlock reachable only
	// on the error path, which is where nobody looks. The socket transport
	// refuses for several good reasons, so that path is now ordinary.
	defer s.markReady()

	var ln net.Listener
	var err error
	if isUnixAddr(s.cfg.Listen) {
		ln, err = listenUnix(s.cfg.Listen)
		if err != nil {
			return err
		}
		// Go's UnixListener unlinks the socket on Close, so a clean shutdown
		// leaves nothing behind for the next start to treat as stale.
		s.addr = socketPath(s.cfg.Listen)
		if abs, aerr := filepath.Abs(s.addr); aerr == nil {
			s.addr = abs
		}
	} else {
		ln, err = net.Listen("tcp", s.cfg.Listen)
		if err != nil {
			return fmt.Errorf("listen on %s: %w", s.cfg.Listen, err)
		}
		s.addr = ln.Addr().String()
	}
	// Bind the metrics listener before announcing readiness. Ready has to mean
	// both listeners are up, or a caller that waits on Addr and then reads
	// MetricsAddr races the bind and sees an empty string.
	mln, merr := bindMetrics(s.cfg.MetricsListen)
	if merr != nil {
		_ = ln.Close()
		return merr
	}
	if mln != nil {
		s.metricsAddr = mln.Addr().String()
		if isUnixAddr(s.cfg.MetricsListen) {
			s.metricsAddr = socketPath(s.cfg.MetricsListen)
			if abs, aerr := filepath.Abs(s.metricsAddr); aerr == nil {
				s.metricsAddr = abs
			}
		}
	}
	s.markReady()

	// nil when there is no metrics listener. Receiving from a nil channel
	// blocks forever, which is exactly what the select below needs: without
	// this, serveMetrics returns nil immediately for the no-listener case and
	// the "metrics listener stopped" arm fires at once, shutting the proxy
	// down the moment it starts.
	var mdone chan error
	if mln != nil {
		mdone = make(chan error, 1)
		go func() { mdone <- s.serveMetrics(ctx, mln) }()
	}
	errc := make(chan error, 1)
	go func() { errc <- s.http.Serve(ln) }()
	select {
	case <-ctx.Done():
		err := s.shutdown()
		if mdone != nil {
			if mErr := <-mdone; err == nil {
				err = mErr
			}
		}
		return err
	case err := <-mdone:
		// A metrics listener that dies mid-run must not be tolerated in
		// silence: somebody who asked to be scraped and quietly is not finds
		// out from a gap in a dashboard days later, and reads it as an
		// outage. Stop the proxy too, so the failure is loud at the point it
		// happens rather than at the next shutdown.
		_ = s.shutdown()
		if err == nil {
			return nil
		}
		return fmt.Errorf("metrics listener stopped: %w", err)
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// markReady unblocks Addr exactly once, whether the server bound or failed.
func (s *Server) markReady() {
	s.readyOnce.Do(func() { close(s.ready) })
}

// noteConnState tracks connections that have been accepted but have not
// sent a request. Every other state means the connection either carries a
// request or is finished, and a graceful shutdown handles both.
func (s *Server) noteConnState(c net.Conn, state http.ConnState) {
	s.idleMu.Lock()
	defer s.idleMu.Unlock()
	if state == http.StateNew {
		s.idleConns[c] = struct{}{}
		return
	}
	delete(s.idleConns, c)
}

// closeUnusedConns closes the connections that never sent a request and
// returns how many. A connection that started a request between the state
// callback and this call is no longer in the map, so no turn is cut.
func (s *Server) closeUnusedConns() int {
	s.idleMu.Lock()
	conns := make([]net.Conn, 0, len(s.idleConns))
	for c := range s.idleConns {
		conns = append(conns, c)
	}
	s.idleConns = map[net.Conn]struct{}{}
	s.idleMu.Unlock()
	for _, c := range conns {
		// The connection is being discarded; a close error says only that
		// the peer got there first.
		_ = c.Close()
	}
	return len(conns)
}

// shutdown stops serving: turns in flight get the grace period, then
// whatever is left is closed.
//
// A graceful shutdown alone is not enough. It waits for every connection
// to become idle, and one that has been accepted but has not sent a
// request never does; it is closed only when ReadHeaderTimeout expires,
// which is deliberately longer than the grace period so a slow client is
// not cut off mid-header. Agents hold pooled connections open exactly
// like that, so waiting for them turned Ctrl-C into a five-second hang
// and a non-zero exit. Those connections carry no turn, so they are
// closed first; the force-close afterwards is the backstop for a turn
// that outlasts the grace period.
func (s *Server) shutdown() error {
	if n := s.closeUnusedConns(); n > 0 {
		s.cfg.Logger.Printf("shutdown: closed %d connection(s) with no request in flight", n)
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.grace())
	defer cancel()
	err := s.http.Shutdown(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	s.cfg.Logger.Printf("shutdown: turns still running after %s; closing them", s.grace())
	return s.http.Close()
}

// grace is how long a shutdown waits for connections to finish.
func (s *Server) grace() time.Duration {
	if s.shutdownGrace > 0 {
		return s.shutdownGrace
	}
	return ShutdownTimeout
}

// Addr is the bound address, valid after ListenAndServe has started.
func (s *Server) Addr() string {
	<-s.ready
	return s.addr
}
