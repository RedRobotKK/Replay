package main

import (
	"os"
	"testing"
)

// FD-4, frozen. Test isolation did not isolate on Windows.
//
// isolateHome set HOME and CLAUDE_CONFIG_DIR. The commands reach a home
// directory through os.UserHomeDir, which reads HOME on unix and USERPROFILE on
// Windows, so on the windows-latest runner every test built on this helper ran
// against the runner's REAL home. They found no transcripts and failed on empty
// output, which is indistinguishable from the product being broken, and the job
// stayed red long enough to read as background.
//
// The shape worth remembering is that the helper was not silent about failing.
// It failed loudly, on the wrong subject. A harness bug wearing a product bug's
// clothes costs more than either.
//
// What would be true again if it returned: after isolateHome, one of the
// variables os.UserHomeDir consults on some supported platform would still name
// the developer's or the runner's own home.
//
// PASS: every home-derived variable points inside the directory the test owns.
// FAIL: one of them was left pointing at the real machine.
func TestFrozenFD4_IsolateHomeRedirectsEveryHomeVariableOnEveryPlatform(t *testing.T) {
	machineHome, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("cannot read the real home to compare against: %v", err)
	}

	owned := t.TempDir()
	isolateHome(t, owned)

	// The variables os.UserHomeDir reads, across the platforms this repo
	// builds for and the one it refuses to. Asserted unconditionally: a check
	// that only runs on the platform with the defect is the check that let the
	// defect ship.
	for _, name := range []string{"HOME", "USERPROFILE"} {
		got, ok := os.LookupEnv(name)
		if !ok {
			t.Errorf("%s is unset after isolateHome. os.UserHomeDir reads HOME on unix and "+
				"USERPROFILE on Windows; leaving either unset hands that platform's tests "+
				"the real home", name)
			continue
		}
		if got != owned {
			t.Errorf("%s = %q after isolateHome, want the directory the test owns (%q). "+
				"This is the defect that kept windows-latest red: HOME was redirected, "+
				"USERPROFILE was not, and the tests read the runner's own home",
				name, got, owned)
		}
		if got == machineHome {
			t.Errorf("%s still names the real home %q", name, machineHome)
		}
	}

	// And the derived path actually moves, which is the fact the tests depend
	// on rather than the variables themselves.
	after, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("os.UserHomeDir after isolateHome: %v", err)
	}
	if after != owned {
		t.Errorf("os.UserHomeDir() = %q after isolateHome, want %q. The variables can all "+
			"be set and still miss the one this platform reads", after, owned)
	}
}
