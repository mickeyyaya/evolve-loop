package acsrunner

import (
	"context"
	"io"
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
	withCommander(func(_ context.Context, _ []string, args ...string) (io.ReadCloser, func() error, error) {
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

func TestRun_ALaneChildGetsGOMAXPROCSAndNoLaneKey(t *testing.T) {
	t.Setenv(ipcenv.FleetWidthKey, "4")
	var env []string
	withCommander(func(_ context.Context, got []string, _ ...string) (io.ReadCloser, func() error, error) {
		env = got
		return &closingReader{strings.NewReader("")}, func() error { return nil }, nil
	}, func() {
		_, _ = Run(context.Background(), 1, "./acs/cycle1")
	})

	want := "GOMAXPROCS=" + strconv.Itoa(lanebudget.Procs(runtime.NumCPU(), 4))
	if !slices.Contains(env, want) || slices.ContainsFunc(env, func(kv string) bool { return strings.HasPrefix(kv, ipcenv.FleetWidthKey+"=") }) {
		t.Errorf("child env lacks %q or carries a lane key: %q", want, env)
	}
}
