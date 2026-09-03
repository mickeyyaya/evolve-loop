package treefence

import (
	"context"
	"testing"
)

// TestAPICoverNamedExports names and EXERCISES every exported symbol of this
// package (ADR-0069 new-package graduation) through the shapes its consumer —
// the phase runner's worktree fence — relies on.
func TestAPICoverNamedExports(t *testing.T) {
	t.Parallel()
	dir := initRepo(t)
	snap, err := Take(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	var _ Snapshot = snap
	res, err := snap.Restore(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var _ Result = res
	if snap.Worktree != dir || snap.Tree == "" || len(res.Restored) != 0 {
		t.Fatalf("unexpected shapes: %+v %+v", snap, res)
	}
}
