package cycleoutcome

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxstamps"
)

func TestApplyFailure_TheProtectedRouteWriteIsAStampThatSyncMainLands(t *testing.T) {
	root, inbox, _ := runProtectedScenario(t, defaultScenario())
	routed, err := inboxmover.FindFileByTaskID(inbox, routedItem)
	if err != nil {
		t.Fatal(err)
	}
	written, err := os.ReadFile(routed)
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(root, routed)
	if err != nil {
		t.Fatal(err)
	}
	plane := gittest.Fixture(t)
	plane.Write(rel, routedFixture)
	plane.Git("add", "-A")
	plane.Git("commit", "-q", "-m", "the item before the cycle")
	plane.Write(rel, string(written))

	partition, err := inboxstamps.Classify(context.Background(), gitexec.Default(plane.Dir))

	if err != nil || len(partition.Other) != 0 || len(partition.Stamps) != 1 || partition.Stamps[0].Path != filepath.ToSlash(rel) {
		t.Errorf("Classify = (%+v, %v): the loop's protected route must change stamp fields only, so sync-main lands it instead of refusing the boundary", partition, err)
	}
}
