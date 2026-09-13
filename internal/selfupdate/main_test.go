package selfupdate

import (
	"errors"
	"os"
	"testing"
)

// TestMain takes cosign off the machine for the whole package.
//
// WHY THIS EXISTS. On 2026-09-13 the v0.6.0 release build failed and CI had
// been green the entire time. Fetch gained a Sigstore check that afternoon;
// the tests written for it all pinned LookCosign, but two OLDER tests,
// TestFetchRefusesASymlinkMember and TestFetchReturnsTheVerifiedBinary, were
// written before Fetch called cosign at all and inherited whatever the host
// had. Their httptest server serves the archive and checksums.txt and nothing
// else, so:
//
//   - no cosign on the machine: LookCosign fails, the check returns
//     SignatureUnchecked with no error, both tests pass
//   - cosign on the machine: the check asks for checksums.txt.pem, does not
//     get it, and refuses the whole download, so both tests fail
//
// Every CI runner in the test matrix has no cosign. The release job installs
// it, to sign with. So the first machine in this project's history to run
// these tests WITH cosign present was the one publishing the release, and the
// tag it was cutting is the one that failed.
//
// The defect is not the Sigstore check and it is not those two tests. It is
// that their outcome was decided by a program that happened to be on the PATH,
// which makes a green run evidence about the machine rather than about the
// code. ADR-0014's rule is that a check which cannot fail is not evidence;
// this is the neighbouring case, a check whose result is not about its subject.
//
// So the default here is "no cosign", pinned, and a test that wants it present
// says so by assigning LookCosign itself, which is what every test in
// verify_test.go already does.
func TestMain(m *testing.M) {
	productionLookCosign = LookCosign
	LookCosign = func() (string, error) {
		return "", errors.New("cosign is pinned absent by TestMain; a test that needs it present must assign LookCosign itself")
	}
	os.Exit(m.Run())
}

// productionLookCosign holds what LookCosign was before TestMain replaced it,
// so the replacement cannot hide a broken production value. Without this, the
// pin above would make the package's tests pass even if LookCosign shipped
// pointing at nothing.
var productionLookCosign func() (string, error)

// SU1: the production seam is intact behind the pin.
//
// TestMain replaces LookCosign for every other test in this package. That is
// the right default and it is also a blindfold: with the seam pinned, nothing
// else here observes what production actually does. This is the one test that
// looks at the real one.
//
// It asserts the CONTRACT rather than a fixed answer, for two reasons. The
// answer differs between a machine with cosign and a machine without, and
// requiring either would put this test back in the position that caused the
// defect it was written for. And comparing against exec.LookPath directly
// would import os/exec into a test file, which
// TestX402_ExecIsConfinedToTheMutationHarness refuses: this repository allows
// os/exec in two reviewed places and a test file is not one of them. That
// guard caught this test on the first run.
//
// The contract: exactly one of path and error is set, and a path that is
// returned names something that exists.
func TestSU1_TheProductionCosignLookupIsIntact(t *testing.T) {
	if productionLookCosign == nil {
		t.Fatal("TestMain did not capture the production LookCosign")
	}
	path, err := productionLookCosign()

	switch {
	case err == nil && path == "":
		t.Error("the production lookup reported success with no path, so a caller " +
			"would run the empty string as a program")
	case err != nil && path != "":
		t.Errorf("the production lookup reported both a failure and a path %q", path)
	case err == nil:
		// It found something. It must be something that is there, or the
		// signature check would fail at exec time on a machine that just
		// reported cosign present.
		if _, statErr := os.Stat(path); statErr != nil {
			t.Errorf("the production lookup returned %q, which does not stat: %v", path, statErr)
		}
	}
}

// SU2: the pin is actually in force, so the tests below it are hermetic.
//
// If TestMain stopped running, or something restored LookCosign globally, the
// package would silently go back to reading the machine and this whole file
// would be decoration. The failure that prompted it looked exactly like
// nothing being wrong.
func TestSU2_CosignIsPinnedAbsentForThisPackage(t *testing.T) {
	if _, err := LookCosign(); err == nil {
		t.Fatal("LookCosign reports cosign present, so this package's tests are " +
			"reading the machine again and their result is about the runner, " +
			"not about the code")
	}
}
