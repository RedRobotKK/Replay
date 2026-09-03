package masking

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The pattern set is judged on a labeled corpus: every positive must be
// found with the right name and no negative may match. The numbers are
// reported so the README can cite them.
func TestPatternCorpusPrecisionAndRecall(t *testing.T) {
	pos, err := os.ReadFile("testdata/positives.txt")
	if err != nil {
		t.Fatal(err)
	}
	neg, err := os.ReadFile("testdata/negatives.txt")
	if err != nil {
		t.Fatal(err)
	}
	found, total := 0, 0
	for _, line := range strings.Split(strings.TrimSpace(string(pos)), "\n") {
		name, text, _ := strings.Cut(line, "\t")
		total++
		ms := Find([]byte(text), Patterns)
		if len(ms) == 1 && ms[0].Pattern == name {
			found++
			continue
		}
		t.Errorf("positive %q: got %+v, want one %s match", name, ms, name)
	}
	falsePositives := 0
	negatives := strings.Split(strings.TrimSpace(string(neg)), "\n")
	for _, line := range negatives {
		if ms := Find([]byte(line), Patterns); len(ms) != 0 {
			falsePositives++
			t.Errorf("negative matched: %+v", ms)
		}
	}
	precision := float64(found) / float64(found+falsePositives)
	recall := float64(found) / float64(total)
	t.Logf("pattern corpus: precision %.2f recall %.2f (%d positives, %d negatives)", precision, recall, total, len(negatives))
	if precision < 1 || recall < 1 {
		t.Fatalf("precision %.2f recall %.2f", precision, recall)
	}
}

func TestFindDoesNotOverlapAndKeepsContext(t *testing.T) {
	text := []byte("Authorization: Bearer sk-ant-api03-AbCdEfGhIjKlMnOpQrStUvWxYz0123456789 and AKIAIOSFODNN7EXAMPLE")
	ms := Find(text, Patterns)
	if len(ms) != 2 || ms[0].Pattern != "anthropic-api-key" || ms[1].Pattern != "aws-access-key-id" {
		t.Fatalf("matches: %+v", ms)
	}
	if string(text[ms[0].Start:ms[0].End]) != "sk-ant-api03-AbCdEfGhIjKlMnOpQrStUvWxYz0123456789" {
		t.Fatalf("bearer context must stay outside the match: %q", text[ms[0].Start:ms[0].End])
	}
}

func TestUserPatterns(t *testing.T) {
	ps, err := ParseUserPatterns("# comment\n\ninternal-id\tINT-[0-9]{6}\n")
	if err != nil || len(ps) != 1 || ps[0].Name != "user:internal-id" {
		t.Fatalf("%+v %v", ps, err)
	}
	if ms := Find([]byte("ticket INT-123456 done"), ps); len(ms) != 1 {
		t.Fatalf("user pattern must match: %+v", ms)
	}
	if _, err := ParseUserPatterns("bad line without tab\n"); err == nil {
		t.Fatal("a line without a tab must be rejected")
	}
	if _, err := ParseUserPatterns("broken\t([\n"); err == nil {
		t.Fatal("an invalid expression must be rejected")
	}
}

func TestVaultIsDeterministicAndPersistent(t *testing.T) {
	dir := t.TempDir()
	v, err := OpenVault(dir)
	if err != nil {
		t.Fatal(err)
	}
	a, err := v.Placeholder("sk-ant-secret-one", "anthropic-api-key")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := v.Placeholder("sk-ant-secret-one", "anthropic-api-key")
	c, _ := v.Placeholder("sk-ant-secret-two", "anthropic-api-key")
	if a != b || a == c || !strings.HasPrefix(a, PlaceholderPrefix) || len(a) != PlaceholderLength {
		t.Fatalf("placeholders: %q %q %q", a, b, c)
	}
	// The vault on disk holds no secret in clear.
	raw, err := os.ReadFile(filepath.Join(dir, vaultFile))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("secret-one")) {
		t.Fatal("vault must be encrypted at rest")
	}
	again, err := OpenVault(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s, p, ok := again.Secret(a); !ok || s != "sk-ant-secret-one" || p != "anthropic-api-key" || again.Len() != 2 {
		t.Fatalf("vault must survive a restart: %q %v %d", s, ok, again.Len())
	}
	if d, _ := again.Placeholder("sk-ant-secret-one", "anthropic-api-key"); d != a {
		t.Fatal("placeholder must be the same after a restart")
	}
	other, err := OpenVault(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if e, _ := other.Placeholder("sk-ant-secret-one", "anthropic-api-key"); e == a {
		t.Fatal("another vault key must give another placeholder")
	}
}

const secret = "sk-ant-api03-AbCdEfGhIjKlMnOpQrStUvWxYz0123456789abcdefghij"

func TestMaskChangesOnlyTheSecretBytes(t *testing.T) {
	v, err := OpenVault(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m := New(v, nil)
	body := []byte(`{"model":"m",  "system":"key is ` + secret + ` ok","messages":[{"role":"user","content":[{"type":"text","text":"and\n` + secret + `\ttoo"},{"type":"thinking","thinking":"` + secret + `","signature":"` + secret + `"},{"type":"tool_use","id":"x","name":"Bash","input":{"command":"export K=` + secret + `"}}]}],"metadata":{"user_id":"u"}}`)
	out, rep, err := m.Mask(body)
	if err != nil {
		t.Fatal(err)
	}
	ph, _ := v.Placeholder(secret, "anthropic-api-key")
	if rep["anthropic-api-key"] != 3 || rep.Total() != 3 {
		t.Fatalf("report: %v", rep)
	}
	if !json.Valid(out) {
		t.Fatalf("masked body must stay valid JSON: %s", out)
	}
	if bytes.Count(out, []byte(secret)) != 2 {
		t.Fatalf("the thinking and signature copies must be untouched, all others replaced: %s", out)
	}
	if !bytes.Contains(out, []byte(`"thinking":"`+secret+`","signature":"`+secret+`"`)) {
		t.Fatalf("thinking block changed: %s", out)
	}
	// Putting the secrets back gives the original bytes exactly.
	restored := bytes.ReplaceAll(out, []byte(ph), []byte(secret))
	if !bytes.Equal(restored, body) {
		t.Fatalf("bytes outside secrets changed:\n%s\n%s", body, restored)
	}
	// Masking again is a no-op: placeholders are never re-masked.
	again, rep2, err := m.Mask(out)
	if err != nil || !bytes.Equal(again, out) || rep2.Total() != 0 {
		t.Fatalf("second pass must change nothing: %v %v", rep2, err)
	}
}

func TestMaskHandlesEscapesAndNonJSON(t *testing.T) {
	v, err := OpenVault(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	m := New(v, nil)
	body := []byte(`{"messages":[{"role":"user","content":"say \"` + secret + `\" é \\ done"}]}`)
	out, rep, err := m.Mask(body)
	if err != nil || rep.Total() != 1 || !json.Valid(out) || bytes.Contains(out, []byte(secret)) {
		t.Fatalf("escaped context: %v %v %s", rep, err, out)
	}
	var decoded struct {
		Messages []struct{ Content string } `json:"messages"`
	}
	if err := json.Unmarshal(out, &decoded); err != nil || !strings.Contains(decoded.Messages[0].Content, `say "`+PlaceholderPrefix) {
		t.Fatalf("decoded: %+v %v", decoded, err)
	}
	if out, _, err := m.Mask([]byte("not json")); err == nil || string(out) != "not json" {
		t.Fatal("a body that is not JSON is returned unchanged with an error")
	}
	if out, rep, err := m.Mask([]byte(`{"a":"nothing here"}`)); err != nil || rep.Total() != 0 || string(out) != `{"a":"nothing here"}` {
		t.Fatal("a body without secrets is returned as is")
	}
}
