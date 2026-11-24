package quota

import (
	"strings"
	"testing"
	"time"
)

// A binding-window change makes prior utilization readings incomparable, even
// when their numeric values happen to form an increasing trend.
func TestForecastBindingWindowChangeRequiresFreshTrend(t *testing.T) {
	readings := []Reading{
		r(0, 0.20),
		r(60, 0.35),
		r(120, 0.50),
	}
	readings[2].Window = "seven_day"

	got := Forecast(readings)
	if got.Reportable || got.TimeToLimit != 0 || got.RatePerHour != 0 {
		t.Fatalf("mixed quota windows produced a forecast: %+v", got)
	}
	if got.Window != "seven_day" || got.Util != 0.50 {
		t.Errorf("latest binding window/level lost: %+v", got)
	}
	if !strings.Contains(got.Why, "1 reading(s)") {
		t.Errorf("expected insufficient fresh readings, got %q", got.Why)
	}
}

// Only the newest uninterrupted segment of the binding window may be fitted.
func TestForecastBindingWindowChangeUsesNewWindowOnly(t *testing.T) {
	readings := []Reading{
		r(0, 0.10),
		r(60, 0.20),
		r(120, 0.30),
		r(180, 0.40),
		r(240, 0.50),
	}
	for i := 2; i < len(readings); i++ {
		readings[i].Window = "seven_day"
	}
	got := Forecast(readings)
	if !got.Reportable || got.Window != "seven_day" {
		t.Fatalf("new binding window with three readings should forecast: %+v", got)
	}
	if got.RatePerHour < 0.099 || got.RatePerHour > 0.101 {
		t.Errorf("rate = %.4f/hour, want 0.10 based on new window only", got.RatePerHour)
	}
	if got.TimeToLimit != 5*time.Hour {
		t.Errorf("time to limit = %s, want 5h", got.TimeToLimit)
	}
}
