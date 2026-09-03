package masking

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
)

// literal is one JSON string value's raw bytes in a document and the key
// path that leads to it: object keys and array indexes from the root.
type literal struct {
	start, end int // including the quotes
	path       []string
}

// literals locates every string value in a JSON document with the
// decoder's own offsets, so a caller can rewrite the bytes of one literal
// and leave every other byte as it was. Object keys are not values.
func literals(doc []byte) ([]literal, error) {
	dec := json.NewDecoder(bytes.NewReader(doc))
	dec.UseNumber()
	type frame struct {
		object  bool
		key     string
		haveKey bool
		index   int
		label   string // this container's key or index in its parent
	}
	var stack []*frame
	var out []literal
	// current is the label of the value about to be read at the top frame.
	current := func(top *frame) string {
		if top.object {
			return top.key
		}
		return strconv.Itoa(top.index)
	}
	// advance moves the top frame past the value just read.
	advance := func(top *frame) {
		if top.object {
			top.haveKey = false
		} else {
			top.index++
		}
	}
	path := func(leaf string) []string {
		p := make([]string, 0, len(stack)+1)
		for _, f := range stack[1:] {
			p = append(p, f.label)
		}
		return append(p, leaf)
	}
	before := int64(0)
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			if len(stack) > 0 {
				return nil, errors.New("json: unexpected end of document")
			}
			break
		}
		if err != nil {
			return nil, fmt.Errorf("json: %w", err)
		}
		after := dec.InputOffset()
		var top *frame
		if len(stack) > 0 {
			top = stack[len(stack)-1]
		}
		switch t := tok.(type) {
		case json.Delim:
			switch t {
			case '{', '[':
				f := &frame{object: t == '{'}
				if top != nil {
					f.label = current(top)
				}
				stack = append(stack, f)
			case '}', ']':
				stack = stack[:len(stack)-1]
				if len(stack) > 0 {
					advance(stack[len(stack)-1])
				}
			}
		case string:
			if top != nil && top.object && !top.haveKey {
				top.key, top.haveKey = t, true
				break
			}
			// The literal's opening quote is the first quote after the
			// previous token; only separators and whitespace lie between.
			start := bytes.IndexByte(doc[before:after], '"') + int(before)
			var p []string
			if top != nil {
				p = path(current(top))
				advance(top)
			}
			out = append(out, literal{start: start, end: int(after), path: p})
		default:
			if top != nil {
				advance(top)
			}
		}
		before = after
	}
	return out, nil
}
