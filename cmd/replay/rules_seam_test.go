package main

import "net/http"

// swapTransport points rules fetching at a test server and returns a function
// restoring the previous transport.
//
// Test-only, and in a _test.go file so the shipped binary carries no way to
// redirect where rules come from. Callers must restore before returning: the
// variable is not synchronised, and no test in this package calls t.Parallel(),
// so Go runs them sequentially and there is exactly one writer at a time. A
// test added with t.Parallel() that touches the update path would make this a
// real data race under -race, which is the intended warning.
func swapTransport(rt http.RoundTripper) func() {
	prev := rulesTransport
	rulesTransport = rt
	return func() { rulesTransport = prev }
}
