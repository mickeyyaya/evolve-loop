package ciparity

import (
	"runtime"
	"slices"
	"strconv"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/lanebudget"
)

func TestCoverageTestArgs_ALaneCoverageRunCarriesTheLaneCPUBudget(t *testing.T) {
	t.Setenv(ipcenv.FleetWidthKey, "4")
	procs := strconv.Itoa(lanebudget.Procs(runtime.NumCPU(), 4))

	got := CoverageTestArgs("cov.out", []string{"./internal/a"})

	want := []string{"test", "-p", procs, "-parallel", procs, "-tags", CoverageTags, "-coverprofile=cov.out", "./internal/a"}
	if !slices.Equal(got, want) {
		t.Errorf("CoverageTestArgs = %q, want %q", got, want)
	}
}

func TestCoverageTestArgs_AConsoleCoverageRunKeepsTheGoDefaults(t *testing.T) {
	t.Setenv(ipcenv.FleetWidthKey, "")

	got := CoverageTestArgs("cov.out", []string{"./internal/a"})

	if want := []string{"test", "-tags", CoverageTags, "-coverprofile=cov.out", "./internal/a"}; !slices.Equal(got, want) {
		t.Errorf("CoverageTestArgs = %q, want %q", got, want)
	}
}
