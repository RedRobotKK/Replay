//go:build mutation

package blackbox

// The first screen of `replay serve`, read from the shipped binary. A user
// who runs an OpenAI-compatible client is told how to point it at the proxy,
// and a user who runs Codex CLI is told, before any traffic flows, that the
// Responses path is read and what is not measured on it. Written RED against
// the v0.7.0 banner, which named ANTHROPIC_BASE_URL and nothing else; the
// Responses clause flipped with R-1, when the path became readable.

import (
	"bytes"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func serveBannerFromBinary(t *testing.T, bin string) string {
	t.Helper()
	home := t.TempDir()
	port := freePort(t)
	addr := "127.0.0.1:" + itoa(port)
	cmd := exec.Command(bin, "serve", "--listen", addr, "--upstream", "https://api.anthropic.com", "--ledger", filepath.Join(home, ".replay", "ledger"))
	cmd.Env = append(baseEnv(), "HOME="+home, "USERPROFILE="+home, "CLAUDE_CONFIG_DIR=")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = cmd.Process.Signal(os.Interrupt)
		done := make(chan struct{})
		go func() { _ = cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
		}
	}()
	for i := 0; i < 100; i++ {
		if resp, err := http.Get("http://" + addr + "/replay/healthz"); err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	// The banner is printed by a goroutine once the listener is up; give it
	// a moment after healthz answers.
	for i := 0; i < 40 && !strings.Contains(out.String(), "Stop with Ctrl-C"); i++ {
		time.Sleep(50 * time.Millisecond)
	}
	return out.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func TestBB_ServeBannerNamesEveryClientAndEveryReadPath(t *testing.T) {
	bin := productionBinary(t)
	banner := serveBannerFromBinary(t, bin.Path)
	for _, want := range []string{
		"ANTHROPIC_BASE_URL=http://",
		"OPENAI_BASE_URL=http://",
		"OPENAI_API_BASE=http://",
		"/v1/messages",
		"/v1/chat/completions",
		"/v1/responses",
		"not measured",
	} {
		if !strings.Contains(banner, want) {
			t.Errorf("the serve banner does not say %q:\n%s", want, banner)
		}
	}
	for _, forbidden := range []string{"forwarded unread", "no ledger"} {
		if strings.Contains(banner, forbidden) {
			t.Errorf("the banner claims %q, which stopped being true with R-1", forbidden)
		}
	}
}
