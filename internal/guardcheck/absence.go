package guardcheck

// Telling a new package from a broken checkout.
//
// baseSurvivorCounts (scripts/guard-reachability/main.go) asks the base tree
// whether it already had each survivor, by checking the base ref out into a
// worktree and running that worktree's own tests. On 2026-10-08 that batched
// `go test` call covered every package any survivor lives in, including
// internal/tenancy and scripts/ttl-block — both new in this branch and absent
// from origin/main. go test reported `FAIL ./internal/tenancy [setup failed]`
// for each (`stat …: directory not found`), the batch exited non-zero, and the
// caller read that as "the base tree's own suite is red" — the fail-closed
// path ClassifySurvivors already has, which marks every survivor in the whole
// batch as introduced, including ones in cmd/replay, internal/ledger,
// internal/analysis and internal/transcript that DO exist on the base and
// whose own comparison the missing packages had nothing to do with.
//
// A package absent from the base tree is not evidence the base suite is red.
// It is evidence the package is new, which is a normal thing for a branch to
// do and already has a correct, existing answer: baseConditionals treats an
// unreadable base directory as an empty index, and an empty index already
// reads as "introduced" through PairSurvivors. The fix is not a new
// exemption — it is routing the missing packages out of the batched test
// run before it executes, so a brand-new package cannot poison the answer
// for every other package asked about in the same run.
//
// The two kinds of "absent" are not the same claim, though, and the second
// one must still fail loudly. If the base ref's own git tree attests to a
// path the checkout does not have, the package is not new — the checkout is
// broken, or something is misconfigured about which ref was asked for — and
// treating that as "no base counterpart, nothing to compare" would silently
// exempt whatever survivors live there once the checkout happens to be more
// broken than this one case. That must still report the base as unable to
// answer.

// PackageAbsence records why a package was not run against the base tree.
type PackageAbsence struct {
	// Pkg is the package as runTests names it, e.g. "./internal/tenancy".
	Pkg string
	// CheckoutBroken is true when the base ref's own tree has this path but
	// the checkout does not: a checkout defect, not a new package. False means
	// the base ref never had it either — a package genuinely new to this
	// branch, with nothing broken about the comparison.
	CheckoutBroken bool
}

// ClassifyAbsentPackages splits pkgs into the ones the base checkout
// materialised and the ones it did not, and says which of the absent ones
// the base ref's own tree disagrees with the checkout about.
//
// onDisk and inBaseTree are keyed by the same strings pkgs carries, and are
// answered independently: onDisk by stat-ing the worktree, inBaseTree by
// asking git about the base ref directly, not about the checkout of it. A
// package can be in neither (new to this branch), in both (ordinary, go to
// present), or in inBaseTree alone (the checkout lost something git says
// should be there). It cannot be in onDisk alone — the worktree is a checkout
// of that exact ref — and a caller that manages to produce that combination
// gets CheckoutBroken: false, same as a genuinely new package, because this
// function has no fourth category to put it in and silence here is louder
// than inventing one.
//
// present is safe to hand to the base test run as-is. Every absent entry
// with CheckoutBroken true means the base cannot be asked anything at all
// this run: see the caller, which turns that into the same "base could not
// answer" failure a red base suite already produces, rather than a per-package
// exemption that would need its own fail-closed proof.
func ClassifyAbsentPackages(pkgs []string, onDisk, inBaseTree map[string]bool) (present []string, absent []PackageAbsence) {
	for _, pkg := range pkgs {
		if onDisk[pkg] {
			present = append(present, pkg)
			continue
		}
		absent = append(absent, PackageAbsence{
			Pkg:            pkg,
			CheckoutBroken: inBaseTree[pkg],
		})
	}
	return present, absent
}
