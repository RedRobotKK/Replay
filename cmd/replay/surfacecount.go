package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// What Replay knows about its own use, which until now was nothing.
//
// `replay advise` has printed findings since it shipped without any record of
// whether one was ever read. The product direction proposes building more
// findings. Building more of a thing nobody has established is read is the
// error this repository keeps finding in other forms, so the census comes
// first.
//
// THE FUNNEL, AND WHAT EACH STAGE COSTS TO KNOW:
//
//	shown       Recorded here. It was the only gap.
//	opened      NOT OBSERVABLE from the command line, ever. `advise` prints
//	            every finding in one pass, so there is no open. The TUI has a
//	            cursor and could observe it; this build does not.
//	acted upon  Already recorded, as the decisions object on the advice file.
//	            Nothing new is needed and nothing new is written.
//	outcome     Already recorded, as the computed status, and bounded by what
//	            the verifier can reach.
//
// So this file adds one number and declares one absence. That is smaller than
// it looks: three quarters of the funnel already existed, unreported.
//
// IT NEVER LEAVES THE MACHINE. A test reads this file's own imports and fails
// if a transport appears. A local counter that can reach the network is a
// telemetry client wearing a different name, and this binary sends nothing.
const surfaceCountFileName = "surfaces.json"

// surfaceCounts is what one machine did, by subcommand.
//
// A surface that has never run has NO ENTRY. It is not present with a zero.
// "Nobody ran `replay trim`" and "`replay trim` ran and showed nothing" are
// different facts and a reader would act differently on each, which is the
// distinction ADR-0018 draws for prices and which matters at least as much
// here.
type surfaceCounts struct {
	Schema   int                     `json:"schema"`
	Surfaces map[string]surfaceCount `json:"surfaces"`
}

type surfaceCount struct {
	Runs int `json:"runs"`
	// Shown is the count of findings put in front of a person, summed over
	// runs. Zero is meaningful here and different from absent: a surface that
	// ran and had nothing to say is a real observation.
	Shown     int       `json:"shown"`
	LastRunAt time.Time `json:"lastRunAt"`
}

const surfaceCountSchema = 1

// observableStages names the funnel stages this build can see, and the ones it
// cannot.
//
// The value of the false entries is the whole point. Without them, a later
// reader finds no open counts and concludes nobody opened anything, when the
// truth is that nobody could have measured it. Missing and false are different
// and this is where the difference is written down.
func observableStages() map[string]bool {
	return map[string]bool{
		"shown":   true,  // recorded here
		"opened":  false, // no open exists on the command line
		"acted":   true,  // the advice file's decisions object
		"outcome": true,  // the computed status, within the verifier's reach
	}
}

func surfaceCountPath() string { return filepath.Join(tipStateDir(), surfaceCountFileName) }

// readSurfaceCounts returns what is on disk, or an empty record.
//
// A missing file is the first run on a machine. A truncated one is an ordinary
// consequence of a kill. Neither is an error, and neither may break the command
// the user actually asked for: a counter that can fail `replay advise` is worse
// than no counter.
func readSurfaceCounts() (surfaceCounts, error) {
	empty := surfaceCounts{Schema: surfaceCountSchema, Surfaces: map[string]surfaceCount{}}
	b, err := os.ReadFile(surfaceCountPath())
	if err != nil {
		return empty, nil
	}
	var rec surfaceCounts
	if json.Unmarshal(b, &rec) != nil || rec.Schema != surfaceCountSchema {
		return empty, nil
	}
	if rec.Surfaces == nil {
		rec.Surfaces = map[string]surfaceCount{}
	}
	return rec, nil
}

// recordSurfaceRun adds one run of a surface, and how many findings it showed.
//
// shown is 0 for surfaces that do not produce findings, which is a real
// observation rather than a gap: the surface ran and put nothing in front of
// anybody.
func recordSurfaceRun(surface string, shown int) error {
	rec, _ := readSurfaceCounts()
	s := rec.Surfaces[surface]
	s.Runs++
	s.Shown += shown
	s.LastRunAt = time.Now().UTC()
	rec.Surfaces[surface] = s

	out, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(surfaceCountPath()), 0o700); err != nil {
		return err
	}
	// Temp and rename, like every other file this tool owns. A half-written
	// counter read on the next run would lose the history it exists to keep.
	tmp := surfaceCountPath() + ".tmp"
	if err := os.WriteFile(tmp, append(out, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, surfaceCountPath())
}
