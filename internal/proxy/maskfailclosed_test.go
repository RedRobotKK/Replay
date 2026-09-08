package proxy

import (
	"bytes"
	"errors"
	"log"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/ledger"
	"github.com/RedRobotKK/Replay/internal/masking"
)

// brokenVault refuses to store a mapping, which is what a full disk or a
// read-only vault directory produces.
type brokenVault struct{}

func (brokenVault) Placeholder(string, string) (string, error) {
	return "", errors.New("vault is unwritable")
}

// A masking failure must not put the secret on the wire.
//
// masking.Mask was fixed on 2026-09-06 to fail SECURE: when the mapping cannot
// be stored it blind-scrubs the region and returns the SAFE body alongside the
// error, so that "the stream survives, the credential does not". Server.mask
// then saw a non-nil error and returned the ORIGINAL body, discarding the
// scrubbed one, and the fix never reached anything that ships.
//
// internal/masking/failclosed_test.go could not catch this. It drives
// Masker.Mask directly, which is the layer that was already correct. This test
// drives Server.mask, which is the layer that runs.
func TestServerMaskDoesNotForwardASecretWhenTheVaultFails(t *testing.T) {
	const secret = "sk-ant-api03-ZZZZYYYYXXXXWWWWVVVVUUUUTTTTSSSS"
	body := []byte(`{"content":"my key is ` + secret + ` please continue"}`)

	var logs bytes.Buffer
	s := &Server{cfg: Config{
		Masker: masking.NewWithVault(nil, brokenVault{}),
		Logger: log.New(&logs, "", 0),
	}}

	out := s.mask(&ledger.Record{SessionID: "abcd1234"}, body)

	if bytes.Contains(out, []byte(secret)) {
		t.Fatalf("the secret was forwarded verbatim after a masking failure:\n%s", out)
	}
	if !bytes.Contains(out, []byte(masking.BlindPlaceholder)) {
		t.Errorf("the region was not blind-scrubbed; got:\n%s", out)
	}
	if !strings.Contains(logs.String(), "MASKING") {
		t.Error("a masking failure was not reported at all")
	}
}
