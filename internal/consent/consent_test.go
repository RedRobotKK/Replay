package consent

import (
	"os"
	"path/filepath"
	"testing"
)

// Update checks are off until the user says otherwise, and "otherwise" has to
// be said out loud.
//
// Replay's headline promise is that it makes no network call except to the
// provider you configured. An update check is a network call, so it cannot be
// a default, a heuristic, or something an agent decides on a person's behalf.
// It is a decision the user makes once, in a file they can read and delete.
//
// The three states matter, and collapsing them to a boolean loses the one that
// carries the most information:
//
//	Unset     — nobody has decided. Ask. Do not check.
//	Granted   — checking is permitted.
//	Declined  — checking is forbidden, and stop asking.
//
// Unset and Declined both mean "do not check now", so a boolean would merge
// them — and then a tool either nags someone who already said no, or never
// asks someone who was never asked.
//
//	U1  no file means undecided, not permission
//	U2  permission must be explicit
//	U3  a refusal is remembered, and is not the same as silence
//	U4  anything unparseable is undecided, never permission
//	U5  a symlink at the consent path is refused
//	U6  a file others can write is not this user's decision
//	U7  only Granted permits a network call

func write(t *testing.T, dir, body string) string {
	t.Helper()
	cfg := filepath.Join(dir, "replay")
	if err := os.MkdirAll(cfg, 0o700); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(cfg, FileName)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// U1: absence is not permission.
// PASS: Unset, and no network permitted.
// FAIL: treating a missing file as either yes or a settled no.
func TestU1_NoFileIsUndecided(t *testing.T) {
	d, err := ReadUpdateConsent(t.TempDir())
	if err != nil {
		t.Fatalf("a missing consent file is the normal case, not an error: %v", err)
	}
	if d.State != Unset {
		t.Errorf("state = %v, want Unset", d.State)
	}
	if d.Allowed() {
		t.Error("no file must never permit a network call")
	}
	if !d.ShouldAsk() {
		t.Error("an undecided user is the one case where asking is correct")
	}
}

// U2: permission is explicit, and only the exact affirmative counts.
// PASS: `update_checks = true` grants; nothing else does.
// FAIL: a near-miss read as consent, which is how a promise about network
// calls quietly stops being true.
func TestU2_PermissionMustBeExplicit(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "update_checks = true\n")
	d, err := ReadUpdateConsent(dir)
	if err != nil {
		t.Fatal(err)
	}
	if d.State != Granted || !d.Allowed() {
		t.Fatalf("explicit true must grant; got %v", d.State)
	}
	if d.ShouldAsk() {
		t.Error("a decided user must not be asked again")
	}

	for _, body := range []string{
		"update_checks = TRUE\n",
		"update_checks = yes\n",
		"update_checks = 1\n",
		"update_checks=true extra\n",
		"# update_checks = true\n",
		"updatechecks = true\n",
		"corpus_opt_in = true\n",
	} {
		dir := t.TempDir()
		write(t, dir, body)
		d, err := ReadUpdateConsent(dir)
		if err == nil && d.Allowed() {
			t.Errorf("%q was read as permission; only an exact affirmative may grant", body)
		}
	}
}

// U3: a refusal is durable and distinguishable from never having been asked.
// PASS: Declined, no network, and no further asking.
// FAIL: merging it with Unset, which turns a considered no into a repeated
// prompt.
func TestU3_RefusalIsRememberedAndDistinct(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "update_checks = false\n")
	d, err := ReadUpdateConsent(dir)
	if err != nil {
		t.Fatal(err)
	}
	if d.State != Declined {
		t.Errorf("state = %v, want Declined", d.State)
	}
	if d.Allowed() {
		t.Error("a refusal must forbid the call")
	}
	if d.ShouldAsk() {
		t.Error("someone who said no must not be asked again; that is what makes saying no worth doing")
	}
}

// U4: unparseable is undecided.
// PASS: no grant from anything malformed.
// FAIL: a partial or corrupt file producing permission.
func TestU4_UnparseableIsNeverPermission(t *testing.T) {
	for _, body := range []string{
		"", "\x00\x01\x02", "update_checks", "update_checks =",
		"[section]\n", "update_checks = true\nupdate_checks = false\n",
		// An affirmative with anything else beside it. This is the realistic
		// shape: a legitimate file with a line appended to it. A parser that
		// skips what it does not recognise reads this as permission, so the
		// unreadable line has to be refused rather than ignored.
		"update_checks = true\nsend_everything = true\n",
		"update_checks = true\n[remote]\nurl = \"http://attacker.test\"\n",
		"update_checks = true\nupdate_checks_extra\n",
	} {
		dir := t.TempDir()
		write(t, dir, body)
		d, err := ReadUpdateConsent(dir)
		if err == nil && d.Allowed() {
			t.Errorf("%q produced permission; anything unclear must be undecided", body)
		}
	}
}

// U5: a symlink at the consent path is refused.
// PASS: an error, and no permission.
// FAIL: following it — the file says what the user decided, and a link means
// someone else chose where that answer comes from.
func TestU5_SymlinkIsRefused(t *testing.T) {
	dir := t.TempDir()
	other := filepath.Join(dir, "elsewhere.toml")
	if err := os.WriteFile(other, []byte("update_checks = true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(dir, "replay")
	if err := os.MkdirAll(cfg, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, filepath.Join(cfg, FileName)); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	d, err := ReadUpdateConsent(dir)
	if err == nil {
		t.Error("a symlink at the consent path must be refused")
	}
	if d.Allowed() {
		t.Error("a symlink must never grant permission")
	}
}

// U6: a file others can write is not this user's decision.
// PASS: refused when group- or world-writable.
// FAIL: honouring a grant any process on the box could have written.
func TestU6_WorldWritableIsRefused(t *testing.T) {
	dir := t.TempDir()
	p := write(t, dir, "update_checks = true\n")
	if err := os.Chmod(p, 0o666); err != nil {
		t.Skipf("chmod unavailable: %v", err)
	}
	d, err := ReadUpdateConsent(dir)
	if err == nil {
		t.Error("a world-writable consent file must be refused")
	}
	if d.Allowed() {
		t.Error("a world-writable file must never grant permission")
	}
}

// U7: Allowed is the single gate.
// PASS: exactly one state permits the call.
// FAIL: more than one, which means the gate is not a gate.
func TestU7_OnlyGrantedPermits(t *testing.T) {
	var permitting int
	for _, s := range []State{Unset, Granted, Declined} {
		if (Decision{State: s}).Allowed() {
			permitting++
			if s != Granted {
				t.Errorf("state %v permits a network call; only Granted may", s)
			}
		}
	}
	if permitting != 1 {
		t.Errorf("%d states permit the call, want exactly 1", permitting)
	}
}
