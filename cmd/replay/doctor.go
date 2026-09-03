package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
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
	if err := fs.Parse(args); err != nil {
		return errUsage
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
	projects := filepath.Join(home, ".claude", "projects")
	files, dirs := countTranscripts(projects)
	if dirs == 0 {
		p.Printf("transcripts   none found under %s\n", projects)
		p.Printf("              run a Claude Code session first, or point replay at another directory\n")
	} else {
		p.Printf("transcripts   %d sessions across %d projects under %s\n", files, dirs, projects)
		p.Printf("              next: replay %s\n", filepath.Join(projects, "<project>"))
	}

	// Proxy configuration.
	base := os.Getenv(envBaseURL)
	if base == "" {
		p.Printf("proxy         %s is not set in this shell; the agent talks to the provider directly\n", envBaseURL)
		p.Printf("              to record live turns: run 'replay serve', then in the agent's own shell\n")
		p.Printf("              export %s=http://%s\n", envBaseURL, defaultListen)
	} else {
		p.Printf("proxy         %s=%s\n", envBaseURL, base)
		switch state, detail := probeProxy(base); state {
		case proxyHealthy:
			p.Printf("              replay is answering there (%s); turns through this shell are recorded\n", detail)
		case proxyForeign:
			// Another gateway, or the provider itself. The agent works;
			// it is only replay that sees nothing.
			p.Printf("              something other than replay answers there (%s)\n", detail)
			p.Printf("              the agent works, but replay records nothing; to record live turns run\n")
			p.Printf("              replay serve --upstream %s, then point %s at http://%s\n", base, envBaseURL, defaultListen)
		case proxyDown:
			p.Printf("              nothing is listening there (%s); the agent will fail until 'replay serve' runs or %s is unset\n", detail, envBaseURL)
		}
	}
	if os.Getenv(envDisabled) != "" {
		p.Printf("              %s is set: serve will refuse to start\n", envDisabled)
	}

	// Ledger.
	n := countFiles(ledgerDir, "*.jsonl")
	if n == 0 {
		p.Printf("ledger        empty (%s)\n", ledgerDir)
	} else {
		p.Printf("ledger        %d sessions recorded under %s\n", n, ledgerDir)
		p.Printf("              next: replay %s  (measured tier)\n", ledgerDir)
	}
	return p.Err()
}

func countTranscripts(projects string) (files int, dirs int) {
	entries, err := os.ReadDir(projects)
	if err != nil {
		return 0, 0
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if n := countFiles(filepath.Join(projects, e.Name()), "*.jsonl"); n > 0 {
			dirs++
			files += n
		}
	}
	return files, dirs
}

func countFiles(dir, pattern string) int {
	matches, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return 0
	}
	return len(matches)
}

// proxyState is what answered at the configured base URL. The three are
// meaningfully different: only nothing listening is a failure for the
// agent, while another gateway answering is a failure only for recording.
type proxyState int

const (
	proxyDown proxyState = iota
	proxyForeign
	proxyHealthy
)

// probeProxy asks the health endpoint and reports what answered.
func probeProxy(base string) (proxyState, string) {
	ctx, cancel := context.WithTimeout(context.Background(), doctorTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+proxy.HealthPath, nil)
	if err != nil {
		return proxyDown, err.Error()
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return proxyDown, "connection failed"
	}
	defer resp.Body.Close() //nolint:errcheck // probe only
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64))
	if resp.StatusCode == http.StatusOK && strings.TrimSpace(string(body)) == "ok" {
		return proxyHealthy, "healthy"
	}
	return proxyForeign, fmt.Sprintf("status %d at %s", resp.StatusCode, strings.TrimRight(base, "/")+proxy.HealthPath)
}
