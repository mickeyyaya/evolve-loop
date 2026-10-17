package acsrunner

import (
	"context"
	"io"
	"os/exec"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/lanebudget"
)

func TestRun_ALaneGoTestCarriesTheLaneCPUBudget(t *testing.T) {
	t.Setenv(ipcenv.FleetWidthKey, "4")
	procs := strconv.Itoa(lanebudget.Procs(runtime.NumCPU(), 4))
	var seen []string
	withCommander(func(_ context.Context, args ...string) (io.ReadCloser, func() error, error) {
		seen = args
		return &closingReader{strings.NewReader("")}, func() error { return nil }, nil
	}, func() {
		_, _ = Run(context.Background(), 1, "./acs/cycle1")
	})

	want := []string{"go", "test", "-p", procs, "-parallel", procs, "-json", "-count=1", "./acs/cycle1"}
	if !slices.Equal(seen, want) {
		t.Errorf("Run ran %q, want %q", seen, want)
	}
}

func TestExecCommand_ALaneChildGetsGOMAXPROCS(t *testing.T) {
	if _, err := exec.LookPath("env"); err != nil {
		t.Skip("env is not on PATH")
	}
	t.Setenv(ipcenv.FleetWidthKey, "4")
	stdout, wait, err := execCommand(context.Background(), "env")
	if err != nil {
		t.Fatal(err)
	}
	out, readErr := io.ReadAll(stdout)
	if err := wait(); err != nil || readErr != nil {
		t.Fatal(err, readErr)
	}
	want := "GOMAXPROCS=" + strconv.Itoa(lanebudget.Procs(runtime.NumCPU(), 4)) + "\n"
	if !strings.Contains(string(out), want) || strings.Contains(string(out), ipcenv.FleetWidthKey+"=") {
		t.Errorf("child env = %q, want %q and no lane key", out, want)
	}
}
