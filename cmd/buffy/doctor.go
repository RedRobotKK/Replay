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
)

// doctorTimeout bounds the probe of a running proxy.
const doctorTimeout = 2 * time.Second

// Environment variables the doctor inspects.
const (
	envBaseURL = "ANTHROPIC_BASE_URL"
)

// runDoctor reports what Buffy can see on this machine and what to do
// next. It reads nothing but directory listings and a health endpoint.
func runDoctor(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return errUsage
	}
	p := &corpusPrinter{w: stdout}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("find home directory: %w", err)
	}

	p.printf("buffy doctor\n\n")

	// Transcripts.
	projects := filepath.Join(home, ".claude", "projects")
	files, dirs := countTranscripts(projects)
	if dirs == 0 {
		p.printf("transcripts   none found under %s\n", projects)
		p.printf("              run a Claude Code session first, or point replay at another directory\n")
	} else {
		p.printf("transcripts   %d sessions across %d projects under %s\n", files, dirs, projects)
		p.printf("              next: buffy replay %s\n", filepath.Join(projects, "<project>"))
	}

	// Proxy configuration.
	base := os.Getenv(envBaseURL)
	if base == "" {
		p.printf("proxy         %s is not set in this shell; the agent talks to the provider directly\n", envBaseURL)
		p.printf("              next: buffy serve, then export %s=http://%s\n", envBaseURL, defaultListen)
	} else {
		p.printf("proxy         %s=%s\n", envBaseURL, base)
		if ok, detail := probeProxy(base); ok {
			p.printf("              buffy is answering there (%s)\n", detail)
		} else {
			p.printf("              nothing answered at %s/buffy/healthz (%s); the agent will fail until buffy serve runs or the variable is unset\n", strings.TrimRight(base, "/"), detail)
		}
	}
	if os.Getenv(envDisabled) != "" {
		p.printf("              %s is set: serve will refuse to start\n", envDisabled)
	}

	// Ledger.
	ledgerDir := filepath.Join(home, ".buffy", ledgerDirName)
	n := countFiles(ledgerDir, "*.jsonl")
	if n == 0 {
		p.printf("ledger        empty (%s)\n", ledgerDir)
	} else {
		p.printf("ledger        %d sessions recorded under %s\n", n, ledgerDir)
		p.printf("              next: buffy replay %s  (measured tier)\n", ledgerDir)
	}
	return p.err
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+"/buffy/healthz", nil)
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
	return false, fmt.Sprintf("status %d; something other than buffy is listening", resp.StatusCode)
}
