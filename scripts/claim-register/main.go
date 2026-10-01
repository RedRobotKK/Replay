//go:build ignore

// Command claim-register renders internal/claims to CLAIM-REGISTER.md.
//
// The register is the source. This only formats it, so the document cannot
// drift from the code: regenerate rather than edit.
//
//	go run scripts/claim-register/main.go

package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/RedRobotKK/Replay/internal/claims"
)

func list(label string, xs []string) string {
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

func main() {
	var b strings.Builder
	b.WriteString("# Claim register, complete\n\n")
	b.WriteString("**Generated from `internal/claims` at build time. Do not edit by hand:\nregenerate it, or it will drift from the code it describes.**\n\n")

	byResult := map[claims.Result]int{}
	for _, c := range claims.Register {
		byResult[c.Result]++
	}
	b.WriteString("| Result | Count |\n|---|---|\n")
	var keys []string
	for k := range byResult {
		keys = append(keys, string(k))
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "| %s | %d |\n", k, byResult[claims.Result(k)])
	}
	fmt.Fprintf(&b, "| **Total** | **%d** |\n\n---\n\n", len(claims.Register))

	for _, c := range claims.Register {
		ctl := claims.ControlsFor[c.ID]
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
		b.WriteString(list("Establishes", c.Establishes))
		b.WriteString(list("Does NOT establish", c.DoesNotEstablish))
		b.WriteString(list("Assumptions Replay does not verify", ctl.Assumption))
		b.WriteString(list("Known gaps", ctl.Gap))
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
		b.WriteString(list("Tests", c.Tests))
		fmt.Fprintf(&b, "\n**Why this result:** %s\n\n---\n\n", c.Why)
	}
	if err := os.WriteFile("CLAIM-REGISTER.md", []byte(b.String()), 0o644); err != nil {
		panic(err)
	}
	fmt.Printf("wrote CLAIM-REGISTER.md: %d claims\n", len(claims.Register))
}
