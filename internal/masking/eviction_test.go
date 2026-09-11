package masking

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Finding 3 of the 2026-09-04 adversarial security review, half closed here
// and half stated.
//
// Verbatim, from docs/evidence/security-review-2026-09-04.md:
//
//	The vault key file sits beside the ciphertext, so the vault is
//	plaintext-equivalent to anyone who can read the directory. Masking
//	converts transient secrets into secrets at rest, with no eviction
//	(`masking/vault.go:29-38`)
//
// RELEASE-CRITERIA.md names three ways to discharge it and asks for one,
// deliberately:
//
//	Either the key moves to the OS keychain, or vault entries expire, or the
//	README stops implying masking is durable protection. One of the three,
//	chosen deliberately.
//
// The second is taken. Entries expire.
//
// Why this one. The keychain is not available to this binary: reaching
// macOS's requires shelling out to `security`, and cmd/replay/x402_test.go's
// X6c confines every os/exec importer to the `mutation` build tag or to a
// named exemption, on the grounds that os/exec can call anything. Widening
// that to buy a key store is trading a capability guard that protects the
// whole binary for a boundary on one file. The third option is a
// documentation change, which leaves the retention itself unbounded.
//
// What expiry actually buys, stated narrowly so it is not read as more:
//
//   - It bounds the window. Before this, `~/.replay/vault` accumulated every
//     secret the proxy ever masked, for the life of the machine, and only
//     `replay purge` removed any of it. A host compromised on day 200 handed
//     over 200 days of credentials. Now it hands over the TTL.
//   - It costs almost nothing, because the placeholder is the HMAC of the
//     secret and is therefore stable. A client re-sending a secret after its
//     entry expired gets the same placeholder back and the entry returns —
//     EV4 pins that. The only loss is rehydrating a placeholder whose secret
//     has not been seen for longer than the TTL.
//
// What it does NOT do, which stays open and is written into the review file
// rather than argued away: the key file still sits next to the ciphertext.
// Within the TTL the vault remains plaintext-equivalent to anyone who can
// read the directory. Finding 7's directory check narrows who that is on a
// shared machine; it does nothing against a compromised host.

// EV1: a secret older than the TTL does not come back.
//
// PASS: Secret reports it is not held.
// FAIL: it rehydrates, which is retention with an expiry field bolted on.
func TestEV1_AnExpiredSecretDoesNotRehydrate(t *testing.T) {
	at := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	restore := now
	now = func() time.Time { return at }
	defer func() { now = restore }()

	v, err := OpenVaultWithTTL(t.TempDir(), time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	ph, err := v.Placeholder("sk-ant-api03-a-real-looking-secret", "anthropic")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := v.Secret(ph); !ok {
		t.Fatal("the secret must be held immediately after masking; this test would be vacuous otherwise")
	}
	now = func() time.Time { return at.Add(time.Hour + time.Second) }
	if secret, _, ok := v.Secret(ph); ok {
		t.Errorf("an expired entry rehydrated to %q", secret)
	}
}

// EV2: expiry removes the ciphertext, it does not merely hide it on read.
//
// The finding is about what is AT REST. A filter applied on the way out
// leaves the secret in `~/.replay/vault` and changes nothing for anyone
// holding the file and the key beside it.
//
// PASS: after a sweep, the vault file no longer decrypts to the secret.
// FAIL: the bytes are still there, which is the finding untouched.
func TestEV2_ExpiryRemovesTheCiphertextFromDisk(t *testing.T) {
	dir := t.TempDir()
	at := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	restore := now
	now = func() time.Time { return at }
	defer func() { now = restore }()

	const kept = "sk-ant-api03-a-real-looking-secret"
	v, err := OpenVaultWithTTL(dir, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Placeholder(kept, "anthropic"); err != nil {
		t.Fatal(err)
	}
	sealedBefore, err := os.ReadFile(filepath.Join(dir, vaultFile))
	if err != nil {
		t.Fatal(err)
	}
	key, err := os.ReadFile(filepath.Join(dir, keyFile))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := open(key, sealedBefore)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(plain), kept) {
		t.Fatal("the secret is not in the vault to begin with; this test would be vacuous")
	}

	// Reopen past the TTL, which is what the next `replay serve` does.
	now = func() time.Time { return at.Add(2 * time.Hour) }
	again, err := OpenVaultWithTTL(dir, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if n := again.Len(); n != 0 {
		t.Errorf("vault holds %d entries after the TTL, want 0", n)
	}
	sealedAfter, err := os.ReadFile(filepath.Join(dir, vaultFile))
	if err != nil {
		t.Fatal(err)
	}
	plainAfter, err := open(key, sealedAfter)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plainAfter), kept) {
		t.Error("the expired secret is still in the vault file; expiry hid it from readers and left it at rest")
	}
}

// EV3: an entry inside the TTL is untouched, so a sweep is not a purge.
func TestEV3_ALiveEntrySurvivesASweep(t *testing.T) {
	dir := t.TempDir()
	at := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	restore := now
	now = func() time.Time { return at }
	defer func() { now = restore }()

	v, err := OpenVaultWithTTL(dir, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	ph, err := v.Placeholder("sk-ant-api03-a-real-looking-secret", "anthropic")
	if err != nil {
		t.Fatal(err)
	}
	now = func() time.Time { return at.Add(23 * time.Hour) }
	again, err := OpenVaultWithTTL(dir, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	secret, pattern, ok := again.Secret(ph)
	if !ok || secret != "sk-ant-api03-a-real-looking-secret" || pattern != "anthropic" {
		t.Errorf("a live entry was lost: %q %q %v", secret, pattern, ok)
	}
}

// EV4: expiry is cheap because the placeholder is stable.
//
// This is the reason eviction is affordable at all, and it is worth pinning
// rather than asserting in a comment. The placeholder is the HMAC of the
// secret under the vault key, so a client that re-sends a secret whose entry
// expired gets the identical placeholder and the entry comes back — the
// agent's own transcript still holds the original, so this is the ordinary
// case, not the exception.
//
// PASS: same placeholder, and it rehydrates again.
// FAIL: a different placeholder, which would mean expiry silently changes
// what the provider sees and breaks correlation across a session boundary.
func TestEV4_ARemaskedSecretGetsTheSamePlaceholderBack(t *testing.T) {
	dir := t.TempDir()
	at := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	restore := now
	now = func() time.Time { return at }
	defer func() { now = restore }()

	const s = "sk-ant-api03-a-real-looking-secret"
	v, err := OpenVaultWithTTL(dir, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	first, err := v.Placeholder(s, "anthropic")
	if err != nil {
		t.Fatal(err)
	}
	now = func() time.Time { return at.Add(2 * time.Hour) }
	again, err := OpenVaultWithTTL(dir, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := again.Secret(first); ok {
		t.Fatal("the entry did not expire; the rest of this test would be vacuous")
	}
	second, err := again.Placeholder(s, "anthropic")
	if err != nil {
		t.Fatal(err)
	}
	if second != first {
		t.Errorf("placeholder changed across expiry: %q then %q", first, second)
	}
	if got, _, ok := again.Secret(second); !ok || got != s {
		t.Errorf("the re-masked secret does not rehydrate: %q %v", got, ok)
	}
}

// EV5: a vault written before entries carried a timestamp is not thrown away
// on the first run after an upgrade, and does not live forever either.
//
// An existing vault has no stamps. Reading a missing stamp as "epoch" would
// evict everything the moment a user upgrades, which looks like masking
// breaking. Reading it as "never expires" would leave every pre-upgrade
// secret at rest for the life of the machine, which is the finding. The
// stamp is set on load, so the clock starts at the upgrade.
func TestEV5_ALegacyVaultStartsItsClockAtTheUpgrade(t *testing.T) {
	dir := t.TempDir()
	at := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	restore := now
	now = func() time.Time { return at }
	defer func() { now = restore }()

	// Write a vault in the pre-timestamp shape, by hand.
	v, err := OpenVaultWithTTL(dir, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	ph, err := v.Placeholder("sk-ant-api03-a-real-looking-secret", "anthropic")
	if err != nil {
		t.Fatal(err)
	}
	key, err := os.ReadFile(filepath.Join(dir, keyFile))
	if err != nil {
		t.Fatal(err)
	}
	legacy := `{"` + ph + `":{"s":"sk-ant-api03-a-real-looking-secret","p":"anthropic"}}`
	sealed, err := seal(key, []byte(legacy))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, vaultFile), sealed, filePerm); err != nil {
		t.Fatal(err)
	}

	// The upgrade run: the stamp-less entry is kept.
	upgraded, err := OpenVaultWithTTL(dir, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := upgraded.Secret(ph); !ok {
		t.Error("an unstamped entry was evicted on the first run after the upgrade")
	}
	// A run past the TTL from that point: it is gone.
	now = func() time.Time { return at.Add(2 * time.Hour) }
	later, err := OpenVaultWithTTL(dir, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := later.Secret(ph); ok {
		t.Error("an unstamped entry never expires; the retention is still unbounded")
	}
}

// EV6: the default is a real bound, and zero means no eviction.
//
// The zero case is the escape hatch for anyone who needs rehydration across
// a long gap, and it must be explicit rather than the accidental result of an
// unset field — which is exactly what a `ttl == 0 means forever` default
// would produce for every caller of OpenVault.
func TestEV6_TheDefaultIsBoundedAndZeroIsAnExplicitOptOut(t *testing.T) {
	if DefaultVaultTTL <= 0 {
		t.Fatalf("DefaultVaultTTL = %v; the finding is unbounded retention", DefaultVaultTTL)
	}
	dir := t.TempDir()
	at := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	restore := now
	now = func() time.Time { return at }
	defer func() { now = restore }()

	// OpenVault, the plain constructor, must carry the default.
	v, err := OpenVault(dir)
	if err != nil {
		t.Fatal(err)
	}
	ph, err := v.Placeholder("sk-ant-api03-a-real-looking-secret", "anthropic")
	if err != nil {
		t.Fatal(err)
	}
	now = func() time.Time { return at.Add(DefaultVaultTTL + time.Second) }
	expired, err := OpenVault(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := expired.Secret(ph); ok {
		t.Error("OpenVault does not apply DefaultVaultTTL")
	}

	// Zero: kept.
	other := t.TempDir()
	now = func() time.Time { return at }
	forever, err := OpenVaultWithTTL(other, 0)
	if err != nil {
		t.Fatal(err)
	}
	ph2, err := forever.Placeholder("sk-ant-api03-a-real-looking-secret", "anthropic")
	if err != nil {
		t.Fatal(err)
	}
	now = func() time.Time { return at.Add(100 * 24 * time.Hour) }
	still, err := OpenVaultWithTTL(other, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := still.Secret(ph2); !ok {
		t.Error("a zero TTL evicted; the opt-out does not work")
	}
}

// EV7: the clock seam is not load-bearing in production.
func TestEV7_TheClockDefaultsToTimeNow(t *testing.T) {
	if reflect.ValueOf(now).Pointer() != reflect.ValueOf(time.Now).Pointer() {
		t.Error("the vault clock is not time.Now")
	}
}

// EV8: a long-running proxy evicts too, and persists the eviction.
//
// The sweep on open bounds retention for a proxy that restarts. A proxy that
// runs for a week never reopens the vault, so without this the file keeps
// everything for a week regardless of the TTL — the finding, surviving in the
// case that matters most, because a machine left running is the machine worth
// compromising.
//
// The specific branch: Placeholder is asked for a secret it ALREADY holds, so
// it returns early. If the early return skips the save, the sweep that just
// happened lives only in memory and the expired ciphertext stays on disk.
// guard-reachability reported that branch as unobserved before this test.
//
// PASS: the lapsed secret is gone from the file, without a reopen.
// FAIL: it is still there, which is eviction that only takes effect on
// restart.
func TestEV8_ALongRunningVaultPersistsItsSweep(t *testing.T) {
	dir := t.TempDir()
	at := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	restore := now
	now = func() time.Time { return at }
	defer func() { now = restore }()

	const lapsed = "sk-ant-api03-the-one-that-should-go"
	const fresh = "ghp_theonethatshouldstay0000000000000000"

	v, err := OpenVaultWithTTL(dir, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Placeholder(lapsed, "anthropic"); err != nil {
		t.Fatal(err)
	}
	now = func() time.Time { return at.Add(30 * time.Minute) }
	if _, err := v.Placeholder(fresh, "github"); err != nil {
		t.Fatal(err)
	}

	// Past the first secret's TTL but not the second's. The same live vault
	// object is asked for a placeholder it already holds.
	now = func() time.Time { return at.Add(90 * time.Minute) }
	if _, err := v.Placeholder(fresh, "github"); err != nil {
		t.Fatal(err)
	}

	key, err := os.ReadFile(filepath.Join(dir, keyFile))
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := os.ReadFile(filepath.Join(dir, vaultFile))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := open(key, sealed)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(plain), fresh) {
		t.Fatal("the live secret is not in the file; this test is measuring the wrong thing")
	}
	if strings.Contains(string(plain), lapsed) {
		t.Error("the lapsed secret is still in the vault file: a running proxy evicts in memory only")
	}
}

// EV9: a sweep whose write fails is an error, not a silent in-memory eviction.
//
// The two save() calls that persist a sweep — one on open, one when a
// long-running proxy is asked for a placeholder it already holds — were both
// UNREACHED under guard-reachability. That is the shape that matters here: if
// the write fails and nobody says so, the vault reports fewer entries than it
// holds and the expired ciphertext stays on disk, which is the finding
// surviving while the fix reports success.
//
// A read-only vault directory is the honest trigger: a locked-down home, a
// read-only mount, a container running with a different uid.
func TestEV9_ASweepThatCannotBeWrittenIsAnError(t *testing.T) {
	requireWritableModes(t)
	at := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	restore := now
	now = func() time.Time { return at }
	defer func() { now = restore }()

	// On open.
	dir := t.TempDir()
	v, err := OpenVaultWithTTL(dir, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Placeholder("sk-ant-api03-a-real-looking-secret", "anthropic"); err != nil {
		t.Fatal(err)
	}
	// 0500 carries no group or other bits, so the finding-7 check leaves it
	// alone; it is the owner's own write bit that is gone.
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	now = func() time.Time { return at.Add(2 * time.Hour) }
	if _, err := OpenVaultWithTTL(dir, time.Hour); err == nil {
		t.Error("OpenVault reported success after failing to write the sweep; the expired secret is still on disk")
	}

	// On a live vault answering for a placeholder it already holds.
	other := t.TempDir()
	now = func() time.Time { return at }
	live, err := OpenVaultWithTTL(other, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	const fresh = "ghp_theonethatshouldstay0000000000000000"
	if _, err := live.Placeholder("sk-ant-api03-the-one-that-should-go", "anthropic"); err != nil {
		t.Fatal(err)
	}
	now = func() time.Time { return at.Add(30 * time.Minute) }
	if _, err := live.Placeholder(fresh, "github"); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(other, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(other, 0o700) })
	now = func() time.Time { return at.Add(90 * time.Minute) }
	if _, err := live.Placeholder(fresh, "github"); err == nil {
		t.Error("Placeholder reported success after failing to persist its sweep")
	}
}

// requireWritableModes skips where the owner's write bit is not what decides
// whether a file can be created — Windows, and running as root.
func requireWritableModes(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores the directory write bit")
	}
}
