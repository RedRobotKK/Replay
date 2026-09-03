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
		} else {
			p.Printf("              nothing answered at %s%s (%s); the agent will fail until replay serve runs or the variable is unset\n", strings.TrimRight(base, "/"), proxy.HealthPath, detail)
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
		p.Printf("              next: replay replay %s  (measured tier)\n", ledgerDir)
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

// probeProxy asks the health endpoint and reports what answered.
func probeProxy(base string) (bool, string) {
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
