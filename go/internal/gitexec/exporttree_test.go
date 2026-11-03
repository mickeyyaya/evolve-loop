package gitexec_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

func TestExportTree_WritesTheRevisionIntoTheDirectoryWithoutTouchingTheIndex(t *testing.T) {
	r, index := scratchFixture(t)
	base := r.Git("rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(r.Dir, "a.txt"), []byte("lane\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.Git("add", "a.txt")
	before, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()

	tree, err := gitexec.Default(r.Dir).ExportTree(context.Background(), base, dir)

	if err != nil || tree != filepath.Join(dir, "tree") {
		t.Fatalf("ExportTree = %q, %v; want %s", tree, err, filepath.Join(dir, "tree"))
	}
	if got, err := os.ReadFile(filepath.Join(tree, "a.txt")); err != nil || string(got) != "a\n" {
		t.Errorf("exported a.txt = %q, %v; want the base bytes", got, err)
	}
	if after, err := os.ReadFile(index); err != nil || !bytes.Equal(after, before) {
		t.Errorf("the worktree index changed: %v", err)
	}
	if staged := r.Git("diff", "--cached", "--name-only"); staged != "a.txt" {
		t.Errorf("staged = %q, want a.txt still staged", staged)
	}
}

func TestExportTree_ARevisionThatIsNotATreeIsAnError(t *testing.T) {
	r, _ := scratchFixture(t)

	if tree, err := gitexec.Default(r.Dir).ExportTree(context.Background(), "no-such-ref", t.TempDir()); err == nil || tree != "" {
		t.Errorf("ExportTree = %q, %v; want an error", tree, err)
	}
}
