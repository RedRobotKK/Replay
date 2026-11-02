package evidenceclaim

// Guard-reachability contract tests for the artifact-binding fields and the
// renderer's conditional emission (claim.go:171,200,203,207). A non-Unverified
// verdict carries the artifact digest and value span; the renderer emits each
// field only when it is actually present, and never implies evidence that was
// not established.

import (
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/evidenceverify"
)

func TestNonUnverifiedCarriesArtifactBinding(t *testing.T) {
	// claim.go:171 — a Confirmed verdict must carry the whole-artifact digest
	// and the pinned value span.
	r := fullEval(t, []byte(`{"is_error":true}`))
	if r.Verdict != evidenceverify.Confirmed {
		t.Fatalf("fixture did not confirm: %s %s", r.Verdict, r.Reason)
	}
	if r.ArtifactSHA256 == "" {
		t.Fatalf("confirmed result must carry ArtifactSHA256")
	}
	if r.ValueSpan == (evidenceverify.Span{}) {
		t.Fatalf("confirmed result must carry ValueSpan")
	}
}

func TestRendererEmitsOnlyPresentFields(t *testing.T) {
	confirmed := fullEval(t, []byte(`{"is_error":true}`))
	out := confirmed.Render()

	// claim.go:200 — the artifact line appears only when the digest is set.
	if !strings.Contains(out, "artifact sha256") {
		t.Fatalf("confirmed render missing artifact sha256 line:\n%s", out)
	}
	// claim.go:203 — the value-span line appears only when the span is set.
	if !strings.Contains(out, "value span") {
		t.Fatalf("confirmed render missing value span line:\n%s", out)
	}
	// claim.go:207 — the reason line appears only when a reason is set.
	if !strings.Contains(out, "reason") {
		t.Fatalf("confirmed render missing reason line:\n%s", out)
	}

	// An unverified result (no basis) must NOT claim an artifact digest or a
	// value span, and must not emit those lines.
	unverified := Evaluate(ClaimProposition, "s", nil, evidenceverify.Locator{}, claimPredicate(), CoverageNone)
	if unverified.ArtifactSHA256 != "" || unverified.ValueSpan != (evidenceverify.Span{}) {
		t.Fatalf("unverified result must carry no binding: %+v", unverified)
	}
	uout := unverified.Render()
	if strings.Contains(uout, "artifact sha256") || strings.Contains(uout, "value span") {
		t.Fatalf("unverified render must not imply absent evidence:\n%s", uout)
	}
}
