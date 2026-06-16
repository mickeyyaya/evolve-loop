package treediff

import (
	"context"
	"testing"
)

// TestGuard_DetectsLeakAcrossSnapshotAndCheck names the treediff.Guard type (New
// returns *Guard but the type is never named in a test) and pins the leak-
// detection contract end-to-end: a path dirty AFTER the phase but not BEFORE is
// reported as a leak (CheckResult.OK()==false, path in Leaked).
func TestGuard_DetectsLeakAcrossSnapshotAndCheck(t *testing.T) {
	var g *Guard = New(fakeGit([]string{}, []string{"docs/leaked.md"}, nil))

	before, err := g.Snapshot(context.Background(), "/repo")
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	res := g.Check(context.Background(), "/repo", before)
	if res.OK() {
		t.Fatal("Guard must flag a path newly-dirtied during the phase as a leak")
	}
	if !equalLeaks(res.Leaked, []string{"docs/leaked.md"}) {
		t.Errorf("Leaked = %v, want [docs/leaked.md]", res.Leaked)
	}
}
