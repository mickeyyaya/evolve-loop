package shipmanifest

import (
	"context"
	"errors"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/treefence"
)

func TestTakeShipTree_AFailedStatusReadStopsBeforeAnySnapshotIsTaken(t *testing.T) {
	boom := errors.New("git status failed")
	read := func(args ...string) (string, error) { return "", boom }
	taken := false
	take := func(context.Context, string, []string) (treefence.Snapshot, error) {
		taken = true
		return treefence.Snapshot{}, nil
	}

	snap, err := takeShipTree(context.Background(), read, t.TempDir(), "", take)

	if !errors.Is(err, boom) || snap.Tree != "" || snap.Worktree != "" {
		t.Fatalf("snap=%+v err=%v", snap, err)
	}
	if taken {
		t.Error("no snapshot may be taken when the selection could not be read")
	}
}
