// Package controlplane is the smallest coordination primitive that Git
// history does not provide: one task, one owner, created only if absent.
//
// A wake file means work is waiting. It does not start a process.
// A claim file means one owner holds the task. The second create fails.
// Neither file is a Replay measurement. Usage that a surface does not
// expose stays the product's existing NOT_MEASURED, not a new status here.
package controlplane

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// ErrClaimed means this task id already has an owner.
var ErrClaimed = errors.New("task already claimed")

// ErrInvalid means the task id is not a single path segment.
var ErrInvalid = errors.New("invalid task id")

var taskID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,80}$`)

// Wake writes queue/<taskID>.md. The file is a request that work exists.
// It names no owner and starts no agent.
func Wake(root, id, note string) error {
	if !taskID.MatchString(id) {
		return ErrInvalid
	}
	dir := filepath.Join(root, "queue")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return createNew(filepath.Join(dir, id+".md"), note+"\n")
}

// Claim creates claims/<taskID> for owner. If the file exists, Claim
// returns ErrClaimed and does not modify it.
func Claim(root, id, owner string) error {
	if !taskID.MatchString(id) || !taskID.MatchString(owner) {
		return ErrInvalid
	}
	dir := filepath.Join(root, "claims")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	body := fmt.Sprintf("task: %s\nowner: %s\nstatus: claimed\n", id, owner)
	return createNew(filepath.Join(dir, id), body)
}

// createNew fails if path exists. O_EXCL is the whole lock.
func createNew(path, body string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return ErrClaimed
		}
		return err
	}
	_, werr := f.WriteString(body)
	cerr := f.Close()
	if werr != nil {
		return werr
	}
	return cerr
}
