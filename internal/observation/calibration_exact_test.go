package observation

import "testing"

// Validate refuses a record claiming more exact turns than matched ones.
//
// Exact is a SUBSET of matched: a turn reproduced exactly is necessarily a
// turn that matched. A record where exact exceeds matched is not a bad
// measurement, it is an impossible one, and the pooled record is the single
// place these figures leave the machine.
//
// The branch was unreached. Nothing built the impossible record, so nothing
// established the validator refuses it — and a pool that accepted one would
// reconstruct an exact rate above its own match rate for every contributor
// downstream, with no way to tell which record poisoned it.
func TestOC1_ARecordCannotBeMoreExactThanMatched(t *testing.T) {
	// Built from the package's own valid fixture, so the only thing wrong
	// with this record is the impossible pair.
	c := sampleCalibration()
	c.Models[0].Exact = c.Models[0].Matched + 1
	c = c.Digested()
	if err := c.Validate(); err == nil {
		t.Errorf("a record claiming %d exact turns out of %d matched validated. Exact is "+
			"a subset of matched, so this record is impossible rather than merely wrong, "+
			"and it is the form these figures leave the machine in",
			c.Models[0].Exact, c.Models[0].Matched)
	}

	// The possible case still passes, or the refusal above proves nothing.
	c.Models[0].Exact = c.Models[0].Matched
	c = c.Digested()
	if err := c.Validate(); err != nil {
		t.Errorf("a record with exact == matched was refused: %v", err)
	}
}
