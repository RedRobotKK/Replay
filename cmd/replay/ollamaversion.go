package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// ollamaVersion asks the local Ollama for its version, and returns "" when it
// cannot be read.
//
// A loopback GET rather than shelling out. os/exec appears nowhere in this
// program's production code and this is not the reason to start: the guard in
// x402_test.go allowlists it, but every path that can run a subprocess is a
// path an auditor has to read.
//
// Empty on failure is deliberate and is not a fallback value. facts.Set.Note
// treats an unreadable version as "cannot confirm" rather than as agreement,
// so a machine where Ollama is not running gets told the behaviour table is
// unconfirmed instead of being quietly told it is current.
func ollamaVersion() string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"http://127.0.0.1:11434/api/version", nil)
	if err != nil {
		return ""
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close() //nolint:errcheck // read-only; a close error tells us nothing actionable
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var v struct {
		Version string `json:"version"`
	}
	if json.NewDecoder(resp.Body).Decode(&v) != nil {
		return ""
	}
	return v.Version
}

// wrapAt breaks text to a width and indents continuation lines.
//
// The behaviour note is a paragraph, and the surfaces it prints beside are
// tables. An unwrapped paragraph in a table is the one thing that makes a
// terminal report look unmaintained.
func wrapAt(s string, width int, indent string) string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return ""
	}
	var b strings.Builder
	line := words[0]
	for _, w := range words[1:] {
		if len(line)+1+len(w) > width {
			b.WriteString(line + "\n" + indent)
			line = w
			continue
		}
		line += " " + w
	}
	b.WriteString(line)
	return b.String()
}
