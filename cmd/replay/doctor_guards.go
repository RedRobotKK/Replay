package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/RedRobotKK/Replay/internal/proxy"
)

// guardLines renders what the proxy's guards have actually done.
//
// The guards were the strongest thing Replay had that nobody could find. The
// engineering exists and is tested; `doctor` reported transcripts, proxy and
// ledger and then stopped, so a guard saving somebody money overnight was
// invisible and a guard that was not being applied was equally invisible. This
// prints unconditionally, because a block that appears only when something is
// wrong teaches the reader nothing about what is watching.
func guardLines(st proxy.Status) []string {
	var out []string
	refused := st.Requests["refused"]
	switch {
	case refused == 0:
		out = append(out, "no request has been refused locally")
	case len(st.Refusals) == 0:
		out = append(out, fmt.Sprintf("%d request(s) refused locally", refused))
	default:
		kinds := make([]string, 0, len(st.Refusals))
		for k := range st.Refusals {
			kinds = append(kinds, k)
		}
		// Sorted so two runs of doctor on the same state read the same.
		sort.Strings(kinds)
		detail := ""
		for i, k := range kinds {
			if i > 0 {
				detail += ", "
			}
			detail += fmt.Sprintf("%s %d", k, st.Refusals[k])
		}
		out = append(out, fmt.Sprintf("%d request(s) refused locally: %s", refused, detail))
	}
	if st.CostUSD > 0 {
		out = append(out, fmt.Sprintf("$%.2f at list price since start, $%.2f today (UTC)", st.CostUSD, st.DayCostUSD))
	}
	if st.SpendCapNotEnforced {
		// Loud on purpose. The failure is that the user believes they have a
		// limit they do not have, which is worse than having set none.
		out = append(out,
			"WARNING: a dollar cap is set, but some traffic could not be priced, so the",
			"         cap is not being applied to it. Check `replay rules` covers every",
			"         model in use, or cap on tokens instead, which always applies.")
	}
	return out
}

// probeStatus fetches the proxy's status for the guards block.
//
// Loopback only, for the same reason probeProxy is: this is a GET to whatever
// the environment names, and doctor's job is to report what it can see here,
// not to become a request generator pointed at somebody's network.
func probeStatus(base string) (proxy.Status, bool) {
	var st proxy.Status
	if !isLoopbackURL(base) {
		return st, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), doctorTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+proxy.StatusPath, nil)
	if err != nil {
		return st, false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return st, false
	}
	defer resp.Body.Close() //nolint:errcheck // probe only
	if resp.StatusCode != http.StatusOK {
		return st, false
	}
	// Bounded: this is an untrusted body from whatever is listening on the port.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil || json.Unmarshal(body, &st) != nil {
		return st, false
	}
	return st, true
}
