package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"net"
	"net/http"
	"strconv"
	"time"
)

// RetrySettings bounds the proxy's own retries. Zero Attempts is off.
type RetrySettings struct {
	// Attempts is how many times a failed request is sent again.
	Attempts int
	// BaseDelay is the first backoff; each retry doubles it, with jitter,
	// up to MaxDelay. A provider Retry-After header replaces the backoff
	// when it fits under MaxDelay and ends the retries when it does not.
	BaseDelay time.Duration
	MaxDelay  time.Duration
}

// Retry defaults, sized to the provider's own guidance of a short first
// wait and a ceiling an agent's user would still sit through.
const (
	DefaultRetryBaseDelay = 500 * time.Millisecond
	DefaultRetryMaxDelay  = 30 * time.Second
	// retryDrainBytes bounds how much of a failed response is read so the
	// connection can be reused before the retry.
	retryDrainBytes = 64 << 10
)

// retryCounter is the per-request tally the transport fills in and the
// handler records. It travels in the request context.
type retryCounter struct{ n int }

type retryKey struct{}

// withRetryCounter attaches a counter to the request the handler forwards.
func withRetryCounter(r *http.Request) (*http.Request, *retryCounter) {
	c := &retryCounter{}
	return r.WithContext(context.WithValue(r.Context(), retryKey{}, c)), c
}

// retryTransport sends a request again when the provider answered with a
// retryable status or the connection failed. It sits below the reverse
// proxy, so a retry can only happen before any byte of a response has
// been written to the client: the proxy writes headers only after
// RoundTrip returns. Client errors are never retried.
type retryTransport struct {
	next     http.RoundTripper
	settings RetrySettings
	logger   *log.Logger
	// sleep and jitter are replaced in tests.
	sleep  func(ctx context.Context, d time.Duration) error
	jitter func() float64
}

func newRetryTransport(next http.RoundTripper, s RetrySettings, logger *log.Logger) *retryTransport {
	return &retryTransport{next: next, settings: s, logger: logger, sleep: sleepContext, jitter: rand.Float64}
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	counter, _ := req.Context().Value(retryKey{}).(*retryCounter)
	for attempt := 0; ; attempt++ {
		resp, err := t.next.RoundTrip(req)
		retryable := (err != nil && beforeRequestSent(err)) || (err == nil && IsRetryableStatus(resp.StatusCode))
		if !retryable || attempt >= t.settings.Attempts || req.GetBody == nil {
			return resp, err
		}
		delay, ok := t.delay(attempt, resp)
		if !ok {
			return resp, err
		}
		reason := "connection failed"
		if err == nil {
			reason = "status " + strconv.Itoa(resp.StatusCode)
			drain(resp)
		} else if req.Context().Err() != nil {
			// The client went away; nothing to retry for.
			return resp, err
		}
		t.logger.Printf("retry %d/%d in %s after %s", attempt+1, t.settings.Attempts, delay.Round(time.Millisecond), reason)
		if err := t.sleep(req.Context(), delay); err != nil {
			return nil, err
		}
		body, err := req.GetBody()
		if err != nil {
			return nil, fmt.Errorf("retry: reopen request body: %w", err)
		}
		req = req.Clone(req.Context())
		req.Body = body
		if counter != nil {
			counter.n++
		}
	}
}

// delay picks the wait before the next attempt: the provider's
// Retry-After when it gives one and it fits, otherwise doubled backoff
// with full jitter. A Retry-After beyond the ceiling ends the retries,
// since the client would wait longer than the user expects.
func (t *retryTransport) delay(attempt int, resp *http.Response) (time.Duration, bool) {
	if resp != nil {
		if after, ok := retryAfter(resp.Header.Get("Retry-After"), time.Now()); ok {
			return after, after <= t.settings.MaxDelay
		}
	}
	backoff := t.settings.BaseDelay << attempt
	if backoff > t.settings.MaxDelay || backoff <= 0 {
		backoff = t.settings.MaxDelay
	}
	return time.Duration(float64(backoff) * t.jitter()), true
}

// retryAfter parses the header's two forms: seconds, or an HTTP date.
func retryAfter(value string, now time.Time) (time.Duration, bool) {
	if value == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(value); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second, true
	}
	if at, err := http.ParseTime(value); err == nil {
		return max(at.Sub(now), 0), true
	}
	return 0, false
}

// dialError marks a failure to connect, which is the one transport
// failure that cannot have billed the request.
type dialError struct{ err error }

func (e dialError) Error() string { return e.err.Error() }
func (e dialError) Unwrap() error { return e.err }

// dialContext wraps a dialer so its failures are recognizable.
func dialContext(d *net.Dialer) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := d.DialContext(ctx, network, addr)
		if err != nil {
			return nil, dialError{err}
		}
		return conn, nil
	}
}

// beforeRequestSent reports whether a transport error happened before
// the request could have reached the provider. A reset after the body
// was written, or a timeout waiting for headers, may already have been
// billed and is not resent.
func beforeRequestSent(err error) bool {
	var d dialError
	return errors.As(err, &d)
}

func drain(resp *http.Response) {
	// Best effort: a failed drain only costs the pooled connection.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, retryDrainBytes))
	_ = resp.Body.Close()
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// errRetriesOff marks settings that disable retries; used by New.
var errRetriesOff = errors.New("retries off")

func (s RetrySettings) validate() error {
	if s.Attempts <= 0 {
		return errRetriesOff
	}
	if s.BaseDelay <= 0 || s.MaxDelay < s.BaseDelay {
		return fmt.Errorf("retry delays must satisfy 0 < base <= max, got base=%s max=%s", s.BaseDelay, s.MaxDelay)
	}
	return nil
}
