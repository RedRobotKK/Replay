package cachemodel

import "time"

// PriceForAt is the production price lookup: the installed document at the
// request's timestamp, then the compiled table.
//
// PriceFor ignores dated windows. PriceAt honours them and had no production
// callers, so a promotion or a negotiated discount that was only wired through
// PriceAt could not reach a number a user sees. Every priced request in cost,
// burn, and as-run session totals goes through here.
//
// A zero timestamp means "no time was recorded": fall back to PriceFor rather
// than invent a clock.
func PriceForAt(model string, t time.Time) (Price, bool) {
	if t.IsZero() {
		return PriceFor(model)
	}
	overrideMu.RLock()
	r := override
	overrideMu.RUnlock()
	if r != nil {
		if p, ok := r.PriceAt(model, t); ok {
			return p, true
		}
		// The installed document has no row in force at t. Do not call
		// PriceFor: that walks the same document without the window and
		// would apply a dated row as if it had none.
		row := lookup(model)
		return row.price, row.priced
	}
	return PriceFor(model)
}
