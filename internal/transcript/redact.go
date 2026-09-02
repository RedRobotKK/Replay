package transcript

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path"
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
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 1<<20), maxLineBytes)
	bw := bufio.NewWriter(w)
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
		redactLine(obj)
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

func redactLine(obj map[string]any) {
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
		msg["content"] = filler(len(c))
	case []any:
		for _, item := range c {
			if b, ok := item.(map[string]any); ok {
				redactBlock(b)
			}
		}
	}
}

func redactBlock(b map[string]any) {
	for k, v := range b {
		switch k {
		case "type", "id", "tool_use_id", "is_error", "name":
			// structural; keep
		case "input":
			b[k] = redactInput(v)
		case "content":
			b[k] = redactContent(v)
		default:
			if s, ok := v.(string); ok {
				b[k] = filler(len(s))
			} else {
				delete(b, k)
			}
		}
	}
}

// Argument names whose values are paths or commands; kept as hashed labels
// so tool results still attribute to a stable, meaningless name.
var labelArgs = map[string]bool{"file_path": true, "path": true, "pattern": true, "command": true, "url": true, "query": true}

func redactInput(v any) any {
	m, ok := v.(map[string]any)
	if !ok {
		return map[string]any{}
	}
	out := make(map[string]any, len(m))
	for k, val := range m {
		s, isString := val.(string)
		switch {
		case !isString:
			out[k] = filler(len(fmt.Sprint(val)))
		case labelArgs[k]:
			out[k] = hashedLabel(s)
		default:
			out[k] = filler(len(s))
		}
	}
	return out
}

func redactContent(v any) any {
	switch c := v.(type) {
	case string:
		return filler(len(c))
	case []any:
		for _, item := range c {
			if b, ok := item.(map[string]any); ok {
				redactBlock(b)
			}
		}
		return c
	default:
		return nil
	}
}

// hashedLabel keeps the extension of a path so per-language patterns stay
// visible, and replaces the rest with a short hash.
func hashedLabel(s string) string {
	sum := sha256.Sum256([]byte(s))
	ext := path.Ext(s)
	if len(ext) > 8 || strings.ContainsAny(ext, " /") {
		ext = ""
	}
	return "r/" + hex.EncodeToString(sum[:6]) + ext
}

// filler returns a string of the same byte length as the original so size
// based analysis is unchanged. Wire bytes of real content are UTF-8; the
// filler is ASCII, which is what the analysis measures anyway.
func filler(n int) string {
	return strings.Repeat("x", n)
}
