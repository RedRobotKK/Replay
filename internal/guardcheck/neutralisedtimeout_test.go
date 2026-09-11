package guardcheck_test

import (
	"testing"
	"time"

	"github.com/RedRobotKK/Replay/internal/guardcheck"
)

// TestGC29_ANeutralisedRunIsBoundedByTheBaseline pins the three properties the
// bound has to have, because a wrong one is invisible: too low and a slow
// machine reports a mutant caught that was not, too high and the reviewer
// times out a CI runner before it finishes.
func TestGC29_ANeutralisedRunIsBoundedByTheBaseline(t *testing.T) {
	cases := []struct {
		name     string
		baseline time.Duration
		want     time.Duration
		why      string
	}{
		{"a fast suite still gets the floor", 2 * time.Second, 60 * time.Second,
			"four times two seconds is eight, and a loaded machine would fail a guard that is fine"},
		{"the floor holds right up to it", 15 * time.Second, 60 * time.Second,
			"four times fifteen is the floor exactly"},
		{"past the floor it scales with the suite", 30 * time.Second, 2 * time.Minute,
			"a suite that takes thirty seconds gets two minutes"},
		{"and is capped at the go test default", 5 * time.Minute, 10 * time.Minute,
			"never longer than the default, so this can only make the reviewer faster"},
		{"a pathological baseline is still capped", time.Hour, 10 * time.Minute, "same"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := guardcheck.NeutralisedTimeout(c.baseline); got != c.want {
				t.Fatalf("NeutralisedTimeout(%s) = %s, want %s — %s", c.baseline, got, c.want, c.why)
			}
		})
	}
}
