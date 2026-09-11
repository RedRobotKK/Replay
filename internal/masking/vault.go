package masking

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/RedRobotKK/Replay/internal/ownerdir"
)

// Placeholder shape: a fixed prefix the rehydrator can scan for, and a
// keyed hash so the same secret always maps to the same placeholder and
// the placeholder reveals nothing about the secret.
const (
	PlaceholderPrefix = "REPLAY_SECRET_"
	placeholderHex    = 16
	// PlaceholderLength is the full placeholder length, which a stream
	// rehydrator holds back at chunk boundaries.
	PlaceholderLength = len(PlaceholderPrefix) + placeholderHex
)

// Vault files. The key file is owner-only; the vault is AES-256-GCM
// under that key. Holding the key in the operating system keychain is a
// follow-up (MK-3); until then the key file is the boundary.
const (
	keyFile   = ".vault-key"
	vaultFile = "vault"
	keyBytes  = 32
	dirPerm   = 0o700
	filePerm  = 0o600
)

// DefaultVaultTTL bounds how long a masked secret stays at rest.
//
// Finding 3 of the 2026-09-04 security review: masking converts transient
// secrets into secrets AT REST, and there was no eviction, so the vault
// accumulated every secret the proxy had ever masked for the life of the
// machine. The key file sits beside the ciphertext, so a host compromised on
// day 200 handed over 200 days of credentials rather than a day of them.
// RELEASE-CRITERIA.md offers three ways to discharge the finding and asks for
// one; this is the second, "vault entries expire".
//
// A day, because that is the span over which rehydration is actually used —
// a placeholder is restored seconds after it is created, in the response to
// the request that produced it — and because expiry is nearly free: the
// placeholder is the HMAC of the secret, so a client re-sending a secret
// whose entry lapsed gets the same placeholder back and the entry returns.
// The only thing lost is rehydrating a placeholder whose secret has not been
// seen in a day.
//
// Not a claim that the vault is safe within the day. The key is still next to
// the ciphertext; see the package's eviction_test.go for what this does and
// does not buy.
const DefaultVaultTTL = 24 * time.Hour

// now is the vault's clock, indirected so expiry can be tested without
// sleeping. A test that waits out a real TTL is a test nobody runs.
var now = time.Now

// Vault maps placeholders to the secrets they replaced and persists the
// mapping encrypted at rest, so a restart loses nothing.
//
// A secret is held exactly as it appeared inside the request's JSON string
// literal, escapes included, so restoring it into a response string
// literal is a direct substitution.
type Vault struct {
	dir string
	key []byte
	// ttl is how long an entry survives. Zero disables eviction, which is an
	// explicit choice a caller makes rather than a zero value it inherits:
	// OpenVault applies DefaultVaultTTL.
	ttl     time.Duration
	mu      sync.Mutex
	secrets map[string]entry // placeholder -> secret and pattern
}

// entry is one vault record. The pattern name lets rehydration apply
// per-pattern scope to a placeholder found in a response.
type entry struct {
	Secret  string `json:"s"`
	Pattern string `json:"p,omitempty"`
	// At is when this secret was first vaulted, in Unix seconds. Absent in a
	// vault written before eviction existed; load stamps those with the time
	// of the first run that reads them, so an upgrade neither throws the
	// vault away nor leaves it unbounded.
	At int64 `json:"t,omitempty"`
}

// OpenVault loads or creates the vault under dir, with the default retention.
func OpenVault(dir string) (*Vault, error) {
	return OpenVaultWithTTL(dir, DefaultVaultTTL)
}

// OpenVaultWithTTL loads or creates the vault under dir and evicts entries
// older than ttl. A ttl of zero or less disables eviction.
//
// The sweep happens here, on open, rather than only on write: a vault whose
// proxy is not running is exactly the vault an attacker copies, and the next
// `replay serve` is the last moment before it is read again.
func OpenVaultWithTTL(dir string, ttl time.Duration) (*Vault, error) {
	// Create it if it is missing and VERIFY it if it is not. Finding 7: on an
	// existing directory os.MkdirAll ignores its mode, so a 0777 ~/.replay/vault
	// stayed 0777 while this line read as though it had set 0700. The key file
	// and the ciphertext are checked by name too, because os.WriteFile leaves
	// an existing file's mode alone and finding 3 already records that the key
	// sits next to what it decrypts.
	if err := ownerdir.EnsureDir(dir); err != nil {
		return nil, fmt.Errorf("vault directory: %w", err)
	}
	for _, name := range []string{keyFile, vaultFile} {
		if err := ownerdir.EnsureFile(filepath.Join(dir, name)); err != nil {
			return nil, fmt.Errorf("vault: %w", err)
		}
	}
	key, err := loadOrCreateKey(filepath.Join(dir, keyFile))
	if err != nil {
		return nil, err
	}
	v := &Vault{dir: dir, key: key, ttl: ttl, secrets: map[string]entry{}}
	if err := v.load(); err != nil {
		return nil, err
	}
	// Sweep and persist. Rewriting only when something changed keeps an
	// unchanged vault's mtime and bytes alone, so "the file changed" stays a
	// signal.
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.sweepLocked() {
		if err := v.save(); err != nil {
			return nil, err
		}
	}
	return v, nil
}

// sweepLocked drops expired entries and stamps unstamped ones, reporting
// whether the map changed. Callers hold the lock.
func (v *Vault) sweepLocked() bool {
	changed := false
	for ph, e := range v.secrets {
		if e.At == 0 {
			e.At = now().Unix()
			v.secrets[ph] = e
			changed = true
			continue
		}
		if v.expired(e) {
			delete(v.secrets, ph)
			changed = true
		}
	}
	return changed
}

// expired reports whether an entry has outlived the retention window.
func (v *Vault) expired(e entry) bool {
	if v.ttl <= 0 {
		return false
	}
	return now().Sub(time.Unix(e.At, 0)) > v.ttl
}

func loadOrCreateKey(path string) ([]byte, error) {
	key, err := os.ReadFile(path)
	if err == nil && len(key) == keyBytes {
		return key, nil
	}
	key = make([]byte, keyBytes)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate vault key: %w", err)
	}
	if err := os.WriteFile(path, key, filePerm); err != nil {
		return nil, fmt.Errorf("write vault key: %w", err)
	}
	return key, nil
}

// Placeholder returns the placeholder for a secret matched by the named
// pattern, recording the pair when it is new. The placeholder is the HMAC
// of the secret under the vault key, so it is stable across sessions and
// restarts, and it depends on the secret alone, not on the pattern.
func (v *Vault) Placeholder(secret, pattern string) (string, error) {
	mac := hmac.New(sha256.New, v.key)
	mac.Write([]byte(secret))
	ph := PlaceholderPrefix + hex.EncodeToString(mac.Sum(nil))[:placeholderHex]
	v.mu.Lock()
	defer v.mu.Unlock()
	// Sweep before answering, so a long-running proxy evicts too rather than
	// only a restarting one.
	swept := v.sweepLocked()
	if _, ok := v.secrets[ph]; ok {
		if swept {
			if err := v.save(); err != nil {
				return "", err
			}
		}
		return ph, nil
	}
	v.secrets[ph] = entry{Secret: secret, Pattern: pattern, At: now().Unix()}
	if err := v.save(); err != nil {
		delete(v.secrets, ph)
		return "", err
	}
	return ph, nil
}

// Secret returns the secret behind a placeholder and the pattern that
// matched it, if the vault holds it.
func (v *Vault) Secret(placeholder string) (secret, pattern string, ok bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	e, ok := v.secrets[placeholder]
	if !ok || v.expired(e) {
		return "", "", false
	}
	return e.Secret, e.Pattern, true
}

// Len is how many secrets the vault holds.
func (v *Vault) Len() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	n := 0
	for _, e := range v.secrets {
		if !v.expired(e) {
			n++
		}
	}
	return n
}

// save writes the whole map encrypted, to a temporary file first so a
// crash mid-write leaves the previous vault intact. Callers hold the lock.
func (v *Vault) save() error {
	plain, err := json.Marshal(v.secrets)
	if err != nil {
		return fmt.Errorf("encode vault: %w", err)
	}
	sealed, err := seal(v.key, plain)
	if err != nil {
		return err
	}
	path := filepath.Join(v.dir, vaultFile)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, sealed, filePerm); err != nil {
		return fmt.Errorf("write vault: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("replace vault: %w", err)
	}
	return nil
}

func (v *Vault) load() error {
	sealed, err := os.ReadFile(filepath.Join(v.dir, vaultFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read vault: %w", err)
	}
	plain, err := open(v.key, sealed)
	if err != nil {
		return fmt.Errorf("decrypt vault: %w", err)
	}
	if err := json.Unmarshal(plain, &v.secrets); err == nil {
		return nil
	}
	// A vault written before patterns were recorded holds bare secrets;
	// they rehydrate under the default scope.
	var bare map[string]string
	if err := json.Unmarshal(plain, &bare); err != nil {
		return fmt.Errorf("decode vault: %w", err)
	}
	for ph, secret := range bare {
		v.secrets[ph] = entry{Secret: secret}
	}
	// Left unstamped on purpose: the sweep in OpenVaultWithTTL stamps them,
	// so the retention clock on a pre-eviction vault starts at the upgrade.
	return nil
}

func seal(key, plain []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("vault nonce: %w", err)
	}
	return append(nonce, gcm.Seal(nil, nonce, plain, nil)...), nil
}

func open(key, sealed []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(sealed) < gcm.NonceSize() {
		return nil, errors.New("vault too short")
	}
	return gcm.Open(nil, sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():], nil)
}
