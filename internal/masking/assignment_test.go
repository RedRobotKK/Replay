package masking

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func newVault(t *testing.T) *Vault {
	t.Helper()
	v, err := OpenVault(t.TempDir())
	if err != nil {
		t.Fatalf("OpenVault: %v", err)
	}
	return v
}

// body wraps text the way a client would send it: as a tool result, which is
// where a credential in an agent session actually turns up.
func body(text string) []byte {
	b, _ := json.Marshal(map[string]any{
		"model": "m",
		"messages": []any{map[string]any{
			"role":    "user",
			"content": []any{map[string]any{"type": "text", "text": text}},
		}},
	})
	return b
}

// Hex and lowercase credentials cannot be caught by shape: a 32-character
// Twilio token and a 40-character git SHA are the same string to an entropy
// test, and the SHA actually scores lower (3.63 bits against 3.91). These are
// caught by the name sitting beside them instead.
func TestCredentialAssignmentCatchesHexSecretsInContext(t *testing.T) {
	for _, tc := range []struct{ name, text, secret string }{
		{"twilio env", "TWILIO_AUTH_TOKEN=9f8b7c6d5e4a3b2c1d0e9f8a7b6c5d4e", "9f8b7c6d5e4a3b2c1d0e9f8a7b6c5d4e"},
		{"datadog yaml", "DATADOG_API_KEY: a3f9c21e8b7d4056af219c3e7b8d1f04", "a3f9c21e8b7d4056af219c3e7b8d1f04"},
		{"json field", `"apiKey": "e7c1b93a4f28d605a1b2c3d4e5f60718"`, "e7c1b93a4f28d605a1b2c3d4e5f60718"},
		{"lowercase password", `password = "correcthorsebatterystaplexyz"`, "correcthorsebatterystaplexyz"},
		{"dotted secret", "app.client_secret=Zm9vYmFyYmF6cXV4MTIzNDU2Nzg5MA", "Zm9vYmFyYmF6cXV4MTIzNDU2Nzg5MA"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, report, err := New(newVault(t), nil).Mask(body(tc.text))
			if err != nil {
				t.Fatalf("Mask: %v", err)
			}
			if report.Total() == 0 {
				t.Fatalf("no secret found in %q", tc.text)
			}
			if strings.Contains(string(out), tc.secret) {
				t.Fatalf("secret survived masking: %s", out)
			}
			if !json.Valid(out) {
				t.Fatalf("masked body must stay valid JSON: %s", out)
			}
		})
	}
}

// The reason the rule is contextual rather than shape-based: a bare hash has to
// pass through untouched, or every diff and checksum a coding agent reads gets
// corrupted. This is the regression that keeps the cure from being worse than
// the disease.
func TestBareHashesAreNeverMasked(t *testing.T) {
	b, err := os.ReadFile("testdata/entropy-negatives.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out, report, err := New(newVault(t), nil).Mask(body(line))
		if err != nil {
			t.Fatalf("Mask(%q): %v", line, err)
		}
		if report.Total() != 0 {
			t.Errorf("false positive on %q -> %s", line, out)
		}
	}
}
