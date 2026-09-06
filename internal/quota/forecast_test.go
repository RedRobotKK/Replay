package quota

import (
	"testing"
	"time"
)

// When does this seat run out?
//
// The titration failed to establish how many tokens move the counter - 3.09M
// moved it zero steps. That null does NOT block this, and the reason is the
// whole design: a forecast does not need a token model, because the provider
// reports the LEVEL directly. What is needed is the rate of change over TIME,
// and two readings an hour apart give it without any conversion.
//
// The binding window is named by the provider too, in
// anthropic-ratelimit-unified-representative-claim, so there is no guessing
// which of the 5h and 7d deadlines matters.
//
// This forecasts and never blocks. A tool that fabricates a 429 to protect a
// budget has caused the outage it was preventing, and the agent cannot tell it
// from the provider's own.

var f0 = time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

func r(mins int, util float64) Reading {
	return Reading{At: f0.Add(time.Duration(mins) * time.Minute), Util: util,
		Window: "five_hour", Status: "allowed"}
}

// FC-1: a steady climb projects to the limit.
//
// PASS: 0.20 to 0.50 over two hours is 0.15/hour, so 0.50 of headroom is
// three and a third hours away.
// FAIL: any answer derived from token counts, which the null says nobody has.
func TestFC1_SteadyClimbProjectsToTheLimit(t *testing.T) {
	f := Forecast([]Reading{r(0, 0.20), r(60, 0.35), r(120, 0.50)})
	if !f.Reportable {
		t.Fatalf("a clean climb must produce a forecast: %s", f.Why)
	}
	if got := f.RatePerHour; got < 0.14 || got > 0.16 {
		t.Errorf("rate = %.3f/hour, want ~0.15", got)
	}
	want := 200 * time.Minute
	if f.TimeToLimit < want-10*time.Minute || f.TimeToLimit > want+10*time.Minute {
		t.Errorf("time to limit = %v, want ~%v", f.TimeToLimit, want)
	}
}

// FC-2: a window reset restarts the fit.
//
// The 5h window rolls. A reading below the previous one is a reset, and fitting
// across it gives a negative rate and a forecast of never - on a seat that is
// filling normally.
//
// PASS: only the readings since the reset are fitted.
// FAIL: a slope computed across the boundary.
func TestFC2_AResetRestartsTheFit(t *testing.T) {
	f := Forecast([]Reading{
		r(0, 0.80), r(60, 0.95),
		r(120, 0.05), // the window rolled
		r(180, 0.20), r(240, 0.35),
	})
	if !f.Reportable {
		t.Fatalf("there is a clean climb after the reset: %s", f.Why)
	}
	if f.RatePerHour < 0.14 || f.RatePerHour > 0.16 {
		t.Errorf("rate = %.3f/hour, want ~0.15 from the post-reset readings only", f.RatePerHour)
	}
}

// FC-3: movement below the counter's resolution is not a rate.
//
// Utilization carries two decimals, so a change under 0.01 is quantization, not
// consumption. Fitting it produces a confident forecast from noise.
//
// PASS: refuses, and says why.
// FAIL: a number, which is the failure this project has now made three times.
func TestFC3_BelowResolutionIsNotARate(t *testing.T) {
	// A rise that is real but smaller than the counter can express. A FLAT
	// series would refuse because the rate is zero, leaving the resolution
	// guard unexercised - mutation showed exactly that, the third time today
	// a test in this project refused for the wrong reason.
	f := Forecast([]Reading{r(0, 0.200), r(60, 0.204), r(120, 0.208)})
	if f.Reportable {
		t.Errorf("forecast from a counter that never moved: %+v", f)
	}
	if f.Why == "" {
		t.Error("a refusal must say what is missing")
	}
}

// FC-4: an idle or falling seat is not forecast to exhaust.
//
// PASS: no forecast when the rate is not positive.
// FAIL: a negative time-to-limit, or a forecast of the past.
func TestFC4_NoForecastWhenNotRising(t *testing.T) {
	f := Forecast([]Reading{r(0, 0.50), r(60, 0.45), r(120, 0.40)})
	if f.Reportable || f.TimeToLimit != 0 {
		t.Errorf("a falling counter must produce no forecast: %+v", f)
	}
}

// FC-5: two readings are not a trend.
//
// PASS: refuses under three.
// FAIL: a forecast from a single interval, which is one burst away from wrong.
func TestFC5_TwoReadingsAreNotATrend(t *testing.T) {
	if f := Forecast([]Reading{r(0, 0.20), r(60, 0.40)}); f.Reportable {
		t.Errorf("forecast from a single interval: %+v", f)
	}
}

// FC-6: an already-exhausted seat reports zero, not a negative.
func TestFC6_AlreadyAtTheLimit(t *testing.T) {
	// Past the limit, not exactly at it. At exactly 1.00 the headroom is zero
	// and the quotient is zero anyway, so the guard against a NEGATIVE
	// time-to-limit goes unexercised - which mutation caught.
	f := Forecast([]Reading{r(0, 0.95), r(60, 1.00), r(120, 1.05)})
	if f.TimeToLimit != 0 {
		t.Errorf("time to limit = %v past an exhausted window, want 0 rather than a "+
			"negative duration", f.TimeToLimit)
	}
}

// FC-7: the forecast names the window the provider says is binding.
//
// Two windows run at once and they exhaust at different times. Forecasting the
// 5h when the 7d is the constraint answers a question nobody asked.
func TestFC7_ItNamesTheBindingWindow(t *testing.T) {
	rs := []Reading{r(0, 0.20), r(60, 0.35), r(120, 0.50)}
	for i := range rs {
		rs[i].Window = "seven_day"
	}
	if got := Forecast(rs).Window; got != "seven_day" {
		t.Errorf("window = %q, want seven_day", got)
	}
}
