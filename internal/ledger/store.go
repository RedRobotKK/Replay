package ledger

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/RedRobotKK/Replay/internal/ownerdir"
	"github.com/RedRobotKK/Replay/internal/tenancy"
	"github.com/RedRobotKK/Replay/internal/transcript"
)

// File permissions: the ledger is the user's own data and nobody else's.
const (
	dirPerm  = 0o700
	filePerm = 0o600
)

// labelKeyFile holds the per-ledger key that keys path hashes in labels.
const (
	labelKeyFile  = ".label-key"
	labelKeyBytes = 32
)

// probeBytes is how much of a file IsLedgerFile reads.
const probeBytes = 4096

// safeName restricts session ids to characters that are safe in a file name.
var safeName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// Store appends records to one file per session.
type Store struct {
	dir     string
	labeler *Labeler
	mu      sync.Mutex
	// pins is nested by tenant exactly like internal/proxy's stats.sessions
	// and SpendGuard.dayUsed (SP-6, SP-9): a persisted policy pin is owned
	// by (tenant, sessionID), never by sessionID alone (SP-10). A
	// session id is a client-chosen string this proxy never issues or
	// namespaces, so two tenants that happen to send the same one must
	// not land in one shared Pin and inherit each other's policy decision.
	pins     map[tenancy.TenantID]map[string]Pin
	revert   Revert
	reverted bool
}

// Open creates the ledger directory and its label key if needed.
func Open(dir string) (*Store, error) {
	// Create it if it is missing and VERIFY it if it is not: os.MkdirAll
	// applies its mode only when it creates, so a pre-existing world-writable
	// ~/.replay/ledger used to be accepted in silence. Finding 7.
	if err := ownerdir.EnsureDir(dir); err != nil {
		return nil, fmt.Errorf("ledger directory: %w", err)
	}
	keyPath := filepath.Join(dir, labelKeyFile)
	// The label key is what makes the hashed path labels and the tool call
	// keys anonymous, so it is checked by name rather than left to the
	// directory: os.WriteFile does not change the mode of a file that is
	// already there.
	if err := ownerdir.EnsureFile(keyPath); err != nil {
		return nil, fmt.Errorf("ledger label key: %w", err)
	}
	key, err := loadOrCreateKey(keyPath)
	if err != nil {
		return nil, err
	}
	pins, err := loadPins(filepath.Join(dir, pinsFile))
	if err != nil {
		return nil, err
	}
	revert, reverted := loadRevert(filepath.Join(dir, revertFile))
	return &Store{dir: dir, labeler: NewLabeler(key), pins: pins, revert: revert, reverted: reverted}, nil
}

// errCorruptLabelKey is returned when .label-key exists but does not contain
// exactly labelKeyBytes. The original bytes are left untouched so an operator
// can recover or investigate; silently re-keying would make prior path labels
// and tool-call keys unverifiable against the evidence that used the old key.
var errCorruptLabelKey = errors.New("ledger label key has invalid length")

func loadOrCreateKey(path string) ([]byte, error) {
	key, err := os.ReadFile(path)
	if err == nil {
		if len(key) == labelKeyBytes {
			return key, nil
		}
		// File exists but is the wrong size. Refuse and preserve the original
		// bytes. Treat empty (including a zero-byte artifact) the same way:
		// EnsureFile does not create the file, so a present empty path is not
		// "missing" — it is corrupt identity material.
		return nil, fmt.Errorf("%w: got %d bytes, want %d; original bytes preserved at %s",
			errCorruptLabelKey, len(key), labelKeyBytes, path)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read label key: %w", err)
	}
	// Missing: create a new key.
	key = make([]byte, labelKeyBytes)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate label key: %w", err)
	}
	if err := os.WriteFile(path, key, filePerm); err != nil {
		return nil, fmt.Errorf("write label key: %w", err)
	}
	return key, nil
}

// Labeler is the store's content-free labeler.
func (s *Store) Labeler() *Labeler { return s.labeler }
