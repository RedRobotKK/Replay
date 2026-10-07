package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// toolTreeGoFiles are the directories the measurement tool for the TTL study
// lives in: this program, and the transcript reader its arm-membership rule
// depends on. Named here, once, so startBlock and endBlock capture exactly
// the same tree.
var toolTreeGoFiles = []string{"scripts/ttl-block", "internal/transcript"}

// toolTreeSHA256 computes a sha256 over the measurement tool: the sorted,
// concatenated contents of every .go file under the given root directories,
// read directly rather than through git or any other subprocess (this
// repository keeps os/exec out of everything but the mutation harness).
//
// Scoped to .go files on purpose: an editor artifact (.DS_Store), a compiled
// test binary, or any other non-source file that can appear in a working
// tree must not move the hash, because none of them is the measurement
// tool's own logic. A file that cannot be read fails the whole call with an
// explicit error rather than being silently left out of the digest, because
// a hash that omits an unreadable file is indistinguishable from one that
// read it and found no change.
//
// files is every hashed file's path (as the caller named its root) mapped to
// that file's own sha256, so a mismatch between two calls can name exactly
// which files changed rather than only disclosing that something did.
func toolTreeSHA256(roots ...string) (sum string, files map[string]string, err error) {
	var all []string
	for _, root := range roots {
		walked, err := listGoFiles(root)
		if err != nil {
			return "", nil, err
		}
		all = append(all, walked...)
	}
	sort.Strings(all)

	files = make(map[string]string, len(all))
	h := sha256.New()
	for _, f := range all {
		b, err := os.ReadFile(f)
		if err != nil {
			return "", nil, fmt.Errorf("reading %s for the tool-tree hash: %w", f, err)
		}
		files[f] = bytesSHA256(b)
		// The path is written into the digest ahead of the content, with a
		// separator that cannot appear in a path, so a file renamed without
		// its content changing still moves the hash, and two trees that
		// happen to share file bytes under different names do not collide.
		h.Write([]byte(f))
		h.Write([]byte{0})
		h.Write(b)
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), files, nil
}

// listGoFiles returns every regular .go file under root, in no particular
// order (toolTreeSHA256 sorts the combined result itself).
func listGoFiles(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walking %s for the tool-tree hash: %w", path, err)
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		out = append(out, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// toolTreeDiff names every path whose hash differs between two tool-tree
// snapshots, or that exists in only one of them, sorted.
func toolTreeDiff(before, after map[string]string) []string {
	changed := map[string]bool{}
	for p, h := range before {
		if after[p] != h {
			changed[p] = true
		}
	}
	for p, h := range after {
		if before[p] != h {
			changed[p] = true
		}
	}
	out := make([]string, 0, len(changed))
	for p := range changed {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}
