package version

import (
	"strings"
	"testing"
)

func TestStringContainsAllFields(t *testing.T) {
	Version, Commit, Date = "1.2.3", "abc1234", "2026-09-02T00:00:00Z"
	got := String()
	for _, want := range []string{"1.2.3", "abc1234", "2026-09-02T00:00:00Z"} {
		if !strings.Contains(got, want) {
			t.Errorf("String() = %q, missing %q", got, want)
		}
	}
}
