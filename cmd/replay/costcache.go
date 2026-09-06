package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// An index over transcripts already understood.
//
// `replay cost` over 1,483 transcripts costs 6.3s wall and 19.6s CPU, and
// almost none of that work is new: transcripts are append-only, most never
// change again, and a corpus grows by a handful of sessions a day. Parsing
// 1,483 files to learn about 3 is a full scan of a table that wanted an index.
//
// Caching is the easy half. INVALIDATION is the reason this is a type rather
// than a map: a figure derived by code that has since changed, or priced from a
// table that has since moved, is worse than no cache at all - a wrong number
// that arrives instantly and looks exactly like a right one. So the index is
// keyed on the file's identity AND on a schema string the caller derives from
// everything the figure depends on.

type cachedUnit struct {
	Size int64    `json:"size"`
	Mod  int64    `json:"mod"`
	Unit costUnit `json:"unit"`
	// ReqIDs are the request ids this transcript contributed.
	//
	// Stored because transcript overlap is a CORPUS-level property: the same
	// requestId appears in several files when a sub-agent re-renders its
	// parent's requests, and that can only be seen by comparing files. Cache
	// the per-file figures alone and a warm run reports zero overlap - a
	// disclosure silently dropped, which is worse than the slow run it
	// replaced.
	ReqIDs []string `json:"reqIds,omitempty"`
}

type costCache struct {
	path   string
	schema string
	// Schema is stored so a file written by a differently-configured binary is
	// discarded wholesale rather than consulted entry by entry.
	entries map[string]cachedUnit
	dirty   bool
}

type costCacheFile struct {
	Schema  string                `json:"schema"`
	Entries map[string]cachedUnit `json:"entries"`
}

func newCostCache(path, schema string) *costCache {
	return &costCache{path: path, schema: schema, entries: map[string]cachedUnit{}}
}

// load reads the index, or leaves it empty.
//
// Every failure is a cache miss and never an error: a corrupt, unreadable or
// stale index must degrade to the cold run that would have happened anyway.
// Returning an error here would let a scratch file break the whole command.
func (c *costCache) load() error {
	b, err := os.ReadFile(c.path)
	if err != nil {
		return nil
	}
	var f costCacheFile
	if json.Unmarshal(b, &f) != nil {
		return nil
	}
	if f.Schema != c.schema || f.Entries == nil {
		return nil
	}
	c.entries = f.Entries
	return nil
}

// stat returns the identity of a file: size and modification time.
func stat(path string) (int64, int64, bool) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, 0, false
	}
	return fi.Size(), fi.ModTime().UnixNano(), true
}

// get returns the stored figure when the file is byte-identical by size and
// mtime.
//
// Size is checked as well as mtime because a restore from backup, a checkout or
// clock skew all produce a changed file under an unchanged timestamp, and an
// index that trusts a timestamp alone trusts anything that can set one.
func (c *costCache) get(path string) (costUnit, []string, bool) {
	e, ok := c.entries[path]
	if !ok {
		return costUnit{}, nil, false
	}
	size, mod, ok := stat(path)
	if !ok || e.Size != size || e.Mod != mod {
		return costUnit{}, nil, false
	}
	return e.Unit, e.ReqIDs, true
}

func (c *costCache) put(path string, u costUnit, reqIDs []string) {
	size, mod, ok := stat(path)
	if !ok {
		return
	}
	c.entries[path] = cachedUnit{Size: size, Mod: mod, Unit: u, ReqIDs: reqIDs}
	c.dirty = true
}

// save writes the index, atomically. A failure to persist costs a slow next
// run and nothing else, so it is reported but never fatal to the caller.
func (c *costCache) save() error {
	if !c.dirty {
		return nil
	}
	b, err := json.Marshal(costCacheFile{Schema: c.schema, Entries: c.entries})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.path), 0o700); err != nil {
		return err
	}
	tmp := c.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, c.path)
}
