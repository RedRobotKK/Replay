package proxy

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
)

// The read-only metrics listener.
//
// Prometheus cannot scrape a Unix socket. Without this, moving the proxy onto
// a socket for the isolation it gives costs you the ability to be scraped, and
// being scraped costs you the isolation — the two features cancel. A second
// listener resolves it: the proxy stays where it is, and the counters get
// their own address.
//
// Its load-bearing property is that it CANNOT proxy, and that is a structural
// property here rather than a guarded one. It is served by a mux of its own
// carrying exactly three routes and no "/" catch-all, so there is no path for
// a request to reach the provider through it — not a check that could be
// removed, but a route that does not exist. If it could proxy, this port would
// be a complete bypass of the transport it exists to complement: anyone able
// to open a loopback connection could spend against the key the proxy holds.
//
// It is off unless asked for. A port that appeared by default would hand the
// counters to every local process, for users who never wanted to be scraped.

// readOnlyMux builds the handler for the metrics listener.
//
// Deliberately not the server's main mux. Sharing it would mean any route
// added there later — including the "/" that forwards to the provider —
// appearing here too, silently, with nothing to notice it. The separation is
// what keeps that from being possible.
func (s *Server) readOnlyMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(HealthPath, s.health)
	mux.HandleFunc(StatusPath, s.status)
	mux.HandleFunc(MetricsPath, s.metrics)
	return mux
}

// listenMetrics binds the metrics listener, or returns nil when none was asked
// for. It accepts the same address forms as the proxy: a loopback host:port,
// or unix:// for a socket that is owner-only like any other.
func listenMetrics(addr string) (net.Listener, error) {
	if addr == "" {
		return nil, nil
	}
	if isUnixAddr(addr) {
		return listenUnix(addr)
	}
	if !isLoopback(addr) {
		return nil, fmt.Errorf("metrics listen address %q is not loopback; these counters name "+
			"repositories and token spend, and Replay will not publish them to a network", addr)
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", addr, err)
	}
	return ln, nil
}

// serveMetrics runs the metrics listener until ctx is done. It returns nil
// when there is nothing to serve.
//
// A failure here is reported rather than tolerated: somebody who asked to be
// scraped and silently is not would discover it from a gap in a dashboard,
// days later, and read it as an outage.
func (s *Server) serveMetrics(ctx context.Context, ln net.Listener) error {
	if ln == nil {
		return nil
	}
	srv := &http.Server{Handler: s.readOnlyMux(), ReadHeaderTimeout: ReadHeaderTimeout}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	select {
	case <-ctx.Done():
		return srv.Close()
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// MetricsAddr is where the metrics listener bound, or "" when none was asked
// for. It blocks until the server is ready, like Addr.
func (s *Server) MetricsAddr() string {
	<-s.ready
	return s.metricsAddr
}
