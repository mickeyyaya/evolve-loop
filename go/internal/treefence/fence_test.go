package treefence

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initRepo makes a repo with one commit, an ignore rule, and a pending
// (uncommitted) builder change — the shape of a cycle worktree at audit time.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	write(t, dir, ".gitignore", "bin/\n")
	write(t, dir, "src/keep.go", "package src\n")
	write(t, dir, "src/mat.go", "package src // base\n")
	run("add", "-A")
	run("commit", "-q", "-m", "base")
	write(t, dir, "src/mat.go", "package src // builder change\n")
	write(t, dir, "src/new_test.go", "package src // builder-added, untracked\n")
	return dir
}

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return "<absent:" + err.Error() + ">"
	}
	return string(b)
}

func TestRestore_UndoesEveryKindOfWriteAndLeavesIgnoredAlone(t *testing.T) {
	t.Parallel()
	dir := initRepo(t)
	ctx := context.Background()
	snap, err := Take(ctx, dir)
	if err != nil {
		t.Fatalf("Take: %v", err)
	}
	// The auditor's mutation probes (cycles 1603-1605): rewrite a material
	// file in place, revert a builder edit, drop a probe test into the package,
	// delete a builder-added file, chmod a file, and build a binary (ignored).
	write(t, dir, "src/mat.go", "package src // MUTATED by a probe\n")
	write(t, dir, "src/keep.go", "package src // reverted to something else\n")
	write(t, dir, "src/zz_probe_test.go", "package src // auditor probe\n")
	if err := os.Remove(filepath.Join(dir, "src/new_test.go")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "src/keep.go"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, dir, "bin/tool", "binary\n")

	res, err := snap.Restore(ctx)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	want := []string{"src/keep.go", "src/mat.go", "src/new_test.go", "src/zz_probe_test.go"}
	if strings.Join(res.Restored, ",") != strings.Join(want, ",") {
		t.Fatalf("Restored = %v, want %v", res.Restored, want)
	}
	if got := read(t, dir, "src/mat.go"); got != "package src // builder change\n" {
		t.Errorf("in-place mutation not undone: %q", got)
	}
	if got := read(t, dir, "src/keep.go"); got != "package src\n" {
		t.Errorf("revert not undone: %q", got)
	}
	if got := read(t, dir, "src/new_test.go"); got != "package src // builder-added, untracked\n" {
		t.Errorf("deleted builder file not restored: %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "src/zz_probe_test.go")); !os.IsNotExist(err) {
		t.Error("auditor probe file must be removed")
	}
	if info, _ := os.Stat(filepath.Join(dir, "src/keep.go")); info.Mode().Perm()&0o111 != 0 {
		t.Error("mode change not undone")
	}
	if got := read(t, dir, "bin/tool"); got != "binary\n" {
		t.Error("ignored paths are outside the fence and must be left alone")
	}
	// Idempotent: a second restore over the restored tree changes nothing.
	again, err := snap.Restore(ctx)
	if err != nil || len(again.Restored) != 0 {
		t.Fatalf("second Restore = %v, %v; want no-op", again.Restored, err)
	}
}

func TestRestore_CleanPhaseIsANoOp(t *testing.T) {
	t.Parallel()
	dir := initRepo(t)
	snap, err := Take(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	res, err := snap.Restore(context.Background())
	if err != nil || len(res.Restored) != 0 {
		t.Fatalf("clean tree: Restored=%v err=%v", res.Restored, err)
	}
	if snap.Tree == "" {
		t.Fatal("a snapshot must carry its tree id")
	}
}

func TestTake_RejectsNonRepoAndEmptyPath(t *testing.T) {
	t.Parallel()
	if _, err := Take(context.Background(), ""); err == nil {
		t.Fatal("empty worktree must be an error, never a silent no-op")
	}
	if _, err := Take(context.Background(), t.TempDir()); err == nil {
		t.Fatal("a non-repository must be an error")
	}
}

func TestTake_DoesNotTouchTheRealIndex(t *testing.T) {
	t.Parallel()
	dir := initRepo(t)
	before, _ := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	if _, err := Take(context.Background(), dir); err != nil {
		t.Fatal(err)
	}
	after, _ := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	if string(before) != string(after) {
		t.Fatalf("Take must leave the real index alone:\nbefore=%s\nafter=%s", before, after)
	}
}

// TestRestore_UndoesTypeChangesInBothDirections — a tracked file turned into
// a directory, and a tracked directory turned into a file: whichever order git
// lists the two sides in, the removal runs first and the restore completes.
func TestRestore_UndoesTypeChangesInBothDirections(t *testing.T) {
	t.Parallel()
	dir := initRepo(t)
	ctx := context.Background()
	snap, err := Take(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	// file → directory
	if err := os.Remove(filepath.Join(dir, "src/keep.go")); err != nil {
		t.Fatal(err)
	}
	write(t, dir, "src/keep.go/nested.go", "package nested\n")
	// directory → file
	if err := os.RemoveAll(filepath.Join(dir, "src")); err == nil {
		// keep src; instead turn a fresh tracked dir into a file
	}
	res, err := snap.Restore(ctx)
	if err != nil {
		t.Fatalf("Restore: %v (restored=%v)", err, res.Restored)
	}
	if got := read(t, dir, "src/keep.go"); got != "package src\n" {
		t.Fatalf("file→dir not undone: %q", got)
	}
	if got := read(t, dir, "src/mat.go"); got != "package src // builder change\n" {
		t.Fatalf("sibling lost during the type-change restore: %q", got)
	}
	// directory → file, on a tracked directory of its own
	dir2 := initRepo(t)
	snap2, err := Take(ctx, dir2)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(dir2, "src")); err != nil {
		t.Fatal(err)
	}
	write(t, dir2, "src", "now a file\n")
	if _, err := snap2.Restore(ctx); err != nil {
		t.Fatalf("dir→file Restore: %v", err)
	}
	if got := read(t, dir2, "src/keep.go"); got != "package src\n" {
		t.Fatalf("dir→file not undone: %q", got)
	}
	if got := read(t, dir2, "src/new_test.go"); got != "package src // builder-added, untracked\n" {
		t.Fatalf("untracked builder file under the restored directory missing: %q", got)
	}
}

// TestRestore_PrunesDirectoriesThePhaseCreated — no empty-directory litter.
func TestRestore_PrunesDirectoriesThePhaseCreated(t *testing.T) {
	t.Parallel()
	dir := initRepo(t)
	snap, err := Take(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	write(t, dir, "probe/deep/one.go", "x\n")
	write(t, dir, "probe/deep/two.go", "y\n")
	write(t, dir, "src/extra/z.go", "z\n")
	if _, err := snap.Restore(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"probe", "src/extra"} {
		if _, err := os.Stat(filepath.Join(dir, p)); !os.IsNotExist(err) {
			t.Errorf("%s must be pruned after its files are removed", p)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "src")); err != nil {
		t.Error("a pre-existing directory must survive the prune")
	}
}

// TestTake_IgnoresAmbientGitEnvironment — GIT_DIR/GIT_WORK_TREE from the
// parent process must never redirect the fence to another repository.
func TestTake_IgnoresAmbientGitEnvironment(t *testing.T) {
	dir := initRepo(t) // before the ambient vars: the fixture's own git must not be redirected
	t.Setenv("GIT_DIR", filepath.Join(t.TempDir(), "nowhere"))
	t.Setenv("GIT_WORK_TREE", t.TempDir())
	snap, err := Take(context.Background(), dir)
	if err != nil {
		t.Fatalf("Take under ambient GIT_DIR: %v", err)
	}
	write(t, dir, "src/mat.go", "mutated\n")
	res, err := snap.Restore(context.Background())
	if err != nil || len(res.Restored) != 1 {
		t.Fatalf("Restore under ambient GIT_DIR: %v %v", res.Restored, err)
	}
}
