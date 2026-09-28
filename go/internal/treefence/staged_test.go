//go:build integration

package treefence

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTakeStaged_IsTheRealIndexEveryTrackedEditAndTheDeclaredPaths(t *testing.T) {
	root := initRepo(t)
	ctx := context.Background()
	index := filepath.Join(root, ".git", "index")
	before, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	full, err := Take(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	declared, err := TakeStaged(ctx, root, []string{"src/new_test.go"})
	if err != nil || declared.Tree != full.Tree {
		t.Fatalf("the declared untracked file plus every tracked edit is the full tree (%v): %s vs %s", err, declared.Tree, full.Tree)
	}
	none, err := TakeStaged(ctx, root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if none.Tree == full.Tree {
		t.Fatal("an undeclared untracked file never enters the staged tree")
	}
	if got, err := git(ctx, root, nil, "show", none.Tree+":src/mat.go"); err != nil || !strings.Contains(got, "builder change") {
		t.Fatalf("an unstaged tracked edit is in the staged tree, as the audit binding's add -u stages it (%v): %q", err, got)
	}
	if _, err := git(ctx, root, nil, "show", none.Tree+":src/new_test.go"); err == nil {
		t.Fatal("an empty pathspec adds the tracked edits only, never the untracked file")
	}
	if after, err := os.ReadFile(index); err != nil || string(after) != string(before) {
		t.Fatalf("the staged snapshot never touches the real index (%v)", err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("staged, undeclared\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := git(ctx, root, nil, "add", "notes.txt"); err != nil {
		t.Fatal(err)
	}
	withStaged, err := TakeStaged(ctx, root, []string{"src/new_test.go"})
	if err != nil {
		t.Fatal(err)
	}
	withAll, err := Take(ctx, root)
	if err != nil || withStaged.Tree != withAll.Tree {
		t.Fatalf("what the real index already stages ships too, declared or not (%v)", err)
	}
	if _, err := withStaged.Restore(ctx); err == nil {
		t.Fatal("a staged snapshot never restores a worktree: only a full snapshot knows every path")
	}
	if err := os.Remove(index); err != nil {
		t.Fatal(err)
	}
	if _, err := TakeStaged(ctx, root, []string{"src/new_test.go"}); err == nil {
		t.Fatal("a staged snapshot without a readable real index is a refusal, never a narrowed tree")
	}
	if _, err := TakeStaged(ctx, "", nil); err == nil {
		t.Fatal("an empty worktree path is refused")
	}
	if _, err := TakeStaged(ctx, t.TempDir(), nil); err == nil {
		t.Fatal("a non-repository is refused")
	}
}

func TestTakeStaged_ADeclaredFileDeletedButNotStagedIsADeletion(t *testing.T) {
	root := initRepo(t)
	ctx := context.Background()
	if err := os.Remove(filepath.Join(root, "src", "keep.go")); err != nil {
		t.Fatal(err)
	}
	snap, err := TakeStaged(ctx, root, []string{"src/keep.go", "src/new_test.go"})
	if err != nil {
		t.Fatalf("a declared file removed with plain rm is a deletion to stage, not a pathspec that matches nothing: %v", err)
	}
	full, err := Take(ctx, root)
	if err != nil || snap.Tree != full.Tree {
		t.Fatalf("the declared deletion, the declared new file and the tracked edit are the full tree (%v): %s vs %s", err, snap.Tree, full.Tree)
	}
}
