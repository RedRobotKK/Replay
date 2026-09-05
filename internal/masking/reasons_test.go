package masking

import (
	"path/filepath"
	"strings"
	"testing"
)

// Every denial carries a reason, and until now no test asserted which one.
// The tests that exist check *that* a placeholder was left in place; the
// reason travels further than the decision does. It reaches the operator's
// report and replay_rehydration_denied_total, and it is what someone reads
// when they ask why a secret did not come back. A wrong reason sends them to
// the wrong fix, which is worse than no reason at all.
//
// One case per reason, asserted by name.
func TestEveryDenialReasonIsReportedCorrectly(t *testing.T) {
	project := t.TempDir()
	outside := jsonString(filepath.Join(filepath.Dir(project), "elsewhere.go"))

	for _, tc := range []struct {
		name  string
		specs []string
		block func(ph map[string]string) string
		want  string
	}{
		{
			name: "a tool the pattern's scope excludes",
			block: func(ph map[string]string) string {
				// Bash is not a file-edit tool and the default scope names no
				// tools, so this is the scope refusing, not a path check.
				return `{"type":"tool_use","id":"t","name":"Bash","input":{"command":"export K=` + ph[secret] + `"}}`
			},
			want: ReasonScope,
		},
		{
			name: "a file edit pointing out of the project",
			block: func(ph map[string]string) string {
				return `{"type":"tool_use","id":"t","name":"Edit","input":{"file_path":` + outside + `,"new_string":"` + ph[secret] + `"}}`
			},
			want: ReasonOutsideProject,
		},
		{
			name: "a file-edit tool with no path at all",
			block: func(ph map[string]string) string {
				return `{"type":"tool_use","id":"t","name":"Edit","input":{"new_string":"` + ph[secret] + `"}}`
			},
			want: ReasonNoPath,
		},
		{
			name: "tool input that is not a JSON object",
			block: func(ph map[string]string) string {
				return `{"type":"tool_use","id":"t","name":"Edit","input":"` + ph[secret] + `"}`
			},
			want: ReasonUnparsedInput,
		},
		{
			name: "a placeholder the vault has never seen",
			block: func(map[string]string) string {
				return `{"type":"text","text":"key is ` + PlaceholderPrefix + `0000000000000000"}`
			},
			want: ReasonUnknown,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, ph := fixture(t, project, tc.specs...)
			body := []byte(`{"id":"m","type":"message","role":"assistant","content":[` +
				tc.block(ph) + `],"model":"m","usage":{"input_tokens":1,"output_tokens":1}}`)
			_, rep, err := r.Body(body)
			if err != nil {
				t.Fatal(err)
			}
			if !hasReason(rep.Denied, tc.want) {
				t.Fatalf("want reason %q, got denials %v", tc.want, rep.Denied)
			}
		})
	}
}

// A project root that was never configured cannot be compared against, so an
// in-project edit is refused rather than assumed safe. The CLI always fills
// this in from the working directory, so the branch is defensive; a test keeps
// it honest if that ever changes.
func TestAnUnsetProjectRootDeniesFileEdits(t *testing.T) {
	r, ph := fixture(t, "")
	body := []byte(`{"id":"m","type":"message","role":"assistant","content":[` +
		`{"type":"tool_use","id":"t","name":"Edit","input":{"file_path":"/anywhere/x.go","new_string":"` + ph[secret] + `"}}` +
		`],"model":"m","usage":{"input_tokens":1,"output_tokens":1}}`)
	_, rep, err := r.Body(body)
	if err != nil {
		t.Fatal(err)
	}
	if !hasReason(rep.Denied, ReasonNoProject) {
		t.Fatalf("want %q, got %v", ReasonNoProject, rep.Denied)
	}
}

// The reason is half of the key; the destination is the other half, and an
// operator needs both to know which tool to look at.
func TestDenialsNameTheDestinationAsWellAsTheReason(t *testing.T) {
	project := t.TempDir()
	r, ph := fixture(t, project)
	body := []byte(`{"id":"m","type":"message","role":"assistant","content":[` +
		`{"type":"tool_use","id":"t","name":"Bash","input":{"command":"` + ph[secret] + `"}}` +
		`],"model":"m","usage":{"input_tokens":1,"output_tokens":1}}`)
	_, rep, err := r.Body(body)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for k := range rep.Denied {
		if strings.Contains(k, "tool:Bash") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the denial must name the tool it refused: %v", rep.Denied)
	}
}

func hasReason(denied map[string]int, reason string) bool {
	for k, n := range denied {
		if n > 0 && strings.HasSuffix(k, "/"+reason) {
			return true
		}
	}
	return false
}
