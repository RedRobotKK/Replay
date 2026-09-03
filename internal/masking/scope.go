package masking

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// Destination kinds. Text is an assistant text block. Edit is the input of
// a file-edit tool, restorable only when its path is inside the project.
// Tool is the input of any other tool, denied unless a scope names it.
const (
	DestinationText = "text"
	DestinationEdit = "edit"
	DestinationTool = "tool"
)

// Reasons a placeholder was left in place, reported with the destination.
const (
	ReasonScope          = "scope"           // the pattern's scope excludes this destination
	ReasonOutsideProject = "outside-project" // a file edit whose path is not under the project
	ReasonNoPath         = "no-path"         // a file edit without a usable path
	ReasonNoProject      = "no-project"      // no project root configured
	ReasonUnparsedInput  = "unparsed-input"  // tool input that is not a JSON object
	ReasonTooLarge       = "too-large"       // tool input past the held-bytes limit
	ReasonUnknown        = "unknown"         // the vault does not hold this placeholder
)

// Destination is where a placeholder would be restored.
type Destination struct {
	Kind string
	Tool string
	// Inside is true for a file edit whose path is under the project.
	Inside bool
	// Reason is set when the destination can never receive a secret,
	// whatever the pattern's scope.
	Reason string
}

// String names the destination for logs, reports, and metrics: "text",
// "edit:Edit", "tool:Bash".
func (d Destination) String() string {
	if d.Kind == DestinationText {
		return DestinationText
	}
	return d.Kind + ":" + d.Tool
}

// Scope says where one pattern's secrets may be restored.
type Scope struct {
	Text bool
	// Edit allows file-edit tool inputs whose path is inside the project.
	Edit bool
	// Tools allows named tools whatever their input.
	Tools map[string]bool
}

// DefaultScope is ADR-0004's default: assistant text and in-project file
// edits, never shell or network tools.
var DefaultScope = Scope{Text: true, Edit: true}

// deny returns why the scope keeps a secret out of a destination, or "".
func (s Scope) deny(d Destination) string {
	if s.Tools[d.Tool] && d.Kind != DestinationText {
		return ""
	}
	switch {
	case d.Reason != "":
		return d.Reason
	case d.Kind == DestinationText && s.Text:
		return ""
	case d.Kind == DestinationEdit && d.Inside && s.Edit:
		return ""
	}
	return ReasonScope
}

// String renders the scope in the flag's own grammar.
func (s Scope) String() string {
	var parts []string
	if s.Text {
		parts = append(parts, "text")
	}
	if s.Edit {
		parts = append(parts, "edit")
	}
	tools := make([]string, 0, len(s.Tools))
	for t := range s.Tools {
		tools = append(tools, "tool:"+t)
	}
	sort.Strings(tools)
	parts = append(parts, tools...)
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ",")
}

// Scopes is the rehydration policy: a project root, a default scope, and
// per-pattern overrides.
type Scopes struct {
	// Project is the absolute root under which file edits may receive
	// secrets. Empty denies every file edit.
	Project   string
	Default   Scope
	ByPattern map[string]Scope
}

// For returns the scope for a pattern name.
func (s Scopes) For(pattern string) Scope {
	if sc, ok := s.ByPattern[pattern]; ok {
		return sc
	}
	return s.Default
}

// AnyPattern in a scope spec sets the default scope.
const AnyPattern = "*"

// ParseScope reads one scope spec, "name=dest[,dest]", where a dest is
// text, edit, tool:NAME, or none on its own. The name is a pattern name
// or "*" for the default.
func ParseScope(spec string) (name string, s Scope, err error) {
	name, list, ok := strings.Cut(spec, "=")
	name = strings.TrimSpace(name)
	if !ok || name == "" {
		return "", s, fmt.Errorf("scope %q: expected name=destinations", spec)
	}
	dests := strings.Split(list, ",")
	for _, d := range dests {
		d = strings.TrimSpace(d)
		switch {
		case d == "text":
			s.Text = true
		case d == "edit":
			s.Edit = true
		case d == "none" && len(dests) == 1:
			return name, Scope{}, nil
		case strings.HasPrefix(d, "tool:") && len(d) > len("tool:"):
			if s.Tools == nil {
				s.Tools = map[string]bool{}
			}
			s.Tools[d[len("tool:"):]] = true
		default:
			return "", s, fmt.Errorf("scope %q: %q is not text, edit, tool:NAME, or none", spec, d)
		}
	}
	return name, s, nil
}

// ParseScopes builds the policy from specs, checking every pattern name
// against the known set.
func ParseScopes(project string, specs []string, known []Pattern) (Scopes, error) {
	out := Scopes{Project: project, Default: DefaultScope, ByPattern: map[string]Scope{}}
	names := map[string]bool{}
	for _, p := range known {
		names[p.Name] = true
	}
	for _, spec := range specs {
		name, s, err := ParseScope(spec)
		if err != nil {
			return Scopes{}, err
		}
		if name == AnyPattern {
			out.Default = s
			continue
		}
		if !names[name] {
			return Scopes{}, fmt.Errorf("scope %q: unknown pattern %q", spec, name)
		}
		out.ByPattern[name] = s
	}
	return out, nil
}

// fileEditTools are tools whose input names a file the agent writes,
// each with the input key that names its target. Anything else, shell and
// network tools included, is denied by default.
var fileEditTools = map[string]string{
	"Edit": "file_path", "Write": "file_path", "MultiEdit": "file_path", "NotebookEdit": "notebook_path",
	"str_replace_editor": "path", "str_replace_based_edit_tool": "path",
	"create_file": "path", "write_to_file": "path", "replace_in_file": "path",
	"write_file": "file_path", "edit_file": "target_file",
}

// pathKeys are every input field a file-edit tool is known to use for
// its target. A file edit receives a secret only when its own key is
// present and every path-like field is inside the project, so a decoy
// in-project path next to the real target does not open the scope.
var pathKeys = []string{"file_path", "path", "notebook_path", "filePath", "target_file"}

// toolDestination classifies a tool call by name and input. The input is
// the tool's JSON object as text.
func (s Scopes) toolDestination(tool string, input []byte) Destination {
	key, ok := fileEditTools[tool]
	if !ok {
		return Destination{Kind: DestinationTool, Tool: tool}
	}
	d := Destination{Kind: DestinationEdit, Tool: tool}
	paths, err := inputPaths(input)
	switch {
	case err != nil:
		d.Reason = ReasonUnparsedInput
	case paths[key] == "":
		d.Reason = ReasonNoPath
	case s.Project == "":
		d.Reason = ReasonNoProject
	case !allInside(s.Project, paths):
		d.Reason = ReasonOutsideProject
	default:
		d.Inside = true
	}
	return d
}

func allInside(root string, paths map[string]string) bool {
	for _, p := range paths {
		if !insideProject(root, p) {
			return false
		}
	}
	return true
}

// insideProject reports whether a path, relative ones taken from the
// root, stays under the root. Symbolic links along the part of the path
// that exists are resolved, and the root is resolved the same way so a
// root given through a link (macOS's /var to /private/var, say) compares
// equal to the agent's paths; a link inside the project that points
// outside is seen, and a link that does not exist yet cannot be.
func insideProject(root, path string) bool {
	if strings.Contains(path, PlaceholderPrefix) {
		return false
	}
	root = resolveExisting(filepath.Clean(root))
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	rel, err := filepath.Rel(root, resolveExisting(filepath.Clean(path)))
	if err != nil || filepath.IsAbs(rel) {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// ResolveRoot returns the project root with symbolic links resolved, as
// paths are compared against it.
func ResolveRoot(root string) (string, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	return resolveExisting(abs), nil
}

// resolveExisting resolves symbolic links in the deepest ancestor of a
// clean path that exists and re-attaches the rest unchanged.
func resolveExisting(path string) string {
	rest := ""
	for cur := path; ; {
		if resolved, err := filepath.EvalSymlinks(cur); err == nil {
			return filepath.Join(resolved, rest)
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return path
		}
		rest = filepath.Join(filepath.Base(cur), rest)
		cur = parent
	}
}
