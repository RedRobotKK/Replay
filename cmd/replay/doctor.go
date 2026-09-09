package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/RedRobotKK/Replay/internal/analysis"
	"github.com/RedRobotKK/Replay/internal/proxy"
)

// doctorTimeout bounds the probe of a running proxy.
const doctorTimeout = 2 * time.Second

// Environment variables the doctor inspects.
const (
	envBaseURL = "ANTHROPIC_BASE_URL"
)

// runDoctor reports what Replay can see on this machine and what to do
// next. It reads nothing but directory listings and a health endpoint.
func runDoctor(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := parseArgs(fs, args, stdout); err != nil {
		return err
	}
	p := analysis.NewPrinter(stdout)
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("find home directory: %w", err)
	}
	ledgerDir, err := defaultLedgerDir()
	if err != nil {
		return err
	}

	p.Printf("replay doctor\n\n")

	// Transcripts.
	projects := filepath.Join(claudeConfigDir(home), "projects")
	c := countTranscripts(projects)
	if c.projects == 0 {
		p.Printf("transcripts   none found under %s\n", projects)
		p.Printf("              run a Claude Code session first, or point replay at another directory\n")
	} else {
		p.Printf("transcripts   %d sessions across %d projects under %s\n", c.sessions, c.projects, projects)
		// Printed only when the two figures differ. "3 transcript files" beside
		// "3 sessions" is a distinction where there is none, which misleads the
		// same way as hiding one that exists.
		if c.lanes > 0 {
			p.Printf("              %d transcript files in all: a session writes one per agent lane, so\n", c.sessions+c.lanes)
			p.Printf("              replay cost over this directory reports on every one of them\n")
		}
		p.Printf("              next: replay replay %s\n", filepath.Join(projects, "<project>"))
	}

	// Proxy configuration.
	base := os.Getenv(envBaseURL)
	if base == "" {
		p.Printf("proxy         %s is not set in this shell; the agent talks to the provider directly\n", envBaseURL)
		p.Printf("              next: replay serve, then export %s=http://%s\n", envBaseURL, defaultListen)
	} else {
		p.Printf("proxy         %s=%s\n", envBaseURL, base)
		if ok, detail := probeProxy(base); ok {
			p.Printf("              replay is answering there (%s)\n", detail)
			if st, ok := probeStatus(base); ok {
				for i, l := range guardLines(st) {
					if i == 0 {
						p.Printf("guards        %s\n", l)
						continue
					}
					p.Printf("              %s\n", l)
				}
			}
		} else {
			p.Printf("              nothing answered at %s%s (%s); the agent will fail until replay serve runs or the variable is unset\n", strings.TrimRight(base, "/"), proxy.HealthPath, detail)
		}
	}
	if os.Getenv(envDisabled) != "" {
		p.Printf("              %s is set: serve will refuse to start\n", envDisabled)
	}
	// REPLAY_UPSTREAM redirects every request the proxy forwards, and it is
	// read as a flag default, so it does not show up as an override anywhere
	// either. The one command whose job is to say what Replay can see here has
	// to be able to see a hijacked upstream.
	if up := strings.TrimSpace(os.Getenv(envUpstream)); up != "" {
		p.Printf("              %s is set: every forwarded request goes to %s\n", envUpstream, up)
		if !strings.HasPrefix(up, "https://") {
			p.Printf("              WARNING: that upstream is not https, so your provider credential would travel in clear text\n")
		}
	}

	// Ledger.
	n := countFiles(ledgerDir, "*.jsonl")
	if n == 0 {
		p.Printf("ledger        empty (%s)\n", ledgerDir)
	} else {
		p.Printf("ledger        %d sessions recorded under %s\n", n, ledgerDir)
		p.Printf("              next: replay replay %s  (measured tier)\n", ledgerDir)
	}
	return p.Err()
}

// transcriptCounts splits a projects directory into the two things the word
// "transcript" was covering at once.
//
// Claude Code writes one transcript per session at
// <project>/<sessionId>.jsonl, and one more per sub-agent lane nested beneath
// it at <project>/<sessionId>/subagents/agent-*.jsonl. On the machine that
// produced the contradiction this exists to fix, that was 91 sessions and 1403
// lanes — so `doctor` reported 90 and `cost` reported 1477 over the same
// directory a second apart, and both were right about different things.
//
// doctor happened to print the session count only because its glob was never
// recursive. That is an accident, and the number it produced described a fifth
// of one percent of what the next command would read.
type transcriptCounts struct {
	sessions int // main-lane transcripts: one per session
	lanes    int // sub-agent lane transcripts, nested under the session that spawned them
	projects int
}

// countTranscripts reports both counts for a projects directory.
//
// The walk mirrors transcriptFiles deliberately, dot directories included: the
// file total is a promise about how much the next command will read, and a
// promise derived by a second, subtly different walk is exactly how these two
// numbers drifted apart. The test asserts the two walks agree rather than
// trusting them to.
func countTranscripts(projects string) transcriptCounts {
	var c transcriptCounts
	entries, err := os.ReadDir(projects)
	if err != nil {
		return c
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		dir := filepath.Join(projects, e.Name())
		sessions := countFiles(dir, "*.jsonl")
		lanes := countNestedTranscripts(dir)
		if sessions == 0 && lanes == 0 {
			continue
		}
		c.projects++
		c.sessions += sessions
		c.lanes += lanes
	}
	return c
}

// countNestedTranscripts counts the transcripts below a project's own level,
// which is where the sub-agent lanes live.
func countNestedTranscripts(project string) int {
	n := 0
	// A walk error is not fatal. This is a directory listing behind a status
	// line, and one unreadable subdirectory should make the report undercount,
	// not make `replay doctor` the command that fails when something is wrong.
	_ = filepath.WalkDir(project, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != project && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		// A project's own children are the session transcripts; the caller
		// already has them, and counting them here would put every session in
		// both figures.
		if filepath.Dir(path) == project {
			return nil
		}
		if strings.HasSuffix(d.Name(), ".jsonl") {
			n++
		}
		return nil
	})
	return n
}

func countFiles(dir, pattern string) int {
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return 0
	}
	return len(matches)
}

// probeProxy asks the health endpoint and reports what answered.
func probeProxy(base string) (bool, string) {
	// Only loopback. This probe is a GET to whatever the environment names, so
	// without this check `ANTHROPIC_BASE_URL=http://internal.corp/admin` turns
	// the one command whose job is "what can Replay see here" into a request
	// generator pointed at somebody's network.
	if !isLoopbackURL(base) {
		return false, "not probed: only a loopback address is contacted, and this is not one"
	}
	ctx, cancel := context.WithTimeout(context.Background(), doctorTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+proxy.HealthPath, nil)
	if err != nil {
		return false, err.Error()
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, "connection failed"
	}
	defer resp.Body.Close() //nolint:errcheck // probe only
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64))
	if resp.StatusCode == http.StatusOK && strings.TrimSpace(string(body)) == "ok" {
		return true, "healthy"
	}
	return false, fmt.Sprintf("status %d; something other than replay is listening", resp.StatusCode)
}

// claudeConfigDir resolves where Claude Code keeps its data. CLAUDE_CONFIG_DIR
// relocates it, and a user who has set it and gets told "none found" concludes
// Replay does not work rather than that it looked in the wrong place.
//
// Two nearby directories are deliberately NOT this one, because both exist on a
// normal machine and neither holds transcripts:
//
//	~/.local/share/claude              the installed binary and versions
//	~/Library/Application Support/Claude   the desktop app's Electron profile
func claudeConfigDir(home string) string {
	if d := strings.TrimSpace(os.Getenv("CLAUDE_CONFIG_DIR")); d != "" {
		return d
	}
	return filepath.Join(home, ".claude")
}

// isLoopbackURL reports whether a base URL names this machine.
func isLoopbackURL(base string) bool {
	u, err := url.Parse(strings.TrimSpace(base))
	if err != nil || u.Host == "" {
		return false
	}
	host := u.Hostname()
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
