// Command buffy is the Project Buffy daemon and command-line entry point.
//
// The proxy itself is not implemented yet. This skeleton exists so the
// build, test, and lint pipeline is real from the first commit. See
// docs/ROADMAP.md for what lands in v0.1.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/RedRobotKK/Buffy/internal/version"
)

// errNotImplemented is returned by subcommands that are scheduled but not yet built.
var errNotImplemented = errors.New("not implemented yet; see docs/ROADMAP.md")

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "buffy:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}
	switch args[0] {
	case "version", "--version", "-v":
		fmt.Println("buffy", version.String())
		return nil
	case "serve":
		return errNotImplemented
	case "help", "--help", "-h":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printUsage() {
	fmt.Print(`buffy - local context proxy for coding agents

Usage:
  buffy serve      start the proxy daemon (not implemented yet)
  buffy version    print build information
  buffy help       show this message
`)
}
