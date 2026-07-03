package gc

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/runlease"
)

// discover_fleet_lease_test.go — regression pin for the fleet cycle-state
// isolation fix (2026-07-03). Under fleet each lane writes cycle state to its
// OWN per-run file (ipcenv.CycleStateFileKey), so the host-global
// .evolve/cycle-state.json is absent/stale and currentWorkspace() returns "".
// The GC must NOT reap a live lane's run dir on that basis: the per-run .lease
// (ADR-0049 G16) is the independent liveness signal in
// `Live: dir == currentWS || leaseFresh(dir)`. This test proves that with an
// EMPTY currentWorkspace, a fresh-lease lane stays Live and a stale-lease lane
// does not — the exact protection that keeps concurrent fleet lanes from being
// deleted mid-cycle.
func TestDiscover_EmptyCurrentWorkspace_FreshLeaseStaysLive(t *testing.T) {
	dir := t.TempDir()
	// No cycle-state.json at the host root ⇒ currentWorkspace() == "" (the fleet
	// case, where lanes wrote per-run files instead of the host singleton).
	t0 := time.Unix(1_700_000_000, 0)

	liveLane := mkRun(t, dir, "cycle-201", t0.Add(-time.Hour)) // old mtime, but heartbeating
	deadLane := mkRun(t, dir, "cycle-202", t0.Add(-time.Hour))
	// Every real fleet lane writes run.json (the CB.4 mirror) — the run marker
	// hasRunMarker() requires. Without it Discover skips the dir as noise, so
	// the fixture must carry it to exercise the liveness classification at all.
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
