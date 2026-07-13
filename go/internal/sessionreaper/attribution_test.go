package sessionreaper

import (
	"context"
	"os"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/sessionrecord"
)

// TDD RED (cycle-806, task sweep-tombstone-attribution).
//
// End-to-end proof that a session killed by ReapOrphans stays attributable to
// its owning run AFTER the registry is tombstoned. This is the exact accounting
// hole the soak test's Invariant 3 hit: post-reap the live registry is renamed
// to `.reaped`, so sessionrecord.ReadAll(PathIn(runDir)) finds nothing. The
// tombstone-aware sessionrecord.ReadAllResolving must recover it. RED until
// Builder adds ReadAllResolving (compile failure — symbol absent).

func TestReapOrphans_AttributionDiscoverableAfterTombstone(t *testing.T) {
	evolveDir, runDir := makeRun(t, "attr", "evolve-bridge-attr")

	killer := &countingKiller{}
	if _, err := ReapOrphans(context.Background(), evolveDir, Options{Kill: killer.kill}); err != nil {
		t.Fatal(err)
	}
	if len(killer.calls) != 1 || killer.calls[0] != "evolve-bridge-attr" {
		t.Fatalf("precondition: reap should kill the one stale session, killed=%v", killer.calls)
	}

	// The live registry must now be tombstoned (proves we resolve the tombstone,
	// not a still-live file).
	if _, err := os.Stat(sessionrecord.PathIn(runDir)); !os.IsNotExist(err) {
		t.Fatalf("precondition: live registry should be tombstoned after a full reap, stat err=%v", err)
	}

	// Attribution must survive the tombstone via the resolver.
	recs, err := sessionrecord.ReadAllResolving(sessionrecord.PathIn(runDir))
	if err != nil {
		t.Fatalf("ReadAllResolving: %v", err)
	}
	found := false
	for _, r := range recs {
		if r.Session == "evolve-bridge-attr" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("killed session %q not discoverable post-tombstone via ReadAllResolving — attribution lost; got %+v", "evolve-bridge-attr", recs)
	}
}
