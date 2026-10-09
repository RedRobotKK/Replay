// ttl-block starts and ends the blocks of the preregistered cache-TTL field
// study, docs/evidence/ttl-register-2026-10-05.md, and records each as it
// happened.
//
// It is experiment tooling, not the product: `replay advise --apply` cannot
// start the treatment arm on the corpus the study runs on (it proposes 1h,
// which is the automatic default there), so the treatment is written here,
// to the Claude Code settings file, and read back before a block counts as
// started. Nothing in this program reads a transcript or an outcome.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	settingKey  = "promptCacheTtl"
	armTreat    = "treatment"
	armControl  = "control"
	treatValue  = "5m"
	schemaBlock = "replay.ttl-block.v1"
)

// schedule is the frozen block order, derived from the register's hash.
type schedule struct {
	Schema         string   `json:"schema"`
	RegisterSHA256 string   `json:"register_sha256"`
	Method         string   `json:"method"`
	Blocks         []string `json:"blocks"`
}

// blockRecord is one event in the block log, as it happened.
type blockRecord struct {
	Schema          string `json:"schema"`
	Event           string `json:"event"`
	Block           int    `json:"block"`
	Arm             string `json:"arm"`
	Mechanism       string `json:"mechanism"`
	At              string `json:"at"`
	SettingsPath    string `json:"settings_path"`
	PriorValue      string `json:"prior_value,omitempty"`
	RequestedValue  string `json:"requested_value,omitempty"`
	ConfiguredValue string `json:"configured_value,omitempty"`
	StateChange     string `json:"state_change,omitempty"`
	BackupPath      string `json:"backup_path,omitempty"`
	SettingsSHA256  string `json:"settings_sha256"`
	SpanIntact      *bool  `json:"span_intact,omitempty"`
	RegisterSHA256  string `json:"register_sha256"`
	ScheduleSHA256  string `json:"schedule_sha256"`
	ToolCommit      string `json:"tool_commit"`
	Reason          string `json:"reason,omitempty"`

	// ToolTreeSHA256 is the sha256 of the measurement tool itself (toolTreeSHA256
	// over toolTreeGoFiles), captured at BLOCK_STARTED and BLOCK_ENDED.
	// UNAVAILABLE when it could not be computed (see ToolTreeError), distinct
	// from SettingsSHA256 and SpanIntact above: that pair is about the
	// settings file this study writes, this pair is about the code that
	// measures it. Neither is voided by a change in the other.
	ToolTreeSHA256 string `json:"tool_tree_sha256,omitempty"`
	// ToolTreeFiles is every hashed file mapped to its own sha256, carried so
	// a later BLOCK_ENDED can name exactly what changed rather than only
	// disclosing that something did.
	ToolTreeFiles map[string]string `json:"tool_tree_files,omitempty"`
	// ToolTreeError explains a ToolTreeSHA256 of UNAVAILABLE.
	ToolTreeError string `json:"tool_tree_error,omitempty"`
	// ToolTreeIntact is set only on BLOCK_ENDED, when both the start and end
	// hashes were computed: whether the measurement tool was unchanged across
	// the block. nil when either hash is UNAVAILABLE, because intactness is
	// then not known, not false. A false value does NOT void the block and
	// excludes no session on its own; it is disclosed, and analysis of the
	// block's sessions is pinned to the tool_commit recorded at BLOCK_STARTED.
	ToolTreeIntact *bool `json:"tool_tree_intact,omitempty"`
	// ToolTreeChangedFiles names every path whose hash differed between the
	// block's start and end snapshots, set only when ToolTreeIntact is false.
	ToolTreeChangedFiles []string `json:"tool_tree_changed_files,omitempty"`
}

// readBack is readSetting behind a variable, so a test can make the read
// disagree with the write. marshalJSON and marshalIndent are the encoder
// behind variables for the same reason: nothing in this program can hand
// them a value they refuse, and the refusal paths still have to be seen.
//
// readLogForFrozen and readLogAgain are readLog behind two further
// variables, one for frozen's own read of the log (the one historyAgrees
// checks against) and one for startBlock's and endBlock's later read of the
// same file, to get the records they act on. Nothing in this program
// writes to the log between those two reads, so in production the two
// variables are both readLog and always agree with each other; a test can
// set either one apart from the other to drive the read-failure path on
// just one of them, standing in for the log becoming unreadable (or
// readable again) in the window between the two reads, without needing an
// actual race.
var (
	readBack         = readSetting
	marshalJSON      = json.Marshal
	marshalIndent    = json.MarshalIndent
	readLogForFrozen = readLog
	readLogAgain     = readLog
)

// readSetting opens the settings file again and reports the value it holds,
// UNSET when the key is absent, or false when it cannot be read as JSON.
func readSetting(path string) (string, bool) {
	// A file that cannot be read leaves b nil, and the decoder refuses nil
	// the same way it refuses anything that is not JSON.
	b, _ := os.ReadFile(path)
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return "", false
	}
	v, ok := m[settingKey].(string)
	if !ok {
		return "UNSET", true
	}
	return v, true
}

func fileSHA256(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return bytesSHA256(b), nil
}

func bytesSHA256(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// toolCommit names the commit this tool ran from, so a block record can be
// tied to the code that wrote it, or UNAVAILABLE when the repository cannot
// say. Read from the git files, never by running git: this repository keeps
// os/exec out of everything but the mutation harness (cmd/replay X6c).
var toolCommit = func() string {
	// A working directory that cannot be determined is the empty string,
	// which the walk below exhausts at once and reports UNAVAILABLE.
	wd, _ := os.Getwd()
	return gitHead(wd)
}

// gitHead resolves HEAD for the repository or worktree containing dir, an
// absolute path: a detached hash, a loose ref file, or a packed ref.
// UNAVAILABLE otherwise.
func gitHead(dir string) string {
	gitDir, commonDir := gitDirs(dir)
	if gitDir == "" {
		return "UNAVAILABLE"
	}
	head, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return "UNAVAILABLE"
	}
	line := strings.TrimSpace(string(head))
	if !strings.HasPrefix(line, "ref: ") {
		return line
	}
	ref := strings.TrimPrefix(line, "ref: ")
	if b, err := os.ReadFile(filepath.Join(commonDir, ref)); err == nil {
		return strings.TrimSpace(string(b))
	}
	// No packed-refs file reads as no lines, and no line names the ref.
	packed, _ := os.ReadFile(filepath.Join(commonDir, "packed-refs"))
	for _, l := range strings.Split(string(packed), "\n") {
		if strings.HasSuffix(l, " "+ref) {
			return strings.Fields(l)[0]
		}
	}
	return "UNAVAILABLE"
}

// gitDirs finds the .git directory for dir, following a worktree's gitdir
// file, and the common directory that holds refs and packed-refs.
func gitDirs(dir string) (gitDir, commonDir string) {
	for d := dir; ; d = filepath.Dir(d) {
		candidate := filepath.Join(d, ".git")
		info, err := os.Stat(candidate)
		if err == nil {
			if info.IsDir() {
				return candidate, candidate
			}
			// A .git file that cannot be read names no gitdir, and an empty
			// gitdir resolves to nothing below.
			b, _ := os.ReadFile(candidate)
			g := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(b)), "gitdir:"))
			if c, err := os.ReadFile(filepath.Join(g, "commondir")); err == nil {
				return g, filepath.Join(g, strings.TrimSpace(string(c)))
			}
			return g, g
		}
		if parent := filepath.Dir(d); parent == d {
			return "", ""
		}
	}
}

// makeSchedule derives the block order from the register's hash: block 1 is
// the treatment when the digest's first byte is even and the control when it
// is odd, and the arms alternate from there. Any researcher with the frozen
// register reproduces it; nobody chooses it.
func makeSchedule(registerSHA string, blocks int) (schedule, error) {
	raw, err := hex.DecodeString(registerSHA)
	if err != nil || len(raw) != 32 {
		return schedule{}, fmt.Errorf("register hash must be 64 hex characters: %q", registerSHA)
	}
	if blocks < 6 {
		return schedule{}, fmt.Errorf("the register requires at least 6 blocks, got %d", blocks)
	}
	first := armTreat
	if raw[0]%2 == 1 {
		first = armControl
	}
	s := schedule{Schema: "replay.ttl-schedule.v1", RegisterSHA256: registerSHA,
		Method: "sha256 of the frozen register; block 1 is treatment when the first byte of the digest is even, control when odd; arms alternate"}
	for i := 0; i < blocks; i++ {
		arm := first
		if i%2 == 1 {
			arm = other(first)
		}
		s.Blocks = append(s.Blocks, arm)
	}
	return s, nil
}

func other(arm string) string {
	if arm == armTreat {
		return armControl
	}
	return armTreat
}

// readLog returns every record in the block log, oldest first.
func readLog(path string) ([]blockRecord, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading the block log: %w", err)
	}
	var out []blockRecord
	for _, line := range splitLines(b) {
		var r blockRecord
		if err := json.Unmarshal(line, &r); err != nil {
			return nil, fmt.Errorf("block log is not one JSON object per line: %w", err)
		}
		out = append(out, r)
	}
	return out, nil
}

func splitLines(b []byte) [][]byte {
	var out [][]byte
	start := 0
	for i, c := range b {
		if c == '\n' {
			if i > start {
				out = append(out, b[start:i])
			}
			start = i + 1
		}
	}
	if start < len(b) {
		out = append(out, b[start:])
	}
	return out
}

func appendRecord(path string, r blockRecord) error {
	body, err := marshalJSON(r)
	if err != nil {
		return fmt.Errorf("encoding the block record: %w", err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, werr := f.Write(append(body, '\n'))
	return errors.Join(werr, f.Close())
}

// paths is everything a block operation needs to find.
type paths struct {
	settings, register, schedule, log string
	// toolTreeRoots names the directories hashed for ToolTreeSHA256. Carried
	// on paths, like every other location this program reads or writes,
	// rather than as a package-level constant, so a test can point it at a
	// temporary tree instead of this repository's own source.
	toolTreeRoots []string
}

// captureToolTree hashes the measurement tool and reports it the way a
// blockRecord carries it: a sum and its per-file breakdown, or UNAVAILABLE
// and the reason when the hash could not be computed. It never returns an
// error itself, because a disclosure failure must not be confused with the
// settings-file failures that startBlock and endBlock do return as errors.
func captureToolTree(p paths) (sum string, files map[string]string, errMsg string) {
	sum, files, err := toolTreeSHA256(p.toolTreeRoots...)
	if err != nil {
		return "UNAVAILABLE", nil, err.Error()
	}
	return sum, files, ""
}

// startBlock puts the settings file into the arm the schedule names for the
// block, reads it back, and records the block as started only when the
// read-back agrees. A disagreement is recorded as UNVERIFIED and is an error:
// the block has not started.
func startBlock(p paths, block int, arm string, now time.Time) (blockRecord, error) {
	sched, schedSHA, regSHA, err := frozen(p)
	if err != nil {
		return blockRecord{}, err
	}
	if block < 1 || block > len(sched.Blocks) {
		return blockRecord{}, fmt.Errorf("block %d is not in the schedule of %d", block, len(sched.Blocks))
	}
	if sched.Blocks[block-1] != arm {
		return blockRecord{}, fmt.Errorf("the schedule names block %d as %s, not %s; the order was fixed before block 1 and is not chosen now", block, sched.Blocks[block-1], arm)
	}
	log, err := readLogAgain(p.log)
	if err != nil {
		return blockRecord{}, err
	}
	if err := mayStart(log, block); err != nil {
		return blockRecord{}, err
	}

	settings := map[string]any{}
	existing, err := os.ReadFile(p.settings)
	backup, prior := "NONE", "UNSET"
	switch {
	case err == nil:
		if err := json.Unmarshal(existing, &settings); err != nil {
			return blockRecord{}, fmt.Errorf("%s is not valid JSON, so it will not be modified: %w", p.settings, err)
		}
		if v, ok := settings[settingKey].(string); ok {
			prior = v
		}
		backup = fmt.Sprintf("%s.bak-%s", p.settings, now.UTC().Format("20060102T150405Z"))
		if err := os.WriteFile(backup, existing, 0o600); err != nil {
			return blockRecord{}, fmt.Errorf("backup not written, so the settings file was not touched: %w", err)
		}
	case !errors.Is(err, os.ErrNotExist):
		return blockRecord{}, err
	default:
		if err := os.MkdirAll(filepath.Dir(p.settings), 0o700); err != nil {
			return blockRecord{}, err
		}
	}
	requested := "UNSET"
	if arm == armTreat {
		requested = treatValue
		settings[settingKey] = treatValue
	} else {
		delete(settings, settingKey)
	}
	body, err := marshalIndent(settings, "", "  ")
	if err != nil {
		return blockRecord{}, fmt.Errorf("encoding the settings, so nothing was written: %w", err)
	}
	written := make([]byte, 0, len(body)+1)
	written = append(written, body...)
	written = append(written, '\n')
	if err := os.WriteFile(p.settings, written, 0o600); err != nil {
		return blockRecord{}, err
	}
	configured, readable := readBack(p.settings)
	toolSum, toolFiles, toolErr := captureToolTree(p)
	rec := blockRecord{Schema: schemaBlock, Event: "BLOCK_STARTED", Block: block, Arm: arm,
		Mechanism: "settings-file", At: now.UTC().Format(time.RFC3339), SettingsPath: p.settings,
		PriorValue: prior, RequestedValue: requested, ConfiguredValue: configured, StateChange: "VERIFIED",
		BackupPath: backup, SettingsSHA256: bytesSHA256(written), RegisterSHA256: regSHA, ScheduleSHA256: schedSHA,
		ToolCommit: toolCommit(), ToolTreeSHA256: toolSum, ToolTreeFiles: toolFiles, ToolTreeError: toolErr}
	switch {
	case !readable:
		rec.Event, rec.StateChange, rec.ConfiguredValue = "BLOCK_NOT_STARTED", "UNVERIFIED", "UNREADABLE"
	case configured != requested:
		rec.Event, rec.StateChange = "BLOCK_NOT_STARTED", "UNVERIFIED"
	}
	if err := appendRecord(p.log, rec); err != nil {
		return rec, err
	}
	if rec.StateChange != "VERIFIED" {
		return rec, fmt.Errorf("block %d did not start: the settings file reads %s after the write, not %s", block, rec.ConfiguredValue, requested)
	}
	return rec, nil
}

// mayStart refuses a block that already started, and any block while an
// earlier one is still open.
func mayStart(log []blockRecord, block int) error {
	for _, r := range log {
		if r.Event == "BLOCK_STARTED" && r.Block == block {
			return fmt.Errorf("block %d already started at %s", block, r.At)
		}
	}
	if open := openBlock(log); open != 0 && open != block {
		return fmt.Errorf("block %d is still open; end it before starting block %d", open, block)
	}
	return nil
}

// openBlock is the block that started and has not ended, or 0.
func openBlock(log []blockRecord) int {
	open := 0
	for _, r := range log {
		switch r.Event {
		case "BLOCK_STARTED":
			open = r.Block
		case "BLOCK_ENDED":
			if r.Block == open {
				open = 0
			}
		}
	}
	return open
}

// endBlock records the end of the open block and whether the settings file
// is still the file the block started with. A changed file does not void
// the record; it marks the span as not intact, and the analysis excludes
// what it must.
func endBlock(p paths, block int, now time.Time) (blockRecord, error) {
	_, schedSHA, regSHA, err := frozen(p)
	if err != nil {
		return blockRecord{}, err
	}
	log, err := readLogAgain(p.log)
	if err != nil {
		return blockRecord{}, err
	}
	if openBlock(log) != block {
		return blockRecord{}, fmt.Errorf("block %d is not the open block", block)
	}
	var startSHA, startToolSHA string
	var startToolFiles map[string]string
	for _, r := range log {
		if r.Event == "BLOCK_STARTED" && r.Block == block {
			startSHA = r.SettingsSHA256
			startToolSHA = r.ToolTreeSHA256
			startToolFiles = r.ToolTreeFiles
		}
	}
	sha, err := fileSHA256(p.settings)
	if err != nil {
		return blockRecord{}, err
	}
	intact := sha == startSHA
	toolSum, toolFiles, toolErr := captureToolTree(p)
	rec := blockRecord{Schema: schemaBlock, Event: "BLOCK_ENDED", Block: block, Mechanism: "settings-file",
		At: now.UTC().Format(time.RFC3339), SettingsPath: p.settings, SettingsSHA256: sha, SpanIntact: &intact,
		RegisterSHA256: regSHA, ScheduleSHA256: schedSHA, ToolCommit: toolCommit(),
		ToolTreeSHA256: toolSum, ToolTreeFiles: toolFiles, ToolTreeError: toolErr}
	if !intact {
		rec.Reason = "the settings file changed during the block; sessions after the change are excluded by the register's span rule"
	}
	// ToolTreeIntact is left nil (unknown, not false) unless both the start
	// and end hashes were actually computed. A mismatch does not void the
	// block or exclude any session on its own; it is disclosed, with the
	// changed files named, and analysis stays pinned to the tool_commit
	// recorded at BLOCK_STARTED.
	if startToolSHA != "" && startToolSHA != "UNAVAILABLE" && toolSum != "UNAVAILABLE" {
		toolIntact := toolSum == startToolSHA
		rec.ToolTreeIntact = &toolIntact
		if !toolIntact {
			rec.ToolTreeChangedFiles = toolTreeDiff(startToolFiles, toolFiles)
		}
	}
	return rec, appendRecord(p.log, rec)
}

// frozen loads the schedule and checks it was derived from the register as
// it is now: a register edited after the schedule was fixed is refused.
// It also checks the schedule against the block log: makeSchedule has no
// input from the log, so a register that still hashes correctly can still
// have been re-derived into a schedule that disagrees with what an
// already-started block's own record says happened. historyAgrees catches
// that before any further block starts or ends.
func frozen(p paths) (schedule, string, string, error) {
	regSHA, err := fileSHA256(p.register)
	if err != nil {
		return schedule{}, "", "", err
	}
	b, err := os.ReadFile(p.schedule)
	if err != nil {
		return schedule{}, "", "", fmt.Errorf("reading the schedule: %w", err)
	}
	var s schedule
	if err := json.Unmarshal(b, &s); err != nil {
		return schedule{}, "", "", err
	}
	if s.RegisterSHA256 != regSHA {
		return schedule{}, "", "", fmt.Errorf("the schedule was derived from register %s but the register now hashes to %s; nothing runs against an amended register until the schedule is re-derived and refrozen", short(s.RegisterSHA256), short(regSHA))
	}
	log, err := readLogForFrozen(p.log)
	if err != nil {
		return schedule{}, "", "", err
	}
	if err := historyAgrees(log, s); err != nil {
		return schedule{}, "", "", err
	}
	return s, bytesSHA256(b), regSHA, nil
}

// historyAgrees refuses a schedule whose recorded arm for an already-started
// block differs from that block's own BLOCK_STARTED record in the log. The
// schedule is derived purely from the register's hash and has no input from
// the log, so this is the only thing that would catch a re-derived schedule
// silently contradicting a block that already ran, however the two came to
// disagree. It checks every started block, not only the most recent, so a
// second or third re-derivation is caught exactly the same way the first
// would be.
func historyAgrees(log []blockRecord, s schedule) error {
	for _, r := range log {
		if r.Event != "BLOCK_STARTED" {
			continue
		}
		if r.Block < 1 || r.Block > len(s.Blocks) {
			continue
		}
		if s.Blocks[r.Block-1] != r.Arm {
			return fmt.Errorf("the schedule names block %d as %s, but its own log record says it started as %s; the schedule disagrees with block %d's own history and is refused", r.Block, s.Blocks[r.Block-1], r.Arm, r.Block)
		}
	}
	return nil
}

func short(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}
