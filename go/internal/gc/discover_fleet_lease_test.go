package gc

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

func TestDiscover_EmptyCurrentWorkspace_FreshLeaseStaysLive(t *testing.T) {
	dir := t.TempDir()
	t0 := time.Unix(1_700_000_000, 0)

	liveLane := mkRun(t, dir, "cycle-201", t0.Add(-time.Hour))
	deadLane := mkRun(t, dir, "cycle-202", t0.Add(-time.Hour))
	writeFile(t, filepath.Join(liveLane.Path, "run.json"), `{"cycle_id":201}`)
	writeFile(t, filepath.Join(deadLane.Path, "run.json"), `{"cycle_id":202}`)

	if err := runlease.Write(liveLane.Path, runlease.Lease{RunID: "laneA"}, t0.Add(-time.Minute)); err != nil {
		t.Fatalf("write fresh lease: %v", err)
	}
	if err := runlease.Write(deadLane.Path, runlease.Lease{RunID: "laneB"}, t0.Add(-2*time.Hour)); err != nil {
		t.Fatalf("write stale lease: %v", err)
	}

	got, err := Discover(dir, DiscoverOptions{Now: func() time.Time { return t0 }})
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	live := map[string]bool{}
	for _, r := range got {
		live[filepath.Base(r.Path)] = r.Live
	}
	if !live["cycle-201"] {
		t.Error("fresh-lease lane must be LIVE even with empty currentWorkspace — the GC would reap a live fleet lane mid-cycle (the isolation-fix regression)")
	}
	if live["cycle-202"] {
		t.Error("stale-lease lane must NOT be live (no host currentWorkspace, lease expired)")
	}
}
