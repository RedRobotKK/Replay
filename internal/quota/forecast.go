package quota

import (
	"fmt"
	"time"
)

// When does this seat run out?
//
// The titration failed to establish how many tokens move the counter: 3.09M
// moved it zero steps. That null does not block a forecast, and the reason is
// the whole design. A forecast needs no token model, because the provider
// reports the LEVEL directly - what is needed is its rate of change over TIME,
// and two readings an hour apart give that without any conversion.
//
// The provider also names the binding window in
// anthropic-ratelimit-unified-representative-claim, so there is no guessing
// which of the five-hour and seven-day deadlines applies.
//
// This forecasts and never blocks. A tool that fabricates a 429 to protect a
// budget has caused the outage it claimed to prevent, and the agent cannot tell
// a fabricated one from the provider's own.

// minReadings is the fewest readings that can describe a trend.
//
// Two readings are one interval, and one interval is a single burst away from
// wrong. Three is still thin; it is a floor, not a claim of precision.
const minReadings = 3

// resolution is the smallest movement the counter can express. The wire carries
// two decimals, so anything under this is quantization rather than consumption,
// and fitting it manufactures a confident forecast out of nothing.
const resolution = 0.01

// Reading is one observation of the binding window's utilization.
type Reading struct {
	At     time.Time
	Util   float64
	Window string
	Status string
}

// Forecast is when the binding window runs out, or why it cannot be said.
type QuotaForecast struct {
	Window      string
	Util        float64
	RatePerHour float64
	// TimeToLimit is how long until utilization reaches 1, and zero when the
	// window is already exhausted or the rate is not positive.
	TimeToLimit time.Duration
	Reportable  bool
	Why         string
}

// Forecast projects the binding window forward from observed readings.
func Forecast(rs []Reading) QuotaForecast {
	f := QuotaForecast{}
	if len(rs) == 0 {
		f.Why = "no quota readings yet; run the proxy and the provider supplies them"
		return f
	}
	last := rs[len(rs)-1]
	f.Window, f.Util = last.Window, last.Util

	// A reading below its predecessor means the window rolled. Fitting across
	// that boundary gives a negative rate and a forecast of never, on a seat
	// that is filling normally - so only the readings since the last reset are
	// used.
	start := 0
	for i := 1; i < len(rs); i++ {
		if rs[i].Util < rs[i-1].Util {
			start = i
		}
	}
	cur := rs[start:]
	if len(cur) < minReadings {
		f.Why = fmt.Sprintf("%d reading(s) since the window last reset, %d needed: two points "+
			"are one interval, and one interval is a burst away from wrong", len(cur), minReadings)
		return f
	}

	span := cur[len(cur)-1].At.Sub(cur[0].At)
	rise := cur[len(cur)-1].Util - cur[0].Util
	if span <= 0 {
		f.Why = "readings carry no elapsed time between them"
		return f
	}
	if rise < resolution {
		f.Why = fmt.Sprintf("the counter moved %.3f over %s, below its own %.2f resolution: "+
			"that is quantization, not consumption", rise, span.Round(time.Minute), resolution)
		return f
	}
	f.RatePerHour = rise / span.Hours()
	if f.RatePerHour <= 0 {
		f.Why = "utilization is not rising"
		return f
	}
	if headroom := 1 - last.Util; headroom > 0 {
		f.TimeToLimit = time.Duration(headroom / f.RatePerHour * float64(time.Hour))
	}
	f.Reportable = true
	return f
}
