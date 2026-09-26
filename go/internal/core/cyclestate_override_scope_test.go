package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
)

// Every process a fleet lane spawns inherits its override, go test binaries included; a test's own
// evolve dir must still resolve to its own file.
func TestResolveCycleStatePath_AnOverrideOutsideTheEvolveDirIsNotItsFile(t *testing.T) {
	live := filepath.Join(t.TempDir(), ".evolve", "runs", "cycle-1700", CycleStateFile)
	t.Setenv(ipcenv.CycleStateFileKey, live)
	fixture := filepath.Join(t.TempDir(), ".evolve")
	if got, want := ResolveCycleStatePath(fixture), filepath.Join(fixture, CycleStateFile); got != want {
		t.Errorf("ResolveCycleStatePath(%q) = %q, want the fixture's own %q, not another tree's live lane file", fixture, got, want)
	}
}

// Discovery stands down only for an override that governs this evolve dir: another tree's lane
// override says nothing about where this tree's checkpoints are.
func TestLoadResumeState_AnotherTreesLaneOverrideDoesNotStopDiscovery(t *testing.T) {
	t.Setenv(ipcenv.CycleStateFileKey, filepath.Join(t.TempDir(), ".evolve", "runs", "cycle-1700", CycleStateFile))
	tmp := t.TempDir()
	evolveDir := filepath.Join(tmp, ".evolve")
	wt := filepath.Join(tmp, "wt")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	writeStateFile(t, filepath.Join(evolveDir, "runs", "cycle-1582"), fleetCheckpointState(1582, "2026-08-29T04:55:00Z", "triage", wt))

	rp, err := LoadResumeState(context.Background(), tmp, evolveDir, ResumeOptions{})
	if err != nil {
		t.Fatalf("this tree's own per-run checkpoint went undiscovered under another tree's override: %v", err)
	}
	if rp.CycleID != 1582 {
		t.Errorf("CycleID = %d, want 1582", rp.CycleID)
	}
}

func TestResolveCycleStatePath_ALaneOverrideInsideItsEvolveDirIsHonored(t *testing.T) {
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	lane := filepath.Join(evolveDir, "runs", "cycle-1700", CycleStateFile)
	t.Setenv(ipcenv.CycleStateFileKey, lane)
	if got := ResolveCycleStatePath(evolveDir); got != lane {
		t.Errorf("ResolveCycleStatePath(%q) = %q, want the lane's own file %q", evolveDir, got, lane)
	}
	if got, want := ResolveCycleStatePath(evolveDir+"-sibling"), filepath.Join(evolveDir+"-sibling", CycleStateFile); got != want {
		t.Errorf("a sibling dir sharing the prefix resolved %q, want %q", got, want)
	}
}
