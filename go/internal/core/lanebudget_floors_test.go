package core

import (
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

func laneWithFakeGo(t *testing.T) (probe, procs string) {
	t.Helper()
	bin := t.TempDir()
	probe = filepath.Join(t.TempDir(), "probe.txt")
	script := "#!/bin/sh\nprintf 'GOMAXPROCS=%s %s\\n' \"$GOMAXPROCS\" \"$*\" >> \"$LANEBUDGET_PROBE\"\necho TestLaneProbe\n"
	if err := os.WriteFile(filepath.Join(bin, "go"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("LANEBUDGET_PROBE", probe)
	return probe, strconv.Itoa(lanebudget.Procs(runtime.NumCPU(), 4))
}

func probeLines(t *testing.T, probe string) []string {
	t.Helper()
	raw, err := os.ReadFile(probe)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(raw)), "\n")
}

func budgetPrefix(procs string) string {
	return "GOMAXPROCS=" + procs + " test -p " + procs + " -parallel " + procs + " "
}

func TestBuildFloorCover_ALaneCoverRunCarriesTheLaneCPUBudget(t *testing.T) {
	probe, procs := laneWithFakeGo(t)
	t.Setenv(ipcenv.FleetWidthKey, "4")
	moduleDir := filepath.Join(t.TempDir(), "go")
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatal(err)
	}

	profile, _, status := scopedCoverProfile(context.Background(), moduleDir, []string{"./internal/x"}, []string{"integration"})
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(profile)) })

	want := budgetPrefix(procs) + "-count=1 -timeout 20m -tags integration -coverprofile " + profile + " ./internal/x"
	if lines := probeLines(t, probe); status != coverStatusOK || len(lines) != 1 || lines[0] != want {
		t.Errorf("cover run = %q (status %d), want %q", lines, status, want)
	}
}

func TestBuildSelfCheck_ALaneUnitRunCarriesTheLaneCPUBudget(t *testing.T) {
	probe, procs := laneWithFakeGo(t)
	t.Setenv(ipcenv.FleetWidthKey, "4")
	moduleDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(moduleDir, "internal", "x"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, plain := realGoUnitTest(context.Background(), moduleDir, "./internal/x")
	_, tagged := realGoUnitTestTagged(context.Background(), moduleDir, "./internal/x", []string{"acs"})

	lines := probeLines(t, probe)
	want := []string{
		budgetPrefix(procs) + "-count=1 -timeout 20m ./internal/x",
		budgetPrefix(procs) + "-count=1 -timeout 20m -tags acs ./internal/x",
	}
	if !plain || !tagged || strings.Join(lines, "\n") != strings.Join(want, "\n") {
		t.Errorf("self-check runs = %q (passed %v, %v), want %q", lines, plain, tagged, want)
	}
}

func TestListACSPredicates_ALaneListRunCarriesTheLaneCPUBudget(t *testing.T) {
	probe, procs := laneWithFakeGo(t)
	t.Setenv(ipcenv.FleetWidthKey, "4")
	wt := t.TempDir()
	if err := os.MkdirAll(filepath.Join(wt, "go", "acs", "cycle7"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := listACSPredicates(context.Background(), wt, 7)

	want := budgetPrefix(procs) + "-list . -tags acs ./acs/cycle7"
	if lines := probeLines(t, probe); len(got.names) != 1 || len(lines) != 1 || lines[0] != want {
		t.Errorf("list run = %q (names %q, note %q), want %q", lines, got.names, got.note, want)
	}
}
