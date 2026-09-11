package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/RedRobotKK/Replay/internal/selfupdate"
	"github.com/RedRobotKK/Replay/internal/version"
)

// runUpgrade replaces this binary with a newer release.
//
// It exists because there was no way to learn a newer release had shipped. The
// only upgrade path was re-pasting the install line from the README, so a
// machine sat on whatever build it happened to get: this one ran 0.4.0 for
// three days while 0.5.4 was published, and the upgrade happened only because
// somebody re-read the install instructions by chance.
//
// The command reaches the network. That is allowed here and only here, because
// the user typed it — the promise in the README is that replay originates no
// request you did not type, not that it never sends one. `internal/selfupdate`
// is named in cmd/replay/outbound_drift_test.go's inventory and in
// docs/SURFACES.md §2 for that reason.
func runUpgrade(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("upgrade", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		check  = fs.Bool("check", false, "report what is available and exit; change nothing")
		dryRun = fs.Bool("dry-run", false, "download and verify, but do not replace the binary")
		tag    = fs.String("version", "", "install a specific release tag instead of the latest")
	)
	fs.Usage = func() {
		_, _ = fmt.Fprint(stderr, "Usage of upgrade:\n"+
			"  replay upgrade              install the newest release over this binary\n"+
			"  replay upgrade --check      say what is available, change nothing\n"+
			"  replay upgrade --dry-run    download and verify, then stop\n"+
			"  replay upgrade --version v0.5.4\n\n"+
			"Resolves the latest tag from the releases/latest redirect, downloads the\n"+
			"archive for this platform, and refuses to install anything whose sha256\n"+
			"does not match the published checksums. The replacement is staged beside\n"+
			"the current binary and run once before it is moved into place, so a build\n"+
			"that cannot execute here leaves your working copy untouched.\n\n"+
			"This is the only command besides `rules --check-prices` and `probe\n"+
			"--execute` that reaches the network, and like them it does so only when\n"+
			"you type it. Nothing is sent: every request is a GET.\n")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return errHelpShown
		}
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	client := selfupdate.NewClient()
	current := version.Version

	target := *tag
	if target == "" {
		_, _ = fmt.Fprintln(stderr, "→ Resolving the latest release")
		var err error
		target, err = client.Latest(ctx)
		if err != nil {
			return fmt.Errorf("could not resolve the latest release: %w", err)
		}
	}

	_, _ = fmt.Fprintf(stdout, "installed  %s\nlatest     %s\n", version.String(), target)

	switch {
	case current == "dev":
		// A source build has no release to compare against, and overwriting the
		// binary someone is actively building is the wrong default. Say so and
		// stop rather than guessing.
		_, _ = fmt.Fprintf(stdout,
			"\nThis is a source build, so there is no release to compare it against.\n"+
				"Pass --version %s to install that release over it.\n", target)
		if *tag == "" {
			return nil
		}
	case !selfupdate.Newer(current, target) && *tag == "":
		_, _ = fmt.Fprintf(stdout, "\nAlready current. Nothing to do.\n")
		return nil
	}

	if *check {
		_, _ = fmt.Fprintf(stdout, "\n%s is available. Run `replay upgrade` to install it.\n", target)
		return nil
	}

	// Where does this binary actually live? Not "where would the installer put
	// one": the answer has to be the file that is running, or an upgrade
	// silently installs beside the copy the user actually invokes.
	dest, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not locate the running binary: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(dest); err == nil {
		dest = resolved
	}

	_, _ = fmt.Fprintf(stderr, "→ Downloading %s %s\n", selfupdate.Binary, target)
	bin, err := client.Fetch(ctx, target, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintln(stderr, "✓ Checksum verified")

	if *dryRun {
		_, _ = fmt.Fprintf(stdout, "\nVerified %s for %s/%s (%d bytes). Dry run: %s was not touched.\n",
			target, runtime.GOOS, runtime.GOARCH, len(bin), dest)
		return nil
	}

	if err := selfupdate.Apply(bin, dest); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "\n✓ Upgraded to %s at %s\n", target, dest)
	return nil
}

// The staleness hint that used to sit here was removed rather than carried
// forward, and the note left in its place said the decision about where it
// belonged should be made deliberately rather than inherited from a commit that
// arrived by accident. That decision has now been made: cmd/replay/doctorbuild.go.
//
// It is not under every report. Printing a staleness line unprompted is what
// the note was refusing, and the refusal still stands: README's Footprint
// section promises one ask at most once every thirty days, and a line a reader
// meets on every run is one they learn to skip. `replay doctor` is a command an
// operator types to ask what is wrong here, so the notice is answering rather
// than volunteering.
//
// What that does not fix: an operator who never runs `doctor` still learns
// nothing, and nothing here reaches the network, so a build inside the window
// is reported as current whether or not a newer release exists. This says how
// old your binary is, not what the newest one is.
