package proxy

import "encoding/json"

// jsonUnmarshal is a seam for the one place the proxy decodes a request
// body for attribution. It is deliberately the standard decoder; nothing
// about the body is changed by reading it.
func jsonUnmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}
