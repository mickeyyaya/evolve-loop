package acssuite

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/lanebudget"
)

func TestDefaultGoExec_ALaneSuiteRunsOnTheLaneCPUBudget(t *testing.T) {
	bin := t.TempDir()
	script := "#!/bin/sh\nprintf 'GOMAXPROCS=%s %s' \"$GOMAXPROCS\" \"$*\"\n"
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv(ipcenv.FleetWidthKey, "4")
	procs := strconv.Itoa(lanebudget.Procs(runtime.NumCPU(), 4))

	out, err := defaultGoExec(context.Background(), t.TempDir(), "./acs/cycle1", []string{"PATH=" + bin})

	want := "GOMAXPROCS=" + procs + " test -p " + procs + " -parallel " + procs + " -json -tags acs -count=1 ./acs/cycle1"
	if err != nil || out != want {
		t.Errorf("defaultGoExec = %q, %v, want %q", out, err, want)
	}
}
