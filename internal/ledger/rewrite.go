package ledger

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
)

// RewriteLocked replaces a ledger file with kept, the lines of read that an
// erasure keeps, plus anything appended since read was taken. It is
// `replay purge --session`'s rewrite (ADR-0030), and runs entirely under the
// writers' lock: re-read, prefix check, sibling write and rename are one
// critical section, so an Append either lands before the re-read and is
// carried over, or waits and lands in the replacement.
//
// read is the file exactly as the caller matched it. If it is no longer a
// prefix of the file, something other than an append changed the file and
// nothing is removed. writeFile and rename are os.WriteFile and os.Rename in
// production; cmd/replay passes its own indirections so its tests can fail
// them.
func RewriteLocked(path string, read []byte, kept []string, writeFile func(string, []byte, os.FileMode) error, rename func(string, string) error) error {
	f, err := OpenLocked(path, os.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("locking %s: %w", path, err)
	}
	defer f.Close() //nolint:errcheck // read-only handle; closing it only releases the lock
	now, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("re-reading %s: %w", path, err)
	}
	if !bytes.HasPrefix(now, read) {
		return fmt.Errorf("%s changed other than by an append after it was read, so nothing was removed from it; run the erasure again", path)
	}
	appended := now[len(read):]
	if len(appended) > 0 && len(read) > 0 && read[len(read)-1] != '\n' {
		// The walk read half of a record still being written. Its rest is
		// in appended, and the kept lines end with a newline the record
		// would be split across.
		return fmt.Errorf("%s was being written when it was read, so nothing was removed from it; run the erasure again", path)
	}
	out := strings.Join(kept, "\n")
	if out != "" {
		out += "\n"
	}
	out += string(appended)
	// Through a sibling and renamed: a half-written ledger read on the next
	// keystroke would lose every session in the file, not just this one. The
	// fixed name is safe only because the lock is held while it exists.
	tmp := path + ".tmp"
	if werr := writeFile(tmp, []byte(out), 0o600); werr != nil {
		return fmt.Errorf("rewriting %s: %w", path, werr)
	}
	if rerr := rename(tmp, path); rerr != nil {
		return fmt.Errorf("replacing %s: %w", path, rerr)
	}
	return nil
}
