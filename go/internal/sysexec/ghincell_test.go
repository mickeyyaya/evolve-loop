package sysexec_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func TestCommand_GhInACellReturnsErrGhInCellFromRun(t *testing.T) {
	cases := map[string][2]string{
		"phase marker": {"build", ""},
		"depth marker": {"", "2"},
	}
	for name, markers := range cases {
		t.Run(name, func(t *testing.T) {
			t.Setenv(ipcenv.PhaseKey, markers[0])
			t.Setenv(ipcenv.DispatchDepthKey, markers[1])
			for _, bin := range []string{"gh", "/usr/local/bin/gh"} {
				if err := sysexec.Command(context.Background(), bin, "auth", "status").Run(); !errors.Is(err, sysexec.ErrGhInCell) {
					t.Errorf("%s in a cell returned %v, want ErrGhInCell", bin, err)
				}
			}
			if err := sysexec.Command(context.Background(), "true").Run(); err != nil {
				t.Errorf("a command other than gh must run in a cell: %v", err)
			}
		})
	}
}

func TestCommand_GhOutsideACellIsNotRefused(t *testing.T) {
	t.Setenv(ipcenv.PhaseKey, "")
	t.Setenv(ipcenv.DispatchDepthKey, "")
	t.Setenv("PATH", t.TempDir())
	err := sysexec.Command(context.Background(), "gh", "auth", "status").Run()
	if err == nil || errors.Is(err, sysexec.ErrGhInCell) {
		t.Errorf("outside a cell gh must fail only as a missing binary, got %v", err)
	}
}

func TestGhCalls_EachEntryIsCompleteAndTheTableIsACopy(t *testing.T) {
	var calls []sysexec.GhCall = sysexec.GhCalls()
	if len(calls) == 0 {
		t.Fatal("no gh call listed")
	}
	seen := make(map[string]bool)
	for _, c := range calls {
		if c.Site == "" || c.Verb == "" || !strings.HasPrefix(c.Command, "gh ") || len(strings.Fields(c.Purpose)) < 3 {
			t.Errorf("incomplete entry %+v", c)
		}
		key := c.Site + "|" + c.Command
		if seen[key] {
			t.Errorf("duplicate entry %s", key)
		}
		seen[key] = true
	}
	calls[0].Site = "changed"
	if sysexec.GhCalls()[0].Site == "changed" {
		t.Error("GhCalls returns the shared table")
	}
}
