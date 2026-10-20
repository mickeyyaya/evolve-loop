package treefence

import (
	"context"
	"os"
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

func TestRetract_FailsLoudWhenAWriteCannotBeRead(t *testing.T) {
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
	if _, err := snap.Retract(ctx, t.TempDir()); err == nil {
		t.Fatal("Retract must fail when it cannot read a write to keep it")
	}
	if _, err := os.Lstat(filepath.Join(dir, "dirlink")); err != nil {
		t.Errorf("a failed retraction must leave the write in place: %v", err)
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
