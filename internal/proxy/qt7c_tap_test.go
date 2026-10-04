package proxy

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// QT-7c at the tap. The tap knows three things the parser cannot: that it
// dropped a body past the size cap, that a body announced as gzip did not
// open, and that a gzip stream ended early. Each already yields no usage;
// each must now also say the body was not read, and a body that parsed and
// simply carried no usage must not be marked. Written after the change, with
// the four branches hand-mutated to prove these fail without it.
func TestQT7c_TheTapMarksABodyItCouldNotReadAsUnparsed(t *testing.T) {
	newTap := func(gz bool) (*responseTap, *httptest.ResponseRecorder) {
		rec := httptest.NewRecorder()
		tap := &responseTap{ResponseWriter: rec}
		tap.Header().Set("Content-Type", "application/json")
		if gz {
			tap.Header().Set("Content-Encoding", "gzip")
		}
		tap.WriteHeader(http.StatusOK)
		return tap, rec
	}
	t.Run("dropped past the size cap", func(t *testing.T) {
		tap, _ := newTap(false)
		if _, err := tap.Write([]byte(`{"id":"m","type":"message","content":[],"usage":{"input_tokens":4242}}`)); err != nil {
			t.Fatal(err)
		}
		if _, err := tap.Write(bytes.Repeat([]byte("x"), MaxResponseBytes+1024)); err != nil {
			t.Fatal(err)
		}
		if got := tap.result(); !got.Unparsed || got.Usage != nil {
			t.Errorf("dropped body: unparsed=%v usage=%v; want unparsed and no usage", got.Unparsed, got.Usage)
		}
	})
	t.Run("announced as gzip and not gzip", func(t *testing.T) {
		tap, _ := newTap(true)
		if _, err := tap.Write([]byte("this is not gzip")); err != nil {
			t.Fatal(err)
		}
		if got := tap.result(); !got.Unparsed || got.Usage != nil {
			t.Errorf("unopenable gzip: unparsed=%v usage=%v", got.Unparsed, got.Usage)
		}
	})
	t.Run("gzip cut short", func(t *testing.T) {
		var full bytes.Buffer
		zw := gzip.NewWriter(&full)
		if _, err := zw.Write([]byte(`{"type":"message","content":[],"usage":{"input_tokens":4242}}` + strings.Repeat("x", 200000))); err != nil {
			t.Fatal(err)
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		tap, _ := newTap(true)
		if _, err := tap.Write(full.Bytes()[:full.Len()/2]); err != nil {
			t.Fatal(err)
		}
		if got := tap.result(); !got.Unparsed || got.Usage != nil {
			t.Errorf("truncated gzip: unparsed=%v usage=%v", got.Unparsed, got.Usage)
		}
	})
	t.Run("control: a message that parsed with no usage is not marked", func(t *testing.T) {
		tap, _ := newTap(false)
		if _, err := tap.Write([]byte(messageWithoutUsage)); err != nil {
			t.Fatal(err)
		}
		if got := tap.result(); got.Unparsed || got.Usage != nil || len(got.Blocks) == 0 {
			t.Errorf("parsed message without usage: unparsed=%v usage=%v blocks=%d; want parsed, no usage, blocks kept", got.Unparsed, got.Usage, len(got.Blocks))
		}
	})
}
