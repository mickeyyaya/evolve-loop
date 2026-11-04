package treefence

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWithWorkspace_NilFenceStaysNil(t *testing.T) {
	t.Parallel()
	var f *Fence
	if got := f.WithWorkspace(t.TempDir()); got != nil {
		t.Errorf("WithWorkspace on a nil fence = %v, want nil", got)
	}
}

func TestSavePatch_RecordsTheFailureThatStoppedIt(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := initRepo(t)
	snap, err := Take(ctx, dir)
	if err != nil {
		t.Fatalf("Take: %v", err)
	}

	badTree := Outcome{workspace: t.TempDir()}
	badTree.savePatch(ctx, dir, snap.Tree, "0000000000000000000000000000000000000000", []string{"src/mat.go"})
	if badTree.patchErr == nil || badTree.patchPath != "" {
		t.Errorf("a git failure must set patchErr and no path: err=%v path=%q", badTree.patchErr, badTree.patchPath)
	}

	workspace := t.TempDir()
	if err := os.Mkdir(filepath.Join(workspace, patchFileName), 0o755); err != nil {
		t.Fatal(err)
	}
	blocked := Outcome{workspace: workspace}
	blocked.savePatch(ctx, dir, snap.Tree, snap.Tree, []string{"src/mat.go"})
	if blocked.patchErr == nil || blocked.patchPath != "" {
		t.Errorf("a write failure must set patchErr and no path: err=%v path=%q", blocked.patchErr, blocked.patchPath)
	}
}
