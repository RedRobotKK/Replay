package guardcheck

import "testing"

// These pin the three cases PR #336's CI run actually produced or could have
// produced, and the fourth is the one it didn't but a less careful fix would
// have let through.
//
//   - an existing package (cmd/replay): present, go test it against base
//   - a package new to this branch (internal/tenancy, scripts/ttl-block):
//     absent, but not a checkout defect — nothing to compare, nothing broken
//   - a package the checkout lost that the base ref's own tree has: absent
//     AND a checkout defect, which must still fail the base comparison
//   - a mixed batch: the new package must not keep the existing one out of
//     present, which is the actual CI defect this exists to fix

func TestCAP1_AnExistingPackageIsPresent(t *testing.T) {
	present, absent := ClassifyAbsentPackages(
		[]string{"./cmd/replay"},
		map[string]bool{"./cmd/replay": true},
		map[string]bool{"./cmd/replay": true},
	)
	if len(absent) != 0 {
		t.Fatalf("an on-disk package was reported absent: %+v", absent)
	}
	if len(present) != 1 || present[0] != "./cmd/replay" {
		t.Fatalf("got present=%v, want [./cmd/replay]", present)
	}
}

func TestCAP2_ANewPackageIsAbsentButNotACheckoutDefect(t *testing.T) {
	_, absent := ClassifyAbsentPackages(
		[]string{"./internal/tenancy"},
		map[string]bool{}, // not on disk in the base worktree
		map[string]bool{}, // and the base ref's tree never had it either
	)
	if len(absent) != 1 {
		t.Fatalf("got %d absent package(s), want 1", len(absent))
	}
	if absent[0].CheckoutBroken {
		t.Fatalf("a package absent from both the checkout and the base ref's "+
			"own tree was flagged as a checkout defect: %+v", absent[0])
	}
}

func TestCAP3_APackageTheCheckoutLostIsFlaggedAsBroken(t *testing.T) {
	_, absent := ClassifyAbsentPackages(
		[]string{"./internal/ledger"},
		map[string]bool{},                          // the worktree does not have it
		map[string]bool{"./internal/ledger": true}, // but the base ref's own tree does
	)
	if len(absent) != 1 {
		t.Fatalf("got %d absent package(s), want 1", len(absent))
	}
	if !absent[0].CheckoutBroken {
		t.Fatalf("a package the base ref's own tree has, but the checkout does "+
			"not, was not flagged as a checkout defect: %+v", absent[0])
	}
}

func TestCAP4_ANewPackageDoesNotPoisonAnExistingOneInTheSameBatch(t *testing.T) {
	pkgs := []string{"./cmd/replay", "./internal/tenancy", "./scripts/ttl-block"}
	onDisk := map[string]bool{"./cmd/replay": true}
	inBaseTree := map[string]bool{"./cmd/replay": true}

	present, absent := ClassifyAbsentPackages(pkgs, onDisk, inBaseTree)

	if len(present) != 1 || present[0] != "./cmd/replay" {
		t.Fatalf("got present=%v, want exactly [./cmd/replay]; a new package in "+
			"the same batch must not remove an existing one from the set the base "+
			"test run covers", present)
	}
	if len(absent) != 2 {
		t.Fatalf("got %d absent, want 2 (internal/tenancy, scripts/ttl-block)", len(absent))
	}
	for _, a := range absent {
		if a.CheckoutBroken {
			t.Errorf("%s is new to the branch and was flagged as a checkout "+
				"defect: %+v", a.Pkg, a)
		}
	}
}

func TestCAP5_EmptyInputProducesNoPresentAndNoAbsent(t *testing.T) {
	present, absent := ClassifyAbsentPackages(nil, nil, nil)
	if len(present) != 0 || len(absent) != 0 {
		t.Fatalf("got present=%v absent=%v for no packages, want both empty", present, absent)
	}
}
