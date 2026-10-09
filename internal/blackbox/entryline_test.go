//go:build mutation

package blackbox

import "testing"

// The matrix's Production Entry cell is the exact argv the black box handed
// the binary. `serve`'s refusal check passes an empty `--upstream`, and the
// cell rendered that as `replay serve --preflight -5 --upstream ` with a
// trailing space inside the code span: a reader cannot see that an argument
// was given at all, and Markdown lint reads the space as MD038. An empty
// argument is rendered as `""`, which is what the shell would have needed.
//
// PASS: an empty argument renders as "" and every other argument is unchanged.
// FAIL: the empty argument disappears into whitespace.
func TestBB_EntryLineQuotesAnEmptyArgument(t *testing.T) {
	got := entryLine("/nonexistent-home", []string{"serve", "--preflight", "-5", "--upstream", ""})
	want := `replay serve --preflight -5 --upstream ""`
	if got != want {
		t.Fatalf("entryLine = %q, want %q", got, want)
	}
}
