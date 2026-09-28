//go:build integration

package treefence

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTakeStaged_IsTheRealIndexPlusTheDeclaredPaths(t *testing.T) {
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
	both, err := TakeStaged(ctx, root, []string{"src/new_test.go", "src/mat.go"})
	if err != nil || both.Tree != full.Tree {
		t.Fatalf("declaring every change stages the full tree (%v): %s vs %s", err, both.Tree, full.Tree)
	}
	onlyNew, err := TakeStaged(ctx, root, []string{"src/new_test.go"})
	if err != nil || onlyNew.Tree == full.Tree {
		t.Fatalf("an undeclared tracked edit stays out of the staged tree (%v)", err)
	}
	tracked, err := TakeTracked(ctx, root)
	if err != nil || onlyNew.Tree == tracked.Tree {
		t.Fatalf("the staged tree is neither the full nor the tracked tree: it holds the declared new file without the undeclared edit (%v)", err)
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
	stagedIndex, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	withStaged, err := TakeStaged(ctx, root, []string{"src/new_test.go", "src/mat.go"})
	if err != nil {
		t.Fatal(err)
	}
	if after, err := os.ReadFile(index); err != nil || string(after) != string(stagedIndex) {
		t.Fatalf("the staged snapshot never touches a real index that already stages a path (%v)", err)
	}
	withAll, err := Take(ctx, root)
	if err != nil || withStaged.Tree != withAll.Tree {
		t.Fatalf("what the real index already stages ships too, declared or not (%v)", err)
	}
	if _, err := withStaged.Restore(ctx); err == nil {
		t.Fatal("a staged snapshot never restores a worktree: only a full snapshot knows every path")
	}
	if tracked, err := TakeTracked(ctx, root); err != nil {
		t.Fatal(err)
	} else if _, err := tracked.Restore(ctx); err == nil {
		t.Fatal("a tracked snapshot never restores a worktree either")
	}
	if _, err := git(ctx, root, nil, "rm", "-q", "--cached", "notes.txt"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "notes.txt")); err != nil {
		t.Fatal(err)
	}
	none, err := TakeStaged(ctx, root, nil)
	if err != nil {
		t.Fatal(err)
	}
	head, err := git(ctx, root, nil, "rev-parse", "HEAD^{tree}")
	if err != nil || none.Tree != strings.TrimSpace(head) {
		t.Fatalf("an empty pathspec adds nothing, never the whole tree (%v): %s vs HEAD %s", err, none.Tree, head)
	}
	if err := os.Remove(index); err != nil {
		t.Fatal(err)
	}
	if _, err := TakeStaged(ctx, root, []string{"src/new_test.go"}); err == nil {
		t.Fatal("a staged snapshot without a readable real index is a refusal, never a narrowed tree")
	}
}
