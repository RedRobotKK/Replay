package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/RedRobotKK/Replay/internal/proxy"
)

// QT-7c, the operator's surface. A 2xx body that could not be read is a
// different fact from a message that carried no usage, and the doctor must
// say which. Built from JSON so this compiles before the field exists.
func TestQT7c_DoctorReportsBodiesThatCouldNotBeRead(t *testing.T) {
	var st proxy.Status
	if err := json.Unmarshal([]byte(`{"requests":{"2xx":4},"caps":{"day_usd":true},"responses_unparsed":4}`), &st); err != nil {
		t.Fatal(err)
	}
	got := strings.ToLower(strings.Join(guardLines(st), "\n"))
	if !strings.Contains(got, "warning") || !strings.Contains(got, "could not be read") {
		t.Errorf("four 2xx bodies could not be read under a day dollar cap and the doctor does not say so:\n%s", got)
	}
	if strings.Contains(got, "no usage") {
		t.Errorf("the doctor describes unreadable bodies as messages without usage:\n%s", got)
	}
}
