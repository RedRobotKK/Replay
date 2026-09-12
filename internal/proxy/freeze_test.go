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
	if toolsWireHash([]byte(`{"model":"x"}`)) != "" {
		t.Fatal("a body with no tools key is unlabelled, not a hash of absence")
	}
	if toolsWireHash([]byte(`not-json`)) != "" {
		t.Fatal("unreadable body is unlabelled")
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
	if strings.Contains(string(out), "2.1.88") {
		t.Fatalf("hash remnant still present: %s", out)
	}
	if !strings.Contains(string(out), "cc_version=frozen") {
		t.Fatalf("frozen marker missing: %s", out)
	}
	// The match is longer than frozenVersion, so the replacement is
	// space-padded. Without the pad, copy leaves the tail of the old hash.
	want := "cc_version=frozen    " // 21 bytes, same as cc_version=2.1.88.a3f
	if !strings.Contains(string(out), want) {
		t.Fatalf("replacement was not padded to the match length:\n%s", out)
	}
}

func TestFreezeBillingHeader_RefusesALengtheningPin(t *testing.T) {
	in := []byte(`{"system":"cc_version=x; rest"}`)
	out, ok := freezeBillingHeader(in)
	if ok {
		t.Fatal("a match shorter than the pin must be refused, not lengthened")
	}
	if !bytes.Equal(out, in) {
		t.Fatal("a refused pin must leave the body untouched")
	}
}

func TestFreezeBillingHeader_NoMatchIsNotAPin(t *testing.T) {
	in := []byte(`{"system":"hello"}`)
	out, ok := freezeBillingHeader(in)
	if ok {
		t.Fatal("no cc_version is not a pin")
	}
	if !bytes.Equal(out, in) {
		t.Fatal("no match must leave the body untouched")
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
	if recs[0].Policy != "freeze-prefix" {
		t.Fatalf("ledger policy = %q, want freeze-prefix", recs[0].Policy)
	}
}
