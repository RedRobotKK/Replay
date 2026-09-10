package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/RedRobotKK/Replay/internal/observation"
)

// runPool aggregates corpus submissions into one figure whose parts a reader
// can fetch.
//
// `cost --contribute` has written submissions since it shipped and nothing read
// one back: internal/observation.Pool was complete — totals as methods over the
// roster so no drifted figure can be stored, digests re-derived on Add, a
// RulesVersion refusal — and had no caller in the binary. The unwired-packages
// guard could not see it, because it asks whether a PACKAGE is reachable and
// internal/observation is: contribute.go imports Corpus, and that one live type
// vouched for the dead one beside it.
//
// Pooling happens where the submissions were collected — a pull request, a
// directory of checked-in files — never over a wire. This command reads local
// paths and nothing else, which is why it needs no allowlist of its own.
func runPool(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("pool", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit the pooled document rather than the table")
	// pooledAt is a flag with a dated default rather than a clock read, for the
	// reason NewPool states: a document reporting a different timestamp on every
	// regeneration cannot be diffed against the one that was published.
	pooledAt := fs.String("pooled-at", timeNow().UTC().Format("2006-01-02"),
		"the date this pool was assembled, recorded in the document")
	fs.Usage = func() {
		fmt.Fprint(stderr, "Usage: replay pool <submission.json...> [--json] [--pooled-at <date>]\n\n"+
			"Aggregate corpus submissions written by `replay cost --contribute`.\n"+
			"Reads local files. Sends nothing.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	files := fs.Args()

	p := observation.NewPool(*pooledAt)
	admitted := 0
	for _, f := range files {
		body, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintf(stderr, "replay pool: %s: %v\n", f, err)
			continue
		}
		var c observation.Corpus
		if err := json.Unmarshal(body, &c); err != nil {
			fmt.Fprintf(stderr, "replay pool: %s: not a submission: %v\n", f, err)
			continue
		}
		// A refusal is reported and the run continues. Failing the whole pool
		// on one bad file would make a reader re-run it without that file and
		// never learn what was wrong with it; swallowing the refusal would
		// publish a figure covering fewer submissions than the operator handed
		// over, which is the failure the roster exists to prevent.
		if err := p.Add(c, filepath.Base(f)); err != nil {
			fmt.Fprintf(stderr, "replay pool: %s: %v\n", f, err)
			continue
		}
		admitted++
	}

	if *asJSON {
		// Totals refuses an empty pool, and MarshalJSON is built on the same
		// roster. Ask first so the refusal is the error rather than a document
		// stating nothing.
		if _, err := p.Totals(); err != nil {
			return err
		}
		b, err := json.MarshalIndent(p, "", "  ")
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "%s\n", b)
		return nil
	}

	out, err := p.Render()
	if err != nil {
		return err
	}
	fmt.Fprint(stdout, out)
	return nil
}
