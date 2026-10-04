package regression

import (
	"regexp"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/proxy"
)

// QT-7b. A model the price table does not carry is priced at the dearest
// known rate and labelled an upper bound (#261, TOKEN-PRICES.md), so a dollar
// cap fires early on it rather than never. The qualification pass of
// 2026-10-03 found the user-facing surfaces saying the opposite: the serve
// flag help said such a model "counts as free", the guide's cap row said the
// same, the alerting guide and the doctor warning said an agent on an
// unpriced model "runs up a real bill without ever reaching the limit".
// Three documents, one flag and one warning, all describing the behaviour
// the code was changed away from.
//
// This guard ties every statement of the rule to the code: the positive
// control calls the production pricing function on an unknown model and
// fails if it ever prices at zero again, and each surface must say "upper
// bound" and must not say the traffic is free or uncounted. If the code's
// rule changes, this test fails on the positive control and the surfaces
// are rewritten together with it, never apart from it.
func TestQT7b_EverySurfaceStatesTheUnpricedModelRuleTheCodeApplies(t *testing.T) {
	usd, upperBound := proxy.ListCost(ledger.Usage{Input: 1_000_000}, "qt7b-model-that-no-table-carries")
	if !upperBound || usd <= 0 {
		t.Fatalf("positive control: ListCost on an unknown model gave $%.4f, upperBound=%v; the code prices unpriced models at the dearest known rate as an upper bound, and this guard's wording checks assume so", usd, upperBound)
	}

	forbidden := []string{
		"count as free", "counts as free", "counted as free", "treated as free",
		"without ever reaching", "contributes nothing", "never fires", "is not being applied",
		"cannot be reached", "add nothing to the total", "will never be met", "not being enforced",
	}
	surfaces := []struct {
		file     string
		scope    *regexp.Regexp // the part of the file that states the rule
		required string
	}{
		{"cmd/replay/serve.go", regexp.MustCompile(`"max-session-usd", 0, "[^"]*"`), "upper bound"},
		{"cmd/replay/serve.go", regexp.MustCompile(`"max-day-usd", 0, "[^"]*"`), "upper bound"},
		{"cmd/replay/doctor_guards.go", regexp.MustCompile(`(?s)if st\.SpendCapNotEnforced \{.*?\n\t\}`), "upper bound"},
		{"internal/proxy/guards.go", regexp.MustCompile(`(?s)// CapNotEnforced.*?func \(g \*SpendGuard\) CapNotEnforced`), "upper bound"},
		{"internal/tui/guards.go", regexp.MustCompile(`(?s)if g\.SpendCapNotEnforced \{\n\t\t// Urgent.*?\n\t\}`), "upper bound"},
		{"internal/tui/storyboard.go", regexp.MustCompile(`(?s)kv\("could not be priced".*?note\(false`), "upper bound"},
		{"docs/guide/commands.md", regexp.MustCompile("\\| `--max-session-usd`, `--max-day-usd` \\|[^\n]*"), "upper bound"},
		{"docs/DASHBOARD-DESIGN.md", regexp.MustCompile(`(?s)  ! the day dollar cap.*?\n  - `), "upper bound"},
		{"docs/TUI-FLAG-SURFACE.md", regexp.MustCompile("(?s)A fifth condition matters.*?No flag expresses it"), "upper bound"},
		{"docs/guide/alerting.md", regexp.MustCompile(`(?s)### A dollar cap.*?\n### `), "upper bound"},
	}
	for _, s := range surfaces {
		body := readDoc(t, s.file)
		m := s.scope.FindString(body)
		if m == "" {
			t.Errorf("%s: the passage that states the unpriced-model rule was not found (%s)", s.file, s.scope)
			continue
		}
		low := strings.ToLower(m)
		for _, f := range forbidden {
			if strings.Contains(low, f) {
				t.Errorf("%s says %q of an unpriced model; the code prices it at the dearest known rate as an upper bound", s.file, f)
			}
		}
		if !strings.Contains(low, s.required) {
			t.Errorf("%s does not say %q where it states the unpriced-model rule", s.file, s.required)
		}
	}
}
