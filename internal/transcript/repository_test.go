package transcript

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// minimalTranscript is one valid request, just enough for ParseClaudeCodeFile
// to succeed; repository identity comes entirely from the file's own path,
// never from anything inside it, so the content here is deliberately inert.
const minimalTranscript = `{"type":"user","uuid":"u1","parentUuid":null,"timestamp":"2026-09-02T00:00:00Z","sessionId":"s","message":{"role":"user","content":"hi"}}` + "\n" +
	`{"type":"assistant","uuid":"a1","parentUuid":"u1","requestId":"r1","apiBlockIndex":0,"timestamp":"2026-09-02T00:00:01Z","message":{"role":"assistant","model":"m","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":1,"cache_creation_input_tokens":2,"cache_read_input_tokens":3,"output_tokens":4}}}` + "\n"

func writeTranscript(t *testing.T, dir, name string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(minimalTranscript), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// Before this change, nothing recorded which project directory a session
// came from: Session carried Path (the file's own location) but nothing
// mechanically checkable as "repository identity" distinct from the raw,
// PII-bearing path, and two sessions from different project directories
// were indistinguishable by any field meant for that comparison.
func TestRepositoryIDDistinguishesDifferentRepositories(t *testing.T) {
	root := t.TempDir()
	pathA := writeTranscript(t, filepath.Join(root, "-Users-daniel-Development-Replay-p5"), "a.jsonl")
	pathB := writeTranscript(t, filepath.Join(root, "-Users-daniel-Development-DoorKik"), "b.jsonl")
	sa, err := ParseClaudeCodeFile(pathA)
	if err != nil {
		t.Fatal(err)
	}
	sb, err := ParseClaudeCodeFile(pathB)
	if err != nil {
		t.Fatal(err)
	}
	if sa.RepositoryID == "" || sa.RepositoryID == RepositoryUnknown {
		t.Fatalf("RepositoryID not set for a session read from a file: %q", sa.RepositoryID)
	}
	if sa.RepositoryID == sb.RepositoryID {
		t.Fatalf("two sessions from different project directories got the same RepositoryID: %q", sa.RepositoryID)
	}
}

func TestRepositoryIDMatchesForTheSameRepository(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "-Users-daniel-Development-Replay-p5")
	path1 := writeTranscript(t, dir, "session1.jsonl")
	path2 := writeTranscript(t, dir, "session2.jsonl")
	s1, err := ParseClaudeCodeFile(path1)
	if err != nil {
		t.Fatal(err)
	}
	s2, err := ParseClaudeCodeFile(path2)
	if err != nil {
		t.Fatal(err)
	}
	if s1.RepositoryID != s2.RepositoryID {
		t.Fatalf("two sessions from the same project directory got different RepositoryIDs: %q vs %q", s1.RepositoryID, s2.RepositoryID)
	}
}

// A session built from a reader, never written to disk, has no path to
// derive identity from. That absence is represented explicitly, not as a
// blank string a reader could mistake for a valid (if empty) identifier.
func TestRepositoryIDUnknownWhenSessionHasNoPath(t *testing.T) {
	s, err := ParseClaudeCode(strings.NewReader(minimalTranscript))
	if err != nil {
		t.Fatal(err)
	}
	if s.RepositoryID != RepositoryUnknown {
		t.Fatalf("RepositoryID = %q, want %q", s.RepositoryID, RepositoryUnknown)
	}
	if RepositoryUnknown == "" {
		t.Fatal("RepositoryUnknown must not be the blank string: a missing identity must be explicit")
	}
}

// Repository identity must never be guessed from the transcript's own
// content: two sessions with identical lanes, requests and usage but
// different file locations still get different identifiers, proving the
// field is derived from Path alone.
func TestRepositoryIDNeverInferredFromContent(t *testing.T) {
	root := t.TempDir()
	pathA := writeTranscript(t, filepath.Join(root, "repoA"), "same.jsonl")
	pathB := writeTranscript(t, filepath.Join(root, "repoB"), "same.jsonl") // byte-identical content
	sa, err := ParseClaudeCodeFile(pathA)
	if err != nil {
		t.Fatal(err)
	}
	sb, err := ParseClaudeCodeFile(pathB)
	if err != nil {
		t.Fatal(err)
	}
	if sa.RepositoryID == sb.RepositoryID {
		t.Fatalf("identical content under different directories produced the same RepositoryID: %q", sa.RepositoryID)
	}
}

// A trailing slash or a doubled separator in the path must not create a
// spurious new identifier for the same directory: filepath.Clean normalizes
// the path before the identifier is taken from it.
func TestRepositoryIDNormalizesTrailingSlashAndDoubleSeparator(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "project")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.jsonl"), []byte(minimalTranscript), 0o600); err != nil {
		t.Fatal(err)
	}
	clean := repositoryIDFromPath(filepath.Join(dir, "a.jsonl"))
	messy := repositoryIDFromPath(dir + string(filepath.Separator) + string(filepath.Separator) + "a.jsonl")
	if clean != messy {
		t.Fatalf("a doubled separator changed the identifier: %q vs %q", clean, messy)
	}
}

// Case and symlinks are NOT resolved or folded: this project cannot safely
// assume a filesystem's case sensitivity or that a symlink's target is the
// repository's real identity, so two literally different path strings are
// treated as different identifiers by design, never silently merged. This
// is the conservative choice: it may occasionally under-merge the same
// repository recorded two different ways, but it never over-merges two
// different repositories into one, which is the error the gate cannot
// afford (a false >=2-repositories pass).
func TestRepositoryIDDoesNotFoldCase(t *testing.T) {
	root := t.TempDir()
	pathUpper := writeTranscript(t, filepath.Join(root, "ProjectName"), "a.jsonl")
	pathLower := writeTranscript(t, filepath.Join(root, "projectname"), "b.jsonl")
	sUpper, err := ParseClaudeCodeFile(pathUpper)
	if err != nil {
		t.Fatal(err)
	}
	sLower, err := ParseClaudeCodeFile(pathLower)
	if err != nil {
		t.Fatal(err)
	}
	if sUpper.RepositoryID == sLower.RepositoryID {
		t.Fatal("case was folded; this project must not assume filesystem case-insensitivity")
	}
}

// RepositoryID must survive serialization: it carries a json tag, like
// ClientVersions, for a type that is not serialized today but whose fields
// must round-trip correctly if something downstream (a future eligible-
// session record, or the study's own gate) persists it.
func TestRepositoryIDSurvivesJSONRoundTrip(t *testing.T) {
	root := t.TempDir()
	path := writeTranscript(t, filepath.Join(root, "project"), "a.jsonl")
	s, err := ParseClaudeCodeFile(path)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var round Session
	if err := json.Unmarshal(b, &round); err != nil {
		t.Fatal(err)
	}
	if round.RepositoryID != s.RepositoryID {
		t.Fatalf("round trip RepositoryID = %q, want %q", round.RepositoryID, s.RepositoryID)
	}
}

// A path with no directory component at all (a bare filename, or one
// directly at the filesystem root) reduces to filepath.Base(".") or the
// separator itself, neither of which is a real project directory name.
// No existing test reaches repositoryIDFromPath with such a path: every
// other test writes its fixture into a real subdirectory first.
//
// PASS: a bare filename and a root-level path both resolve to
// RepositoryUnknown, never to "." or the separator itself.
// FAIL: either is returned as if it were a legitimate repository id.
func TestRepositoryIDUnknownWithNoDirectoryComponent(t *testing.T) {
	if got := repositoryIDFromPath("a.jsonl"); got != RepositoryUnknown {
		t.Errorf("repositoryIDFromPath(%q) = %q, want %q", "a.jsonl", got, RepositoryUnknown)
	}
	if got := repositoryIDFromPath(string(filepath.Separator) + "a.jsonl"); got != RepositoryUnknown {
		t.Errorf("repositoryIDFromPath(%q) = %q, want %q", string(filepath.Separator)+"a.jsonl", got, RepositoryUnknown)
	}
}
