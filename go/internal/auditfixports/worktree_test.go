package auditfixports

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/auditfix"
	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

func faultGit(match func(args []string) bool) func(string) gitexec.Git {
	return func(dir string) gitexec.Git {
		return gitexec.Git{Dir: dir, Exec: func(ctx context.Context, name, d string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
			if match(args) {
				return 1, errors.New("injected git fault")
			}
			return sysexec.DefaultRunner(ctx, name, d, args, env, stdin, stdout, stderr)
		}}
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

type buildTree struct {
	repo *gittest.Repo
	wt   Worktree
	base string
}

func newBuildTree(t *testing.T) buildTree {
	t.Helper()
	repo := gittest.Fixture(t)
	repo.Write("a.txt", "a committed\n")
	repo.Write("d.txt", "d committed\n")
	repo.Git("add", "-A")
	repo.Git("commit", "-q", "-m", "base")
	repo.Write("a.txt", "a built\n")
	repo.Write("b.txt", "b built\n")
	wt := Worktree{Git: gitexec.Isolated}
	base, err := wt.Snapshot(context.Background(), repo.Dir)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	return buildTree{repo: repo, wt: wt, base: base}
}

func (b buildTree) fix(t *testing.T) {
	t.Helper()
	b.repo.Write("a.txt", "a fixed\nsecond line\n")
	b.repo.Write("b.txt", "b fixed\n")
	b.repo.Write("c.txt", "c added\n")
	b.repo.Write("café.txt", "a name git quotes by default\n")
	if err := os.Remove(filepath.Join(b.repo.Dir, "d.txt")); err != nil {
		t.Fatal(err)
	}
}

func TestWorktree_DiffMeasuresOnlyTheFixAndRestoreReturnsTheBuiltTree(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	b := newBuildTree(t)
	statusBefore := b.repo.Git("status", "--porcelain")
	b.fix(t)

	patch, err := b.wt.Diff(ctx, b.repo.Dir, b.base)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	want := []auditfix.Change{{Path: "a.txt", Lines: 3}, {Path: "b.txt", Lines: 2}, {Path: "c.txt", Lines: 1}, {Path: "café.txt", Lines: 1}, {Path: "d.txt", Lines: 1}}
	if !slices.Equal(patch.Changes, want) || !strings.Contains(patch.Diff, "+a fixed") || strings.Contains(patch.Diff, "a committed") {
		t.Fatalf("patch = %+v, want only the changes of the fix against the built tree", patch)
	}

	if err := b.wt.Restore(ctx, b.repo.Dir, b.base); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if got := readFile(t, filepath.Join(b.repo.Dir, "a.txt")) + readFile(t, filepath.Join(b.repo.Dir, "b.txt")) + readFile(t, filepath.Join(b.repo.Dir, "d.txt")); got != "a built\nb built\nd committed\n" {
		t.Fatalf("restored files = %q, want the built tree", got)
	}
	for _, added := range []string{"c.txt", "café.txt"} {
		if _, err := os.Stat(filepath.Join(b.repo.Dir, added)); !os.IsNotExist(err) {
			t.Fatalf("the added file %s survived the restore: %v", added, err)
		}
	}
	if after := b.repo.Git("status", "--porcelain"); after != statusBefore {
		t.Fatalf("status after restore = %q, want %q: the index or the tree changed", after, statusBefore)
	}
	if err := b.wt.Restore(ctx, b.repo.Dir, b.base); err != nil {
		t.Fatalf("a restore of an unchanged tree = %v, want nil", err)
	}
}

func TestWorktree_EachGitFailureIsLoud(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	if _, err := (Worktree{Git: gitexec.Isolated}).Snapshot(ctx, t.TempDir()); err == nil {
		t.Fatal("a snapshot outside a git worktree passed")
	}
	isDiff := func(flag string) func([]string) bool {
		return func(args []string) bool { return slices.Contains(args, "diff") && slices.Contains(args, flag) }
	}
	noNumstat := func(args []string) bool {
		return slices.Contains(args, "diff") && !slices.Contains(args, "--numstat") && !slices.Contains(args, "--name-status")
	}
	for name, match := range map[string]func([]string) bool{"numstat": isDiff("--numstat"), "patch text": noNumstat} {
		b := newBuildTree(t)
		b.fix(t)
		if _, err := (Worktree{Git: faultGit(match)}).Diff(ctx, b.repo.Dir, b.base); err == nil {
			t.Errorf("a failing %s diff passed", name)
		}
	}
	for name, match := range map[string]func([]string) bool{
		"name-status": isDiff("--name-status"),
		"restore":     func(args []string) bool { return len(args) > 0 && args[0] == "restore" },
		"snapshot":    func(args []string) bool { return len(args) > 0 && args[0] == "write-tree" },
	} {
		b := newBuildTree(t)
		b.fix(t)
		if err := (Worktree{Git: faultGit(match)}).Restore(ctx, b.repo.Dir, b.base); err == nil {
			t.Errorf("a failing %s passed the restore", name)
		}
		if _, err := (Worktree{Git: faultGit(match)}).Diff(ctx, b.repo.Dir, b.base); name == "snapshot" && err == nil {
			t.Error("a failing snapshot passed the diff")
		}
	}
}

func TestWorktree_RestoreFailsWhenAnAddedFileCannotBeRemoved(t *testing.T) {
	t.Parallel()
	b := newBuildTree(t)
	dir := filepath.Join(b.repo.Dir, "locked")
	b.repo.Write("locked/new.txt", "added by the fixer\n")
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	if err := b.wt.Restore(context.Background(), b.repo.Dir, b.base); err == nil || !strings.Contains(err.Error(), "locked/new.txt") {
		t.Fatalf("Restore = %v, want the path that could not be removed", err)
	}
}

func TestParseNumstat_CountsEachBinarySideAsOneLineAndRefusesGarbage(t *testing.T) {
	t.Parallel()
	got, err := parseNumstat("2\t1\tgo/x.go\n-\t-\tlogo.png\n\nnot a numstat line")
	if err != nil || !slices.Equal(got, []auditfix.Change{{Path: "go/x.go", Lines: 3}, {Path: "logo.png", Lines: 2}}) {
		t.Fatalf("parseNumstat = %+v, %v", got, err)
	}
	if _, err := parseNumstat("x\t1\tgo/x.go"); err == nil {
		t.Fatal("a numstat line with a bad count parsed")
	}
}
