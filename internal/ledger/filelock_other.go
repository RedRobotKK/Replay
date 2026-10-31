//go:build !unix

package ledger

import "os"

// lockFile takes no lock where there is no flock. The only such platform this
// module builds for is Windows, where the binary refuses to run; the file
// still compiles there so the Windows job keeps vetting and testing the rest.
func lockFile(*os.File) error { return nil }
