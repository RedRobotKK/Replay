package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// run is the whole command: simexp count <dataset.json>. Exit 0 with the
// result as JSON on stdout; 1 for a usage error; 2 when the dataset is
// rejected, with the reason on stderr and nothing on stdout.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 || args[0] != "count" {
		_, _ = fmt.Fprintln(stderr, "usage: simexp count <dataset.json>")
		_, _ = fmt.Fprintln(stderr, "  counts the pre-registered primary outcome of "+preregistration)
		_, _ = fmt.Fprintln(stderr, "  over a frozen, participant-reported dataset of exactly ten rows")
		return 1
	}
	raw, err := os.ReadFile(args[1])
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "simexp: dataset rejected: %v\n", err)
		return 2
	}
	r, err := count(raw)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "simexp: %v\n", err)
		return 2
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r); err != nil {
		_, _ = fmt.Fprintf(stderr, "simexp: %v\n", err)
		return 2
	}
	return 0
}
