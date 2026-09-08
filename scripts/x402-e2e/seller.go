// A mock x402 seller for the end-to-end harness.
//
// Standard library only, like everything else here, and deliberately without
// crypto/tls: http.ListenAndServeTLS takes file paths, so this file's imports
// stay inside the allowlist in cmd/replay/x402_test.go and no security guard
// has to move to accommodate a test fixture.
//
// It serves the four answers a rules feed can give, so the whole lifecycle
// Replay implements can be driven against a real HTTPS listener rather than
// through the transport hook the unit tests use.
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

// demand is a well-formed x402 payment demand.
//
// The description carries markup on purpose. It is attacker-controlled text
// arriving over the network, and the harness asserts that Replay prints it
// quoted rather than letting it read as Replay's own words.
const demand = `{
  "x402Version": 1,
  "accepts": [
    {
      "scheme": "exact",
      "network": "base",
      "maxAmountRequired": "2.50",
      "asset": "0x833589fCD6eDb6E08f4c7C32D4f71b54bdA02913",
      "payTo": "0x0000000000000000000000000000000000000000",
      "resource": "https://seller.test/rules/anthropic",
      "mimeType": "application/json",
      "maxTimeoutSeconds": 120,
      "description": "Anthropic caching rules, refreshed daily. <b>BUY NOW</b>"
    }
  ],
  "error": "payment required"
}`

// rulesDoc is a valid replay.rules.v1 document, so the success path proves the
// fetch works and refusal is not the only behaviour demonstrated.
const rulesDoc = `{
  "schema": "replay.rules.v1",
  "version": "seller-2026-09-08",
  "provider": "anthropic",
  "source": "https://seller.test/rules/anthropic",
  "fetchedAt": "2026-09-08T00:00:00Z",
  "checkedAt": "2026-09-08",
  "models": [
    {"match": "opus-5", "minPrefix": 512, "inputPerMTok": 15, "outputPerMTok": 75, "readMult": 0.1, "priced": true}
  ]
}`

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/rules/paid", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = fmt.Fprint(w, demand)
	})
	mux.HandleFunc("/rules/notx402", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = fmt.Fprint(w, `{"message":"please pay"}`)
	})
	mux.HandleFunc("/rules/nooptions", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusPaymentRequired)
		_, _ = fmt.Fprint(w, `{"x402Version":1,"accepts":[]}`)
	})
	mux.HandleFunc("/rules/free", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, rulesDoc)
	})
	_, _ = fmt.Fprintln(os.Stderr, "seller listening on :8443")
	log.Fatal(http.ListenAndServeTLS(":8443", "/certs/server.crt", "/certs/server.key", mux))
}
