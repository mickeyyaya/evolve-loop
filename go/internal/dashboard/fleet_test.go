package dashboard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/cyclestate"
	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func TestServer_FleetPhaseUpdateRefreshesSnapshot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	seedFleetLane(t, root, 1, "build", now)
	seedFleetLane(t, root, 2, "audit", now)
	s := New(root, Options{Now: func() time.Time { return now }})
	_, before := s.current()
	path := filepath.Join(core.RunWorkspacePath(root, 1), core.CycleStateFile)
	// Update the existing file without changing its directory or lease. Both
	// phase names have the same length, so size alone cannot detect the change.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, strings.Replace(string(raw), `"phase":"build"`, `"phase":"audit"`, 1))
	if err := os.Chtimes(path, now, now); err != nil {
		t.Fatal(err)
	}
	s.refresh(false)
	snap, after := s.current()
	if after <= before {
		t.Fatal("fleet phase update did not publish a new snapshot")
	}
	for _, lane := range snap.Cycles {
		if lane.ID == 1 && lane.CurrentPhase == "audit" && lane.State == StateRunning {
			return
		}
	}
	t.Fatalf("updated lane missing from snapshot: %+v", snap.Cycles)
}

func seedFleetLane(t *testing.T, root string, id int, phase string, heartbeat time.Time) {
	t.Helper()
	ws := core.RunWorkspacePath(root, id)
	runID := "run-" + itoa(id)
	b, err := json.Marshal(cyclestate.CycleState{CycleID: id, RunID: runID, Phase: phase, WorkspacePath: ws})
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(ws, core.CycleStateFile), string(b))
	if err := runlease.Write(ws, runlease.Lease{RunID: runID}, heartbeat); err != nil {
		t.Fatal(err)
	}
}

func TestCollect_FleetLivenessSurvivesHistoryCap(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	seedFleetLane(t, root, 1, "build", now)
	seedFleetLane(t, root, 2, "audit", now)
	seedFleetLane(t, root, 3, "build", now.Add(-time.Hour))
	writeCycleState(t, root, cyclestate.CycleState{CycleID: 3, Phase: "tdd"})
	c := newCollector(root)
	c.maxCycles = 1
	s, _ := c.collect(now)
	got := map[int]string{}
	for _, cycle := range s.Cycles {
		if cycle.State == StateRunning {
			got[cycle.ID] = cycle.CurrentPhase
		}
	}
	if len(got) != 2 || got[1] != "build" || got[2] != "audit" {
		t.Fatalf("running lanes = %v; want both active lanes despite newer stale singleton and cap", got)
	}
	if !s.Loop.Running || (s.Loop.CycleID != 1 && s.Loop.CycleID != 2) {
		t.Fatalf("fleet has active lanes but summary is stopped: %+v", s.Loop)
	}
}

func TestCollect_FleetStateIdentityAndCorruptionWarn(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	seedFleetLane(t, root, 1, "build", now)
	seedFleetLane(t, root, 2, "audit", now)
	writeFile(t, filepath.Join(core.RunWorkspacePath(root, 1), core.CycleStateFile), "{")
	writeFile(t, filepath.Join(core.RunWorkspacePath(root, 2), core.CycleStateFile), `{"cycle_id":99,"phase":"audit"}`)
	s := Collect(root, now)
	if len(s.Warnings) < 2 || !strings.Contains(strings.Join(s.Warnings, " "), "cycle") {
		t.Fatalf("corrupt and foreign lane state must be visible: %v", s.Warnings)
	}
	for _, c := range s.Cycles {
		if c.State == StateRunning {
			t.Fatalf("invalid state treated as running: %+v", c)
		}
	}
}
