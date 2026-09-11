package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// `replay privacy` answers "what do you hold about me, and where".
//
// The question a subject access request asks, and one this tool could not
// answer until now. A local-first tool is unusually well placed to answer it:
// nothing has to be requested from anyone, because the whole answer is a
// directory on the reader's own disk.
//
// It reports and never removes. Every path it names is one `replay purge` can
// act on, and keeping the two apart means reading what you hold is never one
// keystroke away from destroying it.
//
// The descriptions come from the store registry rather than being written here,
// so a store added later is disclosed by this command without anybody
// remembering to update a second list.

type privacyStore struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Holds     string `json:"holds"`
	Sensitive bool   `json:"sensitive"`
	Purgeable bool   `json:"purgeable"`
	Bytes     int64  `json:"bytes"`
	Files     int    `json:"files"`
	// Unmeasured is how many entries under this store could not be walked.
	// A store printed as "0 B" because nobody could read it is not a store
	// known to be empty, and the reader has to be able to tell those apart.
	Unmeasured int `json:"unmeasured,omitempty"`
}

type privacyReport struct {
	Root   string         `json:"root"`
	Stores []privacyStore `json:"stores"`
}

func runPrivacy(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("privacy", flag.ContinueOnError)
	fs.SetOutput(stderr)
	asJSON := fs.Bool("json", false, "emit the report as JSON")
	fs.Usage = func() {
		_, _ = fmt.Fprint(stderr, "Usage: replay privacy [--json]\n\n"+
			"Lists everything Replay has written to this machine, what each store holds,\n"+
			"and which of them a retention window may remove. Changes nothing.\n\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(hoistFlagsFor(fs, args)); err != nil {
		return errUsage
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("locating the home directory: %w", err)
	}
	root := filepath.Join(home, ".replay")

	stores, rerr := resolveStoresErr(root)
	// "I could not look" is not "there is nothing here". Printing the empty
	// message for an unreadable directory is a false absence claim in the
	// command that answers "what do you hold about me".
	if rerr != nil && !os.IsNotExist(rerr) {
		return fmt.Errorf("reading %s: %w", root, rerr)
	}

	rep := privacyReport{Root: root}
	for _, r := range stores {
		bytes, files, unmeasured := measureStore(r.Full)
		rep.Stores = append(rep.Stores, privacyStore{
			Name: r.Actual, Path: r.Full, Holds: r.Holds,
			Sensitive: r.Sensitive, Purgeable: r.Purgeable,
			Bytes: bytes, Files: files, Unmeasured: unmeasured,
		})
	}

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(rep)
	}

	if len(rep.Stores) == 0 {
		_, _ = fmt.Fprintf(stdout, "Replay has written nothing to this machine. %s does not exist.\n", root)
		return nil
	}

	_, _ = fmt.Fprintf(stdout, "Everything Replay holds on this machine, under %s\n\n", root)
	for _, s := range rep.Stores {
		mark := " "
		if s.Sensitive {
			mark = "!"
		}
		_, _ = fmt.Fprintf(stdout, "%s %-20s %8s", mark, s.Name, formatBytes(s.Bytes))
		if s.Unmeasured > 0 {
			_, _ = fmt.Fprintf(stdout, "  (%d entr(ies) could not be measured)", s.Unmeasured)
		}
		if s.Files > 1 {
			_, _ = fmt.Fprintf(stdout, "  %d files", s.Files)
		}
		_, _ = fmt.Fprintf(stdout, "\n    %s\n", wrapAt(s.Holds, 74, "    "))
		if !s.Purgeable {
			_, _ = fmt.Fprintf(stdout, "    Not removed by a retention window; deleting it is a decision.\n")
		}
		_, _ = fmt.Fprintln(stdout)
	}
	_, _ = fmt.Fprintf(stdout, "Nothing here leaves this machine. `replay purge --older-than <window>`\n"+
		"removes what a window covers; `replay purge --session <id>` removes one session.\n")
	return nil
}

// measureStore totals a file or a directory of them.
func measureStore(path string) (int64, int, int) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, 0, 0
	}
	if !info.IsDir() {
		return info.Size(), 1, 0
	}
	var total int64
	var n, unmeasured int
	_ = filepath.Walk(path, func(_ string, fi os.FileInfo, err error) error {
		// An entry that cannot be walked is not an entry known to be empty.
		// measureStore's caller prints the total as a size, so a swallow here
		// renders an unmeasurable store as "0 B" — indistinguishable from one
		// holding nothing. Counted as unmeasured so the caller can say so.
		if err != nil {
			unmeasured++
			return nil
		}
		if fi.IsDir() {
			return nil
		}
		total += fi.Size()
		n++
		return nil
	})
	return total, n, unmeasured
}
