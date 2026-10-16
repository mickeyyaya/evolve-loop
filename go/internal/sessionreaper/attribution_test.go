package sessionreaper

import (
	"context"
	"os"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/sessionrecord"
)

func TestReapOrphans_AttributionDiscoverableAfterTombstone(t *testing.T) {
	evolveDir, runDir := makeRun(t, "attr", "evolve-bridge-attr")

	killer := &countingKiller{}
	if _, err := ReapOrphans(context.Background(), evolveDir, Options{Kill: killer.kill}); err != nil {
		t.Fatal(err)
	}
	if len(killer.calls) != 1 || killer.calls[0] != "evolve-bridge-attr" {
		t.Fatalf("precondition: reap should kill the one stale session, killed=%v", killer.calls)
	}

	if _, err := os.Stat(sessionrecord.PathIn(runDir)); !os.IsNotExist(err) {
		t.Fatalf("precondition: live registry should be tombstoned after a full reap, stat err=%v", err)
	}

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
