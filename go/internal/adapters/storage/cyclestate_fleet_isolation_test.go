package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

// cyclestate_fleet_isolation_test.go — the fix for the fleet cycle-state
// singleton race (2026-07-03): two concurrent lanes wrote the host-global
// .evolve/cycle-state.json (last-writer-wins), so lane A's phase-gate
// (guards.Phase reads cycle state) saw lane B's Phase and stalled before audit.
// When ipcenv.CycleStateFileKey is set, each lane reads+writes its OWN per-run
// file — no shared singleton.

// The bug reproduction + fix: with the per-run override set to distinct files,
// two lanes writing DIFFERENT phases never clobber each other. The negative
// control (no override) reproduces the clobber, proving the test bites.
func TestCycleState_FleetPerRunOverride_NoClobber(t *testing.T) {
	dir := t.TempDir()
	host := New(dir) // shared host-global storage both lanes would otherwise use

	laneAFile := filepath.Join(dir, "runs", "cycle-101", "cycle-state.json")
	laneBFile := filepath.Join(dir, "runs", "cycle-102", "cycle-state.json")
	mkParents(t, laneAFile, laneBFile)

	ctx := context.Background()

	// --- Negative control: NO override => shared singleton => clobber. ---
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

	// --- Fix: per-lane override => each lane reads back ITS OWN phase. ---
	// Lane A writes coverage-gate to its per-run file...
	t.Setenv(ipcenv.CycleStateFileKey, laneAFile)
	if err := host.WriteCycleState(ctx, core.CycleState{CycleID: 101, Phase: "coverage-gate", WorkspacePath: filepath.Dir(laneAFile)}); err != nil {
		t.Fatalf("laneA per-run write: %v", err)
	}
	// ...lane B writes build to a DIFFERENT per-run file (interleaved: this is the
	// peer write that used to clobber lane A in the singleton)...
	t.Setenv(ipcenv.CycleStateFileKey, laneBFile)
	if err := host.WriteCycleState(ctx, core.CycleState{CycleID: 102, Phase: "build", WorkspacePath: filepath.Dir(laneBFile)}); err != nil {
		t.Fatalf("laneB per-run write: %v", err)
	}
	// ...lane A's phase-gate reads back: MUST see coverage-gate, not build.
	t.Setenv(ipcenv.CycleStateFileKey, laneAFile)
	gotA, err := host.ReadCycleState(ctx)
	if err != nil {
		t.Fatalf("laneA per-run read: %v", err)
	}
	if gotA.Phase != "coverage-gate" || gotA.CycleID != 101 {
		t.Errorf("laneA read cycle=%d phase=%q, want 101/coverage-gate — the peer lane's write clobbered it (isolation broken)", gotA.CycleID, gotA.Phase)
	}
	// Lane B still sees its own.
	t.Setenv(ipcenv.CycleStateFileKey, laneBFile)
	gotB, err := host.ReadCycleState(ctx)
	if err != nil {
		t.Fatalf("laneB per-run read: %v", err)
	}
	if gotB.Phase != "build" || gotB.CycleID != 102 {
		t.Errorf("laneB read cycle=%d phase=%q, want 102/build", gotB.CycleID, gotB.Phase)
	}
}

// Absent override ⇒ the host-global path, byte-identical to pre-fix behaviour.
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
