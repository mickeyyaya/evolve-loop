package treefence

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRetract_KeepsThePhaseWritesAsEvidenceAndRestoresTheDispatchTree(t *testing.T) {
	t.Parallel()
	dir := initRepo(t)
	ctx := context.Background()
	snap, err := Take(ctx, dir)
	if err != nil {
		t.Fatalf("Take: %v", err)
	}
	write(t, dir, "go/acs/cycle7/amplified_test.go", "package cycle7 // broken meta-test\n")
	write(t, dir, "src/mat.go", "package src // phase rewrote the builder file\n")
	if err := os.Remove(filepath.Join(dir, "src/new_test.go")); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(t.TempDir(), "retracted")

	res, err := snap.Retract(ctx, keep)
	if err != nil {
		t.Fatalf("Retract: %v", err)
	}

	want := "go/acs/cycle7/amplified_test.go,src/mat.go,src/new_test.go"
	if got := strings.Join(res.Restored, ","); got != want {
		t.Fatalf("Restored = %s, want %s", got, want)
	}
	if got := read(t, dir, "go/acs/cycle7/amplified_test.go"); !strings.HasPrefix(got, "<absent:") {
		t.Errorf("the phase-added test must leave the worktree, got %q", got)
	}
	if got := read(t, dir, "src/mat.go"); got != "package src // builder change\n" {
		t.Errorf("a builder file the phase rewrote must come back byte-identical, got %q", got)
	}
	if got := read(t, dir, "src/new_test.go"); got != "package src // builder-added, untracked\n" {
		t.Errorf("a builder file the phase deleted must come back byte-identical, got %q", got)
	}
	if got := read(t, keep, "go/acs/cycle7/amplified_test.go"); got != "package cycle7 // broken meta-test\n" {
		t.Errorf("the retracted test must be kept as evidence, got %q", got)
	}
	if got := read(t, keep, "src/mat.go"); got != "package src // phase rewrote the builder file\n" {
		t.Errorf("the phase's version of a rewritten file must be kept as evidence, got %q", got)
	}
	if got := read(t, keep, "src/new_test.go"); !strings.HasPrefix(got, "<absent:") {
		t.Errorf("a path the phase deleted has no phase content to keep, got %q", got)
	}
}

func TestRetract_AnUntouchedTreeKeepsNothing(t *testing.T) {
	t.Parallel()
	dir := initRepo(t)
	ctx := context.Background()
	snap, err := Take(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(t.TempDir(), "retracted")

	res, err := snap.Retract(ctx, keep)
	if err != nil || len(res.Restored) != 0 {
		t.Fatalf("Retract = %+v, %v; want nothing restored", res, err)
	}
	if _, err := os.Stat(keep); !os.IsNotExist(err) {
		t.Errorf("no evidence directory for an untouched tree, stat err = %v", err)
	}
}

func TestRetract_AnEvidenceFailureRestoresNothing(t *testing.T) {
	t.Parallel()
	dir := initRepo(t)
	ctx := context.Background()
	snap, err := Take(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	write(t, dir, "go/acs/cycle7/amplified_test.go", "package cycle7\n")
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := snap.Retract(ctx, blocker); err == nil {
		t.Fatal("Retract must fail when it cannot keep the evidence")
	}
	if got := read(t, dir, "go/acs/cycle7/amplified_test.go"); got != "package cycle7\n" {
		t.Errorf("a retraction that cannot keep its evidence must leave the tree as the phase left it, got %q", got)
	}
}

func TestRetract_RefusesAStagedSnapshot(t *testing.T) {
	t.Parallel()
	dir := initRepo(t)
	snap, err := TakeStaged(context.Background(), dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := snap.Retract(context.Background(), t.TempDir()); err == nil {
		t.Fatal("a staged snapshot cannot restore a worktree, so it cannot retract one")
	}
}

func TestRetract_FailsLoudWhenTheTreeCannotBeRead(t *testing.T) {
	t.Parallel()
	dir := initRepo(t)
	ctx := context.Background()
	snap, err := Take(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(dir, ".git")); err != nil {
		t.Fatal(err)
	}
	if _, err := snap.Retract(ctx, t.TempDir()); err == nil {
		t.Fatal("Retract must fail when it cannot snapshot the phase's tree")
	}
}

func TestRetract_FailsLoudOnAnUnknownDispatchTree(t *testing.T) {
	t.Parallel()
	dir := initRepo(t)
	snap := Snapshot{Worktree: dir, Tree: strings.Repeat("0", 39) + "1"}
	if _, err := snap.Retract(context.Background(), t.TempDir()); err == nil {
		t.Fatal("Retract must fail when the dispatch tree does not resolve")
	}
}

func TestRetract_KeepsASymlinkAsALinkAndAnExecutableWithItsMode(t *testing.T) {
	t.Parallel()
	dir := initRepo(t)
	ctx := context.Background()
	snap, err := Take(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("src", filepath.Join(dir, "dirlink")); err != nil {
		t.Fatal(err)
	}
	write(t, dir, "tool.sh", "#!/bin/sh\n")
	if err := os.Chmod(filepath.Join(dir, "tool.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(t.TempDir(), "retracted")

	if _, err := snap.Retract(ctx, keep); err != nil {
		t.Fatalf("a symlink to a directory must not defeat the retraction: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "dirlink")); !os.IsNotExist(err) {
		t.Errorf("the phase's symlink must leave the worktree, lstat err = %v", err)
	}
	if target, err := os.Readlink(filepath.Join(keep, "dirlink")); err != nil || target != "src" {
		t.Errorf("the evidence must keep the link itself, got %q, %v", target, err)
	}
	if info, err := os.Stat(filepath.Join(keep, "tool.sh")); err != nil || info.Mode().Perm() != 0o755 {
		t.Errorf("the evidence must keep the mode of an executable, got %v, %v", info, err)
	}
}

func TestRetract_FailsLoudWhenAWriteCannotBeRead(t *testing.T) {
	dir := initRepo(t)
	ctx := context.Background()
	snap, err := Take(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	write(t, dir, "go/acs/cycle7/amplified_test.go", "package cycle7\n")
	readKept = func(string) ([]byte, error) { return nil, os.ErrPermission }
	t.Cleanup(func() { readKept = os.ReadFile })
	if _, err := snap.Retract(ctx, t.TempDir()); err == nil {
		t.Fatal("Retract must fail when it cannot read a write to keep it")
	}
	if got := read(t, dir, "go/acs/cycle7/amplified_test.go"); got != "package cycle7\n" {
		t.Errorf("a failed retraction must leave the write in place, got %q", got)
	}
}

func TestRetract_FailsLoudWhenALinkCannotBeKept(t *testing.T) {
	t.Parallel()
	dir := initRepo(t)
	ctx := context.Background()
	snap, err := Take(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("src", filepath.Join(dir, "dirlink")); err != nil {
		t.Fatal(err)
	}
	keep := t.TempDir()
	if err := os.WriteFile(filepath.Join(keep, "dirlink"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := snap.Retract(ctx, keep); err == nil {
		t.Fatal("Retract must fail when it cannot keep the link")
	}
}

func TestRetract_FailsLoudWhenTheEvidenceFileCannotBeWritten(t *testing.T) {
	t.Parallel()
	dir := initRepo(t)
	ctx := context.Background()
	snap, err := Take(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	write(t, dir, "go/acs/cycle7/amplified_test.go", "package cycle7\n")
	keep := t.TempDir()
	if err := os.MkdirAll(filepath.Join(keep, "go/acs/cycle7/amplified_test.go"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := snap.Retract(ctx, keep); err == nil {
		t.Fatal("Retract must fail when it cannot write the evidence copy")
	}
}

func TestRetract_RemovesANestedRepositoryAndAPathWhoseParentBecameAFile(t *testing.T) {
	t.Parallel()
	dir := initRepo(t)
	ctx := context.Background()
	snap, err := Take(ctx, dir)
	if err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(dir, "nested")
	if out, err := exec.Command("git", "init", "-q", nested).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	write(t, nested, "x.txt", "x\n")
	if out, err := exec.Command("git", "-C", nested, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "n").CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, out)
	}
	if err := os.RemoveAll(filepath.Join(dir, "src")); err != nil {
		t.Fatal(err)
	}
	write(t, dir, "src", "now a file\n")

	keep := filepath.Join(t.TempDir(), "retracted")
	if _, err := snap.Retract(ctx, keep); err != nil {
		t.Fatalf("Retract: %v", err)
	}
	if _, err := os.Stat(filepath.Join(keep, "nested")); !os.IsNotExist(err) {
		t.Errorf("a nested repository has no content to keep, stat err = %v", err)
	}
	if _, err := os.Stat(nested); !os.IsNotExist(err) {
		t.Errorf("the phase's nested repository must leave the worktree, stat err = %v", err)
	}
	if got := read(t, dir, "src/mat.go"); got != "package src // builder change\n" {
		t.Errorf("a builder file under a replaced directory must come back, got %q", got)
	}
}

func TestRetract_FailsLoudWhenAWriteCannotBeStatedOrItsLinkRead(t *testing.T) {
	for name, fail := range map[string]func(){
		"stat":     func() { lstatKept = func(string) (os.FileInfo, error) { return nil, os.ErrPermission } },
		"readlink": func() { readlinkKept = func(string) (string, error) { return "", os.ErrPermission } },
	} {
		t.Run(name, func(t *testing.T) {
			dir := initRepo(t)
			ctx := context.Background()
			snap, err := Take(ctx, dir)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink("src", filepath.Join(dir, "dirlink")); err != nil {
				t.Fatal(err)
			}
			fail()
			t.Cleanup(func() { lstatKept, readlinkKept = os.Lstat, os.Readlink })
			if _, err := snap.Retract(ctx, t.TempDir()); err == nil {
				t.Fatalf("%s: Retract must fail when it cannot keep the write", name)
			}
			if _, err := os.Lstat(filepath.Join(dir, "dirlink")); err != nil {
				t.Errorf("%s: a failed retraction must leave the write in place: %v", name, err)
			}
		})
	}
}
