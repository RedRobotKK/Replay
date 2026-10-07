package transcript

// Client version provenance states.
//
// Distinct from the settings-span exclusion (SpanIntact / span_intact in
// scripts/ttl-block/block.go), which is about whether the Claude Code
// settings file changed during a block; this is about whether the client
// binary itself changed during a SESSION. The two can both be true or false
// independently of each other and neither is derived from the other.
const (
	// ClientVersionUnmeasured means no line in the transcript carried a
	// version string at all. Not the same value as a version string ever
	// could be, so a reader cannot mistake "no data" for "one version".
	ClientVersionUnmeasured = "unmeasured"
	// ClientVersionSingle means every line that carried a version carried
	// the same one.
	ClientVersionSingle = "single"
	// ClientVersionStraddling means the session's own transcript shows more
	// than one client version. The session was built partly under each of
	// them, and nothing here may assign it one version in their place.
	ClientVersionStraddling = "CLIENT_VERSION_STRADDLING"
)

// ClientVersionProvenance classifies Session.ClientVersions: unmeasured, a
// single version, or straddling. It is a pure function of ClientVersions, so
// it agrees with it by construction rather than by two fields kept in sync
// by hand.
func (s *Session) ClientVersionProvenance() string {
	switch len(s.ClientVersions) {
	case 0:
		return ClientVersionUnmeasured
	case 1:
		return ClientVersionSingle
	default:
		return ClientVersionStraddling
	}
}

// recordClientVersion appends version to versions in first-seen order,
// skipping a blank version and a version already recorded. It is the single
// place that decides what "observed" means for this field, so every call
// site agrees without duplicating the dedup rule.
func recordClientVersion(versions []string, version string) []string {
	if version == "" {
		return versions
	}
	for _, v := range versions {
		if v == version {
			return versions
		}
	}
	return append(versions, version)
}
