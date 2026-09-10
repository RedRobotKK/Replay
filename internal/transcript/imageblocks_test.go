package transcript

import (
	"encoding/json"
	"strings"
	"testing"
)

// An Anthropic image measures its payload, not zero.
//
// RawBlock has no `source` field, and DecodeBlock's image arm reads `content`.
// Anthropic puts an image under `source`, so every image in a Claude Code
// transcript decoded to Bytes: 0 — content the provider billed for, recorded
// here as weighing nothing.
//
// Its effect is not confined to the image. Bytes feeds the byte-to-token fit,
// and a block contributing zero bytes and a real share of tokens drags the
// ratio down for everything else in the session. So this is not a cosmetic
// undercount in one label; it is a distortion of every estimated figure in a
// session that contained an image.
//
// The audit that found this left it alone, on the grounds that changing Bytes
// changes the population the fit was calibrated on. That caution was right to
// have and turns out not to bind, which is worth recording because the reason
// is not obvious:
//
//	fit before   user-content fit 0.715 tokens/byte ±87% from 217 turns
//	fit after    user-content fit 0.715 tokens/byte ±87% from 217 turns
//
// Byte-identical on a real 10,141-request session carrying 675,873 bytes of
// base64 in its main lane. Fit samples user-content turns, and an image does
// not enter that sample, so the ratio is untouched.
//
// It matters that it does not, because an image would poison it. A provider
// bills an image by its dimensions, not by the length of its base64: roughly
// 1,600 tokens for 675KB, a ratio of 0.002 against prose at 0.715. One image in
// the sample would drag the fitted ratio toward zero for every other block in
// the session — the "denser than prose" problem from ADR-0018, running the
// other way and far harder.
//
// So this fixes what Bytes reports and deliberately does not feed it anywhere
// new. See BI4.
//
// Base64 is measured as the encoded bytes, which is what was sent. The decoded
// image is smaller and irrelevant: the provider was handed the encoding, and
// this is a measure of what crossed the wire.

func imageBlock(t *testing.T, raw string) Block {
	t.Helper()
	var rb RawBlock
	if err := json.Unmarshal([]byte(raw), &rb); err != nil {
		t.Fatalf("fixture is not JSON: %v", err)
	}
	return DecodeBlock(rb, RoleUser, nil, nil)
}

// BI1: an Anthropic image carries its payload's size.
//
// PASS: Bytes tracks the base64 data.
// FAIL: zero, which is what shipped — the provider billed for it and the
// transcript recorded it as empty.
func TestBI1_AnAnthropicImageIsNotZeroBytes(t *testing.T) {
	payload := strings.Repeat("A", 2000)
	b := imageBlock(t, `{"type":"image","source":{"type":"base64","media_type":"image/png","data":"`+payload+`"}}`)
	if b.Bytes == 0 {
		t.Fatal("an Anthropic image decodes to zero bytes. The provider billed for it; the " +
			"fit is told it weighed nothing, and every estimate in the session moves.")
	}
	if b.Bytes < len(payload) {
		t.Errorf("the image measures %d bytes against a %d-byte payload", b.Bytes, len(payload))
	}
}

// BI2: the OpenAI-shaped image still measures.
//
// The arm already handled `content`. Fixing `source` must not cost the shape
// that worked, or this trades one silent zero for another.
func TestBI2_TheContentShapedImageStillMeasures(t *testing.T) {
	b := imageBlock(t, `{"type":"image","content":"`+strings.Repeat("B", 500)+`"}`)
	if b.Bytes < 500 {
		t.Errorf("a content-shaped image measures %d bytes, was 500+", b.Bytes)
	}
}

// BI3: a document under source measures too.
//
// The same arm serves both, and a PDF handed to a model is the case most likely
// to be large.
func TestBI3_ADocumentUnderSourceMeasures(t *testing.T) {
	b := imageBlock(t, `{"type":"document","source":{"type":"base64","media_type":"application/pdf","data":"`+strings.Repeat("C", 4000)+`"}}`)
	if b.Bytes < 4000 {
		t.Errorf("a document under source measures %d bytes, was 4000+", b.Bytes)
	}
}

// BI4: an image with neither shape is zero, and that is honest.
//
// A url-referenced image carries no payload in the transcript. Zero is the
// right answer there — the bytes genuinely are not present — and this pins the
// distinction so a future change cannot make one stand for the other.
func TestBI4_AnImageWithNoPayloadIsHonestlyZero(t *testing.T) {
	b := imageBlock(t, `{"type":"image","source":{"type":"url","url":"https://example.com/x.png"}}`)
	if b.Bytes > 200 {
		t.Errorf("a url-referenced image measures %d bytes; the payload is not in the "+
			"transcript, so there is nothing here to measure", b.Bytes)
	}
}
