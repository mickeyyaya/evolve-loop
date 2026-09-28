package shipmanifest

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/treefence"
)

func gitRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, args := range [][]string{{"init", "-q"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"}} {
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	return root
}

func gitAt(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
	return string(out)
}

func TestTakeShipTree_RetriesWithoutThePathsGitRefusesAsIgnored(t *testing.T) {
	root := gitRepo(t)
	writeUnder(t, root, "a.txt", "a")
	writeUnder(t, root, "blind.txt", "blind")
	ws := t.TempDir()
	writeUnder(t, ws, "build-report.md", "`a.txt` and `blind.txt`\n")
	var calls [][]string
	take := func(_ context.Context, worktree string, paths []string) (treefence.Snapshot, error) {
		calls = append(calls, paths)
		if slices.Contains(paths, "blind.txt") {
			return treefence.Snapshot{}, errors.New("treefence: git add: exit status 1: " + refusalHeader + "blind.txt\nhint: Use -f if you really want to add them.")
		}
		return treefence.Snapshot{Worktree: worktree, Tree: "retried"}, nil
	}
	snap, err := takeShipTree(context.Background(), GitIn(context.Background(), root), root, ws, take)
	if err != nil || snap.Tree != "retried" {
		t.Fatalf("a path the ignore probe missed but git refused is dropped, as Ship drops it: %+v %v", snap, err)
	}
	if want := [][]string{{"a.txt", "blind.txt"}, {"a.txt"}}; !slices.EqualFunc(calls, want, slices.Equal[[]string]) {
		t.Fatalf("snapshot attempts %v, want %v", calls, want)
	}
}

func TestTakeShipTree_AnEmptySelectionIsTheIndexAndTheTrackedEdits(t *testing.T) {
	root := gitRepo(t)
	writeUnder(t, root, "base.txt", "base\n")
	gitAt(t, root, "add", "base.txt")
	gitAt(t, root, "commit", "-q", "-m", "base")
	writeUnder(t, root, "base.txt", "base\nedit\n")
	writeUnder(t, root, "residue.txt", "residue\n")
	var calls [][]string
	take := func(ctx context.Context, worktree string, paths []string) (treefence.Snapshot, error) {
		calls = append(calls, paths)
		return treefence.TakeStaged(ctx, worktree, paths)
	}
	snap, err := takeShipTree(context.Background(), GitIn(context.Background(), root), root, t.TempDir(), take)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0] != nil {
		t.Fatalf("an empty selection still takes one snapshot, with no declared paths: %v", calls)
	}
	if names := gitAt(t, root, "ls-tree", "-r", "--name-only", snap.Tree); names != "base.txt\n" {
		t.Fatalf("a report-less cycle ships its tracked edits and adopts no residue: %q", names)
	}
}

func TestTakeShipTree_IsTheTreeShipWillCommit(t *testing.T) {
	root := gitRepo(t)
	writeUnder(t, root, "go/source.go", "package source\n")
	gitAt(t, root, "add", "-A")
	gitAt(t, root, "commit", "-q", "-m", "base")
	writeUnder(t, root, "go/source.go", "package source\n\nfunc Edited() {}\n")
	writeUnder(t, root, "docs/explain/cycle-9.md", "# Why\n")
	writeUnder(t, root, "residue.txt", "another lane's scratch\n")
	ws := t.TempDir()
	writeUnder(t, ws, "build-report.md", "Wrote `docs/explain/cycle-9.md`.\n")
	snap, err := TakeShipTree(context.Background(), GitIn(context.Background(), root), root, ws)
	if err != nil {
		t.Fatal(err)
	}
	if names := gitAt(t, root, "ls-tree", "-r", "--name-only", snap.Tree); names != "docs/explain/cycle-9.md\ngo/source.go\n" {
		t.Fatalf("the declared untracked document and the undeclared tracked edit ship; the residue does not: %q", names)
	}
	if got := gitAt(t, root, "show", snap.Tree+":go/source.go"); got != "package source\n\nfunc Edited() {}\n" {
		t.Fatalf("the tracked edit's content is the worktree's: %q", got)
	}
}

func TestTakeShipTree_ADeclaredFileRemovedWithPlainRmShipsItsDeletion(t *testing.T) {
	root := gitRepo(t)
	writeUnder(t, root, "go/old.go", "package old\n")
	writeUnder(t, root, "go/kept.go", "package kept\n")
	gitAt(t, root, "add", "-A")
	gitAt(t, root, "commit", "-q", "-m", "base")
	if err := os.Remove(filepath.Join(root, "go", "old.go")); err != nil {
		t.Fatal(err)
	}
	ws := t.TempDir()
	writeUnder(t, ws, "build-report.md", "Deleted `go/old.go`.\n")
	snap, err := TakeShipTree(context.Background(), GitIn(context.Background(), root), root, ws)
	if err != nil {
		t.Fatalf("a refactor that deletes a declared file with rm has a ship tree: %v", err)
	}
	if names := gitAt(t, root, "ls-tree", "-r", "--name-only", snap.Tree); names != "go/kept.go\n" {
		t.Fatalf("the declared deletion ships: %q", names)
	}
}
