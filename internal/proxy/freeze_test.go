package proxy

import (
	"bytes"
	"strings"
	"testing"
)

func TestToolsWireHash_IsTheForwardedBytes(t *testing.T) {
	a := []byte(`{"model":"x","tools":[{"name":"Read"}]}`)
	b := []byte(`{"model":"x","tools":[{"name":"Write"}]}`)
	if toolsWireHash(a) == "" || toolsWireHash(a) == toolsWireHash(b) {
		t.Fatal("different tools JSON must hash differently")
	}
	same := []byte(`{"model":"y","tools":[{"name":"Read"}]}`)
	if toolsWireHash(a) != toolsWireHash(same) {
		t.Fatal("the hash is of the tools value, not the whole body")
	}
}

func TestFreezeBillingHeader_SameLength(t *testing.T) {
	in := []byte(`{"system":"cc_version=2.1.88.a3f; rest"}`)
	out, ok := freezeBillingHeader(in)
	if !ok {
		t.Fatal("expected a replacement")
	}
	if len(out) != len(in) {
		t.Fatalf("length %d -> %d; a length change is a different kind of break", len(in), len(out))
	}
	if strings.Contains(string(out), "2.1.88.a3f") {
		t.Fatalf("hash still present: %s", out)
	}
	if !strings.Contains(string(out), "cc_version=frozen") {
		t.Fatalf("frozen marker missing: %s", out)
	}
}

func TestFreezePrefix_PinsHeaderAndLabelsToolsEpoch(t *testing.T) {
	up := &upstream{t: t}
	base, dir, _ := startProxyWith(t, up, Config{FreezePrefix: true})
	body := `{"model":"claude-opus-5","max_tokens":1,"system":"hello cc_version=2.1.88.a3f;","tools":[{"name":"Read"}],"messages":[{"role":"user","content":"hi"}]}`
	postWith(t, base, body, nil)
	got := up.seen().gotBody
	if bytes.Contains(got, []byte("2.1.88.a3f")) {
		t.Fatalf("cc_version hash reached the provider: %s", got)
	}
	recs := waitLedger(t, dir, 1)
	if len(recs) == 0 || recs[0].Epoch == "" {
		t.Fatal("tool-set epoch was not labelled on the record")
	}
}
