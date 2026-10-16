package treediff

import (
	"context"
	"testing"
)

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
