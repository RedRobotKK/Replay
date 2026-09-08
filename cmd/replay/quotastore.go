package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// The status line is a sensor, and it was throwing its reading away.
//
// Claude Code sends `rate_limits` to a statusline script on a 300ms debounce.
// Every other command in this program, and every screen in the surface, had no
// way to reach it: `replay burn` reported "not reported" for the largest surface
// on the machine while the number arrived several times a second and was
// discarded.
//
// So the reading is persisted where the rest of the program can see it. Nothing
// is sent anywhere; this is a file next to the ledger, owner-only, holding
// percentages and reset times for one account.
type quotaReading struct {
	RateLimits *rateLimits `json:"rate_limits,omitempty"`
	TakenAt    time.Time   `json:"taken_at"`
}

// Age is how old the reading is. Callers must show it.
//
// A quota figure without its age is the defect this area keeps producing in
// different costumes: a reading from three hours ago renders identically to a
// live one, and a person deciding whether to start a long run acts on it.
func (q quotaReading) Age(now time.Time) time.Duration { return now.Sub(q.TakenAt) }

// saveQuota records a reading, and writes nothing when there is none.
//
// A metered account has no windows at all. Writing an empty file on every tick
// would be a disk write per keystroke describing a reading that does not exist.
func saveQuota(path string, s statusInput, now time.Time) error {
	if s.RateLimits == nil {
		return nil
	}
	if s.RateLimits.FiveHour == nil && s.RateLimits.SevenDay == nil && s.RateLimits.SpendLimit == nil {
		return nil
	}
	b, err := json.Marshal(quotaReading{RateLimits: s.RateLimits, TakenAt: now})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	// Temp file and rename, because the reader is another process and a
	// partially written file is indistinguishable from a small one.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".quota-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		_ = os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	if err := os.Chmod(name, 0o600); err != nil {
		_ = os.Remove(name)
		return err
	}
	return os.Rename(name, path)
}

// loadQuota reads a stored reading, or returns an empty one.
//
// Absent and corrupt both mean "no reading". Neither is an error to the caller
// and neither is a zero: a zeroed window would tell a reader they have a full
// allowance, which is the most expensive possible way to be wrong here. The
// status line writes this on a hot path, so a partial write must not take out
// every other command in the program.
func loadQuota(path string) (quotaReading, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return quotaReading{}, nil
	}
	var q quotaReading
	if json.Unmarshal(b, &q) != nil {
		return quotaReading{}, nil
	}
	return q, nil
}

// defaultQuotaPath is where the reading lives, beside the ledger.
func defaultQuotaPath() string {
	dir, err := defaultLedgerDir()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(dir), "quota.json")
}
