package controlplane

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// Two agents must not both own one task. The second create fails and the
// first file is unchanged. This is the coordination defect Git history did
// not prevent: two branches can each hold a note and neither is the owner.
func TestSecondClaimantLosesAndFirstOwnerStands(t *testing.T) {
	root := t.TempDir()
	if err := Claim(root, "perf-review", "claude"); err != nil {
		t.Fatal(err)
	}
	err := Claim(root, "perf-review", "grok")
	if !errors.Is(err, ErrClaimed) {
		t.Fatalf("second claim err = %v, want ErrClaimed", err)
	}
	body, err := os.ReadFile(filepath.Join(root, "claims", "perf-review"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "owner: claude") {
		t.Fatalf("owner was replaced:\n%s", text)
	}
	if strings.Contains(text, "owner: grok") {
		t.Fatalf("second claimant wrote into the first claim:\n%s", text)
	}
}

// Concurrent claimants are the case sequential calls do not prove. Exactly
// one wins. The other gets ErrClaimed. The file names the winner.
func TestParallelClaimantsOneWinner(t *testing.T) {
	root := t.TempDir()
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, owner := range []string{"claude", "grok"} {
		wg.Add(1)
		go func(owner string) {
			defer wg.Done()
			errs <- Claim(root, "same-task", owner)
		}(owner)
	}
	wg.Wait()
	close(errs)
	wins := 0
	var winner string
	for err := range errs {
		if err == nil {
			wins++
			continue
		}
		if !errors.Is(err, ErrClaimed) {
			t.Fatal(err)
		}
	}
	if wins != 1 {
		t.Fatalf("wins = %d, want 1", wins)
	}
	body, err := os.ReadFile(filepath.Join(root, "claims", "same-task"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	switch {
	case strings.Contains(text, "owner: claude"):
		winner = "claude"
	case strings.Contains(text, "owner: grok"):
		winner = "grok"
	default:
		t.Fatalf("claim names no winner:\n%s", text)
	}
	other := "grok"
	if winner == "grok" {
		other = "claude"
	}
	if strings.Contains(text, "owner: "+other) {
		t.Fatalf("both owners present:\n%s", text)
	}
}

// A wake file means work is waiting. It does not start a process and it
// does not assign an owner. Claiming is a separate exclusive create.
func TestWakeIsAFileNotAProcess(t *testing.T) {
	root := t.TempDir()
	if err := Wake(root, "measure-deepseek", "compare one prompt; usage may be NOT_MEASURED"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "claims", "measure-deepseek")); !os.IsNotExist(err) {
		t.Fatalf("wake created a claim: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(root, "queue", "measure-deepseek.md"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "owner:") {
		t.Fatalf("wake assigned an owner:\n%s", body)
	}
	if err := Claim(root, "measure-deepseek", "grok"); err != nil {
		t.Fatal(err)
	}
	// The queue file is still the request. Claiming does not delete it.
	if _, err := os.Stat(filepath.Join(root, "queue", "measure-deepseek.md")); err != nil {
		t.Fatal(err)
	}
}

func TestTaskIDWithASlashIsRejected(t *testing.T) {
	root := t.TempDir()
	if err := Claim(root, "a/b", "claude"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
	if err := Wake(root, "..", "x"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("wake err = %v, want ErrInvalid", err)
	}
}
