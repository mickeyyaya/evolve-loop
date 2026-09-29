package explanationdocs

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiveSupersededPredicatePackages_ArchivesOnlyTheAncestorsOwnPackage(t *testing.T) {
	f := newFixture(t)
	f.write(t, "go/acs/cycle10/existing_test.go", "package cycle10\n")
	f.git(t, "add", "-A")
	f.git(t, "commit", "-q", "-m", "main already holds an old cycle package")
	base := f.git(t, "rev-parse", "HEAD")
	ancestor := "go/acs/cycle41/predicates_test.go"
	f.write(t, ancestor, "//go:build acs\n\npackage cycle41\n")
	f.write(t, "go/acs/cycle42/predicates_test.go", "//go:build acs\n\npackage cycle42\n")
	f.write(t, "go/acs/cycle10/existing_test.go", "package cycle10\n\nvar edited = true\n")
	f.git(t, "add", "-A")
	f.git(t, "commit", "-q", "-m", "failed attempt snapshot")

	archived, err := ArchiveSupersededPredicatePackages(context.Background(), f.worktree, base, 42)

	if err != nil {
		t.Fatalf("ArchiveSupersededPredicatePackages: %v", err)
	}
	if len(archived) != 1 || !strings.HasPrefix(archived[0], "docs/private/research/archived-") || !strings.HasSuffix(archived[0], "/superseded-predicate-packages/cycle41") {
		t.Fatalf("archived=%v, want the one ancestor package under the dated research archive", archived)
	}
	if _, err := os.Stat(filepath.Join(f.worktree, filepath.FromSlash(ancestor))); !os.IsNotExist(err) {
		t.Errorf("the ancestor's package is still at its canonical path: %v", err)
	}
	if body, err := os.ReadFile(filepath.Join(f.worktree, filepath.FromSlash(archived[0]), "predicates_test.go")); err != nil || !strings.Contains(string(body), "package cycle41") {
		t.Errorf("archived predicate body=%q err=%v", body, err)
	}
	for _, kept := range []string{"go/acs/cycle42/predicates_test.go", "go/acs/cycle10/existing_test.go"} {
		if _, err := os.Stat(filepath.Join(f.worktree, filepath.FromSlash(kept))); err != nil {
			t.Errorf("%s was moved: this cycle's own package and one already on main are never archived", kept)
		}
	}
	paths, err := changedSince(context.Background(), f.worktree, base)
	if err != nil {
		t.Fatal(err)
	}
	if contains(paths, ancestor) {
		t.Errorf("the ancestor's predicate file remains in the base-bound diff: %v", paths)
	}
}

func TestArchiveSupersededPredicatePackages_NothingToArchiveIsANoOp(t *testing.T) {
	f := newFixture(t)
	f.write(t, "go/acs/cycle42/predicates_test.go", "//go:build acs\n\npackage cycle42\n")
	f.git(t, "add", "-A")
	f.git(t, "commit", "-q", "-m", "this cycle's own package")

	archived, err := ArchiveSupersededPredicatePackages(context.Background(), f.worktree, f.base, 42)

	if err != nil || len(archived) != 0 {
		t.Errorf("archived=%v err=%v, want nothing", archived, err)
	}
}
