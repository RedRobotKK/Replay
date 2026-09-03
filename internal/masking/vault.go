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
)

// Placeholder shape: a fixed prefix the rehydrator can scan for, and a
// keyed hash so the same secret always maps to the same placeholder and
// the placeholder reveals nothing about the secret.
const (
	PlaceholderPrefix = "BUFFY_SECRET_"
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

// Vault maps placeholders to the secrets they replaced and persists the
// mapping encrypted at rest, so a restart loses nothing.
type Vault struct {
	dir     string
	key     []byte
	mu      sync.Mutex
	secrets map[string]string // placeholder -> secret
}

// OpenVault loads or creates the vault under dir.
func OpenVault(dir string) (*Vault, error) {
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return nil, fmt.Errorf("create vault directory: %w", err)
	}
	key, err := loadOrCreateKey(filepath.Join(dir, keyFile))
	if err != nil {
		return nil, err
	}
	v := &Vault{dir: dir, key: key, secrets: map[string]string{}}
	if err := v.load(); err != nil {
		return nil, err
	}
	return v, nil
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

// Placeholder returns the placeholder for a secret, recording the pair
// when it is new. The placeholder is the HMAC of the secret under the
// vault key, so it is stable across sessions and restarts.
func (v *Vault) Placeholder(secret string) (string, error) {
	mac := hmac.New(sha256.New, v.key)
	mac.Write([]byte(secret))
	ph := PlaceholderPrefix + hex.EncodeToString(mac.Sum(nil))[:placeholderHex]
	v.mu.Lock()
	defer v.mu.Unlock()
	if _, ok := v.secrets[ph]; ok {
		return ph, nil
	}
	v.secrets[ph] = secret
	if err := v.save(); err != nil {
		delete(v.secrets, ph)
		return "", err
	}
	return ph, nil
}

// Secret returns the secret behind a placeholder, if the vault holds it.
func (v *Vault) Secret(placeholder string) (string, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()
	s, ok := v.secrets[placeholder]
	return s, ok
}

// Len is how many secrets the vault holds.
func (v *Vault) Len() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return len(v.secrets)
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
	return json.Unmarshal(plain, &v.secrets)
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
