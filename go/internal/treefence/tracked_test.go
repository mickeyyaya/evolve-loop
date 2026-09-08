//go:build integration

package treefence

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestTakeTracked(t *testing.T) {
	root := initRepo(t)
	index := filepath.Join(root, ".git", "index")
	before, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	tracked, err := TakeTracked(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	full, err := Take(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if tracked.Tree == full.Tree {
		t.Fatal("tracked snapshot adopted the untracked builder test")
	}
	if _, err := git(context.Background(), root, nil, "add", "src/new_test.go"); err != nil {
		t.Fatal(err)
	}
	stagedIndex, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	staged, err := TakeTracked(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if staged.Tree != full.Tree {
		t.Fatal("explicitly staged input did not reach the ship snapshot")
	}
	after, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(stagedIndex) {
		t.Fatal("tracked snapshot modified the real index")
	}
	// A missing seed is a refusal, never an empty successful ship tree.
	if err := os.Remove(index); err != nil {
		t.Fatal(err)
	}
	if _, err := TakeTracked(context.Background(), root); err == nil {
		t.Fatal("missing index silently narrowed the tracked snapshot")
	}
	if err := os.WriteFile(index, before, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := TakeTracked(context.Background(), ""); err == nil {
		t.Fatal("empty worktree accepted")
	}
	if _, err := TakeTracked(context.Background(), t.TempDir()); err == nil {
		t.Fatal("non-repository accepted")
	}
}
