package proxy

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// The tools hash is taken from the bytes as forwarded, not from a re-encode.
//
// This is parser's Fisher objection from the kernel debate, made checkable. If
// the epoch were a hash of our PARSE of tools, there would be two readings of
// one artefact — ours and the provider's — and the label would be correct only
// while every normalisation agreed: key order, whitespace, unicode escaping,
// integer formatting, absent versus empty. This repository has shipped that
// defect twice already (gzip-vs-readable, and a struct that lost two fields in
// transit), and it fails silently both times.
//
// So: two bodies whose tools differ ONLY in whitespace must hash differently,
// because the bytes differ. That reads as a wart and it is the point — the
// label tracks what went on the wire, and nothing here claims to know how the
// provider normalises.
func TestFreeze_TheToolsHashIsTheForwardedBytesNotAParse(t *testing.T) {
	compact := []byte(`{"tools":[{"name":"a"}],"model":"m"}`)
	spaced := []byte(`{"tools":[ {"name":"a"} ],"model":"m"}`)

	// The premise: these really do parse to the same thing. If they stop doing
	// so the comparison below says nothing about parsing versus bytes.
	var a, b struct {
		Tools []map[string]any `json:"tools"`
	}
	if err := json.Unmarshal(compact, &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(spaced, &b); err != nil {
		t.Fatal(err)
	}
	if len(a.Tools) != len(b.Tools) || a.Tools[0]["name"] != b.Tools[0]["name"] {
		t.Fatal("the two fixtures no longer parse the same; this test cannot say what it claims")
	}

	if toolsWireHash(compact) == toolsWireHash(spaced) {
		t.Fatal("two different tool bytes hashed the same, so the epoch is being taken from a " +
			"parse rather than from the wire. That is two readings of one artefact, and the " +
			"label would be right only while every normalisation agreed with the provider's")
	}
}

// No tools key is no epoch, not an epoch called empty.
//
// A first request, or a sub-agent lane carrying no tools, has no tool set to
// label. Absence is not a value (ADR-0018), and downstream MixedEpochs skips
// the empty string for exactly this reason: a session mixing tool-bearing and
// tool-free requests is not a session spanning two epochs.
func TestFreeze_NoToolsKeyIsNoEpoch(t *testing.T) {
	for _, body := range []string{
		`{"model":"m"}`,
		`{"model":"m","tools":[]}`,
		`{"model":"m","tools":null}`,
	} {
		if got := toolsWireHash([]byte(body)); got != "" {
			t.Errorf("%s labelled an epoch %q; a body with no tool set has no tool-set epoch", body, got)
		}
	}
}

// A body that cannot be read is not labelled either.
func TestFreeze_AnUnreadableBodyIsNotLabelled(t *testing.T) {
	if got := toolsWireHash([]byte(`{"tools": not json`)); got != "" {
		t.Fatalf("an unreadable body was labelled %q; a label nobody can derive is worse than none", got)
	}
}

// The pin is the same LENGTH, and leaves nothing of the old hash behind.
//
// Same length because the point is that the cacheable prefix does not move: a
// replacement that shortened the body would shift every byte after it and break
// the very prefix this is protecting.
func TestFreeze_ThePinIsSameLengthAndLeavesNoRemnant(t *testing.T) {
	body := []byte(`{"system":[{"text":"env cc_version=a1b2c3d4e5f6; rest"}]}`)
	out, ok := freezeBillingHeader(body)
	if !ok {
		t.Fatal("a cc_version in the body was not pinned")
	}
	if len(out) != len(body) {
		t.Fatalf("the pin changed the body length %d -> %d, which moves every byte after it "+
			"and breaks the prefix it exists to hold still", len(body), len(out))
	}
	if bytes.Contains(out, []byte("a1b2c3d4e5f6")) {
		t.Fatalf("the old version survived the pin:\n%s", out)
	}
	if !bytes.Contains(out, []byte(frozenVersion)) {
		t.Fatalf("the constant is not in the result:\n%s", out)
	}
	// The result must still be a body, not a mangled one.
	var parsed map[string]any
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("the pin produced a body that no longer parses: %v\n%s", err, out)
	}
}

// A body with no cc_version is returned untouched, and says so.
func TestFreeze_NothingToPinChangesNothing(t *testing.T) {
	body := []byte(`{"system":[{"text":"no version here"}]}`)
	out, ok := freezeBillingHeader(body)
	if ok {
		t.Fatal("a body with no cc_version reported that it pinned one")
	}
	if !bytes.Equal(out, body) {
		t.Fatalf("a body with nothing to pin came back changed:\n%s", out)
	}
}

// A version shorter than the constant is refused rather than lengthened.
//
// Lengthening would move the bytes after it, which is the one thing this may
// not do. Refusing leaves the prefix forking on the billing header — worse for
// caching, correct for the invariant — and that trade is deliberate.
func TestFreeze_AVersionTooShortToPinIsRefusedNotLengthened(t *testing.T) {
	body := []byte(`{"system":[{"text":"cc_version=x rest"}]}`)
	out, ok := freezeBillingHeader(body)
	if ok {
		t.Fatalf("a cc_version shorter than %q was pinned, which lengthens the body and moves "+
			"the prefix this exists to hold still", frozenVersion)
	}
	if !bytes.Equal(out, body) {
		t.Fatal("a refused pin still changed the body")
	}
	if len(out) != len(body) {
		t.Fatalf("length changed %d -> %d on a refusal", len(body), len(out))
	}
}

// The pin replaces only the version, not the text around it.
func TestFreeze_OnlyTheVersionIsReplaced(t *testing.T) {
	body := []byte(`{"system":[{"text":"BEFORE cc_version=abcdefgh AFTER"}]}`)
	out, _ := freezeBillingHeader(body)
	if !bytes.Contains(out, []byte("BEFORE")) || !bytes.Contains(out, []byte("AFTER")) {
		t.Fatalf("the pin ate text around the version:\n%s", out)
	}
	if strings.Count(string(out), "cc_version=") != 1 {
		t.Fatalf("the pin duplicated or dropped the marker:\n%s", out)
	}
}
