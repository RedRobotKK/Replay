package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/proxy"
)

// QT-7a, the operator's surface. When the running proxy has forwarded
// responses that carried no usage while a dollar cap is set, the doctor
// must say so: nothing was counted for that traffic and the cap did not see
// it. This is a different state from the upper-bound one (QT-7b): there the
// cap works on an over-estimate; here it does not work at all on that
// traffic.
//
// The status is built from JSON rather than a struct literal so that this
// test compiles against a Status that does not yet carry the field, and
// fails on what the doctor prints rather than on what the compiler accepts.
func TestQT7a_DoctorWarnsWhenResponsesCarriedNoUsageUnderADollarCap(t *testing.T) {
	var st proxy.Status
	if err := json.Unmarshal([]byte(`{"requests":{"2xx":5},"caps":{"day_usd":true},"responses_without_usage":5}`), &st); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(guardLines(st), "\n")
	if !strings.Contains(got, "WARNING") || !strings.Contains(strings.ToLower(got), "no usage") {
		t.Errorf("five responses carried no usage under a day dollar cap and the doctor says nothing about it:\n%s", got)
	}
	// Without a dollar cap the same traffic is a fact, not a warning: the
	// token caps count whether or not usage is priced, but they too need the
	// usage object, so the count is still worth a line.
	var quiet proxy.Status
	if err := json.Unmarshal([]byte(`{"requests":{"2xx":5},"caps":{},"responses_without_usage":5}`), &quiet); err != nil {
		t.Fatal(err)
	}
	if q := strings.Join(guardLines(quiet), "\n"); !strings.Contains(strings.ToLower(q), "no usage") {
		t.Errorf("five responses carried no usage and no cap is set; the doctor should still say the count:\n%s", q)
	}
}
