package masking

import (
	"bytes"
	"encoding/json"
	"io"
	"strconv"
)

// Stream limits. An event longer than maxPendingEvent, or a tool input
// held past MaxHeldToolInputBytes, stops rehydration for that stream or
// block and forwards the bytes unchanged: the stream must reach the
// client whatever the rehydrator makes of it.
const (
	maxPendingEvent       = 4 << 20
	MaxHeldToolInputBytes = 4 << 20
)

// StreamRehydrator restores placeholders in a server-sent event stream as
// its bytes pass through. Text deltas are rewritten as they arrive; a
// delta whose text ends in bytes that could begin a placeholder is held
// until the next delta shows whether one crosses the boundary, and goes
// out unchanged when none does. A tool call's input deltas are held until
// the block ends, because the destination (the tool's target path) can
// arrive after the placeholder; a block without a placeholder is then
// forwarded exactly as received.
type StreamRehydrator struct {
	r       *Rehydrator
	rep     RehydrationReport
	pending []byte
	blocks  map[int]*streamBlock
	// passthrough is set once the stream stops looking like events; from
	// then on every byte is forwarded as it arrives.
	passthrough bool
}

type streamBlock struct {
	kind string
	tool string
	// heldText is a text delta event whose literal ends in bytes that
	// could begin a placeholder; tail is how many. It is released
	// unchanged unless the next delta completes a placeholder across the
	// boundary.
	heldText []byte
	heldLit  literal
	tail     int
	// held are a tool block's raw delta events awaiting the block's end,
	// and input the decoded JSON text they carry.
	held      [][]byte
	heldBytes int
	input     []byte
	gaveUp    bool
}

// NewStream starts rehydration for one event stream.
func (r *Rehydrator) NewStream() *StreamRehydrator {
	return &StreamRehydrator{r: r, blocks: map[int]*streamBlock{}}
}

// Report is what the stream's rehydration did so far.
func (s *StreamRehydrator) Report() RehydrationReport { return s.rep }

// Transform takes the next bytes from the provider and returns what goes
// to the client now. Bytes of an incomplete event or a held block are
// returned by a later call or by Flush.
func (s *StreamRehydrator) Transform(p []byte) []byte {
	if s.passthrough {
		return p
	}
	s.pending = append(s.pending, p...)
	var out []byte
	for {
		end := eventEnd(s.pending)
		if end < 0 {
			break
		}
		out = append(out, s.event(s.pending[:end])...)
		s.pending = s.pending[end:]
	}
	if len(s.pending) > maxPendingEvent {
		out = append(out, s.giveUp()...)
	}
	return out
}

// Flush returns everything still held when the stream ends: unfinished
// text tails, held tool deltas, and any incomplete event, all unchanged.
func (s *StreamRehydrator) Flush() []byte {
	if s.passthrough {
		return nil
	}
	return s.giveUp()
}

func (s *StreamRehydrator) giveUp() []byte {
	var out []byte
	for _, idx := range sortedBlocks(s.blocks) {
		b := s.blocks[idx]
		out = append(out, s.flushCarry(b)...)
		out = append(out, s.releaseHeld(b)...)
	}
	out = append(out, s.pending...)
	s.pending = nil
	s.passthrough = true
	return out
}

func sortedBlocks(blocks map[int]*streamBlock) []int {
	idx := make([]int, 0, len(blocks))
	for i := range blocks {
		idx = append(idx, i)
	}
	for i := 1; i < len(idx); i++ {
		for j := i; j > 0 && idx[j] < idx[j-1]; j-- {
			idx[j], idx[j-1] = idx[j-1], idx[j]
		}
	}
	return idx
}

var (
	lfLF     = []byte("\n\n")
	crlfCRLF = []byte("\r\n\r\n")
	dataLine = []byte("data:")
)

// eventEnd returns the length of the first complete event in buf, or -1.
func eventEnd(buf []byte) int {
	i := bytes.Index(buf, lfLF)
	j := bytes.Index(buf, crlfCRLF)
	switch {
	case i < 0 && j < 0:
		return -1
	case j >= 0 && (i < 0 || j < i):
		return j + len(crlfCRLF)
	default:
		return i + len(lfLF)
	}
}

// dataRange finds the JSON of an event's single data line: its start and
// end offsets in the raw event. Events with no or several data lines are
// not rewritten.
func dataRange(raw []byte) (start, end int, ok bool) {
	found := false
	pos := 0
	for pos < len(raw) {
		nl := bytes.IndexByte(raw[pos:], '\n')
		lineEnd := len(raw)
		if nl >= 0 {
			lineEnd = pos + nl
		}
		line := raw[pos:lineEnd]
		if bytes.HasPrefix(line, dataLine) {
			if found {
				return 0, 0, false
			}
			found = true
			start = pos + len(dataLine)
			if start < lineEnd && raw[start] == ' ' {
				start++
			}
			end = lineEnd
			if end > start && raw[end-1] == '\r' {
				end--
			}
		}
		pos = lineEnd + 1
	}
	return start, end, found
}

type streamEvent struct {
	Type         string `json:"type"`
	Index        int    `json:"index"`
	ContentBlock *struct {
		Type string `json:"type"`
		Name string `json:"name"`
	} `json:"content_block"`
	Delta *struct {
		Type        string `json:"type"`
		PartialJSON string `json:"partial_json"`
	} `json:"delta"`
}

// event processes one complete raw event and returns what to forward.
func (s *StreamRehydrator) event(raw []byte) []byte {
	start, end, ok := dataRange(raw)
	if !ok {
		return raw
	}
	var ev streamEvent
	if err := json.Unmarshal(raw[start:end], &ev); err != nil {
		return raw
	}
	switch ev.Type {
	case "content_block_start":
		if ev.ContentBlock == nil {
			return raw
		}
		// A provider that reuses an index is broken, but nothing held for
		// the old block may be lost.
		var out []byte
		if old := s.blocks[ev.Index]; old != nil {
			out = append(s.flushCarry(old), s.releaseHeld(old)...)
		}
		s.blocks[ev.Index] = &streamBlock{kind: ev.ContentBlock.Type, tool: ev.ContentBlock.Name}
		return append(out, raw...)
	case "content_block_delta":
		b := s.blocks[ev.Index]
		if b == nil || ev.Delta == nil {
			return raw
		}
		switch {
		case b.kind == "text" && ev.Delta.Type == "text_delta":
			return s.textDelta(b, raw, start, end)
		case b.kind == "tool_use" && ev.Delta.Type == "input_json_delta" && !b.gaveUp:
			return s.holdToolDelta(b, raw, ev.Delta.PartialJSON)
		}
		return raw
	case "content_block_stop":
		b := s.blocks[ev.Index]
		if b == nil {
			return raw
		}
		delete(s.blocks, ev.Index)
		var out []byte
		switch b.kind {
		case "text":
			out = s.flushCarry(b)
		case "tool_use":
			out = s.releaseTool(b, ev.Index)
		}
		return append(out, raw...)
	}
	return raw
}

// textDelta rewrites a text delta. A held previous delta is released
// first: cut before its tail when a placeholder crosses into this delta,
// unchanged otherwise. Placeholders in the text are restored, and the
// event is held when its own tail could begin one.
func (s *StreamRehydrator) textDelta(b *streamBlock, raw []byte, start, end int) []byte {
	lit, ok := findLiteral(raw[start:end], []string{"delta", "text"})
	if !ok {
		return append(s.flushCarry(b), raw...)
	}
	litStart, litEnd := start+lit.start, start+lit.end
	inner := raw[litStart+1 : litEnd-1]
	var out []byte
	crossing := false
	if b.heldText != nil {
		tail := b.heldText[b.heldLit.end-1-b.tail : b.heldLit.end-1]
		combined := append(append([]byte(nil), tail...), inner...)
		for _, m := range placeholderRE.FindAllIndex(combined, -1) {
			if m[0] < len(tail) {
				crossing = true
				break
			}
		}
		// A prefix that still spans the boundary (a placeholder cut into
		// three or more deltas, or an empty delta between two halves)
		// moves to this delta so it keeps being held.
		spanning := partialPlaceholderSuffix(combined) > len(inner)
		if crossing || spanning {
			out = append(out, s.cutHeld(b)...)
			inner = combined
			crossing = true
		} else {
			out = append(out, b.heldText...)
		}
		b.heldText, b.tail = nil, 0
	}
	hold := partialPlaceholderSuffix(inner)
	restored, changed := s.r.restore(inner[:len(inner)-hold], Destination{Kind: DestinationText}, &s.rep)
	ev := raw
	newLit := literal{start: litStart, end: litEnd}
	if changed || crossing {
		full := append(append([]byte(nil), restored...), inner[len(inner)-hold:]...)
		ev = splice(raw, litStart+1, litEnd-1, full)
		newLit.end = litStart + 1 + len(full) + 1
	}
	if hold > 0 {
		b.heldText, b.heldLit, b.tail = ev, newLit, hold
		return out
	}
	return append(out, ev...)
}

// cutHeld returns the held text delta without its tail, or nothing when
// the tail was the whole text.
func (s *StreamRehydrator) cutHeld(b *streamBlock) []byte {
	litStart, litEnd := b.heldLit.start, b.heldLit.end
	if litEnd-1-b.tail == litStart+1 {
		return nil
	}
	return splice(b.heldText, litEnd-1-b.tail, litEnd-1, nil)
}

// flushCarry releases a held text delta unchanged.
func (s *StreamRehydrator) flushCarry(b *streamBlock) []byte {
	out := b.heldText
	b.heldText, b.tail = nil, 0
	return out
}

// holdToolDelta keeps a tool input delta until the block ends. Past the
// limit, the block's deltas are released unchanged and the block is no
// longer inspected.
func (s *StreamRehydrator) holdToolDelta(b *streamBlock, raw []byte, partial string) []byte {
	b.held = append(b.held, append([]byte(nil), raw...))
	b.heldBytes += len(raw)
	b.input = append(b.input, partial...)
	if b.heldBytes <= MaxHeldToolInputBytes {
		return nil
	}
	dest := Destination{Kind: DestinationTool, Tool: b.tool}
	if _, ok := fileEditTools[b.tool]; ok {
		dest.Kind = DestinationEdit
	}
	for range placeholderRE.FindAllIndex(b.input, -1) {
		s.rep.denied(dest, ReasonTooLarge)
	}
	b.gaveUp = true
	return s.releaseHeld(b)
}

// releaseTool decides a held tool block. Without a placeholder, or with
// every placeholder denied, the held events go out exactly as received;
// otherwise one delta carries the whole restored input.
func (s *StreamRehydrator) releaseTool(b *streamBlock, index int) []byte {
	if !bytes.Contains(b.input, []byte(PlaceholderPrefix)) {
		return s.releaseHeld(b)
	}
	dest := s.r.scopes.toolDestination(b.tool, b.input)
	restored, changed := s.r.restoreJSONText(b.input, dest, &s.rep)
	if !changed {
		return s.releaseHeld(b)
	}
	b.held, b.heldBytes, b.input = nil, 0, nil
	return inputDeltaEvent(index, restored)
}

// releaseHeld returns a tool block's held events exactly as received.
func (s *StreamRehydrator) releaseHeld(b *streamBlock) []byte {
	out := make([]byte, 0, b.heldBytes)
	for _, ev := range b.held {
		out = append(out, ev...)
	}
	b.held, b.heldBytes, b.input = nil, 0, nil
	return out
}

// inputDeltaEvent builds one content_block_delta carrying a whole tool
// input as JSON text.
func inputDeltaEvent(index int, input []byte) []byte {
	var buf bytes.Buffer
	buf.WriteString("event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":")
	buf.WriteString(strconv.Itoa(index))
	buf.WriteString(",\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":")
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(string(input)) // encoding a string never fails
	buf.Truncate(buf.Len() - 1)   // the encoder's trailing newline
	buf.WriteString("}}\n\n")
	return buf.Bytes()
}

// findLiteral locates the string literal at a key path in a document.
func findLiteral(doc []byte, path []string) (literal, bool) {
	lits, err := literals(doc)
	if err != nil {
		return literal{}, false
	}
	for _, l := range lits {
		if samePath(l.path, path) {
			return l, true
		}
	}
	return literal{}, false
}

func samePath(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// partialPlaceholderSuffix is the length of the longest tail of text that
// could be the beginning of a placeholder cut by a chunk boundary.
func partialPlaceholderSuffix(text []byte) int {
	longest := PlaceholderLength - 1
	if len(text) < longest {
		longest = len(text)
	}
	for k := longest; k > 0; k-- {
		if isPlaceholderPrefix(text[len(text)-k:]) {
			return k
		}
	}
	return 0
}

func isPlaceholderPrefix(s []byte) bool {
	if len(s) <= len(PlaceholderPrefix) {
		return bytes.HasPrefix([]byte(PlaceholderPrefix), s)
	}
	if !bytes.HasPrefix(s, []byte(PlaceholderPrefix)) || len(s) >= PlaceholderLength {
		return false
	}
	for _, c := range s[len(PlaceholderPrefix):] {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func splice(raw []byte, start, end int, repl []byte) []byte {
	out := make([]byte, 0, len(raw)-(end-start)+len(repl))
	out = append(out, raw[:start]...)
	out = append(out, repl...)
	return append(out, raw[end:]...)
}

// TransformReader applies a stream rehydrator to a body as it is read.
type TransformReader struct {
	src io.ReadCloser
	t   *StreamRehydrator
	out []byte
	buf []byte
	// err is the source's terminal error, returned once the transformed
	// bytes are drained.
	err error
}

// readChunk is how much is read from the provider at a time.
const readChunk = 32 << 10

// NewTransformReader wraps a response body with a stream rehydrator.
func NewTransformReader(src io.ReadCloser, t *StreamRehydrator) *TransformReader {
	return &TransformReader{src: src, t: t, buf: make([]byte, readChunk)}
}

// Read returns transformed bytes, reading from the source until some are
// available or the source ends.
func (r *TransformReader) Read(p []byte) (int, error) {
	for len(r.out) == 0 {
		if r.err != nil {
			return 0, r.err
		}
		n, err := r.src.Read(r.buf)
		if n > 0 {
			r.out = append(r.out, r.t.Transform(r.buf[:n])...)
		}
		if err != nil {
			r.err = err
			r.out = append(r.out, r.t.Flush()...)
		}
	}
	n := copy(p, r.out)
	r.out = r.out[n:]
	return n, nil
}

// Close closes the source.
func (r *TransformReader) Close() error { return r.src.Close() }
