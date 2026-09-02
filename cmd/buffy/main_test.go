package main

import (
	"errors"
	"testing"
)

func TestRun(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantErr error
	}{
		{name: "no args prints usage", args: nil},
		{name: "version", args: []string{"version"}},
		{name: "help", args: []string{"help"}},
		{name: "serve is scheduled work", args: []string{"serve"}, wantErr: errNotImplemented},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := run(tc.args)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("run(%v) error = %v, want %v", tc.args, err, tc.wantErr)
			}
		})
	}
}

func TestRunRejectsUnknownCommand(t *testing.T) {
	if err := run([]string{"bogus"}); err == nil {
		t.Fatal("expected an error for an unknown command")
	}
}
