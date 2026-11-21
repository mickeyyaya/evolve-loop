package core

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/lanebudget"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/verifylock"
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

func TestBuildFloorCover_ALaneCoverRunCarriesTheHolderCPUBudget(t *testing.T) {
	probe, _ := laneWithFakeGo(t)
	procs := strconv.Itoa(lanebudget.HolderProcs(runtime.NumCPU(), 4))
	t.Setenv(ipcenv.FleetWidthKey, "4")
	t.Setenv(ipcenv.FleetLiveWidthKey, "")
	moduleDir := filepath.Join(t.TempDir(), "go")
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatal(err)
	}

	profile, _, status, _ := scopedCoverProfile(context.Background(), ReviewInput{}, moduleDir, []string{"./internal/x"}, []string{"integration"})
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(profile)) })

	want := budgetPrefix(procs) + "-count=1 -timeout 20m -tags integration -coverprofile " + profile + " ./internal/x"
	if lines := probeLines(t, probe); status != coverStatusOK || len(lines) != 1 || lines[0] != want {
		t.Errorf("cover run = %q (status %d), want %q", lines, status, want)
	}
}

func TestBuildSelfCheck_ALaneUnitRunCarriesTheLaneCPUBudget(t *testing.T) {
	probe, procs := laneWithFakeGo(t)
	t.Setenv(ipcenv.FleetWidthKey, "4")
	t.Setenv(ipcenv.FleetLiveWidthKey, "")
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
	t.Setenv(ipcenv.FleetLiveWidthKey, "")
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

func holderFloorRun(t *testing.T, in ReviewInput, holdLock bool) (probe string, line string, profile string) {
	t.Helper()
	probe, _ = laneWithFakeGo(t)
	t.Setenv(ipcenv.FleetWidthKey, "3")
	t.Setenv(ipcenv.FleetLiveWidthKey, "")
	root := t.TempDir()
	moduleDir := filepath.Join(root, "go")
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if holdLock {
		hold, err := verifylock.Acquire(context.Background(), root, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(hold)
		prev := coverLockWait
		coverLockWait = 50 * time.Millisecond
		t.Cleanup(func() { coverLockWait = prev })
	}
	profile, _, status, _ := scopedCoverProfile(context.Background(), in, moduleDir, []string{"./internal/x"}, nil)
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(profile)) })
	lines := probeLines(t, probe)
	if status != coverStatusOK || len(lines) != 1 {
		t.Fatalf("cover run = %q (status %d), want one run that passed", lines, status)
	}
	return probe, lines[0], profile
}

func TestBuildFloorCover_TheRunThatHoldsTheHostLockGetsTheHolderBudget(t *testing.T) {
	_, line, profile := holderFloorRun(t, ReviewInput{}, false)

	holder := strconv.Itoa(lanebudget.HolderProcs(runtime.NumCPU(), 3))
	want := budgetPrefix(holder) + "-count=1 -timeout 20m -coverprofile " + profile + " ./internal/x"
	if line != want {
		t.Errorf("cover run = %q, want %q: the one run that holds the host lock gets the host budget, not the lane share", line, want)
	}
}

func TestBuildFloorCover_ARunThatLostTheHostLockKeepsTheLaneBudget(t *testing.T) {
	_, line, profile := holderFloorRun(t, ReviewInput{}, true)

	lane := strconv.Itoa(lanebudget.Procs(runtime.NumCPU(), 3))
	want := budgetPrefix(lane) + "-count=1 -timeout 20m -coverprofile " + profile + " ./internal/x"
	if line != want {
		t.Errorf("cover run = %q, want %q: a run without the lock must not take the holder budget", line, want)
	}
}

func TestBuildFloorCover_TheRetakeOutsideTheLockKeepsTheLaneBudget(t *testing.T) {
	t.Setenv(ipcenv.FleetWidthKey, "3")
	t.Setenv(ipcenv.FleetLiveWidthKey, "")
	moduleDir := filepath.Join(t.TempDir(), "go")
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	laneWithFakeGo(t)

	plan, _, _ := firstCoverPass(context.Background(), ReviewInput{}, coverPlan{moduleDir: moduleDir, pkgs: []string{"./internal/x"}, share: lanebudget.Current()})
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(plan.profile)) })

	if plan.share != lanebudget.Current() {
		t.Errorf("plan share = %d, want the lane share %d: the retake runs after the lock is released", plan.share, lanebudget.Current())
	}
}

func TestBuildFloorCover_EachLockAcquireEmitsOneWaitSignal(t *testing.T) {
	center := signalcenter.New()

	holderFloorRun(t, ReviewInput{Signals: center, Cycle: 9, RunID: "r9"}, false)

	var waits []signalcenter.Event
	for _, e := range center.Recent() {
		if e.Code == signalcenter.CodeHostVerifyLockWait {
			waits = append(waits, e)
		}
	}
	if len(waits) != 1 || waits[0].Fields["source"] != "build-floor" || waits[0].Fields["outcome"] != "acquired" || waits[0].Cycle != 9 {
		t.Errorf("wait signals = %+v, want exactly one from build-floor, acquired, cycle 9", waits)
	}
}
