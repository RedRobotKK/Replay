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

	"github.com/RedRobotKK/Replay/internal/claims"
)

func main() {
	body := claims.Render()
	if err := os.WriteFile("CLAIM-REGISTER.md", []byte(body), 0o644); err != nil {
		panic(err)
	}
	fmt.Printf("wrote CLAIM-REGISTER.md: %d claims\n", len(claims.Register))
}
