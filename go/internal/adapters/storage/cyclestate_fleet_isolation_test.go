package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

func TestCycleState_FleetPerRunOverride_NoClobber(t *testing.T) {
	dir := t.TempDir()
	host := New(dir)

	laneAFile := filepath.Join(dir, "runs", "cycle-101", "cycle-state.json")
	laneBFile := filepath.Join(dir, "runs", "cycle-102", "cycle-state.json")
	mkParents(t, laneAFile, laneBFile)

	ctx := context.Background()

	if err := host.WriteCycleState(ctx, core.CycleState{CycleID: 101, Phase: "coverage-gate"}); err != nil {
		t.Fatalf("laneA host write: %v", err)
	}
	if err := host.WriteCycleState(ctx, core.CycleState{CycleID: 102, Phase: "build"}); err != nil {
		t.Fatalf("laneB host write: %v", err)
	}
	got, err := host.ReadCycleState(ctx)
	if err != nil {
		t.Fatalf("host read: %v", err)
	}
	if got.Phase != "build" || got.CycleID != 102 {
		t.Fatalf("control: expected the clobber (laneB wins the singleton), got cycle=%d phase=%q — the singleton is no longer shared, test is stale", got.CycleID, got.Phase)
	}

	t.Setenv(ipcenv.CycleStateFileKey, laneAFile)
	if err := host.WriteCycleState(ctx, core.CycleState{CycleID: 101, Phase: "coverage-gate", WorkspacePath: filepath.Dir(laneAFile)}); err != nil {
		t.Fatalf("laneA per-run write: %v", err)
	}
	t.Setenv(ipcenv.CycleStateFileKey, laneBFile)
	if err := host.WriteCycleState(ctx, core.CycleState{CycleID: 102, Phase: "build", WorkspacePath: filepath.Dir(laneBFile)}); err != nil {
		t.Fatalf("laneB per-run write: %v", err)
	}
	t.Setenv(ipcenv.CycleStateFileKey, laneAFile)
	gotA, err := host.ReadCycleState(ctx)
	if err != nil {
		t.Fatalf("laneA per-run read: %v", err)
	}
	if gotA.Phase != "coverage-gate" || gotA.CycleID != 101 {
		t.Errorf("laneA read cycle=%d phase=%q, want 101/coverage-gate — the peer lane's write clobbered it (isolation broken)", gotA.CycleID, gotA.Phase)
	}
	t.Setenv(ipcenv.CycleStateFileKey, laneBFile)
	gotB, err := host.ReadCycleState(ctx)
	if err != nil {
		t.Fatalf("laneB per-run read: %v", err)
	}
	if gotB.Phase != "build" || gotB.CycleID != 102 {
		t.Errorf("laneB read cycle=%d phase=%q, want 102/build", gotB.CycleID, gotB.Phase)
	}
}

func TestCycleState_NoOverride_UsesHostSingleton(t *testing.T) {
	dir := t.TempDir()
	s := New(dir)
	want := filepath.Join(dir, core.CycleStateFile)
	if got := s.cycleStatePath(); got != want {
		t.Errorf("cycleStatePath() without override = %q, want host-global %q", got, want)
	}
	t.Setenv(ipcenv.CycleStateFileKey, filepath.Join(dir, "runs", "cycle-7", "cycle-state.json"))
	if got := s.cycleStatePath(); got == want {
		t.Errorf("cycleStatePath() ignored the fleet override; got host-global %q", got)
	}
}

func mkParents(t *testing.T, files ...string) {
	t.Helper()
	for _, f := range files {
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(f), err)
		}
	}
}
