package transcript

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
)

// Redact rewrites a Claude Code transcript so it keeps everything the
// analysis needs (structure, block kinds, byte lengths, usage, timestamps,
// ids, tool names) and nothing anyone could read: text becomes filler of
// the same length, file paths become hashes that keep their extension, and
// machine-specific fields are replaced.
//
// The output is what a user can attach to a bug report, and what this
// repository uses as test fixtures.
func Redact(r io.Reader, w io.Writer) error {
	salt := make([]byte, saltBytes)
	if _, err := rand.Read(salt); err != nil {
		return fmt.Errorf("generate redaction salt: %w", err)
	}
	red := &redactor{salt: salt}
	scanner := NewLineScanner(r)
	bw := newBufferedWriter(w)
	for scanner.Scan() {
		raw := bytes.TrimSpace(scanner.Bytes())
		if len(raw) == 0 {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			// Unparseable lines carry unknown content; drop them.
			continue
		}
		if _, ok := obj["uuid"]; !ok {
			continue
		}
		red.line(obj)
		out, err := json.Marshal(obj)
		if err != nil {
			return fmt.Errorf("encode redacted line: %w", err)
		}
		if _, err := bw.Write(out); err != nil {
			return err
		}
		if err := bw.WriteByte('\n'); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read transcript: %w", err)
	}
	return bw.Flush()
}

// Top-level fields kept verbatim; everything else at the top level is
// dropped unless listed in replaced.
var keptTop = map[string]bool{
	"type": true, "uuid": true, "parentUuid": true, "timestamp": true,
	"requestId": true, "apiBlockIndex": true, "isSidechain": true,
	"effort": true, "message": true, "version": true, "userType": true,
}

// saltBytes sizes the per-file salt that keys every filler string.
const saltBytes = 16

// redactor carries the per-file salt. Filler is a keyed hash of the
// original content expanded to the same length, so equal values stay equal
// within one redacted file (repeated tool calls remain detectable), distinct
// values stay distinct, and nothing can be confirmed against the output
// without the salt, which is never written anywhere.
type redactor struct {
	salt []byte
}

func (rd *redactor) line(obj map[string]any) {
	for k := range obj {
		if !keptTop[k] {
			delete(obj, k)
		}
	}
	obj["sessionId"] = "redacted"
	obj["cwd"] = "/redacted"
	if t, _ := obj["type"].(string); t != lineTypeUser && t != lineTypeAssistant {
		delete(obj, "message")
		return
	}
	msg, ok := obj["message"].(map[string]any)
	if !ok {
		return
	}
	for k := range msg {
		switch k {
		case "role", "model", "usage", "content", "stop_reason", "type":
		default:
			delete(msg, k)
		}
	}
	switch c := msg["content"].(type) {
	case string:
		msg["content"] = rd.filler(c)
	case []any:
		for _, item := range c {
			if b, ok := item.(map[string]any); ok {
				rd.block(b)
			}
		}
	}
}

func (rd *redactor) block(b map[string]any) {
	for k, v := range b {
		switch k {
		case "type", "id", "tool_use_id", "is_error", "name":
			// structural; keep
		case "input":
			b[k] = rd.input(v)
		case "content":
			b[k] = rd.content(v)
		default:
			if s, ok := v.(string); ok {
				b[k] = rd.filler(s)
			} else {
				delete(b, k)
			}
		}
	}
}

// redactInput rewrites a tool call's arguments. Strings become filler or a
// hashed label of the same length; numbers, booleans, and null are kept
// because they are not content and their JSON length must not change;
// nested values are handled the same way recursively.
func (rd *redactor) input(v any) any {
	switch val := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, inner := range val {
			// Label arguments become a hash of the same length so tool
			// results still attribute to a stable, meaningless name; only
			// true file paths keep their extension.
			if s, ok := inner.(string); ok && slices.Contains(LabelArgs, k) {
				out[k] = rd.hashedLabel(s, slices.Contains(PathArgs, k))
				continue
			}
			out[k] = rd.input(inner)
		}
		return out
	case []any:
		for i := range val {
			val[i] = rd.input(val[i])
		}
		return val
	case string:
		return rd.filler(val)
	default:
		return val
	}
}

func (rd *redactor) content(v any) any {
	switch c := v.(type) {
	case string:
		return rd.filler(c)
	case []any:
		for _, item := range c {
			if b, ok := item.(map[string]any); ok {
				rd.block(b)
			}
		}
		return c
	default:
		return nil
	}
}

// hashedLabel replaces a label with a hash of the same byte length. For file
// paths the extension survives so per-language patterns stay visible; for
// commands, queries, and URLs nothing after a dot is content-free, so
// nothing is kept.
func (rd *redactor) hashedLabel(s string, keepExtension bool) string {
	digest := rd.digest(s)
	label, ext := "r/"+digest[:HashedLabelBytes], ""
	if keepExtension {
		label = HashedPathLabel(digest, s)
		ext = label[len("r/")+HashedLabelBytes:]
		label = label[:len("r/")+HashedLabelBytes]
	}
	room := len(s) - len(ext)
	switch {
	case room <= 0:
		return rd.filler(s)
	case len(label) > room:
		return label[:room] + ext
	default:
		return label + rd.filler(s)[:room-len(label)] + ext
	}
}

// digest returns the salted hash of s as hex, long enough to fill any
// label or string this file contains.
func (rd *redactor) digest(s string) string {
	h := sha256.New()
	h.Write(rd.salt)
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

// newBufferedWriter wraps a writer for line output.
func newBufferedWriter(w io.Writer) *bufio.Writer {
	return bufio.NewWriter(w)
}

// filler returns a string of the same byte length as the original, derived
// from its salted hash, so size-based analysis and equality are unchanged.
// Real content is UTF-8; the filler is ASCII, which is what the analysis
// measures anyway.
func (rd *redactor) filler(s string) string {
	n := len(s)
	if n == 0 {
		return ""
	}
	seed := rd.digest(s)
	var sb strings.Builder
	sb.Grow(n)
	for sb.Len() < n {
		sb.WriteString(seed)
	}
	return sb.String()[:n]
}
