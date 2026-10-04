package main

import (
	"strings"
	"testing"
)

// The first screen of replay serve names every client variable and says
// which request shapes are read and which are not. The black-box test in
// internal/blackbox reads the same lines from the shipped binary; this one is
// the registered killer for the frozen mutant on the banner.
func TestServeBannerNamesEveryClientAndWhatItCannotRead(t *testing.T) {
	b := serveBanner("127.0.0.1:4000", "https://api.anthropic.com", "/home/x/.replay/ledger")
	for _, want := range []string{
		"export ANTHROPIC_BASE_URL=http://127.0.0.1:4000",
		"export OPENAI_BASE_URL=http://127.0.0.1:4000/v1",
		"export OPENAI_API_BASE=http://127.0.0.1:4000/v1",
		"/v1/messages", "/v1/chat/completions",
		"/v1/responses", "forwarded unread",
		"replay replay /home/x/.replay/ledger",
	} {
		if !strings.Contains(b, want) {
			t.Errorf("banner lacks %q:\n%s", want, b)
		}
	}
	for _, forbidden := range []string{"reads /v1/responses", "Responses API is supported"} {
		if strings.Contains(b, forbidden) {
			t.Errorf("banner claims %q", forbidden)
		}
	}
}
