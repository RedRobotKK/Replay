package transcript

import "path/filepath"

// RepositoryUnknown represents a session whose repository identity could not
// be established: no transcript file location to derive it from. Not the
// blank string, so a reader cannot mistake "not recorded" for a valid (if
// empty) identifier, matching ClientVersionUnmeasured above.
const RepositoryUnknown = "UNKNOWN"

// repositoryIDFromPath derives a session's repository identity from where
// its transcript file sits on disk: the directory immediately containing it.
//
// Claude Code groups a project's session files into one directory per
// project path (replacing path separators in the project's own path with
// dashes), so the directory name already distinguishes one project from
// another without this package inverting that encoding back to a real
// filesystem path, which would be lossy and is not needed here: two
// sessions sharing a directory came from the same project, two under
// different directories did not, and that is the whole of what the
// register's ">=2 repositories" rule needs to check.
//
// Deliberately conservative: filepath.Dir cleans a trailing slash or a
// doubled separator on its own, because those are the same directory spelled
// two ways, but nothing here folds case or resolves a symlink. Either could
// silently treat two different repositories as one, which is the failure
// this field exists to prevent; under-merging the same repository recorded
// two different ways is the safer error.
//
// An empty path, and a path with no directory component, both reduce to
// filepath.Base(".") == "." here (filepath.Dir("") is "."), so the single
// check below catches both without a separate empty-string branch that
// mutation testing could not distinguish from this one.
func repositoryIDFromPath(path string) string {
	base := filepath.Base(filepath.Dir(path))
	if base == "" || base == "." || base == string(filepath.Separator) {
		return RepositoryUnknown
	}
	return base
}
