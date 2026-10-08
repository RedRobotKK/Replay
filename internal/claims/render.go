package claims

import (
	"fmt"
	"sort"
	"strings"
)

// Render produces CLAIM-REGISTER.md's exact content from Register and
// ControlsFor. It is the one place that formatting exists, so
// scripts/claim-register/main.go and the test that guards against drift
// (TestRegisterMatchesRenderedSource) call the same code a human reading the
// committed file is trusting.
func Render() string {
	var b strings.Builder
	b.WriteString("# Claim register, complete\n\n")
	b.WriteString("**Generated from `internal/claims` at build time. Do not edit by hand:\nregenerate it, or it will drift from the code it describes.**\n\n")

	byResult := map[Result]int{}
	for _, c := range Register {
		byResult[c.Result]++
	}
	b.WriteString("| Result | Count |\n|---|---|\n")
	var keys []string
	for k := range byResult {
		keys = append(keys, string(k))
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "| %s | %d |\n", k, byResult[Result(k)])
	}
	fmt.Fprintf(&b, "| **Total** | **%d** |\n\n---\n\n", len(Register))

	for _, c := range Register {
		ctl := ControlsFor[c.ID]
		fmt.Fprintf(&b, "## %s\n\n> %s\n\n", c.ID, c.Text)
		fmt.Fprintf(&b, "| | |\n|---|---|\n")
		fmt.Fprintf(&b, "| **Result** | **%s** |\n", c.Result)
		fmt.Fprintf(&b, "| Evidence basis | %s |\n", ctl.Basis)
		fmt.Fprintf(&b, "| Deciding layer | %s |\n", ctl.Layer)
		fmt.Fprintf(&b, "| Scope | %s |\n", c.Scope)
		fmt.Fprintf(&b, "| Oracle | %s |\n", c.Oracle)
		if len(c.Asserted) > 0 {
			fmt.Fprintf(&b, "| Asserted at | %s |\n", strings.Join(c.Asserted, ", "))
		} else {
			b.WriteString("| Asserted at | _nowhere. This is a non-claim or an inferred boundary_ |\n")
		}
		b.WriteString("\n")
		b.WriteString(renderList("Establishes", c.Establishes))
		b.WriteString(renderList("Does NOT establish", c.DoesNotEstablish))
		b.WriteString(renderList("Assumptions Replay does not verify", ctl.Assumption))
		b.WriteString(renderList("Known gaps", ctl.Gap))
		pc, nc, ic := ctl.Positive, ctl.Negative, ctl.Insufficient
		if pc == "" {
			pc = "_none; not applicable to a non-claim_"
		}
		if nc == "" {
			nc = "_none; not applicable to a non-claim_"
		}
		if ic == "" {
			ic = "_none. Recorded as a gap_"
		}
		fmt.Fprintf(&b, "- **Positive control:** %s\n", pc)
		fmt.Fprintf(&b, "- **Negative control:** %s\n", nc)
		fmt.Fprintf(&b, "- **Insufficient-evidence control:** %s\n", ic)
		b.WriteString(renderList("Tests", c.Tests))
		fmt.Fprintf(&b, "\n**Why this result:** %s\n\n---\n\n", c.Why)
	}
	return b.String()
}

// renderList formats one labelled bullet list inside a claim's rendered
// section. A nil or empty slice renders as an explicit "none" rather than a
// missing section, so a reader cannot mistake "not populated" for "omitted".
func renderList(label string, xs []string) string {
	if len(xs) == 0 {
		return "- **" + label + ":** _none_\n"
	}
	var b strings.Builder
	b.WriteString("- **" + label + ":**\n")
	for _, x := range xs {
		b.WriteString("  - " + x + "\n")
	}
	return b.String()
}
