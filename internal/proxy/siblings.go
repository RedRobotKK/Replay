package proxy

import (
	"context"
	"sync"
	"time"

	"github.com/RedRobotKK/Buffy/internal/cachemodel"
)

// SiblingSettings configures the hold-parallel-siblings policy. A cache
// entry becomes readable only once the first response that writes it
// begins streaming, so parallel requests with the same prefix (sub-agents
// started together) all pay the write price. With the policy on, a request
// whose prefix is already in flight and not yet cached waits for that
// first response to begin, then goes out and reads the entry.
type SiblingSettings struct {
	// MaxWait bounds how long a request waits behind a sibling. Zero is
	// off: nothing is ever held.
	MaxWait time.Duration
}

// DefaultSiblingWait is the longest a held request waits, chosen above
// the provider's usual time to first byte and well below a client's
// request timeout.
const DefaultSiblingWait = 10 * time.Second

// siblingWarmWindow is how long after a response began that its prefix is
// assumed cached, so later requests with that prefix are not held. It is
// the short cache lifetime; a refreshed entry lives longer, which only
// means a request that could have been held is not.
const siblingWarmWindow = cachemodel.TTLShort

// maxSiblingPrefixes bounds the warm table; the oldest entries are dropped
// past it.
const maxSiblingPrefixes = 1024

// siblingGate is the policy's state: which prefixes have a first response
// in flight and which are known cached.
type siblingGate struct {
	settings SiblingSettings
	now      func() time.Time
	mu       sync.Mutex
	flights  map[string]*flight
	warm     map[string]time.Time
}

// flight is one prefix's first request; done closes when its response
// begins or it ends without one.
type flight struct {
	done chan struct{}
	once sync.Once
}

func (f *flight) finish() { f.once.Do(func() { close(f.done) }) }

func newSiblingGate(settings SiblingSettings) *siblingGate {
	return &siblingGate{settings: settings, now: time.Now, flights: map[string]*flight{}, warm: map[string]time.Time{}}
}

// enter is called before a request is forwarded. A request whose prefix is
// warm goes straight through. One whose prefix has no request in flight
// becomes the leader and must call release once its response has begun
// (see began) or it ended without one. Any other waits, up to MaxWait or
// the request's own cancellation, then goes through. The wait is returned.
func (g *siblingGate) enter(ctx context.Context, prefix string) (release func(), waited time.Duration) {
	noop := func() {}
	if g == nil || g.settings.MaxWait <= 0 || prefix == "" {
		return noop, 0
	}
	start := g.now()
	deadline := time.NewTimer(g.settings.MaxWait)
	defer deadline.Stop()
	for {
		g.mu.Lock()
		if g.isWarm(prefix) {
			g.mu.Unlock()
			return noop, g.now().Sub(start)
		}
		f := g.flights[prefix]
		if f == nil {
			f = &flight{done: make(chan struct{})}
			g.flights[prefix] = f
			g.mu.Unlock()
			return func() { g.finish(prefix, f) }, g.now().Sub(start)
		}
		g.mu.Unlock()
		select {
		case <-f.done:
		case <-ctx.Done():
			return noop, g.now().Sub(start)
		case <-deadline.C:
			return noop, g.now().Sub(start)
		}
	}
}

// began records that a leader's response has begun, which is when the
// provider's cache entry becomes readable, and lets its siblings go.
func (g *siblingGate) began(prefix string) {
	if g == nil {
		return
	}
	g.mu.Lock()
	g.warm[prefix] = g.now()
	g.prune()
	f := g.flights[prefix]
	delete(g.flights, prefix)
	g.mu.Unlock()
	if f != nil {
		f.finish()
	}
}

// finish releases a leader that ended without a successful response;
// the next sibling to wake becomes the leader.
func (g *siblingGate) finish(prefix string, f *flight) {
	g.mu.Lock()
	if g.flights[prefix] == f {
		delete(g.flights, prefix)
	}
	g.mu.Unlock()
	f.finish()
}

// isWarm reports whether a prefix had a response begin within the window.
// Callers hold the lock.
func (g *siblingGate) isWarm(prefix string) bool {
	at, ok := g.warm[prefix]
	return ok && g.now().Sub(at) < siblingWarmWindow
}

// prune drops expired entries, and then the oldest, past the bound.
// Callers hold the lock.
func (g *siblingGate) prune() {
	if len(g.warm) <= maxSiblingPrefixes {
		return
	}
	now := g.now()
	for p, at := range g.warm {
		if now.Sub(at) >= siblingWarmWindow {
			delete(g.warm, p)
		}
	}
	for len(g.warm) > maxSiblingPrefixes {
		oldest, oldestAt := "", now
		for p, at := range g.warm {
			if at.Before(oldestAt) {
				oldest, oldestAt = p, at
			}
		}
		delete(g.warm, oldest)
	}
}
