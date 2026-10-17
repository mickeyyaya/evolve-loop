package ship

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/lanebudget"
)

func fakeGoOnPath(t *testing.T) string {
	t.Helper()
	bin := t.TempDir()
	probe := filepath.Join(t.TempDir(), "probe.txt")
	script := "#!/bin/sh\nprintf 'GOMAXPROCS=%s %s\\n' \"$GOMAXPROCS\" \"$*\" > \"$LANEBUDGET_PROBE\"\n"
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("LANEBUDGET_PROBE", probe)
	return probe
}

func TestRepoContractPack_ALaneGoTestRunsOnTheLaneCPUBudget(t *testing.T) {
	probe := fakeGoOnPath(t)
	t.Setenv(ipcenv.FleetWidthKey, "4")
	procs := strconv.Itoa(lanebudget.Procs(runtime.NumCPU(), 4))
	var notes, raw bytes.Buffer

	out := runRepoContractPackages(context.Background(), t.TempDir(), packLog{notes: &notes, raw: &raw}, []string{"./internal/core"})

	got, err := os.ReadFile(probe)
	want := "GOMAXPROCS=" + procs + " test -p " + procs + " -parallel " + procs + " -json -count=1 -timeout " + repoContractTestTimeout + " ./internal/core\n"
	if out.err != nil || err != nil || string(got) != want {
		t.Errorf("child saw %q (%v, %v), want %q", got, out.err, err, want)
	}
}

func TestRepoContractPack_AConsoleGoTestKeepsTheGoDefaults(t *testing.T) {
	probe := fakeGoOnPath(t)
	t.Setenv(ipcenv.FleetWidthKey, "")
	t.Setenv("GOMAXPROCS", "")
	var notes, raw bytes.Buffer

	out := runRepoContractPackages(context.Background(), t.TempDir(), packLog{notes: &notes, raw: &raw}, []string{"./internal/core"})

	got, err := os.ReadFile(probe)
	if out.err != nil || err != nil || strings.Contains(string(got), "-p ") || !strings.HasPrefix(string(got), "GOMAXPROCS= test -json") {
		t.Errorf("child saw %q (%v, %v), want no budget", got, out.err, err)
	}
}
