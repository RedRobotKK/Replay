package policy

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

const clientBody = `{"model":"claude-opus-5","max_tokens":50,"system":"be brief","messages":[{"role":"user","content":"hi"}]}`

// The client's bytes must survive untouched: removing the spliced member
// gives back exactly what was sent, and the member is the provider's
// documented shape.
func TestApplyKeepsClientBytesAndAddsOnlyTheParameter(t *testing.T) {
	p := ContextEdit{TriggerTokens: 100000, KeepLast: 6}
	for _, body := range []string{clientBody, clientBody + "\n", "  " + clientBody + "  \r\n"} {
		out, d := p.Apply([]byte(body))
		if d != Applied {
			t.Fatalf("decision = %q", d)
		}
		i := bytes.Index(out, []byte(`,"context_management":`))
		if i < 0 {
			t.Fatalf("parameter not spliced: %s", out)
		}
		end := bytes.LastIndexByte(out, '}')
		restored := string(out[:i]) + string(out[end:])
		if restored != body {
			t.Fatalf("client bytes changed:\n%s\n%s", body, restored)
		}
		var parsed struct {
			ContextManagement struct {
				Edits []struct {
					Type    string `json:"type"`
					Trigger struct {
						Type  string `json:"type"`
						Value int    `json:"value"`
					} `json:"trigger"`
					Keep struct {
						Value int `json:"value"`
					} `json:"keep"`
					ClearAtLeast struct {
						Value int `json:"value"`
					} `json:"clear_at_least"`
				} `json:"edits"`
			} `json:"context_management"`
		}
		if err := json.Unmarshal(out, &parsed); err != nil {
			t.Fatal(err)
		}
		e := parsed.ContextManagement.Edits
		if len(e) != 1 || e[0].Type != editType || e[0].Trigger.Value != 100000 || e[0].Keep.Value != 6 || e[0].ClearAtLeast.Value != 25000 {
			t.Fatalf("parameter wrong: %+v", e)
		}
		again, _ := p.Apply([]byte(body))
		if !bytes.Equal(out, again) {
			t.Fatal("apply must be deterministic")
		}
	}
}

func TestApplyLeavesNonObjectsAlone(t *testing.T) {
	p := ContextEdit{TriggerTokens: 1000, KeepLast: 1}
	for _, body := range []string{"", "[]", "not json", "{"} {
		out, d := p.Apply([]byte(body))
		if d == Applied || string(out) != body {
			t.Fatalf("%q: decision %q out %q", body, d, out)
		}
	}
	out, d := p.Apply([]byte("{}"))
	if d != Applied || !json.Valid(out) || strings.HasPrefix(string(out), "{,") {
		t.Fatalf("empty object: %q %q", d, out)
	}
}

func TestAdmissibleNeedsBetaAndNoClientParameter(t *testing.T) {
	p := ContextEdit{TriggerTokens: 1000, KeepLast: 1}
	cases := []struct {
		beta      string
		clientSet bool
		want      Decision
	}{
		{"", false, SkipNoBeta},
		{"fast-mode-2026-02-01", false, SkipNoBeta},
		{"fast-mode-2026-02-01, " + BetaFeature, false, Applied},
		{BetaFeature, true, SkipClientSet},
	}
	for _, c := range cases {
		if got := p.Admissible(c.beta, c.clientSet); got != c.want {
			t.Errorf("beta=%q clientSet=%v: got %q want %q", c.beta, c.clientSet, got, c.want)
		}
	}
}

func TestValidate(t *testing.T) {
	if err := (ContextEdit{TriggerTokens: 0, KeepLast: 6}).Validate(); err == nil {
		t.Fatal("zero trigger must be rejected")
	}
	if err := (ContextEdit{TriggerTokens: 10, KeepLast: -1}).Validate(); err == nil {
		t.Fatal("negative keep must be rejected")
	}
	if err := (ContextEdit{TriggerTokens: 10, KeepLast: 0}).Validate(); err != nil {
		t.Fatal(err)
	}
	if s := (ContextEdit{TriggerTokens: 200000, KeepLast: 6}).String(); s != "context-edit(keep=6,trigger=200000)" {
		t.Fatalf("String = %q", s)
	}
}
