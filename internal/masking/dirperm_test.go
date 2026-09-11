package masking

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Finding 7 of the 2026-09-04 adversarial security review, at the vault.
//
// The vault directory is the sharper half of the finding, because finding 3
// records that the key file sits beside the ciphertext: "Anyone who can read
// `~/.replay/vault` can read the key next to it." A directory left at 0777 by
// os.MkdirAll's no-op on an existing path is precisely how another local
// account comes to be able to read both, and it is what turns masking from a
// protection into a second copy of every secret the proxy has seen.

// VP1: OpenVault over a pre-existing 0777 directory leaves it private.
func TestVP1_OpenVaultTightensAPreExistingWorldWritableDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits")
	}
	dir := filepath.Join(t.TempDir(), "vault")
	if err := os.Mkdir(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenVault(dir); err != nil {
		t.Fatalf("OpenVault: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got&0o077 != 0 {
		t.Errorf("vault directory left at %04o after OpenVault", got)
	}
}

// VP2: a pre-existing vault key and vault file that others can read are
// tightened.
//
// The key decrypts the ciphertext beside it. Both are checked, because
// tightening one and not the other protects nothing.
func TestVP2_OpenVaultTightensAWorldReadableKeyAndCiphertext(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits")
	}
	dir := t.TempDir()
	// Create a real vault first, so the ciphertext is one this code wrote.
	v, err := OpenVault(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Placeholder("sk-ant-a-real-looking-secret", "anthropic"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{keyFile, vaultFile} {
		if err := os.Chmod(filepath.Join(dir, name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := OpenVault(dir); err != nil {
		t.Fatalf("re-open: %v", err)
	}
	for _, name := range []string{keyFile, vaultFile} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got&0o077 != 0 {
			t.Errorf("%s left at %04o after OpenVault", name, got)
		}
	}
}

// VP3: a vault directory that cannot be made private stops OpenVault.
//
// Same reasoning as the ledger's LP3, and sharper here: what would be written
// into the directory is the key that decrypts every secret the proxy has
// masked.
func TestVP3_AVaultDirectoryThatCannotBeCreatedStopsOpenVault(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenVault(filepath.Join(file, "vault")); err == nil {
		t.Fatal("OpenVault succeeded with a regular file in the path of its directory")
	} else if !strings.Contains(err.Error(), "vault directory") {
		t.Errorf("the error must say which directory failed; got %q", err)
	}
}

// VP4: a vault key that cannot be read is a refusal, not a new key.
//
// loadOrCreateKey reads any failure as "no key yet" and generates one. On the
// vault that is worse than on the ledger: a new key cannot decrypt the
// ciphertext sitting next to it, so OpenVault would then fail at "decrypt
// vault" and the operator would be looking at a corruption message for what is
// really an unreadable key file.
func TestVP4_AnUnreadableVaultKeyStopsOpenVault(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink loops")
	}
	dir := t.TempDir()
	key := filepath.Join(dir, keyFile)
	if err := os.Symlink(key, key); err != nil {
		t.Skipf("cannot create a symlink loop here: %v", err)
	}
	// The prefix, not a substring: neutralise the EnsureFile check and
	// loadOrCreateKey fails its own write on the same loop and returns
	// "write vault key: ...", which also contains "vault". That is what
	// guard-reachability flags as INERT — the branch runs and nothing depends
	// on whether it did.
	if _, err := OpenVault(dir); err == nil {
		t.Fatal("OpenVault succeeded with an unreadable key file")
	} else if !strings.HasPrefix(err.Error(), "vault: check file") {
		t.Errorf("the key must be refused before loadOrCreateKey decides to generate a new one; got %q", err)
	}
}
